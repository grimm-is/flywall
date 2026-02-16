#!/bin/sh
# Port Management Library
# Handles port allocation and prevents conflicts

# Port pool configuration
PORT_POOL_MIN=${PORT_POOL_MIN:-10000}
PORT_POOL_MAX=${PORT_POOL_MAX:-20000}
PORT_LOCK_FILE="/tmp/flywall-test-ports.lock"

# Acquire port lock
port_lock() {
    mkdir -p "$(dirname "$PORT_LOCK_FILE")"
    while ! (set -C; echo $$ > "$PORT_LOCK_FILE") 2>/dev/null; do
        sleep 0.1
    done
}

# Release port lock
port_unlock() {
    rm -f "$PORT_LOCK_FILE"
}

# Check if port is in use
port_in_use() {
    local port="$1"
    
    # Try multiple methods to check port usage
    if command -v netstat >/dev/null 2>&1; then
        netstat -ln 2>/dev/null | grep -q ":$port "
    elif command -v ss >/dev/null 2>&1; then
        ss -ln 2>/dev/null | grep -q ":$port "
    else
        # Fallback: try to bind to the port
        (echo >/dev/tcp/127.0.0.1/$port) >/dev/null 2>&1 && return 1 || return 0
    fi
}

# Find next available port
find_available_port() {
    local start_port="${1:-$PORT_POOL_MIN}"
    local port="$start_port"
    
    port_lock
    
    while [ $port -le $PORT_POOL_MAX ]; do
        if ! port_in_use "$port"; then
            # Mark port as used
            echo "$port $$" >> "/tmp/flywall-test-ports.$$"
            port_unlock
            echo "$port"
            return 0
        fi
        port=$((port + 1))
    done
    
    port_unlock
    echo "# ERROR: No available ports in range $PORT_POOL_MIN-$PORT_POOL_MAX" >&2
    return 1
}

# Allocate a port for a specific service
allocate_port() {
    local var_name="$1"
    local preferred="${2:-}"
    local port
    
    if [ -n "$preferred" ] && ! port_in_use "$preferred"; then
        port="$preferred"
    else
        port=$(find_available_port)
    fi
    
    if [ $? -eq 0 ] && [ -n "$port" ]; then
        if [ -n "$var_name" ]; then
            eval "$var_name=$port"
            export "$var_name"
        fi
        echo "# Allocated port $port for $var_name"
        return 0
    fi
    
    return 1
}

# Release a port
release_port() {
    local port="$1"
    
    port_lock
    # Remove from our tracking file
    if [ -f "/tmp/flywall-test-ports.$$" ]; then
        sed -i "/^$port /d" "/tmp/flywall-test-ports.$$"
    fi
    port_unlock
    
    echo "# Released port $port"
}

# Get random port in range
get_random_port() {
    awk "BEGIN { srand(); print int(rand()*($PORT_POOL_MAX-$PORT_POOL_MIN) + $PORT_POOL_MIN) }"
}

# Reserve a specific port (fails if in use)
reserve_port() {
    local port="$1"
    local var_name="$2"
    
    if port_in_use "$port"; then
        echo "# ERROR: Port $port is already in use" >&2
        return 1
    fi
    
    port_lock
    echo "$port $$" >> "/tmp/flywall-test-ports.$$"
    port_unlock
    
    if [ -n "$var_name" ]; then
        eval "$var_name=$port"
        export "$var_name"
    fi
    
    echo "# Reserved port $port for $var_name"
    return 0
}

# Cleanup all ports for this test
cleanup_ports() {
    if [ -f "/tmp/flywall-test-ports.$$" ]; then
        while read port _; do
            echo "# Cleaning up port $port"
        done < "/tmp/flywall-test-ports.$$"
        rm -f "/tmp/flywall-test-ports.$$"
    fi
}

# Show port usage statistics
show_port_stats() {
    echo "# Port Usage Statistics:"
    echo "# Pool range: $PORT_POOL_MIN-$PORT_POOL_MAX"
    
    local used=0
    local available=0
    
    for port in $(seq $PORT_POOL_MIN $PORT_POOL_MAX); do
        if port_in_use "$port"; then
            used=$((used + 1))
        else
            available=$((available + 1))
        fi
    done
    
    echo "# Used: $used, Available: $available"
}

# Auto-cleanup on exit
trap cleanup_ports EXIT
