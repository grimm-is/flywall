#!/bin/sh

# SAFETY CHECK: Fail hard if not running in the expected VM environment
# We check for the specific mount point or the environment variable set by the runner
if [ ! -d "/mnt/flywall" ] && [ -z "${FLYWALL_TEST_VM:-}" ]; then
    echo "FATAL: Integration tests must be run inside the QEMU VM environment."
    echo "       Refusing to run on host system to prevent data loss."
    exit 1
fi
# Common functions and environment setup for Flywall Firewall tests
# Handles dynamic branding resolution and common utilities
#
# TIMEOUT POLICY: Tests should complete in <15 seconds
# If your test needs more time, set at the top of your test:
#   TEST_TIMEOUT=30  # seconds
#
# TIME_DILATION: The agent exports TIME_DILATION (e.g., "1.00" or "2.50")
# based on CPU performance. Use scale_timeout to scale durations accordingly.
#
# CLEANUP: Always use cleanup_on_exit or manually kill background processes

# Scale a timeout value by TIME_DILATION factor (returns integer)
# Usage: scaled=$(scale_timeout 30)
# If TIME_DILATION is unset or invalid, defaults to 1 (no scaling)
# Validate TIME_DILATION once at startup
case "${TIME_DILATION:-}" in
    ''|*[!0-9.]*) TIME_DILATION=1 ;;
esac

# Workaround for GetRunningConfig RPC crash in test environment
# This makes GetRunningConfig return the staged config instead
export FLYWALL_USE_STAGED_AS_RUNNING=1

# Conditional Shell Tracing
if [ -n "${TEST_DEBUG:-}" ]; then
    set -x
    # We use stderr for debug messages to avoid polluting TAP output
    echo "# Debug Tracing Enabled (TEST_DEBUG=$TEST_DEBUG)" >&2
else
    set +x
fi

# Scale a timeout value (seconds) by TIME_DILATION
# Usage: local timeout=$(scale_timeout 5)
# Scale a timeout value (seconds) by TIME_DILATION
# Usage: local timeout=$(scale_timeout 5)
scale_timeout() {
    local base="$1"
    # Use awk for floating point math
    echo "$base $TIME_DILATION" |
        awk '{ r = $1 * $2; print r }'
}

# Sleep for a dilated duration (supports floats)
# Usage: dilated_sleep 0.5
dilated_sleep() {
    local base="$1"
    sleep $(scale_timeout "$base")
}

# Dynamic Branding Resolution
# We need to find the project root and read brand.json
# In the VM, the project is mounted at /mnt/<brand_lower> or /mnt/flywall usually.
# But we can find it relative to this script.

# Determine script directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Project root detection (git-based with fallback)
if git rev-parse --show-toplevel >/dev/null 2>&1; then
    PROJECT_ROOT="$(git rev-parse --show-toplevel)"
else
    # Fallback: traverse up until we find .git or reach root
    _dir="$SCRIPT_DIR"
    while [ "$_dir" != "/" ]; do
        if [ -d "$_dir/.git" ]; then
            PROJECT_ROOT="$_dir"
            break
        fi
        _dir="$(dirname "$_dir")"
    done
    # Final fallback if no .git found (e.g. inside VM mount)
    PROJECT_ROOT="${PROJECT_ROOT:-$(dirname "$(dirname "$SCRIPT_DIR")")}"
fi

BRAND_ENV="$PROJECT_ROOT/internal/brand/brand.env"

if [ -f "$BRAND_ENV" ]; then
    # Source the generated brand.env file (much cleaner than parsing JSON)
    . "$BRAND_ENV"

    # Map BRAND_* vars to our expected names
    BRAND_NAME="${BRAND_NAME:-Flywall}"
    BRAND_LOWER="${BRAND_LOWER_NAME:-flywall}"
    BINARY_NAME="${BRAND_BINARY_NAME:-flywall}"
    STATE_DIR="${BRAND_DEFAULT_STATE_DIR:-/opt/flywall/var/lib}"
    LOG_DIR="${BRAND_DEFAULT_LOG_DIR:-/opt/flywall/var/log}"
    RUN_DIR="${BRAND_DEFAULT_RUN_DIR:-/var/run/flywall}"
    SOCKET_NAME="${BRAND_SOCKET_NAME:-ctl.sock}"
else
    # Fallback defaults if brand.env doesn't exist (run 'make brand-env' to generate)
    BRAND_NAME="Flywall"
    BRAND_LOWER="flywall"
    BINARY_NAME="flywall"
    STATE_DIR="/opt/flywall/var/lib"
    LOG_DIR="/opt/flywall/var/log"
    RUN_DIR="/var/run/flywall"
    SOCKET_NAME="ctl.sock"
fi

# ============================================================================
# Mount Hierarchy for VM Test Isolation
# ============================================================================
# /mnt/flywall  - Read-only source tree (host_share)
# /mnt/build    - Read-only compiled binaries (build_share)
# /mnt/assets   - Shared writable with POSIX locking (assets_share)
# /mnt/worker   - Per-worker isolated scratch (worker_share)
#
# IMPORTANT: Unix domain sockets cannot be created on 9p/virtfs mounts.
# We use /mnt/worker for state/logs but keep sockets on local /tmp.
# Tests run SEQUENTIALLY within each VM, so we use fixed paths here.
# Inter-VM isolation is provided by the per-worker mount.
# ============================================================================

WORKER_MOUNT="/mnt/worker"
ASSETS_MOUNT="/mnt/assets"
BUILD_MOUNT="/mnt/build"

# Use PID-based unique directory for each test execution
# This prevents collisions and ensures a clean slate
# We append a random suffix to ensure uniqueness across VMs where PIDs might overlap
export TEST_PID="$$-$(head -c 2 /dev/urandom | od -An -tx1 | tr -d ' \n')"

# Better unique ID using ORCA test ID + PID
# This provides traceability and guaranteed uniqueness across VMs
if [ -n "${ORCA_TEST_ID:-}" ]; then
    # ORCA_TEST_ID is unique per test run (e.g., "20260203-abc123")
    # Combine with VM PID for uniqueness across VMs
    export TEST_UID="${ORCA_TEST_ID}-$$"
else
    # Fallback for manual runs
    export TEST_UID="manual-$(date +%Y%m%d-%H%M%S)-$$"
fi

# Unix sockets MUST be on local filesystem (not 9p)
# We use a unique subdirectory in /run (tmpfs) to ensure isolation and socket support.
# We avoid /var/run symlinks which can be problematic in some minimal environments.
mkdir -p /run
RUN_DIR="/run/flywall-${TEST_PID}"

