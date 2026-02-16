// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package stats

import (
	"os"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
)

// TestCollector tests the statistics collector
func TestCollector(t *testing.T) {
	// Skip if not running as root (eBPF requires root)
	if os.Getuid() != 0 {
		t.Skip("Skipping eBPF test - requires root privileges")
	}
	collector := NewCollector()

	// Create a mock collection
	spec := &ebpf.CollectionSpec{
		Programs: map[string]*ebpf.ProgramSpec{
			"test_prog": {
				Instructions: asm.Instructions{
					asm.LoadImm(asm.R0, 0, asm.DWord),
					asm.Return(),
				},
				License: "MIT",
				Type:    ebpf.SocketFilter,
			},
		},
		Maps: map[string]*ebpf.MapSpec{
			"test_map": {
				Type:       ebpf.Array,
				KeySize:    4,
				ValueSize:  4,
				MaxEntries: 10,
			},
		},
	}

	// Load the collection
	collection, err := ebpf.NewCollection(spec)
	if err != nil {
		t.Fatalf("Failed to load collection: %v", err)
	}
	defer collection.Close()

	// Register collection
	collector.RegisterCollection("test", collection)

	// Collect statistics
	stats := collector.Collect()

	// Verify statistics
	if stats.Programs == nil {
		t.Error("Programs map is nil")
	}

	if stats.Maps == nil {
		t.Error("Maps map is nil")
	}

	// Check program is registered
	if _, exists := stats.Programs["test.test_prog"]; !exists {
		t.Error("Test program not found in statistics")
	}

	// Check map is registered
	if _, exists := stats.Maps["test.test_map"]; !exists {
		t.Error("Test map not found in statistics")
	}

	// Test packet counter updates
	collector.UpdatePacketCounters(100, 10, 90, 1000)
	stats = collector.Collect()

	if stats.PacketsProcessed != 100 {
		t.Errorf("Expected 100 processed packets, got %d", stats.PacketsProcessed)
	}

	if stats.PacketsDropped != 10 {
		t.Errorf("Expected 10 dropped packets, got %d", stats.PacketsDropped)
	}

	if stats.PacketsPassed != 90 {
		t.Errorf("Expected 90 passed packets, got %d", stats.PacketsPassed)
	}

	if stats.BytesProcessed != 1000 {
		t.Errorf("Expected 1000 bytes processed, got %d", stats.BytesProcessed)
	}

	// Test unregister
	collector.UnregisterCollection("test")
	stats = collector.Collect()

	if _, exists := stats.Programs["test.test_prog"]; exists {
		t.Error("Test program still found after unregister")
	}
}

// TestExportStats tests the statistics export
func TestExportStats(t *testing.T) {
	collector := NewCollector()

	// Update some counters
	collector.UpdatePacketCounters(1000, 100, 900, 10000)

	// Export stats
	exported := collector.ExportStats()

	// Verify exported structure
	if timestamp, ok := exported["timestamp"].(int64); !ok {
		t.Error("Timestamp not found or not int64")
	} else if timestamp <= 0 {
		t.Error("Invalid timestamp")
	}

	if exported["packets_processed"] != uint64(1000) {
		t.Errorf("Expected 1000 processed packets in export, got %v", exported["packets_processed"])
	}

	if exported["packets_dropped"] != uint64(100) {
		t.Errorf("Expected 100 dropped packets in export, got %v", exported["packets_dropped"])
	}

	if exported["packets_passed"] != uint64(900) {
		t.Errorf("Expected 900 passed packets in export, got %v", exported["packets_passed"])
	}

	if exported["bytes_processed"] != uint64(10000) {
		t.Errorf("Expected 10000 bytes processed in export, got %v", exported["bytes_processed"])
	}
}

// TestCollectorConcurrency tests concurrent access to the collector
func TestCollectorConcurrency(t *testing.T) {
	collector := NewCollector()

	// Run concurrent operations
	done := make(chan bool, 2)

	// Goroutine 1: Update counters
	go func() {
		for i := 0; i < 100; i++ {
			collector.UpdatePacketCounters(uint64(i), uint64(i/10), uint64(i-i/10), uint64(i*100))
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Goroutine 2: Collect statistics
	go func() {
		for i := 0; i < 100; i++ {
			stats := collector.Collect()
			if stats == nil {
				t.Error("Statistics collection returned nil")
			}
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Final verification
	stats := collector.Collect()
	if stats.PacketsProcessed != 99 {
		t.Errorf("Expected final processed count to be 99, got %d", stats.PacketsProcessed)
	}
}
