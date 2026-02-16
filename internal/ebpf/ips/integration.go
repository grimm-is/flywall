// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ips

import (
	"fmt"
	"net"
	"sync"
	"time"

	"grimm.is/flywall/internal/ebpf/flow"
	"grimm.is/flywall/internal/ebpf/programs"
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/learning"
	"grimm.is/flywall/internal/logging"
)

// Integration connects the IPS learning engine with eBPF TC fast path
type Integration struct {
	tcProgram         *programs.TCOffloadProgram
	flowManager       *flow.Manager
	engine            *learning.Engine
	patternMatcher    *PatternMatcher
	patternDB         *PatternDB
	verdictApplicator *VerdictApplicator
	verdictCache      *VerdictCacheManager
	logger            *logging.Logger

	// Configuration
	config *Config

	// State
	mutex   sync.RWMutex
	enabled bool
	stats   *Statistics

	// Channels for async processing
	verdictCh chan VerdictRequest
	statsCh   chan StatUpdate
}

// Config for IPS integration
type Config struct {
	Enabled            bool          `json:"enabled"`
	InspectionWindow   int           `json:"inspection_window"` // Packets before offload
	OffloadThreshold   int           `json:"offload_threshold"` // Flows before offload
	MaxPendingFlows    int           `json:"max_pending_flows"` // Max flows tracking
	CleanupInterval    time.Duration `json:"cleanup_interval"`
	StatsFlushInterval time.Duration `json:"stats_flush_interval"`

	// Pattern matching configuration
	PatternConfig   *PatternConfig   `json:"pattern_config"`
	PatternDBConfig *PatternDBConfig `json:"pattern_db_config"`

	// Verdict application configuration
	VerdictConfig    *VerdictConfig `json:"verdict_config"`
	VerdictCacheSize int            `json:"verdict_cache_size"`
	VerdictCacheTTL  time.Duration  `json:"verdict_cache_ttl"`
}

// DefaultConfig returns default IPS integration configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:            true,
		InspectionWindow:   10,
		OffloadThreshold:   5,
		MaxPendingFlows:    10000,
		CleanupInterval:    5 * time.Minute,
		StatsFlushInterval: 30 * time.Second,
		PatternConfig:      DefaultPatternConfig(),
		PatternDBConfig:    DefaultPatternDBConfig(),
		VerdictConfig:      DefaultVerdictConfig(),
		VerdictCacheSize:   10000,
		VerdictCacheTTL:    5 * time.Minute,
	}
}

// VerdictRequest represents a packet needing IPS inspection
type VerdictRequest struct {
	Key        types.FlowKey
	PacketInfo *learning.PacketInfo
	ResponseCh chan VerdictResponse
	Timestamp  time.Time
}

// VerdictResponse is the IPS decision for a packet
type VerdictResponse struct {
	Verdict     types.FlowState
	Offload     bool
	Error       error
	ProcessTime time.Duration
}

// Statistics for IPS integration
type Statistics struct {
	mutex             sync.RWMutex
	PacketsInspected  uint64
	PacketsAllowed    uint64
	PacketsDropped    uint64
	PacketsOffloaded  uint64
	FlowsTracked      uint64
	FlowsOffloaded    uint64
	InspectionLatency time.Duration
	LastUpdate        time.Time
}

// NewIntegration creates a new IPS integration
func NewIntegration(
	tcProgram *programs.TCOffloadProgram,
	flowManager *flow.Manager,
	engine *learning.Engine,
	logger *logging.Logger,
	config *Config,
) *Integration {
	if config == nil {
		config = DefaultConfig()
	}

	integration := &Integration{
		tcProgram:   tcProgram,
		flowManager: flowManager,
		engine:      engine,
		logger:      logger,
		config:      config,
		stats:       &Statistics{},
		verdictCh:   make(chan VerdictRequest, 1000),
		statsCh:     make(chan StatUpdate, 100),
	}

	// Initialize pattern matcher
	if config.PatternConfig != nil {
		integration.patternMatcher = NewPatternMatcher(logger, config.PatternConfig)
	}

	// Initialize pattern database
	if integration.patternMatcher != nil && config.PatternDBConfig != nil {
		integration.patternDB = NewPatternDB(integration.patternMatcher, logger, config.PatternDBConfig)
	}

	// Initialize verdict applicator
	if config.VerdictConfig != nil {
		integration.verdictApplicator = NewVerdictApplicator(logger, config.VerdictConfig)
	}

	// Initialize verdict cache
	if config.VerdictCacheSize > 0 {
		integration.verdictCache = NewVerdictCacheManager(config.VerdictCacheSize, config.VerdictCacheTTL)
	}

	return integration
}

