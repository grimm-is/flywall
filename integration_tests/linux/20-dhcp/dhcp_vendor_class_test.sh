#!/bin/sh
# Integration test for DHCP Vendor Class Matching (Option 60)
set -e
set -x

TEST_TIMEOUT=30

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
. "$SCRIPT_DIR/../common.sh"

CONFIG_FILE=$(mktemp /tmp/dhcp-vendor-config.hcl.XXXXXX)
DHCP_LOG=$(mktemp /tmp/dhcp-vendor.log.XXXXXX)

cat > "$CONFIG_FILE" << 'EOF'
schema_version = "1.1"

interface "lo" {
  zone = "lan"
  ipv4 = ["192.168.1.1/24"]
}

zone "lan" {
  match {
    interface = "lo"
  }
}

dhcp {
  enabled = true

  # Global vendor classes
  vendor_class "voip_phone" {
    identifier = "Polycom"
    options = {
      tftp_server = "tftp-voip.example.com"
      bootfile = "polycom.cfg"
    }
  }

  vendor_class "cisco_phone" {
    identifier = "Cisco"
    options = {
      tftp_server = "tftp-cisco.example.com"
      bootfile = "cisco.cfg"
    }
  }

  scope "lan_pool" {
    interface = "lo"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"
    router = "192.168.1.1"
    dns = ["1.1.1.1"]

    # Standard scope options
    options = {
      domain_name = "standard.lan"
    }
  }
}
EOF

plan 3

# Start control plane
# Note: lo usually has 127.0.0.1, we add 192.168.1.1
ip addr add 192.168.1.1/24 dev lo 2>/dev/null || true
start_ctl "$CONFIG_FILE"
wait_for_port 67 10 udp

# Create udhcpc script to capture options
UDHCPC_SCRIPT=$(mktemp /tmp/udhcpc-vendor-script.XXXXXX)
cat > "$UDHCPC_SCRIPT" << 'UDHCPC_EOF'
#!/bin/sh
echo "DHCP_EVENT=$1" >> /tmp/dhcp-vendor.log
env | grep -E "^(tftp|bootfile|domain)" >> /tmp/dhcp-vendor.log
UDHCPC_EOF
chmod +x "$UDHCPC_SCRIPT"

# --- Test Case 1: Matching Vendor Class "Polycom" ---
diag "Testing matching vendor class 'Polycom'..."
rm -f /tmp/dhcp-vendor.log
ip addr del 192.168.1.1/24 dev lo 2>/dev/null || true
# -V Polycom-VVX400 (should match "Polycom" identifier via strings.Contains)
timeout 10 udhcpc -f -i lo -s "$UDHCPC_SCRIPT" -V "Polycom-VVX400" -q -n -t 2 || true
ip addr add 192.168.1.1/24 dev lo 2>/dev/null || true

if grep -q "tftp=tftp-voip.example.com" /tmp/dhcp-vendor.log && \
   grep -q "bootfile=polycom.cfg" /tmp/dhcp-vendor.log; then
    ok 0 "Vendor class 'Polycom' matched and options injected"
else
    ok 1 "Vendor class 'Polycom' FAILED to match or inject options"
    cat /tmp/dhcp-vendor.log
fi

# --- Test Case 2: Matching Vendor Class "Cisco" ---
diag "Testing matching vendor class 'Cisco'..."
rm -f /tmp/dhcp-vendor.log
ip addr del 192.168.1.1/24 dev lo 2>/dev/null || true
timeout 10 udhcpc -f -i lo -s "$UDHCPC_SCRIPT" -V "Cisco-CP-8841" -q -n -t 2 || true
ip addr add 192.168.1.1/24 dev lo 2>/dev/null || true

if grep -q "tftp=tftp-cisco.example.com" /tmp/dhcp-vendor.log && \
   grep -q "bootfile=cisco.cfg" /tmp/dhcp-vendor.log; then
    ok 0 "Vendor class 'Cisco' matched and options injected"
else
    ok 1 "Vendor class 'Cisco' FAILED to match or inject options"
    cat /tmp/dhcp-vendor.log
fi

# --- Test Case 3: No Match (Standard Options) ---
diag "Testing no vendor match (standard options only)..."
rm -f /tmp/dhcp-vendor.log
ip addr del 192.168.1.1/24 dev lo 2>/dev/null || true
timeout 10 udhcpc -f -i lo -s "$UDHCPC_SCRIPT" -V "Generic-Device" -q -n -t 2 || true
ip addr add 192.168.1.1/24 dev lo 2>/dev/null || true

if ! grep -q "tftp" /tmp/dhcp-vendor.log && \
   grep -q "domain=standard.lan" /tmp/dhcp-vendor.log; then
    ok 0 "No vendor match correctly returns only standard options"
else
    ok 1 "No vendor match FAILED: found unexpected options or missed standard options"
    cat /tmp/dhcp-vendor.log
fi

# Cleanup
stop_ctl
ip addr del 192.168.1.1/24 dev lo 2>/dev/null || true
rm -f "$CONFIG_FILE" "$UDHCPC_SCRIPT" /tmp/dhcp-vendor.log
