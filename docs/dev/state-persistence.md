# State Persistence Implementation Guide

## Overview

Flywall provides robust state persistence for:
- DHCP leases
- DNS cache and records
- Connection tracking data
- Learning engine state
- Configuration history
- System metrics

**Note**: For information about replicating state between nodes, see [State Replication](state-replication.md). For HA-specific persistence considerations, see [HA Configuration](ha-configuration.md).

## Architecture

### Persistence Components
1. **State Manager**: Coordinates all persistence operations
2. **Storage Backend**: SQLite database with WAL mode
3. **Serializer**: Converts internal state to storage format
4. **Recovery Manager**: Handles crash recovery
5. **Backup Manager**: Manages state backups

### Storage Location
- Primary: `/var/lib/flywall/state.db`
- Backups: `/var/lib/flywall/backups/`
- Temporary: `/tmp/flywall/`
- Runtime: `/run/flywall/`

## Configuration

### Basic Persistence Setup
```hcl
# State persistence configuration
state {
  # Storage location
  path = "/var/lib/flywall/state.db"

  # Backup settings
  backup {
    enabled = true
    directory = "/var/lib/flywall/backups"
    interval = "1h"
    retention = "7d"
  }

  # Persistence options
  auto_save = true
  save_interval = "5m"
  compression = true
}
```

### Advanced Persistence Configuration
```hcl
state {
  path = "/var/lib/flywall/state.db"

  # Database settings
  database {
    journal_mode = "WAL"
    synchronous = "NORMAL"
    cache_size = 10000
    temp_store = "memory"
    mmap_size = "256MB"
  }

  # What to persist
  persistence {
    dhcp_leases = true
    dns_cache = true
    dns_records = true
    conntrack = false  # Usually not persisted
    learning_state = true
    metrics_history = true
    configuration = true
  }

  # Backup configuration
  backup {
    enabled = true
    directory = "/var/lib/flywall/backups"
    interval = "1h"
    retention = {
      hourly = 24
      daily = 7
      weekly = 4
      monthly = 12
    }

    # Compression
    compression = true
    compression_level = 6

    # Encryption
    encryption = true
    encryption_key = "/etc/flywall/backup.key"
  }

  # Recovery settings
  recovery {
    auto_recover = true
    validate_on_start = true
    max_recovery_attempts = 3
    recovery_timeout = "30s"
  }

  # Sync settings
  sync {
    mode = "full"  # full, incremental
    batch_size = 1000
    sync_timeout = "10s"
  }
}
```

### Per-Feature Persistence
```hcl
# DHCP persistence
dhcp {
  enabled = true

  persistence {
    enabled = true
    save_leases = true
    save_options = true
    save_reservations = true

    # Lease persistence across restarts
    persist_lease_time = true

    # Cleanup expired leases
    cleanup_expired = true
    cleanup_interval = "1h"
  }
}

# DNS persistence
dns {
  enabled = true

  persistence {
    enabled = true
    save_cache = true
    save_records = true
    save_blocklists = false

    # Cache persistence
    cache_ttl = "1d"
    max_cache_size = 100000

    # Record persistence
    persist_dynamic_records = true
    persist_statistics = true
  }
}

# Learning engine persistence
rule_learning {
  enabled = true

  persistence {
    enabled = true
    save_flows = true
    save_suggestions = true
    save_statistics = true

    # Data retention
    flow_retention = "7d"
    suggestion_retention = "30d"
    statistics_retention = "90d"

    # Cleanup
    cleanup_interval = "24h"
  }
}
```

## Implementation Details

### Database Schema
```sql
-- DNS records
CREATE TABLE dns_records (
  name TEXT,
  type TEXT,
  value TEXT,
  ttl INTEGER,
  zone TEXT,
  created_at INTEGER,
  updated_at INTEGER,
  PRIMARY KEY (name, type, zone)
);

-- Learning flows
CREATE TABLE learning_flows (
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
  last_seen INTEGER
);

-- Configuration history
CREATE TABLE config_history (
  id INTEGER PRIMARY KEY,
  config_hash TEXT,
  config_data TEXT,
  applied_at INTEGER,
  description TEXT
);
```

