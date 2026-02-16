---
title: "mark_rule"
linkTitle: "mark_rule"
weight: 33
description: >
  MarkRule represents a rule for setting routing marks on packets. Marks are set in nftables and ma...
---

MarkRule represents a rule for setting routing marks on packets.
Marks are set in nftables and matched by ip rule for routing decisions.

## Syntax

```hcl
mark_rule "name" {
  mark = "..."
  mask = "..."
  proto = "..."
  src_ip = "..."
  dst_ip = "..."
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
| `mark` | `string` | Yes | Mark value to set (hex: 0x10) |
| `mask` | `string` | No | Mask for mark operations |
| `proto` | `string` | No | Match criteria tcp, udp, icmp, all |
| `src_ip` | `string` | No |  |
| `dst_ip` | `string` | No |  |
| `src_port` | `number` | No |  |
| `dst_port` | `number` | No |  |
| `dst_ports` | `list(number)` | No | Multiple ports |
| `in_interface` | `string` | No |  |
| `out_interface` | `string` | No |  |
| `src_zone` | `string` | No |  |
| `dst_zone` | `string` | No |  |
| `ipset` | `string` | No | Match against IPSet |
| `conn_state` | `list(string)` | No | NEW, ESTABLISHED, etc. |
| `save_mark` | `bool` | No | Mark behavior Save to conntrack |
| `restore_mark` | `bool` | No | Restore from conntrack |
| `enabled` | `bool` | No |  |
| `comment` | `string` | No |  |
