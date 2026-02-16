# Bandwidth Management Implementation Guide

## Overview

Flywall provides comprehensive bandwidth management for:
- Rate limiting per client/application
- Bandwidth allocation and quotas
- Traffic shaping and policing
- Fair bandwidth distribution
- Usage monitoring and reporting

## Architecture

### Bandwidth Management Components
1. **Rate Limiter**: Enforces rate limits
2. **Shaper**: Shapes traffic flow
3. **Quota Manager**: Manages bandwidth quotas
4. **Monitor**: Tracks bandwidth usage
5. **Policy Engine**: Applies bandwidth policies

### Control Mechanisms
- **Token Bucket**: Rate limiting algorithm
- **Leaky Bucket**: Traffic shaping
- **Fair Queueing**: Fair distribution
- **Hierarchical**: Nested bandwidth controls

## Configuration

### Basic Bandwidth Management
```hcl
# Bandwidth management
bandwidth {
  enabled = true

  # Interface limits
  interface "eth0" {
    egress_rate = "1gbit"
    ingress_rate = "1gbit"
  }
}
```

### Advanced Bandwidth Management
```hcl
bandwidth {
  enabled = true

  # Global settings
  global = {
    # Default limits
    default_egress = "100mbit"
    default_ingress = "100mbit"

    # Burst handling
    burst_factor = 2.0
    max_burst = "1gbit"

    # Queue management
    queue_size = 1000
    queue_type = "fq_codel"

    # Congestion control
    congestion_control = "bbr"
  }

  # Interface configuration
  interfaces = [
    {
      name = "eth0"

      # Total bandwidth
      total_bandwidth = "1gbit"

      # Egress (outbound)
      egress = {
        rate = "1gbit"
        ceil = "1gbit"
        burst = "10mbit"

        # Shaping
        shaping = {
          enabled = true
          algorithm = "htb"
          quantum = 1514
        }

        # Queuing
        queue = {
          type = "fq_codel"
          target = "5ms"
          limit = 1000
          interval = "100ms"
          ecn = true
        }
      }

      # Ingress (inbound)
      ingress = {
        rate = "1gbit"

        # Policing (drop excess)
        policing = {
          enabled = true
          rate = "800mbit"
          burst = "20mbit"
          action = "drop"
        }

        # Ingress shaping (ifb)
        shaping = {
          enabled = true
          ifb_device = "ifb0"
        }
      }
    },
    {
      name = "eth1"

      total_bandwidth = "100mbit"

      egress = {
        rate = "100mbit"
        ceil = "100mbit"

        # Strict for guest network
        shaping = {
          enabled = true
          algorithm = "tbf"
          latency = "50ms"
        }
      }

      ingress = {
        rate = "100mbit"
        policing = {
          enabled = true
          rate = "100mbit"
          action = "drop"
        }
      }
    }
  ]

  # Bandwidth classes
  classes = [
    {
      name = "voip"
      parent = "root"
      priority = 1

      # Guaranteed bandwidth
      rate = "10mbit"
      ceil = "50mbit"

      # Low latency
      latency = "10ms"
      jitter = "5ms"

      # Match criteria
      match = {
        protocol = "udp"
        dport = [5060, 5061]
        dport_range = "10000:20000"
      }

      # Queue settings
      queue = {
        type = "pfifo"
        limit = 100
      }
    },
    {
      name = "video"
      parent = "root"
      priority = 2

      rate = "100mbit"
      ceil = "500mbit"

      match = {
        protocol = "tcp"
        dport = [443, 8080, 1935]
        dscp = "af41"
      }

      # Adaptive queue
      queue = {
        type = "fq_codel"
        target = "20ms"
      }
    },
    {
      name = "critical"
      parent = "root"
      priority = 1

      rate = "50mbit"
      ceil = "200mbit"

      match = {
        dscp = ["cs6", "cs7"]
        protocol = ["tcp", "udp"]
      }
    },
    {
      name = "default"
      parent = "root"
      priority = 3

      rate = "200mbit"
      ceil = "1gbit"

      # Default class
      default = true
    },
    {
      name = "bulk"
      parent = "root"
      priority = 4

      rate = "50mbit"
      ceil = "300mbit"

      match = {
        protocol = "tcp"
        dport = [20, 21, 22, 25, 53, 80, 110, 143, 993, 995]
        packet_size = "1200-"
      }

      # Bulk transfer queue
      queue = {
        type = "sfq"
        perturb = 10
        quantum = 1514
      }
    }
  ]
}
```

