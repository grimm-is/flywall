#!/bin/sh
# Scripts/test/reload_test.sh
# Verifies 'flywall reload' functionality

TEST_TIMEOUT=60
set -e
set -x

# Path to flywall binary
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
. "$SCRIPT_DIR/../common.sh"
FLYWALL="$APP_BIN"
PID_FILE="$RUN_DIR/flywall.pid"
# Log file will be set by start_ctl
LOG_FILE=""

echo "TAP version 13"
echo "1..4"

# Clean previous
rm -f $PID_FILE $LOG_FILE

# 1. Start daemon
CONFIG_FILE=$(mktemp_compatible "reload_config.hcl")
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"
ip_forwarding = false
interface "lo" {
  description = "Loopback"
  ipv4 = ["127.0.0.1/8"]
  zone = "management"
}
EOF

echo "# Starting daemon..."
export FLYWALL_LOG_FILE="$LOG_FILE"
export FLYWALL_LOG_LEVEL="debug"
export FLYWALL_SKIP_API=1
start_ctl "$CONFIG_FILE"

if [ -n "$CTL_PID" ]; then
    echo "ok 1 - Daemon started"
else
    echo "not ok 1 - Daemon failed to start"
    exit 1
fi

# 2. Modify config
echo "# Modifying config..."
# Modify description to see if it changes (would verify if we had a way to query running config easily via CLI json)
# But we can check logs for "Reloading configuration"
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"
# Changed
ip_forwarding = true
interface "lo" {
  description = "Loopback Reloaded"
  ipv4 = ["127.0.0.1/8"]
  zone = "management"
}
EOF

# 3. Reload
echo "# Running reload command..."
# Wait a moment to ensure daemon is fully ready
sleep 2
# Must specify config file because reload defaults to /opt/flywall/etc/flywall.hcl
# but we started daemon with a custom config
if $FLYWALL reload "$CONFIG_FILE"; then
    echo "ok 2 - Reload command succeeded"
else
    echo "not ok 2 - Reload command failed"
    exit 1
fi

# 4. Check logs with retry (use CTL_LOG set by start_ctl)
ACTUAL_LOG="$CTL_LOG"

diag "checking logs at $ACTUAL_LOG"
reload_logged=""
for retry in 1 2 3 4 5 6 7 8 9 10; do
    if [ -f "$ACTUAL_LOG" ] && grep -q "Received SIGHUP, reloading configuration" "$ACTUAL_LOG"; then
        reload_logged="yes"
        diag "Found reload message on attempt $retry"
        break
    fi
    diag "Retry $retry: waiting for reload message..."
    sleep 1
done

if [ "$reload_logged" = "yes" ]; then
    echo "ok 3 - Daemon received SIGHUP and reloaded"
else
    echo "not ok 3 - Daemon did not log reload in $ACTUAL_LOG"
    if [ -f "$ACTUAL_LOG" ]; then
        tail -n 20 "$ACTUAL_LOG" | sed 's/^/# /'
    else
        echo "# Log file not found"
    fi
fi

# 5. Stop
$FLYWALL stop
echo "ok 4 - Stopped"
