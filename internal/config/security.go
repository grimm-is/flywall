// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"fmt"
	"strconv"
	"strings"
)

// VPNConfig configures VPN integrations.
// Supports multiple connections per provider, each with its own zone or combined.
type VPNConfig struct {
	// Tailscale/Headscale connections (multiple allowed)
	Tailscale []TailscaleConfig `hcl:"tailscale,block"`

	// WireGuard connections (multiple allowed)
	WireGuard []WireGuardConfig `hcl:"wireguard,block"`

	// 6to4 Tunnels (multiple allowed, usually one)
	SixToFour []SixToFourConfig `hcl:"six_to_four,block"`

	// Interface prefix matching for zones (like firehol's "wg+" syntax)
	// Maps interface prefix to zone name
	// Example: {"wg": "vpn", "tailscale": "tailscale"} means wg0, wg1 -> vpn zone
	InterfacePrefixZones map[string]string `hcl:"interface_prefix_zones,optional"`
}

// GetAllInterfaces returns all configured VPN interface names.
func (c *VPNConfig) GetAllInterfaces() []string {
	var interfaces []string
	for _, ts := range c.Tailscale {
		if ts.Enabled && ts.Interface != "" {
			interfaces = append(interfaces, ts.Interface)
		}
	}
	for _, wg := range c.WireGuard {
		if wg.Enabled && wg.Interface != "" {
			interfaces = append(interfaces, wg.Interface)
		}
	}
	return interfaces
}

// GetManagementInterfaces returns VPN interfaces that should bypass firewall.
func (c *VPNConfig) GetManagementInterfaces() []string {
	var interfaces []string
	for _, ts := range c.Tailscale {
		if ts.Enabled && ts.ManagementAccess {
			iface := ts.Interface
			if iface == "" {
				iface = "tailscale0"
			}
			interfaces = append(interfaces, iface)
		}
	}
	for _, wg := range c.WireGuard {
		if wg.Enabled && wg.ManagementAccess {
			iface := wg.Interface
			if iface == "" {
				iface = "wg0"
			}
			interfaces = append(interfaces, iface)
		}
	}
	return interfaces
}

// GetInterfaceZone returns the zone for a given interface, checking both
// explicit interface->zone mappings and prefix matching.
func (c *VPNConfig) GetInterfaceZone(iface string) string {
	// Check explicit Tailscale configs
	for _, ts := range c.Tailscale {
		if ts.Interface == iface && ts.Zone != "" {
			return ts.Zone
		}
	}
	// Check explicit WireGuard configs
	for _, wg := range c.WireGuard {
		if wg.Interface == iface && wg.Zone != "" {
			return wg.Zone
		}
	}
	// Check prefix matching (like firehol's "wg+" syntax)
	for prefix, zone := range c.InterfacePrefixZones {
		if len(iface) > len(prefix) && iface[:len(prefix)] == prefix {
			return zone
		}
	}
	return ""
}

// TailscaleConfig configures a Tailscale/Headscale connection.
type TailscaleConfig struct {
	// Connection name (label for multiple connections)
	Name string `hcl:"name,label"`

	// Enable this Tailscale connection
	Enabled bool `hcl:"enabled,optional"`

	// Interface name (default: tailscale0, or tailscale1, etc. for multiple)
	Interface string `hcl:"interface,optional"`

	// Auth key for unattended setup (or use AuthKeyEnv)
	AuthKey SecureString `hcl:"auth_key,optional"`

	// Environment variable containing auth key
	AuthKeyEnv string `hcl:"auth_key_env,optional"`

	// Control server URL (for Headscale)
	ControlURL string `hcl:"control_url,optional"`

	// Always allow management access via Tailscale (lockout protection)
	// This inserts accept rules BEFORE all other firewall rules
	ManagementAccess bool `hcl:"management_access,optional"`

	// Zone name for this interface (default: tailscale)
	// Use same zone name across multiple connections to combine them
	Zone string `hcl:"zone,optional"`

	// Routes to advertise to Tailscale network
	AdvertiseRoutes []string `hcl:"advertise_routes,optional"`

	// Accept routes from other Tailscale nodes
	AcceptRoutes bool `hcl:"accept_routes,optional"`

	// Advertise this node as an exit node
	AdvertiseExitNode bool `hcl:"advertise_exit_node,optional"`

	// Use a specific exit node (Tailscale IP or hostname)
	ExitNode string `hcl:"exit_node,optional"`
}

