// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package socket

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/ebpf/interfaces"
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/learning"
	"grimm.is/flywall/internal/logging"
)

type learningServiceInterface interface {
	Engine() *learning.Engine
	IsRunning() bool
	IngestPacket(learning.PacketInfo)
}

// Manager manages all socket filters
type Manager struct {
	// Configuration
	config *SocketFilterConfig

	// Socket filters
	dnsFilter       *DNSFilter
	queryLogger     *QueryLogger
	responseFilter  *ResponseFilter
	dhcpFilter      *DHCPFilter
	tlsFilter       *TLSFilter
	deviceDiscovery *DeviceDiscovery
	deviceDatabase  *DeviceDatabase
	learningService learningServiceInterface // Internal interface for forwarding
	alerts          *alerting.Engine         // Alerting engine for notifications
	blockHandler    func(ip net.IP, reason string) error // External handler for blocking IPs

	// State
	mutex   sync.RWMutex
	enabled bool

	// Statistics
	stats *ManagerStats

	// Logger
	logger *logging.Logger

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// SocketFilterConfig holds configuration for socket filters
type SocketFilterConfig struct {
	// Global settings
	Enabled bool `hcl:"enabled,optional"`

	// DNS filter configuration
	DNSFilter *DNSFilterConfig `hcl:"dns_filter,block"`

	// Query logger configuration
	QueryLogger *QueryLoggerConfig `hcl:"query_logger,block"`

	// Response filter configuration
	ResponseFilter *ResponseFilterConfig `hcl:"response_filter,block"`

	// DHCP filter configuration
	DHCPFilter *DHCPFilterConfig `hcl:"dhcp_filter,block"`

	// TLS filter configuration
	TLSFilter *TLSFilterConfig `hcl:"tls_filter,block"`

	// Device discovery configuration
	DeviceDiscovery *DeviceDiscoveryConfig `hcl:"device_discovery,block"`

	// Device database configuration
	DeviceDatabase *DeviceDatabaseConfig `hcl:"device_database,block"`

	// Integration settings
	ForwardToIPS      bool `hcl:"forward_to_ips,optional"`
	ForwardToLearning bool `hcl:"forward_to_learning,optional"`
}

// ManagerStats holds aggregated statistics
type ManagerStats struct {
	DNSFilterStats       interface{} `json:"dns_filter_stats,omitempty"`
	QueryLoggerStats     interface{} `json:"query_logger_stats,omitempty"`
	ResponseFilterStats  interface{} `json:"response_filter_stats,omitempty"`
	DHCPFilterStats      interface{} `json:"dhcp_filter_stats,omitempty"`
	TLSFilterStats       interface{} `json:"tls_filter_stats,omitempty"`
	DeviceDiscoveryStats interface{} `json:"device_discovery_stats,omitempty"`
	DeviceDatabaseStats  interface{} `json:"device_database_stats,omitempty"`
	LastUpdate           time.Time   `json:"last_update"`
}

// DefaultSocketFilterConfig returns default configuration
func DefaultSocketFilterConfig() *SocketFilterConfig {
	return &SocketFilterConfig{
		Enabled:           false,
		DNSFilter:         DefaultDNSFilterConfig(),
		QueryLogger:       DefaultQueryLoggerConfig(),
		ResponseFilter:    DefaultResponseFilterConfig(),
		DHCPFilter:        DefaultDHCPFilterConfig(),
		TLSFilter:         DefaultTLSFilterConfig(),
		DeviceDiscovery:   DefaultDeviceDiscoveryConfig(),
		DeviceDatabase:    DefaultDeviceDatabaseConfig(),
		ForwardToIPS:      true,
		ForwardToLearning: true,
	}
}

// NewManager creates a new socket filter manager
func NewManager(logger *logging.Logger, config *SocketFilterConfig, alerts *alerting.Engine) *Manager {
	if config == nil {
		config = DefaultSocketFilterConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config: config,
		stats: &ManagerStats{
			LastUpdate: time.Now(),
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
		alerts: alerts,
	}

	return m
}

// Start starts the socket filter manager
func (m *Manager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.config.Enabled {
		m.logger.Info("Socket filter manager disabled")
		return nil
	}

	m.logger.Info("Starting socket filter manager")

	// Initialize DNS filter
	if m.config.DNSFilter != nil {
		m.dnsFilter = NewDNSFilter(m.logger, m.config.DNSFilter)

		// Set up event handlers
		m.dnsFilter.SetQueryHandler(m.handleDNSQuery)
		m.dnsFilter.SetResponseHandler(m.handleDNSResponse)

		if err := m.dnsFilter.Start(); err != nil {
			m.logger.Error("Failed to start DNS filter", "error", err)
			return err
		}
	}

	// Initialize query logger
	if m.config.QueryLogger != nil {
		m.queryLogger = NewQueryLogger(m.logger, m.config.QueryLogger)
		if err := m.queryLogger.Start(); err != nil {
			m.logger.Error("Failed to start query logger", "error", err)
			return err
		}
	}

	// Initialize response filter
	if m.config.ResponseFilter != nil {
		m.responseFilter = NewResponseFilter(m.logger, m.config.ResponseFilter)

		// Set up event handlers
		m.responseFilter.SetBlockHandler(m.handleBlockedResponse)
		m.responseFilter.SetAllowHandler(m.handleAllowedResponse)

		if err := m.responseFilter.Start(); err != nil {
			m.logger.Error("Failed to start response filter", "error", err)
			return err
		}
	}

	// Initialize device database first (needed by device discovery)
	if m.config.DeviceDatabase != nil {
		m.deviceDatabase = NewDeviceDatabase(m.logger, m.config.DeviceDatabase)

		if err := m.deviceDatabase.Start(); err != nil {
			m.logger.Error("Failed to start device database", "error", err)
			return err
		}
	}

	// Initialize DHCP filter
	if m.config.DHCPFilter != nil {
		m.dhcpFilter = NewDHCPFilter(m.logger, m.config.DHCPFilter)

		// Set up event handlers
		m.dhcpFilter.SetDiscoverHandler(m.handleDHCPDiscover)
		m.dhcpFilter.SetOfferHandler(m.handleDHCPOffer)
		m.dhcpFilter.SetRequestHandler(m.handleDHCPRequest)
		m.dhcpFilter.SetAckHandler(m.handleDHCPAck)

		if err := m.dhcpFilter.Start(); err != nil {
			m.logger.Error("Failed to start DHCP filter", "error", err)
			return err
		}
	}

	// Initialize TLS filter
	if m.config.TLSFilter != nil {
		m.tlsFilter = NewTLSFilter(m.logger, m.config.TLSFilter)

		// Set up event handlers
		m.tlsFilter.SetHandshakeHandler(m.handleTLSHandshake)

		if err := m.tlsFilter.Start(); err != nil {
			m.logger.Error("Failed to start TLS filter", "error", err)
			return err
		}
	}

	// Initialize device discovery
	if m.config.DeviceDiscovery != nil {
		m.deviceDiscovery = NewDeviceDiscovery(m.logger, m.config.DeviceDiscovery)

		// Set up event handlers
		m.deviceDiscovery.SetNewDeviceHandler(m.handleNewDevice)
		m.deviceDiscovery.SetDeviceUpdateHandler(m.handleDeviceUpdate)

		if err := m.deviceDiscovery.Start(); err != nil {
			m.logger.Error("Failed to start device discovery", "error", err)
			return err
		}
	}

	m.enabled = true
	m.logger.Info("Socket filter manager started")

	return nil
}

// Stop stops the socket filter manager
func (m *Manager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.enabled {
		return
	}

	m.logger.Info("Stopping socket filter manager")

	// Cancel context
	m.cancel()

	// Stop components
	if m.dnsFilter != nil {
		m.dnsFilter.Stop()
	}
	if m.queryLogger != nil {
		m.queryLogger.Stop()
	}
	if m.responseFilter != nil {
		m.responseFilter.Stop()
	}
	if m.dhcpFilter != nil {
		m.dhcpFilter.Stop()
	}
	if m.tlsFilter != nil {
		m.tlsFilter.Stop()
	}
	if m.deviceDiscovery != nil {
		m.deviceDiscovery.Stop()
	}
	if m.deviceDatabase != nil {
		m.deviceDatabase.Stop()
	}

	m.enabled = false
	m.logger.Info("Socket filter manager stopped")
}

// handleTLSHandshake handles TLS handshake events
func (m *Manager) handleTLSHandshake(event *types.TLSHandshakeEvent) error {
	m.logger.Debug("TLS handshake observed",
		"sni", event.SNI,
		"version", event.Version,
		"src", event.SourceIP)

	// Forward to learning engine if enabled
	if m.config.ForwardToLearning {
		m.mutex.RLock()
		ls := m.learningService
		m.mutex.RUnlock()

		if ls != nil && ls.IsRunning() {
			pkt := learning.PacketInfo{
				SrcIP:    event.SourceIP.String(),
				DstIP:    event.DestIP.String(),
				DstPort:  int(event.DestPort),
				Protocol: "tcp",
				Payload:  event.SNI,
			}
			ls.IngestPacket(pkt)
		}

		m.logger.Debug("Forwarding TLS handshake to learning engine",
			"sni", event.SNI)
	}

	return nil
}

// handleDNSQuery handles DNS query events
func (m *Manager) handleDNSQuery(event *types.DNSQueryEvent) error {
	// Log query
	if m.queryLogger != nil {
		m.queryLogger.LogQuery(event)
	}

	// Forward to learning engine if enabled
	if m.config.ForwardToLearning {
		m.mutex.RLock()
		ls := m.learningService
		m.mutex.RUnlock()

		if ls != nil && ls.IsRunning() {
			pkt := learning.PacketInfo{
				SrcIP:    event.SourceIP.String(),
				DstIP:    event.DestIP.String(),
				DstPort:  int(event.DestPort),
				Protocol: "udp",
				Payload:  event.Domain,
			}
			ls.IngestPacket(pkt)
		}

		m.logger.Debug("Forwarding DNS query to learning engine",
			"domain", event.Domain,
			"query_type", event.QueryType)
	}

	return nil
}

// handleDNSResponse handles DNS response events
func (m *Manager) handleDNSResponse(event *types.DNSResponseEvent) error {
	// Filter response
	var blocked bool
	var reason string
	if m.responseFilter != nil {
		allowed, r := m.responseFilter.FilterResponse(event)
		blocked = !allowed
		reason = r

		if blocked {
			m.logger.Debug("DNS response blocked",
				"domain", event.Domain,
				"reason", reason)

			// Forward to IPS if enabled
			if m.config.ForwardToIPS {
				for _, ans := range event.Answers {
					if ans.Type == types.DNSTypeA || ans.Type == types.DNSTypeAAAA {
						ip := net.ParseIP(ans.Data)
						if ip != nil {
							m.BlockIP(ip, fmt.Sprintf("DNS blocklist: %s (%s)", event.Domain, reason))
						}
					}
				}
			}
		}
	}

	// Log response
	if m.queryLogger != nil {
		m.queryLogger.LogResponse(event, blocked, reason)
	}

	// Forward to learning engine if enabled
	if m.config.ForwardToLearning {
		m.mutex.RLock()
		ls := m.learningService
		m.mutex.RUnlock()

		if ls != nil && ls.IsRunning() {
			engine := ls.Engine()
			if engine != nil {
				for _, ans := range event.Answers {
					if ans.Type == types.DNSTypeA {
						ip := net.ParseIP(ans.Data)
						if ip != nil {
							engine.HandleDNSResponse(event.Domain, ip, ans.TTL)
						}
					}
				}
			}
		}

		m.logger.Debug("Forwarding DNS response to learning engine",
			"domain", event.Domain,
			"response_code", event.ResponseCode)
	}

	return nil
}

// handleBlockedResponse handles blocked DNS responses
func (m *Manager) handleBlockedResponse(event *types.DNSResponseEvent, reason string) error {
	m.logger.Info("DNS response blocked",
		"domain", event.Domain,
		"response_code", event.ResponseCode,
		"reason", reason)

	// Send alert
	if m.alerts != nil {
		m.alerts.Trigger(alerting.AlertEvent{
			ID:        fmt.Sprintf("dns-blocked-%s", event.Domain),
			RuleName:  "DNS Domain Blocked",
			Severity:  alerting.LevelWarning,
			Message:   fmt.Sprintf("DNS response for %s was blocked: %s", event.Domain, reason),
			Timestamp: time.Now(),
		})
	}

	if m.config.ForwardToIPS {
		for _, ans := range event.Answers {
			if ans.Type == types.DNSTypeA || ans.Type == types.DNSTypeAAAA {
				ip := net.ParseIP(ans.Data)
				if ip != nil {
					m.BlockIP(ip, fmt.Sprintf("Blocked DNS response: %s (%s)", event.Domain, reason))
				}
			}
		}
	}

	return nil
}

// handleAllowedResponse handles allowed DNS responses
func (m *Manager) handleAllowedResponse(event *types.DNSResponseEvent) error {
	m.logger.Debug("DNS response allowed",
		"domain", event.Domain,
		"response_code", event.ResponseCode,
		"answer_count", event.AnswerCount)

	return nil
}

// handleDHCPDiscover handles DHCP discover events
func (m *Manager) handleDHCPDiscover(event *types.DHCPDiscoverEvent) error {
	m.logger.Debug("DHCP discover observed",
		"mac", event.MACAddress,
		"hostname", event.HostName,
		"vendor_class", event.VendorClass)

	// Process device discovery
	if m.deviceDiscovery != nil {
		if err := m.deviceDiscovery.ProcessDHCPDiscover(event); err != nil {
			m.logger.Error("Failed to process DHCP discover in device discovery", "error", err)
		}
	}

	// Forward to learning engine if enabled
	if m.config.ForwardToLearning {
		m.mutex.RLock()
		ls := m.learningService
		m.mutex.RUnlock()

		if ls != nil && ls.IsRunning() {
			pkt := learning.PacketInfo{
				SrcMAC:      event.MACAddress,
				SrcIP:       event.SourceIP.String(),
				SrcHostname: event.HostName,
				DstIP:       event.DestIP.String(),
				DstPort:     int(event.DestPort),
				Protocol:    "udp",
			}
			ls.IngestPacket(pkt)
		}

		m.logger.Debug("Forwarding DHCP discover to learning engine",
			"mac", event.MACAddress,
			"hostname", event.HostName)
	}

	return nil
}

// handleDHCPOffer handles DHCP offer events
func (m *Manager) handleDHCPOffer(event *types.DHCPOfferEvent) error {
	m.logger.Debug("DHCP offer observed",
		"your_ip", event.YourIP,
		"server_ip", event.ServerIP,
		"lease_time", event.LeaseTime)

	// Forward to learning engine if enabled
	if m.config.ForwardToLearning {
		m.mutex.RLock()
		ls := m.learningService
		m.mutex.RUnlock()

		if ls != nil && ls.IsRunning() {
			pkt := learning.PacketInfo{
				SrcIP:    event.SourceIP.String(),
				DstIP:    event.DestIP.String(),
				DstPort:  int(event.DestPort),
				Protocol: "udp",
			}
			ls.IngestPacket(pkt)
		}

		m.logger.Debug("Forwarding DHCP offer to learning engine",
			"your_ip", event.YourIP,
			"server_ip", event.ServerIP)
	}

	return nil
}

// handleDHCPRequest handles DHCP request events
func (m *Manager) handleDHCPRequest(event *types.DHCPRequestEvent) error {
	m.logger.Debug("DHCP request observed",
		"mac", event.MACAddress,
		"requested_ip", event.RequestedIP,
		"server_ip", event.ServerIP)

	// Forward to learning engine if enabled
	if m.config.ForwardToLearning {
		m.mutex.RLock()
		ls := m.learningService
		m.mutex.RUnlock()

		if ls != nil && ls.IsRunning() {
			pkt := learning.PacketInfo{
				SrcMAC:      event.MACAddress,
				SrcIP:       event.SourceIP.String(),
				SrcHostname: event.HostName,
				DstIP:       event.DestIP.String(),
				DstPort:     int(event.DestPort),
				Protocol:    "udp",
			}
			ls.IngestPacket(pkt)
		}

		m.logger.Debug("Forwarding DHCP request to learning engine",
			"mac", event.MACAddress,
			"requested_ip", event.RequestedIP)
	}

	return nil
}

// handleDHCPAck handles DHCP acknowledge events
func (m *Manager) handleDHCPAck(event *types.DHCPAckEvent) error {
	m.logger.Debug("DHCP ACK observed",
		"your_ip", event.YourIP,
		"server_ip", event.ServerIP,
		"lease_time", event.LeaseTime)

	// Process device discovery for IP assignment
	if m.deviceDiscovery != nil {
		if err := m.deviceDiscovery.ProcessDHCPAck(event); err != nil {
			m.logger.Error("Failed to process DHCP ACK in device discovery", "error", err)
		}
	}

	// Forward to learning engine if enabled
	if m.config.ForwardToLearning {
		m.mutex.RLock()
		ls := m.learningService
		m.mutex.RUnlock()

		if ls != nil && ls.IsRunning() {
			pkt := learning.PacketInfo{
				SrcIP:    event.SourceIP.String(),
				DstIP:    event.DestIP.String(),
				DstPort:  int(event.DestPort),
				Protocol: "udp",
			}
			ls.IngestPacket(pkt)
		}

		m.logger.Debug("Forwarding DHCP ACK to learning engine",
			"your_ip", event.YourIP,
			"server_ip", event.ServerIP)
	}

	return nil
}

// handleNewDevice handles new device discoveries
func (m *Manager) handleNewDevice(device *types.DeviceInfo) error {
	m.logger.Info("New device discovered",
		"mac", device.MACAddress,
		"hostname", device.HostName,
		"vendor", device.Vendor,
		"type", device.DeviceType)

	// Send alert
	if m.alerts != nil {
		m.alerts.Trigger(alerting.AlertEvent{
			ID:        fmt.Sprintf("new-device-%s", device.MACAddress),
			RuleName:  "New Device Discovered",
			Severity:  alerting.LevelInfo,
			Message:   fmt.Sprintf("New device seen on network: %s (%s) [%s]", device.MACAddress, device.HostName, device.Vendor),
			Timestamp: time.Now(),
		})
	}

	// Store in device database
	if m.deviceDatabase != nil {
		if err := m.deviceDatabase.StoreDevice(device); err != nil {
			m.logger.Error("Failed to store new device in database", "error", err)
		}
	}

	// Forward to IPS if enabled
	if m.config.ForwardToIPS {
		if device.DeviceType == "Rogue" {
			if device.IPAddress != nil {
				m.BlockIP(device.IPAddress, "Rogue DHCP server detected")
			}
		}
	}

	return nil
}

// handleDeviceUpdate handles device updates
func (m *Manager) handleDeviceUpdate(device *types.DeviceInfo) error {
	m.logger.Debug("Device updated",
		"mac", device.MACAddress,
		"hostname", device.HostName,
		"ip", device.IPAddress)

	// Update in device database
	if m.deviceDatabase != nil {
		if err := m.deviceDatabase.StoreDevice(device); err != nil {
			m.logger.Error("Failed to update device in database", "error", err)
		}
	}

	return nil
}

// GetStatistics returns aggregated statistics
func (m *Manager) GetStatistics() interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := &ManagerStats{
		LastUpdate: time.Now(),
	}

	// Collect DNS filter stats
	if m.dnsFilter != nil {
		if dnsStats := m.dnsFilter.GetStatistics(); dnsStats != nil {
			stats.DNSFilterStats = dnsStats
		}
	}

	// Collect query logger stats
	if m.queryLogger != nil {
		if qlStats := m.queryLogger.GetStatistics(); qlStats != nil {
			stats.QueryLoggerStats = qlStats
		}
	}

	// Collect response filter stats
	if m.responseFilter != nil {
		if rfStats := m.responseFilter.GetStatistics(); rfStats != nil {
			stats.ResponseFilterStats = rfStats
		}
	}

	// Collect DHCP filter stats
	if m.dhcpFilter != nil {
		if dhcpStats := m.dhcpFilter.GetStatistics(); dhcpStats != nil {
			stats.DHCPFilterStats = dhcpStats
		}
	}

	// Collect TLS filter stats
	if m.tlsFilter != nil {
		if tlsStats := m.tlsFilter.GetStatistics(); tlsStats != nil {
			stats.TLSFilterStats = tlsStats
		}
	}

	// Collect device discovery stats
	if m.deviceDiscovery != nil {
		if ddStats := m.deviceDiscovery.GetStatistics(); ddStats != nil {
			stats.DeviceDiscoveryStats = ddStats
		}
	}

	// Collect device database stats
	if m.deviceDatabase != nil {
		if dbStats := m.deviceDatabase.GetStatistics(); dbStats != nil {
			stats.DeviceDatabaseStats = dbStats
		}
	}

	return stats
}

// IsEnabled returns whether the manager is enabled
func (m *Manager) IsEnabled() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.enabled
}

