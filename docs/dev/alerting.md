# Alerting System Implementation Guide

## Overview

Flywall provides a comprehensive alerting system for:
- Rule-based alert generation
- Multiple notification channels
- Alert aggregation and deduplication
- Historical alert tracking
- Integration with external monitoring

## Architecture

### Alert Components
1. **Rule Engine**: Evaluates conditions against metrics/events
2. **Alert Manager**: Manages alert lifecycle
3. **Notification Engine**: Sends alerts via various channels
4. **Storage**: Persistent alert history
5. **API**: Alert management interface

### Alert Flow
1. Event/Metric → Rule Engine
2. Rule Evaluation → Alert Generation
3. Alert Manager → Deduplication
4. Notification Engine → Send
5. Storage → History

## Configuration

### Basic Alerting Setup
```hcl
# Enable alerting
notifications {
  enabled = true

  # Default settings
  default_severity = "warning"
  default_timeout = "5m"

  # Global settings
  max_alerts_per_minute = 100
  deduplication_window = "1h"
}

# Simple alert rule
alert "high_cpu" {
  description = "High CPU usage detected"

  condition = "system_cpu_usage > 80"

  for = "5m"
  severity = "warning"

  labels = {
    service = "firewall"
    component = "cpu"
  }

  annotations = {
    summary = "CPU usage is {{ $value }}%"
    description = "CPU usage has been above 80% for 5 minutes"
  }
}
```

### Advanced Alert Rules
```hcl
# Complex alert with multiple conditions
alert "wan_link_down" {
  description = "WAN link failure"

  # Multiple conditions
  condition = "all(
    interface_link_status{interface=\"eth0\"} == 0,
    interface_rx_packets{interface=\"eth0\"} == 0
  )"

  for = "30s"
  severity = "critical"

  # Escalation
  escalation {
    after = "5m"
    severity = "critical"
    annotations = {
      summary = "WAN link still down after 5 minutes!"
    }
  }

  labels = {
    interface = "eth0"
    zone = "wan"
  }

  annotations = {
    runbook = "https://wiki.example.com/wan-failure"
    contact = "network-team@example.com"
  }
}

# Rate-based alert
alert "excessive_drops" {
  description = "High packet drop rate"

  condition = "rate(firewall_drops_total[5m]) > 1000"

  for = "2m"
  severity = "warning"

  # Dynamic severity
  severity_map = {
    "0-1000" = "info"
    "1000-5000" = "warning"
    "5000-" = "critical"
  }

  labels = {
    type = "security"
  }
}

# Threshold alert with prediction
alert "disk_space" {
  description = "Disk space running low"

  condition = "predict_linear(
    system_disk_usage_bytes{device=\"/var\"}[1h],
    24*3600
  ) > system_disk_capacity_bytes{device=\"/var\"} * 0.9"

  for = "10m"
  severity = "warning"

  annotations = {
    summary = "Disk will be full in less than 24 hours"
    current_usage = "{{ $value | humanizePercentage }}"
  }
}
```

### Notification Channels
```hcl
# Email notifications
notification "email" {
  type = "email"

  smtp {
    host = "smtp.example.com"
    port = 587
    username = "flywall@example.com"
    password = "secret"
    from = "flywall@example.com"
  }

  recipients = ["admin@example.com", "ops@example.com"]

  # Template
  template = "/etc/flywall/alerts/email.tmpl"

  # Conditions
  when = ["severity == 'critical'", "severity == 'warning'"]
}

# Webhook notifications
notification "slack" {
  type = "webhook"

  webhook {
    url = "https://hooks.slack.com/services/..."
    method = "POST"
    headers = {
      "Content-Type" = "application/json"
    }
  }

  # Custom payload
  template = "/etc/flywall/alerts/slack.tmpl"

  # Rate limiting
  rate_limit = {
    max_per_minute = 10
    burst = 20
  }
}

# PagerDuty notifications
notification "pagerduty" {
  type = "pagerduty"

  pagerduty {
    integration_key = "pagerduty-key"
    severity_map = {
      "critical" = "critical"
      "warning" = "warning"
      "info" = "info"
    }
  }

  # Deduplication key
  dedup_key = "{{ .labels.alertname }}-{{ .labels.interface }}"
}
```

### Alert Groups and Routing
```hcl
# Alert grouping
alert_group "security" {
  name = "Security Alerts"

  # Group by labels
  group_by = ["alertname", "zone"]

  # Wait time for grouping
  group_wait = "30s"
  group_interval = "5m"
  repeat_interval = "12h"

  # Routing rules
  routes {
    # Critical alerts go to PagerDuty
    match = {
      severity = "critical"
    }
    receiver = "pagerduty"

    # Security alerts go to security team
    match = {
      type = "security"
    }
    receiver = "security_team"

    # Default
    receiver = "default"
  }
}

# Route with inhibition
inhibit_rule {
  source_match = {
    alertname = "NodeDown"
  }
  target_match = {
    alertname = "HighLatency"
  }

  equal = ["instance"]
}
```

