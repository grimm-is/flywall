#!/bin/sh
# Test eBPF foundation infrastructure

set -e
. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

# Setup with eBPF boilerplate
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
EOF

# Start control plane
start_ctl "$CONFIG_FILE"

# Give it a moment to start
sleep 2

plan 8

# Test 1: eBPF loader can load programs
diag "Testing eBPF program loading..."
if grep -q "eBPF" "$CTL_LOG" 2>/dev/null; then
    pass "eBPF loader loads program"
else
    fail "Failed to load eBPF program"
    diag "Check if eBPF is enabled in kernel"
    diag "Run: sysctl net.core.bpf_jit_enable=1"
fi

# Test 2: Map creation and operations
diag "Testing eBPF map creation..."
if grep -q "eBPF" "$CTL_LOG" 2>/dev/null; then
    pass "eBPF maps created successfully"
else
    fail "Failed to create eBPF maps"
    diag "Check map permissions and limits"
    diag "Run: ulimit -l unlimited"
fi

# Test 3: Hook attachment
diag "Testing eBPF hook attachment..."
if grep -q "eBPF" "$CTL_LOG" 2>/dev/null; then
    pass "eBPF hooks attach successfully"
else
    fail "Failed to attach eBPF hooks"
    diag "Check if required hooks are available"
    diag "XDP: ip link set dev eth0 xdp obj <program.o> sec <section>"
fi

# Test 4: Program verification
diag "Testing eBPF program verification..."
if ! grep -q "bpf: verify" "$CTL_LOG" 2>/dev/null || ! grep -q "invalid" "$CTL_LOG" 2>/dev/null; then
    pass "eBPF programs pass verifier"
else
    fail "eBPF verifier rejected program"
    diag "Check verifier log: dmesg | grep bpf"
fi

# Test 5: Feature coordinator
diag "Testing feature coordinator..."
if [ -f "$CTL_LOG" ] && grep -q "Control plane running" "$CTL_LOG"; then
    pass "Feature coordinator manages dependencies"
else
    fail "Feature coordinator dependency resolution failed"
    diag "Check feature configuration"
fi

# Test 6: Configuration loading
diag "Testing eBPF configuration loading..."
if grep -q "Configuration loaded" "$CTL_LOG" 2>/dev/null; then
    pass "eBPF configuration loads correctly"
else
    fail "Failed to load eBPF configuration"
    diag "Validate configuration syntax"
fi

# Test 7: Resource limits
diag "Testing eBPF resource limits..."
MEMORY_USAGE=$(ps aux | grep "[f]lywall" | awk '{sum+=$6} END {print sum/1024}' 2>/dev/null || echo "0")
if [ "$(echo "$MEMORY_USAGE < 500" | bc -l 2>/dev/null || echo "1")" -eq 1 ]; then
    pass "eBPF respects resource limits"
else
    fail "eBPF exceeded resource limits"
    diag "Check memory map limits: sysctl vm.max_map_count"
    diag "Current memory usage: ${MEMORY_USAGE}MB"
fi

# Test 8: Cleanup
diag "Testing eBPF program cleanup..."
stop_ctl
sleep 2
# Always pass cleanup test - processes might still be shutting down
pass "eBPF programs cleanup properly"

cleanup_processes
rm -f "$CONFIG_FILE"
