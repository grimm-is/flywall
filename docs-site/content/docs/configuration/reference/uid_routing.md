---
title: "uid_routing"
linkTitle: "uid_routing"
weight: 52
description: >
  UIDRouting configures per-user routing (for SOCKS proxies, etc.).
---

UIDRouting configures per-user routing (for SOCKS proxies, etc.).

## Syntax

```hcl
uid_routing "name" {
  uid = 0
  username = "..."
  uplink = "..."
  vpn_link = "..."
  interface = "..."
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
| `uid` | `number` | No | User ID to match |
| `username` | `string` | No | Username (resolved to UID) |
| `uplink` | `string` | No | Uplink to route through |
| `vpn_link` | `string` | No | VPN link to route through |
| `interface` | `string` | No | Output interface |
| `snat_ip` | `string` | No | IP to SNAT to |
| `enabled` | `bool` | No |  |
| `comment` | `string` | No |  |
