// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"grimm.is/flywall/internal/logging"
)

// MemoryPool manages memory pools for high-performance packet processing
type MemoryPool struct {
	// Pool configuration
	config *MemoryPoolConfig

	// Memory pools
	packetPool    sync.Pool
	flowKeyPool   sync.Pool
	flowStatePool sync.Pool
	resultPool    sync.Pool
	batchPool     sync.Pool

	// Statistics
	stats *MemoryPoolStats

	// Logger
	logger *logging.Logger
}

// MemoryPoolConfig for memory pool configuration
type MemoryPoolConfig struct {
	Enabled           bool          `json:"enabled"`
	PacketPoolSize    int           `json:"packet_pool_size"`
	FlowKeyPoolSize   int           `json:"flow_key_pool_size"`
	FlowStatePoolSize int           `json:"flow_state_pool_size"`
	ResultPoolSize    int           `json:"result_pool_size"`
	BatchPoolSize     int           `json:"batch_pool_size"`
	MaxPacketSize     int           `json:"max_packet_size"`
	PreAllocate       bool          `json:"pre_allocate"`
	GCInterval        time.Duration `json:"gc_interval"`
	StatsInterval     time.Duration `json:"stats_interval"`
}

// DefaultMemoryPoolConfig returns default memory pool configuration
func DefaultMemoryPoolConfig() *MemoryPoolConfig {
	return &MemoryPoolConfig{
		Enabled:           true,
		PacketPoolSize:    1000,
		FlowKeyPoolSize:   1000,
		FlowStatePoolSize: 1000,
		ResultPoolSize:    1000,
		BatchPoolSize:     100,
		MaxPacketSize:     4096,
		PreAllocate:       true,
		GCInterval:        30 * time.Second,
		StatsInterval:     10 * time.Second,
	}
}

// MemoryPoolStats tracks memory pool statistics
type MemoryPoolStats struct {
	PacketPoolHits      uint64    `json:"packet_pool_hits"`
	PacketPoolMisses    uint64    `json:"packet_pool_misses"`
	FlowKeyPoolHits     uint64    `json:"flow_key_pool_hits"`
	FlowKeyPoolMisses   uint64    `json:"flow_key_pool_misses"`
	FlowStatePoolHits   uint64    `json:"flow_state_pool_hits"`
	FlowStatePoolMisses uint64    `json:"flow_state_pool_misses"`
	ResultPoolHits      uint64    `json:"result_pool_hits"`
	ResultPoolMisses    uint64    `json:"result_pool_misses"`
	BatchPoolHits       uint64    `json:"batch_pool_hits"`
	BatchPoolMisses     uint64    `json:"batch_pool_misses"`
	MemoryAllocated     uint64    `json:"memory_allocated"`
	MemoryReleased      uint64    `json:"memory_released"`
	LastUpdate          time.Time `json:"last_update"`
}

// NewMemoryPool creates a new memory pool
func NewMemoryPool(logger *logging.Logger, config *MemoryPoolConfig) *MemoryPool {
	if config == nil {
		config = DefaultMemoryPoolConfig()
	}

	pool := &MemoryPool{
		config: config,
		stats:  &MemoryPoolStats{},
		logger: logger,
	}

	// Initialize pools
	pool.initializePools()

	// Pre-allocate memory if enabled
	if config.PreAllocate {
		pool.preAllocate()
	}

	// Start background tasks
	go pool.gcWorker()
	go pool.statsWorker()

	return pool
}

// initializePools initializes all memory pools
func (mp *MemoryPool) initializePools() {
	// Packet buffer pool
	mp.packetPool = sync.Pool{
		New: func() interface{} {
			atomic.AddUint64(&mp.stats.PacketPoolMisses, 1)
			atomic.AddUint64(&mp.stats.MemoryAllocated, uint64(mp.config.MaxPacketSize))
			return make([]byte, mp.config.MaxPacketSize)
		},
	}

	// Flow key pool
	mp.flowKeyPool = sync.Pool{
		New: func() interface{} {
			atomic.AddUint64(&mp.stats.FlowKeyPoolMisses, 1)
			return &FlowKeyWrapper{}
		},
	}

	// Flow state pool
	mp.flowStatePool = sync.Pool{
		New: func() interface{} {
			atomic.AddUint64(&mp.stats.FlowStatePoolMisses, 1)
			return &FlowStateWrapper{}
		},
	}

	// Result pool
	mp.resultPool = sync.Pool{
		New: func() interface{} {
			atomic.AddUint64(&mp.stats.ResultPoolMisses, 1)
			return &PacketResultWrapper{}
		},
	}

	// Batch pool
	mp.batchPool = sync.Pool{
		New: func() interface{} {
			atomic.AddUint64(&mp.stats.BatchPoolMisses, 1)
			return make([]*PacketTask, 0, mp.config.BatchPoolSize)
		},
	}
}

