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
cat > "$TEST_CONFIG" <<EOF
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

api {
  enabled = false
  require_auth = false
}

notifications {
  enabled = true
}
EOF

# Test 1: Start system
start_ctl "$TEST_CONFIG"
start_api -listen :$TEST_API_PORT
ok 0 "System started with API"

# Test 2: List empty rules
diag "Checking initial rules..."
# Ensure clean state
curl -s -m 5 --retry 2 --retry-delay 1 --retry-all-errors \
    -X DELETE "http://127.0.0.1:$TEST_API_PORT/api/alerts/rules" > /dev/null 2>&1 || true

RULES_RESPONSE=$(curl -s -m 10 --retry 3 --retry-delay 1 --retry-all-errors \
    "http://127.0.0.1:$TEST_API_PORT/api/alerts/rules")

# Safely get length - default to 0 if null, empty, or invalid JSON
RULE_COUNT=$(echo "$RULES_RESPONSE" | jq -r 'if type == "array" then length else 0 end' 2>/dev/null)
if [ -z "$RULE_COUNT" ] || [ "$RULE_COUNT" = "null" ]; then
    RULE_COUNT="0"
fi
if [ "$RULE_COUNT" = "0" ]; then
    ok 0 "Initial rules empty"
else
    diag "Found $RULE_COUNT initial rules"
    ok 1 "Initial rules empty" severity fail error "Rules found where none expected"
fi

# Test 3: Create a rule
diag "Creating a new alert rule..."
HTTP_CODE=$(curl -s -m 15 --retry 3 --retry-delay 1 --retry-all-errors \
    -o /tmp/create_response_$$.json -w "%{http_code}" \
    -X POST "http://127.0.0.1:$TEST_API_PORT/api/alerts/rules" \
    -H "X-API-Key: test-bypass" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "High Bandwidth",
      "enabled": true,
      "severity": "warning",
      "condition": "bandwidth.wan > 100Mbps",
      "channels": ["log"],
      "cooldown": 300000000000
    }')

if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 201 ]; then
    # Verify it exists
    RULES=$(curl -s -m 10 --retry 2 --retry-delay 1 --retry-all-errors \
        "http://127.0.0.1:$TEST_API_PORT/api/alerts/rules")
    if echo "$RULES" | jq -e '.[] | select(.name == "High Bandwidth")' >/dev/null 2>&1; then
        ok 0 "Rule created and persisted"
    else
        diag "Rules response: $RULES"
        ok 1 "Rule created and persisted" severity fail error "Rule not found in list"
    fi
else
    diag "Create response: $(cat /tmp/create_response_$$.json 2>/dev/null)"
    ok 1 "Rule creation failed" severity fail expected "200/201" actual "$HTTP_CODE"
fi

# Test 4: Query History (should be empty or return list)
diag "Querying alert history..."
HTTP_CODE=$(curl -s -m 10 --retry 3 --retry-delay 1 --retry-all-errors \
    -o /dev/null -w "%{http_code}" \
    "http://127.0.0.1:$TEST_API_PORT/api/alerts/history?limit=10")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "History endpoint returns 200 OK"
else
    ok 1 "History endpoint returns 200 OK" severity fail expected "200" actual "$HTTP_CODE"
fi

diag "All tests completed"