### Alert Templates
```go
{{/* Email template */}}
To: {{ .Recipients }}
Subject: [{{ .Status | toUpper }}] {{ .GroupLabels.alertname }}

{{ range .Alerts }}
Alert: {{ .Annotations.summary }}
Description: {{ .Annotations.description }}
Severity: {{ .Labels.severity }}
Time: {{ .StartsAt.Format "2006-01-02 15:04:05" }}

{{- if .RunsURL }}
Runbook: {{ .RunsURL }}
{{- end }}

{{ end }}

--
Flywall Alerting System
```

## Implementation Details

### Alert States
1. **Pending**: Condition met, waiting for duration
2. **Firing**: Alert active
3. **Resolved**: Condition no longer met
4. **Silenced**: Temporarily suppressed

### Rule Evaluation
- Continuous evaluation
- Configurable intervals
- Parallel rule execution
- Efficient query optimization

## Testing

### Integration Tests
- `alerting_test.sh`: Basic alert CRUD
- `notification_test.sh`: Notification delivery
- `alert_aggregation_test.sh`: Alert grouping

### Manual Testing
```bash
# Trigger test alert
curl -X POST "http://localhost:8080/api/alerts/test" \
  -H "Content-Type: application/json" \
  -d '{
    "alertname": "test",
    "severity": "warning",
    "description": "Test alert"
  }'

# Check alert status
curl -s "http://localhost:8080/api/alerts"

# Get alert history
curl -s "http://localhost:8080/api/alerts/history"
```

## API Integration

### Alert Management API
```bash
# List active alerts
curl -s "http://localhost:8080/api/alerts"

# Get specific alert
curl -s "http://localhost:8080/api/alerts/123"

# Create alert rule
curl -X POST "http://localhost:8080/api/alerts/rules" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test_rule",
    "condition": "cpu > 80",
    "severity": "warning"
  }'

# Update rule
curl -X PUT "http://localhost:8080/api/alerts/rules/test_rule" \
  -H "Content-Type: application/json" \
  -d '{
    "condition": "cpu > 90"
  }'

# Delete rule
curl -X DELETE "http://localhost:8080/api/alerts/rules/test_rule"

# Silence alert
curl -X POST "http://localhost:8080/api/alerts/123/silence" \
  -H "Content-Type: application/json" \
  -d '{
    "duration": "1h",
    "comment": "Maintenance window"
  }'
```

### Notification API
```bash
# List notification channels
curl -s "http://localhost:8080/api/alerts/notifications"

# Test notification
curl -X POST "http://localhost:8080/api/alerts/notifications/test" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "email",
    "message": "Test notification"
  }'

# Get notification history
curl -s "http://localhost:8080/api/alerts/notifications/history"
```

## Best Practices

1. **Rule Design**
   - Keep conditions simple
   - Use appropriate thresholds
   - Avoid alert fatigue
   - Document runbooks

2. **Notification Strategy**
   - Escalate appropriately
   - Use different channels
   - Respect quiet hours
   - Group related alerts

3. **Performance**
   - Optimize queries
   - Cache expensive calculations
   - Limit rule complexity
   - Monitor evaluation time

4. **Maintenance**
   - Regular rule review
   - Update thresholds
   - Test notifications
   - Monitor alert volume

## Troubleshooting

### Common Issues
1. **Alerts not firing**: Check rule syntax
2. **Duplicate alerts**: Check deduplication
3. **Missing notifications**: Verify channel config
4. **High CPU usage**: Optimize rules

### Debug Commands
```bash
# Check alert rules
flywall alerts rules list

# Test rule evaluation
flywall alerts rules test high_cpu

# Check notification status
flywall alerts notifications status

# View alert history
flywall alerts history --last 1h
```

### Advanced Debugging
```bash
# Debug rule evaluation
flywall alerts rules debug --rule high_cpu --verbose

# Check evaluation metrics
curl -s "http://localhost:8080/api/alerts/metrics"

# Monitor alert queue
flywall alerts queue status

# Force rule evaluation
curl -X POST "http://localhost:8080/api/alerts/rules/evaluate"
```

## Performance Considerations

- Rule evaluation scales with rule count
- Efficient query optimization critical
- Alert aggregation reduces noise
- Notification rate limiting prevents spam

## Security Considerations

- Secure notification credentials
- Limit alert data exposure
- Audit alert access
- Validate rule conditions

## Related Features

- [Metrics Collection](metrics-collection.md)
- [Analytics Engine](analytics-engine.md)
- [Syslog Integration](syslog.md)
- [API Reference](api-reference.md)
