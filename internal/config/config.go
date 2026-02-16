// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// CurrentSchemaVersion defines the current schema version of the configuration.
const CurrentSchemaVersion = "1.1"

// Config is the top-level structure for the firewall configuration.
// It defines all aspects of the firewall including interfaces, zones, policies,
// NAT rules, services (DHCP, DNS), VPN integrations, and security features.
type Config struct {
	// Schema version for backward compatibility.
	// @enum: 1.0
	// @default: "1.0"
	// @example: "1.0"
	SchemaVersion string `hcl:"schema_version,optional" json:"schema_version,omitempty"`

	// Enable IP forwarding between interfaces (required for routing).
	// @default: false
	IPForwarding bool `hcl:"ip_forwarding,optional" json:"ip_forwarding,omitempty"`
	// Enable TCP MSS clamping to PMTU (recommended for VPNs).
	// @default: false
	MSSClamping bool `hcl:"mss_clamping,optional" json:"mss_clamping,omitempty"`
	// Enable hardware flow offloading for improved performance.
	// @default: false
	EnableFlowOffload bool           `hcl:"enable_flow_offload,optional" json:"enable_flow_offload,omitempty"`
	Interfaces        []Interface    `hcl:"interface,block" json:"interface,omitempty"`
	VRFs              []VRF          `hcl:"vrf,block" json:"vrf,omitempty"`
	Routes            []Route        `hcl:"route,block" json:"route,omitempty"`
	RoutingTables     []RoutingTable `hcl:"routing_table,block" json:"routing_table,omitempty"`
	PolicyRoutes      []PolicyRoute  `hcl:"policy_route,block" json:"policy_route,omitempty"`
	MarkRules         []MarkRule     `hcl:"mark_rule,block" json:"mark_rule,omitempty"`
	MultiWAN          *MultiWAN      `hcl:"multi_wan,block" json:"multi_wan,omitempty"`
	UplinkGroups      []UplinkGroup  `hcl:"uplink_group,block" json:"uplink_group,omitempty"`
	UIDRouting        []UIDRouting   `hcl:"uid_routing,block" json:"uid_routing,omitempty"`
	FRR               *FRRConfig     `hcl:"frr,block" json:"frr,omitempty"`
	Policies          []Policy       `hcl:"policy,block" json:"policy,omitempty"`
	NAT               []NATRule      `hcl:"nat,block" json:"nat,omitempty"`
	DHCP              *DHCPServer    `hcl:"dhcp,block" json:"dhcp,omitempty"`
	DNSServer         *DNSServer     `hcl:"dns_server,block" json:"dns_server,omitempty"` // Deprecated: use DNS
	DNS               *DNS           `hcl:"dns,block" json:"dns,omitempty"`               // New consolidated DNS config
	ThreatIntel       *ThreatIntel   `hcl:"threat_intel,block" json:"threat_intel,omitempty"`

	IPSets         []IPSet          `hcl:"ipset,block" json:"ipset,omitempty"`
	Zones          []Zone           `hcl:"zone,block" json:"zone,omitempty"`
	Scheduler      *SchedulerConfig `hcl:"scheduler,block" json:"scheduler,omitempty"`
	ScheduledRules []ScheduledRule  `hcl:"scheduled_rule,block" json:"scheduled_rule,omitempty"`

	// Per-interface settings (first-class)
	QoSPolicies []QoSPolicy           `hcl:"qos_policy,block" json:"qos_policy,omitempty"`
	Protections []InterfaceProtection `hcl:"protection,block" json:"protection,omitempty"`

	// Rule learning and notifications
	RuleLearning  *RuleLearningConfig  `hcl:"rule_learning,block" json:"rule_learning,omitempty"`
	AnomalyConfig *AnomalyConfig       `hcl:"anomaly_detection,block" json:"anomaly_detection,omitempty"`
	Notifications *NotificationsConfig `hcl:"notifications,block" json:"notifications,omitempty"`

	// State Replication configuration
	Replication *ReplicationConfig `hcl:"replication,block" json:"replication,omitempty"`

	// VPN integrations (Tailscale, WireGuard, etc.) for secure remote access
	VPN *VPNConfig `hcl:"vpn,block" json:"vpn,omitempty"`

	// API configuration
	API *APIConfig `hcl:"api,block" json:"api,omitempty"`

	// Web Server configuration (previously part of API)
	Web *WebConfig `hcl:"web,block" json:"web,omitempty"`

	// mDNS Reflector configuration
	MDNS *MDNSConfig `hcl:"mdns,block" json:"mdns,omitempty"`

	// UPnP IGD configuration
	UPnP *UPnPConfig `hcl:"upnp,block" json:"upnp,omitempty"`

	// NTP configuration
	NTP *NTPConfig `hcl:"ntp,block" json:"ntp,omitempty"`

	// Feature Flags
	Features *Features `hcl:"features,block" json:"features,omitempty"`

	// Syslog remote logging
	Syslog *SyslogConfig `hcl:"syslog,block" json:"syslog,omitempty"`

	// Dynamic DNS
	DDNS *DDNSConfig `hcl:"ddns,block" json:"ddns,omitempty"`

	// System tuning and settings
	System *SystemConfig `hcl:"system,block" json:"system,omitempty"`

	// Audit logging configuration
	Audit *AuditConfig `hcl:"audit,block" json:"audit,omitempty"`

	// GeoIP configuration for country-based filtering
	GeoIP *GeoIPConfig `hcl:"geoip,block" json:"geoip,omitempty"`

	// State Directory (overrides default)
	StateDir string `hcl:"state_dir,optional" json:"state_dir,omitempty"`

	// Cloud Management
	Cloud *CloudConfig `hcl:"cloud,block" json:"cloud,omitempty"`

	// Log Directory (overrides default /var/log/flywall)
	LogDir string `hcl:"log_dir,optional" json:"log_dir,omitempty"`

	// Scanner configuration
	Scanner *ScannerConfig `hcl:"scanner,block" json:"scanner,omitempty"`

	// SSH Server configuration
	SSH *SSHConfig `hcl:"ssh,block" json:"ssh,omitempty"`

	// eBPF configuration for high-performance packet processing
	EBPF *EBPFConfig `hcl:"ebpf,block" json:"ebpf,omitempty"`
}

// CloudConfig defines settings for the Flywall Cloud Control Plane
type CloudConfig struct {
	Enabled    bool   `hcl:"enabled,optional"`
	HubAddress string `hcl:"hub_address,optional"`
	DeviceID   string `hcl:"device_id,optional"`

	// Certificate paths (override defaults)
	CertFile string `hcl:"cert_file,optional"`
	KeyFile  string `hcl:"key_file,optional"`
	CAFile   string `hcl:"ca_file,optional"`

	// Local config priority field
	LocalPriority []string `hcl:"local_priority,optional"`
}
