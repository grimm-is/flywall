---
title: "protection"
linkTitle: "protection"
weight: 41
description: >
  InterfaceProtection defines security protection settings for an interface.
---

InterfaceProtection defines security protection settings for an interface.

## Syntax

```hcl
protection "name" {
  interface = "..."
  enabled = true
  anti_spoofing = true
  bogon_filtering = true
  private_filtering = true
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
| `interface` | `string` | Yes | Interface name or "*" for all |
| `enabled` | `bool` | No |  |
| `anti_spoofing` | `bool` | No | AntiSpoofing drops packets with spoofed source IPs (recommended for WAN) |
| `bogon_filtering` | `bool` | No | BogonFiltering drops packets from reserved/invalid IP ranges |
| `private_filtering` | `bool` | No | PrivateFiltering drops packets from private IP ranges on WAN (RFC1918) |
| `invalid_packets` | `bool` | No | InvalidPackets drops malformed/invalid packets |
| `syn_flood_protection` | `bool` | No | SynFloodProtection limits SYN packets to prevent SYN floods |
| `syn_flood_rate` | `number` | No | packets/sec (default: 25) |
| `syn_flood_burst` | `number` | No | burst allowance (default: 50) |
| `icmp_rate_limit` | `bool` | No | ICMPRateLimit limits ICMP packets to prevent ping floods |
| `icmp_rate` | `number` | No | packets/sec (default: 10) |
| `icmp_burst` | `number` | No | burst (default: 20) |
| `new_conn_rate_limit` | `bool` | No | NewConnRateLimit limits new connections per second |
| `new_conn_rate` | `number` | No | per second (default: 100) |
| `new_conn_burst` | `number` | No | burst (default: 200) |
| `port_scan_protection` | `bool` | No | PortScanProtection detects and blocks port scanning |
| `port_scan_threshold` | `number` | No | ports/sec (default: 10) |
| `geo_blocking` | `bool` | No | GeoBlocking blocks traffic from specific countries (requires GeoIP database) |
| `blocked_countries` | `list(string)` | No | ISO country codes |
| `allowed_countries` | `list(string)` | No | If set, only these allowed |