// preAllocate pre-allocates memory for pools
func (mp *MemoryPool) preAllocate() {
	mp.logger.Info("Pre-allocating memory pools")

	// Pre-allocate packet buffers
	for i := 0; i < mp.config.PacketPoolSize; i++ {
		buf := make([]byte, mp.config.MaxPacketSize)
		mp.packetPool.Put(buf)
	}

	// Pre-allocate flow keys
	for i := 0; i < mp.config.FlowKeyPoolSize; i++ {
		key := &FlowKeyWrapper{}
		mp.flowKeyPool.Put(key)
	}

	// Pre-allocate flow states
	for i := 0; i < mp.config.FlowStatePoolSize; i++ {
		state := &FlowStateWrapper{}
		mp.flowStatePool.Put(state)
	}

	// Pre-allocate results
	for i := 0; i < mp.config.ResultPoolSize; i++ {
		result := &PacketResultWrapper{}
		mp.resultPool.Put(result)
	}

	// Pre-allocate batches
	for i := 0; i < mp.config.BatchPoolSize; i++ {
		batch := make([]*PacketTask, 0, mp.config.BatchPoolSize)
		mp.batchPool.Put(batch)
	}
}

// GetPacketBuffer gets a packet buffer from the pool
func (mp *MemoryPool) GetPacketBuffer() []byte {
	if !mp.config.Enabled {
		return make([]byte, mp.config.MaxPacketSize)
	}

	buf := mp.packetPool.Get().([]byte)
	atomic.AddUint64(&mp.stats.PacketPoolHits, 1)
	return buf[:0] // Reset length but keep capacity
}

// PutPacketBuffer returns a packet buffer to the pool
func (mp *MemoryPool) PutPacketBuffer(buf []byte) {
	if !mp.config.Enabled {
		return
	}

	// Check if buffer is from our pool
	if cap(buf) == mp.config.MaxPacketSize {
		mp.packetPool.Put(buf)
	}
}

// GetFlowKey gets a flow key from the pool
func (mp *MemoryPool) GetFlowKey() *FlowKeyWrapper {
	if !mp.config.Enabled {
		return &FlowKeyWrapper{}
	}

	key := mp.flowKeyPool.Get().(*FlowKeyWrapper)
	atomic.AddUint64(&mp.stats.FlowKeyPoolHits, 1)
	key.Reset()
	return key
}

// PutFlowKey returns a flow key to the pool
func (mp *MemoryPool) PutFlowKey(key *FlowKeyWrapper) {
	if !mp.config.Enabled {
		return
	}

	mp.flowKeyPool.Put(key)
}

// GetFlowState gets a flow state from the pool
func (mp *MemoryPool) GetFlowState() *FlowStateWrapper {
	if !mp.config.Enabled {
		return &FlowStateWrapper{}
	}

	state := mp.flowStatePool.Get().(*FlowStateWrapper)
	atomic.AddUint64(&mp.stats.FlowStatePoolHits, 1)
	state.Reset()
	return state
}

// PutFlowState returns a flow state to the pool
func (mp *MemoryPool) PutFlowState(state *FlowStateWrapper) {
	if !mp.config.Enabled {
		return
	}

	mp.flowStatePool.Put(state)
}

// GetPacketResult gets a packet result from the pool
func (mp *MemoryPool) GetPacketResult() *PacketResultWrapper {
	if !mp.config.Enabled {
		return &PacketResultWrapper{}
	}

	result := mp.resultPool.Get().(*PacketResultWrapper)
	atomic.AddUint64(&mp.stats.ResultPoolHits, 1)
	result.Reset()
	return result
}

// PutPacketResult returns a packet result to the pool
func (mp *MemoryPool) PutPacketResult(result *PacketResultWrapper) {
	if !mp.config.Enabled {
		return
	}

	mp.resultPool.Put(result)
}

// GetBatch gets a batch from the pool
func (mp *MemoryPool) GetBatch() []*PacketTask {
	if !mp.config.Enabled {
		return make([]*PacketTask, 0, mp.config.BatchPoolSize)
	}

	batch := mp.batchPool.Get().([]*PacketTask)
	atomic.AddUint64(&mp.stats.BatchPoolHits, 1)
	return batch[:0] // Reset length but keep capacity
}

// PutBatch returns a batch to the pool
func (mp *MemoryPool) PutBatch(batch []*PacketTask) {
	if !mp.config.Enabled {
		return
	}

	// Check if batch is from our pool
	if cap(batch) == mp.config.BatchPoolSize {
		mp.batchPool.Put(batch)
	}
}

// gcWorker performs periodic garbage collection
func (mp *MemoryPool) gcWorker() {
	ticker := time.NewTicker(mp.config.GCInterval)
	defer ticker.Stop()

	for range ticker.C {
		// Force garbage collection
		// Note: This is aggressive and may impact performance
		// In production, tune the interval based on metrics
		runtime.GC()

		mp.logger.Debug("Memory pool GC completed")
	}
}

