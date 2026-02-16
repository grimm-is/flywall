# Analytics Engine Implementation Guide

## Overview

Flywall provides a powerful analytics engine for:
- Traffic analysis and reporting
- Pattern detection
- Anomaly identification
- Capacity planning
- Security analytics

## Architecture

### Analytics Components
1. **Data Collector**: Gathers metrics and logs
2. **Processor**: Analyzes and correlates data
3. **Storage Engine**: Time-series database
4. **Query Engine**: Executes analytics queries
5. **Report Generator**: Creates reports and visualizations

### Data Sources
- Packet counters
- Flow records
- Connection tracking data
- DNS queries
- DHCP leases
- System metrics
- Security events

## Configuration

### Basic Analytics Setup
```hcl
# Analytics configuration
analytics {
  enabled = true

  # Data collection
  collection_interval = "30s"
  retention_period = "30d"

  # Storage
  storage = {
    type = "tsdb"
    path = "/var/lib/flywall/analytics"
  }
}
```

### Advanced Analytics Configuration
```hcl
analytics {
  enabled = true

  # Data collection
  collection_interval = "30s"
  batch_size = 1000
  compression = true

  # Data retention
  retention = {
    raw_data = "7d"
    aggregated_1m = "30d"
    aggregated_5m = "90d"
    aggregated_1h = "1y"
  }

  # Storage configuration
  storage = {
    type = "tsdb"
    path = "/var/lib/flywall/analytics"

    # Performance tuning
    cache_size = "1GB"
    write_buffer_size = "64MB"
    max_series = 1000000

    # Backup
    backup = {
      enabled = true
      interval = "24h"
      retention = "7d"
    }
  }

  # Analytics modules
  modules = {
    traffic_analysis = true
    security_analysis = true
    performance_analysis = true
    capacity_planning = true
  }

  # Query settings
  query = {
    timeout = "30s"
    max_results = 10000
    parallel_queries = 4
  }
}
```

### Traffic Analytics
```hcl
analytics {
  enabled = true

  # Traffic analysis
  traffic_analysis = {
    enabled = true

    # Metrics to collect
    metrics = [
      "bytes_per_second",
      "packets_per_second",
      "connections_per_second",
      "active_connections",
      "new_connections",
      "failed_connections"
    ]

    # Aggregation levels
    aggregation = [
      "interface",
      "zone",
      "protocol",
      "port",
      "src_ip",
      "dst_ip"
    ]

    # Top N analysis
    top_talkers = {
      enabled = true
      top_count = 100
      refresh_interval = "5m"
    }

    # Protocol analysis
    protocol_analysis = {
      enabled = true
      protocols = ["tcp", "udp", "icmp"]
      deep_packet_inspection = true
    }
  }
}
```

### Security Analytics
```hcl
analytics {
  enabled = true

  # Security analytics
  security_analysis = {
    enabled = true

    # Threat detection
    threat_detection = {
      port_scan_detection = true
      dos_detection = true
      ddos_detection = true
      brute_force_detection = true
      dns_tunneling_detection = true
    }

    # Anomaly detection
    anomaly_detection = {
      enabled = true
      baseline_period = "7d"
      sensitivity = "medium"
      alert_on_anomaly = true
    }

    # Pattern analysis
    pattern_analysis = {
      enabled = true
      time_window = "1h"
      min_occurrences = 10
      correlation_threshold = 0.8
    }

    # Security metrics
    security_metrics = [
      "blocked_connections",
      "threat_events",
      "anomaly_score",
      "risk_level"
    ]
  }
}
```

### Performance Analytics
```hcl
analytics {
  enabled = true

  # Performance analytics
  performance_analysis = {
    enabled = true

    # System metrics
    system_metrics = {
      cpu_usage = true
      memory_usage = true
      disk_usage = true
      network_usage = true
    }

    # Service metrics
    service_metrics = {
      dns_response_time = true
      dhcp_response_time = true
      api_response_time = true
      vpn_handshake_time = true
    }

    # Performance thresholds
    thresholds = {
      cpu_warning = 70
      cpu_critical = 90
      memory_warning = 80
      memory_critical = 95
      response_time_warning = "100ms"
      response_time_critical = "500ms"
    }

    # Capacity planning
    capacity_planning = {
      enabled = true
      forecast_period = "30d"
      growth_factor = 1.2
      alert_threshold = 80
    }
  }
}
```

## Implementation Details

