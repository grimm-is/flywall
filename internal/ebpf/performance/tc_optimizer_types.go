// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"runtime"
	"time"

	"grimm.is/flywall/internal/ebpf/types"
)

// TCOptimizerConfig for TC performance optimization
type TCOptimizerConfig struct {
	Enabled              bool          `json:"enabled"`
	CPUAffinity          bool          `json:"cpu_affinity"`
	HugePages            bool          `json:"huge_pages"`
	BatchSize            int           `json:"batch_size"`
	MaxConcurrency       int           `json:"max_concurrency"`
	WorkerQueueSize      int           `json:"worker_queue_size"`
	BatchTimeout         time.Duration `json:"batch_timeout"`
	MetricInterval       time.Duration `json:"metric_interval"`
	OptimizationInterval time.Duration `json:"optimization_interval"`
	AdaptiveBatching     bool          `json:"adaptive_batching"`
	LoadBalancing        bool          `json:"load_balancing"`
}

// DefaultTCOptimizerConfig returns default TC optimizer configuration
func DefaultTCOptimizerConfig() *TCOptimizerConfig {
	return &TCOptimizerConfig{
		Enabled:              true,
		CPUAffinity:          true,
		HugePages:            false, // Requires root privileges
		BatchSize:            32,
		MaxConcurrency:       runtime.NumCPU(),
		WorkerQueueSize:      1000,
		BatchTimeout:         100 * time.Microsecond,
		MetricInterval:       1 * time.Second,
		OptimizationInterval: 10 * time.Second,
		AdaptiveBatching:     true,
		LoadBalancing:        true,
	}
}

// TCMetrics tracks TC performance metrics
type TCMetrics struct {
	PacketsPerSecond uint64        `json:"packets_per_second"`
	BytesPerSecond   uint64        `json:"bytes_per_second"`
	AvgLatency       time.Duration `json:"avg_latency"`
	P95Latency       time.Duration `json:"p95_latency"`
	P99Latency       time.Duration `json:"p99_latency"`
	CPUUsage         float64       `json:"cpu_usage"`
	MemoryUsage      uint64        `json:"memory_usage"`
	CacheHitRate     float64       `json:"cache_hit_rate"`
	OffloadRate      float64       `json:"offload_rate"`
	DropRate         float64       `json:"drop_rate"`
	LastUpdate       time.Time     `json:"last_update"`
}

// PacketWorker processes packets in parallel
type PacketWorker struct {
	id      int
	queue   chan *PacketTask
	quit    chan struct{}
	metrics *WorkerMetrics
}

// PacketTask represents a packet processing task
type PacketTask struct {
	Packet     []byte
	Key        types.FlowKey
	Timestamp  time.Time
	ResponseCh chan *PacketResult
}

// PacketResult represents packet processing result
type PacketResult struct {
	Action      string
	FlowState   types.FlowState
	ProcessTime time.Duration
	Error       error
}

// BatchWorker processes packets in batches
type BatchWorker struct {
	id         int
	batchQueue chan *BatchTask
	quit       chan struct{}
	metrics    *WorkerMetrics
}

// BatchTask represents a batch processing task
type BatchTask struct {
	Packets    []*PacketTask
	BatchSize  int
	Timestamp  time.Time
	ResponseCh chan *TCBatchResult
}

// TCBatchResult represents TC batch processing result
type TCBatchResult struct {
	Results       []*PacketResult
	BatchTime     time.Duration
	PacketsPerSec float64
	Error         error
}

// WorkerMetrics tracks worker performance
type WorkerMetrics struct {
	PacketsProcessed uint64        `json:"packets_processed"`
	BatchesProcessed uint64        `json:"batches_processed"`
	AvgProcessTime   time.Duration `json:"avg_process_time"`
	QueueDepth       int           `json:"queue_depth"`
	LastUpdate       time.Time     `json:"last_update"`
}

// OptimizerStats tracks optimizer statistics
type OptimizerStats struct {
	TotalOptimizations uint64        `json:"total_optimizations"`
	BatchSizeChanges   uint64        `json:"batch_size_changes"`
	WorkerAdjustments  uint64        `json:"worker_adjustments"`
	CPUAffinityChanges uint64        `json:"cpu_affinity_changes"`
	LastOptimization   time.Time     `json:"last_optimization"`
	OptimizationTime   time.Duration `json:"optimization_time"`
}
