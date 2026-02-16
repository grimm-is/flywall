// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"fmt"
	"sync"
	"time"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// PerformanceManager manages all performance optimization components
type PerformanceManager struct {
	// Components
	optimizer       *TCOptimizer
	memoryPool      *MemoryPool
	cacheOptimizer  *CacheOptimizer
	hardwareOffload *HardwareOffload
	batchingManager *BatchingManager

	// Configuration
	config *PerformanceConfig

	// State
	mutex   sync.RWMutex
	enabled bool

	// Statistics
	stats *PerformanceStats

	// Logger
	logger *logging.Logger
}

// PerformanceConfig for performance management
type PerformanceConfig struct {
	Enabled               bool                   `json:"enabled"`
	TCOptimizer           *TCOptimizerConfig     `json:"tc_optimizer"`
	MemoryPool            *MemoryPoolConfig      `json:"memory_pool"`
	CacheOptimizer        *CacheConfig           `json:"cache_optimizer"`
	HardwareOffload       *HardwareOffloadConfig `json:"hardware_offload"`
	Batching              *BatchingConfig        `json:"batching"`
	AutoTuning            bool                   `json:"auto_tuning"`
	TuningInterval        time.Duration          `json:"tuning_interval"`
	MetricsExport         bool                   `json:"metrics_export"`
	MetricsExportInterval time.Duration          `json:"metrics_export_interval"`
}

// DefaultPerformanceConfig returns default performance configuration
func DefaultPerformanceConfig() *PerformanceConfig {
	return &PerformanceConfig{
		Enabled:               true,
		TCOptimizer:           DefaultTCOptimizerConfig(),
		MemoryPool:            DefaultMemoryPoolConfig(),
		CacheOptimizer:        DefaultCacheConfig(),
		HardwareOffload:       DefaultHardwareOffloadConfig(),
		Batching:              DefaultBatchingConfig(),
		AutoTuning:            true,
		TuningInterval:        30 * time.Second,
		MetricsExport:         true,
		MetricsExportInterval: 10 * time.Second,
	}
}

// PerformanceStats aggregates all performance statistics
type PerformanceStats struct {
	TCMetrics            *TCMetrics            `json:"tc_metrics"`
	MemoryPoolStats      *MemoryPoolStats      `json:"memory_pool_stats"`
	CacheStats           *CacheStats           `json:"cache_stats"`
	HardwareOffloadStats *HardwareOffloadStats `json:"hardware_offload_stats"`
	BatchingStats        *BatchingStats        `json:"batching_stats"`
	LastUpdate           time.Time             `json:"last_update"`
}

// NewPerformanceManager creates a new performance manager
func NewPerformanceManager(logger *logging.Logger, config *PerformanceConfig) *PerformanceManager {
	if config == nil {
		config = DefaultPerformanceConfig()
	}

	pm := &PerformanceManager{
		config: config,
		stats:  &PerformanceStats{},
		logger: logger,
	}

	// Initialize components
	pm.initializeComponents()

	// Start background tasks
	if config.AutoTuning {
		go pm.tuningWorker()
	}

	if config.MetricsExport {
		go pm.metricsExportWorker()
	}

	return pm
}

// initializeComponents initializes all performance components
func (pm *PerformanceManager) initializeComponents() {
	// Initialize TC optimizer
	if pm.config.TCOptimizer != nil {
		pm.optimizer = NewTCOptimizer(pm.logger, pm.config.TCOptimizer)
	}

	// Initialize memory pool
	if pm.config.MemoryPool != nil {
		pm.memoryPool = NewMemoryPool(pm.logger, pm.config.MemoryPool)
	}

	// Initialize cache optimizer
	if pm.config.CacheOptimizer != nil {
		pm.cacheOptimizer = NewCacheOptimizer(pm.logger, pm.config.CacheOptimizer)
	}

	// Initialize hardware offload
	if pm.config.HardwareOffload != nil {
		pm.hardwareOffload = NewHardwareOffload(pm.logger, pm.config.HardwareOffload)
	}

	// Initialize batching manager
	if pm.config.Batching != nil {
		pm.batchingManager = NewBatchingManager(pm.logger, pm.config.Batching)
	}
}

// Start starts the performance manager
func (pm *PerformanceManager) Start() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.config.Enabled {
		pm.logger.Info("Performance manager disabled")
		return nil
	}

	// Start TC optimizer
	if pm.optimizer != nil {
		if err := pm.optimizer.Start(); err != nil {
			return fmt.Errorf("failed to start TC optimizer: %w", err)
		}
	}

	// Start hardware offload
	if pm.hardwareOffload != nil {
		if err := pm.hardwareOffload.Start(); err != nil {
			pm.logger.Warn("Failed to start hardware offload", "error", err)
		}
	}

	// Start batching manager
	if pm.batchingManager != nil {
		if err := pm.batchingManager.Start(); err != nil {
			return fmt.Errorf("failed to start batching manager: %w", err)
		}
	}

	pm.enabled = true
	pm.logger.Info("Performance manager started")

	return nil
}

