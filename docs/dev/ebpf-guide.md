# eBPF Developer Guide for Flywall

> **Comprehensive guide for eBPF development, testing, and integration in Flywall**

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Implementation Strategy](#implementation-strategy)
3. [Feature Mapping](#feature-mapping)
4. [Testing](#testing)
5. [Performance Considerations](#performance-considerations)
6. [Development Workflow](#development-workflow)
7. [Troubleshooting](#troubleshooting)

---

## Architecture Overview

### Internal eBPF Design Philosophy

Flywall uses eBPF as an **internal implementation detail**, not as a user-facing extensibility mechanism. All eBPF programs are:
- Written as part of Flywall source code
- Compiled during the Flywall build process
- Embedded in the binary
- Loaded at startup based on configuration
- Managed entirely by the Flywall control plane

```
┌─────────────────────────────────────────┐
│           Flywall Userspace             │
│  ┌─────────────┐  ┌─────────────────┐   │
│  │   IPS       │  │  Learning       │   │
│  │   Engine    │  │  Engine         │   │
│  └─────────────┘  └─────────────────┘   │
│           │                │            │
│  ┌─────────────────────────────────┐    │
│  │    Internal eBPF Manager        │    │
│  │  (loads/manages internal progs) │    │
│  └─────────────────────────────────┘    │
└─────────────────────────────────────────┘
                    │
           ┌────────▼────────┐
           │   Kernel eBPF   │
           │  (internal progs)│
           └─────────────────┘
```

### Key Principles

- **No external program support** - No program directory configuration
- **No runtime loading** of user programs
- **No API endpoints** for program management
- **No compilation** of user eBPF code
- **Automatic feature detection** - Features use eBPF when available, fallback gracefully

### eBPF Hook Points

| Hook Point | Use Case | Performance |
|------------|----------|-------------|
| **XDP** | Early packet filtering (DDoS, DNS blocklist) | Wire-speed |
| **TC Ingress/Egress** | Traffic control, flow classification, QoS | Near wire-speed |
| **Socket Filters** | Protocol-specific monitoring (DNS, TLS, DHCP) | Low overhead |
| **Tracepoints** | Kernel event tracing | Minimal |
| **Kprobes** | Kernel function probing | Low overhead |

---

## Implementation Strategy

### Phase 1: Foundation

**Core Components:**
- **Program Loader** (`internal/ebpf/loader/`) - ELF loading, verification, attachment
- **Map Manager** (`internal/ebpf/maps/`) - Hash maps, per-CPU arrays, LRU maps, ring buffers
- **Hook Manager** (`internal/ebpf/hooks/`) - XDP, TC, socket filter attachment
- **Feature Coordinator** (`internal/ebpf/coordinator/`) - Cross-feature interactions

**Embedded Programs:**

```go
//go:embed xdp_blocklist.o
var xdpBlocklistBytes []byte

//go:embed tc_classifier.o
var tcClassifierBytes []byte

//go:embed socket_dns.o
var socketDNSBytes []byte
```

### Phase 2: XDP Fast Path (Weeks 5-8)

Unified XDP program for early packet processing:

```c
SEC("xdp")
int xdp_unified_processor(struct xdp_md *ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;

    // Update global statistics
    increment_counter(COUNTER_PACKETS_TOTAL);

    // IPv4 processing
    if (eth->h_proto == __constant_htons(ETH_P_IP)) {
        struct iphdr *ip = (void *)(eth + 1);
        
        // DNS Blocklist - highest priority
        if (ip->protocol == IPPROTO_UDP) {
            struct udphdr *udp = (void *)(ip + 1);
            if (udp->dest == __constant_htons(53)) {
                if (process_dns_blocklist(ctx, ip, udp) == XDP_DROP) {
                    increment_counter(COUNTER_DNS_BLOCKED);
                    return XDP_DROP;
                }
            }
        }

        // DDoS Protection
        if (is_ip_blocked(ip->saddr) || rate_exceeded(ip->saddr)) {
            return XDP_DROP;
        }
    }

    return XDP_PASS;
}
```

### Phase 3: TC-Based Hybrid Offload (Weeks 9-12)

**The Hybrid Handshake:**

Instead of replacing NFQUEUE, use TC to accelerate flows *after* they have been vetted by the userspace Learning Engine.

**Flow Lifecycle:**
1. **New Flow:** Packets pass XDP/TC → hit `nftables` → **NFQUEUE** (Userspace Go)
2. **Inspection:** Go engine performs DPI, TLS JA3, and state tracking
3. **Approval:** Once flow is trusted (packet_window > N), Go updates the **eBPF Flow Map**
4. **Fast Path:** Subsequent packets match in **TC Ingress**

```c
SEC("tc")
int tc_fast_path(struct __sk_buff *skb) {
    struct flow_key key = {};
    if (extract_flow_key(skb, &key) < 0)
        return TC_ACT_OK;

    // LOOKUP ONLY - Do not create new state here!
    struct flow_state *state = bpf_map_lookup_elem(&flow_map, &key);

    if (!state) {
        // Unknown flow -> Pass to Stack -> NFQUEUE
        return TC_ACT_OK;
    }

    // Update stats for the Go engine to read later
    __sync_fetch_and_add(&state->packet_count, 1);
    __sync_fetch_and_add(&state->byte_count, skb->len);

    // Trusted Flow Logic
    if (state->verdict == VERDICT_TRUSTED) {
        skb->mark = state->offload_mark;
        return TC_ACT_OK;
    }

    // Blocked Flow Logic (Early Drop)
    if (state->verdict == VERDICT_DROP) {
        return TC_ACT_SHOT;
    }

    return TC_ACT_OK;
}
```

### Phase 4: Socket Filters (Weeks 13-16)

Multi-protocol socket filters for deep inspection:

```c
// DNS Monitoring
SEC("socket_filter")
int dns_monitor_socket(struct __sk_buff *skb) {
    if (is_dns_query(skb)) {
        struct dns_event event = {
            .timestamp = bpf_ktime_get_ns(),
            .query_type = extract_dns_type(skb),
            .domain_hash = hash_domain(skb),
        };

        // Check blocklist first
        if (is_domain_blocked(event.domain_hash)) {
            increment_counter(COUNTER_DNS_BLOCKED_SOCKET);
            return 0;  // Drop packet
        }

        // Send to learning engine
        send_feature_event(skb, extract_flow_key(skb),
                          EVENT_DNS_QUERY, &event, sizeof(event));
    }
    return 0;
}

// TLS Monitoring
SEC("socket_filter")
int tls_monitor_socket(struct __sk_buff *skb) {
    if (is_tls_handshake(skb)) {
        struct tls_event event = {
            .timestamp = bpf_ktime_get_ns(),
            .ja3_partial = extract_ja3_partial(skb),
            .sni_hash = extract_sni_hash(skb),
        };
        
        // Update flow state with TLS info
        struct flow_key key = extract_flow_key(skb);
        struct flow_state *state = bpf_map_lookup_elem(&flow_map, &key);
        if (state) {
            state->tls_detected = 1;
            state->ja3_hash = event.ja3_partial;
        }

        send_feature_event(skb, &key, EVENT_TLS_HANDSHAKE,
                          &event, sizeof(event));
    }
    return 0;
}
```

### Phase 5: Advanced Features (Weeks 17-20)

- **Kprobe-based monitoring** - System-level anomaly detection
- **Hardware offload** - NIC acceleration support
- **Performance optimization** - Map prefetching, batch updates

---

## Feature Mapping

### Current vs eBPF Implementation

#### 1. Inline IPS

| Current (NFQUEUE) | Hybrid eBPF (Target) | Benefits |
| --- | --- | --- |
| NFQUEUE for ALL packets | NFQUEUE for First-N only | DPI + Speed |
| Userspace Inspection | Userspace (Handshake) + TC (Data) | Complex logic in Go |
| Fail-open on queue full | TC Default Pass | Robustness |
| Performance limit ~1M pps | TC Fast Path ~10M pps | Wire-speed for bulk |

#### 2. Learning Engine

| Current | eBPF Enhancement | Benefits |
|---------|------------------|----------|
| NFLOG packet capture | Ring buffer events | Async, higher throughput |
| SQLite flow storage | eBPF maps + periodic sync | Real-time processing |
| Userspace pattern detection | Kernel-level aggregation | Reduced CPU |
| Batch rule generation | Event-driven suggestions | Immediate response |

#### 3. DDoS Protection

| Current | eBPF Implementation | Benefits |
|---------|---------------------|----------|
| nftables rate limits | XDP with token bucket | Early drop, wire speed |
| IP blocklists | eBPF hash map | O(1) lookup, millions of entries |
| Connection tracking | XDP flow state | No conntrack overhead |
| SYN cookies | XDP SYN validation | Prevents SYN flood at NIC |

#### 4. DNS Security

| Current | eBPF Enhancement | Benefits |
|---------|------------------|----------|
| Userspace DNS inspection | Socket filter on port 53 | Zero-copy inspection |
| Query logging | eBPF perf events | Minimal overhead |
| Response validation | Kernel-level validation | Faster response |

### Shared Infrastructure Benefits

- **Single flow state map** used by IPS, QoS, Learning, and TLS
- **Unified statistics** across all features
- **Common event system** for efficient userspace communication
- **Coordinated program loading** to ensure proper ordering

---

## Testing

### Test Categories

#### 1. Unit Tests
- **Location**: `internal/ebpf/*/...`
- **Naming**: `TestUnit*` or any test not containing "Integration"
- **Requirements**: None (can run as regular user)
- **Command**: `go test -race ./internal/ebpf/... -run "^Test[^I]"`

#### 2. Integration Tests
- **Location**: `internal/ebpf/...`
- **Naming**: Contains "Integration" in name
- **Requirements**: Root privileges or VM environment

#### 3. End-to-End Tests
- **Location**: `internal/ebpf/integration_e2e_test.go`
- **Requirements**: Root privileges, full system setup

#### 4. Performance Tests
- **Location**: `internal/ebpf/performance/...`
- **Requirements**: Root privileges, resource-intensive

#### 5. Control Plane Tests
- **Location**: `internal/ebpf/controlplane/...`
- **Requirements**: Root privileges

### Running Tests Locally

```bash
# Unit tests only (no root required)
go test -race ./internal/ebpf/... -run "^Test[^I]"

# All tests (requires root)
sudo go test -v ./internal/ebpf/...

# In VM
./flywall.sh vm start
./flywall.sh vm exec "go test -v ./internal/ebpf/..."
```

### eBPF Programs Tested

1. **TC Offload** (`tc_offload.c`)
   - 2 programs: `tc_fast_path`, `tc_egress_fast_path`
   - 3 maps: `flow_map`, `qos_profiles`, `tc_stats_map`

2. **DNS Socket** (`dns_socket.c`)
   - 1 program: DNS filter
   - 4 maps: DNS tracking, statistics

3. **DHCP Socket** (`dhcp_socket.c`)
   - 1 program: DHCP filter
   - 6 maps: DHCP lease tracking

4. **XDP Blocklist** (`xdp_blocklist.c`)
   - 1 program: XDP-based packet filtering
   - 6 maps: IP blocklist, flow tracking

---

## Performance Considerations

### Performance Multiplication

| Layer | Packet Handling | Use Case |
|-------|-----------------|----------|
| XDP | 80% of packets | Simple rules (DDoS, DNS blocklist) |
| TC | 19% of packets | Complex inspection (IPS, QoS) |
| Socket Filters | 1% of packets | Protocol-specific (DNS, TLS, DHCP) |
| **Result** | **10x overall improvement** |  |

### Optimization Techniques

1. **Per-CPU Maps** - Avoid locking overhead
2. **Bloom Filters** - O(1) lookup for large datasets
3. **Batch Updates** - Reduce map update overhead
4. **Map Prefetching** - Cache optimization
5. **Hardware Offload** - NIC acceleration when available

### Expected Outcomes

| Metric | Current | With eBPF | Improvement |
|--------|---------|-----------|-------------|
| Packet Processing | 1M pps | 10M pps | 10x |
| CPU Usage | 30% | 5% | 6x |
| Latency | 100μs | 10μs | 10x |
| Feature Overhead | High | Minimal | 5x |

---

## Development Workflow

### Build System Integration

```makefile
# eBPF compilation
EBPF_DIR = internal/ebpf/programs
EBPF_OUTPUT = build/ebpf

.PHONY: ebpf
ebpf:
	@mkdir -p $(EBPF_OUTPUT)
	for prog in $(EBPF_DIR)/*.c; do \
		$(CLANG) -O2 -target bpf -c $$prog -o $(EBPF_OUTPUT)/$$(basename $$prog .c).o; \
	done

# Include in main build
build: ebpf
```

### Adding New eBPF Features

1. **Write eBPF program** in `internal/ebpf/programs/`
2. **Add embedding** in `internal/ebpf/programs.go`
3. **Create loader** in `internal/ebpf/loader/`
4. **Add tests** in appropriate test file
5. **Update coordinator** for cross-feature integration

### Map Types Reference

| Type | Use Case | Complexity |
|------|----------|------------|
| `BPF_MAP_TYPE_HASH` | Key-value store | O(1) |
| `BPF_MAP_TYPE_ARRAY` | Fixed-size array | O(1) |
| `BPF_MAP_TYPE_PERCPU_ARRAY` | Per-CPU counters | O(1), no locking |
| `BPF_MAP_TYPE_LRU_HASH` | Cache with eviction | O(1) |
| `BPF_MAP_TYPE_RINGBUF` | Event streaming | Lock-free |
| `BPF_MAP_TYPE_BLOOM_FILTER` | Fast membership test | O(1) |

---

## Troubleshooting

### Common Issues

#### 1. Program Load Failure
- Check verifier output: `bpftool prog dump xlated id 1`
- Kernel logs: `dmesg | grep -i bpf`
- Verify eBPF support: `ls /proc/sys/net/core/bpf_*`

#### 2. Map Access Errors
- Verify map permissions
- Check map size limits
- Ensure map is pinned if needed

#### 3. Performance Issues
- Profile with: `perf record -e bpf_output_call`
- Check for excessive map lookups
- Verify JIT compilation is enabled

#### 4. Integration Test Failures
- Ensure VM is running: `./flywall.sh vm status`
- Check root privileges in VM
- Verify eBPF kernel support

### Debug Commands

```bash
# Check eBPF status
bpftool prog show

# Verify program
bpftool prog dump xlated id 1

# Check verifier log
dmesg | grep -i bpf

# Monitor maps
watch -n 1 'bpftool map list'

# Trace eBPF events
bpftrace -e 'tracepoint:syscalls:sys_enter_* { @[comm] = count(); }'

# Profile program
perf record -e bpf_output_call
perf report
```

### Best Practices

1. **Program Design**
   - Keep programs small and simple
   - Use bounded loops
   - Verify packet bounds
   - Handle errors gracefully

2. **Security**
   - Validate all inputs
   - Use CAP_SYS_ADMIN sparingly
   - Limit map sizes
   - Audit program code

3. **Debugging**
   - Use bpf_printk for logging
   - Monitor with bpftool
   - Test with various traffic
   - Check kernel logs

---

## Related Documents

- [Architecture Overview](../ARCHITECTURE.md)
- [Inline IPS](inline-ips.md)
- [Learning Engine Guide](learning-engine.md)
- [ADR-001: Hybrid eBPF/NFQUEUE Architecture](../design/ADR-001-hybrid-ebpf-nfqueue.md)

---

*This document compiled from the eBPF documentation suite in docs/dev/ebpf*.md*
