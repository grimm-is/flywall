#!/bin/bash
set -e

cd "$(dirname "$0")/.."
PROJECT_ROOT=$(pwd)

# Cleanup function
cleanup() {
    echo "Stopping backend..."
    if [ -f "$PROJECT_ROOT/local/run/flywall_e2e.pid" ]; then
        PID=$(cat "$PROJECT_ROOT/local/run/flywall_e2e.pid")
        kill $PID || true
        rm "$PROJECT_ROOT/local/run/flywall_e2e.pid"
    fi
}
trap cleanup EXIT

# 1. Start Backend
./scripts/run-e2e-backend.sh

# Wait for backend to be ready
echo "Waiting for backend..."
sleep 8
# Optionally check if port 8080 is open
# nc -z localhost 8080 || { echo "Backend failed to start"; exit 1; }

# 2. Run Playwright
echo "Running Playwright Tests..."
cd ui

# Ensure dependencies are up to date (npm ci is fast when lock file matches)
echo "Ensuring UI dependencies are installed..."
npm ci --silent 2>/dev/null || npm install --silent

export API_URL="http://localhost:8080"
npx playwright test --config=tests/e2e/playwright.config.ts

echo "Tests Completed Successfully!"
