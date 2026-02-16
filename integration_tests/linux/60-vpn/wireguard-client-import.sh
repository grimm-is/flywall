#!/bin/sh
set -x
#
# WireGuard Client Import Test
# Verifies the API endpoint for importing WireGuard configurations
# and ensures the custom routing fields (Table, PostUp) are preserved.
#

# Source common functions
. "$(dirname "$0")/../common.sh"

require_binary

# Cleanup
cleanup() {
    stop_ctl
    rm -f "$IMPORT_FILE" "$CONFIG_FILE"
}

trap cleanup EXIT INT TERM

# 1. Setup Config
CONFIG_FILE=$(mktemp_compatible config.hcl)
cat > "$CONFIG_FILE" <<EOF
api {
    enabled = true
    listen = "127.0.0.1:8080"
    require_auth = true
    key "admin-key" {
        key = "testtoken"
        permissions = ["vpn:write", "config:read", "config:write"]
    }
}
EOF

# 2. Start Flywall
plan 4

start_ctl "$CONFIG_FILE"
start_api
wait_for_api_ready 8080
dilated_sleep 2

# 3. Create a sample WireGuard config file
IMPORT_FILE=$(mktemp_compatible wg-client.conf)
cat > "$IMPORT_FILE" <<EOF
[Interface]
PrivateKey = aaaaaa
Address = 10.100.0.2/24
DNS = 1.1.1.1
MTU = 1350
Table = 200
PostUp = ip rule add from 10.100.0.2 table 200
PostDown = ip rule del from 10.100.0.2 table 200

[Peer]
PublicKey = bbbbbb
PresharedKey = cccccc
Endpoint = 203.0.113.1:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
EOF

# 4. Test Import Endpoint
diag "Testing POST /api/vpn/import..."

# Use curl to upload file
RESPONSE=$(curl -s -X POST "http://127.0.0.1:8080/api/vpn/import" \
    -H "X-API-Key: testtoken" \
    -F "file=@$IMPORT_FILE")

diag "Response: $RESPONSE"

if echo "$RESPONSE" | grep -q '"private_key":"******"'; then
    pass "Import successful (PrivateKey present and masked)"
else
    fail "Import failed (PrivateKey mismatch)"
fi

if echo "$RESPONSE" | grep -q '"table":"200"'; then
    pass "Table field imported correctly"
else
    fail "Table field mismatch"
fi

if echo "$RESPONSE" | grep -q '"post_up":\['; then
    pass "PostUp field imported correctly"
else
    fail "PostUp field mismatch"
fi

# 5. Persist the config
diag "Applying imported config..."

# Construct payload from response (wrapping in vpn.wireguard array)
# Note: jq would be ideal here but common.sh avoids external deps if possible.
# We'll construct a minimal payload using the imported values we verified.

PAYLOAD='{
    "wireguard": [{
        "name": "Imported Tunnel",
        "interface": "wg0",
        "private_key": "aaaaaa",
        "mtu": 1350,
        "table": "200",
        "enabled": true,
        "peers": [{
            "name": "Peer 1",
            "public_key": "bbbbbb",
            "allowed_ips": ["0.0.0.0/0"]
        }]
    }]
}'

apply_response=$(curl -s -X POST "http://127.0.0.1:8080/api/config/vpn" \
    -H "X-API-Key: testtoken" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD")

if echo "$apply_response" | grep -q '"success":true'; then
    pass "Config persisted successfully"
else
    diag "Apply response: $apply_response"
    fail "Failed to save config"
fi
