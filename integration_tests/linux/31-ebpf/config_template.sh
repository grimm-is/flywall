#!/bin/sh
# Template for generating eBPF test configs with boilerplate

# Usage: In your test, use this pattern:
#
# CONFIG_FILE=$(mktemp_compatible test.hcl)
# . "$(dirname "$0")/config_template.sh"
# generate_config "$CONFIG_FILE" <<'EOF'
# # Your eBPF config here
# ebpf {
#   enabled = true
#   ...
# }
# EOF

generate_config() {
    local config_file="$1"
    local ebf_content="$2"

    cat > "$config_file" <<'EOF'
schema_version = "1.0"
ip_forwarding = true

interface "eth0" {
    zone = "wan"
    ipv4 = ["10.0.2.15/24"]
    gateway = "10.0.2.2"
}

zone "wan" {
    match {
        interface = "eth0"
    }
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

# eBPF configuration
EOF

    # Append the eBPF specific content
    echo "$ebf_content" >> "$config_file"
    echo "EOF" >> "$config_file"
}
