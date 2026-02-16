// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"encoding/json"
	"fmt"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// EBPFConfig defines the eBPF configuration for high-performance packet processing
type EBPFConfig struct {
	// Enable eBPF subsystem
	// @default: false
	Enabled bool `hcl:"enabled,optional" json:"enabled"`

	// Feature-specific configurations
	Features []*EBPFFeatureConfig `hcl:"feature,block" json:"features,omitempty"`

	// Performance settings
	Performance *EBPFPerformanceConfig `hcl:"performance,block" json:"performance,omitempty"`

	// Adaptive performance management
	Adaptive *EBPFAdaptiveConfig `hcl:"adaptive,block" json:"adaptive,omitempty"`

	// Map configuration
	Maps *EBPFMapConfig `hcl:"maps,block" json:"maps,omitempty"`

	// Program configuration
	Programs *EBPFProgramConfig `hcl:"programs,block" json:"programs,omitempty"`

	// Fallback configuration
	Fallback *EBPFFallbackConfig `hcl:"fallback,block" json:"fallback,omitempty"`

	// Statistics export configuration
	StatsExport *StatsExportConfig `hcl:"stats_export,block" json:"stats_export,omitempty"`
}

// EBPFFeatureConfig defines configuration for a specific eBPF feature
type EBPFFeatureConfig struct {
	// Feature name
	Name string `hcl:"name,label" json:"name"`

	// Enable this feature
	// @default: false
	Enabled bool `hcl:"enabled,optional" json:"enabled"`

	// Feature priority (higher = more important)
	// @default: 50
	Priority int `hcl:"priority,optional" json:"priority"`

	// Feature-specific configuration
	Config cty.Value `hcl:"config,optional" json:"config,omitempty"`
}

// GobEncode encodes the EBPFFeatureConfig for gob serialization
func (c *EBPFFeatureConfig) GobEncode() ([]byte, error) {
	// Convert cty.Value to JSON for serialization
	var configJSON []byte
	var typeJSON []byte
	var err error

	if !c.Config.IsNull() && c.Config.IsKnown() {
		// Marshal the type first
		typeJSON, err = ctyjson.MarshalType(c.Config.Type())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal cty.Type: %w", err)
		}

		// Then marshal the value
		configJSON, err = ctyjson.Marshal(c.Config, c.Config.Type())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal cty.Value: %w", err)
		}
	}

	// Create a serializable version
	type EBPFFeatureConfigSerializable struct {
		Name     string
		Enabled  bool
		Priority int
		Config   []byte // JSON-encoded cty.Value
		TypeJSON []byte // JSON-encoded cty.Type
	}

	serializable := EBPFFeatureConfigSerializable{
		Name:     c.Name,
		Enabled:  c.Enabled,
		Priority: c.Priority,
		Config:   configJSON,
		TypeJSON: typeJSON,
	}

	return json.Marshal(serializable)
}

// GobDecode decodes the EBPFFeatureConfig from gob serialization
func (c *EBPFFeatureConfig) GobDecode(data []byte) error {
	// Create a serializable version
	type EBPFFeatureConfigSerializable struct {
		Name     string
		Enabled  bool
		Priority int
		Config   []byte // JSON-encoded cty.Value
		TypeJSON []byte // JSON-encoded cty.Type
	}

	var serializable EBPFFeatureConfigSerializable
	if err := json.Unmarshal(data, &serializable); err != nil {
		return fmt.Errorf("failed to unmarshal EBPFFeatureConfig: %w", err)
	}

	// Restore fields
	c.Name = serializable.Name
	c.Enabled = serializable.Enabled
	c.Priority = serializable.Priority

	// Restore cty.Value from JSON
	if len(serializable.Config) > 0 && len(serializable.TypeJSON) > 0 {
		// Unmarshal the type first
		typ, err := ctyjson.UnmarshalType(serializable.TypeJSON)
		if err != nil {
			return fmt.Errorf("failed to unmarshal cty.Type: %w", err)
		}

		// Then unmarshal the value with the type
		config, err := ctyjson.Unmarshal(serializable.Config, typ)
		if err != nil {
			return fmt.Errorf("failed to unmarshal cty.Value: %w", err)
		}
		c.Config = config
	} else {
		c.Config = cty.NilVal
	}

	return nil
}

