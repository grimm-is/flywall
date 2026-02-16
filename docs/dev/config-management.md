# Configuration Management Implementation Guide

## Overview

Flywall uses HCL (HashiCorp Configuration Language) for configuration:
- Declarative configuration format
- Schema validation
- Hot reloading
- Configuration versioning
- Backup and rollback

## Architecture

### Configuration Components
1. **Parser**: HCL to internal representation
2. **Validator**: Schema and semantic validation
3. **Applier**: Configuration to system state
4. **Monitor**: File change detection
5. **Persistence**: Configuration storage

### Configuration Files
- Main config: `/etc/flywall/flywall.hcl`
- Includes: `/etc/flywall/conf.d/*.hcl`
- Runtime: `/run/flywall/config.hcl`
- Backup: `/var/lib/flywall/backups/`

## Configuration Structure

### Basic Configuration
```hcl
# Schema version (required)
schema_version = "1.1"

# Global settings
ip_forwarding = true
hostname = "firewall"
description = "Main Firewall"

# Time settings
timezone = "UTC"
ntp_servers = ["0.pool.ntp.org", "1.pool.ntp.org"]

# Logging
log_level = "info"
log_file = "/var/log/flywall.log"
```

### Interface Configuration
```hcl
# Physical interface
interface "eth0" {
  description = "WAN Interface"
  zone = "wan"
  ipv4 = ["203.0.113.10/24"]
  gateway = "203.0.113.1"
  dhcp = false
  mtu = 1500
}

# DHCP client
interface "eth1" {
  description = "LAN Interface"
  zone = "lan"
  dhcp = true

  dhcp_options {
    hostname = "firewall"
    vendor_class = "Flywall"
  }
}

# Virtual interface
interface "br0" {
  description = "LAN Bridge"
  zone = "lan"
  ipv4 = ["192.168.1.1/24"]

  bridge {
    ports = ["eth1", "eth2"]
    stp = true
  }
}
```

### Zone Configuration
```hcl
# Zone definition
zone "wan" {
  interface = "eth0"
  description = "External Network"

  services {
    dns = false
    dhcp = false
    learning = false
  }
}

zone "lan" {
  interface = "br0"
  description = "Internal Network"

  services {
    dns = true
    dhcp = true
    learning = true
  }

  protection {
    anti_spoofing = false
    invalid_packets = true
  }
}
```

### Policy Configuration
```hcl
# Simple policy
policy "lan" "wan" {
  name = "lan_to_wan"
  action = "accept"
}

# Complex policy with rules
policy "wan" "lan" {
  name = "wan_to_lan"

  rule "allow_established" {
    description = "Allow established connections"
    ct_state = "established,related"
    action = "accept"
  }

  rule "allow_ping" {
    description = "Allow ICMP"
    protocol = "icmp"
    action = "accept"
  }

  rule "block_all" {
    description = "Block everything else"
    action = "drop"
    log = true
  }
}
```

### Service Configuration
```hcl
# DHCP Server
dhcp {
  enabled = true

  scope "lan" {
    interface = "br0"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"
    router = "192.168.1.1"
    dns = ["192.168.1.1", "8.8.8.8"]
    lease_time = "24h"
  }
}

# DNS Server
dns {
  enabled = true
  listen_on = ["192.168.1.1"]
  forwarders = ["8.8.8.8", "1.1.1.1"]

  serve "lan" {
    local_domain = "lan"

    host "192.168.1.10" {
      hostnames = ["server", "server.lan"]
    }
  }
}

# API Server
api {
  enabled = true
  listen = "0.0.0.0:8080"

  auth {
    required = true
    token = "secret-token"
  }

  cors {
    allowed_origins = ["*"]
  }
}
```

## Advanced Features

### Configuration Includes
```hcl
# Main configuration
schema_version = "1.1"

# Include other files
include "/etc/flywall/interfaces/*.hcl"
include "/etc/flywall/policies/*.hcl"
include "/etc/flywall/services/*.hcl"

# Override with local config
include "/etc/flywall/local.hcl" {
  required = false
}
```

### Variables and Functions
```hcl
# Define variables
variable "wan_interface" {
  type = string
  default = "eth0"
}

variable "lan_network" {
  type = string
  default = "192.168.1.0/24"
}

# Use variables
interface var.wan_interface {
  zone = "wan"
  dhcp = true
}

# Functions
locals {
  lan_cidr = cidrhost(var.lan_network, 1)
  lan_gateway = cidrhost(var.lan_network, 254)
}

# Use computed values
interface "br0" {
  ipv4 = [local.lan_cidr]
  gateway = local.lan_gateway
}
```

