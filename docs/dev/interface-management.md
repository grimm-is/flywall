# Interface Management Implementation Guide

## Overview

Flywall provides comprehensive interface management for:
- Physical network interfaces
- Bond interfaces (link aggregation)
- VLAN interfaces
- Bridge interfaces
- Virtual interfaces
- Interface dependencies and ordering

## Architecture

### Interface Types
1. **Physical**: Real hardware NICs (eth0, enp0s3, etc.)
2. **Bond**: Aggregated multiple interfaces
3. **VLAN**: Tagged virtual LANs
4. **Bridge**: Layer 2 switching
5. **Dummy**: Virtual interfaces for testing

### Interface Lifecycle
1. **Discovery**: Detect available interfaces
2. **Configuration**: Apply HCL configuration
3. **Dependency Resolution**: Order creation based on dependencies
4. **Creation**: Create interfaces in correct order
5. **Monitoring**: Track interface status
6. **Cleanup**: Remove on config reload/shutdown

## Configuration

### Basic Interface Setup
```hcl
# Physical interface
interface "eth0" {
  description = "WAN Interface"
  ipv4 = ["203.0.113.10/24", "203.0.113.11/24"]
  ipv6 = ["2001:db8::1/64"]
  gateway = "203.0.113.1"
  zone = "wan"
  dhcp = false
  mtu = 1500
}

# Loopback interface
interface "lo" {
  ipv4 = ["127.0.0.1/8"]
  zone = "local"
}
```

### DHCP Client
```hcl
interface "eth1" {
  description = "LAN Interface"
  zone = "lan"
  dhcp = true

  # DHCP options
  dhcp_options {
    hostname = "firewall"
    vendor_class = "Flywall"
    request_options = [1, 3, 6, 15, 119]
  }
}
```

### Bond Interfaces
```hcl
# LACP Bond
interface "bond0" {
  description = "Aggregate Uplinks"
  zone = "wan"
  ipv4 = ["10.0.0.1/24"]

  bond {
    mode = "802.3ad"  # LACP
    interfaces = ["eth0", "eth1"]
    miimon = 100
    lacp_rate = "fast"
    xmit_hash_policy = "layer3+4"

    # Bond monitoring
    arp_interval = 1000
    arp_ip_target = ["10.0.0.254"]
  }
}

# Active-Backup Bond
interface "bond1" {
  description = "Redundant Links"
  zone = "dmz"
  ipv4 = ["10.1.0.1/24"]

  bond {
    mode = "active-backup"
    interfaces = ["eth2", "eth3"]
    miimon = 100
    primary = "eth2"

    # Failover settings
    updelay = 1000
    downdelay = 1000
  }
}
```

### VLAN Interfaces
```hcl
# VLAN on physical interface
interface "eth0.100" {
  description = "Management VLAN"
  zone = "mgmt"
  ipv4 = ["192.168.100.1/24"]

  vlan {
    id = 100
    parent = "eth0"
    protocol = "802.1q"
  }
}

# VLAN on bond
interface "bond0.200" {
  description = "Guest Network VLAN"
  zone = "guest"
  ipv4 = ["10.200.0.1/24"]

  vlan {
    id = 200
    parent = "bond0"
  }
}

# QinQ (Stacked VLANs)
interface "eth0.100.200" {
  description = "Customer VLAN"
  zone = "customer"
  ipv4 = ["10.100.200.1/24"]

  vlan {
    id = 200
    parent = "eth0.100"
    protocol = "802.1ad"
  }
}
```

### Bridge Interfaces
```hcl
# Simple bridge
interface "br0" {
  description = "LAN Bridge"
  zone = "lan"
  ipv4 = ["192.168.1.1/24"]

  bridge {
    ports = ["eth1", "eth2"]
    stp = true
    forward_delay = 15
    max_age = 20
    hello_time = 2
  }
}

# Bridge with VLAN filtering
interface "br0" {
  description = "VLAN-aware Bridge"
  zone = "lan"
  ipv4 = ["192.168.1.1/24"]

  bridge {
    ports = ["eth1", "eth2"]
    vlan_filtering = true

    # VLAN port configuration
    vlan "10" {
      ports = ["eth1", "eth2"]
      pvid = 10
    }

    vlan "20" {
      ports = ["eth1"]
      tagged = true
    }
  }
}
```

