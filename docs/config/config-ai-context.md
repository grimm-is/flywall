# Flywall HCL Configuration Reference

This document describes the HCL configuration format for Flywall firewall.

## Syntax Overview

```hcl
schema_version = "1.0"  # Required

# Global attributes
schema_version = <string>  # Schema version for backward compatibility.
ip_forwarding = <bool>  # Enable IP forwarding between interfaces (requir...
mss_clamping = <bool>  # Enable TCP MSS clamping to PMTU (recommended fo...
enable_flow_offload = <bool>  # Enable hardware flow offloading for improved pe...
state_dir = <string>  # State Directory (overrides default /var/lib/fly...

# Blocks
interface "<name>" { ... }
route "<name>" { ... }
mdns "<name>" { ... }
upnp "<name>" { ... }
audit "<name>" { ... }
notifications "<name>" { ... }
web "<name>" { ... }
policy_route "<name>" { ... }
uplink_group "<name>" { ... }
frr "<name>" { ... }
vpn "<name>" { ... }
ddns "<name>" { ... }
routing_table "<name>" { ... }
dns "<name>" { ... }
rule_learning "<name>" { ... }
anomaly_detection "<name>" { ... }
replication "<name>" { ... }
features "<name>" { ... }
system "<name>" { ... }
zone "<name>" { ... }
scheduled_rule "<name>" { ... }
mark_rule "<name>" { ... }
uid_routing "<name>" { ... }
ntp "<name>" { ... }
geoip "<name>" { ... }
nat "<name>" { ... }
threat_intel "<name>" { ... }
scheduler "<name>" { ... }
syslog "<name>" { ... }
multi_wan "<name>" { ... }
policy "<name>" { ... }
dhcp "<name>" { ... }
dns_server "<name>" { ... }
ipset "<name>" { ... }
qos_policy "<name>" { ... }
protection "<name>" { ... }
api "<name>" { ... }
```

## Block Reference

### scheduled_rule

ScheduledRule defines a firewall rule that activates on a schedule.

```hcl
scheduled_rule "<name>" {
  description = <string>  # optional
  policy = <string>
  schedule = <string>
  end_schedule = <string>  # optional
  enabled = <bool>  # optional
  rule { ... }
}
```

**Attributes:**
- `description` (string, optional):
- `policy` (string, required): Which policy to modify
- `schedule` (string, required): Cron expression for when to enable
- `end_schedule` (string, optional): Cron expression for when to disable
- `enabled` (bool, optional):

### mark_rule

MarkRule represents a rule for setting routing marks on packets.
Marks are set in nftables and matched by ip rule for routing decisions.

```hcl
mark_rule "<name>" {
  mark = <string>
  mask = <string>  # optional
  proto = <string>  # optional
  src_ip = <string>  # optional
  dst_ip = <string>  # optional
  src_port = <number>  # optional
  dst_port = <number>  # optional
  dst_ports = <list(number)>  # optional
  in_interface = <string>  # optional
  out_interface = <string>  # optional
  src_zone = <string>  # optional
  dst_zone = <string>  # optional
  ipset = <string>  # optional
  conn_state = <list(string)>  # optional
  save_mark = <bool>  # optional
  restore_mark = <bool>  # optional
  enabled = <bool>  # optional
  comment = <string>  # optional
}
```

**Attributes:**
- `mark` (string, required): Mark value to set (hex: 0x10)
- `mask` (string, optional): Mask for mark operations
- `proto` (string, optional): Match criteria tcp, udp, icmp, all
- `src_ip` (string, optional):
- `dst_ip` (string, optional):
- `src_port` (number, optional):
- `dst_port` (number, optional):
- `dst_ports` (list(number), optional): Multiple ports
- `in_interface` (string, optional):
- `out_interface` (string, optional):
- `src_zone` (string, optional):
- `dst_zone` (string, optional):
- `ipset` (string, optional): Match against IPSet
- `conn_state` (list(string), optional): NEW, ESTABLISHED, etc.
- `save_mark` (bool, optional): Mark behavior Save to conntrack
- `restore_mark` (bool, optional): Restore from conntrack
- `enabled` (bool, optional):
- `comment` (string, optional):

### uid_routing

UIDRouting configures per-user routing (for SOCKS proxies, etc.).

```hcl
uid_routing "<name>" {
  uid = <number>  # optional
  username = <string>  # optional
  uplink = <string>  # optional
  vpn_link = <string>  # optional
  interface = <string>  # optional
  snat_ip = <string>  # optional
  enabled = <bool>  # optional
  comment = <string>  # optional
}
```

