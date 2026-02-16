// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// BatchingManager manages packet batching for performance optimization
type BatchingManager struct {
	// Configuration
	config *BatchingConfig

	// State
	mutex   sync.RWMutex
	enabled bool

	// Batch processors
	processors map[string]*BatchProcessor

	// Statistics
	stats *BatchingStats

	// Logger
	logger *logging.Logger
}

// BatchingConfig for batching configuration
type BatchingConfig struct {
	Enabled             bool          `json:"enabled"`
	MaxBatchSize        int           `json:"max_batch_size"`
	MaxBatchDelay       time.Duration `json:"max_batch_delay"`
	MinBatchSize        int           `json:"min_batch_size"`
	FlushInterval       time.Duration `json:"flush_interval"`
	AdaptiveBatching    bool          `json:"adaptive_batching"`
	LoadBasedAdjustment bool          `json:"load_based_adjustment"`
	HighLoadThreshold   float64       `json:"high_load_threshold"`
	LowLoadThreshold    float64       `json:"low_load_threshold"`
	StatsInterval       time.Duration `json:"stats_interval"`
}

// DefaultBatchingConfig returns default batching configuration
func DefaultBatchingConfig() *BatchingConfig {
	return &BatchingConfig{
		Enabled:             true,
		MaxBatchSize:        64,
		MaxBatchDelay:       100 * time.Microsecond,
		MinBatchSize:        8,
		FlushInterval:       10 * time.Millisecond,
		AdaptiveBatching:    true,
		LoadBasedAdjustment: true,
		HighLoadThreshold:   0.8,
		LowLoadThreshold:    0.3,
		StatsInterval:       1 * time.Second,
	}
}

// BatchProcessor processes batches of packets
type BatchProcessor struct {
	name      string
	config    *BatchingConfig
	batchChan chan *Batch
	flushChan chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	metrics   *BatchMetrics
	handler   BatchHandler
	logger    *logging.Logger
}

// Batch represents a batch of packets
type Batch struct {
	Packets   []*PacketItem `json:"packets"`
	Timestamp time.Time     `json:"timestamp"`
	Size      int           `json:"size"`
	MaxDelay  time.Duration `json:"max_delay"`
	Processor string        `json:"processor"`
}

// PacketItem represents a packet in a batch
type PacketItem struct {
	Key        types.FlowKey     `json:"key"`
	Data       []byte            `json:"data"`
	Timestamp  time.Time         `json:"timestamp"`
	ResponseCh chan *BatchResult `json:"-"`
}

// BatchResult represents the result of batch processing
type BatchResult struct {
	Success     bool          `json:"success"`
	Results     []interface{} `json:"results"`
	Error       error         `json:"error"`
	ProcessTime time.Duration `json:"process_time"`
}

// BatchMetrics tracks batch processor metrics
type BatchMetrics struct {
	mutex          sync.RWMutex
	TotalBatches   uint64        `json:"total_batches"`
	TotalPackets   uint64        `json:"total_packets"`
	AvgBatchSize   float64       `json:"avg_batch_size"`
	AvgBatchDelay  time.Duration `json:"avg_batch_delay"`
	MaxBatchSize   int           `json:"max_batch_size"`
	MinBatchSize   int           `json:"min_batch_size"`
	FlushedBatches uint64        `json:"flushed_batches"`
	TimeoutBatches uint64        `json:"timeout_batches"`
	ProcessTime    time.Duration `json:"process_time"`
	LastUpdate     time.Time     `json:"last_update"`
}

// BatchingStats aggregates batching statistics
type BatchingStats struct {
	Processors    map[string]*BatchMetrics `json:"processors"`
	TotalBatches  uint64                   `json:"total_batches"`
	TotalPackets  uint64                   `json:"total_packets"`
	AvgBatchSize  float64                  `json:"avg_batch_size"`
	AvgBatchDelay time.Duration            `json:"avg_batch_delay"`
	LastUpdate    time.Time                `json:"last_update"`
}

// BatchHandler handles batch processing
type BatchHandler interface {
	ProcessBatch(batch *Batch) *BatchResult
}

// NewBatchingManager creates a new batching manager
func NewBatchingManager(logger *logging.Logger, config *BatchingConfig) *BatchingManager {
	if config == nil {
		config = DefaultBatchingConfig()
	}

	bm := &BatchingManager{
		config:     config,
		processors: make(map[string]*BatchProcessor),
		stats: &BatchingStats{
			Processors: make(map[string]*BatchMetrics),
		},
		logger: logger,
	}

	return bm
}

// Start starts the batching manager
func (bm *BatchingManager) Start() error {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	if !bm.config.Enabled {
		bm.logger.Info("Batching manager disabled")
		return nil
	}

	bm.enabled = true

	// Start statistics collection
	go bm.statsWorker()

	bm.logger.Info("Batching manager started",
		"max_batch_size", bm.config.MaxBatchSize,
		"max_batch_delay", bm.config.MaxBatchDelay)

	return nil
}

