// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package sentinel

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/network"
)

// DeviceClass represents the classification of a device
type DeviceClass struct {
	Category   string  `json:"category"`   // e.g., "mobile", "desktop", "iot", "console", "network"
	Confidence float64 `json:"confidence"` // 0.0 to 1.0
	Icon       string  `json:"icon"`       // Material icon name
	Vendor     string  `json:"vendor"`     // Detected vendor
	Detail     string  `json:"detail"`     // Specific model if detected
}

// AnomalyCallback is a function that is called when an anomaly is detected.
type AnomalyCallback func(mac string, score float64)

type Service struct {
	mu               sync.RWMutex
	deviceStats      map[string]*DeviceStats
	classifier       *Classifier
	logger           *logging.Logger
	ctx              context.Context
	cancel           context.CancelFunc
	anomalyCallbacks []AnomalyCallback
}

// New creates a new Sentinel service
// New creates a new Sentinel service
func New() *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		deviceStats: make(map[string]*DeviceStats),
		classifier:  NewClassifier(),
		logger:      logging.WithComponent("sentinel"),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Analyze classifies a device based on MAC address and hostname
func (s *Service) Analyze(mac, hostname string) DeviceClass {
	result := DeviceClass{
		Category:   "unknown",
		Confidence: 0.0,
		Icon:       "help_outline",
		Vendor:     network.LookupVendor(mac),
	}

	// Normalize inputs
	hostname = strings.ToLower(hostname)
	vendor := strings.ToLower(result.Vendor)

	// --- 1. Vendor-based Heuristics ---

	if strings.Contains(vendor, "apple") {
		result.Category = "mobile" // Assumption, refined by hostname later
		result.Icon = "smartphone"
		result.Confidence = 0.6
	} else if strings.Contains(vendor, "google") {
		result.Category = "mobile"
		result.Icon = "smartphone"
		result.Confidence = 0.5
	} else if strings.Contains(vendor, "nintendo") {
		result.Category = "console"
		result.Icon = "videogame_asset"
		result.Confidence = 0.9
	} else if strings.Contains(vendor, "sony") {
		result.Category = "console"
		result.Icon = "videogame_asset"
		result.Confidence = 0.7 // Could be TV or Phone too
	} else if strings.Contains(vendor, "microsoft") {
		result.Category = "console" // Xbox
		result.Icon = "videogame_asset"
		result.Confidence = 0.6 // Could be Surface
	} else if strings.Contains(vendor, "synology") || strings.Contains(vendor, "qnap") {
		result.Category = "nas"
		result.Icon = "dns"
		result.Confidence = 0.9
	} else if strings.Contains(vendor, "ubiquiti") || strings.Contains(vendor, "cisco") {
		result.Category = "network"
		result.Icon = "router"
		result.Confidence = 0.9
	} else if strings.Contains(vendor, "raspberry") {
		result.Category = "iot"
		result.Icon = "developer_board"
		result.Confidence = 0.9
	} else if strings.Contains(vendor, "espressif") {
		result.Category = "iot"
		result.Icon = "lightbulb" // Common for smart bulbs
		result.Confidence = 0.9
	} else if strings.Contains(vendor, "nest") {
		result.Category = "iot"
		result.Icon = "thermostat"
		result.Confidence = 0.9
	} else if strings.Contains(vendor, "philips") {
		result.Category = "iot"
		result.Icon = "lightbulb" // Hue
		result.Confidence = 0.7
	} else if strings.Contains(vendor, "sonos") {
		result.Category = "media"
		result.Icon = "speaker"
		result.Confidence = 0.95
	} else if strings.Contains(vendor, "roku") {
		result.Category = "media"
		result.Icon = "tv"
		result.Confidence = 0.95
	} else if strings.Contains(vendor, "samsung") || strings.Contains(vendor, "lg electronics") || strings.Contains(vendor, "vizio") {
		result.Category = "tv"
		result.Icon = "tv"
		result.Confidence = 0.7
	} else if strings.Contains(vendor, "brother") || strings.Contains(vendor, "canon") || strings.Contains(vendor, "epson") || strings.Contains(vendor, "hp") {
		result.Category = "printer"
		result.Icon = "print"
		result.Confidence = 0.8
	}

	// --- 2. Hostname-based Refinements ---

	if hostname != "" {
		if strings.Contains(hostname, "iphone") {
			result.Category = "mobile"
			result.Icon = "smartphone"
			result.Detail = "iPhone"
			result.Confidence = 0.95
		} else if strings.Contains(hostname, "ipad") {
			result.Category = "tablet"
			result.Icon = "tablet_mac"
			result.Detail = "iPad"
			result.Confidence = 0.95
		} else if strings.Contains(hostname, "macbook") {
			result.Category = "laptop"
			result.Icon = "laptop_mac"
			result.Detail = "MacBook"
			result.Confidence = 0.95
		} else if strings.Contains(hostname, "watch") {
			result.Category = "wearable"
			result.Icon = "watch"
			result.Detail = "Apple Watch"
			result.Confidence = 0.9
		} else if strings.Contains(hostname, "tv") || strings.Contains(hostname, "bravia") {
			result.Category = "tv"
			result.Icon = "tv"
			result.Confidence = 0.8
		} else if strings.Contains(hostname, "xbox") {
			result.Category = "console"
			result.Icon = "videogame_asset"
			result.Detail = "Xbox"
			result.Confidence = 0.95
		} else if strings.Contains(hostname, "playstation") || strings.Contains(hostname, "ps4") || strings.Contains(hostname, "ps5") {
			result.Category = "console"
			result.Icon = "videogame_asset"
			result.Detail = "PlayStation"
			result.Confidence = 0.95
		} else if strings.Contains(hostname, "switch") && strings.Contains(vendor, "nintendo") {
			result.Category = "console"
			result.Icon = "videogame_asset"
			result.Detail = "Nintendo Switch"
			result.Confidence = 0.95
		} else if strings.Contains(hostname, "printer") {
			result.Category = "printer"
			result.Icon = "print"
			result.Confidence = 0.8
		} else if strings.Contains(hostname, "desktop") || strings.Contains(hostname, "pc") || strings.Contains(hostname, "win") {
			result.Category = "desktop"
			result.Icon = "desktop_windows"
			result.Confidence = 0.6
		}
	}

	// --- 3. Defaults ---
	if result.Category == "unknown" {
		if result.Vendor != "" {
			// Vendor known but category unknown - default to generic device
			result.Icon = "devices_other"
		} else {
			// Nothing known
			result.Icon = "help_outline"
		}
	}

	return result
}