// Start starts the IPS integration
func (i *Integration) Start() error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if !i.config.Enabled {
		i.logger.Info("IPS integration disabled")
		return nil
	}

	if i.tcProgram == nil {
		return fmt.Errorf("TC program not available")
	}

	if i.flowManager == nil {
		return fmt.Errorf("flow manager not available")
	}

	if i.engine == nil {
		return fmt.Errorf("learning engine not available")
	}

	i.enabled = true

	// Start pattern database
	if i.patternDB != nil {
		if err := i.patternDB.Start(); err != nil {
			i.logger.Warn("Failed to start pattern database", "error", err)
		}
	}

	// Start background workers
	go i.verdictWorker()
	go i.statsWorker()
	go i.cleanupWorker()

	i.logger.Info("IPS integration started",
		"inspection_window", i.config.InspectionWindow,
		"offload_threshold", i.config.OffloadThreshold,
		"pattern_matching", i.patternMatcher != nil)

	return nil
}

// Stop stops the IPS integration
func (i *Integration) Stop() {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if !i.enabled {
		return
	}

	i.enabled = false

	// Stop pattern database
	if i.patternDB != nil {
		i.patternDB.Stop()
	}

	close(i.verdictCh)
	close(i.statsCh)

	i.logger.Info("IPS integration stopped")
}

// ProcessPacket processes a packet through the IPS engine
func (i *Integration) ProcessPacket(key types.FlowKey, skbInfo *SKBInfo) (types.FlowState, error) {
	i.mutex.RLock()
	enabled := i.enabled
	i.mutex.RUnlock()

	if !enabled {
		// Default to allow if integration is disabled
		return types.FlowState{
			Verdict: uint8(types.VerdictTrusted),
		}, nil
	}

	// Convert to learning engine packet info
	packetInfo := &learning.PacketInfo{
		SrcMAC:    skbInfo.SrcMAC,
		SrcIP:     int2ip(key.SrcIP),
		DstIP:     int2ip(key.DstIP),
		Protocol:  protoToString(key.IPProto),
		DstPort:   int(key.DstPort),
		Interface: "",
		Policy:    "",
	}

	// Check if we have an existing flow state
	state, err := i.flowManager.GetFlow(key)
	if err == nil {
		// Existing flow - check if it's offloaded
		if state.Verdict == uint8(types.VerdictTrusted) &&
			(state.Flags&uint16(types.FlowFlagOffloaded)) != 0 {
			// Flow is offloaded, return trusted verdict
			return *state, nil
		}
	}

	// New or non-offloaded flow - request IPS verdict
	responseCh := make(chan VerdictResponse, 1)
	request := VerdictRequest{
		Key:        key,
		PacketInfo: packetInfo,
		ResponseCh: responseCh,
		Timestamp:  time.Now(),
	}

	select {
	case i.verdictCh <- request:
		// Sent for processing
	default:
		// Channel full, default to allow
		i.logger.Warn("IPS verdict channel full, defaulting to allow")
		return types.FlowState{
			Verdict: uint8(types.VerdictTrusted),
		}, nil
	}

	// Wait for response with timeout
	select {
	case response := <-responseCh:
		if response.Error != nil {
			i.logger.Error("IPS processing error", "error", response.Error)
			// Default to allow on error
			return types.FlowState{
				Verdict: uint8(types.VerdictTrusted),
			}, nil
		}
		return response.Verdict, nil

	case <-time.After(100 * time.Millisecond):
		i.logger.Warn("IPS verdict timeout, defaulting to allow")
		return types.FlowState{
			Verdict: uint8(types.VerdictTrusted),
		}, nil
	}
}

