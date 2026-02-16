# Advanced Services Configuration
#
# This example demonstrates:
# 1. DNS configuration (new consolidated block)
# 2. QoS (Quality of Service) policies
# 3. mDNS Reflector, UPnP, NTP services
# 4. Dynamic DNS (DDNS)
# 5. Syslog remote logging

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

interface "eth2" {
  description = "IoT Network"
  zone        = "iot"
  ipv4        = ["192.168.100.1/24"]
}

# -----------------------------------------------------------------------------
# Zones with Services and Management
# -----------------------------------------------------------------------------

zone "wan" {
  description = "Internet"
  external    = true
}

zone "lan" {
  description = "Trusted LAN"

  # Services provided TO this zone (auto-generates firewall rules)
  services {
    dhcp = true
    dns  = true
    ntp  = true

    # Custom service port
    port "iperf" {
      protocol = "tcp"
      port     = 5201
    }
  }

  # Management access FROM this zone
  management {
    web  = true
    ssh  = true
    api  = true
    icmp = true
  }
}

zone "iot" {
  description = "IoT Devices"

  services {
    dhcp = true
    dns  = true
  }

  # IoT devices get limited management access
  management {
    icmp = true
  }
}

# -----------------------------------------------------------------------------
# DNS Configuration (Consolidated)
# -----------------------------------------------------------------------------

dns {
  enabled      = true
  listen_port  = 53
  local_domain = "home.lan"
  expand_hosts = true

  # Upstream DNS (encrypted)
  upstream_doh "cloudflare" {
    url      = "https://cloudflare-dns.com/dns-query"
    enabled  = true
    priority = 1
  }

  upstream_doh "google" {
    url      = "https://dns.google/dns-query"
    enabled  = true
    priority = 2
  }

  # DNS-over-TLS upstream
  upstream_dot "quad9" {
    server      = "9.9.9.9:853"
    server_name = "dns.quad9.net"
    enabled     = true
    priority    = 3
  }

  # Conditional forwarding for internal domains
  conditional_forward "corp.example.com" {
    servers = ["10.10.10.53", "10.10.10.54"]
  }

  # Security settings
  dnssec           = true
  rebind_protection = true
  query_logging    = true

  # Blocklists for ad/malware blocking
  blocklist "oisd" {
    url          = "https://big.oisd.nl/domainswild"
    format       = "domains"
    enabled      = true
    refresh_hours = 24
  }

  blocklist "hagezi-pro" {
    url          = "https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/domains/pro.txt"
    format       = "domains"
    enabled      = true
    refresh_hours = 12
  }

  # Allowlist (bypass blocklists)
  allowlist = ["example.com", "*.microsoft.com"]

  # Caching
  cache_enabled = true
  cache_size    = 10000
  cache_min_ttl = 60
  cache_max_ttl = 86400

  # Static DNS entries
  host "192.168.1.1" {
    hostnames = ["router", "gateway"]
  }

  host "192.168.1.10" {
    hostnames = ["nas", "storage"]
  }

  # Custom DNS zone
  zone "home.lan" {
    record "www" {
      type  = "A"
      value = "192.168.1.50"
      ttl   = 3600
    }

    record "mail" {
      type     = "MX"
      value    = "192.168.1.51"
      priority = 10
    }
  }
}

# -----------------------------------------------------------------------------
# DHCP Server
# -----------------------------------------------------------------------------

dhcp {
  enabled = true

  scope "lan-scope" {
    interface   = "eth1"
    range_start = "192.168.1.100"
    range_end   = "192.168.1.200"
    router      = "192.168.1.1"
    dns         = ["192.168.1.1"]
    domain      = "home.lan"

    reservation "00:11:22:33:44:55" {
      ip           = "192.168.1.10"
      hostname     = "nas"
      register_dns = true
    }
  }

  scope "iot-scope" {
    interface   = "eth2"
    range_start = "192.168.100.100"
    range_end   = "192.168.100.200"
    router      = "192.168.100.1"
    dns         = ["192.168.100.1"]
  }
}

# -----------------------------------------------------------------------------
# QoS (Quality of Service)
# -----------------------------------------------------------------------------

qos_policy "wan-shaping" {
  interface    = "eth0"
  enabled      = true
  direction    = "both"
  download_mbps = 100
  upload_mbps   = 20

  # Traffic classes
  class "realtime" {
    priority   = 1
    rate       = "20%"
    ceil       = "100%"
    queue_type = "fq_codel"
  }

  class "interactive" {
    priority   = 2
    rate       = "30%"
    ceil       = "100%"
    queue_type = "fq_codel"
  }

  class "bulk" {
    priority   = 5
    rate       = "20%"
    ceil       = "80%"
    queue_type = "fq_codel"
  }

  class "background" {
    priority   = 7
    rate       = "5%"
    ceil       = "50%"
    queue_type = "sfq"
  }

  # Classification rules
  rule "voip" {
    class     = "realtime"
    proto     = "udp"
    dest_port = 5060
  }

  rule "ssh" {
    class     = "interactive"
    proto     = "tcp"
    dest_port = 22
  }

  rule "gaming" {
    class    = "interactive"
    proto    = "udp"
    set_dscp = "ef"
  }

  rule "downloads" {
    class     = "bulk"
    proto     = "tcp"
    dest_port = 80
  }
}

# -----------------------------------------------------------------------------
# mDNS Reflector (for IoT/Chromecast/AirPlay across VLANs)
# -----------------------------------------------------------------------------

mdns {
  enabled    = true
  interfaces = ["eth1", "eth2"]
}

# -----------------------------------------------------------------------------
# UPnP IGD (for gaming consoles, etc.)
# -----------------------------------------------------------------------------

upnp {
  enabled            = true
  external_interface = "eth0"
  internal_interfaces = ["eth1"]
  secure_mode        = true
}

# -----------------------------------------------------------------------------
# NTP Service
# -----------------------------------------------------------------------------

ntp {
  enabled  = true
  servers  = ["time.cloudflare.com", "pool.ntp.org"]
  interval = "4h"
}

# -----------------------------------------------------------------------------
# Dynamic DNS
# -----------------------------------------------------------------------------

ddns {
  enabled   = true
  provider  = "cloudflare"
  hostname  = "home.example.com"
  zone_id   = "your-zone-id"
  record_id = "your-record-id"
  token     = "${CLOUDFLARE_API_TOKEN}"
  interface = "eth0"
  interval  = 5
}

# -----------------------------------------------------------------------------
# Remote Syslog
# -----------------------------------------------------------------------------

syslog {
  enabled  = true
  host     = "syslog.example.com"
  port     = 514
  protocol = "udp"
  tag      = "flywall"
}

# -----------------------------------------------------------------------------
# Firewall Policies
# -----------------------------------------------------------------------------

policy "lan" "wan" {
  action = "accept"
}

policy "iot" "wan" {
  action = "accept"
}

policy "lan" "iot" {
  rule "allow-cast" {
    proto     = "tcp"
    dest_port = 8008
    action    = "accept"
  }
}

# -----------------------------------------------------------------------------
# NAT
# -----------------------------------------------------------------------------

nat "masquerade" {
  type          = "masquerade"
  out_interface = "eth0"
}
