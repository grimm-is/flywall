# NAT & Routing Implementation Guide

## Overview

Flywall provides comprehensive NAT and routing capabilities:
- Source NAT (Masquerade/SNAT)
- Destination NAT (DNAT/Port Forwarding)
- Static routing
- Policy-based routing
- Route redistribution
- Multipath routing

## Architecture

### NAT Types
1. **Masquerade**: Dynamic source NAT for dynamic IPs
2. **SNAT**: Static source NAT for fixed IPs
3. **DNAT**: Destination NAT for port forwarding
4. **Redirect**: Local port redirection

### Routing Components
1. **Routing Table**: Kernel routing tables
2. **Policy Rules**: Routing policy database
3. **Route Metrics**: Route priority and cost
4. **Gateway Detection**: Active gateway monitoring

## Configuration

### Basic NAT Setup
```hcl
# Masquerade for dynamic IP
nat "wan_masq" {
  type = "masquerade"
  out_interface = "eth0"
}

# Static SNAT
nat "static_nat" {
  type = "snat"
  out_interface = "eth0"
  source_address = "203.0.113.10"
}

# Port forwarding (DNAT)
nat "web_forward" {
  type = "dnat"
  protocol = "tcp"
  dest_port = 80
  to_dest = "192.168.1.10:8080"
}

# Local redirect
nat "capture_dns" {
  type = "redirect"
  protocol = "tcp"
  dest_port = 53
  to_ports = [5353]
}
```

### Advanced NAT Configuration
```hcl
# 1:1 NAT
nat "server_nat" {
  type = "dnat"
  dest_address = "203.0.113.100"
  to_dest = "192.168.1.100"
}

# Port range forwarding
nat "game_forward" {
  type = "dnat"
  protocol = "udp"
  dest_port = [27015, 27016, 27017, 27018, 27019]
  to_dest = "192.168.1.50"
}

# Conditional NAT
nat "office_nat" {
  type = "masquerade"
  out_interface = "eth0"
  source_address = ["192.168.1.0/24", "192.168.2.0/24"]
}

# NAT with logging
nat "log_nat" {
  type = "dnat"
  protocol = "tcp"
  dest_port = 22
  to_dest = "192.168.1.10:22"
  log = true
  log_prefix = "SSH-FORWARD: "
}
```

### Static Routing
```hcl
# Default route
route "default" {
  destination = "0.0.0.0/0"
  gateway = "203.0.113.1"
  interface = "eth0"
  metric = 100
}

# Static routes
route "lan_network" {
  destination = "10.10.0.0/16"
  gateway = "192.168.1.254"
  interface = "eth1"
}

route "vpn_network" {
  destination = "192.168.200.0/24"
  gateway = "10.200.0.2"
  interface = "wg0"
  metric = 50
}

# Blackhole route
route "blackhole" {
  destination = "192.168.100.0/24"
  type = "blackhole"
}
```

### Policy-Based Routing
```hcl
# Routing policy for traffic shaping
policy_route "voip_traffic" {
  description = "Route VoIP traffic via dedicated link"

  # Match criteria
  protocol = "udp"
  dest_port = [5060, 5061, 10000:20000]

  # Routing decision
  table = 100
  priority = 100
}

# Policy for specific source
policy_route "guest_network" {
  description = "Route guest traffic via filtered gateway"

  source_address = "192.168.200.0/24"

  table = 200
  priority = 200
}

# Policy with mark
policy_route "marked_traffic" {
  description = "Route marked traffic"

  fwmark = 0x1000

  table = 300
  priority = 300
}
```

### Multipath Routing
```hcl
# Multiple default routes
route "default1" {
  destination = "0.0.0.0/0"
  gateway = "203.0.113.1"
  interface = "eth0"
  metric = 100
  weight = 1
}

route "default2" {
  destination = "0.0.0.0/0"
  gateway = "203.0.114.1"
  interface = "eth1"
  metric = 100
  weight = 1
}

# ECMP (Equal Cost Multi-Path)
route "ecmp_default" {
  destination = "0.0.0.0/0"
  nexthop = [
    {
      gateway = "203.0.113.1"
      interface = "eth0"
      weight = 50
    },
    {
      gateway = "203.0.114.1"
      interface = "eth1"
      weight = 50
    }
  ]
}
```

### Routing Tables
```hcl
# Custom routing tables
routing_table "wan_backup" {
  id = 100
  description = "Backup WAN routes"

  route "backup_default" {
    destination = "0.0.0.0/0"
    gateway = "203.0.115.1"
    interface = "eth2"
  }
}

routing_table "vpn_tunnel" {
  id = 200
  description = "VPN specific routes"

  route "vpn_lan" {
    destination = "10.0.0.0/8"
    gateway = "10.200.0.1"
    interface = "wg0"
  }
}
```

