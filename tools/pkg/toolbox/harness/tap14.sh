#!/bin/sh
# TAP14 Utility Library
# Provides standardized functions for emitting TAP14 compatible test output
# Usage: . /path/to/tap14.sh

# ============================================================================
# Global State
# ============================================================================
test_count=0
failed_count=0

# Indentation for subtests (4 spaces per level)
TAP_INDENT=""

# Internal: Track subtest state
# Format: "count:failed" stack
_subtest_stack=""

# ============================================================================
# Core Directives
# ============================================================================

# Emit TAP version header
# Usage: tap_version_14
tap_version_14() {
    echo "${TAP_INDENT}TAP version 14"
}

# Emit plan line
# Usage: plan <count>
plan() {
    echo "${TAP_INDENT}1..$1"
}

# Emit a diagnostic line (comment)
# Usage: diag "message"
diag() {
    echo "${TAP_INDENT}# $1"
}

# Bail out (abort testing)
# Usage: bail "reason"
bail() {
    echo "${TAP_INDENT}Bail out! $1"
    exit 1
}

# ============================================================================
# Test Result Reporting
# ============================================================================

# Internal helper to print result line
# Usage: _result <status> <desc> [directive] [diag_key value ...]
_result() {
    local status=$1
    shift
    local desc="$1"
    shift
    local directive="${1:-}"
    shift || true

    # Increment counter
    test_count=$((test_count + 1))

    # Determine prefix
    local prefix="ok"
    if [ "$status" -ne 0 ]; then
        prefix="not ok"

        # If failure is not expected (TODO), increment failure count
        # Check if directive contains TODO (case insensitive)
        if echo "$directive" | grep -qi "TODO"; then
            : # Expected failure, do not increment failed_count
        else
            failed_count=$((failed_count + 1))
        fi
    fi

    # Format output
    local out="${TAP_INDENT}${prefix} ${test_count} - ${desc}"
    if [ -n "$directive" ]; then
        out="${out} # ${directive}"
    fi
    echo "$out"

    # Print YAML diagnostics if provided
    if [ "$#" -gt 0 ]; then
        yaml_diag "$@"
    fi

    return 0
}

# Report generic result
# Usage: ok <status_code> "description" [directive] [yaml...]
ok() {
    local status=$1; shift
    local desc="$1"; shift
    # Check if next arg matches valid directive (SKIP or TODO)
    # If not, assume it's part of diagnostics or empty
    # For compatibility with older usage: if arg starts with "severity", it's diag.
    local directive=""
    if [ "$#" -gt 0 ] && echo "$1" | grep -Eqi "^(SKIP|TODO)"; then
        directive="$1"
        shift
    fi
    _result "$status" "$desc" "$directive" "$@"
}

# Report success
pass() {
    ok 0 "$@"
}

# Report failure
fail() {
    ok 1 "$@"
}

# Skip a test
# Usage: skip "reason" [description] -> ok N - description # SKIP reason
skip() {
    local reason="$1"
    local desc="${2:-Skipped test}"
    ok 0 "$desc" "SKIP $reason"
}

# TODO: Mark a test as explicitly incomplete/expected failure
# Usage: todo "reason" <command...> or explicit result
todo() {
    local reason="$1"
    shift
    # If remaining args resemble a test command (e.g. ok 0 ...)
    # parse them? No, shell limitations.
    # Usage guideline: ok 0 "desc" "TODO reason"

    # Just a helper to format the directive string?
    # No, let's allow wrapping a command?
    # impossible to intercept 'ok' call inside.

    # Fallback: acts as 'fail' but marked TODO
    ok 1 "${1:-Expected failure}" "TODO $reason"
}

# ============================================================================
# Assertions
# ============================================================================

# Assert two values are equal (string)
# Usage: is <got> <expected> "description"
is() {
    local got="$1"
    local expected="$2"
    local desc="$3"

    if [ "$got" = "$expected" ]; then
        ok 0 "$desc"
    else
        ok 1 "$desc" \
            severity fail \
            expected "$expected" \
            actual "$got"
    fi
}

# Assert two values are NOT equal
# Usage: isnt <got> <unexpected> "description"
isnt() {
    local got="$1"
    local unexpected="$2"
    local desc="$3"

    if [ "$got" != "$unexpected" ]; then
        ok 0 "$desc"
    else
        ok 1 "$desc" \
            severity fail \
            message "values should not be equal" \
            actual "$got" \
            unexpected "$unexpected"
    fi
}

# Assert value matches regex
# Usage: like <got> <regex> "description"
like() {
    local got="$1"
    local regex="$2"
    local desc="$3"

    if echo "$got" | grep -qE "$regex"; then
        ok 0 "$desc"
    else
        ok 1 "$desc" \
            severity fail \
            message "value does not match regex" \
            actual "$got" \
            regex "$regex"
    fi
}

# Assert value does NOT match regex
# Usage: unlike <got> <regex> "description"
unlike() {
    local got="$1"
    local regex="$2"
    local desc="$3"

    if ! echo "$got" | grep -qE "$regex"; then
        ok 0 "$desc"
    else
        ok 1 "$desc" \
            severity fail \
            message "value matches regex (but shouldn't)" \
            actual "$got" \
            regex "$regex"
    fi
}

# ============================================================================
# Output Formatting
# ============================================================================

# Emit YAML diagnostic block
# Usage: yaml_diag key value [key value ...]
# Usage: diag_yaml key value ... (alias)
yaml_diag() {
    echo "${TAP_INDENT}  ---"
    while [ "$#" -gt 0 ]; do
        key="$1"
        shift
        val="${1:-}"
        shift || true
        # Handle multi-line values (very basic)
        if echo "$val" | grep -q $'\n'; then
            echo "${TAP_INDENT}  $key: |"
            echo "$val" | sed "s/^/${TAP_INDENT}    /"
        else
            echo "${TAP_INDENT}  $key: $val"
        fi
    done
    echo "${TAP_INDENT}  ..."
}

diag_yaml() {
    yaml_diag "$@"
}

# ============================================================================
# Subtests
# ============================================================================

# Start a nested subtest scope
# Usage: subtest_start "description"
subtest_start() {
    local desc="$1"

    # Save current state to stack
    _subtest_stack="${test_count}:${failed_count}|${_subtest_stack}"

    # Reset state for subtest
    test_count=0
    failed_count=0

    echo "${TAP_INDENT}# Subtest: ${desc}"

    # Increase indent
    TAP_INDENT="${TAP_INDENT}    "
}

# End a nested subtest scope
# Usage: subtest_end
subtest_end() {
    local description="Subtest complete"
    if [ "$#" -gt 0 ]; then description="$1"; fi

    # Capture result of subtest
    local sub_failed=$failed_count

    # Restore parent state from stack
    # Extract first item from stack
    local saved="${_subtest_stack%%|*}"
    _subtest_stack="${_subtest_stack#*|}"

    # Restore indentation
    TAP_INDENT="${TAP_INDENT%    }"

    # Restore parent counters
    test_count="${saved%%:*}"
    failed_count="${saved##*:}"

    # Report result of subtest to parent
    if [ "$sub_failed" -eq 0 ]; then
        ok 0 "$description"
    else
        ok 1 "$description" severity fail failed_tests "$sub_failed"
    fi
}