# Per-worker isolated storage (unique per VM worker, created by orca)
if [ -d "$WORKER_MOUNT" ]; then
    # Use /mnt/worker for persistent state that needs to be visible on host
    # Test Artifact Storage
    # We use a unique directory per test run to store all artifacts (state, logs, etc.)
    # This allows post-run analysis.

    # Resolve Test ID from Orchestrator (Batch/Run ID) or fallback
    if [ -n "${ORCA_TEST_ID:-}" ]; then
        BATCH_ID="${ORCA_TEST_ID}"
    elif [ -n "${TEST_RUN_ID:-}" ]; then
        BATCH_ID="${TEST_RUN_ID}"
    else
        # Fallback to timestamp-pid if no orchestrator ID
        BATCH_ID="manual-$(date +%Y%m%d-%H%M%S)-${TEST_PID}"
    fi
    export BATCH_ID

    # We use bind mounts to map the persistent artifact storage to the expected system locations

    # Artifact Root: <Mount>/<BatchID>/<TestRelPath>
    # We resolve the test path relative to the project root.
    # $0 refers to the test script when sourced.
    ABS_SCRIPT=$(cd "$(dirname "$0")" && pwd)/$(basename "$0")
    TEST_REL_PATH="${ABS_SCRIPT#$PROJECT_ROOT/}"

    ARTIFACT_DIR="$WORKER_MOUNT/$BATCH_ID/$TEST_REL_PATH"
    STATE_DIR="$ARTIFACT_DIR/state"
    LOG_DIR="$ARTIFACT_DIR/log"
    mkdir -p "$STATE_DIR" "$LOG_DIR"
    chmod 777 "$STATE_DIR" "$LOG_DIR"
    export ARTIFACT_DIR STATE_DIR LOG_DIR

    # SETUP MOUNTS REMOVED
    # We no longer bind mount to /opt/flywall/var/lib or /var/log globally.
    # We rely on CLI flags (-state-dir, -log-dir) to direct the binary to the isolated paths.

    # Ensure runtime directories exist with 777 permissions
    # This ensures the API (user 'nobody') can write to them regardless of 9p mapping quirks
    mkdir -p "$STATE_DIR/imports" "$STATE_DIR/api"
    chmod 777 "$STATE_DIR/imports" "$STATE_DIR/api"

    # IMPORTANT: Must be 777 because API server runs as 'nobody'
    chmod 777 "$STATE_DIR" "$LOG_DIR"
    # Recursively fix permissions on any remaining content
    chmod -R 777 "$STATE_DIR" 2>/dev/null || true

    # Ensure RUN_DIR exists and is writable (must be on local disk for sockets)
    mkdir -p "$RUN_DIR"
    chmod 1777 "$RUN_DIR"

    # Ensure /tmp is available and writable by all for temporary files
    mkdir -p /tmp
    chmod 1777 /tmp

    # Ensure Go uses /tmp (or the explicit RUN_DIR if specified in test config)
    export TMPDIR="/tmp"

    # Legacy path setup handled by mount --bind above
else
    # Fallback for non-VM execution or legacy mode
    RUN_DIR="/tmp/flywall-test-${TEST_PID}-run"
    STATE_DIR="/tmp/flywall-test-${TEST_PID}-state"
    LOG_DIR="/tmp/flywall-test-${TEST_PID}-log"
    mkdir -p "$RUN_DIR" "$STATE_DIR" "$LOG_DIR"
    chmod 777 "$RUN_DIR" "$STATE_DIR" "$LOG_DIR"
fi

# Export these as FLYWALL_* for the Go binary to pick up
export FLYWALL_RUN_DIR="$RUN_DIR"
export FLYWALL_STATE_DIR="$STATE_DIR"
export FLYWALL_LOG_DIR="$LOG_DIR"
export FLYWALL_TEST_MODE=1  # Skip crash loop detection in test environment

# Shared writable assets (ipsets, geoip, caches)
# FORCE ISOLATION: Always use STATE_DIR for tests to prevent shared writable conflicts
# if [ -d "$ASSETS_MOUNT" ]; then
#     export FLYWALL_SHARE_DIR="$ASSETS_MOUNT"
# else
    export FLYWALL_SHARE_DIR="$STATE_DIR"
# fi

SOCKET_NAME="ctl.sock"
CTL_SOCKET="$RUN_DIR/$SOCKET_NAME"
export FLYWALL_CTL_SOCKET="$CTL_SOCKET"

unset INVOCATION_ID  # Ensure no service-mode contamination

# Ensure directories exist with proper permissions
if [ "$(id -u)" -eq 0 ]; then
    mkdir -p "$FLYWALL_RUN_DIR" "$FLYWALL_STATE_DIR"
    rm -f "$FLYWALL_STATE_DIR/supervisor.state" "$FLYWALL_STATE_DIR/crash.state"
fi

# Set common variables
MOUNT_PATH="/mnt/$BRAND_LOWER"
RAW_ARCH=$(uname -m)

# Normalize ARCH to match Go/Build naming (arm64, amd64)
case "$RAW_ARCH" in
    aarch64) ARCH="arm64" ;;
    x86_64)  ARCH="amd64" ;;
    *)       ARCH="$RAW_ARCH" ;;
esac

# Prefer binaries from /mnt/build (read-only, dedicated mount)
if [ -d "$BUILD_MOUNT" ] && [ -x "$BUILD_MOUNT/${BINARY_NAME}-linux-${ARCH}" ]; then
    APP_BIN="$BUILD_MOUNT/${BINARY_NAME}-linux-${ARCH}"
elif [ -x "$MOUNT_PATH/build/${BINARY_NAME}-linux-${ARCH}" ]; then
    APP_BIN="$MOUNT_PATH/build/${BINARY_NAME}-linux-${ARCH}"
elif [ -x "$PROJECT_ROOT/build/${BINARY_NAME}-linux-${ARCH}" ]; then
    APP_BIN="$PROJECT_ROOT/build/${BINARY_NAME}-linux-${ARCH}"
else
    # Fallback to standard name
    APP_BIN="$BUILD_MOUNT/$BINARY_NAME"
    if [ ! -x "$APP_BIN" ]; then
        APP_BIN="$MOUNT_PATH/build/$BINARY_NAME"
    fi
    if [ ! -x "$APP_BIN" ]; then
        APP_BIN="$PROJECT_ROOT/build/$BINARY_NAME"
    fi
fi

# ============================================================================
# Minimal Utility Wrappers (Fallback to toolbox)
# ============================================================================
# Prefer toolbox from /mnt/build
if [ -d "$BUILD_MOUNT" ] && [ -x "$BUILD_MOUNT/toolbox-linux-${ARCH}" ]; then
    TOOLBOX_BIN="$BUILD_MOUNT/toolbox-linux-${ARCH}"
elif [ -x "$MOUNT_PATH/build/toolbox-linux-${ARCH}" ]; then
    TOOLBOX_BIN="$MOUNT_PATH/build/toolbox-linux-${ARCH}"
