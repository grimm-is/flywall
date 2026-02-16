// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ebpf

import (
	"time"

	"grimm.is/flywall/internal/ebpf/stats"
	"grimm.is/flywall/internal/ebpf/types"
)

// ResourceType represents different types of eBPF resources
type ResourceType string

const (
	ResourceTypeProgram ResourceType = "program"
	ResourceTypeMap     ResourceType = "map"
	ResourceTypeLink    ResourceType = "link"
	ResourceTypeTable   ResourceType = "table"
)

// Feature represents an eBPF feature
type Feature struct {
	Name         string                 `json:"name"`
	Enabled      bool                   `json:"enabled"`
	ResourceType ResourceType           `json:"resource_type"`
	Dependencies []string               `json:"dependencies"`
	Priority     int                    `json:"priority"`
	Cost         ResourceCost           `json:"cost"`
	Status       FeatureStatus          `json:"status"`
	Config       map[string]interface{} `json:"config"`
}

// FeatureStatus represents the current status of a feature
type FeatureStatus struct {
	Active       bool      `json:"active"`
	LoadedAt     time.Time `json:"loaded_at"`
	LastCleanup  time.Time `json:"last_cleanup"`
	Error        string    `json:"error,omitempty"`
	SamplingRate float64   `json:"sampling_rate"`
}

// ResourceCost represents the resource cost of a feature
type ResourceCost struct {
	CPU          float64 `json:"cpu_percent"`
	Memory       int     `json:"memory_mb"`
	MapLookups   float64 `json:"lookups_per_packet"`
	EventsPerSec int     `json:"events_per_sec"`
	MaxPPS       float64 `json:"max_mpps"`
}

// TLSInfo contains TLS-specific flow information
type TLSInfo struct {
	JA3Hash     string `json:"ja3_hash,omitempty"`
	SNI         string `json:"sni,omitempty"`
	Version     uint16 `json:"version,omitempty"`
	CipherSuite uint16 `json:"cipher_suite,omitempty"`
}

// Statistics represents various eBPF statistics
type Statistics struct {
	PacketsProcessed uint64 `json:"packets_processed"`
	PacketsDropped   uint64 `json:"packets_dropped"`
	PacketsPassed    uint64 `json:"packets_passed"`
	BytesProcessed   uint64 `json:"bytes_processed"`

	// Per-program stats
	XDPStats    ProgramStats `json:"xdp,omitempty"`
	TCStats     ProgramStats `json:"tc,omitempty"`
	SocketStats ProgramStats `json:"socket,omitempty"`

	// Feature-specific stats
	BlockedIPs      uint64 `json:"blocked_ips"`
	BlockedDNS      uint64 `json:"blocked_dns"`
	FlowsOffloaded  uint64 `json:"flows_offloaded"`
	EventsGenerated uint64 `json:"events_generated"`
}

// ProgramStats represents statistics for a specific program type
type ProgramStats struct {
	Runs     uint64        `json:"runs"`
	Errors   uint64        `json:"errors"`
	Duration time.Duration `json:"avg_duration_ns"`
}

