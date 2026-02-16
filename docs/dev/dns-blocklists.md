# DNS Blocklists Implementation Guide

## Overview

Flywall provides DNS-based blocklisting for:
- Domain filtering
- Ad blocking
- Malware protection
- Content filtering
- Custom blocklists

## Architecture

### Blocklist Components
1. **Blocklist Manager**: Manages blocklist sources
2. **Domain Resolver**: Queries blocklists for domains
3. **Cache Manager**: Caches blocklist results
4. **Updater**: Maintains blocklist freshness
5. **Policy Engine**: Applies blocking decisions

### Blocklist Sources
- Local files (hosts format)
- Remote URLs (HTTP/HTTPS)
- DNS-based blocklists
- API feeds
- Custom providers

## Configuration

### Basic DNS Blocklist Setup
```hcl
# DNS blocklist configuration
dns {
  enabled = true

  # Blocklist
  blocklist {
    enabled = true
    file = "/etc/flywall/blocklists/ads.txt"
    response_ip = "0.0.0.0"
  }
}
```

### Advanced DNS Blocklist Configuration
```hcl
dns {
  enabled = true

  # Multiple blocklists
  blocklists = [
    {
      name = "ads"
      enabled = true
      source = {
        type = "url"
        url = "https://pgl.yoyo.org/adservers/serverlist.php?hostformat=hosts&showintro=0&mimetype=plaintext"
        format = "hosts"
        update_interval = "24h"
      }
      action = "block"
      response_ip = "0.0.0.0"
      log = true
    },
    {
      name = "malware"
      enabled = true
      source = {
        type = "url"
        url = "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"
        format = "hosts"
        update_interval = "12h"
      }
      action = "block"
      response_ip = "127.0.0.1"
      log = true
      log_prefix = "MALWARE-BLOCK: "
    },
    {
      name = "custom"
      enabled = true
      source = {
        type = "file"
        path = "/etc/flywall/blocklists/custom.txt"
        format = "plain"
      }
      action = "redirect"
      response_ip = "192.168.1.254"
    }
  ]

  # Global blocklist settings
  blocklist_settings = {
    cache_ttl = "1h"
    cache_size = 100000
    wildcard_blocking = true
    case_sensitive = false

    # Response settings
    default_response_ip = "0.0.0.0"
    nxdomain = false

    # Logging
    log_all_queries = false
    log_blocked = true
    log_format = "json"
  }
}
```

### Per-Zone Blocklists
```hcl
zone "LAN" {
  interface = "eth1"

  dns {
    enabled = true

    # Zone-specific blocklists
    blocklists = [
      {
        name = "family_filter"
        enabled = true
        source = {
          type = "url"
          url = "https://example.com/family-filter.txt"
          format = "domains"
        }
        action = "block"
        response_ip = "0.0.0.0"
      }
    ]

    # Allowlist overrides
    allowlist = {
      enabled = true
      file = "/etc/flywall/allowlists/lan.txt"
      override_blocklist = true
    }
  }
}

zone "Guest" {
  interface = "eth2"

  dns {
    enabled = true

    # Strict filtering for guests
    blocklists = [
      {
        name = "strict"
        enabled = true
        source = {
          type = "url"
          url = "https://example.com/strict-filter.txt"
        }
        action = "block"
        response_ip = "0.0.0.0"
      }
    ]

    # Block all social media
    categories = ["social_media", "gambling", "adult"]
  }
}
```

### Category-Based Blocking
```hcl
dns {
  enabled = true

  # Category definitions
  categories = {
    social_media = {
      domains = [
        "facebook.com",
        "twitter.com",
        "instagram.com",
        "tiktok.com"
      ]
      description = "Social media platforms"
    }

    gambling = {
      domains = [
        "pokerstars.com",
        "bet365.com",
        "williamhill.com"
      ]
      description = "Gambling sites"
    }

    adult = {
      source = {
        type = "url"
        url = "https://example.com/adult-domains.txt"
      }
      description = "Adult content"
    }
  }

  # Category-based policies
  policies = [
    {
      name = "guest_policy"
      zones = ["Guest"]
      blocked_categories = ["social_media", "gambling", "adult"]
      log = true
    },
    {
      name = "child_policy"
      zones = ["Kids"]
      blocked_categories = ["social_media", "adult"]
      time_restrictions = {
        start = "20:00"
        end = "07:00"
      }
    }
  ]
}
```

