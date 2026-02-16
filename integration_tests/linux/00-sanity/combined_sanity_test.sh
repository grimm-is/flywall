#!/bin/sh
# Combined Sanity Check Test (TAP14 Subtests)
#
# Consolidates small environment sanity checks into a single VM run.
# Each original test is a TAP14 subtest with 4-space indentation.
#
# Usage: ./combined_sanity_test.sh

set -x

. "$(dirname "$0")/../common.sh"

# Require root for most checks
require_root

# Output TAP14 header
tap_version_14

# Plan: 5 subtests
plan 5

# ============================================================================
# Subtest 1: Environment (from sanity_test.sh)
# ============================================================================
run_environment_subtest() {
    subtest_start "Environment"
    subtest_plan 8

    # Helper to wait for interfaces
    wait_for_interfaces() {
        local interfaces="$@"
        local timeout=10
        local count=0
        for iface in $interfaces; do
            until ip link show $iface >/dev/null 2>&1; do
                sleep 1
                count=$((count + 1))
                if [ $count -ge $timeout ]; then
                    return 1
                fi
            done
        done
        return 0
    }

    # 1. Check Kernel Modules (nftables)
    modprobe nf_tables_ipv4 >/dev/null 2>&1
    subtest_ok 0 "Attempted to load nf_tables module"

    # 2. Check Loopback Interface
    ip link show lo >/dev/null 2>&1
    subtest_ok $? "loopback interface exists"

    # 3. Check Test Interfaces Existence
    wait_for_interfaces eth0 eth1 eth2
    subtest_ok $? "Test interfaces (eth0, eth1, eth2) present"

    # 4. Check Essential Tools
    MISSING_TOOLS=""
    which ip >/dev/null || MISSING_TOOLS="$MISSING_TOOLS ip"
    if ! which curl >/dev/null && ! which wget >/dev/null; then
        MISSING_TOOLS="$MISSING_TOOLS curl/wget"
    fi
    which netstat >/dev/null 2>&1 || which ss >/dev/null 2>&1 || MISSING_TOOLS="$MISSING_TOOLS netstat/ss"

    if [ -z "$MISSING_TOOLS" ]; then
        subtest_ok 0 "Essential tools present"
    else
        subtest_ok 1 "Missing tools: $MISSING_TOOLS"
    fi

    # 5. Check DHCP server available
    if which dnsmasq >/dev/null 2>&1 || which dhcpd >/dev/null 2>&1 || which udhcpd >/dev/null 2>&1; then
        subtest_ok 0 "DHCP server available"
    else
        subtest_skip "No DHCP server found"
    fi

    # 6. Check DHCP client available
    if which dhclient >/dev/null 2>&1 || which udhcpc >/dev/null 2>&1; then
        subtest_ok 0 "DHCP client available"
    else
        subtest_skip "No DHCP client found"
    fi

    # 7. Check netcat
    which nc >/dev/null 2>&1
    subtest_ok $? "netcat available"

    # 8. Check socket stats tool
    which ss >/dev/null 2>&1 || which netstat >/dev/null 2>&1
    subtest_ok $? "socket stats tool available"

    subtest_end
}

# ============================================================================
# Subtest 2: Health (from health_test.sh)
# ============================================================================
run_health_subtest() {
    subtest_start "Health"
    subtest_plan 12

    subtest_diag "Testing nftables health..."

    # 1. nft command available
    command -v nft >/dev/null 2>&1
    subtest_ok $? "nft command available"

    # 2. Can list tables
    nft list tables >/dev/null 2>&1
    subtest_ok $? "Can list nftables tables"

    # 3. Can create/delete test table
    nft add table inet health_test 2>/dev/null
    subtest_ok $? "Can create test table"
    nft delete table inet health_test 2>/dev/null

    subtest_diag "Testing conntrack health..."

    # 4. Conntrack count readable
    if [ -f /proc/sys/net/netfilter/nf_conntrack_count ]; then
        subtest_ok 0 "Conntrack count readable"
    else
        subtest_skip "Conntrack not available"
    fi

    subtest_diag "Testing interface health..."

    # 5. ip command available
    command -v ip >/dev/null 2>&1
    subtest_ok $? "ip command available"

    # 6. Can list interfaces
    ip link show >/dev/null 2>&1
    subtest_ok $? "Can list interfaces"

    subtest_diag "Testing disk health..."

    # 7. Can write to /tmp
    echo "test" > /tmp/.health_test 2>/dev/null
    subtest_ok $? "Can write to /tmp"
    rm -f /tmp/.health_test

    subtest_diag "Testing memory health..."

    # 8. /proc/meminfo readable
    if [ -f /proc/meminfo ]; then
        subtest_ok 0 "/proc/meminfo readable"
    else
        subtest_ok 1 "/proc/meminfo not available"
    fi

    # 9. MemAvailable present
    grep -q "MemAvailable" /proc/meminfo 2>/dev/null
    subtest_ok $? "MemAvailable metric present"

    subtest_diag "Testing process health..."

    # 10. /proc/self accessible
    [ -d /proc/self ]
    subtest_ok $? "/proc/self accessible"

    # 11. Uptime readable
    if [ -f /proc/uptime ]; then
        subtest_ok 0 "System uptime readable"
    else
        subtest_skip "Uptime not available"
    fi

    # 12. Loopback exists
    ip link show lo >/dev/null 2>&1
    subtest_ok $? "Loopback interface exists"

    subtest_end
}

