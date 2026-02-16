# DNS Server Implementation Guide

## Overview

Flywall includes a built-in DNS server that provides:
- Local DNS resolution for internal hosts
- DNS forwarding to upstream resolvers
- Domain-based blocking via blocklists
- Split-horizon DNS for different zones
- Query logging and persistence

## Architecture

### DNS Modes
1. **Forward Mode**: Forward queries to upstream DNS servers
2. **Recursive Mode**: Resolve queries recursively (future feature)
3. **Split Horizon**: Different responses based on client zone

### Components
- DNS Server: Core DNS implementation
- Blocklists: Domain filtering system
- Query Logger: Tracks all DNS queries
- Zone Services: Per-zone DNS configuration

## Configuration

### Basic DNS Server
```hcl
dns {
  enabled = true
  listen_on = ["10.1.0.1", "10.2.0.1"]
  forwarders = ["8.8.8.8", "1.1.1.1"]

  # Default zone for all queries
  serve "default" {
    local_domain = "local"
  }
}
```

### Local Host Records
```hcl
dns {
  serve "local" {
    local_domain = "local"

    host "10.0.0.1" {
      hostnames = ["testhost", "testhost.local"]
    }

    host "192.168.1.1" {
      hostnames = ["router", "router.local"]
    }
  }
}
```

### Zone-Based DNS
```hcl
zone "Green" {
  interface = "eth1"
  services {
    dns = true
  }
}

zone "Orange" {
  interface = "eth2"
  services {
    dns = false  # No DNS for this zone
  }
}
```

### DNS Blocklists
```hcl
dns {
  serve "local" {
    # File-based blocklist
    blocklist "ads" {
      file = "/etc/flywall/blocklists/ads.txt"
    }

    # URL-based blocklist
    blocklist "malware" {
      url = "https://example.com/malware-domains.txt"
      refresh = "24h"
    }
  }
}
```

Blocklist file format (hosts-style):
```
# Comments start with #
0.0.0.0 ads.example.com
127.0.0.1 tracking.example.com
```

### Conditional Forwarding
```hcl
dns {
  serve "local" {
    # Forward specific domains to custom DNS
    forward "corp.local" {
      to = ["10.10.10.10", "10.10.10.11"]
    }

    forward "internal.example.com" {
      to = ["192.168.1.100"]
    }
  }
}
```

### Split Horizon DNS
```hcl
# Different responses for different zones
dns {
  # Green zone - internal IPs
  serve "green" {
    zone = "Green"
    local_domain = "green"

    host "10.1.0.10" {
      hostnames = ["server", "server.green"]
    }
  }

  # WAN zone - public IPs
  serve "wan" {
    zone = "WAN"
    local_domain = "example.com"

    host "203.0.113.10" {
      hostnames = ["server", "server.example.com"]
    }
  }
}
```

## Implementation Details

### Query Processing Flow
1. Client sends DNS query
2. Check local host records first
3. Check blocklists (if domain blocked, return 0.0.0.0)
4. Check conditional forwarders
5. Forward to upstream DNS
6. Cache response for future queries

### DNS Cache
- Positive responses cached based on TTL
- Negative responses cached for minimum TTL
- Cache size limited by configuration
- Cache persists across restarts

### Query Logging
```hcl
dns {
  # Enable query logging
  query_log = true

  serve "local" {
    # Per-zone logging
    query_log = true
  }
}
```

## Testing

### Integration Tests
- `dns_traffic_test.sh`: Basic DNS resolution
- `dns_blocklist_file_test.sh`: File-based blocklists
- `dns_blocklist_url_test.sh`: URL-based blocklists
- `dns_querylog_test.sh`: Query logging
- `split_horizon_test.sh`: Split horizon DNS

### Manual Testing
```bash
# Test DNS resolution
dig @10.1.0.1 testhost.local

# Test blocked domain
dig @10.1.0.1 ads.example.com

# Test conditional forwarding
dig @10.1.0.1 server.corp.local

# Check DNS cache
flywall dns cache list

# View query log
flywall dns querylog list --limit 100
```

## API Integration

### DNS Query API
```bash
# Get query history
curl -s "http://localhost:8080/api/dns/queries?limit=100"

# Get DNS stats
curl -s "http://localhost:8080/api/dns/stats"

# Get cache contents
curl -s "http://localhost:8080/api/dns/cache"
```

### Blocklist Management
```bash
# List blocklists
curl -s "http://localhost:8080/api/dns/blocklists"

# Reload blocklists
curl -X POST "http://localhost:8080/api/dns/blocklists/reload"
```

## Best Practices

1. **Performance**
   - Use multiple forwarders for redundancy
   - Enable query caching
   - Monitor cache hit rates

2. **Security**
   - Regularly update blocklists
   - Use DNSSEC validation when available
   - Limit DNS to trusted zones

3. **Reliability**
   - Configure backup forwarders
   - Monitor DNS response times
   - Log failed queries

4. **Split Horizon**
   - Document zone-specific responses
   - Test from all zones
   - Use consistent naming

## Troubleshooting

### Common Issues
1. **DNS not responding**: Check if DNS is enabled for the zone
2. **Blocked domains still resolving**: Verify blocklist format
3. **Slow responses**: Check forwarder latency

### Debug Commands
```bash
# Check DNS server status
flywall show dns

# Test with verbose output
dig @10.1.0.1 test.local +verbose

# Monitor DNS queries
journalctl -u flywall | grep "DNS query"

# Check blocklist status
flywall dns blocklist status
```

### DNS Debug Tools
```bash
# Query specific record type
dig @10.1.0.1 example.com MX

# Trace DNS path
dig @10.1.0.1 example.com +trace

# Test DNS over TCP
dig @10.1.0.1 example.com +tcp
```

## Performance Considerations

- DNS server handles thousands of queries per second
- Cache hit ratio significantly impacts performance
- Blocklist size affects lookup time
- Consider using DNS prefetching for popular domains

## Security Considerations

- DNS amplification attacks mitigated by default
- Rate limiting can be applied per-client
- DNS over TLS support planned for future release
- Query logging helps identify suspicious patterns

## Related Features

- [Split Horizon DNS](dns-split-horizon.md)
- [DNS Blocklists](dns-blocklists.md)
- [Query Logging](dns-querylog.md)
- [Encrypted DNS](dns-encrypted.md)
- [Learning Engine](learning-engine.md) - DNS-based learning
