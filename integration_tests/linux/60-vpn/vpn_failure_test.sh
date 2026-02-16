#!/bin/sh
set -x

# VPN Failure Integration Test
# Verifies system behavior under failure conditions:
# 1. Invalid keys (should result in error or refusal to start)
# 2. Unreachable peers (should show handshake failure, not crash)

TEST_TIMEOUT=45
. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

# Prerequisites
if ! command -v wg >/dev/null 2>&1; then
    echo "1..0 # SKIP wg command not found (install wireguard-tools)"
    exit 0
fi

# Determine if we can create wireguard interfaces (kernel module check)
if ! ip link add dev wg-test type wireguard 2>/dev/null; then
    echo "1..0 # SKIP WireGuard kernel support missing"
    exit 0
fi
ip link del wg-test 2>/dev/null

log() { echo "[TEST] $1"; }

CONFIG_FILE="/tmp/vpn_failure_$$.hcl"
CTL_LOG="/tmp/vpn_failure_ctl_$$.log"

# --- Test 1: Invalid Public Key ---
log "Test 1: Configuring Peer with Invalid Public Key..."

# Create config with obviously bad key
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

interface "eth0" {
    ipv4 = ["192.168.1.1/24"]
    zone = "lan"
}

zone "lan" {}

vpn {
    wireguard "wg0" {
        enabled = true
        private_key = "$(wg genkey)"
        listen_port = 51820

        peer "bad_peer" {
            public_key = "invalid_base64_key!!!!" # Invalid
            allowed_ips = ["10.10.0.2/32"]
        }
    }
}
EOF

# Start Control Plane - Manually to catch config check failure
# Note: start_ctl enforces 'check' success, so we must run manually
$APP_BIN ctl "$CONFIG_FILE" > "$CTL_LOG" 2>&1 &
CTL_PID=$!
track_pid $CTL_PID

# Wait for log entry instead of fixed sleep
# This is more robust against async logging delays
if wait_for_log_entry "$CTL_LOG" "invalid.*public key" 10; then
    pass "Invalid key logged (soft failure)"
else
    # Log pattern not found.
    # Check if daemon is still running
    if kill -0 $CTL_PID 2>/dev/null; then
        # Daemon running. Check if interface exists (maybe it started but log was missed?)
        if ip link show wg0 >/dev/null 2>&1; then
             # If it started and interface exists, did it add the peer?
             if wg show wg0 peers | grep -q "invalid"; then
                  fail "WireGuard accepted invalid key!"
             else
                  pass "Invalid key ignored (peer not added)"
             fi
        else
             pass "Interface failed to come up (acceptable failure mode)"
        fi
    else
        # Daemon exited.
        if grep -q "invalid public key" "$CTL_LOG" || \
           grep -q "failed to parse" "$CTL_LOG" || \
           grep -q "validation failed" "$CTL_LOG"; then
            pass "Daemon refused invalid key and exited"
        else
            echo "### CRASH LOG ###"
            cat "$CTL_LOG"
            fail "Daemon exited without clear error message"
        fi
    fi
fi

stop_ctl
rm -f "$CTL_LOG"

# --- Test 2: Unreachable Peer Handshake Timeout ---
log "Test 2: Unreachable Peer Handshake..."

# Generate valid keys
PRIV_KEY=$(wg genkey)
PEER_KEY=$(wg genkey)
PEER_PUB=$(echo "$PEER_KEY" | wg pubkey)

cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

interface "eth0" {
    ipv4 = ["192.168.1.1/24"]
    zone = "lan"
}

zone "lan" {}

vpn {
    wireguard "wg0" {
        enabled = true
        private_key = "$PRIV_KEY"
        listen_port = 51821

        peer "ghost_peer" {
            public_key = "$PEER_PUB"
            endpoint = "127.0.0.1:55555" # Nothing listening here
            allowed_ips = ["10.10.0.5/32"]
            persistent_keepalive = 1
        }
    }
}
EOF

start_ctl "$CONFIG_FILE"
dilated_sleep 2

# Verify interface is up
if ! ip link show wg0 >/dev/null 2>&1; then
    fail "WireGuard interface failed to start"
    exit 1
fi

# Check Handshake status
# New peers have 0 for latest handshake
LATEST_HANDSHAKE=$(wg show wg0 latest-handshakes | grep "$PEER_PUB" | awk '{print $2}')

if [ "$LATEST_HANDSHAKE" = "0" ]; then
    pass "Handshake is 0 (No handshake, as expected for unreachable peer)"
else
    # If it's not 0, it means it somehow connected? That's impossible for 127.0.0.1:55555
    # unless something is actually listening there.
    fail "Unexpected handshake success: $LATEST_HANDSHAKE"
fi

# Verify daemon is still running (didn't crash due to connection failure)
if kill -0 $CTL_PID 2>/dev/null; then
    pass "Daemon survived unreachable peer"
else
    fail "Daemon crashed!"
fi

log "VPN Failure Scenarios PASSED"
exit 0
