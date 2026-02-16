# DHCP Server Implementation Guide

## Overview

Flywall includes a full-featured DHCP server that provides:
- Dynamic IP address allocation
- Lease management and persistence
- Per-zone DHCP scopes
- Custom DHCP options
- DHCP failover support

## Architecture

### DHCP Components
1. **DHCP Server**: Core DHCP implementation
2. **Lease Manager**: Tracks IP allocations
3. **Option Parser**: Handles DHCP options
4. **Scope Manager**: Manages IP pools
5. **Persistence Layer**: Stores leases in database

## Configuration

### Basic DHCP Setup
```hcl
dhcp {
  enabled = true

  scope "lan" {
    interface = "eth1"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"
    router = "192.168.1.1"
    dns = ["192.168.1.1", "8.8.8.8"]
    lease_time = "24h"
  }
}
```

### Multiple DHCP Scopes
```hcl
dhcp {
  enabled = true

  # Green Zone
  scope "green" {
    interface = "eth1"
    range_start = "10.1.0.100"
    range_end = "10.1.0.200"
    router = "10.1.0.1"
    dns = ["10.1.0.1"]
    lease_time = "12h"
    domain_name = "green.local"
  }

  # Orange Zone (DMZ)
  scope "orange" {
    interface = "eth2"
    range_start = "10.2.0.100"
    range_end = "10.2.0.150"
    router = "10.2.0.1"
    dns = ["10.2.0.1"]
    lease_time = "6h"
    domain_name = "dmz.local"

    # Static reservations
    reservation "server1" {
      mac = "00:11:22:33:44:55"
      ip = "10.2.0.10"
    }
  }

  # Red Zone (Isolated)
  scope "red" {
    interface = "eth3"
    range_start = "10.3.0.100"
    range_end = "10.3.0.120"
    router = "10.3.0.1"
    dns = ["10.3.0.1"]
    lease_time = "1h"

    # Restrict to known MACs
    allow_unknown = false
  }
}
```

### Advanced DHCP Options
```hcl
dhcp {
  enabled = true

  scope "lan" {
    interface = "eth1"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"
    router = "192.168.1.1"
    dns = ["192.168.1.1", "8.8.8.8"]
    lease_time = "24h"

    # Custom DHCP options
    options {
      # NTP servers
      ntp_servers = ["192.168.1.10", "192.168.1.11"]

      # SIP servers
      sip_servers = ["192.168.1.20"]

      # WPAD URL
      wpad = "http://wpad.local/wpad.dat"

      # Custom option 150 (TFTP server)
      option_150 = "192.168.1.30"

      # Vendor-specific options
      vendor_class = "Flywall"
      vendor_options = {
        "option-1" = "value1"
        "option-2" = "value2"
      }
    }

    # Boot options for PXE
    boot {
      next_server = "192.168.1.40"
      filename = "pxelinux.0"
    }
  }
}
```

### DHCP Reservations
```hcl
dhcp {
  enabled = true

  scope "lan" {
    interface = "eth1"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"

    # Static reservations
    reservation "print-server" {
      mac = "00:11:22:33:44:55"
      ip = "192.168.1.10"
      hostname = "printer"
      lease_time = "infinite"
    }

    reservation "camera-01" {
      mac = "AA:BB:CC:DD:EE:FF"
      ip = "192.168.1.11"
      hostname = "cam01"
      options {
        ntp_servers = ["192.168.1.1"]
      }
    }

    # Reservation from MAC pool
    reservation_pool "iot-devices" {
      mac_pattern = "00:1A:2B:*"
      ip_range = "192.168.1.150-192.168.1.160"
    }
  }
}
```

### DHCP Failover
```hcl
dhcp {
  enabled = true

  # Primary DHCP server
  failover {
    role = "primary"
    peer_address = "192.168.1.2"
    port = 647
    max_lease_delay = "1h"
    load_balance = true
  }

  scope "lan" {
    interface = "eth1"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"

    # Split range for failover
    failover_pool {
      primary_range = "192.168.1.100-192.168.1.150"
      secondary_range = "192.168.1.151-192.168.1.200"
    }
  }
}
```

