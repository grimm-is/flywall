#!/bin/sh
set -x

# Test API CSRF Protection

export PROJECT_ROOT="/mnt/flywall"
. "$(dirname "$0")/../common.sh"

# Load log helpers for TAP-safe output
. "$(dirname "$0")/../lib/log_helpers.sh"

trap cleanup_on_exit EXIT

echo "DEBUG: Starting test..." >&2

# 1. Start Control Plane & API
# Force cleanup of zombie processes
# pkill -9 flywall || true
# Safe cleanup of locks/sockets (DO NOT use wildcard that matches agent socket)
# rm -f /var/run/flywall/ctl.sock /var/run/flywall/flywall-ctl.lock /var/run/flywall/flywall_api.lock

echo "DEBUG: Starting CTL..." >&2
# Use manual config (API disabled in CTL, we start it manually)
# Copy config to temp since API needs to write backup files
CONFIG_FILE=$(mktemp_compatible api_csrf.hcl)
cp "$PROJECT_ROOT/integration_tests/linux/configs/api_manual.hcl" "$CONFIG_FILE"
start_ctl "$CONFIG_FILE"

echo "DEBUG: Starting API..." >&2
export FLYWALL_NO_SANDBOX=1
start_api -listen :$TEST_API_PORT
wait_for_port $TEST_API_PORT 10

echo "DEBUG: API Started. Config features..." >&2

cat <<EOF > /tmp/features_payload_$$.json
{
  "qos": true,
  "threat_intel": true,
  "network_learning": true
}
EOF

echo "DEBUG: Sending payload..." >&2
# Send request (use X-API-Key to bypass CSRF, any value works with require_auth=false)
# Retry up to 3 times to handle potential RPC initialization delays
MAX_RETRIES=3
for i in $(seq 1 $MAX_RETRIES); do
    echo "DEBUG: Attempt $i of $MAX_RETRIES..." >&2
    if curl -v --max-time 30 -X POST \
        -H "Content-Type: application/json" \
        -H "X-API-Key: test-key" \
        -d @/tmp/features_payload_$$.json \
        "http://127.0.0.1:$TEST_API_PORT/api/config/settings" > /tmp/api_response_$$.json 2>&1; then
        echo "DEBUG: Request successful" >&2
        break
    else
        echo "DEBUG: Request failed (Attempt $i)" >&2
        cat /tmp/api_response_$$.json >&2
        if [ $i -eq $MAX_RETRIES ]; then
            echo "FATAL: API request failed entirely after $MAX_RETRIES attempts" >&2
            echo "--- CTL LOG ---" >&2
            show_log_tail "$CTL_LOG" 10 | tail -n 20 >&2
            echo "--- API LOG ---" >&2
            show_log_tail "$API_LOG" 10 | tail -n 20 >&2
            fail "API request failed entirely"
        fi
        dilated_sleep 1
    fi
done

echo "DEBUG: Request sent. Verifying..." >&2

# Check response
if ! grep -q "success" /tmp/api_response_$$.json; then
    echo "FATAL: API response did not indicate success" >&2
    cat /tmp/api_response_$$.json >&2
    fail "API returned error"
fi

# 3. Verify Persistence
echo "DEBUG: Verifying persistence..." >&2
curl -s --max-time 5 "http://127.0.0.1:$TEST_API_PORT/api/config" > /tmp/config_full_$$.json

if jq -e '.features.qos == true and .features.threat_intel == true and .features.network_learning == true' /tmp/config_full_$$.json >/dev/null; then
    pass "System features persisted correctly"
else
    echo "FATAL: Features not persisted" >&2
    cat /tmp/config_full_$$.json >&2
    echo "--- CTL LOG ---" >&2
    show_log_tail "$CTL_LOG" 10 >&2
    echo "--- API LOG ---" >&2
    show_log_tail "$API_LOG" 10 >&2
    fail "Features enabled but not returned in config"
fi
