#!/bin/sh
# TAP (Test Anything Protocol) Reporter Library
# Provides standardized TAP output

# TAP state
TAP_TEST_COUNT=0
TAP_FAILED_COUNT=0
TAP_SKIP_COUNT=0
TAP_PLAN_COUNT=0
TAP_PLAN_SET=0
TAP_OUTPUT_STARTED=0

# Initialize TAP output
tap_init() {
    TAP_TEST_COUNT=0
    TAP_FAILED_COUNT=0
    TAP_SKIP_COUNT=0
    TAP_PLAN_SET=0
    TAP_OUTPUT_STARTED=0
}

# Set test plan
tap_plan() {
    local count="$1"
    TAP_PLAN_COUNT="$count"
    TAP_PLAN_SET=1
    
    if [ $TAP_OUTPUT_STARTED -eq 0 ]; then
        echo "TAP version 14"
        echo "1..$count"
        TAP_OUTPUT_STARTED=1
    fi
}

# Output a test result
tap_result() {
    local status="$1"  # 0 = pass, 1 = fail
    local message="$2"
    local directive="${3:-}"  # SKIP or TODO
    
    TAP_TEST_COUNT=$((TAP_TEST_COUNT + 1))
    
    # Auto-plan if not set
    if [ $TAP_PLAN_SET -eq 0 ] && [ $TAP_OUTPUT_STARTED -eq 0 ]; then
        echo "# Warning: No plan set, auto-planning"
        echo "1.."
        TAP_PLAN_SET=1
        TAP_OUTPUT_STARTED=1
    fi
    
    if [ "$status" -eq 0 ]; then
        if [ -n "$directive" ]; then
            echo "ok $TAP_TEST_COUNT - $message # $directive"
            if [ "$directive" = "SKIP" ]; then
                TAP_SKIP_COUNT=$((TAP_SKIP_COUNT + 1))
            fi
        else
            echo "ok $TAP_TEST_COUNT - $message"
        fi
    else
        TAP_FAILED_COUNT=$((TAP_FAILED_COUNT + 1))
        if [ -n "$directive" ]; then
            echo "not ok $TAP_TEST_COUNT - $message # $directive"
        else
            echo "not ok $TAP_TEST_COUNT - $message"
        fi
    fi
}

# Success (ok)
tap_ok() {
    local status="$1"
    local message="$2"
    
    if [ "$status" -eq 0 ]; then
        tap_result 0 "$message"
    else
        tap_result 1 "$message"
    fi
}

# Explicit not ok
tap_not_ok() {
    local message="$1"
    local directive="${2:-}"
    
    tap_result 1 "$message" "$directive"
}

# Skip test
tap_skip() {
    local message="$1"
    local reason="$2"
    
    tap_result 0 "$message" "SKIP $reason"
}

# TODO test
tap_todo() {
    local message="$1"
    local reason="$2"
    
    tap_result 1 "$message" "TODO $reason"
}

# Bail out
tap_bail() {
    local reason="$1"
    
    echo "Bail out! $reason"
    exit 1
}

# Diagnostic output
tap_diag() {
    local message="$1"
    
    echo "# $message"
}

# Finish testing
tap_finish() {
    if [ $TAP_PLAN_SET -eq 1 ] && [ $TAP_PLAN_COUNT -gt 0 ]; then
        if [ $TAP_TEST_COUNT -ne $TAP_PLAN_COUNT ]; then
            echo "# Warning: Planned $TAP_PLAN_COUNT tests but ran $TAP_TEST_COUNT"
        fi
    fi
    
    # Output summary if requested
    if [ "${TAP_SUMMARY:-1}" -eq 1 ]; then
        echo "# Test summary:"
        echo "# Total: $TAP_TEST_COUNT"
        echo "# Passed: $((TAP_TEST_COUNT - TAP_FAILED_COUNT - TAP_SKIP_COUNT))"
        echo "# Failed: $TAP_FAILED_COUNT"
        echo "# Skipped: $TAP_SKIP_COUNT"
        
        if [ $TAP_FAILED_COUNT -gt 0 ]; then
            return 1
        fi
    fi
    
    return 0
}

# Compatibility with existing test patterns
ok() {
    local status="$1"
    local message="$2"
    
    # Handle legacy ok $? pattern
    if [ "$status" = "$?" ] && [ -z "$message" ]; then
        message="Test completed"
    fi
    
    tap_ok "$status" "$message"
}

not_ok() {
    local message="$1"
    local directive="${2:-}"
    
    tap_not_ok "$message" "$directive"
}

pass() {
    local message="$1"
    
    tap_result 0 "$message"
}

fail() {
    local message="$1"
    
    tap_result 1 "$message"
}

skip() {
    local message="$1"
    local reason="${2:-}"
    
    tap_skip "$message" "$reason"
}

# Plan function for compatibility
plan() {
    tap_plan "$1"
}

# Diag function for compatibility
diag() {
    tap_diag "$1"
}

# Initialize on load
tap_init
