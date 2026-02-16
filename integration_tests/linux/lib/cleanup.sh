#!/bin/sh
# Cleanup Library for VM Reuse
# Ensures perfect cleanup for test isolation

# Global cleanup tracking
declare -a CLEANUP_PIDS=()
declare -a CLEANUP_NAMESPACES=()
declare -a CLEANUP_FILES=()
declare -a CLEANUP_DIRS=()
declare -a CLEANUP_HOOKS=()

# Test identifier for unique resources
TEST_ID="${TEST_NAME:-test}_$$"
CLEANUP_DONE=0

# Register cleanup function
add_cleanup() {
    CLEANUP_HOOKS+=("$1")
}

# Process tracking
track_process() {
    local pid="$1"
    local name="${2:-process}"
    
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        CLEANUP_PIDS+=("$pid:$name")
        echo "# Tracking process $pid ($name)"
    fi
}

# Process cleanup
cleanup_processes() {
    echo "# Cleaning up tracked processes..."
    
    for proc in "${CLEANUP_PIDS[@]}"; do
        pid="${proc%:*}"
        name="${proc#*:}"
        
        if kill -0 "$pid" 2>/dev/null; then
            echo "# Stopping process $pid ($name)"
            kill -TERM "$pid" 2>/dev/null || true
            
            # Wait for graceful shutdown
            local count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 5 ]; do
                sleep 1
                count=$((count + 1))
            done
            
            # Force kill if still running
            if kill -0 "$pid" 2>/dev/null; then
                kill -KILL "$pid" 2>/dev/null || true
                echo "# Force killed process $pid ($name)"
            fi
        fi
    done
    
    # Kill any remaining flywall processes
    pkill -f "flywall" 2>/dev/null || true
    pkill -f "ctlplane" 2>/dev/null || true
    
    CLEANUP_PIDS=()
}

# Namespace management
create_namespace() {
    local name="$1"
    
    # Clean if exists
    if ip netns list | grep -q "^$name "; then
        echo "# Cleaning existing namespace $name"
        ip netns del "$name" 2>/dev/null || true
    fi
    
    # Create new
    ip netns add "$name"
    CLEANUP_NAMESPACES+=("$name")
    echo "# Created namespace $name"
}

cleanup_namespaces() {
    echo "# Cleaning up namespaces..."
    
    for ns in "${CLEANUP_NAMESPACES[@]}"; do
        if ip netns list | grep -q "^$ns "; then
            ip netns del "$ns" 2>/dev/null || true
            echo "# Deleted namespace $ns"
        fi
    done
    
    # Clean any remaining test namespaces
    ip netns list | grep "^test_" | while read ns; do
        ns=$(echo "$ns" | cut -d' ' -f1)
        ip netns del "$ns" 2>/dev/null || true
    done
    
    CLEANUP_NAMESPACES=()
}

# Network interface cleanup
cleanup_interfaces() {
    echo "# Cleaning up network interfaces..."
    
    # Remove veth pairs
    ip link show | grep "veth-" | while read line; do
        iface=$(echo "$line" | cut -d: -f1 | tr -d ' ')
        if [ -n "$iface" ]; then
            ip link del "$iface" 2>/dev/null || true
            echo "# Deleted interface $iface"
        fi
    done
    
    # Remove test bridges
    ip link show | grep "br-test" | while read line; do
        iface=$(echo "$line" | cut -d: -f1 | tr -d ' ')
        if [ -n "$iface" ]; then
            ip link del "$iface" 2>/dev/null || true
            echo "# Deleted bridge $iface"
        fi
    done
}

# File tracking
create_temp() {
    local pattern="${1:-/tmp/test.XXXXXX}"
    local file=$(mktemp "$pattern")
    CLEANUP_FILES+=("$file")
    echo "$file"
}

create_dir() {
    local path="${1:-/tmp/test-dir.XXXXXX}"
    mkdir -p "$path"
    CLEANUP_DIRS+=("$path")
    echo "$path"
}