// verdictWorker processes packets through the IPS engine
func (i *Integration) verdictWorker() {
	for request := range i.verdictCh {
		start := time.Now()

		// Check verdict cache first
		var flowState types.FlowState
		var patternResult *MatchResult
		var cached bool

		if i.verdictCache != nil {
			cacheKey := request.Key.Hash()
			if cachedVerdict := i.verdictCache.Get(cacheKey); cachedVerdict != nil {
				// Use cached verdict
				flowState = types.FlowState{
					Verdict: cachedVerdict.Verdict,
					Flags:   cachedVerdict.Flags,
				}
				cached = true
			}
		}

		if !cached {
			// Process through learning engine
			verdict, err := i.engine.ProcessPacketInline(request.PacketInfo)

			if err != nil {
				i.logger.Error("IPS engine error", "error", err)
				flowState = types.FlowState{
					Verdict: uint8(types.VerdictTrusted),
				}
			} else {
				// Convert engine verdict to flow state
				switch verdict {
				case learning.VerdictDrop:
					flowState = types.FlowState{
						Verdict: uint8(types.VerdictDrop),
					}

				case learning.VerdictAllow:
					flowState = types.FlowState{
						Verdict: uint8(types.VerdictTrusted),
					}

				case learning.VerdictOffload:
					flowState = types.FlowState{
						Verdict: uint8(types.VerdictTrusted),
						Flags:   uint16(types.FlowFlagOffloaded),
					}

				default: // VerdictInspect
					flowState = types.FlowState{
						Verdict: uint8(types.VerdictUnknown),
					}
				}

				// Apply pattern matching if enabled and verdict is not drop
				if i.patternMatcher != nil && verdict != learning.VerdictDrop {
					// Create packet data for pattern matching
					packetData := &PacketData{
						SrcIP:    request.Key.SrcIP,
						DstIP:    request.Key.DstIP,
						SrcPort:  request.Key.SrcPort,
						DstPort:  request.Key.DstPort,
						Protocol: request.Key.IPProto,
						Payload:  []byte(request.PacketInfo.Payload),
					}

					// Match against patterns
					patternResult = i.patternMatcher.MatchPacket(packetData)
					if patternResult.Matched {
						i.logger.Debug("Pattern match detected",
							"flow", request.Key.String(),
							"rules", patternResult.RuleIDs,
							"action", patternResult.Action)

						// Override verdict based on pattern match
						switch patternResult.Action {
						case "block":
							flowState = types.FlowState{
								Verdict: uint8(types.VerdictDrop),
							}
						case "monitor":
							// Keep current verdict but mark for monitoring
							flowState.Flags |= uint16(types.FlowFlagMonitored)
						}
					}
				}
			}

			// Cache the verdict
			if i.verdictCache != nil {
				cacheKey := request.Key.Hash()
				i.verdictCache.Set(cacheKey, &VerdictResult{
					Action:  VerdictActionAllow, // Will be updated by applicator
					Verdict: flowState.Verdict,
					Flags:   flowState.Flags,
				})
			}
		}

		// Apply verdict logic
		var verdictResult *VerdictResult
		if i.verdictApplicator != nil {
			verdictResult = i.verdictApplicator.ApplyVerdict(request.Key, &flowState, patternResult)
		} else {
			// Default verdict application
			verdictResult = &VerdictResult{
				Action:      VerdictActionAllow,
				Reason:      "no verdict applicator",
				FlowKey:     request.Key,
				Verdict:     flowState.Verdict,
				Flags:       flowState.Flags,
				ProcessTime: time.Since(start),
			}
		}

		// Update flow state based on verdict result
		flowState.Verdict = verdictResult.Verdict
		flowState.Flags = verdictResult.Flags

		// Update flow state in TC program
		if err := i.tcProgram.UpdateFlow(request.Key, flowState); err != nil {
			i.logger.Error("Failed to update flow state", "error", err)
		}

		// Update flow manager
		if err := i.flowManager.UpdateFlow(request.Key, &flowState); err != nil {
			i.logger.Error("Failed to update flow manager", "error", err)
		}

		// Send response
		response := VerdictResponse{
			Verdict:     flowState,
			Offload:     verdictResult.Action == VerdictActionOffload,
			Error:       nil,
			ProcessTime: verdictResult.ProcessTime,
		}

		select {
		case request.ResponseCh <- response:
		default:
			i.logger.Warn("Failed to send IPS verdict response")
		}

		// Update statistics
		i.statsCh <- StatUpdate{
			Type:      "packet",
			Verdict:   flowState.Verdict,
			Offloaded: verdictResult.Action == VerdictActionOffload,
			Latency:   verdictResult.ProcessTime,
		}
	}
}

// SKBInfo represents packet information from skb
type SKBInfo struct {
	SrcMAC    string
	DstMAC    string
	Length    uint32
	Timestamp time.Time
}

// StatUpdate represents a statistics update
type StatUpdate struct {
	Type      string
	Verdict   uint8
	Offloaded bool
	Latency   time.Duration
}

