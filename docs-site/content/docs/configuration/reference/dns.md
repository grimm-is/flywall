---
title: "dns"
linkTitle: "dns"
weight: 26
description: >
  New consolidated DNS config
---

New consolidated DNS config

## Syntax

```hcl
dns {
  mode = "..."
  forwarders = [...]
  upstream_timeout = 0
  dnssec = true
  egress_filter = true
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | No | Resolution mode for upstream queries:   - "forward" (default): Forward querie... |
| `forwarders` | `list(string)` | No | Upstream DNS servers for forwarding mode |
| `upstream_timeout` | `number` | No | seconds |
| `dnssec` | `bool` | No | DNSSEC validation for upstream queries |
| `egress_filter` | `bool` | No | Egress Filter (DNS Wall) If enabled, firewall blocks outbound traffic to IPs ... |
| `egress_filter_ttl` | `number` | No | Seconds (default: matches record TTL) |

## Nested Blocks

### conditional_forward

ConditionalForward routes specific domains to specific DNS servers.

```hcl
conditional_forward "domain" {
  servers = [...]
}
```

**Labels:**

- `domain` (required) - e.g., "corp.example.com"

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `servers` | `list(string)` | Yes | DNS servers for this domain |

### upstream_doh

Encrypted DNS upstreams

```hcl
upstream_doh "name" {
  url = "..."
  bootstrap = "..."
  server_name = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

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

```hcl
upstream_dot "name" {
  server = "..."
  server_name = "..."
  enabled = true
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `server` | `string` | Yes | IP:port or hostname:port |
| `server_name` | `string` | No | For TLS verification |
| `enabled` | `bool` | No |  |
| `priority` | `number` | No |  |

### upstream_dnscrypt

DNSCryptUpstream configures a DNSCrypt upstream server.

```hcl
upstream_dnscrypt "name" {
  stamp = "..."
  provider_name = "..."
  server_addr = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

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

```hcl
serve "zone" {
  listen_port = 0
  local_domain = "..."
  expand_hosts = true
  # ...
}
```

**Labels:**

- `zone` (required) - Zone name (label) - can use wildcards

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

#### blocklist

Filtering

```hcl
blocklist "name" {
  url = "..."
  file = "..."
  format = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

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

```hcl
host "ip" {
  hostnames = [...]
}
```

**Labels:**

- `ip` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `hostnames` | `list(string)` | Yes |  |

#### zone

DNSZone configuration for authoritative zones.

```hcl
zone "name" {
}
```

**Labels:**

- `name` (required) -

##### record

DNSRecord configuration.

```hcl
record "name" {
  type = "..."
  value = "..."
  ttl = 0
  # ...
}
```

**Labels:**

- `name` (required) -

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

```hcl
inspect "zone" {
  mode = "..."
  exclude_router = true
}
```

**Labels:**

- `zone` (required) - Zone name (label) - can use wildcards

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | No | Mode determines what happens to intercepted traffic:   - "redirect": DNAT to ... |
| `exclude_router` | `bool` | No | ExcludeRouter prevents redirecting the router's own DNS traffic |