// Event represents an eBPF event sent to userspace
type Event struct {
	Type      types.EventType `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Source    string          `json:"source"`
	Data      interface{}     `json:"data"`
}

// MapInfo represents information about an eBPF map
type MapInfo struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	MaxEntries   uint32    `json:"max_entries"`
	CurrentSize  uint32    `json:"current_size"`
	KeySize      uint32    `json:"key_size"`
	ValueSize    uint32    `json:"value_size"`
	Flags        uint32    `json:"flags"`
	CreatedAt    time.Time `json:"created_at"`
	LastAccessed time.Time `json:"last_accessed"`
}

// ProgramInfo represents information about a loaded eBPF program
type ProgramInfo struct {
	Name       string            `json:"name"`
	Type       types.ProgramType `json:"program_type"`
	Tag        string            `json:"tag"`
	ID         uint32            `json:"id"`
	AttachedTo []string          `json:"attached_to"`
	LoadedAt   time.Time         `json:"loaded_at"`
	RunCount   uint64            `json:"run_count"`
	LastRun    time.Time         `json:"last_run"`
	Maps       map[string]MapInfo `json:"maps"`
}

// Config represents the eBPF configuration
type Config struct {
	Enabled     bool                     `json:"enabled"`
	Features    map[string]FeatureConfig `json:"features"`
	Performance PerformanceConfig        `json:"performance"`
	Adaptive    AdaptiveConfig           `json:"adaptive"`
	Maps        MapConfig                `json:"maps"`
	Programs    ProgramConfig            `json:"programs"`
	StatsExport *stats.ExportConfig      `json:"stats_export,omitempty"`
}

// ConfigFromGlobal converts a config.EBPFConfig to ebpf.Config
func ConfigFromGlobal(cfg interface{}) *Config {
	// Handle nil config
	if cfg == nil {
		return &Config{Enabled: false}
	}

	// Build config with defaults
	result := &Config{
		Enabled:  false,
		Features: make(map[string]FeatureConfig),
		Performance: PerformanceConfig{
			MaxCPUPercent:   80,
			MaxMemoryMB:     500,
			MaxEventsPerSec: 10000,
			MaxPPS:          10000000,
		},
		Adaptive: AdaptiveConfig{
			Enabled:            false,
			ScaleBackThreshold: 80,
			ScaleBackRate:      0.1,
			MinimumFeatures:    []string{"ddos_protection", "flow_monitoring"},
			SamplingConfig: SamplingConfig{
				Enabled:       false,
				MinSampleRate: 0.1,
				MaxSampleRate: 1.0,
				AdaptiveRate:  true,
			},
		},
		Maps: MapConfig{
			MaxMaps:       100,
			MaxMapEntries: 1000000,
			MaxMapMemory:  100,
			CacheSize:     1000,
		},
		Programs: ProgramConfig{
			XDPBlocklist: "xdp_blocklist.o",
			TCClassifier: "tc_classifier.o",
			SocketDNS:    "socket_dns.o",
			SocketTLS:    "socket_tls.o",
			SocketDHCP:   "socket_dhcp.o",
		},
	}

	return result
}

// FeatureConfig represents configuration for a specific feature
type FeatureConfig struct {
	Enabled  bool                   `json:"enabled"`
	Priority int                    `json:"priority"`
	Config   map[string]interface{} `json:"config"`
}

// PerformanceConfig represents performance-related configuration
type PerformanceConfig struct {
	MaxCPUPercent   float64 `json:"max_cpu_percent"`
	MaxMemoryMB     int     `json:"max_memory_mb"`
	MaxEventsPerSec int     `json:"max_events_per_sec"`
	MaxPPS          uint64  `json:"max_pps"`
}

// AdaptiveConfig represents adaptive behavior configuration
type AdaptiveConfig struct {
	Enabled            bool           `json:"enabled"`
	ScaleBackThreshold float64        `json:"scale_back_threshold"`
	ScaleBackRate      float64        `json:"scale_back_rate"`
	MinimumFeatures    []string       `json:"minimum_features"`
	SamplingConfig     SamplingConfig `json:"sampling"`
}

// SamplingConfig represents sampling configuration
type SamplingConfig struct {
	Enabled       bool    `json:"enabled"`
	MinSampleRate float64 `json:"min_sample_rate"`
	MaxSampleRate float64 `json:"max_sample_rate"`
	AdaptiveRate  bool    `json:"adaptive_rate"`
}

// MapConfig represents map configuration
type MapConfig struct {
	MaxMaps       uint32 `json:"max_maps"`
	MaxMapEntries uint32 `json:"max_map_entries"`
	MaxMapMemory  uint32 `json:"max_map_memory"`
	CacheSize     uint32 `json:"cache_size"`
}

// ProgramConfig represents program configuration
type ProgramConfig struct {
	XDPBlocklist string `json:"xdp_blocklist"`
	TCClassifier string `json:"tc_classifier"`
	SocketDNS    string `json:"socket_dns"`
	SocketTLS    string `json:"socket_tls"`
	SocketDHCP   string `json:"socket_dhcp"`
}
