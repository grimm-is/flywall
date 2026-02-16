// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"grimm.is/flywall/internal/logging"
)

// CacheOptimizer optimizes cache performance for TC operations
type CacheOptimizer struct {
	// Configuration
	config *CacheConfig

	// Caches
	flowCache    *FastCache
	verdictCache *FastCache
	patternCache *FastCache
	qosCache     *FastCache

	// Statistics
	stats *CacheStats

	// Logger
	logger *logging.Logger
}

// CacheConfig for cache optimization
type CacheConfig struct {
	Enabled          bool          `json:"enabled"`
	FlowCacheSize    int           `json:"flow_cache_size"`
	VerdictCacheSize int           `json:"verdict_cache_size"`
	PatternCacheSize int           `json:"pattern_cache_size"`
	QoSCacheSize     int           `json:"qos_cache_size"`
	FlowCacheTTL     time.Duration `json:"flow_cache_ttl"`
	VerdictCacheTTL  time.Duration `json:"verdict_cache_ttl"`
	PatternCacheTTL  time.Duration `json:"pattern_cache_ttl"`
	QoSCacheTTL      time.Duration `json:"qos_cache_ttl"`
	ShardCount       int           `json:"shard_count"`
	EnableMetrics    bool          `json:"enable_metrics"`
	MetricsInterval  time.Duration `json:"metrics_interval"`
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:          true,
		FlowCacheSize:    100000,
		VerdictCacheSize: 50000,
		PatternCacheSize: 10000,
		QoSCacheSize:     5000,
		FlowCacheTTL:     5 * time.Minute,
		VerdictCacheTTL:  1 * time.Minute,
		PatternCacheTTL:  30 * time.Minute,
		QoSCacheTTL:      10 * time.Minute,
		ShardCount:       64,
		EnableMetrics:    true,
		MetricsInterval:  10 * time.Second,
	}
}

// CacheStats tracks cache statistics
type CacheStats struct {
	FlowHits       uint64    `json:"flow_hits"`
	FlowMisses     uint64    `json:"flow_misses"`
	VerdictHits    uint64    `json:"verdict_hits"`
	VerdictMisses  uint64    `json:"verdict_misses"`
	PatternHits    uint64    `json:"pattern_hits"`
	PatternMisses  uint64    `json:"pattern_misses"`
	QoSHits        uint64    `json:"qos_hits"`
	QoSMisses      uint64    `json:"qos_misses"`
	FlowHitRate    float64   `json:"flow_hit_rate"`
	VerdictHitRate float64   `json:"verdict_hit_rate"`
	PatternHitRate float64   `json:"pattern_hit_rate"`
	QoSHitRate     float64   `json:"qos_hit_rate"`
	Evictions      uint64    `json:"evictions"`
	Expirations    uint64    `json:"expirations"`
	LastUpdate     time.Time `json:"last_update"`
}

// FastCache implements a high-performance cache with sharding
type FastCache struct {
	shards []*cacheShard
	config *FastCacheConfig
	stats  *atomic.Value // *CacheShardStats
}

// FastCacheConfig for fast cache configuration
type FastCacheConfig struct {
	Name       string
	Size       int
	TTL        time.Duration
	ShardCount int
}

// cacheShard represents a cache shard
type cacheShard struct {
	items   map[uint64]*cacheItem
	lru     *list.List
	mutex   sync.RWMutex
	stats   CacheShardStats
	ttl     time.Duration
	maxSize int
}

// cacheItem represents a cached item
type cacheItem struct {
	key     uint64
	value   interface{}
	expires time.Time
	element *list.Element
}

// CacheShardStats tracks shard statistics
type CacheShardStats struct {
	Hits        uint64
	Misses      uint64
	Evictions   uint64
	Expirations uint64
}

// NewCacheOptimizer creates a new cache optimizer
func NewCacheOptimizer(logger *logging.Logger, config *CacheConfig) *CacheOptimizer {
	if config == nil {
		config = DefaultCacheConfig()
	}

	optimizer := &CacheOptimizer{
		config: config,
		stats:  &CacheStats{},
		logger: logger,
	}

	// Initialize caches
	optimizer.initializeCaches()

	// Start metrics collection
	if config.EnableMetrics {
		go optimizer.metricsWorker()
	}

	return optimizer
}

