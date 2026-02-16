# Conntrack Implementation Guide

## Overview

Flywall uses Linux connection tracking (conntrack) for stateful firewalling:
- Connection state tracking
- NAT helper modules
- Connection timeouts
- Statistics collection
- Connection synchronization

## Architecture

### Conntrack Components
1. **Connection Tracker**: Core conntrack subsystem
2. **Helper Modules**: Protocol-specific helpers
3. **Timeout Manager**: Manages connection lifetimes
4. **Statistics Collector**: Tracks connection metrics
5. **Sync Manager**: Synchronizes connections across nodes

### Connection States
- **NEW**: First packet of new connection
- **ESTABLISHED**: Part of existing connection
- **RELATED**: Related to existing connection
- **INVALID**: Invalid or unrecognized packet
- **UNREPLIED**: Connection awaiting reply

## Configuration

### Basic Conntrack Setup
```hcl
# Connection tracking settings
conntrack {
  enabled = true

  # Maximum connections
  max_connections = 1000000

  # Hash table size
  hash_size = 65536

  # Connection timeouts
  timeouts = {
    tcp = "24h"
    udp = "30s"
    icmp = "30s"
    generic = "60s"
  }
}
```

### Advanced Conntrack Configuration
```hcl
conntrack {
  enabled = true

  # Performance tuning
  max_connections = 2000000
  hash_size = 131072
  buckets = 65536

  # Protocol-specific timeouts
  timeouts = {
    # TCP timeouts
    tcp_established = "5d"
    tcp_close_wait = "60s"
    tcp_fin_wait = "60s"
    tcp_syn_sent = "60s"
    tcp_syn_recv = "60s"
    tcp_time_wait = "60s"
    tcp_last_ack = "30s"
    tcp_close = "10s"
    tcp_syn_sent2 = "60s"

    # UDP timeouts
    udp_stream = "180s"
    udp = "30s"
    icmp = "30s"

    # General timeouts
    generic = "600s"
    unreplied = "30s"

    # SCTP timeouts
    sctp_closed = "10s"
    sctp_cookie_waited = "3s"
    sctp_cookie_echoed = "3s"
    sctp_established = "210s"
    sctp_shutdown_sent = "30s"
    sctp_shutdown_recd = "30s"
    sctp_shutdown_ack_sent = "3s"
  }

  # Helper modules
  helpers = {
    ftp = true
    tftp = true
    amanda = true
    h323 = true
    sip = true
    pptp = true
    snmp = true
  }

  # Logging
  log_invalid = true
  log_max = 100

  # Event filtering
  events = {
    new = true
    update = false
    destroy = true
  }
}
```

### Zone-Based Conntrack
```hcl
zone "WAN" {
  interface = "eth0"

  conntrack {
    enabled = true

    # Zone-specific timeouts
    timeouts = {
      tcp = "2h"
      udp = "10s"
    }

    # Track specific protocols
    track_protocols = ["tcp", "udp", "icmp"]

    # Connection limits
    limits = {
      max_per_ip = 1000
      max_concurrent = 100000
      new_per_second = 1000
    }
  }
}

zone "LAN" {
  interface = "eth1"

  conntrack {
    enabled = true

    # More permissive for internal traffic
    timeouts = {
      tcp = "24h"
      udp = "60s"
    }

    # No limits for trusted zone
    limits = {
      max_per_ip = 0
      max_concurrent = 0
    }
  }
}
```

### NAT Helpers
```hcl
conntrack {
  enabled = true

  # NAT helper configuration
  nat_helpers = {
    # FTP helper
    ftp = {
      enabled = true
      ports = [21]
      loose = false
    }

    # SIP helper
    sip = {
      enabled = true
      ports = [5060, 5061]
      media_ports = [10000:20000]
    }

    # H.323 helper
    h323 = {
      enabled = true
      ports = [1720]
    }

    # PPTP helper
    pptp = {
      enabled = true
      ports = [1723]
    }
  }
}
```

## Implementation Details

### Connection Tracking Flow
1. Packet enters system
2. Conntrack lookup
3. State determination
4. Helper processing (if applicable)
5. Policy application
6. Connection update