// EBPFPerformanceConfig defines performance-related settings
type EBPFPerformanceConfig struct {
	// Maximum CPU usage percentage
	// @default: 80
	MaxCPUPercent float64 `hcl:"max_cpu_percent,optional" json:"max_cpu_percent"`

	// Maximum memory usage in MB
	// @default: 500
	MaxMemoryMB int `hcl:"max_memory_mb,optional" json:"max_memory_mb"`

	// Maximum events per second
	// @default: 10000
	MaxEventsPerSec int `hcl:"max_events_per_sec,optional" json:"max_events_per_sec"`

	// Maximum packets per second
	// @default: 10000000
	MaxPPS uint64 `hcl:"max_pps,optional" json:"max_pps"`
}

// EBPFAdaptiveConfig defines adaptive performance management
type EBPFAdaptiveConfig struct {
	// Enable adaptive performance management
	// @default: false
	Enabled bool `hcl:"enabled,optional" json:"enabled"`

	// CPU usage threshold to trigger scale-back
	// @default: 80
	ScaleBackThreshold float64 `hcl:"scale_back_threshold,optional" json:"scale_back_threshold"`

	// Scale-back rate (0.0-1.0)
	// @default: 0.1
	ScaleBackRate float64 `hcl:"scale_back_rate,optional" json:"scale_back_rate"`

	// Minimum features to keep active
	// @default: ["ddos_protection", "flow_monitoring"]
	MinimumFeatures []string `hcl:"minimum_features,optional" json:"minimum_features,omitempty"`

	// Sampling configuration
	Sampling *EBPFSamplingConfig `hcl:"sampling,block" json:"sampling,omitempty"`
}

// EBPFSamplingConfig defines adaptive sampling settings
type EBPFSamplingConfig struct {
	// Enable adaptive sampling
	// @default: false
	Enabled bool `hcl:"enabled,optional" json:"enabled"`

	// Minimum sampling rate
	// @default: 0.1
	MinSampleRate float64 `hcl:"min_sample_rate,optional" json:"min_sample_rate"`

	// Maximum sampling rate
	// @default: 1.0
	MaxSampleRate float64 `hcl:"max_sample_rate,optional" json:"max_sample_rate"`

	// Enable adaptive rate adjustment
	// @default: true
	AdaptiveRate bool `hcl:"adaptive_rate,optional" json:"adaptive_rate"`
}

// EBPFMapConfig defines eBPF map settings
type EBPFMapConfig struct {
	// Maximum number of maps
	// @default: 100
	MaxMaps uint32 `hcl:"max_maps,optional" json:"max_maps"`

	// Maximum entries per map
	// @default: 1000000
	MaxMapEntries uint32 `hcl:"max_map_entries,optional" json:"max_map_entries"`

	// Maximum map memory in MB
	// @default: 100
	MaxMapMemory uint32 `hcl:"max_map_memory,optional" json:"max_map_memory"`

	// Map cache size
	// @default: 1000
	CacheSize uint32 `hcl:"cache_size,optional" json:"cache_size"`
}

// EBPFProgramConfig defines eBPF program settings
type EBPFProgramConfig struct {
	// XDP blocklist program
	XDPBlocklist string `hcl:"xdp_blocklist,optional" json:"xdp_blocklist,omitempty"`

	// TC classifier program
	TCClassifier string `hcl:"tc_classifier,optional" json:"tc_classifier,omitempty"`

	// DNS socket filter program
	SocketDNS string `hcl:"socket_dns,optional" json:"socket_dns,omitempty"`

	// TLS socket filter program
	SocketTLS string `hcl:"socket_tls,optional" json:"socket_tls,omitempty"`

	// DHCP socket filter program
	SocketDHCP string `hcl:"socket_dhcp,optional" json:"socket_dhcp,omitempty"`
}

// EBPFFallbackConfig defines fallback behavior
type EBPFFallbackConfig struct {
	// Enable NFQUEUE fallback when eBPF fails
	// @default: true
	EnableNFQUEUE bool `hcl:"enable_nfqueue,optional" json:"enable_nfqueue"`

	// Enable partial eBPF support
	// @default: true
	PartialSupport bool `hcl:"partial_support,optional" json:"partial_support"`

	// Action on program load failure
	// @enum: disable_feature, use_fallback, fail
	// @default: "disable_feature"
	OnLoadFailure string `hcl:"on_load_failure,optional" json:"on_load_failure,omitempty"`

	// Action on map creation failure
	// @enum: reduce_capacity, use_fallback, fail
	// @default: "reduce_capacity"
	OnMapFailure string `hcl:"on_map_failure,optional" json:"on_map_failure,omitempty"`

	// Action on hook attachment failure
	// @enum: try_alternative, use_fallback, fail
	// @default: "try_alternative"
	OnHookFailure string `hcl:"on_hook_failure,optional" json:"on_hook_failure,omitempty"`

	// Action on verifier rejection
	// @enum: use_simpler_version, use_fallback, fail
	// @default: "use_simpler_version"
	OnVerifierFailure string `hcl:"on_verifier_failure,optional" json:"on_verifier_failure,omitempty"`

	// Recovery interval for complete failures
	// @default: "30s"
	RecoveryInterval string `hcl:"recovery_interval,optional" json:"recovery_interval,omitempty"`
}

