// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package socket

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// DNS event field offsets matching C struct dns_event layout
//
//	struct dns_event {
//	    __u64 timestamp;        // 0
//	    __u32 pid;              // 8
//	    __u32 tid;              // 12
//	    __be32 src_ip;          // 16
//	    __be32 dst_ip;          // 20
//	    __u16 src_port;         // 24
//	    __u16 dst_port;         // 26
//	    __u16 query_id;         // 28
//	    __u8 is_response;       // 30
//	    __u16 query_type;       // 31 (unaligned!)
//	    __u16 query_class;      // 33
//	    __u8 response_code;     // 35
//	    __u16 answer_count;     // 36
//	    char domain[253];       // 38
//	    __u16 packet_size;      // 291
//	    __u64 response_time_ns; // 293
//	};
const (
	dnsEventMinSize   = 38 // Minimum size before domain
	dnsEventDomainMax = 253
)

// dnsKey matches struct dns_key in common.h
type dnsKey struct {
	SrcIP   uint32
	DstIP   uint32
	SrcPort uint16
	DstPort uint16
	QueryID uint16
	_       uint16 // Padding
}

// dnsQueryInfo matches struct dns_query_info in common.h
type dnsQueryInfo struct {
	QueryType  uint16
	QueryClass uint16
	Domain     [253]byte
	PacketSize uint16
	Timestamp  uint64
}

// dnsResponseInfo matches struct dns_response_info in common.h
type dnsResponseInfo struct {
	ResponseCode      uint8
	AnswerCount       uint16
	AuthorityCount    uint16
	AdditionalCount   uint16
	QueryTimestamp    uint64
	ResponseTimestamp uint64
	Domain            [253]byte
	PacketSize        uint16
}

