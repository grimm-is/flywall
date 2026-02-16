#!/bin/sh
# Boot Loop Protection / Supervisor Integration Test
#
# This test verifies the supervisor's intelligent crash detection:
# 1. SIGKILL (OOM/forced kill) counts as a crash
# 2. SIGTERM (graceful stop) does NOT count as a crash
# 3. After 3 actual crashes, system enters Safe Mode
#
# NOTE: Since SIGKILL terminates the process immediately (no chance to record),
# this test simulates what a real service manager (systemd) would do by
# manually recording crash events to the supervisor state file.

TEST_TIMEOUT=45

set -e
set -x

# Setup test environment
source "$(dirname "$0")/../common.sh"

# This test requires root (for managing ctl/logs)
require_root
require_binary

# CRITICAL: Disable test mode for THIS specific test so supervisor actually runs
unset FLYWALL_TEST_MODE
export INVOCATION_ID=test  # Force supervisor to think it's running as a service

echo "Starting Boot Loop Protection / Supervisor test..."

# Ensure fresh state (no previous crash/supervisor state)
rm -f "$FLYWALL_STATE_DIR/crash.state"
rm -f "$FLYWALL_STATE_DIR/supervisor.state"

# Create a valid config (to prove we don't load it in Safe Mode)
CONFIG_FILE=$(mktemp_compatible config.hcl)
cat > "$CONFIG_FILE" <<EOF
interface "eth0" {
  zone = "lan"
  ipv4 = ["192.168.1.1/24"]
}
EOF

# Helper: Record a crash event to supervisor state (simulating systemd behavior)
# Args: $1 = signal number (9 for SIGKILL, 15 for SIGTERM)
record_exit() {
    local signal=$1
    local state_file="$FLYWALL_STATE_DIR/supervisor.state"
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    if [ ! -f "$state_file" ]; then
        # Create new state file
        echo "{\"events\":[{\"exit_code\":0,\"signal\":$signal,\"timestamp\":\"$timestamp\",\"was_panic\":false}]}" > "$state_file"
    else
        # Append to existing (using jq if available, else simple append)
        if command -v jq >/dev/null 2>&1; then
            jq --arg ts "$timestamp" --argjson sig "$signal" \
               '.events += [{"exit_code":0,"signal":$sig,"timestamp":$ts,"was_panic":false}]' \
               "$state_file" > "${state_file}.tmp" && mv "${state_file}.tmp" "$state_file"
        else
            # Fallback: reconstruct (assumes < 10 events for simplicity)
            local events=$(cat "$state_file" | tr -d '\n' | sed 's/]}$//')
            echo "${events},{\"exit_code\":0,\"signal\":$signal,\"timestamp\":\"$timestamp\",\"was_panic\":false}]}" > "$state_file"
        fi
    fi
}

# ============================================================
# Part 1: Test that SIGTERM (graceful stop) does NOT count as crash
# ============================================================
echo "=== Part 1: SIGTERM should NOT count as crash ==="

for i in 1 2 3 4; do
    echo "SIGTERM Stop $i..."
    rm -f "$CTL_SOCKET"
    start_ctl "$CONFIG_FILE"
    sleep 1  # Let it initialize

    # Graceful stop (SIGTERM)
    kill -TERM $CTL_PID
    wait $CTL_PID 2>/dev/null || true
    track_pid "" # untrack

    # Record the graceful exit (SIGTERM = 15)
    record_exit 15
done

# Start again - should NOT be in safe mode (SIGTERM doesn't count)
echo "Starting CTL after 4 SIGTERMs (should NOT be Safe Mode)..."
rm -f "$CTL_SOCKET"
start_ctl "$CONFIG_FILE"
sleep 2

# Check logs - should NOT contain Safe Mode
if grep -q "ENTERING SAFE MODE" "$CTL_LOG" 2>/dev/null; then
    echo "FAILURE: Safe Mode triggered by SIGTERM (should not count as crash)"
    cat "$CTL_LOG"
    exit 1
fi
pass "SIGTERM correctly NOT counted as crash"

# Clean stop and reset for next part
kill -TERM $CTL_PID 2>/dev/null || true
wait $CTL_PID 2>/dev/null || true
track_pid ""
rm -f "$FLYWALL_STATE_DIR/supervisor.state"

# ============================================================
# Part 2: Test that SIGKILL (forced kill) DOES count as crash
# ============================================================
echo "=== Part 2: SIGKILL should count as crash ==="

# Crash 1
echo "Simulating Crash 1 (SIGKILL)..."
rm -f "$CTL_SOCKET"
start_ctl "$CONFIG_FILE"
sleep 0.5
kill -9 $CTL_PID
wait $CTL_PID 2>/dev/null || true
track_pid ""
record_exit 9  # SIGKILL

# Crash 2
echo "Simulating Crash 2 (SIGKILL)..."
rm -f "$CTL_SOCKET"
start_ctl "$CONFIG_FILE"
sleep 0.5
kill -9 $CTL_PID
wait $CTL_PID 2>/dev/null || true
track_pid ""
record_exit 9

# Crash 3
echo "Simulating Crash 3 (SIGKILL)..."
rm -f "$CTL_SOCKET"
start_ctl "$CONFIG_FILE"
sleep 0.5
kill -9 $CTL_PID
wait $CTL_PID 2>/dev/null || true
track_pid ""
record_exit 9

# Next start should trigger Safe Mode (3 crashes >= threshold)
echo "Starting CTL (Expect Safe Mode)..."
rm -f "$CTL_SOCKET"
start_ctl "$CONFIG_FILE"

# Wait for Safe Mode log
if wait_for_log_entry "$CTL_LOG" "ENTERING SAFE MODE" 10; then
    pass "Safe Mode correctly triggered after 3 SIGKILL crashes"
else
    # Fallback check for system log
    if [ -f "$LOG_DIR/flywall.log" ] && grep -q "ENTERING SAFE MODE" "$LOG_DIR/flywall.log"; then
        pass "Safe Mode detected in system log file"
    else
        echo "FAILURE: Safe Mode NOT detected after 3 SIGKILL crashes"
        echo "Checking $CTL_LOG:"
        cat "$CTL_LOG"
        echo "Supervisor state:"
        cat "$FLYWALL_STATE_DIR/supervisor.state" 2>/dev/null || echo "(no state file)"
        exit 1
    fi
fi

echo "Boot Loop Protection / Supervisor verification passed!"