**Attributes:**
- `uid` (number, optional): User ID to match
- `username` (string, optional): Username (resolved to UID)
- `uplink` (string, optional): Uplink to route through
- `vpn_link` (string, optional): VPN link to route through
- `interface` (string, optional): Output interface
- `snat_ip` (string, optional): IP to SNAT to
- `enabled` (bool, optional):
- `comment` (string, optional):

### ntp

NTP configuration

```hcl
ntp {
  enabled = <bool>  # optional
  servers = <list(string)>  # optional
  interval = <string>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional):
- `servers` (list(string), optional): Upstream servers
- `interval` (string, optional): Sync interval (e.g. "4h")

### geoip

GeoIP configuration for country-based filtering

```hcl
geoip {
  enabled = <bool>  # optional
  database_path = <string>  # optional
  auto_update = <bool>  # optional
  license_key = <string>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional): Enabled activates GeoIP matching in firewall rules.
- `database_path` (string, optional): DatabasePath is the path to the MMDB file (MaxMind or DB-IP format). Default: /var/lib/flywall/ge...
- `auto_update` (bool, optional): AutoUpdate enables automatic database updates (future feature).
- `license_key` (string, optional): LicenseKey for premium MaxMind database updates (future feature). Not required for DB-IP or GeoLi...

### nat

NATRule defines Network Address Translation rules.

```hcl
nat "<name>" {
  description = <string>  # optional
  type = <string>
  proto = <string>  # optional
  out_interface = <string>  # optional
  in_interface = <string>  # optional
  src_ip = <string>  # optional
  dest_ip = <string>  # optional
  mark = <number>  # optional
  dest_port = <string>  # optional
  to_ip = <string>  # optional
  to_port = <string>  # optional
  snat_ip = <string>  # optional
  hairpin = <bool>  # optional
}
```

**Attributes:**
- `description` (string, optional):
- `type` (string, required): masquerade, dnat, snat, redirect
- `proto` (string, optional): tcp, udp
- `out_interface` (string, optional): for masquerade/snat
- `in_interface` (string, optional): for dnat
- `src_ip` (string, optional): Source IP match
- `dest_ip` (string, optional): Dest IP match
- `mark` (number, optional): FWMark match
- `dest_port` (string, optional): Dest Port match (supports ranges "80-90")
- `to_ip` (string, optional): Target IP for DNAT
- `to_port` (string, optional): Target Port for DNAT
- `snat_ip` (string, optional): for snat (Target IP)
- `hairpin` (bool, optional): Enable Hairpin NAT (NAT Reflection)

### threat_intel

ThreatIntel configures threat intelligence feeds.

```hcl
threat_intel {
  enabled = <bool>  # optional
  interval = <string>  # optional
  source { ... }  # multiple allowed
}
```

**Attributes:**
- `enabled` (bool, optional):
- `interval` (string, optional): e.g. "1h"

### scheduler

SchedulerConfig defines scheduler settings.

```hcl
scheduler {
  enabled = <bool>  # optional
  ipset_refresh_hours = <number>  # optional
  dns_refresh_hours = <number>  # optional
  backup_enabled = <bool>  # optional
  backup_schedule = <string>  # optional
  backup_retention_days = <number>  # optional
  backup_dir = <string>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional):
- `ipset_refresh_hours` (number, optional): Default: 24
- `dns_refresh_hours` (number, optional): Default: 24
- `backup_enabled` (bool, optional): Enable auto backups
- `backup_schedule` (string, optional): Cron expression, default: "0 2 * * *"
- `backup_retention_days` (number, optional): Default: 7
- `backup_dir` (string, optional): Default: /var/lib/firewall/backups

### syslog

Syslog remote logging

```hcl
syslog {
  enabled = <bool>  # optional
  host = <string>
  port = <number>  # optional
  protocol = <string>  # optional
  tag = <string>  # optional
  facility = <number>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional):
- `host` (string, required): Remote syslog server hostname/IP
- `port` (number, optional): Default: 514
- `protocol` (string, optional): udp or tcp (default: udp)
- `tag` (string, optional): Syslog tag (default: flywall)
- `facility` (number, optional): Syslog facility (default: 1)

### multi_wan

MultiWAN represents multi-WAN configuration for failover and load balancing.

```hcl
multi_wan {
  enabled = <bool>  # optional
  mode = <string>  # optional
  wan { ... }  # multiple allowed
  health_check { ... }
}
```

**Attributes:**
- `enabled` (bool, optional):
- `mode` (string, optional): "failover", "loadbalance", "both" (values: failover, both)

### policy

Policy defines traffic rules between zones.
Rules are evaluated in order - first match wins.