// WireGuardConfig configures a WireGuard VPN connection.
type WireGuardConfig struct {
	// Connection name (label for multiple connections)
	Name string `hcl:"name,label" json:"name"`

	// Enable this WireGuard connection
	Enabled bool `hcl:"enabled,optional" json:"enabled"`

	// Interface name (default: wg0, or wg1, etc. for multiple)
	Interface string `hcl:"interface,optional" json:"interface"`

	// Always allow management access via WireGuard (lockout protection)
	ManagementAccess bool `hcl:"management_access,optional" json:"management_access"`

	// Zone name for this interface (default: vpn)
	// Use same zone name across multiple connections to combine them
	Zone string `hcl:"zone,optional" json:"zone"`

	// Private key (or use PrivateKeyFile)
	PrivateKey SecureString `hcl:"private_key,optional" json:"private_key,omitempty"`

	// Path to private key file
	PrivateKeyFile string `hcl:"private_key_file,optional" json:"private_key_file,omitempty"`

	// Listen port (default: 51820)
	ListenPort int `hcl:"listen_port,optional" json:"listen_port"`

	// Interface addresses
	Address []string `hcl:"address,optional" json:"address"`

	// DNS servers to use when connected
	DNS []string `hcl:"dns,optional" json:"dns"`

	// MTU (default: 1420)
	MTU int `hcl:"mtu,optional" json:"mtu"`

	// Peer configurations
	Peers []WireGuardPeerConfig `hcl:"peer,block" json:"peer,omitempty"`

	// Firewall Mark (fwmark) for routing
	FWMark int `hcl:"fwmark,optional" json:"fwmark"`

	// Routing Table (default: auto)
	// If set to "off" or "auto", behaves effectively like standard WG.
	// If set to a table ID/name, routes are added to that table.
	Table string `hcl:"table,optional" json:"table"`

	// Hooks
	PostUp   []string `hcl:"post_up,optional" json:"post_up,omitempty"`
	PostDown []string `hcl:"post_down,optional" json:"post_down,omitempty"`
}

// WireGuardPeerConfig configures a WireGuard peer.
type WireGuardPeerConfig struct {
	// Peer name (label)
	Name string `hcl:"name,label" json:"name"`

	// Peer's public key
	PublicKey string `hcl:"public_key" json:"public_key"`

	// Optional preshared key for additional security
	PresharedKey SecureString `hcl:"preshared_key,optional" json:"preshared_key,omitempty"`

	// Peer's endpoint (host:port)
	Endpoint string `hcl:"endpoint,optional" json:"endpoint"`

	// Allowed IP ranges for this peer
	AllowedIPs []string `hcl:"allowed_ips" json:"allowed_ips"`

	// Keepalive interval in seconds (useful for NAT traversal)
	PersistentKeepalive int `hcl:"persistent_keepalive,optional" json:"persistent_keepalive"`
}

// ThreatIntel configures threat intelligence feeds.
type ThreatIntel struct {
	Enabled  bool           `hcl:"enabled,optional"`
	Interval string         `hcl:"interval,optional"` // e.g. "1h"
	Sources  []ThreatSource `hcl:"source,block" json:"source,omitempty"`
}

type ThreatSource struct {
	Name         string       `hcl:"name,label"`
	URL          string       `hcl:"url"`
	Format       string       `hcl:"format,optional"` // "taxii", "text", "json"
	CollectionID string       `hcl:"collection_id,optional"`
	Username     string       `hcl:"username,optional"`
	Password     SecureString `hcl:"password,optional"`
}

