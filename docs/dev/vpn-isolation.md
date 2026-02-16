# VPN Isolation Implementation Guide

## Overview

Flywall provides VPN isolation for:
- Traffic segmentation for VPN clients
- Separate routing tables
- Isolated DNS resolution
- Access control policies
- Preventing VPN bypass

## Architecture

### VPN Isolation Components
1. **Isolation Manager**: Manages VPN client isolation
2. **Routing Table**: Separate routing for VPN traffic
3. **Policy Engine**: Enforces isolation policies
4. **DNS Manager**: Isolated DNS for VPN clients
5. **Monitor**: Tracks VPN client activity

### Isolation Types
- **Full Isolation**: Complete network separation
- **Split Tunnel**: Selective routing
- **Policy-Based**: Rule-based isolation
- **Per-Client**: Individual client isolation

## Configuration

### Basic VPN Isolation
```hcl
# VPN isolation configuration
wireguard "wg0" {
  enabled = true
  listen_port = 51820
  private_key = "${WG_PRIVATE_KEY}"

  # Isolation settings
  isolation = {
    enabled = true
    type = "full"
  }

  # Peers
  peer "client1" {
    public_key = "CLIENT1_PUBLIC_KEY"
    allowed_ips = ["10.200.0.2/32"]
  }
}
```

### Advanced VPN Isolation
```hcl
wireguard "wg0" {
  enabled = true
  listen_port = 51820
  private_key = "${WG_PRIVATE_KEY}"

  # Isolation configuration
  isolation = {
    enabled = true
    type = "policy_based"

    # Routing table
    routing_table = 100

    # DNS settings
    dns = {
      enabled = true
      servers = ["10.200.0.1", "1.1.1.1"]
      domains = ["vpn.example.com"]
    }

    # Firewall rules
    firewall = {
      default_policy = "drop"

      rules = [
        # Allow VPN clients to access specific services
        {
          description = "Allow access to internal servers"
          src = "10.200.0.0/24"
          dst = "192.168.1.0/24"
          ports = [22, 443, 8443]
          action = "accept"
        },

        # Allow internet access
        {
          description = "Allow internet access"
          src = "10.200.0.0/24"
          dst = "0.0.0.0/0"
          action = "accept"
        },

        # Block access to LAN
        {
          description = "Block LAN access"
          src = "10.200.0.0/24"
          dst = "192.168.0.0/16"
          action = "drop"
        }
      ]
    }

    # NAT settings
    nat = {
      enabled = true
      outbound_interface = "eth0"
    }

    # Logging
    log = {
      enabled = true
      dropped = true
      accepted = false
    }
  }

  # Per-peer isolation
  peer "client1" {
    public_key = "CLIENT1_PUBLIC_KEY"
    allowed_ips = ["10.200.0.2/32"]

    # Client-specific isolation
    isolation = {
      override = true

      # Restricted access
      allowed_networks = ["10.200.0.0/24", "203.0.113.0/24"]
      blocked_networks = ["192.168.0.0/16"]

      # Time restrictions
      time_restrictions = {
        start = "09:00"
        end = "17:00"
        days = ["mon", "tue", "wed", "thu", "fri"]
      }
    }
  }

  peer "client2" {
    public_key = "CLIENT2_PUBLIC_KEY"
    allowed_ips = ["10.200.0.3/32"]

    # Full access for admin
    isolation = {
      override = true
      type = "split_tunnel"

      # Route all traffic through VPN
      allowed_networks = ["0.0.0.0/0"]

      # Full LAN access
      allowed_networks = ["192.168.0.0/16"]
    }
  }
}
```

### Split Tunnel Configuration
```hcl
wireguard "wg0" {
  enabled = true
  listen_port = 51820
  private_key = "${WG_PRIVATE_KEY}"

  # Split tunnel isolation
  isolation = {
    enabled = true
    type = "split_tunnel"

    # Routes to push to clients
    routes = [
      "192.168.100.0/24",  # Office network
      "10.0.0.0/8",        # Private networks
      "203.0.113.0/24"     # Datacenter
    ]

    # DNS configuration
    dns = {
      enabled = true
      servers = ["192.168.1.1", "8.8.8.8"]
      search_domains = ["office.example.com"]
    }

    # Default route (optional)
    default_route = false

    # Exclusions
    exclude_routes = [
      "192.168.0.0/16"  # Local LAN
    ]
  }
}
```

### Multi-Zone VPN Isolation
```hcl
# Define VPN zones
zone "VPN-Admin" {
  interface = "wg0"

  # Admin VPN zone
  isolation = {
    type = "full"

    # Full network access
    allowed_zones = ["LAN", "WAN", "DMZ"]

    # No restrictions
    restrictions = []
  }
}

zone "VPN-Guest" {
  interface = "wg1"

  # Guest VPN zone
  isolation = {
    type = "restricted"

    # Internet only
    allowed_zones = ["WAN"]

    # Block internal networks
    blocked_zones = ["LAN", "DMZ", "SERVERS"]

    # Time restrictions
    time_restrictions = {
      start = "08:00"
      end = "22:00"
    }
  }
}

zone "VPN-IoT" {
  interface = "wg2"

  # IoT device VPN
  isolation = {
    type = "device_specific"

    # Only to IoT servers
    allowed_zones = ["IOT-SERVERS"]

    # Device-specific rules
    device_rules = [
      {
        device_id = "camera01"
        allowed_destinations = ["192.168.50.10:443"]
        allowed_protocols = ["tcp"]
      },
      {
        device_id = "sensor01"
        allowed_destinations = ["192.168.50.20:8080"]
        allowed_protocols = ["tcp", "udp"]
      }
    ]
  }
}
```

