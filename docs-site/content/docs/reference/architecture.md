---
title: "Architecture"
linkTitle: "Architecture"
weight: 30
description: >
  How Flywall works under the hood.
---

## Overview

Flywall is a single-binary Linux firewall and router. It uses a privilege-separated architecture for security.

```
┌────────────────────────────────────────────────────────────┐
│  flywall api (nobody, sandboxed)   HTTP:8080 / HTTPS:8443  │
│  └─ REST + WebSocket + Web UI                              │
├────────────────────────────────────────────────────────────┤
│  flywall ctl (root)                Unix Socket RPC          │
│  └─ Firewall │ Network │ DHCP │ DNS │ VPN │ Learning       │
└────────────────────────────────────────────────────────────┘
```

## Components

### Control Plane (flywall ctl)

Runs as **root**. Manages:
- Network interfaces (via netlink)
- Firewall rules (nftables)
- DHCP server
- DNS resolver
- VPN tunnels

### API Server (flywall api)

Runs as **nobody** in a network namespace sandbox. Provides:
- REST API for configuration
- WebSocket for real-time events
- Web UI (embedded Svelte application)

Communication between API and Control Plane happens over a Unix socket with authentication.

## Network Management

Flywall uses **netlink** for direct kernel communication:
- Interface configuration (IP addresses, MTU, state)
- Routing table management
- WireGuard configuration (via wgctrl)

## Firewall Engine

The firewall uses **nftables** with:
- Atomic rule application (all-or-nothing updates)
- Dynamic sets for runtime updates
- Connection tracking for stateful inspection
- Integrity monitoring (auto-restore on external changes)

### Rule Generation

```
HCL Config → Parsed Model → nftables Script → Kernel
```

1. Configuration is validated
2. Policies are expanded into nftables rules
3. Rules are applied atomically via `nft -f`
4. Integrity monitor watches for external changes

## Configuration System

### Hot Reload

Configuration changes can be applied without restart:
1. New config is parsed and validated
2. Diff is computed against running config
3. Only changed components are updated
4. Atomic rollback on failure

### Staged Configuration

The API supports a staged configuration model:
1. Changes are staged (not applied)
2. Diff can be reviewed
3. Apply commits all staged changes
4. Discard reverts to running config

## Services

### DHCP Server

Built-in DHCPv4 server:
- Lease management with persistence
- Static reservations
- DHCP options (DNS, gateway, NTP, etc.)
- Integration with DNS for automatic hostname registration

### DNS Resolver

Caching DNS resolver with:
- Multiple upstream support (UDP, DoH, DoT)
- Blocklist integration (hosts, domains, adblock format)
- Split-horizon DNS for local domains
- DHCP hostname integration

### VPN

Native WireGuard support via netlink/wgctrl:
- No external tools required
- Key management
- Peer configuration
- Tailscale integration (optional)

## Security Model

### Privilege Separation

| Component | User | Capabilities |
|-----------|------|--------------|
| Control Plane | root | Full network control |
| API Server | nobody | No direct network access |
| Web UI | N/A | Static files served by API |

### Network Namespace Sandbox

The API server runs in an isolated network namespace:
- Cannot bind to host network interfaces
- Communicates only via Unix socket to control plane
- Limits impact of API vulnerabilities

### Integrity Monitoring

The firewall monitors for external rule changes:
- Detects manual `nft` modifications
- Auto-restores authoritative ruleset
- Logs tampering attempts

## Data Storage

| Path | Purpose |
|------|---------|
| `/etc/flywall/flywall.hcl` | Configuration |
| `/var/lib/flywall/` | State (leases, keys, DB) |
| `/var/lib/flywall/flywall.db` | SQLite database |
| `/var/log/flywall/` | Logs |

## Upgrade Process

Seamless upgrades use socket handoff:
1. New binary starts, validates config
2. Old process transfers listening sockets
3. New process takes over connections
4. Old process exits
