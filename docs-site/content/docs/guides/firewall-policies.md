---
title: "Zones & Firewall Policies"
linkTitle: "Firewall Policies"
weight: 10
description: >
  Understanding Flywall's zone-based firewall model.
---

Flywall uses a **zone-based firewall model** where network interfaces are assigned to security zones, and policies define what traffic is allowed between zones.

## Core Concepts

### Zones

A **zone** is a logical security boundary. Interfaces assigned to the same zone are considered equally trusted and can communicate freely.

Common zone patterns:
- **WAN** - Untrusted internet-facing interfaces
- **LAN** - Trusted internal network
- **DMZ** - Semi-trusted servers accessible from internet
- **GUEST** - Limited-trust guest network

### Policies

A **policy** defines traffic rules between two zones. The syntax is:

```hcl
policy "<source_zone>" "<destination_zone>" {
  # ...rules...
}
```

Key principles:
- **Implicit deny** - Traffic not explicitly allowed is blocked
- **Stateful** - Return traffic for established connections is automatically allowed
- **First match** - Rules are evaluated in order; first match wins

## Basic Configuration

### Define Zones

```hcl
# WAN zone - no management access
zone "WAN" {
  description = "Internet-facing interfaces"
}

# LAN zone - full trust
zone "LAN" {
  description = "Trusted internal network"

  management {
    ssh     = true
    web_ui  = true
    api     = true
  }
}

# Guest zone - limited access
zone "GUEST" {
  description = "Guest WiFi network"
}
```

### Assign Interfaces to Zones

```hcl
interface "eth0" {
  zone = "WAN"
  dhcp = true
}

interface "eth1" {
  zone = "LAN"
  ipv4 = ["192.168.1.1/24"]
}

interface "wlan0" {
  zone = "GUEST"
  ipv4 = ["10.10.10.1/24"]
}
```

### Define Policies

```hcl
# LAN can access everything
policy "LAN" "WAN" {
  rule "allow-all" {
    action = "accept"
  }
}

# Guest can only access internet (HTTP/HTTPS/DNS)
policy "GUEST" "WAN" {
  rule "allow-web" {
    action       = "accept"
    protocol     = "tcp"
    dest_port    = ["80", "443"]
    description  = "Allow web browsing"
  }

  rule "allow-dns" {
    action       = "accept"
    protocol     = "udp"
    dest_port    = ["53"]
    description  = "Allow DNS queries"
  }
}

# Block guest from accessing LAN
policy "GUEST" "LAN" {
  rule "block-all" {
    action = "reject"
  }
}
```

## Rule Options

### Actions

| Action | Description |
|--------|-------------|
| `accept` | Allow the traffic |
| `drop` | Silently discard (no response) |
| `reject` | Discard with ICMP unreachable |
| `log` | Log and continue evaluation |

### Matching Criteria

```hcl
rule "example" {
  action = "accept"

  # Protocol (tcp, udp, icmp, or number)
  protocol = "tcp"

  # Port matching (single, list, or range)
  dest_port   = ["22", "80", "443", "8000-8999"]
  source_port = ["1024-65535"]

  # IP matching (CIDR or IPSet name)
  source      = ["192.168.1.0/24"]
  destination = ["10.0.0.0/8"]

  # Time-based rules (kernel 5.4+)
  time_start = "09:00"
  time_end   = "17:00"
  weekdays   = ["mon", "tue", "wed", "thu", "fri"]

  # Rate limiting
  rate_limit = "10/second"
  rate_burst = 20

  # Logging
  log        = true
  log_prefix = "BLOCKED: "
}
```

## Common Patterns

### Allow Established/Related

Flywall automatically tracks connection state. You don't need explicit rules for return traffic.

### Allow ICMP (Ping)

```hcl
policy "WAN" "LAN" {
  rule "allow-ping" {
    action   = "accept"
    protocol = "icmp"
    # Optional: limit ping types
    # icmp_type = ["echo-request", "echo-reply"]
  }
}
```

### Time-Based Access Control

Block social media during work hours:

```hcl
policy "LAN" "WAN" {
  rule "block-social-workday" {
    action      = "reject"
    destination = ["@social_media"]  # IPSet reference
    time_start  = "09:00"
    time_end    = "17:00"
    weekdays    = ["mon", "tue", "wed", "thu", "fri"]
  }

  rule "allow-all" {
    action = "accept"
  }
}
```

### Rate Limiting

Prevent brute force:

```hcl
policy "WAN" "LAN" {
  rule "rate-limit-ssh" {
    action     = "accept"
    protocol   = "tcp"
    dest_port  = ["22"]
    rate_limit = "3/minute"
    rate_burst = 5
  }
}
```

## IPSets for Grouping

Create reusable address groups:

```hcl
ipset "trusted_admins" {
  type    = "hash:ip"
  entries = ["192.168.1.10", "192.168.1.11"]
}

ipset "blocked_countries" {
  type = "hash:net"
  # Populated by threat intel service
}

policy "WAN" "LAN" {
  rule "allow-admin-ssh" {
    action    = "accept"
    protocol  = "tcp"
    dest_port = ["22"]
    source    = ["@trusted_admins"]  # Reference IPSet with @
  }
}
```

## Viewing Active Rules

```bash
# Show generated nftables rules
sudo nft list ruleset

# Show policies via API
curl -s http://localhost:8080/api/policies | jq
```

## Troubleshooting

### Traffic Being Blocked

1. Enable logging on the default deny rule
2. Check `/var/log/flywall/firewall.log`
3. Use `flywall debug trace` for real-time packet tracing

### Rules Not Applying

1. Validate configuration: `flywall validate`
2. Reload: `flywall reload` or `SIGHUP`
3. Check for syntax errors in logs

## Next Steps

- [NAT Configuration]({{< relref "nat-port-forwarding" >}}) - Set up NAT and port forwarding
- [Configuration Reference]({{< relref "../configuration/reference/" >}}) - Full policy options
