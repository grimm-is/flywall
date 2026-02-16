#!/bin/bash
set -e
source "$(dirname "$0")/../common.sh"
cleanup_on_exit

# Test QoS FWMark Classification
# Verifies that QoS policies create correct fwmark filters and nftables mangle rules.

setup_test_env "qos_fwmark"

# Helper aliases
start_flywall() {
    start_ctl "$CONFIG_FILE"
    wait_for_file "$CTL_SOCKET" 15
}

reload_flywall() {
    # For this test, valid reload is just restarting or sending signal if supported
    # Easier to just restart for clean state or use reload command if available
    # The integration test common.sh has start_ctl which kills previous instance.
    start_flywall
}


# 1. Apply QoS Configuration with Rules
echo ">>> Configuring QoS Policy..."
cat <<EOF > "$CONFIG_FILE"
interface "eth1" {
    ipv4 = ["192.168.10.1/24"]
    zone = "lan"
}

qos_policy "lan-qos" {
    interface = "eth1"
    enabled   = true
    upload_mbps = 100
    download_mbps = 100

    class "voip" {
        priority = 1
        rate     = "10%"
    }

    class "web" {
        priority = 3
        rate     = "50%"
    }

    rule "sip-rule" {
        class = "voip"
        proto = "udp"
        dest_port = 5060
    }

    rule "http-rule" {
        class = "web"
        proto = "tcp"
        dest_port = 80
    }
}
EOF

echo ">>> Starting Flywall with Config..."
start_flywall

# 2. Verify nftables Mangle Rules
echo ">>> Verifying nftables mangle rules..."
if ! NFT_OUTPUT=$(nft list table ip flywall 2>&1); then
    echo "ERROR: Failed to list mangle table. Output: $NFT_OUTPUT"
    echo "--- CTL LOG ---"
    if [ -f "$CTL_LOG" ]; then
        cat "$CTL_LOG"
    else
        echo "CTL_LOG not found at $CTL_LOG"
    fi
    echo "---------------"
    exit 1
fi
echo "Nftables output:"
echo "$NFT_OUTPUT"

# Expect mark setting for the rules
# Marks are calculated: 0xF000 + (policyIdx << 8) + classIdx
# Policy 0.
# Class "voip" index 0 -> Mark 0xF000
# Class "web" index 1 -> Mark 0xF001

if ! echo "$NFT_OUTPUT" | grep -qE "meta mark set 0x0*f000"; then
    fail "Missing mangle rule for VOIP (mark 0xf000)"
fi

if ! echo "$NFT_OUTPUT" | grep -qE "meta mark set 0x0*f001"; then
    fail "Missing mangle rule for WEB (mark 0xf001)"
fi

# 3. Verify TC Filters
echo ">>> Verifying TC filters..."
TC_OUTPUT=$(tc filter show dev eth1)
echo "TC output:"
echo "$TC_OUTPUT"

# We added fw filters for these marks
# Expect: fw ... handle 0xf000 ... classid 1:a
# Expect: fw ... handle 0xf001 ... classid 1:b (indexes start at 10)

if ! echo "$TC_OUTPUT" | grep -qE "handle 0x0*f000"; then
    fail "Missing TC filter for VOIP (mark 0xf000)"
fi

if ! echo "$TC_OUTPUT" | grep -qE "handle 0x0*f001"; then
    fail "Missing TC filter for WEB (mark 0xf001)"
fi

pass "QoS FWMark test passed"
