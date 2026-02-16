// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// Features defines feature flags for the application
type Features struct {
	ThreatIntel         bool `hcl:"threat_intel,optional" json:"threat_intel" tui:"title=Threat Intelligence,desc=Enable IP reputation and threat feeds"`
	NetworkLearning     bool `hcl:"network_learning,optional" json:"network_learning" tui:"title=Network Learning,desc=Auto-learn traffic patterns"`
	QoS                 bool `hcl:"qos,optional" json:"qos" tui:"title=QoS,desc=Quality of Service traffic shaping"`
	IntegrityMonitoring bool `hcl:"integrity_monitoring,optional" json:"integrity_monitoring" tui:"title=Integrity Monitor,desc=Detect external system changes"`
}

// APIConfig configures the REST API server.
type APIConfig struct {
	Enabled             bool   `hcl:"enabled,optional" json:"enabled,omitempty" tui:"title=Enable API,desc=Enable REST API Server"`
	DisableSandbox      bool   `hcl:"disable_sandbox,optional" json:"disable_sandbox,omitempty" tui:"title=Disable Sandbox,desc=DANGEROUS: Disable security sandbox"`
	Listen              string `hcl:"listen,optional" json:"listen,omitempty"`                               // Deprecated: use web.listen
	TLSListen           string `hcl:"tls_listen,optional" json:"tls_listen,omitempty"`                       // Deprecated: use web.tls_listen
	TLSCert             string `hcl:"tls_cert,optional" json:"tls_cert,omitempty"`                           // Deprecated: use web.tls_cert
	TLSKey              string `hcl:"tls_key,optional" json:"tls_key,omitempty"`                             // Deprecated: use web.tls_key
	DisableHTTPRedirect bool   `hcl:"disable_http_redirect,optional" json:"disable_http_redirect,omitempty"` // Deprecated: use web.disable_redirect
	RequireAuth         bool   `hcl:"require_auth,optional" json:"require_auth,omitempty"`                   // Require API key auth

	// Bootstrap key (for initial setup, should be removed after creating real keys)
	BootstrapKey string `hcl:"bootstrap_key,optional" json:"bootstrap_key,omitempty"`

	// API key storage
	KeyStorePath string `hcl:"key_store_path,optional" json:"key_store_path,omitempty"` // Path to key store file

	// Predefined API keys (for config-based key management)
	Keys []APIKeyConfig `hcl:"key,block" json:"key,omitempty"`

	// CORS settings
	CORSOrigins []string `hcl:"cors_origins,optional" json:"cors_origins,omitempty"`

	// Let's Encrypt automatic TLS
	LetsEncrypt *LetsEncryptConfig `hcl:"letsencrypt,block" json:"letsencrypt,omitempty"`

	// Tailscale/tsnet configuration
	TsNet *TsNetConfig `hcl:"tsnet,block" json:"tsnet,omitempty"`
}

// TsNetConfig configures the embedded Tailscale client.
type TsNetConfig struct {
	Enabled   bool   `hcl:"enabled,optional" json:"enabled"`
	Hostname  string `hcl:"hostname,optional" json:"hostname,omitempty"`   // Node name
	AuthKey   string `hcl:"auth_key,optional" json:"auth_key,omitempty"`   // Auth key (tskey-auth-...)
	Ephemeral bool   `hcl:"ephemeral,optional" json:"ephemeral,omitempty"` // Delete node on exit
	LogWebUI  bool   `hcl:"log_web_ui,optional" json:"log_web_ui,omitempty"`
}

// APIKeyConfig defines an API key in the config file.
type APIKeyConfig struct {
	Name         string   `hcl:"name,label" json:"name"`
	Key          string   `hcl:"key" json:"key"`                 // The actual key value
	Permissions  []string `hcl:"permissions" json:"permissions"` // Permission strings
	AllowedIPs   []string `hcl:"allowed_ips,optional" json:"allowed_ips,omitempty"`
	AllowedPaths []string `hcl:"allowed_paths,optional" json:"allowed_paths,omitempty"`
	RateLimit    int      `hcl:"rate_limit,optional" json:"rate_limit,omitempty"`
	Enabled      bool     `hcl:"enabled,optional" json:"enabled,omitempty"`
	Description  string   `hcl:"description,optional" json:"description,omitempty"`
}

// LetsEncryptConfig configures automatic TLS certificate provisioning.
type LetsEncryptConfig struct {
	Enabled  bool   `hcl:"enabled,optional" json:"enabled"`
	Email    string `hcl:"email" json:"email"`                            // Contact email for certificate
	Domain   string `hcl:"domain" json:"domain"`                          // Domain name for certificate
	CacheDir string `hcl:"cache_dir,optional" json:"cache_dir,omitempty"` // Certificate cache directory
	Staging  bool   `hcl:"staging,optional" json:"staging,omitempty"`     // Use staging server for testing
}

