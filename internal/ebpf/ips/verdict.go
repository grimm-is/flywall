// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ips

import (
	"fmt"
	"sync"
	"time"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// VerdictApplicator applies IPS verdicts to packets
type VerdictApplicator struct {
	// Configuration
	config *VerdictConfig

	// State
	mutex sync.RWMutex

	// Statistics
	stats *VerdictStats

	// Logger
	logger *logging.Logger
}

// VerdictConfig for verdict application
type VerdictConfig struct {
	Enabled            bool          `json:"enabled"`
	DefaultAction      string        `json:"default_action"` // "allow", "drop", "monitor"
	HardFailOnErrors   bool          `json:"hard_fail_on_errors"`
	VerdictTimeout     time.Duration `json:"verdict_timeout"`
	MaxPendingVerdicts int           `json:"max_pending_verdicts"`
	BypassOnOverload   bool          `json:"bypass_on_overload"`
	LogVerdicts        bool          `json:"log_verdicts"`
	LogPacketSamples   bool          `json:"log_packet_samples"`
	SampleRate         float64       `json:"sample_rate"`
}

// DefaultVerdictConfig returns default verdict configuration
func DefaultVerdictConfig() *VerdictConfig {
	return &VerdictConfig{
		Enabled:            true,
		DefaultAction:      "allow",
		HardFailOnErrors:   false,
		VerdictTimeout:     100 * time.Millisecond,
		MaxPendingVerdicts: 1000,
		BypassOnOverload:   true,
		LogVerdicts:        true,
		LogPacketSamples:   false,
		SampleRate:         0.01, // 1%
	}
}

// VerdictStats tracks verdict application statistics
type VerdictStats struct {
	mutex             sync.RWMutex
	TotalVerdicts     uint64        `json:"total_verdicts"`
	AllowedVerdicts   uint64        `json:"allowed_verdicts"`
	DroppedVerdicts   uint64        `json:"dropped_verdicts"`
	MonitoredVerdicts uint64        `json:"monitored_verdicts"`
	OffloadedVerdicts uint64        `json:"offloaded_verdicts"`
	TimeoutVerdicts   uint64        `json:"timeout_verdicts"`
	ErrorVerdicts     uint64        `json:"error_verdicts"`
	BypassedVerdicts  uint64        `json:"bypassed_verdicts"`
	AvgVerdictTime    time.Duration `json:"avg_verdict_time"`
	LastUpdate        time.Time     `json:"last_update"`
}

// VerdictAction represents the action to take on a packet
type VerdictAction int

const (
	VerdictActionAllow VerdictAction = iota
	VerdictActionDrop
	VerdictActionMonitor
	VerdictActionOffload
	VerdictActionBypass
)

// String returns string representation of verdict action
func (v VerdictAction) String() string {
	switch v {
	case VerdictActionAllow:
		return "allow"
	case VerdictActionDrop:
		return "drop"
	case VerdictActionMonitor:
		return "monitor"
	case VerdictActionOffload:
		return "offload"
	case VerdictActionBypass:
		return "bypass"
	default:
		return "unknown"
	}
}

// VerdictResult represents the result of verdict application
type VerdictResult struct {
	Action      VerdictAction `json:"action"`
	Reason      string        `json:"reason"`
	FlowKey     types.FlowKey `json:"flow_key"`
	Verdict     uint8         `json:"verdict"`
	Flags       uint16        `json:"flags"`
	ProcessTime time.Duration `json:"process_time"`
	Sampled     bool          `json:"sampled"`
}

// VerdictCache caches recent verdicts for performance
type VerdictCache struct {
	cache   map[uint64]*CacheEntry
	mutex   sync.RWMutex
	maxSize int
}

// CacheEntry represents a cached verdict
type CacheEntry struct {
	Verdict *VerdictResult
	Expires time.Time
}

// NewVerdictApplicator creates a new verdict applicator
func NewVerdictApplicator(logger *logging.Logger, config *VerdictConfig) *VerdictApplicator {
	if config == nil {
		config = DefaultVerdictConfig()
	}

	va := &VerdictApplicator{
		config: config,
		stats:  &VerdictStats{},
		logger: logger,
	}

	return va
}

// ApplyVerdict applies an IPS verdict to a packet
func (va *VerdictApplicator) ApplyVerdict(
	flowKey types.FlowKey,
	flowState *types.FlowState,
	patternResult *MatchResult,
) *VerdictResult {
	start := time.Now()

	va.mutex.RLock()
	config := va.config
	va.mutex.RUnlock()

	if !config.Enabled {
		return &VerdictResult{
			Action:      VerdictActionAllow,
			Reason:      "verdict application disabled",
			FlowKey:     flowKey,
			ProcessTime: time.Since(start),
		}
	}

	result := &VerdictResult{
		FlowKey:     flowKey,
		Verdict:     flowState.Verdict,
		Flags:       flowState.Flags,
		ProcessTime: time.Since(start),
		Sampled:     va.shouldSample(),
	}

	// Determine action based on flow state and pattern matches
	result.Action = va.determineAction(flowState, patternResult, result)

	// Apply verdict-specific logic
	va.applyVerdictLogic(result, flowState)

	// Log verdict if enabled
	if config.LogVerdicts && (config.SampleRate == 0 || result.Sampled) {
		va.logVerdict(result)
	}

	// Update statistics
	va.updateStats(result)

	return result
}

// determineAction determines the action to take based on verdict and patterns
func (va *VerdictApplicator) determineAction(
	flowState *types.FlowState,
	patternResult *MatchResult,
	result *VerdictResult,
) VerdictAction {
	// Check pattern matches first (highest priority)
	if patternResult != nil && patternResult.Matched {
		switch patternResult.Action {
		case "block":
			result.Reason = fmt.Sprintf("pattern match blocked: %s", patternResult.Description)
			return VerdictActionDrop
		case "monitor":
			result.Reason = fmt.Sprintf("pattern match monitored: %s", patternResult.Description)
			return VerdictActionMonitor
		}
	}

	// Check flow state verdict
	switch flowState.Verdict {
	case uint8(types.VerdictDrop):
		result.Reason = "flow verdict: drop"
		return VerdictActionDrop

	case uint8(types.VerdictTrusted):
		// Check if flow is offloaded
		if flowState.Flags&uint16(types.FlowFlagOffloaded) != 0 {
			result.Reason = "flow offloaded to kernel"
			return VerdictActionOffload
		}

		// Check if flow is monitored
		if flowState.Flags&uint16(types.FlowFlagMonitored) != 0 {
			result.Reason = "flow monitored"
			return VerdictActionMonitor
		}

		result.Reason = "flow trusted"
		return VerdictActionAllow

	default: // VerdictUnknown
		// Apply default action
		switch va.config.DefaultAction {
		case "drop":
			result.Reason = "default action: drop"
			return VerdictActionDrop
		case "monitor":
			result.Reason = "default action: monitor"
			return VerdictActionMonitor
		default:
			result.Reason = "default action: allow"
			return VerdictActionAllow
		}
	}
}

// applyVerdictLogic applies verdict-specific logic
func (va *VerdictApplicator) applyVerdictLogic(result *VerdictResult, flowState *types.FlowState) {
	switch result.Action {
	case VerdictActionDrop:
		// Ensure drop verdict is set
		result.Verdict = uint8(types.VerdictDrop)

	case VerdictActionAllow:
		// Ensure allow verdict is set
		result.Verdict = uint8(types.VerdictTrusted)

	case VerdictActionMonitor:
		// Allow but mark for monitoring
		result.Verdict = uint8(types.VerdictTrusted)
		result.Flags |= uint16(types.FlowFlagMonitored)

	case VerdictActionOffload:
		// Mark for kernel offload
		result.Verdict = uint8(types.VerdictTrusted)
		result.Flags |= uint16(types.FlowFlagOffloaded)

	case VerdictActionBypass:
		// Bypass all processing
		result.Verdict = uint8(types.VerdictTrusted)
	}
}

// shouldSample determines if a packet should be sampled for logging
func (va *VerdictApplicator) shouldSample() bool {
	if va.config.SampleRate <= 0 {
		return false
	}
	if va.config.SampleRate >= 1 {
		return true
	}
	// Simple pseudo-random sampling
	return time.Now().UnixNano()%int64(1/va.config.SampleRate) == 0
}

// logVerdict logs a verdict result
func (va *VerdictApplicator) logVerdict(result *VerdictResult) {
	va.logger.Info("IPS verdict applied",
		"action", result.Action.String(),
		"reason", result.Reason,
		"flow", result.FlowKey.String(),
		"verdict", result.Verdict,
		"flags", result.Flags,
		"process_time", result.ProcessTime,
		"sampled", result.Sampled)
}

// updateStats updates verdict statistics
func (va *VerdictApplicator) updateStats(result *VerdictResult) {
	va.stats.mutex.Lock()
	defer va.stats.mutex.Unlock()

	va.stats.TotalVerdicts++

	switch result.Action {
	case VerdictActionAllow:
		va.stats.AllowedVerdicts++
	case VerdictActionDrop:
		va.stats.DroppedVerdicts++
	case VerdictActionMonitor:
		va.stats.MonitoredVerdicts++
	case VerdictActionOffload:
		va.stats.OffloadedVerdicts++
	case VerdictActionBypass:
		va.stats.BypassedVerdicts++
	}

	// Update average verdict time
	if va.stats.AvgVerdictTime == 0 {
		va.stats.AvgVerdictTime = result.ProcessTime
	} else {
		alpha := 0.1
		va.stats.AvgVerdictTime = time.Duration(
			float64(va.stats.AvgVerdictTime)*(1-alpha) + float64(result.ProcessTime)*alpha,
		)
	}

	va.stats.LastUpdate = time.Now()
}

// GetStatistics returns verdict application statistics
func (va *VerdictApplicator) GetStatistics() *VerdictStats {
	va.stats.mutex.Lock()
	defer va.stats.mutex.Unlock()

	stats := *va.stats
	return &stats
}

// SetConfig updates the verdict configuration
func (va *VerdictApplicator) SetConfig(config *VerdictConfig) {
	va.mutex.Lock()
	defer va.mutex.Unlock()
	va.config = config
	va.logger.Info("Verdict applicator configuration updated")
}

// GetConfig returns the current verdict configuration
func (va *VerdictApplicator) GetConfig() *VerdictConfig {
	va.mutex.RLock()
	defer va.mutex.RUnlock()
	return va.config
}

// ResetStatistics resets all statistics
func (va *VerdictApplicator) ResetStatistics() {
	va.stats.mutex.Lock()
	defer va.stats.mutex.Unlock()

	va.stats = &VerdictStats{}
	va.logger.Info("Verdict applicator statistics reset")
}

// VerdictCacheManager manages verdict caching
type VerdictCacheManager struct {
	cache *VerdictCache
	ttl   time.Duration
}

// NewVerdictCacheManager creates a new verdict cache manager
func NewVerdictCacheManager(maxSize int, ttl time.Duration) *VerdictCacheManager {
	return &VerdictCacheManager{
		cache: &VerdictCache{
			cache:   make(map[uint64]*CacheEntry),
			maxSize: maxSize,
		},
		ttl: ttl,
	}
}

// Get retrieves a cached verdict
func (vcm *VerdictCacheManager) Get(key uint64) *VerdictResult {
	vcm.cache.mutex.RLock()
	defer vcm.cache.mutex.RUnlock()

	entry, exists := vcm.cache.cache[key]
	if !exists || time.Now().After(entry.Expires) {
		return nil
	}

	return entry.Verdict
}

// Set stores a verdict in the cache
func (vcm *VerdictCacheManager) Set(key uint64, verdict *VerdictResult) {
	vcm.cache.mutex.Lock()
	defer vcm.cache.mutex.Unlock()

	// Remove oldest entry if cache is full
	if len(vcm.cache.cache) >= vcm.cache.maxSize {
		for k := range vcm.cache.cache {
			delete(vcm.cache.cache, k)
			break
		}
	}

	vcm.cache.cache[key] = &CacheEntry{
		Verdict: verdict,
		Expires: time.Now().Add(vcm.ttl),
	}
}

// Clear clears all cached verdicts
func (vcm *VerdictCacheManager) Clear() {
	vcm.cache.mutex.Lock()
	defer vcm.cache.mutex.Unlock()
	vcm.cache.cache = make(map[uint64]*CacheEntry)
}
