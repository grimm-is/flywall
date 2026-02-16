# Split Horizon DNS Implementation Guide

## Overview

Flywall provides split horizon DNS (also known as split DNS) for:
- Different responses based on client location
- Internal vs external name resolution
- Network segmentation
- Compliance requirements
- Optimized routing

## Architecture

### Split Horizon Components
1. **View Manager**: Manages DNS views
2. **Client Classifier**: Identifies client view
3. **Zone Manager**: Manages per-view zones
4. **Response Engine**: Generates appropriate responses
5. **Policy Engine**: Applies view policies

### View Types
- **Internal**: Internal network clients
- **External**: External clients
- **Guest**: Guest network clients
- **VPN**: VPN clients
- **Custom**: User-defined views

## Configuration

### Basic Split Horizon Setup
```hcl
# Split horizon DNS
dns {
  enabled = true

  # Views
  views = [
    {
      name = "internal"
      clients = ["192.168.0.0/16", "10.0.0.0/8"]
    },
    {
      name = "external"
      clients = ["0.0.0.0/0"]
    }
  ]

  # Zone configuration
  zones = [
    {
      name = "example.com"
      views = {
        internal = {
          records = [
            {name = "www", type = "A", address = "192.168.1.10"},
            {name = "mail", type = "A", address = "192.168.1.20"}
          ]
        },
        external = {
          records = [
            {name = "www", type = "A", address = "203.0.113.10"},
            {name = "mail", type = "A", address = "203.0.113.20"}
          ]
        }
      }
    }
  ]
}
```

### Advanced Split Horizon Configuration
```hcl
dns {
  enabled = true

  # View definitions
  views = [
    {
      name = "internal"
      description = "Internal corporate network"

      # Client matching
      match = {
        subnets = ["192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"]
        interfaces = ["eth1", "eth2"]
        mac_prefixes = ["00:11:22", "aa:bb:cc"]
      }

      # View-specific settings
      recursion = true
      forwarders = ["8.8.8.8", "1.1.1.1"]
      blocklists = ["internal-ads"]

      # Response policies
      policies = {
        nx_domain = ["internal-only.example.com"]
        redirect = {
          "intranet" = "192.168.1.100"
        }
      }
    },
    {
      name = "guest"
      description = "Guest network"

      match = {
        subnets = ["192.168.200.0/24"]
        interfaces = ["eth3"]
      }

      # Restricted guest access
      recursion = true
      forwarders = ["8.8.8.8"]
      blocklists = ["guest-filter", "ads", "malware"]

      # Time restrictions
      time_restrictions = {
        start = "06:00"
        end = "23:00"
      }
    },
    {
      name = "vpn"
      description = "VPN clients"

      match = {
        subnets = ["10.200.0.0/24"]
        interfaces = ["wg0"]
      }

      # Full internal access
      recursion = true
      forwarders = ["192.168.1.1", "8.8.8.8"]
      blocklists = ["malware"]
    },
    {
      name = "external"
      description = "External clients"

      match = {
        subnets = ["0.0.0.0/0"]
        exclude = ["192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"]
      }

      # Public DNS only
      recursion = false
      blocklists = []

      # Rate limiting
      rate_limit = {
        requests_per_second = 100
        burst = 200
      }
    }
  ]

  # Zone configurations
  zones = [
    {
      name = "company.com"

      # Per-view records
      views = {
        internal = {
          # Internal services
          records = [
            {name = "www", type = "A", ttl = 300, address = "192.168.1.10"},
            {name = "www", type = "A", ttl = 300, address = "192.168.1.11"},
            {name = "api", type = "A", ttl = 300, address = "192.168.1.20"},
            {name = "git", type = "A", ttl = 300, address = "192.168.1.30"},
            {name = "mail", type = "A", ttl = 300, address = "192.168.1.40"},
            {name = "vpn", type = "A", ttl = 300, address = "192.168.1.50"},
            # Internal-only services
            {name = "intranet", type = "A", ttl = 300, address = "192.168.1.100"},
            {name = "dev", type = "A", ttl = 300, address = "192.168.1.200"}
          ]

          # Internal forwarders
          forwarders = ["192.168.1.1"]
        },

        vpn = {
          # Most internal services accessible via VPN
          records = [
            {name = "www", type = "A", ttl = 300, address = "192.168.1.10"},
            {name = "api", type = "A", ttl = 300, address = "192.168.1.20"},
            {name = "git", type = "A", ttl = 300, address = "192.168.1.30"},
            {name = "mail", type = "A", ttl = 300, address = "192.168.1.40"},
            {name = "vpn", type = "A", ttl = 300, address = "192.168.1.50"}
          ]

          # VPN-specific forwarders
          forwarders = ["10.200.0.1", "192.168.1.1"]
        },

        external = {
          # Public services only
          records = [
            {name = "www", type = "A", ttl = 3600, address = "203.0.113.10"},
            {name = "www", type = "A", ttl = 3600, address = "203.0.113.11"},
            {name = "mail", type = "A", ttl = 3600, address = "203.0.113.20"},
            {name = "vpn", type = "A", ttl = 3600, address = "203.0.113.50"}
          ]

          # No internal forwarders
          forwarders = []
        }
      }
    },

    # Internal-only zone
    {
      name = "internal.company.com"

      views = {
        internal = {
          records = [
            {name = "dc1", type = "A", ttl = 300, address = "192.168.1.1"},
            {name = "dc2", type = "A", ttl = 300, address = "192.168.1.2"},
            {name = "files", type = "A", ttl = 300, address = "192.168.1.50"}
          ]
        },

        vpn = {
          records = [
            {name = "dc1", type = "A", ttl = 300, address = "192.168.1.1"},
            {name = "files", type = "A", ttl = 300, address = "192.168.1.50"}
          ]
        },

        # NXDOMAIN for external
        external = {
          action = "nxdomain"
        }
      }
    }
  ]
}
```