// StatsExportConfig defines statistics export settings
type StatsExportConfig struct {
	EnablePrometheus bool   `hcl:"enable_prometheus,optional" json:"enable_prometheus"`
	PrometheusPort   int    `hcl:"prometheus_port,optional" json:"prometheus_port"`
	EnableJSON       bool   `hcl:"enable_json,optional" json:"enable_json"`
	JSONEndpoint     string `hcl:"json_endpoint,optional" json:"json_endpoint"`
}

// DefaultEBPFConfig returns the default eBPF configuration
func DefaultEBPFConfig() *EBPFConfig {
	return &EBPFConfig{
		Enabled: false,
		Features: []*EBPFFeatureConfig{
			{
				Name:     "ddos_protection",
				Enabled:  false,
				Priority: 100,
			},
			{
				Name:     "dns_blocklist",
				Enabled:  false,
				Priority: 95,
			},
			{
				Name:     "inline_ips",
				Enabled:  false,
				Priority: 90,
			},
			{
				Name:     "flow_monitoring",
				Enabled:  false,
				Priority: 85,
			},
			{
				Name:     "tls_fingerprinting",
				Enabled:  false,
				Priority: 70,
			},
			{
				Name:     "device_discovery",
				Enabled:  false,
				Priority: 40,
			},
			{
				Name:     "statistics",
				Enabled:  false,
				Priority: 20,
			},
			{
				Name:     "qos",
				Enabled:  false,
				Priority: 60,
			},
		},
		Performance: &EBPFPerformanceConfig{
			MaxCPUPercent:   80,
			MaxMemoryMB:     500,
			MaxEventsPerSec: 10000,
			MaxPPS:          10000000,
		},
		Adaptive: &EBPFAdaptiveConfig{
			Enabled:            false,
			ScaleBackThreshold: 80,
			ScaleBackRate:      0.1,
			MinimumFeatures:    []string{"ddos_protection", "flow_monitoring"},
			Sampling: &EBPFSamplingConfig{
				Enabled:       false,
				MinSampleRate: 0.1,
				MaxSampleRate: 1.0,
				AdaptiveRate:  true,
			},
		},
		Maps: &EBPFMapConfig{
			MaxMaps:       100,
			MaxMapEntries: 1000000,
			MaxMapMemory:  100,
			CacheSize:     1000,
		},
		Programs: &EBPFProgramConfig{
			XDPBlocklist: "xdp_blocklist.o",
			TCClassifier: "tc_classifier.o",
			SocketDNS:    "socket_dns.o",
			SocketTLS:    "socket_tls.o",
			SocketDHCP:   "socket_dhcp.o",
		},
		Fallback: &EBPFFallbackConfig{
			EnableNFQUEUE:     true,
			PartialSupport:    true,
			OnLoadFailure:     "disable_feature",
			OnMapFailure:      "reduce_capacity",
			OnHookFailure:     "try_alternative",
			OnVerifierFailure: "use_simpler_version",
			RecoveryInterval:  "30s",
		},
		StatsExport: &StatsExportConfig{
			EnablePrometheus: true,
			PrometheusPort:   9090,
			EnableJSON:       true,
			JSONEndpoint:     ":8080",
		},
	}
}

