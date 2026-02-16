---
title: "Quickstart"
linkTitle: "Quickstart"
weight: 20
description: >
  Set up a basic home router with NAT, DHCP, and DNS in 5 minutes.
---

This guide walks you through setting up a basic home router configuration with:
- WAN interface connected to your ISP (DHCP client)
- LAN interface serving your local network
- NAT masquerading for internet access
- DHCP server for automatic IP assignment
- DNS caching and forwarding

## Prerequisites

- Flywall [installed]({{< relref "installation" >}})
- A Linux system with at least 2 network interfaces
- Root access

## Step 1: Create Configuration

Create the configuration file at `/opt/flywall/etc/flywall.hcl`:

```hcl
# Flywall Home Router Configuration
schema_version = "1.0"

# Enable routing between interfaces
ip_forwarding = true

# WAN Interface - connects to your ISP
interface "eth0" {
  zone = "WAN"
  dhcp = true  # Get IP from ISP
}

# LAN Interface - your local network
interface "eth1" {
  zone = "LAN"
  ipv4 = ["192.168.1.1/24"]
}

# Define security zones
zone "WAN" {
  # No management access from WAN by default
}

zone "LAN" {
  # Allow management from LAN
  management {
    ssh     = true
    web_ui  = true
    api     = true
  }
}

# Allow LAN to access WAN (internet)
policy "LAN" "WAN" {
  rule "allow-all" {
    action = "accept"
  }
}

# NAT for internet access
nat "outbound" {
  type          = "masquerade"
  out_interface = "eth0"  # WAN interface
}

# DHCP Server for LAN
dhcp {
  scope "lan" {
    interface   = "eth1"
    range_start = "192.168.1.100"
    range_end   = "192.168.1.200"
    lease_time  = "24h"

    # Network settings pushed to clients
    gateway = "192.168.1.1"
    dns     = ["192.168.1.1"]
  }
}

# DNS Server
dns {
  enabled = true
  listen  = "192.168.1.1:53"

  # Forward queries to public DNS
  forwarders = ["1.1.1.1", "8.8.8.8"]

  # Enable caching
  cache {
    enabled  = true
    max_size = 10000
  }
}

# Web UI
web {
  listen = ":8080"
}
```

## Step 2: Validate Configuration

Before starting, validate your configuration:

```bash
sudo flywall validate -c /opt/flywall/etc/flywall.hcl
```

Expected output:
```
Configuration is valid.
```

## Step 3: Start Flywall

Start Flywall (in foreground for initial testing):

```bash
sudo flywall start -c /opt/flywall/etc/flywall.hcl
```

Or start via systemd:

```bash
sudo systemctl start flywall
sudo systemctl status flywall
```

## Step 4: Verify Services

### Check Firewall Rules

```bash
# View generated nftables rules
sudo nft list ruleset
```

### Check DHCP Leases

```bash
# View active leases
flywall dhcp leases
```

### Access Web UI

Open your browser to `http://192.168.1.1:8080` to access the Flywall dashboard.

## Network Diagram

```
┌─────────────┐      ┌─────────────────────────┐      ┌─────────────┐
│   Internet  │──────│  eth0 (WAN)             │      │   Laptop    │
│   (ISP)     │ DHCP │  192.168.0.x            │      │ 192.168.1.x │
└─────────────┘      │                         │      └──────┬──────┘
                     │       Flywall           │             │
                     │                         │      ┌──────┴──────┐
                     │  eth1 (LAN)             │──────│   Phone     │
                     │  192.168.1.1/24         │      │ 192.168.1.x │
                     └─────────────────────────┘      └─────────────┘
```

## What's Next?

Now that you have a basic router running:

- [Add DNS Filtering]({{< relref "../guides/dhcp-dns" >}}) - Block ads and malware
- [Set Up Port Forwarding]({{< relref "../guides/nat-port-forwarding" >}}) - Expose services
- [Add VPN]({{< relref "../guides/wireguard" >}}) - Secure remote access
- [Web UI Guide]({{< relref "../guides/web-ui" >}}) - Learn the dashboard

## Troubleshooting

### No Internet Access from LAN

1. Check that IP forwarding is enabled:
   ```bash
   cat /proc/sys/net/ipv4/ip_forward
   # Should output: 1
   ```

2. Verify NAT rules are applied:
   ```bash
   sudo nft list chain inet fw nat_postrouting
   ```

### DHCP Not Working

1. Check that the DHCP service is listening:
   ```bash
   ss -ulnp | grep :67
   ```

2. Verify the interface has the correct IP:
   ```bash
   ip addr show eth1
   ```

### Can't Access Web UI

1. Check the web server is running:
   ```bash
   ss -tlnp | grep :8080
   ```

2. Verify you're connecting from an allowed zone (LAN).
