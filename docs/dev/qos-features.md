# QoS Features Implementation Guide

## Overview

Flywall provides comprehensive Quality of Service (QoS) features for:
- Traffic shaping and prioritization
- Bandwidth management
- Latency control
- Traffic classification
- Fair queueing

## Architecture

### QoS Components
1. **Traffic Classifier**: Identifies traffic types
2. **Queue Manager**: Manages packet queues
3. **Shaper**: Controls traffic rates
4. **Scheduler**: Determines packet transmission order
5. **Monitor**: Tracks QoS performance

### QoS Mechanisms
- **HTB**: Hierarchical Token Bucket
- **HFSC**: Hierarchical Fair Service Curve
- **CBQ**: Class-Based Queueing
- **SFQ**: Stochastic Fairness Queueing
- **CoDel**: Controlled Delay

## Configuration

### Basic QoS Setup
```hcl
# QoS configuration
qos {
  enabled = true

  # Default class
  default_class = "best_effort"

  # Root queue
  root_queue = {
    rate = "1gbit"
    ceil = "1gbit"
  }
}
```

### Advanced QoS Configuration
```hcl
qos {
  enabled = true

  # Interface settings
  interfaces = [
    {
      name = "eth0"
      egress_rate = "1gbit"
      ingress_rate = "1gbit"

      # Root qdisc
      root_qdisc = {
        type = "htb"
        default_class = 30
      }

      # Classes
      classes = [
        {
          id = 10
          name = "voip"
          parent = "root"
          rate = "100mbit"
          ceil = "500mbit"
          priority = 1

          # Match criteria
          match = {
            protocol = "udp"
            dport = [5060, 5061]
            dport_range = "10000:20000"
          }

          # Queue settings
          queue = {
            type = "sfq"
            perturb = 10
          }
        },
        {
          id = 20
          name = "video"
          parent = "root"
          rate = "200mbit"
          ceil = "800mbit"
          priority = 2

          match = {
            protocol = "tcp"
            dport = [443, 8080]
            dscp = "af41"
          }
        },
        {
          id = 25
          name = "business_critical"
          parent = "root"
          rate = "300mbit"
          ceil = "900mbit"
          priority = 2

          match = {
            src_ip = ["192.168.1.0/24"]
            dst_ip = ["10.0.0.0/8"]
            dscp = "af31"
          }
        },
        {
          id = 30
          name = "best_effort"
          parent = "root"
          rate = "200mbit"
          ceil = "1gbit"
          priority = 3

          # Default class
          default = true
        },
        {
          id = 40
          name = "low_priority"
          parent = "root"
          rate = "50mbit"
          ceil = "200mbit"
          priority = 4

          match = {
            protocol = "tcp"
            dport = [6881, 6882, 6883, 6884, 6885]
          }
        }
      ]
    }
  ]
}
```

### Zone-Based QoS
```hcl
zone "WAN" {
  interface = "eth0"

  qos = {
    enabled = true

    # Upload shaping
    egress = {
      rate = "500mbit"
      burst = "10mbit"

      # Traffic classes
      classes = [
        {
          name = "interactive"
          rate = "100mbit"
          priority = 1

          match = {
            dscp = ["cs6", "cs7"]
            protocol = ["tcp", "udp"]
            packet_size = "0-1200"
          }
        },
        {
          name = "bulk"
          rate = "300mbit"
          priority = 3

          match = {
            dscp = "af11"
            packet_size = "1200-"
          }
        }
      ]
    }

    # Download shaping
    ingress = {
      rate = "1gbit"

      # Policing (drop excess)
      police = {
        rate = "800mbit"
        burst = "20mbit"
        action = "drop"
      }
    }
  }
}

zone "Guest" {
  interface = "eth2"

  qos = {
    enabled = true

    # Strict limits for guests
    egress = {
      rate = "100mbit"
      ceil = "200mbit"

      # Per-client limits
      per_client = {
        rate = "5mbit"
        ceil = "10mbit"
      }
    }

    ingress = {
      rate = "200mbit"

      per_client = {
        rate = "10mbit"
        ceil = "20mbit"
      }
    }

    # Blocked applications
    block = {
      protocols = ["bittorrent", "emule"]
      ports = [6881, 6882, 6883, 6884, 6885]
    }
  }
}
```

### Application-Based QoS
```hcl
qos {
  enabled = true

  # Application definitions
  applications = [
    {
      name = "voip"
      description = "Voice over IP"

      match = {
        protocol = "udp"
        dport = [5060, 5061]
        dport_range = "10000:20000"
        packet_size = "20-200"
      }

      qos = {
        priority = 1
        dscp = "ef"
        rate = "2mbit"
        latency = "50ms"
        jitter = "10ms"
      }
    },
    {
      name = "video_streaming"
      description = "Video streaming"

      match = {
        protocol = "tcp"
        dport = [443, 8080, 1935]
        dscp = "af41"
        flow_size = "1MB-"
      }

      qos = {
        priority = 2
        dscp = "af41"
        rate = "10mbit"
        buffer = "2MB"
      }
    },
    {
      name = "gaming"
      description = "Online gaming"

      match = {
        protocol = "udp"
        packet_size = "40-1400"
        rate = "0-1mbit"
      }

      qos = {
        priority = 1
        dscp = "cs5"
        latency = "30ms"
      }
    },
    {
      name = "file_transfer"
      description = "File transfers"

      match = {
        protocol = "tcp"
        flow_size = "10MB-"
        duration = "60s-"
      }

      qos = {
        priority = 4
        dscp = "af11"
        rate = "50mbit"
      }
    }
  ]

  # Application policies
  policies = [
    {
      name = "work_hours"
      time = "08:00-18:00"
      days = ["mon", "tue", "wed", "thu", "fri"]

      # Prioritize business apps
      prioritize = ["voip", "video_conferencing", "business_critical"]
      limit = ["file_transfer", "streaming"]
    },
    {
      name = "after_hours"
      time = "18:00-08:00"
      days = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]

      # More relaxed
      prioritize = ["voip", "gaming", "streaming"]
    }
  ]
}
```

