---
title: "ddns"
linkTitle: "ddns"
weight: 24
description: >
  Dynamic DNS
---

Dynamic DNS

## Syntax

```hcl
ddns {
  enabled = true
  provider = "..."
  hostname = "..."
  token = "..."
  username = "..."
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `provider` | `string` | Yes | duckdns, cloudflare, noip |
| `hostname` | `string` | Yes | Hostname to update |
| `token` | `string` | No | API token/password |
| `username` | `string` | No | For providers requiring username |
| `zone_id` | `string` | No | For Cloudflare |
| `record_id` | `string` | No | For Cloudflare |
| `interface` | `string` | No | Interface to get IP from |
| `interval` | `number` | No | Update interval in minutes (default: 5) |
