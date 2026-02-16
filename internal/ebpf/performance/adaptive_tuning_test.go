// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"grimm.is/flywall/internal/logging"
)

func TestPerformanceManager_AdaptiveTuning(t *testing.T) {
	logger := logging.New(logging.DefaultConfig())
	config := DefaultPerformanceConfig()
	config.AutoTuning = false // We'll trigger manually

	pm := NewPerformanceManager(logger, config)
	pm.enabled = true

	// 1. Test Memory Pool Tuning
	t.Run("TuneMemoryPool", func(t *testing.T) {
		initialCount := config.MemoryPool.PacketPoolSize
		
		// Simulate low hit rate (10 hits, 90 misses = 10%)
		stats := &MemoryPoolStats{
			PacketPoolHits:   10,
			PacketPoolMisses: 90,
		}
		
		pm.tuneMemoryPool(stats)
		
		assert.Equal(t, initialCount*2, pm.config.MemoryPool.PacketPoolSize)
	})

	// 2. Test Cache Tuning
	t.Run("TuneCache", func(t *testing.T) {
		initialFlowSize := config.CacheOptimizer.FlowCacheSize
		initialVerdictTTL := config.CacheOptimizer.VerdictCacheTTL
		
		// Simulate low hit rates
		stats := &CacheStats{
			FlowHitRate:    0.5, // Below 0.9
			VerdictHitRate: 0.5, // Below 0.8
		}
		
		pm.tuneCache(stats)
		
		assert.Equal(t, initialFlowSize*2, pm.config.CacheOptimizer.FlowCacheSize)
		assert.Equal(t, initialVerdictTTL*2, pm.config.CacheOptimizer.VerdictCacheTTL)
	})
}
