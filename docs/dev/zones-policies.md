# Zones and Policies Implementation Guide

## Overview

Flywall uses a zone-based firewall model where network interfaces are grouped into zones, and traffic between zones is controlled by policies. This provides a clear, hierarchical way to manage network security.

## Architecture

### Zones
Zones are logical groupings of network interfaces that share the same security level and trust boundaries:

- **WAN Zone**: Untrusted external network (internet)
- **Green Zone**: Trusted internal network (full access)
- **Orange Zone**: DMZ network (limited access)
- **Red Zone**: Isolated network (highly restricted)

### Policies
Policies define traffic rules between zones:
- Source zone: Where traffic originates
- Destination zone: Where traffic is going
- Rules: Specific allow/deny actions with criteria

## Configuration

### Zone Definition
```hcl
# Define zones with their interfaces
zone "WAN" {
  interface = "eth0"
}

zone "Green" {
  interface = "eth1"
}

zone "Orange" {
  interface = "eth2"
}

zone "Red" {
  interface = "eth3"
}
```

### Interface Assignment
```hcl
# WAN Interface
interface "eth0" {
  description = "WAN Link"
  dhcp        = true
}

# Green Zone - Full internet access
interface "eth1" {
  description = "Green Zone (Trusted)"
  ipv4        = ["10.1.0.1/24"]
}

# Orange Zone - DMZ
interface "eth2" {
  description = "Orange Zone (DMZ)"
  ipv4        = ["10.2.0.1/24"]
}

# Red Zone - Isolated
interface "eth3" {
  description = "Red Zone (Isolated)"
  ipv4        = ["10.3.0.1/24"]
}
```

### Policy Rules
```hcl
# Green to Orange: Allow all traffic
policy "Green" "Orange" {
  name = "green_to_orange"

  rule "allow_all" {
    description = "Allow Green zone to access Orange zone services"
    action = "accept"
  }
}

# Red to Green: Block all traffic
policy "Red" "Green" {
  name = "red_isolation"

  rule "block_all" {
    description = "Block Red zone from accessing other zones"
    action = "drop"
  }
}

# Green to WAN: Internet access
policy "Green" "WAN" {
  name = "green_to_wan"

  rule "allow_internet" {
    description = "Allow Green zone full internet access"
    action = "accept"
  }
}
```

### Service-Based Rules
Use service helpers for common protocols:
```hcl
policy "Green" "self" {
  name = "green_to_firewall"

  rule "allow_dns" {
    description = "Allow DNS access to firewall"
    services = ["dns"]  # Expands to tcp/53 + udp/53
    action = "accept"
  }

  rule "allow_dhcp" {
    description = "Allow DHCP access"
    services = ["dhcp"]  # Expands to udp/67 + udp/68
    action = "accept"
  }

  rule "allow_ping" {
    description = "Allow ICMP ping"
    services = ["ping"]
    action = "accept"
  }
}
```

### Advanced Rule Matching
```hcl
policy "Orange" "Red" {
  name = "orange_to_red"

  rule "allow_web_only" {
    description = "Only allow HTTP/HTTPS to Red zone"
    proto     = "tcp"
    dest_port = [80, 443]
    action    = "accept"
  }

  rule "allow_specific_ips" {
    description = "Allow specific Orange IPs to Red zone"
    src_ip    = ["10.2.0.10", "10.2.0.20"]
    action    = "accept"
  }

  rule "log_and_drop" {
    description = "Log and drop everything else"
    action     = "drop"
    log        = true
    log_prefix = "ORANGE-RED-DROP: "
  }
}
```

## Implementation Details

### Rule Processing Order
1. Rules are evaluated in order within a policy
2. First matching rule determines the action
3. If no rule matches, default policy is drop
4. Self-referential policies use "self" as destination

### NAT Integration
```hcl
# NAT for internet access
nat "green_masquerade" {
  type         = "masquerade"
  out_interface = "eth0"
}

# Port forwarding
nat "web_forward" {
  type        = "dnat"
  proto       = "tcp"
  dest_port   = 80
  to_dest     = "10.2.0.10:8080"
}
```

### Special Zones
- **self**: Represents the firewall itself
- **any**: Wildcard for any zone (use carefully)

## Testing

### Integration Test Reference
- Test file: `integration_tests/linux/30-firewall/zones_test.sh`
- Creates network namespaces to simulate zones
- Tests zone isolation and connectivity
- Verifies policy enforcement

### Manual Testing
```bash
# Check zone assignments
flywall show zones

# List active policies
flywall show policies

# View generated nftables rules
nft list table inet flywall

# Test connectivity between zones
ip netns exec green ping 10.2.0.1  # Green to Orange
ip netns exec red ping 10.1.0.1    # Red to Green (should fail)
```

## Best Practices

1. **Zone Design**
   - Keep zones simple and focused
   - Use descriptive names
   - Document zone purposes

2. **Policy Structure**
   - One policy per zone pair
   - Use descriptive policy names
   - Document complex rules

3. **Rule Organization**
   - Put specific rules first
   - Use service helpers when possible
   - Add logging for dropped traffic

4. **Security Considerations**
   - Default to deny
   - Minimize Red zone access
   - Log denied traffic for monitoring

## Troubleshooting

### Common Issues
1. **Traffic not flowing**: Check if policy exists for the zone pair
2. **Allowed traffic blocked**: Verify rule order and specificity
3. **NAT not working**: Ensure NAT rule matches traffic

### Debug Commands
```bash
# Check zone-to-interface mapping
flywall show interfaces

# View policy hit counts
flywall show stats | grep policy

# Trace packet flow
nft trace table inet flywall ip saddr 10.1.0.100 ip daddr 10.2.0.100

# Check conntrack entries
conntrack -L | grep "10.1.0"
```

## Performance Considerations

- Zone-based rules are compiled to nftables sets for O(1) lookup
- Policy evaluation happens once per connection
- Use ipsets for large IP lists instead of multiple rules
- Consider rule count impact on CPU usage

## Related Features

- [Interface Management](interface-management.md)
- [NAT & Routing](nat-routing.md)
- [Protection Features](protection-features.md)
- [Learning Engine](learning-engine.md)
