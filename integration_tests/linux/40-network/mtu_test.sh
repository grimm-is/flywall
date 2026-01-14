#!/bin/sh
set -x
#
# MTU Configuration Integration Test
# Verifies MTU settings are applied to interfaces
#

. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

CONFIG_FILE="/tmp/mtu_test.hcl"

# Create test config with MTU settings
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

interface "lo" {
    ipv4 = ["127.0.0.1/8"]
    mtu = 16384
}

zone "local" {}
EOF

plan 3

# Test 1: Config with MTU parses correctly
diag "Test 1: Config parsing with MTU"
OUTPUT=$($APP_BIN show "$CONFIG_FILE" 2>&1)
if [ $? -eq 0 ]; then
    pass "Config with MTU parses successfully"
else
    diag "Output: $OUTPUT"
    fail "Config with MTU failed to parse"
fi

# Test 2: Start control plane with MTU config
diag "Test 2: Control plane starts with MTU config"
start_ctl "$CONFIG_FILE"
dilated_sleep 2

if [ -n "$CTL_PID" ] && kill -0 $CTL_PID 2>/dev/null; then
    pass "Control plane runs with MTU configuration"

    # Test 3: Verify MTU applied
    diag "Test 3: Verify MTU on interface"
    if ip link show lo | grep -q "mtu 16384"; then
        pass "MTU 16384 applied to lo"
    else
        CURRENT_MTU=$(ip link show lo | grep -o "mtu [0-9]*" | awk '{print $2}')
        fail "MTU mismatch: expected 16384, got ${CURRENT_MTU:-unknown}"
    fi
else
    fail "Control plane failed with MTU config"
fi

rm -f "$CONFIG_FILE"
diag "MTU configuration test completed"
