# Writing Integration Tests for Flywall

This guide explains how to write integration tests for Flywall that run in Linux VMs using the orca framework.

## Overview

Flywall integration tests:
- Run in isolated Linux VMs via `./flywall.sh test int`
- Use TAP (Test Anything Protocol) format
- Have root privileges and full network namespace support
- Can test kernel features like NFQueue, conntrack, and nftables

## Test File Structure

```bash
#!/bin/sh
# Test description
set -e
. "$(dirname "$0")/../common.sh"

# Prerequisites
require_root
require_binary
cleanup_on_exit

# Declare number of tests
plan N

# Test implementation...
```

## Essential Functions from common.sh

### Test Control
- `start_ctl "$CONFIG_FILE"` - Start the control plane
- `stop_ctl` - Stop the control plane
- `start_api -listen :PORT` - Start API server
- `wait_for_api_ready PORT` - Wait for API to respond
- `wait_for_log_entry "$LOG_FILE" "pattern"` - Wait for log message

### TAP Output
- `ok 0 "description"` - Report passing test
- `not ok 0 "description"` - Report failing test
- `diag "message"` - Output diagnostic info
- `fail "message"` - Fail immediately
- `skip "message"` - Skip test

### File Handling
- `mktemp_compatible name.hcl` - Create temp file
- `cleanup_processes` - Kill all spawned processes

## Test Patterns

### 1. Basic Test Template

```bash
#!/bin/sh
# Basic integration test template
set -e
. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

plan 3

# Test 1: Create config
CONFIG_FILE=$(mktemp_compatible test.hcl)
cat > "$CONFIG_FILE" <<'EOF'
schema_version = "1.0"
interface "eth0" {
  ipv4 = ["10.0.0.1/24"]
  zone = "lan"
}
zone "lan" {}
EOF

if [ -f "$CONFIG_FILE" ]; then
    ok 0 "Test configuration created"
else
    not ok 0 "Test configuration created"
    fail "Failed to create config file"
fi

# Test 2: Start control plane
start_ctl "$CONFIG_FILE"
if kill -0 $CTL_PID 2>/dev/null; then
    ok 0 "Control plane started"
else
    not ok 0 "Control plane started" severity fail error "Process not running"
    fail "Control plane failed to start"
fi

# Test 3: Verify functionality
if grep -q "Control plane running" "$CTL_LOG"; then
    ok 0 "Control plane ready"
else
    not ok 0 "Control plane ready" severity fail error "Ready message not found"
fi

# Cleanup
cleanup_processes
rm -f "$CONFIG_FILE"
```

### 2. Network Topology Tests

```bash
# Create network namespaces
setup_test_topology() {
    ip netns add test_ns
    ip link add veth-test type veth peer name veth-peer
    ip link set veth-peer netns test_ns
    ip addr add 10.0.0.1/24 dev veth-test
    ip link set veth-test up
    ip netns exec test_ns ip addr add 10.0.0.2/24 dev veth-peer
    ip netns exec test_ns ip link set veth-peer up
    ip netns exec test_ns ip link set lo up
}

teardown_test_topology() {
    ip netns del test_ns 2>/dev/null || true
    ip link del veth-test 2>/dev/null || true
}
```

### 3. API Testing

```bash
# Start API server
start_api -listen :8080
wait_for_api_ready 8080

# Test API endpoint
RESPONSE=$(curl -s "http://127.0.0.1:8080/api/status" || echo "")
if echo "$RESPONSE" | grep -q "online"; then
    ok 0 "API status endpoint responding"
else
    not ok 0 "API status endpoint responding" severity fail error "API not responding"
fi
```

### 4. Traffic Generation Tests

```bash
# Generate test traffic
diag "Generating ICMP traffic..."
if ping -c 1 -W 1 10.0.0.2 >/dev/null 2>&1; then
    ok 0 "ICMP connectivity working"
else
    not ok 0 "ICMP connectivity working"
fi

# Generate TCP traffic
diag "Generating TCP traffic..."
if echo "test" | nc -w 1 10.0.0.2 80 >/dev/null 2>&1; then
    ok 0 "TCP connectivity working"
else
    ok 0 "TCP connectivity working"  # Connection refused is ok
fi
```

### 5. Kernel Feature Tests

```bash
# Test nftables rules
RULES_COUNT=$(nft list table inet flywall 2>/dev/null | grep -c "accept" || echo "0")
if [ "$RULES_COUNT" -gt 0 ]; then
    ok 0 "Firewall rules present"
    diag "Found $RULES_COUNT accept rules"
else
    not ok 0 "Firewall rules present"
fi

# Test conntrack entries
CONNTRACK_COUNT=$(conntrack -L 2>/dev/null | wc -l)
if [ "$CONNTRACK_COUNT" -gt 0 ]; then
    ok 0 "Conntrack entries created"
    diag "Found $CONNTRACK_COUNT conntrack entries"
else
    ok 0 "Conntrack entries created"  # No entries might be ok
fi
```

## Best Practices

1. **Always use TAP format**: Declare plan with `plan N` and use `ok`/`not ok`
2. **Clean up resources**: Use `cleanup_on_exit` and explicit cleanup
3. **Use descriptive test names**: Make it clear what each test verifies
4. **Handle failures gracefully**: Use `severity fail` with context
5. **Test edge cases**: Verify both success and failure scenarios
6. **Use time dilation**: Call `dilated_sleep` instead of `sleep`
7. **Check logs**: Use `$CTL_LOG` for control plane diagnostics

## Running Tests

```bash
# Run single test
./flywall.sh test int my_test

# Run all tests in a directory
./flywall.sh test int 30-firewall

# Run all integration tests
./flywall.sh test int

# Run with verbose output
./flywall.sh test int my_test --verbose
```

## Debugging Failed Tests

1. Check the test log in `build/test-results/`
2. Look for `not ok` entries with severity details
3. Examine control plane logs with `log_tail` or `log_head` directives
4. Use `diag` statements for debugging output

## Example: Complete Integration Test

See `integration_tests/linux/70-system/inline_ips_test.sh` for a complete example that:
- Creates network topology
- Starts control plane with inline IPS mode
- Verifies nftables rules
- Generates traffic and verifies learning
- Tests fail-open behavior
- Outputs proper TAP format
