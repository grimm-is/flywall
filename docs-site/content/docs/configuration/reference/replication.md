---
title: "replication"
linkTitle: "replication"
weight: 43
description: >
  State Replication configuration
---

State Replication configuration

## Syntax

```hcl
replication {
  mode = "primary"
  listen_addr = "..."
  primary_addr = "..."
  peer_addr = "..."
  secret_key = "..."
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | Yes | Mode: "primary", "replica", or "standalone" (default, no HA) Values: `primary`, `replica` |
| `listen_addr` | `string` | No | Listen address for replication traffic (e.g. ":9000") |
| `primary_addr` | `string` | No | Address of the primary node (only for replica mode) |
| `peer_addr` | `string` | No | Address of the peer node (used for HA heartbeat - both nodes need this) |
| `secret_key` | `string` | No | Secret key for PSK authentication (required for secure replication) |
| `tls_cert` | `string` | No | TLS configuration for encrypted replication |
| `tls_key` | `string` | No |  |
| `tls_ca` | `string` | No |  |
| `tls_mutual` | `bool` | No | Require client certs |

## Nested Blocks

### ha

HA configuration for high-availability failover

```hcl
ha {
  enabled = true
  priority = 0
  heartbeat_interval = 0
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enabled activates HA monitoring and failover |
| `priority` | `number` | No | Priority determines which node becomes primary (lower = higher priority) Defa... |
| `heartbeat_interval` | `number` | No | HeartbeatInterval is seconds between heartbeat messages (default: 1) |
| `failure_threshold` | `number` | No | FailureThreshold is missed heartbeats before declaring peer dead (default: 3) |
| `failback_mode` | `string` | No | FailbackMode controls behavior when original primary recovers:   "auto"   - a... |
| `failback_delay` | `number` | No | FailbackDelay is seconds to wait before automatic failback (default: 60) |
| `heartbeat_port` | `number` | No | HeartbeatPort is the UDP port for HA heartbeat messages (default: 9002) |

#### virtual_ip

Virtual IPs to migrate on failover (for LAN-side gateway addresses)

```hcl
virtual_ip {
  address = "..."
  interface = "..."
  label = "..."
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `address` | `string` | Yes | Address is the virtual IP in CIDR notation (e.g., "192.168.1.1/24") |
| `interface` | `string` | Yes | Interface is the network interface to add the VIP to (e.g., "eth1") |
| `label` | `string` | No | Label is an optional interface label for the address (e.g., "eth1:vip") |

#### virtual_mac

Virtual MACs to migrate on failover (for DHCP-assigned WAN interfaces)

```hcl
virtual_mac {
  address = "..."
  interface = "..."
  dhcp = true
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `address` | `string` | No | Address is the virtual MAC address (e.g., "02:gc:ic:00:00:01"). If empty, a l... |
| `interface` | `string` | Yes | Interface is the network interface to apply the VMAC to (e.g., "eth0") |
| `dhcp` | `bool` | No | DHCP indicates this interface uses DHCP. On failover, the backup will attempt... |

#### conntrack_sync

ConntrackSync enables connection state replication via conntrackd

```hcl
conntrack_sync {
  enabled = true
  interface = "..."
  multicast_group = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enabled activates conntrack synchronization |
| `interface` | `string` | No | Interface is the network interface for sync traffic (default: HA peer link) |
| `multicast_group` | `string` | No | MulticastGroup for sync traffic (default: 225.0.0.50) Set to empty string to ... |
| `port` | `number` | No | Port for sync traffic (default: 3780) |
| `expect_sync` | `bool` | No | ExpectSync enables expectation table sync for ALG protocols (FTP, SIP, etc.) |