### Dynamic Blocklists
```hcl
dns {
  enabled = true

  # Dynamic blocklist from API
  blocklist "threat_intel" {
    enabled = true
    source = {
      type = "api"
      url = "https://api.threatintel.com/domains"
      method = "GET"
      headers = {
        "Authorization" = "Bearer API_KEY"
        "User-Agent" = "Flywall/1.0"
      }
      format = "json"
      json_path = "$.domains[*].name"
      update_interval = "5m"
    }

    # Dynamic response based on threat level
    response_mapping = {
      "high" = {
        action = "block"
        response_ip = "127.0.0.1"
        log = true
        alert = true
      }
      "medium" = {
        action = "redirect"
        response_ip = "192.168.1.254"
        log = true
      }
      "low" = {
        action = "monitor"
        log = false
      }
    }
  }
}
```

## Implementation Details

### Blocklist Formats
```
# Hosts format
0.0.0.0 ads.example.com
127.0.0.1 tracker.example.com

# Plain domain format
ads.example.com
tracker.example.com

# JSON format
{
  "domains": [
    {"name": "ads.example.com", "category": "advertising"},
    {"name": "malware.example.com", "category": "malware"}
  ]
}
```

### Resolution Process
1. Receive DNS query
2. Check allowlist first
3. Query blocklists in order
4. Apply category rules
5. Return appropriate response
6. Log if configured

## Testing

### Blocklist Testing
```bash
# Test blocked domain
nslookup ads.example.com 192.168.1.1

# Test with dig
dig @192.168.1.1 facebook.com

# Check blocklist status
flywall dns blocklist status

# Update blocklists
flywall dns blocklist update

# Test category blocking
dig @192.168.1.1 pokerstars.com
```

### Integration Tests
- `dns_blocklist_file_test.sh`: File-based blocklists
- `dns_blocklist_url_test.sh`: URL-based blocklists
- `dns_category_test.sh`: Category blocking

## API Integration

### Blocklist API
```bash
# List blocklists
curl -s "http://localhost:8080/api/dns/blocklists"

# Get blocklist details
curl -s "http://localhost:8080/api/dns/blocklists/ads"

# Update blocklist
curl -X POST "http://localhost:8080/api/dns/blocklists/ads/update"

# Add domain to blocklist
curl -X POST "http://localhost:8080/api/dns/blocklists/custom/domains" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "badsite.com",
    "reason": "Manual block"
  }'

# Check if domain is blocked
curl -s "http://localhost:8080/api/dns/check?domain=facebook.com"
```

### Statistics API
```bash
# Get blocklist statistics
curl -s "http://localhost:8080/api/dns/blocklists/stats"

# Get top blocked domains
curl -s "http://localhost:8080/api/dns/blocklists/top-blocked"

# Get category statistics
curl -s "http://localhost:8080/api/dns/blocklists/stats/categories"
```

## Best Practices

1. **Blocklist Management**
   - Use reputable sources
   - Regular updates critical
   - Test before deployment
   - Monitor false positives

2. **Performance**
   - Enable caching
   - Use appropriate TTL
   - Monitor cache hit ratio
   - Limit blocklist size

3. **User Experience**
   - Provide block page
   - Allow override mechanism
   - Log blocked queries
   - Document policies

4. **Security**
   - Validate blocklist sources
   - Use HTTPS for URLs
   - Monitor for poisoning
   - Regular audits

## Troubleshooting

### Common Issues
1. **Domains not blocked**: Check blocklist loading
2. **False positives**: Review and adjust lists
3. **Performance issues**: Check cache settings
4. **Update failures**: Verify URLs and authentication

### Debug Commands
```bash
# Check blocklist status
flywall dns blocklist status --verbose

# Test specific domain
flywall dns test --domain ads.example.com

# Check cache
flywall dns cache status

# Monitor queries
flywall dns monitor
```

### Advanced Debugging
```bash
# Debug blocklist lookup
flywall dns debug --domain facebook.com

# Check blocklist content
head -20 /etc/flywall/blocklists/ads.txt

# Validate blocklist format
flywall dns blocklist validate /path/to/blocklist.txt

# Clear cache
flywall dns cache clear
```

## Performance Considerations

- Blocklist size affects memory usage
- Cache improves lookup speed
- Wildcard matching adds overhead
- Regular updates needed

## Security Considerations

- Blocklist sources must be trusted
- DNS poisoning possible
- Cache poisoning risk
- Privacy implications

## Related Features

- [DNS Server](dns-server.md)
- [DNS Query Logging](dns-querylog.md)
- [IP Sets & Blocklists](ipsets-blocklists.md)
- [Content Filtering](content-filtering.md)
