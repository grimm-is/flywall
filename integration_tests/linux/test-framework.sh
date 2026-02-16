#!/bin/sh
# Flywall Test Framework
# Provides a standardized environment for integration tests
# Replaces manual sourcing of common.sh

set -e

# Framework version
FRAMEWORK_VERSION="1.0.0"

# Global state
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="$TEST_DIR/lib"
PORT_POOL_MIN=10000
PORT_POOL_MAX=20000
CURRENT_PORT=$PORT_POOL_MIN
ALLOCATED_PORTS=""

# Load core libraries
. "$TEST_DIR/common.sh"

# Load framework libraries
for lib in port service config network api tap; do
    if [ -f "$LIB_DIR/${lib}.sh" ]; then
        . "$LIB_DIR/${lib}.sh"
    fi
done

# Test metadata
TEST_NAME=""
TEST_TIMEOUT="${TEST_TIMEOUT:-30}"
TEST_CLEANUP_HOOKS=""

# Framework initialization
framework_init() {
    TEST_NAME="$(basename "$0" .sh)"
    export TEST_NAME
    export TEST_DIR
    export LIB_DIR
    
    # Setup signal handlers for cleanup
    trap 'framework_cleanup' EXIT INT TERM
    
    # Initialize TAP if plan is defined
    if [ -n "$TEST_PLAN" ]; then
        tap_plan "$TEST_PLAN"
    fi
}

# Cleanup framework
framework_cleanup() {
    # Run cleanup hooks in reverse order
    for hook in $(echo "$TEST_CLEANUP_HOOKS" | tac); do
        eval "$hook" 2>/dev/null || true
    done
    
    # Release allocated ports
    for port in $ALLOCATED_PORTS; do
        release_port "$port" 2>/dev/null || true
    done
    
    # Final TAP output
    tap_finish
}

# Add cleanup hook
add_cleanup() {
    TEST_CLEANUP_HOOKS="$TEST_CLEANUP_HOOKS $1"
}

# Port management
allocate_port() {
    local var_name="$1"
    local port="$CURRENT_PORT"
    
    # Find next available port
    while netstat -ln 2>/dev/null | grep -q ":$port " || \
          ss -ln 2>/dev/null | grep -q ":$port "; do
        port=$((port + 1))
        if [ $port -gt $PORT_POOL_MAX ]; then
            port=$PORT_POOL_MIN
        fi
        if [ $port -eq $CURRENT_PORT ]; then
            echo "# ERROR: No available ports in range $PORT_POOL_MIN-$PORT_POOL_MAX" >&2
            exit 1
        fi
    done
    
    CURRENT_PORT=$((port + 1))
    ALLOCATED_PORTS="$ALLOCATED_PORTS $port"
    
    if [ -n "$var_name" ]; then
        eval "$var_name=$port"
        export "$var_name"
    fi
    
    echo "# Allocated port $port for $var_name"
    echo "$port"
}

release_port() {
    local port="$1"
    # Remove from allocated list
    ALLOCATED_PORTS=$(echo "$ALLOCATED_PORTS" | sed "s/\\b$port\\b//g")
}

# Service management with automatic port tracking
start_ctl_enhanced() {
    local config_file="$1"
    local port_var="${2:-CTL_PORT}"
    
    if [ -z "$(eval echo \$$port_var)" ]; then
        allocate_port "$port_var"
    fi
    
    start_ctl "$config_file"
    add_cleanup "stop_ctl"
}

start_api_enhanced() {
    local listen="${1:-:8080}"
    local port_var="${2:-API_PORT}"
    
    # Extract port from listen address
    if [ "$listen" = ":8080" ] || [ "$listen" = ":8081" ] || [ "$listen" = ":8082" ]; then
        if [ -z "$(eval echo \$$port_var)" ]; then
            allocate_port "$port_var"
        fi
        listen=":$(eval echo \$$port_var)"
    fi
    
    start_api -listen "$listen"
    add_cleanup "stop_api"
}

# Wait for service with polling and exponential backoff
wait_for_service() {
    local url="$1"
    local max_wait="${2:-30}"
    local interval="${3:-1}"
    local waited=0
    
    while [ $waited -lt $max_wait ]; do
        if curl -s -m 2 "$url" >/dev/null 2>&1; then
            return 0
        fi
        sleep $interval
        waited=$((waited + interval))
        # Exponential backoff with jitter
        interval=$(awk "BEGIN {print $interval * 1.5 + int(rand()*2)}")
    done
    
    echo "# ERROR: Service at $url not ready after ${max_wait}s" >&2
    return 1
}

# Configuration management
create_config() {
    local template="$1"
    local output_file="${2:-$CONFIG_FILE}"
    local port_replacements=""
    
    # Replace common port placeholders
    for var in API_PORT TLS_PORT CTL_PORT TEST_API_PORT; do
        if [ -n "$(eval echo \$$var)" ]; then
            port_replacements="$port_replacements -e 's/\$$var/$(eval echo \$$var)/g'"
        fi
    done
    
    if [ -n "$port_replacements" ]; then
        eval "sed $port_replacements \"$template\" > \"$output_file\""
    else
        cp "$template" "$output_file"
    fi
    
    add_cleanup "rm -f \"$output_file\""
    echo "$output_file"
}

# Network namespace helpers
create_namespace() {
    local name="$1"
    
    ip netns add "$name" 2>/dev/null || {
        echo "# Cleaning existing namespace $name"
        ip netns del "$name" 2>/dev/null || true
        ip netns add "$name"
    }
    
    add_cleanup "ip netns del \"$name\" 2>/dev/null || true"
    echo "# Created namespace $name"
}

# API testing helpers
api_login() {
    local base_url="$1"
    local username="${2:-admin}"
    local password="${3:-admin}"
    local cookie_file="${4:-/tmp/test_cookies}"
    
    local response=$(curl -s -c "$cookie_file" -X POST \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"$username\",\"password\":\"$password\"}" \
        "$base_url/api/auth/login")
    
    if echo "$response" | grep -q '"authenticated":true'; then
        echo "# Login successful"
        return 0
    else
        echo "# Login failed: $response"
        return 1
    fi
}

# Test execution helpers
run_test() {
    local test_name="$1"
    local test_command="$2"
    
    echo "# Running: $test_name"
    if eval "$test_command"; then
        tap_ok 0 "$test_name"
        return 0
    else
        tap_ok 1 "$test_name"
        return 1
    fi
}

skip_test() {
    local test_name="$1"
    local reason="$2"
    
    tap_skip "$test_name" "$reason"
}

# Main execution
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    # Framework being run directly
    echo "Flywall Test Framework v$FRAMEWORK_VERSION"
    echo "Usage: . test-framework.sh  # Source in your test"
    exit 0
fi

# Auto-initialize when sourced
framework_init