### Dynamic VPN Isolation
```hcl
wireguard "wg0" {
  enabled = true
  listen_port = 51820
  private_key = "${WG_PRIVATE_KEY}"

  # Dynamic isolation
  isolation = {
    enabled = true
    type = "dynamic"

    # Authentication-based isolation
    auth_based = {
      enabled = true

      # Authentication sources
      sources = ["ldap", "radius", "certificate"]

      # Group mappings
      group_mappings = [
        {
          groups = ["admins", "network_ops"]
          isolation_type = "full"
          allowed_zones = ["all"]
        },
        {
          groups = ["developers", "qa"]
          isolation_type = "split_tunnel"
          allowed_zones = ["LAN", "WAN"]
          routes = ["10.0.0.0/8", "192.168.100.0/24"]
        },
        {
          groups = ["contractors"]
          isolation_type = "restricted"
          allowed_zones = ["WAN"]
          time_restrictions = {
            start = "09:00"
            end = "17:00"
          }
        }
      ]
    }

    # Context-aware isolation
    context_aware = {
      enabled = true

      # Factors
      factors = [
        "time_of_day",
        "day_of_week",
        "client_location",
        "device_type",
        "security_posture"
      ]

      # Policies
      policies = [
        {
          name = "business_hours_only"
          condition = {
            time_range = "09:00-17:00"
            days = ["mon", "tue", "wed", "thu", "fri"]
          }
          isolation = {
            type = "full"
          }
        },
        {
          name = "after_hours_restricted"
          condition = {
            time_range = "17:00-09:00"
          }
          isolation = {
            type = "restricted"
            allowed_zones = ["WAN"]
          }
        },
        {
          name = "high_security_location"
          condition = {
            client_location = "high_risk_country"
          }
          isolation = {
            type = "full"
            require_mfa = true
            log_all = true
          }
        }
      ]
    }
  }
}
```

## Implementation Details

### Isolation Process
1. Client connects to VPN
2. Authentication and authorization
3. Isolation policy selection
4. Routing table configuration
5. Firewall rule application
6. DNS configuration
7. Ongoing monitoring

### Routing Table Setup
```bash
# Create VPN routing table
ip route add table 100 default dev wg0

# Add VPN routes
ip route add 192.168.100.0/24 dev wg0 table 100
ip route add 10.0.0.0/8 dev wg0 table 100

# Add rules for VPN traffic
ip rule add from 10.200.0.0/24 table 100
ip rule add fwmark 0x1 table 100
```

## Testing

### VPN Isolation Testing
```bash
# Test from VPN client
ping 192.168.1.1    # Should fail if blocked
ping 8.8.8.8        # Should work if allowed

# Test routing
ip route get 8.8.8.8 from 10.200.0.2

# Test DNS
nslookup google.com

# Check firewall rules
iptables -L -v -n | grep wg0
```

### Integration Tests
- `vpn_isolation_test.sh`: Basic isolation
- `split_tunnel_test.sh`: Split tunnel
- `multi_zone_test.sh`: Multi-zone isolation

## API Integration

### VPN Isolation API
```bash
# Get VPN isolation status
curl -s "http://localhost:8080/api/vpn/isolation/status"

# Get client isolation
curl -s "http://localhost:8080/api/vpn/clients/10.200.0.2/isolation"

# Update client isolation
curl -X PUT "http://localhost:8080/api/vpn/clients/10.200.0.2/isolation" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "restricted",
    "allowed_zones": ["WAN"]
  }'

# Get isolation policies
curl -s "http://localhost:8080/api/vpn/isolation/policies"
```

### Monitoring API
```bash
# Get VPN client statistics
curl -s "http://localhost:8080/api/vpn/clients/stats"

# Get isolation violations
curl -s "http://localhost:8080/api/vpn/isolation/violations"

# Get traffic statistics
curl -s "http://localhost:8080/api/vpn/traffic/stats"
```

## Best Practices

1. **Security**
   - Start with restrictive policies
   - Log all isolation violations
   - Regularly review policies
   - Test bypass attempts

2. **Performance**
   - Optimize routing tables
   - Monitor connection counts
   - Balance security and usability
   - Use hardware acceleration

3. **Usability**
   - Document access policies
   - Provide clear error messages
   - Offer multiple isolation levels
   - Allow temporary exceptions

4. **Compliance**
   - Meet regulatory requirements
   - Maintain audit trails
   - Implement data segregation
   - Regular compliance reviews

## Troubleshooting

### Common Issues
1. **Client can't access resources**: Check isolation rules
2. **Traffic leaks**: Verify routing and firewall
3. **DNS issues**: Check DNS configuration
4. **Performance problems**: Review routing efficiency

### Debug Commands
```bash
# Check VPN status
flywall vpn status

# Check client isolation
flywall vpn client isolation --ip 10.200.0.2

# Monitor VPN traffic
tcpdump -i wg0 -n

# Check routing
ip route show table 100
```

### Advanced Debugging
```bash
# Trace packet path
iptables -t raw -A PREROUTING -s 10.200.0.2 -j TRACE

# Check connection tracking
conntrack -L | grep 10.200.0.2

# Monitor isolation
flywall vpn isolation monitor

# Test isolation rules
flywall vpn test-isolation --client 10.200.0.2 --target 192.168.1.1
```

## Performance Considerations

- Isolation adds routing overhead
- Per-client rules scale poorly
- Hardware offload helps
- Monitor CPU usage

## Security Considerations

- Prevent VPN bypass
- Protect against DNS leaks
- Monitor for lateral movement
- Validate client certificates

## Related Features

- [WireGuard VPN](wireguard-vpn.md)
- [Zones & Policies](zones-policies.md)
- [Split Horizon DNS](dns-split-horizon.md)
- [Access Control](access-control.md)