// IPSet defines a named set of IPs/networks for use in firewall rules.
type IPSet struct {
	Name        string   `hcl:"name,label"`
	Description string   `hcl:"description,optional"`
	Type        string   `hcl:"type,optional"` // ipv4_addr (default), ipv6_addr, inet_service, or dns
	Entries     []string `hcl:"entries,optional"`

	// Domains for dynamic resolution (only for type="dns")
	Domains []string `hcl:"domains,optional"`

	// Refresh interval for DNS resolution (e.g., "5m", "1h") - only for type="dns"
	RefreshInterval string `hcl:"refresh_interval,optional"`

	// Optimization: Pre-allocated size for dynamic sets to prevent resizing (suggested: 65535)
	Size int `hcl:"size,optional"`

	// Managed List (replaces FireHOLList)
	ManagedList string `hcl:"managed_list,optional"` // e.g., "firehol_level1"

	// Deprecated: Use ManagedList instead
	FireHOLList   string `hcl:"firehol_list,optional"`
	URL           string `hcl:"url,optional"`             // Custom URL for IP list
	RefreshHours  int    `hcl:"refresh_hours,optional"`   // How often to refresh (default: 24)
	AutoUpdate    bool   `hcl:"auto_update,optional"`     // Enable automatic updates
	Action        string `hcl:"action,optional"`          // drop, reject, log (for auto-generated rules)
	ApplyTo       string `hcl:"apply_to,optional"`        // input, forward, both (for auto-generated rules)
	MatchOnSource bool   `hcl:"match_on_source,optional"` // Match source IP (default: true)
	MatchOnDest   bool   `hcl:"match_on_dest,optional"`   // Match destination IP
}

// InterfaceProtection defines security protection settings for an interface.
type InterfaceProtection struct {
	Name      string `hcl:"name,label"`
	Interface string `hcl:"interface"` // Interface name or "*" for all
	Enabled   bool   `hcl:"enabled,optional" default:"true"`

	// AntiSpoofing drops packets with spoofed source IPs (recommended for WAN)
	AntiSpoofing bool `hcl:"anti_spoofing,optional"`
	// BogonFiltering drops packets from reserved/invalid IP ranges
	BogonFiltering bool `hcl:"bogon_filtering,optional"`
	// PrivateFiltering drops packets from private IP ranges on WAN (RFC1918)
	PrivateFiltering bool `hcl:"private_filtering,optional"`
	// InvalidPackets drops malformed/invalid packets
	InvalidPackets bool `hcl:"invalid_packets,optional"`

	// SynFloodProtection limits SYN packets to prevent SYN floods
	SynFloodProtection bool `hcl:"syn_flood_protection,optional"`
	SynFloodRate       int  `hcl:"syn_flood_rate,optional"`  // packets/sec (default: 25)
	SynFloodBurst      int  `hcl:"syn_flood_burst,optional"` // burst allowance (default: 50)

	// ICMPRateLimit limits ICMP packets to prevent ping floods
	ICMPRateLimit bool `hcl:"icmp_rate_limit,optional"`
	ICMPRate      int  `hcl:"icmp_rate,optional"`  // packets/sec (default: 10)
	ICMPBurst     int  `hcl:"icmp_burst,optional"` // burst (default: 20)

	// NewConnRateLimit limits new connections per second
	NewConnRateLimit bool `hcl:"new_conn_rate_limit,optional"`
	NewConnRate      int  `hcl:"new_conn_rate,optional"`  // per second (default: 100)
	NewConnBurst     int  `hcl:"new_conn_burst,optional"` // burst (default: 200)

	// PortScanProtection detects and blocks port scanning
	PortScanProtection bool `hcl:"port_scan_protection,optional"`
	PortScanThreshold  int  `hcl:"port_scan_threshold,optional"` // ports/sec (default: 10)

	// GeoBlocking blocks traffic from specific countries (requires GeoIP database)
	GeoBlocking      bool     `hcl:"geo_blocking,optional"`
	BlockedCountries []string `hcl:"blocked_countries,optional"` // ISO country codes
	AllowedCountries []string `hcl:"allowed_countries,optional"` // If set, only these allowed
}