// initializeCaches initializes all caches
func (co *CacheOptimizer) initializeCaches() {
	// Flow cache
	co.flowCache = NewFastCache(&FastCacheConfig{
		Name:       "flow",
		Size:       co.config.FlowCacheSize,
		TTL:        co.config.FlowCacheTTL,
		ShardCount: co.config.ShardCount,
	})

	// Verdict cache
	co.verdictCache = NewFastCache(&FastCacheConfig{
		Name:       "verdict",
		Size:       co.config.VerdictCacheSize,
		TTL:        co.config.VerdictCacheTTL,
		ShardCount: co.config.ShardCount,
	})

	// Pattern cache
	co.patternCache = NewFastCache(&FastCacheConfig{
		Name:       "pattern",
		Size:       co.config.PatternCacheSize,
		TTL:        co.config.PatternCacheTTL,
		ShardCount: co.config.ShardCount,
	})

	// QoS cache
	co.qosCache = NewFastCache(&FastCacheConfig{
		Name:       "qos",
		Size:       co.config.QoSCacheSize,
		TTL:        co.config.QoSCacheTTL,
		ShardCount: co.config.ShardCount,
	})
}

// NewFastCache creates a new fast cache
func NewFastCache(config *FastCacheConfig) *FastCache {
	shards := make([]*cacheShard, config.ShardCount)
	shardSize := config.Size / config.ShardCount

	for i := 0; i < config.ShardCount; i++ {
		shards[i] = &cacheShard{
			items:   make(map[uint64]*cacheItem, shardSize),
			lru:     list.New(),
			ttl:     config.TTL,
			maxSize: shardSize,
		}
	}

	return &FastCache{
		shards: shards,
		config: config,
		stats:  &atomic.Value{},
	}
}

// Get gets an item from the cache
func (fc *FastCache) Get(key uint64) (interface{}, bool) {
	shard := fc.getShard(key)
	return shard.get(key, fc.config.TTL)
}

// Set sets an item in the cache
func (fc *FastCache) Set(key uint64, value interface{}) {
	shard := fc.getShard(key)
	shard.set(key, value, fc.config.TTL, fc.config.Name)
}

// Delete deletes an item from the cache
func (fc *FastCache) Delete(key uint64) {
	shard := fc.getShard(key)
	shard.delete(key)
}

// Clear clears the cache
func (fc *FastCache) Clear() {
	for _, shard := range fc.shards {
		shard.clear()
	}
}

// getShard returns the shard for a key
func (fc *FastCache) getShard(key uint64) *cacheShard {
	return fc.shards[key%uint64(len(fc.shards))]
}

// get gets an item from a shard
func (cs *cacheShard) get(key uint64, ttl time.Duration) (interface{}, bool) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	item, exists := cs.items[key]
	if !exists {
		atomic.AddUint64(&cs.stats.Misses, 1)
		return nil, false
	}

	// Check expiration
	if time.Now().After(item.expires) {
		cs.mutex.RUnlock()
		cs.mutex.Lock()
		cs.removeItem(item)
		atomic.AddUint64(&cs.stats.Expirations, 1)
		cs.mutex.Unlock()
		cs.mutex.RLock()
		return nil, false
	}

	// Move to front of LRU
	cs.lru.MoveToFront(item.element)
	atomic.AddUint64(&cs.stats.Hits, 1)

	return item.value, true
}

// set sets an item in a shard
func (cs *cacheShard) set(key uint64, value interface{}, ttl time.Duration, name string) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// Check if item already exists
	if item, exists := cs.items[key]; exists {
		// Update existing item
		item.value = value
		item.expires = time.Now().Add(ttl)
		cs.lru.MoveToFront(item.element)
		return
	}

	// Check if cache is full
	if len(cs.items) >= cs.maxSize {
		// Evict LRU item
		if back := cs.lru.Back(); back != nil {
			cs.removeItem(back.Value.(*cacheItem))
			atomic.AddUint64(&cs.stats.Evictions, 1)
		}
	}

	// Add new item
	item := &cacheItem{
		key:     key,
		value:   value,
		expires: time.Now().Add(ttl),
	}
	item.element = cs.lru.PushFront(item)
	cs.items[key] = item
}

