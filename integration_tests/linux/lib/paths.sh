#!/bin/sh
# Path Management Library
# Ensures unique paths for parallel test execution

# Test identifier
TEST_ID="$(basename "$0" .sh)_$$"
TEST_BASE_DIR="/tmp/flywall-tests"

# Ensure base directory exists
mkdir -p "$TEST_BASE_DIR"

# Unique directories for this test
get_test_dir() {
    local subdir="${1:-tmp}"
    echo "$TEST_BASE_DIR/$TEST_ID/$subdir"
}

# Create unique test directory
create_test_dir() {
    local subdir="${1:-tmp}"
    local dir=$(get_test_dir "$subdir")
    mkdir -p "$dir"
    echo "$dir"
}

# Unique config file path
get_config_path() {
    local name="${1:-config.hcl}"
    echo "$(get_test_dir)/$name"
}

# Unique log file path
get_log_path() {
    local name="${1:-test.log}"
    echo "$(get_test_dir)/$name"
}

# Unique socket path
get_socket_path() {
    local name="${1:-flywall.sock}"
    echo "$(get_test_dir)/$name"
}

# Setup unique environment variables
setup_unique_paths() {
    # Create base test directory
    local test_dir=$(create_test_dir)
    
    # Set common variables with unique paths
    export CONFIG_FILE="${CONFIG_FILE:-$(get_config_path)}"
    export STATE_DIR="${STATE_DIR:-$(create_test_dir state)}"
    export LOG_FILE="${LOG_FILE:-$(get_log_path)}"
    export SOCKET_PATH="${SOCKET_PATH:-$(get_socket_path)}"
    
    # Common temp files
    export TEST_CONFIG="${TEST_CONFIG:-$(get_config_path test.hcl)}"
    export CTL_CONFIG="${CTL_CONFIG:-$(get_config_path ctl.hcl)}"
    export API_CONFIG="${API_CONFIG:-$(get_config_path api.hcl)}"
    
    # Log directories
    export API_LOG="${API_LOG:-$(get_log_path api.log)}"
    export CTL_LOG="${CTL_LOG:-$(get_log_path ctl.log)}"
    
    # Certificate directory
    export CERT_DIR="${CERT_DIR:-$(create_test_dir certs)}"
    
    echo "# Using unique paths for test $TEST_ID"
    echo "# Base dir: $test_dir"
}

# Cleanup test paths
cleanup_test_paths() {
    local test_dir="$TEST_BASE_DIR/$TEST_ID"
    if [ -d "$test_dir" ]; then
        rm -rf "$test_dir" 2>/dev/null || true
        echo "# Cleaned up test directory: $test_dir"
    fi
}

# Fix hardcoded paths in a file
fix_hardcoded_paths() {
    local file="$1"
    local temp_file=$(mktemp)
    
    # Replace common hardcoded paths with unique ones
    sed -e "s|/tmp/api_crud_$$.hcl|$(get_config_path api_crud.hcl)|g" \
        -e "s|/tmp/api_key_$$.hcl|$(get_config_path api_key.hcl)|g" \
        -e "s|/tmp/learning_api_$$.hcl|$(get_config_path learning_api.hcl)|g" \
        -e "s|/tmp/api_staging_$$.hcl|$(get_config_path api_staging.hcl)|g" \
        -e "s|/tmp/websocket_$$.hcl|$(get_config_path websocket.hcl)|g" \
        -e "s|/tmp/flywall-test.log|$(get_log_path)|g" \
        -e "s|/tmp/flywall.ctl|$(get_socket_path)|g" \
        "$file" > "$temp_file"
    
    mv "$temp_file" "$file"
}

# Auto-cleanup on exit
trap cleanup_test_paths EXIT

# Initialize unique paths when sourced
setup_unique_paths
