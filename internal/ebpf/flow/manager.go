// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package flow

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cilium/ebpf"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// Manager manages flow state and coordinates with eBPF programs
type Manager struct {
	flowMap   *ebpf.Map
	logger    *logging.Logger
	config    *Config
	mutex     sync.RWMutex
	flows     map[types.FlowKey]*types.FlowState
	cleanupCh chan struct{}
	stopCh    chan struct{}

	// Tuning state
	lastTuningTime time.Time
	lowUsageStart  time.Time
}

// Config for the flow manager
type Config struct {
	FlowTimeout     time.Duration `json:"flow_timeout"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	MaxFlows        int           `json:"max_flows"`
}

// DefaultConfig returns default flow manager configuration
func DefaultConfig() *Config {
	return &Config{
		FlowTimeout:     5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		MaxFlows:        100000,
	}
}

// NewManager creates a new flow manager
func NewManager(flowMap *ebpf.Map, logger *logging.Logger, config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	return &Manager{
		flowMap:   flowMap,
		logger:    logger,
		config:    config,
		flows:     make(map[types.FlowKey]*types.FlowState),
		cleanupCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}
}

// Start starts the flow manager
func (m *Manager) Start() error {
	// Re-initialize channels to support restart
	m.stopCh = make(chan struct{})
	m.cleanupCh = make(chan struct{}, 1)

	// Start cleanup routine
	go m.cleanupRoutine()

	m.logger.Info("Flow manager started",
		"flow_timeout", m.config.FlowTimeout,
		"cleanup_interval", m.config.CleanupInterval,
		"max_flows", m.config.MaxFlows)

	return nil
}

// Stop stops the flow manager
func (m *Manager) Stop() error {
	// Safely close stopCh
	select {
	case <-m.stopCh:
		// Already closed
	default:
		close(m.stopCh)
	}

	// Wait for cleanup routine to finish
	select {
	case <-m.cleanupCh:
		m.logger.Info("Flow manager stopped")
	default:
		// If cleanupCh isn't closed yet, wait for it
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-m.cleanupCh:
			m.logger.Info("Flow manager stopped")
		case <-timer.C:
			m.logger.Warn("Flow manager stop timed out")
		}
	}

	return nil
}

// CreateFlow creates a new flow state
func (m *Manager) CreateFlow(key types.FlowKey, verdict uint8) (*types.FlowState, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if we're at max flows
	if len(m.flows) >= m.config.MaxFlows {
		return nil, fmt.Errorf("maximum number of flows reached (%d)", m.config.MaxFlows)
	}

	now := uint64(time.Now().UnixNano())
	state := &types.FlowState{
		CreatedAt:   now,
		LastSeen:    now,
		PacketCount: 0,
		ByteCount:   0,
		Verdict:     verdict,
		Flags:       0,
	}

	// Store in local cache
	m.flows[key] = state

	// Update eBPF map
	if m.flowMap != nil {
		if err := m.updateFlowInMap(key, state); err != nil {
			m.logger.Error("Failed to update flow in eBPF map",
				"error", err,
				"src_ip", int2ip(key.SrcIP),
				"dst_ip", int2ip(key.DstIP))
			delete(m.flows, key)
			return nil, fmt.Errorf("failed to update eBPF map: %w", err)
		}
	}

	m.logger.Debug("Created flow",
		"src_ip", int2ip(key.SrcIP),
		"dst_ip", int2ip(key.DstIP),
		"src_port", key.SrcPort,
		"dst_port", key.DstPort,
		"verdict", verdict)

	return state, nil
}

// UpdateFlow updates an existing flow state
func (m *Manager) UpdateFlow(key types.FlowKey, state *types.FlowState) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Update timestamp
	state.LastSeen = uint64(time.Now().UnixNano())

	// Store in local cache
	m.flows[key] = state

	// Update eBPF map
	if m.flowMap != nil {
		if err := m.updateFlowInMap(key, state); err != nil {
			return fmt.Errorf("failed to update eBPF map: %w", err)
		}
	}

	return nil
}

// GetFlow retrieves a flow state
func (m *Manager) GetFlow(key types.FlowKey) (*types.FlowState, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Check local cache first
	if state, exists := m.flows[key]; exists {
		return state, nil
	}

	// Check eBPF map
	if m.flowMap != nil {
		var state types.FlowState
		if err := m.flowMap.Lookup(&key, &state); err == nil {
			// Cache it locally
			m.flows[key] = &state
			return &state, nil
		}
	}

	return nil, fmt.Errorf("flow not found")
}

// DeleteFlow removes a flow
func (m *Manager) DeleteFlow(key types.FlowKey) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Remove from local cache
	delete(m.flows, key)

	// Remove from eBPF map
	if m.flowMap != nil {
		if err := m.flowMap.Delete(&key); err != nil {
			return fmt.Errorf("failed to delete from eBPF map: %w", err)
		}
	}

	return nil
}

// ListFlows returns all active flows
func (m *Manager) ListFlows() map[types.FlowKey]*types.FlowState {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Make a copy of the flows map
	result := make(map[types.FlowKey]*types.FlowState, len(m.flows))
	for k, v := range m.flows {
		state := *v // Copy the state
		result[k] = &state
	}

	return result
}

// GetFlowCount returns the number of active flows
func (m *Manager) GetFlowCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.flows)
}

// TrustFlow marks a flow as trusted
func (m *Manager) TrustFlow(key types.FlowKey) error {
	return m.setFlowVerdict(key, uint8(types.VerdictTrusted))
}

// BlockFlow marks a flow as blocked
func (m *Manager) BlockFlow(key types.FlowKey) error {
	return m.setFlowVerdict(key, uint8(types.VerdictDrop))
}

// setFlowVerdict updates the verdict for a flow
func (m *Manager) setFlowVerdict(key types.FlowKey, verdict uint8) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	state, exists := m.flows[key]
	if !exists {
		return fmt.Errorf("flow not found")
	}

	state.Verdict = verdict
	state.LastSeen = uint64(time.Now().UnixNano())

	// Set flags based on verdict
	if verdict == uint8(types.VerdictTrusted) {
		state.Flags |= uint16(types.FlowFlagOffloaded)
	} else if verdict == uint8(types.VerdictDrop) {
		state.Flags |= uint16(types.FlowFlagBlocked)
	}

	// Update eBPF map
	if m.flowMap != nil {
		if err := m.updateFlowInMap(key, state); err != nil {
			return fmt.Errorf("failed to update eBPF map: %w", err)
		}
	}

	m.logger.Info("Updated flow verdict",
		"src_ip", int2ip(key.SrcIP),
		"dst_ip", int2ip(key.DstIP),
		"verdict", verdict)

	return nil
}

// cleanupRoutine runs periodic cleanup of expired flows
func (m *Manager) cleanupRoutine() {
	cleanupCh := m.cleanupCh
	stopCh := m.stopCh
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredFlows()

			// Run adaptive tuning
			m.tuneMemoryPool()
			newInterval := m.tuneCache()
			if newInterval != m.config.CleanupInterval {
				m.config.CleanupInterval = newInterval
				ticker.Reset(newInterval)
			}
		case <-stopCh:
			// Safely close cleanupCh
			select {
			case <-cleanupCh:
				// Already closed
			default:
				close(cleanupCh)
			}
			return
		}
	}
}

// cleanupExpiredFlows removes flows that have expired
func (m *Manager) cleanupExpiredFlows() {
	timeoutNs := uint64(m.config.FlowTimeout.Nanoseconds())
	batchSize := 1000

	for {
		// Scanned expired keys
		var expiredBatch []types.FlowKey

		// Read lock to find expired flows
		m.mutex.RLock()
		now := uint64(time.Now().UnixNano())
		for key, state := range m.flows {
			if now-state.LastSeen > timeoutNs {
				expiredBatch = append(expiredBatch, key)
				if len(expiredBatch) >= batchSize {
					break
				}
			}
		}
		m.mutex.RUnlock()

		if len(expiredBatch) == 0 {
			break
		}

		// Write lock to delete them
		m.mutex.Lock()
		// Re-check time in case state changed between RUnlock and Lock
		// although for simple timeout it's usually fine to just delete.
		// We'll trust the scan but verify existence.
		deletedCount := 0
		for _, key := range expiredBatch {
			if state, exists := m.flows[key]; exists {
				// Re-verify timeout to be safe against race where packet came in
				if now-state.LastSeen > timeoutNs {
					delete(m.flows, key)
					deletedCount++

					if m.flowMap != nil {
						if err := m.flowMap.Delete(&key); err != nil {
							m.logger.Error("Failed to delete expired flow from eBPF map",
								"error", err,
								"src_ip", int2ip(key.SrcIP),
								"dst_ip", int2ip(key.DstIP))
						}
					}
				}
			}
		}
		m.mutex.Unlock()

		if deletedCount > 0 {
			m.logger.Debug("Cleaned up expired flows batch", "count", deletedCount)
		}

		// Small yield if we are processing many batches
		if len(expiredBatch) >= batchSize {
			time.Sleep(time.Millisecond)
		} else {
			break // Finished
		}
	}
}

// updateFlowInMap updates a flow in the eBPF map
func (m *Manager) updateFlowInMap(key types.FlowKey, state *types.FlowState) error {
	// Convert to C structure if needed
	// For now, we assume the Go structures are compatible
	return m.flowMap.Update(&key, state, ebpf.UpdateAny)
}

// Helper function to convert uint32 IP to string
func int2ip(ip uint32) string {
	return net.IPv4(
		byte(ip>>24),
		byte(ip>>16),
		byte(ip>>8),
		byte(ip),
	).String()
}

// tuneMemoryPool checks usage and logs recommendations
func (m *Manager) tuneMemoryPool() {
	m.mutex.RLock()
	count := len(m.flows)
	m.mutex.RUnlock()

	usage := float64(count) / float64(m.config.MaxFlows)

	if usage > 0.9 {
		m.logger.Warn("High flow table usage detected",
			"usage_percent", usage*100,
			"count", count,
			"max", m.config.MaxFlows,
			"recommendation", "Consider increasing MaxFlows and eBPF map size")
		// Reset low usage timer
		m.lowUsageStart = time.Time{}
	} else if usage < 0.1 {
		if m.lowUsageStart.IsZero() {
			m.lowUsageStart = time.Now()
		} else if time.Since(m.lowUsageStart) > 1*time.Hour {
			m.logger.Info("Sustained low flow table usage detected",
				"usage_percent", usage*100,
				"duration", time.Since(m.lowUsageStart),
				"recommendation", "Consider decreasing MaxFlows to save memory")
			// Reset to avoid spamming
			m.lowUsageStart = time.Now()
		}
	} else {
		// Reset low usage timer if we are in normal range
		m.lowUsageStart = time.Time{}
	}
}

// tuneCache adjusts cleanup interval based on load
func (m *Manager) tuneCache() time.Duration {
	m.mutex.RLock()
	count := len(m.flows)
	m.mutex.RUnlock()

	max := m.config.MaxFlows
	current := m.config.CleanupInterval

	// Dynamic adjustment
	// If load is high (>50%), clean up more frequently to free slots
	// If load is low (<10%), clean up less frequently to save CPU

	var target time.Duration = current

	if float64(count) > float64(max)*0.5 {
		// High load: Decrease interval (min 10s)
		target = current / 2
		if target < 10*time.Second {
			target = 10 * time.Second
		}
	} else if float64(count) < float64(max)*0.1 {
		// Low load: Increase interval (max 5m)
		target = current * 2
		if target > 5*time.Minute {
			target = 5 * time.Minute
		}
	} else {
		// Normal load: return to default if needed, or keep current?
		// For now, let's keep current unless it's very skewed from default?
		// Simple approach: just Stick to the adjusted value until load changes again.
		return current
	}

	if target != current {
		m.logger.Info("Adaptive cache tuning",
			"active_flows", count,
			"old_interval", current,
			"new_interval", target)
	}

	return target
}

// Global config instance
var config = DefaultConfig()
