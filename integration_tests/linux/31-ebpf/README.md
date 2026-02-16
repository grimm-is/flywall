# eBPF Integration Tests

This directory contains comprehensive integration tests for all eBPF features in Flywall.

## Important: Configuration Boilerplate

All eBPF test configs MUST include the standard boilerplate to avoid safemode activation. The boilerplate includes:

- Schema version
- Interface configuration (eth0)
- Zone definitions
- API settings
- Logging configuration
- Control plane settings

See `common_ebpf_config.hcl` for the complete boilerplate.

### Using the Boilerplate

**Method 1: Use common.sh function (recommended)**
```bash
# In your test:
CONFIG_FILE=$(mktemp_compatible test.hcl)
ebpf_config_file "$CONFIG_FILE"  # Adds boilerplate

# Append your eBPF specific config
cat >> "$CONFIG_FILE" <<'EOF'
ebpf {
    enabled = true
    features {
        ddos_protection = true
    }
}
EOF
```

**Method 2: Generate config string**
```bash
# Get config as string and append
CONFIG_FILE=$(mktemp_compatible test.hcl)
{
    ebpf_config
    echo "ebpf {"
    echo "  enabled = true"
    echo "}"
} > "$CONFIG_FILE"
```

**Method 3: Include directly (for special cases)**
```bash
cat > "$CONFIG_FILE" <<'EOF'
schema_version = "1.0"
ip_forwarding = true
# ... full boilerplate
EOF
```

## Test Suite Overview

### Test Files

1. **ebpf_foundation_test.sh** - Core eBPF infrastructure tests
   - Program loading and verification
   - Map creation and operations
   - Hook attachment
   - Feature coordination
   - Configuration loading
   - Resource limits
   - Cleanup procedures

2. **xdp_blocklist_test.sh** - XDP-based fast path tests
   - XDP program attachment
   - IP blocklist functionality
   - DNS blocklist integration
   - Statistics collection
   - Performance under load
   - Dynamic updates
   - Rate limiting

3. **tc_classifier_test.sh** - TC-based flow classification tests
   - TC program attachment
   - Flow state management
   - IPS verdict application
   - Flow offloading
   - QoS classification
   - Learning engine integration
   - NFQUEUE migration
   - Performance metrics

4. **socket_filters_test.sh** - Socket filter tests
   - DNS monitoring
   - TLS fingerprinting
   - DHCP monitoring
   - Device discovery
   - ARP tracking
   - Cross-feature correlation
   - Event rate limiting

5. **performance_benchmark_test.sh** - Performance benchmarks
   - XDP packet processing (>5M pps)
   - TC flow classification (>100K fps)
   - Memory usage under load
   - CPU usage under load
   - Event generation rates
   - Adaptive scaling
   - Concurrent feature performance
   - Map lookup performance
   - Latency measurements
   - Regression detection

6. **feature_interaction_test.sh** - Cross-feature interaction tests
   - Dependency resolution
   - DNS-QoS interaction
   - TLS-aware IPS
   - Learning engine events
   - Statistics aggregation
   - Priority-based scaling
   - Shared state consistency
   - Configuration propagation
   - Graceful degradation

7. **fallback_mechanisms_test.sh** - Fallback and degradation tests
   - eBPF disabled fallback
   - Partial support handling
   - Program load failures
   - Map creation failures
   - Hook attachment failures
   - Verifier rejection
   - Runtime error handling
   - Performance degradation
   - Complete failure recovery

## Running Tests

### Individual Tests

```bash
# Run single test
./flywall.sh test int 31-ebpf/ebpf_foundation_test.sh

# Run with verbose output
./flywall.sh test int 31-ebpf/ebpf_foundation_test.sh --verbose

# Run all eBPF tests
./flywall.sh test int 31-ebpf
```

### Test Categories

```bash
# Foundation tests only
./flywall.sh test int 31-ebpf/ebpf_foundation_test.sh

# XDP blocklist tests
./flywall.sh test int 31-ebpf/xdp_blocklist_test.sh

# TC classifier tests
./flywall.sh test int 31-ebpf/tc_classifier_test.sh

# Socket filter tests
./flywall.sh test int 31-ebpf/socket_filters_test.sh

# Performance benchmarks
./flywall.sh test int 31-ebpf/performance_benchmark_test.sh

# Feature interaction tests
./flywall.sh test int 31-ebpf/feature_interaction_test.sh

# Fallback mechanism tests
./flywall.sh test int 31-ebpf/fallback_mechanisms_test.sh
```

## Test Requirements

### System Requirements

- Linux kernel 5.8+ (for eBPF features)
- Root privileges
- Required tools:
  - clang/llvm
  - iproute2
  - bpftool
  - hping3
  - openssl
  - nslookup
  - jq

### Network Requirements

- Network interface: eth0 (or configurable)
- Test IPs: 192.0.2.0/24 (RFC 5737 test network)
- Internet access for some tests (DNS, TLS)

## Expected Results

### Performance Targets

| Test | Target | Pass Condition |
|------|--------|---------------|
| XDP Processing | >5M pps | XDP_PPS > 5000000 |
| TC Classification | >100K fps | TC_FPS > 100000 |
| Memory Usage | <80% | MEM_PEAK < 80 |
| CPU Usage | <80% | CPU_PEAK < 80 |
| Latency | <100μs | AVG_LATENCY < 100 |

### Feature Requirements

- All eBPF programs must load and verify
- Maps must be created and accessible
- Hooks must attach successfully
- Features must respect dependencies
- Fallbacks must work on failures

## Troubleshooting

### Common Failures

1. **eBPF not enabled**
   ```
   Error: eBPF programs not supported
   Fix: sysctl net.core.bpf_jit_enable=1
   ```

2. **Insufficient permissions**
   ```
   Error: Permission denied
   Fix: Run with root privileges
   ```

3. **Memory limits**
   ```
   Error: Map creation failed
   Fix: ulimit -l unlimited
   ```

4. **Kernel version**
   ```
   Error: eBPF feature not supported
   Fix: Update kernel to 5.8+
   ```

### Debug Mode

Enable debug logging:
```bash
# In test config
logging {
  level = "debug"
  ebpf = true
}

# Or check kernel logs
dmesg | grep -i bpf
```

### Test Isolation

Each test runs in a fresh VM with:
- Clean network namespace
- Fresh eBPF program state
- Isolated configuration
- Automatic cleanup

## Adding New Tests

### Test Template

```bash
#!/bin/sh
set -e
. "$(dirname "$0")/../common.sh"

plan N  # Number of tests

# Setup
CONFIG_FILE=$(mktemp_compatible test.hcl)
cat > "$CONFIG_FILE" <<'EOF'
schema_version = "1.0"
# Configuration here
EOF

start_ctl "$CONFIG_FILE"
wait_for_log_entry "$CTL_LOG" "Expected log entry"

# Test 1: Description
ok 0 "Test description" || {
    not ok 0 "Test description" severity fail error "Failure reason"
    diag "Debugging info"
}

# Cleanup
cleanup_processes
rm -f "$CONFIG_FILE"
```

### Best Practices

1. Use TAP format consistently
2. Clean up all resources
3. Use descriptive test names
4. Provide helpful diagnostics
5. Test both success and failure cases
6. Verify performance requirements
7. Test edge cases

## Continuous Integration

These tests run automatically in CI:
- On every PR to main branch
- On nightly builds
- On release candidates

Results are stored in:
```
build/test-results/integration_tests/31-ebpf/
├── ebpf_foundation/
│   ├── test.log
│   ├── results.tap
│   └── diagnostics.txt
└── ...
```
