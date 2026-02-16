#!/bin/sh

# API Proxy Test
# Verifies: Unix Socket + Userspace Proxy architecture
# 1. API creates Unix Socket.
# 2. Control Plane spawns Proxy.
# 3. Proxy listens on TCP and forwards to Socket.

set -e
# set -x # Enable for debug
source "$(dirname "$0")/../common.sh"

TEST_TIMEOUT=30

require_root
require_binary
cleanup_on_exit

diag "Test: API Unix Socket Proxy"
plan 3

# Pick random port
export API_PORT=$TEST_API_PORT

# Create config
CTL_CONFIG=$(mktemp_compatible "ctl_test.hcl")
cat > "$CTL_CONFIG" << EOF
schema_version = "1.1"
api {
  enabled = true
  require_auth = false
  listen = ":$API_PORT"
}
EOF

# Start Control Plane (Daemon)
# This spawns _api-server and _proxy
export FLYWALL_SKIP_API=0  # We NEED the proxy for this test
export FLYWALL_NO_SANDBOX=1 # Disable chroot as test environment lacks /proc mount in jail
start_ctl "$CTL_CONFIG"
diag "Control plane started (PID $CTL_PID)"

# Wait for sockets (Manual loop to capture logs on failure)
diag "Waiting for API socket..."
for i in $(seq 1 15); do
    if [ -S "$RUN_DIR/api/api.sock" ]; then
        break
    fi
    sleep 1
done

if [ ! -S "$RUN_DIR/api/api.sock" ]; then
    diag "TIMEOUT waiting for API socket"
    diag "Process List:"
    ps aux
    diag "Network Sockets:"
    netstat -tlpn
    diag "CTL Log Content:"
    cat "$CTL_LOG"
    fail "API socket never appeared"
fi

diag "Waiting for Proxy port $API_PORT..."
wait_for_port $API_PORT 10 tcp

# Verify Socket
if [ -S "$RUN_DIR/api/api.sock" ]; then
    pass "Unix socket $RUN_DIR/api/api.sock created"
else
    fail "Unix socket missing"
fi

# Verify Connectivity via Proxy
diag "Testing HTTP connectivity via Proxy (localhost:$API_PORT)..."
# Retry up to 3 times as proxy might need time to start
PROXY_CONNECTED=0
for i in 1 2 3; do
    if curl -s --connect-timeout 2 http://127.0.0.1:$API_PORT/api/status > /dev/null; then
        PROXY_CONNECTED=1
        break
    else
        diag "Proxy attempt $i failed, retrying..."
        sleep 1
    fi
done

if [ $PROXY_CONNECTED -eq 1 ]; then
    pass "Can connect via Proxy"
else
    fail "Proxy connectivity failed (curl)"
fi

# Check logs for confirmation of architecture
if grep -q "Starting API server on $RUN_DIR/api/api.sock (unix, HTTP)" "$CTL_LOG"; then
    pass "API log confirms Unix listener"
else
    diag "CTL Log Content:"
    cat "$CTL_LOG"
    fail "API log does not confirm Unix listener"
fi

# Explicitly verify process ownership of port $API_PORT
diag "Verifying process ownership of port $API_PORT..."
NETSTAT_OUT=$(netstat -tlpn | grep ":$API_PORT")
diag "Netstat for $API_PORT: $NETSTAT_OUT"

if echo "$NETSTAT_OUT" | grep -q "_proxy"; then
    pass "Port $API_PORT is held by _proxy"
elif echo "$NETSTAT_OUT" | grep -q "_api-server"; then
    fail "Port $API_PORT is held by _api-server (Arch violation!)"
else
    # Fallback to checking PID if name truncated
    PROXY_PID=$(pgrep -f "_proxy" | head -n1)
    if echo "$NETSTAT_OUT" | grep -q "$PROXY_PID"; then
        pass "Port $API_PORT is held by _proxy (PID match)"
    else
        fail "Port $API_PORT held by unknown process"
    fi
fi
