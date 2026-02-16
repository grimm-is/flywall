---
title: "DHCP & DNS"
linkTitle: "DHCP & DNS"
weight: 20
description: >
  Configure DHCP server and DNS resolver with filtering.
---

Flywall includes a built-in DHCP server and DNS resolver, eliminating the need for separate dnsmasq or ISC DHCP installations.

## DHCP Server

### Basic Configuration

```hcl
dhcp {
  scope "lan" {
    interface   = "eth1"
    range_start = "192.168.1.100"
    range_end   = "192.168.1.200"
    lease_time  = "24h"

    # Network settings
    gateway = "192.168.1.1"
    dns     = ["192.168.1.1"]  # Point to Flywall's DNS
    domain  = "home.local"
  }
}
```

### Static Reservations

Assign fixed IPs to specific devices:

```hcl
dhcp {
  scope "lan" {
    interface   = "eth1"
    range_start = "192.168.1.100"
    range_end   = "192.168.1.200"

    reservation "dc:a6:32:xx:xx:xx" {
      ip          = "192.168.1.10"
      hostname    = "homeserver"
      description = "Home NAS"
    }

    reservation "aa:bb:cc:dd:ee:ff" {
      ip       = "192.168.1.11"
      hostname = "printer"
    }
  }
}
```

### Multiple Scopes

For networks with multiple VLANs:

```hcl
dhcp {
  scope "lan" {
    interface   = "eth1.10"  # VLAN 10
    range_start = "192.168.10.100"
    range_end   = "192.168.10.200"
    gateway     = "192.168.10.1"
    dns         = ["192.168.10.1"]
  }

  scope "iot" {
    interface   = "eth1.20"  # VLAN 20
    range_start = "192.168.20.100"
    range_end   = "192.168.20.200"
    gateway     = "192.168.20.1"
    dns         = ["192.168.20.1"]
    lease_time  = "12h"
  }

  scope "guest" {
    interface   = "eth1.30"  # VLAN 30
    range_start = "10.10.10.100"
    range_end   = "10.10.10.200"
    gateway     = "10.10.10.1"
    dns         = ["10.10.10.1"]
    lease_time  = "2h"  # Short leases for guests
  }
}
```

### DHCP Options

Pass additional options to clients:

```hcl
dhcp {
  scope "lan" {
    interface   = "eth1"
    range_start = "192.168.1.100"
    range_end   = "192.168.1.200"

    options {
      ntp_servers  = ["192.168.1.1"]
      tftp_server  = "192.168.1.5"
      boot_file    = "pxelinux.0"
      time_offset  = -21600  # CST timezone
    }
  }
}
```

### View Leases

```bash
# CLI
flywall dhcp leases

# API
curl http://localhost:8080/api/dhcp/leases
```

---

## DNS Resolver

### Basic Forwarding

```hcl
dns {
  enabled = true
  listen  = "0.0.0.0:53"

  forwarders = ["1.1.1.1", "8.8.8.8"]

  cache {
    enabled  = true
    max_size = 10000
  }
}
```

### DNS over HTTPS (DoH)

```hcl
dns {
  enabled = true

  upstream_doh "cloudflare" {
    url      = "https://cloudflare-dns.com/dns-query"
    priority = 1
    enabled  = true
  }

  upstream_doh "google" {
    url      = "https://dns.google/dns-query"
    priority = 2
    enabled  = true
  }
}
```

### DNS over TLS (DoT)

```hcl
dns {
  enabled = true

  upstream_dot "cloudflare" {
    server      = "1.1.1.1:853"
    server_name = "cloudflare-dns.com"
    priority    = 1
  }
}
```

### Split-Horizon DNS

Serve different answers based on zone:

```hcl
dns {
  serve "home.local" {
    zone = "LAN"

    # Integrate with DHCP for automatic hostname registration
    dhcp_integration = true
    local_domain     = "home.local"

    # Static entries
    record "A" {
      name  = "nas"
      value = "192.168.1.10"
    }

    record "CNAME" {
      name  = "media"
      value = "nas.home.local"
    }
  }
}
```

---

## DNS Filtering

### Ad & Malware Blocking

```hcl
dns {
  serve "*" {
    blocklist "ads" {
      url    = "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"
      format = "hosts"
    }

    blocklist "malware" {
      url    = "https://urlhaus.abuse.ch/downloads/hostfile/"
      format = "hosts"
    }

    blocklist "tracking" {
      url    = "https://v.firebog.net/hosts/Easyprivacy.txt"
      format = "domains"
    }

    # Refresh blocklists daily
    blocklist_refresh = "24h"
  }
}
```

### Allowlist Exceptions

```hcl
dns {
  serve "*" {
    blocklist "ads" {
      url = "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"
    }

    # Override blocklist for specific domains
    allowlist = [
      "analytics.example.com",
      "tracking.partner.com"
    ]
  }
}
```

### Custom Blocklist File

```hcl
dns {
  serve "*" {
    blocklist "custom" {
      file   = "/etc/flywall/blocked-domains.txt"
      format = "domains"
    }
  }
}
```

---

## DHCP-DNS Integration

Automatically register DHCP hostnames in DNS:

```hcl
dhcp {
  scope "lan" {
    interface = "eth1"
    # ...
  }
}

dns {
  serve "home.local" {
    dhcp_integration = true
    local_domain     = "home.local"
    expand_hosts     = true
  }
}
```

Now `homeserver.home.local` resolves automatically when the device gets a DHCP lease.

---

## Monitoring

### DNS Statistics

```bash
curl http://localhost:8080/api/dns/stats
```

Returns:
```json
{
  "queries_total": 15234,
  "cache_hits": 12456,
  "cache_hit_rate": 0.82,
  "blocked_queries": 1823,
  "upstream_latency_ms": 15
}
```

### Query Log

Enable query logging for debugging:

```hcl
dns {
  query_log {
    enabled = true
    path    = "/var/log/flywall/dns-queries.log"
  }
}
```

---

## Troubleshooting

### DNS Not Resolving

1. Check DNS is listening:
   ```bash
   ss -ulnp | grep :53
   ```

2. Test resolution directly:
   ```bash
   dig @192.168.1.1 google.com
   ```

3. Check upstream connectivity:
   ```bash
   flywall dns test-upstream
   ```

### DHCP Not Assigning IPs

1. Check DHCP is listening:
   ```bash
   ss -ulnp | grep :67
   ```

2. Verify interface has static IP:
   ```bash
   ip addr show eth1
   ```

3. Check for IP conflicts in the range

### Blocked Domain Not Loading

1. Check if domain is in blocklist:
   ```bash
   flywall dns lookup blocked-domain.com
   ```

2. Add to allowlist if needed

## Next Steps

- [NAT Configuration]({{< relref "nat-port-forwarding" >}})
- [WireGuard VPN]({{< relref "wireguard" >}})