### Data Model
```go
// Time series data point
type DataPoint struct {
    Timestamp time.Time
    Metric    string
    Value     float64
    Labels    map[string]string
}

// Aggregated data
type AggregatedData struct {
    TimeWindow time.Duration
    Metric     string
    Value      float64
    Count      uint64
    Min        float64
    Max        float64
    Avg        float64
    Labels     map[string]string
}

// Analytics query
type Query struct {
    Metric      string
    StartTime   time.Time
    EndTime     time.Time
    Aggregation string
    Filters     map[string]string
    GroupBy     []string
}
```

### Query Examples
```sql
-- Top talkers by bandwidth
SELECT src_ip, SUM(bytes) as total_bytes
FROM traffic_metrics
WHERE timestamp >= now() - 1h
GROUP BY src_ip
ORDER BY total_bytes DESC
LIMIT 10;

-- Protocol distribution
SELECT protocol, SUM(packets) as packet_count
FROM traffic_metrics
WHERE timestamp >= now() - 24h
GROUP BY protocol;

-- Anomaly detection
SELECT timestamp, anomaly_score
FROM security_metrics
WHERE metric = 'anomaly_score'
  AND timestamp >= now() - 1h
  AND anomaly_score > 0.8;
```

## Testing

### Analytics Testing
```bash
# Generate test traffic
iperf3 -c target -t 60

# Check analytics data
flywall analytics query "SELECT * FROM traffic_metrics LIMIT 10"

# Test aggregation
flywall analytics aggregate --metric bytes --window 5m

# Check reports
flywall analytics report --type traffic --period 1h
```

### Integration Tests
- `analytics_test.sh`: Basic analytics functionality
- `aggregation_test.sh`: Data aggregation
- `report_test.sh`: Report generation

## API Integration

### Analytics API
```bash
# Query analytics data
curl -s "http://localhost:8080/api/analytics/query" \
  -H "Content-Type: application/json" \
  -d '{
    "metric": "bytes_per_second",
    "start_time": "2023-12-01T00:00:00Z",
    "end_time": "2023-12-01T23:59:59Z",
    "aggregation": "avg",
    "group_by": ["interface"]
  }'

# Get top talkers
curl -s "http://localhost:8080/api/analytics/top-talkers"

# Get traffic summary
curl -s "http://localhost:8080/api/analytics/summary"

# Get security events
curl -s "http://localhost:8080/api/analytics/security/events"

# Get performance metrics
curl -s "http://localhost:8080/api/analytics/performance"
```

### Report API
```bash
# Generate report
curl -X POST "http://localhost:8080/api/analytics/reports" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "traffic",
    "period": "24h",
    "format": "pdf"
  }'

# List reports
curl -s "http://localhost:8080/api/analytics/reports"

# Download report
curl -s "http://localhost:8080/api/analytics/reports/123/download" > report.pdf
```

## Best Practices

1. **Data Collection**
   - Collect relevant metrics only
   - Use appropriate intervals
   - Consider storage costs
   - Validate data quality

2. **Performance**
   - Optimize queries with indexes
   - Use time-based partitioning
   - Cache frequent queries
   - Monitor query performance

3. **Security**
   - Secure analytics data
   - Implement access controls
   - Audit data access
   - Anonymize sensitive data

4. **Retention**
   - Define retention policies
   - Compress old data
   - Archive important data
   - Monitor storage usage

## Troubleshooting

### Common Issues
1. **Missing data**: Check collection configuration
2. **Slow queries**: Optimize or add indexes
3. **High memory usage**: Adjust cache settings
4. **Incorrect aggregations**: Verify time windows

### Debug Commands
```bash
# Check analytics status
flywall analytics status

# Check collection
flywall analytics collection status

# Test query
flywall analytics query "SELECT 1"

# Check storage
du -h /var/lib/flywall/analytics
```

### Advanced Debugging
```bash
# Debug query execution
flywall analytics query --debug "SELECT * FROM traffic_metrics"

# Check data ingestion
watch -n 1 'flywall analytics stats'

# Validate data
flywall analytics validate --start "2023-12-01" --end "2023-12-02"

# Compact storage
flywall analytics compact
```

## Performance Considerations

- Time-series databases optimize for time-based queries
- Aggregation reduces storage requirements
- Indexing improves query performance
- Compression saves space but uses CPU

## Security Considerations

- Analytics data may be sensitive
- Implement role-based access
- Encrypt data at rest
- Audit all access

## Related Features

- [Metrics Collection](metrics-collection.md)
- [Alerting System](alerting.md)
- [Security Analysis](security-analysis.md)
- [Reporting](reporting.md)
