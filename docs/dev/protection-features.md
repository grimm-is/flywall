# Protection Features Implementation Guide

## Overview

Flywall includes comprehensive protection features:
- Anti-spoofing protection
- Bogon filtering
- Invalid packet filtering
- SYN flood protection
- Rate limiting
- Port scan detection
- Fragmentation protection

## Architecture

### Protection Layers
1. **Invalid Packet Filter**: Drops malformed packets
2. **Anti-Spoofing**: Prevents IP spoofing
3. **Bogon Filter**: Blocks invalid addresses
4. **Rate Limiter**: Controls packet rates
5. **Connection Tracking**: Stateful inspection

## Configuration

### Basic Protection Setup
```hcl
# Global protection (legacy format)
global_protection {
  anti_spoofing = true
  bogon_filtering = true
  invalid_packets = true
  syn_flood_protection = true
  syn_flood_rate = 25
  syn_flood_burst = 50
  icmp_rate_limit = true
  icmp_rate = 10
}

# Zone-based protection (preferred)
zone "WAN" {
  interface = "eth0"

  protection {
    anti_spoofing = true
    bogon_filtering = true
    invalid_packets = true
  }
}

zone "LAN" {
  interface = "eth1"

  protection {
    anti_spoofing = false  # Allow private IPs internally
    invalid_packets = true
  }
}
```

### Advanced Protection Configuration
```hcl
zone "WAN" {
  interface = "eth0"

  protection {
    # Anti-spoofing
    anti_spoofing = true
    spoofed_networks = [
      "10.0.0.0/8",
      "172.16.0.0/12",
      "192.168.0.0/16",
      "169.254.0.0/16",
      "127.0.0.0/8",
      "0.0.0.0/8",
      "224.0.0.0/4"
    ]

    # Bogon filtering
    bogon_filtering = true
    custom_bogons = [
      "203.0.113.0/24",
      "198.51.100.0/24"
    ]

    # Invalid packets
    invalid_packets = true
    drop_tcp_flags = ["FIN,SYN,RST,PSH,ACK,URG"]
    allow_tcp_flags = ["SYN", "ACK", "FIN", "RST"]

    # SYN flood protection
    syn_flood_protection = true
    syn_flood_rate = 100
    syn_flood_burst = 200
    syn_flood_limit_per_ip = true
    syn_flood_ip_rate = 10

    # ICMP protection
    icmp_rate_limit = true
    icmp_rate = 20
    icmp_burst = 10
    icmp_types_allowed = ["echo-request", "echo-reply", "destination-unreachable"]

    # Fragmentation
    drop_fragments = true
    reassemble_fragments = false

    # UDP protection
    udp_flood_protection = true
    udp_flood_rate = 1000
    udp_flood_burst = 2000
  }
}
```

### Rate Limiting
```hcl
zone "WAN" {
  interface = "eth0"

  protection {
    # General rate limiting
    rate_limit = true
    rate_limit_burst = 100

    # Per-protocol limits
    rate_limits {
      tcp = {
        rate = 1000
        burst = 200
      }

      udp = {
        rate = 500
        burst = 100
      }

      icmp = {
        rate = 10
        burst = 5
      }
    }

    # Per-service limits
    service_limits {
      ssh = {
        rate = 5
        burst = 10
        log = true
      }

      http = {
        rate = 100
        burst = 200
      }

      dns = {
        rate = 50
        burst = 100
      }
    }
  }
}
```

### Port Scan Detection
```hcl
zone "WAN" {
  interface = "eth0"

  protection {
    # Port scan detection
    port_scan_detection = true
    port_scan_threshold = 10
    port_scan_window = "60s"

    # Actions on detection
    port_scan_action = "drop"
    port_scan_log = true
    port_scan_duration = "1h"

    # Add to blocklist
    port_scan_blocklist = "scanners"

    # Custom scan patterns
    scan_patterns {
      tcp_connect = true
      tcp_syn = true
      udp_scan = true
      xmas_scan = true
      null_scan = true
    }
  }
}
```

### Connection Protection
```hcl
zone "WAN" {
  interface = "eth0"

  protection {
    # Connection tracking
    connection_limits = {
      max_connections = 10000
      max_per_ip = 100
      max_unreplied = 100
      tcp_timeout = "5m"
      udp_timeout = "30s"
      icmp_timeout = "10s"
    }

    # TCP protection
    tcp_protection = {
      drop_invalid = true
      drop_syn_with_ack = true
      drop_fin_with_ack = false
      strict_rfc = true
      window_check = true
    }

    # UDP protection
    udp_protection = {
      drop_invalid = true
      drop_broadcast = true
      drop_multicast = false
    }
  }
}
```

