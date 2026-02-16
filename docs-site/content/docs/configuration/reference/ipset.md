---
title: "ipset"
linkTitle: "ipset"
weight: 32
description: >
  IPSet defines a named set of IPs/networks for use in firewall rules.
---

IPSet defines a named set of IPs/networks for use in firewall rules.

## Syntax

```hcl
ipset "name" {
  description = "..."
  type = "..."
  entries = [...]
  domains = [...]
  refresh_interval = "5m"
  # ...
}
```

## Labels

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | `string` | No |  |
| `type` | `string` | No | ipv4_addr (default), ipv6_addr, inet_service, or dns |
| `entries` | `list(string)` | No |  |
| `domains` | `list(string)` | No | Domains for dynamic resolution (only for type="dns") |
| `refresh_interval` | `string` | No | Refresh interval for DNS resolution (e.g., "5m", "1h") - only for type="dns" Values: `5m`, `1h` |
| `size` | `number` | No | Optimization: Pre-allocated size for dynamic sets to prevent resizing (sugges... |
| `firehol_list` | `string` | No | FireHOL import e.g., "firehol_level1", "spamhaus_drop" Values: `firehol_level1`, `spamhaus_drop` |
| `url` | `string` | No | Custom URL for IP list |
| `refresh_hours` | `number` | No | How often to refresh (default: 24) |
| `auto_update` | `bool` | No | Enable automatic updates |
| `action` | `string` | No | drop, reject, log (for auto-generated rules) |
| `apply_to` | `string` | No | input, forward, both (for auto-generated rules) |
| `match_on_source` | `bool` | No | Match source IP (default: true) |
| `match_on_dest` | `bool` | No | Match destination IP |
