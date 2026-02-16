# eBPF Integration Implementation Guide

## Overview

Flywall supports eBPF for high-performance packet processing:
- XDP programs for early packet filtering
- TC programs for traffic control
- Socket filters for monitoring
- kprobes for kernel instrumentation
- Maps for data sharing

## Architecture

### eBPF Components
1. **Program Loader**: Loads and verifies eBPF programs
2. **Map Manager**: Handles eBPF maps
3. **Hook Manager**: Attaches programs to kernel hooks
4. **Collector**: Gathers data from eBPF programs
5. **Compiler**: Compiles eBPF bytecode

### eBPF Hook Points
- XDP: Early packet processing
- TC Ingress/Egress: Traffic control
- Socket Filter: Per-socket filtering
- Tracepoint: Kernel event tracing
- Kprobe: Kernel function probing

## Configuration

### Basic eBPF Setup
```hcl
# Enable eBPF
ebpf {
  enabled = true

  # Program directory
  program_dir = "/etc/flywall/ebpf"

  # Map settings
  map_size = 1024

  # JIT compilation
  jit = true
}
```

### XDP Programs
```hcl
ebpf {
  enabled = true

  # XDP program for DDoS protection
  xdp "ddos_filter" {
    program = "/etc/flywall/ebpf/ddos_filter.o"
    interface = "eth0"

    # Map configuration
    maps = {
      "blocklist" = {
        type = "hash"
        size = 100000
      }
      "rate_limit" = {
        type = "percpu_array"
        size = 1024
      }
    }

    # Program arguments
    args = {
      max_pps = 1000000
      blocklist_size = 100000
    }
  }
}
```

### TC Programs
```hcl
ebpf {
  enabled = true

  # TC ingress program
  tc "ingress_filter" {
    program = "/etc/flywall/ebpf/ingress_filter.o"
    interface = "eth0"
    direction = "ingress"

    # Maps
    maps = {
      "conntrack" = {
        type = "lru_hash"
        size = 1000000
      }
      "counters" = {
        type = "percpu_counter"
        size = 64
      }
    }
  }

  # TC egress program
  tc "egress_shaper" {
    program = "/etc/flywall/ebpf/egress_shaper.o"
    interface = "eth0"
    direction = "egress"

    # Traffic shaping
    shaping = {
      rate = "1gbit"
      burst = "100mbit"
    }
  }
}
```

### Socket Filters
```hcl
ebpf {
  enabled = true

  # Socket filter for monitoring
  socket_filter "monitor" {
    program = "/etc/flywall/ebpf/socket_monitor.o"

    # Attach to specific sockets
    sockets = ["dns", "dhcp"]

    # Monitoring maps
    maps = {
      "stats" = {
        type = "percpu_array"
        size = 256
      }
    }
  }
}
```

### Tracepoints and Kprobes
```hcl
ebpf {
  enabled = true

  # Tracepoint for network events
  tracepoint "network_events" {
    program = "/etc/flywall/ebpf/net_trace.o"
    tracepoint = "net:netif_receive_skb"

    # Event buffer
    maps = {
      "events" = {
        type = "perf_event_array"
        size = 1024
      }
    }
  }

  # Kprobe for function tracing
  kprobe "tcp_trace" {
    program = "/etc/flywall/ebpf/tcp_trace.o"
    function = "tcp_v4_connect"

    maps = {
      "stack_traces" = {
        type = "stack_trace"
        size = 1024
      }
    }
  }
}
```

## Implementation Details

### eBPF Program Structure
```c
// XDP program example
SEC("xdp")
int xdp_ddos_filter(struct xdp_md *ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;

    // Check blocklist
    if (is_blocked(eth->h_source)) {
        __sync_fetch_and_add(&blocked_count, 1);
        return XDP_DROP;
    }

    // Rate limiting
    if (rate_exceeded(eth->h_source)) {
        return XDP_DROP;
    }

    return XDP_PASS;
}

// Map definitions
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 100000);
} blocklist_map SEC(".maps");
```