### Complex Interface Stacking
```hcl
# Physical interfaces
interface "eth0" {
  zone = "wan"
  dhcp = true
}

interface "eth1" {
  zone = "wan"
  dhcp = true
}

# Bond on physical
interface "bond0" {
  description = "Uplink Aggregate"
  zone = "wan"
  ipv4 = ["10.0.0.1/24"]

  bond {
    mode = "lacp"
    interfaces = ["eth0", "eth1"]
  }
}

# Bridge on bond
interface "br0" {
  description = "LAN Bridge"
  zone = "lan"
  ipv4 = ["192.168.1.1/24"]

  bridge {
    ports = ["bond0.100"]
  }

  # VLAN on bridge
  vlan "100" {
    id = 100
    parent = "br0"
  }
}

# VLAN on bond
interface "bond0.100" {
  description = "Trunk VLAN"
  zone = "lan"

  vlan {
    id = 100
    parent = "bond0"
  }
}
```

### Interface Options
```hcl
interface "eth0" {
  description = "Advanced Configuration"
  zone = "wan"
  ipv4 = ["203.0.113.10/24"]
  gateway = "203.0.113.1"

  # MTU and offloading
  mtu = 9000
  tx_queue_length = 1000

  # Offload settings
  offload {
    tx_checksum = true
    rx_checksum = true
    scatter_gather = true
    tcp_segmentation = true
    udp_fragmentation = true
  }

  # Traffic control
  traffic_control {
    qdisc = "fq_codel"
    bandwidth = "1gbit"
  }

  # Interface metrics
  metrics = {
    priority = 100
    weight = 1
  }

  # ARP settings
  arp {
    announce = 2
    ignore = false
    validate = true
  }

  # Promiscuous mode
  promiscuous = false

  # All-multicast mode
  allmulticast = false
}
```

## Implementation Details

### Interface Discovery
```bash
# List all interfaces
ip link show

# Show interface details
ip addr show eth0

# Check interface capabilities
ethtool eth0
```

### Dependency Resolution
- Interfaces created in dependency order
- VLANs wait for parent interfaces
- Bonds wait for member interfaces
- Bridges wait for port interfaces

### Interface Monitoring
- Link status monitoring
- Carrier detection
- Speed and duplex detection
- Error counter tracking

## Testing

### Integration Tests
- `interface_deps_test.sh`: Complex interface stacking
- `vlan_test.sh`: VLAN configuration
- `bond_test.sh`: Bond aggregation
- `mtu_test.sh`: MTU handling

### Manual Testing
```bash
# Check interface status
flywall show interfaces

# Test interface connectivity
ip link set eth0 up
ping -I eth0 8.8.8.8

# Monitor interface stats
cat /proc/net/dev
```

## API Integration

### Interface Management API
```bash
# List all interfaces
curl -s "http://localhost:8080/api/interfaces"

# Get specific interface
curl -s "http://localhost:8080/api/interfaces/eth0"

# Update interface
curl -X PUT "http://localhost:8080/api/interfaces/eth0" \
  -H "Content-Type: application/json" \
  -d '{
    "ipv4": ["192.168.1.1/24"],
    "zone": "lan"
  }'

# Get interface statistics
curl -s "http://localhost:8080/api/interfaces/eth0/stats"
```

### Interface Events
```bash
# Get interface events
curl -s "http://localhost:8080/api/interfaces/events"

# Monitor link status
curl -s "http://localhost:8080/api/interfaces/eth0/link"
```

## Best Practices

1. **Interface Naming**
   - Use consistent naming conventions
   - Document interface purposes
   - Avoid special characters

2. **MTU Configuration**
   - Match MTU across path
   - Consider VLAN overhead
   - Use jumbo frames with care

3. **Bond Configuration**
   - Use LACP for switches that support it
   - Monitor bond status
   - Test failover scenarios

4. **VLAN Design**
   - Plan VLAN ID allocation
   - Document VLAN purposes
   - Use descriptive names

## Troubleshooting

### Common Issues
1. **Interface not coming up**: Check dependencies
2. **VLAN not working**: Verify parent interface
3. **Bond not aggregating**: Check switch configuration

### Debug Commands
```bash
# Check interface details
ip -d link show eth0

# Monitor interface events
ip monitor link

# Check bridge status
brctl show br0

# Check bond status
cat /proc/net/bonding/bond0

# Check VLAN info
ip -d link show eth0.100
```

### Advanced Debugging
```bash
# Check interface driver
ethtool -i eth0

# Monitor interface errors
watch -n 1 'cat /proc/net/dev | grep eth0'

# Check ARP table
ip neigh show

# Test with specific MTU
ping -M do -s 1472 -c 3 8.8.8.8
```

## Performance Considerations

- Interface offload reduces CPU usage
- Jumbo frames improve throughput
- Bond interfaces distribute load
- VLAN processing adds minimal overhead

## Security Considerations

- Disable unused interfaces
- Use promiscuous mode carefully
- Monitor for MAC flooding
- Secure management interfaces

## Related Features

- [Zones & Policies](zones-policies.md)
- [NAT & Routing](nat-routing.md)
- [VLAN Configuration](vlan-config.md)
- [Bond Configuration](bond-config.md)
