---
title: "uplink_group"
linkTitle: "uplink_group"
weight: 53
description: >
  UplinkGroup configures a group of uplinks (WAN, VPN, etc.) with failover/load balancing. This ena...
---

UplinkGroup configures a group of uplinks (WAN, VPN, etc.) with failover/load balancing.
This enables dynamic switching between uplinks while preserving existing connections.

## Syntax

```hcl
uplink_group "name" {
  source_networks = [...]
  source_interfaces = [...]
  source_zones = [...]
  failover_mode = "immediate"
  failback_mode = "immediate"
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

## Nested Blocks

### uplink

UplinkDef defines an uplink within a group.

```hcl
uplink "name" {
  type = "wan"
  interface = "..."
  gateway = "..."
  # ...
}
```

**Labels:**

- `name` (required) - e.g., "wg0", "wan1", "primary-vpn"

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
