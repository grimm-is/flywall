#!/bin/sh
set -x

# Alerting Integration Test
# Tests the Alerting API and basic Rule management:
# - Create Event Rule
# - Verify Rule persistence
# - Query Alert History (basic check)

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
diag "Alerting API Test"
diag "Tests Rule CRUD and History API"
diag "================================================"

# --- Setup ---
TEST_CONFIG=$(mktemp_compatible "alerting.hcl")
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

notifications {
  enabled = true
}
EOF

# Test 1: Start system using helpers
start_ctl "$TEST_CONFIG"
start_api -listen :8080
ok 0 "System started with API"

# Test 2: List empty rules
diag "Checking initial rules..."
curl -s "http://127.0.0.1:8080/api/alerts/rules" > /tmp/alert_rules.json
if [ "$(jq '. | length' /tmp/alert_rules.json)" -eq 0 ]; then
    ok 0 "Initial rules empty"
else
    cat /tmp/alert_rules.json
    ok 1 "Initial rules empty" severity fail error "Rules found where none expected"
fi

# Test 3: Create a rule
diag "Creating a new alert rule..."
curl -s -X POST "http://127.0.0.1:8080/api/alerts/rules" \
  -H "X-API-Key: test-bypass" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "High Bandwidth",
    "enabled": true,
    "severity": "warning",
    "condition": "bandwidth.wan > 100Mbps",
    "channels": ["log"],
    "cooldown": 300000000000
  }' > /tmp/create_rule.json

# Verify it exists
curl -s "http://127.0.0.1:8080/api/alerts/rules" > /tmp/alert_rules.json
if jq -e '.[] | select(.name == "High Bandwidth")' /tmp/alert_rules.json >/dev/null; then
    ok 0 "Rule created and persisted"
else
    cat /tmp/alert_rules.json
    ok 1 "Rule created and persisted" severity fail error "Rule not found"
fi

# Test 4: Query History (should be empty or return list)
diag "Querying alert history..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:8080/api/alerts/history?limit=10")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "History endpoint returns 200 OK"
else
    ok 1 "History endpoint returns 200 OK" severity fail expected "200" actual "$HTTP_CODE"
fi

diag "All tests completed"
