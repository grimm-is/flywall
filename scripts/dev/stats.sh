#!/bin/sh
set -e

# Ensure dependencies
if ! command -v scc >/dev/null 2>&1; then
    echo "Error: 'scc' not found. Please install it (e.g., go install github.com/boyter/scc/v3@latest)"
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "Error: 'jq' not found. Please install it (e.g., brew install jq)"
    exit 1
fi

echo "Scanning..."

# Get Code LOC (excluding tests and vendor)
# We use a temporary file to debug if needed, but direct pipe is fine if tools work
APP_LOC=$(scc . --exclude-dir vendor --exclude-file "_test.go" --format json 2>/dev/null | jq -r '[.[] | select(.Name=="Go") | .Code] | add // 0')

# Get Total LOC (including tests)
TOTAL_LOC=$(scc . --exclude-dir vendor --format json 2>/dev/null | jq -r '[.[] | select(.Name=="Go") | .Code] | add // 0')

# Default to 0 if empty
APP_LOC=${APP_LOC:-0}
TOTAL_LOC=${TOTAL_LOC:-0}

# Calculate Test LOC
TEST_LOC=$((TOTAL_LOC - APP_LOC))

# Calculate Ratio
# Calculate Test Percentage (Test / Code)
PERCENTAGE="0.0%"
if [ "$APP_LOC" -gt 0 ] && [ "$TEST_LOC" -gt 0 ]; then
    PERCENTAGE=$(awk -v t="$TEST_LOC" -v a="$APP_LOC" 'BEGIN { printf "%.1f%%", (t/a)*100 }')
fi

# Calculate Reading Time (25 lines/min)
TOTAL_MINUTES=$((TOTAL_LOC / 25))
HOURS=$((TOTAL_MINUTES / 60))
MINUTES=$((TOTAL_MINUTES % 60))

TIME_STR="${MINUTES}m"
if [ "$HOURS" -gt 0 ]; then
    TIME_STR="${HOURS}h ${MINUTES}m"
fi

echo "-----------------------------------"
echo "Application LOC: $APP_LOC"
echo "Test LOC:        $TEST_LOC"
echo "Test Percentage: $PERCENTAGE (Test/Code)"
echo "Reading Time:    $TIME_STR (@25 lines/min)"
echo "-----------------------------------"