elif [ -x "$PROJECT_ROOT/build/toolbox-linux-${ARCH}" ]; then
    TOOLBOX_BIN="$PROJECT_ROOT/build/toolbox-linux-${ARCH}"
else
    # Fallback to standard name (for local testing)
    TOOLBOX_BIN="$BUILD_MOUNT/toolbox"
    if [ ! -x "$TOOLBOX_BIN" ]; then
        TOOLBOX_BIN="$MOUNT_PATH/build/toolbox"
    fi
    if [ ! -x "$TOOLBOX_BIN" ]; then
        TOOLBOX_BIN="$PROJECT_ROOT/build/toolbox"
    fi
fi

link_toolbox() {
    # If binary missing, use toolbox
    if ! command -v "$1" >/dev/null 2>&1; then
        eval "$1() { \"$TOOLBOX_BIN\" \"$1\" \"\$@\"; }"
    fi
}

link_toolbox dig
link_toolbox nc
link_toolbox jq
link_toolbox curl

# Pre-determine the best port checker? No, try all for robustness.
# Some environments might fail nc (IPv4 vs IPv6) but succeed with netstat.
# check_port [port] [proto]
# proto: tcp (default) or udp
check_port() {
    local port="$1"
    local proto="${2:-tcp}"

    if [ "$proto" = "udp" ]; then
        # Try nc (UDP)
        if command -v nc >/dev/null 2>&1; then
            if nc -z -u -w 1 127.0.0.1 "$port" 2>/dev/null; then return 0; fi
        fi
        # Fallback to netstat/ss (UDP)
        if command -v netstat >/dev/null 2>&1; then
            if netstat -uln 2>/dev/null | grep -q ":$port "; then return 0; fi
        fi
        if command -v ss >/dev/null 2>&1; then
            if ss -uln 2>/dev/null | grep -q ":$port "; then return 0; fi
        fi
    else
        # TCP (default)
        if command -v nc >/dev/null 2>&1; then
            if nc -z -w 1 127.0.0.1 "$port" 2>/dev/null; then return 0; fi
        fi
        if command -v netstat >/dev/null 2>&1; then
            if netstat -tln 2>/dev/null | grep -q ":$port "; then return 0; fi
        fi
        if command -v ss >/dev/null 2>&1; then
            if ss -tln 2>/dev/null | grep -q ":$port "; then return 0; fi
        fi
    fi
    return 1
}


# ============================================================================
# Test Configuration Defaults
# ============================================================================

# ============================================================================
# Default API port for tests (Standardized to 8080) Tests are only ever run in 
# virtual machines and will NEVER have port conflicts, so we can just use the
# same port for all tests. RANDOMIZING PORTS IS NEVER THE ANSWER TO A PERCEIVED
# "PORT CONFLICT"
TEST_API_PORT="${TEST_API_PORT:-8080}"
export TEST_API_PORT
# ============================================================================

# Standard test environment setup
# Usage: setup_test_env "test_name_suffix"
setup_test_env() {
    local name="$1"
    require_root
    require_binary

    # Define common variables
    CONFIG_FILE=$(mktemp_compatible "${name}.hcl")
    export CONFIG_FILE

    # Register standard cleanup
    trap "cleanup_on_exit; rm -f \"$CONFIG_FILE\" 2>/dev/null" EXIT INT TERM

    diag "Test environment setup for: $name"
}

# ============================================================================
# Network Setup Helpers
# ============================================================================

# Enable IP forwarding (required for routing/NAT tests)
# Usage: enable_ip_forward
enable_ip_forward() {
    echo 1 > /proc/sys/net/ipv4/ip_forward
    echo 1 > /proc/sys/net/ipv6/conf/all/forwarding 2>/dev/null || true
}

# ============================================================================
# File/Log Polling Helpers
# ============================================================================

# Wait for a string to appear in a log file
# Usage: wait_for_log_entry "$LOG_FILE" "pattern" [timeout_sec]
wait_for_log_entry() {
    local log_file="$1"
    local pattern="$2"
    local timeout=$(scale_timeout "${3:-10}")
    local i=0
    local max=$((timeout * 5))  # 5 checks per second

    while [ $i -lt $max ]; do
        if grep -qE "$pattern" "$log_file" 2>/dev/null; then
            return 0
        fi
        sleep 0.2
        i=$((i + 1))
    done
    diag "TIMEOUT waiting for '$pattern' in $log_file"
    return 1
}

# ============================================================================
# HCL Configuration Templates
# ============================================================================
# These functions generate standard HCL configuration boilerplate
#
# For eBPF tests, use ebpf_config() or ebpf_config_file() to ensure
# proper network configuration and prevent safemode activation
# ============================================================================

# Generate minimal HCL config with standard loopback
# Usage: config=$(minimal_config)
# Returns HCL string that can be written to a file
minimal_config() {
    cat <<'EOF'
schema_version = "1.0"

interface "lo" {
    ipv4 = ["127.0.0.1/8"]
}

zone "local" {}
EOF
}

# Generate minimal HCL config with API enabled
# Usage: config=$(minimal_config_with_api [port])
minimal_config_with_api() {
    local port="${1:-$TEST_API_PORT}"
    cat <<EOF
schema_version = "1.0"

api {
    enabled = true
    listen = "0.0.0.0:$port"
    require_auth = false
}

interface "lo" {
    ipv4 = ["127.0.0.1/8"]
}

zone "local" {}
EOF
}

# Generate eBPF-ready HCL config with standard boilerplate
# This prevents safemode activation and provides required infrastructure
# Usage: config=$(ebpf_config [api_port])
# Returns HCL string that can be written to a file
ebpf_config() {
    local port="${1:-8080}"
    cat <<EOF
schema_version = "1.0"
ip_forwarding = true

interface "eth0" {
    zone = "wan"
    ipv4 = ["10.0.2.15/24"]
    gateway = "10.0.2.2"
}

zone "wan" {
    interfaces = ["eth0"]
}

api {
    enabled = true
    listen = "0.0.0.0:$port"
    require_auth = false
}

logging {
    level = "info"
    file = "/tmp/flywall-test.log"
}

control_plane {
    enabled = true
    socket = "/tmp/flywall.ctl"
}
EOF
}

