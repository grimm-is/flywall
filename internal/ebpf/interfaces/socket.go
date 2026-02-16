// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package interfaces

import (
	"time"

	"grimm.is/flywall/internal/ebpf/types"
)

// SocketFilterManager defines the interface for managing socket filters
type SocketFilterManager interface {
	// Lifecycle
	Start() error
	Stop()
	IsEnabled() bool

	// Statistics
	GetStatistics() interface{}

	// Component access
	GetDNSFilter() DNSFilterInterface
	GetQueryLogger() QueryLoggerInterface
	GetResponseFilter() ResponseFilterInterface
}

// DNSFilterInterface defines the interface for DNS socket filters
type DNSFilterInterface interface {
	// Lifecycle
	Start() error
	Stop()
	IsEnabled() bool

	// Configuration
	SetQueryHandler(handler func(event *types.DNSQueryEvent) error)
	SetResponseHandler(handler func(event *types.DNSResponseEvent) error)

	// Statistics
	GetStatistics() interface{}
}

// QueryLoggerInterface defines the interface for DNS query logging
type QueryLoggerInterface interface {
	// Lifecycle
	Start() error
	Stop()
	IsEnabled() bool

	// Logging
	LogQuery(event *types.DNSQueryEvent)
	LogResponse(event *types.DNSResponseEvent, blocked bool, reason string)

	// Statistics
	GetStatistics() interface{}
}

// ResponseFilterInterface defines the interface for DNS response filtering
type ResponseFilterInterface interface {
	// Lifecycle
	Start() error
	Stop()
	IsEnabled() bool

	// Filtering
	FilterResponse(event *types.DNSResponseEvent) (bool, string)

	// Configuration
	SetBlockHandler(handler func(event *types.DNSResponseEvent, reason string) error)
	SetAllowHandler(handler func(event *types.DNSResponseEvent) error)

	// Statistics
	GetStatistics() interface{}
}

// TLSFilterInterface defines the interface for TLS socket filters
type TLSFilterInterface interface {
	// Lifecycle
	Start() error
	Stop()
	IsEnabled() bool

	// Event handling
	SetHandshakeHandler(handler func(event *types.TLSHandshakeEvent) error)
	SetCertificateHandler(handler func(event *types.TLSCertificateEvent) error)

	// Statistics
	GetStatistics() interface{}
}

// DHCPFilterInterface defines the interface for DHCP socket filters
type DHCPFilterInterface interface {
	// Lifecycle
	Start() error
	Stop()
	IsEnabled() bool

	// Event handling
	SetDiscoverHandler(handler func(event *types.DHCPDiscoverEvent) error)
	SetOfferHandler(handler func(event *types.DHCPOfferEvent) error)
	SetRequestHandler(handler func(event *types.DHCPRequestEvent) error)
	SetAckHandler(handler func(event *types.DHCPAckEvent) error)

	// Device discovery
	GetDiscoveredDevices() []types.DeviceInfo

	// Statistics
	GetStatistics() interface{}
}

// SocketFilterConfig defines the configuration interface for socket filters
type SocketFilterConfig interface {
	// Global settings
	IsEnabled() bool

	// Component configurations
	GetDNSFilterConfig() interface{}
	GetTLSFilterConfig() interface{}
	GetDHCPFilterConfig() interface{}

	// Integration settings
	ForwardToIPS() bool
	ForwardToLearning() bool
}

// SocketFilterStats defines aggregated statistics for all socket filters
type SocketFilterStats struct {
	// Timestamps
	LastUpdate time.Time `json:"last_update"`

	// Component statistics
	DNSStats      interface{} `json:"dns_stats,omitempty"`
	TLSStats      interface{} `json:"tls_stats,omitempty"`
	DHCPStats     interface{} `json:"dhcp_stats,omitempty"`

	// Aggregated metrics
	TotalEvents   uint64 `json:"total_events"`
	TotalBlocked  uint64 `json:"total_blocked"`
	TotalAllowed  uint64 `json:"total_allowed"`
	Errors        uint64 `json:"errors"`
}
