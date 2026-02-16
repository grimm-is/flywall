---
title: "WireGuard VPN"
linkTitle: "WireGuard VPN"
weight: 40
description: >
  Set up WireGuard VPN for secure remote access.
---

Flywall includes native WireGuard support via netlink, requiring no external tools. This guide covers common VPN configurations.

## Prerequisites

- Linux kernel 5.6+ (WireGuard built-in) or wireguard-dkms installed
- WireGuard key pairs for server and clients

## Generate Keys

Generate keys on the Flywall router:

```bash
# Server keys
wg genkey | tee /etc/flywall/wg-server.key | wg pubkey > /etc/flywall/wg-server.pub

# Client keys (do this for each client)
wg genkey | tee client1.key | wg pubkey > client1.pub
```

---

## Road Warrior VPN

Allow remote clients to access your home network.

### Server Configuration

```hcl
# WireGuard VPN
vpn {
  wireguard "wg0" {
    address     = "10.100.0.1/24"
    listen_port = 51820
    private_key = "file:///etc/flywall/wg-server.key"

    # Remote client
    peer "client1" {
      public_key  = "CLIENT1_PUBLIC_KEY_HERE"
      allowed_ips = ["10.100.0.2/32"]
    }

    peer "client2" {
      public_key  = "CLIENT2_PUBLIC_KEY_HERE"
      allowed_ips = ["10.100.0.3/32"]
    }
  }
}

# Assign VPN interface to a zone
interface "wg0" {
  zone = "VPN"
}

# VPN zone configuration
zone "VPN" {
  description = "WireGuard VPN clients"
}

# Allow VPN to access LAN
policy "VPN" "LAN" {
  rule "allow-all" {
    action = "accept"
  }
}

# Allow VPN to access internet
policy "VPN" "WAN" {
  rule "allow-all" {
    action = "accept"
  }
}
```

### Client Configuration

Generate a client config file:

```ini
[Interface]
PrivateKey = CLIENT1_PRIVATE_KEY
Address = 10.100.0.2/24
DNS = 192.168.1.1

[Peer]
PublicKey = SERVER_PUBLIC_KEY_HERE
Endpoint = your-public-ip.example.com:51820
AllowedIPs = 0.0.0.0/0  # Route all traffic through VPN
# Or for split tunnel:
# AllowedIPs = 192.168.1.0/24, 10.100.0.0/24
PersistentKeepalive = 25
```

### Port Forward for WireGuard

If Flywall is behind another router, forward UDP 51820:

```hcl
nat "wireguard" {
  type         = "dnat"
  in_interface = "eth0"
  protocol     = "udp"
  dest_port    = 51820
  to_address   = "192.168.1.1"  # Flywall's LAN IP
}
```

---

## Site-to-Site VPN

Connect two networks together.

### Site A (Main Office)

```hcl
vpn {
  wireguard "wg-s2s" {
    address     = "10.200.0.1/30"
    listen_port = 51821
    private_key = "file:///etc/flywall/wg-sitea.key"

    peer "site-b" {
      public_key  = "SITE_B_PUBLIC_KEY"
      endpoint    = "site-b.example.com:51821"
      allowed_ips = ["10.200.0.2/32", "192.168.2.0/24"]  # Site B networks
    }
  }
}

# Route to Site B network
route "site-b" {
  destination = "192.168.2.0/24"
  gateway     = "10.200.0.2"  # Via WireGuard tunnel
}
```

### Site B (Branch Office)

```hcl
vpn {
  wireguard "wg-s2s" {
    address     = "10.200.0.2/30"
    listen_port = 51821
    private_key = "file:///etc/flywall/wg-siteb.key"

    peer "site-a" {
      public_key  = "SITE_A_PUBLIC_KEY"
      endpoint    = "site-a.example.com:51821"
      allowed_ips = ["10.200.0.1/32", "192.168.1.0/24"]  # Site A networks
    }
  }
}

route "site-a" {
  destination = "192.168.1.0/24"
  gateway     = "10.200.0.1"
}
```

---

## Split Tunnel vs Full Tunnel

### Full Tunnel (All Traffic)

Route all client traffic through VPN:

```ini
# Client config
[Peer]
AllowedIPs = 0.0.0.0/0, ::/0
```

Server needs NAT for VPN clients:

```hcl
nat "vpn-outbound" {
  type          = "masquerade"
  out_interface = "eth0"
  source        = ["10.100.0.0/24"]  # VPN subnet
}
```

### Split Tunnel (Specific Networks)

Only route specific networks through VPN:

```ini
# Client config
[Peer]
AllowedIPs = 192.168.1.0/24, 10.100.0.0/24
```

---

## Tailscale Integration

Flywall can integrate with an existing Tailscale network:

```hcl
vpn {
  tailscale {
    enabled     = true
    auth_key    = "tskey-xxx"  # Or use interactive auth
    hostname    = "flywall"

    # Advertise local routes
    advertise_routes = ["192.168.1.0/24"]

    # Accept routes from other nodes
    accept_routes = true

    # Act as exit node
    exit_node = false
  }
}
```

Check Tailscale status:

```bash
flywall tailscale status
```

---

## VPN Lockout Protection

Prevent locking yourself out when configuring VPN:

```hcl
vpn {
  wireguard "wg0" {
    # ...configuration...

    # Automatically revert if no keepalive in 5 minutes
    lockout_protection {
      enabled = true
      timeout = "5m"
    }
  }
}
```

---

## Monitoring

### Check WireGuard Status

```bash
# Via flywall CLI
flywall vpn status

# Direct wg command (if available)
sudo wg show
```

### View Connected Peers

```bash
curl http://localhost:8080/api/vpn/wireguard/wg0/peers
```

---

## Troubleshooting

### Peers Not Connecting

1. **Check UDP port is open:**
   ```bash
   ss -ulnp | grep 51820
   ```

2. **Verify firewall allows WireGuard:**
   ```bash
   sudo nft list ruleset | grep 51820
   ```

3. **Check keys match:**
   - Server's public key must be in client config
   - Client's public key must be in server config

### Handshake But No Traffic

1. **Check AllowedIPs:**
   - Server: Should include client's VPN IP
   - Client: Should include networks to route through VPN

2. **Check routing:**
   ```bash
   ip route get 192.168.1.10
   ```

### Performance Issues

1. **Check MTU:**
   ```hcl
   vpn {
     wireguard "wg0" {
       mtu = 1420  # Default, reduce if needed
     }
   }
   ```

2. **Enable TCP MSS clamping if needed**

## Next Steps

- [Multi-WAN Failover]({{< relref "multi-wan-failover" >}})
- [Firewall Policies]({{< relref "firewall-policies" >}})
