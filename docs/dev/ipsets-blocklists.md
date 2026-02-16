# IP Sets & Blocklists Implementation Guide

## Overview

Flywall provides IP sets and blocklist management for:
- Efficient IP address matching
- Dynamic blocklist updates
- FireHOL list format support
- URL-based blocklist feeds
- Automatic refresh and expiration

## Architecture

### IP Set Types
1. **IPv4 Sets**: IPv4 addresses and ranges
2. **IPv6 Sets**: IPv6 addresses and ranges
3. **Mixed Sets**: Combined IPv4/IPv6
4. **Named Sets**: Reusable across rules

### Blocklist Sources
1. **Local Files**: Hosts format lists
2. **URL Feeds**: HTTP/HTTPS sources
3. **API Feeds**: REST API sources
4. **Dynamic Sets**: Runtime additions

## Configuration

### Basic IP Sets
```hcl
# Create IPv4 blocklist set
ipset "blacklist" {
  type = "ipv4_addr"
  timeout = "24h"

  elements = [
    "192.168.1.100",
    "10.0.0.50",
    "203.0.113.0/24"
  ]
}

# Create CIDR set
ipset "bogon_networks" {
  type = "ipv4_addr"
  flags = ["interval"]

  elements = [
    "0.0.0.0/8",
    "10.0.0.0/8",
    "127.0.0.0/8",
    "169.254.0.0/16",
    "192.168.0.0/16",
    "224.0.0.0/4"
    # Add more as needed
  ]
}

# Create temporary block set
ipset "temp_blocks" {
  type = "ipv4_addr"
  flags = ["timeout"]
  timeout = "1h"

  elements = [
    "203.0.113.100"
  ]
}
```

### Blocklist Files
```hcl
# File-based blocklist
blocklist "ads" {
  file = "/etc/flywall/blocklists/ads.txt"
  format = "hosts"
  refresh = "1h"

  # Post-processing
  extract_domains = true
  resolve_to_ip = true
}

# FireHOL format
blocklist "firehol_level1" {
  file = "/etc/flywall/blocklists/firehol_level1.netset"
  format = "netset"
  refresh = "6h"

  # Validation
  validate_ranges = true
  skip_invalid = true
}

# Custom format
blocklist "custom" {
  file = "/etc/flywall/blocklists/custom.txt"
  format = "plain"
  refresh = "30m"

  # Field mapping
  field_separator = " "
  ip_field = 1
  comment_field = 2
}
```

### URL-Based Blocklists
```hcl
# URL blocklist with authentication
blocklist "threat_feed" {
  url = "https://api.threatintel.com/v1/blocked"
  refresh = "5m"

  # Authentication
  headers = {
    "Authorization" = "Bearer TOKEN"
    "User-Agent" = "Flywall/1.0"
  }

  # Parsing
  format = "json"
  json_path = "$.blocked_ips[*].ip"

  # Validation
  max_size = "100MB"
  timeout = "30s"
}

# Multiple URL sources
blocklist "malware_domains" {
  sources = [
    {
      url = "https://example.com/malware1.txt"
      format = "hosts"
    },
    {
      url = "https://example.com/malware2.txt"
      format = "plain"
    }
  ]

  refresh = "1h"
  merge_strategy = "union"
}
```

### Dynamic IP Sets
```hcl
# Fail2ban integration
ipset "fail2ban_ssh" {
  type = "ipv4_addr"
  flags = ["timeout"]
  timeout = "10m"

  # Auto-populate from log
  auto_populate = true
  log_pattern = "sshd.*Failed password for .* from ([0-9.]+)"
  log_file = "/var/log/auth.log"
}

# Rate limiting set
ipset "rate_limited" {
  type = "ipv4_addr"
  flags = ["timeout"]
  timeout = "5m"

  # Auto-add based on rate
  auto_add = true
  rate_threshold = "100/s"
  burst = 200
}
```

### Using IP Sets in Policies
```hcl
# Block IPs from set
policy "WAN" "self" {
  name = "block_blacklist"

  rule "block_blacklisted" {
    description = "Block blacklisted IPs"
    src_ip = "@blacklist"
    action = "drop"
    log = true
    log_prefix = "BLACKLIST-DROP: "
  }
}

# Allow only from whitelist
policy "WAN" "self" {
  name = "admin_access"

  rule "allow_admins" {
    description = "Allow admin IPs only"
    src_ip = "@admin_whitelist"
    dest_port = 22
    action = "accept"
  }

  rule "block_others" {
    description = "Block everyone else"
    action = "drop"
  }
}

# Dynamic blocking
policy "WAN" "self" {
  name = "dynamic_block"

  rule "block_rate_limit" {
    description = "Block rate limited IPs"
    src_ip = "@rate_limited"
    action = "drop"
  }
}
```

