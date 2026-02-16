// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package programs

import (
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// TCOffloadProgram manages the TC fast-path program
type TCOffloadProgram struct {
	collection *ebpf.Collection
	links      []link.Link
	logger     *logging.Logger
	stats      *TCStats
}

// TCStats holds TC program statistics
type TCStats struct {
	PacketsProcessed uint64 `json:"packets_processed"`
	PacketsFastPath  uint64 `json:"packets_fast_path"`
	PacketsSlowPath  uint64 `json:"packets_slow_path"`
	PacketsDropped   uint64 `json:"packets_dropped"`
	BytesProcessed   uint64 `json:"bytes_processed"`
}

// NewTCOffloadProgram loads and attaches the TC offload program
func NewTCOffloadProgram(logger *logging.Logger) (*TCOffloadProgram, error) {
	// Load pre-compiled program from embedded bytecode
	spec, err := LoadTcOffload()
	if err != nil {
		return nil, fmt.Errorf("failed to load TC offload spec: %w", err)
	}

	// Disable pinning for all maps (not needed in most environments/tests)
	for _, m := range spec.Maps {
		m.Pinning = ebpf.PinNone
	}

	// Workaround for tc_stats_map issues on some kernels/environments
	if m, ok := spec.Maps["tc_stats_map"]; ok {
		m.Type = ebpf.Array
	}

	// Increase map sizes for production
	if spec.Maps["flow_map"] != nil {
		spec.Maps["flow_map"].MaxEntries = 100000
	}

	// Load the collection
	collection, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to load TC offload collection: %w", err)
	}

	program := &TCOffloadProgram{
		collection: collection,
		links:      make([]link.Link, 0),
		logger:     logger,
		stats:      &TCStats{},
	}

	// Initialize statistics map
	if err := program.initStats(); err != nil {
		program.Close()
		return nil, fmt.Errorf("failed to initialize stats: %w", err)
	}

	return program, nil
}

// Attach attaches the TC program to the specified interface
func (p *TCOffloadProgram) Attach(ifaceName string) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("failed to find interface %s: %w", ifaceName, err)
	}

	// Attach ingress TC program
	ingressProg := p.collection.Programs["tc_fast_path"]
	if ingressProg == nil {
		return fmt.Errorf("tc_fast_path program not found")
	}

	// For now, skip actual attachment on macOS
	if runtime.GOOS != "linux" {
		p.logger.Info("Skipping TC attachment on non-Linux platform", "interface", ifaceName)
		return nil
	}

	ingressLink, err := link.AttachTCX(link.TCXOptions{
		Program:   ingressProg,
		Interface: iface.Index,
		Attach:    ebpf.AttachTCXIngress,
	})
	if err != nil {
		return fmt.Errorf("failed to attach ingress TC program: %w", err)
	}

	p.links = append(p.links, ingressLink)
	p.logger.Info("Attached TC ingress program", "interface", ifaceName)

	// Attach egress TC program
	egressProg := p.collection.Programs["tc_egress_fast_path"]
	if egressProg == nil {
		return fmt.Errorf("tc_egress_fast_path program not found")
	}

	if runtime.GOOS != "linux" {
		p.logger.Info("Skipping TC egress attachment on non-Linux platform", "interface", ifaceName)
		return nil
	}

	egressLink, err := link.AttachTCX(link.TCXOptions{
		Program:   egressProg,
		Interface: iface.Index,
		Attach:    ebpf.AttachTCXEgress,
	})
	if err != nil {
		// Cleanup ingress link on failure
		ingressLink.Close()
		p.links = p.links[:len(p.links)-1]
		return fmt.Errorf("failed to attach egress TC program: %w", err)
	}

	p.links = append(p.links, egressLink)
	p.logger.Info("Attached TC egress program", "interface", ifaceName)

	return nil
}

// Detach removes the TC program from all interfaces
func (p *TCOffloadProgram) Detach() error {
	var lastErr error

	for _, link := range p.links {
		if err := link.Close(); err != nil {
			lastErr = err
		}
	}

	p.links = p.links[:0]

	return lastErr
}

