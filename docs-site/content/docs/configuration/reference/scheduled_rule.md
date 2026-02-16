---
title: "scheduled_rule"
linkTitle: "scheduled_rule"
weight: 47
description: >
  ScheduledRule defines a firewall rule that activates on a schedule.
---

ScheduledRule defines a firewall rule that activates on a schedule.

## Syntax

```hcl
scheduled_rule "name" {
  description = "..."
  policy = "..."
  schedule = "..."
  end_schedule = "..."
  enabled = true
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
| `policy` | `string` | Yes | Which policy to modify |
| `schedule` | `string` | Yes | Cron expression for when to enable |
| `end_schedule` | `string` | No | Cron expression for when to disable |
| `enabled` | `bool` | No |  |

## Nested Blocks

### rule

The rule to add/remove

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
