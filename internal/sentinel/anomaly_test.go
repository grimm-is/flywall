// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package sentinel

import (
	"math"
	"testing"
)

func TestWelfordTracker(t *testing.T) {
	tracker := Tracker{}

	// Dataset: 2, 4, 4, 4, 5, 5, 7, 9
	// Mean: 5
	// Variance: 4.857...
	// StdDev: 2.203...

	data := []float64{2, 4, 4, 4, 5, 5, 7, 9}

	for _, val := range data {
		tracker.Update(val)
	}

	if tracker.Count != 8 {
		t.Errorf("Expected Count 8, got %d", tracker.Count)
	}

	if tracker.Mean != 5.0 {
		t.Errorf("Expected Mean 5.0, got %f", tracker.Mean)
	}

	// Population Variance formula vs Sample Variance
	// Welford usually calculates sample variance (n-1)
	expectedVariance := 4.571428 // 32 / 7
	if math.Abs(tracker.Variance()-expectedVariance) > 0.0001 {
		t.Errorf("Expected Variance %f, got %f", expectedVariance, tracker.Variance())
	}

	expectedStdDev := math.Sqrt(expectedVariance)
	if math.Abs(tracker.StdDev()-expectedStdDev) > 0.0001 {
		t.Errorf("Expected StdDev %f, got %f", expectedStdDev, tracker.StdDev())
	}

	// Test Z-Score
	// Value 15 -> (15 - 5) / 2.267... = 4.41...
	z := tracker.ZScore(15)
	if z < 4.0 {
		t.Errorf("Expected Z-Score > 4.0 for value 15, got %f", z)
	}
}

func TestDeviceStatsAnomaly(t *testing.T) {
	s := New()

	// Simulate normal traffic
	for i := 0; i < 60; i++ {
		// 1000 bytes/sec steady
		s.IngestPacket(PacketMetadata{SrcMAC: "mac1", PayloadLen: 1000})
		s.updateTrackers() // Simulate 1 second passing
	}

	status := s.GetAnomalyStatus("mac1")
	if status.IsAnomalous {
		t.Errorf("Device should not be anomalous yet")
	}

	// Spike! 100x traffic
	s.IngestPacket(PacketMetadata{SrcMAC: "mac1", PayloadLen: 100000})
	s.updateTrackers()

	status = s.GetAnomalyStatus("mac1")
	if !status.IsAnomalous {
		t.Errorf("Device SHOULD be anomalous after spike. Score: %f", status.Score)
	}
}
