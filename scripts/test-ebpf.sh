#!/bin/bash

# Test script for eBPF programs
# This script loads and tests each eBPF program

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${PROJECT_ROOT}/scripts/utils.sh"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test results
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local test_command="$2"
    
    echo -e "\n${BLUE}Testing: ${test_name}${NC}"
    echo "----------------------------------------"
    
    if eval "$test_command"; then
        echo -e "${GREEN}✓ PASSED: ${test_name}${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗ FAILED: ${test_name}${NC}"
        ((TESTS_FAILED++))
    fi
}

# Function to check if running in VM
check_vm() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if grep -q "flywall-vm" /proc/cmdline 2>/dev/null; then
            return 0
        fi
    fi
    return 1
}

# Main testing function
main() {
    echo -e "${BLUE}eBPF Program Testing${NC}"
    echo "===================="
    
    # Check if we have root privileges (needed for eBPF)
    if [[ $EUID -ne 0 ]]; then
        echo -e "${YELLOW}Warning: eBPF tests require root privileges. Running without may limit functionality.${NC}"
    fi
    
    # Build eBPF programs first
    echo -e "\n${YELLOW}Building eBPF programs...${NC}"
    "${PROJECT_ROOT}/scripts/build-ebpf-host.sh"
    
    # Test 1: Load TC Offload program
    run_test "TC Offload Program Loading" \
        "go run -c 'package main; import \"fmt\"; import \"grimm.is/flywall/internal/ebpf/programs\"; import \"grimm.is/flywall/internal/logging\"; logger := logging.New(\"test\"); _, err := programs.NewTCOffloadProgram(logger); if err != nil { fmt.Printf(\"Error: %v\\n\", err); os.Exit(1) }; fmt.Println(\"TC Offload loaded successfully\")'"
    
    # Test 2: Load DNS Socket program
    run_test "DNS Socket Program Loading" \
        "go run -c 'package main; import \"fmt\"; import \"grimm.is/flywall/internal/ebpf/socket\"; import \"grimm.is/flywall/internal/logging\"; logger := logging.New(\"test\"); _, err := socket.NewDNSFilter(logger); if err != nil { fmt.Printf(\"Error: %v\\n\", err); os.Exit(1) }; fmt.Println(\"DNS Socket loaded successfully\")'"
    
    # Test 3: Load DHCP Socket program
    run_test "DHCP Socket Program Loading" \
        "go run -c 'package main; import \"fmt\"; import \"grimm.is/flywall/internal/ebpf/socket\"; import \"grimm.is/flywall/internal/logging\"; logger := logging.New(\"test\"); _, err := socket.NewDHCPFilter(logger); if err != nil { fmt.Printf(\"Error: %v\\n\", err); os.Exit(1) }; fmt.Println(\"DHCP Socket loaded successfully\")'"
    
    # Test 4: Load XDP Blocklist program
    run_test "XDP Blocklist Program Loading" \
        "go run -c 'package main; import \"fmt\"; import \"grimm.is/flywall/internal/ebpf/xdp_blocklist\"; import \"grimm.is/flywall/internal/logging\"; logger := logging.New(\"test\"); _, err := xdp_blocklist.NewBlocklistProgram(logger); if err != nil { fmt.Printf(\"Error: %v\\n\", err); os.Exit(1) }; fmt.Println(\"XDP Blocklist loaded successfully\")'"
    
    # Test 5: Verify eBPF maps are accessible
    run_test "eBPF Maps Accessibility" \
        "go test -v ./internal/ebpf/loader -run TestMapAccess"
    
    # Test 6: Integration test - full eBPF manager
    run_test "eBPF Manager Integration" \
        "go test -v ./internal/ebpf -run TestManagerIntegration"
    
    # Print summary
    echo -e "\n${BLUE}Test Summary${NC}"
    echo "=============="
    echo -e "Total tests: $((TESTS_PASSED + TESTS_FAILED))"
    echo -e "${GREEN}Passed: ${TESTS_PASSED}${NC}"
    echo -e "${RED}Failed: ${TESTS_FAILED}${NC}"
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "\n${GREEN}All tests passed! 🎉${NC}"
        exit 0
    else
        echo -e "\n${RED}Some tests failed. Please check the output above.${NC}"
        exit 1
    fi
}

# Check dependencies
check_dependencies() {
    echo -e "${YELLOW}Checking dependencies...${NC}"
    
    # Check for Go
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed${NC}"
        exit 1
    fi
    
    # Check for clang (for building)
    if ! command -v /opt/homebrew/opt/llvm/bin/clang &> /dev/null; then
        if ! command -v clang &> /dev/null; then
            echo -e "${RED}Error: clang is not installed${NC}"
            exit 1
        fi
    fi
    
    echo -e "${GREEN}Dependencies OK${NC}"
}

# Run main function
main "$@"
