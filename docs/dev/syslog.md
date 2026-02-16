# Syslog Integration Implementation Guide

## Overview

Flywall provides comprehensive syslog integration for:
- Log forwarding to external syslog servers
- Structured logging support
- Multiple facility and severity levels
- Reliable delivery with buffering
- Log filtering and routing

## Architecture

### Syslog Components
1. **Log Collector**: Gathers logs from all components
2. **Formatter**: Converts logs to syslog format
3. **Transport Manager**: Handles delivery protocols
4. **Buffer Manager**: Manages log buffering and retry
5. **Filter Engine**: Routes logs based on criteria

### Supported Protocols
- **UDP**: Fast but unreliable
- **TCP**: Reliable delivery
- **TLS**: Encrypted secure delivery
- **RELP**: Reliable Event Logging Protocol

## Configuration

### Basic Syslog Setup
```hcl
# Syslog configuration
syslog {
  enabled = true

  # Remote syslog server
  server = "syslog.example.com"
  port = 514
  protocol = "udp"

  # Facility
  facility = "local0"

  # Default severity
  severity = "info"
}
```

### Advanced Syslog Configuration
```hcl
syslog {
  enabled = true

  # Multiple servers
  servers = [
    {
      name = "primary"
      address = "syslog1.example.com"
      port = 514
      protocol = "tcp"
      facility = "local0"
      severity = "info"
      tls = false
    },
    {
      name = "backup"
      address = "syslog2.example.com"
      port = 6514
      protocol = "tcp"
      facility = "local1"
      severity = "warning"
      tls = {
        enabled = true
        cert_file = "/etc/flywall/syslog-client.crt"
        key_file = "/etc/flywall/syslog-client.key"
        ca_file = "/etc/flywall/syslog-ca.crt"
        verify_server = true
      }
    }
  ]

  # Global settings
  hostname = "firewall-01"
  app_name = "flywall"

  # Buffering
  buffer = {
    enabled = true
    max_size = "10MB"
    flush_interval = "5s"
    flush_on_error = true
  }

  # Retry settings
  retry = {
    enabled = true
    max_attempts = 5
    backoff = "exponential"
    initial_delay = "1s"
    max_delay = "30s"
  }

  # Formatting
  format = "rfc5424"
  include_timestamp = true
  include_hostname = true
  include_app_name = true
  include_procid = true
  include_msgid = true
}
```

### Log Filtering and Routing
```hcl
syslog {
  enabled = true
  server = "syslog.example.com"
  port = 514
  protocol = "tcp"

  # Log routing rules
  routes = [
    {
      # Security events to security team
      name = "security"
      match = {
        component = "protection",
        severity = ["warning", "error", "critical"]
      }
      server = "syslog-security.example.com"
      facility = "local4"
      severity = "warning"
    },
    {
      # Performance metrics to monitoring
      name = "performance"
      match = {
        component = "metrics",
        type = "performance"
      }
      server = "syslog-metrics.example.com"
      facility = "local5"
      severity = "info"
    },
    {
      # Audit logs to compliance
      name = "audit"
      match = {
        component = ["api", "config", "auth"]
      }
      server = "syslog-audit.example.com"
      facility = "local6"
      severity = "notice"
    },
    {
      # Default for everything else
      name = "default"
      server = "syslog.example.com"
      facility = "local0"
      severity = "info"
    }
  ]
}
```

### Component-Specific Logging
```hcl
# Firewall logging
firewall {
  log {
    # Enable syslog forwarding
    syslog = true

    # Firewall-specific settings
    facility = "local0"
    severity = "info"

    # What to log
    log_drops = true
    log_accepts = false
    log_invalid = true

    # Rate limiting
    rate_limit = {
      enabled = true
      max_per_second = 100
      burst = 1000
    }
  }
}

# API logging
api {
  logging = {
    # Enable syslog
    syslog = true

    # API-specific settings
    facility = "local1"
    severity = "info"

    # What to log
    log_requests = true
    log_responses = false
    log_errors = true

    # Sensitive data
    sanitize_auth = true
    sanitize_tokens = true
  }
}

# DHCP logging
dhcp {
  logging = {
    syslog = true
    facility = "local2"
    severity = "info"

    # DHCP events
    log_discover = true
    log_offer = true
    log_request = true
    log_release = true
    log_decline = true
  }
}
```

