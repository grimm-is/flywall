#!/bin/bash
# integration_tests/linux/70-system/graceful_shutdown_test.sh
# Test graceful shutdown

# Standard TAP test preamble
. "$(dirname "$0")/../common.sh"

plan 4

# 1. Start the control plane
# 1. Start the control plane
CONFIG=$(minimal_config_with_api)
CONFIG_FILE=$(mktemp_compatible "graceful.hcl")
echo "$CONFIG" > "$CONFIG_FILE"

export FLYWALL_SKIP_API=0
start_ctl "$CONFIG_FILE"

# Wait for API to be ready
wait_for_api_ready "$TEST_API_PORT" 30 || fail "API failed to start"

# Get PID of the API server (it's a child process, so we need to find it)
# The ctl process spawns `_api-server`.
# We can find it via pgrep or ps
API_PID=$(pgrep -f "_api-server" | head -n 1)

if [ -z "$API_PID" ]; then
    fail "Could not find API server process"
else
    ok 0 "Found API server PID: $API_PID"
fi

# 2. Simulate a "slow" request or just ensure service is up
# We'll run a request in the background that takes some time,
# or just continuous requests.
# Since we can't easily make the server sleep, we'll blast it with Status requests
# and ensure they don't fail immediately upon SIGTERM.

(
    for i in {1..20}; do
        curl -s -o /dev/null http://127.0.0.1:$TEST_API_PORT/api/status
        # If we get connection refused/reset, that's bad (if it happens too early)
        # But eventually it will fail.
        sleep 0.1
    done
) &
BG_PID=$!

diag "Sending SIGTERM to API server (PID $API_PID)..."
kill -TERM "$API_PID"

# 3. Wait for process to exit
# It should exit within 5-10 seconds
wait_pid_term() {
    local pid=$1
    local timeout=$2
    local interval=0.5
    local elapsed=0
    # Use integer math for simplicity, or awk
    local max_checks=$(echo "$timeout / $interval" | awk '{print int($1)}')
    
    local i=0
    while kill -0 "$pid" 2>/dev/null; do
        sleep "$interval"
        i=$((i + 1))
        if [ $i -ge $max_checks ]; then
            return 1
        fi
    done
    return 0
}

if wait_pid_term "$API_PID" 10; then
    ok 0 "API server exited gracefully within timeout"
else
    fail "API server did not exit within timeout"
    kill -9 "$API_PID"
fi

# 4. Verify ctl is still running (or should it be?)
# We killed the API server child. The ctl parent might restart it (watchdog).
# Let's check if it restarts.
diag "Waiting for API server check/restart..."
sleep 2

NEW_API_PID=$(pgrep -f "_api-server" | head -n 1)
if [ -n "$NEW_API_PID" ] && [ "$NEW_API_PID" != "$API_PID" ]; then
    ok 0 "API server was restarted by watchdog"
else
    diag "New PID: $NEW_API_PID (Old: $API_PID)"
    # It might take longer to restart?
    # Or maybe it didn't restart if ctl was also killed?
    # We only killed the child.
    fail "API server was not restarted immediately"
fi

# 5. Clean shutdown of everything
stop_ctl
ok 0 "Full shutdown complete"
