// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package interfaces

import (
	"context"
	"time"
)

// FeatureStatus represents the status of an eBPF feature
type FeatureStatus struct {
	Enabled      bool    `json:"enabled"`
	Active       bool    `json:"active"`
	SamplingRate float64 `json:"sampling_rate"`
	PacketCount  uint64  `json:"packet_count"`
	DropCount    uint64  `json:"drop_count"`
	ErrorCount   uint64  `json:"error_count"`
	LastError    string  `json:"last_error"`
}

// Statistics represents eBPF statistics
type Statistics struct {
	PacketsProcessed uint64            `json:"packets_processed"`
	PacketsDropped   uint64            `json:"packets_dropped"`
	PacketsPassed    uint64            `json:"packets_passed"`
	BytesProcessed   uint64            `json:"bytes_processed"`
	Features         map[string]uint64 `json:"features"`
	Maps             map[string]uint64 `json:"maps"`
	Programs         map[string]uint64 `json:"programs"`
}

// DNSBlocklistService defines the interface for DNS blocklist service
type DNSBlocklistService interface {
	AddDomain(domain string) error
	RemoveDomain(domain string) error
	IsBlocked(domain string) bool
	GetStats() *Stats
	Export() []string
	Import(domains []string) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Stats represents DNS blocklist statistics
type Stats struct {
	DomainCount    int           `json:"domain_count"`
	BloomSize      uint32        `json:"bloom_size"`
	HashCount      uint32        `json:"hash_count"`
	SourceCount    int           `json:"source_count"`
	LastUpdate     time.Time     `json:"last_update"`
	UpdateInterval time.Duration `json:"update_interval"`
}

// Manager defines the interface for eBPF manager
type Manager interface {
	// Lifecycle management
	Load() error
	Start() error
	Stop() error
	Close() error

	// Feature management
	GetFeatureStatus() map[string]FeatureStatus
	EnableFeature(name string) error
	DisableFeature(name string) error

	// Statistics
	GetStatistics() *Statistics

	// Map and hook information
	GetMapInfo() map[string]MapInfo
	GetHookInfo() map[string]interface{}

	// DNS blocklist service
	GetDNSBlocklistService() DNSBlocklistService
}
