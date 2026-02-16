# Metrics Collection Implementation Guide

## Overview

Flywall provides comprehensive metrics collection compatible with Prometheus:
- Packet and byte counters
- Connection tracking metrics
- Performance indicators
- Alert-ready metrics
- Custom metrics support

## Architecture

### Metrics Types
1. **Counters**: Monotonically increasing values
2. **Gauges**: Values that can go up or down
3. **Histograms**: Value distributions
4. **Summaries**: quantile calculations

### Metrics Sources
1. **nftables counters**: Packet/byte counts
2. **Connection tracking**: Active connections
3. **System metrics**: CPU, memory, disk
4. **Application metrics**: DHCP leases, DNS queries

## Configuration

### Basic Metrics Setup
```hcl
# Enable metrics endpoint
metrics {
  enabled = true
  listen = "0.0.0.0:9090"
  path = "/metrics"

  # Collection interval
  collect_interval = "30s"

  # Expose system metrics
  system_metrics = true

  # Expose process metrics
  process_metrics = true
}
```

### Advanced Metrics Configuration
```hcl
metrics {
  enabled = true
  listen = "0.0.0.0:9090"
  path = "/metrics"

  # Authentication
  auth = {
    enabled = true
    username = "prometheus"
    password = "secret"
  }

  # TLS
  tls = {
    enabled = true
    cert_file = "/etc/flywall/metrics.crt"
    key_file = "/etc/flywall/metrics.key"
  }

  # Collection settings
  collect_interval = "15s"
  timeout = "5s"

  # Metrics to collect
  include = [
    "firewall_*",
    "system_*",
    "dhcp_*",
    "dns_*",
    "vpn_*"
  ]

  exclude = [
    "debug_*"
  ]

  # Custom labels
  labels = {
    datacenter = "dc1",
    cluster = "firewall"
  }

  # Export format
  format = "prometheus"

  # Compression
  compression = true
}
```

### Custom Metrics
```hcl
# Define custom metrics
metrics {
  enabled = true

  # Custom counters
  custom_metrics {
    blocked_connections {
      type = "counter"
      help = "Number of blocked connections"
      labels = ["zone", "protocol"]
    }

    dns_queries {
      type = "counter"
      help = "DNS queries processed"
      labels = ["query_type", "response_code"]
    }

    active_vpn_clients {
      type = "gauge"
      help = "Number of active VPN clients"
      labels = ["vpn_type"]
    }

    packet_processing_time {
      type = "histogram"
      help = "Packet processing time distribution"
      buckets = [0.001, 0.01, 0.1, 1.0, 10.0]
    }
  }
}
```

### Per-Zone Metrics
```hcl
zone "WAN" {
  interface = "eth0"

  # Zone-specific metrics
  metrics = {
    enabled = true
    track_packets = true
    track_bytes = true
    track_connections = true
    track_drops = true

    # Custom zone metrics
    custom = {
      threat_blocks = true
      rate_limit_drops = true
    }
  }
}

zone "LAN" {
  interface = "eth1"

  metrics = {
    enabled = true
    track_packets = true
    track_bytes = false  # Skip for performance

    # Reduced collection for internal zone
    collect_interval = "60s"
  }
}
```

### Service Metrics
```hcl
# DHCP metrics
dhcp {
  enabled = true

  metrics = {
    enabled = true
    track_leases = true
    track_discover = true
    track_offer = true
    track_request = true
    track_release = true
  }
}

# DNS metrics
dns {
  enabled = true

  metrics = {
    enabled = true
    track_queries = true
    track_cache_hits = true
    track_cache_misses = true
    track_blocklist_hits = true
    track_response_time = true
  }
}

# VPN metrics
wireguard "wg0" {
  enabled = true

  metrics = {
    enabled = true
    track_handshakes = true
    track_bytes = true
    track_peers = true
    track_errors = true
  }
}
```

## Implementation Details

### Default Metrics
```prometheus
# Firewall metrics
firewall_packets_total{interface, direction, action}
firewall_bytes_total{interface, direction, action}
firewall_connections_active{zone, protocol}
firewall_connections_new{zone, protocol}
firewall_drops_total{zone, reason}

# System metrics
system_cpu_usage_percent
system_memory_usage_bytes
system_disk_usage_bytes{device}
system_network_bytes_total{interface, direction}
system_load_average{period}

# Application metrics
flywall_uptime_seconds
flywall_config_hash
flywall_version_info{version, build}
```

