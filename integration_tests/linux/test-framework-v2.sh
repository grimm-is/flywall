#!/bin/sh
# Flywall Test Framework v2.0
# Focused on VM isolation and perfect cleanup

set -e

# Framework version
FRAMEWORK_VERSION="2.0.0"

# Test identification
TEST_NAME="$(basename "$0" .sh)"
TEST_ID="${TEST_NAME}_$$"
export TEST_NAME TEST_ID

# Paths
TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
LIB_DIR="$TEST_DIR/lib"

# Load core libraries
. "$TEST_DIR/common.sh"

# Load framework libraries
for lib in cleanup tap; do
    if [ -f "$LIB_DIR/${lib}.sh" ]; then
        . "$LIB_DIR/${lib}.sh"
    fi
done

# Test configuration
TEST_TIMEOUT="${TEST_TIMEOUT:-30}"
TEST_PLAN="${TEST_PLAN:-}"
TEST_COUNT=0
TEST_FAILED=0

# Framework initialization
framework_init() {
    echo "# Starting test $TEST_NAME (ID: $TEST_ID)"
    echo "# Framework v$FRAMEWORK_VERSION"
    
    # Ensure clean VM state
    ensure_clean_vm
    
    # Set up TAP if plan is defined
    if [ -n "$TEST_PLAN" ]; then
        tap_plan "$TEST_PLAN"
    fi
}

# Enhanced test runner with cleanup tracking
run_test() {
    local description="$1"
    local command="$2"
    local expected_exit="${3:-0}"
    
    TEST_COUNT=$((TEST_COUNT + 1))
    
    echo "# Test $TEST_COUNT: $description"
    
    # Create temp files for this test
    local test_output=$(create_temp)
    local test_error=$(create_temp)
    
    # Run the command with timeout
    if eval "$command" >"$test_output" 2>"$test_error"; then
        local exit_code=$?
        if [ $exit_code -eq $expected_exit ]; then
            echo "ok $TEST_COUNT - $description"
        else
            echo "not ok $TEST_COUNT - $description (exit $exit_code, expected $expected_exit)"
            TEST_FAILED=$((TEST_FAILED + 1))
            if [ -s "$test_error" ]; then
                echo "# Error output:"
                sed 's/^/# /' "$test_error"
            fi
        fi
    else
        echo "not ok $TEST_COUNT - $description (command failed)"
        TEST_FAILED=$((TEST_FAILED + 1))
        if [ -s "$test_error" ]; then
            echo "# Error output:"
            sed 's/^/# /' "$test_error"
        fi
    fi
    
    # Cleanup test files
    rm -f "$test_output" "$test_error"
}

# Service management with perfect cleanup
start_service() {
    local service_type="$1"
    local config="$2"
    local args="${3:-}"
    
    case "$service_type" in
        "ctl")
            start_ctl_tracked "$config"
            ;;
        "api")
            start_api_tracked "$args"
            ;;
        *)
            echo "# ERROR: Unknown service type: $service_type"
            return 1
            ;;
    esac
}

# Configuration management with cleanup
create_config() {
    local template_content="$1"
    local config_file="${2:-$(create_temp)}"
    
    # Substitute variables
    echo "$template_content" | envsubst > "$config_file"
    
    # Track for cleanup
    CLEANUP_FILES+=("$config_file")
    
    echo "# Created config: $config_file"
    echo "$config_file"
}

# Network setup with cleanup
setup_network_namespace() {
    local name="$1"
    local ip_addr="${2:-}"
    
    create_namespace "$name"
    
    if [ -n "$ip_addr" ]; then
        ip netns exec "$name" ip addr add "$ip_addr" dev lo
        ip netns exec "$name" ip link set lo up
    fi
    
    echo "# Setup namespace $name ($ip_addr)"
}

# Wait for service with better error handling
wait_for_service() {
    local url="$1"
    local max_wait="${2:-30}"
    local service_name="${3:-service}"
    
    echo "# Waiting for $service_name at $url..."
    
    local count=0
    while [ $count -lt $max_wait ]; do
        if curl -s -m 2 "$url" >/dev/null 2>&1; then
            echo "# $service_name is ready"
            return 0
        fi
        sleep 1
        count=$((count + 1))
    done
    
    echo "# ERROR: $service_name not ready after ${max_wait}s"
    return 1
}

# Test completion
framework_complete() {
    echo "# Test $TEST_NAME completed"
    echo "# Tests run: $TEST_COUNT"
    echo "# Tests failed: $TEST_FAILED"
    
    if [ $TEST_FAILED -gt 0 ]; then
        echo "# FAILURE: $TEST_FAILED tests failed"
        return 1
    else
        echo "# SUCCESS: All tests passed"
        return 0
    fi
}

# Retry mechanism for flaky operations
retry() {
    local max_attempts="$1"
    local delay="$2"
    local description="$3"
    shift 3
    local command="$@"
    
    echo "# Retrying: $description (max $max_attempts attempts)"
    
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        echo "# Attempt $attempt/$max_attempts"
        if eval "$command"; then
            echo "# Success on attempt $attempt"
            return 0
        fi
        
        if [ $attempt -lt $max_attempts ]; then
            echo "# Failed, waiting ${delay}s before retry..."
            sleep "$delay"
        fi
        
        attempt=$((attempt + 1))
    done
    
    echo "# All $max_attempts attempts failed"
    return 1
}

# Resource monitoring
check_resources() {
    echo "# Resource check:"
    
    # Process count
    local process_count=$(ps aux | wc -l)
    echo "# Processes: $process_count"
    
    # Memory usage
    local mem_usage=$(free -m | awk 'NR==2{printf "%.1f%%", $3*100/$2}')
    echo "# Memory usage: $mem_usage"
    
    # Disk space
    local disk_usage=$(df /tmp | awk 'NR==2{print $5}')
    echo "# /tmp usage: $disk_usage"
    
    # Check for issues
    if [ "$process_count" -gt 500 ]; then
        echo "# WARNING: High process count"
    fi
}

# Auto-initialize when sourced
framework_init

# Export key functions for test use
export -f run_test start_service create_config setup_network_namespace
export -f wait_for_service retry check_resources framework_complete