// TLSConfig defines TLS/certificate configuration for an interface.
// The certificate presented depends on which interface the client connects through.
type TLSConfig struct {
	Mode     string `hcl:"mode,optional" json:"mode,omitempty"`         // "self-signed", "acme", "tailscale", "manual"
	Hostname string `hcl:"hostname,optional" json:"hostname,omitempty"` // For Tailscale mode

	// ACME (Let's Encrypt) settings
	Email   string   `hcl:"email,optional" json:"email,omitempty"`
	Domains []string `hcl:"domains,optional" json:"domains,omitempty"`

	// Manual certificate (bring your own)
	CertFile string `hcl:"cert_file,optional" json:"cert_file,omitempty"`
	KeyFile  string `hcl:"key_file,optional" json:"key_file,omitempty"`
}

// ReplicationConfig configures state replication and HA between nodes.
// ReplicationConfig configures replication and high-availability.
type ReplicationConfig struct {
	// Mode: "primary", "replica", or "standalone" (default, no HA)
	Mode string `hcl:"mode" json:"mode"`

	// Listen address for replication traffic (e.g. ":9000")
	ListenAddr string `hcl:"listen_addr,optional" json:"listen_addr,omitempty"`

	// Address of the primary node (only for replica mode)
	PrimaryAddr string `hcl:"primary_addr,optional" json:"primary_addr,omitempty"`

	// Address of the peer node (used for HA heartbeat - both nodes need this)
	PeerAddr string `hcl:"peer_addr,optional" json:"peer_addr,omitempty"`

	// Secret key for PSK authentication (required for secure replication)
	SecretKey string `hcl:"secret_key,optional" json:"secret_key,omitempty"`

	// TLS configuration for encrypted replication
	TLSCertFile string `hcl:"tls_cert,optional" json:"tls_cert,omitempty"`
	TLSKeyFile  string `hcl:"tls_key,optional" json:"tls_key,omitempty"`
	TLSCAFile   string `hcl:"tls_ca,optional" json:"tls_ca,omitempty"`
	TLSMutual   bool   `hcl:"tls_mutual,optional" json:"tls_mutual,omitempty"` // Require client certs

	// HA configuration for high-availability failover
	HA *HAConfig `hcl:"ha,block" json:"ha,omitempty"`
}

// HAConfig configures high-availability failover between two nodes.
type HAConfig struct {
	// Enabled activates HA monitoring and failover
	Enabled bool `hcl:"enabled,optional" json:"enabled,omitempty"`

	// Priority determines which node becomes primary (lower = higher priority)
	// Default: 100. Set one node to 50 and another to 150 for deterministic election.
	Priority int `hcl:"priority,optional" json:"priority,omitempty"`

	// Virtual IPs to migrate on failover (for LAN-side gateway addresses)
	VirtualIPs []VirtualIP `hcl:"virtual_ip,block" json:"virtual_ip,omitempty"`

	// Virtual MACs to migrate on failover (for DHCP-assigned WAN interfaces)
	VirtualMACs []VirtualMAC `hcl:"virtual_mac,block" json:"virtual_mac,omitempty"`

	// HeartbeatInterval is seconds between heartbeat messages (default: 1)
	HeartbeatInterval int `hcl:"heartbeat_interval,optional" json:"heartbeat_interval,omitempty"`

	// FailureThreshold is missed heartbeats before declaring peer dead (default: 3)
	FailureThreshold int `hcl:"failure_threshold,optional" json:"failure_threshold,omitempty"`

	// FailbackMode controls behavior when original primary recovers:
	//   "auto"   - automatically failback after FailbackDelay
	//   "manual" - require manual intervention to failback
	//   "never"  - never failback, new primary stays primary
	FailbackMode string `hcl:"failback_mode,optional" json:"failback_mode,omitempty"`

	// FailbackDelay is seconds to wait before automatic failback (default: 60)
	FailbackDelay int `hcl:"failback_delay,optional" json:"failback_delay,omitempty"`

	// HeartbeatPort is the UDP port for HA heartbeat messages (default: 9002)
	HeartbeatPort int `hcl:"heartbeat_port,optional" json:"heartbeat_port,omitempty"`

	// ConntrackSync enables connection state replication via conntrackd
	ConntrackSync *ConntrackSyncConfig `hcl:"conntrack_sync,block" json:"conntrack_sync,omitempty"`
}

