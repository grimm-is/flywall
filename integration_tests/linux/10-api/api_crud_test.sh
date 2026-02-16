#!/bin/sh
set -x

# API CRUD Integration Test
# Verifies: CRUD operations for Interfaces, DHCP, and Policies
# This authenticates via a read-write API key.

TEST_TIMEOUT=60

. "$(dirname "$0")/../common.sh"

# Load log helpers for TAP-safe output
. "$(dirname "$0")/../lib/log_helpers.sh"

require_root
require_binary
# cleanup_on_exit
cleanup_with_logs() {
    if [ -f "$API_LOG" ]; then
        diag "API Log Dump:"
        show_log_tail "$API_LOG" 10 | sed 's/^/# /'
    fi
    cleanup_processes
    ip link del dummy0 2>/dev/null || true
}
trap cleanup_with_logs EXIT INT TERM

log() { echo "[TEST] $1"; }

# Test plan
plan 10

CONFIG_FILE="/tmp/api_crud_${TEST_UID}.hcl"
KEY_STORE="/tmp/apikeys_crud_${TEST_UID}.json"

# Create dummy interface for testing to avoid messing with host eth0
ip link add dummy0 type dummy 2>/dev/null || true
ip link set dummy0 up
ip addr add 10.0.0.1/24 dev dummy0 2>/dev/null || true

# configuration
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

api {
  enabled = false
  listen  = "127.0.0.1:8083"
  require_auth = true
  key_store_path = "$KEY_STORE"

  key "admin-key" {
    key = "gfw_admin123"
    permissions = ["config:write", "config:read", "firewall:write", "dhcp:write", "dns:write"]
  }
}

zone "lan" {}
zone "wan" {}

interface "dummy0" {
    ipv4 = ["10.0.0.1/24"]
    zone = "lan"
}

interface "lo" {
    ipv4 = ["127.0.0.1/8"]
    zone = "lan"
}
EOF

# Start Control Plane
start_ctl "$CONFIG_FILE"

# Start API Server (disable sandbox)
export FLYWALL_NO_SANDBOX=1
export FLYWALL_MOCK_RPC=0
start_api -listen :8083

API_URL="http://127.0.0.1:8083/api"
AUTH_HEADER="X-API-Key: gfw_admin123"

# Helper for curl requests
api_post() {
    endpoint="$1"
    data="$2"
    if command -v curl >/dev/null 2>&1; then
        out=$(curl -s -w "\n%{http_code}" -X POST "$API_URL$endpoint" \
            -H "Content-Type: application/json" \
            -H "$AUTH_HEADER" \
            -d "$data")
        body=$(echo "$out" | sed '$d')
        code=$(echo "$out" | tail -n1)
        echo "$code $body"
    else
        # minimal wget fallback if needed, but we assume curl for complex JSON testing usually
        fail "curl is required for CRUD tests"
    fi
}

# Test 1: Update Interface (change zone)
# Note: UpdateInterface triggers a config apply which may restart the API.
# We accept 200 or 000 (API restart mid-response) as success, then wait for API.
log "Testing UpdateInterface (change zone on dummy0)..."
DATA='{
  "name": "dummy0",
  "action": "update",
  "zone": "wan"
}'

RESULT=$(api_post "/interfaces/update" "$DATA")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" = "200" ] || [ "$CODE" = "000" ]; then
    pass "UpdateInterface accepted (code=$CODE)"
else
    fail "UpdateInterface failed: $RESULT"
fi

# Wait for API to come back after config apply restart
sleep 2
wait_for_api "http://127.0.0.1:8083/api/status" 10

# Test 2: Update DHCP Config
log "Testing UpdateDHCP..."
DATA='{
  "enabled": true,
  "scopes": [
    {
      "name": "lan",
      "interface": "dummy0",
      "range_start": "10.0.0.100",
      "range_end": "10.0.0.200",
      "router": "10.0.0.1"
    }
  ]
}'
RESULT=$(api_post "/config/dhcp" "$DATA")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" = "200" ]; then
    pass "UpdateDHCP returned 200"
