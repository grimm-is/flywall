// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package socket

import (
	"bytes"
	"context"
	"encoding/binary"
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

// Local struct definitions removed in favor of types package

// DHCPFilter implements a socket filter for DHCP monitoring
type DHCPFilter struct {
	// Configuration
	config *DHCPFilterConfig

	// eBPF components
	program    *ebpf.Program
	attachLink link.Link

	// Maps for tracking DHCP transactions
	discoverMap *ebpf.Map
	offerMap    *ebpf.Map
	requestMap  *ebpf.Map
	ackMap      *ebpf.Map
	statsMap    *ebpf.Map
	eventsMap   *ebpf.Map

	// Event handlers
	discoverHandler DHCPDiscoverHandler
	offerHandler    DHCPOfferHandler
	requestHandler  DHCPRequestHandler
	ackHandler      DHCPAckHandler

	// State
	mutex   sync.RWMutex
	enabled bool

	// Statistics
	stats *DHCPFilterStats

	// Logger
	logger *logging.Logger

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// DHCPFilterConfig holds configuration for DHCP socket filter
type DHCPFilterConfig struct {
	// Filter settings
	Enabled               bool          `hcl:"enabled,optional"`
	Interface             string        `hcl:"interface,optional"`
	TransactionBufferSize int           `hcl:"transaction_buffer_size,optional"`
	TransactionTimeout    time.Duration `hcl:"transaction_timeout,optional"`
	MaxTransactions       int           `hcl:"max_transactions,optional"`

	// Monitoring settings
	LogDiscovers      bool `hcl:"log_discovers,optional"`
	LogOffers         bool `hcl:"log_offers,optional"`
	LogRequests       bool `hcl:"log_requests,optional"`
	LogAcks           bool `hcl:"log_acks,optional"`
	TrackTransactions bool `hcl:"track_transactions,optional"`

	// Device discovery settings
	DiscoverDevices bool `hcl:"discover_devices,optional"`
	TrackVendorInfo bool `hcl:"track_vendor_info,optional"`
	TrackHostname   bool `hcl:"track_hostname,optional"`
	TrackDomain     bool `hcl:"track_domain,optional"`

	// Security settings
	ValidateDHCP     bool `hcl:"validate_dhcp,optional"`
	BlockInvalidDHCP bool `hcl:"block_invalid_dhcp,optional"`
	DetectRogueDHCP  bool `hcl:"detect_rogue_dhcp,optional"`
	AlertOnRogueDHCP bool `hcl:"alert_on_rogue_dhcp,optional"`

	// Server filtering
	AllowedServers []string `hcl:"allowed_servers,optional"`
	BlockedServers []string `hcl:"blocked_servers,optional"`
	TrustedServers []string `hcl:"trusted_servers,optional"`
}

// DHCPFilterStats holds statistics for the DHCP filter
type DHCPFilterStats struct {
	DiscoversSeen         uint64    `json:"discovers_seen"`
	OffersSeen            uint64    `json:"offers_seen"`
	RequestsSeen          uint64    `json:"requests_seen"`
	AcksSeen              uint64    `json:"acks_seen"`
	TransactionsTracked   uint64    `json:"transactions_tracked"`
	DevicesDiscovered     uint64    `json:"devices_discovered"`
	RogueServersDetected  uint64    `json:"rogue_servers_detected"`
	InvalidPacketsBlocked uint64    `json:"invalid_packets_blocked"`
	Errors                uint64    `json:"errors"`
	LastUpdate            time.Time `json:"last_update"`
}

// DHCPDiscoverHandler handles DHCP discover events
type DHCPDiscoverHandler func(event *types.DHCPDiscoverEvent) error

// DHCPOfferHandler handles DHCP offer events
type DHCPOfferHandler func(event *types.DHCPOfferEvent) error

// DHCPRequestHandler handles DHCP request events
type DHCPRequestHandler func(event *types.DHCPRequestEvent) error

// DHCPAckHandler handles DHCP acknowledge events
type DHCPAckHandler func(event *types.DHCPAckEvent) error

// DefaultDHCPFilterConfig returns default configuration
func DefaultDHCPFilterConfig() *DHCPFilterConfig {
	return &DHCPFilterConfig{
		Enabled:               false,
		TransactionBufferSize: 65536,
		TransactionTimeout:    60 * time.Second,
		MaxTransactions:       10000,
		LogDiscovers:          true,
		LogOffers:             true,
		LogRequests:           true,
		LogAcks:               true,
		TrackTransactions:     true,
		DiscoverDevices:       true,
		TrackVendorInfo:       true,
		TrackHostname:         true,
		TrackDomain:           true,
		ValidateDHCP:          true,
		BlockInvalidDHCP:      false,
		DetectRogueDHCP:       true,
		AlertOnRogueDHCP:      false,
	}
}

// NewDHCPFilter creates a new DHCP socket filter
func NewDHCPFilter(logger *logging.Logger, config *DHCPFilterConfig) *DHCPFilter {
	if config == nil {
		config = DefaultDHCPFilterConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	dhcp := &DHCPFilter{
		config: config,
		stats: &DHCPFilterStats{
			LastUpdate: time.Now(),
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	return dhcp
}

// Start starts the DHCP socket filter
func (dhcp *DHCPFilter) Start() error {
	dhcp.mutex.Lock()
	defer dhcp.mutex.Unlock()

	if !dhcp.config.Enabled {
		dhcp.logger.Info("DHCP socket filter disabled")
		return nil
	}

	dhcp.logger.Info("Starting DHCP socket filter")

	// Load eBPF program
	if err := dhcp.loadProgram(); err != nil {
		return err
	}

	// Start event processing
	go dhcp.processEvents()
	go dhcp.cleanupExpiredTransactions()

	dhcp.enabled = true
	dhcp.logger.Info("DHCP socket filter started")

	return nil
}

// processEvents reads and dispatches events from the ring buffer
func (dhcp *DHCPFilter) processEvents() {
	reader, err := ringbuf.NewReader(dhcp.eventsMap)
	if err != nil {
		dhcp.logger.Error("Failed to create ring buffer reader", "error", err)
		return
	}
	defer reader.Close()

	dhcp.logger.Info("Listening for DHCP events from eBPF")

	for {
		select {
		case <-dhcp.ctx.Done():
			return
		default:
			record, err := reader.Read()
			if err != nil {
				if err == ringbuf.ErrClosed {
					return
				}
				dhcp.logger.Warn("Error reading from DHCP ring buffer", "error", err)
				continue
			}

			var event types.DHCPEvent
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
				dhcp.logger.Warn("Failed to parse DHCP event", "error", err)
				continue
			}

			dhcp.handleEvent(&event)
		}
	}
}

func (dhcp *DHCPFilter) handleEvent(raw *types.DHCPEvent) {
	// timestamp := time.Unix(0, int64(raw.Timestamp)) // ktime is not wall clock
	timestamp := time.Now()

	mac := net.HardwareAddr(raw.MACAddress[:]).String()
	// common.h uses __be32, binary.Read(LittleEndian) reads it as LE derived from BE memory.
	// We extract bytes in LE order to reconstruct the IP order (1, 2, 3, 4).
	uint32ToIP := func(n uint32) net.IP {
		return net.IPv4(byte(n), byte(n>>8), byte(n>>16), byte(n>>24))
	}

	srcIP := uint32ToIP(raw.SrcIP)
	dstIP := uint32ToIP(raw.DstIP)

	// Basic packet validation
	if dhcp.config.ValidateDHCP {
		if !dhcp.validatePacket(raw) {
			dhcp.mutex.Lock()
			dhcp.stats.InvalidPacketsBlocked++
			dhcp.mutex.Unlock()
			return
		}
	}

	switch raw.EventType {
	case 1: // Discover
		ev := &types.DHCPDiscoverEvent{
			Timestamp:   timestamp,
			PID:         raw.PID,
			TID:         raw.TID,
			SourceIP:    srcIP,
			SourcePort:  raw.SrcPort,
			DestIP:      dstIP,
			DestPort:    raw.DstPort,
			MACAddress:  mac,
			HostName:    string(bytes.TrimRight(raw.HostName[:raw.HostNameLen], "\x00")),
			VendorClass: string(bytes.TrimRight(raw.VendorClass[:raw.VendorClassLen], "\x00")),
			PacketSize:  raw.PacketSize,
		}
		if dhcp.config.LogDiscovers {
			dhcp.logger.Debug("DHCP Discover", "mac", mac, "hostname", ev.HostName)
		}

		dhcp.mutex.Lock()
		dhcp.stats.DiscoversSeen++
		dhcp.stats.LastUpdate = time.Now()
		dhcp.mutex.Unlock()

		if dhcp.discoverHandler != nil {
			dhcp.discoverHandler(ev)
		}

	case 2: // Offer
		serverIP := uint32ToIP(raw.ServerIP)

		// Rogue DHCP detection
		if dhcp.config.DetectRogueDHCP {
			if !dhcp.isServerAllowed(serverIP) {
				dhcp.logger.Warn("Rogue DHCP Offer detected", "server_ip", serverIP, "mac", mac)
				dhcp.mutex.Lock()
				dhcp.stats.RogueServersDetected++
				dhcp.mutex.Unlock()
				if dhcp.config.BlockInvalidDHCP {
					return
				}
			}
		}

		ev := &types.DHCPOfferEvent{
			Timestamp:  timestamp,
			PID:        raw.PID,
			TID:        raw.TID,
			SourceIP:   srcIP,
			SourcePort: raw.SrcPort,
			DestIP:     dstIP,
			DestPort:   raw.DstPort,
			YourIP:     uint32ToIP(raw.YourIP),
			ServerIP:   serverIP,
			SubnetMask: uint32ToIP(raw.SubnetMask),
			Router:     uint32ToIP(raw.Router),
			LeaseTime:  raw.LeaseTime,
			PacketSize: raw.PacketSize,
		}
		for i := 0; i < 4; i++ {
			if raw.DNSServers[i] != 0 {
				ev.DNSServers = append(ev.DNSServers, uint32ToIP(raw.DNSServers[i]))
			}
		}
		if dhcp.config.LogOffers {
			dhcp.logger.Debug("DHCP Offer", "your_ip", ev.YourIP, "server_ip", ev.ServerIP)
		}

		dhcp.mutex.Lock()
		dhcp.stats.OffersSeen++
		dhcp.stats.LastUpdate = time.Now()
		dhcp.mutex.Unlock()

		if dhcp.offerHandler != nil {
			dhcp.offerHandler(ev)
		}

	case 3: // Request
		ev := &types.DHCPRequestEvent{
			Timestamp:   timestamp,
			PID:         raw.PID,
			TID:         raw.TID,
			SourceIP:    srcIP,
			SourcePort:  raw.SrcPort,
			DestIP:      dstIP,
			DestPort:    raw.DstPort,
			MACAddress:  mac,
			RequestedIP: uint32ToIP(raw.RequestedIP),
			ServerIP:    uint32ToIP(raw.ServerIP),
			HostName:    string(bytes.TrimRight(raw.HostName[:raw.HostNameLen], "\x00")),
			PacketSize:  raw.PacketSize,
		}
		if dhcp.config.LogRequests {
			dhcp.logger.Debug("DHCP Request", "mac", mac, "requested_ip", ev.RequestedIP)
		}

		dhcp.mutex.Lock()
		dhcp.stats.RequestsSeen++
		dhcp.stats.LastUpdate = time.Now()
		dhcp.mutex.Unlock()

		if dhcp.requestHandler != nil {
			dhcp.requestHandler(ev)
		}

	case 4: // ACK
		serverIP := uint32ToIP(raw.ServerIP)

		// Rogue DHCP detection
		if dhcp.config.DetectRogueDHCP {
			if !dhcp.isServerAllowed(serverIP) {
				dhcp.logger.Warn("Rogue DHCP ACK detected", "server_ip", serverIP, "mac", mac)
				dhcp.mutex.Lock()
				dhcp.stats.RogueServersDetected++
				dhcp.mutex.Unlock()
				if dhcp.config.BlockInvalidDHCP {
					return
				}
			}
		}

		ev := &types.DHCPAckEvent{
			Timestamp:     timestamp,
			PID:           raw.PID,
			TID:           raw.TID,
			SourceIP:      srcIP,
			SourcePort:    raw.SrcPort,
			DestIP:        dstIP,
			DestPort:      raw.DstPort,
			MACAddress:    mac,
			YourIP:        uint32ToIP(raw.YourIP),
			ServerIP:      serverIP,
			SubnetMask:    uint32ToIP(raw.SubnetMask),
			Router:        uint32ToIP(raw.Router),
			LeaseTime:     raw.LeaseTime,
			RenewalTime:   raw.RenewalTime,
			RebindingTime: raw.RebindingTime,
			PacketSize:    raw.PacketSize,
		}
		for i := 0; i < 4; i++ {
			if raw.DNSServers[i] != 0 {
				ev.DNSServers = append(ev.DNSServers, uint32ToIP(raw.DNSServers[i]))
			}
		}
		if dhcp.config.LogAcks {
			dhcp.logger.Debug("DHCP Ack", "your_ip", ev.YourIP, "mac", mac)
		}

		dhcp.mutex.Lock()
		dhcp.stats.AcksSeen++
		dhcp.stats.LastUpdate = time.Now()
		dhcp.mutex.Unlock()

		if dhcp.ackHandler != nil {
			dhcp.ackHandler(ev)
		}
	}
}

func (dhcp *DHCPFilter) isServerAllowed(ip net.IP) bool {
	ipStr := ip.String()

	// If allowed servers list is empty, any server is allowed unless explicitly blocked
	if len(dhcp.config.AllowedServers) > 0 {
		allowed := false
		for _, s := range dhcp.config.AllowedServers {
			if s == ipStr {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// Check blocked servers
	for _, s := range dhcp.config.BlockedServers {
		if s == ipStr {
			return false
		}
	}

	return true
}

func (dhcp *DHCPFilter) validatePacket(raw *types.DHCPEvent) bool {
	// Basic validation: packet size should be reasonable
	if raw.PacketSize < 240 {
		return false
	}

	// MAC address should not be all zeros if it's a client message
	if raw.EventType == 1 || raw.EventType == 3 { // Discover or Request
		allZeros := true
		for _, b := range raw.MACAddress {
			if b != 0 {
				allZeros = false
				break
			}
		}
		if allZeros {
			return false
		}
	}

	return true
}

// Stop stops the DHCP socket filter
func (dhcp *DHCPFilter) Stop() {
	dhcp.mutex.Lock()
	defer dhcp.mutex.Unlock()

	if !dhcp.enabled {
		return
	}

	dhcp.logger.Info("Stopping DHCP socket filter")

	// Cancel context
	dhcp.cancel()

	// Detach socket filter
	if dhcp.attachLink != nil {
		dhcp.attachLink.Close()
		dhcp.attachLink = nil
	}

	// Close eBPF maps
	if dhcp.discoverMap != nil {
		dhcp.discoverMap.Close()
		dhcp.discoverMap = nil
	}
	if dhcp.offerMap != nil {
		dhcp.offerMap.Close()
		dhcp.offerMap = nil
	}
	if dhcp.requestMap != nil {
		dhcp.requestMap.Close()
		dhcp.requestMap = nil
	}
	if dhcp.ackMap != nil {
		dhcp.ackMap.Close()
		dhcp.ackMap = nil
	}
	if dhcp.statsMap != nil {
		dhcp.statsMap.Close()
		dhcp.statsMap = nil
	}
	if dhcp.eventsMap != nil {
		dhcp.eventsMap.Close()
		dhcp.eventsMap = nil
	}

	// Close eBPF program
	if dhcp.program != nil {
		dhcp.program.Close()
		dhcp.program = nil
	}

	dhcp.enabled = false
	dhcp.logger.Info("DHCP socket filter stopped")
}

// loadProgram loads the eBPF program
func (dhcp *DHCPFilter) loadProgram() error {
	dhcp.logger.Info("Loading DHCP socket filter eBPF program")

	// Load compiled eBPF program from file
	collection, err := ebpf.LoadCollectionSpec("dhcp_socket.o")
	if err != nil {
		return fmt.Errorf("failed to load eBPF collection: %w", err)
	}

	// Load the program
	program, err := ebpf.NewProgram(collection.Programs["dhcp_socket_filter"])
	if err != nil {
		return fmt.Errorf("failed to create eBPF program: %w", err)
	}
	dhcp.program = program

	// Load maps from collection
	if discoverMap, ok := collection.Maps["dhcp_discovers"]; ok {
		dhcp.discoverMap, err = ebpf.NewMap(discoverMap)
		if err != nil {
			return fmt.Errorf("failed to create dhcp_discovers map: %w", err)
		}
	} else {
		return fmt.Errorf("dhcp_discovers map not found in collection")
	}

	if offerMap, ok := collection.Maps["dhcp_offers"]; ok {
		dhcp.offerMap, err = ebpf.NewMap(offerMap)
		if err != nil {
			return fmt.Errorf("failed to create dhcp_offers map: %w", err)
		}
	} else {
		return fmt.Errorf("dhcp_offers map not found in collection")
	}

	if requestMap, ok := collection.Maps["dhcp_requests"]; ok {
		dhcp.requestMap, err = ebpf.NewMap(requestMap)
		if err != nil {
			return fmt.Errorf("failed to create dhcp_requests map: %w", err)
		}
	} else {
		return fmt.Errorf("dhcp_requests map not found in collection")
	}

	if ackMap, ok := collection.Maps["dhcp_acks"]; ok {
		dhcp.ackMap, err = ebpf.NewMap(ackMap)
		if err != nil {
			return fmt.Errorf("failed to create dhcp_acks map: %w", err)
		}
	} else {
		return fmt.Errorf("dhcp_acks map not found in collection")
	}

	if statsMap, ok := collection.Maps["dhcp_stats"]; ok {
		dhcp.statsMap, err = ebpf.NewMap(statsMap)
		if err != nil {
			return fmt.Errorf("failed to create dhcp_stats map: %w", err)
		}
	} else {
		return fmt.Errorf("dhcp_stats map not found in collection")
	}

	if eventsMap, ok := collection.Maps["dhcp_events"]; ok {
		dhcp.eventsMap, err = ebpf.NewMap(eventsMap)
		if err != nil {
			return fmt.Errorf("failed to create dhcp_events map: %w", err)
		}
	} else {
		return fmt.Errorf("dhcp_events map not found in collection")
	}

	return nil
}

// SetDiscoverHandler sets the discover event handler
func (dhcp *DHCPFilter) SetDiscoverHandler(handler func(event *types.DHCPDiscoverEvent) error) {
	dhcp.discoverHandler = handler
}

// SetOfferHandler sets the offer event handler
func (dhcp *DHCPFilter) SetOfferHandler(handler func(event *types.DHCPOfferEvent) error) {
	dhcp.offerHandler = handler
}

// SetRequestHandler sets the request event handler
func (dhcp *DHCPFilter) SetRequestHandler(handler func(event *types.DHCPRequestEvent) error) {
	dhcp.requestHandler = handler
}

// SetAckHandler sets the acknowledge event handler
func (dhcp *DHCPFilter) SetAckHandler(handler func(event *types.DHCPAckEvent) error) {
	dhcp.ackHandler = handler
}

// GetStatistics returns DHCP filter statistics
func (dhcp *DHCPFilter) GetStatistics() interface{} {
	dhcp.mutex.RLock()
	defer dhcp.mutex.RUnlock()

	stats := *dhcp.stats
	return &stats
}

// IsEnabled returns whether the DHCP filter is enabled
func (dhcp *DHCPFilter) IsEnabled() bool {
	dhcp.mutex.RLock()
	defer dhcp.mutex.RUnlock()
	return dhcp.enabled
}

// cleanupExpiredTransactions cleans up expired DHCP transactions
func (dhcp *DHCPFilter) cleanupExpiredTransactions() {
	ticker := time.NewTicker(dhcp.config.TransactionTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-dhcp.ctx.Done():
			return
		case <-ticker.C:
			dhcp.cleanupMaps()
		}
	}
}

func (dhcp *DHCPFilter) cleanupMaps() {
	// We use a relative check. Since we don't easily have the kernel's ktime_get_ns
	dhcp.cleanupDiscoverMap()
	dhcp.cleanupOfferMap()
	dhcp.cleanupRequestMap()
	dhcp.cleanupAckMap()
}

func (dhcp *DHCPFilter) cleanupDiscoverMap() {
	if dhcp.discoverMap == nil {
		return
	}

	var key types.DHCPKey
	var info types.DHCPDiscoverInfo
	var expiredKeys []types.DHCPKey

	now := dhcp.getKtime()
	timeout := uint64(dhcp.config.TransactionTimeout.Nanoseconds())

	iter := dhcp.discoverMap.Iterate()
	for iter.Next(&key, &info) {
		if now > info.Timestamp && now-info.Timestamp > timeout {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, k := range expiredKeys {
		dhcp.discoverMap.Delete(k)
	}
}

func (dhcp *DHCPFilter) cleanupOfferMap() {
	if dhcp.offerMap == nil {
		return
	}

	var key types.DHCPKey
	var info types.DHCPOfferInfo
	var expiredKeys []types.DHCPKey

	now := dhcp.getKtime()
	timeout := uint64(dhcp.config.TransactionTimeout.Nanoseconds())

	iter := dhcp.offerMap.Iterate()
	for iter.Next(&key, &info) {
		if now > info.Timestamp && now-info.Timestamp > timeout {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, k := range expiredKeys {
		dhcp.offerMap.Delete(k)
	}
}

func (dhcp *DHCPFilter) cleanupRequestMap() {
	if dhcp.requestMap == nil {
		return
	}

	var key types.DHCPKey
	var info types.DHCPRequestInfo
	var expiredKeys []types.DHCPKey

	now := dhcp.getKtime()
	timeout := uint64(dhcp.config.TransactionTimeout.Nanoseconds())

	iter := dhcp.requestMap.Iterate()
	for iter.Next(&key, &info) {
		if now > info.Timestamp && now-info.Timestamp > timeout {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, k := range expiredKeys {
		dhcp.requestMap.Delete(k)
	}
}

func (dhcp *DHCPFilter) cleanupAckMap() {
	if dhcp.ackMap == nil {
		return
	}

	var key types.DHCPKey
	var info types.DHCPAckInfo
	var expiredKeys []types.DHCPKey

	now := dhcp.getKtime()
	timeout := uint64(dhcp.config.TransactionTimeout.Nanoseconds())

	iter := dhcp.ackMap.Iterate()
	for iter.Next(&key, &info) {
		if now > info.Timestamp && now-info.Timestamp > timeout {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, k := range expiredKeys {
		dhcp.ackMap.Delete(k)
	}
}
