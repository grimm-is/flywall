#!/bin/sh
set -x

# Uplink API Integration Test
# Tests the Multi-WAN uplink API endpoints:
# - GET /api/uplinks/groups
# - POST /api/uplinks/toggle
# - POST /api/uplinks/test

TEST_TIMEOUT=60
. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

if ! command -v curl >/dev/null 2>&1; then
    echo "1..0 # SKIP curl command not found"
    exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "1..0 # SKIP jq command not found"
    exit 0
fi

# --- Test Suite ---
plan 4

diag "================================================"
diag "Uplink API Test"
diag "Tests Multi-WAN API endpoints"
diag "================================================"

# --- Setup ---
TEST_CONFIG=$(mktemp_compatible "uplink.hcl")
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

# Simulated uplink group (no real WAN, just testing API response)
uplink_group "default" {
  enabled = true
  failover_mode = "priority"
  source_networks = ["0.0.0.0/0"]
  
  uplink "primary" {
    interface = "eth0"
    type = "wan"
    enabled = true
  }
}
EOF

# Test 1: Start system
export FLYWALL_LOG_LEVEL=debug
start_ctl "$TEST_CONFIG"
start_api -listen :8080
ok 0 "System started with API"

# Wait for initialization
dilated_sleep 2

# Test 2: Get uplink groups
diag "Testing GET /api/uplinks/groups..."
HTTP_CODE=$(curl -s -o /tmp/uplink_groups.json -w "%{http_code}" "http://127.0.0.1:8080/api/uplinks/groups")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "GET /api/uplinks/groups returns 200"
else
    ok 1 "GET /api/uplinks/groups returns 200" severity fail expected "200" actual "$HTTP_CODE"
fi

# Test 3: Toggle uplink (disable then enable)
diag "Testing POST /api/uplinks/toggle..."
TOGGLE_RESPONSE=$(curl -s -X POST "http://127.0.0.1:8080/api/uplinks/toggle" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-bypass" \
  -d '{"group_name": "default", "uplink_name": "primary", "enabled": false}')

if echo "$TOGGLE_RESPONSE" | grep -q '"success":true'; then
    ok 0 "Toggle uplink successful"
else
    diag "Toggle response: $TOGGLE_RESPONSE"
    ok 1 "Toggle uplink successful" severity fail
fi

# Test 4: Test uplink connectivity
diag "Testing POST /api/uplinks/test..."
HTTP_CODE=$(curl -s -o /tmp/uplink_test.json -w "%{http_code}" -X POST "http://127.0.0.1:8080/api/uplinks/test" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-bypass" \
  -d '{"group_name": "default", "uplink_name": "primary"}')

if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "POST /api/uplinks/test returns 200"
else
    cat /tmp/uplink_test.json
    ok 1 "POST /api/uplinks/test returns 200" severity fail expected "200" actual "$HTTP_CODE"
fi

diag "All tests completed"
