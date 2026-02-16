#!/bin/sh
set -x

# Persistence Integration Test
# Verifies that the system recovers from a "Dirty Shutdown" (SIGKILL)
# and that config changes are persisted to disk via HCL.

TEST_TIMEOUT=60
. "$(dirname "$0")/../common.sh"
require_root
plan 10
require_binary
cleanup_on_exit

log() { echo "[TEST] $1"; }

# 1. Setup
TEST_STATE_DIR="/tmp/flywall_persist_test_$$"
rm -rf "$TEST_STATE_DIR"
mkdir -p "$TEST_STATE_DIR"

CONFIG_FILE="/tmp/persist_test_$$.hcl"
export FLYWALL_STATE_DIR="$TEST_STATE_DIR"

# Minimal config with auth
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

interface "lo" {
  ipv4 = ["127.0.0.1/8"]
}
interface "eth1" {
  ipv4 = ["10.0.0.1/24"]
}
zone "local" {
  match {
    interface = "eth1"
  }
}
api {
  enabled = false
  require_auth = false
}
EOF

# DEBUG: Dump config
log "Generated Config:"
cat "$CONFIG_FILE"

# DEBUG: Check interfaces
log "System Interfaces:"
ip link show
ip link set eth1 up 2>/dev/null || true

# 2. Start Daemon (Round 1)
log "Starting Daemon (Round 1)..."
start_ctl "$CONFIG_FILE"
start_api -listen :$TEST_API_PORT
ok 0 "System started with API"

# 3. Modify State via API (Creating VLAN)
log "Modifying state via API (Creating VLAN)..."
HTTP_CODE=$(curl -s -m 15 --retry 3 --retry-delay 1 --retry-all-errors \
    -o /tmp/create_vlan_response_$$.json -w "%{http_code}" \
    -X POST "http://127.0.0.1:$TEST_API_PORT/api/vlans" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-bypass" \
    -d '{"parent_interface": "eth1", "vlan_id": 99, "zone": "local", "ipv4": ["10.99.99.1/24"]}')

if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 201 ]; then
    ok 0 "VLAN create API returned $HTTP_CODE"
else
    log "VLAN creation failed with HTTP $HTTP_CODE"
    cat /tmp/create_vlan_response_$$.json 2>/dev/null
    ok 1 "VLAN create API returned $HTTP_CODE" severity fail expected "200" actual "$HTTP_CODE"
fi

# Apply changes (commit to runtime + disk)
log "Applying Config..."
HTTP_CODE=$(curl -s -m 15 --retry 3 --retry-delay 1 --retry-all-errors \
    -o /tmp/apply_response_$$.json -w "%{http_code}" \
    -X POST "http://127.0.0.1:$TEST_API_PORT/api/config/apply" \
    -H "X-API-Key: test-bypass")

if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "Config apply returned 200"
else
    log "Apply failed with HTTP $HTTP_CODE"
    cat /tmp/apply_response_$$.json 2>/dev/null
    ok 1 "Config apply returned $HTTP_CODE" severity fail expected "200" actual "$HTTP_CODE"
fi

# Give a moment for apply to propagate to runtime
sleep 1

# Verify VLAN exists in runtime
if ip link show eth1.99 >/dev/null 2>&1; then
    ok 0 "VLAN eth1.99 created in Runtime"
else
    log "VLAN not found. Available interfaces:"
    ip link show | grep eth1
    ok 1 "VLAN eth1.99 failed to create in runtime"
fi

# 4. Dirty Shutdown (SIGKILL)
log "Simulating Power Loss (kill -9)..."
# Kill children first (API)
[ -n "$API_PID" ] && kill $API_PID 2>/dev/null || true
wait $API_PID 2>/dev/null || true
# Kill control plane
kill -9 $CTL_PID
wait $CTL_PID 2>/dev/null
# Ensure any orphaned flywall processes are gone
pkill -f "flywall.*$CONFIG_FILE" 2>/dev/null || true
sleep 1

# 5. Verify config was persisted to disk
if grep -q 'vlan "99"' "$CONFIG_FILE" 2>/dev/null || grep -q 'eth1.99' "$CONFIG_FILE" 2>/dev/null; then
    ok 0 "Config file updated on disk"
else
    ok 1 "Config file NOT updated on disk before kill! (Changes lost?)"
    log "Config contents:"
    cat "$CONFIG_FILE"
fi

# 6. Restart Daemon (Round 2)
log "Restarting Daemon (Round 2)..."
log "Config content after persistence:"
cat "$CONFIG_FILE"

start_ctl "$CONFIG_FILE"
start_api -listen :$TEST_API_PORT
ok 0 "System restarted (Round 2)"

# 7. Verify Persistence
log "Verifying Persistence..."

# Check Runtime (Linux)
if ip link show eth1.99 >/dev/null 2>&1; then
    ok 0 "VLAN restored in Runtime after restart"
else
    ok 1 "VLAN NOT restored in Runtime after restart"
fi

# Check API State (returns JSON)
MAX_RETRIES=3
ATTEMPT=1
while [ $ATTEMPT -le $MAX_RETRIES ]; do
    curl -s -m 10 --retry 2 --retry-delay 1 --retry-all-errors \
        "http://127.0.0.1:$TEST_API_PORT/api/config" > /tmp/api_result_$$.json 2>/dev/null

    # Verify non-empty response
    if [ ! -s "/tmp/api_result_$$.json" ]; then
        log "WARN: API response empty (attempt $ATTEMPT/$MAX_RETRIES), retrying..."
        sleep 1
        ATTEMPT=$((ATTEMPT + 1))
        continue
    fi

    if grep -q "99" /tmp/api_result_$$.json; then
        ok 0 "API reports VLAN exists"
        break
    else
        if [ $ATTEMPT -eq $MAX_RETRIES ]; then
            log "API Response:"
            cat /tmp/api_result_$$.json
            ok 1 "API lost VLAN config!"
        fi
    fi
    ATTEMPT=$((ATTEMPT + 1))
done

if [ $ATTEMPT -gt $MAX_RETRIES ]; then
    ok 1 "API failed to return valid config after $MAX_RETRIES attempts"
fi

log "Persistence Test completed"
stop_ctl
rm -rf "$TEST_STATE_DIR"
