---
title: "Configuration"
linkTitle: "Configuration"
weight: 40
description: >
  HCL configuration reference and examples.
---

Flywall is configured using HCL (HashiCorp Configuration Language) files, typically at `/opt/flywall/etc/flywall.hcl`.

## Quick Links

- [Configuration Reference]({{< relref "reference/" >}}) - Full reference of all options
- [Examples]({{< relref "examples/" >}}) - Real-world configuration examples

## Basic Structure

```hcl
# Schema version (for forward compatibility)
schema_version = "1.0"

# Global settings
ip_forwarding = true
state_dir     = "/opt/flywall/var/lib"

# Interface definitions
interface "eth0" {
  zone = "WAN"
  dhcp = true
}

interface "eth1" {
  zone = "LAN"
  ipv4 = ["192.168.1.1/24"]
}

# Zone definitions
zone "WAN" { }
zone "LAN" {
  management {
    web_ui = true
  }
}

# Policies (traffic rules)
policy "LAN" "WAN" {
  rule "allow-all" {
    action = "accept"
  }
}

# NAT
nat "outbound" {
  type          = "masquerade"
  out_interface = "eth0"
}

# Services
dhcp {
  scope "lan" {
    interface   = "eth1"
    range_start = "192.168.1.100"
    range_end   = "192.168.1.200"
  }
}

dns {
  forwarders = ["1.1.1.1", "8.8.8.8"]
}

# Web UI
web {
  listen = ":8080"
}
```

## Configuration Management

### Validate Before Applying

```bash
flywall validate -c /opt/flywall/etc/flywall.hcl
```

### Hot Reload

Apply changes without restart:

```bash
flywall reload
# or
kill -HUP $(cat /var/run/flywall.pid)
```

### Atomic Apply

Flywall applies configuration atomically. If any part fails, the entire change is rolled back.

## Environment Variable Substitution

Use environment variables in configuration:

```hcl
api {
  key {
    name = "admin"
    key  = env("FLYWALL_ADMIN_KEY")
  }
}
```

## File Includes

Split large configurations:

```hcl
# Main config
include "zones.hcl"
include "policies.hcl"
include "services.hcl"
```

## Next Steps

- Browse the [Configuration Reference]({{< relref "reference/" >}})
- See [Example Configurations]({{< relref "examples/" >}})