### Metrics Collection Process
1. Collect from nftables counters
2. Query system statistics
3. Aggregate application metrics
4. Apply labels and filters
5. Export in Prometheus format

## Testing

### Integration Tests
- `metrics_test.sh`: Prometheus format validation
- `metrics_endpoint_test.sh`: Metrics endpoint
- `metrics_collection_test.sh`: Data collection

### Manual Testing
```bash
# Get metrics
curl -s http://localhost:9090/metrics

# Check specific metric
curl -s http://localhost:9090/metrics | grep firewall_packets

# Test with labels
curl -s 'http://localhost:9090/metrics' | grep 'firewall_packets_total{interface="eth0"'

# Verify Prometheus format
curl -s http://localhost:9090/metrics | promtool check metrics
```

## API Integration

### Metrics API
```bash
# Get all metrics
curl -s "http://localhost:8080/api/metrics"

# Get metric summary
curl -s "http://localhost:8080/api/metrics/summary"

# Get specific metric
curl -s "http://localhost:8080/api/metrics/firewall_packets_total"

# Query metrics with filters
curl -s "http://localhost:8080/api/metrics?name=firewall_*&interface=eth0"

# Get metric labels
curl -s "http://localhost:8080/api/metrics/labels"
```

### Metrics Configuration API
```bash
# Update metrics config
curl -X PUT "http://localhost:8080/api/metrics/config" \
  -H "Content-Type: application/json" \
  -d '{
    "collect_interval": "10s",
    "include": ["firewall_*", "system_*"]
  }'

# Get metrics status
curl -s "http://localhost:8080/api/metrics/status"

# Reset metrics
curl -X POST "http://localhost:8080/api/metrics/reset"
```

## Prometheus Configuration

### Prometheus Server Config
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'flywall'
    static_configs:
      - targets: ['firewall1:9090', 'firewall2:9090']

    metrics_path: /metrics
    scrape_interval: 30s

    # Authentication
    basic_auth:
      username: prometheus
      password: secret

    # TLS
    scheme: https
    tls_config:
      insecure_skip_verify: false
      cert_file: /etc/prometheus/firewall.crt

    # Relabeling
    relabel_configs:
      - source_labels: [__address__]
        target_label: instance
        replacement: 'firewall-${1}'
```

### Grafana Dashboard
```json
{
  "dashboard": {
    "panels": [
      {
        "title": "Firewall Throughput",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(firewall_bytes_total[5m])",
            "legendFormat": "{{interface}}-{{direction}}"
          }
        ]
      },
      {
        "title": "Active Connections",
        "type": "singlestat",
        "targets": [
          {
            "expr": "sum(firewall_connections_active)",
            "legendFormat": "Total Connections"
          }
        ]
      }
    ]
  }
}
```

## Best Practices

1. **Performance**
   - Adjust collection intervals
   - Filter unnecessary metrics
   - Use efficient queries
   - Monitor metric overhead

2. **Label Design**
   - Use low-cardinality labels
   - Consistent naming conventions
   - Avoid dynamic values
   - Document label meanings

3. **Storage Planning**
   - Estimate retention needs
   - Plan storage capacity
   - Configure compression
   - Monitor disk usage

4. **Query Optimization**
   - Use recording rules
   - Pre-aggregate data
   - Optimize PromQL
   - Cache frequent queries

## Troubleshooting

### Common Issues
1. **Metrics not updating**: Check collection interval
2. **High memory usage**: Reduce metric count
3. **Slow queries**: Optimize PromQL
4. **Missing labels**: Check configuration

### Debug Commands
```bash
# Check metrics endpoint
curl -v http://localhost:9090/metrics

# Validate format
curl -s http://localhost:9090/metrics | promtool check metrics

# Check collection status
flywall metrics status

# Monitor performance
top -p $(pgrep flywall)
```

### Advanced Debugging
```bash
# Debug specific metric
curl -s http://localhost:9090/metrics | debug-prometheus

# Check nftables counters
nft list counters

# Monitor collection
watch -n 5 'curl -s http://localhost:9090/metrics | wc -l'
```

## Performance Considerations

- Metrics collection uses minimal CPU
- Memory scales with metric count
- Network I/O from scraping
- Storage needs planning

## Security Considerations

- Restrict metrics access
- Use authentication
- Enable TLS for external access
- Monitor for scraping abuse

## Related Features

- [Analytics Engine](analytics-engine.md)
- [Alerting System](alerting.md)
- [Monitoring](monitoring.md)
- [API Reference](api-reference.md)
