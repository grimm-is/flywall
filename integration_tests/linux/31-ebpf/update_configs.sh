#!/bin/sh
# Update all eBPF test configs to include boilerplate

BOILERPLATE='schema_version = "1.0"
ip_forwarding = true

interface "eth0" {
    zone = "wan"
    ipv4 = ["10.0.2.15/24"]
    gateway = "10.0.2.2"
}

zone "wan" {
    interfaces = ["eth0"]
}

api {
    enabled = true
    listen = "0.0.0.0:8080"
}

logging {
    level = "info"
    file = "/tmp/flywall-test.log"
}

control_plane {
    enabled = true
    socket = "/tmp/flywall.ctl"
}
'

# Function to update a test file
update_test() {
    local test_file="$1"

    echo "Updating $test_file..."

    # Find the start and end of the config
    start_line=$(grep -n 'cat > "$CONFIG_FILE" <<'EOF'' "$test_file" | cut -d: -f1)
    end_line=$(grep -n '^EOF$' "$test_file" | head -1 | cut -d: -f1)

    if [ -z "$start_line" ] || [ -z "$end_line" ]; then
        echo "  Could not find config in $test_file"
        return
    fi

    # Extract the eBPF specific part
    ebf_part=$(sed -n "$((start_line + 1)),$((end_line - 1))p" "$test_file" | grep -v '^schema_version')

    # Create new config
    new_config="$BOILERPLATE

# eBPF configuration
$ebf_part"

    # Replace the config in the file
    sed -i.bak "${start_line},${end_line}c\\
cat > \"\$CONFIG_FILE\" <<'EOF'\\
$new_config\\
EOF" "$test_file"

    echo "  Updated successfully"
}

# Update all test files except the ones already done
for test in 03-tc-classifier.sh 04-socket-filters.sh 05-performance-benchmark.sh 06-feature-interaction.sh 07-fallback-mechanisms.sh; do
    if [ -f "$test" ]; then
        update_test "$test"
    fi
done

echo "All configs updated!"
