#!/bin/sh

# VM Test Entrypoint - TAP Output Mode
# Outputs clean TAP format for host-side prove consumption
# All diagnostic output goes to stderr, TAP to stdout

# Brand mount path (matches flywall-builder)
MOUNT_PATH="/mnt/flywall"
BUILD_PATH="$MOUNT_PATH/build"



# Mount shared folder if not already mounted
if ! grep -q " $MOUNT_PATH " /proc/mounts; then
    # Mount project root as read-only
    mount -t 9p -o trans=virtio,version=9p2000.L,ro host_share "$MOUNT_PATH"
fi

# Mount build directory as writable (overlays the read-only mount)
if ! grep -q " $BUILD_PATH " /proc/mounts; then
    # We don't mkdir because it's inside a RO mount, but the directory
    # should already exist in the host_share.
    mount -t 9p -o trans=virtio,version=9p2000.L build_share "$BUILD_PATH"
fi

cd "$MOUNT_PATH"

# Set up Go environment
export GOPATH=/tmp/go
export GOCACHE=/tmp/go-cache
export HOME=/root
mkdir -p "$GOPATH" "$GOCACHE" 2>/dev/null

# Diagnostic info to stderr
echo "# VM booted: $(uname -r)" >&2
echo "# Alpine: $(cat /etc/alpine-release 2>/dev/null || echo 'unknown')" >&2

# OVERRIDE: Use local filesystem for Run/State dirs to avoid VirtFS locking issues
# We sync these back to the host share on exit for inspection.
export FLYWALL_RUN_DIR="/var/run/flywall"
export FLYWALL_STATE_DIR="/var/lib/flywall"
mkdir -p "$FLYWALL_RUN_DIR" "$FLYWALL_STATE_DIR"

cleanup_and_sync() {
    echo "# Syncing state/logs back to host..." >&2
    # Sync StateDir back to host mount (lazy sync)
    if [ -d "$FLYWALL_STATE_DIR" ]; then
        cp -r "$FLYWALL_STATE_DIR/"* "$MOUNT_PATH/var/lib/" 2>/dev/null || true
    fi
    poweroff
}
trap "cleanup_and_sync" EXIT

# Determine which test to run based on TEST_NAME env or arg
TEST_NAME="${1:-all}"

case "$TEST_NAME" in
    go)
        # Run Go tests with TAP output
        echo "# Running Go tests..." >&2
        "$MOUNT_PATH/tests/go_tap_test.sh"
        ;;
    nftables)
        "$MOUNT_PATH/tests/nftables_test.sh"
        ;;
    qos)
        "$MOUNT_PATH/tests/qos_test.sh"
        ;;
    protection)
        "$MOUNT_PATH/tests/protection_test.sh"
        ;;
    conntrack)
        "$MOUNT_PATH/tests/conntrack_helpers_test.sh"
        ;;
    all)
        # Run all tests sequentially, output combined TAP
        "$MOUNT_PATH/tests/tap_runner.sh"
        ;;
    *)
        echo "Unknown test: $TEST_NAME" >&2
        exit 1
        ;;
esac
