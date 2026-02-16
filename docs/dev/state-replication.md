# State Replication Implementation Guide

## Overview

Flywall provides robust state replication for HA deployments:
- Real-time state synchronization
- Conflict resolution
- Incremental updates
- Compression and encryption
- Recovery and resync

**Note**: For information about local state persistence, see [State Persistence](state-persistence.md). For HA configuration details including replication settings, see [HA Configuration](ha-configuration.md).

## Architecture

### Replication Components
1. **Replication Manager**: Coordinates replication
2. **Change Detector**: Identifies state changes
3. **Transport Layer**: Handles data transfer
4. **Conflict Resolver**: Manages conflicts
5. **Recovery Manager**: Handles recovery scenarios

### Replication Modes
- **Primary-Replica**: One-way replication
- **Active-Active**: Bidirectional replication
- **Multi-Master**: Multiple writable nodes
- **Chain**: Linear replication topology

## Configuration

### Basic Replication Setup
```hcl
# Replication configuration
replication {
  mode = "replica"
  listen_addr = "192.168.100.2:9001"
  primary_addr = "192.168.100.1:9001"
  peer_addr = "192.168.100.1:9002"
  secret_key = "SHARED_SECRET_KEY"
}
```

### Advanced Replication Configuration
```hcl
replication {
  mode = "primary"
  node_id = "node1"
  listen_addr = "192.168.100.1:9001"

  # Multiple peers
  peers = [
    {
      id = "node2"
      address = "192.168.100.2:9001"
      role = "replica"
    },
    {
      id = "node3"
      address = "192.168.100.3:9001"
      role = "replica"
    }
  ]

  # Authentication
  auth = {
    method = "psk"
    secret_key = "32-char-secret-key-here"
    key_rotation = "24h"
  }

  # Transport settings
  transport = {
    protocol = "tcp"
    compression = true
    encryption = true
    tls = {
      cert_file = "/etc/flywall/replication.crt"
      key_file = "/etc/flywall/replication.key"
      ca_file = "/etc/flywall/ca.crt"
    }
  }

  # Replication settings
  sync = {
    mode = "incremental"
    interval = "1s"
    batch_size = 1000
    max_lag = "10s"
  }

  # Data to replicate
  data = {
    dhcp_leases = true
    dns_records = true
    learning_state = true
    configuration = true
    metrics = false
  }
}
```

### Conflict Resolution
```hcl
replication {
  mode = "active-active"

  # Conflict resolution
  conflict_resolution = {
    strategy = "timestamp"  # timestamp, priority, manual

    # Priority-based resolution
    priorities = {
      "node1" = 100
      "node2" = 50
      "node3" = 25
    }

    # Timestamp resolution
    timestamp_resolution = "millisecond"
    clock_skew_tolerance = "100ms"

    # Manual resolution
    manual_resolution = {
      require_approval = true
      approvers = ["admin", "ops"]
      timeout = "1h"
    }
  }
}
```

### Replication Filtering
```hcl
replication {
  mode = "primary"

  # Data filtering
  filters = {
    # Include only specific zones
    zones = ["wan", "lan"]

    # Exclude sensitive data
    exclude_sensitive = true

    # Custom filters
    custom_filters = [
      {
        type = "dhcp"
        condition = "scope != 'test'"
      },
      {
        type = "dns"
        condition = "ttl > 60"
      }
    ]
  }

  # Transformations
  transformations = {
    anonymize_ips = false
    compress_data = true
    encrypt_data = true
  }
}
```

### Recovery and Resync
```hcl
replication {
  mode = "replica"

  # Recovery settings
  recovery = {
    auto_resync = true
    resync_threshold = 1000
    resync_interval = "1h"

    # Full resync settings
    full_resync = {
      schedule = "0 2 * * *"  # Daily at 2 AM
      compression = true
      checksum_verification = true
    }

    # Incremental recovery
    incremental = {
      enabled = true
      checkpoint_interval = "10m"
      max_checkpoints = 100
    }
  }

  # Backup before resync
  backup_before_resync = true
  backup_retention = "7d"
}
```

## Implementation Details

