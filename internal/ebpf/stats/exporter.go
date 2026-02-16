// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Exporter exports eBPF statistics to various monitoring systems
type Exporter struct {
	collector *Collector
	config    ExportConfig

	// Prometheus metrics
	promPacketsProcessed *prometheus.CounterVec
	promPacketsDropped   *prometheus.CounterVec
	promPacketsPassed    *prometheus.CounterVec
	promBytesProcessed   *prometheus.CounterVec
	promMapEntries       *prometheus.GaugeVec
	promProgramsLoaded   *prometheus.GaugeVec

	// HTTP servers
	promServer *http.Server
	jsonServer *http.Server
}

// ExportConfig configuration for statistics export
type ExportConfig struct {
	EnablePrometheus bool          `json:"enable_prometheus"`
	PrometheusPort   int           `json:"prometheus_port"`
	EnableJSON       bool          `json:"enable_json"`
	JSONEndpoint     string        `json:"json_endpoint"`
	ExportInterval   time.Duration `json:"export_interval"`
}

// DefaultExportConfig returns default export configuration
func DefaultExportConfig() ExportConfig {
	return ExportConfig{
		EnablePrometheus: true,
		PrometheusPort:   9090,
		EnableJSON:       true,
		JSONEndpoint:     ":8080",
		ExportInterval:   10 * time.Second,
	}
}

// NewExporter creates a new statistics exporter
func NewExporter(collector *Collector, config ExportConfig) *Exporter {
	e := &Exporter{
		collector: collector,
		config:    config,
	}

	// Initialize Prometheus metrics
	e.initPrometheusMetrics()

	return e
}

// initPrometheusMetrics initializes Prometheus metrics
func (e *Exporter) initPrometheusMetrics() {
	e.promPacketsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flywall_ebpf_packets_processed_total",
			Help: "Total number of packets processed by eBPF programs",
		},
		[]string{"program"},
	)

	e.promPacketsDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flywall_ebpf_packets_dropped_total",
			Help: "Total number of packets dropped by eBPF programs",
		},
		[]string{"program"},
	)

	e.promPacketsPassed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flywall_ebpf_packets_passed_total",
			Help: "Total number of packets passed by eBPF programs",
		},
		[]string{"program"},
	)

	e.promBytesProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flywall_ebpf_bytes_processed_total",
			Help: "Total number of bytes processed by eBPF programs",
		},
		[]string{"program"},
	)

	e.promMapEntries = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "flywall_ebpf_map_entries",
			Help: "Number of entries in eBPF maps",
		},
		[]string{"program", "map"},
	)

	e.promProgramsLoaded = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "flywall_ebpf_programs_loaded",
			Help: "Number of eBPF programs loaded",
		},
		[]string{"program"},
	)
}

// Start starts the statistics exporter
func (e *Exporter) Start(ctx context.Context) error {
	// Register Prometheus metrics
	if e.config.EnablePrometheus {
		prometheus.MustRegister(e.promPacketsProcessed)
		prometheus.MustRegister(e.promPacketsDropped)
		prometheus.MustRegister(e.promPacketsPassed)
		prometheus.MustRegister(e.promBytesProcessed)
		prometheus.MustRegister(e.promMapEntries)
		prometheus.MustRegister(e.promProgramsLoaded)

		// Start Prometheus server
		go e.startPrometheusServer()
	}

	// Start JSON endpoint
	if e.config.EnableJSON {
		go e.startJSONEndpoint()
	}

	// Start periodic export
	go e.periodicExport(ctx)

	return nil
}

// startPrometheusServer starts the Prometheus metrics server
func (e *Exporter) startPrometheusServer() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	e.promServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", e.config.PrometheusPort),
		Handler: mux,
	}

	log.Printf("Prometheus metrics server listening on :%d/metrics", e.config.PrometheusPort)
	if err := e.promServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Prometheus server error: %v", err)
	}
}

// startJSONEndpoint starts the JSON metrics endpoint
func (e *Exporter) startJSONEndpoint() {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", e.handleJSONMetrics)

	e.jsonServer = &http.Server{
		Addr:    e.config.JSONEndpoint,
		Handler: mux,
	}

	log.Printf("JSON metrics endpoint listening on %s", e.config.JSONEndpoint)
	if err := e.jsonServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("JSON endpoint error: %v", err)
	}
}

// handleJSONMetrics handles JSON metrics requests
func (e *Exporter) handleJSONMetrics(w http.ResponseWriter, r *http.Request) {
	stats := e.collector.ExportStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// periodicExport periodically updates Prometheus metrics
func (e *Exporter) periodicExport(ctx context.Context) {
	ticker := time.NewTicker(e.config.ExportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.updatePrometheusMetrics()
		}
	}
}

// updatePrometheusMetrics updates Prometheus metrics with current statistics
func (e *Exporter) updatePrometheusMetrics() {
	stats := e.collector.Collect()

	// Update packet counters
	e.promPacketsProcessed.WithLabelValues("total").Add(float64(stats.PacketsProcessed))
	e.promPacketsDropped.WithLabelValues("total").Add(float64(stats.PacketsDropped))
	e.promPacketsPassed.WithLabelValues("total").Add(float64(stats.PacketsPassed))
	e.promBytesProcessed.WithLabelValues("total").Add(float64(stats.BytesProcessed))

	// Update map metrics
	for mapName, count := range stats.Maps {
		// Parse program and map name
		// This is a simple parsing - adjust based on your naming convention
		parts := parseMapName(mapName)
		if len(parts) == 2 {
			e.promMapEntries.WithLabelValues(parts[0], parts[1]).Set(float64(count))
		}
	}

	// Update program metrics
	for progName := range stats.Programs {
		e.promProgramsLoaded.WithLabelValues(progName).Set(1)
	}
}

// parseMapName parses a full map name into program and map components
func parseMapName(fullName string) []string {
	// Simple parsing - assumes format "program.map"
	// Adjust based on your actual naming convention
	parts := []string{"unknown", "unknown"}
	if dot := findLastDot(fullName); dot != -1 {
		parts[0] = fullName[:dot]
		parts[1] = fullName[dot+1:]
	}
	return parts
}

// Stop stops the statistics exporter and cleans up resources
func (e *Exporter) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if e.promServer != nil {
		if err := e.promServer.Shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown Prometheus server: %v", err)
		}
	}

	if e.jsonServer != nil {
		if err := e.jsonServer.Shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown JSON server: %v", err)
		}
	}

	if e.config.EnablePrometheus {
		prometheus.Unregister(e.promPacketsProcessed)
		prometheus.Unregister(e.promPacketsDropped)
		prometheus.Unregister(e.promPacketsPassed)
		prometheus.Unregister(e.promBytesProcessed)
		prometheus.Unregister(e.promMapEntries)
		prometheus.Unregister(e.promProgramsLoaded)
	}
}

// findLastDot finds the last dot in a string
func findLastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}
