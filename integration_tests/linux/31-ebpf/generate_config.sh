#!/bin/sh
# Helper script to generate eBPF test configs with proper boilerplate

# Usage: ./generate_config.sh <test_name> <ebpf_config_content>

if [ $# -lt 2 ]; then
    echo "Usage: $0 <test_name> <ebpf_config_content_file>"
    echo "Example: $0 test_xdp xdp_config.hcl"
    exit 1
fi

TEST_NAME="$1"
EBPF_CONTENT="$2"
OUTPUT_FILE="${TEST_NAME}.hcl"

# Read the boilerplate
BOILERPLATE="$(dirname "$0")/common_ebpf_config.hcl"

# Generate the config
cat > "$OUTPUT_FILE" <<EOF
$(cat "$BOILERPLATE")

# eBPF configuration for $TEST_NAME
$(cat "$EBPF_CONTENT")
EOF

echo "Generated: $OUTPUT_FILE"
