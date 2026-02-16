# High Availability Implementation Guide

## Overview

Flywall High Availability (HA) provides:
- Active-passive failover with automatic VIP management
- State replication between primary and backup nodes
- Configurable failback policies
- Heartbeat-based failure detection
- Split-brain prevention

**Note**: For detailed information about state replication between nodes, see [State Replication](state-replication.md). For local state persistence details, see [State Persistence](state-persistence.md).

## Architecture

### HA Components
1. **Heartbeat Service**: Monitors node health
2. **VIP Manager**: Controls virtual IP assignment
3. **State Replication**: Syncs configuration and state
4. **Failover Controller**: Manages failover/failback logic
5. **Conntrack Sync**: Optional connection state sync

### Node Roles
- **Primary**: Active node handling traffic
- **Backup**: Standby node ready to take over
- **Auto-negotiation**: Nodes determine roles based on priority

## Configuration

### Basic HA Setup
```hcl
# Primary Node
replication {
  mode = "primary"
  listen_addr = "192.168.100.1:9001"
  peer_addr = "192.168.100.2:9002"
  secret_key = "SHARED_SECRET_KEY"

  ha {
    enabled = true
    priority = 50  # Lower = more preferred
    failback_mode = "auto"
    failback_delay = 5

    # Heartbeat settings
    heartbeat_interval = 1
    failure_threshold = 10
    heartbeat_port = 9002

    # Virtual IP
    virtual_ip {
      address = "10.0.0.100/24"
      interface = "eth1"
    }
  }
}

# Backup Node
replication {
  mode = "replica"
  listen_addr = "192.168.100.2:9001"
  primary_addr = "192.168.100.1:9001"
  peer_addr = "192.168.100.1:9002"
  secret_key = "SHARED_SECRET_KEY"

  ha {
    enabled = true
    priority = 150  # Higher = less preferred
    heartbeat_interval = 1
    failure_threshold = 10
    heartbeat_port = 9002

    virtual_ip {
      address = "10.0.0.100/24"
      interface = "eth1"
    }
  }
}
```

### Advanced HA Configuration
```hcl
replication {
  mode = "primary"
  listen_addr = "10.0.0.1:9001"
  peer_addr = "10.0.0.2:9002"
  secret_key = "COMPLEX_SECRET_32_CHARS"

  # Replication settings
  compression = true
  encryption = true
  sync_interval = "1s"
  batch_size = 1000

  ha {
    enabled = true
    priority = 50

    # Failback control
    failback_mode = "manual"  # auto, manual, disabled
    failback_delay = 30
    failback_window = "5m"

    # Failure detection
    heartbeat_interval = 1
    failure_threshold = 5
    failure_timeout = "10s"
    heartbeat_port = 9002

    # Network monitoring
    monitor_links = ["eth0", "eth1"]
    monitor_gateways = ["192.168.1.1"]

    # Conntrack sync
    conntrack_sync {
      enabled = true
      interface = "eth0"
      sync_interval = "5s"
      max_entries = 100000
    }

    # Virtual IP
    virtual_ip {
      address = "10.0.0.100/24"
      interface = "eth1"
      arp_ping = true
      gratuitous_arp = true
    }

    # Pre/post failover scripts
    pre_failover_script = "/etc/flywall/pre-failover.sh"
    post_failover_script = "/etc/flywall/post-failover.sh"
  }
}
```

### Multi-Node HA
```hcl
# Node 1 (Primary)
replication {
  mode = "primary"
  node_id = "node1"
  listen_addr = "10.0.0.1:9001"
  peer_addrs = ["10.0.0.2:9001", "10.0.0.3:9001"]
  secret_key = "SHARED_SECRET"

  ha {
    enabled = true
    priority = 50
    virtual_ip {
      address = "10.0.0.100/24"
      interface = "eth1"
    }
  }
}

# Node 2 (Backup)
replication {
  mode = "replica"
  node_id = "node2"
  listen_addr = "10.0.0.2:9001"
  peer_addrs = ["10.0.0.1:9001", "10.0.0.3:9001"]
  secret_key = "SHARED_SECRET"

  ha {
    enabled = true
    priority = 100
    virtual_ip {
      address = "10.0.0.100/24"
      interface = "eth1"
    }
  }
}

# Node 3 (Backup)
replication {
  mode = "replica"
  node_id = "node3"
  listen_addr = "10.0.0.3:9001"
  peer_addrs = ["10.0.0.1:9001", "10.0.0.2:9001"]
  secret_key = "SHARED_SECRET"

  ha {
    enabled = true
    priority = 150
    virtual_ip {
      address = "10.0.0.100/24"
      interface = "eth1"
    }
  }
}
```