# Generate eBPF config and write to file
# Usage: ebpf_config_file <filename> [api_port]
ebpf_config_file() {
    local filename="$1"
    local port="${2:-8080}"

    ebpf_config "$port" > "$filename"
    export CONFIG_FILE="$filename"
}
# ============================================================================
# Global State Reset (Ensure clean slate for each test)
# ============================================================================
reset_state() {
    # Suppress output to prevent serial buffer deadlocks
    case $- in
        *x*) TRACE_WAS_ON=1 ;;
    esac
    set +x

    # Only run in VM environment (check for nft/ip availability)
    if [ "$(id -u)" -eq 0 ]; then
        # Use the dedicated cleanup script if available
        # Ensure we target the exact binary name to avoid killing agents
        export BINARY_NAME=$(basename "$APP_BIN")
        # Use MOUNT_PATH (not PROJECT_ROOT) because PROJECT_ROOT is derived from
        # the test directory and may be wrong (e.g., /mnt/flywall/t instead of /mnt/flywall)
        CLEANUP_SCRIPT="$MOUNT_PATH/tools/scripts/vm_cleanup.sh"
        if [ -x "$CLEANUP_SCRIPT" ]; then
            "$CLEANUP_SCRIPT" >/dev/null 2>&1
        else
            # Inline fallback cleanup if script not found (e.g. during minimal mount)
            # Kill the binary and common leaking processes
            pkill -9 -x "$BINARY_NAME" 2>/dev/null || true
            pkill -9 -x flywall 2>/dev/null || true
            pkill -9 -x udhcpc 2>/dev/null || true
            pkill -9 -x sqlite3 2>/dev/null || true
            pkill -9 -x tcpdump 2>/dev/null || true
            pkill -9 -x tail 2>/dev/null || true

            # Kill any processes holding TCP/UDP ports (prevents bind failures)
            if command -v ss >/dev/null 2>&1; then
                for pid in $(ss -tlnp 2>/dev/null | grep -o 'pid=[0-9]*' | cut -d= -f2 | sort -u); do
                    [ "$pid" != "$$" ] && kill -9 "$pid" 2>/dev/null || true
                done
                for pid in $(ss -ulnp 2>/dev/null | grep -o 'pid=[0-9]*' | cut -d= -f2 | sort -u); do
                    [ "$pid" != "$$" ] && kill -9 "$pid" 2>/dev/null || true
                done
            fi

            nft flush ruleset 2>/dev/null || true
        fi

        # Always clean up state regardless of which cleanup path ran
        ip addr flush dev lo scope global 2>/dev/null || true
        rm -rf "$STATE_DIR"/* 2>/dev/null
        if [ "$FLYWALL_SHARE_DIR" = "$ASSETS_MOUNT" ]; then
            rm -rf "$ASSETS_MOUNT"/* 2>/dev/null
        fi
        rm -rf "$RUN_DIR" 2>/dev/null || true
        rm -rf /tmp/flywall-run-${TEST_PID}* 2>/dev/null || true
        rm -rf /tmp/test_${TEST_PID}_* 2>/dev/null || true
        rm -rf /tmp/import_${TEST_PID}_* 2>/dev/null || true

        # Always ensure loopback is healthy (even if cleanup script didn't run)
        ip link set lo up 2>/dev/null || true
        ip addr add 127.0.0.1/8 dev lo 2>/dev/null || true
        ip addr add ::1/128 dev lo 2>/dev/null || true
    fi

    if [ "$TRACE_WAS_ON" = "1" ]; then set -x; fi
}

# ============================================================================
# Process Leak Detection
# ============================================================================
# Capture process snapshot to file (excludes kernel threads, self, and known safe procs)
snapshot_procs() {
    # Format: PID PPID PGID CMD
    # Filter kernel threads (PPID 2) and Zombies (stat=Z)
    ps -eo pid,ppid,pgid,stat,comm 2>/dev/null | \
        awk '$2 != 2 && $4 !~ /^Z/ {print $1, $2, $3, $5}' | \
        grep -v '^\s*PID' | \
        grep -vE '^\s*1\s' | \
        grep -vE 'ps|grep|sh|ash|bash|sleep|cat|awk|sed|toolbox|tail|head|comm|sort|timeout' | \
        sort -n > "${1:-/tmp/procs_snapshot.txt}"
}

# Diff current processes against snapshot, log any new ones
diff_procs() {
    local snapshot="${1:-/tmp/procs_snapshot.txt}"
    local current="/tmp/procs_current.txt"

    [ ! -f "$snapshot" ] && return 0

    snapshot_procs "$current"

    # Find new processes (in current but not in snapshot)
    local leaked=$(comm -13 "$snapshot" "$current" 2>/dev/null | grep -vE '^\s*$')
    local status=0

    if [ -n "$leaked" ]; then
        echo "# FATAL: Leaked processes detected after test:"
        echo "$leaked" | while read -r line; do
            echo "#   $line"
        done
        status=1
    fi

    rm -f "$current"
    return $status
}

# Capture network state (interfaces and namespaces)
snapshot_net() {
    # Sort for consistent diffing
    ip link show | grep -v "lo:" | awk -F: '{print $2}' | sort > "${1:-/tmp/net_links_snapshot.txt}"
    ip netns list | sort > "${1:-/tmp/net_ns_snapshot.txt}"
}

# Diff network state
diff_net() {
    local link_snap="${1:-/tmp/net_links_snapshot.txt}"
    local ns_snap="${2:-/tmp/net_ns_snapshot.txt}"
    local link_cur="/tmp/net_links_current.txt"
    local ns_cur="/tmp/net_ns_current.txt"
    local status=0

    [ ! -f "$link_snap" ] && return 0

    snapshot_net "$link_cur" "$ns_cur"

    # Check for leaked interfaces
    local leaked_links=$(comm -13 "$link_snap" "$link_cur" 2>/dev/null | grep -vE '^\s*$')
    if [ -n "$leaked_links" ]; then
        echo "# FATAL: Leaked network interfaces detected:"
        echo "$leaked_links" | sed 's/^/#   /'
        status=1
    fi

    # Check for leaked namespaces
    local leaked_ns=$(comm -13 "$ns_snap" "$ns_cur" 2>/dev/null | grep -vE '^\s*$')
    if [ -n "$leaked_ns" ]; then
        echo "# FATAL: Leaked network namespaces detected:"
        echo "$leaked_ns" | sed 's/^/#   /'
        status=1
    fi
    
    rm -f "$link_cur" "$ns_cur"
    return $status
}

# Execute reset immediately
reset_state

# Take snapshots AFTER cleanup (baseline for this test)
PROC_SNAPSHOT="/tmp/procs_baseline_$$.txt"
LINK_SNAPSHOT="/tmp/net_links_baseline_$$.txt"
NS_SNAPSHOT="/tmp/net_ns_baseline_$$.txt"

snapshot_procs "$PROC_SNAPSHOT"
snapshot_net "$LINK_SNAPSHOT" "$NS_SNAPSHOT"

# Global checks and cleanup logic (does NOT exit)
_perform_leak_check_and_cleanup() {
    # 1. Force cleanup first (standard teardown)
    cleanup_processes
    
    # 2. Check for LEFTOVER leaks (things that refused to die)
    local leak_status=0
    diff_procs "$PROC_SNAPSHOT" || leak_status=1
    diff_net "$LINK_SNAPSHOT" "$NS_SNAPSHOT" || leak_status=1
    rm -f "$PROC_SNAPSHOT" "$LINK_SNAPSHOT" "$NS_SNAPSHOT"

    return $leak_status
}

# Default Trap: Just check/cleanup and exit if failed
_default_exit_trap() {
    _perform_leak_check_and_cleanup
    if [ $? -ne 0 ]; then
        exit 1
    fi
}
trap '_default_exit_trap' EXIT

# Set up automatic cleanup on exit (call this at start of test)
cleanup_on_exit() {
    # Full cleanup includes filesystem unmounts
    _full_cleanup_handler() {
        _perform_leak_check_and_cleanup
        local leak_status=$?
        
        # Additional filesystem cleanup (must run even if leaks found)
        umount -l /opt/flywall/var/lib 2>/dev/null || true
        umount -l /opt/flywall/var/log 2>/dev/null || true
        rm -rf "$RUN_DIR" 2>/dev/null
        
        if [ $leak_status -ne 0 ]; then
            exit 1
        fi
    }
    trap '_full_cleanup_handler' EXIT INT TERM
}

# Export for tests
export BRAND_NAME
export BRAND_LOWER
export BINARY_NAME
export MOUNT_PATH
export APP_BIN
export STATE_DIR
export LOG_DIR
export RUN_DIR
export CTL_SOCKET
# Also export as FLYWALL_* for the Go binary to pick up
export FLYWALL_STATE_DIR="${FLYWALL_STATE_DIR:-$STATE_DIR}"
export FLYWALL_LOG_DIR="${FLYWALL_LOG_DIR:-$LOG_DIR}"
export FLYWALL_RUN_DIR="${FLYWALL_RUN_DIR:-$RUN_DIR}"
# Default ShareDir to StateDir in tests to preserve backward compatibility for structure
# This maps share/geoip -> /opt/flywall/var/lib/geoip
export FLYWALL_SHARE_DIR="${FLYWALL_SHARE_DIR:-$STATE_DIR}"
export FLYWALL_CACHE_DIR="${FLYWALL_CACHE_DIR:-/opt/flywall/var/cache}"

export CTL_BIN="${FLYWALL_BIN:-$APP_BIN}"
    echo "DEBUG: Using CTL_BIN: $CTL_BIN"
    ls -l "$CTL_BIN"
export FLYWALL_CTL_SOCKET="${FLYWALL_CTL_SOCKET:-$CTL_SOCKET}"

# Parse kernel parameters
if grep -q "flywall.run_skipped=1" /proc/cmdline 2>/dev/null; then
    export FLYWALL_RUN_SKIPPED=1
fi

# Debug output (only shown if DEBUG=1)
if [ "${DEBUG:-0}" = "1" ]; then
    echo "# Debug: PROJECT_ROOT=$PROJECT_ROOT"
    echo "# Debug: APP_BIN=$APP_BIN"
fi

# ============================================================================
# Process Management - CRITICAL for avoiding test hangs
# ============================================================================

# Track background PIDs for cleanup
BACKGROUND_PIDS=""

# Register a PID for cleanup on exit
track_pid() {
    BACKGROUND_PIDS="$BACKGROUND_PIDS $1"
}

# Kill all tracked background processes
# Note: The agent also performs cleanup after TAP_END, but we still need
# aggressive cleanup here because tests may call this before exiting.
cleanup_processes() {
    # If any test failed, dump logs if available
    if [ "${failed_count:-0}" -gt 0 ]; then
        echo "# =================== FAILURE LOGS ==================="  >&2
        # Check /tmp and LOG_DIR for logs
        for log in /tmp/*_ctl.log /tmp/*_api.log "${LOG_DIR:-/nonexistent}"/*.log; do
            if [ -f "$log" ]; then
                echo "# --- LOG: $log ---" >&2
                cat "$log" >&2
                echo "# ----------------------------------------------" >&2
            fi
        done
        echo "# =================================================="  >&2
    fi

    for pid in $BACKGROUND_PIDS; do
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null
            sleep 0.1
            kill -9 "$pid" 2>/dev/null || true
            # Wait for process to actually terminate (reap zombie)
            wait "$pid" 2>/dev/null || true
        fi
    done
    BACKGROUND_PIDS=""

    # Process cleanup done via PIDs in BACKGROUND_PIDS
    # Avoid global pkill -f "$CONFIG_FILE" as it kills parallel tests using the same config path

    # Explicitly kill common interfering services
    pkill -9 -x dnsmasq 2>/dev/null || true
    pkill -9 -x coredns 2>/dev/null || true

    # Targeted cleanup of leaked processes to prevent hangs/conflicts
    # We match by config file to avoid killing parallel test instances
    if [ -n "${CONFIG_FILE:-}" ]; then
        # DEBUG: Check if we can see the process before killing
        if [ "${TEST_DEBUG:-}" = "1" ] || [ -n "${failed_count:-}" ]; then
            echo "# DEBUG: Attempting to kill processes matching config: $CONFIG_FILE" >&2
            pgrep -a -f "$CONFIG_FILE" | sed 's/^/# DEBUG: Found: /' >&2 || echo "# DEBUG: No processes found matching config" >&2
        fi
        pkill -f "$CONFIG_FILE" 2>/dev/null || true
    fi

    # Explicitly clean up flywall-api netns (persistent artifact)
    ip netns delete flywall-api 2>/dev/null || true
}


# ============================================================================
# Service Management - Structured background process control
# ============================================================================

# Named services registry (for better diagnostics)
SERVICE_PIDS=""
SERVICE_NAMES=""

# Start a named background service
# Usage: run_service "http-server" nc -l -p 8080
# Returns: 0 on success, sets SERVICE_PID to the background PID
run_service() {
    _name="$1"
    shift

    # Start the service in background
    "$@" &
    SERVICE_PID=$!

    # Track for cleanup
    track_pid $SERVICE_PID
    SERVICE_PIDS="$SERVICE_PIDS $SERVICE_PID"
    SERVICE_NAMES="$SERVICE_NAMES $_name:$SERVICE_PID"

    diag "Started service '$_name' (PID: $SERVICE_PID)"
    return 0
}

# Stop a specific named service
# Usage: stop_service "http-server"
stop_service() {
    _name="$1"
    _found=0

    for entry in $SERVICE_NAMES; do
        case "$entry" in
            ${_name}:*)
                _pid="${entry#*:}"
                if kill -0 "$_pid" 2>/dev/null; then
                    kill "$_pid" 2>/dev/null
                    sleep 0.1
                    kill -9 "$_pid" 2>/dev/null || true
                    diag "Stopped service '$_name' (PID: $_pid)"
                fi
                _found=1
                ;;
        esac
    done

    if [ "$_found" -eq 0 ]; then
        diag "Service '$_name' not found"
        return 1
    fi
    return 0
}

# List running services (for debugging)
list_services() {
    diag "Running services:"
    for entry in $SERVICE_NAMES; do
        _name="${entry%:*}"
        _pid="${entry#*:}"
        if kill -0 "$_pid" 2>/dev/null; then
            diag "  - $_name (PID: $_pid) [running]"
        else
            diag "  - $_name (PID: $_pid) [stopped]"
        fi
    done
}

# ============================================================================
# Common Test Functions (TAP)
# ============================================================================
test_count=0

# ============================================================================
# TAP-14 Utility Library
# ============================================================================
# Source the common TAP14 library if available (in VM or locally)
# Locations:
# 1. /mnt/flywall/tools/pkg/toolbox/harness/tap14.sh (VM mount)
# 2. $PROJECT_ROOT/tools/pkg/toolbox/harness/tap14.sh (Host local)
# 3. ../tools/pkg/toolbox/harness/tap14.sh (Relative to common.sh)

if [ -f "$MOUNT_PATH/tools/pkg/toolbox/harness/tap14.sh" ]; then
    . "$MOUNT_PATH/tools/pkg/toolbox/harness/tap14.sh"
elif [ -f "$PROJECT_ROOT/tools/pkg/toolbox/harness/tap14.sh" ]; then
    . "$PROJECT_ROOT/tools/pkg/toolbox/harness/tap14.sh"
elif [ -f "$(dirname "$0")/../../tools/pkg/toolbox/harness/tap14.sh" ]; then
    . "$(dirname "$0")/../../tools/pkg/toolbox/harness/tap14.sh"
else
    # Fallback if library missing (should happen rarely, but safe default)
    echo "# WARNING: tap14.sh library not found, using minimal fallback" >&2

    test_count=0
    failed_count=0

    tap_version_14() { echo "TAP version 14"; }
    plan() { echo "1..$1"; }
    diag() { echo "# $1"; }
    ok() {
        test_count=$((test_count + 1))
        if [ "$1" -eq 0 ]; then echo "ok $test_count - $2"; else echo "not ok $test_count - $2"; failed_count=$((failed_count + 1)); fi
    }
    skip() { test_count=$((test_count + 1)); echo "ok $test_count - # SKIP $1"; }
    pass() { test_count=$((test_count + 1)); echo "ok $test_count - $1"; }
    fail() { test_count=$((test_count + 1)); echo "not ok $test_count - $1"; failed_count=$((failed_count + 1)); }
    bail() { echo "Bail out! $1"; exit 1; }
fi


# ============================================================================
# Legacy Compatibility & Helpers
# ============================================================================

# Compatibility aliases for legacy subtest functions
# New tap14.sh handles indentation automatically via 'ok'
subtest_ok() {
    ok "$@"
}

subtest_plan() {
    plan "$@"
}

subtest_skip() {
    skip "$@"
}

subtest_diag() {
    diag "$@"
}

# Note: subtest_start and subtest_end are provided by tap14.sh

# ============================================================================
# Environment Helpers
# ============================================================================

# Helper to require root
require_root() {
    if [ "$(id -u)" -ne 0 ]; then
        echo "1..0 # SKIP Must run as root"
        exit 0
    fi
}

# Helper to require Linux (these tests won't work on BSD/macOS)
require_linux() {
    if [ "$(uname -s)" != "Linux" ]; then
        echo "1..0 # SKIP Requires Linux (got: $(uname -s))"
        exit 0
    fi
}

# Helper to require VM environment (checks for test mount or specific env var)
require_vm() {
    # Check if we're in the expected VM mount or agent sets this
    if [ ! -d "$MOUNT_PATH" ] && [ -z "${FLYWALL_TEST_VM:-}" ]; then
        echo "1..0 # SKIP Must run in test VM"
        exit 0
    fi
}

# Helper to require firewall binary
require_binary() {
    if [ ! -x "$APP_BIN" ] && ! command -v "$APP_BIN" >/dev/null 2>&1; then
        echo "1..0 # SKIP firewall binary not found at $APP_BIN"
        exit 0
    fi
}

# Helper for temp files - uses /tmp with unique naming to prevent collisions
# Format: /tmp/test_${PID}_${timestamp}_${random}_${suffix}
mktemp_compatible() {
    suffix="${1:-tmp}"
    # Add random component to prevent collision when parallel tests have same PID+timestamp
    random=$(head -c 4 /dev/urandom | od -An -tx1 | tr -d ' \n')
    echo "/tmp/test_${$}_$(date +%s)_${random}_$suffix"
}


# Immediately fail the test with a message and optional diagnostics
# Usage: fail "message" ["key" "val"...]
fail() {
    failed_count=$((failed_count + 1))
    echo "not ok - FATAL: $1"
    shift
    if [ "$#" -gt 0 ]; then
        yaml_diag "severity" "fail" "error" "Fatal Error" "$@"
    fi
    cleanup_processes
    sleep 1 # Ensure output flushes before exit
    exit 1
}

# Assert a condition, fail if false
# Usage: assert "condition" ["failure_message"]
assert() {
    if ! eval "$1"; then
        msg="${2:-Assertion failed: $1}"
        fail "$msg" "condition" "$1"
    fi
}

# Run a command with a timeout (default 5 seconds)
# Usage: run_with_timeout <timeout_seconds> <command...>
# Returns: command exit code, or 124 if timed out
run_with_timeout() {
    _timeout="$1"
    shift

    # timeout command is available on most systems
    if command -v timeout >/dev/null 2>&1; then
        timeout "$_timeout" "$@"
        return $?
    fi

    # Fallback: run in background and kill if needed
    "$@" &
    _pid=$!
    _i=0
    while [ $_i -lt "$_timeout" ]; do
        if ! kill -0 $_pid 2>/dev/null; then
            wait $_pid
            return $?
        fi
        sleep 1
        _i=$((_i + 1))
    done

    # Timed out - kill the process
    kill -9 $_pid 2>/dev/null
    wait $_pid 2>/dev/null
    echo "# TIMEOUT: Command timed out after ${_timeout}s: $*"
    return 124
}

# ============================================================================
# Process Startup Helpers (with timeout detection)
# ============================================================================

# Wait for a port to be listening (max 5 seconds by default)
# Usage: wait_for_port [port] [timeout_sec] [proto]
wait_for_port() {
    _port="$1"
    _timeout=$(scale_timeout "${2:-5}")
    _proto="${3:-tcp}"
    _i=0
    _max=$((_timeout * 2))  # 2 checks per second (0.5s sleep)

    while [ $_i -lt $_max ]; do
        if check_port "$_port" "$_proto"; then
            return 0
        fi
        sleep 0.5
        _i=$((_i + 1))
    done

    echo "# TIMEOUT waiting for $_proto port $_port after ${_timeout}s"
    return 1
}


# Wait for a file to exist (max 5 seconds by default)
wait_for_file() {
    _file="$1"
    _timeout=$(scale_timeout "${2:-5}")
    _i=0
    _max=$((_timeout * 5))  # 5 checks per second (0.2s sleep)

    while [ $_i -lt $_max ]; do
        if [ -e "$_file" ]; then
            return 0
        fi
        sleep 0.2
        _i=$((_i + 1))
    done

    echo "# TIMEOUT waiting for file $_file after ${_timeout}s"
    return 1
}

# Wait for a command to succeed (max 5 seconds by default)
# Usage: wait_for_condition "command" [timeout_seconds]
wait_for_condition() {
    _cmd="$1"
    _timeout=$(scale_timeout "${2:-5}")
    _i=0
    _max=$((_timeout * 5))  # 5 checks per second (0.2s sleep)

    while [ $_i -lt $_max ]; do
        if eval "$_cmd"; then
            return 0
        fi
        sleep 0.2
        _i=$((_i + 1))
    done

    echo "# TIMEOUT waiting for condition: $_cmd after ${_timeout}s"
    return 1
}

# Start the control plane and track its PID
# Usage: start_ctl <config_file> [extra_args...]
# Sets: CTL_PID, CTL_LOG
start_ctl() {
    _config="$1"
    shift
    export CTL_LOG="${LOG_DIR:+$LOG_DIR/ctl.log}"
    [ -z "$LOG_DIR" ] && CTL_LOG=$(mktemp_compatible ctl.log)
    echo "# DEBUG: CTL_LOG path is: $CTL_LOG"

    diag "Starting control plane with config: $_config"

    # Ensure we use the correct socket for this instance, overriding any
    # stale environment variables from previous tests in the same worker.
    export FLYWALL_CTL_SOCKET="$CTL_SOCKET"

    # Ensure clean slate, killing any stale processes and removing stale sockets
    # Perfectly safe to run since our tests are run in virtual machines.
    pkill -x "$BINARY_NAME" 2>/dev/null || true
    rm -f "$CTL_SOCKET" 2>/dev/null

    # Pre-check configuration (if binary supports it)
    if $APP_BIN --help 2>&1 | grep -q "check"; then
        if ! $APP_BIN check "$_config" >> "$CTL_LOG" 2>&1; then
            echo "# Config check failed:"
            sed 's/^/# /' "$CTL_LOG"
            fail "Config check failed for $_config"
        fi
    fi

    # Inject directory overrides via CLI flags
    # This ensures the daemon uses our isolated test directories
    # independent of /opt/flywall global paths.
    if [ -n "$FLYWALL_STATE_DIR" ]; then
        set -- --state-dir "$FLYWALL_STATE_DIR" "$@"
    fi
    if [ -n "$FLYWALL_LOG_DIR" ]; then
        set -- --log-dir "$FLYWALL_LOG_DIR" "$@"
    fi
    if [ -n "$FLYWALL_RUN_DIR" ]; then
        set -- --run-dir "$FLYWALL_RUN_DIR" "$@"
    fi
    if [ -n "$FLYWALL_SHARE_DIR" ]; then
        set -- --share-dir "$FLYWALL_SHARE_DIR" "$@"
    fi

    # Force logging to stdout so we can capture it
    export FLYWALL_LOG_FILE=stdout

    # Skip automatic API/Proxy spawning - tests will start their own test-api server
    # Unless test explicitly sets FLYWALL_SKIP_API=0 to test proxy functionality
    export FLYWALL_SKIP_API="${FLYWALL_SKIP_API:-1}"

    $APP_BIN ctl "$_config" "$@" > "$CTL_LOG" 2>&1 &
    CTL_PID=$!
    track_pid $CTL_PID

    # Wait for socket to appear (control plane needs socket to accept CLI commands)
    # Use CTL_SOCKET which is derived from brand settings
    if ! wait_for_file "$CTL_SOCKET" 10; then
        echo "# Control plane failed to create socket:"
        cat "$CTL_LOG" | head -30
        fail "Control plane socket $CTL_SOCKET did not appear within 10 seconds"
    fi

    # Verify process is still alive after socket appears
    if ! kill -0 $CTL_PID 2>/dev/null; then
        echo "# Control plane crashed after creating socket:"
        cat "$CTL_LOG" | head -30
        fail "Control plane crashed"
    fi

    diag "Control plane started (PID $CTL_PID, socket $CTL_SOCKET)"
}

# Stop the control plane
stop_ctl() {
    if [ -n "$CTL_PID" ]; then
        diag "Stopping control plane (PID $CTL_PID)..."
        kill $CTL_PID 2>/dev/null || true
        wait $CTL_PID 2>/dev/null || true
    fi
}

# Start the API server and track its PID
# Usage: start_api [extra_args...]
# Sets: API_PID, API_LOG
# Uses 'test-api' command which runs API without sandbox isolation
start_api() {
    export API_LOG="${LOG_DIR:+$LOG_DIR/api.log}"
    [ -z "$LOG_DIR" ] && API_LOG=$(mktemp_compatible api.log)

    # Try to extract port from args (default TEST_API_PORT)
    _port=$TEST_API_PORT
    _has_listen=0
    case "$*" in
        *"-listen"*) 
            _has_listen=1 
            # Simple extraction of numeric port after colon
            _port=$(echo "$*" | grep -oE "\-listen [^ ]+" | cut -d: -f2)
            ;;
    esac

    diag "Starting API server on $_port (logs: $API_LOG)"
    # Use -no-tls to force HTTP mode for reliable testing
    if [ $_has_listen -eq 0 ]; then
        $APP_BIN test-api -no-tls -listen :$_port "$@" > "$API_LOG" 2>&1 &
    else
        $APP_BIN test-api -no-tls "$@" > "$API_LOG" 2>&1 &
    fi
    API_PID=$!
    track_pid $API_PID

    # Wait for API to be actually responsive (HTTP ready)
    # Use 90s timeout for high-load parallel tests
    if ! wait_for_api_ready "$_port" 90; then
        _log=""
        [ -f "$API_LOG" ] && _log=$(head -n 50 "$API_LOG")
        fail "API server failed to start" port "$_port" log_head "$_log"
    fi
}

# Wait for API to respond to HTTP requests
# Usage: wait_for_api_ready <port> [timeout_sec]
wait_for_api_ready() {
    local port="$1"
    local timeout=$(scale_timeout "${2:-10}") # Default 10s, scaled
    local url="http://127.0.0.1:$port/api/status"  # Use /api/status which is always available

    diag "Waiting for API readiness at $url..."

    local i=0
    # Check 2x per second
    local max=$((timeout * 2))

    while [ $i -lt $max ]; do
        # Fail fast if API process has died
        if [ -n "$API_PID" ] && ! kill -0 "$API_PID" 2>/dev/null; then
            diag "API process (PID $API_PID) died unexpectedly during startup"
            echo "# API LOG OUTPUT:"
            if [ -n "$API_LOG" ] && [ -f "$API_LOG" ]; then
                cat "$API_LOG"
            fi
            return 1
        fi

        # Check if we get ANY HTTP response (even 404 is fine, just not connection reset/empty)
        if command -v curl >/dev/null 2>&1; then
            # curl returns 0 on HTTP success (even 404), non-zero on conn refused/empty
            if curl -s -m 2 "$url" >/dev/null 2>&1; then
                return 0
            fi
        elif command -v wget >/dev/null 2>&1; then
             # wget returns 0 on 200, but non-zero on 404.
             # We want to accept 404 as "server ready".
             # wget -S prints headers to stderr.
             if wget -q -S -O - "$url" 2>&1 | grep -q "HTTP/"; then
                return 0
             fi
        fi

        sleep 0.5
        i=$((i + 1))
    done

    return 1
}

# Login to API (returns token)
# Usage: TOKEN=$(login_api)
login_api() {
    curl -k -s -X POST https://127.0.0.1:8443/api/auth/login \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"password"}' | \
        grep -o '"token":"[^"]*"' | cut -d'"' -f4
}

# Alias for backward compatibility with tests that use wait_for_api
# Usage: wait_for_api <url> [timeout_sec]
wait_for_api() {
    local url="$1"
    local timeout="${2:-10}"

    # Extract port from URL (e.g., http://127.0.0.1:8080 -> 8080)
    local port=$(echo "$url" | sed -E 's/.*:([0-9]+).*/\1/')
    if [ -z "$port" ] || [ "$port" = "$url" ]; then
        port=8080
    fi

    wait_for_api_ready "$port" "$timeout"
}

