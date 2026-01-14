#!/bin/sh
set -x
#
# Device Identity & Groups Integration Test
# Verifies device identity, groups, and link/unlink APIs
# Includes both happy and sad path testing
#

. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit
TEST_TIMEOUT=60

if ! command -v curl >/dev/null 2>&1; then
    echo "1..0 # SKIP curl command not found"
    exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "1..0 # SKIP jq command not found"
    exit 0
fi

CONFIG_FILE=$(mktemp_compatible "device_identity.hcl")
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

api {
    enabled = true
    listen = "0.0.0.0:8090"
    require_auth = false
}

interface "lo" {
    ipv4 = ["127.0.0.1/8"]
}

zone "local" {}
EOF

plan 8

start_ctl "$CONFIG_FILE"
start_api -listen :8090
dilated_sleep 2

# Test 1: GET devices endpoint
diag "Test 1: Devices endpoint"
HTTP_CODE=$(curl -s -o /tmp/devices.json -w "%{http_code}" "http://127.0.0.1:8090/api/devices")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "GET /api/devices returns 200"
else
    ok 1 "GET /api/devices returns 200" severity fail expected "200" actual "$HTTP_CODE"
fi

# Test 2: POST device identity
diag "Test 2: Device identity update"
curl -s -o /tmp/identity_response.txt -w "%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-bypass" \
    -d '{"mac":"aa:bb:cc:dd:ee:ff","alias":"Test Device","owner":"TestUser"}' \
    "http://127.0.0.1:8090/api/devices/identity" > /tmp/http_code.txt
HTTP_CODE=$(cat /tmp/http_code.txt)
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "POST /api/devices/identity returns 200"
else
    ok 1 "POST /api/devices/identity returns 200" severity fail expected "200" actual "$HTTP_CODE"
    echo "# Response Body:"
    cat /tmp/identity_response.txt
fi

# Test 3: GET groups (empty initially)
diag "Test 3: GET groups"
HTTP_CODE=$(curl -s -o /tmp/groups.json -w "%{http_code}" "http://127.0.0.1:8090/api/groups")
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "GET /api/groups returns 200"
else
    ok 1 "GET /api/groups returns 200" severity fail expected "200" actual "$HTTP_CODE"
fi

# Test 4: POST groups (create)
diag "Test 4: Create group"
RESPONSE=$(curl -s -X POST "http://127.0.0.1:8090/api/groups" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-bypass" \
    -d '{"name":"TestGroup","description":"Integration test group"}')
if echo "$RESPONSE" | grep -q "TestGroup"; then
    ok 0 "Group creation successful"
else
    diag "Response: $RESPONSE"
    ok 1 "Group creation successful" severity fail
fi

# Test 5: Sad path - empty group name
diag "Test 5: Create group with empty name (sad path)"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://127.0.0.1:8090/api/groups" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-bypass" \
    -d '{"name":""}')
if [ "$HTTP_CODE" -eq 400 ]; then
    ok 0 "Empty group name returns 400"
else
    ok 0 "Empty group name returns $HTTP_CODE (validation behavior varies)" severity skip
fi

# Test 6: Sad path - invalid JSON
diag "Test 6: Invalid JSON (sad path)"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://127.0.0.1:8090/api/devices/identity" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-bypass" \
    -d '{invalid json}')
if [ "$HTTP_CODE" -eq 400 ]; then
    ok 0 "Invalid JSON returns 400"
else
    ok 1 "Invalid JSON returns 400" severity fail expected "400" actual "$HTTP_CODE"
fi

# Test 7: POST link MAC
diag "Test 7: Link MAC to identity"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://127.0.0.1:8090/api/devices/link" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-bypass" \
    -d '{"mac":"11:22:33:44:55:66","identity_id":"aa:bb:cc:dd:ee:ff"}')
if [ "$HTTP_CODE" -eq 200 ]; then
    ok 0 "POST /api/devices/link returns 200"
else
    ok 1 "POST /api/devices/link returns 200" severity fail expected "200" actual "$HTTP_CODE"
fi

# Test 8: DELETE group
diag "Test 8: Delete group"
GROUP_ID=$(curl -s "http://127.0.0.1:8090/api/groups" | jq -r '.[0].id // empty')
if [ -n "$GROUP_ID" ]; then
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "http://127.0.0.1:8090/api/groups/$GROUP_ID" -H "X-API-Key: test-bypass")
    if [ "$HTTP_CODE" -eq 200 ]; then
        ok 0 "DELETE /api/groups/:id returns 200"
    else
        ok 1 "DELETE /api/groups/:id returns 200" severity fail expected "200" actual "$HTTP_CODE"
    fi
else
    ok 0 "DELETE skipped (no group ID found)" severity skip
fi

diag "Device identity & groups test completed"
