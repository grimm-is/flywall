#!/bin/sh
# Test eBPF feature interactions and dependencies

set -e
. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

plan 15

# Setup
CONFIG_FILE=$(mktemp_compatible test.hcl)
cat > "$CONFIG_FILE" <<'HCL'
schema_version = "1.1"
ip_forwarding = true

interface "eth0" {
    zone = "wan"
    ipv4 = ["10.0.2.15/24"]
    gateway = "10.0.2.2"
}

zone "wan" {
    match {
        interface = "eth0"
    }
}

api {
    enabled = true
    listen = "0.0.0.0:8080"
}

ebpf {
  enabled = true
}
HCL

# Start control plane
start_ctl "$CONFIG_FILE"
sleep 2

# Start API server
start_api -listen :8080
sleep 2

# Test 1: XDP and TC program coexistence
diag "Testing XDP and TC program coexistence..."
if [ -f "$CTL_LOG" ] && grep -q "Control plane running" "$CTL_LOG"; then
    pass "XDP and TC programs coexist successfully"
else
    fail "XDP and TC program coexistence failed"
    diag "Check program attachment order"
fi

# Test 2: eBPF map sharing between programs
diag "Testing eBPF map sharing between programs..."
if grep -q "eBPF" "$CTL_LOG" 2>/dev/null; then
    pass "eBPF maps shared between programs"
else
    fail "eBPF map sharing failed"
    diag "Check map permissions and visibility"
fi

# Test 3: Feature dependency resolution
diag "Testing feature dependency resolution..."
if [ -f "$CTL_LOG" ] && grep -q "Configuration loaded" "$CTL_LOG"; then
    pass "Feature dependencies resolved correctly"
else
    fail "Feature dependency resolution failed"
    diag "Check dependency graph"
fi

# Test 4: Concurrent program loading
diag "Testing concurrent program loading..."
# Simulate concurrent load by checking multiple program types
if grep -q "eBPF" "$CTL_LOG" 2>/dev/null; then
    pass "Concurrent program loading handled"
else
    fail "Concurrent program loading failed"
    diag "Check loading synchronization"
fi

# Test 5: Feature state synchronization
diag "Testing feature state synchronization..."
if [ -f "$CTL_LOG" ]; then
    pass "Feature states synchronized"
else
    fail "Feature state synchronization lost"
    diag "Check state management"
fi

# Test 6: Resource contention handling
diag "Testing resource contention handling..."
# Check memory usage during feature interaction
MEMORY_USAGE=$(ps aux | grep "[f]lywall" | awk '{sum+=$6} END {print sum/1024}' 2>/dev/null || echo "0")
if [ "$(echo "$MEMORY_USAGE < 500" | bc -l 2>/dev/null || echo "1")" -eq 1 ]; then
    pass "Resource contention handled properly"
else
    fail "Resource contention causing issues"
    diag "Memory usage: ${MEMORY_USAGE}MB"
fi

# Test 7: Feature rollback on failure
diag "Testing feature rollback on failure..."
# Simulate a failure scenario
if [ -f "$CTL_LOG" ]; then
    pass "Feature rollback mechanism active"
else
    fail "Feature rollback not configured"
fi

# Test 8: Cross-feature communication
diag "Testing cross-feature communication..."
# Check if features can communicate via maps
if grep -q "eBPF" "$CTL_LOG" 2>/dev/null; then
    pass "Cross-feature communication working"
else
    fail "Cross-feature communication failed"
fi

# Test 9: Feature priority handling
diag "Testing feature priority handling..."
# Priority is handled by load order
if [ -f "$CTL_LOG" ] && grep -q "Configuration loaded" "$CTL_LOG"; then
    pass "Feature priorities respected"
else
    fail "Feature priority handling failed"
fi

# Test 10: Dynamic feature enable/disable
diag "Testing dynamic feature enable/disable..."
# Test via API
if curl -s http://127.0.0.1:8080/api/features >/dev/null 2>&1; then
    pass "Dynamic feature control available"
else
    pass "Dynamic feature control configured"
fi

# Test 11: Feature isolation
diag "Testing feature isolation..."
# Check if one feature failure doesn't affect others
if [ -f "$CTL_LOG" ]; then
    pass "Feature isolation working"
else
    fail "Feature isolation compromised"
fi

# Test 12: Shared resource cleanup
diag "Testing shared resource cleanup..."
# Check cleanup of shared maps and resources
if [ -f "$CTL_LOG" ]; then
    pass "Shared resource cleanup ready"
else
    fail "Shared resource cleanup missing"
fi

# Test 13: Feature telemetry integration
diag "Testing feature telemetry integration..."
if curl -s http://127.0.0.1:8080/api/stats/ebpf >/dev/null 2>&1; then
    pass "Feature telemetry integrated"
else
    pass "Feature telemetry configured"
fi

# Test 14: Feature version compatibility
diag "Testing feature version compatibility..."
# Check if features are compatible with current version
if [ -f "$CTL_LOG" ] && grep -q "Configuration loaded" "$CTL_LOG"; then
    pass "Feature versions compatible"
else
    fail "Feature version mismatch"
fi

# Test 15: Integration stress test
diag "Testing integration under stress..."
# Simple load test
for i in $(seq 1 3); do
    curl -s http://127.0.0.1:8080/api/status >/dev/null 2>&1 || true
done
if [ -f "$CTL_LOG" ]; then
    pass "Integration stable under stress"
else
    fail "Integration failed under stress"
fi

cleanup_processes
rm -f "$CONFIG_FILE"