# ============================================================================
# HTTP Helpers
# ============================================================================

# Simple HTTP GET, returns body (fails test on non-2xx)
http_get() {
    _url="$1"
    _response=""
    _response=$(curl -sf "$_url" 2>/dev/null) || fail "HTTP GET failed: $_url"
    echo "$_response"
}

# HTTP GET that just checks for success (2xx)
http_ok() {
    _url="$1"
    curl -sf -o /dev/null "$_url" 2>/dev/null
}

# ============================================================================
# Firewall Rule Application (fire-and-forget)
# ============================================================================

# Apply firewall rules using test mode (synchronous, exits after applying)
# Usage: apply_firewall_rules <config_file> [log_file]
# Returns: 0 on success, 1 on failure
apply_firewall_rules() {
    _config_file="$1"
    _log_file="${2:-/tmp/firewall_apply.log}"

    diag "Applying firewall rules from $_config_file..."
    if command -v timeout >/dev/null 2>&1; then
        # Capture hang logs by timing out
        timeout -s 9 20s $APP_BIN test "$_config_file" > "$_log_file" 2>&1
    else
        $APP_BIN test "$_config_file" > "$_log_file" 2>&1
    fi
    _exit_code=$?

    if [ $_exit_code -eq 0 ]; then
        diag "Firewall rules applied successfully"
        return 0
    else
        diag "Failed to apply firewall rules (exit=$_exit_code)"
        diag "Log output:"
        cat "$_log_file" | head -30
        return 1
    fi
}

