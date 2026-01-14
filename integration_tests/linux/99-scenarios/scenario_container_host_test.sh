#!/bin/sh
set -x

# Scenario: Container Host
# Use Case: Running containers/VMs that need connectivity but should be isolated from Host sensitive services.
#
# Topology:
# [ Container NS ] --(veth)--> [ br0 (Flywall) ]
#
# Verifies:
# 1. Bridge creation
# 2. DHCP service on Bridge
# 3. Isolation: Container cannot access protected Host port
# 4. Connectivity: Container can ping Host bridge IP

. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

# Dependencies
if ! command -v udhcpc >/dev/null 2>&1 && ! command -v dhclient >/dev/null 2>&1; then
    echo "1..0 # SKIP No DHCP client found"
    exit 0
fi

# Cleanup
cleanup_scenario() {
    ip netns del container 2>/dev/null || true
    ip link del br0 2>/dev/null || true
    ip link del veth-br 2>/dev/null || true
    rm -f "$CONFIG_FILE"
    stop_ctl
    # Kill background listeners
    if [ -n "$NC_PID" ]; then kill $NC_PID 2>/dev/null; fi
}
trap cleanup_scenario EXIT

plan 4

# Setup Topology
# We create veth pair: veth-c (Container) <-> veth-br (Host)
# We DON'T create br0 manually; Flywall should create it if configured, or we create it?
# Flywall typically expects interfaces to exist. We'll create the bridge manually to simulate system setup.
ip link add name br0 type bridge
ip link set br0 up

ip netns add container
ip link add veth-c type veth peer name veth-br
ip link set veth-c netns container
ip link set veth-br master br0
ip link set veth-br up

# Config
CONFIG_FILE=$(mktemp_compatible scenario_container.hcl)
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

interface "br0" {
    ipv4 = ["172.16.10.1/24"]
    zone = "containers"
}

zone "containers" {
    interfaces = ["br0"]
}

dhcp {
    enabled = true
    scope "pkgs" {
        interface = "br0"
        range_start = "172.16.10.100"
        range_end = "172.16.10.200"
        router = "172.16.10.1"
    }
}

policy "containers" "local" {
    action = "accept" # Allow ping etc

    # Explicitly block access to secret port 9999
    rule "block_secret" {
        proto = "tcp"
        dest_port = 9999
        action = "drop"
    }
}

api {
    enabled = true
    listen = "0.0.0.0:8090"
}
EOF

# 1. Start Flywall
start_ctl "$CONFIG_FILE"

# Wait for bridge IP to be applied by Flywall
start_time=$(date +%s)
while ! ip addr show br0 | grep -q "172.16.10.1"; do
    if [ $(($(date +%s) - start_time)) -gt 10 ]; then
        fail "Bridge br0 did not get IP 172.16.10.1"
        exit 1
    fi
    sleep 0.5
done
ok 0 "Flywall configured bridge IP"

# 2. Container DHCP
diag "Starting DHCP in container..."
ip netns exec container ip link set lo up
ip netns exec container ip link set veth-c up

if command -v udhcpc >/dev/null 2>&1; then
    ip netns exec container udhcpc -i veth-c -n -q -f &
else
    ip netns exec container dhclient -v veth-c &
fi

# Wait for IP
count=0
CLIENT_IP=""
while [ $count -lt 30 ]; do
    CLIENT_IP=$(ip netns exec container ip addr show veth-c | grep "inet " | awk '{print $2}' | cut -d/ -f1)
    [ -n "$CLIENT_IP" ] && break
    sleep 1
    count=$((count+1))
done

if [ -n "$CLIENT_IP" ]; then
    ok 0 "Container received IP: $CLIENT_IP"
else
    fail "Container failed to get IP"
fi

# 3. Connectivity Check (Ping Host)
if ip netns exec container ping -c 1 -W 2 172.16.10.1 >/dev/null 2>&1; then
    pass "Container can ping Host"
else
    fail "Container cannot ping Host"
fi

# 4. Service Isolation Check
# Start a listener on Host port 9999 (simulating sensitive service)
diag "Starting listener on Host:9999..."
nc -l -k -p 9999 -s 172.16.10.1 > /dev/null 2>&1 &
NC_PID=$!
sleep 1

# Try to connect from container (Should Fail/Timeout)
diag "Attempting access to blocked port 9999..."
# Timeout 3s. If connection succeeds, it exits 0 immediately. If timeout, 124. If refused/drop, 1.
# We want it to FAIL to connect.
if ip netns exec container nc -z -w 2 172.16.10.1 9999; then
    fail "Container was able to access secret port 9999 (Should be BLOCKED)"
else
    pass "Container access to internal port 9999 blocked"
fi
