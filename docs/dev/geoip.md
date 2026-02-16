# GeoIP Integration Implementation Guide

## Overview

Flywall provides GeoIP integration for:
- Geographic-based filtering
- Location-aware policies
- Country statistics
- Compliance enforcement
- Threat intelligence

## Architecture

### GeoIP Components
1. **GeoIP Database**: IP to location mapping
2. **Lookup Engine**: Fast IP resolution
3. **Policy Engine**: Location-based rules
4. **Updater**: Database update service
5. **Statistics Collector**: Geographic metrics

### Supported Databases
- **MaxMind GeoLite2**: Free country/city database
- **MaxMind GeoIP2**: Commercial precision database
- **DB-IP**: Alternative GeoIP provider
- **Custom**: User-provided databases

## Configuration

### Basic GeoIP Setup
```hcl
# GeoIP configuration
geoip {
  enabled = true

  # Database
  database = "/etc/flywall/GeoLite2-Country.mmdb"

  # Update settings
  auto_update = true
  update_interval = "7d"

  # License key (for commercial databases)
  license_key = "MAXMIND_LICENSE_KEY"
}
```

### Advanced GeoIP Configuration
```hcl
geoip {
  enabled = true

  # Multiple databases
  databases = [
    {
      type = "country"
      path = "/etc/flywall/GeoLite2-Country.mmdb"
      description = "Country database"
    },
    {
      type = "city"
      path = "/etc/flywall/GeoLite2-City.mmdb"
      description = "City database"
    },
    {
      type = "asn"
      path = "/etc/flywall/GeoLite2-ASN.mmdb"
      description = "ASN database"
    }
  ]

  # Update configuration
  update = {
    enabled = true
    source = "maxmind"
    interval = "7d"
    retry = {
      attempts = 3
      backoff = "exponential"
    }

    # Authentication
    account_id = "123456"
    license_key = "MAXMIND_LICENSE_KEY"

    # Proxy support
    proxy = {
      url = "http://proxy.example.com:8080"
      username = "user"
      password = "pass"
    }
  }

  # Cache settings
  cache = {
    enabled = true
    size = 100000
    ttl = "1h"
  }

  # Performance
  performance = {
    memory_map = true
    preload = true
  }
}
```

### GeoIP Policies
```hcl
# Country-based filtering
policy "WAN" "self" {
  name = "geoip_filter"

  rule "block_countries" {
    description = "Block specific countries"
    src_geo = ["CN", "RU", "KP"]
    action = "drop"
    log = true
  }

  rule "allow_allies" {
    description = "Allow allied countries"
    src_geo = ["US", "CA", "GB", "AU", "NZ"]
    action = "accept"
  }

  rule "default_deny" {
    description = "Default deny for others"
    action = "drop"
  }
}

# GeoIP with rate limiting
policy "WAN" "self" {
  name = "geoip_rate_limit"

  rule "limit_china" {
    description = "Strict rate limit for China"
    src_geo = ["CN"]
    rate_limit = {
      rate = "10/s"
      burst = 20
    }
    action = "accept"
  }

  rule "limit_russia" {
    description = "Moderate rate limit for Russia"
    src_geo = ["RU"]
    rate_limit = {
      rate = "100/s"
      burst = 200
    }
    action = "accept"
  }
}
```

### Zone-Based GeoIP
```hcl
zone "WAN" {
  interface = "eth0"

  # GeoIP settings
  geoip = {
    enabled = true

    # Default action
    default_action = "accept"

    # Blocked countries
    blocked_countries = ["CN", "RU", "KP", "IR"]

    # Allowed countries (whitelist mode)
    # allowed_countries = ["US", "CA", "GB"]

    # Country-specific settings
    country_settings = {
      "US" = {
        action = "accept"
        rate_limit = "1000/s"
        log = false
      }
      "CN" = {
        action = "drop"
        log = true
        log_prefix = "GEOIP-BLOCK-CN: "
      }
      "RU" = {
        action = "accept"
        rate_limit = "100/s"
        inspection = "deep"
      }
    }
  }
}
```

### GeoIP in DNS
```hcl
dns {
  enabled = true

  # GeoIP-based DNS responses
  geoip_responses = true

  serve "geo" {
    # Country-specific responses
    geo_responses = [
      {
        countries = ["US", "CA"]
        response = "192.168.1.10"
      },
      {
        countries = ["GB", "DE", "FR"]
        response = "192.168.1.20"
      },
      {
        countries = ["CN", "RU"]
        response = "192.168.1.30"
      }
    ]

    # Default response
    default_response = "192.168.1.1"
  }
}
```

