#!/bin/sh
set -x

# VPN Sensitive Data Masking Test
# Verifies that private keys and preshared keys are masked in API responses

TEST_TIMEOUT=30
. "$(dirname "$0")/../common.sh"

require_root
require_binary

cleanup_with_logs() {
    if [ -f "$API_LOG" ]; then
        diag "API Log Dump:"
        cat "$API_LOG" | sed 's/^/# /'
    fi
    cleanup_processes
}
trap cleanup_with_logs EXIT INT TERM

log() { echo "[TEST] $1"; }

CONFIG_FILE="/tmp/vpn_masking_$$.hcl"

# Configuration with Sensitive Data
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.1"

api {
  enabled = true
  listen  = "127.0.0.1:8085"
  require_auth = false
}

vpn {
  wireguard "wg0" {
    enabled = true
    interface = "wg0"
    private_key = "PRIVATE_KEY_SHOULD_BE_HIDDEN"
    listen_port = 51820

    peer "peer1" {
      public_key = "PUBLIC_KEY_VISIBLE"
      preshared_key = "PRESHARED_KEY_SHOULD_BE_HIDDEN"
      allowed_ips = ["10.100.0.2/32"]
    }
  }
}

zone "vpn" {}
interface "wg0" {
    ipv4 = ["10.100.0.1/24"]
    zone = "vpn"
}
EOF

# Start Control Plane
start_ctl "$CONFIG_FILE"

# Start API Server
export FLYWALL_NO_SANDBOX=1
start_api -listen :8085

API_URL="http://127.0.0.1:8085/api"

# 1. Fetch VPN Config
log "Fetching VPN Config..."
if command -v curl >/dev/null 2>&1; then
    RESULT=$(curl -s "$API_URL/config/vpn")
else
    fail "curl required"
fi

# 2. Verify Private Key Masking
if echo "$RESULT" | grep -q "PRIVATE_KEY_SHOULD_BE_HIDDEN"; then
    fail "Security Leak: Private key leaked in API response!"
elif echo "$RESULT" | grep -q "(hidden)"; then
    pass "Private Key is masked"
else
    # It might be empty if zero value, but we set it.
    # Or maybe SecureString json marshaling behaves differently?
    # Our implementation: MarshalJSON returns quotes "*******"
    fail "Private Key not found or not masked as expected. Got: $RESULT"
fi

# 3. Verify Preshared Key Masking
if echo "$RESULT" | grep -q "PRESHARED_KEY_SHOULD_BE_HIDDEN"; then
    fail "Security Leak: Preshared key leaked in API response!"
elif echo "$RESULT" | grep -q "(hidden)"; then
    pass "Preshared Key is masked"
else
     fail "Preshared Key not found or not masked as expected."
fi

# 4. Verify Public Key Visibility (Should remain visible)
if echo "$RESULT" | grep -q "PUBLIC_KEY_VISIBLE"; then
    pass "Public Key is visible (correct)"
else
    fail "Public Key missing from response"
fi

exit 0