### Advanced Blocklist Configuration
```hcl
# Blocklist with caching
blocklist "geoip_block" {
  url = "https://example.com/geoip/cn.txt"
  refresh = "24h"

  # Caching
  cache_dir = "/var/cache/flywall/blocklists"
  cache_ttl = "48h"

  # Processing
  deduplicate = true
  sort_entries = true

  # Notifications
  on_update {
    script = "/etc/flywall/notify-blocklist-update.sh"
    email = "admin@example.com"
  }
}

# Conditional blocklist
blocklist "conditional" {
  file = "/etc/flywall/blocklists/conditional.txt"

  # Only apply if condition met
  condition = {
    script = "/etc/flywall/check-condition.sh"
    expected = "true"
  }

  # Fallback action
  on_condition_fail = "skip"
}
```

## Implementation Details

### Set Operations
- Add/remove elements dynamically
- Bulk operations for efficiency
- Atomic updates
- Memory-efficient storage

### Blocklist Processing
1. Download source
2. Parse and validate
3. Deduplicate entries
4. Update set atomically
5. Log changes

### Performance Optimization
- Hardware offload support
- Hash-based lookups
- Lazy loading for large sets
- Incremental updates

## Testing

### Integration Tests
- `ipset_test.sh`: Basic IP set operations
- `dns_blocklist_file_test.sh`: File-based blocklists
- `dns_blocklist_url_test.sh`: URL-based blocklists
- `ipset_traffic_test.sh`: Traffic filtering

### Manual Testing
```bash
# List IP sets
nft list sets

# Check set contents
nft get element inet flywall blacklist

# Add element
nft add element inet flywall blacklist { 192.168.1.100 }

# Test blocking
ping -c 1 192.168.1.100
```

## API Integration

### IP Set Management API
```bash
# List all IP sets
curl -s "http://localhost:8080/api/ipsets"

# Get specific set
curl -s "http://localhost:8080/api/ipsets/blacklist"

# Add element
curl -X POST "http://localhost:8080/api/ipsets/blacklist/elements" \
  -H "Content-Type: application/json" \
  -d '{"element": "192.168.1.100"}'

# Remove element
curl -X DELETE "http://localhost:8080/api/ipsets/blacklist/elements/192.168.1.100"

# Get set statistics
curl -s "http://localhost:8080/api/ipsets/blacklist/stats"
```

### Blocklist Management API
```bash
# List blocklists
curl -s "http://localhost:8080/api/blocklists"

# Refresh blocklist
curl -X POST "http://localhost:8080/api/blocklists/ads/refresh"

# Get blocklist status
curl -s "http://localhost:8080/api/blocklists/ads/status"

# Add custom entry
curl -X POST "http://localhost:8080/api/blocklists/custom/entries" \
  -H "Content-Type: application/json" \
  -d '{"ip": "203.0.113.100", "comment": "Manual block"}'
```

## Best Practices

1. **Set Design**
   - Use appropriate set types
   - Consider memory usage
   - Plan for growth
   - Document purposes

2. **Blocklist Management**
   - Use reliable sources
   - Validate entries
   - Monitor update failures
   - Keep backups

3. **Performance**
   - Limit set sizes
   - Use intervals for ranges
   - Monitor memory usage
   - Consider hardware limits

4. **Security**
   - Verify blocklist sources
   - Monitor for false positives
   - Log all blocks
   - Review regularly

## Troubleshooting

### Common Issues
1. **Set not updating**: Check permissions and URLs
2. **High memory usage**: Reduce set size or use intervals
3. **False positives**: Validate blocklist sources

### Debug Commands
```bash
# Check set contents
nft list set inet flywall blacklist

# Monitor set changes
nft monitor

# Check blocklist download
curl -I https://example.com/blocklist.txt

# Validate blocklist format
head -20 /etc/flywall/blocklists/ads.txt
```

### Advanced Debugging
```bash
# Check nftables performance
nft list counters

# Monitor memory usage
watch -n 1 'cat /proc/meminfo | grep -E "(MemTotal|MemAvailable)"'

# Test set lookup
nft get element inet flywall blacklist { 192.168.1.100 }
```

## Performance Considerations

- IP sets provide O(1) lookup
- Memory scales with entry count
- Hardware offload for large sets
- Update frequency affects performance

## Security Considerations

- Validate all blocklist sources
- Monitor for poisoning attempts
- Use HTTPS for URL sources
- Regular audit of blocked IPs

## Related Features

- [Zone Policies](zones-policies.md)
- [DNS Blocklists](dns-blocklists.md)
- [Fail2Ban Integration](fail2ban.md)
- [Threat Intelligence](threat-intel.md)