// delete deletes an item from a shard
func (cs *cacheShard) delete(key uint64) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if item, exists := cs.items[key]; exists {
		cs.removeItem(item)
	}
}

// clear clears a shard
func (cs *cacheShard) clear() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.items = make(map[uint64]*cacheItem)
	cs.lru = list.New()
}

// removeItem removes an item from the shard
func (cs *cacheShard) removeItem(item *cacheItem) {
	delete(cs.items, item.key)
	cs.lru.Remove(item.element)
}

// GetFlow gets a flow from the flow cache
func (co *CacheOptimizer) GetFlow(key uint64) (interface{}, bool) {
	if !co.config.Enabled {
		return nil, false
	}

	value, hit := co.flowCache.Get(key)
	if hit {
		atomic.AddUint64(&co.stats.FlowHits, 1)
	} else {
		atomic.AddUint64(&co.stats.FlowMisses, 1)
	}
	return value, hit
}

// SetFlow sets a flow in the flow cache
func (co *CacheOptimizer) SetFlow(key uint64, value interface{}) {
	if !co.config.Enabled {
		return
	}
	co.flowCache.Set(key, value)
}

// GetVerdict gets a verdict from the verdict cache
func (co *CacheOptimizer) GetVerdict(key uint64) (interface{}, bool) {
	if !co.config.Enabled {
		return nil, false
	}

	value, hit := co.verdictCache.Get(key)
	if hit {
		atomic.AddUint64(&co.stats.VerdictHits, 1)
	} else {
		atomic.AddUint64(&co.stats.VerdictMisses, 1)
	}
	return value, hit
}

// SetVerdict sets a verdict in the verdict cache
func (co *CacheOptimizer) SetVerdict(key uint64, value interface{}) {
	if !co.config.Enabled {
		return
	}
	co.verdictCache.Set(key, value)
}

// GetPattern gets a pattern from the pattern cache
func (co *CacheOptimizer) GetPattern(key uint64) (interface{}, bool) {
	if !co.config.Enabled {
		return nil, false
	}

	value, hit := co.patternCache.Get(key)
	if hit {
		atomic.AddUint64(&co.stats.PatternHits, 1)
	} else {
		atomic.AddUint64(&co.stats.PatternMisses, 1)
	}
	return value, hit
}

// SetPattern sets a pattern in the pattern cache
func (co *CacheOptimizer) SetPattern(key uint64, value interface{}) {
	if !co.config.Enabled {
		return
	}
	co.patternCache.Set(key, value)
}

// GetQoS gets a QoS profile from the QoS cache
func (co *CacheOptimizer) GetQoS(key uint64) (interface{}, bool) {
	if !co.config.Enabled {
		return nil, false
	}

	value, hit := co.qosCache.Get(key)
	if hit {
		atomic.AddUint64(&co.stats.QoSHits, 1)
	} else {
		atomic.AddUint64(&co.stats.QoSMisses, 1)
	}
	return value, hit
}

// SetQoS sets a QoS profile in the QoS cache
func (co *CacheOptimizer) SetQoS(key uint64, value interface{}) {
	if !co.config.Enabled {
		return
	}
	co.qosCache.Set(key, value)
}

// ClearExpired clears expired items from all caches
func (co *CacheOptimizer) ClearExpired() {
	if !co.config.Enabled {
		return
	}

	co.flowCache.Clear()
	co.verdictCache.Clear()
	co.patternCache.Clear()
	co.qosCache.Clear()
}

// metricsWorker collects cache metrics
func (co *CacheOptimizer) metricsWorker() {
	ticker := time.NewTicker(co.config.MetricsInterval)
	defer ticker.Stop()

	for range ticker.C {
		co.collectMetrics()
	}
}