### Gateway Monitoring
```hcl
# Active gateway monitoring
gateway_monitor "primary_wan" {
  interface = "eth0"
  gateway = "203.0.113.1"

  # Monitoring settings
  check_interval = "5s"
  failure_threshold = 3
  recovery_threshold = 2

  # Check methods
  ping {
    target = "8.8.8.8"
    count = 3
    timeout = "1s"
  }

  arp {
    target = "203.0.113.1"
    interface = "eth0"
  }

  # Track script
  track_script = "/etc/flywall/check-wan.sh"

  # On failure
  on_failure {
    withdraw_route = true
    log = true
    run_script = "/etc/flywall/wan-failed.sh"
  }
}
```

## Implementation Details

### NAT Processing Order
1. DNAT (pre-routing)
2. Routing decision
3. SNAT/Masquerade (post-routing)

### Route Selection
1. Longest prefix match
2. Metric comparison
3. Policy rules
4. Default route

### Connection Tracking
- NAT entries tracked in conntrack
- Timeouts per protocol
- Helper modules for protocols

## Testing

### Integration Tests
- `nat_traffic_test.sh`: NAT translation verification
- `routing_test.sh`: Static routing
- `policy_test.sh`: Policy-based routing
- `multi_wan_test.sh`: Multiple WAN links

### Manual Testing
```bash
# Check NAT rules
nft list table ip nat

# Check routing table
ip route show

# Check policy routing
ip rule list

# Test NAT
curl ifconfig.me

# Check conntrack
conntrack -L
```

## API Integration

### NAT Management API
```bash
# List NAT rules
curl -s "http://localhost:8080/api/nat"

# Get NAT rule
curl -s "http://localhost:8080/api/nat/masq"

# Create NAT rule
curl -X POST "http://localhost:8080/api/nat" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "new_forward",
    "type": "dnat",
    "protocol": "tcp",
    "dest_port": 443,
    "to_dest": "192.168.1.10:8443"
  }'

# Get NAT statistics
curl -s "http://localhost:8080/api/nat/stats"
```

### Routing API
```bash
# List routes
curl -s "http://localhost:8080/api/routes"

# Get routing tables
curl -s "http://localhost:8080/api/routes/tables"

# Add route
curl -X POST "http://localhost:8080/api/routes" \
  -H "Content-Type: application/json" \
  -d '{
    "destination": "10.10.0.0/24",
    "gateway": "192.168.1.254",
    "interface": "eth1"
  }'

# Get gateway status
curl -s "http://localhost:8080/api/routes/gateways"
```

## Best Practices

1. **NAT Design**
   - Minimize NAT rules
   - Use 1:1 NAT when possible
   - Document NAT mappings
   - Consider security implications

2. **Routing Design**
   - Use appropriate metrics
   - Plan for redundancy
   - Monitor route flaps
   - Test failover paths

3. **Performance**
   - Optimize route tables
   - Use hardware offload
   - Monitor CPU usage
   - Consider route aggregation

4. **Security**
   - Restrict port forwarding
   - Log NAT translations
   - Validate routes
   - Monitor for anomalies

## Troubleshooting

### Common Issues
1. **NAT not working**: Check interface and direction
2. **Port forwarding fails**: Verify firewall rules
3. **Route not used**: Check metric and priority
4. **Asymmetric routing**: Check return path

### Debug Commands
```bash
# Check NAT translations
conntrack -L | grep NAT

# Trace packet flow
nft trace table ip nat ip saddr 192.168.1.100 ip daddr 8.8.8.8

# Check route cache
ip route get 8.8.8.8

# Monitor routing
ip monitor route

# Check ARP
ip neigh show
```

### Advanced Debugging
```bash
# Check routing policy
ip rule show
ip route show table 100

# Check multipath
ip route show default

# Monitor conntrack
conntrack -E

# Check NAT statistics
nft list counter inet flywall nat_counter
```

## Performance Considerations

- NAT adds minimal overhead
- Route lookup is O(log n)
- Hardware offload available
- Consider route cache size

## Security Considerations

- NAT provides some security
- Port forwarding creates exposure
- Route hijacking prevention
- Filter invalid routes

## Related Features

- [Zones & Policies](zones-policies.md)
- [Interface Management](interface-management.md)
- [Conntrack](conntrack.md)
- [Multipath Routing](multipath-routing.md)
