---
title: "dns_server"
linkTitle: "dns_server"
weight: 27
description: >
  Deprecated: use DNS
---

{{% alert title="Deprecated" color="warning" %}}
use DNS
{{% /alert %}}

Deprecated: use DNS

## Syntax

```hcl
dns_server {
  enabled = true
  listen_on = [...]
  listen_port = 0
  local_domain = "lan"
  expand_hosts = true
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `listen_on` | `list(string)` | No |  |
| `listen_port` | `number` | No | Default 53 |
| `local_domain` | `string` | No | Local domain configuration e.g., "lan", "home.arpa" Values: `lan`, `home.arpa` |
| `expand_hosts` | `bool` | No | Append local domain to simple hostnames |
| `dhcp_integration` | `bool` | No | Auto-register DHCP hostnames |
| `authoritative_for` | `string` | No | Return NXDOMAIN for unknown local hosts |
| `mode` | `string` | No | Resolution mode:   - "forward" (default): Forward queries to upstream DNS ser... |
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

Encrypted DNS - Upstream (client mode) DNS-over-HTTPS upstreams

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

DNS-over-TLS upstreams

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

DNSCrypt upstreams

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

### doh_server

Encrypted DNS - Server (serve DoH/DoT/DNSCrypt to clients) Serve DNS-over-HTTPS

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

### host

Static entries and zones Static /etc/hosts style entries

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

### zone

DNSZone configuration for authoritative zones.

```hcl
zone "name" {
}
```

**Labels:**

- `name` (required) -

#### record

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
