// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package stats

import (
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"grimm.is/flywall/internal/ebpf/interfaces"
)

// Collector collects statistics from eBPF programs and maps
type Collector struct {
	mu          sync.RWMutex
	collections map[string]*ebpf.Collection
	stats       *interfaces.Statistics
	lastUpdate  time.Time
}

// NewCollector creates a new statistics collector
func NewCollector() *Collector {
	return &Collector{
		collections: make(map[string]*ebpf.Collection),
		stats: &interfaces.Statistics{
			Features: make(map[string]uint64),
			Maps:     make(map[string]uint64),
			Programs: make(map[string]uint64),
		},
		lastUpdate: time.Now(),
	}
}

// RegisterCollection registers an eBPF collection for statistics collection
func (c *Collector) RegisterCollection(name string, collection *ebpf.Collection) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.collections[name] = collection
}

// UnregisterCollection removes a collection from statistics collection
func (c *Collector) UnregisterCollection(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.collections, name)
}

// Collect collects statistics from all registered collections
func (c *Collector) Collect() *interfaces.Statistics {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reset counters
	c.stats.Features = make(map[string]uint64)
	c.stats.Maps = make(map[string]uint64)
	c.stats.Programs = make(map[string]uint64)

	// Collect from each collection
	for name, collection := range c.collections {
		c.collectFromCollection(name, collection)
	}

	c.lastUpdate = time.Now()
	return c.stats
}

// collectFromCollection collects statistics from a specific collection
func (c *Collector) collectFromCollection(name string, collection *ebpf.Collection) {
	// Collect map statistics
	for mapName, m := range collection.Maps {
		info, err := m.Info()
		if err != nil {
			continue
		}

		// Get map count if possible
		var count uint64
		if m.Type() == ebpf.Array || m.Type() == ebpf.Hash {
			// For array/hash maps, try to get the number of elements
			// Note: This is a simplified approach - in production you might want
			// to track this more efficiently
			count = uint64(info.MaxEntries)
		}

		fullName := name + "." + mapName
		c.stats.Maps[fullName] = count
		c.stats.Features[fullName+"_max_entries"] = uint64(info.MaxEntries)
		c.stats.Features[fullName+"_type"] = uint64(m.Type())
	}

	// Collect program statistics
	for progName, prog := range collection.Programs {
		info, err := prog.Info()
		if err != nil {
			continue
		}

		fullName := name + "." + progName
		c.stats.Programs[fullName] = 1 // Program is loaded
		c.stats.Features[fullName+"_type"] = uint64(info.Type)
		// Note: Skip tag export as it's a string, not uint64
	}
}

// GetLastUpdate returns the time of the last statistics collection
func (c *Collector) GetLastUpdate() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUpdate
}

// UpdatePacketCounters updates the packet-related statistics
func (c *Collector) UpdatePacketCounters(processed, dropped, passed, bytes uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.PacketsProcessed = processed
	c.stats.PacketsDropped = dropped
	c.stats.PacketsPassed = passed
	c.stats.BytesProcessed = bytes
}

// ExportStats exports statistics in a format suitable for monitoring systems
func (c *Collector) ExportStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"timestamp":         c.lastUpdate.Unix(),
		"packets_processed": c.stats.PacketsProcessed,
		"packets_dropped":   c.stats.PacketsDropped,
		"packets_passed":    c.stats.PacketsPassed,
		"bytes_processed":   c.stats.BytesProcessed,
		"features":          c.stats.Features,
		"maps":              c.stats.Maps,
		"programs":          c.stats.Programs,
	}
}