// FlowMap returns the flow state map
func (p *TCOffloadProgram) FlowMap() *ebpf.Map {
	return p.collection.Maps["flow_map"]
}

// StatsMap returns the statistics map
func (p *TCOffloadProgram) StatsMap() *ebpf.Map {
	return p.collection.Maps["tc_stats_map"]
}

// UpdateFlow updates or creates a flow state
func (p *TCOffloadProgram) UpdateFlow(key types.FlowKey, state types.FlowState) error {
	flowMap := p.FlowMap()
	if flowMap == nil {
		return fmt.Errorf("flow map not available")
	}

	// Convert to C structures
	cKey := keyToC(key)
	cState := stateToC(state)

	return flowMap.Update(&cKey, &cState, ebpf.UpdateAny)
}

// DeleteFlow removes a flow state
func (p *TCOffloadProgram) DeleteFlow(key types.FlowKey) error {
	flowMap := p.FlowMap()
	if flowMap == nil {
		return fmt.Errorf("flow map not available")
	}

	cKey := keyToC(key)
	return flowMap.Delete(&cKey)
}

// GetFlow retrieves a flow state
func (p *TCOffloadProgram) GetFlow(key types.FlowKey) (*types.FlowState, error) {
	flowMap := p.FlowMap()
	if flowMap == nil {
		return nil, fmt.Errorf("flow map not available")
	}

	cKey := keyToC(key)
	var cState CFlowState

	if err := flowMap.Lookup(&cKey, &cState); err != nil {
		return nil, err
	}

	state := cStateToState(cState)
	return &state, nil
}

// GetStats returns current TC statistics
func (p *TCOffloadProgram) GetStats() *TCStats {
	statsMap := p.StatsMap()
	if statsMap == nil {
		return p.stats
	}

	var key uint32 = 0
	var cStats CTCStats

	if err := statsMap.Lookup(&key, &cStats); err == nil {
		p.stats.PacketsProcessed = cStats.PacketsProcessed
		p.stats.PacketsFastPath = cStats.PacketsFastPath
		p.stats.PacketsSlowPath = cStats.PacketsSlowPath
		p.stats.PacketsDropped = cStats.PacketsDropped
		p.stats.BytesProcessed = cStats.BytesProcessed
	}

	return p.stats
}

// initStats initializes the statistics map
func (p *TCOffloadProgram) initStats() error {
	statsMap := p.StatsMap()
	if statsMap == nil {
		return fmt.Errorf("stats map not found")
	}

	var key uint32 = 0
	initialStats := CTCStats{}

	return statsMap.Update(&key, &initialStats, ebpf.UpdateAny)
}

// Cleanup removes expired flows
func (p *TCOffloadProgram) Cleanup(timeout time.Duration) error {
	flowMap := p.FlowMap()
	if flowMap == nil {
		return fmt.Errorf("flow map not available")
	}

	now := time.Now().UnixNano()
	timeoutNs := uint64(timeout.Nanoseconds())

	var keys []CFlowKey
	var values []CFlowState

	// Collect all entries
	iterator := flowMap.Iterate()
	for iterator.Next(&keys, &values) {
		if uint64(now)-values[0].LastSeen < timeoutNs {
			// Flow has expired, mark for deletion
			if err := flowMap.Delete(&keys[0]); err != nil {
				p.logger.Error("Failed to delete expired flow", "error", err)
			}
		}
	}

	if err := iterator.Err(); err != nil {
		return fmt.Errorf("failed to iterate flow map: %w", err)
	}

	return nil
}

// Close detaches the program and cleans up resources
func (p *TCOffloadProgram) Close() error {
	if err := p.Detach(); err != nil {
		p.logger.Error("Error detaching TC program", "error", err)
	}

	if p.collection != nil {
		p.collection.Close()
	}

	return nil
}

// GetCollection returns the eBPF collection for statistics
func (p *TCOffloadProgram) GetCollection() *ebpf.Collection {
	return p.collection
}

// C structures for eBPF map compatibility
type CFlowKey struct {
	SrcIP   uint32
	DstIP   uint32
	SrcPort uint16
	DstPort uint16
	IPProto uint8
	Padding [3]uint8
	Ifindex uint32
}