// DNSFilter implements a socket filter for DNS monitoring
type DNSFilter struct {
	// Configuration
	config *DNSFilterConfig

	// eBPF components
	program    *ebpf.Program
	attachLink link.Link

	// Maps
	queryMap    *ebpf.Map
	responseMap *ebpf.Map
	statsMap    *ebpf.Map
	eventsMap   *ebpf.Map // Ring buffer for events

	// State
	mutex   sync.RWMutex
	enabled bool

	// Statistics
	stats *DNSFilterStats

	// Event handlers
	queryHandler    DNSQueryHandler
	responseHandler DNSResponseHandler

	// Logger
	logger *logging.Logger

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// DNSFilterConfig holds configuration for DNS socket filter
type DNSFilterConfig struct {
	// Filter settings
	Enabled          bool          `hcl:"enabled,optional"`
	Interface        string        `hcl:"interface,optional"`
	QueryBufferSize  int           `hcl:"query_buffer_size,optional"`
	ResponseTimeout  time.Duration `hcl:"response_timeout,optional"`
	MaxQueriesPerSec int           `hcl:"max_queries_per_sec,optional"`

	// Monitoring settings
	LogQueries        bool `hcl:"log_queries,optional"`
	LogResponses      bool `hcl:"log_responses,optional"`
	TrackResponseTime bool `hcl:"track_response_time,optional"`

	// Filtering settings
	BlockMaliciousDomains bool `hcl:"block_malicious_domains,optional"`
	AllowlistOnly         bool `hcl:"allowlist_only,optional"`
	BlockPrivateDNS       bool `hcl:"block_private_dns,optional"`
}

// DNSFilterStats holds statistics for the DNS filter
type DNSFilterStats struct {
	QueriesProcessed   uint64    `json:"queries_processed"`
	ResponsesProcessed uint64    `json:"responses_processed"`
	QueriesBlocked     uint64    `json:"queries_blocked"`
	ResponsesBlocked   uint64    `json:"responses_blocked"`
	PacketsDropped     uint64    `json:"packets_dropped"`
	Errors             uint64    `json:"errors"`
	LastUpdate         time.Time `json:"last_update"`
}

// DNSQueryHandler handles DNS query events
type DNSQueryHandler func(event *types.DNSQueryEvent) error

// DNSResponseHandler handles DNS response events
type DNSResponseHandler func(event *types.DNSResponseEvent) error

// DefaultDNSFilterConfig returns default configuration
func DefaultDNSFilterConfig() *DNSFilterConfig {
	return &DNSFilterConfig{
		Enabled:               false,
		QueryBufferSize:       65536,
		ResponseTimeout:       5 * time.Second,
		MaxQueriesPerSec:      10000,
		LogQueries:            true,
		LogResponses:          true,
		TrackResponseTime:     true,
		BlockMaliciousDomains: false,
		AllowlistOnly:         false,
		BlockPrivateDNS:       false,
	}
}

// NewDNSFilter creates a new DNS socket filter
func NewDNSFilter(logger *logging.Logger, config *DNSFilterConfig) *DNSFilter {
	if config == nil {
		config = DefaultDNSFilterConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	dns := &DNSFilter{
		config: config,
		stats: &DNSFilterStats{
			LastUpdate: time.Now(),
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	return dns
}

// Start starts the DNS socket filter
func (dns *DNSFilter) Start() error {
	dns.mutex.Lock()
	defer dns.mutex.Unlock()

	if !dns.config.Enabled {
		dns.logger.Info("DNS socket filter disabled")
		return nil
	}

	dns.logger.Info("Starting DNS socket filter")

	// Load eBPF program
	if err := dns.loadProgram(); err != nil {
		return err
	}

	// Start event processing
	go dns.cleanupExpiredQueries()
	go dns.processQueryEvents()
	go dns.processResponseEvents()

	dns.enabled = true
	dns.logger.Info("DNS socket filter started")

	return nil
}

// Stop stops the DNS socket filter
func (dns *DNSFilter) Stop() {
	dns.mutex.Lock()
	defer dns.mutex.Unlock()

	if !dns.enabled {
		return
	}

	dns.logger.Info("Stopping DNS socket filter")

	// Cancel context
	dns.cancel()

	// Detach socket filter
	if dns.attachLink != nil {
		dns.attachLink.Close()
		dns.attachLink = nil
	}

	// Close eBPF maps
	if dns.queryMap != nil {
		dns.queryMap.Close()
		dns.queryMap = nil
	}
	if dns.responseMap != nil {
		dns.responseMap.Close()
		dns.responseMap = nil
	}
	if dns.statsMap != nil {
		dns.statsMap.Close()
		dns.statsMap = nil
	}
	if dns.eventsMap != nil {
		dns.eventsMap.Close()
		dns.eventsMap = nil
	}

	// Close eBPF program
	if dns.program != nil {
		dns.program.Close()
		dns.program = nil
	}

	dns.enabled = false
	dns.logger.Info("DNS socket filter stopped")
}

// loadProgram loads the eBPF program
func (dns *DNSFilter) loadProgram() error {
	dns.logger.Info("Loading DNS socket filter eBPF program")

	// Load compiled eBPF program from file
	collection, err := ebpf.LoadCollectionSpec("dns_socket.o")
	if err != nil {
		return fmt.Errorf("failed to load eBPF collection: %w", err)
	}

	// Load the program
	program, err := ebpf.NewProgram(collection.Programs["dns_socket_filter"])
	if err != nil {
		return fmt.Errorf("failed to create eBPF program: %w", err)
	}
	dns.program = program

	// Load maps from collection
	if queryMap, ok := collection.Maps["dns_queries"]; ok {
		dns.queryMap, err = ebpf.NewMap(queryMap)
		if err != nil {
			return fmt.Errorf("failed to create dns_queries map: %w", err)
		}
	} else {
		return fmt.Errorf("dns_queries map not found in collection")
	}

	if responseMap, ok := collection.Maps["dns_responses"]; ok {
		dns.responseMap, err = ebpf.NewMap(responseMap)
		if err != nil {
			return fmt.Errorf("failed to create dns_responses map: %w", err)
		}
	} else {
		return fmt.Errorf("dns_responses map not found in collection")
	}

	if statsMap, ok := collection.Maps["dns_stats"]; ok {
		dns.statsMap, err = ebpf.NewMap(statsMap)
		if err != nil {
			return fmt.Errorf("failed to create dns_stats map: %w", err)
		}
	} else {
		return fmt.Errorf("dns_stats map not found in collection")
	}

	// Load ring buffer for events
	if eventsMap, ok := collection.Maps["dns_events"]; ok {
		dns.eventsMap, err = ebpf.NewMap(eventsMap)
		if err != nil {
			dns.logger.Warn("Failed to create dns_events ring buffer", "error", err)
			// Not fatal - events will just not be processed
		}
	} else {
		dns.logger.Debug("dns_events ring buffer not found in collection")
	}

	return nil
}

// ... (rest of the code remains the same)

func (dns *DNSFilter) cleanupExpiredQueries() {
	ticker := time.NewTicker(dns.config.ResponseTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-dns.ctx.Done():
			return
		case <-ticker.C:
			// Expired query cleanup is handled kernel-side by the LRU map eviction policy.
			// Userspace cleanup would require iterating the eBPF map via bpf_map_get_next_key,
			// which is not yet exposed through this code path.
			dns.logger.Debug("Query map cleanup tick (LRU-managed)")
		}
	}
}

// SetQueryHandler sets the query event handler
func (dns *DNSFilter) SetQueryHandler(handler func(event *types.DNSQueryEvent) error) {
	dns.queryHandler = handler
}

// SetResponseHandler sets the response event handler
func (dns *DNSFilter) SetResponseHandler(handler func(event *types.DNSResponseEvent) error) {
	dns.responseHandler = handler
}

// GetStatistics returns DNS filter statistics
func (dns *DNSFilter) GetStatistics() interface{} {
	dns.mutex.RLock()
	defer dns.mutex.RUnlock()

	stats := *dns.stats
	return &stats
}

// IsEnabled returns whether the DNS filter is enabled
func (dns *DNSFilter) IsEnabled() bool {
	dns.mutex.RLock()
	defer dns.mutex.RUnlock()
	return dns.enabled
}

// processQueryEvents processes DNS query events from the ring buffer
func (dns *DNSFilter) processQueryEvents() {
	if dns.eventsMap == nil {
		dns.logger.Debug("No events map available, skipping event processing")
		<-dns.ctx.Done()
		return
	}

	// Create ring buffer reader
	rd, err := ringbuf.NewReader(dns.eventsMap)
	if err != nil {
		dns.logger.Error("Failed to create ring buffer reader", "error", err)
		return
	}
	defer rd.Close()

	dns.logger.Info("Started DNS event ring buffer reader")

	// Close reader when context is cancelled
	go func() {
		<-dns.ctx.Done()
		rd.Close()
	}()

	for {
		record, err := rd.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				dns.logger.Debug("Ring buffer reader closed")
				return
			}
			dns.logger.Debug("Ring buffer read error", "error", err)
			continue
		}

		// Parse and dispatch event
		if err := dns.handleRingbufEvent(record.RawSample); err != nil {
			dns.logger.Debug("Failed to handle DNS event", "error", err)
		}
	}
}

// processResponseEvents is now merged into processQueryEvents
// since both queries and responses come through the same ring buffer
func (dns *DNSFilter) processResponseEvents() {
	// Responses are processed in processQueryEvents via handleRingbufEvent
	<-dns.ctx.Done()
}

// handleRingbufEvent parses and dispatches a DNS event from the ring buffer
func (dns *DNSFilter) handleRingbufEvent(data []byte) error {
	if len(data) < dnsEventMinSize {
		return fmt.Errorf("event too short: %d bytes", len(data))
	}

	// Parse event header
	isResponse := data[30] != 0

	if isResponse {
		event, err := dns.parseRingbufResponseEvent(data)
		if err != nil {
			return err
		}

		// Update stats
		dns.mutex.Lock()
		dns.stats.ResponsesProcessed++
		dns.stats.LastUpdate = time.Now()
		dns.mutex.Unlock()

		// Call handler if set
		if dns.responseHandler != nil {
			return dns.responseHandler(event)
		}
	} else {
		event, err := dns.parseRingbufQueryEvent(data)
		if err != nil {
			return err
		}

		// Update stats
		dns.mutex.Lock()
		dns.stats.QueriesProcessed++
		dns.stats.LastUpdate = time.Now()
		dns.mutex.Unlock()

		// Call handler if set
		if dns.queryHandler != nil {
			return dns.queryHandler(event)
		}
	}

	return nil
}

// parseRingbufQueryEvent parses a DNS query event from ring buffer data
func (dns *DNSFilter) parseRingbufQueryEvent(data []byte) (*types.DNSQueryEvent, error) {
	// Parse according to C struct layout (see comments at top of file)
	event := &types.DNSQueryEvent{
		Timestamp:  time.Now(), // Use kernel timestamp if needed: binary.LittleEndian.Uint64(data[0:8])
		PID:        binary.LittleEndian.Uint32(data[8:12]),
		TID:        binary.LittleEndian.Uint32(data[12:16]),
		SourceIP:   net.IP(data[16:20]),
		DestIP:     net.IP(data[20:24]),
		SourcePort: binary.LittleEndian.Uint16(data[24:26]),
		DestPort:   binary.LittleEndian.Uint16(data[26:28]),
		QueryID:    binary.LittleEndian.Uint16(data[28:30]),
		QueryType:  binary.LittleEndian.Uint16(data[31:33]),
		QueryClass: binary.LittleEndian.Uint16(data[33:35]),
	}

	// Extract domain (null-terminated string starting at offset 38)
	domainEnd := 38 + dnsEventDomainMax
	if len(data) < domainEnd {
		domainEnd = len(data)
	}
	for i := 38; i < domainEnd; i++ {
		if data[i] == 0 {
			event.Domain = string(data[38:i])
			break
		}
	}

	// Packet size at offset 291
	if len(data) >= 293 {
		event.PacketSize = binary.LittleEndian.Uint16(data[291:293])
	}

	return event, nil
}

// parseRingbufResponseEvent parses a DNS response event from ring buffer data
func (dns *DNSFilter) parseRingbufResponseEvent(data []byte) (*types.DNSResponseEvent, error) {
	event := &types.DNSResponseEvent{
		Timestamp:    time.Now(),
		QueryID:      binary.LittleEndian.Uint16(data[28:30]),
		ResponseCode: data[35],
		AnswerCount:  binary.LittleEndian.Uint16(data[36:38]),
	}

	// Extract domain
	domainEnd := 38 + dnsEventDomainMax
	if len(data) < domainEnd {
		domainEnd = len(data)
	}
	for i := 38; i < domainEnd; i++ {
		if data[i] == 0 {
			event.Domain = string(data[38:i])
			break
		}
	}

	// Packet size at offset 291, response time at 293
	if len(data) >= 301 {
		event.PacketSize = binary.LittleEndian.Uint16(data[291:293])
		// Convert nanoseconds to time.Duration
		responseTimeNs := binary.LittleEndian.Uint64(data[293:301])
		event.ResponseTime = time.Duration(responseTimeNs) * time.Nanosecond
	}

	return event, nil
}
