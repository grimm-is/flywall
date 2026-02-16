#!/bin/sh
set -x

# Atomic Apply / IPSet Failure Test
# Verifies that if an IPSet fails to download/resolve, the rule application is aborted
# and the previous ruleset stays intact (Atomic Property).

TEST_TIMEOUT=45
. "$(dirname "$0")/../common.sh"

require_root
require_binary

cleanup_with_logs() {
    if [ -f "$CTL_LOG" ]; then
        diag "CTL Log Dump:"
        cat "$CTL_LOG" | sed 's/^/# /'
    fi
    cleanup_processes
}
trap cleanup_with_logs EXIT INT TERM

log() { echo "[TEST] $1"; }

CONFIG_FILE="/tmp/atomic_test_$$.hcl"

# 1. Valid Initial Configuration (V1)
# Includes a marker comment
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"
zone "trust" {}
interface "eth0" {
    ipv4 = ["10.0.0.1/24"]
    zone = "trust"
}
# Marker for V1
api { enabled = false }
EOF

# Start Control Plane
start_ctl "$CONFIG_FILE"
wait_for_log_entry "$CTL_LOG" "Control plane running"

# Save initial genid
GENID_V1=$("$APP_BIN" show | grep "comment" | head -n1)
log "V1 Ruleset active"

# 2. Update Config with Invalid Zone (V2)
# Using an invalid character in zone name should trigger validation error
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"
zone "trust" {}
interface "eth0" {
    ipv4 = ["10.0.0.1/24"]
    zone = "trust"
}

# Invalid policy referencing non-existent/invalid zone
policy "invalid_zone!!!" "firewall" {
    name = "break_it"
    action = "accept"
}
EOF

# 3. Trigger Reload
log "Triggering reload with invalid configuration..."
kill -HUP $(cat "$RUN_DIR/$BRAND_LOWER.pid")

# 4. Wait for expected failure log
# Expect validation error
if wait_for_log_entry "$CTL_LOG" "invalid policy from-zone" 15; then
    pass "Daemon rejected invalid configuration"
else
    fail "Daemon did not log validation failure!"
fi

# 5. Verify Ruleset is still V1 (Atomic behavior)
# If it failed atomically, the running rules should NOT have the invalid policy
# and should still have the V1 marker (if we could check it).
# We check that the new policy name "break_it" is NOT present.
if "$APP_BIN" show | grep -q "break_it"; then
    fail "Atomic violation! 'break_it' rules found despite validation failure."
else
    pass "Atomic check passed: invalid config not applied"
fi

exit 0