type CFlowState struct {
	Verdict     uint8
	QoSProfile  uint8
	Flags       uint16
	PacketCount uint32
	ByteCount   uint32
	LastSeen    uint64
	CreatedAt   uint64
	ExpiresAt   uint64
	JA3Hash     [4]uint32
	SNI         [64]byte
}

type CTCStats struct {
	PacketsProcessed uint64
	PacketsFastPath  uint64
	PacketsSlowPath  uint64
	PacketsDropped   uint64
	BytesProcessed   uint64
}

type CQoSProfile struct {
	RateLimit  uint32
	BurstLimit uint32
	Priority   uint8
	AppClass   uint8
	Padding    [2]uint8
}

// SetQoSProfile sets the QoS profile for a flow
func (p *TCOffloadProgram) SetQoSProfile(key types.FlowKey, profileID uint8) error {
	flowMap := p.FlowMap()
	if flowMap == nil {
		return fmt.Errorf("flow map not available")
	}

	// Get existing flow state
	state, err := p.GetFlow(key)
	if err != nil {
		return fmt.Errorf("flow not found: %w", err)
	}

	// Update QoS profile
	state.QoSProfile = profileID

	// Update in map
	return p.UpdateFlow(key, *state)
}

// UpdateQoSProfile updates a QoS profile configuration
func (p *TCOffloadProgram) UpdateQoSProfile(profileID uint32, profile types.QoSProfile) error {
	qosMap := p.collection.Maps["qos_profiles"]
	if qosMap == nil {
		return fmt.Errorf("QoS profiles map not available")
	}

	cProfile := CQoSProfile{
		RateLimit:  profile.RateLimit,
		BurstLimit: profile.BurstLimit,
		Priority:   profile.Priority,
		AppClass:   profile.AppClass,
	}

	return qosMap.Update(&profileID, &cProfile, ebpf.UpdateAny)
}

// GetQoSProfile retrieves a QoS profile configuration
func (p *TCOffloadProgram) GetQoSProfile(profileID uint32) (*types.QoSProfile, error) {
	qosMap := p.collection.Maps["qos_profiles"]
	if qosMap == nil {
		return nil, fmt.Errorf("QoS profiles map not available")
	}

	var cProfile CQoSProfile
	if err := qosMap.Lookup(&profileID, &cProfile); err != nil {
		return nil, err
	}

	return &types.QoSProfile{
		RateLimit:  cProfile.RateLimit,
		BurstLimit: cProfile.BurstLimit,
		Priority:   cProfile.Priority,
		AppClass:   cProfile.AppClass,
	}, nil
}

// Helper functions to convert between Go and C structures
func keyToC(key types.FlowKey) CFlowKey {
	return CFlowKey{
		SrcIP:   key.SrcIP,
		DstIP:   key.DstIP,
		SrcPort: key.SrcPort,
		DstPort: key.DstPort,
		IPProto: key.IPProto,
	}
}

func stateToC(state types.FlowState) CFlowState {
	return CFlowState{
		Verdict:     state.Verdict,
		QoSProfile:  state.QoSProfile,
		Flags:       state.Flags,
		PacketCount: uint32(state.PacketCount),
		ByteCount:   uint32(state.ByteCount),
		LastSeen:    state.LastSeen,
		CreatedAt:   state.CreatedAt,
		ExpiresAt:   state.ExpiresAt,
		JA3Hash:     state.JA3Hash,
		SNI:         state.SNI,
	}
}

func cStateToState(state CFlowState) types.FlowState {
	return types.FlowState{
		Verdict:     state.Verdict,
		QoSProfile:  state.QoSProfile,
		Flags:       state.Flags,
		PacketCount: uint64(state.PacketCount),
		ByteCount:   uint64(state.ByteCount),
		LastSeen:    state.LastSeen,
		CreatedAt:   state.CreatedAt,
		ExpiresAt:   state.ExpiresAt,
		JA3Hash:     state.JA3Hash,
		SNI:         state.SNI,
	}
}
