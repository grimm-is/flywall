#!/bin/bash
set -e

# Performance Bisect Helper
# Returns 0 if tests are FAST (Good)
# Returns 1 if tests are SLOW (Bad)

# Cleanup function to revert go.mod/sum changes
cleanup() {
    git checkout -- go.mod go.sum >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Threshold in seconds (User said "around a minute", so let's say < 90s is good)
THRESHOLD=180

# Run the integration suite
echo "Bisect Step: Building..."
go mod tidy || true
./flywall.sh build linux toolbox > /dev/null 2>&1

echo "Bisect Step: Running Tests..."
START_TIME=$(date +%s)

# Capture output to temp file
TEST_LOG=$(mktemp)
set +e
./build/toolbox orca test -j1 integration_tests/linux/10-api/api_crud_test.sh > "$TEST_LOG" 2>&1
TEST_EXIT=$?
set -e

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "Test Duration: ${DURATION}s"

if [ $TEST_EXIT -ne 0 ]; then
    echo "Result: FAIL (Bad - Exit Code $TEST_EXIT)"
    echo "--- Test Output ---"
    tail -n 20 "$TEST_LOG"
    rm "$TEST_LOG"
    exit 1
fi

if [ $DURATION -lt $THRESHOLD ]; then
    echo "Result: FAST (Good)"
    rm "$TEST_LOG"
    exit 0
else
    echo "Result: SLOW (Bad)"
    # Optional: Print logs if slow
    echo "--- Test Output (Slow) ---"
    tail -n 10 "$TEST_LOG"
    rm "$TEST_LOG"
    exit 1
fi
