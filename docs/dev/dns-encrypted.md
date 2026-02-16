# Encrypted DNS Implementation Guide

## Overview

Flywall provides encrypted DNS support for:
- DNS over HTTPS (DoH)
- DNS over TLS (DoT)
- DNS over QUIC (DoQ)
- Encrypted upstream resolvers
- Privacy-focused DNS resolution

## Architecture

### Encrypted DNS Components
1. **DoH Client**: HTTPS-based DNS queries
2. **DoT Client**: TLS-based DNS queries
3. **DoQ Client**: QUIC-based DNS queries
3. **Certificate Manager**: TLS certificate validation
4. **Fallback Manager**: Fallback to plain DNS
5. **Privacy Manager**: Query privacy features

### Supported Protocols
- **DoH**: RFC 8484 (DNS over HTTPS)
- **DoT**: RFC 7858 (DNS over TLS)
- **DoQ**: RFC 9250 (DNS over QUIC)
- **Plain DNS**: Fallback support

## Configuration

### Basic Encrypted DNS Setup
```hcl
# Encrypted DNS configuration
dns {
  enabled = true

  # Encrypted forwarders
  forwarders = [
    {
      type = "doh"
      url = "https://dns.google/dns-query"
    },
    {
      type = "dot"
      address = "dns.quad9.net"
      port = 853
    }
  ]
}
```

### Advanced Encrypted DNS Configuration
```hcl
dns {
  enabled = true

  # Multiple encrypted upstreams
  forwarders = [
    {
      name = "google_doh"
      type = "doh"
      url = "https://dns.google/dns-query"

      # HTTP settings
      http = {
        method = "GET"
        headers = {
          "Accept" = "application/dns-message"
          "User-Agent" = "Flywall/1.0"
        }

        # Connection pooling
        max_connections = 10
        keep_alive = "30s"

        # Proxy support
        proxy = {
          url = "http://proxy.example.com:8080"
          auth = {
            username = "user"
            password = "pass"
          }
        }
      }

      # TLS settings
      tls = {
        verify = true
        ca_file = "/etc/flywall/ca-bundle.crt"
        server_name = "dns.google"

        # Session resumption
        session_tickets = true
        session_timeout = "1h"
      }

      # Performance
      timeout = "5s"
      retries = 3
      backoff = "exponential"
    },

    {
      name = "quad9_dot"
      type = "dot"
      address = "dns.quad9.net"
      port = 853

      # TLS settings
      tls = {
        verify = true
        ca_file = "/etc/flywall/ca-bundle.crt"
        server_name = "dns.quad9.net"

        # Cipher suites
        cipher_suites = [
          "TLS_AES_256_GCM_SHA384",
          "TLS_CHACHA20_POLY1305_SHA256",
          "TLS_AES_128_GCM_SHA256"
        ]

        # Minimum version
        min_version = "1.2"
      }

      # Connection settings
      tcp = {
        keep_alive = true
        nodelay = true
      }

      # Performance
      timeout = "3s"
      retries = 2
    },

    {
      name = "cloudflare_doq"
      type = "doq"
      address = "dns.cloudflare.com"
      port = 784

      # QUIC settings
      quic = {
        versions = ["v1", "draft-29"]
        require_retry = true

        # Connection settings
        max_idle_timeout = "30s"
        max_udp_payload_size = 1200

        # Migration
        disable_active_migration = false
      }

      # TLS settings
      tls = {
        verify = true
        ca_file = "/etc/flywall/ca-bundle.crt"
        server_name = "dns.cloudflare.com"
        alpn = ["doq"]
      }
    }
  ]

  # Fallback configuration
  fallback = {
    enabled = true

    # Fallback to plain DNS
    plain_dns = [
      "8.8.8.8",
      "1.1.1.1"
    ]

    # Fallback conditions
    conditions = [
      "all_encrypted_failed",
      "timeout",
      "certificate_error"
    ]

    # Fallback timeout
    timeout = "2s"
  }

  # Privacy settings
  privacy = {
    # EDNS(0) Client Subnet
    ecs = {
      enabled = false
      policy = "hide"  # hide, expose, custom

      # Custom subnet
      custom = {
        family = "ipv4"
        source_prefix = 24
        scope_prefix = 24
      }
    }

    # Query padding
    padding = {
      enabled = true
      block_size = 128
    }

    # Cache poisoning protection
    cache_poison_protection = true
  }
}
```

