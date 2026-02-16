---
title: "policy"
linkTitle: "policy"
weight: 39
description: >
  Policy defines traffic rules between zones. Rules are evaluated in order - first match wins.
---

Policy defines traffic rules between zones.
Rules are evaluated in order - first match wins.

## Syntax

```hcl
policy "from" "to" {
  name = "..."
  description = "..."
  priority = 0
  disabled = true
  action = "accept"
  # ...
}
```

## Labels

| Label | Description | Required |
|-------|-------------|----------|
| `from` | Source zone name | Yes |
| `to` | Destination zone name | Yes |

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | `string` | No | Optional descriptive name |
| `description` | `string` | No |  |
| `priority` | `number` | No | Policy priority (lower = evaluated first) |
| `disabled` | `bool` | No | Temporarily disable this policy |
| `action` | `string` | No | Action for traffic matching this policy (when no specific rule matches) Value... Values: `accept`, `reject` |
| `masquerade` | `bool` | No | Masquerade controls NAT for outbound traffic through this policy nil = auto (... |
| `log` | `bool` | No | Log packets matching default action |
| `log_prefix` | `string` | No | Prefix for log messages |
| `inherits` | `string` | No | Inheritance - allows policies to inherit rules from a parent policy Child pol... |

## Nested Blocks

### rule

PolicyRule defines a specific rule within a policy.
Rules are matched in the order they appear - first match wins.

```hcl
rule "name" {
  id = "..."
  description = "..."
  disabled = true
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | No | Identity Unique ID for referencing in reorder operations |
| `description` | `string` | No |  |
| `disabled` | `bool` | No | Temporarily disable this rule |
| `order` | `number` | No | Ordering - rules are processed in order, these help with insertion Explicit o... |
| `insert_after` | `string` | No | Insert after rule with this ID/name |
| `proto` | `string` | No | Match conditions |
| `dest_port` | `number` | No |  |
| `dest_ports` | `list(number)` | No | Multiple ports |
| `src_port` | `number` | No |  |
| `src_ports` | `list(number)` | No |  |
| `service` | `string` | No | Service Macro (e.g. "ssh") - expands to Proto/Port |
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
| `source_country` | `string` | No | GeoIP matching (requires MaxMind database) ISO 3166-1 alpha-2 country code (e... Values: `US`, `CN` |
| `dest_country` | `string` | No | ISO 3166-1 alpha-2 country code |
| `invert_src` | `bool` | No | Invert matching (match everything EXCEPT the specified value) Negate source I... |
| `invert_dest` | `bool` | No | Negate destination IP/IPSet match |
| `tcp_flags` | `string` | No | TCP Flags matching (for SYN flood protection, connection state filtering) Val... Values: `syn`, `urg` |
| `max_connections` | `number` | No | Connection limiting (prevent abuse/DoS) Max concurrent connections per source |
| `time_start` | `string` | No | Time-of-day matching (uses nftables meta hour/day, requires kernel 5.4+) Star... |
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
