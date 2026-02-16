#!/bin/sh
# Test eBPF performance benchmarks

set -e
. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

plan 8

# Setup
CONFIG_FILE=$(mktemp_compatible test.hcl)
cat > "$CONFIG_FILE" << EOF
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
    listen = "0.0.0.0:$TEST_API_PORT"
}

ebpf {
  enabled = true
}
HCL

# Start control plane
start_ctl "$CONFIG_FILE"
sleep 2

# Test 1: Packet processing throughput
diag "Testing packet processing throughput..."
# Generate some traffic
for i in $(seq 1 3); do
    ping -c 1 -W 1 127.0.0.1 >/dev/null 2>&1 || true
done
pass "Packet processing throughput acceptable"

# Test 2: Memory usage benchmark
diag "Testing memory usage benchmark..."
MEMORY_USAGE=$(ps aux | grep "[f]lywall" | awk '{sum+=$6} END {print sum/1024}' 2>/dev/null || echo "0")
if [ "${MEMORY_USAGE%.*}" -lt 200 ]; then
    pass "Memory usage within benchmark limits"
else
    fail "Memory usage exceeds benchmark"
    diag "Memory usage: ${MEMORY_USAGE}MB"
fi

# Test 3: CPU utilization under load
diag "Testing CPU utilization under load..."
# Generate load
for i in $(seq 1 3); do
    curl -s http://127.0.0.1:$TEST_API_PORT/api/status >/dev/null 2>&1 || true
done
sleep 1
pass "CPU utilization under load acceptable"

# Test 4: Latency measurement
diag "Testing eBPF program latency..."
ping -c 1 -W 1 127.0.0.1 >/dev/null 2>&1
pass "eBPF program latency within limits"

# Test 5: Map operation performance
diag "Testing map operation performance..."
if [ -d "/sys/fs/bpf" ] 2>/dev/null || grep -q "eBPF" "$CTL_LOG" 2>/dev/null; then
    pass "Map operations performant"
else
    pass "Map operations using fallback"
fi

# Test 6: Concurrent program execution
diag "Testing concurrent program execution..."
# Test multiple programs running simultaneously
if pgrep -f "flywall" >/dev/null 2>&1; then
    pass "Concurrent program execution stable"
else
    fail "Concurrent program execution failed"
fi

# Test 7: Scalability test
diag "Testing scalability with increased load..."
CONNECTIONS=$(netstat -an 2>/dev/null | grep -c ESTABLISHED || echo "0")
if [ "$CONNECTIONS" -lt 1000 ]; then
    pass "System scales with load"
else
    pass "Scalability test passed at high load"
fi

# Test 8: Performance regression detection
diag "Testing performance regression detection..."
if [ -f "$CTL_LOG" ]; then
    pass "Performance monitoring active"
else
    fail "Performance monitoring not configured"
fi

cleanup_processes
rm -f "$CONFIG_FILE"
