#!/bin/bash
# eBPF Build Script for Flywall
# Uses the existing VM infrastructure for cross-compilation

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Project paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo -e "${YELLOW}Building eBPF programs...${NC}"

# Check if we're on macOS (need VM) or Linux (can build natively)
if [[ "$(uname)" == "Darwin" ]]; then
    echo "Detected macOS, using VM for eBPF compilation..."
    
    # Ensure VM is running
    "${PROJECT_ROOT}/flywall.sh" vm ensure
    
    # Check if VM is actually running
    if ! pgrep -f "qemu.*rootfs" >/dev/null 2>&1; then
        echo -e "${YELLOW}Starting VM...${NC}"
        "${PROJECT_ROOT}/flywall.sh" vm start
        sleep 5  # Give VM time to boot
    fi
    
    # Build in VM using orca - compile directly without make
    echo "Building eBPF programs in VM..."
    "${PROJECT_ROOT}/flywall.sh" build toolbox
    
    # Create build directory
    "${PROJECT_ROOT}/build/toolbox" orca exec mkdir -p internal/ebpf/programs/build
    
    # Compile each eBPF program directly
    for prog in tc_offload dns_socket dhcp_socket xdp_blocklist; do
        echo "Compiling $prog..."
        "${PROJECT_ROOT}/build/toolbox" orca exec sh -c "cd internal/ebpf/programs && clang -O2 -target bpf -c c/$prog.c -o build/$prog.o -I. -I ../../../ -Wno-unused-value -Wno-pointer-sign -Wno-compare-distinct-pointer-types"
    done
    
else
    echo "Detected Linux, building natively..."
    # Build natively on Linux
    make -C "${PROJECT_ROOT}/internal/ebpf/programs"
fi

# Generate Go embeddings
echo -e "${YELLOW}Generating Go embeddings...${NC}"
cd "${PROJECT_ROOT}"
go generate ./internal/ebpf/...

echo -e "${GREEN}✓ eBPF build complete!${NC}"
echo ""
echo "Built objects:"
ls -la "${PROJECT_ROOT}/internal/ebpf/programs/build/" 2>/dev/null || echo "No build directory found"
