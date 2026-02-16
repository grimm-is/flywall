---
title: "nat"
linkTitle: "nat"
weight: 36
description: >
  NATRule defines Network Address Translation rules.
---

NATRule defines Network Address Translation rules.

## Syntax

```hcl
nat "name" {
  description = "..."
  type = "..."
  proto = "..."
  out_interface = "..."
  in_interface = "..."
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
| `type` | `string` | Yes | masquerade, dnat, snat, redirect |
| `proto` | `string` | No | tcp, udp |
| `out_interface` | `string` | No | for masquerade/snat |
| `in_interface` | `string` | No | for dnat |
| `src_ip` | `string` | No | Source IP match |
| `dest_ip` | `string` | No | Dest IP match |
| `mark` | `number` | No | FWMark match |
| `dest_port` | `string` | No | Dest Port match (supports ranges "80-90") |
| `to_ip` | `string` | No | Target IP for DNAT |
| `to_port` | `string` | No | Target Port for DNAT |
| `snat_ip` | `string` | No | for snat (Target IP) |
| `hairpin` | `bool` | No | Enable Hairpin NAT (NAT Reflection) |