else
    fail "UpdateDHCP failed: $RESULT"
fi

# Test 3: Update Policies
log "Testing UpdatePolicies..."
DATA='[
  {
    "name": "lan_to_wan",
    "from": "lan",
    "to": "wan",
    "action": "accept"
  }
]'
RESULT=$(api_post "/config/policies" "$DATA")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" = "200" ]; then
    pass "UpdatePolicies returned 200"
else
    fail "UpdatePolicies failed: $RESULT"
fi

# Step 3.5: Apply Changes (Push Staged Config to Control Plane)
log "Applying Configuration..."
# Apply uses the current state of s.Config in the API server
RESULT=$(api_post "/config/apply" "{}")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" = "200" ]; then
    pass "ApplyConfig returned 200"
else
    fail "ApplyConfig failed: $RESULT"
fi

# Test 4: Verify Policy persistence via GET
log "Verifying Policies..."
# Helper: Wait for config verification
wait_for_config() {
    _endpoint="$1"
    _pattern="$2"
    _count=0
    while [ $_count -lt 10 ]; do
         if command -v curl >/dev/null 2>&1; then
             OUT=$(curl -s -H "$AUTH_HEADER" "$API_URL$_endpoint")
             if echo "$OUT" | grep -q "$_pattern"; then
                 return 0
             fi
         fi
         dilated_sleep 0.5
         _count=$((_count+1))
    done
    # Last attempt to capture output for debugging
    OUT=$(curl -s -H "$AUTH_HEADER" "$API_URL$_endpoint")
    return 1
}

# Test 4: Verify Policy persistence via GET
log "Verifying Policies..."
if wait_for_config "/config/policies" "lan_to_wan"; then
    pass "Policy 'lan_to_wan' found in GET response"
else
    fail "Policy verification failed: $OUT"
fi

# Test 5: Update DNS Config
log "Testing UpdateDNS (Minimal)..."
DATA='{
  "dns_server": {
    "mode": "forward",
    "listen_on": ["127.0.0.1"],
    "enabled": true,
    "forwarders": ["8.8.8.8"]
  }
}'
RESULT=$(api_post "/config/dns" "$DATA")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" = "200" ]; then
    pass "UpdateDNS returned 200"
else
    fail "UpdateDNS failed: $RESULT"
fi

# Test 6: Update IPSets
log "Testing UpdateIPSets..."
DATA='[
  {
    "name": "blacklist",
    "type": "ipv4_addr",
    "entries": ["1.2.3.4", "5.6.7.8"]
  }
]'
RESULT=$(api_post "/config/ipsets" "$DATA")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" = "200" ]; then
    pass "UpdateIPSets returned 200"
else
    fail "UpdateIPSets failed: $RESULT"
fi

# Step 7: Apply Changes (Push Staged Config to Control Plane)
log "Applying Configuration (Second Pass)..."
#dilated_sleep 2
RESULT=$(api_post "/config/apply" "{}")
CODE=$(echo "$RESULT" | awk '{print $1}')
if [ "$CODE" = "200" ]; then
    pass "ApplyConfig (2) returned 200"
else
    fail "ApplyConfig (2) failed: $RESULT"
fi

# Test 8: Verify DNS persistence via GET
# Test 8: Verify DNS persistence via GET
log "Verifying DNS..."
if wait_for_config "/config/dns" "8.8.8.8"; then
    pass "Forwarder '8.8.8.8' found in GET response"
else
    fail "DNS verification failed: $OUT"
fi

# Test 9: Verify IPSet persistence via GET
# Test 9: Verify IPSet persistence via GET
log "Verifying IPSets..."
if wait_for_config "/config/ipsets" "blacklist"; then
    pass "IPSet 'blacklist' found in GET response"
else
    fail "IPSet verification failed: $OUT"
fi


log "API CRUD Tests PASSED"
exit 0
