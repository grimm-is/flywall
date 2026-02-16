#!/bin/sh
set -x
# Password API Integration Test
# Verifies self-service password change and admin password reset

TEST_TIMEOUT=30

# Source common functions
. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

# --- Setup ---
plan 6

diag "Starting Password API Test"

# 1. Create config
TEST_CONFIG=$(mktemp_compatible "password_test.hcl")
cat > "$TEST_CONFIG" << 'EOF'
schema_version = "1.0"
api {
  enabled = true
  require_auth = true
}
EOF

# 2. Start control plane and API
start_ctl "$TEST_CONFIG"
if ! kill -0 $CTL_PID 2>/dev/null; then
    ok 1 "Control plane failed to start"
    exit 1
fi

rm -f "$STATE_DIR"/auth.json
start_api -listen :$TEST_API_PORT

if ! kill -0 $API_PID 2>/dev/null; then
    ok 1 "API server failed to start"
    stop_ctl
    exit 1
fi

wait_for_port $TEST_API_PORT 10

COOKIES=$(mktemp_compatible "cookies.txt")

# 3. Create Admin User (this usually logs us in too, via cookie?)
# But let's login explicitly to be sure and save cookie.
diag "Creating admin user..."
curl -s -X POST http://127.0.0.1:$TEST_API_PORT/api/setup/create-admin \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"InitialPassword123!"}' > /dev/null

# Login to get cookie and CSRF token
# We use -c to save cookies (jar)
LOGIN_RESP=$(curl -s -c "$COOKIES" -X POST http://127.0.0.1:$TEST_API_PORT/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"InitialPassword123!"}')

if grep -q "session" "$COOKIES"; then
    ok 0 "Admin login successful (cookie set)"
else
    ok 1 "Admin login failed (no session cookie). Resp: $LOGIN_RESP"
    exit 1
fi

# Extract CSRF token
CSRF_TOKEN=$(echo "$LOGIN_RESP" | grep -o '"csrf_token":"[^"]*"' | cut -d'"' -f4)
if [ -z "$CSRF_TOKEN" ]; then
    ok 1 "Failed to extract CSRF token from login response"
    exit 1
fi

# 4. Test Self-Service Password Change
diag "Testing self-service password change..."
# Use -b to send cookies AND X-CSRF-Token header
CHANGE_RESP=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIES" -X PUT http://127.0.0.1:$TEST_API_PORT/api/auth/password \
    -H "Content-Type: application/json" \
    -H "X-CSRF-Token: $CSRF_TOKEN" \
    -d '{"current_password":"InitialPassword123!","new_password":"NewSelfServicePass123!"}')

if [ "$CHANGE_RESP" = "200" ]; then
    ok 0 "Password change API returned 200"
else
    ok 1 "Password change failed: $CHANGE_RESP"
    exit 1
fi

# Verify old password fails
# We clear cookies to force new login
rm -f "$COOKIES"
FAIL_RESP=$(curl -s -o /dev/null -w "%{http_code}" -c "$COOKIES" -X POST http://127.0.0.1:$TEST_API_PORT/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"InitialPassword123!"}')

if [ "$FAIL_RESP" = "401" ]; then
    ok 0 "Old password rejected"
else
    ok 1 "Old password still accepted (Code: $FAIL_RESP)"
fi

# Verify new password works
LOGIN_RESP_2=$(curl -s -c "$COOKIES" -X POST http://127.0.0.1:$TEST_API_PORT/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"NewSelfServicePass123!"}')

if grep -q "session" "$COOKIES"; then
    ok 0 "Login with new password successful"
else
    ok 1 "Login with new password failed"
fi

CSRF_TOKEN_2=$(echo "$LOGIN_RESP_2" | grep -o '"csrf_token":"[^"]*"' | cut -d'"' -f4)

# 5. Test Admin Reset Password
diag "Testing admin password reset..."
# Admin (using new cookie) resets own password
RESET_RESP=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIES" -X PUT http://127.0.0.1:$TEST_API_PORT/api/users/admin/password \
    -H "Content-Type: application/json" \
    -H "X-CSRF-Token: $CSRF_TOKEN_2" \
    -d '{"new_password":"SecretResetPass123!"}')

if [ "$RESET_RESP" = "200" ]; then
    ok 0 "Admin reset API returned 200"
else
    ok 1 "Admin reset failed: $RESET_RESP"
fi

# Verify reset password works
rm -f "$COOKIES"
LOGIN_RESP_3=$(curl -s -c "$COOKIES" -X POST http://127.0.0.1:$TEST_API_PORT/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"SecretResetPass123!"}')

if grep -q "session" "$COOKIES"; then
    ok 0 "Login with reset password successful"
else
    ok 1 "Login with reset password failed"
fi

rm -f "$COOKIES"
exit 0