// ProtectionConfig is an alias for InterfaceProtection for backwards compatibility.
type ProtectionConfig = InterfaceProtection

// Zone defines a network security zone.
// Zones can match traffic by interface, source/destination IP, VLAN, or combinations.
// Simple zones use top-level fields, complex zones use match blocks.
type Zone struct {
	Name        string `hcl:"name,label"`
	Color       string `hcl:"color,optional"`
	Description string `hcl:"description,optional"`

	// Simple match criteria (use for single-interface zones)
	// These are effectively a single implicit match rule
	// Interface can be exact ("eth0") or prefix with + or * suffix ("wg+" or "wg*" matches wg0, wg1...)
	Interface string `hcl:"interface,optional"`
	Src       string `hcl:"src,optional"`  // Source IP/network (e.g., "192.168.1.0/24")
	Dst       string `hcl:"dst,optional"`  // Destination IP/network
	VLAN      int    `hcl:"vlan,optional"` // VLAN tag

	// Complex match criteria (OR logic between matches, AND logic within each match)
	// Global fields above apply to ALL matches as defaults
	Matches []RuleMatch `hcl:"match,block" json:"match,omitempty"`

	// Legacy fields (kept for backwards compat)
	IPSets   []string `hcl:"ipsets,optional"`   // IPSet names for IP-based membership
	Networks []string `hcl:"networks,optional"` // Direct CIDR ranges

	// Zone behavior
	// Action for intra-zone traffic: "accept", "drop", "reject" (default: accept)
	Action string `hcl:"action,optional"`

	// External marks this as an external/WAN zone (used for auto-masquerade detection)
	// If not set, detected from: DHCP client enabled, non-RFC1918 address, or "wan"/"external" in name
	External *bool `hcl:"external,optional"`

	// Services provided TO this zone (firewall auto-generates rules)
	// These define what the firewall offers to clients in this zone
	Services *ZoneServices `hcl:"services,block"`

	// Management access FROM this zone to the firewall
	Management *ZoneManagement `hcl:"management,block"`

	// IP assignment for simple zones (shorthand - assigns to the interface)
	IPv4 []string `hcl:"ipv4,optional"`
	IPv6 []string `hcl:"ipv6,optional"`
	DHCP bool     `hcl:"dhcp,optional"` // Use DHCP client on this interface
}

// RuleMatch defines criteria for zone membership and policy rules.
// Multiple criteria within a match are ANDed together.
type RuleMatch struct {
	// Interface can be exact ("eth0") or prefix with + or * suffix ("wg+" matches wg0, wg1...)
	Interface string `hcl:"interface,optional"`
	Src       string `hcl:"src,optional"`
	Dst       string `hcl:"dst,optional"`
	VLAN      int    `hcl:"vlan,optional"`

	// Advanced matching
	Protocol     string `hcl:"protocol,optional"`
	Mac          string `hcl:"mac,optional"`
	DSCP         string `hcl:"dscp,optional"` // value, class, or classid
	Mark         string `hcl:"mark,optional"`
	TOS          int    `hcl:"tos,optional"`
	InterfaceOut string `hcl:"out_interface,optional"`
	PhysIn       string `hcl:"phys_in,optional"`
	PhysOut      string `hcl:"phys_out,optional"`
}