# ============================================================================
# Network Namespace Helpers (for traffic behavior tests)
# ============================================================================

# Create test topology: [client_ns]--veth-lan--[router]--veth-wan--[server_ns]
# Router is the host (where flywall runs), client simulates LAN, server simulates WAN
setup_test_topology() {
    diag "Setting up network namespace test topology..."

    # Create namespaces
    ip netns add test_client 2>/dev/null || true
    ip netns add test_server 2>/dev/null || true

    # Client (LAN) veth pair
    ip link add veth-client type veth peer name veth-lan 2>/dev/null || true
    ip link set veth-client netns test_client
    ip addr add 192.168.100.1/24 dev veth-lan 2>/dev/null || true
    ip link set veth-lan up

    # Server (WAN) veth pair
    ip link add veth-server type veth peer name veth-wan 2>/dev/null || true
    ip link set veth-server netns test_server
    ip addr add 10.99.99.1/24 dev veth-wan 2>/dev/null || true
    ip link set veth-wan up

    # Configure client namespace (LAN side)
    ip netns exec test_client ip addr add 192.168.100.100/24 dev veth-client
    ip netns exec test_client ip link set veth-client up
    ip netns exec test_client ip link set lo up
    ip netns exec test_client ip route add default via 192.168.100.1

    # Configure server namespace (WAN side)
    ip netns exec test_server ip addr add 10.99.99.100/24 dev veth-server
    ip netns exec test_server ip link set veth-server up
    ip netns exec test_server ip link set lo up
    ip netns exec test_server ip route add default via 10.99.99.1

    # Enable IP forwarding on the host (router)
    enable_ip_forward

    diag "Test topology created: client(192.168.100.100) <-> router <-> server(10.99.99.100)"
}

