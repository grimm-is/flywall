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

// tlsEvent matches the C struct in socket_tls.c
type tlsEvent struct {
	Timestamp    uint64
	PID          uint32
	TID          uint32
	SrcIP        [4]byte
	DstIP        [4]byte
	SrcPort      uint16
	DstPort      uint16
	Version      uint16
	CipherSuite  uint16
	SNI          [64]byte
	JA3Hash      [4]uint32
	PacketSize   uint16
	_            [6]byte // Padding for 8-byte alignment
}

// TLSFilter implements a socket filter for TLS monitoring
type TLSFilter struct {
	// Configuration
	config *TLSFilterConfig

	// eBPF components
	program    *ebpf.Program
	attachLink link.Link

	// Maps
	handshakeMap *ebpf.Map
	statsMap     *ebpf.Map
	eventsMap    *ebpf.Map

	// Event handlers
	handshakeHandler   func(event *types.TLSHandshakeEvent) error
	certificateHandler func(event *types.TLSCertificateEvent) error

	// State
	mutex   sync.RWMutex
	enabled bool

	// Statistics
	stats *TLSFilterStats

	// Logger
	logger *logging.Logger

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// TLSFilterConfig holds configuration for TLS socket filter
type TLSFilterConfig struct {
	Enabled            bool `hcl:"enabled,optional"`
	Interface          string `hcl:"interface,optional"`
	LogHandshakes      bool `hcl:"log_handshakes,optional"`
	ExtractSNI         bool `hcl:"extract_sni,optional"`
	JA3Fingerprinting  bool `hcl:"ja3_fingerprinting,optional"`
	InspectCertificates bool `hcl:"inspect_certificates,optional"`
}

// TLSFilterStats holds statistics for the TLS filter
type TLSFilterStats struct {
	HandshakesObserved uint64    `json:"handshakes_observed"`
	CertificatesValid  uint64    `json:"certificates_valid"`
	CertificatesInvalid uint64   `json:"certificates_invalid"`
	Errors             uint64    `json:"errors"`
	LastUpdate         time.Time `json:"last_update"`
}

// DefaultTLSFilterConfig returns default configuration
func DefaultTLSFilterConfig() *TLSFilterConfig {
	return &TLSFilterConfig{
		Enabled:            false,
		LogHandshakes:      true,
		ExtractSNI:         true,
		JA3Fingerprinting:  true,
		InspectCertificates: true,
	}
}

// NewTLSFilter creates a new TLS socket filter
func NewTLSFilter(logger *logging.Logger, config *TLSFilterConfig) *TLSFilter {
	if config == nil {
		config = DefaultTLSFilterConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TLSFilter{
		config: config,
		stats:  &TLSFilterStats{LastUpdate: time.Now()},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the TLS socket filter
func (tls *TLSFilter) Start() error {
	tls.mutex.Lock()
	defer tls.mutex.Unlock()

	if !tls.config.Enabled {
		return nil
	}

	tls.logger.Info("Starting TLS socket filter")

	if err := tls.loadProgram(); err != nil {
		return err
	}

	go tls.processEvents()

	tls.enabled = true
	return nil
}

// Stop stops the TLS socket filter
func (tls *TLSFilter) Stop() {
	tls.mutex.Lock()
	defer tls.mutex.Unlock()

	if !tls.enabled {
		return
	}

	tls.cancel()

	if tls.attachLink != nil {
		tls.attachLink.Close()
		tls.attachLink = nil
	}

	if tls.handshakeMap != nil {
		tls.handshakeMap.Close()
		tls.handshakeMap = nil
	}
	if tls.statsMap != nil {
		tls.statsMap.Close()
		tls.statsMap = nil
	}
	if tls.eventsMap != nil {
		tls.eventsMap.Close()
		tls.eventsMap = nil
	}

	if tls.program != nil {
		tls.program.Close()
		tls.program = nil
	}

	tls.enabled = false
	tls.logger.Info("TLS socket filter stopped")
}

// loadProgram loads the eBPF program
func (tls *TLSFilter) loadProgram() error {
	spec, err := ebpf.LoadCollectionSpec("socket_tls.o")
	if err != nil {
		return fmt.Errorf("failed to load eBPF spec: %w", err)
	}

	program, err := ebpf.NewProgram(spec.Programs["tls_socket_filter"])
	if err != nil {
		return fmt.Errorf("failed to create eBPF program: %w", err)
	}
	tls.program = program

	tls.handshakeMap, err = ebpf.NewMap(spec.Maps["tls_handshakes"])
	if err != nil {
		return fmt.Errorf("failed to create handshakes map: %w", err)
	}

	tls.statsMap, err = ebpf.NewMap(spec.Maps["tls_stats"])
	if err != nil {
		return fmt.Errorf("failed to create stats map: %w", err)
	}

	tls.eventsMap, err = ebpf.NewMap(spec.Maps["tls_events"])
	if err != nil {
		return fmt.Errorf("failed to create events map: %w", err)
	}

	return nil
}

// processEvents reads and dispatches events from the ring buffer
func (tls *TLSFilter) processEvents() {
	reader, err := ringbuf.NewReader(tls.eventsMap)
	if err != nil {
		tls.logger.Error("Failed to create ring buffer reader", "error", err)
		return
	}
	defer reader.Close()

	for {
		select {
		case <-tls.ctx.Done():
			return
		default:
			record, err := reader.Read()
			if err != nil {
				if err == ringbuf.ErrClosed {
					return
				}
				continue
			}

			var event tlsEvent
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
				continue
			}

			tls.handleEvent(&event)
		}
	}
}

func (tls *TLSFilter) handleEvent(raw *tlsEvent) {
	ev := &types.TLSHandshakeEvent{
		Timestamp:   time.Now(),
		PID:         raw.PID,
		TID:         raw.TID,
		SourceIP:    net.IP(raw.SrcIP[:]),
		DestIP:      net.IP(raw.DstIP[:]),
		SourcePort:  raw.SrcPort,
		DestPort:    raw.DstPort,
		Version:     raw.Version,
		CipherSuite: raw.CipherSuite,
		SNI:         string(bytes.TrimRight(raw.SNI[:], "\x00")),
		JA3Hash:     fmt.Sprintf("%x%x%x%x", raw.JA3Hash[0], raw.JA3Hash[1], raw.JA3Hash[2], raw.JA3Hash[3]),
		PacketSize:  raw.PacketSize,
	}

	tls.mutex.Lock()
	tls.stats.HandshakesObserved++
	tls.stats.LastUpdate = time.Now()
	tls.mutex.Unlock()

	if tls.handshakeHandler != nil {
		tls.handshakeHandler(ev)
	}
}

func (tls *TLSFilter) SetHandshakeHandler(handler func(event *types.TLSHandshakeEvent) error) {
	tls.handshakeHandler = handler
}

func (tls *TLSFilter) SetCertificateHandler(handler func(event *types.TLSCertificateEvent) error) {
	tls.certificateHandler = handler
}

func (tls *TLSFilter) GetStatistics() interface{} {
	tls.mutex.RLock()
	defer tls.mutex.RUnlock()
	stats := *tls.stats
	return &stats
}

func (tls *TLSFilter) IsEnabled() bool {
	tls.mutex.RLock()
	defer tls.mutex.RUnlock()
	return tls.enabled
}