### Per-Client Bandwidth Management
```hcl
bandwidth {
  enabled = true

  # Per-client policies
  client_policies = [
    {
      # VIP clients
      clients = {
        mac = ["00:11:22:33:44:55", "aa:bb:cc:dd:ee:ff"]
        ip = ["192.168.1.100", "192.168.1.101"]
      }

      # High priority
      priority = 1

      # Guaranteed bandwidth
      guaranteed_rate = "100mbit"
      maximum_rate = "500mbit"

      # Burst allowance
      burst = "50mbit"

      # Time-based limits
      time_limits = [
        {
          time_range = "09:00-17:00"
          days = ["mon", "tue", "wed", "thu", "fri"]
          guaranteed_rate = "200mbit"
          maximum_rate = "1gbit"
        }
      ]
    },
    {
      # Standard users
      clients = {
        subnet = "192.168.1.0/24"
        exclude = ["192.168.1.100", "192.168.1.101"]
      }

      priority = 2
      guaranteed_rate = "10mbit"
      maximum_rate = "100mbit"
      burst = "20mbit"
    },
    {
      # Guest network
      clients = {
        subnet = "192.168.200.0/24"
      }

      priority = 3
      guaranteed_rate = "1mbit"
      maximum_rate = "10mbit"
      burst = "5mbit"

      # Fair sharing
      fair_sharing = true
    }
  ]

  # Application-based limits
  application_limits = [
    {
      name = "streaming"
      applications = ["netflix", "youtube", "hulu", "amazon_prime"]

      # Per-client limit
      per_client = {
        rate = "10mbit"
        burst = "20mbit"
      }

      # Global limit
      global = {
        rate = "500mbit"
        burst = "1gbit"
      }
    },
    {
      name = "gaming"
      applications = ["steam", "xbox_live", "playstation_network", "nintendo_switch"]

      per_client = {
        rate = "5mbit"
        priority = 1
        latency = "30ms"
      }
    },
    {
      name = "file_sharing"
      applications = ["bittorrent", "emule", "directconnect"]

      per_client = {
        rate = "1mbit"
        priority = 4
      }

      # Block during business hours
      time_block = {
        enabled = true
        time_range = "09:00-17:00"
        days = ["mon", "tue", "wed", "thu", "fri"]
      }
    }
  ]
}
```

### Bandwidth Quotas
```hcl
bandwidth {
  enabled = true

  # Quota configuration
  quotas = {
    enabled = true

    # Quota periods
    periods = [
      {
        name = "daily"
        duration = "24h"
        reset_time = "00:00"
      },
      {
        name = "weekly"
        duration = "7d"
        reset_time = "monday 00:00"
      },
      {
        name = "monthly"
        duration = "30d"
        reset_time = "1st 00:00"
      }
    ]

    # Quota policies
    policies = [
      {
        name = "guest_quota"
        clients = {
          subnet = "192.168.200.0/24"
        }

        quotas = [
          {
            period = "daily"
            upload_limit = "100MB"
            download_limit = "500MB"
          },
          {
            period = "weekly"
            upload_limit = "500MB"
            download_limit = "2GB"
          }
        ]

        # Exceeded action
        exceeded_action = "throttle"  # throttle, block, warn

        # Throttle settings
        throttle = {
          upload_rate = "128kbit"
          download_rate = "512kbit"
        }
      },
      {
        name = "student_quota"
        clients = {
          group = "students"
        }

        quotas = [
          {
            period = "daily"
            upload_limit = "1GB"
            download_limit = "5GB"
          }
        ]

        # Time-based exceptions
        exceptions = [
          {
            time_range = "20:00-23:00"
            days = ["sun", "mon", "tue", "wed", "thu"]
            multiplier = 2.0  # Double quota during study hours
          }
        ]
      }
    ]

    # Quota tracking
    tracking = {
      # What to track
      track = ["upload", "download", "total"]

      # Granularity
      granularity = "1min"

      # Retention
      retention = "90d"

      # Database
      database = {
        type = "sqlite"
        path = "/var/lib/flywall/bandwidth_quotas.db"
      }
    }
  }
}
```

