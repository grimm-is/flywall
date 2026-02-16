# Learning Engine Implementation Guide

## Overview

The Flywall Learning Engine automatically learns network traffic patterns and suggests firewall rules. It captures flow information from dropped packets and provides recommendations for rule creation.

## Architecture

### Components
1. **Flow Capture**: Collects packet metadata via NFLOG/NFQueue
2. **Flow Database**: Stores flow information in SQLite
3. **Analysis Engine**: Identifies patterns and suggests rules
4. **Rule Generator**: Creates suggested firewall rules
5. **API Interface**: Exposes learning data and suggestions

### Learning Modes
- **Learning Mode**: Captures all traffic without blocking
- **Inline Mode**: Synchronous packet inspection with verdict
- **Monitoring Mode**: Passive observation only

## Configuration

### Basic Learning Configuration
```hcl
rule_learning {
  enabled = true
  learning_mode = true
  log_group = 100

  # Flow database settings
  flow_db_path = "/opt/flywall/var/lib/learning.db"
  max_flows = 100000

  # Learning parameters
  min_packets = 5
  confidence_threshold = 0.8

  # Auto-rule generation
  auto_suggest = true
  min_confidence = 0.9
}
```

### Inline IPS Mode
```hcl
rule_learning {
  enabled = true
  inline_mode = true

  # Packet window before offload
  packet_window = 10

  # Conntrack mark for offloaded flows
  offload_mark = "2097152"  # 0x200000

  # Learning parameters
  min_packets = 3
  trusted_threshold = 0.95
}
```

### Zone-Based Learning
```hcl
zone "Green" {
  interface = "eth1"
  services {
    learning = true
  }
}

zone "Orange" {
  interface = "eth2"
  services {
    learning = true
  }
}

# Learning policies
policy "Green" "Orange" {
  name = "green_to_orange_learning"

  # Default to learning mode
  rule "default_learn" {
    description = "Learn all traffic"
    action = "learn"
  }
}
```

### Advanced Learning Configuration
```hcl
rule_learning {
  enabled = true
  learning_mode = true

  # Per-protocol learning
  protocols {
    tcp = {
      enabled = true
      min_packets = 3
      connection_timeout = "5m"
    }

    udp = {
      enabled = true
      min_packets = 10
      flow_timeout = "30s"
    }

    icmp = {
      enabled = false  # Skip ICMP
    }
  }

  # Learning filters
  filters {
    # Skip local traffic
    skip_local = true

    # Skip specific ports
    skip_ports = [53, 67, 68]

    # Only learn specific zones
    zones = ["Green", "Orange"]
  }

  # Rule suggestions
  suggestions {
    # Group similar flows
    group_by_port = true

    # Group by subnet
    group_by_subnet = true

    # Minimum flows for suggestion
    min_flows = 10
  }
}
```

## Implementation Details

### Flow Capture Process
1. Packet hits firewall rule (usually drop)
2. Metadata sent via NFLOG to learning engine
3. Flow information stored in database
4. Pattern analysis runs periodically
5. Rules suggested based on patterns

### Flow Database Schema
```sql
-- Flows table
CREATE TABLE flows (
  id INTEGER PRIMARY KEY,
  src_ip TEXT,
  dst_ip TEXT,
  src_port INTEGER,
  dst_port INTEGER,
  protocol TEXT,
  zone_src TEXT,
  zone_dst TEXT,
  packet_count INTEGER,
  byte_count INTEGER,
  first_seen INTEGER,
  last_seen INTEGER,
  status TEXT
);

-- Suggestions table
CREATE TABLE suggestions (
  id INTEGER PRIMARY KEY,
  src_zone TEXT,
  dst_zone TEXT,
  protocol TEXT,
  dst_port INTEGER,
  action TEXT,
  confidence REAL,
  flow_count INTEGER,
  created_at INTEGER
);
```

### Learning Algorithm
1. **Flow Aggregation**: Group similar flows
2. **Pattern Detection**: Identify recurring patterns
3. **Confidence Calculation**: Based on frequency and consistency
4. **Rule Generation**: Create firewall rule suggestions
5. **Validation**: Check for conflicts and security issues

## Testing

### Integration Tests
- `learning_traffic_test.sh`: Basic flow capture
- `network_learning_test.sh`: Network-based learning
- `inline_ips_test.sh`: Inline IPS mode

### Manual Testing
```bash
# Generate some traffic
ping 8.8.8.8
curl http://example.com

# Check learned flows
flywall learning flows list

# View suggestions
flywall learning suggestions list

# Apply suggestion
flywall learning suggestions apply 123
```

## API Integration

### Learning API Endpoints
```bash
# Get all flows
curl -s "http://localhost:8080/api/learning/flows"

# Get flows by zone
curl -s "http://localhost:8080/api/learning/flows?src_zone=Green"

# Get suggestions
curl -s "http://localhost:8080/api/learning/suggestions"

# Get suggestion details
curl -s "http://localhost:8080/api/learning/suggestions/123"

# Apply suggestion
curl -X POST "http://localhost:8080/api/learning/suggestions/123/apply"

# Reject suggestion
curl -X POST "http://localhost:8080/api/learning/suggestions/123/reject"
```

### Flow Statistics
```bash
# Get learning stats
curl -s "http://localhost:8080/api/learning/stats"

# Get top talkers
curl -s "http://localhost:8080/api/learning/top-talkers"

# Get protocol distribution
curl -s "http://localhost:8080/api/learning/protocols"
```

## Best Practices

1. **Learning Duration**
   - Run learning mode for sufficient time
   - Consider business cycles and patterns
   - Avoid learning during maintenance windows

2. **Rule Review**
   - Always review suggested rules
   - Check for overly permissive rules
   - Validate security implications

3. **Database Management**
   - Regular cleanup of old flows
   - Backup learning database
   - Monitor database size

4. **Performance**
   - Limit learning to high-traffic zones
   - Adjust flow retention periods
   - Monitor CPU/memory usage

## Troubleshooting

### Common Issues
1. **No flows captured**: Check NFLOG configuration
2. **High memory usage**: Reduce flow retention
3. **No suggestions**: Check confidence thresholds

### Debug Commands
```bash
# Check learning status
flywall show learning

# View learning logs
journalctl -u flywall | grep learning

# Check flow database
sqlite3 /opt/flywall/var/lib/learning.db "SELECT COUNT(*) FROM flows"

# Monitor NFLOG
tcpdump -i any -n 'ip proto 2'
```

### Database Queries
```sql
-- View recent flows
SELECT * FROM flows ORDER BY last_seen DESC LIMIT 10;

-- View top flows by packet count
SELECT src_ip, dst_ip, dst_port, SUM(packet_count)
FROM flows
GROUP BY src_ip, dst_ip, dst_port
ORDER BY SUM(packet_count) DESC;

-- View high-confidence suggestions
SELECT * FROM suggestions
WHERE confidence > 0.9
ORDER BY confidence DESC;
```

## Performance Considerations

- Flow capture adds minimal overhead
- Database size grows with traffic volume
- Analysis runs in background
- Consider SSD for flow database

## Security Considerations

- Learning mode doesn't block traffic
- Review all suggested rules
- Consider privacy implications
- Secure flow database access

## Related Features

- [Inline IPS](inline-ips.md)
- [Zone Policies](zones-policies.md)
- [API Reference](api-reference.md)
- [Monitoring](monitoring.md)