### Connection Table Structure
```c
struct nf_conn {
    // Tuple identification
    struct nf_conntrack_tuple tuple[IP_CT_DIR_MAX];

    // Connection state
    enum ip_conntrack_info ctinfo;
    unsigned long status;

    // Timeouts
    unsigned long timeout;

    // Protocol-specific data
    union nf_conntrack_proto proto;

    // Helper data
    struct nf_conntrack_helper *helper;
    void *help;

    // Extensions
    struct nf_ct_ext *ext;
};
```

### Helper Modules
- **FTP**: Track FTP data connections
- **SIP**: Track SIP media streams
- **H.323**: Track H.323 calls
- **PPTP**: Track PPTP GRE tunnels
- **TFTP**: Track TFTP transfers

## Testing

### Conntrack Testing
```bash
# Check conntrack table
conntrack -L

# Check conntrack statistics
conntrack -S

# Monitor conntrack events
conntrack -E

# Check specific connection
conntrack -L -s 192.168.1.100 -d 8.8.8.8

# Delete connection
conntrack -D -s 192.168.1.100 -d 8.8.8.8
```

### Integration Tests
- `conntrack_test.sh`: Basic conntrack functionality
- `nat_helper_test.sh`: NAT helper modules
- `timeout_test.sh`: Connection timeout handling

## API Integration

### Conntrack API
```bash
# Get conntrack statistics
curl -s "http://localhost:8080/api/conntrack/stats"

# List active connections
curl -s "http://localhost:8080/api/conntrack/connections"

# Get specific connection
curl -s "http://localhost:8080/api/conntrack/connections/123456"

# Get connections by source
curl -s "http://localhost:8080/api/conntrack/connections?src=192.168.1.100"

# Delete connection
curl -X DELETE "http://localhost:8080/api/conntrack/connections/123456"
```

### Statistics API
```bash
# Get connection statistics
curl -s "http://localhost:8080/api/conntrack/stats"

# Get protocol breakdown
curl -s "http://localhost:8080/api/conntrack/stats/protocols"

# Get zone statistics
curl -s "http://localhost:8080/api/conntrack/stats/zones"

# Get top talkers
curl -s "http://localhost:8080/api/conntrack/stats/top-talkers"
```

## Best Practices

1. **Performance Tuning**
   - Adjust hash size based on connections
   - Set appropriate timeouts
   - Monitor memory usage
   - Use hardware offload when available

2. **Security**
   - Limit connection tracking per IP
   - Track INVALID packets
   - Use connection limits
   - Monitor for DoS

3. **Reliability**
   - Configure appropriate timeouts
   - Monitor table usage
   - Plan for table overflow
   - Use connection synchronization

4. **Troubleshooting**
   - Monitor conntrack events
   - Check table statistics
   - Verify helper modules
   - Track resource usage

## Troubleshooting

### Common Issues
1. **Table full**: Increase max_connections or hash_size
2. **High memory usage**: Adjust timeouts or limits
3. **Connections not tracked**: Check module loading
4. **NAT not working**: Verify helper modules

### Debug Commands
```bash
# Check conntrack status
cat /proc/net/nf_conntrack

# Check conntrack settings
cat /proc/sys/net/netfilter/nf_conntrack_*

# Monitor conntrack
watch -n 1 'conntrack -L | wc -l'

# Check memory usage
cat /proc/slabinfo | grep nf_conntrack
```

### Advanced Debugging
```bash
# Trace conntrack events
strace -e trace=conntrack -p $(pidof flywall)

# Debug specific connection
conntrack -L -p tcp --src-nat

# Check helper status
cat /proc/net/nf_conntrack_expect

# Monitor table size
watch -n 1 'cat /proc/sys/net/netfilter/nf_conntrack_count'
```

## Performance Considerations

- Memory usage scales with connections
- Hash size affects lookup performance
- Helper modules add overhead
- Hardware offload available for some NICs

## Security Considerations

- Conntrack bypass possible with raw sockets
- SYN floods can fill table
- Connection tracking metadata exposure
- Helper module vulnerabilities

## Related Features

- [Zones & Policies](zones-policies.md)
- [NAT & Routing](nat-routing.md)
- [Protection Features](protection-features.md)
- [HA Configuration](ha-configuration.md)
