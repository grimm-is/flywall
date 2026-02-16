#!/bin/sh
set -x

# IPSet Failure Integration Test
# Verifies that external IPSet failures (404/Timeout) do NOT crash the firewall.

TEST_TIMEOUT=30
. "$(dirname "$0")/../common.sh"
require_root
require_binary
cleanup_on_exit

log() { echo "[TEST] $1"; }

CONFIG_FILE="/tmp/ipset_failure_$$.hcl"
CTL_LOG="/tmp/ipset_failure_ctl_$$.log"

# Define a non-existent local URL
# Using port 12345 (nothing listening) -> Connection Refused (fast failure)
# Using a reserved IP -> Timeout (slower)
# Let's use 127.0.0.1:12345 (Connection Refused)
BAD_URL="http://127.0.0.1:12345/bad_list.txt"

cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

interface "eth0" {
    ipv4 = ["192.168.1.1/24"]
    zone = "lan"
}

zone "lan" {}

# External IPSet with unreachable URL
ipset "external_list" {
    type = "ipv4_addr"
    url = "${BAD_URL}"
    refresh_interval = "1m"
    # Static entries should still be loaded even if URL fails
    entries = ["1.1.1.1"]
}

policy "lan" "firewall" {
    name = "allow_all"
    action = "accept"
}
EOF

log "Starting Control Plane with bad IPSet URL..."
start_ctl "$CONFIG_FILE"

# Wait for startup
dilated_sleep 2

# Test 1: Verify Daemon is Running (Didn't crash)
if kill -0 $CTL_PID 2>/dev/null; then
    pass "Daemon started successfully despite invalid IPSet URL"
else
    fail "Daemon crashed!"
fi

# Test 2: Verify Error Log
# Should see something like "failed to fetch" or "connection refused"
if grep -q "failed to fetch" "$CTL_LOG" || grep -q "connection refused" "$CTL_LOG" || grep -q "error" "$CTL_LOG"; then
    pass "Failure logged correctly"
else
    log "Warning: No explicit error found in logs (check debug level?)"
    # cat "$CTL_LOG"
fi

# Test 3: Verify Set Exists and contains static entries
# Even if URL fails, the set should be created with 'entries'
log "Checking IPSet content..."
if command -v nft >/dev/null 2>&1; then
    if nft list set inet flywall external_list | grep -q "1.1.1.1"; then
        pass "Static entries loaded despite URL failure"
    else
        fail "IPSet missing or empty"
        nft list ruleset
    fi
else
    log "SKIP: nft command not found"
fi

stop_ctl
log "IPSet Failure Test PASSED"
exit 0
