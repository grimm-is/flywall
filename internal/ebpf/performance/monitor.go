// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"grimm.is/flywall/internal/logging"
)

// LatencyStats contains latency measurements
type LatencyStats struct {
	Min  time.Duration `json:"min"`
	Max  time.Duration `json:"max"`
	Mean time.Duration `json:"mean"`
	P50  time.Duration `json:"p50"`
	P95  time.Duration `json:"p95"`
	P99  time.Duration `json:"p99"`
	P999 time.Duration `json:"p99_9"`
}

// PerformanceMonitor continuously monitors eBPF performance metrics
type PerformanceMonitor struct {
	logger          *logging.Logger
	metrics         atomic.Value // Stores *PerformanceMetrics
	alerts          chan Alert
	thresholds      Thresholds
	collectInterval time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
}

// PerformanceMetrics contains current performance metrics
type PerformanceMetrics struct {
	Timestamp        time.Time         `json:"timestamp"`
	PacketsPerSecond float64           `json:"packets_per_second"`
	ThroughputMbps   float64           `json:"throughput_mbps"`
	CPUUsage         float64           `json:"cpu_usage"`
	MemoryUsageMB    float64           `json:"memory_usage_mb"`
	LatencyStats     LatencyStats      `json:"latency_stats"`
	ErrorRate        float64           `json:"error_rate"`
	DropRate         float64           `json:"drop_rate"`
	ProgramMetrics   map[string]uint64 `json:"program_metrics"`
	SystemLoad       SystemLoad        `json:"system_load"`
}

// SystemLoad contains system-wide load information
type SystemLoad struct {
	LoadAvg1min  float64 `json:"load_avg_1min"`
	LoadAvg5min  float64 `json:"load_avg_5min"`
	LoadAvg15min float64 `json:"load_avg_15min"`
	ProcRunning  int     `json:"proc_running"`
	ProcTotal    int     `json:"proc_total"`
}

