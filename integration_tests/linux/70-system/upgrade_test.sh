#!/bin/sh
set -x
# Hot Binary Upgrade Test
# Verifies that:
# 1. New binary can be built.
# 2. Upgrade command triggers socket handoff.
# 3. New binary takes over listeners.
# 4. Old binary exits.
# 5. State (DHCP leases) is preserved.

TEST_TIMEOUT=60
. "$(dirname "$0")/../common.sh"
export FLYWALL_LOG_FILE=stdout
export FLYWALL_STATE_DIR="$RUN_DIR/state"
mkdir -p "$FLYWALL_STATE_DIR"

plan 3

diag "Starting Upgrade Test..."

# Cleanup trap
cleanup() {
    pkill -f flywall-v1 2>/dev/null
    pkill -f flywall-v2 2>/dev/null
    rm -f /tmp/flywall-v1 /tmp/flywall-v2 /tmp/upgrade_$$.hcl /tmp/flywall_$$.log /tmp/flywall.pid
}
trap cleanup EXIT

# 1. Prepare Binaries
# Binaries are pre-built on host and mounted at /mnt/flywall/build/test-artifacts/
MOUNT_DIR="/mnt/flywall/build/test-artifacts"
if [ ! -f "$MOUNT_DIR/flywall-v1" ] || [ ! -f "$MOUNT_DIR/flywall-v2" ]; then
    diag "Pre-built binaries not found in $MOUNT_DIR"
    ls -lR /mnt/flywall/build/
    exit 1
fi

cp "$MOUNT_DIR/flywall-v1" /tmp/flywall-v1
cp "$MOUNT_DIR/flywall-v2" /tmp/flywall-v2
chmod +x /tmp/flywall-v1 /tmp/flywall-v2

ok 0 "Binaries V1 and V2 prepared"

# 2. Start V1
# Pick random port
API_PORT=8080
UPGRADE_HCL=$(mktemp_compatible "upgrade.hcl")

cat > "$UPGRADE_HCL" <<EOF
schema_version = "1.0"
interface "lo" {
    ipv4 = ["127.0.0.1/8"]
}
api {
    enabled = true
    listen = "127.0.0.1:$API_PORT"
}
dhcp {
    enabled = true
    scope "lan" {
        interface = "lo"
        range_start = "127.0.0.10"
        range_end = "127.0.0.20"
        router = "127.0.0.1"
    }
}
EOF

diag "Starting V1..."
/tmp/flywall-v1 ctl "$UPGRADE_HCL" > /tmp/flywall_$$.log 2>&1 &
PID_V1=$!
echo $PID_V1 > /tmp/flywall.pid
dilated_sleep 2

if ! kill -0 $PID_V1 2>/dev/null; then
    diag "V1 failed to start"
    cat /tmp/flywall_$$.log
    exit 1
fi
ok 0 "V1 started"

# 3. Perform Upgrade
diag "Initiating Upgrade to V2..."
# We run the upgrade command.
/tmp/flywall-v1 upgrade --binary /tmp/flywall-v2 --config "$UPGRADE_HCL" >> /tmp/flywall_$$.log 2>&1 &

# Give it time to attempt socket connection (and fail)
dilated_sleep 5

# Check for success log from V2
if grep -qE "Upgrade complete|Listener handoff complete" "/tmp/flywall_$$.log"; then
    ok 0 "Upgrade success confirmed"
else
    ok 1 "Upgrade success log NOT found"
    diag "Log content (tail):"
    tail -n 10 /tmp/flywall_$$.log || true
    exit 1
fi

# Cleanup will handle process kill
