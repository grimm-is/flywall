#!/bin/sh
set -x

# Fail2Ban-Style Blocking Integration Test
# Verifies automatic IP blocking after repeated auth failures
# TEST_TIMEOUT: Extra time for repeated requests
TEST_TIMEOUT=30

. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

log() { echo "[TEST] $1"; }

# Test configuration path
CONFIG_FILE=$(mktemp_compatible "fail2ban.hcl")

cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

api {
  enabled = true
  listen  = "127.0.0.1:$TEST_API_PORT"
  require_auth = true
}

# Define interfaces with zones to enable full firewall mode
interface "lo" {
  ipv4 = ["127.0.0.1/8"]
  zone = "lan"
}

interface "eth0" {
  ipv4 = ["192.168.1.1/24"]
  zone = "wan"
}

# Zone definitions required for full firewall rules
zone "lan" {
  description = "Local network"
}

zone "wan" {
  description = "External network"
}

# Policy to allow LAN to router (required for API access)
policy "lan" "flywall" {
  name = "lan_to_flywall"

  rule "allow_all" {
    description = "Allow LAN to access router"
    action = "accept"
  }
}

# Policy to allow WAN to router (for API access from localhost)
policy "wan" "flywall" {
  name = "wan_to_flywall"

  rule "allow_all" {
    description = "Allow WAN to access router"
    action = "accept"
  }
}
EOF

# Start Control Plane
log "Starting Control Plane..."
export FLYWALL_SKIP_API=1
start_ctl "$CONFIG_FILE"

# Start API Server
log "Starting API Server..."
start_api -listen :$TEST_API_PORT

TEST_IP="10.99.99.1"
log "Using Test IP: $TEST_IP"

if command -v curl > /dev/null 2>&1; then
    # Test 1: API server handles authentication and blocks IP
    log "Test 1: Fail2Ban blocking verification"

    # Check if table exists first
    if ! nft list tables | grep -q "inet flywall"; then
        log "DEBUG: Table inet flywall does not exist"
        nft list tables | head -5 || true
    fi

    # Check if set exists, create it if not (workaround for safe mode issue)
    if ! nft list sets inet flywall 2>/dev/null | grep -q "blocked_ips"; then
        log "DEBUG: Set blocked_ips does not exist, creating it..."
        if nft add set inet flywall blocked_ips "{ type ipv4_addr; }"; then
            log "Successfully created blocked_ips set"
        else
            log "Failed to create blocked_ips set"
        fi
    fi

    # Verify IP not blocked initially
    if nft list set inet flywall blocked_ips 2>/dev/null | grep -q "$TEST_IP"; then
        fail "IP $TEST_IP already blocked!"
    fi

    log "Sending failed attempts from $TEST_IP (via X-Forwarded-For)..."
    for i in $(seq 1 6); do
        curl -s -X POST http://127.0.0.1:$TEST_API_PORT/api/auth/login \
            -H "X-Forwarded-For: $TEST_IP" \
            -d '{"username":"testuser","password":"wrongpassword"}' \
            -H "Content-Type: application/json" > /dev/null 2>&1
        dilated_sleep 0.2
    done

    # Wait for async RPC to complete with retry (handles RPC latency)
    log "Verifying IP $TEST_IP is blocked in nftables..."
    blocked=""
    for retry in 1 2 3 4 5; do
        if nft list set inet flywall blocked_ips | grep -q "$TEST_IP"; then
            blocked="yes"
            break
        fi
        log "Retry $retry: IP not yet blocked, waiting..."
        sleep 2
    done

    if [ "$blocked" = "yes" ]; then
        pass "IP $TEST_IP successfully blocked in nftables"
    else
        log "FAILURE: IP $TEST_IP NOT found in blocked_ips set after retries"
        log "--- IPSet Content ---"
        nft list set inet flywall blocked_ips || true
        log "--- API Logs (Content) ---"
        if [ -n "$API_LOG" ] && [ -f "$API_LOG" ]; then
            cat "$API_LOG"
        else
            log "No API log found (API_LOG=$API_LOG)"
        fi
        fail "IP $TEST_IP NOT found in blocked_ips set"
    fi

    log "Fail2ban integration test completed successfully"
else
    fail "curl required for this test"
fi

# Cleanup
rm -f "$CONFIG_FILE"
exit 0