## Implementation Details

### Lease Lifecycle
1. **Discover**: Client broadcasts DHCPDISCOVER
2. **Offer**: Server offers IP address
3. **Request**: Client requests specific IP
4. **Ack**: Server acknowledges lease
5. **Renewal**: Client renews before expiry
6. **Release**: Client releases on shutdown

### Database Schema

For detailed database schema information, see [DHCP Lease Management](dhcp-lease-mgmt.md#database-schema).

### DHCP Options
- Standard options fully supported
- Custom options via numeric codes
- Vendor-specific options
- Option 82 (relay agent info)

## Testing

### Integration Tests
- `dhcp_lease_lifecycle_test.sh`: Lease allocation and renewal
- `dhcp_exhaustion_test.sh`: Pool exhaustion handling
- `dhcp_options_test.sh`: Custom DHCP options
- `dhcp_traffic_test.sh`: End-to-end DHCP traffic

### Manual Testing
```bash
# Request DHCP lease
dhclient -v eth1

# Check current leases
flywall dhcp leases list

# Check specific scope
flywall dhcp scope show lan

# Monitor DHCP traffic
tcpdump -i any port 67 -vv
```

## API Integration

### DHCP Management API
```bash
# Get all leases
curl -s "http://localhost:8080/api/dhcp/leases"

# Get active leases
curl -s "http://localhost:8080/api/dhcp/leases?active=true"

# Get specific lease
curl -s "http://localhost:8080/api/dhcp/leases/00:11:22:33:44:55"

# Add reservation
curl -X POST "http://localhost:8080/api/dhcp/reservations" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "server1",
    "mac": "00:11:22:33:44:55",
    "ip": "192.168.1.10",
    "scope": "lan"
  }'

# Delete lease
curl -X DELETE "http://localhost:8080/api/dhcp/leases/00:11:22:33:44:55"

# Get scope statistics
curl -s "http://localhost:8080/api/dhcp/scopes/lan/stats"
```

## Best Practices

1. **IP Planning**
   - Reserve ranges for static assignments
   - Consider future expansion
   - Document IP allocation scheme

2. **Lease Management**
   - Appropriate lease times per zone
   - Monitor lease utilization
   - Regular cleanup of expired leases

3. **Security**
   - Use MAC filtering where appropriate
   - Monitor for unknown devices
   - Consider DHCP snooping

4. **Performance**
   - Multiple scopes for large networks
   - Optimize lease times
   - Monitor server load

## Troubleshooting

### Common Issues
1. **No IP allocation**: Check scope availability
2. **Duplicate IPs**: Check for stale leases
3. **Slow responses**: Check server load

### Debug Commands
```bash
# Check DHCP server status
flywall show dhcp

# View DHCP logs
journalctl -u flywall | grep dhcp

# Monitor DHCP traffic
tcpdump -i any port 67 -n

# Check lease database
sqlite3 /opt/flywall/var/lib/state.db \
  "SELECT * FROM dhcp_leases"

# Test with specific client
dhclient -d -v eth1
```

### Advanced Debugging
```bash
# Check DHCP options
dhcpdump -i eth1

# Simulate DHCP client
nmap -sU -p 67 --script dhcp-discover

# Check for rogue DHCP servers
dhcping -s 255.255.255.255
```

## Performance Considerations

- DHCP server handles thousands of clients
- Lease database queries optimized
- Memory usage scales with lease count
- Consider SSD for high lease turnover

## Security Considerations

- DHCP spoofing protection
- Option 82 for switch port identification
- Rate limiting per MAC
- Monitor for DHCP starvation attacks

## Related Features

- [DNS Server](dns-server.md)
- [Zone Policies](zones-policies.md)
- [Network Learning](learning-engine.md)
- [HA Configuration](ha-configuration.md)
