# VPN Configuration: WireGuard and Tailscale
#
# This example demonstrates:
# 1. Native WireGuard tunnel configuration
# 2. Tailscale/Headscale integration
# 3. VPN zone management
# 4. Management access via VPN

schema_version = "1.0"
ip_forwarding = true

# -----------------------------------------------------------------------------
# Interfaces
# -----------------------------------------------------------------------------

interface "eth0" {
  description = "WAN"
  zone        = "wan"
  dhcp        = true
}

interface "eth1" {
  description = "LAN"
  zone        = "lan"
  ipv4        = ["192.168.1.1/24"]
}

# -----------------------------------------------------------------------------
# VPN Configuration
# -----------------------------------------------------------------------------

vpn {
  # WireGuard VPN Server
  wireguard "home-vpn" {
    enabled    = true
    interface  = "wg0"
    zone       = "vpn"
    listen_port = 51820

    private_key_file = "/etc/flywall/wg-private.key"
    address          = ["10.200.0.1/24"]
    mtu              = 1420

    # Management access via WireGuard (lockout protection)
    management_access = true

    # Road warrior peer (mobile device)
    peer "phone" {
      public_key  = "PEER_PUBLIC_KEY_HERE"
      allowed_ips = ["10.200.0.2/32"]
      persistent_keepalive = 25
    }

    # Site-to-site peer (remote office)
    peer "remote-office" {
      public_key  = "REMOTE_OFFICE_PUBKEY"
      endpoint    = "office.example.com:51820"
      allowed_ips = ["10.200.0.3/32", "192.168.10.0/24"]
      persistent_keepalive = 25
    }

    # Peer with preshared key for extra security
    peer "laptop" {
      public_key   = "LAPTOP_PUBLIC_KEY"
      preshared_key = "${WG_LAPTOP_PSK}"
      allowed_ips  = ["10.200.0.4/32"]
    }
  }

  # Second WireGuard tunnel (VPN provider for privacy)
  wireguard "privacy-vpn" {
    enabled    = true
    interface  = "wg1"
    zone       = "vpn_exit"

    private_key_file = "/etc/flywall/mullvad-private.key"
    address          = ["10.66.0.100/32"]
    dns              = ["10.64.0.1"]
    fwmark           = 51821
    table            = "auto"

    peer "mullvad" {
      public_key  = "MULLVAD_SERVER_PUBKEY"
      endpoint    = "mullvad-us-nyc.example.com:51820"
      allowed_ips = ["0.0.0.0/0"]
    }
  }

  # Tailscale integration
  tailscale "default" {
    enabled = true
    zone    = "tailscale"

    # For unattended setup
    auth_key_env = "TAILSCALE_AUTH_KEY"

    # Use Headscale instead of Tailscale control server
    # control_url = "https://headscale.example.com"

    # Always allow management access via Tailscale
    management_access = true

    # Advertise local networks to Tailscale
    advertise_routes = ["192.168.1.0/24"]
    accept_routes    = true

    # Act as exit node
    # advertise_exit_node = true
  }

  # Interface prefix zones (match wg0, wg1, wg2... to vpn zone)
  interface_prefix_zones = {
    "wg"        = "vpn"
    "tailscale" = "tailscale"
  }
}

# -----------------------------------------------------------------------------
# Zones
# -----------------------------------------------------------------------------

zone "wan" {
  description = "Internet"
  external    = true
}

zone "lan" {
  description = "Local Network"

  management {
    web  = true
    ssh  = true
    icmp = true
  }
}

zone "vpn" {
  description = "WireGuard VPN Clients"

  management {
    web  = true
    ssh  = true
    icmp = true
  }
}

zone "vpn_exit" {
  description = "Privacy VPN Exit"
  external    = true
}

zone "tailscale" {
  description = "Tailscale Network"

  management {
    web  = true
    ssh  = true
    api  = true
    icmp = true
  }
}

# -----------------------------------------------------------------------------
# Firewall Policies
# -----------------------------------------------------------------------------

# VPN clients can access LAN
policy "vpn" "lan" {
  action = "accept"
}

# VPN clients can access internet
policy "vpn" "wan" {
  action = "accept"
}

# Tailscale can access everything
policy "tailscale" "lan" {
  action = "accept"
}

policy "tailscale" "wan" {
  action = "accept"
}

# LAN to internet
policy "lan" "wan" {
  action = "accept"
}

# LAN to VPN exit (privacy routing)
policy "lan" "vpn_exit" {
  action = "accept"
}

# WAN to self (VPN port)
policy "wan" "self" {
  rule "allow-wireguard" {
    proto     = "udp"
    dest_port = 51820
    action    = "accept"
  }

  rule "allow-icmp" {
    proto  = "icmp"
    action = "accept"
  }
}

# -----------------------------------------------------------------------------
# NAT
# -----------------------------------------------------------------------------

nat "masquerade-wan" {
  type          = "masquerade"
  out_interface = "eth0"
}

nat "masquerade-vpn-exit" {
  type          = "masquerade"
  out_interface = "wg1"
}

# -----------------------------------------------------------------------------
# Protection
# -----------------------------------------------------------------------------

protection "wan_protection" {
  interface            = "eth0"
  anti_spoofing        = true
  bogon_filtering      = true
  syn_flood_protection = true
  syn_flood_rate       = 25
  syn_flood_burst      = 50
}