// Stop stops the batching manager
func (bm *BatchingManager) Stop() {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	if !bm.enabled {
		return
	}

	bm.enabled = false

	// Stop all processors
	for _, processor := range bm.processors {
		processor.Stop()
	}

	bm.processors = make(map[string]*BatchProcessor)

	bm.logger.Info("Batching manager stopped")
}

// RegisterProcessor registers a batch processor
func (bm *BatchingManager) RegisterProcessor(name string, handler BatchHandler, config *BatchingConfig) error {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	if _, exists := bm.processors[name]; exists {
		return fmt.Errorf("processor %s already registered", name)
	}

	if config == nil {
		config = bm.config
	}

	processor := NewBatchProcessor(name, handler, config, bm.logger)
	bm.processors[name] = processor
	bm.stats.Processors[name] = processor.metrics

	// Start the processor
	if err := processor.Start(); err != nil {
		return fmt.Errorf("failed to start processor %s: %w", name, err)
	}

	bm.logger.Info("Batch processor registered", "name", name)

	return nil
}

// ProcessPacket adds a packet to the appropriate batch processor
func (bm *BatchingManager) ProcessPacket(processorName string, key types.FlowKey, data []byte) (*BatchResult, error) {
	bm.mutex.RLock()
	processor, exists := bm.processors[processorName]
	enabled := bm.enabled
	bm.mutex.RUnlock()

	if !enabled {
		return nil, fmt.Errorf("batching manager not enabled")
	}

	if !exists {
		return nil, fmt.Errorf("processor %s not found", processorName)
	}

	return processor.ProcessPacket(key, data)
}

// GetStatistics returns batching statistics
func (bm *BatchingManager) GetStatistics() *BatchingStats {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()

	stats := &BatchingStats{
		Processors: make(map[string]*BatchMetrics),
		LastUpdate: time.Now(),
	}

	var totalBatches, totalPackets uint64
	var totalBatchSize int64
	var totalBatchDelay time.Duration

	for name, processor := range bm.processors {
		metrics := processor.GetMetrics()
		stats.Processors[name] = metrics

		totalBatches += metrics.TotalBatches
		totalPackets += metrics.TotalPackets
		totalBatchSize += int64(metrics.AvgBatchSize * float64(metrics.TotalBatches))
		totalBatchDelay += metrics.AvgBatchDelay * time.Duration(metrics.TotalBatches)
	}

	stats.TotalBatches = totalBatches
	stats.TotalPackets = totalPackets

	if totalBatches > 0 {
		stats.AvgBatchSize = float64(totalBatchSize) / float64(totalBatches)
		stats.AvgBatchDelay = totalBatchDelay / time.Duration(totalBatches)
	}

	return stats
}

// statsWorker collects statistics periodically
func (bm *BatchingManager) statsWorker() {
	ticker := time.NewTicker(bm.config.StatsInterval)
	defer ticker.Stop()

	for range ticker.C {
		bm.collectStats()
	}
}

// collectStats collects and processes statistics
func (bm *BatchingManager) collectStats() {
	stats := bm.GetStatistics()

	// Adaptive batch size adjustment
	if bm.config.AdaptiveBatching {
		bm.adjustBatchSizes(stats)
	}

	bm.logger.Debug("Batching stats",
		"total_batches", stats.TotalBatches,
		"total_packets", stats.TotalPackets,
		"avg_batch_size", stats.AvgBatchSize,
		"processors", len(bm.processors))
}

// adjustBatchSizes adjusts batch sizes based on statistics.
// Stub: requires load, latency, and batch efficiency metrics to be plumbed.
func (bm *BatchingManager) adjustBatchSizes(stats *BatchingStats) {
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(name string, handler BatchHandler, config *BatchingConfig, logger *logging.Logger) *BatchProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	return &BatchProcessor{
		name:      name,
		config:    config,
		batchChan: make(chan *Batch, config.MaxBatchSize*2),
		flushChan: make(chan struct{}, 1),
		ctx:       ctx,
		cancel:    cancel,
		metrics:   &BatchMetrics{},
		handler:   handler,
		logger:    logger,
	}
}

// Start starts the batch processor
func (bp *BatchProcessor) Start() error {
	go bp.processBatches()
	go bp.flushWorker()
	return nil
}

// Stop stops the batch processor
func (bp *BatchProcessor) Stop() {
	bp.cancel()
	close(bp.flushChan)
}