// collectMetrics collects cache statistics
func (co *CacheOptimizer) collectMetrics() {
	// Calculate hit rates
	flowHits := atomic.LoadUint64(&co.stats.FlowHits)
	flowMisses := atomic.LoadUint64(&co.stats.FlowMisses)
	if flowHits+flowMisses > 0 {
		co.stats.FlowHitRate = float64(flowHits) / float64(flowHits+flowMisses)
	}

	verdictHits := atomic.LoadUint64(&co.stats.VerdictHits)
	verdictMisses := atomic.LoadUint64(&co.stats.VerdictMisses)
	if verdictHits+verdictMisses > 0 {
		co.stats.VerdictHitRate = float64(verdictHits) / float64(verdictHits+verdictMisses)
	}

	patternHits := atomic.LoadUint64(&co.stats.PatternHits)
	patternMisses := atomic.LoadUint64(&co.stats.PatternMisses)
	if patternHits+patternMisses > 0 {
		co.stats.PatternHitRate = float64(patternHits) / float64(patternHits+patternMisses)
	}

	qosHits := atomic.LoadUint64(&co.stats.QoSHits)
	qosMisses := atomic.LoadUint64(&co.stats.QoSMisses)
	if qosHits+qosMisses > 0 {
		co.stats.QoSHitRate = float64(qosHits) / float64(qosHits+qosMisses)
	}

	co.stats.LastUpdate = time.Now()

	// Log metrics
	co.logger.Debug("Cache metrics",
		"flow_hit_rate", co.stats.FlowHitRate,
		"verdict_hit_rate", co.stats.VerdictHitRate,
		"pattern_hit_rate", co.stats.PatternHitRate,
		"qos_hit_rate", co.stats.QoSHitRate)
}

// GetStatistics returns cache statistics
func (co *CacheOptimizer) GetStatistics() *CacheStats {
	return &CacheStats{
		FlowHits:       atomic.LoadUint64(&co.stats.FlowHits),
		FlowMisses:     atomic.LoadUint64(&co.stats.FlowMisses),
		VerdictHits:    atomic.LoadUint64(&co.stats.VerdictHits),
		VerdictMisses:  atomic.LoadUint64(&co.stats.VerdictMisses),
		PatternHits:    atomic.LoadUint64(&co.stats.PatternHits),
		PatternMisses:  atomic.LoadUint64(&co.stats.PatternMisses),
		QoSHits:        atomic.LoadUint64(&co.stats.QoSHits),
		QoSMisses:      atomic.LoadUint64(&co.stats.QoSMisses),
		FlowHitRate:    co.stats.FlowHitRate,
		VerdictHitRate: co.stats.VerdictHitRate,
		PatternHitRate: co.stats.PatternHitRate,
		QoSHitRate:     co.stats.QoSHitRate,
		Evictions:      atomic.LoadUint64(&co.stats.Evictions),
		Expirations:    atomic.LoadUint64(&co.stats.Expirations),
		LastUpdate:     co.stats.LastUpdate,
	}
}

// GetCacheStatistics returns detailed cache statistics
func (co *CacheOptimizer) GetCacheStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	// Flow cache stats
	stats["flow"] = map[string]interface{}{
		"hits":   atomic.LoadUint64(&co.stats.FlowHits),
		"misses": atomic.LoadUint64(&co.stats.FlowMisses),
		"rate":   co.stats.FlowHitRate,
	}

	// Verdict cache stats
	stats["verdict"] = map[string]interface{}{
		"hits":   atomic.LoadUint64(&co.stats.VerdictHits),
		"misses": atomic.LoadUint64(&co.stats.VerdictMisses),
		"rate":   co.stats.VerdictHitRate,
	}

	// Pattern cache stats
	stats["pattern"] = map[string]interface{}{
		"hits":   atomic.LoadUint64(&co.stats.PatternHits),
		"misses": atomic.LoadUint64(&co.stats.PatternMisses),
		"rate":   co.stats.PatternHitRate,
	}

	// QoS cache stats
	stats["qos"] = map[string]interface{}{
		"hits":   atomic.LoadUint64(&co.stats.QoSHits),
		"misses": atomic.LoadUint64(&co.stats.QoSMisses),
		"rate":   co.stats.QoSHitRate,
	}

	return stats
}

// SetFlowCacheSize updates the flow cache size
func (co *CacheOptimizer) SetFlowCacheSize(size int) {
	co.config.FlowCacheSize = size
	// Note: Existing cache is not resized, but configuration is updated
}

// SetVerdictCacheTTL updates the verdict cache TTL
func (co *CacheOptimizer) SetVerdictCacheTTL(ttl time.Duration) {
	co.config.VerdictCacheTTL = ttl
	// Update TTL in the verdict cache shards
	if co.verdictCache != nil {
		co.verdictCache.config.TTL = ttl
	}
}
