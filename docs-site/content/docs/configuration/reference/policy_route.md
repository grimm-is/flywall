---
title: "policy_route"
linkTitle: "policy_route"
weight: 40
description: >
  PolicyRoute represents a policy-based routing rule. Policy routes use firewall marks to direct tr...
---

PolicyRoute represents a policy-based routing rule.
Policy routes use firewall marks to direct traffic to specific routing tables.

## Syntax

```hcl
policy_route "name" {
  priority = 0
  mark = "..."
  mark_mask = "..."
  from = "..."
  to = "..."
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
| `priority` | `number` | No | Rule priority (lower = higher priority) |
| `mark` | `string` | No | Match criteria (combined with AND) Firewall mark to match (hex: 0x10) |
| `mark_mask` | `string` | No | Mask for mark matching |
| `from` | `string` | No | Source IP/CIDR |
| `to` | `string` | No | Destination IP/CIDR |
| `iif` | `string` | No | Input interface |
| `oif` | `string` | No | Output interface |
| `fwmark` | `string` | No | Alternative mark syntax |
| `table` | `number` | No | Action Routing table to use |
| `blackhole` | `bool` | No | Drop matching packets |
| `prohibit` | `bool` | No | Return ICMP prohibited |
| `enabled` | `bool` | No | Default true |
| `comment` | `string` | No |  |