### Dynamic Bandwidth Management
```hcl
bandwidth {
  enabled = true

  # Dynamic adjustment
  dynamic = {
    enabled = true

    # Monitoring
    monitoring = {
      interval = "30s"
      window = "5m"
      metrics = ["utilization", "latency", "packet_loss"]
    }

    # Adjustment policies
    policies = [
      {
        name = "congestion_control"

        # Trigger conditions
        triggers = [
          {
            metric = "utilization"
            operator = ">"
            value = 80
          },
          {
            metric = "latency"
            operator = ">"
            value = "100ms"
          }
        ]

        # Actions
        actions = [
          {
            type = "throttle_bulk"
            factor = 0.5
          },
          {
            type = "prioritize_interactive"
          },
          {
            type = "increase_queue_size"
            factor = 1.5
          }
        ]
      },
      {
        name = "off_peak_optimization"

        triggers = [
          {
            metric = "utilization"
            operator = "<"
            value = 30
          },
          {
            time_range = "01:00-06:00"
          }
        ]

        actions = [
          {
            type = "increase_limits"
            factor = 2.0
            classes = ["bulk", "default"]
          }
        ]
      }
    ]

    # Machine learning
    ml = {
      enabled = true

      # Learn patterns
      learn_patterns = true
      learning_period = "30d"

      # Predictive adjustments
      predictive = true

      # Model
      model = {
        type = "lstm"
        features = ["hour", "day", "utilization", "client_count"]
        predictions = ["utilization", "congestion"]
      }
    }
  }
}
```

## Implementation Details

### Bandwidth Control Flow
1. Packet enters interface
2. Classify traffic
3. Check rate limits
4. Apply shaping/policing
5. Queue packet
7. Transmit packet

### Token Bucket Algorithm
```
bucket_size = burst_size
fill_rate = rate_limit

if bucket_size > 0:
    bucket_size -= packet_size
    transmit_packet
else:
    drop_or_delay_packet

bucket_size = min(bucket_size + fill_rate * time, max_bucket_size)
```

## Testing

### Bandwidth Management Testing
```bash
# Test rate limit
iperf3 -c server -t 60 -b 100M

# Test with multiple streams
iperf3 -c server -P 4 -t 60

# Test latency
ping -c 100 8.8.8.8

# Check queue status
tc -s qdisc show dev eth0
```

### Integration Tests
- `bandwidth_test.sh`: Basic bandwidth control
- `rate_limit_test.sh`: Rate limiting
- `quota_test.sh`: Bandwidth quotas

## API Integration

### Bandwidth Management API
```bash
# Get bandwidth status
curl -s "http://localhost:8080/api/bandwidth/status"

# Get interface bandwidth
curl -s "http://localhost:8080/api/bandwidth/interfaces/eth0"

# Get client bandwidth
curl -s "http://localhost:8080/api/bandwidth/clients/192.168.1.100"

# Update limits
curl -X PUT "http://localhost:8080/api/bandwidth/clients/192.168.1.100" \
  -H "Content-Type: application/json" \
  -d '{
    "upload_rate": "50mbit",
    "download_rate": "100mbit"
  }'
```

### Quota API
```bash
# Get quotas
curl -s "http://localhost:8080/api/bandwidth/quotas"

# Get client quota usage
curl -s "http://localhost:8080/api/bandwidth/quotas/192.168.1.100/usage"

# Reset quota
curl -X POST "http://localhost:8080/api/bandwidth/quotas/192.168.1.100/reset"
```

## Best Practices

1. **Bandwidth Planning**
   - Understand traffic patterns
   - Set realistic limits
   - Monitor utilization
   - Adjust as needed

2. **Fairness**
   - Use fair queuing
   - Prevent starvation
   - Balance priorities
   - Consider user experience

3. **Performance**
   - Optimize queue sizes
   - Monitor latency
   - Choose right algorithms
   - Test thoroughly

4. **Monitoring**
   - Track usage patterns
   - Monitor congestion
   - Analyze performance
   - Generate reports

## Troubleshooting

### Common Issues
1. **Not shaping**: Check qdisc attachment
2. **Wrong limits**: Verify configuration
3. **High latency**: Check queue sizes
4. **Unfair distribution**: Review queue types

### Debug Commands
```bash
# Check qdiscs
tc qdisc show dev eth0

# Check classes
tc class show dev eth0

# Check filters
tc filter show dev eth0

# Monitor in real-time
watch -n 1 'tc -s class show dev eth0'
```

### Advanced Debugging
```bash
# Show detailed stats
tc -s qdisc show dev eth0

# Trace packet
tc -s filter show dev eth0 parent 1:

# Check bandwidth usage
iftop -i eth0

# Analyze traffic
nethogs -t eth0
```

## Performance Considerations

- Bandwidth control adds CPU overhead
- Complex rules increase latency
- Per-client rules don't scale well
- Hardware offload helps

## Security Considerations

- Bandwidth bypass attempts
- DoS via bandwidth exhaustion
- Priority escalation
- Traffic analysis resistance

## Related Features

- [QoS Features](qos-features.md)
- [Traffic Shaping](traffic-shaping.md)
- [Network Policies](network-policies.md)
- [Monitoring](monitoring.md)
