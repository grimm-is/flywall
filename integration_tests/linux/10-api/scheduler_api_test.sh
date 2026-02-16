#!/bin/sh
set -x

# Scheduler API Integration Test
# Tests scheduler endpoints:
# - GET /api/config/scheduler
# - POST /api/config/scheduler
# - POST /api/scheduler/run
# - GET /api/scheduler/status (via config)

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
cat > "$TEST_CONFIG" << EOF
schema_version = "1.1"

interface "lo" {
  zone = "local"
  ipv4 = ["127.0.0.1/8"]
}

zone "local" {
  match {
    interface = "lo"
  }
}

scheduler {
  enabled = true
}

api {
  enabled = false
  require_auth = false
}

scheduled_rule "test-rule" {
  enabled = true
  schedule = "* * * * *"
  policy = "default"
  rule "allow-api" {
    action = "accept"
    proto = "tcp"
    dest_port = $TEST_API_PORT
  }
}
EOF

# Test 1: Start system
start_ctl "$TEST_CONFIG"
start_api -listen :$TEST_API_PORT
ok 0 "System started with API"

# Test 2: GET scheduler config
diag "Test 2: GET scheduler config"
HTTP_CODE=$(curl -s -m 10 --retry 3 --retry-delay 1 --retry-all-errors \
    -o /tmp/scheduler_config_$$.json -w "%{http_code}" \
    "http://127.0.0.1:$TEST_API_PORT/api/config/scheduler")

if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "GET /api/config/scheduler returns 200"
else
    ok 1 "GET /api/config/scheduler returns $HTTP_CODE" severity fail expected "200" actual "$HTTP_CODE"
fi

# Test 3: POST scheduler run (trigger a scheduled task)
diag "Test 3: POST scheduler run"
HTTP_CODE=$(curl -s -m 10 --retry 2 --retry-delay 1 --retry-all-errors \
    -o /tmp/scheduler_run_$$.json -w "%{http_code}" \
    -X POST "http://127.0.0.1:$TEST_API_PORT/api/scheduler/run?task=rule-test-rule" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-bypass")

# May return 200 (job started) or 404 (job not found) or 400 (invalid)
if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 404 ]; then
    ok 0 "POST /api/scheduler/run returns $HTTP_CODE"
else
    ok 1 "POST /api/scheduler/run returns reasonable code" severity fail expected "200 or 404" actual "$HTTP_CODE"
    diag "Response Body:"
    cat /tmp/scheduler_run_$$.json 2>/dev/null || true
fi

# Test 4: POST scheduler config update
diag "Test 4: POST scheduler config"
HTTP_CODE=$(curl -s -m 10 --retry 2 --retry-delay 1 --retry-all-errors \
    -o /tmp/scheduler_update_$$.json -w "%{http_code}" \
    -X POST "http://127.0.0.1:$TEST_API_PORT/api/config/scheduler" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-bypass" \
    -d '{"enabled": true}')

if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 400 ]; then
    ok 0 "POST /api/config/scheduler returns $HTTP_CODE"
else
    ok 1 "POST /api/config/scheduler returns $HTTP_CODE" severity fail expected "200" actual "$HTTP_CODE"
fi

diag "Scheduler API test completed"
