# Schema Migration Implementation Guide

## Overview

Flywall provides automated schema migration for:
- Configuration format updates
- Database schema changes
- Feature deprecations
- Version compatibility
- Rollback capabilities

## Architecture

### Migration Components
1. **Migration Engine**: Executes migration scripts
2. **Version Manager**: Tracks current version
3. **Validator**: Ensures migration validity
4. **Rollback Manager**: Handles migration rollbacks
5. **Backup Manager**: Creates pre-migration backups

### Migration Types
- **Configuration**: HCL format changes
- **Database**: SQLite schema updates
- **State**: Data structure changes
- **API**: Endpoint modifications

## Configuration

### Basic Migration Setup
```hcl
# Migration configuration
migration {
  enabled = true

  # Migration directory
  directory = "/etc/flywall/migrations"

  # Auto-migration
  auto_migrate = true

  # Backup before migration
  backup_before = true

  # Migration timeout
  timeout = "30m"
}
```

### Advanced Migration Configuration
```hcl
migration {
  enabled = true
  directory = "/etc/flywall/migrations"

  # Version tracking
  version_table = "schema_versions"
  current_version = "1.2.0"

  # Migration strategy
  strategy = "incremental"  # incremental, full, custom

  # Validation
  validate_before = true
  validate_after = true
  dry_run = false

  # Backup settings
  backup_before = true
  backup_retention = "7d"
  backup_location = "/var/lib/flywall/migration-backups"

  # Rollback settings
  auto_rollback = true
  rollback_timeout = "10m"
  max_rollback_attempts = 3

  # Notifications
  notify_on_start = true
  notify_on_complete = true
  notify_on_failure = true
  notification_channels = ["email", "slack"]
}
```

## Migration Scripts

### Configuration Migration
```go
// migrations/202312010001_add_zone_services.go
package migrations

import (
    "github.com/flywall/migration"
)

type AddZoneServices struct{}

func (m *AddZoneServices) Version() string {
    return "202312010001"
}

func (m *AddZoneServices) Description() string {
    return "Add services section to zone configuration"
}

func (m *AddZoneServices) Up(ctx context.Context, config *Config) error {
    // Add services section to all zones
    for _, zone := range config.Zones {
        if zone.Services == nil {
            zone.Services = &ZoneServices{
                DNS:  false,
                DHCP: false,
            }
        }
    }

    // Update schema version
    config.SchemaVersion = "1.2.0"

    return nil
}

func (m *AddZoneServices) Down(ctx context.Context, config *Config) error {
    // Remove services section
    for _, zone := range config.Zones {
        zone.Services = nil
    }

    // Revert schema version
    config.SchemaVersion = "1.1.0"

    return nil
}
```

### Database Migration
```go
// migrations/202312010002_add_dns_cache_table.go
package migrations

import (
    "database/sql"
    "github.com/flywall/migration"
)

type AddDNSCacheTable struct{}

func (m *AddDNSCacheTable) Version() string {
    return "202312010002"
}

func (m *AddDNSCacheTable) Up(ctx context.Context, db *sql.DB) error {
    query := `
    CREATE TABLE IF NOT EXISTS dns_cache (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        type TEXT NOT NULL,
        value TEXT NOT NULL,
        ttl INTEGER NOT NULL,
        created_at INTEGER NOT NULL,
        expires_at INTEGER NOT NULL,
        UNIQUE(name, type)
    );

    CREATE INDEX IF NOT EXISTS idx_dns_cache_name ON dns_cache(name);
    CREATE INDEX IF NOT EXISTS idx_dns_cache_expires ON dns_cache(expires_at);
    `

    _, err := db.ExecContext(ctx, query)
    return err
}

func (m *AddDNSCacheTable) Down(ctx context.Context, db *sql.DB) error {
    _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS dns_cache")
    return err
}
```