// ConntrackSyncConfig configures connection tracking state synchronization.
// Uses conntrackd to replicate established connections between HA nodes,
// allowing TCP sessions to survive failover without being reset.
type ConntrackSyncConfig struct {
	// Enabled activates conntrack synchronization
	Enabled bool `hcl:"enabled,optional" json:"enabled,omitempty"`

	// Interface is the network interface for sync traffic (default: HA peer link)
	Interface string `hcl:"interface,optional" json:"interface,omitempty"`

	// MulticastGroup for sync traffic (default: 225.0.0.50)
	// Set to empty string to use unicast mode with peer address
	MulticastGroup string `hcl:"multicast_group,optional" json:"multicast_group,omitempty"`

	// Port for sync traffic (default: 3780)
	Port int `hcl:"port,optional" json:"port,omitempty"`

	// ExpectSync enables expectation table sync for ALG protocols (FTP, SIP, etc.)
	ExpectSync bool `hcl:"expect_sync,optional" json:"expect_sync,omitempty"`
}

// VirtualIP defines a shared IP address for HA failover.
// This IP is added to the interface on the active node and removed on failover.
type VirtualIP struct {
	// Address is the virtual IP in CIDR notation (e.g., "192.168.1.1/24")
	Address string `hcl:"address" json:"address"`

	// Interface is the network interface to add the VIP to (e.g., "eth1")
	Interface string `hcl:"interface" json:"interface"`

	// Label is an optional interface label for the address (e.g., "eth1:vip")
	Label string `hcl:"label,optional" json:"label,omitempty"`
}

// VirtualMAC defines a shared MAC address for HA failover.
// Used when the interface gets its IP via DHCP from an upstream provider.
// On failover, the backup node applies this MAC and reclaims the DHCP lease.
type VirtualMAC struct {
	// Address is the virtual MAC address (e.g., "02:gc:ic:00:00:01").
	// If empty, a locally-administered MAC is auto-generated from the interface name.
	Address string `hcl:"address,optional" json:"address,omitempty"`

	// Interface is the network interface to apply the VMAC to (e.g., "eth0")
	Interface string `hcl:"interface" json:"interface"`

	// DHCP indicates this interface uses DHCP. On failover, the backup will
	// attempt to reclaim the same DHCP lease by sending a REQUEST with the
	// previous lease's offered IP.
	DHCP bool `hcl:"dhcp,optional" json:"dhcp,omitempty"`
}

// SchedulerConfig defines scheduler settings.
type SchedulerConfig struct {
	Enabled             bool   `hcl:"enabled,optional" json:"enabled"`
	IPSetRefreshHours   int    `hcl:"ipset_refresh_hours,optional" json:"ipset_refresh_hours"`     // Default: 24
	DNSRefreshHours     int    `hcl:"dns_refresh_hours,optional" json:"dns_refresh_hours"`         // Default: 24
	BackupEnabled       bool   `hcl:"backup_enabled,optional" json:"backup_enabled"`               // Enable auto backups
	BackupSchedule      string `hcl:"backup_schedule,optional" json:"backup_schedule,omitempty"`   // Cron expression, default: "0 2 * * *"
	BackupRetentionDays int    `hcl:"backup_retention_days,optional" json:"backup_retention_days"` // Default: 7
	BackupDir           string `hcl:"backup_dir,optional" json:"backup_dir,omitempty"`             // Default: /var/lib/firewall/backups
}

// ScheduledRule defines a firewall rule that activates on a schedule.
type ScheduledRule struct {
	Name        string     `hcl:"name,label" json:"name"`
	Description string     `hcl:"description,optional" json:"description,omitempty"`
	PolicyName  string     `hcl:"policy" json:"policy"`                                // Which policy to modify
	Rule        PolicyRule `hcl:"rule,block" json:"rule"`                              // The rule to add/remove
	Schedule    string     `hcl:"schedule" json:"schedule"`                            // Cron expression for when to enable
	EndSchedule string     `hcl:"end_schedule,optional" json:"end_schedule,omitempty"` // Cron expression for when to disable
	Enabled     bool       `hcl:"enabled,optional" json:"enabled"`
}

// SystemConfig contains system-level tuning and preferences.
type SystemConfig struct {
	// SysctlProfile selects a preset sysctl tuning profile
	// Options: "default", "performance", "low-memory", "security"
	SysctlProfile string `hcl:"sysctl_profile,optional" json:"sysctl_profile,omitempty"`

	// Sysctl allows manual override of sysctl parameters
	// Applied after profile tuning
	Sysctl map[string]string `hcl:"sysctl,optional" json:"sysctl,omitempty"`

	// Timezone for scheduled rules (e.g. "America/Los_Angeles"). Defaults to "UTC".
	Timezone string `hcl:"timezone,optional" json:"timezone,omitempty"`

	// UpdateCheckInterval is the duration between checking for updates (default: "24h")
	UpdateCheckInterval string `hcl:"update_check_interval,optional" json:"update_check_interval,omitempty"`

	// OUIDBPath is the location to store the OUI database
	OUIDBPath string `hcl:"oui_db_path,optional" json:"oui_db_path,omitempty"`
}
