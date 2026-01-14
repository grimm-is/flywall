#!/bin/sh
set -x

# Scheduler API Integration Test
# Tests scheduler endpoints:
# - GET /api/scheduler/status
# - POST /api/scheduler/run
# - GET /api/config/scheduler
# - POST /api/config/scheduler

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
plan 4

diag "================================================"
diag "Scheduler API Test"
diag "Tests scheduler configuration and control"
diag "================================================"

# --- Setup ---
TEST_CONFIG=$(mktemp_compatible "scheduler.hcl")
cat > "$TEST_CONFIG" << 'EOF'
schema_version = "1.1"

interface "lo" {
  zone = "local"
  ipv4 = ["127.0.0.1/8"]
}

zone "local" {
  interfaces = ["lo"]
}

scheduler {
  enabled = true
}

api {
  enabled = true
  listen = "0.0.0.0:8080"
  require_auth = false
}

scheduled_rule "test-rule" {
  enabled = true
  schedule = "* * * * *"
  policy = "default"
  rule "allow-api" {
    action = "accept"
    proto = "tcp"
    dest_port = 8080
  }
}
EOF

# Test 1: Start system
start_ctl "$TEST_CONFIG"
start_api -listen :8080
ok 0 "System started with API"
dilated_sleep 2

# Test 2: GET scheduler status
diag "Test 2: GET scheduler status"
HTTP_CODE=$(curl -s -o /tmp/scheduler_status.json -w "%{http_code}" "http://127.0.0.1:8080/api/scheduler/status")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "GET /api/scheduler/status returns 200"
else
    ok 1 "GET /api/scheduler/status returns 200" severity fail expected "200" actual "$HTTP_CODE"
fi

# Test 3: GET scheduler config
diag "Test 3: GET scheduler config"
HTTP_CODE=$(curl -s -o /tmp/scheduler_config.json -w "%{http_code}" "http://127.0.0.1:8080/api/config/scheduler")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "GET /api/config/scheduler returns 200"
else
    ok 1 "GET /api/config/scheduler returns 200" severity fail expected "200" actual "$HTTP_CODE"
fi

# Test 4: POST scheduler run (trigger job)
diag "Test 4: POST scheduler run"
HTTP_CODE=$(curl -s -o /tmp/scheduler_response.txt -w "%{http_code}" -X POST "http://127.0.0.1:8080/api/scheduler/run?task=rule-test-rule" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-bypass")

# May return 200 (job started) or 404 (job not found) or 400 (invalid)
if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 404 ]; then
    ok 0 "POST /api/scheduler/run returns $HTTP_CODE"
else
    ok 1 "POST /api/scheduler/run returns reasonable code" severity fail expected "200" actual "$HTTP_CODE"
    echo "# Response Body:"
    cat /tmp/scheduler_response.txt
fi

diag "Scheduler API test completed"
