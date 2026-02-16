---
title: "vpn"
linkTitle: "vpn"
weight: 55
description: >
  VPN integrations (Tailscale, WireGuard, etc.) for secure remote access
---

VPN integrations (Tailscale, WireGuard, etc.) for secure remote access

## Syntax

```hcl
vpn {
  interface_prefix_zones = "vpn"

  tailscale { ... }

  wireguard { ... }

  six_to_four { ... }
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface_prefix_zones` | `map` | No | Interface prefix matching for zones (like firehol's "wg+" syntax) Maps interf... Values: `vpn`, `tailscale` |

## Nested Blocks

### tailscale

Tailscale/Headscale connections (multiple allowed)

```hcl
tailscale "name" {
  enabled = true
  interface = "..."
  auth_key = "..."
  # ...
}
```

**Labels:**

- `name` (required) - Connection name (label for multiple connections)

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enable this Tailscale connection |
| `interface` | `string` | No | Interface name (default: tailscale0, or tailscale1, etc. for multiple) |
| `auth_key` | `string` | No | Auth key for unattended setup (or use AuthKeyEnv) |
| `auth_key_env` | `string` | No | Environment variable containing auth key |
| `control_url` | `string` | No | Control server URL (for Headscale) |
| `management_access` | `bool` | No | Always allow management access via Tailscale (lockout protection) This insert... |
| `zone` | `string` | No | Zone name for this interface (default: tailscale) Use same zone name across m... |
| `advertise_routes` | `list(string)` | No | Routes to advertise to Tailscale network |
| `accept_routes` | `bool` | No | Accept routes from other Tailscale nodes |
| `advertise_exit_node` | `bool` | No | Advertise this node as an exit node |
| `exit_node` | `string` | No | Use a specific exit node (Tailscale IP or hostname) |

### wireguard

WireGuard connections (multiple allowed)

```hcl
wireguard "name" {
  enabled = true
  interface = "..."
  management_access = true
  # ...
}
```

**Labels:**

- `name` (required) - Connection name (label for multiple connections)

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enable this WireGuard connection |
| `interface` | `string` | No | Interface name (default: wg0, or wg1, etc. for multiple) |
| `management_access` | `bool` | No | Always allow management access via WireGuard (lockout protection) |
| `zone` | `string` | No | Zone name for this interface (default: vpn) Use same zone name across multipl... |
| `private_key` | `string` | No | Private key (or use PrivateKeyFile) |
| `private_key_file` | `string` | No | Path to private key file |
| `listen_port` | `number` | No | Listen port (default: 51820) |
| `address` | `list(string)` | No | Interface addresses |
| `dns` | `list(string)` | No | DNS servers to use when connected |
| `mtu` | `number` | No | MTU (default: 1420) |
| `fwmark` | `number` | No | Firewall Mark (fwmark) for routing |
| `table` | `string` | No | Routing Table (default: auto) If set to "off" or "auto", behaves effectively ... |
| `post_up` | `list(string)` | No | Hooks |
| `post_down` | `list(string)` | No |  |

#### peer

Peer configurations

```hcl
peer "name" {
  public_key = "..."
  preshared_key = "..."
  endpoint = "..."
  # ...
}
```

**Labels:**

- `name` (required) - Peer name (label)

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `public_key` | `string` | Yes | Peer's public key |
| `preshared_key` | `string` | No | Optional preshared key for additional security |
| `endpoint` | `string` | No | Peer's endpoint (host:port) |
| `allowed_ips` | `list(string)` | Yes | Allowed IP ranges for this peer |
| `persistent_keepalive` | `number` | No | Keepalive interval in seconds (useful for NAT traversal) |

### six_to_four

6to4 Tunnels (multiple allowed, usually one)

```hcl
six_to_four "name" {
  interface = "..."
  enabled = true
  zone = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | Yes | Physical interface name (usually WAN) |
| `enabled` | `bool` | No |  |
| `zone` | `string` | No | Zone for the tunnel interface (tun6to4) |
| `mtu` | `number` | No | Default 1480 |
