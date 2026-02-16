# DHCP Lease Management Implementation Guide

## Overview

Flywall provides comprehensive DHCP lease management for:
- Lease allocation and tracking
- Lease persistence and recovery
- Lease expiration and cleanup
- Static lease reservations
- Lease analytics and reporting

## Architecture

### Lease Management Components
1. **Lease Manager**: Manages lease lifecycle
2. **Lease Database**: Persistent lease storage
3. **Expiration Manager**: Handles lease expiration
4. **Reservation Manager**: Manages static leases
5. **Analytics Engine**: Lease statistics and reporting

### Lease States
- **ALLOCATED**: Lease assigned to client
- **EXPIRED**: Lease time elapsed
- **RELEASED**: Client released lease
- **ABANDONED**: Client disappeared
- **RESERVED**: Statically reserved

## Configuration

### Basic Lease Management
```hcl
# DHCP lease management
dhcp {
  enabled = true

  scope "lan" {
    interface = "eth1"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"

    # Lease settings
    lease_time = "24h"
    max_lease_time = "7d"
    min_lease_time = "1h"
  }
}
```

### Advanced Lease Management Configuration
```hcl
dhcp {
  enabled = true

  # Lease database
  database = {
    type = "sqlite"
    path = "/var/lib/flywall/dhcp_leases.db"

    # Backup settings
    backup = {
      enabled = true
      interval = "1h"
      retention = "7d"
      path = "/var/backups/dhcp/"
    }

    # Performance
    cache_size = 10000
    sync_mode = "NORMAL"
    journal_mode = "WAL"
  }

  # Lease policies
  lease_policies = {
    # Default lease times
    default_lease_time = "24h"
    max_lease_time = "7d"
    min_lease_time = "1h"

    # Grace periods
    grace_period = "2h"
    renewal_time = "12h"
    rebinding_time = "21h"

    # Reuse policy
    reuse_expired = true
    reuse_after = "1h"

    # Conflict handling
    conflict_resolution = "deny"  # deny, replace, warn

    # Client validation
    validate_client = true
    require_client_identifier = false
  }

  # Static reservations
  reservations = [
    {
      mac = "00:11:22:33:44:55"
      ip = "192.168.1.10"
      hostname = "server01"

      # Reservation options
      options = {
        lease_time = "infinite"
        boot_file = "server01.cfg"
      }
    },
    {
      mac = "aa:bb:cc:dd:ee:ff"
      ip = "192.168.1.20"
      hostname = "printer01"

      options = {
        lease_time = "infinite"
      }
    },
    {
      hostname = "voip01"
      ip = "192.168.1.30"

      # Allow any MAC for this hostname
      allow_any_mac = true
    }
  ]

  scope "lan" {
    interface = "eth1"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"

    # Scope-specific lease settings
    lease_time = "24h"
    max_lease_time = "7d"

    # Per-client lease times
    client_lease_times = [
      {
        mac_prefix = ["00:11:22", "aa:bb:cc"]
        lease_time = "12h"
      },
      {
        vendor_class = "Printer"
        lease_time = "7d"
      },
      {
        hostname_regex = "server.*"
        lease_time = "7d"
      }
    ]

    # Address pools
    pools = [
      {
        name = "servers"
        range = "192.168.1.10-192.168.1.50"
        lease_time = "7d"
        allow = ["server-*", "static-*"]
      },
      {
        name = "workstations"
        range = "192.168.1.100-192.168.1.200"
        lease_time = "24h"
      },
      {
        name = "guests"
        range = "192.168.1.201-192.168.1.250"
        lease_time = "2h"
        max_lease_time = "4h"
      }
    ]

    # Lease restrictions
    restrictions = [
      {
        # Limit leases per MAC
        max_leases_per_mac = 1
      },
      {
        # Limit leases per hostname
        max_leases_per_hostname = 1
      },
      {
        # Block known bad MACs
        block_macs = ["00:00:00:00:00:00"]
      }
    ]
  }

  scope "guest" {
    interface = "eth2"
    range_start = "192.168.200.100"
    range_end = "192.168.200.200"

    # Strict guest policies
    lease_time = "2h"
    max_lease_time = "4h"
    grace_period = "30m"

    # Limit concurrent leases
    max_concurrent_leases = 50

    # Auto-cleanup
    cleanup_abandoned = true
    cleanup_interval = "15m"
  }
}
```