// Stop stops the performance manager
func (pm *PerformanceManager) Stop() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.enabled {
		return
	}

	pm.enabled = false

	// Stop TC optimizer
	if pm.optimizer != nil {
		pm.optimizer.Stop()
	}

	// Stop hardware offload
	if pm.hardwareOffload != nil {
		pm.hardwareOffload.Stop()
	}

	// Stop batching manager
	if pm.batchingManager != nil {
		pm.batchingManager.Stop()
	}

	pm.logger.Info("Performance manager stopped")
}

// ProcessPacketOptimized processes a packet with all optimizations
func (pm *PerformanceManager) ProcessPacketOptimized(packet []byte, key types.FlowKey, processor PacketProcessor) *PacketResult {
	pm.mutex.RLock()
	enabled := pm.enabled
	pm.mutex.RUnlock()

	if !enabled {
		// Bypass optimizations
		return processor(key)
	}

	// Get packet buffer from memory pool
	var packetBuf []byte
	if pm.memoryPool != nil {
		packetBuf = pm.memoryPool.GetPacketBuffer()
		defer pm.memoryPool.PutPacketBuffer(packetBuf)
		// Copy packet to pooled buffer
		copy(packetBuf, packet)
		packet = packetBuf
	}

	// Check flow cache first
	if pm.cacheOptimizer != nil {
		if cached, hit := pm.cacheOptimizer.GetFlow(key.Hash()); hit {
			// Return cached result
			if result, ok := cached.(*PacketResult); ok {
				return result
			}
		}
	}

	// Process packet through TC optimizer
	var result *PacketResult
	if pm.optimizer != nil {
		result = pm.optimizer.ProcessPacket(packet, key)
	} else {
		// Fallback to direct processing
		result = processor(key)
	}

	// Cache the result
	if pm.cacheOptimizer != nil {
		pm.cacheOptimizer.SetFlow(key.Hash(), result)
	}

	return result
}

// ProcessBatchOptimized processes a batch of packets with optimizations
func (pm *PerformanceManager) ProcessBatchOptimized(packets []*PacketTask, processor BatchProcessorFunc) *BatchResult {
	pm.mutex.RLock()
	enabled := pm.enabled
	pm.mutex.RUnlock()

	if !enabled {
		// Bypass optimizations
		return processor(packets)
	}

	// Get batch from memory pool
	var batch []*PacketTask
	if pm.memoryPool != nil {
		batch = pm.memoryPool.GetBatch()
		defer pm.memoryPool.PutBatch(batch)
		// Copy packets to pooled batch
		batch = append(batch, packets...)
	} else {
		batch = packets
	}

	// Process batch through TC optimizer
	var result *BatchResult
	if pm.optimizer != nil {
		tcResult := pm.optimizer.ProcessBatch(batch)
		// Convert TCBatchResult to BatchResult
		result = &BatchResult{
			Success: tcResult.Error == nil,
			Results: make([]interface{}, len(tcResult.Results)),
			Error:   tcResult.Error,
		}
		for i, r := range tcResult.Results {
			result.Results[i] = r
		}
	} else {
		// Fallback to direct processing
		result = processor(batch)
	}

	return result
}

// GetFlowStateOptimized gets flow state with cache optimization
func (pm *PerformanceManager) GetFlowStateOptimized(key types.FlowKey, getter FlowStateGetter) (types.FlowState, error) {
	pm.mutex.RLock()
	enabled := pm.enabled
	pm.mutex.RUnlock()

	if !enabled {
		// Bypass cache
		return getter(key)
	}

	// Check cache first
	if pm.cacheOptimizer != nil {
		if cached, hit := pm.cacheOptimizer.GetFlow(key.Hash()); hit {
			if state, ok := cached.(types.FlowState); ok {
				return state, nil
			}
		}
	}

	// Get from source
	state, err := getter(key)
	if err != nil {
		return state, err
	}

	// Cache the result
	if pm.cacheOptimizer != nil {
		pm.cacheOptimizer.SetFlow(key.Hash(), state)
	}

	return state, nil
}

// tuningWorker performs automatic performance tuning
func (pm *PerformanceManager) tuningWorker() {
	ticker := time.NewTicker(pm.config.TuningInterval)
	defer ticker.Stop()

	for range ticker.C {
		pm.performTuning()
	}
}

