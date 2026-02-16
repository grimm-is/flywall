---
title: "route"
linkTitle: "route"
weight: 44
description: >
  Route represents a static route configuration.
---

Route represents a static route configuration.

## Syntax

```hcl
route "name" {
  destination = "..."
  gateway = "..."
  interface = "..."
  monitor_ip = "..."
  table = 0
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
| `destination` | `string` | Yes |  |
| `gateway` | `string` | No |  |
| `interface` | `string` | No |  |
| `monitor_ip` | `string` | No |  |
| `table` | `number` | No | Routing table ID (default: main) |
| `metric` | `number` | No | Route metric/priority |
