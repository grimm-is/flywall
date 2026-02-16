---
title: "multi_wan"
linkTitle: "multi_wan"
weight: 35
description: >
  MultiWAN represents multi-WAN configuration for failover and load balancing.
---

MultiWAN represents multi-WAN configuration for failover and load balancing.

## Syntax

```hcl
multi_wan {
  enabled = true
  mode = "failover"

  wan { ... }

  health_check { ... }
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `mode` | `string` | No | "failover", "loadbalance", "both" Values: `failover`, `both` |

## Nested Blocks

### wan

WANLink represents a WAN connection for multi-WAN.

```hcl
wan "name" {
  interface = "..."
  gateway = "..."
  weight = 0
  # ...
}
```

**Labels:**

- `name` (required) -

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