### Per-Zone Encrypted DNS
```hcl
zone "LAN" {
  interface = "eth1"

  dns = {
    enabled = true

    # Use encrypted DNS for LAN
    forwarders = [
      {
        type = "doh"
        url = "https://dns.google/dns-query"
      }
    ]

    # Local cache
    cache = {
      enabled = true
      ttl = "1h"
      max_size = 10000
    }
  }
}

zone "Guest" {
  interface = "eth2"

  dns = {
    enabled = true

    # Privacy-focused for guests
    forwarders = [
      {
        type = "doh"
        url = "https://dns.adguard.com/dns-query"

        # Ad blocking
        features = ["blocking", "safe_search"]
      },
      {
        type = "dot"
        address = "dns9.quad9.net"
        port = 853

        # Security features
        features = ["malware_blocking", "botnet_protection"]
      }
    ]

    # Strict privacy
    privacy = {
      ecs = { enabled = false }
      padding = { enabled = true }
      no_logs = true
    }
  }
}

zone "Kids" {
  interface = "eth3"

  dns = {
    enabled = true

    # Family-friendly DNS
    forwarders = [
      {
        type = "doh"
        url = "https://family.opendns.com/dns-query"

        # Family features
        features = [
          "adult_content_filter",
          "malware_blocking",
          "phishing_protection"
        ]
      }
    ]

    # Time restrictions
    time_restrictions = {
      start = "20:00"
      end = "07:00"
      timezone = "America/New_York"
    }
  }
}
```

### Conditional Encrypted DNS
```hcl
dns {
  enabled = true

  # Conditional forwarding
  conditional_forwarders = [
    {
      # Use encrypted for sensitive domains
      domains = ["*.bank.com", "*.paypal.com", "*.government.gov"]
      forwarders = [
        {
          type = "doh"
          url = "https://dns.google/dns-query"
        }
      ]
    },
    {
      # Use plain DNS for local domains
      domains = ["*.local", "*.lan", "*.internal"]
      forwarders = [
        "192.168.1.1",
        "192.168.1.2"
      ]
    },
    {
      # Default encrypted
      domains = ["*"]
      forwarders = [
        {
          type = "dot"
          address = "1.1.1.1"
          port = 853
        }
      ]
    }
  ]

  # Split tunneling
  split_tunnel = {
    enabled = true

    # Encrypted domains
    encrypted = [
      "social_media.com",
      "streaming.com",
      "news.com"
    ]

    # Plain DNS domains
    plain = [
      "local.lan",
      "company.internal"
    ]
  }
}
```

### Performance Optimization
```hcl
dns {
  enabled = true

  # Connection pooling
  connection_pool = {
    enabled = true
    max_connections = 20
    max_idle = 5
    idle_timeout = "60s"
  }

  # Query batching
  batching = {
    enabled = true
    max_batch_size = 10
    batch_timeout = "10ms"
  }

  # Prefetching
  prefetch = {
    enabled = true
    threshold = 3
    ttl_multiplier = 0.8
  }

  # Caching
  cache = {
    enabled = true

    # Cache size
    max_size = 100000

    # Cache TTL
    min_ttl = "30s"
    max_ttl = "1d"

    # Cache persistence
    persist = true
    persist_file = "/var/lib/flywall/dns_cache.db"

    # Cache validation
    validate = true
    stale_ttl = "1h"
  }

  # Compression
  compression = {
    enabled = true
    algorithm = "gzip"
    min_size = 1024
  }
}
```

## Implementation Details

### DoH Request Format
```http
GET /dns-query?dns=AAABAAABAAAAAAAAB2V4YW1wbGUDY29tAAABAAE HTTP/1.1
Host: dns.google
Accept: application/dns-message
User-Agent: Flywall/1.0
Content-Type: application/dns-message
```