// OnAnomaly registers a callback for when an anomaly is detected.
func (s *Service) OnAnomaly(cb AnomalyCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.anomalyCallbacks = append(s.anomalyCallbacks, cb)
}

// IngestPacket processes a packet, updates anomaly trackers, and returns traffic classification
func (s *Service) IngestPacket(pkt PacketMetadata) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Update Anomaly Stats
	stat, exists := s.deviceStats[pkt.SrcMAC]
	if !exists {
		stat = &DeviceStats{MAC: pkt.SrcMAC}
		s.deviceStats[pkt.SrcMAC] = stat
	}

	stat.RxBytes += int64(pkt.PayloadLen)
	stat.RxPackets++

	// 2. Classify Traffic
	return s.classifier.Classify(pkt)
}

// GetAnomalyStatus returns the current anomaly state for a device (Thread-safe)
func (s *Service) GetAnomalyStatus(mac string) AnomalyStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if stat, exists := s.deviceStats[mac]; exists {
		return AnomalyStatus{
			Score:       stat.LastAnomalyScore,
			IsAnomalous: stat.IsAnomalous,
		}
	}
	return AnomalyStatus{}
}

// Classify returns the traffic class for a packet without updating stats
func (s *Service) Classify(pkt PacketMetadata) string {
	return s.classifier.Classify(pkt)
}

// Start begins the background analysis loop
func (s *Service) Start() {
	s.logger.Info("Starting Sentinel Anomaly Detection")
	go s.analysisLoop()
}

// Stop stops the background analysis loop
func (s *Service) Stop() {
	if s.cancel != nil {
		s.cancel()
		s.logger.Info("Stopping Sentinel Anomaly Detection")
	}
}

func (s *Service) analysisLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.updateTrackers()
		}
	}
}

// updateTrackers runs once per second to update Welford trackers
func (s *Service) updateTrackers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, stat := range s.deviceStats {
		// Calculate rates (since window is 1 second, rate = count)
		bytesRate := float64(stat.RxBytes)
		packetsRate := float64(stat.RxPackets)

		// 1. Calculate Z-Scores against *previous* baseline
		zBytes := stat.BytesTracker.ZScore(bytesRate)
		zPackets := stat.PacketsTracker.ZScore(packetsRate)

		// 2. Update Anomaly State
		maxZ := math.Max(math.Abs(zBytes), math.Abs(zPackets))
		stat.LastAnomalyScore = maxZ
		stat.IsAnomalous = maxZ > AnomalyThreshold

		// 3. Update Trackers with new data
		// Only update if not anomalous? Or always update?
		// Welford usually updates always, but for anomaly detection sometimes you skip pollution.
		// For simplicity V1, we always update, so it adapts to new normals.
		stat.BytesTracker.Update(bytesRate)
		stat.PacketsTracker.Update(packetsRate)

		// 4. Reset current counters
		stat.RxBytes = 0
		stat.RxPackets = 0

		// 5. Trigger callbacks if anomalous
		if stat.IsAnomalous {
			for _, cb := range s.anomalyCallbacks {
				// Run in a goroutine to avoid blocking the analysis loop?
				// For now, let's just call it, but maybe safer to go.
				cb(stat.MAC, stat.LastAnomalyScore)
			}
		}
	}
}