### Dynamic View Assignment
```hcl
dns {
  enabled = true

  # Dynamic view assignment
  views = [
    {
      name = "premium"
      description = "Premium customers"

      # Dynamic matching
      match = {
        # By authentication
        auth_tokens = ["premium-token-.*"]

        # By API key
        api_keys = ["premium-.*"]

        # By custom attribute
        attributes = {
          customer_tier = "premium"
        }
      }

      # Premium features
      recursion = true
      forwarders = ["8.8.8.8", "1.1.1.1", "9.9.9.9"]
      response_compression = true
      edns = {
        enabled = true
        udp_size = 4096
      }
    },

    {
      name = "basic"
      description = "Basic customers"

      match = {
        auth_tokens = ["basic-token-.*"]
        api_keys = ["basic-.*"]
        attributes = {
          customer_tier = "basic"
        }
      }

      # Basic features
      recursion = true
      forwarders = ["8.8.8.8"]
      rate_limit = {
        requests_per_second = 10
      }
    }
  ]
}
```

### GeoIP-Based Views
```hcl
dns {
  enabled = true

  # GeoIP-based views
  views = [
    {
      name = "us"
      description = "United States clients"

      match = {
        geoip = {
          countries = ["US"]
        }
      }

      # US-specific responses
      records = [
        {name = "cdn.example.com", type = "A", address = "192.0.2.1"},
        {name = "api.example.com", type = "A", address = "192.0.2.10"}
      ]
    },

    {
      name = "eu"
      description = "European clients"

      match = {
        geoip = {
          countries = ["GB", "DE", "FR", "IT", "ES"]
        }
      }

      # EU-specific responses (GDPR compliance)
      records = [
        {name = "cdn.example.com", type = "A", address = "198.51.100.1"},
        {name = "api.example.com", type = "A", address = "198.51.100.10"}
      ]

      # EU privacy settings
      privacy = {
        anonymize_queries = true
        retention_days = 30
      }
    },

    {
      name = "asia"
      description = "Asian clients"

      match = {
        geoip = {
          countries = ["JP", "SG", "HK", "KR"]
        }
      }

      # Asia-specific responses
      records = [
        {name = "cdn.example.com", type = "A", address = "203.0.113.1"},
        {name = "api.example.com", type = "A", address = "203.0.113.10"}
      ]
    }
  ]
}
```

