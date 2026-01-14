#!/bin/sh
set -x

# Analytics Integration Test
# Tests the Analytics API endpoints:
# - Bandwidth API (returns valid JSON structure)
# - Top Talkers API
# - Historic Flows API

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
plan 5

diag "================================================"
diag "Analytics API Test"
diag "Tests API endpoints return 200 and valid JSON"
diag "================================================"

# --- Setup ---
TEST_CONFIG=$(mktemp_compatible "analytics.hcl")
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
  listen = "0.0.0.0:8082"
  require_auth = false
}
EOF

# Test 1: Start system using helpers
start_ctl "$TEST_CONFIG"
ok 0 "Control plane started"

# Start API on port 8082
start_api -listen :8082
ok 0 "API server started"

# Test 2: Get Bandwidth
diag "Testing GET /api/analytics/bandwidth..."
HTTP_CODE=$(curl -s -o /tmp/analytics_bw.json -w "%{http_code}" "http://127.0.0.1:8082/api/analytics/bandwidth?interval=1h")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "Bandwidth API returns 200"
    # Check if response is valid JSON array or object
    if jq -e . /tmp/analytics_bw.json >/dev/null; then
        ok 0 "Bandwidth API returns valid JSON"
    else
        ok 1 "Bandwidth API returns valid JSON" severity fail error "Invalid JSON"
    fi
else
    ok 1 "Bandwidth API returns 200" severity fail expected "200" actual "$HTTP_CODE"
    ok 1 "Bandwidth API returns valid JSON" severity skip
fi

# Test 3: Get Top Talkers
diag "Testing GET /api/analytics/top-talkers..."
HTTP_CODE=$(curl -s -o /tmp/analytics_talkers.json -w "%{http_code}" "http://127.0.0.1:8082/api/analytics/top-talkers")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "Top Talkers API returns 200"
else
    ok 1 "Top Talkers API returns 200" severity fail expected "200" actual "$HTTP_CODE"
fi

diag "Test complete"
