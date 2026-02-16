---
title: "routing_table"
linkTitle: "routing_table"
weight: 45
description: >
  RoutingTable represents a custom routing table configuration.
---

RoutingTable represents a custom routing table configuration.

## Syntax

```hcl
routing_table "name" {
  id = 0

  route { ... }
}
```

## Labels

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `number` | Yes | Table ID (1-252 for custom tables) |

## Nested Blocks

### route

Routes in this table

```hcl
route "name" {
  destination = "..."
  gateway = "..."
  interface = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `destination` | `string` | Yes |  |
| `gateway` | `string` | No |  |
| `interface` | `string` | No |  |
| `monitor_ip` | `string` | No |  |
| `table` | `number` | No | Routing table ID (default: main) |
| `metric` | `number` | No | Route metric/priority |