// Validate validates the eBPF configuration
func (c *EBPFConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	// Validate performance settings
	if c.Performance != nil {
		if c.Performance.MaxCPUPercent < 0 || c.Performance.MaxCPUPercent > 100 {
			return fmt.Errorf("ebpf.performance.max_cpu_percent must be between 0 and 100")
		}
		if c.Performance.MaxMemoryMB < 0 {
			return fmt.Errorf("ebpf.performance.max_memory_mb must be positive")
		}
		if c.Performance.MaxEventsPerSec < 0 {
			return fmt.Errorf("ebpf.performance.max_events_per_sec must be positive")
		}
	}

	// Validate adaptive settings
	if c.Adaptive != nil && c.Adaptive.Enabled {
		if c.Adaptive.ScaleBackThreshold < 0 || c.Adaptive.ScaleBackThreshold > 100 {
			return fmt.Errorf("ebpf.adaptive.scale_back_threshold must be between 0 and 100")
		}
		if c.Adaptive.ScaleBackRate < 0 || c.Adaptive.ScaleBackRate > 1 {
			return fmt.Errorf("ebpf.adaptive.scale_back_rate must be between 0 and 1")
		}
		if c.Adaptive.Sampling != nil {
			if c.Adaptive.Sampling.MinSampleRate < 0 || c.Adaptive.Sampling.MinSampleRate > 1 {
				return fmt.Errorf("ebpf.adaptive.sampling.min_sample_rate must be between 0 and 1")
			}
			if c.Adaptive.Sampling.MaxSampleRate < 0 || c.Adaptive.Sampling.MaxSampleRate > 1 {
				return fmt.Errorf("ebpf.adaptive.sampling.max_sample_rate must be between 0 and 1")
			}
			if c.Adaptive.Sampling.MinSampleRate > c.Adaptive.Sampling.MaxSampleRate {
				return fmt.Errorf("ebpf.adaptive.sampling.min_sample_rate must be <= max_sample_rate")
			}
		}
	}

	// Validate map settings
	if c.Maps != nil {
		if c.Maps.MaxMaps == 0 {
			return fmt.Errorf("ebpf.maps.max_maps must be positive")
		}
		if c.Maps.MaxMapEntries == 0 {
			return fmt.Errorf("ebpf.maps.max_map_entries must be positive")
		}
	}

	// Validate fallback settings
	if c.Fallback != nil {
		validActions := map[string]bool{
			"disable_feature":     true,
			"use_fallback":        true,
			"fail":                true,
			"reduce_capacity":     true,
			"try_alternative":     true,
			"use_simpler_version": true,
		}

		if !validActions[c.Fallback.OnLoadFailure] {
			return fmt.Errorf("invalid ebpf.fallback.on_load_failure: %s", c.Fallback.OnLoadFailure)
		}
		if !validActions[c.Fallback.OnMapFailure] {
			return fmt.Errorf("invalid ebpf.fallback.on_map_failure: %s", c.Fallback.OnMapFailure)
		}
		if !validActions[c.Fallback.OnHookFailure] {
			return fmt.Errorf("invalid ebpf.fallback.on_hook_failure: %s", c.Fallback.OnHookFailure)
		}
		if !validActions[c.Fallback.OnVerifierFailure] {
			return fmt.Errorf("invalid ebpf.fallback.on_verifier_failure: %s", c.Fallback.OnVerifierFailure)
		}
	}

	return nil
}

// Merge merges another eBPF config into this one
func (c *EBPFConfig) Merge(other *EBPFConfig) {
	if other == nil {
		return
	}

	if other.Enabled {
		c.Enabled = other.Enabled
	}

	// Merge features
	if c.Features == nil {
		c.Features = []*EBPFFeatureConfig{}
	}

	// Create a map of existing features for quick lookup
	existingMap := make(map[string]*EBPFFeatureConfig)
	for _, f := range c.Features {
		existingMap[f.Name] = f
	}

	// Merge or add features from other
	for _, feature := range other.Features {
		if existing, exists := existingMap[feature.Name]; exists {
			if feature.Enabled {
				existing.Enabled = feature.Enabled
			}
			if feature.Priority != 0 {
				existing.Priority = feature.Priority
			}
			if !feature.Config.IsNull() {
				if existing.Config.IsNull() {
					existing.Config = feature.Config
				}
				// For now, just replace - merging cty.Value is complex
			}
		} else {
			c.Features = append(c.Features, feature)
		}
	}

	// Merge other sections
	if other.Performance != nil {
		if c.Performance == nil {
			c.Performance = other.Performance
		} else {
			c.Performance.Merge(other.Performance)
		}
	}

	if other.Adaptive != nil {
		if c.Adaptive == nil {
			c.Adaptive = other.Adaptive
		} else {
			c.Adaptive.Merge(other.Adaptive)
		}
	}

	if other.Maps != nil {
		if c.Maps == nil {
			c.Maps = other.Maps
		} else {
			c.Maps.Merge(other.Maps)
		}
	}

	if other.Programs != nil {
		if c.Programs == nil {
			c.Programs = other.Programs
		} else {
			c.Programs.Merge(other.Programs)
		}
	}

	if other.Fallback != nil {
		if c.Fallback == nil {
			c.Fallback = other.Fallback
		} else {
			c.Fallback.Merge(other.Fallback)
		}
	}

	if other.StatsExport != nil {
		if c.StatsExport == nil {
			c.StatsExport = other.StatsExport
		} else {
			c.StatsExport.Merge(other.StatsExport)
		}
	}
}