// ZoneServices defines which firewall services are available to a zone.
// The firewall automatically generates accept rules for enabled services.
type ZoneServices struct {
	// Network services
	DHCP bool `hcl:"dhcp,optional"` // Allow DHCP requests (udp/67-68)
	DNS  bool `hcl:"dns,optional"`  // Allow DNS queries (udp/53, tcp/53)
	NTP  bool `hcl:"ntp,optional"`  // Allow NTP sync (udp/123)

	// Captive portal / guest access
	CaptivePortal bool `hcl:"captive_portal,optional"` // Redirect HTTP to portal

	// Custom service ports (auto-allow)
	CustomPorts []ZoneServicePort `hcl:"port,block"`
}

// ZoneServicePort defines a custom service port to allow from a zone.
type ZoneServicePort struct {
	Name     string `hcl:"name,label"`
	Protocol string `hcl:"protocol"`          // tcp, udp
	Port     int    `hcl:"port"`              // Port number
	EndPort  int    `hcl:"port_end,optional"` // For port ranges
}

// ZoneManagement defines what management access is allowed from a zone.
type ZoneManagement struct {
	WebUI  bool `hcl:"web_ui,optional"` // Legacy: Allow Web UI access (tcp/80, tcp/443) -> Use Web
	Web    bool `hcl:"web,optional"`    // Allow Web UI access (tcp/80, tcp/443)
	SSH    bool `hcl:"ssh,optional"`    // Allow SSH access (tcp/22)
	API    bool `hcl:"api,optional"`    // Allow API access (used for L7 filtering, implies HTTPS access)
	ICMP   bool `hcl:"icmp,optional"`   // Allow ping to firewall
	SNMP   bool `hcl:"snmp,optional"`   // Allow SNMP queries (udp/161)
	Syslog bool `hcl:"syslog,optional"` // Allow syslog sending (udp/514)
}

// RuleLearningConfig configures the learning engine.
type RuleLearningConfig struct {
	Enabled       bool     `hcl:"enabled,optional" json:"enabled"`
	LogGroup      int      `hcl:"log_group,optional" json:"log_group"`                       // nflog group (default: 100)
	RateLimit     string   `hcl:"rate_limit,optional" json:"rate_limit,omitempty"`           // e.g., "10/minute"
	AutoApprove   bool     `hcl:"auto_approve,optional" json:"auto_approve"`                 // Auto-approve learned rules (legacy)
	IgnoreNets    []string `hcl:"ignore_networks,optional" json:"ignore_networks,omitempty"` // Networks to ignore from learning
	RetentionDays int      `hcl:"retention_days,optional" json:"retention_days"`             // How long to keep pending rules
	CacheSize     int      `hcl:"cache_size,optional" json:"cache_size"`                     // Flow cache size (default: 10000)

	// TOFU (Trust On First Use) mode
	LearningMode bool `hcl:"learning_mode,optional"`

	// InlineMode uses nfqueue instead of nflog for packet inspection.
	// This holds packets until a verdict is returned, fixing the "first packet" problem
	// where the first packet of a new flow would be dropped before an allow rule is added.
	// Trade-off: Adds latency (~microseconds) and requires the engine to be running.
	// Recommended: Enable only during initial learning phase, disable after flows are learned.
	InlineMode bool `hcl:"inline_mode,optional"`

	// PacketWindow defines how many packets of a flow are inspected in userspace
	// before offloading to kernel. After this window, trusted flows are marked
	// and bypass userspace entirely for wire-speed performance.
	// Only used when InlineMode is true.
	PacketWindow int `hcl:"packet_window,optional"` // default: 10

	// OffloadMark is the conntrack mark value used to identify trusted flows
	// that should bypass userspace inspection. Flows with this mark are
	// accepted by kernel before reaching nfqueue.
	// Only used when InlineMode is true.
	// NOTE: HCL2 doesn't support hex literals, so use decimal format (e.g., 2097152 for 0x200000)
	OffloadMark string `hcl:"offload_mark,optional"` // default: "0x200000"

	// NOTE: DNS visibility is now configured via dns { inspect "[zone]" { mode = "passive" } }
}

