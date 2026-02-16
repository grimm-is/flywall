#!/bin/sh
set -x

# Config Reload Validation Test
# Verifies that the daemon rejects invalid configurations during reload
# and preserves the running state.

TEST_TIMEOUT=60
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

CONFIG_FILE=$(mktemp_compatible "reload_validation.hcl")

# 1. Valid Initial Configuration
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"
zone "trust" {}
interface "eth0" {
    ipv4 = ["10.0.0.1/24"]
    zone = "trust"
}
EOF

# Start Control Plane
start_ctl "$CONFIG_FILE"

# Wait for startup
wait_for_log_entry "$CTL_LOG" "Control plane running"

# 2. Corrupt the config file (Invalid Syntax/Logic)
# Invalid IPSet name (semicolon) should trigger validation error
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"
zone "trust" {}
interface "eth0" {
    ipv4 = ["10.0.0.1/24"]
    zone = "trust"
}
ipset "bad;name" {
    type = "ipv4_addr"
}
EOF

# 3. Trigger Reload (SIGHUP)
log "Triggering reload with invalid config..."
kill -HUP $(cat "$RUN_DIR/$BRAND_LOWER.pid")

# 4. Assert Failure in Logs
log "Waiting for validation error..."
if wait_for_log_entry "$CTL_LOG" "config validation failed" 30; then
    pass "Daemon rejected invalid config"
else
    fail "Daemon did not log validation failure!"
fi

# 5. Assert Daemon Still Running
if kill -0 $(cat "$RUN_DIR/$BRAND_LOWER.pid"); then
    pass "Daemon survived invalid reload"
else
    fail "Daemon crashed after invalid reload!"
fi

# 6. Verify Running Config is still Valid (via nft)
# We expect eth0 to still exist in ruleset, but no 'bad;name' set
if "$APP_BIN" show | grep -q "bad;name"; then
    fail "Invalid config was partially applied!"
fi

log "Reload Validation PASSED"
exit 0