// Alert represents a performance alert
type Alert struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"` // warning, critical
	Metric      string    `json:"metric"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	Description string    `json:"description"`
}

// Thresholds defines alert thresholds
type Thresholds struct {
	MaxCPUUsage       float64 `json:"max_cpu_usage"`
	MaxMemoryMB       float64 `json:"max_memory_mb"`
	MaxLatencyNs      int64   `json:"max_latency_ns"`
	MinThroughputMbps float64 `json:"min_throughput_mbps"`
	MaxDropRate       float64 `json:"max_drop_rate"`
	MaxErrorRate      float64 `json:"max_error_rate"`
}

// DefaultThresholds returns default monitoring thresholds
func DefaultThresholds() Thresholds {
	return Thresholds{
		MaxCPUUsage:       80.0,
		MaxMemoryMB:       500,
		MaxLatencyNs:      5000,
		MinThroughputMbps: 100,
		MaxDropRate:       5.0,
		MaxErrorRate:      1.0,
	}
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(logger *logging.Logger, thresholds Thresholds) *PerformanceMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	pm := &PerformanceMonitor{
		logger:          logger,
		alerts:          make(chan Alert, 100),
		thresholds:      thresholds,
		collectInterval: 5 * time.Second,
		ctx:             ctx,
		cancel:          cancel,
	}

	// Initialize metrics
	pm.metrics.Store(&PerformanceMetrics{
		ProgramMetrics: make(map[string]uint64),
	})

	return pm
}

// Start begins performance monitoring
func (pm *PerformanceMonitor) Start() {
	go pm.collectLoop()
	go pm.alertLoop()

	pm.logger.Info("Performance monitor started",
		"interval", pm.collectInterval,
		"thresholds", pm.thresholds,
	)
}

// Stop stops performance monitoring
func (pm *PerformanceMonitor) Stop() {
	pm.cancel()
	close(pm.alerts)
	pm.logger.Info("Performance monitor stopped")
}

// GetMetrics returns current performance metrics
func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	return pm.metrics.Load().(*PerformanceMetrics)
}

// GetAlerts returns a channel for receiving alerts
func (pm *PerformanceMonitor) GetAlerts() <-chan Alert {
	return pm.alerts
}

// collectLoop continuously collects metrics
func (pm *PerformanceMonitor) collectLoop() {
	ticker := time.NewTicker(pm.collectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			metrics := pm.collectMetrics()
			pm.metrics.Store(metrics)
			pm.checkThresholds(metrics)
		}
	}
}

// collectMetrics gathers current performance metrics
func (pm *PerformanceMonitor) collectMetrics() *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		Timestamp:      time.Now(),
		ProgramMetrics: make(map[string]uint64),
		SystemLoad:     pm.collectSystemLoad(),
	}

	// Collect memory statistics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	metrics.MemoryUsageMB = float64(m.Alloc) / 1024 / 1024

	// Collect CPU usage (simplified)
	// In production, use /proc/stat for accurate CPU metrics
	metrics.CPUUsage = float64(m.GCCPUFraction) * 100

	// eBPF program metrics require the eBPF manager to be wired in via UpdateProgramMetrics().

	return metrics
}

// collectSystemLoad gathers system load information
func (pm *PerformanceMonitor) collectSystemLoad() SystemLoad {
	load := SystemLoad{}

	// Read load average
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fmt.Sscanf(string(data), "%f %f %f %d/%d",
			&load.LoadAvg1min,
			&load.LoadAvg5min,
			&load.LoadAvg15min,
			&load.ProcRunning,
			&load.ProcTotal,
		)
	}

	return load
}

// checkThresholds checks if any metrics exceed thresholds
func (pm *PerformanceMonitor) checkThresholds(metrics *PerformanceMetrics) {
	// Check CPU usage
	if metrics.CPUUsage > pm.thresholds.MaxCPUUsage {
		pm.sendAlert(Alert{
			Timestamp: time.Now(),
			Level:     "warning",
			Metric:    "cpu_usage",
			Value:     metrics.CPUUsage,
			Threshold: pm.thresholds.MaxCPUUsage,
			Description: fmt.Sprintf("CPU usage (%.2f%%) exceeds threshold (%.2f%%)",
				metrics.CPUUsage, pm.thresholds.MaxCPUUsage),
		})
	}

	// Check memory usage
	if metrics.MemoryUsageMB > pm.thresholds.MaxMemoryMB {
		pm.sendAlert(Alert{
			Timestamp: time.Now(),
			Level:     "warning",
			Metric:    "memory_usage",
			Value:     metrics.MemoryUsageMB,
			Threshold: pm.thresholds.MaxMemoryMB,
			Description: fmt.Sprintf("Memory usage (%.2f MB) exceeds threshold (%.2f MB)",
				metrics.MemoryUsageMB, pm.thresholds.MaxMemoryMB),
		})
	}

	// Check throughput
	if metrics.ThroughputMbps > 0 && metrics.ThroughputMbps < pm.thresholds.MinThroughputMbps {
		pm.sendAlert(Alert{
			Timestamp: time.Now(),
			Level:     "critical",
			Metric:    "throughput",
			Value:     metrics.ThroughputMbps,
			Threshold: pm.thresholds.MinThroughputMbps,
			Description: fmt.Sprintf("Throughput (%.2f Mbps) below minimum (%.2f Mbps)",
				metrics.ThroughputMbps, pm.thresholds.MinThroughputMbps),
		})
	}
}

// sendAlert sends an alert
func (pm *PerformanceMonitor) sendAlert(alert Alert) {
	select {
	case pm.alerts <- alert:
		pm.logger.Warn("Performance alert",
			"metric", alert.Metric,
			"value", alert.Value,
			"threshold", alert.Threshold,
			"description", alert.Description,
		)
	default:
		pm.logger.Error("Alert channel full, dropping alert")
	}
}

// alertLoop processes alerts
func (pm *PerformanceMonitor) alertLoop() {
	for alert := range pm.alerts {
		// In a real implementation, you might:
		// - Send to monitoring system
		// - Write to log file
		// - Send email/SMS notification
		_ = alert
	}
}

// ExportMetrics exports metrics in JSON format
func (pm *PerformanceMonitor) ExportMetrics() ([]byte, error) {
	metrics := pm.GetMetrics()
	return json.MarshalIndent(metrics, "", "  ")
}

// SetCollectInterval updates the metrics collection interval
func (pm *PerformanceMonitor) SetCollectInterval(interval time.Duration) {
	pm.collectInterval = interval
	pm.logger.Info("Updated collection interval", "interval", interval)
}

// UpdateProgramMetrics updates metrics for a specific program
func (pm *PerformanceMonitor) UpdateProgramMetrics(program string, metrics map[string]uint64) {
	current := pm.GetMetrics()

	// Create a copy to avoid race conditions
	newMetrics := &PerformanceMetrics{
		Timestamp:        time.Now(),
		PacketsPerSecond: current.PacketsPerSecond,
		ThroughputMbps:   current.ThroughputMbps,
		CPUUsage:         current.CPUUsage,
		MemoryUsageMB:    current.MemoryUsageMB,
		LatencyStats:     current.LatencyStats,
		ErrorRate:        current.ErrorRate,
		DropRate:         current.DropRate,
		ProgramMetrics:   make(map[string]uint64),
		SystemLoad:       current.SystemLoad,
	}

	// Copy existing metrics
	for k, v := range current.ProgramMetrics {
		newMetrics.ProgramMetrics[k] = v
	}

	// Update with new metrics
	for k, v := range metrics {
		newMetrics.ProgramMetrics[program+"."+k] = v
	}

	pm.metrics.Store(newMetrics)
}
