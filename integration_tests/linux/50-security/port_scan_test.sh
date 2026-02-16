#!/bin/sh
TEST_TIMEOUT=30
set -x
#
# Port Scanning Integration Test
# Verifies active port scanning functionality
#

. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

if ! command -v nmap >/dev/null 2>&1; then
    echo "1..0 # SKIP nmap not found"
    exit 0
fi

# Test plan
plan 3

# Pick random port to avoid conflicts
API_PORT=8080

export FLYWALL_SKIP_API=1

CONFIG_FILE=$(mktemp_compatible "port_scan.hcl")

cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

api {
    enabled = true
    listen = "0.0.0.0:$API_PORT"
    require_auth = true
}
EOF

# Ensure directory for auth.json exists
mkdir -p /opt/flywall/var/lib

# Use unique cookie file
COOKIE_FILE="$STATE_DIR/cookies.txt"

export FLYWALL_NO_SANDBOX=1
start_ctl "$CONFIG_FILE"

start_api -listen :$API_PORT

# Test 1: Setup Admin User (Public Endpoint)
diag "Test 1: Setup Admin"
# Ensure state is clean, so we are in setup mode (no users)
# POST /api/setup/create-admin
# Use stronger password in case complexity requirements are enforced
# Capture response to debug
create_resp=$(curl -s -X POST http://127.0.0.1:$API_PORT/api/setup/create-admin \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"StrongPassword123!"}')

if echo "$create_resp" | grep -q "error"; then
    diag "Create Admin Failed: $create_resp"
fi

# Test 2: Login to get session and CSRF token
diag "Test 2: Login"
login_resp=$(curl -s -c "$COOKIE_FILE" -X POST http://127.0.0.1:$API_PORT/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"StrongPassword123!"}')

# Verify login success
if echo "$login_resp" | grep -q '"authenticated":true'; then
    ok 0 "Login successful"
else
    ok 1 "Login failed"
    diag "Login response: $login_resp"
    diag "--- API LOG ---"
    cat "$API_LOG"
    exit 1
fi

CSRF_TOKEN=$(echo "$login_resp" | jq -r .csrf_token)
diag "CSRF Token: $CSRF_TOKEN"

# Test 3: Start scan endpoint (Network Scan)
diag "Test 3: Start scan endpoint"

# Give control plane a moment to stabilize (RPC reconnection can cause hangs)
dilated_sleep 1

# Note: Using /api/scanner/network which returns 202 Accepted
# Use --max-time to avoid hanging if RPC is slow
response=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 -X POST \
    -b "$COOKIE_FILE" \
    -H "Content-Type: application/json" \
    -H "X-CSRF-Token: $CSRF_TOKEN" \
    -d '{"cidr":"127.0.0.1/32"}' \
    "http://127.0.0.1:$API_PORT/api/scanner/network")

# Retry once if we got 000 (transient RPC connection issue)
if [ "$response" = "000" ]; then
    diag "Got 000, retrying after sleep..."
    dilated_sleep 2
    response=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 -X POST \
        -b "$COOKIE_FILE" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $CSRF_TOKEN" \
        -d '{"cidr":"127.0.0.1/32"}' \
        "http://127.0.0.1:$API_PORT/api/scanner/network")
fi

if [ "$response" = "202" ]; then
    ok 0 "Scan start endpoint returned success ($response)"
else
    ok 1 "Scan start endpoint failed (expected 202, got $response)"
    if [ -f "$API_LOG" ]; then
        diag "--- API LOG (Failure Context) ---"
        cat "$API_LOG"
    fi
fi

# Wait for scan to potentially finish (async)
dilated_sleep 2

# Test 4: Get scan status
diag "Test 4: Scan status endpoint"
response=$(curl -s -o /dev/null -w "%{http_code}" \
    -b "$COOKIE_FILE" \
    "http://127.0.0.1:$API_PORT/api/scanner/status")

if [ "$response" = "200" ]; then
    ok 0 "Scan status endpoint returned 200"
else
    ok 1 "Scan status endpoint failed (expected 200, got $response)"
fi

rm -f "$CONFIG_FILE"
diag "Port scanning test completed"
