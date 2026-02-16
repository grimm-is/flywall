#!/bin/sh
set -x

# NFQueue Fail-Open Test
# Verifies that inline learning rules use NFQUEUE with the 'bypass' flag
# to ensure traffic is allowed if the user-space agent crashes/hangs.

TEST_TIMEOUT=30
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

CONFIG_FILE="/tmp/nfqueue_test_$$.hcl"

# 1. Config with Inline Learning Enabled
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"
zone "trust" {}
interface "eth0" {
    ipv4 = ["10.0.0.1/24"]
    zone = "trust"
}

rule_learning {
    enabled = true
    inline_mode = true
    log_group = 100
}
EOF

# Start Control Plane
start_ctl "$CONFIG_FILE"

# Wait for startup
wait_for_log_entry "$CTL_LOG" "Control plane running"

# 2. Check nftables rules for bypass flag
log "Verifying NFQUEUE bypass flag..."
NFT_RULES=$("$APP_BIN" show)

# Expect: queue flags bypass to 100
if echo "$NFT_RULES" | grep -q "queue.*bypass.*100"; then
    pass "Found NFQUEUE rule with bypass flag"
else
    diag "Ruleset:"
    echo "$NFT_RULES"
    fail "NFQUEUE rule missing 'bypass' flag! Traffic will drop on agent failure."
fi

exit 0
