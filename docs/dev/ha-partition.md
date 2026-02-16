# HA Partition Handling Implementation Guide

## Overview

Flywall provides robust network partition handling for HA deployments:
- Split-brain detection and prevention
- Partition recovery mechanisms
- Quorum-based decision making
- Graceful degradation
- Automatic reconciliation

## Architecture

### Partition Handling Components
1. **Partition Detector**: Detects network partitions
2. **Quorum Manager**: Manages quorum calculations
3. **State Manager**: Handles state during partition
4. **Recovery Manager**: Manages partition recovery
5. **Reconciler**: Reconciles state after recovery

### Partition Types
- **Network Partition**: Network connectivity loss
- **Node Failure**: Individual node failure
- **Split-Brain**: Multiple isolated clusters
- **Partial Partition**: Some nodes unreachable

## Configuration

### Basic HA Partition Handling
```hcl
# HA configuration with partition handling
ha {
  enabled = true
  mode = "primary"

  # Partition handling
  partition_handling = {
    enabled = true
    quorum = "majority"
  }

  # Peer configuration
  peers = [
    {
      address = "192.168.100.2:9001"
      role = "replica"
    },
    {
      address = "192.168.100.3:9001"
      role = "replica"
    }
  ]
}
```

### Advanced Partition Handling Configuration
```hcl
ha {
  enabled = true
  mode = "primary"
  node_id = "node1"

  # Partition handling
  partition_handling = {
    enabled = true

    # Quorum settings
    quorum = {
      type = "majority"  # majority, strict, none

      # Quorum calculation
      include_primary = true
      include_replicas = true

      # Minimum nodes
      minimum_nodes = 2

      # Weight-based quorum
      weights = {
        "node1" = 1.0
        "node2" = 1.0
        "node3" = 0.5
      }
    }

    # Detection
    detection = {
      # Heartbeat settings
      heartbeat_interval = "1s"
      heartbeat_timeout = "5s"
      heartbeat_failures = 3

      # Network tests
      network_tests = {
        enabled = true
        test_interval = "5s"
        test_timeout = "2s"
        test_addresses = ["8.8.8.8", "1.1.1.1"]
      }

      # Partition detection
      partition_threshold = "10s"
      split_brain_detection = true
    }

    # Actions on partition
    actions = {
      # When partitioned
      on_partition = {
        # Stop accepting new connections
        stop_accepting = true

        # Continue serving existing
        continue_serving = true

        # Enter read-only mode
        read_only = false

        # Grace period before action
        grace_period = "30s"
      }

      # When losing quorum
      on_quorum_loss = {
        # Stop all services
        stop_all = false

        # Enter read-only mode
        read_only = true

        # Keep essential services
        essential_only = true

        # Alert administrators
        alert = true
      }

      # When split-brain detected
      on_split_brain = {
        # Auto-resolve
        auto_resolve = true

        # Resolution method
        resolution_method = "lowest_node_id"  # lowest_node_id, highest_weight, manual

        # Manual intervention required
        require_manual = false

        # Fence offending nodes
        fencing = true
      }
    }

    # Recovery
    recovery = {
      # Auto-recovery
      auto_recover = true

      # Recovery delay
      recovery_delay = "30s"

      # State reconciliation
      reconcile_state = true

      # Reconciliation method
      reconciliation_method = "merge"  # merge, primary_wins, timestamp

      # Full sync
      full_sync = true
      full_sync_threshold = "1h"
    }
  }

  # Peer configuration
  peers = [
    {
      id = "node2"
      address = "192.168.100.2:9001"
      role = "replica"
      weight = 1.0

      # Connection settings
      connection = {
        timeout = "5s"
        keep_alive = "30s"
        retry_interval = "5s"
        max_retries = 3
      }

      # Health checks
      health_check = {
        enabled = true
        interval = "10s"
        timeout = "3s"

        # Check endpoints
        endpoints = [
          "/api/health",
          "/api/ha/status"
        ]
      }
    },
    {
      id = "node3"
      address = "192.168.100.3:9001"
      role = "replica"
      weight = 0.5

      # Witness node (doesn't serve traffic)
      witness = true

      connection = {
        timeout = "5s"
        keep_alive = "30s"
      }
    }
  ]
}
```