## Implementation Details

### GeoIP Lookup
```go
type GeoIPResult struct {
    Country struct {
        ISOCode  string `json:"iso_code"`
        Name     string `json:"name"`
        IsEU     bool   `json:"is_in_european_union"`
    } `json:"country"`

    City struct {
        Name     string  `json:"name"`
        Postal   string  `json:"postal_code"`
        Location struct {
            Latitude  float64 `json:"latitude"`
            Longitude float64 `json:"longitude"`
        } `json:"location"`
    } `json:"city,omitempty"`

    ASN struct {
        Number    uint32 `json:"autonomous_system_number"`
        Organization string `json:"autonomous_system_organization"`
    } `json:"asn,omitempty"`
}
```

### Lookup Process
1. Extract IP from packet
2. Check cache first
3. Query GeoIP database
4. Cache result
5. Apply policy rules

## Testing

### GeoIP Testing
```bash
# Test GeoIP lookup
flywall geoip lookup 8.8.8.8

# Test blocked country
ping -c 1 116.251.211.0  # China IP

# Check GeoIP stats
flywall geoip stats

# Update database
flywall geoip update
```

### Integration Tests
- `geoip_test.sh`: Basic GeoIP functionality
- `geoip_policy_test.sh`: Policy enforcement
- `geoip_update_test.sh`: Database updates

## API Integration

### GeoIP API
```bash
# Lookup IP
curl -s "http://localhost:8080/api/geoip/lookup/8.8.8.8"

# Lookup multiple IPs
curl -s "http://localhost:8080/api/geoip/lookup" \
  -H "Content-Type: application/json" \
  -d '{
    "ips": ["8.8.8.8", "1.1.1.1", "8.8.4.4"]
  }'

# Get GeoIP statistics
curl -s "http://localhost:8080/api/geoip/stats"

# Get country statistics
curl -s "http://localhost:8080/api/geoip/stats/countries"

# Update database
curl -X POST "http://localhost:8080/api/geoip/update"
```

### Policy API
```bash
# Get GeoIP policies
curl -s "http://localhost:8080/api/policies?geoip=true"

# Update GeoIP settings
curl -X PUT "http://localhost:8080/api/zones/WAN/geoip" \
  -H "Content-Type: application/json" \
  -d '{
    "blocked_countries": ["CN", "RU"],
    "default_action": "accept"
  }'
```

## Best Practices

1. **Database Management**
   - Keep databases updated
   - Validate database integrity
   - Monitor update failures
   - Backup databases

2. **Performance**
   - Enable caching
   - Use memory mapping
   - Monitor lookup performance
   - Optimize cache size

3. **Policy Design**
   - Use specific country codes
   - Document policy rationale
   - Review blocked countries
   - Consider compliance needs

4. **Privacy**
   - Follow privacy laws
   - Minimize data collection
   - Anonymize when possible
   - Document data usage

## Troubleshooting

### Common Issues
1. **Database not loading**: Check file permissions
2. **Incorrect results**: Update database
3. **Performance issues**: Enable caching
4. **Update failures**: Check license key

### Debug Commands
```bash
# Check GeoIP status
flywall geoip status

# Validate database
flywall geoip validate

# Check cache
flywall geoip cache stats

# Test lookup
flywall geoip test --ip 8.8.8.8
```

### Advanced Debugging
```bash
# Debug specific IP
flywall geoip debug --ip 116.251.211.0

# Check database info
mmdblookup --file /etc/flywall/GeoLite2-Country.mmdb --ip 8.8.8.8

# Monitor cache hits
watch -n 1 'flywall geoip cache stats'

# Force database reload
flywall geoip reload
```

## Performance Considerations

- Database size affects memory usage
- Cache improves lookup speed
- Memory mapping reduces I/O
- Batch lookups more efficient

## Security Considerations

- GeoIP data may be inaccurate
- VPNs bypass GeoIP blocking
- Consider privacy implications
- Regular database updates needed

## Compliance

- GDPR: EU data protection
- Export controls: Country restrictions
- Data residency: Local storage
- Audit requirements: Logging

## Related Features

- [IP Sets & Blocklists](ipsets-blocklists.md)
- [Protection Features](protection-features.md)
- [Analytics Engine](analytics-engine.md)
- [Policy Management](policy-management.md)