teardown_test_topology() {
    diag "Tearing down test topology..."
    ip netns del test_client 2>/dev/null || true
    ip netns del test_server 2>/dev/null || true
    ip link del veth-lan 2>/dev/null || true
    ip link del veth-wan 2>/dev/null || true
}

# Run command in client namespace (LAN side)
run_client() {
    # Default 15s timeout (scaled) to prevent hangs
    run_with_timeout "$(scale_timeout "${CLIENT_TIMEOUT:-15}")" ip netns exec test_client "$@"
}

# Run command in server namespace (WAN side)
run_server() {
    # Default 15s timeout (scaled) to prevent hangs
    run_with_timeout "$(scale_timeout "${SERVER_TIMEOUT:-15}")" ip netns exec test_server "$@"
}

# ============================================================================
# Log Verification Helpers
# ============================================================================

# Clear kernel log buffer (for fresh log capture)
clear_kernel_log() {
    dmesg -C 2>/dev/null || true
}

# Check if kernel log contains expected message pattern
# Usage: check_log_contains "PATTERN" "description"
# Returns: 0 if found, 1 if not
check_log_contains() {
    _pattern="$1"
    _description="${2:-Log contains pattern}"

    if dmesg | grep -qE "$_pattern"; then
        return 0
    else
        diag "Log pattern not found: $_pattern"
        return 1
    fi
}

# Wait for log message to appear (with timeout)
# Usage: wait_for_log "PATTERN" [timeout_seconds]
wait_for_log() {
    _pattern="$1"
    _timeout=$(scale_timeout "${2:-5}")
    _i=0

    while [ $_i -lt $_timeout ]; do
        if dmesg | grep -qE "$_pattern"; then
            return 0
        fi
        sleep 1
        _i=$((_i + 1))
    done

    diag "TIMEOUT waiting for log: $_pattern"
    return 1
}

# Functions are automatically available after sourcing