### Replication Protocol
1. **Handshake**: Authentication and capability negotiation
2. **Sync**: Initial state synchronization
3. **Incremental**: Ongoing change replication
4. **Recovery**: Handle failures and resync

### Change Detection
```go
type Change struct {
    Type      string    `json:"type"`
    Table     string    `json:"table"`
    Key       string    `json:"key"`
    OldValue  interface{} `json:"old_value,omitempty"`
    NewValue  interface{} `json:"new_value,omitempty"`
    Timestamp time.Time `json:"timestamp"`
    NodeID    string    `json:"node_id"`
}
```

### Replication Message
```go
type ReplicationMessage struct {
    Header MessageHeader `json:"header"`
    Body   MessageBody   `json:"body"`
}

type MessageHeader struct {
    Version    string    `json:"version"`
    NodeID     string    `json:"node_id"`
    Sequence   uint64    `json:"sequence"`
    Timestamp  time.Time `json:"timestamp"`
    Checksum   string    `json:"checksum"`
}

type MessageBody struct {
    Type    string   `json:"type"`
    Changes []Change `json:"changes"`
}
```

## Testing

### Replication Testing
```bash
# Test replication status
flywall replication status

# Check lag
flywall replication lag

# Force resync
flywall replication resync

# Test conflict resolution
flywall replication test-conflict
```

### Integration Tests
- `replication_test.sh`: Basic replication
- `conflict_test.sh`: Conflict resolution
- `recovery_test.sh`: Recovery scenarios

## API Integration

### Replication API
```bash
# Get replication status
curl -s "http://localhost:8080/api/replication/status"

# Get peer status
curl -s "http://localhost:8080/api/replication/peers"

# Get replication lag
curl -s "http://localhost:8080/api/replication/lag"

# Force resync
curl -X POST "http://localhost:8080/api/replication/resync"

# Pause replication
curl -X POST "http://localhost:8080/api/replication/pause"

# Resume replication
curl -X POST "http://localhost:8080/api/replication/resume"
```

### Conflict Management API
```bash
# List conflicts
curl -s "http://localhost:8080/api/replication/conflicts"

# Resolve conflict
curl -X POST "http://localhost:8080/api/replication/conflicts/123/resolve" \
  -H "Content-Type: application/json" \
  -d '{
    "resolution": "use_local",
    "reason": "Manual override"
  }'

# Get conflict details
curl -s "http://localhost:8080/api/replication/conflicts/123"
```

## Best Practices

1. **Network Design**
   - Use dedicated replication network
   - Ensure low latency connectivity
   - Provide redundant paths
   - Monitor network health

2. **Security**
   - Use strong authentication
   - Encrypt replication traffic
   - Rotate keys regularly
   - Limit peer access

3. **Performance**
   - Optimize batch sizes
   - Use compression
   - Monitor replication lag
   - Tune sync intervals

4. **Reliability**
   - Test failover scenarios
   - Monitor for split-brain
   - Have manual override
   - Document recovery procedures

## Troubleshooting

### Common Issues
1. **High replication lag**: Check network and load
2. **Conflicts increasing**: Review conflict resolution
3. **Sync failures**: Check authentication and connectivity
4. **Split-brain**: Check quorum settings

### Debug Commands
```bash
# Check replication status
flywall replication status --verbose

# Monitor replication
watch -n 1 'flywall replication lag'

# Check peer connectivity
nc -zv 192.168.100.2 9001

# Debug replication logs
journalctl -u flywall | grep replication
```

### Advanced Debugging
```bash
# Check replication queue
flywall replication queue status

# Validate data consistency
flywall replication validate

# Force checkpoint
flywall replication checkpoint

# Debug specific peer
flywall replication debug --peer node2
```

## Performance Considerations

- Replication overhead scales with change rate
- Compression reduces bandwidth but increases CPU
- Batch size affects latency and throughput
- Network latency impacts replication lag

## Security Considerations

- Replication data contains state information
- Man-in-the-middle attacks possible
- Key compromise affects all nodes
- Need secure key distribution

## Related Features

- [HA Configuration](ha-configuration.md)
- [State Persistence](state-persistence.md)
- [Configuration Management](config-management.md)
- [API Reference](api-reference.md)