// performTuning performs automatic performance tuning
func (pm *PerformanceManager) performTuning() {
	pm.mutex.RLock()
	enabled := pm.enabled
	pm.mutex.RUnlock()

	if !enabled {
		return
	}

	// Get current metrics
	metrics := pm.GetMetrics()

	// TC optimizer tuning
	if pm.optimizer != nil && metrics.TCMetrics != nil {
		pm.tuneTCOptimizer(metrics.TCMetrics)
	}

	// Memory pool tuning
	if pm.memoryPool != nil && metrics.MemoryPoolStats != nil {
		pm.tuneMemoryPool(metrics.MemoryPoolStats)
	}

	// Cache tuning
	if pm.cacheOptimizer != nil && metrics.CacheStats != nil {
		pm.tuneCache(metrics.CacheStats)
	}

	pm.logger.Debug("Performance tuning completed")
}

// tuneTCOptimizer tunes TC optimizer parameters
func (pm *PerformanceManager) tuneTCOptimizer(metrics *TCMetrics) {
	// Adjust batch size based on latency
	if metrics.AvgLatency > 100*time.Microsecond {
		// High latency - increase batch size
		newSize := pm.config.TCOptimizer.BatchSize * 2
		if newSize <= 256 {
			pm.logger.Info("Increasing batch size due to high latency", "old", pm.config.TCOptimizer.BatchSize, "new", newSize)
			pm.config.TCOptimizer.BatchSize = newSize
		}
	} else if metrics.AvgLatency < 10*time.Microsecond && pm.config.TCOptimizer.BatchSize > 16 {
		// Low latency - can decrease batch size
		newSize := pm.config.TCOptimizer.BatchSize / 2
		if newSize >= 16 {
			pm.logger.Info("Decreasing batch size due to low latency", "old", pm.config.TCOptimizer.BatchSize, "new", newSize)
			pm.config.TCOptimizer.BatchSize = newSize
		}
	}

	// Adjust worker count based on CPU usage
	if metrics.CPUUsage > 0.9 && pm.config.TCOptimizer.MaxConcurrency > 1 {
		newCount := pm.config.TCOptimizer.MaxConcurrency - 1
		pm.logger.Info("High CPU usage detected, reducing worker count", "old", pm.config.TCOptimizer.MaxConcurrency, "new", newCount)
		pm.config.TCOptimizer.MaxConcurrency = newCount
		// In a real implementation we would signal the optimizer to adjust
	} else if metrics.CPUUsage < 0.4 && pm.config.TCOptimizer.MaxConcurrency < 32 {
		newCount := pm.config.TCOptimizer.MaxConcurrency + 1
		pm.logger.Info("Low CPU usage detected, increasing worker count", "old", pm.config.TCOptimizer.MaxConcurrency, "new", newCount)
		pm.config.TCOptimizer.MaxConcurrency = newCount
	}
}

// tuneMemoryPool tunes memory pool parameters
func (pm *PerformanceManager) tuneMemoryPool(stats *MemoryPoolStats) {
	// Check hit rates
	totalHits := stats.PacketPoolHits + stats.FlowKeyPoolHits + stats.FlowStatePoolHits
	totalMisses := stats.PacketPoolMisses + stats.FlowKeyPoolMisses + stats.FlowStatePoolMisses

	if totalHits+totalMisses > 0 {
		hitRate := float64(totalHits) / float64(totalHits+totalMisses)
		if hitRate < 0.8 {
			pm.logger.Warn("Low memory pool hit rate, increasing buffer count", "rate", hitRate)
			// Adjust pool size
			newCount := pm.config.MemoryPool.PacketPoolSize * 2
			if newCount <= 10000 {
				pm.config.MemoryPool.PacketPoolSize = newCount
				if pm.memoryPool != nil {
					pm.memoryPool.SetBufferCount(newCount)
				}
			}
		}
	}
}

// tuneCache tunes cache parameters
func (pm *PerformanceManager) tuneCache(stats *CacheStats) {
	// Check flow cache hit rate
	if stats.FlowHitRate < 0.9 {
		pm.logger.Info("Low flow cache hit rate, increasing cache size", "rate", stats.FlowHitRate)
		newSize := pm.config.CacheOptimizer.FlowCacheSize * 2
		if newSize <= 1000000 {
			pm.config.CacheOptimizer.FlowCacheSize = newSize
			if pm.cacheOptimizer != nil {
				pm.cacheOptimizer.SetFlowCacheSize(newSize)
			}
		}
	}

	// Check verdict cache hit rate
	if stats.VerdictHitRate < 0.8 {
		pm.logger.Info("Low verdict cache hit rate, increasing TTL", "rate", stats.VerdictHitRate)
		newTTL := pm.config.CacheOptimizer.VerdictCacheTTL * 2
		if newTTL <= 1*time.Hour {
			pm.config.CacheOptimizer.VerdictCacheTTL = newTTL
			if pm.cacheOptimizer != nil {
				pm.cacheOptimizer.SetVerdictCacheTTL(newTTL)
			}
		}
	}
}