// ParseOffloadMark parses the offload mark from string.
// Supports decimal format (e.g., "2097152") for HCL configs.
// Also supports hex with 0x prefix for API/programmatic use (e.g., "0x200000").
func ParseOffloadMark(s string) (uint32, error) {
	s = strings.TrimSpace(s)

	if s == "" {
		return 0x200000, nil // Default value
	}

	var val uint64
	var err error

	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		val, err = strconv.ParseUint(s[2:], 16, 32)
	} else {
		val, err = strconv.ParseUint(s, 10, 32)
	}

	if err != nil {
		return 0, fmt.Errorf("invalid offload mark value: %w", err)
	}

	return uint32(val), nil
}

// PolicyLearning configures per-policy learning settings (overrides global).
type PolicyLearning struct {
	Enabled     bool   `hcl:"enabled,optional"`
	LogGroup    int    `hcl:"log_group,optional"`
	RateLimit   string `hcl:"rate_limit,optional"`
	AutoApprove bool   `hcl:"auto_approve,optional"`
}

// AnomalyConfig configures traffic anomaly detection.
type AnomalyConfig struct {
	Enabled           bool    `hcl:"enabled,optional"`
	BaselineWindow    string  `hcl:"baseline_window,optional"`     // e.g., "7d"
	MinSamples        int     `hcl:"min_samples,optional"`         // Min hits before alerting
	SpikeStdDev       float64 `hcl:"spike_stddev,optional"`        // Alert if > N stddev
	DropStdDev        float64 `hcl:"drop_stddev,optional"`         // Alert if < N stddev
	AlertCooldown     string  `hcl:"alert_cooldown,optional"`      // e.g., "15m"
	PortScanThreshold int     `hcl:"port_scan_threshold,optional"` // Ports hit before alert
}

// NotificationsConfig configures the notification system.
type NotificationsConfig struct {
	Enabled  bool                  `hcl:"enabled,optional"`
	Channels []NotificationChannel `hcl:"channel,block" json:"channel,omitempty"`
	Rules    []AlertRule           `hcl:"rule,block" json:"rule,omitempty"`
}

// AlertRule defines when an alert should be triggered.
type AlertRule struct {
	Name      string   `hcl:"name,label"`
	Enabled   bool     `hcl:"enabled,optional"`
	Condition string   `hcl:"condition"`
	Severity  string   `hcl:"severity,optional"` // info, warning, critical
	Channels  []string `hcl:"channels,optional"`
	Cooldown  string   `hcl:"cooldown,optional"` // e.g. "1h"
}

// NotificationChannel defines a notification destination.
type NotificationChannel struct {
	Name    string `hcl:"name,label"`
	Type    string `hcl:"type"`           // email, pushover, slack, discord, ntfy, webhook
	Level   string `hcl:"level,optional"` // critical, warning, info
	Enabled bool   `hcl:"enabled,optional"`

	// Email settings
	SMTPHost     string       `hcl:"smtp_host,optional"`
	SMTPPort     int          `hcl:"smtp_port,optional"`
	SMTPUser     string       `hcl:"smtp_user,optional"`
	SMTPPassword SecureString `hcl:"smtp_password,optional"`
	From         string       `hcl:"from,optional"`
	To           []string     `hcl:"to,optional"`

	// Webhook/Slack/Discord settings
	WebhookURL string `hcl:"webhook_url,optional"`
	Channel    string `hcl:"channel,optional"`
	Username   string `hcl:"username,optional"`

	// Pushover settings
	APIToken SecureString `hcl:"api_token,optional"`
	UserKey  SecureString `hcl:"user_key,optional"`
	Priority int          `hcl:"priority,optional"`
	Sound    string       `hcl:"sound,optional"`

	// ntfy settings
	Server string `hcl:"server,optional"`
	Topic  string `hcl:"topic,optional"`

	// Generic auth (for ntfy, webhook)
	Password SecureString      `hcl:"password,optional"`
	Headers  map[string]string `hcl:"headers,optional"`
}
