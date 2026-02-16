# DNS Query Logging Implementation Guide

## Overview

Flywall provides comprehensive DNS query logging for:
- Query tracking and persistence
- Response logging
- Performance metrics
- Security analysis
- Compliance requirements

## Architecture

### Logging Components
1. **Query Logger**: Captures all DNS queries
2. **Response Logger**: Captures DNS responses
3. **Storage Backend**: Persistent storage of logs
4. **Query Processor**: Analyzes and enriches logs
5. **Retention Manager**: Manages log lifecycle

### Log Data
- Query timestamp
- Client IP
- Query domain
- Query type
- Response code
- Response data
- Processing time
- Server identity

## Configuration

### Basic DNS Logging Setup
```hcl
dns {
  enabled = true

  # Enable query logging
  query_log = true

  # Log file
  query_log_file = "/var/log/flywall/dns_queries.log"

  # Log format
  query_log_format = "json"
}
```

### Advanced DNS Logging Configuration
```hcl
dns {
  enabled = true

  # Query logging
  query_log = true
  query_log_file = "/var/log/flywall/dns_queries.log"

  # Log rotation
  query_log_rotation = {
    enabled = true
    max_size = "100MB"
    max_files = 10
    compress = true
  }

  # Log format
  query_log_format = "json"
  query_log_fields = [
    "timestamp",
    "client_ip",
    "query_name",
    "query_type",
    "response_code",
    "response_time",
    "server",
    "zone"
  ]

  # What to log
  log_queries = true
  log_responses = true
  log_cached = true
  log_blocked = true

  # Performance logging
  log_performance = true
  performance_threshold = "100ms"

  # Security logging
  log_security = true
  security_events = [
    "suspicious_queries",
    "excessive_queries",
    "blocked_domains",
    "dns_tunneling"
  ]
}
```

### Per-Zone DNS Logging
```hcl
zone "LAN" {
  interface = "eth1"

  dns {
    enabled = true

    # Zone-specific logging
    query_log = true
    query_log_file = "/var/log/flywall/dns_lan.log"

    # Log all queries for internal zone
    log_all_queries = true
    log_cached_responses = true

    # Anonymization for privacy
    anonymize_ips = false
    anonymize_domains = false
  }
}

zone "Guest" {
  interface = "eth2"

  dns {
    enabled = true

    # Privacy-focused logging
    query_log = true
    query_log_file = "/var/log/flywall/dns_guest.log"

    # Anonymize for privacy compliance
    anonymize_ips = true
    anonymize_domains = true

    # Only log suspicious activity
    log_only_suspicious = true
    suspicious_threshold = 100  # queries per minute
  }
}
```

### Database Logging
```hcl
dns {
  enabled = true

  # Database logging
  query_log_database = true
  query_log_table = "dns_queries"

  # Database configuration
  database = {
    type = "sqlite"
    path = "/var/lib/flywall/dns_logs.db"

    # Connection pool
    max_connections = 10
    connection_timeout = "5s"

    # Batch inserts
    batch_size = 100
    batch_timeout = "5s"
  }

  # Table schema
  table_schema = {
    timestamp = "INTEGER",
    client_ip = "TEXT",
    query_name = "TEXT",
    query_type = "TEXT",
    response_code = "INTEGER",
    response_data = "TEXT",
    response_time = "INTEGER",
    server = "TEXT",
    zone = "TEXT",
    cached = "BOOLEAN"
  }

  # Indexes
  indexes = [
    "CREATE INDEX idx_timestamp ON dns_queries(timestamp)",
    "CREATE INDEX idx_client_ip ON dns_queries(client_ip)",
    "CREATE INDEX idx_query_name ON dns_queries(query_name)",
    "CREATE INDEX idx_zone ON dns_queries(zone)"
  ]
}
```

### Remote Logging
```hcl
dns {
  enabled = true

  # Remote logging
  query_log_remote = true

  # Syslog configuration
  syslog = {
    enabled = true
    server = "syslog.example.com"
    port = 514
    protocol = "udp"
    facility = "local0"
    severity = "info"
  }

  # Fluentd configuration
  fluentd = {
    enabled = true
    server = "fluentd.example.com"
    port = 24224
    tag = "flywall.dns"
    format = "json"
  }

  # Elasticsearch configuration
  elasticsearch = {
    enabled = true
    servers = ["es1.example.com", "es2.example.com"]
    index = "flywall-dns-logs"
    type = "dns_query"
    template = "/etc/flywall/es-template.json"
  }
}
```

