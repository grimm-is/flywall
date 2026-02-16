#!/bin/sh
set -x
#
# Pending Rule Approval Integration Test
# Verifies pending rule approval workflow
#

. "$(dirname "$0")/../common.sh"

# Load log helpers for TAP-safe output
. "$(dirname "$0")/../lib/log_helpers.sh"

require_root
require_binary
cleanup_on_exit

CONFIG_FILE=$(mktemp_compatible "pending_rules.hcl")

cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

api {
    enabled = true
    listen = "0.0.0.0:8080"
    require_auth = false
}

interface "lo" {
    ipv4 = ["127.0.0.1/8", "192.168.1.1/24"]
}

zone "local" {}
EOF

plan 2

start_ctl "$CONFIG_FILE"

start_api -listen :8080

# Wait for API to be ready
wait_for_port 8080 10

# Additional wait for API to be fully ready
for i in $(seq 1 10); do
    if curl -s --connect-timeout 2 http://127.0.0.1:8080/api/status >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Test 1: Get pending rules
# Test 1: Pending rules endpoint
diag "Test 1: Pending rules endpoint"
response=$(curl -s -m 5 -o /dev/null -w "%{http_code}" "http://127.0.0.1:8080/api/learning/rules?status=pending")
if [ "$response" != "200" ]; then
    diag "Request failed with status $response"
    show_log_tail "$API_LOG" 10
    show_log_errors "$CTL_LOG"
    ok 1 "Pending rules endpoint failed"
else
    ok 0 "Pending rules endpoint accessible (status: $response)"
fi

# Test 2: Approve/reject endpoint exists
diag "Test 2: Approve endpoint"
# Note: This checks strictly for 404 (Not Found rule) or 200 (Success) or 500.
# Since 'test-rule' doesn't exist, we expect 404, but definitely NOT 000/Timeouts.
# The endpoint structure is POST /api/learning/rules/{id}/{action}
response=$(curl -s -m 5 -o /dev/null -w "%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -H "X-API-Key: bypass-csrf" \
    "http://127.0.0.1:8080/api/learning/rules/test-rule/approve" 2>&1)

# We accept 200 (Success) or 404 (Rule not found) or 503 (Service unavailable)
# We strictly fail on 000 (Timeout) or 405 (Method Not Allowed - implying wrong path)
if [ "$response" != "200" ] && [ "$response" != "404" ] && [ "$response" != "503" ]; then
    diag "Request failed with status $response"
    show_log_tail "$API_LOG" 10
    show_log_errors "$CTL_LOG"
    ok 1 "Approve endpoint failed"
else
    ok 0 "Approve endpoint accessible (status: $response)"
fi

rm -f "$CONFIG_FILE"
diag "Pending rule approval test completed"
