#!/bin/sh
set -x

# Test API CSRF Protection

export PROJECT_ROOT="/mnt/flywall"
. "$(dirname "$0")/../common.sh"

trap cleanup_on_exit EXIT

echo "DEBUG: Starting test..." >&2

# 1. Start Control Plane & API
# Force cleanup of zombie processes
# pkill -9 flywall || true
# Safe cleanup of locks/sockets (DO NOT use wildcard that matches agent socket)
# rm -f /var/run/flywall-ctl.sock /var/run/flywall-ctl.lock /var/run/flywall_api.lock

echo "DEBUG: Starting CTL..." >&2
# Use manual config (API disabled in CTL, we start it manually)
# Copy config to temp since API needs to write backup files
CONFIG_FILE=$(mktemp_compatible api_csrf.hcl)
cp "$PROJECT_ROOT/integration_tests/linux/configs/api_manual.hcl" "$CONFIG_FILE"
start_ctl "$CONFIG_FILE"

echo "DEBUG: Starting API..." >&2
start_api

echo "DEBUG: API Started. Config features..." >&2

cat <<EOF > /tmp/features_payload.json
{
  "qos": true,
  "threat_intel": true,
  "network_learning": true
}
EOF

echo "DEBUG: Sending payload..." >&2
# Send request (use X-API-Key to bypass CSRF, any value works with require_auth=false)
# Timeout increased from 10s to 30s to allow for ApplyConfig RPC delay
if ! curl -v --max-time 30 -X POST \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-key" \
    -d @/tmp/features_payload.json \
    "http://127.0.0.1:8080/api/config/settings" > /tmp/api_response.json 2>&1; then
    echo "FATAL: API request failed" >&2
    cat /tmp/api_response.json >&2
    echo "--- CTL LOG ---" >&2
    cat "$CTL_LOG" | tail -n 20 >&2
    echo "--- API LOG ---" >&2
    cat "$API_LOG" | tail -n 20 >&2
    fail "API request failed entirely"
fi

echo "DEBUG: Request sent. Verifying..." >&2

# Check response
if ! grep -q "success" /tmp/api_response.json; then
    echo "FATAL: API response did not indicate success" >&2
    cat /tmp/api_response.json >&2
    fail "API returned error"
fi

# 3. Verify Persistence
echo "DEBUG: Verifying persistence..." >&2
curl -s --max-time 5 "http://127.0.0.1:8080/api/config" > /tmp/config_full.json

if jq -e '.features.qos == true and .features.threat_intel == true and .features.network_learning == true' /tmp/config_full.json >/dev/null; then
    pass "System features persisted correctly"
else
    echo "FATAL: Features not persisted" >&2
    cat /tmp/config_full.json >&2
    echo "--- CTL LOG ---" >&2
    cat "$CTL_LOG" >&2
    echo "--- API LOG ---" >&2
    cat "$API_LOG" >&2
    fail "Features enabled but not returned in config"
fi