### Structured Logging
```hcl
syslog {
  enabled = true
  server = "syslog.example.com"
  port = 514
  protocol = "tcp"

  # Structured data (RFC5424)
  structured_data = true

  # SD elements
  structured_elements = [
    {
      sd_id = "flywall@1.0"
      sd_params = {
        version = "1.2.0"
        component = "firewall"
        zone = "WAN"
        rule_id = "100"
      }
    },
    {
      sd_id = "auth@1.0"
      sd_params = {
        user = "admin"
        method = "api_key"
        source_ip = "192.168.1.100"
      }
    }
  ]

  # JSON structured logging
  json_format = {
    enabled = true
    pretty_print = false
    include_metadata = true
  }
}
```

## Implementation Details

### Syslog Message Format (RFC5424)
```
<PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
```

Example:
```
<34>1 2023-12-01T10:30:45.123Z firewall-01 flywall 1234 - [flywall@1.0 version="1.2.0" component="firewall" zone="WAN"] Blocked connection from 192.168.1.100 to 8.8.8.8:53
```

### Log Processing Flow
1. Component generates log event
2. Log collector receives event
3. Filter engine evaluates routing rules
4. Formatter converts to syslog format
5. Transport manager delivers to server
6. Buffer manager handles failures

## Testing

### Syslog Testing
```bash
# Test syslog connectivity
nc -u syslog.example.com 514

# Send test message
logger -n syslog.example.com -P 514 -p local0.info "Test message from flywall"

# Monitor syslog server
tail -f /var/log/syslog | grep flywall

# Test TLS connection
openssl s_client -connect syslog.example.com:6514 -cert client.crt -key client.key
```

### Integration Tests
- `syslog_test.sh`: Basic syslog forwarding
- `syslog_tls_test.sh`: TLS encrypted syslog
- `syslog_buffer_test.sh`: Buffering and retry

## API Integration

### Syslog Management API
```bash
# Get syslog status
curl -s "http://localhost:8080/api/syslog/status"

# Get syslog configuration
curl -s "http://localhost:8080/api/syslog/config"

# Update syslog configuration
curl -X PUT "http://localhost:8080/api/syslog/config" \
  -H "Content-Type: application/json" \
  -d '{
    "servers": [...]
  }'

# Test syslog connectivity
curl -X POST "http://localhost:8080/api/syslog/test"

# Get buffer status
curl -s "http://localhost:8080/api/syslog/buffer/status"
```

### Log Statistics API
```bash
# Get syslog statistics
curl -s "http://localhost:8080/api/syslog/stats"

# Get error statistics
curl -s "http://localhost:8080/api/syslog/stats/errors"

# Get buffer statistics
curl -s "http://localhost:8080/api/syslog/stats/buffer"

# Flush buffer
curl -X POST "http://localhost:8080/api/syslog/buffer/flush"
```

## Best Practices

1. **Reliability**
   - Use TCP or RELP for critical logs
   - Enable buffering for unreliable networks
   - Configure backup syslog servers
   - Monitor delivery status

2. **Security**
   - Use TLS for sensitive logs
   - Validate server certificates
   - Implement access controls
   - Encrypt logs at rest

3. **Performance**
   - Use appropriate buffer sizes
   - Monitor network bandwidth
   - Batch log transmissions
   - Filter unnecessary logs

4. **Compliance**
   - Use structured logging
   - Include audit trails
   - Meet retention requirements
   - Ensure log integrity

## Troubleshooting

### Common Issues
1. **Logs not arriving**: Check network connectivity and firewall
2. **Message format errors**: Verify RFC compliance
3. **TLS failures**: Check certificates and configuration
4. **Buffer overflow**: Increase buffer size or reduce log volume

### Debug Commands
```bash
# Check syslog status
flywall syslog status

# Test syslog server
nc -v syslog.example.com 514

# Monitor network traffic
tcpdump -i any port 514 -A

# Check buffer
flywall syslog buffer status
```

### Advanced Debugging
```bash
# Debug syslog with strace
strace -e sendto -p $(pidof flywall)

# Check TLS connection
openssl s_client -connect syslog.example.com:6514 -servername syslog.example.com

# Monitor buffer directory
ls -la /var/lib/flywall/syslog-buffer/

# Force flush buffer
flywall syslog flush
```

## Performance Considerations

- UDP has lowest overhead but no reliability
- TCP adds overhead but guarantees delivery
- TLS adds CPU overhead for encryption
- Buffering uses memory but improves reliability

## Security Considerations

- Syslog data may contain sensitive information
- Network transmission needs protection
- Validate log sources
- Implement log tampering detection

## Related Features

- [Logging](logging.md)
- [Alerting System](alerting.md)
- [Configuration Management](config-management.md)
- [API Reference](api-reference.md)