### Advanced Partition Strategies
```hcl
ha {
  enabled = true

  partition_handling = {
    enabled = true

    # Advanced quorum
    quorum = {
      type = "weighted"

      # Dynamic weights
      dynamic_weights = true

      # Weight factors
      weight_factors = [
        {
          factor = "node_capability"
          weights = {
            "primary" = 2.0
            "replica" = 1.0
            "witness" = 0.5
          }
        },
        {
          factor = "network_quality"
          metric = "latency"
          weights = {
            "excellent" = 1.5
            "good" = 1.0
            "poor" = 0.5
          }
        },
        {
          factor = "load"
          metric = "cpu_usage"
          inverse = true  # Lower load = higher weight
        }
      ]
    }

    # Partition strategies
    strategies = [
      {
        name = "graceful_degradation"

        # Trigger conditions
        triggers = [
          {
            condition = "partition_detected"
            nodes_lost = "< 50%"
          }
        ]

        # Actions
        actions = [
          {
            type = "reduce_services"
            services = ["analytics", "reporting"]
          },
          {
            type = "increase_cache"
            factor = 2.0
          },
          {
            type = "batch_operations"
            enabled = true
            batch_size = 100
          }
        ]
      },
      {
        name = "emergency_mode"

        triggers = [
          {
            condition = "quorum_lost"
            nodes_remaining = "< 50%"
          }
        ]

        actions = [
          {
            type = "read_only_mode"
            enabled = true
          },
          {
            type = "essential_services_only"
            services = ["firewall", "dns"]
          },
          {
            type = "disable_replication"
          }
        ]
      }
    ]

    # Split-brain prevention
    split_brain_prevention = {
      # Fencing
      fencing = {
        enabled = true
        method = "stonith"  # shoot the other node in the head

        # Fencing agents
        agents = [
          {
            type = "ipmi"
            config = {
              host = "192.168.100.2"
              username = "admin"
              password = "password"
            }
          },
          {
            type = "network"
            config = {
              interface = "eth0"
              action = "disable"
            }
          }
        ]
      }

      # Arbitration
      arbitration = {
        enabled = true
        method = "witness"  # witness, shared_storage, quorum_disk

        # Witness configuration
        witness = {
          address = "192.168.100.254:9001"
          timeout = "5s"
        }
      }

      # Lease mechanism
      lease = {
        enabled = true
        lease_time = "10s"
        renew_before = "7s"
        storage = "shared"  # shared, local
      }
    }
  }
}
```

### Partition Recovery and Reconciliation
```hcl
ha {
  enabled = true

  partition_handling = {
    # Recovery configuration
    recovery = {
      # Auto-recovery settings
      auto_recovery = {
        enabled = true

        # Recovery conditions
        conditions = [
          "network_restored",
          "quorum_regained",
          "split_brain_resolved"
        ]

        # Recovery delay
        delay = "30s"

        # Max attempts
        max_attempts = 3
        backoff = "exponential"
      }

      # State reconciliation
      reconciliation = {
        enabled = true

        # What to reconcile
        reconcile = [
          "dhcp_leases",
          "dns_records",
          "configuration",
          "metrics",
          "alerts"
        ]

        # Reconciliation strategies
        strategies = [
          {
            data_type = "dhcp_leases"
            strategy = "merge_with_conflict_resolution"

            # Conflict resolution
            conflict_resolution = {
              method = "most_recent"
              tie_breaker = "primary_wins"
            }
          },
          {
            data_type = "configuration"
            strategy = "primary_wins"

            # Validation
            validate = true
            backup_before_apply = true
          },
          {
            data_type = "metrics"
            strategy = "aggregate"

            # Aggregation method
            aggregation = {
              method = "sum"
              time_window = "5m"
            }
          }
        ]

        # Reconciliation process
        process = {
          # Step 1: Compare states
          compare_states = true

          # Step 2: Identify conflicts
          identify_conflicts = true

          # Step 3: Resolve conflicts
          resolve_conflicts = true

          # Step 4: Apply changes
          apply_changes = true

          # Step 5: Verify
          verify = true
        }
      }

      # Full sync
      full_sync = {
        # Trigger conditions
        triggers = [
          {
            condition = "partition_duration"
            duration = "1h"
          },
          {
            condition = "conflict_count"
            count = 100
          },
          {
            condition = "manual"
          }
        ]

        # Sync process
        process = {
          # Stop services
          stop_services = true

          # Sync data
          sync_data = true

          # Validate
          validate = true

          # Start services
          start_services = true
        }
      }
    }
  }
}
```

## Implementation Details