// statsWorker aggregates statistics
func (i *Integration) statsWorker() {
	ticker := time.NewTicker(i.config.StatsFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case update, ok := <-i.statsCh:
			if !ok {
				return
			}
			i.updateStats(update)

		case <-ticker.C:
			i.flushStats()
		}
	}
}

// updateStats updates internal statistics
func (i *Integration) updateStats(update StatUpdate) {
	i.stats.mutex.Lock()
	defer i.stats.mutex.Unlock()

	i.stats.PacketsInspected++

	switch update.Verdict {
	case types.VerdictTrusted:
		i.stats.PacketsAllowed++
		if update.Offloaded {
			i.stats.PacketsOffloaded++
		}
	case types.VerdictDrop:
		i.stats.PacketsDropped++
	}

	// Update latency (exponential moving average)
	if i.stats.InspectionLatency == 0 {
		i.stats.InspectionLatency = update.Latency
	} else {
		alpha := 0.1
		i.stats.InspectionLatency = time.Duration(
			float64(i.stats.InspectionLatency)*(1-alpha) + float64(update.Latency)*alpha,
		)
	}

	i.stats.LastUpdate = time.Now()
}

// flushStats writes statistics to logs/metrics
func (i *Integration) flushStats() {
	i.stats.mutex.RLock()
	stats := *i.stats
	i.stats.mutex.RUnlock()

	i.logger.Debug("IPS integration statistics",
		"packets_inspected", stats.PacketsInspected,
		"packets_allowed", stats.PacketsAllowed,
		"packets_dropped", stats.PacketsDropped,
		"packets_offloaded", stats.PacketsOffloaded,
		"flows_tracked", stats.FlowsTracked,
		"flows_offloaded", stats.FlowsOffloaded,
		"avg_latency", stats.InspectionLatency)
}

// cleanupWorker performs periodic cleanup
func (i *Integration) cleanupWorker() {
	ticker := time.NewTicker(i.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		i.cleanup()
	}
}

// cleanup removes old flows and performs maintenance
func (i *Integration) cleanup() {
	// Get all flows from flow manager
	flows := i.flowManager.ListFlows()

	now := time.Now()
	expired := 0

	for key, state := range flows {
		// Check if flow is expired (no activity for 10 minutes)
		if now.Sub(time.Unix(0, int64(state.CreatedAt))) > 10*time.Minute {
			if err := i.flowManager.DeleteFlow(key); err != nil {
				i.logger.Error("Failed to delete expired flow", "error", err)
			} else {
				expired++
			}
		}
	}

	if expired > 0 {
		i.logger.Info("Cleaned up expired flows", "count", expired)
	}
}

// GetStatistics returns current statistics
func (i *Integration) GetStatistics() *Statistics {
	i.stats.mutex.RLock()
	defer i.stats.mutex.RUnlock()

	// Return a copy to prevent modification
	stats := *i.stats
	return &stats
}

// GetPatternMatcher returns the pattern matcher
func (i *Integration) GetPatternMatcher() *PatternMatcher {
	return i.patternMatcher
}

// GetPatternDB returns the pattern database
func (i *Integration) GetPatternDB() *PatternDB {
	return i.patternDB
}

// UpdatePatterns triggers a pattern database update
func (i *Integration) UpdatePatterns() error {
	if i.patternDB == nil {
		return fmt.Errorf("pattern database not available")
	}
	return i.patternDB.UpdateNow()
}

// AddPatternSignature adds a new signature
func (i *Integration) AddPatternSignature(sig *Signature) error {
	if i.patternMatcher == nil {
		return fmt.Errorf("pattern matcher not available")
	}
	return i.patternMatcher.AddSignature(sig)
}

// GetVerdictApplicator returns the verdict applicator
func (i *Integration) GetVerdictApplicator() *VerdictApplicator {
	return i.verdictApplicator
}

// GetVerdictCache returns the verdict cache
func (i *Integration) GetVerdictCache() *VerdictCacheManager {
	return i.verdictCache
}

// ClearVerdictCache clears the verdict cache
func (i *Integration) ClearVerdictCache() {
	if i.verdictCache != nil {
		i.verdictCache.Clear()
		i.logger.Info("Verdict cache cleared")
	}
}

// Helper functions
func int2ip(ip uint32) string {
	return net.IPv4(
		byte(ip>>24),
		byte(ip>>16&0xff),
		byte(ip>>8&0xff),
		byte(ip&0xff),
	).String()
}

func protoToString(proto uint8) string {
	switch proto {
	case 1:
		return "ICMP"
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	default:
		return fmt.Sprintf("PROTO_%d", proto)
	}
}
