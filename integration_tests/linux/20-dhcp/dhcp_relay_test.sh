#!/bin/bash
set -e
source "$(dirname "$0")/../common.sh"

# Test: DHCP Relay
# Topology:
#   ns_client -- veth1 <-> veth1_br -- br0 (Flywall)
#   ns_upstream -- veth2 <-> veth2_br -- br0 (Flywall)
#
# Goal: Verify DHCP Request from ns_client is relayed to ns_upstream, and Reply is relayed back.

TEST_NAME="DHCP Relay"
setup_env() {
    ip netns add ns_client
    ip netns add ns_upstream
    
    # Client Link (eth0 in ns_client)
    ip link add veth1 type veth peer name veth1_host
    ip link set veth1 netns ns_client
    ip netns exec ns_client ip link set veth1 name eth0
    ip netns exec ns_client ip link set eth0 up
    
    # Upstream Link (eth0 in ns_upstream)
    ip link add veth2 type veth peer name veth2_host
    ip link set veth2 netns ns_upstream
    ip netns exec ns_upstream ip link set veth2 name eth0
    ip netns exec ns_upstream ip addr add 192.168.20.2/24 dev eth0
    ip netns exec ns_upstream ip link set eth0 up
    ip netns exec ns_upstream ip link set lo up
    ip netns exec ns_upstream ip route add default via 192.168.20.1
    
    # Host Links (Flywall)
    ip addr add 192.168.10.1/24 dev veth1_host
    ip link set veth1_host up
    
    ip addr add 192.168.20.1/24 dev veth2_host
    ip link set veth2_host up
    
    # Setup Upstream DHCP Server (dnsmasq)
    # We need a DHCP server listening on 192.168.20.2 that serves 192.168.10.0/24 (Subnet of client)
    # Dnsmasq can do this if we configure extensive log.
    
    # Create simple dnsmasq config
    # bind-interfaces is needed to ensure it only listens on eth0 in this namespace
    # no-ping skips the ICMP ping check (which would fail due to safe mode blocking forwarding)
    cat > /tmp/dnsmasq_upstream.conf <<EOF
interface=eth0
bind-interfaces
no-ping
dhcp-range=192.168.10.100,192.168.10.200,1h
dhcp-option=3,192.168.10.1
log-dhcp
EOF

    # Start dnsmasq in ns_upstream
    # Note: dnsmasq supports DHCP relay via GIADDR field
    ip netns exec ns_upstream dnsmasq -C /tmp/dnsmasq_upstream.conf -d > /tmp/dnsmasq.log 2>&1 &
    DNSMASQ_PID=$!




    # Start tcpdump on veth2_host (Upstream Link)
    tcpdump -i veth2_host -n -l -e > /tmp/tcpdump.log 2>&1 &
    TCPDUMP_PID=$!
}

cleanup_env() {
    kill $DNSMASQ_PID || true
    kill $TCPDUMP_PID || true
    stop_ctl
    ip link del veth1_host || true
    ip link del veth2_host || true
    ip netns del ns_client 2>/dev/null || true
    ip netns del ns_upstream 2>/dev/null || true
}

run_test() {
    echo "Starting test setup..."
    setup_env
    
    # Write Flywall Config (Relay)
    cat > /tmp/flywall_dhcp.hcl <<EOF
    ip_forwarding = true
    interface "veth1_host" {
        ipv4 = ["192.168.10.1/24"]
        zone = "trusted"
    }
    interface "veth2_host" {
        ipv4 = ["192.168.20.1/24"]
        zone = "trusted"
    }
    
    zone "trusted" {
        services {
            dhcp = true
        }
    }

    policy "trusted" "trusted" {
        rule "accept_all" {
            action = "accept"
        }
    }
    
    dhcp {
        enabled = true
        scope "client_net" {
            interface = "veth1_host"
            range_start = "192.168.10.100"
            range_end = "192.168.10.200"
            router = "192.168.10.1"
            relay_to = ["192.168.20.2"]
        }
    }
EOF

    # Start Flywall
    echo "Starting Flywall..."
    start_ctl /tmp/flywall_dhcp.hcl
    
    sleep 5
    
    # Run DHCP Client in ns_client
    echo "Starting DHCP client..."
    # using `dhclient` or `udhcpc`
    # checking output/IP assignment
    
    if command -v dhclient >/dev/null; then
        ip netns exec ns_client dhclient -v -d --no-pid eth0 > /tmp/dhclient.log 2>&1 &
        DHCLIENT_PID=$!
    elif command -v udhcpc >/dev/null; then
        ip netns exec ns_client udhcpc -i eth0 -f -v > /tmp/dhclient.log 2>&1 &
        DHCLIENT_PID=$!
    else
        echo "No dhcp client found"
        exit 1
    fi
    
    # Wait for lease
    echo "Waiting for lease (up to 20s)..."
    for i in $(seq 1 20); do
        if ip netns exec ns_client ip addr show eth0 | grep -q "192.168.10."; then
            echo "PASS: Client got IP address"
            ip netns exec ns_client ip addr show eth0
            kill $DHCLIENT_PID || true
            return 0
        fi
        echo "Attempt $i: No IP yet..."
        sleep 1
    done
    
    echo "FAIL: Client did not get IP"
    echo "--- Dnsmasq Log ---"
    cat /tmp/dnsmasq.log
    echo "--- Client Log ---"
    cat /tmp/dhclient.log
    echo "--- Control Plane Log ---"
    [ -f "$CTL_LOG" ] && cat "$CTL_LOG"
    echo "--- TCPDump Log ---"
    cat /tmp/tcpdump.log
    echo "--- Active NFT Ruleset ---"
    nft list ruleset
    kill $DHCLIENT_PID || true
    exit 1
}

trap cleanup_env EXIT
run_test