### Data Migration
```go
// migrations/202312010003_migrate_dhcp_leases.go
package migrations

import (
    "database/sql"
    "github.com/flywall/migration"
)

type MigrateDHCPLeases struct{}

func (m *MigrateDHCPLeases) Up(ctx context.Context, db *sql.DB) error {
    // Add new columns
    _, err := db.ExecContext(ctx, `
    ALTER TABLE dhcp_leases ADD COLUMN hostname TEXT;
    ALTER TABLE dhcp_leases ADD COLUMN vendor_class TEXT;
    `)
    if err != nil {
        return err
    }

    // Migrate data from old table
    _, err = db.ExecContext(ctx, `
    UPDATE dhcp_leases l
    SET hostname = h.hostname
    FROM dhcp_hosts h
    WHERE l.mac_address = h.mac_address;
    `)

    return err
}

func (m *MigrateDHCPLeases) Down(ctx context.Context, db *sql.DB) error {
    // Backup data before dropping columns
    _, err := db.ExecContext(ctx, `
    CREATE TABLE dhcp_leases_backup AS
    SELECT * FROM dhcp_leases;
    `)
    if err != nil {
        return err
    }

    // Drop columns (SQLite specific)
    return migrateSQLiteRecreateTable(ctx, db, "dhcp_leases", []string{
        "mac_address", "ip_address", "scope",
        "lease_time", "issued_at", "expires_at",
    })
}
```

## Implementation Details

### Migration Process
1. **Detection**: Identify version differences
2. **Planning**: Create migration plan
3. **Backup**: Create pre-migration backup
4. **Execution**: Run migration scripts
5. **Validation**: Verify migration success
6. **Cleanup**: Remove temporary data

### Version Tracking
```sql
CREATE TABLE schema_versions (
    component TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    applied_at INTEGER NOT NULL,
    checksum TEXT NOT NULL
);
```

### Migration States
- **Pending**: Not yet applied
- **Running**: Currently executing
- **Applied**: Successfully applied
- **Failed**: Migration failed
- **Rolled Back**: Successfully rolled back

## Testing

### Migration Testing
```bash
# Check current version
flywall migration status

# List pending migrations
flywall migration list --pending

# Dry run migration
flywall migration migrate --dry-run

# Apply migration
flywall migration migrate

# Verify migration
flywall migration validate

# Rollback migration
flywall migration rollback 202312010001
```

### Integration Testing
```bash
# Test migration with data
flywall migration test --with-data

# Test rollback
flywall migration test --rollback

# Test performance
flywall migration test --performance

# Create test migration
flywall migration create test_migration
```

## API Integration

### Migration API
```bash
# Get migration status
curl -s "http://localhost:8080/api/migration/status"

# List migrations
curl -s "http://localhost:8080/api/migrations"

# Get specific migration
curl -s "http://localhost:8080/api/migrations/202312010001"

# Apply migration
curl -X POST "http://localhost:8080/api/migrations/apply" \
  -H "Content-Type: application/json" \
  -d '{
    "target_version": "1.2.0",
    "dry_run": false
  }'

# Rollback migration
curl -X POST "http://localhost:8080/api/migrations/rollback" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "202312010001"
  }'
```

### Migration History
```bash
# Get migration history
curl -s "http://localhost:8080/api/migrations/history"

# Get migration details
curl -s "http://localhost:8080/api/migrations/202312010001/details"

# Download migration log
curl -s "http://localhost:8080/api/migrations/202312010001/log" > migration.log
```

## Best Practices

1. **Migration Design**
   - Make migrations reversible
   - Keep migrations small
   - Test thoroughly
   - Document changes

2. **Data Safety**
   - Always backup first
   - Validate data integrity
   - Test rollback procedures
   - Monitor for issues

3. **Performance**
   - Minimize downtime
   - Use batches for large data
   - Optimize queries
   - Monitor resources

4. **Compatibility**
   - Support multiple versions
   - Handle edge cases
   - Test upgrade paths
   - Document requirements

## Troubleshooting

### Common Issues
1. **Migration fails**: Check logs and error messages
2. **Data corruption**: Restore from backup
3. **Performance issues**: Optimize migration
4. **Version conflicts**: Resolve dependencies

### Debug Commands
```bash
# Check migration status
flywall migration status --verbose

# Validate migrations
flywall migration validate --all

# Check database
sqlite3 /var/lib/flywall/state.db "SELECT * FROM schema_versions;"

# Debug specific migration
flywall migration debug 202312010001
```

### Advanced Debugging
```bash
# Test migration in isolation
flywall migration test --isolation 202312010001

# Check migration checksums
flywall migration checksums

# Verify data integrity
flywall migration verify --data

# Force migration state
flywall migration force --version 202312010001 --state applied
```

## Performance Considerations

- Large migrations may require maintenance windows
- Batch operations improve performance
- Index rebuilds may be necessary
- Monitor system resources

## Security Considerations

- Validate migration signatures
- Secure backup storage
- Audit migration access
- Review migration code

## Related Features

- [Configuration Management](config-management.md)
- [State Persistence](state-persistence.md)
- [Backup & Recovery](backup-recovery.md)
- [API Versioning](api-versioning.md)