### Persistence Process
1. **Collection**: Gather state from components
2. **Serialization**: Convert to storage format
3. **Transaction**: Begin database transaction
4. **Storage**: Write to database
5. **Commit**: Commit transaction
6. **Cleanup**: Remove expired data

### Recovery Process
1. **Detection**: Identify crash or inconsistency
2. **Validation**: Check database integrity
3. **Repair**: Fix minor issues automatically
4. **Restore**: Load last known good state
5. **Rebuild**: Reconstruct transient state

## Testing

### Integration Tests
- `state_persistence_test.sh`: DHCP lease persistence
- `persistence_test.sh`: General persistence
- `recovery_test.sh`: Crash recovery
- `backup_test.sh`: Backup/restore

### Manual Testing
```bash
# Check state database
sqlite3 /var/lib/flywall/state.db ".tables"

# View DHCP leases
sqlite3 /var/lib/flywall/state.db "SELECT * FROM dhcp_leases;"

# Check database integrity
sqlite3 /var/lib/flywall/state.db "PRAGMA integrity_check;"

# Create backup
flywall state backup create

# Restore backup
flywall state backup restore backup-20231201-120000.db
```

## API Integration

### State Management API
```bash
# Get state status
curl -s "http://localhost:8080/api/state/status"

# Get persistence statistics
curl -s "http://localhost:8080/api/state/stats"

# Force state save
curl -X POST "http://localhost:8080/api/state/save"

# Validate state
curl -X POST "http://localhost:8080/api/state/validate"

# Compact database
curl -X POST "http://localhost:8080/api/state/compact"
```

### Backup API
```bash
# List backups
curl -s "http://localhost:8080/api/state/backups"

# Create backup
curl -X POST "http://localhost:8080/api/state/backups" \
  -H "Content-Type: application/json" \
  -d '{"description": "Before upgrade"}'

# Download backup
curl -s "http://localhost:8080/api/state/backups/backup-20231201.db" > backup.db

# Restore backup
curl -X POST "http://localhost:8080/api/state/backups/backup-20231201.db/restore"
```

## Best Practices

1. **Storage Management**
   - Monitor disk usage
   - Regular cleanup of old data
   - Adequate space for backups
   - Use SSD for better performance

2. **Backup Strategy**
   - Regular automated backups
   - Offsite backup storage
   - Test restore procedures
   - Encrypt sensitive backups

3. **Performance**
   - Optimize database settings
   - Use appropriate batch sizes
   - Monitor I/O performance
   - Consider memory mapping

4. **Reliability**
   - Validate data integrity
   - Test recovery procedures
   - Monitor for corruption
   - Keep multiple backups

## Troubleshooting

### Common Issues
1. **Database locked**: Check for hanging transactions
2. **Corruption detected**: Run integrity check
3. **Out of space**: Clean up old data
4. **Slow performance**: Optimize database settings

### Debug Commands
```bash
# Check database status
flywall state status

# Validate database
flywall state validate

# Check database size
du -h /var/lib/flywall/state.db

# Monitor I/O
iotop -p $(pgrep flywall)

# Check locks
lsof /var/lib/flywall/state.db
```

### Advanced Debugging
```bash
# Database analysis
sqlite3 /var/lib/flywall/state.db "ANALYZE;"

# Check query plans
sqlite3 /var/lib/flywall/state.db "EXPLAIN QUERY PLAN SELECT * FROM dhcp_leases;"

# Vacuum database
sqlite3 /var/lib/flywall/state.db "VACUUM;"

# Check WAL mode
sqlite3 /var/lib/flywall/state.db "PRAGMA journal_mode;"
```

## Performance Considerations

- SQLite with WAL provides good performance
- Compression reduces storage but increases CPU
- Batch operations improve throughput
- Memory mapping speeds up access

## Security Considerations

- Secure database permissions
- Encrypt sensitive backups
- Audit state access
- Validate input data

## Related Features

- [Configuration Management](config-management.md)
- [HA Configuration](ha-configuration.md)
- [Schema Migration](schema-migration.md)
- [CLI Tools](cli-tools.md)
