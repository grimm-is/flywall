#!/bin/bash

# eBPF Performance Benchmark Script
# Runs comprehensive performance tests on eBPF programs

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${PROJECT_ROOT}/scripts/utils.sh" 2>/dev/null || true

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default configuration
DURATION=30
TARGET_PPS=1000000
PACKET_SIZE=1500
WORKERS=$(nproc)
INTERFACE="lo"
OUTPUT_DIR="${PROJECT_ROOT}/benchmark-results"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --duration)
            DURATION="$2"
            shift 2
            ;;
        --pps)
            TARGET_PPS="$2"
            shift 2
            ;;
        --size)
            PACKET_SIZE="$2"
            shift 2
            ;;
        --workers)
            WORKERS="$2"
            shift 2
            ;;
        --interface)
            INTERFACE="$2"
            shift 2
            ;;
        --output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo "  --duration SECONDS    Test duration (default: 30)"
            echo "  --pps PPS            Target packets per second (default: 1000000)"
            echo "  --size BYTES         Packet size in bytes (default: 1500)"
            echo "  --workers COUNT      Number of worker threads (default: nproc)"
            echo "  --interface IFACE    Network interface to use (default: lo)"
            echo "  --output DIR         Output directory for results"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}eBPF Performance Benchmark${NC}"
echo "=========================="
echo -e "Duration: ${YELLOW}${DURATION}s${NC}"
echo -e "Target PPS: ${YELLOW}${TARGET_PPS}${NC}"
echo -e "Packet Size: ${YELLOW}${PACKET_SIZE} bytes${NC}"
echo -e "Workers: ${YELLOW}${WORKERS}${NC}"
echo -e "Interface: ${YELLOW}${INTERFACE}${NC}"
echo -e "Output: ${YELLOW}${OUTPUT_DIR}${NC}"
echo ""

# Check if running in VM
if ! grep -q "flywall-vm" /proc/cmdline 2>/dev/null; then
    echo -e "${RED}Error: This benchmark must be run in the VM${NC}"
    echo -e "Use: ${YELLOW}./flywall.sh vm start${NC}"
    echo -e "Then: ${YELLOW}./flywall.sh vm exec ./scripts/benchmark-ebpf.sh${NC}"
    exit 1
fi

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}Error: Benchmark requires root privileges${NC}"
    exit 1
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${OUTPUT_DIR}/benchmark_${TIMESTAMP}.json"

# System information
echo -e "\n${YELLOW}Collecting system information...${NC}"
{
    echo "=== System Information ==="
    echo "Timestamp: $(date)"
    echo "Kernel: $(uname -r)"
    echo "CPU: $(grep 'model name' /proc/cpuinfo | head -1 | cut -d: -f2 | xargs)"
    echo "Cores: $(nproc)"
    echo "Memory: $(free -h | grep '^Mem:' | awk '{print $2}')"
    echo "eBPF JIT: $(cat /proc/sys/net/core/bpf_jit_enable 2>/dev/null || echo 'N/A')"
    echo ""
    
    echo "=== Network Interface Information ==="
    ip addr show "$INTERFACE" 2>/dev/null || echo "Interface $INTERFACE not found"
    echo ""
    
    echo "=== eBPF Limits ==="
    grep . /proc/sys/net/core/bpf_* 2>/dev/null || echo "eBPF limits not available"
    echo ""
} > "${OUTPUT_DIR}/system_info_${TIMESTAMP}.txt"

# Build eBPF programs
echo -e "\n${YELLOW}Building eBPF programs...${NC}"
cd /flywall
./scripts/build-ebpf.sh

# Run unit benchmarks
echo -e "\n${YELLOW}Running unit benchmarks...${NC}"
go test -bench=. -benchmem ./internal/ebpf/programs 2>&1 | \
    tee "${OUTPUT_DIR}/unit_benchmarks_${TIMESTAMP}.txt"

# Run load tests
echo -e "\n${YELLOW}Running load tests...${NC}"

# TC Program Load Test
echo -e "\n${BLUE}Testing TC Program Performance${NC}"
go test -v -run TestLoadTest ./internal/ebpf/performance \
    -args \
    -duration=${DURATION} \
    -target_pps=${TARGET_PPS} \
    -packet_size=${PACKET_SIZE} \
    -workers=${WORKERS} \
    -interface=${INTERFACE} \
    2>&1 | tee "${OUTPUT_DIR}/tc_loadtest_${TIMESTAMP}.txt"

# Performance benchmarks
echo -e "\n${YELLOW}Running performance benchmarks...${NC}"
go test -bench=BenchmarkTCProgram -benchmem ./internal/ebpf/performance \
    2>&1 | tee "${OUTPUT_DIR}/tc_benchmark_${TIMESTAMP}.txt"

# Generate summary report
echo -e "\n${YELLOW}Generating summary report...${NC}"
cat > "${REPORT_FILE}" << EOF
{
  "timestamp": "$(date -Iseconds)",
  "test_config": {
    "duration_seconds": ${DURATION},
    "target_pps": ${TARGET_PPS},
    "packet_size_bytes": ${PACKET_SIZE},
    "workers": ${WORKERS},
    "interface": "${INTERFACE}"
  },
  "system": {
    "kernel": "$(uname -r)",
    "cpu_cores": $(nproc),
    "memory_gb": $(free -g | awk '/^Mem:/{print $2}')
  },
  "results": {
    "note": "Detailed results in separate files in ${OUTPUT_DIR}"
  }
}
EOF

# Extract key metrics
echo -e "\n${GREEN}Benchmark Results Summary${NC}"
echo "========================"
echo -e "Report: ${YELLOW}${REPORT_FILE}${NC}"
echo -e "System Info: ${YELLOW}${OUTPUT_DIR}/system_info_${TIMESTAMP}.txt${NC}"
echo -e "Unit Benchmarks: ${YELLOW}${OUTPUT_DIR}/unit_benchmarks_${TIMESTAMP}.txt${NC}"
echo -e "TC Load Test: ${YELLOW}${OUTPUT_DIR}/tc_loadtest_${TIMESTAMP}.txt${NC}"
echo -e "TC Benchmark: ${YELLOW}${OUTPUT_DIR}/tc_benchmark_${TIMESTAMP}.txt${NC}"

# Performance recommendations
echo -e "\n${YELLOW}Performance Recommendations${NC}"
echo "==============================="

# Check if JIT is enabled
if [[ "$(cat /proc/sys/net/core/bpf_jit_enable 2>/dev/null)" == "1" ]]; then
    echo -e "${GREEN}✓ eBPF JIT is enabled${NC}"
else
    echo -e "${RED}✗ eBPF JIT is disabled - enable for better performance:${NC}"
    echo "  echo 1 > /proc/sys/net/core/bpf_jit_enable"
fi

# Check memory limits
if grep -q "bpf_jit_limit" /proc/sys/net/core/ 2>/dev/null; then
    JIT_LIMIT=$(cat /proc/sys/net/core/bpf_jit_limit)
    if [[ $JIT_LIMIT -lt 1000000000 ]]; then
        echo -e "${YELLOW}⚠ JIT limit is low ($(($JIT_LIMIT / 1024 / 1024))MB) - consider increasing${NC}"
    else
        echo -e "${GREEN}✓ JIT limit is sufficient ($(($JIT_LIMIT / 1024 / 1024))MB)${NC}"
    fi
fi

echo -e "\n${GREEN}Benchmark completed!${NC}"
echo -e "View detailed results in: ${YELLOW}${OUTPUT_DIR}${NC}"
