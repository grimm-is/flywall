#!/bin/sh
set -x

# Flexible Upgrade Path Test
# Verifies that upgrades work correctly when running from a non-standard location.

TEST_TIMEOUT=60
. "$(dirname "$0")/../common.sh"
export FLYWALL_LOG_FILE=stdout
export FLYWALL_STATE_DIR="$RUN_DIR/state"
mkdir -p "$FLYWALL_STATE_DIR"

require_root
require_binary

cleanup_custom() {
    if [ -n "$CUSTOM_INSTALL_DIR" ]; then
        rm -rf "$CUSTOM_INSTALL_DIR"
    fi
}
trap 'cleanup_custom' EXIT

log() { echo "[TEST] $1"; }

# 1. Setup Custom Install Directory
log "Setting up custom install directory..."
CUSTOM_INSTALL_DIR=$(mktemp -d)
# Force name to "flywall" because InPlaceStrategy uses brand.BinaryName (hardcoded)
# regardless of what we start it as.
CUSTOM_BIN_PATH="$CUSTOM_INSTALL_DIR/flywall"

# Copy binary to custom location
cp "$APP_BIN" "$CUSTOM_BIN_PATH"
chmod 755 "$CUSTOM_BIN_PATH"

log "Binary installed to: $CUSTOM_BIN_PATH"

# Override APP_BIN for start_ctl
# This tells common.sh to launch OUR binary
export APP_BIN="$CUSTOM_BIN_PATH"

# 2. Config
CONFIG_FILE=$(mktemp_compatible "upgrade_custom.hcl")
# Pick random port
API_PORT=8080

cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"
api {
  enabled = true
  listen  = "127.0.0.1:$API_PORT"
  require_auth = false
}
system {
  timezone = "UTC"
}
EOF

# 3. Start Control Plane from custom location
log "Starting daemon from custom location..."
start_ctl "$CONFIG_FILE"

ORIG_PID=$CTL_PID
log "Original PID: $ORIG_PID"

# Verify execution path
if [ -d /proc ]; then
    EXE_LINK=$(readlink /proc/$ORIG_PID/exe)
    if [ "$EXE_LINK" != "$CUSTOM_BIN_PATH" ]; then
        fail "Daemon running from wrong location: $EXE_LINK (expected $CUSTOM_BIN_PATH)"
    fi
fi

# 4. Trigger Self-Upgrade
log "Triggering upgrade --self..."
# Use the same binary to trigger the client command
if ! "$APP_BIN" upgrade --self --config "$CONFIG_FILE"; then
    fail "Upgrade command failed"
fi

# 5. Wait for Old Process to Exit
log "Waiting for old PID $ORIG_PID to exit..."
wait_for_condition "! kill -0 $ORIG_PID 2>/dev/null" 20

# 6. Find New Process
# Process name should still be "flywall", but PID changed
log "Searching for new daemon process..."
sleep 2 # Give it a moment to stabilize

# Find new PID that is NOT the old one
# Filter for binary name
NEW_PID=$(pgrep -f "$CUSTOM_BIN_PATH" | grep -v "^$ORIG_PID$" | head -n 1)

if [ -z "$NEW_PID" ]; then
    # Maybe pgrep -f matches full command line?
    # Try finding by exact name if linux
    NEW_PID=$(pgrep -x "$BINARY_NAME" | grep -v "^$ORIG_PID$" | head -n 1)
fi

if [ -z "$NEW_PID" ]; then
    fail "New daemon process not found"
else
    log "New PID found: $NEW_PID"
fi

# 7. Verify New Process State
if [ -d /proc ]; then
    EXE_LINK=$(readlink /proc/$NEW_PID/exe)
    if [ "$EXE_LINK" != "$CUSTOM_BIN_PATH" ]; then
        fail "New daemon running from wrong location: $EXE_LINK (expected $CUSTOM_BIN_PATH)"
    fi
    log "Verified new daemon path: $EXE_LINK"
fi

# 8. Verify Cleanup
# The _new binary should be gone (renamed to current)
if [ -f "${CUSTOM_BIN_PATH}_new" ]; then
    fail "Temporary binary ${CUSTOM_BIN_PATH}_new still exists! (Rename failed?)"
fi

pass "Upgrade path verification successful"
exit 0