### DoT Connection Flow
1. TCP connection to server
2. TLS handshake
3. Certificate validation
4. DNS queries over TLS
5. Connection reuse

### DoQ Connection Flow
1. UDP connection to server
2. QUIC handshake
3. TLS 1.3 handshake
4. DNS queries over QUIC
5. Connection migration support

## Testing

### Encrypted DNS Testing
```bash
# Test DoH
curl -H "Accept: application/dns-message" \
  "https://dns.google/dns-query?dns=AAABAAABAAAAAAAAB2V4YW1wbGUDY29tAAABAAE"

# Test DoT
dig @dns.quad9.net -p 853 +tls example.com

# Test DoQ
dig @dns.cloudflare.com -p 784 +quic example.com

# Test with kdig
kdig @dns.google +https example.com
```

### Integration Tests
- `encrypted_dns_test.sh`: Basic encrypted DNS
- `doh_test.sh`: DNS over HTTPS
- `dot_test.sh`: DNS over TLS
- `doq_test.sh`: DNS over QUIC

## API Integration

### Encrypted DNS API
```bash
# Get encrypted DNS status
curl -s "http://localhost:8080/api/dns/encrypted/status"

# List forwarders
curl -s "http://localhost:8080/api/dns/forwarders"

# Test encrypted DNS
curl -X POST "http://localhost:8080/api/dns/test" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "type": "A",
    "forwarder": "google_doh"
  }'

# Get certificate status
curl -s "http://localhost:8080/api/dns/encrypted/certificates"
```

### Statistics API
```bash
# Get encrypted DNS statistics
curl -s "http://localhost:8080/api/dns/encrypted/stats"

# Get per-forwarder stats
curl -s "http://localhost:8080/api/dns/forwarders/stats"

# Get privacy statistics
curl -s "http://localhost:8080/api/dns/privacy/stats"
```

## Best Practices

1. **Privacy**
   - Use reputable providers
   - Enable query padding
   - Disable ECS when possible
   - Rotate providers regularly

2. **Performance**
   - Enable caching
   - Use connection pooling
   - Monitor latency
   - Optimize timeouts

3. **Security**
   - Validate certificates
   - Use secure TLS settings
   - Monitor for failures
   - Have fallback options

4. **Reliability**
   - Configure multiple providers
   - Test failover
   - Monitor provider status
   - Update certificates

## Troubleshooting

### Common Issues
1. **Certificate errors**: Check CA bundle
2. **High latency**: Try different providers
3. **Connection failures**: Check firewall rules
4. **Fallback not working**: Verify configuration

### Debug Commands
```bash
# Check encrypted DNS status
flywall dns encrypted status

# Test specific forwarder
flywall dns test --forwarder google_doh --domain example.com

# Check certificates
flywall dns encrypted certificates

# Monitor queries
flywall dns monitor --type encrypted
```

### Advanced Debugging
```bash
# Debug DoH
curl -v -H "Accept: application/dns-message" \
  "https://dns.google/dns-query?dns=AAABAAABAAAAAAAAB2V4YW1wbGUDY29tAAABAAE"

# Debug DoT
openssl s_client -connect dns.quad9.net:853 -servername dns.quad9.net

# Check DNS cache
flywall dns cache stats

# Force cache clear
flywall dns cache clear
```

## Performance Considerations

- Encrypted DNS adds latency
- TLS handshake overhead
- Connection pooling helps
- Caching is critical

## Security Considerations

- Provider trust is essential
- Certificate validation critical
- Query timing attacks possible
- DNSSEC support recommended

## Privacy Considerations

- Providers can see queries
- Metadata still exposed
- Consider multi-provider approach
- Use padding when possible

## Related Features

- [DNS Server](dns-server.md)
- [DNS Blocklists](dns-blocklists.md)
- [Split Horizon DNS](dns-split-horizon.md)
- [Privacy Features](privacy-features.md)