// GetDNSFilter returns the DNS filter
func (m *Manager) GetDNSFilter() interfaces.DNSFilterInterface {
	return m.dnsFilter
}

// GetQueryLogger returns the query logger
func (m *Manager) GetQueryLogger() interfaces.QueryLoggerInterface {
	return m.queryLogger
}

// GetResponseFilter returns the response filter
func (m *Manager) GetResponseFilter() interfaces.ResponseFilterInterface {
	return m.responseFilter
}

// GetDHCPFilter returns the DHCP filter
func (m *Manager) GetDHCPFilter() *DHCPFilter {
	return m.dhcpFilter
}

// GetTLSFilter returns the TLS filter
func (m *Manager) GetTLSFilter() *TLSFilter {
	return m.tlsFilter
}

// GetDeviceDiscovery returns the device discovery module
func (m *Manager) GetDeviceDiscovery() *DeviceDiscovery {
	return m.deviceDiscovery
}

// GetDeviceDatabase returns the device database
func (m *Manager) GetDeviceDatabase() *DeviceDatabase {
	return m.deviceDatabase
}

func (m *Manager) SetLearningService(svc learningServiceInterface) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.learningService = svc
}

func (m *Manager) SetBlockHandler(handler func(ip net.IP, reason string) error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.blockHandler = handler
}

func (m *Manager) BlockIP(ip net.IP, reason string) error {
	m.mutex.RLock()
	handler := m.blockHandler
	m.mutex.RUnlock()

	if handler != nil {
		return handler(ip, reason)
	}

	m.logger.Info("Blocking IP (no handler)", "ip", ip, "reason", reason)
	return nil
}

// Ensure Manager implements interfaces.SocketFilterManager
var _ interfaces.SocketFilterManager = (*Manager)(nil)