# ============================================================================
# Subtest 3: Validation (from validation_test.sh)
# ============================================================================
run_validation_subtest() {
    subtest_start "Validation"
    subtest_plan 10

    subtest_diag "Testing interface validation..."

    # 1. Valid interface name
    echo "eth0" | grep -qE '^[a-zA-Z][a-zA-Z0-9._-]*$'
    subtest_ok $? "Valid interface name pattern"

    # 2. Invalid interface name rejected
    echo "0eth" | grep -qE '^[a-zA-Z][a-zA-Z0-9._-]*$'
    [ $? -ne 0 ]
    subtest_ok $? "Rejects invalid interface name"

    # 3. Valid VLAN ID
    vlan_id=100
    [ "$vlan_id" -ge 1 ] && [ "$vlan_id" -le 4094 ]
    subtest_ok $? "Valid VLAN ID (100)"

    subtest_diag "Testing IP/CIDR validation..."

    # 4. Valid IPv4 address
    echo "192.168.1.1" | grep -qE '^([0-9]{1,3}\.){3}[0-9]{1,3}$'
    subtest_ok $? "Valid IPv4 pattern"

    # 5. Valid CIDR notation
    echo "10.0.0.0/8" | grep -qE '^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$'
    subtest_ok $? "Valid CIDR notation"

    subtest_diag "Testing policy validation..."

    # 6. Valid action values
    for action in accept drop reject; do
        echo "$action" | grep -qiE '^(accept|drop|reject)$' || { subtest_ok 1 "Valid actions"; break; }
    done
    subtest_ok 0 "Valid action values"

    # 7. Invalid action rejected
    echo "allow" | grep -qiE '^(accept|drop|reject)$'
    [ $? -ne 0 ]
    subtest_ok $? "Rejects invalid action"

    # 8. Valid port range
    port=443
    [ "$port" -ge 1 ] && [ "$port" -le 65535 ]
    subtest_ok $? "Valid port number"

    subtest_diag "Testing NAT validation..."

    # 9. Valid NAT types
    for nat_type in masquerade snat dnat; do
        echo "$nat_type" | grep -qiE '^(masquerade|snat|dnat)$' || { subtest_ok 1 "Valid NAT types"; break; }
    done
    subtest_ok 0 "Valid NAT types"

    # 10. Invalid NAT type rejected
    echo "nat66" | grep -qiE '^(masquerade|snat|dnat)$'
    [ $? -ne 0 ]
    subtest_ok $? "Rejects invalid NAT type"

    subtest_end
}

