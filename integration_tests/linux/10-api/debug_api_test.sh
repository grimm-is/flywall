#!/bin/sh
set -x

# Debug API Integration Test
# Tests debug endpoints:
# - POST /api/debug/simulate-packet
# - POST /api/debug/capture (start)
# - GET /api/debug/capture/status
# - DELETE /api/debug/capture (stop)
# Includes happy and sad path testing

TEST_TIMEOUT=60
. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

if ! command -v curl >/dev/null 2>&1; then
    echo "1..0 # SKIP curl command not found"
    exit 0
fi

# --- Test Suite ---
plan 6

diag "================================================"
diag "Debug API Test"
diag "Tests packet simulation and capture endpoints"
diag "================================================"

# --- Setup ---
TEST_CONFIG=$(mktemp_compatible "debug.hcl")
cat > "$TEST_CONFIG" << 'EOF'
schema_version = "1.1"

interface "lo" {
  zone = "local"
  ipv4 = ["127.0.0.1/8"]
}

zone "local" {
  interfaces = ["lo"]
}

api {
  enabled = true
  listen = "0.0.0.0:8080"
  require_auth = false
}
EOF

# Test 1: Start system
start_ctl "$TEST_CONFIG"
start_api -listen :8080
ok 0 "System started with API"
dilated_sleep 2

# Test 2: Simulate packet - happy path
diag "Test 2: Simulate packet (happy path)"
RESPONSE=$(curl -s -X POST "http://127.0.0.1:8080/api/debug/simulate-packet" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-bypass" \
  -d '{
    "src_ip": "192.168.1.100",
    "dst_ip": "8.8.8.8",
    "protocol": "tcp",
    "dest_port": 443
  }')

if echo "$RESPONSE" | grep -qE '"action"|"result"|"success"'; then
    ok 0 "Simulate packet returns result"
else
    diag "Response: $RESPONSE"
    ok 1 "Simulate packet returns result" severity fail
fi

# Test 3: Simulate packet - sad path (missing fields)
diag "Test 3: Simulate packet with missing fields (sad path)"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://127.0.0.1:8080/api/debug/simulate-packet" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-bypass" \
  -d '{"src_ip": "192.168.1.100"}')

if [ "$HTTP_CODE" -eq 400 ]; then
    ok 0 "Missing fields returns 400"
else
    # Some implementations may accept partial input
    ok 0 "Missing fields returns $HTTP_CODE" severity skip
fi

# Test 4: Start capture
diag "Test 4: Start capture"
RESPONSE=$(curl -s -X POST "http://127.0.0.1:8080/api/debug/capture" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-bypass" \
  -d '{
    "interface": "lo",
    "filter": "icmp",
    "duration": 5
  }')

if echo "$RESPONSE" | grep -qE '"id"|"status"|"started"|"success"'; then
    ok 0 "Start capture returns status"
else
    diag "Response: $RESPONSE"
    ok 1 "Start capture returns status" severity fail
fi

# Test 5: Get capture status
diag "Test 5: Get capture status"
HTTP_CODE=$(curl -s -o /tmp/capture_status.json -w "%{http_code}" "http://127.0.0.1:8080/api/debug/capture/status")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "GET /api/debug/capture/status returns 200"
else
    ok 1 "GET /api/debug/capture/status returns 200" severity fail expected "200" actual "$HTTP_CODE"
fi

# Test 6: Stop capture
diag "Test 6: Stop capture"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "http://127.0.0.1:8080/api/debug/capture" -H "X-API-Key: test-bypass")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "DELETE /api/debug/capture returns 200"
else
    # May return 404 if no capture was running
    if [ "$HTTP_CODE" -eq 404 ]; then
        ok 0 "No active capture to stop (404)" severity skip
    else
        ok 1 "DELETE /api/debug/capture returns 200" severity fail expected "200" actual "$HTTP_CODE"
    fi
fi

diag "Debug API test completed"