```hcl
policy "<from>" "<to>" {
  name = <string>  # optional
  description = <string>  # optional
  priority = <number>  # optional
  disabled = <bool>  # optional
  action = <string>  # optional
  masquerade = <bool>  # optional
  log = <bool>  # optional
  log_prefix = <string>  # optional
  inherits = <string>  # optional
  rule { ... }  # multiple allowed
}
```

**Attributes:**
- `name` (string, optional): Optional descriptive name
- `description` (string, optional):
- `priority` (number, optional): Policy priority (lower = evaluated first)
- `disabled` (bool, optional): Temporarily disable this policy
- `action` (string, optional): Action for traffic matching this policy (when no specific rule matches) Values: "accept", "drop",... (values: accept, reject)
- `masquerade` (bool, optional): Masquerade controls NAT for outbound traffic through this policy nil = auto (enable when RFC1918 ...
- `log` (bool, optional): Log packets matching default action
- `log_prefix` (string, optional): Prefix for log messages
- `inherits` (string, optional): Inheritance - allows policies to inherit rules from a parent policy Child policies get all parent...

### dhcp

DHCPServer configuration.

```hcl
dhcp {
  enabled = <bool>  # optional
  mode = <string>  # optional
  external_lease_file = <string>  # optional
  scope { ... }  # multiple allowed
}
```

**Attributes:**
- `enabled` (bool, optional):
- `mode` (string, optional): Mode specifies how DHCP server is managed:   - "builtin" (default): Use Flywall's built-in DHCP s...
- `external_lease_file` (string, optional): ExternalLeaseFile is the path to external DHCP server's lease file (for import mode)

### dns_server

**⚠️ DEPRECATED:** use DNS

Deprecated: use DNS

```hcl
dns_server {
  enabled = <bool>  # optional
  listen_on = <list(string)>  # optional
  listen_port = <number>  # optional
  local_domain = <string>  # optional
  expand_hosts = <bool>  # optional
  dhcp_integration = <bool>  # optional
  authoritative_for = <string>  # optional
  mode = <string>  # optional
  forwarders = <list(string)>  # optional
  upstream_timeout = <number>  # optional
  dnssec = <bool>  # optional
  rebind_protection = <bool>  # optional
  query_logging = <bool>  # optional
  rate_limit_per_sec = <number>  # optional
  allowlist = <list(string)>  # optional
  blocked_ttl = <number>  # optional
  blocked_address = <string>  # optional
  cache_enabled = <bool>  # optional
  cache_size = <number>  # optional
  cache_min_ttl = <number>  # optional
  cache_max_ttl = <number>  # optional
  negative_cache_ttl = <number>  # optional
  conditional_forward { ... }  # multiple allowed
  upstream_doh { ... }  # multiple allowed
  upstream_dot { ... }  # multiple allowed
  upstream_dnscrypt { ... }  # multiple allowed
  doh_server { ... }
  dot_server { ... }
  dnscrypt_server { ... }
  recursive { ... }
  blocklist { ... }  # multiple allowed
  host { ... }  # multiple allowed
  zone { ... }  # multiple allowed
}
```

**Attributes:**
- `enabled` (bool, optional):
- `listen_on` (list(string), optional):
- `listen_port` (number, optional): Default 53
- `local_domain` (string, optional): Local domain configuration e.g., "lan", "home.arpa" (values: lan, home.arpa)
- `expand_hosts` (bool, optional): Append local domain to simple hostnames
- `dhcp_integration` (bool, optional): Auto-register DHCP hostnames
- `authoritative_for` (string, optional): Return NXDOMAIN for unknown local hosts
- `mode` (string, optional): Resolution mode:   - "forward" (default): Forward queries to upstream DNS servers   - "recursive"...
- `forwarders` (list(string), optional): Upstream DNS (for forwarding mode)
- `upstream_timeout` (number, optional): seconds
- `dnssec` (bool, optional): Security Validate DNSSEC
- `rebind_protection` (bool, optional): Block private IPs in public responses
- `query_logging` (bool, optional):
- `rate_limit_per_sec` (number, optional): Per-client rate limit
- `allowlist` (list(string), optional): Domains that bypass blocklists
- `blocked_ttl` (number, optional): TTL for blocked responses
- `blocked_address` (string, optional): IP to return for blocked (default 0.0.0.0)
- `cache_enabled` (bool, optional): Caching
- `cache_size` (number, optional): Max entries
- `cache_min_ttl` (number, optional): Minimum TTL to cache
- `cache_max_ttl` (number, optional): Maximum TTL to cache
- `negative_cache_ttl` (number, optional): TTL for NXDOMAIN

### ipset

IPSet defines a named set of IPs/networks for use in firewall rules.

```hcl
ipset "<name>" {
  description = <string>  # optional
  type = <string>  # optional
  entries = <list(string)>  # optional
  domains = <list(string)>  # optional
  refresh_interval = <string>  # optional
  size = <number>  # optional
  firehol_list = <string>  # optional
  url = <string>  # optional
  refresh_hours = <number>  # optional
  auto_update = <bool>  # optional
  action = <string>  # optional
  apply_to = <string>  # optional
  match_on_source = <bool>  # optional
  match_on_dest = <bool>  # optional
}
```

**Attributes:**
- `description` (string, optional):
- `type` (string, optional): ipv4_addr (default), ipv6_addr, inet_service, or dns
- `entries` (list(string), optional):
- `domains` (list(string), optional): Domains for dynamic resolution (only for type="dns")
- `refresh_interval` (string, optional): Refresh interval for DNS resolution (e.g., "5m", "1h") - only for type="dns" (values: 5m, 1h)
- `size` (number, optional): Optimization: Pre-allocated size for dynamic sets to prevent resizing (suggested: 65535)
- `firehol_list` (string, optional): FireHOL import e.g., "firehol_level1", "spamhaus_drop" (values: firehol_level1, spamhaus_drop)
- `url` (string, optional): Custom URL for IP list
- `refresh_hours` (number, optional): How often to refresh (default: 24)
- `auto_update` (bool, optional): Enable automatic updates
- `action` (string, optional): drop, reject, log (for auto-generated rules)
- `apply_to` (string, optional): input, forward, both (for auto-generated rules)
- `match_on_source` (bool, optional): Match source IP (default: true)
- `match_on_dest` (bool, optional): Match destination IP

### qos_policy

Per-interface settings (first-class)

```hcl
qos_policy "<name>" {
  interface = <string>
  enabled = <bool>  # optional
  direction = <string>  # optional
  download_mbps = <number>  # optional
  upload_mbps = <number>  # optional
  class { ... }  # multiple allowed
  rule { ... }  # multiple allowed
}
```

**Attributes:**
- `interface` (string, required): Interface to apply QoS
- `enabled` (bool, optional):
- `direction` (string, optional): "ingress", "egress", "both" (default: both) (values: ingress, both)
- `download_mbps` (number, optional):
- `upload_mbps` (number, optional):

### protection

InterfaceProtection defines security protection settings for an interface.

```hcl
protection "<name>" {
  interface = <string>
  enabled = <bool>  # optional
  anti_spoofing = <bool>  # optional
  bogon_filtering = <bool>  # optional
  private_filtering = <bool>  # optional
  invalid_packets = <bool>  # optional
  syn_flood_protection = <bool>  # optional
  syn_flood_rate = <number>  # optional
  syn_flood_burst = <number>  # optional
  icmp_rate_limit = <bool>  # optional
  icmp_rate = <number>  # optional
  icmp_burst = <number>  # optional
  new_conn_rate_limit = <bool>  # optional
  new_conn_rate = <number>  # optional
  new_conn_burst = <number>  # optional
  port_scan_protection = <bool>  # optional
  port_scan_threshold = <number>  # optional
  geo_blocking = <bool>  # optional
  blocked_countries = <list(string)>  # optional
  allowed_countries = <list(string)>  # optional
}
```

**Attributes:**
- `interface` (string, required): Interface name or "*" for all
- `enabled` (bool, optional):
- `anti_spoofing` (bool, optional): AntiSpoofing drops packets with spoofed source IPs (recommended for WAN)
- `bogon_filtering` (bool, optional): BogonFiltering drops packets from reserved/invalid IP ranges
- `private_filtering` (bool, optional): PrivateFiltering drops packets from private IP ranges on WAN (RFC1918)
- `invalid_packets` (bool, optional): InvalidPackets drops malformed/invalid packets
- `syn_flood_protection` (bool, optional): SynFloodProtection limits SYN packets to prevent SYN floods
- `syn_flood_rate` (number, optional): packets/sec (default: 25)
- `syn_flood_burst` (number, optional): burst allowance (default: 50)
- `icmp_rate_limit` (bool, optional): ICMPRateLimit limits ICMP packets to prevent ping floods
- `icmp_rate` (number, optional): packets/sec (default: 10)
- `icmp_burst` (number, optional): burst (default: 20)
- `new_conn_rate_limit` (bool, optional): NewConnRateLimit limits new connections per second
- `new_conn_rate` (number, optional): per second (default: 100)
- `new_conn_burst` (number, optional): burst (default: 200)
- `port_scan_protection` (bool, optional): PortScanProtection detects and blocks port scanning
- `port_scan_threshold` (number, optional): ports/sec (default: 10)
- `geo_blocking` (bool, optional): GeoBlocking blocks traffic from specific countries (requires GeoIP database)
- `blocked_countries` (list(string), optional): ISO country codes
- `allowed_countries` (list(string), optional): If set, only these allowed

### api

API configuration

```hcl
api {
  enabled = <bool>  # optional
  disable_sandbox = <bool>  # optional
  listen = <string>  # optional
  tls_listen = <string>  # optional
  tls_cert = <string>  # optional
  tls_key = <string>  # optional
  disable_http_redirect = <bool>  # optional
  require_auth = <bool>  # optional
  bootstrap_key = <string>  # optional
  key_store_path = <string>  # optional
  cors_origins = <list(string)>  # optional
  key { ... }  # multiple allowed
  letsencrypt { ... }
}
```

**Attributes:**
- `enabled` (bool, optional):
- `disable_sandbox` (bool, optional): Default: false (Sandbox Enabled)
- `listen` (string, optional): Deprecated: use web.listen
- `tls_listen` (string, optional): Deprecated: use web.tls_listen
- `tls_cert` (string, optional): Deprecated: use web.tls_cert
- `tls_key` (string, optional): Deprecated: use web.tls_key
- `disable_http_redirect` (bool, optional): Deprecated: use web.disable_redirect
- `require_auth` (bool, optional): Require API key auth
- `bootstrap_key` (string, optional): Bootstrap key (for initial setup, should be removed after creating real keys)
- `key_store_path` (string, optional): API key storage Path to key store file
- `cors_origins` (list(string), optional): CORS settings

### interface

Interface represents a physical or virtual network interface configuration.
Each interface can be assigned to a security zone and configured with
static IPs, DHCP, VLANs, and other network settings.

```hcl
interface "<name>" {
  description = <string>  # optional
  disabled = <bool>  # optional
  zone = <string>  # optional
  ipv4 = <list(string)>  # optional
  ipv6 = <list(string)>  # optional
  dhcp = <bool>  # optional
  dhcp_v6 = <bool>  # optional
  ra = <bool>  # optional
  dhcp_client = <string>  # optional
  table = <number>  # optional
  gateway = <string>  # optional
  gateway_v6 = <string>  # optional
  mtu = <number>  # optional
  disable_anti_lockout = <bool>  # optional
  access_web_ui = <bool>  # optional
  web_ui_port = <number>  # optional
  new_zone { ... }
  bond { ... }
  vlan { ... }  # multiple allowed
  management { ... }
  tls { ... }
}
```

**Attributes:**
- `description` (string, optional): Human-readable description for this interface.
- `disabled` (bool, optional): Temporarily disable this interface (brings it down).
- `zone` (string, optional): Assign this interface to a security zone.
- `ipv4` (list(string), optional): Static IPv4 addresses in CIDR notation.
- `ipv6` (list(string), optional): Static IPv6 addresses in CIDR notation.
- `dhcp` (bool, optional): Enable DHCP client on this interface.
- `dhcp_v6` (bool, optional): Enable DHCPv6 client for IPv6 address assignment.
- `ra` (bool, optional): Enable Router Advertisements (for IPv6 server mode).
- `dhcp_client` (string, optional): DHCPClient specifies how DHCP client is managed:   - "builtin" (default): Use Flywall's built-in ...
- `table` (number, optional): Table specifies the routing table ID for this interface. If set to > 0 (and not 254/main), Flywal...
- `gateway` (string, optional): Default gateway for static IPv4 configuration.
- `gateway_v6` (string, optional): Default gateway for static IPv6 configuration.
- `mtu` (number, optional): Maximum Transmission Unit size in bytes.
- `disable_anti_lockout` (bool, optional): Anti-Lockout protection (sandbox mode only) When true, implicit accept rules are created for this...
- `access_web_ui` (bool, optional): Web UI / API Access Deprecated: Use Management block instead
- `web_ui_port` (number, optional): Deprecated: Use Management block instead Port to map (external)

### route

Route represents a static route configuration.

```hcl
route "<name>" {
  destination = <string>
  gateway = <string>  # optional
  interface = <string>  # optional
  monitor_ip = <string>  # optional
  table = <number>  # optional
  metric = <number>  # optional
}
```

**Attributes:**
- `destination` (string, required):
- `gateway` (string, optional):
- `interface` (string, optional):
- `monitor_ip` (string, optional):
- `table` (number, optional): Routing table ID (default: main)
- `metric` (number, optional): Route metric/priority

### mdns

mDNS Reflector configuration

```hcl
mdns {
  enabled = <bool>  # optional
  interfaces = <list(string)>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional):
- `interfaces` (list(string), optional): Interfaces to reflect between

### upnp

UPnP IGD configuration

```hcl
upnp {
  enabled = <bool>  # optional
  external_interface = <string>  # optional
  internal_interfaces = <list(string)>  # optional
  secure_mode = <bool>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional):
- `external_interface` (string, optional): WAN interface
- `internal_interfaces` (list(string), optional): LAN interfaces
- `secure_mode` (bool, optional): Only allow mapping to requesting IP

### audit

Audit logging configuration

```hcl
audit {
  enabled = <bool>  # optional
  retention_days = <number>  # optional
  kernel_audit = <bool>  # optional
  database_path = <string>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional): Enabled activates audit logging to SQLite.
- `retention_days` (number, optional): RetentionDays is the number of days to retain audit events. Default: 90 days.
- `kernel_audit` (bool, optional): KernelAudit enables writing to the Linux kernel audit log (auditd). Useful for compliance with SO...
- `database_path` (string, optional): DatabasePath overrides the default audit database location. Default: /var/lib/flywall/audit.db

### notifications

NotificationsConfig configures the notification system.

```hcl
notifications {
  enabled = <bool>  # optional
  channel { ... }  # multiple allowed
  rule { ... }  # multiple allowed
}
```

**Attributes:**
- `enabled` (bool, optional):

### web

Web Server configuration (previously part of API)

```hcl
web {
  listen = <string>  # optional
  tls_listen = <string>  # optional
  tls_cert = <string>  # optional
  tls_key = <string>  # optional
  disable_redirect = <bool>  # optional
  serve_ui = <bool>  # optional
  serve_api = <bool>  # optional
  allow { ... }  # multiple allowed
  deny { ... }  # multiple allowed
}
```

**Attributes:**
- `listen` (string, optional): Listen addresses HTTP listen address (default :80)
- `tls_listen` (string, optional): HTTPS listen address (default :443)
- `tls_cert` (string, optional): TLS Configuration Path to TLS certificate
- `tls_key` (string, optional): Path to TLS key
- `disable_redirect` (bool, optional): Behavior Disable HTTP->HTTPS redirect
- `serve_ui` (bool, optional): Serve the dashboard (default true)
- `serve_api` (bool, optional): Serve API paths (default true)

### policy_route

PolicyRoute represents a policy-based routing rule.
Policy routes use firewall marks to direct traffic to specific routing tables.

```hcl
policy_route "<name>" {
  priority = <number>  # optional
  mark = <string>  # optional
  mark_mask = <string>  # optional
  from = <string>  # optional
  to = <string>  # optional
  iif = <string>  # optional
  oif = <string>  # optional
  fwmark = <string>  # optional
  table = <number>  # optional
  blackhole = <bool>  # optional
  prohibit = <bool>  # optional
  enabled = <bool>  # optional
  comment = <string>  # optional
}
```

**Attributes:**
- `priority` (number, optional): Rule priority (lower = higher priority)
- `mark` (string, optional): Match criteria (combined with AND) Firewall mark to match (hex: 0x10)
- `mark_mask` (string, optional): Mask for mark matching
- `from` (string, optional): Source IP/CIDR
- `to` (string, optional): Destination IP/CIDR
- `iif` (string, optional): Input interface
- `oif` (string, optional): Output interface
- `fwmark` (string, optional): Alternative mark syntax
- `table` (number, optional): Action Routing table to use
- `blackhole` (bool, optional): Drop matching packets
- `prohibit` (bool, optional): Return ICMP prohibited
- `enabled` (bool, optional): Default true
- `comment` (string, optional):

### uplink_group

UplinkGroup configures a group of uplinks (WAN, VPN, etc.) with failover/load balancing.
This enables dynamic switching between uplinks while preserving existing connections.

```hcl
uplink_group "<name>" {
  source_networks = <list(string)>
  source_interfaces = <list(string)>  # optional
  source_zones = <list(string)>  # optional
  failover_mode = <string>  # optional
  failback_mode = <string>  # optional
  failover_delay = <number>  # optional
  failback_delay = <number>  # optional
  load_balance_mode = <string>  # optional
  sticky_connections = <bool>  # optional
  enabled = <bool>  # optional
  uplink { ... }  # multiple allowed
  health_check { ... }
}
```

**Attributes:**
- `source_networks` (list(string), required): CIDRs that use this group
- `source_interfaces` (list(string), optional): Interfaces for connmark restore
- `source_zones` (list(string), optional): Zones that use this group
- `failover_mode` (string, optional): Failover configuration "immediate", "graceful", "manual", "programmatic" (values: immediate, programmatic)
- `failback_mode` (string, optional): "immediate", "graceful", "manual", "never" (values: immediate, never)
- `failover_delay` (number, optional): Seconds before failover
- `failback_delay` (number, optional): Seconds before failback
- `load_balance_mode` (string, optional): Load balancing configuration "none", "roundrobin", "weighted", "latency" (values: none, latency)
- `sticky_connections` (bool, optional):
- `enabled` (bool, optional):

### frr

FRRConfig holds configuration for Free Range Routing (FRR).

```hcl
frr {
  enabled = <bool>  # optional
  ospf { ... }
  bgp { ... }
}
```

**Attributes:**
- `enabled` (bool, optional):

### vpn

VPN integrations (Tailscale, WireGuard, etc.) for secure remote access

```hcl
vpn {
  interface_prefix_zones = <map>  # optional
  tailscale { ... }  # multiple allowed
  wireguard { ... }  # multiple allowed
  six_to_four { ... }  # multiple allowed
}
```

**Attributes:**
- `interface_prefix_zones` (map, optional): Interface prefix matching for zones (like firehol's "wg+" syntax) Maps interface prefix to zone n... (values: vpn, tailscale)

### ddns

Dynamic DNS

```hcl
ddns {
  enabled = <bool>  # optional
  provider = <string>
  hostname = <string>
  token = <string>  # optional
  username = <string>  # optional
  zone_id = <string>  # optional
  record_id = <string>  # optional
  interface = <string>  # optional
  interval = <number>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional):
- `provider` (string, required): duckdns, cloudflare, noip
- `hostname` (string, required): Hostname to update
- `token` (string, optional): API token/password
- `username` (string, optional): For providers requiring username
- `zone_id` (string, optional): For Cloudflare
- `record_id` (string, optional): For Cloudflare
- `interface` (string, optional): Interface to get IP from
- `interval` (number, optional): Update interval in minutes (default: 5)

### routing_table

RoutingTable represents a custom routing table configuration.

```hcl
routing_table "<name>" {
  id = <number>
  route { ... }  # multiple allowed
}
```

**Attributes:**
- `id` (number, required): Table ID (1-252 for custom tables)

### dns

New consolidated DNS config

```hcl
dns {
  mode = <string>  # optional
  forwarders = <list(string)>  # optional
  upstream_timeout = <number>  # optional
  dnssec = <bool>  # optional
  egress_filter = <bool>  # optional
  egress_filter_ttl = <number>  # optional
  conditional_forward { ... }  # multiple allowed
  upstream_doh { ... }  # multiple allowed
  upstream_dot { ... }  # multiple allowed
  upstream_dnscrypt { ... }  # multiple allowed
  recursive { ... }
  serve { ... }  # multiple allowed
  inspect { ... }  # multiple allowed
}
```

**Attributes:**
- `mode` (string, optional): Resolution mode for upstream queries:   - "forward" (default): Forward queries to upstream DNS se...
- `forwarders` (list(string), optional): Upstream DNS servers for forwarding mode
- `upstream_timeout` (number, optional): seconds
- `dnssec` (bool, optional): DNSSEC validation for upstream queries
- `egress_filter` (bool, optional): Egress Filter (DNS Wall) If enabled, firewall blocks outbound traffic to IPs not recently resolve...
- `egress_filter_ttl` (number, optional): Seconds (default: matches record TTL)

### rule_learning

Rule learning and notifications

```hcl
rule_learning {
  enabled = <bool>  # optional
  log_group = <number>  # optional
  rate_limit = <string>  # optional
  auto_approve = <bool>  # optional
  ignore_networks = <list(string)>  # optional
  retention_days = <number>  # optional
  cache_size = <number>  # optional
  learning_mode = <bool>  # optional
  inline_mode = <bool>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional):
- `log_group` (number, optional): nflog group (default: 100)
- `rate_limit` (string, optional): e.g., "10/minute"
- `auto_approve` (bool, optional): Auto-approve learned rules (legacy)
- `ignore_networks` (list(string), optional): Networks to ignore from learning
- `retention_days` (number, optional): How long to keep pending rules
- `cache_size` (number, optional): Flow cache size (default: 10000)
- `learning_mode` (bool, optional): TOFU (Trust On First Use) mode
- `inline_mode` (bool, optional): InlineMode uses nfqueue instead of nflog for packet inspection. This holds packets until a verdic...

### anomaly_detection

AnomalyConfig configures traffic anomaly detection.

```hcl
anomaly_detection {
  enabled = <bool>  # optional
  baseline_window = <string>  # optional
  min_samples = <number>  # optional
  spike_stddev = <number>  # optional
  drop_stddev = <number>  # optional
  alert_cooldown = <string>  # optional
  port_scan_threshold = <number>  # optional
}
```

**Attributes:**
- `enabled` (bool, optional):
- `baseline_window` (string, optional): e.g., "7d"
- `min_samples` (number, optional): Min hits before alerting
- `spike_stddev` (number, optional): Alert if > N stddev
- `drop_stddev` (number, optional): Alert if < N stddev
- `alert_cooldown` (string, optional): e.g., "15m"
- `port_scan_threshold` (number, optional): Ports hit before alert

### replication

State Replication configuration

```hcl
replication {
  mode = <string>
  listen_addr = <string>  # optional
  primary_addr = <string>  # optional
  peer_addr = <string>  # optional
  secret_key = <string>  # optional
  tls_cert = <string>  # optional
  tls_key = <string>  # optional
  tls_ca = <string>  # optional
  tls_mutual = <bool>  # optional
  ha { ... }
}
```

**Attributes:**
- `mode` (string, required): Mode: "primary", "replica", or "standalone" (default, no HA) (values: primary, replica)
- `listen_addr` (string, optional): Listen address for replication traffic (e.g. ":9000")
- `primary_addr` (string, optional): Address of the primary node (only for replica mode)
- `peer_addr` (string, optional): Address of the peer node (used for HA heartbeat - both nodes need this)
- `secret_key` (string, optional): Secret key for PSK authentication (required for secure replication)
- `tls_cert` (string, optional): TLS configuration for encrypted replication
- `tls_key` (string, optional):
- `tls_ca` (string, optional):
- `tls_mutual` (bool, optional): Require client certs

### features

Feature Flags

```hcl
features {
  threat_intel = <bool>  # optional
  network_learning = <bool>  # optional
  qos = <bool>  # optional
  integrity_monitoring = <bool>  # optional
}
```

**Attributes:**
- `threat_intel` (bool, optional): Phase 5: Threat Intelligence
- `network_learning` (bool, optional): Automated rule learning
- `qos` (bool, optional): Traffic Shaping
- `integrity_monitoring` (bool, optional): Detect and revert external changes

### system

System tuning and settings

```hcl
system {
  sysctl_profile = <string>  # optional
  sysctl = <map>  # optional
  timezone = <string>  # optional
}
```

**Attributes:**
- `sysctl_profile` (string, optional): SysctlProfile selects a preset sysctl tuning profile Options: "default", "performance", "low-memo... (values: default, security)
- `sysctl` (map, optional): Sysctl allows manual override of sysctl parameters Applied after profile tuning
- `timezone` (string, optional): Timezone for scheduled rules (e.g. "America/Los_Angeles"). Defaults to "UTC".

### zone

Zone defines a network security zone.
Zones can match traffic by interface, source/destination IP, VLAN, or combinations.
Simple zones use top-level fields, complex zones use match blocks.

```hcl
zone "<name>" {
  color = <string>  # optional
  description = <string>  # optional
  interface = <string>  # optional
  src = <string>  # optional
  dst = <string>  # optional
  vlan = <number>  # optional
  interfaces = <list(string)>  # optional
  ipsets = <list(string)>  # optional
  networks = <list(string)>  # optional
  action = <string>  # optional
  external = <bool>  # optional
  ipv4 = <list(string)>  # optional
  ipv6 = <list(string)>  # optional
  dhcp = <bool>  # optional
  match { ... }  # multiple allowed
  services { ... }
  management { ... }
}
```

**Attributes:**
- `color` (string, optional):
- `description` (string, optional):
- `interface` (string, optional): Simple match criteria (use for single-interface zones) These are effectively a single implicit ma...
- `src` (string, optional): Source IP/network (e.g., "192.168.1.0/24")
- `dst` (string, optional): Destination IP/network
- `vlan` (number, optional): VLAN tag
- `interfaces` (list(string), optional): DEPRECATED: Use Interface or Matches instead Will be auto-converted to Matches with warning
- `ipsets` (list(string), optional): Legacy fields (kept for backwards compat) IPSet names for IP-based membership
- `networks` (list(string), optional): Direct CIDR ranges
- `action` (string, optional): Zone behavior Action for intra-zone traffic: "accept", "drop", "reject" (default: accept) (values: accept, reject)
- `external` (bool, optional): External marks this as an external/WAN zone (used for auto-masquerade detection) If not set, dete...
- `ipv4` (list(string), optional): IP assignment for simple zones (shorthand - assigns to the interface)
- `ipv6` (list(string), optional):
- `dhcp` (bool, optional): Use DHCP client on this interface
