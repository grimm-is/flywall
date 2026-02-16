#!/bin/sh
#
# TAP14 Parser Coverage Test
# ============================================
# This test exercises basic and advanced TAP14 features.
# It is designed to verify that the 'orca' test runner correctly:
# 1. Parses 'ok' and 'not ok'
# 2. Handles YAML diagnostics
# 3. Respects 'SKIP' and 'TODO' directives
# 4. Correctly nests subtests
# 5. Fails the run if there are any non-TODO failures

# LOCATE TAP14 LIBRARY
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo "$SCRIPT_DIR/../../../../..")"

# Try to load tap14.sh from various locations
if [ -f "$PROJECT_ROOT/internal/toolbox/harness/tap14.sh" ]; then
    . "$PROJECT_ROOT/internal/toolbox/harness/tap14.sh"
elif [ -f "$SCRIPT_DIR/../tap14.sh" ]; then
    . "$SCRIPT_DIR/../tap14.sh"
elif [ -f "../../tap14.sh" ]; then
    . "../../tap14.sh"
else
    echo "Bail out! Cannot find tap14.sh library"
    exit 1
fi

tap_version_14
plan 12

# 1. Basic Pass
pass "Simple pass"

# 2. Basic Fail (Should cause suite failure)
fail "Simple fail" severity fail

# 3. Skip
skip "Skipping this one"

# 4. Todo (Should NOT cause suite failure)
todo "Implement feature X" "This feature is missing"

# 5. Assertions - is (Pass)
is "apple" "apple" "Strings matches"

# 6. Assertions - is (Fail)
is "apple" "orange" "Strings mismatch" # Expected failure

# 7. Assertions - like (Pass)
like "foobar" "^foo" "Regex matches"

# 8. Assertions - unlike (Fail)
unlike "foobar" "bar" "Regex matches but shouldn't" # Expected failure

# 9. Subtests (Passing)
subtest_start "Group 1: Passing"
    pass "Child test 1"
    pass "Child test 2"
subtest_end "Group 1 passed"

# 10. Subtests (Failing)
subtest_start "Group 2: Failing"
    pass "Child test 1"
    fail "Child test 2"
subtest_end "Group 2 failed"

# 11. YAML Diagnostics Helper
ok 0 "YAML Diagnostics Check"
diag_yaml \
    foo bar \
    clean true \
    multiline "Line 1
Line 2"

# 12. Final Check
pass "Final sanity check"

diag "================================================"
diag "Expected Failures:"
diag "  - Test 2 (Simple fail)"
diag "  - Test 4 (marked TODO - should NOT count as fail)"
diag "  - Test 6 (is mismatch)"
diag "  - Test 8 (unlike fail)"
diag "  - Test 10 (Group 2 failed)"
diag ""
diag "TOTAL FAILURES EXPECTED: 4 (Tests 2, 6, 8, 10)"
diag "If 'orca' reports passed or anything other than ~4 failures, it's broken."
diag "================================================"
