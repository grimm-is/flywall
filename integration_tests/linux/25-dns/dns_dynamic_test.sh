#!/bin/sh
set -x
#
# DNS Dynamic Updates Integration Test
# Verifies DHCP hostnames are added to DNS
#

. "$(dirname "$0")/../common.sh"


# Toplogy:
# [ client ] <--> [ Flywall ]
# client requests DHCP with hostname -> checks if Flywall DNS resolves it

require_root

# Check for DHCP client
DHCP_CLIENT=""
if command -v udhcpc >/dev/null 2>&1; then
    DHCP_CLIENT="udhcpc"
elif command -v dhclient >/dev/null 2>&1; then
    DHCP_CLIENT="dhclient"
else
    echo "1..0 # SKIP No DHCP client (udhcpc or dhclient) found"
    exit 0
fi

if ! command -v dig >/dev/null 2>&1; then
    echo "1..0 # SKIP dig not found"
    exit 0
fi

cleanup_dns() {
    ip netns del client-dns 2>/dev/null || true
    ip link del veth-dns 2>/dev/null || true
    rm -f "$CONFIG_FILE"
    stop_ctl
}
trap cleanup_dns EXIT

plan 3

# Setup Topology
ip netns add client-dns
ip link add veth-dns type veth peer name veth-client
ip link set veth-client netns client-dns
ip link set veth-dns up
ip addr add 192.168.50.1/24 dev veth-dns

# Create Config matching topology
CONFIG_FILE=$(mktemp_compatible dns_dynamic.hcl)
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

interface "veth-dns" {
    zone = "lan"
    ipv4 = ["192.168.50.1/24"]
}

zone "lan" {
    match {
        interface = "veth-dns"
    }
}

dhcp {
    enabled = true
    scope "test" {
        interface = "veth-dns"
        range_start = "192.168.50.100"
        range_end = "192.168.50.200"
        router = "192.168.50.1"
        domain = "test.lan"
    }
}

dns {
    forwarders = ["1.1.1.1"]
    serve "lan" {
        # DNS listens on zone interfaces automatically
        listen_port = 53
        local_domain = "test.lan"
        dhcp_integration = true
    }
}

api {
    enabled = true
    listen = "0.0.0.0:8089"
}
EOF

# 1. Start Control Plane
start_ctl "$CONFIG_FILE"
ok 0 "Control plane started"

# 2. DHCP Loop
diag "Starting DHCP client..."
ip netns exec client-dns ip link set lo up
ip netns exec client-dns ip link set veth-client up

HOSTNAME="myhost"
EXPECTED_FQDN="myhost.test.lan"

if [ "$DHCP_CLIENT" = "udhcpc" ]; then
    ip netns exec client-dns udhcpc -i veth-client -x hostname:$HOSTNAME -n -q -f &
else
    ip netns exec client-dns dhclient -v veth-client -H $HOSTNAME &
fi

# Wait for lease
diag "Waiting for IP assignment..."
count=0
CLIENT_IP=""
while [ $count -lt 30 ]; do
    CLIENT_IP=$(ip netns exec client-dns ip addr show veth-client | grep "inet " | awk '{print $2}' | cut -d/ -f1)
    [ -n "$CLIENT_IP" ] && break
    sleep 1
    count=$((count+1))
done

if [ -n "$CLIENT_IP" ]; then
    ok 0 "Client got IP: $CLIENT_IP"
else
    fail "Client failed to get IP"
fi

# 3. DNS Verification
diag "Verifying DNS resolution for $EXPECTED_FQDN..."
dilated_sleep 2 # Allow propagation

# Query localhost or interface IP. Since we listen on interface IP 192.168.50.1:
# We can query from host or client.
DNS_RES=$(dig @192.168.50.1 -p 53 $EXPECTED_FQDN +short +timeout=5)

if [ "$DNS_RES" = "$CLIENT_IP" ]; then
    pass "DNS resolved $EXPECTED_FQDN -> $CLIENT_IP"
else
    fail "DNS resolution failed: expected $CLIENT_IP, got '$DNS_RES'"
    dig @192.168.50.1 -p 53 $EXPECTED_FQDN
fi

# Cleanup handled by trap
