#!/bin/sh
set -x

# API Validation Integration Test (Sad Paths)
# Verifies: Input validation, Malformed JSON handling, and Referential Integrity

TEST_TIMEOUT=60
. "$(dirname "$0")/../common.sh"

# Load log helpers for TAP-safe output
. "$(dirname "$0")/../lib/log_helpers.sh"

require_root
require_binary
cleanup_with_logs() {
    if [ -f "$API_LOG" ]; then
        diag "API Log Dump:"
        show_log_tail "$API_LOG" 10 | sed 's/^/# /'
    fi
    cleanup_processes
}
trap cleanup_with_logs EXIT INT TERM

log() { echo "[TEST] $1"; }

CONFIG_FILE=$(mktemp_compatible "api_validation.hcl")
KEY_STORE=$(mktemp_compatible "apikeys_validation.json")

# Configuration
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

api {
  enabled = false
  listen  = "127.0.0.1:$TEST_API_PORT"
  require_auth = true
  key_store_path = "$KEY_STORE"

  key "admin-key" {
    key = "gfw_admin123"
    permissions = ["config:write", "config:read", "dhcp:write"]
  }
}

zone "lan" {}
interface "eth0" {
    ipv4 = ["10.0.0.1/24"]
    zone = "lan"
}
EOF

# Start Control Plane
start_ctl "$CONFIG_FILE"

# Start API Server
export FLYWALL_NO_SANDBOX=1

start_api -listen :$TEST_API_PORT

API_URL="http://127.0.0.1:$TEST_API_PORT/api"
AUTH_HEADER="X-API-Key: gfw_admin123"

api_post() {
    endpoint="$1"
    data="$2"
    if command -v curl >/dev/null 2>&1; then
        # Retry up to 3 times to handle RPC instability
        for attempt in 1 2 3; do
            body_file=$(mktemp)
            code=$(curl -s -o "$body_file" -w "%{http_code}" -X POST "$API_URL$endpoint" \
                -H "Content-Type: application/json" \
                -H "$AUTH_HEADER" \
                -d "$data" 2>/dev/null)
            body=$(cat "$body_file")
            rm -f "$body_file"
            # If we got a valid HTTP response, return it
            if [ "$code" != "000" ]; then
                echo "$code $body"
                return 0
            fi
            sleep 1
        done
        echo "$code $body"
    else
        fail "curl is required for validation tests"
    fi
}

api_delete() {
    endpoint="$1"
    if command -v curl >/dev/null 2>&1; then
        out=$(curl -s -w "\n%{http_code}" -X DELETE "$API_URL$endpoint" \
            -H "Content-Type: application/json" \
            -H "$AUTH_HEADER")
        body=$(echo "$out" | sed '$d')
        code=$(echo "$out" | tail -n1)
        echo "$code $body"
    else
        fail "curl is required"
    fi
}

# 1. Test Malformed JSON
log "Test 1: Malformed JSON..."
DATA='{broken' # Invalid JSON (Start but no end)
RESULT=$(api_post "/config/dhcp" "$DATA")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" = "400" ]; then
    pass "Malformed JSON rejected with 400"
else
    fail "Malformed JSON check failed. Expected 400, Got code $CODE. Body: $RESULT"
fi

# Brief pause to let API stabilize
sleep 1

# 2. Test Invalid Logic (Bad CIDR)
log "Test 2: Invalid CIDR..."
DATA='{
  "name": "eth1",
  "action": "create",
  "ipv4": ["999.999.999.999/24"]
}'
RESULT=$(api_post "/interfaces/update" "$DATA")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" = "400" ]; then
    pass "Invalid CIDR rejected with 400"
else
    fail "Invalid CIDR check failed. Got: $RESULT"
fi

# 3. Setup Dependencies for Integrity Test
log "Test 3: Setting up dependency chain..."
# Ensure DHCP is configured on eth0
DATA='{
  "enabled": true,
  "scopes": [
    {
      "name": "lan-scope",
      "interface": "eth0",
      "range_start": "10.0.0.100",
      "range_end": "10.0.0.200"
    }
  ]
}'
RESULT=$(api_post "/config/dhcp" "$DATA")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" != "200" ]; then
    fail "Setup failed: Could not configure DHCP: $RESULT"
fi

# Apply config to ensure state is active
api_post "/config/apply" "{}" >/dev/null

# 4. Test Referential Integrity (Delete in-use resource)
log "Test 4: Deleting in-use interface (eth0)..."
# Attempt to delete eth0, which is referenced by DHCP scope 'lan-scope'
DATA='{
  "name": "eth0",
  "action": "delete"
}'
# Note: interfaces/update might handle delete via action
RESULT=$(api_post "/interfaces/update" "$DATA")
CODE=$(echo "$RESULT" | awk '{print $1}')

# We expect either 400 (Bad Request) or 409 (Conflict)
if [ "$CODE" = "400" ] || [ "$CODE" = "409" ]; then
    pass "Referential integrity check passed (Got $CODE)"
else
    # NOTE: If this fails with 200, it means we have a bug: we allowed deleting a dependency!
    fail "Referential integrity check FAILED. Allowed deleting in-use interface! Got: $RESULT"
fi

log "Sad Path Tests PASSED"
exit 0
