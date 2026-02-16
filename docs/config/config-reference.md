# Flywall Configuration

Config is the top-level structure for the firewall configuration.
It defines all aspects of the firewall including interfaces, zones, policies,
NAT rules, services (DHCP, DNS), VPN integrations, and security features.

**Schema Version:** 1.0

## Table of Contents

- [Global Attributes](#global-attributes)
- [anomaly_detection](#anomaly-detection)
- [api](#api)
- [audit](#audit)
- [ddns](#ddns)
- [dhcp](#dhcp)
- [dns](#dns)
- [dns_server](#dns-server)
 ⚠️ *deprecated*- [features](#features)
- [frr](#frr)
- [geoip](#geoip)
- [interface](#interface)
- [ipset](#ipset)
- [mark_rule](#mark-rule)
- [mdns](#mdns)
- [multi_wan](#multi-wan)
- [nat](#nat)
- [notifications](#notifications)
- [ntp](#ntp)
- [policy](#policy)
- [policy_route](#policy-route)
- [protection](#protection)
- [qos_policy](#qos-policy)
- [replication](#replication)
- [route](#route)
- [routing_table](#routing-table)
- [rule_learning](#rule-learning)
- [scheduled_rule](#scheduled-rule)
- [scheduler](#scheduler)
- [syslog](#syslog)
- [system](#system)
- [threat_intel](#threat-intel)
- [uid_routing](#uid-routing)
- [uplink_group](#uplink-group)
- [upnp](#upnp)
- [vpn](#vpn)
- [web](#web)
- [zone](#zone)

## Global Attributes

Top-level configuration attributes.

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `schema_version` | `string` | No (default: `"1.0"`) | Schema version for backward compatibility. Values: `1.0` |
| `ip_forwarding` | `bool` | No (default: `false`) | Enable IP forwarding between interfaces (required for routing). |
| `mss_clamping` | `bool` | No (default: `false`) | Enable TCP MSS clamping to PMTU (recommended for VPNs). |
| `enable_flow_offload` | `bool` | No (default: `false`) | Enable hardware flow offloading for improved performance. |
| `state_dir` | `string` | No | State Directory (overrides default /var/lib/flywall) |

## anomaly_detection

AnomalyConfig configures traffic anomaly detection.

**Syntax:**

```hcl
anomaly_detection {
  enabled = true
  baseline_window = "..."
  min_samples = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `baseline_window` | `string` | No | e.g., "7d" |
| `min_samples` | `number` | No | Min hits before alerting |
| `spike_stddev` | `number` | No | Alert if > N stddev |
| `drop_stddev` | `number` | No | Alert if < N stddev |
| `alert_cooldown` | `string` | No | e.g., "15m" |
| `port_scan_threshold` | `number` | No | Ports hit before alert |

## api

API configuration

**Syntax:**

```hcl
api {
  enabled = true
  disable_sandbox = true
  listen = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `disable_sandbox` | `bool` | No | Default: false (Sandbox Enabled) |
| `listen` | `string` | No | ⚠️ *Deprecated.* Deprecated: use web.listen |
| `tls_listen` | `string` | No | ⚠️ *Deprecated.* Deprecated: use web.tls_listen |
| `tls_cert` | `string` | No | ⚠️ *Deprecated.* Deprecated: use web.tls_cert |
| `tls_key` | `string` | No | ⚠️ *Deprecated.* Deprecated: use web.tls_key |
| `disable_http_redirect` | `bool` | No | ⚠️ *Deprecated.* Deprecated: use web.disable_redirect |
| `require_auth` | `bool` | No | Require API key auth |
| `bootstrap_key` | `string` | No | Bootstrap key (for initial setup, should be removed after creating real keys) |
| `key_store_path` | `string` | No | API key storage Path to key store file |
| `cors_origins` | `list(string)` | No | CORS settings |

**Nested Blocks:**

- `key` (multiple allowed) - Predefined API keys (for config-based key management)
- `letsencrypt` - Let's Encrypt automatic TLS

### key

Predefined API keys (for config-based key management)

**Syntax:**

```hcl
key "name" {
  key = "..."
  permissions = [...]
  allowed_ips = [...]
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `key` | `string` | Yes | The actual key value |
| `permissions` | `list(string)` | Yes | Permission strings |
| `allowed_ips` | `list(string)` | No |  |
| `allowed_paths` | `list(string)` | No |  |
| `rate_limit` | `number` | No |  |
| `enabled` | `bool` | No |  |
| `description` | `string` | No |  |

### letsencrypt

Let's Encrypt automatic TLS

**Syntax:**

```hcl
letsencrypt {
  enabled = true
  email = "..."
  domain = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `email` | `string` | Yes | Contact email for certificate |
| `domain` | `string` | Yes | Domain name for certificate |
| `cache_dir` | `string` | No | Certificate cache directory |
| `staging` | `bool` | No | Use staging server for testing |

## audit

Audit logging configuration

**Syntax:**

```hcl
audit {
  enabled = true
  retention_days = 0
  kernel_audit = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enabled activates audit logging to SQLite. |
| `retention_days` | `number` | No | RetentionDays is the number of days to retain audit events. Default: 90 days. |
| `kernel_audit` | `bool` | No | KernelAudit enables writing to the Linux kernel audit log (auditd). Useful for compliance with SO... |
| `database_path` | `string` | No | DatabasePath overrides the default audit database location. Default: /var/lib/flywall/audit.db |

## ddns

Dynamic DNS

**Syntax:**

```hcl
ddns {
  enabled = true
  provider = "..."
  hostname = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `provider` | `string` | Yes | duckdns, cloudflare, noip |
| `hostname` | `string` | Yes | Hostname to update |
| `token` | `string` | No | API token/password |
| `username` | `string` | No | For providers requiring username |
| `zone_id` | `string` | No | For Cloudflare |
| `record_id` | `string` | No | For Cloudflare |
| `interface` | `string` | No | Interface to get IP from |
| `interval` | `number` | No | Update interval in minutes (default: 5) |

## dhcp

DHCPServer configuration.

**Syntax:**

```hcl
dhcp {
  enabled = true
  mode = "..."
  external_lease_file = "..."
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `mode` | `string` | No | Mode specifies how DHCP server is managed:   - "builtin" (default): Use Flywall's built-in DHCP s... |
| `external_lease_file` | `string` | No | ExternalLeaseFile is the path to external DHCP server's lease file (for import mode) |

**Nested Blocks:**

- `scope` (multiple allowed) - DHCPScope defines a DHCP pool.

### scope

DHCPScope defines a DHCP pool.

**Syntax:**

```hcl
scope "name" {
  interface = "..."
  range_start = "..."
  range_end = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | Yes |  |
| `range_start` | `string` | Yes |  |
| `range_end` | `string` | Yes |  |
| `router` | `string` | Yes |  |
| `dns` | `list(string)` | No |  |
| `lease_time` | `string` | No | e.g. "24h" |
| `domain` | `string` | No |  |
| `options` | `map` | No | Custom DHCP options using named options or numeric codes (1-255) Named options: dns_server = "8.8... Values: `str:tftp.boot`, `150` |
| `range_start_v6` | `string` | No | IPv6 Support (SLAAC/DHCPv6) For Stateful DHCPv6 |
| `range_end_v6` | `string` | No |  |
| `dns_v6` | `list(string)` | No |  |

**Nested Blocks:**

- `reservation` (multiple allowed) - DHCPReservation defines a static IP assignment for a MAC address.

#### reservation

DHCPReservation defines a static IP assignment for a MAC address.

**Syntax:**

```hcl
reservation "mac" {
  ip = "..."
  hostname = "..."
  description = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `mac` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `ip` | `string` | Yes |  |
| `hostname` | `string` | No |  |
| `description` | `string` | No |  |
| `options` | `map` | No | Per-host custom DHCP options (same format as scope options) |
| `register_dns` | `bool` | No | DNS integration Auto-register in DNS |

## dns

New consolidated DNS config

**Syntax:**

```hcl
dns {
  mode = "..."
  forwarders = [...]
  upstream_timeout = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | No | Resolution mode for upstream queries:   - "forward" (default): Forward queries to upstream DNS se... |
| `forwarders` | `list(string)` | No | Upstream DNS servers for forwarding mode |
| `upstream_timeout` | `number` | No | seconds |
| `dnssec` | `bool` | No | DNSSEC validation for upstream queries |
| `egress_filter` | `bool` | No | Egress Filter (DNS Wall) If enabled, firewall blocks outbound traffic to IPs not recently resolve... |
| `egress_filter_ttl` | `number` | No | Seconds (default: matches record TTL) |

**Nested Blocks:**

- `conditional_forward` (multiple allowed) - ConditionalForward routes specific domains to specific DNS servers.
- `upstream_doh` (multiple allowed) - Encrypted DNS upstreams
- `upstream_dot` (multiple allowed) - DNSOverTLS configures a DNS-over-TLS upstream server.
- `upstream_dnscrypt` (multiple allowed) - DNSCryptUpstream configures a DNSCrypt upstream server.
- `recursive` - Recursive resolver settings (when mode = "recursive")
- `serve` (multiple allowed) - Zone-based serving and inspection
- `inspect` (multiple allowed) - DNSInspect configures DNS traffic inspection for a zone. Used for transparent...

### conditional_forward

ConditionalForward routes specific domains to specific DNS servers.

**Syntax:**

```hcl
conditional_forward "domain" {
  servers = [...]
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `domain` | e.g., "corp.example.com" | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `servers` | `list(string)` | Yes | DNS servers for this domain |

### upstream_doh

Encrypted DNS upstreams

**Syntax:**

```hcl
upstream_doh "name" {
  url = "..."
  bootstrap = "..."
  server_name = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | `string` | Yes | e.g., "https://cloudflare-dns.com/dns-query" |
| `bootstrap` | `string` | No | IP to use for initial connection |
| `server_name` | `string` | No | SNI override |
| `enabled` | `bool` | No |  |
| `priority` | `number` | No | Lower = preferred |

### upstream_dot

DNSOverTLS configures a DNS-over-TLS upstream server.

**Syntax:**

```hcl
upstream_dot "name" {
  server = "..."
  server_name = "..."
  enabled = true
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `server` | `string` | Yes | IP:port or hostname:port |
| `server_name` | `string` | No | For TLS verification |
| `enabled` | `bool` | No |  |
| `priority` | `number` | No |  |

### upstream_dnscrypt

DNSCryptUpstream configures a DNSCrypt upstream server.

**Syntax:**

```hcl
upstream_dnscrypt "name" {
  stamp = "..."
  provider_name = "..."
  server_addr = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `stamp` | `string` | No | DNS stamp (sdns://...) |
| `provider_name` | `string` | No | Provider name for manual config |
| `server_addr` | `string` | No | Server address (IP:port) |
| `public_key` | `string` | No | Server public key (hex) |
| `enabled` | `bool` | No |  |
| `priority` | `number` | No |  |

### recursive

Recursive resolver settings (when mode = "recursive")

**Syntax:**

```hcl
recursive {
  root_hints_file = "..."
  auto_update_root_hints = true
  max_depth = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `root_hints_file` | `string` | No | Root hints for recursive resolution Path to root hints file |
| `auto_update_root_hints` | `bool` | No |  |
| `max_depth` | `number` | No | Query settings Max recursion depth (default 30) |
| `query_timeout` | `number` | No | Per-query timeout in ms |
| `max_concurrent` | `number` | No | Max concurrent outbound queries |
| `harden_glue` | `bool` | No | Hardening Validate glue records |
| `harden_dnssec_stripped` | `bool` | No | Require DNSSEC if expected |
| `harden_below_nxdomain` | `bool` | No | RFC 8020 compliance |
| `harden_referral_path` | `bool` | No | Validate referral path |
| `qname_minimisation` | `bool` | No | Privacy RFC 7816 |
| `aggressive_nsec` | `bool` | No | RFC 8198 |
| `prefetch` | `bool` | No | Prefetching Prefetch expiring entries |
| `prefetch_key` | `bool` | No | Prefetch DNSKEY records |

### serve

Zone-based serving and inspection

**Syntax:**

```hcl
serve "zone" {
  listen_port = 0
  local_domain = "..."
  expand_hosts = true
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `zone` | Zone name (label) - can use wildcards | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `listen_port` | `number` | No | Listen configuration Default 53 |
| `local_domain` | `string` | No | Local domain configuration |
| `expand_hosts` | `bool` | No |  |
| `dhcp_integration` | `bool` | No |  |
| `authoritative_for` | `string` | No |  |
| `rebind_protection` | `bool` | No | Security |
| `query_logging` | `bool` | No |  |
| `rate_limit_per_sec` | `number` | No |  |
| `allowlist` | `list(string)` | No |  |
| `blocked_ttl` | `number` | No |  |
| `blocked_address` | `string` | No |  |
| `cache_enabled` | `bool` | No | Caching |
| `cache_size` | `number` | No |  |
| `cache_min_ttl` | `number` | No |  |
| `cache_max_ttl` | `number` | No |  |
| `negative_cache_ttl` | `number` | No |  |

**Nested Blocks:**

- `blocklist` (multiple allowed) - Filtering
- `doh_server` - Encrypted DNS servers (serve DoH/DoT to clients in this zone)
- `dot_server` - DoTServerConfig configures the DNS-over-TLS server.
- `dnscrypt_server` - DNSCryptServerConfig configures the DNSCrypt server (serve DNSCrypt to clients).
- `host` (multiple allowed) - Static entries and zones
- `zone` (multiple allowed) - DNSZone configuration for authoritative zones.

#### blocklist

Filtering

**Syntax:**

```hcl
blocklist "name" {
  url = "..."
  file = "..."
  format = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | `string` | No | URL to fetch blocklist |
| `file` | `string` | No | Local file path |
| `format` | `string` | No | hosts, domains, adblock |
| `enabled` | `bool` | No |  |
| `refresh_hours` | `number` | No |  |

#### doh_server

Encrypted DNS servers (serve DoH/DoT to clients in this zone)

**Syntax:**

```hcl
doh_server {
  enabled = true
  listen_addr = "..."
  path = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `listen_addr` | `string` | No | Default: :443 |
| `path` | `string` | No | Default: /dns-query |
| `cert_file` | `string` | No |  |
| `key_file` | `string` | No |  |
| `use_letsencrypt` | `bool` | No |  |
| `domain` | `string` | No | For Let's Encrypt |

#### dot_server

DoTServerConfig configures the DNS-over-TLS server.

**Syntax:**

```hcl
dot_server {
  enabled = true
  listen_addr = "..."
  cert_file = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `listen_addr` | `string` | No | Default: :853 |
| `cert_file` | `string` | No |  |
| `key_file` | `string` | No |  |

#### dnscrypt_server

DNSCryptServerConfig configures the DNSCrypt server (serve DNSCrypt to clients).

**Syntax:**

```hcl
dnscrypt_server {
  enabled = true
  listen_addr = "..."
  provider_name = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `listen_addr` | `string` | No | Default: :5443 |
| `provider_name` | `string` | No | e.g., "2.dnscrypt-cert.example.com" |
| `public_key_file` | `string` | No |  |
| `secret_key_file` | `string` | No |  |
| `cert_file` | `string` | No | DNSCrypt certificate |
| `cert_ttl` | `number` | No | Certificate TTL in hours |
| `es_version` | `number` | No | Encryption suite: 1=XSalsa20Poly1305, 2=XChacha20Poly1305 |

#### host

Static entries and zones

**Syntax:**

```hcl
host "ip" {
  hostnames = [...]
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `ip` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `hostnames` | `list(string)` | Yes |  |

#### zone

DNSZone configuration for authoritative zones.

**Syntax:**

```hcl
zone "name" {
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Nested Blocks:**

- `record` (multiple allowed) - DNSRecord configuration.

##### record

DNSRecord configuration.

**Syntax:**

```hcl
record "name" {
  type = "..."
  value = "..."
  ttl = 0
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | `string` | Yes | A, AAAA, CNAME, MX, TXT, PTR, SRV |
| `value` | `string` | Yes | IP or target |
| `ttl` | `number` | No |  |
| `priority` | `number` | No | For MX, SRV |

### inspect

DNSInspect configures DNS traffic inspection for a zone.
Used for transparent interception or passive visibility.

**Syntax:**

```hcl
inspect "zone" {
  mode = "..."
  exclude_router = true
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `zone` | Zone name (label) - can use wildcards | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | No | Mode determines what happens to intercepted traffic:   - "redirect": DNAT to local DNS server (re... |
| `exclude_router` | `bool` | No | ExcludeRouter prevents redirecting the router's own DNS traffic |

## dns_server

> ⚠️ **Deprecated:** use DNS

Deprecated: use DNS

**Syntax:**

```hcl
dns_server {
  enabled = true
  listen_on = [...]
  listen_port = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `listen_on` | `list(string)` | No |  |
| `listen_port` | `number` | No | Default 53 |
| `local_domain` | `string` | No | Local domain configuration e.g., "lan", "home.arpa" Values: `lan`, `home.arpa` |
| `expand_hosts` | `bool` | No | Append local domain to simple hostnames |
| `dhcp_integration` | `bool` | No | Auto-register DHCP hostnames |
| `authoritative_for` | `string` | No | Return NXDOMAIN for unknown local hosts |
| `mode` | `string` | No | Resolution mode:   - "forward" (default): Forward queries to upstream DNS servers   - "recursive"... |
| `forwarders` | `list(string)` | No | Upstream DNS (for forwarding mode) |
| `upstream_timeout` | `number` | No | seconds |
| `dnssec` | `bool` | No | Security Validate DNSSEC |
| `rebind_protection` | `bool` | No | Block private IPs in public responses |
| `query_logging` | `bool` | No |  |
| `rate_limit_per_sec` | `number` | No | Per-client rate limit |
| `allowlist` | `list(string)` | No | Domains that bypass blocklists |
| `blocked_ttl` | `number` | No | TTL for blocked responses |
| `blocked_address` | `string` | No | IP to return for blocked (default 0.0.0.0) |
| `cache_enabled` | `bool` | No | Caching |
| `cache_size` | `number` | No | Max entries |
| `cache_min_ttl` | `number` | No | Minimum TTL to cache |
| `cache_max_ttl` | `number` | No | Maximum TTL to cache |
| `negative_cache_ttl` | `number` | No | TTL for NXDOMAIN |

**Nested Blocks:**

- `conditional_forward` (multiple allowed) - ConditionalForward routes specific domains to specific DNS servers.
- `upstream_doh` (multiple allowed) - Encrypted DNS - Upstream (client mode) DNS-over-HTTPS upstreams
- `upstream_dot` (multiple allowed) - DNS-over-TLS upstreams
- `upstream_dnscrypt` (multiple allowed) - DNSCrypt upstreams
- `doh_server` - Encrypted DNS - Server (serve DoH/DoT/DNSCrypt to clients) Serve DNS-over-HTTPS
- `dot_server` - Serve DNS-over-TLS
- `dnscrypt_server` - Serve DNSCrypt
- `recursive` - Recursive resolver settings (when mode = "recursive")
- `blocklist` (multiple allowed) - Filtering
- `host` (multiple allowed) - Static entries and zones Static /etc/hosts style entries
- `zone` (multiple allowed) - DNSZone configuration for authoritative zones.

### conditional_forward

ConditionalForward routes specific domains to specific DNS servers.

**Syntax:**

```hcl
conditional_forward "domain" {
  servers = [...]
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `domain` | e.g., "corp.example.com" | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `servers` | `list(string)` | Yes | DNS servers for this domain |

### upstream_doh

Encrypted DNS - Upstream (client mode) DNS-over-HTTPS upstreams

**Syntax:**

```hcl
upstream_doh "name" {
  url = "..."
  bootstrap = "..."
  server_name = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | `string` | Yes | e.g., "https://cloudflare-dns.com/dns-query" |
| `bootstrap` | `string` | No | IP to use for initial connection |
| `server_name` | `string` | No | SNI override |
| `enabled` | `bool` | No |  |
| `priority` | `number` | No | Lower = preferred |

### upstream_dot

DNS-over-TLS upstreams

**Syntax:**

```hcl
upstream_dot "name" {
  server = "..."
  server_name = "..."
  enabled = true
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `server` | `string` | Yes | IP:port or hostname:port |
| `server_name` | `string` | No | For TLS verification |
| `enabled` | `bool` | No |  |
| `priority` | `number` | No |  |

### upstream_dnscrypt

DNSCrypt upstreams

**Syntax:**

```hcl
upstream_dnscrypt "name" {
  stamp = "..."
  provider_name = "..."
  server_addr = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `stamp` | `string` | No | DNS stamp (sdns://...) |
| `provider_name` | `string` | No | Provider name for manual config |
| `server_addr` | `string` | No | Server address (IP:port) |
| `public_key` | `string` | No | Server public key (hex) |
| `enabled` | `bool` | No |  |
| `priority` | `number` | No |  |

### doh_server

Encrypted DNS - Server (serve DoH/DoT/DNSCrypt to clients) Serve DNS-over-HTTPS

**Syntax:**

```hcl
doh_server {
  enabled = true
  listen_addr = "..."
  path = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `listen_addr` | `string` | No | Default: :443 |
| `path` | `string` | No | Default: /dns-query |
| `cert_file` | `string` | No |  |
| `key_file` | `string` | No |  |
| `use_letsencrypt` | `bool` | No |  |
| `domain` | `string` | No | For Let's Encrypt |

### dot_server

Serve DNS-over-TLS

**Syntax:**

```hcl
dot_server {
  enabled = true
  listen_addr = "..."
  cert_file = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `listen_addr` | `string` | No | Default: :853 |
| `cert_file` | `string` | No |  |
| `key_file` | `string` | No |  |

### dnscrypt_server

Serve DNSCrypt

**Syntax:**

```hcl
dnscrypt_server {
  enabled = true
  listen_addr = "..."
  provider_name = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `listen_addr` | `string` | No | Default: :5443 |
| `provider_name` | `string` | No | e.g., "2.dnscrypt-cert.example.com" |
| `public_key_file` | `string` | No |  |
| `secret_key_file` | `string` | No |  |
| `cert_file` | `string` | No | DNSCrypt certificate |
| `cert_ttl` | `number` | No | Certificate TTL in hours |
| `es_version` | `number` | No | Encryption suite: 1=XSalsa20Poly1305, 2=XChacha20Poly1305 |

### recursive

Recursive resolver settings (when mode = "recursive")

**Syntax:**

```hcl
recursive {
  root_hints_file = "..."
  auto_update_root_hints = true
  max_depth = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `root_hints_file` | `string` | No | Root hints for recursive resolution Path to root hints file |
| `auto_update_root_hints` | `bool` | No |  |
| `max_depth` | `number` | No | Query settings Max recursion depth (default 30) |
| `query_timeout` | `number` | No | Per-query timeout in ms |
| `max_concurrent` | `number` | No | Max concurrent outbound queries |
| `harden_glue` | `bool` | No | Hardening Validate glue records |
| `harden_dnssec_stripped` | `bool` | No | Require DNSSEC if expected |
| `harden_below_nxdomain` | `bool` | No | RFC 8020 compliance |
| `harden_referral_path` | `bool` | No | Validate referral path |
| `qname_minimisation` | `bool` | No | Privacy RFC 7816 |
| `aggressive_nsec` | `bool` | No | RFC 8198 |
| `prefetch` | `bool` | No | Prefetching Prefetch expiring entries |
| `prefetch_key` | `bool` | No | Prefetch DNSKEY records |

### blocklist

Filtering

**Syntax:**

```hcl
blocklist "name" {
  url = "..."
  file = "..."
  format = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | `string` | No | URL to fetch blocklist |
| `file` | `string` | No | Local file path |
| `format` | `string` | No | hosts, domains, adblock |
| `enabled` | `bool` | No |  |
| `refresh_hours` | `number` | No |  |

### host

Static entries and zones Static /etc/hosts style entries

**Syntax:**

```hcl
host "ip" {
  hostnames = [...]
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `ip` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `hostnames` | `list(string)` | Yes |  |

### zone

DNSZone configuration for authoritative zones.

**Syntax:**

```hcl
zone "name" {
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Nested Blocks:**

- `record` (multiple allowed) - DNSRecord configuration.

#### record

DNSRecord configuration.

**Syntax:**

```hcl
record "name" {
  type = "..."
  value = "..."
  ttl = 0
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | `string` | Yes | A, AAAA, CNAME, MX, TXT, PTR, SRV |
| `value` | `string` | Yes | IP or target |
| `ttl` | `number` | No |  |
| `priority` | `number` | No | For MX, SRV |

## features

Feature Flags

**Syntax:**

```hcl
features {
  threat_intel = true
  network_learning = true
  qos = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `threat_intel` | `bool` | No | Phase 5: Threat Intelligence |
| `network_learning` | `bool` | No | Automated rule learning |
| `qos` | `bool` | No | Traffic Shaping |
| `integrity_monitoring` | `bool` | No | Detect and revert external changes |

## frr

FRRConfig holds configuration for Free Range Routing (FRR).

**Syntax:**

```hcl
frr {
  enabled = true
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |

**Nested Blocks:**

- `ospf` - OSPF configuration.
- `bgp` - BGP configuration.

### ospf

OSPF configuration.

**Syntax:**

```hcl
ospf {
  router_id = "..."
  networks = [...]
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `router_id` | `string` | No |  |
| `networks` | `list(string)` | No | List of CIDRs to advertise |

**Nested Blocks:**

- `area` (multiple allowed) - OSPFArea configuration.

#### area

OSPFArea configuration.

**Syntax:**

```hcl
area "id" {
  networks = [...]
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `id` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `networks` | `list(string)` | No |  |

### bgp

BGP configuration.

**Syntax:**

```hcl
bgp {
  asn = 0
  router_id = "..."
  networks = [...]
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `asn` | `number` | No |  |
| `router_id` | `string` | No |  |
| `networks` | `list(string)` | No |  |

**Nested Blocks:**

- `neighbor` (multiple allowed) - Neighbor BGP peer configuration.

#### neighbor

Neighbor BGP peer configuration.

**Syntax:**

```hcl
neighbor "ip" {
  remote_asn = 0
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `ip` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `remote_asn` | `number` | No |  |

## geoip

GeoIP configuration for country-based filtering

**Syntax:**

```hcl
geoip {
  enabled = true
  database_path = "..."
  auto_update = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enabled activates GeoIP matching in firewall rules. |
| `database_path` | `string` | No | DatabasePath is the path to the MMDB file (MaxMind or DB-IP format). Default: /var/lib/flywall/ge... |
| `auto_update` | `bool` | No | AutoUpdate enables automatic database updates (future feature). |
| `license_key` | `string` | No | LicenseKey for premium MaxMind database updates (future feature). Not required for DB-IP or GeoLi... |

## interface

Interface represents a physical or virtual network interface configuration.
Each interface can be assigned to a security zone and configured with
static IPs, DHCP, VLANs, and other network settings.

**Syntax:**

```hcl
interface "name" {
  description = "WAN Uplink"
  disabled = false
  zone = "wan"
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` | Name is the system interface name. | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | `string` | No | Human-readable description for this interface. |
| `disabled` | `bool` | No (default: `false`) | Temporarily disable this interface (brings it down). |
| `zone` | `string` | No | Assign this interface to a security zone. |
| `ipv4` | `list(string)` | No | Static IPv4 addresses in CIDR notation. |
| `ipv6` | `list(string)` | No | Static IPv6 addresses in CIDR notation. |
| `dhcp` | `bool` | No (default: `false`) | Enable DHCP client on this interface. |
| `dhcp_v6` | `bool` | No (default: `false`) | Enable DHCPv6 client for IPv6 address assignment. |
| `ra` | `bool` | No (default: `false`) | Enable Router Advertisements (for IPv6 server mode). |
| `dhcp_client` | `string` | No | DHCPClient specifies how DHCP client is managed:   - "builtin" (default): Use Flywall's built-in ... |
| `table` | `number` | No | Table specifies the routing table ID for this interface. If set to > 0 (and not 254/main), Flywal... |
| `gateway` | `string` | No | Default gateway for static IPv4 configuration. |
| `gateway_v6` | `string` | No | Default gateway for static IPv6 configuration. |
| `mtu` | `number` | No (default: `1500`) | Maximum Transmission Unit size in bytes. |
| `disable_anti_lockout` | `bool` | No | Anti-Lockout protection (sandbox mode only) When true, implicit accept rules are created for this... |
| `access_web_ui` | `bool` | No | ⚠️ *Deprecated.* Web UI / API Access Deprecated: Use Management block instead |
| `web_ui_port` | `number` | No | ⚠️ *Deprecated.* Deprecated: Use Management block instead Port to map (external) |

**Nested Blocks:**

- `new_zone` - Create and assign a new zone inline.
- `bond` - Bond represents the configuration for a bonding interface.
- `vlan` (multiple allowed) - VLAN represents a VLAN configuration nested within an interface.
- `management` - Management Access (Interface specific overrides)
- `tls` - TLS/Certificate configuration for this interface

### new_zone

Create and assign a new zone inline.

**Syntax:**

```hcl
new_zone "name" {
  color = "..."
  description = "..."
  interface = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `color` | `string` | No |  |
| `description` | `string` | No |  |
| `interface` | `string` | No | Simple match criteria (use for single-interface zones) These are effectively a single implicit ma... |
| `src` | `string` | No | Source IP/network (e.g., "192.168.1.0/24") |
| `dst` | `string` | No | Destination IP/network |
| `vlan` | `number` | No | VLAN tag |
| `interfaces` | `list(string)` | No | ⚠️ *Deprecated.* DEPRECATED: Use Interface or Matches instead Will be auto-converted to Matches with warning |
| `ipsets` | `list(string)` | No | Legacy fields (kept for backwards compat) IPSet names for IP-based membership |
| `networks` | `list(string)` | No | Direct CIDR ranges |
| `action` | `string` | No | Zone behavior Action for intra-zone traffic: "accept", "drop", "reject" (default: accept) Values: `accept`, `reject` |
| `external` | `bool` | No | External marks this as an external/WAN zone (used for auto-masquerade detection) If not set, dete... |
| `ipv4` | `list(string)` | No | IP assignment for simple zones (shorthand - assigns to the interface) |
| `ipv6` | `list(string)` | No |  |
| `dhcp` | `bool` | No | Use DHCP client on this interface |

**Nested Blocks:**

- `match` (multiple allowed) - Complex match criteria (OR logic between matches, AND logic within each match...
- `services` - Services provided TO this zone (firewall auto-generates rules) These define w...
- `management` - Management access FROM this zone to the firewall

#### match

Complex match criteria (OR logic between matches, AND logic within each match)
Global fields above apply to ALL matches as defaults

**Syntax:**

```hcl
match {
  interface = "..."
  src = "..."
  dst = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | No | Interface can be exact ("eth0") or prefix with + or * suffix ("wg+" matches wg0, wg1...) |
| `src` | `string` | No |  |
| `dst` | `string` | No |  |
| `vlan` | `number` | No |  |

#### services

Services provided TO this zone (firewall auto-generates rules)
These define what the firewall offers to clients in this zone

**Syntax:**

```hcl
services {
  dhcp = true
  dns = true
  ntp = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `dhcp` | `bool` | No | Network services Allow DHCP requests (udp/67-68) |
| `dns` | `bool` | No | Allow DNS queries (udp/53, tcp/53) |
| `ntp` | `bool` | No | Allow NTP sync (udp/123) |
| `captive_portal` | `bool` | No | Captive portal / guest access Redirect HTTP to portal |

**Nested Blocks:**

- `port` (multiple allowed) - Custom service ports (auto-allow)

##### port

Custom service ports (auto-allow)

**Syntax:**

```hcl
port "name" {
  protocol = "..."
  port = 0
  port_end = 0
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `protocol` | `string` | Yes | tcp, udp |
| `port` | `number` | Yes | Port number |
| `port_end` | `number` | No | For port ranges |

#### management

Management access FROM this zone to the firewall

**Syntax:**

```hcl
management {
  web_ui = true
  web = true
  ssh = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `web_ui` | `bool` | No | Legacy: Allow Web UI access (tcp/80, tcp/443) -> Use Web |
| `web` | `bool` | No | Allow Web UI access (tcp/80, tcp/443) |
| `ssh` | `bool` | No | Allow SSH access (tcp/22) |
| `api` | `bool` | No | Allow API access (used for L7 filtering, implies HTTPS access) |
| `icmp` | `bool` | No | Allow ping to firewall |
| `snmp` | `bool` | No | Allow SNMP queries (udp/161) |
| `syslog` | `bool` | No | Allow syslog sending (udp/514) |

### bond

Bond represents the configuration for a bonding interface.

**Syntax:**

```hcl
bond {
  mode = "..."
  interfaces = [...]
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | No |  |
| `interfaces` | `list(string)` | No |  |

### vlan

VLAN represents a VLAN configuration nested within an interface.

**Syntax:**

```hcl
vlan "id" {
  description = "..."
  zone = "..."
  ipv4 = [...]
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `id` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | `string` | No |  |
| `zone` | `string` | No |  |
| `ipv4` | `list(string)` | No |  |
| `ipv6` | `list(string)` | No |  |

**Nested Blocks:**

- `new_zone` - Create zone inline

#### new_zone

Create zone inline

**Syntax:**

```hcl
new_zone "name" {
  color = "..."
  description = "..."
  interface = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `color` | `string` | No |  |
| `description` | `string` | No |  |
| `interface` | `string` | No | Simple match criteria (use for single-interface zones) These are effectively a single implicit ma... |
| `src` | `string` | No | Source IP/network (e.g., "192.168.1.0/24") |
| `dst` | `string` | No | Destination IP/network |
| `vlan` | `number` | No | VLAN tag |
| `interfaces` | `list(string)` | No | ⚠️ *Deprecated.* DEPRECATED: Use Interface or Matches instead Will be auto-converted to Matches with warning |
| `ipsets` | `list(string)` | No | Legacy fields (kept for backwards compat) IPSet names for IP-based membership |
| `networks` | `list(string)` | No | Direct CIDR ranges |
| `action` | `string` | No | Zone behavior Action for intra-zone traffic: "accept", "drop", "reject" (default: accept) Values: `accept`, `reject` |
| `external` | `bool` | No | External marks this as an external/WAN zone (used for auto-masquerade detection) If not set, dete... |
| `ipv4` | `list(string)` | No | IP assignment for simple zones (shorthand - assigns to the interface) |
| `ipv6` | `list(string)` | No |  |
| `dhcp` | `bool` | No | Use DHCP client on this interface |

**Nested Blocks:**

- `match` (multiple allowed) - Complex match criteria (OR logic between matches, AND logic within each match...
- `services` - Services provided TO this zone (firewall auto-generates rules) These define w...
- `management` - Management access FROM this zone to the firewall

##### match

Complex match criteria (OR logic between matches, AND logic within each match)
Global fields above apply to ALL matches as defaults

**Syntax:**

```hcl
match {
  interface = "..."
  src = "..."
  dst = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | No | Interface can be exact ("eth0") or prefix with + or * suffix ("wg+" matches wg0, wg1...) |
| `src` | `string` | No |  |
| `dst` | `string` | No |  |
| `vlan` | `number` | No |  |

##### services

Services provided TO this zone (firewall auto-generates rules)
These define what the firewall offers to clients in this zone

**Syntax:**

```hcl
services {
  dhcp = true
  dns = true
  ntp = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `dhcp` | `bool` | No | Network services Allow DHCP requests (udp/67-68) |
| `dns` | `bool` | No | Allow DNS queries (udp/53, tcp/53) |
| `ntp` | `bool` | No | Allow NTP sync (udp/123) |
| `captive_portal` | `bool` | No | Captive portal / guest access Redirect HTTP to portal |

**Nested Blocks:**

- `port` (multiple allowed) - Custom service ports (auto-allow)

###### port

Custom service ports (auto-allow)

**Syntax:**

```hcl
port "name" {
  protocol = "..."
  port = 0
  port_end = 0
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `protocol` | `string` | Yes | tcp, udp |
| `port` | `number` | Yes | Port number |
| `port_end` | `number` | No | For port ranges |

##### management

Management access FROM this zone to the firewall

**Syntax:**

```hcl
management {
  web_ui = true
  web = true
  ssh = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `web_ui` | `bool` | No | Legacy: Allow Web UI access (tcp/80, tcp/443) -> Use Web |
| `web` | `bool` | No | Allow Web UI access (tcp/80, tcp/443) |
| `ssh` | `bool` | No | Allow SSH access (tcp/22) |
| `api` | `bool` | No | Allow API access (used for L7 filtering, implies HTTPS access) |
| `icmp` | `bool` | No | Allow ping to firewall |
| `snmp` | `bool` | No | Allow SNMP queries (udp/161) |
| `syslog` | `bool` | No | Allow syslog sending (udp/514) |

### management

Management Access (Interface specific overrides)

**Syntax:**

```hcl
management {
  web_ui = true
  web = true
  ssh = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `web_ui` | `bool` | No | Legacy: Allow Web UI access (tcp/80, tcp/443) -> Use Web |
| `web` | `bool` | No | Allow Web UI access (tcp/80, tcp/443) |
| `ssh` | `bool` | No | Allow SSH access (tcp/22) |
| `api` | `bool` | No | Allow API access (used for L7 filtering, implies HTTPS access) |
| `icmp` | `bool` | No | Allow ping to firewall |
| `snmp` | `bool` | No | Allow SNMP queries (udp/161) |
| `syslog` | `bool` | No | Allow syslog sending (udp/514) |

### tls

TLS/Certificate configuration for this interface

**Syntax:**

```hcl
tls {
  mode = "self-signed"
  hostname = "..."
  email = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | No | "self-signed", "acme", "tailscale", "manual" Values: `self-signed`, `manual` |
| `hostname` | `string` | No | For Tailscale mode |
| `email` | `string` | No | ACME (Let's Encrypt) settings |
| `domains` | `list(string)` | No |  |
| `cert_file` | `string` | No | Manual certificate (bring your own) |
| `key_file` | `string` | No |  |

## ipset

IPSet defines a named set of IPs/networks for use in firewall rules.

**Syntax:**

```hcl
ipset "name" {
  description = "..."
  type = "..."
  entries = [...]
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | `string` | No |  |
| `type` | `string` | No | ipv4_addr (default), ipv6_addr, inet_service, or dns |
| `entries` | `list(string)` | No |  |
| `domains` | `list(string)` | No | Domains for dynamic resolution (only for type="dns") |
| `refresh_interval` | `string` | No | Refresh interval for DNS resolution (e.g., "5m", "1h") - only for type="dns" Values: `5m`, `1h` |
| `size` | `number` | No | Optimization: Pre-allocated size for dynamic sets to prevent resizing (suggested: 65535) |
| `firehol_list` | `string` | No | FireHOL import e.g., "firehol_level1", "spamhaus_drop" Values: `firehol_level1`, `spamhaus_drop` |
| `url` | `string` | No | Custom URL for IP list |
| `refresh_hours` | `number` | No | How often to refresh (default: 24) |
| `auto_update` | `bool` | No | Enable automatic updates |
| `action` | `string` | No | drop, reject, log (for auto-generated rules) |
| `apply_to` | `string` | No | input, forward, both (for auto-generated rules) |
| `match_on_source` | `bool` | No | Match source IP (default: true) |
| `match_on_dest` | `bool` | No | Match destination IP |

## mark_rule

MarkRule represents a rule for setting routing marks on packets.
Marks are set in nftables and matched by ip rule for routing decisions.

**Syntax:**

```hcl
mark_rule "name" {
  mark = "..."
  mask = "..."
  proto = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mark` | `string` | Yes | Mark value to set (hex: 0x10) |
| `mask` | `string` | No | Mask for mark operations |
| `proto` | `string` | No | Match criteria tcp, udp, icmp, all |
| `src_ip` | `string` | No |  |
| `dst_ip` | `string` | No |  |
| `src_port` | `number` | No |  |
| `dst_port` | `number` | No |  |
| `dst_ports` | `list(number)` | No | Multiple ports |
| `in_interface` | `string` | No |  |
| `out_interface` | `string` | No |  |
| `src_zone` | `string` | No |  |
| `dst_zone` | `string` | No |  |
| `ipset` | `string` | No | Match against IPSet |
| `conn_state` | `list(string)` | No | NEW, ESTABLISHED, etc. |
| `save_mark` | `bool` | No | Mark behavior Save to conntrack |
| `restore_mark` | `bool` | No | Restore from conntrack |
| `enabled` | `bool` | No |  |
| `comment` | `string` | No |  |

## mdns

mDNS Reflector configuration

**Syntax:**

```hcl
mdns {
  enabled = true
  interfaces = [...]
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `interfaces` | `list(string)` | No | Interfaces to reflect between |

## multi_wan

MultiWAN represents multi-WAN configuration for failover and load balancing.

**Syntax:**

```hcl
multi_wan {
  enabled = true
  mode = "failover"
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `mode` | `string` | No | "failover", "loadbalance", "both" Values: `failover`, `both` |

**Nested Blocks:**

- `wan` (multiple allowed) - WANLink represents a WAN connection for multi-WAN.
- `health_check` - WANHealth configures health checking for multi-WAN.

### wan

WANLink represents a WAN connection for multi-WAN.

**Syntax:**

```hcl
wan "name" {
  interface = "..."
  gateway = "..."
  weight = 0
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | Yes |  |
| `gateway` | `string` | Yes |  |
| `weight` | `number` | No | For load balancing (1-100) |
| `priority` | `number` | No | For failover (lower = preferred) |
| `enabled` | `bool` | No |  |

### health_check

WANHealth configures health checking for multi-WAN.

**Syntax:**

```hcl
health_check {
  interval = 0
  timeout = 0
  threshold = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interval` | `number` | No | Check interval in seconds |
| `timeout` | `number` | No | Timeout per check |
| `threshold` | `number` | No | Failures before marking down |
| `targets` | `list(string)` | No | IPs to ping |
| `http_check` | `string` | No | URL for HTTP health check |

## nat

NATRule defines Network Address Translation rules.

**Syntax:**

```hcl
nat "name" {
  description = "..."
  type = "..."
  proto = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | `string` | No |  |
| `type` | `string` | Yes | masquerade, dnat, snat, redirect |
| `proto` | `string` | No | tcp, udp |
| `out_interface` | `string` | No | for masquerade/snat |
| `in_interface` | `string` | No | for dnat |
| `src_ip` | `string` | No | Source IP match |
| `dest_ip` | `string` | No | Dest IP match |
| `mark` | `number` | No | FWMark match |
| `dest_port` | `string` | No | Dest Port match (supports ranges "80-90") |
| `to_ip` | `string` | No | Target IP for DNAT |
| `to_port` | `string` | No | Target Port for DNAT |
| `snat_ip` | `string` | No | for snat (Target IP) |
| `hairpin` | `bool` | No | Enable Hairpin NAT (NAT Reflection) |

## notifications

NotificationsConfig configures the notification system.

**Syntax:**

```hcl
notifications {
  enabled = true
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |

**Nested Blocks:**

- `channel` (multiple allowed) - NotificationChannel defines a notification destination.
- `rule` (multiple allowed) - AlertRule defines when an alert should be triggered.

### channel

NotificationChannel defines a notification destination.

**Syntax:**

```hcl
channel "name" {
  type = "..."
  level = "..."
  enabled = true
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | `string` | Yes | email, pushover, slack, discord, ntfy, webhook |
| `level` | `string` | No | critical, warning, info |
| `enabled` | `bool` | No |  |
| `smtp_host` | `string` | No | Email settings |
| `smtp_port` | `number` | No |  |
| `smtp_user` | `string` | No |  |
| `smtp_password` | `string` | No |  |
| `from` | `string` | No |  |
| `to` | `list(string)` | No |  |
| `webhook_url` | `string` | No | Webhook/Slack/Discord settings |
| `channel` | `string` | No |  |
| `username` | `string` | No |  |
| `api_token` | `string` | No | Pushover settings |
| `user_key` | `string` | No |  |
| `priority` | `number` | No |  |
| `sound` | `string` | No |  |
| `server` | `string` | No | ntfy settings |
| `topic` | `string` | No |  |
| `password` | `string` | No | Generic auth (for ntfy, webhook) |
| `headers` | `map` | No |  |

### rule

AlertRule defines when an alert should be triggered.

**Syntax:**

```hcl
rule "name" {
  enabled = true
  condition = "..."
  severity = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `condition` | `string` | Yes |  |
| `severity` | `string` | No | info, warning, critical |
| `channels` | `list(string)` | No |  |
| `cooldown` | `string` | No | e.g. "1h" |

## ntp

NTP configuration

**Syntax:**

```hcl
ntp {
  enabled = true
  servers = [...]
  interval = "..."
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `servers` | `list(string)` | No | Upstream servers |
| `interval` | `string` | No | Sync interval (e.g. "4h") |

## policy

Policy defines traffic rules between zones.
Rules are evaluated in order - first match wins.

**Syntax:**

```hcl
policy "from" "to" {
  name = "..."
  description = "..."
  priority = 0
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `from` | Source zone name | Yes |
| `to` | Destination zone name | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | `string` | No | Optional descriptive name |
| `description` | `string` | No |  |
| `priority` | `number` | No | Policy priority (lower = evaluated first) |
| `disabled` | `bool` | No | Temporarily disable this policy |
| `action` | `string` | No | Action for traffic matching this policy (when no specific rule matches) Values: "accept", "drop",... Values: `accept`, `reject` |
| `masquerade` | `bool` | No | Masquerade controls NAT for outbound traffic through this policy nil = auto (enable when RFC1918 ... |
| `log` | `bool` | No | Log packets matching default action |
| `log_prefix` | `string` | No | Prefix for log messages |
| `inherits` | `string` | No | Inheritance - allows policies to inherit rules from a parent policy Child policies get all parent... |

**Nested Blocks:**

- `rule` (multiple allowed) - PolicyRule defines a specific rule within a policy. Rules are matched in the ...

### rule

PolicyRule defines a specific rule within a policy.
Rules are matched in the order they appear - first match wins.

**Syntax:**

```hcl
rule "name" {
  id = "..."
  description = "..."
  disabled = true
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | No | Identity Unique ID for referencing in reorder operations |
| `description` | `string` | No |  |
| `disabled` | `bool` | No | Temporarily disable this rule |
| `order` | `number` | No | Ordering - rules are processed in order, these help with insertion Explicit order (0 = use array ... |
| `insert_after` | `string` | No | Insert after rule with this ID/name |
| `proto` | `string` | No | Match conditions |
| `dest_port` | `number` | No |  |
| `dest_ports` | `list(number)` | No | Multiple ports |
| `src_port` | `number` | No |  |
| `src_ports` | `list(number)` | No |  |
| `services` | `list(string)` | No | Service names like "http", "ssh" Values: `http`, `ssh` |
| `src_ip` | `string` | No | Source IP/CIDR |
| `src_ipset` | `string` | No | Source IPSet name |
| `dest_ip` | `string` | No | Destination IP/CIDR |
| `dest_ipset` | `string` | No | Destination IPSet name |
| `src_zone` | `string` | No | Additional match conditions Override policy's From zone |
| `dest_zone` | `string` | No | Override policy's To zone |
| `in_interface` | `string` | No | Match specific input interface |
| `out_interface` | `string` | No | Match specific output interface |
| `conn_state` | `string` | No | "new", "established", "related", "invalid" Values: `new`, `invalid` |
| `source_country` | `string` | No | GeoIP matching (requires MaxMind database) ISO 3166-1 alpha-2 country code (e.g., "US", "CN") Values: `US`, `CN` |
| `dest_country` | `string` | No | ISO 3166-1 alpha-2 country code |
| `invert_src` | `bool` | No | Invert matching (match everything EXCEPT the specified value) Negate source IP/IPSet match |
| `invert_dest` | `bool` | No | Negate destination IP/IPSet match |
| `tcp_flags` | `string` | No | TCP Flags matching (for SYN flood protection, connection state filtering) Values: "syn", "syn,!ac... Values: `syn`, `urg` |
| `max_connections` | `number` | No | Connection limiting (prevent abuse/DoS) Max concurrent connections per source |
| `time_start` | `string` | No | Time-of-day matching (uses nftables meta hour/day, requires kernel 5.4+) Start time "HH:MM" (24h ... |
| `time_end` | `string` | No | End time "HH:MM" (24h format) |
| `days` | `list(string)` | No | Days of week: "Monday", "Tuesday", etc. Values: `Monday`, `Tuesday` |
| `action` | `string` | Yes | Action accept, drop, reject, jump, return, log |
| `jump_target` | `string` | No | Target chain for jump action |
| `log` | `bool` | No | Logging & accounting |
| `log_prefix` | `string` | No |  |
| `log_level` | `string` | No | "debug", "info", "notice", "warning", "error" Values: `debug`, `error` |
| `limit` | `string` | No | Rate limit e.g. "10/second" |
| `counter` | `string` | No | Named counter for accounting |
| `comment` | `string` | No | Metadata |
| `tags` | `list(string)` | No | For grouping/filtering in UI |
| `group` | `string` | No | UI Organization Section grouping: "User Access", "IoT Isolation" Values: `User Access`, `IoT Isolation` |

## policy_route

PolicyRoute represents a policy-based routing rule.
Policy routes use firewall marks to direct traffic to specific routing tables.

**Syntax:**

```hcl
policy_route "name" {
  priority = 0
  mark = "..."
  mark_mask = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `priority` | `number` | No | Rule priority (lower = higher priority) |
| `mark` | `string` | No | Match criteria (combined with AND) Firewall mark to match (hex: 0x10) |
| `mark_mask` | `string` | No | Mask for mark matching |
| `from` | `string` | No | Source IP/CIDR |
| `to` | `string` | No | Destination IP/CIDR |
| `iif` | `string` | No | Input interface |
| `oif` | `string` | No | Output interface |
| `fwmark` | `string` | No | Alternative mark syntax |
| `table` | `number` | No | Action Routing table to use |
| `blackhole` | `bool` | No | Drop matching packets |
| `prohibit` | `bool` | No | Return ICMP prohibited |
| `enabled` | `bool` | No | Default true |
| `comment` | `string` | No |  |

## protection

InterfaceProtection defines security protection settings for an interface.

**Syntax:**

```hcl
protection "name" {
  interface = "..."
  enabled = true
  anti_spoofing = true
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | Yes | Interface name or "*" for all |
| `enabled` | `bool` | No |  |
| `anti_spoofing` | `bool` | No | AntiSpoofing drops packets with spoofed source IPs (recommended for WAN) |
| `bogon_filtering` | `bool` | No | BogonFiltering drops packets from reserved/invalid IP ranges |
| `private_filtering` | `bool` | No | PrivateFiltering drops packets from private IP ranges on WAN (RFC1918) |
| `invalid_packets` | `bool` | No | InvalidPackets drops malformed/invalid packets |
| `syn_flood_protection` | `bool` | No | SynFloodProtection limits SYN packets to prevent SYN floods |
| `syn_flood_rate` | `number` | No | packets/sec (default: 25) |
| `syn_flood_burst` | `number` | No | burst allowance (default: 50) |
| `icmp_rate_limit` | `bool` | No | ICMPRateLimit limits ICMP packets to prevent ping floods |
| `icmp_rate` | `number` | No | packets/sec (default: 10) |
| `icmp_burst` | `number` | No | burst (default: 20) |
| `new_conn_rate_limit` | `bool` | No | NewConnRateLimit limits new connections per second |
| `new_conn_rate` | `number` | No | per second (default: 100) |
| `new_conn_burst` | `number` | No | burst (default: 200) |
| `port_scan_protection` | `bool` | No | PortScanProtection detects and blocks port scanning |
| `port_scan_threshold` | `number` | No | ports/sec (default: 10) |
| `geo_blocking` | `bool` | No | GeoBlocking blocks traffic from specific countries (requires GeoIP database) |
| `blocked_countries` | `list(string)` | No | ISO country codes |
| `allowed_countries` | `list(string)` | No | If set, only these allowed |

## qos_policy

Per-interface settings (first-class)

**Syntax:**

```hcl
qos_policy "name" {
  interface = "..."
  enabled = true
  direction = "ingress"
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | Yes | Interface to apply QoS |
| `enabled` | `bool` | No |  |
| `direction` | `string` | No | "ingress", "egress", "both" (default: both) Values: `ingress`, `both` |
| `download_mbps` | `number` | No |  |
| `upload_mbps` | `number` | No |  |

**Nested Blocks:**

- `class` (multiple allowed) - QoSClass defines a traffic class for QoS.
- `rule` (multiple allowed) - Traffic classification rules

### class

QoSClass defines a traffic class for QoS.

**Syntax:**

```hcl
class "name" {
  priority = 0
  rate = "..."
  ceil = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `priority` | `number` | No | 1-7, lower is higher priority |
| `rate` | `string` | No | Guaranteed rate e.g., "10mbit" or "10%" |
| `ceil` | `string` | No | Maximum rate |
| `burst` | `string` | No | Burst size |
| `queue_type` | `string` | No | "fq_codel", "sfq", "pfifo" (default: fq_codel) Values: `fq_codel`, `pfifo` |

### rule

Traffic classification rules

**Syntax:**

```hcl
rule "name" {
  class = "..."
  proto = "..."
  src_ip = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `class` | `string` | Yes | Target QoS class |
| `proto` | `string` | No |  |
| `src_ip` | `string` | No |  |
| `dest_ip` | `string` | No |  |
| `src_port` | `number` | No |  |
| `dest_port` | `number` | No |  |
| `services` | `list(string)` | No | Service names |
| `dscp` | `string` | No | Match DSCP value |
| `set_dscp` | `string` | No | Set DSCP on matching traffic |

**Nested Blocks:**

- `threat_intel` - ThreatIntel configures threat intelligence feeds.

#### threat_intel

ThreatIntel configures threat intelligence feeds.

**Syntax:**

```hcl
threat_intel {
  enabled = true
  interval = "..."
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `interval` | `string` | No | e.g. "1h" |

**Nested Blocks:**

- `source` (multiple allowed) -

##### source

**Syntax:**

```hcl
source "name" {
  url = "..."
  format = "taxii"
  collection_id = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | `string` | Yes |  |
| `format` | `string` | No | "taxii", "text", "json" Values: `taxii`, `json` |
| `collection_id` | `string` | No |  |
| `username` | `string` | No |  |
| `password` | `string` | No |  |

## replication

State Replication configuration

**Syntax:**

```hcl
replication {
  mode = "primary"
  listen_addr = "..."
  primary_addr = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | Yes | Mode: "primary", "replica", or "standalone" (default, no HA) Values: `primary`, `replica` |
| `listen_addr` | `string` | No | Listen address for replication traffic (e.g. ":9000") |
| `primary_addr` | `string` | No | Address of the primary node (only for replica mode) |
| `peer_addr` | `string` | No | Address of the peer node (used for HA heartbeat - both nodes need this) |
| `secret_key` | `string` | No | Secret key for PSK authentication (required for secure replication) |
| `tls_cert` | `string` | No | TLS configuration for encrypted replication |
| `tls_key` | `string` | No |  |
| `tls_ca` | `string` | No |  |
| `tls_mutual` | `bool` | No | Require client certs |

**Nested Blocks:**

- `ha` - HA configuration for high-availability failover

### ha

HA configuration for high-availability failover

**Syntax:**

```hcl
ha {
  enabled = true
  priority = 0
  heartbeat_interval = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enabled activates HA monitoring and failover |
| `priority` | `number` | No | Priority determines which node becomes primary (lower = higher priority) Default: 100. Set one no... |
| `heartbeat_interval` | `number` | No | HeartbeatInterval is seconds between heartbeat messages (default: 1) |
| `failure_threshold` | `number` | No | FailureThreshold is missed heartbeats before declaring peer dead (default: 3) |
| `failback_mode` | `string` | No | FailbackMode controls behavior when original primary recovers:   "auto"   - automatically failbac... |
| `failback_delay` | `number` | No | FailbackDelay is seconds to wait before automatic failback (default: 60) |
| `heartbeat_port` | `number` | No | HeartbeatPort is the UDP port for HA heartbeat messages (default: 9002) |

**Nested Blocks:**

- `virtual_ip` (multiple allowed) - Virtual IPs to migrate on failover (for LAN-side gateway addresses)
- `virtual_mac` (multiple allowed) - Virtual MACs to migrate on failover (for DHCP-assigned WAN interfaces)
- `conntrack_sync` - ConntrackSync enables connection state replication via conntrackd

#### virtual_ip

Virtual IPs to migrate on failover (for LAN-side gateway addresses)

**Syntax:**

```hcl
virtual_ip {
  address = "..."
  interface = "..."
  label = "..."
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `address` | `string` | Yes | Address is the virtual IP in CIDR notation (e.g., "192.168.1.1/24") |
| `interface` | `string` | Yes | Interface is the network interface to add the VIP to (e.g., "eth1") |
| `label` | `string` | No | Label is an optional interface label for the address (e.g., "eth1:vip") |

#### virtual_mac

Virtual MACs to migrate on failover (for DHCP-assigned WAN interfaces)

**Syntax:**

```hcl
virtual_mac {
  address = "..."
  interface = "..."
  dhcp = true
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `address` | `string` | No | Address is the virtual MAC address (e.g., "02:gc:ic:00:00:01"). If empty, a locally-administered ... |
| `interface` | `string` | Yes | Interface is the network interface to apply the VMAC to (e.g., "eth0") |
| `dhcp` | `bool` | No | DHCP indicates this interface uses DHCP. On failover, the backup will attempt to reclaim the same... |

#### conntrack_sync

ConntrackSync enables connection state replication via conntrackd

**Syntax:**

```hcl
conntrack_sync {
  enabled = true
  interface = "..."
  multicast_group = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enabled activates conntrack synchronization |
| `interface` | `string` | No | Interface is the network interface for sync traffic (default: HA peer link) |
| `multicast_group` | `string` | No | MulticastGroup for sync traffic (default: 225.0.0.50) Set to empty string to use unicast mode wit... |
| `port` | `number` | No | Port for sync traffic (default: 3780) |
| `expect_sync` | `bool` | No | ExpectSync enables expectation table sync for ALG protocols (FTP, SIP, etc.) |

## route

Route represents a static route configuration.

**Syntax:**

```hcl
route "name" {
  destination = "..."
  gateway = "..."
  interface = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `destination` | `string` | Yes |  |
| `gateway` | `string` | No |  |
| `interface` | `string` | No |  |
| `monitor_ip` | `string` | No |  |
| `table` | `number` | No | Routing table ID (default: main) |
| `metric` | `number` | No | Route metric/priority |

## routing_table

RoutingTable represents a custom routing table configuration.

**Syntax:**

```hcl
routing_table "name" {
  id = 0
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `number` | Yes | Table ID (1-252 for custom tables) |

**Nested Blocks:**

- `route` (multiple allowed) - Routes in this table

### route

Routes in this table

**Syntax:**

```hcl
route "name" {
  destination = "..."
  gateway = "..."
  interface = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `destination` | `string` | Yes |  |
| `gateway` | `string` | No |  |
| `interface` | `string` | No |  |
| `monitor_ip` | `string` | No |  |
| `table` | `number` | No | Routing table ID (default: main) |
| `metric` | `number` | No | Route metric/priority |

## rule_learning

Rule learning and notifications

**Syntax:**

```hcl
rule_learning {
  enabled = true
  log_group = 0
  rate_limit = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `log_group` | `number` | No | nflog group (default: 100) |
| `rate_limit` | `string` | No | e.g., "10/minute" |
| `auto_approve` | `bool` | No | Auto-approve learned rules (legacy) |
| `ignore_networks` | `list(string)` | No | Networks to ignore from learning |
| `retention_days` | `number` | No | How long to keep pending rules |
| `cache_size` | `number` | No | Flow cache size (default: 10000) |
| `learning_mode` | `bool` | No | TOFU (Trust On First Use) mode |
| `inline_mode` | `bool` | No | InlineMode uses nfqueue instead of nflog for packet inspection. This holds packets until a verdic... |

## scheduled_rule

ScheduledRule defines a firewall rule that activates on a schedule.

**Syntax:**

```hcl
scheduled_rule "name" {
  description = "..."
  policy = "..."
  schedule = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | `string` | No |  |
| `policy` | `string` | Yes | Which policy to modify |
| `schedule` | `string` | Yes | Cron expression for when to enable |
| `end_schedule` | `string` | No | Cron expression for when to disable |
| `enabled` | `bool` | No |  |

**Nested Blocks:**

- `rule` - The rule to add/remove

### rule

The rule to add/remove

**Syntax:**

```hcl
rule "name" {
  id = "..."
  description = "..."
  disabled = true
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | No | Identity Unique ID for referencing in reorder operations |
| `description` | `string` | No |  |
| `disabled` | `bool` | No | Temporarily disable this rule |
| `order` | `number` | No | Ordering - rules are processed in order, these help with insertion Explicit order (0 = use array ... |
| `insert_after` | `string` | No | Insert after rule with this ID/name |
| `proto` | `string` | No | Match conditions |
| `dest_port` | `number` | No |  |
| `dest_ports` | `list(number)` | No | Multiple ports |
| `src_port` | `number` | No |  |
| `src_ports` | `list(number)` | No |  |
| `services` | `list(string)` | No | Service names like "http", "ssh" Values: `http`, `ssh` |
| `src_ip` | `string` | No | Source IP/CIDR |
| `src_ipset` | `string` | No | Source IPSet name |
| `dest_ip` | `string` | No | Destination IP/CIDR |
| `dest_ipset` | `string` | No | Destination IPSet name |
| `src_zone` | `string` | No | Additional match conditions Override policy's From zone |
| `dest_zone` | `string` | No | Override policy's To zone |
| `in_interface` | `string` | No | Match specific input interface |
| `out_interface` | `string` | No | Match specific output interface |
| `conn_state` | `string` | No | "new", "established", "related", "invalid" Values: `new`, `invalid` |
| `source_country` | `string` | No | GeoIP matching (requires MaxMind database) ISO 3166-1 alpha-2 country code (e.g., "US", "CN") Values: `US`, `CN` |
| `dest_country` | `string` | No | ISO 3166-1 alpha-2 country code |
| `invert_src` | `bool` | No | Invert matching (match everything EXCEPT the specified value) Negate source IP/IPSet match |
| `invert_dest` | `bool` | No | Negate destination IP/IPSet match |
| `tcp_flags` | `string` | No | TCP Flags matching (for SYN flood protection, connection state filtering) Values: "syn", "syn,!ac... Values: `syn`, `urg` |
| `max_connections` | `number` | No | Connection limiting (prevent abuse/DoS) Max concurrent connections per source |
| `time_start` | `string` | No | Time-of-day matching (uses nftables meta hour/day, requires kernel 5.4+) Start time "HH:MM" (24h ... |
| `time_end` | `string` | No | End time "HH:MM" (24h format) |
| `days` | `list(string)` | No | Days of week: "Monday", "Tuesday", etc. Values: `Monday`, `Tuesday` |
| `action` | `string` | Yes | Action accept, drop, reject, jump, return, log |
| `jump_target` | `string` | No | Target chain for jump action |
| `log` | `bool` | No | Logging & accounting |
| `log_prefix` | `string` | No |  |
| `log_level` | `string` | No | "debug", "info", "notice", "warning", "error" Values: `debug`, `error` |
| `limit` | `string` | No | Rate limit e.g. "10/second" |
| `counter` | `string` | No | Named counter for accounting |
| `comment` | `string` | No | Metadata |
| `tags` | `list(string)` | No | For grouping/filtering in UI |
| `group` | `string` | No | UI Organization Section grouping: "User Access", "IoT Isolation" Values: `User Access`, `IoT Isolation` |

## scheduler

SchedulerConfig defines scheduler settings.

**Syntax:**

```hcl
scheduler {
  enabled = true
  ipset_refresh_hours = 0
  dns_refresh_hours = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `ipset_refresh_hours` | `number` | No | Default: 24 |
| `dns_refresh_hours` | `number` | No | Default: 24 |
| `backup_enabled` | `bool` | No | Enable auto backups |
| `backup_schedule` | `string` | No | Cron expression, default: "0 2 * * *" |
| `backup_retention_days` | `number` | No | Default: 7 |
| `backup_dir` | `string` | No | Default: /var/lib/firewall/backups |

## syslog

Syslog remote logging

**Syntax:**

```hcl
syslog {
  enabled = true
  host = "..."
  port = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `host` | `string` | Yes | Remote syslog server hostname/IP |
| `port` | `number` | No | Default: 514 |
| `protocol` | `string` | No | udp or tcp (default: udp) |
| `tag` | `string` | No | Syslog tag (default: flywall) |
| `facility` | `number` | No | Syslog facility (default: 1) |

## system

System tuning and settings

**Syntax:**

```hcl
system {
  sysctl_profile = "default"
  sysctl = {...}
  timezone = "..."
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `sysctl_profile` | `string` | No | SysctlProfile selects a preset sysctl tuning profile Options: "default", "performance", "low-memo... Values: `default`, `security` |
| `sysctl` | `map` | No | Sysctl allows manual override of sysctl parameters Applied after profile tuning |
| `timezone` | `string` | No | Timezone for scheduled rules (e.g. "America/Los_Angeles"). Defaults to "UTC". |

## threat_intel

ThreatIntel configures threat intelligence feeds.

**Syntax:**

```hcl
threat_intel {
  enabled = true
  interval = "..."
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `interval` | `string` | No | e.g. "1h" |

**Nested Blocks:**

- `source` (multiple allowed) -

### source

**Syntax:**

```hcl
source "name" {
  url = "..."
  format = "taxii"
  collection_id = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | `string` | Yes |  |
| `format` | `string` | No | "taxii", "text", "json" Values: `taxii`, `json` |
| `collection_id` | `string` | No |  |
| `username` | `string` | No |  |
| `password` | `string` | No |  |

## uid_routing

UIDRouting configures per-user routing (for SOCKS proxies, etc.).

**Syntax:**

```hcl
uid_routing "name" {
  uid = 0
  username = "..."
  uplink = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `uid` | `number` | No | User ID to match |
| `username` | `string` | No | Username (resolved to UID) |
| `uplink` | `string` | No | Uplink to route through |
| `vpn_link` | `string` | No | VPN link to route through |
| `interface` | `string` | No | Output interface |
| `snat_ip` | `string` | No | IP to SNAT to |
| `enabled` | `bool` | No |  |
| `comment` | `string` | No |  |

## uplink_group

UplinkGroup configures a group of uplinks (WAN, VPN, etc.) with failover/load balancing.
This enables dynamic switching between uplinks while preserving existing connections.

**Syntax:**

```hcl
uplink_group "name" {
  source_networks = [...]
  source_interfaces = [...]
  source_zones = [...]
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `source_networks` | `list(string)` | Yes | CIDRs that use this group |
| `source_interfaces` | `list(string)` | No | Interfaces for connmark restore |
| `source_zones` | `list(string)` | No | Zones that use this group |
| `failover_mode` | `string` | No | Failover configuration "immediate", "graceful", "manual", "programmatic" Values: `immediate`, `programmatic` |
| `failback_mode` | `string` | No | "immediate", "graceful", "manual", "never" Values: `immediate`, `never` |
| `failover_delay` | `number` | No | Seconds before failover |
| `failback_delay` | `number` | No | Seconds before failback |
| `load_balance_mode` | `string` | No | Load balancing configuration "none", "roundrobin", "weighted", "latency" Values: `none`, `latency` |
| `sticky_connections` | `bool` | No |  |
| `enabled` | `bool` | No |  |

**Nested Blocks:**

- `uplink` (multiple allowed) - UplinkDef defines an uplink within a group.
- `health_check` - WANHealth configures health checking for multi-WAN.

### uplink

UplinkDef defines an uplink within a group.

**Syntax:**

```hcl
uplink "name" {
  type = "wan"
  interface = "..."
  gateway = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` | e.g., "wg0", "wan1", "primary-vpn" | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | `string` | No | "wan", "wireguard", "tailscale", "openvpn", "ipsec", "custom" Values: `wan`, `custom` |
| `interface` | `string` | Yes | Network interface name |
| `gateway` | `string` | No | Gateway IP (for WANs) |
| `local_ip` | `string` | No | Local IP for SNAT |
| `tier` | `number` | No | Failover tier (0 = primary, 1 = secondary, etc.) |
| `weight` | `number` | No | Weight within tier for load balancing (1-100) |
| `enabled` | `bool` | No |  |
| `comment` | `string` | No |  |
| `health_check_cmd` | `string` | No | Custom health check (optional) Custom command for health check |

### health_check

WANHealth configures health checking for multi-WAN.

**Syntax:**

```hcl
health_check {
  interval = 0
  timeout = 0
  threshold = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interval` | `number` | No | Check interval in seconds |
| `timeout` | `number` | No | Timeout per check |
| `threshold` | `number` | No | Failures before marking down |
| `targets` | `list(string)` | No | IPs to ping |
| `http_check` | `string` | No | URL for HTTP health check |

## upnp

UPnP IGD configuration

**Syntax:**

```hcl
upnp {
  enabled = true
  external_interface = "..."
  internal_interfaces = [...]
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `external_interface` | `string` | No | WAN interface |
| `internal_interfaces` | `list(string)` | No | LAN interfaces |
| `secure_mode` | `bool` | No | Only allow mapping to requesting IP |

## vpn

VPN integrations (Tailscale, WireGuard, etc.) for secure remote access

**Syntax:**

```hcl
vpn {
  interface_prefix_zones = "vpn"
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface_prefix_zones` | `map` | No | Interface prefix matching for zones (like firehol's "wg+" syntax) Maps interface prefix to zone n... Values: `vpn`, `tailscale` |

**Nested Blocks:**

- `tailscale` (multiple allowed) - Tailscale/Headscale connections (multiple allowed)
- `wireguard` (multiple allowed) - WireGuard connections (multiple allowed)
- `six_to_four` (multiple allowed) - 6to4 Tunnels (multiple allowed, usually one)

### tailscale

Tailscale/Headscale connections (multiple allowed)

**Syntax:**

```hcl
tailscale "name" {
  enabled = true
  interface = "..."
  auth_key = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` | Connection name (label for multiple connections) | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enable this Tailscale connection |
| `interface` | `string` | No | Interface name (default: tailscale0, or tailscale1, etc. for multiple) |
| `auth_key` | `string` | No | Auth key for unattended setup (or use AuthKeyEnv) |
| `auth_key_env` | `string` | No | Environment variable containing auth key |
| `control_url` | `string` | No | Control server URL (for Headscale) |
| `management_access` | `bool` | No | Always allow management access via Tailscale (lockout protection) This inserts accept rules BEFOR... |
| `zone` | `string` | No | Zone name for this interface (default: tailscale) Use same zone name across multiple connections ... |
| `advertise_routes` | `list(string)` | No | Routes to advertise to Tailscale network |
| `accept_routes` | `bool` | No | Accept routes from other Tailscale nodes |
| `advertise_exit_node` | `bool` | No | Advertise this node as an exit node |
| `exit_node` | `string` | No | Use a specific exit node (Tailscale IP or hostname) |

### wireguard

WireGuard connections (multiple allowed)

**Syntax:**

```hcl
wireguard "name" {
  enabled = true
  interface = "..."
  management_access = true
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` | Connection name (label for multiple connections) | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enable this WireGuard connection |
| `interface` | `string` | No | Interface name (default: wg0, or wg1, etc. for multiple) |
| `management_access` | `bool` | No | Always allow management access via WireGuard (lockout protection) |
| `zone` | `string` | No | Zone name for this interface (default: vpn) Use same zone name across multiple connections to com... |
| `private_key` | `string` | No | Private key (or use PrivateKeyFile) |
| `private_key_file` | `string` | No | Path to private key file |
| `listen_port` | `number` | No | Listen port (default: 51820) |
| `address` | `list(string)` | No | Interface addresses |
| `dns` | `list(string)` | No | DNS servers to use when connected |
| `mtu` | `number` | No | MTU (default: 1420) |
| `fwmark` | `number` | No | Firewall Mark (fwmark) for routing |
| `table` | `string` | No | Routing Table (default: auto) If set to "off" or "auto", behaves effectively like standard WG. If... |
| `post_up` | `list(string)` | No | Hooks |
| `post_down` | `list(string)` | No |  |

**Nested Blocks:**

- `peer` (multiple allowed) - Peer configurations

#### peer

Peer configurations

**Syntax:**

```hcl
peer "name" {
  public_key = "..."
  preshared_key = "..."
  endpoint = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` | Peer name (label) | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `public_key` | `string` | Yes | Peer's public key |
| `preshared_key` | `string` | No | Optional preshared key for additional security |
| `endpoint` | `string` | No | Peer's endpoint (host:port) |
| `allowed_ips` | `list(string)` | Yes | Allowed IP ranges for this peer |
| `persistent_keepalive` | `number` | No | Keepalive interval in seconds (useful for NAT traversal) |

### six_to_four

6to4 Tunnels (multiple allowed, usually one)

**Syntax:**

```hcl
six_to_four "name" {
  interface = "..."
  enabled = true
  zone = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | Yes | Physical interface name (usually WAN) |
| `enabled` | `bool` | No |  |
| `zone` | `string` | No | Zone for the tunnel interface (tun6to4) |
| `mtu` | `number` | No | Default 1480 |

## web

Web Server configuration (previously part of API)

**Syntax:**

```hcl
web {
  listen = "..."
  tls_listen = "..."
  tls_cert = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `listen` | `string` | No | Listen addresses HTTP listen address (default :80) |
| `tls_listen` | `string` | No | HTTPS listen address (default :443) |
| `tls_cert` | `string` | No | TLS Configuration Path to TLS certificate |
| `tls_key` | `string` | No | Path to TLS key |
| `disable_redirect` | `bool` | No | Behavior Disable HTTP->HTTPS redirect |
| `serve_ui` | `bool` | No | Serve the dashboard (default true) |
| `serve_api` | `bool` | No | Serve API paths (default true) |

**Nested Blocks:**

- `allow` (multiple allowed) - Access Control
- `deny` (multiple allowed) - AccessRule defines criteria for allowing or denying access.

### allow

Access Control

**Syntax:**

```hcl
allow {
  interface = "..."
  source = "..."
  interfaces = [...]
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | No | Single value fields |
| `source` | `string` | No |  |
| `interfaces` | `list(string)` | No | List value fields (for brevity) |
| `sources` | `list(string)` | No |  |

### deny

AccessRule defines criteria for allowing or denying access.

**Syntax:**

```hcl
deny {
  interface = "..."
  source = "..."
  interfaces = [...]
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | No | Single value fields |
| `source` | `string` | No |  |
| `interfaces` | `list(string)` | No | List value fields (for brevity) |
| `sources` | `list(string)` | No |  |

## zone

Zone defines a network security zone.
Zones can match traffic by interface, source/destination IP, VLAN, or combinations.
Simple zones use top-level fields, complex zones use match blocks.

**Syntax:**

```hcl
zone "name" {
  color = "..."
  description = "..."
  interface = "..."
  # ...
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `color` | `string` | No |  |
| `description` | `string` | No |  |
| `interface` | `string` | No | Simple match criteria (use for single-interface zones) These are effectively a single implicit ma... |
| `src` | `string` | No | Source IP/network (e.g., "192.168.1.0/24") |
| `dst` | `string` | No | Destination IP/network |
| `vlan` | `number` | No | VLAN tag |
| `interfaces` | `list(string)` | No | ⚠️ *Deprecated.* DEPRECATED: Use Interface or Matches instead Will be auto-converted to Matches with warning |
| `ipsets` | `list(string)` | No | Legacy fields (kept for backwards compat) IPSet names for IP-based membership |
| `networks` | `list(string)` | No | Direct CIDR ranges |
| `action` | `string` | No | Zone behavior Action for intra-zone traffic: "accept", "drop", "reject" (default: accept) Values: `accept`, `reject` |
| `external` | `bool` | No | External marks this as an external/WAN zone (used for auto-masquerade detection) If not set, dete... |
| `ipv4` | `list(string)` | No | IP assignment for simple zones (shorthand - assigns to the interface) |
| `ipv6` | `list(string)` | No |  |
| `dhcp` | `bool` | No | Use DHCP client on this interface |

**Nested Blocks:**

- `match` (multiple allowed) - Complex match criteria (OR logic between matches, AND logic within each match...
- `services` - Services provided TO this zone (firewall auto-generates rules) These define w...
- `management` - Management access FROM this zone to the firewall

### match

Complex match criteria (OR logic between matches, AND logic within each match)
Global fields above apply to ALL matches as defaults

**Syntax:**

```hcl
match {
  interface = "..."
  src = "..."
  dst = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | No | Interface can be exact ("eth0") or prefix with + or * suffix ("wg+" matches wg0, wg1...) |
| `src` | `string` | No |  |
| `dst` | `string` | No |  |
| `vlan` | `number` | No |  |

### services

Services provided TO this zone (firewall auto-generates rules)
These define what the firewall offers to clients in this zone

**Syntax:**

```hcl
services {
  dhcp = true
  dns = true
  ntp = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `dhcp` | `bool` | No | Network services Allow DHCP requests (udp/67-68) |
| `dns` | `bool` | No | Allow DNS queries (udp/53, tcp/53) |
| `ntp` | `bool` | No | Allow NTP sync (udp/123) |
| `captive_portal` | `bool` | No | Captive portal / guest access Redirect HTTP to portal |

**Nested Blocks:**

- `port` (multiple allowed) - Custom service ports (auto-allow)

#### port

Custom service ports (auto-allow)

**Syntax:**

```hcl
port "name" {
  protocol = "..."
  port = 0
  port_end = 0
}
```

**Labels:**

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `protocol` | `string` | Yes | tcp, udp |
| `port` | `number` | Yes | Port number |
| `port_end` | `number` | No | For port ranges |

### management

Management access FROM this zone to the firewall

**Syntax:**

```hcl
management {
  web_ui = true
  web = true
  ssh = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `web_ui` | `bool` | No | Legacy: Allow Web UI access (tcp/80, tcp/443) -> Use Web |
| `web` | `bool` | No | Allow Web UI access (tcp/80, tcp/443) |
| `ssh` | `bool` | No | Allow SSH access (tcp/22) |
| `api` | `bool` | No | Allow API access (used for L7 filtering, implies HTTPS access) |
| `icmp` | `bool` | No | Allow ping to firewall |
| `snmp` | `bool` | No | Allow SNMP queries (udp/161) |
| `syslog` | `bool` | No | Allow syslog sending (udp/514) |