// metricsExportWorker exports metrics periodically
func (pm *PerformanceManager) metricsExportWorker() {
	ticker := time.NewTicker(pm.config.MetricsExportInterval)
	defer ticker.Stop()

	for range ticker.C {
		pm.exportMetrics()
	}
}

// exportMetrics exports performance metrics
func (pm *PerformanceManager) exportMetrics() {
	metrics := pm.GetMetrics()

	// Export to monitoring system (Placeholder for Prometheus/InfluxDB)
	// In a real implementation, we would update Prometheus gauges/counters here.
	
	pm.logger.Debug("Metrics exported",
		"packets/sec", metrics.TCMetrics.PacketsPerSecond,
		"cache_hit_rate", metrics.CacheStats.FlowHitRate,
		"memory_pool_hits", metrics.MemoryPoolStats.PacketPoolHits)
}

// GetMetrics returns aggregated performance metrics
func (pm *PerformanceManager) GetMetrics() *PerformanceStats {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	stats := &PerformanceStats{
		LastUpdate: time.Now(),
	}

	// Get TC metrics
	if pm.optimizer != nil {
		stats.TCMetrics = pm.optimizer.GetMetrics()
	}

	// Get memory pool stats
	if pm.memoryPool != nil {
		stats.MemoryPoolStats = pm.memoryPool.GetStatistics()
	}

	// Get cache stats
	if pm.cacheOptimizer != nil {
		stats.CacheStats = pm.cacheOptimizer.GetStatistics()
	}

	// Get hardware offload stats
	if pm.hardwareOffload != nil {
		stats.HardwareOffloadStats = pm.hardwareOffload.GetStatistics()
	}

	// Get batching stats
	if pm.batchingManager != nil {
		stats.BatchingStats = pm.batchingManager.GetStatistics()
	}

	return stats
}

// GetOptimizer returns the TC optimizer
func (pm *PerformanceManager) GetOptimizer() *TCOptimizer {
	return pm.optimizer
}

// GetMemoryPool returns the memory pool
func (pm *PerformanceManager) GetMemoryPool() *MemoryPool {
	return pm.memoryPool
}

// GetCacheOptimizer returns the cache optimizer
func (pm *PerformanceManager) GetCacheOptimizer() *CacheOptimizer {
	return pm.cacheOptimizer
}

// GetHardwareOffload returns the hardware offload manager
func (pm *PerformanceManager) GetHardwareOffload() *HardwareOffload {
	return pm.hardwareOffload
}

// GetBatchingManager returns the batching manager
func (pm *PerformanceManager) GetBatchingManager() *BatchingManager {
	return pm.batchingManager
}

// Processor interfaces
type PacketProcessor func(key types.FlowKey) *PacketResult
type BatchProcessorFunc func(packets []*PacketTask) *BatchResult
type FlowStateGetter func(key types.FlowKey) (types.FlowState, error)

// Performance tuning recommendations
type TuningRecommendation struct {
	Component   string      `json:"component"`
	Parameter   string      `json:"parameter"`
	Current     interface{} `json:"current"`
	Recommended interface{} `json:"recommended"`
	Reason      string      `json:"reason"`
}

// GetTuningRecommendations returns tuning recommendations
func (pm *PerformanceManager) GetTuningRecommendations() []*TuningRecommendation {
	var recommendations []*TuningRecommendation

	metrics := pm.GetMetrics()

	// TC optimizer recommendations
	if metrics.TCMetrics != nil {
		if metrics.TCMetrics.AvgLatency > 100*time.Microsecond {
			recommendations = append(recommendations, &TuningRecommendation{
				Component:   "tc_optimizer",
				Parameter:   "batch_size",
				Current:     pm.config.TCOptimizer.BatchSize,
				Recommended: pm.config.TCOptimizer.BatchSize * 2,
				Reason:      "High latency detected, increase batch size",
			})
		}

		if metrics.TCMetrics.CPUUsage > 0.9 {
			recommendations = append(recommendations, &TuningRecommendation{
				Component:   "tc_optimizer",
				Parameter:   "max_concurrency",
				Current:     pm.config.TCOptimizer.MaxConcurrency,
				Recommended: pm.config.TCOptimizer.MaxConcurrency / 2,
				Reason:      "High CPU usage, reduce worker count",
			})
		}
	}

	// Cache recommendations
	if metrics.CacheStats != nil {
		if metrics.CacheStats.FlowHitRate < 0.9 {
			recommendations = append(recommendations, &TuningRecommendation{
				Component:   "cache_optimizer",
				Parameter:   "flow_cache_size",
				Current:     pm.config.CacheOptimizer.FlowCacheSize,
				Recommended: pm.config.CacheOptimizer.FlowCacheSize * 2,
				Reason:      "Low flow cache hit rate, increase cache size",
			})
		}
	}

	return recommendations
}