### Map Types
- **Hash**: Key-value store
- **Array**: Fixed-size array
- **PerCPU**: Per-CPU data
- **LRU**: Least recently used
- **PerfEvent**: Event buffer
- **StackTrace**: Stack traces

## Testing

### eBPF Program Testing
```bash
# Compile eBPF program
clang -O2 -target bpf -c ebpf_program.c -o ebpf_program.o

# Load and test
bpftool prog load ebpf_program.o /sys/fs/bpf/program

# Attach to interface
bpftool net attach xdp prog /sys/fs/bpf/program dev eth0

# Check maps
bpftool map list

# Read map values
bpftool map lookup id 1 key 0x12345678
```

### Integration Testing
```bash
# Test XDP program
ping -c 1 -s 1500 target_ip
tcpreplay -i eth0 test.pcap

# Monitor eBPF statistics
bpftool prog show

# Check drop rates
ethtool -S eth0 | grep rx_xdp_drop
```

## API Integration

### eBPF Management API
```bash
# List eBPF programs
curl -s "http://localhost:8080/api/ebpf/programs"

# Get program details
curl -s "http://localhost:8080/api/ebpf/programs/ddos_filter"

# Load program
curl -X POST "http://localhost:8080/api/ebpf/programs/load" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "new_filter",
    "program": "/path/to/program.o",
    "interface": "eth0"
  }'

# Update program
curl -X PUT "http://localhost:8080/api/ebpf/programs/ddos_filter" \
  -H "Content-Type: application/json" \
  -d '{
    "args": {"max_pps": 2000000}
  }'

# Get map data
curl -s "http://localhost:8080/api/ebpf/maps/blocklist/data"
```

### Statistics API
```bash
# Get eBPF statistics
curl -s "http://localhost:8080/api/ebpf/stats"

# Get program counters
curl -s "http://localhost:8080/api/ebpf/programs/ddos_filter/stats"

# Get map statistics
curl -s "http://localhost:8080/api/ebpf/maps/counters/stats"
```

## Best Practices

1. **Program Design**
   - Keep programs small and simple
   - Use bounded loops
   - Verify packet bounds
   - Handle errors gracefully

2. **Performance**
   - Use per-CPU maps
   - Minimize map lookups
   - Batch operations
   - Use JIT compilation

3. **Security**
   - Validate all inputs
   - Use CAP_SYS_ADMIN sparingly
   - Limit map sizes
   - Audit program code

4. **Debugging**
   - Use bpf_printk for logging
   - Monitor with bpftool
   - Test with various traffic
   - Check kernel logs

## Troubleshooting

### Common Issues
1. **Program load failure**: Check verifier output
2. **Map access errors**: Verify map permissions
3. **Performance issues**: Check for bottlenecks
4. **Kernel compatibility**: Check eBPF features

### Debug Commands
```bash
# Check eBPF status
flywall ebpf status

# Verify program
bpftool prog dump xlated id 1

# Check verifier log
dmesg | grep -i bpf

# Monitor maps
watch -n 1 'bpftool map list'
```

### Advanced Debugging
```bash
# Trace eBPF events
bpftrace -e 'tracepoint:syscalls:sys_enter_* { @[comm] = count(); }'

# Profile eBPF program
perf record -e bpf_output_call
perf report

# Debug with bcc tools
tcplife -p $(pidof flywall)
```

## Performance Considerations

- XDP provides highest performance
- TC offers more flexibility
- Map access is O(1) for hash/array
- Per-CPU maps avoid locking

## Security Considerations

- Programs run in kernel space
- Require CAP_SYS_ADMIN
- Verified by kernel verifier
- Limited instruction set

## Related Features

- [Inline IPS](inline-ips.md)
- [Protection Features](protection-features.md)
- [Metrics Collection](metrics-collection.md)
- [Performance Tuning](performance-tuning.md)
