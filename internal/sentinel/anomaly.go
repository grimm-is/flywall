// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package sentinel

import (
	"math"
)

// Welford's Algorithm: https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Welford's_online_algorithm

// Tracker keeps track of mean and variance using Welford's Online Algorithm.
// This allows us to calculate standard deviation and Z-scores on the fly without storing history.
type Tracker struct {
	Count int64   `json:"count"`
	Mean  float64 `json:"mean"`
	M2    float64 `json:"m2"` // Sum of squares of differences from the current mean
}

// Update adds a new value to the tracker
func (t *Tracker) Update(newValue float64) {
	t.Count++
	delta := newValue - t.Mean
	t.Mean += delta / float64(t.Count)
	delta2 := newValue - t.Mean
	t.M2 += delta * delta2
}

// Variance returns the sample variance
func (t *Tracker) Variance() float64 {
	if t.Count < 2 {
		return 0.0
	}
	return t.M2 / float64(t.Count-1)
}

// StdDev returns the standard deviation
func (t *Tracker) StdDev() float64 {
	return math.Sqrt(t.Variance())
}

// ZScore calculates how many standard deviations the value is from the mean.
// Returns 0 if variance is 0.
func (t *Tracker) ZScore(value float64) float64 {
	stdDev := t.StdDev()
	if stdDev == 0 {
		if value == t.Mean {
			return 0.0
		}
		// If variance is 0 but value differs, it's infinitely anomalous.
		// Return a high score to ensure it triggers.
		return 100.0
	}
	return (value - t.Mean) / stdDev
}

// AnomalyThreshold is the Z-score above which a value is considered anomalous
const AnomalyThreshold = 3.0 // 3 standard deviations (99.7% confidence)

// DeviceStats holds traffic statistics and anomaly trackers for a single device
type DeviceStats struct {
	MAC string

	// Current Rate (1-second window)
	RxBytes   int64 `json:"rx_bytes"`
	RxPackets int64 `json:"rx_packets"`

	// Historic Trackers (Welford)
	BytesTracker   Tracker `json:"bytes_tracker"`
	PacketsTracker Tracker `json:"packets_tracker"`

	// Anomaly State
	LastAnomalyScore float64 `json:"last_anomaly_score"`
	IsAnomalous      bool    `json:"is_anomalous"`
}

// AnomalyStatus helper for API responses
type AnomalyStatus struct {
	Score       float64 `json:"score"`        // Max Z-score observed recently
	IsAnomalous bool    `json:"is_anomalous"` // True if score > threshold
}