### Partition Detection Algorithm
```go
type PartitionDetector struct {
    nodeID       string
    peers        []Peer
    heartbeat    map[string]time.Time
    quorum       QuorumCalculator

    // Detection state
    partitioned  bool
    partitionTime time.Time
    lastSeen     map[string]time.Time
}

func (pd *PartitionDetector) DetectPartition() bool {
    // Check peer connectivity
    connected := 0
    for _, peer := range pd.peers {
        if time.Since(pd.lastSeen[peer.ID]) < pd.heartbeatTimeout {
            connected++
        }
    }

    // Calculate quorum
    hasQuorum := pd.quorum.HasQuorum(connected + 1)  // +1 for self

    // Detect partition
    if !hasQuorum {
        if !pd.partitioned {
            pd.partitioned = true
            pd.partitionTime = time.Now()
            return true
        }
    } else {
        if pd.partitioned {
            pd.partitioned = false
            return true  // Partition recovered
        }
    }

    return false
}
```

### Quorum Calculation
```go
type QuorumCalculator struct {
    type_     string
    weights   map[string]float64
    minNodes  int
}

func (qc *QuorumCalculator) HasQuorum(connected int) bool {
    switch qc.type_ {
    case "majority":
        total := len(qc.weights)
        return connected > total/2
    case "strict":
        return connected == len(qc.weights)
    case "weighted":
        var totalWeight, connectedWeight float64
        for _, w := range qc.weights {
            totalWeight += w
        }
        return connectedWeight > totalWeight/2
    case "none":
        return true
    default:
        return connected >= qc.minNodes
    }
}
```

## Testing

### Partition Handling Testing
```bash
# Simulate network partition
iptables -A INPUT -s 192.168.100.2 -j DROP

# Check HA status
flywall ha status

# Check quorum
flywall ha quorum

# Recover from partition
iptables -D INPUT -s 192.168.100.2 -j DROP
```

### Integration Tests
- `ha_partition_test.sh`: Partition detection
- `quorum_test.sh`: Quorum calculation
- `recovery_test.sh`: Partition recovery

## API Integration

### Partition Handling API
```bash
# Get partition status
curl -s "http://localhost:8080/api/ha/partition/status"

# Get quorum status
curl -s "http://localhost:8080/api/ha/quorum"

# Force partition recovery
curl -X POST "http://localhost:8080/api/ha/partition/recover"

# Get reconciliation status
curl -s "http://localhost:8080/api/ha/reconciliation/status"
```

### Recovery API
```bash
# Start reconciliation
curl -X POST "http://localhost:8080/api/ha/reconcile" \
  -H "Content-Type: application/json" \
  -d '{
    "full_sync": true,
    "strategy": "merge"
  }'

# Get reconciliation report
curl -s "http://localhost:8080/api/ha/reconciliation/report"

# Manual conflict resolution
curl -X POST "http://localhost:8080/api/ha/reconciliation/resolve" \
  -H "Content-Type: application/json" \
  -d '{
    "conflict_id": "dhcp_lease_123",
    "resolution": "primary_wins"
  }'
```

## Best Practices

1. **Partition Design**
   - Use majority quorum when possible
   - Implement split-brain prevention
   - Have clear recovery procedures
   - Test partition scenarios

2. **Quorum Management**
   - Consider weighted quorum
   - Include witness nodes
   - Monitor quorum status
   - Plan for node failures

3. **State Management**
   - Reconcile state carefully
   - Validate before applying
   - Keep audit trails
   - Backup before recovery

4. **Recovery**
   - Automate when safe
   - Require manual for critical
   - Document procedures
   - Test regularly

## Troubleshooting

### Common Issues
1. **False partitions**: Check network stability
2. **Split-brain**: Verify fencing mechanisms
3. **Reconciliation failures**: Check data consistency
4. **Quorum loss**: Review node connectivity

### Debug Commands
```bash
# Check partition status
flywall ha partition status --verbose

# Check peer connectivity
flywall ha peers status

# Monitor quorum
watch -n 1 'flywall ha quorum'

# Debug reconciliation
flywall ha debug --reconciliation
```

### Advanced Debugging
```bash
# Check heartbeat
flywall ha heartbeat status

# Test partition detection
flywall ha test-partition

# Check fencing
flywall ha fencing status

# Validate state
flywall ha validate-state
```

## Performance Considerations

- Partition detection adds overhead
- Quorum calculations scale with nodes
- Reconciliation can be intensive
- Network latency affects detection

## Security Considerations

- Partition attacks possible
- Fencing must be secure
- State validation critical
- Access control important

## Related Features

- [HA Configuration](ha-configuration.md)
- [State Replication](state-replication.md)
- [State Persistence](state-persistence.md)
- [Security](security.md)
