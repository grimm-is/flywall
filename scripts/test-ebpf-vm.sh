#!/bin/bash

# Run eBPF integration tests in the VM
# This script runs eBPF tests that require Linux and root privileges

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Running eBPF Integration Tests in VM${NC}"
echo "===================================="

# Check if VM is available
if ! "${PROJECT_ROOT}/flywall.sh" vm status >/dev/null 2>&1; then
    echo -e "${RED}Error: VM is not running. Please start it first:${NC}"
    echo -e "${YELLOW}./flywall.sh vm start${NC}"
    exit 1
fi

# Ensure VM has the latest code
echo -e "\n${YELLOW}Syncing code to VM...${NC}"
"${PROJECT_ROOT}/flywall.sh" vm sync

# Build eBPF programs in VM (to ensure they're built for Linux)
echo -e "\n${YELLOW}Building eBPF programs in VM...${NC}"
"${PROJECT_ROOT}/build/toolbox" orca exec sh -c "cd /flywall && ./scripts/build-ebpf.sh"

# Run the tests in VM
echo -e "\n${YELLOW}Running eBPF integration tests...${NC}"
"${PROJECT_ROOT}/build/toolbox" orca exec sh -c "cd /flywall && INTEGRATION=1 go test -v ./internal/ebpf/programs -run 'TestPrograms'"
echo -e "\n${YELLOW}Running eBPF control plane tests...${NC}"
"${PROJECT_ROOT}/build/toolbox" orca exec sh -c "cd /flywall && INTEGRATION=1 go test -v ./internal/ebpf/controlplane -run 'TestControlPlane'"
echo -e "\n${YELLOW}Running eBPF integration tests...${NC}"
"${PROJECT_ROOT}/build/toolbox" orca exec sh -c "cd /flywall && INTEGRATION=1 go test -v ./internal/ebpf -run 'TestIntegration'"
echo -e "\n${YELLOW}Running eBPF performance tests...${NC}"
"${PROJECT_ROOT}/build/toolbox" orca exec sh -c "cd /flywall && INTEGRATION=1 go test -v ./internal/ebpf/performance -run 'TestPerformance'"
echo -e "\n${YELLOW}Running eBPF statistics tests...${NC}"
"${PROJECT_ROOT}/build/toolbox" orca exec sh -c "cd /flywall && INTEGRATION=1 go test -v ./internal/ebpf/stats"

echo -e "\n${GREEN}Integration tests completed!${NC}"
