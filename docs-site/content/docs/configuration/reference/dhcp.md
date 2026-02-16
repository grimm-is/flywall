---
title: "dhcp"
linkTitle: "dhcp"
weight: 25
description: >
  DHCPServer configuration.
---

DHCPServer configuration.

## Syntax

```hcl
dhcp {
  enabled = true
  mode = "..."
  external_lease_file = "..."

  scope { ... }
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `mode` | `string` | No | Mode specifies how DHCP server is managed:   - "builtin" (default): Use Flywa... |
| `external_lease_file` | `string` | No | ExternalLeaseFile is the path to external DHCP server's lease file (for impor... |

## Nested Blocks

### scope

DHCPScope defines a DHCP pool.

```hcl
scope "name" {
  interface = "..."
  range_start = "..."
  range_end = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

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
| `options` | `map` | No | Custom DHCP options using named options or numeric codes (1-255) Named option... Values: `str:tftp.boot`, `150` |
| `range_start_v6` | `string` | No | IPv6 Support (SLAAC/DHCPv6) For Stateful DHCPv6 |
| `range_end_v6` | `string` | No |  |
| `dns_v6` | `list(string)` | No |  |

#### reservation

DHCPReservation defines a static IP assignment for a MAC address.

```hcl
reservation "mac" {
  ip = "..."
  hostname = "..."
  description = "..."
  # ...
}
```

**Labels:**

- `mac` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `ip` | `string` | Yes |  |
| `hostname` | `string` | No |  |
| `description` | `string` | No |  |
| `options` | `map` | No | Per-host custom DHCP options (same format as scope options) |
| `register_dns` | `bool` | No | DNS integration Auto-register in DNS |