### Dynamic QoS
```hcl
qos {
  enabled = true

  # Dynamic bandwidth allocation
  dynamic = {
    enabled = true

    # Total bandwidth
    total_bandwidth = "1gbit"

    # Minimum guarantees
    guarantees = {
      voip = "10mbit"
      business = "100mbit"
      guest = "50mbit"
    }

    # Allocation strategy
    strategy = "fair_share"  # fair_share, priority, proportional

    # Adjustments
    adjustments = {
      interval = "30s"
      threshold = 80  # percent utilization
      factor = 0.1    # adjustment factor
    }
  }

  # Adaptive shaping
  adaptive = {
    enabled = true

    # Latency targets
    latency_targets = {
      interactive = "50ms"
      streaming = "200ms"
      bulk = "1000ms"
    }

    # Monitoring
    monitor = {
      interval = "10s"
      window = "1m"
      samples = 6
    }

    # Actions
    actions = {
      high_latency = "reduce_rate"
      packet_loss = "increase_buffer"
      congestion = "re_prioritize"
    }
  }
}
```

## Implementation Details

### QoS Hierarchy
```
Root (1gbit)
├── Class 10: VoIP (100mbit/500mbit)
├── Class 20: Video (200mbit/800mbit)
├── Class 25: Business (300mbit/900mbit)
├── Class 30: Best Effort (200mbit/1gbit)
└── Class 40: Low Priority (50mbit/200mbit)
```

### Traffic Classification
```go
type TrafficClass struct {
    ID       int      `json:"id"`
    Name     string   `json:"name"`
    Rate     string   `json:"rate"`
    Ceil     string   `json:"ceil"`
    Priority int      `json:"priority"`
    Match    Match    `json:"match"`
    Queue    Queue    `json:"queue"`
}

type Match struct {
    Protocol    string   `json:"protocol"`
    SrcIP       []string `json:"src_ip"`
    DstIP       []string `json:"dst_ip"`
    SrcPort     []int    `json:"sport"`
    DstPort     []int    `json:"dport"`
    DSCP        string   `json:"dscp"`
    PacketSize  string   `json:"packet_size"`
}
```

## Testing

### QoS Testing
```bash
# Test bandwidth limit
iperf3 -c server -t 60

# Test latency
ping -c 100 8.8.8.8

# Check QoS statistics
tc -s class show dev eth0

# Monitor queue lengths
watch -n 1 'tc -s qdisc show dev eth0'
```

### Integration Tests
- `qos_test.sh`: Basic QoS functionality
- `shaper_test.sh`: Traffic shaping
- `priority_test.sh`: Priority queuing

## API Integration

### QoS API
```bash
# Get QoS status
curl -s "http://localhost:8080/api/qos/status"

# Get interface QoS
curl -s "http://localhost:8080/api/qos/interfaces/eth0"

# Get class statistics
curl -s "http://localhost:8080/api/qos/classes"

# Update QoS settings
curl -X PUT "http://localhost:8080/api/qos/interfaces/eth0" \
  -H "Content-Type: application/json" \
  -d '{
    "egress_rate": "600mbit"
  }'
```

### Statistics API
```bash
# Get QoS statistics
curl -s "http://localhost:8080/api/qos/stats"

# Get class statistics
curl -s "http://localhost:8080/api/qos/stats/classes"

# Get application statistics
curl -s "http://localhost:8080/api/qos/stats/applications"
```

## Best Practices

1. **Bandwidth Planning**
   - Understand traffic patterns
   - Set realistic limits
   - Monitor utilization
   - Adjust as needed

2. **Class Design**
   - Keep classes simple
   - Use clear priorities
   - Document purposes
   - Test thoroughly

3. **Performance**
   - Monitor latency
   - Check for congestion
   - Optimize queue sizes
   - Balance priorities

4. **Troubleshooting**
   - Use traffic generators
   - Monitor statistics
   - Check packet drops
   - Verify classification

## Troubleshooting

### Common Issues
1. **No shaping applied**: Check qdisc attachment
2. **Wrong classification**: Verify match rules
3. **High latency**: Check queue sizes
4. **Packet loss**: Review rate limits

### Debug Commands
```bash
# Check QoS configuration
tc qdisc show dev eth0

# Check class statistics
tc -s class show dev eth0

# Monitor filters
tc -s filter show dev eth0

# Check packet classification
tc -s filter show dev eth0 parent 1:
```

### Advanced Debugging
```bash
# Trace packet through QoS
tc -s class show dev eth0

# Check queue discipline
tc -s qdisc show dev eth0

# Monitor real-time
watch -n 1 'tc -s class show dev eth0 | grep -E "(Sent|rate)"'

# Test with specific traffic
tcpreplay -i eth0 test.pcap
```

## Performance Considerations

- QoS adds CPU overhead
- Complex rules increase latency
- Per-packet processing cost
- Memory for queue management

## Security Considerations

- QoS bypass attempts
- DoS via QoS exhaustion
- Priority escalation
- Traffic analysis resistance

## Related Features

- [Bandwidth Management](bandwidth-mgmt.md)
- [Traffic Shaping](traffic-shaping.md)
- [Network Policies](network-policies.md)
- [Monitoring](monitoring.md)
