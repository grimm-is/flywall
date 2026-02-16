---
title: "Multi-WAN Failover"
linkTitle: "Multi-WAN Failover"
weight: 50
description: >
  Configure multiple internet connections with health checks and failover.
---

Flywall supports multiple WAN connections with automatic failover, load balancing, and policy-based routing.

## Overview

Multi-WAN capabilities:
- **Active/Passive** - Primary connection with automatic failover
- **Load Balancing** - Distribute connections across links
- **Policy Routing** - Route specific traffic through specific WANs

---

## Basic Dual-WAN

### Interface Configuration

```hcl
# Primary WAN
interface "eth0" {
  zone = "WAN"
  dhcp = true

  # Mark this as a WAN for multi-WAN system
  wan {
    priority = 1  # Lower = preferred
    weight   = 1  # For load balancing
  }
}

# Secondary WAN
interface "eth1" {
  zone = "WAN"
  dhcp = true

  wan {
    priority = 2  # Failover target
    weight   = 1
  }
}
```

### Uplink Groups

Define how WANs work together:

```hcl
uplink_group "default" {
  # Failover mode: use primary until it fails
  mode = "failover"

  member "eth0" {
    priority = 1
  }

  member "eth1" {
    priority = 2
  }
}
```

---

## Health Checks

Configure how Flywall detects WAN failures:

```hcl
uplink_group "default" {
  mode = "failover"

  member "eth0" {
    priority = 1

    health_check {
      # Ping these targets
      targets = ["1.1.1.1", "8.8.8.8"]

      # Check every 5 seconds
      interval = "5s"

      # Mark down after 3 failures
      threshold_down = 3

      # Mark up after 2 successes
      threshold_up = 2

      # Timeout for each check
      timeout = "2s"
    }
  }

  member "eth1" {
    priority = 2

    health_check {
      targets        = ["1.0.0.1", "8.8.4.4"]
      interval       = "5s"
      threshold_down = 3
      threshold_up   = 2
    }
  }
}
```

### Custom Health Checks

Use HTTP checks for more reliability:

```hcl
health_check {
  type = "http"
  url  = "http://connectivitycheck.gstatic.com/generate_204"

  # Expect HTTP 204
  expected_status = 204

  interval       = "10s"
  threshold_down = 2
}
```

---

## Load Balancing

Distribute outbound connections across multiple WANs:

```hcl
uplink_group "balanced" {
  mode = "balance"

  member "eth0" {
    weight = 2  # Gets 2/3 of traffic
  }

  member "eth1" {
    weight = 1  # Gets 1/3 of traffic
  }
}
```

### Sticky Sessions

Keep connections from the same source on the same WAN:

```hcl
uplink_group "balanced" {
  mode = "balance"

  # Hash by source IP for sticky sessions
  sticky = true
  sticky_timeout = "1h"

  member "eth0" { weight = 1 }
  member "eth1" { weight = 1 }
}
```

---

## Policy-Based Routing

Route specific traffic through specific WANs:

```hcl
# Work VPN always through primary WAN
policy_route "work-vpn" {
  match {
    protocol  = "udp"
    dest_port = ["1194"]  # OpenVPN
  }

  uplink = "eth0"
}

# Video streaming through secondary (if it's faster)
policy_route "streaming" {
  match {
    destination = ["@streaming_services"]  # IPSet
  }

  uplink = "eth1"
}

# Gaming always through low-latency connection
policy_route "gaming" {
  match {
    source    = ["192.168.1.50"]  # Gaming PC
    protocol  = "udp"
  }

  uplink = "eth0"
}
```

---

## Different ISP Types

### Cable + DSL

```hcl
interface "eth0" {
  zone = "WAN"
  dhcp = true  # Cable modem

  wan {
    priority  = 1
    bandwidth = "100Mbps"  # For QoS calculations
  }
}

interface "eth1" {
  zone = "WAN"

  # DSL with PPPoE
  pppoe {
    username = "user@isp.com"
    password = "secret"
  }

  wan {
    priority  = 2
    bandwidth = "20Mbps"
  }
}
```

### LTE Backup

```hcl
interface "eth0" {
  zone = "WAN"
  dhcp = true

  wan { priority = 1 }
}

# LTE USB modem appears as eth1 or usb0
interface "usb0" {
  zone = "WAN"
  dhcp = true

  wan {
    priority = 10  # Only use when primary is down
  }

  health_check {
    # More aggressive checks for backup
    interval       = "10s"
    threshold_down = 2
    threshold_up   = 3
  }
}
```

---

## Monitoring

### Check WAN Status

```bash
flywall wan status
```

Output:
```
INTERFACE  STATUS  PRIORITY  LATENCY  PACKET LOSS
eth0       UP      1         12ms     0.0%
eth1       UP      2         25ms     0.1%
```

### View Failover Events

```bash
flywall wan history
```

### API Endpoint

```bash
curl http://localhost:8080/api/wan/status
```

---

## Notifications

Get notified on WAN state changes:

```hcl
notifications {
  # Email
  email {
    enabled = true
    smtp    = "smtp.gmail.com:587"
    to      = ["admin@example.com"]
    events  = ["wan_down", "wan_up", "failover"]
  }

  # Webhook
  webhook {
    enabled = true
    url     = "https://hooks.slack.com/services/xxx"
    events  = ["wan_down", "failover"]
  }
}
```

---

## Troubleshooting

### Failover Not Triggering

1. **Check health check targets are reachable:**
   ```bash
   ping -I eth0 1.1.1.1
   ```

2. **View health check status:**
   ```bash
   flywall wan health
   ```

3. **Check logs:**
   ```bash
   journalctl -u flywall | grep -i health
   ```

### Traffic Not Balanced

1. **Verify uplink group mode:**
   ```hcl
   mode = "balance"  # Not "failover"
   ```

2. **Check connection tracking:**
   - Existing connections stay on original WAN
   - Only new connections are balanced

### Slow Failover

1. **Reduce health check interval:**
   ```hcl
   interval = "2s"
   threshold_down = 2
   ```

2. **Use multiple check targets:**
   ```hcl
   targets = ["1.1.1.1", "8.8.8.8", "9.9.9.9"]
   ```

## Next Steps

- [Firewall Policies]({{< relref "firewall-policies" >}})
- [Configuration Reference]({{< relref "../configuration/reference/" >}})
