#!/bin/bash
set -e

# Ensure we are in project root
cd "$(dirname "$0")/.."
PROJECT_ROOT=$(pwd)

# Create local directories
mkdir -p "$PROJECT_ROOT/local/run/api"
mkdir -p "$PROJECT_ROOT/local/log"
mkdir -p "$PROJECT_ROOT/local/state"

# Set environment variables
export FLYWALL_LOG_FILE="$PROJECT_ROOT/local/log/flywall.log"
export FLYWALL_STATE_DIR="$PROJECT_ROOT/local/state"
export FLYWALL_RUN_DIR="$PROJECT_ROOT/local/run"

echo "Starting Flywall Backend for E2E..."
echo "  Run Dir: $FLYWALL_RUN_DIR"
echo "  Log File: $FLYWALL_LOG_FILE"
echo "  Config: configs/mac_test.hcl"

# Run in background
go run main.go ctl configs/mac_test.hcl &
PID=$!
echo $PID > "$PROJECT_ROOT/local/run/flywall_e2e.pid"

echo "Backend started (PID $PID). Listening on :8080"