## Implementation Details

### View Selection Process
1. Receive DNS query
2. Extract client information
3. Evaluate view match rules
4. Select best matching view
5. Query appropriate zone
6. Generate response

### Client Classification
```go
type ClientInfo struct {
    IP        string   `json:"ip"`
    Interface string   `json:"interface"`
    MAC       string   `json:"mac"`
    View      string   `json:"view"`
    Auth      AuthInfo `json:"auth"`
    GeoIP     GeoIPInfo `json:"geoip"`
}

type ViewMatch struct {
    Subnets      []string `json:"subnets"`
    Interfaces   []string `json:"interfaces"`
    MACPrefixes  []string `json:"mac_prefixes"`
    AuthTokens   []string `json:"auth_tokens"`
    GeoCountries []string `json:"geoip_countries"`
}
```

## Testing

### Split Horizon Testing
```bash
# Test from internal network
dig @192.168.1.1 www.company.com

# Test from VPN
dig @10.200.0.1 www.company.com

# Test from external
dig @203.0.113.1 www.company.com

# Check view assignment
flywall dns view-check --client 192.168.1.100
```

### Integration Tests
- `split_horizon_test.sh`: Basic split horizon
- `view_test.sh`: View assignment
- `geoip_view_test.sh`: GeoIP-based views

## API Integration

### View Management API
```bash
# List views
curl -s "http://localhost:8080/api/dns/views"

# Get view details
curl -s "http://localhost:8080/api/dns/views/internal"

# Update view
curl -X PUT "http://localhost:8080/api/dns/views/internal" \
  -H "Content-Type: application/json" \
  -d '{
    "match": {"subnets": ["192.168.0.0/16"]}
  }'

# Check client view
curl -s "http://localhost:8080/api/dns/view-check?client=192.168.1.100"
```

### Zone Management API
```bash
# Get zone for specific view
curl -s "http://localhost:8080/api/dns/zones/company.com?view=internal"

# Update zone records
curl -X PUT "http://localhost:8080/api/dns/zones/company.com/views/internal/records" \
  -H "Content-Type: application/json" \
  -d '{
    "records": [
      {"name": "new", "type": "A", "address": "192.168.1.60"}
    ]
  }'
```

## Best Practices

1. **View Design**
   - Keep views simple and clear
   - Document view purposes
   - Test view boundaries
   - Monitor view performance

2. **Security**
   - Validate client classification
   - Prevent view bypass
   - Log view assignments
   - Audit access patterns

3. **Performance**
   - Optimize view matching
   - Cache view assignments
   - Monitor query latency
   - Use appropriate TTLs

4. **Maintenance**
   - Regular view reviews
   - Update client lists
   - Test failover
   - Document changes

## Troubleshooting

### Common Issues
1. **Wrong view assigned**: Check match rules
2. **External sees internal**: Verify exclude rules
3. **VPN can't resolve**: Check interface matching
4. **High latency**: Optimize view matching

### Debug Commands
```bash
# Check view assignment
flywall dns view-check --client 192.168.1.100 --verbose

# Test specific view
flywall dns test --view internal --domain www.company.com

# Monitor view assignments
watch -n 1 'flywall dns stats views'

# Debug zone resolution
flywall dns debug --zone company.com --client 192.168.1.100
```

### Advanced Debugging
```bash
# Trace query resolution
flywall dns trace --domain www.company.com --client 192.168.1.100

# Check view cache
flywall dns cache view-status

# Validate view configuration
flywall dns validate-views

# Force view reload
flywall dns reload-views
```

## Performance Considerations

- View matching adds minimal overhead
- Complex match rules increase latency
- Caching improves performance
- Monitor view distribution

## Security Considerations

- View bypass attempts
- Information disclosure
- DNS cache poisoning
- Client spoofing

## Related Features

- [DNS Server](dns-server.md)
- [DNS Blocklists](dns-blocklists.md)
- [GeoIP Integration](geoip.md)
- [VPN Isolation](vpn-isolation.md)
