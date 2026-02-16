#!/bin/bash
set -e
source "$(dirname "$0")/../common.sh"

# Test: mDNS Repeater
# Topology:
#   ns1 (client1) -- veth1 <-> veth1_br -- br0 (Flywall) -- veth2_br <-> veth2 -- ns2 (client2)
#
# Goal: Verify mDNS broadcast from ns1 reaches ns2 via Flywall repeater.

TEST_NAME="mDNS Repeater"
setup_env() {
    ip netns add ns1
    ip netns add ns2
    
    # Create Bridge on Host (simulating Flywall interfaces)
    # Actually, Flywall runs on the host (or in a netns if we want full isolation).
    # Standard integration tests usually run Flywall in a separate namespace or use the host as the DUT?
    # Based on existing tests (e.g., mdns_test.sh), we seem to use `fw-test-vm` or namespaces.
    # Let's assume we are the host.
    
    # Create veth pairs
    ip link add veth1 type veth peer name veth1_host
    ip link add veth2 type veth peer name veth2_host
    
    # Connect to namespaces
    ip link set veth1 netns ns1
    ip link set veth2 netns ns2
    
    # Configure IPs
    ip netns exec ns1 ip addr add 192.168.10.2/24 dev veth1
    ip netns exec ns1 ip link set veth1 up
    ip netns exec ns1 ip link set lo up
    
    ip netns exec ns2 ip addr add 192.168.20.2/24 dev veth2
    ip netns exec ns2 ip link set veth2 up
    ip netns exec ns2 ip link set lo up

    # Add multicast route to namespaces
    ip netns exec ns1 ip route add 224.0.0.0/4 dev veth1 || true
    ip netns exec ns2 ip route add 224.0.0.0/4 dev veth2 || true
    
    # Configure Host IPs (Flywall)
    ip addr add 192.168.10.1/24 dev veth1_host
    ip link set veth1_host up
    
    ip addr add 192.168.20.1/24 dev veth2_host
    ip link set veth2_host up
    
    # Enable MULTICAST
    ip link set veth1_host multicast on
    ip link set veth2_host multicast on
}

cleanup_env() {
    ip link del veth1_host || true
    ip link del veth2_host || true
    ip netns del ns1 2>/dev/null || true
    ip netns del ns2 2>/dev/null || true
}

run_test() {
    echo "Starting test setup..."
    setup_env
    
    # Write Flywall Config
    cat > /tmp/flywall_mdns.hcl <<EOF
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
            port "mdns" {
                protocol = "udp"
                port = 5353
                 # mDNS uses 5353
            }
        }
    }

    policy "trusted" "trusted" {
        rule "accept_all" {
            action = "accept"
        }
    }
    
    mdns {
        enabled = true
        interfaces = ["veth1_host", "veth2_host"]
    }

    rule_learning {
        enabled = true
        log_group = 0
    }
EOF

    # Start Flywall
    echo "Starting Flywall..."
    # Assuming we can run `flywall` binary directly or via `go run`.
    # Using existing test pattern:
    # We might need to run it in background with coverage.
    # For now, let's assume we can use the build artifact.
    
    # Start Flywall using common helper
    start_ctl /tmp/flywall_mdns.hcl
    
    # Wait for startup
    sleep 5
    
    # Start mDNS Listener in ns2
    echo "Starting mDNS listener in ns2..."
    # We can use `tcpdump` or a small go tool.
    # Assuming we have `mdns-scan` or similar, or just grep tcpdump.
    
    # Start tcpdump in background on ns2
    ip netns exec ns2 tcpdump -i veth2 -n udp port 5353 -l > /tmp/ns2_mdns.log 2>&1 &
    TCPDUMP_PID=$!
    
    sleep 2
    
    # Announce from ns1
    echo "Announcing from ns1 (multiple times)..."
    for i in $(seq 1 3); do
        echo "Announcement $i..."
        ip netns exec ns1 sh -c 'printf "\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x05_http\x04_tcp\x05local\x00\x00\x0c\x00\x01" | nc -u -w 1 224.0.0.251 5353 || true'
        sleep 2
    done
    
    sleep 5
    
    kill $TCPDUMP_PID || true
    stop_ctl
    
    # Check log
    echo "Checking logs..."
    if grep -q "_http._tcp.local" /tmp/ns2_mdns.log; then
        echo "PASS: mDNS packet seen in ns2"
    else
        echo "FAIL: mDNS packet NOT seen in ns2"
        cat /tmp/ns2_mdns.log
        echo "--- Active NFT Ruleset ---"
        nft list ruleset
        exit 1
    fi
}

trap cleanup_env EXIT
run_test