### Advanced Threat Protection
```hcl
zone "WAN" {
  interface = "eth0"

  protection {
    # DDoS protection
    ddos_protection = true
    ddos_threshold = "100Mbit"
    ddos_burst = "200Mbit"

    # Amplification protection
    amplification_protection = true
    amplification_protocols = ["DNS", "NTP", "SSDP", "CHARGEN"]

    # DNS amplification specific
    dns_amplification = {
      max_query_size = 512
      allowed_types = ["A", "AAAA", "MX", "TXT"]
      drop_any = true
    }

    # NTP amplification
    ntp_amplification = {
      allowed_requests = ["MON_GETLIST_1", "MON_GETLIST"]
      drop_others = true
    }

    # Botnet protection
    botnet_protection = true
    botnet_signatures = "/etc/flywall/botnet.sig"
    botnet_blocklist = "botnet_ips"
  }
}
```

## Implementation Details

### Protection Order
1. Invalid packet filtering
2. Anti-spoofing checks
3. Bogon filtering
4. Rate limiting
5. Connection tracking
6. Application layer checks

### Performance Impact
- Minimal overhead for basic protection
- SYN tracking uses memory
- Rate limiting requires counters
- Connection limits scale with traffic

## Testing

### Integration Tests
- `protection_test.sh`: Basic protection features
- `port_scan_test.sh`: Port scan detection
- `syn_flood_test.sh`: SYN flood protection
- `rate_limit_test.sh`: Rate limiting

### Manual Testing
```bash
# Test anti-spoofing
ping -I eth0 -s 10.0.0.1 8.8.8.8

# Test SYN flood
hping3 -S -p 80 --flood 192.168.1.1

# Test port scan
nmap -sS 192.168.1.1

# Check protection stats
nft list counters inet flywall
```

## API Integration

### Protection API
```bash
# Get protection status
curl -s "http://localhost:8080/api/protection/status"

# Get zone protection
curl -s "http://localhost:8080/api/zones/WAN/protection"

# Update protection settings
curl -X PUT "http://localhost:8080/api/zones/WAN/protection" \
  -H "Content-Type: application/json" \
  -d '{
    "anti_spoofing": true,
    "syn_flood_rate": 50
  }'

# Get protection statistics
curl -s "http://localhost:8080/api/protection/stats"
```

### Threat Intelligence API
```bash
# Get detected scanners
curl -s "http://localhost:8080/api/protection/scanners"

# Get rate limit violations
curl -s "http://localhost:8080/api/protection/rate-violations"

# Clear blocked IPs
curl -X DELETE "http://localhost:8080/api/protection/blocked/192.168.1.100"
```

## Best Practices

1. **Zone Configuration**
   - Enable full protection on WAN
   - Reduce protection on trusted zones
   - Customize per zone needs

2. **Rate Limiting**
   - Set reasonable limits
   - Monitor for false positives
   - Adjust based on usage

3. **Performance**
   - Monitor CPU usage
   - Track memory consumption
   - Optimize rule order

4. **Security**
   - Log all drops
   - Monitor for evasion attempts
   - Update signatures regularly

## Troubleshooting

### Common Issues
1. **Legitimate traffic blocked**: Check rate limits
2. **High CPU usage**: Reduce protection features
3. **Connection issues**: Check invalid packet rules

### Debug Commands
```bash
# Check protection rules
nft list chain inet flywall input

# Monitor drops
nft monitor

# Check conntrack
conntrack -L

# View statistics
nft list table inet flywall
```

### Advanced Debugging
```bash
# Trace packet
nft trace table inet flywall ip saddr 10.0.0.1

# Check rate limit counters
nft list counter inet flywall tcp_limit

# Monitor SYN floods
watch -n 1 'conntrack -L | grep SYN_SENT'
```

## Performance Considerations

- Protection adds minimal overhead
- SYN tracking uses most memory
- Rate limiting scales with connections
- Hardware offload available

## Security Considerations

- Protection bypass attempts
- Evasion techniques
- Resource exhaustion attacks
- False positive management

## Related Features

- [Zone Policies](zones-policies.md)
- [IP Sets & Blocklists](ipsets-blocklists.md)
- [Rate Limiting](rate-limiting.md)
- [Threat Intelligence](threat-intel.md)
