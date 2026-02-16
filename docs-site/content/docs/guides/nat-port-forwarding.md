---
title: "NAT & Port Forwarding"
linkTitle: "NAT & Port Forwarding"
weight: 30
description: >
  Configure NAT masquerading, DNAT, and port forwarding rules.
---

Flywall supports various NAT configurations for sharing internet access and exposing internal services.

## NAT Types

| Type | Use Case |
|------|----------|
| **Masquerade** | Share one public IP across multiple devices (dynamic WAN IP) |
| **SNAT** | Like masquerade but with static source IP |
| **DNAT** | Redirect incoming traffic to internal hosts (port forwarding) |

---

## Outbound NAT (Masquerade)

The most common NAT configuration - allows LAN devices to access the internet through a single WAN IP.

```hcl
nat "outbound" {
  type          = "masquerade"
  out_interface = "eth0"  # WAN interface
}
```

### With Source Filtering

Only NAT traffic from specific networks:

```hcl
nat "outbound-lan" {
  type          = "masquerade"
  out_interface = "eth0"
  source        = ["192.168.1.0/24"]
}

nat "outbound-guest" {
  type          = "masquerade"
  out_interface = "eth0"
  source        = ["10.10.10.0/24"]
}
```

### Static SNAT

Use when your WAN IP is static and you want explicit control:

```hcl
nat "snat-wan" {
  type          = "snat"
  out_interface = "eth0"
  to_address    = "203.0.113.5"  # Your public IP
}
```

---

## Port Forwarding (DNAT)

Expose internal services to the internet.

### Basic Port Forward

Forward external port to internal server:

```hcl
# Forward port 443 to internal web server
nat "web-server" {
  type         = "dnat"
  in_interface = "eth0"          # WAN interface
  protocol     = "tcp"
  dest_port    = 443
  to_address   = "192.168.1.10"  # Internal server
  to_port      = 443
}
```

### Different Internal Port

Forward external port 8443 to internal port 443:

```hcl
nat "web-alt-port" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "tcp"
  dest_port    = 8443            # External port
  to_address   = "192.168.1.10"
  to_port      = 443             # Internal port
}
```

### Port Range

Forward a range of ports:

```hcl
# Game server ports
nat "game-server" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "udp"
  dest_port    = "27015-27030"
  to_address   = "192.168.1.50"
}
```

### Multiple Ports

Forward multiple discrete ports:

```hcl
nat "mail-server" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "tcp"
  dest_port    = ["25", "587", "993"]
  to_address   = "192.168.1.20"
}
```

---

## Hairpin NAT

Allow internal hosts to access DNAT'd services using the public IP/domain.

```hcl
nat "web-server" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "tcp"
  dest_port    = 443
  to_address   = "192.168.1.10"
  to_port      = 443

  # Enable hairpin NAT
  hairpin = true
}
```

Without hairpin NAT:
- External: `https://mysite.com` → Works ✓
- Internal: `https://mysite.com` → Fails ✗

With hairpin NAT:
- External: `https://mysite.com` → Works ✓
- Internal: `https://mysite.com` → Works ✓

---

## 1:1 NAT

Map an entire external IP to an internal IP (requires additional public IPs):

```hcl
nat "dmz-server" {
  type         = "dnat"
  in_interface = "eth0"
  dest_address = "203.0.113.10"  # Additional public IP
  to_address   = "192.168.2.10"  # DMZ server
}

nat "dmz-outbound" {
  type          = "snat"
  out_interface = "eth0"
  source        = ["192.168.2.10"]
  to_address    = "203.0.113.10"
}
```

---

## UPnP / NAT-PMP

Allow applications to automatically request port forwards:

```hcl
upnp {
  enabled = true

  # Interfaces to listen on
  interfaces = ["eth1"]

  # Security settings
  allowed_ranges = ["192.168.1.0/24"]

  # Optional: restrict protocols
  allow_tcp = true
  allow_udp = true

  # Lease duration
  max_lease = "24h"
}
```

View active UPnP mappings:

```bash
flywall upnp list
```

---

## Policy Integration

DNAT changes the destination address **before** policy evaluation. You need policies to allow the traffic:

```hcl
# Port forward
nat "web-server" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "tcp"
  dest_port    = 443
  to_address   = "192.168.1.10"
}

# Policy to allow the forwarded traffic
policy "WAN" "LAN" {
  rule "allow-web-forward" {
    action      = "accept"
    protocol    = "tcp"
    dest_port   = ["443"]
    destination = ["192.168.1.10"]
  }
}
```

---

## Examples

### Home Server Setup

```hcl
# SSH (non-standard external port for security)
nat "ssh" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "tcp"
  dest_port    = 2222        # External
  to_address   = "192.168.1.10"
  to_port      = 22          # Internal
}

# Web Server
nat "https" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "tcp"
  dest_port    = 443
  to_address   = "192.168.1.10"
  hairpin      = true
}

# Plex
nat "plex" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "tcp"
  dest_port    = 32400
  to_address   = "192.168.1.15"
}
```

### Gaming Console

```hcl
# PlayStation / Xbox
nat "gaming-tcp" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "tcp"
  dest_port    = ["3478-3480"]
  to_address   = "192.168.1.100"
}

nat "gaming-udp" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "udp"
  dest_port    = ["3478-3479", "49152-65535"]
  to_address   = "192.168.1.100"
}
```

---

## Viewing NAT Rules

```bash
# View NAT configuration
flywall nat list

# View generated nftables NAT rules
sudo nft list chain inet fw nat_prerouting
sudo nft list chain inet fw nat_postrouting
```

---

## Troubleshooting

### Port Forward Not Working

1. **Check NAT rule exists:**
   ```bash
   sudo nft list chain inet fw nat_prerouting | grep dnat
   ```

2. **Check firewall policy allows traffic:**
   ```bash
   flywall debug trace --proto tcp --dport 443
   ```

3. **Verify internal server is listening:**
   ```bash
   nc -zv 192.168.1.10 443
   ```

### Hairpin NAT Not Working

1. Ensure `hairpin = true` is set on the NAT rule
2. Check that LAN interface is correctly identified
3. Verify internal DNS resolves to the public IP

## Next Steps

- [WireGuard VPN]({{< relref "wireguard" >}})
- [Multi-WAN Failover]({{< relref "multi-wan-failover" >}})