### Conditional Configuration
```hcl
# Environment-specific config
locals {
  is_production = env("ENVIRONMENT") == "production"
  debug_enabled = env("DEBUG") == "true"
}

# Conditional features
dhcp {
  enabled = !local.is_production
  debug = local.debug_enabled
}

# Conditional policies
policy "wan" "lan" {
  name = "wan_to_lan"

  rule "debug_logging" {
    count = local.debug_enabled ? 1 : 0
    action = "accept"
    log = true
  }
}
```

## Configuration Management

### CLI Commands
```bash
# Show current configuration
flywall config show

# Get specific section
flywall config get interfaces
flywall config get zones.wan

# Validate configuration
flywall config validate /path/to/config.hcl

# Apply configuration
flywall config apply /path/to/config.hcl

# Edit configuration
flywall config edit

# Backup configuration
flywall config backup

# Restore configuration
flywall config restore backup-20231201-120000.hcl

# Show diff
flywall config diff /path/to/new.hcl
```

### Hot Reloading
```hcl
# Enable hot reload
watch_config {
  enabled = true
  path = "/etc/flywall"
  pattern = "*.hcl"
  interval = "5s"

  # Reload actions
  on_reload {
    validate = true
    backup = true
    apply = true

    # Notification
    notify {
      email = "admin@example.com"
      webhook = "https://hooks.example.com/flywall"
    }
  }
}
```

### Configuration Validation
```hcl
# Validation rules
validation {
  # Required fields
  required = ["schema_version"]

  # Custom validators
  rules {
    "check_interface_zones" = {
      check = "all_interfaces_have_zones"
      message = "All interfaces must be assigned to zones"
    }

    "check_policy_loops" = {
      check = "no_policy_loops"
      message = "Policies must not create loops"
    }
  }

  # Type checking
  strict_types = true

  # Deprecation warnings
  deprecated = {
    "global_protection" = "Use zone.protection instead"
  }
}
```

## API Integration

### Configuration API
```bash
# Get full configuration
curl -s "http://localhost:8080/api/config"

# Get configuration section
curl -s "http://localhost:8080/api/config/interfaces"

# Update configuration
curl -X PUT "http://localhost:8080/api/config" \
  -H "Content-Type: application/json" \
  -d '{
    "schema_version": "1.1",
    "interfaces": {...}
  }'

# Validate configuration
curl -X POST "http://localhost:8080/api/config/validate" \
  -H "Content-Type: application/json" \
  -d '{"config": {...}}'

# Get configuration diff
curl -s "http://localhost:8080/api/config/diff"
```

### Configuration History
```bash
# Get configuration history
curl -s "http://localhost:8080/api/config/history"

# Get specific version
curl -s "http://localhost:8080/api/config/history/123"

# Rollback to version
curl -X POST "http://localhost:8080/api/config/rollback/123"
```

## Best Practices

1. **Organization**
   - Use separate files for features
   - Group related settings
   - Use descriptive names
   - Document complex configs

2. **Validation**
   - Always validate before applying
   - Use schema versioning
   - Test in staging first
   - Keep backups

3. **Security**
   - Secure config files
   - Use secrets management
   - Audit config changes
   - Limit API access

4. **Maintenance**
   - Regular backups
   - Version control configs
   - Document changes
   - Monitor reloads

## Troubleshooting

### Common Issues
1. **Parse errors**: Check HCL syntax
2. **Validation failures**: Review error messages
3. **Apply failures**: Check dependencies
4. **Reload issues**: Check file permissions

### Debug Commands
```bash
# Validate syntax
flywall config validate --debug

# Check parsing
flywall config parse /path/to/config.hcl

# Show effective config
flywall config show --effective

# Check dependencies
flywall config deps

# Monitor reloads
flywall config watch
```

### Advanced Debugging
```bash
# Parse with HCL tool
hcl2json /etc/flywall/flywall.hcl

# Check schema
flywall schema show

# Trace configuration
flywall config trace /path/to/config.hcl

# Dry run apply
flywall config apply --dry-run /path/to/config.hcl
```

## Performance Considerations

- Parsing is fast (< 100ms for typical configs)
- Validation adds minimal overhead
- Hot reload is non-blocking
- Large configs may need optimization

## Security Considerations

- Encrypt sensitive data
- Use secrets management
- Audit config access
- Validate all inputs

## Related Features

- [Schema Migration](schema-migration.md)
- [State Persistence](state-persistence.md)
- [API Reference](api-reference.md)
- [CLI Tools](cli-tools.md)