### Lease Persistence and Recovery
```hcl
dhcp {
  enabled = true

  # Persistence settings
  persistence = {
    enabled = true

    # What to persist
    persist = [
      "active_leases",
      "expired_leases",
      "reservations",
      "client_history"
    ]

    # Retention periods
    retention = {
      active_leases = "30d"
      expired_leases = "7d"
      abandoned_leases = "1d"
      client_history = "90d"
    }

    # Export format
    export_format = "json"

    # Automatic export
    auto_export = {
      enabled = true
      interval = "1h"
      path = "/var/lib/flywall/dhcp_exports/"
    }
  }

  # Recovery settings
  recovery = {
    # Auto-recover on startup
    auto_recover = true

    # Validation
    validate_on_recover = true

    # Conflict handling
    resolve_conflicts = "keep_existing"  # keep_existing, use_recovered, ask

    # Backup recovery
    recover_from_backup = true
    backup_path = "/var/backups/dhcp/latest/"
  }

  # High availability
  ha = {
    enabled = true

    # Replication
    replicate = true
    replication_servers = [
      "192.168.1.2:9001",
      "192.168.1.3:9001"
    ]

    # Failover
    failover_mode = "hot_standby"
    failover_ip = "192.168.1.254"

    # Sync settings
    sync_interval = "30s"
    sync_timeout = "5s"
  }
}
```

### Lease Analytics
```hcl
dhcp {
  enabled = true

  # Analytics configuration
  analytics = {
    enabled = true

    # Metrics to collect
    metrics = [
      "leases_per_hour",
      "leases_per_day",
      "average_lease_time",
      "lease_renewals",
      "lease_expirations",
      "abandoned_leases",
      "pool_utilization"
    ]

    # Reporting
    reports = {
      enabled = true

      # Report schedules
      schedules = [
        {
          name = "daily_summary"
          schedule = "0 0 * * *"  # Daily at midnight
          format = "html"
          recipients = ["admin@example.com"]
        },
        {
          name = "weekly_report"
          schedule = "0 0 * * 0"  # Weekly on Sunday
          format = "pdf"
          recipients = ["manager@example.com"]
        }
      ]

      # Report content
      include = [
        "lease_statistics",
        "pool_utilization",
        "client_history",
        "anomalies"
      ]
    }

    # Anomaly detection
    anomaly_detection = {
      enabled = true

      # Anomalies to detect
      detect = [
        "excessive_leases",
        "rapid_renewals",
        "unusual_patterns",
        "pool_exhaustion"
      ]

      # Thresholds
      thresholds = {
        excessive_leases_per_minute = 10
        rapid_renewals_per_minute = 5
        pool_utilization_warning = 80
        pool_utilization_critical = 95
      }

      # Actions
      actions = ["alert", "log", "email"]
    }
  }
}
```

## Implementation Details

### Lease Database Schema
```sql
CREATE TABLE dhcp_leases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mac_address TEXT NOT NULL,
    ip_address TEXT NOT NULL UNIQUE,
    hostname TEXT,
    vendor_class TEXT,
    client_identifier TEXT,
    lease_start INTEGER NOT NULL,
    lease_end INTEGER NOT NULL,
    last_seen INTEGER NOT NULL,
    state TEXT NOT NULL,
    options TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX idx_dhcp_leases_mac ON dhcp_leases(mac_address);
CREATE INDEX idx_dhcp_leases_ip ON dhcp_leases(ip_address);
CREATE INDEX idx_dhcp_leases_state ON dhcp_leases(state);
CREATE INDEX idx_dhcp_leases_end ON dhcp_leases(lease_end);
```