# ============================================================================
# Subtest 4: Conntrack Helpers (from conntrack_test.sh)
# ============================================================================
run_conntrack_subtest() {
    subtest_start "Conntrack"
    subtest_plan 8

    # Check modprobe available
    if ! command -v modprobe >/dev/null 2>&1; then
        subtest_diag "modprobe not available, skipping"
        for i in 1 2 3 4 5 6 7 8; do
            subtest_skip "modprobe not available"
        done
        subtest_end
        return
    fi

    subtest_diag "Testing core conntrack module..."

    # 1. nf_conntrack loaded or loadable
    if lsmod | grep -q "nf_conntrack"; then
        subtest_ok 0 "nf_conntrack already loaded"
    else
        modprobe nf_conntrack 2>/dev/null
        subtest_ok $? "Can load nf_conntrack"
    fi

    # 2. Netfilter proc interface
    [ -d /proc/sys/net/netfilter ]
    subtest_ok $? "Netfilter proc interface available"

    subtest_diag "Testing conntrack helpers..."

    # 3. FTP helper
    modprobe nf_conntrack_ftp 2>/dev/null
    if [ $? -eq 0 ]; then
        subtest_ok 0 "Can load FTP helper"
    else
        subtest_skip "FTP helper not available"
    fi

    # 4. TFTP helper
    modprobe nf_conntrack_tftp 2>/dev/null
    if [ $? -eq 0 ]; then
        subtest_ok 0 "Can load TFTP helper"
    else
        subtest_skip "TFTP helper not available"
    fi

    # 5. SIP helper
    modprobe nf_conntrack_sip 2>/dev/null
    if [ $? -eq 0 ]; then
        subtest_ok 0 "Can load SIP helper"
    else
        subtest_skip "SIP helper not available"
    fi

    subtest_diag "Testing conntrack statistics..."

    # 6. Conntrack count readable
    if [ -f /proc/sys/net/netfilter/nf_conntrack_count ]; then
        subtest_ok 0 "Conntrack count readable"
    else
        subtest_skip "nf_conntrack_count not available"
    fi

    # 7. Conntrack max readable
    if [ -f /proc/sys/net/netfilter/nf_conntrack_max ]; then
        subtest_ok 0 "Conntrack max readable"
    else
        subtest_skip "nf_conntrack_max not available"
    fi

    # 8. Conntrack tool available
    if command -v conntrack >/dev/null 2>&1; then
        subtest_ok 0 "conntrack command available"
    else
        subtest_skip "conntrack not installed"
    fi

    subtest_end
}

# ============================================================================
# Subtest 5: QoS (from qos_test.sh)
# ============================================================================
run_qos_subtest() {
    subtest_start "QoS"
    subtest_plan 8

    # Check tc available
    if ! command -v tc >/dev/null 2>&1; then
        subtest_diag "tc not available, skipping"
        for i in 1 2 3 4 5 6 7 8; do
            subtest_skip "tc not available"
        done
        subtest_end
        return
    fi

    TEST_IFACE="lo"
    subtest_diag "Using interface: $TEST_IFACE"

    # Clear existing
    tc qdisc del dev $TEST_IFACE root 2>/dev/null

    # 1. Add HTB root qdisc
    tc qdisc add dev $TEST_IFACE root handle 1: htb default 99
    subtest_ok $? "Can add HTB root qdisc"

    # 2. Verify qdisc exists
    tc qdisc show dev $TEST_IFACE | grep -q "htb 1:"
    subtest_ok $? "HTB qdisc visible"

    # 3. Add root class
    tc class add dev $TEST_IFACE parent 1: classid 1:1 htb rate 100mbit ceil 100mbit
    subtest_ok $? "Can add HTB root class"

    # 4. Add child class
    tc class add dev $TEST_IFACE parent 1:1 classid 1:10 htb rate 50mbit ceil 100mbit prio 1
    subtest_ok $? "Can add child class"

    # 5. Add filter
    tc filter add dev $TEST_IFACE parent 1: protocol ip prio 1 u32 \
        match ip dport 22 0xffff match ip protocol 6 0xff flowid 1:10
    subtest_ok $? "Can add u32 filter"

    # 6. Add SFQ qdisc
    tc qdisc add dev $TEST_IFACE parent 1:10 handle 10: sfq perturb 10
    subtest_ok $? "Can add SFQ qdisc"

    # 7. Class statistics
    tc -s class show dev $TEST_IFACE | grep -q "Sent"
    subtest_ok $? "Can retrieve class statistics"

    # 8. Cleanup
    tc qdisc del dev $TEST_IFACE root
    subtest_ok $? "Can delete root qdisc"

    subtest_end
}

# ============================================================================
# Run All Subtests
# ============================================================================

diag "Running Combined Sanity Checks..."
diag ""

run_environment_subtest
run_health_subtest
run_validation_subtest
run_conntrack_subtest
run_qos_subtest

diag ""
diag "Combined Sanity Check Complete!"

if [ $failed_count -eq 0 ]; then
    exit 0
else
    exit 1
fi