## Implementation Details

### Failover Process
1. **Heartbeat Detection**: Nodes exchange heartbeats
2. **Failure Detection**: Missing heartbeats trigger failure
3. **VIP Migration**: Backup takes over virtual IP
4. **Service Activation**: Backup becomes active
5. **Client Redirect**: Traffic flows to new primary

### State Replication
- Configuration files synchronized
- DHCP leases replicated
- DNS records synced
- Learning engine data transferred
- Connection states (optional)

### Split-Brain Prevention
- Priority-based election
- Quorum requirements
- Fencing mechanisms
- Network partition detection

## Testing

### Integration Tests
- `ha_full_stack_test.sh`: Complete failover lifecycle
- `ha_partition_test.sh`: Network partition handling
- `replication_test.sh`: State synchronization

### Manual Testing
```bash
# Check HA status
flywall ha status

# View cluster state
flywall ha cluster

# Force failover
flywall ha failover

# Check VIP
ip addr show | grep "10.0.0.100"

# Monitor heartbeats
tcpdump -i any port 9002
```

## API Integration

### HA Management API
```bash
# Get HA status
curl -s "http://localhost:8080/api/ha/status"

# Get cluster info
curl -s "http://localhost:8080/api/ha/cluster"

# Force failover
curl -X POST "http://localhost:8080/api/ha/failover"

# Enable/disable failback
curl -X POST "http://localhost:8080/api/ha/failback" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'

# Get replication status
curl -s "http://localhost:8080/api/replication/status"
```

### Monitoring Endpoints
```bash
# Get HA metrics
curl -s "http://localhost:8080/api/ha/metrics"

# Get failover history
curl -s "http://localhost:8080/api/ha/history"

# Get sync statistics
curl -s "http://localhost:8080/api/replication/stats"
```

## Best Practices

1. **Network Design**
   - Use dedicated sync network
   - Redundant heartbeat paths
   - Proper MTU configuration
   - Monitor network latency

2. **Security**
   - Use strong shared secrets
   - Enable encryption
   - Isolate sync traffic
   - Regular key rotation

3. **Performance**
   - Optimize sync intervals
   - Monitor bandwidth usage
   - Tune batch sizes
   - Consider compression

4. **Reliability**
   - Test failover regularly
   - Monitor split-brain scenarios
   - Document recovery procedures
   - Have manual override plans

## Troubleshooting

### Common Issues
1. **Split-brain**: Check network connectivity between nodes
2. **VIP not moving**: Verify interface configuration
3. **Sync failures**: Check authentication and network
4. **Failover loops**: Adjust failure thresholds

### Debug Commands
```bash
# Check HA logs
journalctl -u flywall | grep -E "(ha|failover|vip)"

# Monitor heartbeats
flywall ha monitor

# Check sync status
flywall replication status

# Verify VIP assignment
ip addr show

# Test connectivity
nc -zv 10.0.0.1 9002
```

### Recovery Procedures
```bash
# Manual VIP assignment
ip addr add 10.0.0.100/24 dev eth1

# Force primary role
flywall ha promote

# Reset HA state
flywall ha reset

# Resync from primary
flywall replication resync
```

## Performance Considerations

- Heartbeat traffic minimal (< 1KB/s)
- Sync bandwidth depends on state size
- Failover time typically < 5 seconds
- VIP migration is instantaneous

## Security Considerations

- Encrypt all replication traffic
- Use dedicated management network
- Limit API access to trusted hosts
- Audit failover events

## Related Features

- [State Replication](state-replication.md)
- [Configuration Management](config-management.md)
- [Metrics Collection](metrics-collection.md)
- [API Reference](api-reference.md)