// ProcessPacket processes a single packet
func (bp *BatchProcessor) ProcessPacket(key types.FlowKey, data []byte) (*BatchResult, error) {
	packet := &PacketItem{
		Key:        key,
		Data:       data,
		Timestamp:  time.Now(),
		ResponseCh: make(chan *BatchResult, 1),
	}

	// Wrap packet in a batch
	batch := &Batch{
		Packets:   []*PacketItem{packet},
		Timestamp: time.Now(),
	}

	select {
	case bp.batchChan <- batch:
		// Wait for result
		select {
		case result := <-packet.ResponseCh:
			return result, nil
		case <-time.After(100 * time.Millisecond):
			return nil, fmt.Errorf("packet processing timeout")
		}
	default:
		// Channel full, process immediately
		batch := &Batch{
			Packets:   []*PacketItem{packet},
			Timestamp: time.Now(),
			Size:      1,
			Processor: bp.name,
		}
		result := bp.handler.ProcessBatch(batch)
		packet.ResponseCh <- result
		return result, nil
	}
}

// processBatches processes batches of packets
func (bp *BatchProcessor) processBatches() {
	var currentBatch []*PacketItem
	var batchStart time.Time
	var timer *time.Timer

	for {
		select {
		case batch := <-bp.batchChan:
			// Start new batch if empty
			if len(currentBatch) == 0 {
				currentBatch = make([]*PacketItem, 0, bp.config.MaxBatchSize)
				batchStart = time.Now()
				timer = time.AfterFunc(bp.config.MaxBatchDelay, func() {
					select {
					case bp.flushChan <- struct{}{}:
					default:
					}
				})
			}

			// Add packets from batch to current batch
			currentBatch = append(currentBatch, batch.Packets...)

			// Check if batch is full
			if len(currentBatch) >= bp.config.MaxBatchSize {
				timer.Stop()
				bp.processBatch(currentBatch, batchStart)
				currentBatch = nil
			}

		case <-bp.flushChan:
			// Flush current batch
			if len(currentBatch) > 0 {
				timer.Stop()
				bp.processBatch(currentBatch, batchStart)
				currentBatch = nil
			}

		case <-bp.ctx.Done():
			// Process remaining batch and exit
			if len(currentBatch) > 0 {
				bp.processBatch(currentBatch, batchStart)
			}
			return
		}
	}
}

// processBatch processes a batch of packets
func (bp *BatchProcessor) processBatch(packets []*PacketItem, batchStart time.Time) {
	batch := &Batch{
		Packets:   packets,
		Timestamp: batchStart,
		Size:      len(packets),
		MaxDelay:  time.Since(batchStart),
		Processor: bp.name,
	}

	// Process batch
	start := time.Now()
	result := bp.handler.ProcessBatch(batch)
	processTime := time.Since(start)

	// Send results to packet senders
	for i, packet := range packets {
		var packetResult *BatchResult
		if result.Success && i < len(result.Results) {
			packetResult = &BatchResult{
				Success:     true,
				Results:     []interface{}{result.Results[i]},
				ProcessTime: processTime,
			}
		} else {
			packetResult = &BatchResult{
				Success:     false,
				Error:       result.Error,
				ProcessTime: processTime,
			}
		}

		select {
		case packet.ResponseCh <- packetResult:
		default:
			bp.logger.Warn("Failed to send packet result")
		}
	}

	// Update metrics
	bp.updateMetrics(batch, processTime)
}

// updateMetrics updates batch processor metrics
func (bp *BatchProcessor) updateMetrics(batch *Batch, processTime time.Duration) {
	bp.metrics.mutex.Lock()
	defer bp.metrics.mutex.Unlock()

	bp.metrics.TotalBatches++
	bp.metrics.TotalPackets += uint64(batch.Size)

	// Update average batch size
	if bp.metrics.AvgBatchSize == 0 {
		bp.metrics.AvgBatchSize = float64(batch.Size)
	} else {
		alpha := 0.1
		bp.metrics.AvgBatchSize = bp.metrics.AvgBatchSize*(1-alpha) + float64(batch.Size)*alpha
	}

	// Update average batch delay
	if bp.metrics.AvgBatchDelay == 0 {
		bp.metrics.AvgBatchDelay = batch.MaxDelay
	} else {
		alpha := 0.1
		bp.metrics.AvgBatchDelay = time.Duration(
			float64(bp.metrics.AvgBatchDelay)*(1-alpha) + float64(batch.MaxDelay)*alpha,
		)
	}

	// Update min/max batch size
	if batch.Size > bp.metrics.MaxBatchSize {
		bp.metrics.MaxBatchSize = batch.Size
	}
	if bp.metrics.MinBatchSize == 0 || batch.Size < bp.metrics.MinBatchSize {
		bp.metrics.MinBatchSize = batch.Size
	}

	// Update process time
	bp.metrics.ProcessTime = processTime
	bp.metrics.LastUpdate = time.Now()
}

// flushWorker periodically flushes batches
func (bp *BatchProcessor) flushWorker() {
	ticker := time.NewTicker(bp.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			select {
			case bp.flushChan <- struct{}{}:
			default:
			}
		case <-bp.ctx.Done():
			return
		}
	}
}

// GetMetrics returns batch processor metrics
func (bp *BatchProcessor) GetMetrics() *BatchMetrics {
	bp.metrics.mutex.RLock()
	defer bp.metrics.mutex.RUnlock()

	return bp.metrics
}