// statsWorker collects memory pool statistics
func (mp *MemoryPool) statsWorker() {
	ticker := time.NewTicker(mp.config.StatsInterval)
	defer ticker.Stop()

	for range ticker.C {
		mp.collectStats()
	}
}

// collectStats collects memory pool statistics
func (mp *MemoryPool) collectStats() {
	mp.stats.LastUpdate = time.Now()

	// Calculate hit rates
	totalPacketHits := atomic.LoadUint64(&mp.stats.PacketPoolHits)
	totalPacketMisses := atomic.LoadUint64(&mp.stats.PacketPoolMisses)

	if totalPacketHits+totalPacketMisses > 0 {
		hitRate := float64(totalPacketHits) / float64(totalPacketHits+totalPacketMisses)
		if hitRate < 0.9 {
			mp.logger.Warn("Low packet pool hit rate", "hit_rate", hitRate)
		}
	}

	// Log statistics periodically
	mp.logger.Debug("Memory pool stats",
		"packet_hits", totalPacketHits,
		"packet_misses", totalPacketMisses,
		"memory_allocated", atomic.LoadUint64(&mp.stats.MemoryAllocated),
		"memory_released", atomic.LoadUint64(&mp.stats.MemoryReleased))
}

// GetStatistics returns memory pool statistics
func (mp *MemoryPool) GetStatistics() *MemoryPoolStats {
	return &MemoryPoolStats{
		PacketPoolHits:      atomic.LoadUint64(&mp.stats.PacketPoolHits),
		PacketPoolMisses:    atomic.LoadUint64(&mp.stats.PacketPoolMisses),
		FlowKeyPoolHits:     atomic.LoadUint64(&mp.stats.FlowKeyPoolHits),
		FlowKeyPoolMisses:   atomic.LoadUint64(&mp.stats.FlowKeyPoolMisses),
		FlowStatePoolHits:   atomic.LoadUint64(&mp.stats.FlowStatePoolHits),
		FlowStatePoolMisses: atomic.LoadUint64(&mp.stats.FlowStatePoolMisses),
		ResultPoolHits:      atomic.LoadUint64(&mp.stats.ResultPoolHits),
		ResultPoolMisses:    atomic.LoadUint64(&mp.stats.ResultPoolMisses),
		BatchPoolHits:       atomic.LoadUint64(&mp.stats.BatchPoolHits),
		BatchPoolMisses:     atomic.LoadUint64(&mp.stats.BatchPoolMisses),
		MemoryAllocated:     atomic.LoadUint64(&mp.stats.MemoryAllocated),
		MemoryReleased:      atomic.LoadUint64(&mp.stats.MemoryReleased),
		LastUpdate:          mp.stats.LastUpdate,
	}
}

// SetBufferCount updates the packet buffer pool size
func (mp *MemoryPool) SetBufferCount(count int) {
	mp.config.PacketPoolSize = count
	// Existing pool remains, but configuration is updated for reference
}

// Wrapper types for memory pooling
type FlowKeyWrapper struct {
	// Embed the actual type
	// This is a placeholder - would embed the actual FlowKey
	data [64]byte // Enough space for FlowKey
}

func (fk *FlowKeyWrapper) Reset() {
	// Reset all fields to zero
	for i := range fk.data {
		fk.data[i] = 0
	}
}

type FlowStateWrapper struct {
	// Embed the actual type
	// This is a placeholder - would embed the actual FlowState
	data [128]byte // Enough space for FlowState
}

func (fs *FlowStateWrapper) Reset() {
	// Reset all fields to zero
	for i := range fs.data {
		fs.data[i] = 0
	}
}

type PacketResultWrapper struct {
	// Embed the actual type
	// This is a placeholder - would embed the actual PacketResult
	data [64]byte // Enough space for PacketResult
}

func (pr *PacketResultWrapper) Reset() {
	// Reset all fields to zero
	for i := range pr.data {
		pr.data[i] = 0
	}
}

// Unsafe memory operations for maximum performance
// WARNING: Use with extreme caution
type UnsafeBuffer struct {
	ptr unsafe.Pointer
	len int
	cap int
}

// NewUnsafeBuffer creates an unsafe buffer
func NewUnsafeBuffer(size int) *UnsafeBuffer {
	buf := make([]byte, size)
	return &UnsafeBuffer{
		ptr: unsafe.Pointer(&buf[0]),
		len: 0,
		cap: size,
	}
}

// Bytes returns the byte slice
func (ub *UnsafeBuffer) Bytes() []byte {
	return (*[1 << 30]byte)(ub.ptr)[:ub.len:ub.cap]
}

// SetLength sets the buffer length
func (ub *UnsafeBuffer) SetLength(len int) {
	if len <= ub.cap {
		ub.len = len
	}
}

// Memory alignment utilities
func AlignUp(size, alignment int) int {
	return (size + alignment - 1) & ^(alignment - 1)
}

func IsAligned(ptr unsafe.Pointer, alignment int) bool {
	return uintptr(ptr)%uintptr(alignment) == 0
}
