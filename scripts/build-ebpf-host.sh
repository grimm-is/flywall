#!/bin/bash
# eBPF Build Script for macOS Host
# Uses clang for cross-compilation from macOS to Linux BPF

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Project paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# LLVM path
LLVM_PATH="/opt/homebrew/opt/llvm/bin"
CLANG="$LLVM_PATH/clang"

echo -e "${YELLOW}Building eBPF programs on macOS host...${NC}"

# Check if clang is available
if ! command -v "$CLANG" &> /dev/null; then
    echo -e "${RED}Error: clang not found at $CLANG. Please install LLVM: brew install llvm${NC}"
    exit 1
fi

# Create build directory
mkdir -p "${PROJECT_ROOT}/internal/ebpf/programs/build"

# Compile each eBPF program
for prog in tc_offload dns_socket dhcp_socket xdp_blocklist; do
    echo "Compiling $prog..."
    "$CLANG" -O2 -target bpf -c "${PROJECT_ROOT}/internal/ebpf/programs/c/$prog.c" \
        -o "${PROJECT_ROOT}/internal/ebpf/programs/build/$prog.o" \
        -I "${PROJECT_ROOT}/internal/ebpf/programs" \
        -I "${PROJECT_ROOT}" \
        -Wno-unused-value \
        -Wno-pointer-sign \
        -Wno-compare-distinct-pointer-types \
        -Wno-gnu-variable-sized-type-not-at-end
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Built $prog.o${NC}"
    else
        echo -e "${RED}✗ Failed to build $prog.o${NC}"
        exit 1
    fi
done

# Generate Go embeddings
echo -e "${YELLOW}Generating Go embeddings...${NC}"
cd "${PROJECT_ROOT}"
go generate ./internal/ebpf/...

echo -e "${GREEN}✓ eBPF build complete!${NC}"
echo ""
echo "Built objects:"
ls -la "${PROJECT_ROOT}/internal/ebpf/programs/build/"
