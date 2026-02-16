#!/bin/sh

# API Sanity Test
# Verifies: API server startup and basic connectivity.

set -e
set -x
source "$(dirname "$0")/../common.sh"

TEST_TIMEOUT=30

require_root
require_binary
cleanup_on_exit

diag "Test: API Server Sanity"

# Create a temporary config for the control plane
CTL_CONFIG=$(mktemp_compatible "ctl_test.hcl")
cat > "$CTL_CONFIG" << 'EOF'
schema_version = "1.1"
api {
  enabled = false
  require_auth = false
}
EOF

# Start control plane using the helper from common.sh
export FLYWALL_SKIP_API=1
start_ctl "$CTL_CONFIG"
diag "Control plane started (PID $CTL_PID)"

# Start API server (using test-api to bypass sandbox)
export FLYWALL_NO_SANDBOX=1
start_api -listen :$TEST_API_PORT

# Test connectivity
diag "Testing connectivity to http://127.0.0.1:$TEST_API_PORT/api/status..."
# Retry loop for API availability (up to 30s)
for i in $(seq 1 30); do
    if curl -s --connect-timeout 2 http://127.0.0.1:$TEST_API_PORT/api/status > /dev/null; then
        pass "API server reachable"
        break
    fi
    if [ "$i" -eq 30 ]; then
        diag "API server unreachable after 30 attempts!"
        diag "--- API LOG ---"
        show_log_tail "$API_LOG" 10 | sed 's/^/# /'
        diag "--- END LOG ---"
        fail "API server unreachable"
    fi
    sleep 1
done


diag "API test completed"
