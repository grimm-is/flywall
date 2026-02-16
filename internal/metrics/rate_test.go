// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package metrics

import (
	"testing"
	"time"

	"grimm.is/flywall/internal/logging"
)

// testCollector creates a collector for testing.
func testCollector() *Collector {
	logger := logging.New(logging.DefaultConfig())
	return NewCollector(logger, time.Second)
}

func TestCalculateRate_Normal(t *testing.T) {
	c := testCollector()

	// Normal case: counter increased
	rate := c.calculateRate(1000, 500, 1.0)
	if rate != 500.0 {
		t.Errorf("Expected rate 500.0, got %f", rate)
	}
}

func TestCalculateRate_Reset(t *testing.T) {
	c := testCollector()

	// Reset case: current < previous (counter wrapped or reset)
	// Should treat current value as the delta since reset
	rate := c.calculateRate(100, 1000, 1.0)
	if rate != 100.0 {
		t.Errorf("On reset, expected rate 100.0 (current value), got %f", rate)
	}
}

func TestCalculateRate_ZeroElapsed(t *testing.T) {
	c := testCollector()

	// Zero elapsed time should return 0
	rate := c.calculateRate(1000, 500, 0.0)
	if rate != 0.0 {
		t.Errorf("Expected rate 0.0 for zero elapsed, got %f", rate)
	}
}

func TestCalculateRate_NegativeElapsed(t *testing.T) {
	c := testCollector()

	// Negative elapsed time should return 0
	rate := c.calculateRate(1000, 500, -1.0)
	if rate != 0.0 {
		t.Errorf("Expected rate 0.0 for negative elapsed, got %f", rate)
	}
}

func TestInterfaceStats_RateCalculation(t *testing.T) {
	c := testCollector()
	stats := &InterfaceStats{
		Name:        "eth0",
		prevRxBytes: 1000,
		prevTxBytes: 500,
	}

	// Simulate normal increment
	rxRate := c.calculateRate(2000, stats.prevRxBytes, 1.0)
	txRate := c.calculateRate(1000, stats.prevTxBytes, 1.0)

	if rxRate != 1000.0 {
		t.Errorf("Expected RX rate 1000.0, got %f", rxRate)
	}
	if txRate != 500.0 {
		t.Errorf("Expected TX rate 500.0, got %f", txRate)
	}
}

func TestPolicyStats_RateCalculation(t *testing.T) {
	c := testCollector()
	stats := &PolicyStats{
		Name:        "lan->wan",
		prevPackets: 100,
		prevBytes:   10000,
	}

	// Simulate reset (new node after failover)
	packetRate := c.calculateRate(50, stats.prevPackets, 1.0)
	byteRate := c.calculateRate(5000, stats.prevBytes, 1.0)

	// On reset, should use current value as delta
	if packetRate != 50.0 {
		t.Errorf("On reset, expected packet rate 50.0, got %f", packetRate)
	}
	if byteRate != 5000.0 {
		t.Errorf("On reset, expected byte rate 5000.0, got %f", byteRate)
	}
}