cleanup_files() {
    echo "# Cleaning up files..."
    
    for file in "${CLEANUP_FILES[@]}"; do
        rm -f "$file" 2>/dev/null || true
    done
    
    for dir in "${CLEANUP_DIRS[@]}"; do
        rm -rf "$dir" 2>/dev/null || true
    done
    
    # Clean test files in common locations
    find /tmp -name "test_*" -type f -mmin +60 -delete 2>/dev/null || true
    find /tmp -name "flywall_*" -type f -mmin +60 -delete 2>/dev/null || true
    
    CLEANUP_FILES=()
    CLEANUP_DIRS=()
}

# State directory cleanup
cleanup_state() {
    echo "# Cleaning up state directories..."
    
    # Common state directories
    for dir in /opt/flywall/var/lib /var/lib/flywall /run/flywall; do
        if [ -d "$dir" ]; then
            # Keep auth.json if it exists (shared resource)
            if [ -f "$dir/auth.json" ]; then
                mv "$dir/auth.json" "/tmp/auth_$$.json.backup.$$" 2>/dev/null || true
            fi
            
            rm -rf "$dir"/* 2>/dev/null || true
            
            # Restore auth.json
            if [ -f "/tmp/auth_$$.json.backup.$$" ]; then
                mv "/tmp/auth_$$.json.backup.$$" "$dir/auth.json" 2>/dev/null || true
            fi
        fi
    done
}

# VM health check and cleanup
ensure_clean_vm() {
    echo "# Ensuring clean VM state..."
    
    # Kill all flywall processes
    pkill -f "flywall" 2>/dev/null || true
    pkill -f "ctlplane" 2>/dev/null || true
    pkill -f "firewall" 2>/dev/null || true
    sleep 1
    
    # Kill any remaining test processes
    ps aux | grep "test_" | grep -v grep | while read line; do
        pid=$(echo "$line" | awk '{print $2}')
        kill -TERM "$pid" 2>/dev/null || true
    done
    
    # Clean network
    cleanup_interfaces
    cleanup_namespaces
    
    # Clean IPC resources
    ipcrm -a 2>/dev/null || true
    
    # Check for zombies
    local zombies=$(ps aux | awk '$8 ~ /^Z/ { count++ } END { print count+0 }')
    if [ "$zombies" -gt 0 ]; then
        echo "# WARNING: $zombies zombie processes detected"
        # Clear zombies by reaping
        wait 2>/dev/null || true
    fi
}

# Main cleanup function
cleanup_all() {
    if [ $CLEANUP_DONE -eq 1 ]; then
        return
    fi
    
    echo "# Running cleanup for test $TEST_ID..."
    
    # Run custom hooks first
    for hook in "${CLEANUP_HOOKS[@]}"; do
        eval "$hook" 2>/dev/null || true
    done
    
    # Standard cleanup
    cleanup_processes
    cleanup_namespaces
    cleanup_interfaces
    cleanup_files
    cleanup_state
    
    # Final VM cleanup
    ensure_clean_vm
    
    CLEANUP_DONE=1
    echo "# Cleanup complete"
}

# Set up cleanup trap
trap cleanup_all EXIT INT TERM

# Enhanced service starters with tracking
start_ctl_tracked() {
    local config="$1"
    
    # Ensure clean state first
    ensure_clean_vm
    
    # Start control plane
    start_ctl "$config"
    CTL_PID=$!
    track_process "$CTL_PID" "controlplane"
    
    # Wait for readiness
    local count=0
    while [ $count -lt 30 ]; do
        if [ -f "/run/flywall/ctl.pid" ] && kill -0 $(cat "/run/flywall/ctl.pid") 2>/dev/null; then
            echo "# Control plane ready (PID $(cat /run/flywall/ctl.pid))"
            return 0
        fi
        sleep 1
        count=$((count + 1))
    done
    
    echo "# ERROR: Control plane failed to start"
    return 1
}

start_api_tracked() {
    local args="$1"
    
    start_api $args
    API_PID=$!
    track_process "$API_PID" "api"
    
    # Wait for API readiness
    local count=0
    while [ $count -lt 15 ]; do
        if curl -s http://127.0.0.1:8080/api/status >/dev/null 2>&1; then
            echo "# API ready"
            return 0
        fi
        sleep 1
        count=$((count + 1))
    done
    
    echo "# WARNING: API not ready after 15 seconds"
    return 1
}

# Initialize cleanup
echo "# Cleanup library loaded for test $TEST_ID"
