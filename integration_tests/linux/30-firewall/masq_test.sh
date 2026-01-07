#!/bin/bash
# set -e removed to handle errors manually
set -x

# Path to the Flywall binary
# Path to the Flywall binary
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
. "$SCRIPT_DIR/../common.sh"
FLYWALL_BIN="$APP_BIN"
CONFIG_FILE="$PROJECT_ROOT/integration_tests/linux/configs/masq_match.hcl"

echo "Running masquerade generation test..."

# Generate the ruleset (show command creates a dry-run from config)
OUTPUT=$($FLYWALL_BIN show "$CONFIG_FILE" 2>&1)
EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
  echo "FAIL: flywall command failed with exit code $EXIT_CODE"
  echo "$OUTPUT"
  exit 1
fi

if echo "$OUTPUT" | grep -q 'oifname "eth0" masquerade'; then
  echo "PASS: Masquerade rule found for eth0."
else
  echo "FAIL: Masquerade rule NOT found for eth0."
  echo "Ruleset output:"
  echo "$OUTPUT"
  exit 1
fi