### Lease Lifecycle
1. **DISCOVER**: Client broadcasts discover
2. **OFFER**: Server offers lease
3. **REQUEST**: Client requests lease
4. **ACK**: Server acknowledges lease
5. **RENEWAL**: Client renews lease (T1)
6. **REBINDING**: Client rebinds lease (T2)
7. **RELEASE**: Client releases lease
8. **EXPIRE**: Lease expires

## Testing

### Lease Management Testing
```bash
# Request lease
dhclient eth1

# Check lease
cat /var/lib/dhcp/dhclient.leases

# Release lease
dhclient -r eth1

# Check lease database
sqlite3 /var/lib/flywall/dhcp_leases.db "SELECT * FROM dhcp_leases;"
```

### Integration Tests
- `dhcp_lease_lifecycle_test.sh`: Lease lifecycle
- `dhcp_persistence_test.sh`: Lease persistence
- `dhcp_reservation_test.sh`: Static reservations

## API Integration

### Lease Management API
```bash
# List all leases
curl -s "http://localhost:8080/api/dhcp/leases"

# Get specific lease
curl -s "http://localhost:8080/api/dhcp/leases/192.168.1.100"

# Get lease by MAC
curl -s "http://localhost:8080/api/dhcp/leases/mac/00:11:22:33:44:55"

# Create reservation
curl -X POST "http://localhost:8080/api/dhcp/reservations" \
  -H "Content-Type: application/json" \
  -d '{
    "mac": "00:11:22:33:44:55",
    "ip": "192.168.1.10",
    "hostname": "server01"
  }'

# Release lease
curl -X DELETE "http://localhost:8080/api/dhcp/leases/192.168.1.100"
```

### Analytics API
```bash
# Get lease statistics
curl -s "http://localhost:8080/api/dhcp/leases/stats"

# Get pool utilization
curl -s "http://localhost:8080/api/dhcp/pools/utilization"

# Get lease history
curl -s "http://localhost:8080/api/dhcp/leases/history?mac=00:11:22:33:44:55"

# Get anomalies
curl -s "http://localhost:8080/api/dhcp/anomalies"
```

## Best Practices

1. **Lease Management**
   - Set appropriate lease times
   - Monitor pool utilization
   - Use reservations for critical devices
   - Regular cleanup of old leases

2. **Persistence**
   - Enable lease persistence
   - Regular backups
   - Test recovery procedures
   - Validate lease data

3. **Performance**
   - Optimize database queries
   - Use connection pooling
   - Cache active leases
   - Monitor database size

4. **Security**
   - Validate client identifiers
   - Monitor for abuse
   - Limit lease requests
   - Audit lease assignments

## Troubleshooting

### Common Issues
1. **No available leases**: Check pool utilization
2. **Duplicate IP assignments**: Check database integrity
3. **Lease not persisting**: Check database permissions
4. **High database usage**: Clean old leases

### Debug Commands
```bash
# Check lease status
flywall dhcp lease status

# Check specific lease
flywall dhcp lease show --ip 192.168.1.100

# Check pool utilization
flywall dhcp pool status

# Validate database
flywall dhcp validate-database
```

### Advanced Debugging
```bash
# Monitor lease activity
flywall dhcp monitor --leases

# Debug lease allocation
flywall dhcp debug --lease --verbose

# Check database integrity
sqlite3 /var/lib/flywall/dhcp_leases.db "PRAGMA integrity_check;"

# Export leases
flywall dhcp export-leases --format json > leases.json
```

## Performance Considerations

- Database size grows with lease history
- Frequent writes can impact performance
- Indexes improve query speed
- Connection pooling helps

## Security Considerations

- Lease information reveals network topology
- MAC address privacy concerns
- Need access controls
- Audit trail important

## Related Features

- [DHCP Server](dhcp-server.md)
- [DHCP Options](dhcp-options.md)
- [High Availability](ha-configuration.md)
- [Analytics Engine](analytics-engine.md)
