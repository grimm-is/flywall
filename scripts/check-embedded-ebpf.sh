#!/bin/bash

# Check if eBPF programs are embedded in the binary
# This script builds the project and verifies eBPF programs are included

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Checking Embedded eBPF Programs${NC}"
echo "================================="

# Build the project
echo -e "\n${YELLOW}Building project...${NC}"
go build -o /tmp/flywall-test .

# Check binary size
BINARY_SIZE=$(stat -f%z /tmp/flywall-test/flywall 2>/dev/null || stat -c%s /tmp/flywall-test/flywall)
echo -e "\nBinary size: ${BINARY_SIZE} bytes"

# Check for eBPF program signatures in binary
echo -e "\n${YELLOW}Checking for embedded eBPF programs...${NC}"

EBPF_PROGRAMS=(
	"tc_offload"
	"dns_socket"
	"dhcp_socket"
	"xdp_blocklist"
)

for prog in "${EBPF_PROGRAMS[@]}"; do
	if strings /tmp/flywall-test/flywall | grep -q "$prog"; then
		echo -e "${GREEN}✓ Found $prog${NC}"
	else
		echo -e "${RED}✗ Missing $prog${NC}"
	fi
done

# Check for ELF signatures (eBPF objects)
echo -e "\n${YELLOW}Checking for ELF signatures...${NC}"
if strings /tmp/flywall-test/flywall | grep -q "ELF"; then
	echo -e "${GREEN}✓ ELF objects found (eBPF programs)${NC}"
else
	echo -e "${RED}✗ No ELF objects found${NC}"
fi

# Count embedded objects
echo -e "\n${YELLOW}Embedded object count:${NC}"
OBJECT_COUNT=$(strings /tmp/flywall-test/flywall | grep -c "\.o" || echo "0")
echo "Found $OBJECT_COUNT .o references"

# Clean up
rm -rf /tmp/flywall-test

echo -e "\n${GREEN}Check complete!${NC}"
echo -e "\nNote: The eBPF programs are embedded using Go's embed directive"
echo -      "in the generated *_bpfel.go files. This enables single-binary"
echo -      "deployment without external eBPF object files."