## Implementation Details

### Log Format
```json
{
  "timestamp": "2023-12-01T10:30:45.123Z",
  "client_ip": "192.168.1.100",
  "client_port": 54321,
  "query_name": "example.com",
  "query_type": "A",
  "query_class": "IN",
  "response_code": "NOERROR",
  "response_data": ["93.184.216.34"],
  "response_time": 15,
  "cached": false,
  "blocked": false,
  "server": "10.1.0.1",
  "zone": "LAN",
  "transport": "udp",
  "edns": {
    "version": 0,
    "udp_size": 4096,
    "flags": ["do"]
  }
}
```

### Query Processing Flow
1. Receive DNS query
2. Extract query metadata
3. Check cache/blocklist
4. Forward or respond
5. Log query details
6. Log response details
7. Update statistics

## Testing

### Query Logging Testing
```bash
# Generate test queries
dig @192.168.1.1 example.com
dig @192.168.1.1 google.com

# Check log file
tail -f /var/log/flywall/dns_queries.log

# Check database
sqlite3 /var/lib/flywall/dns_logs.db "SELECT * FROM dns_queries ORDER BY timestamp DESC LIMIT 10;"

# Monitor real-time
watch -n 1 'dig @192.168.1.1 test.com && tail -1 /var/log/flywall/dns_queries.log'
```

### Integration Tests
- `dns_querylog_test.sh`: Basic query logging
- `dns_performance_test.sh`: Performance logging
- `dns_security_test.sh`: Security event logging

## API Integration

### Query Log API
```bash
# Get recent queries
curl -s "http://localhost:8080/api/dns/queries"

# Get queries with filters
curl -s "http://localhost:8080/api/dns/queries?client=192.168.1.100&limit=100"

# Get query statistics
curl -s "http://localhost:8080/api/dns/queries/stats"

# Get top domains
curl -s "http://localhost:8080/api/dns/queries/top-domains"

# Get suspicious queries
curl -s "http://localhost:8080/api/dns/queries/suspicious"
```

### Log Management API
```bash
# Get log status
curl -s "http://localhost:8080/api/dns/logs/status"

# Rotate logs
curl -X POST "http://localhost:8080/api/dns/logs/rotate"

# Export logs
curl -s "http://localhost:8080/api/dns/logs/export?start=2023-12-01&end=2023-12-02" > dns_logs.json

# Clear old logs
curl -X DELETE "http://localhost:8080/api/dns/logs?older=30d"
```

## Best Practices

1. **Performance**
   - Use batch inserts for database
   - Compress old log files
   - Monitor disk usage
   - Use appropriate log levels

2. **Privacy**
   - Anonymize sensitive data
   - Follow GDPR requirements
   - Implement data retention policies
   - Secure log storage

3. **Security**
   - Monitor for DNS tunneling
   - Track suspicious patterns
   - Implement rate limiting
   - Validate log integrity

4. **Compliance**
   - Meet regulatory requirements
   - Maintain audit trails
   - Implement tamper protection
   - Regular log reviews

## Troubleshooting

### Common Issues
1. **Logs not appearing**: Check logging configuration
2. **High disk usage**: Adjust retention or compression
3. **Performance impact**: Reduce logging verbosity
4. **Missing fields**: Check log format configuration

### Debug Commands
```bash
# Check DNS logging status
flywall dns show logging

# Monitor log file
tail -f /var/log/flywall/dns_queries.log

# Check log rotation
logrotate -d /etc/logrotate.d/flywall-dns

# Test query generation
dig @localhost test-query-12345.com
```

### Advanced Debugging
```bash
# Check DNS server logs
journalctl -u flywall | grep dns

# Monitor DNS traffic
tcpdump -i any port 53 -vv

# Check database performance
sqlite3 /var/lib/flywall/dns_logs.db "EXPLAIN QUERY PLAN SELECT * FROM dns_queries WHERE timestamp > ?;"

# Verify remote logging
tcpdump -i any port 514 -A
```

## Performance Considerations

- Logging adds minimal overhead
- Database logging requires tuning
- Remote logging needs reliable network
- Compression reduces storage but increases CPU

## Security Considerations

- Logs contain sensitive information
- Implement access controls
- Encrypt logs at rest and in transit
- Monitor for log tampering

## Related Features

- [DNS Server](dns-server.md)
- [DNS Blocklists](dns-blocklists.md)
- [Analytics Engine](analytics-engine.md)
- [Syslog Integration](syslog.md)
