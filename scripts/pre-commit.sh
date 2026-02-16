#!/bin/bash
# Pre-commit hook to enforce unit tests pass before committing.

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Running pre-commit unit tests...${NC}"

# Check if we are in the flywall root
if [ ! -f "./flywall.sh" ]; then
    echo -e "${YELLOW}flywall.sh not found in current directory. Skipping tests.${NC}"
    exit 0
fi

# Run unit tests
./flywall.sh test unit --vm
RESULT=$?

if [ $RESULT -ne 0 ]; then
    echo -e "${RED}ERROR: Unit tests failed!${NC}"
    echo -e "${YELLOW}Please fix the tests before committing or bypass with --no-verify.${NC}"
    exit 1
fi

echo -e "${GREEN}All unit tests passed. Proceeding with commit.${NC}"
exit 0