// Merge merges performance config
func (c *EBPFPerformanceConfig) Merge(other *EBPFPerformanceConfig) {
	if other.MaxCPUPercent != 0 {
		c.MaxCPUPercent = other.MaxCPUPercent
	}
	if other.MaxMemoryMB != 0 {
		c.MaxMemoryMB = other.MaxMemoryMB
	}
	if other.MaxEventsPerSec != 0 {
		c.MaxEventsPerSec = other.MaxEventsPerSec
	}
	if other.MaxPPS != 0 {
		c.MaxPPS = other.MaxPPS
	}
}

// Merge merges adaptive config
func (c *EBPFAdaptiveConfig) Merge(other *EBPFAdaptiveConfig) {
	if other.Enabled {
		c.Enabled = other.Enabled
	}
	if other.ScaleBackThreshold != 0 {
		c.ScaleBackThreshold = other.ScaleBackThreshold
	}
	if other.ScaleBackRate != 0 {
		c.ScaleBackRate = other.ScaleBackRate
	}
	if other.MinimumFeatures != nil {
		c.MinimumFeatures = other.MinimumFeatures
	}
	if other.Sampling != nil {
		if c.Sampling == nil {
			c.Sampling = other.Sampling
		} else {
			c.Sampling.Merge(other.Sampling)
		}
	}
}

// Merge merges sampling config
func (c *EBPFSamplingConfig) Merge(other *EBPFSamplingConfig) {
	if other.Enabled {
		c.Enabled = other.Enabled
	}
	if other.MinSampleRate != 0 {
		c.MinSampleRate = other.MinSampleRate
	}
	if other.MaxSampleRate != 0 {
		c.MaxSampleRate = other.MaxSampleRate
	}
	if other.AdaptiveRate {
		c.AdaptiveRate = other.AdaptiveRate
	}
}

// Merge merges map config
func (c *EBPFMapConfig) Merge(other *EBPFMapConfig) {
	if other.MaxMaps != 0 {
		c.MaxMaps = other.MaxMaps
	}
	if other.MaxMapEntries != 0 {
		c.MaxMapEntries = other.MaxMapEntries
	}
	if other.MaxMapMemory != 0 {
		c.MaxMapMemory = other.MaxMapMemory
	}
	if other.CacheSize != 0 {
		c.CacheSize = other.CacheSize
	}
}

// Merge merges program config
func (c *EBPFProgramConfig) Merge(other *EBPFProgramConfig) {
	if other.XDPBlocklist != "" {
		c.XDPBlocklist = other.XDPBlocklist
	}
	if other.TCClassifier != "" {
		c.TCClassifier = other.TCClassifier
	}
	if other.SocketDNS != "" {
		c.SocketDNS = other.SocketDNS
	}
	if other.SocketTLS != "" {
		c.SocketTLS = other.SocketTLS
	}
	if other.SocketDHCP != "" {
		c.SocketDHCP = other.SocketDHCP
	}
}

// Merge merges fallback config
func (c *EBPFFallbackConfig) Merge(other *EBPFFallbackConfig) {
	if other.EnableNFQUEUE {
		c.EnableNFQUEUE = other.EnableNFQUEUE
	}
	if other.PartialSupport {
		c.PartialSupport = other.PartialSupport
	}
	if other.OnLoadFailure != "" {
		c.OnLoadFailure = other.OnLoadFailure
	}
	if other.OnMapFailure != "" {
		c.OnMapFailure = other.OnMapFailure
	}
	if other.OnHookFailure != "" {
		c.OnHookFailure = other.OnHookFailure
	}
	if other.OnVerifierFailure != "" {
		c.OnVerifierFailure = other.OnVerifierFailure
	}
	if other.RecoveryInterval != "" {
		c.RecoveryInterval = other.RecoveryInterval
	}
}

// Merge merges stats export config
func (c *StatsExportConfig) Merge(other *StatsExportConfig) {
	if other.EnablePrometheus {
		c.EnablePrometheus = other.EnablePrometheus
	}
	if other.PrometheusPort != 0 {
		c.PrometheusPort = other.PrometheusPort
	}
	if other.EnableJSON {
		c.EnableJSON = other.EnableJSON
	}
	if other.JSONEndpoint != "" {
		c.JSONEndpoint = other.JSONEndpoint
	}
}
