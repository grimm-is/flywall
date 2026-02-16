#!/bin/sh
set -x

# API Scanner Test
# Verifies the Network Scanner API endpoints (/api/scanner/network, /api/scanner/host, status, result)

. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit
TEST_TIMEOUT=60

if ! command -v curl >/dev/null 2>&1; then
    echo "1..0 # SKIP curl command not found"
    exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "1..0 # SKIP jq command not found"
    exit 0
fi

if ! command -v nmap >/dev/null 2>&1; then
    echo "1..0 # SKIP nmap command not found (required for scanner)"
    exit 0
fi

# 1. Setup Topology
# We need a peer to scan.
# [ WAN NS (10.200.200.2) ] <--(veth)--> [ Host eth1 (10.200.200.1) ]
# We will scan 10.200.200.0/24 from Host

plan 6

ip netns add wan
ip link add veth-host type veth peer name veth-wan
ip link set veth-wan netns wan

ip link set veth-host up
ip addr add 10.200.200.1/24 dev veth-host

ip netns exec wan ip link set lo up
ip netns exec wan ip link set veth-wan up
ip netns exec wan ip addr add 10.200.200.2/24 dev veth-wan
ip netns exec wan ip route add default via 10.200.200.1

# Start a service on the peer to detect ports
# Simple nc listener on TCP 8080 and 9090
ip netns exec wan nc -l -k -s 0.0.0.0 -p 8080 >/dev/null 2>&1 &
SRV_PID_1=$!
ip netns exec wan nc -l -k -s 0.0.0.0 -p 9090 >/dev/null 2>&1 &
SRV_PID_2=$!

# Register cleanup for PIDs
trap "ip netns del wan 2>/dev/null; kill $SRV_PID_1 $SRV_PID_2 2>/dev/null; stop_ctl; [ -n \"$API_PID\" ] && kill \$API_PID" EXIT

# 2. Configure Flywall
CONFIG_FILE=$(mktemp_compatible "scanner_test.hcl")
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"

api {
    enabled = true
    listen = "0.0.0.0:$TEST_API_PORT"
    require_auth = false
}

scanner {
    disable_rdns = true
}

interface "veth-host" {
    ipv4 = ["10.200.200.1/24"]
    zone = "lan"
}

zone "lan" {
    match {
        interface = "veth-host"
    }
}

# Allow everything for simplicity
policy "lan" "lan" {
    action = "accept"
}

# Allow traffic destinated to the firewall (Input) from LAN
policy "lan" "local" {
    action = "accept"
}

EOF

# Check connectivity BEFORE Flywall (ruled out setup issues)
diag "Test 0.1: Connectivity Check (Pre-Flywall)"
if ping -c 1 -W 1 10.200.200.2 >/dev/null; then
    ok 0 "Peer reachable via ICMP (Pre-Flywall)"
else
    ok 1 "Peer unreachable via ICMP (Pre-Flywall)" severity fail
    ip addr
    ip route
    ip neigh
fi

export FLYWALL_SKIP_API=1
start_ctl "$CONFIG_FILE"
export FLYWALL_NO_SANDBOX=1
start_api -listen :$TEST_API_PORT
dilated_sleep 3

API_URL="http://127.0.0.1:$TEST_API_PORT"

# Test 0.2: Verify Connectivity (Post-Flywall)
diag "Test 0.2: Connectivity Check (Post-Flywall)"
if nc -z -v -w 2 10.200.200.2 8080; then
    ok 0 "Connectivity to 10.200.200.2:8080 confirmed"
else
    ok 1 "Failed to connect to 10.200.200.2:8080" severity fail
    # Debug info
    ip addr
    ip route
    ip neigh
    nft list ruleset | head -n 50
fi

# Test 1: Start Network Scan
diag "Test 1: Start Network Scan (10.200.200.0/24)"
HTTP_CODE=$(curl -s -o /tmp/scan_start_$$.json -w "%{http_code}" -X POST "$API_URL/api/scanner/network" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-key-bypass-csrf" \
    -d '{"cidr":"10.200.200.0/24", "timeout_seconds": 15}')

if [ "$HTTP_CODE" -eq 202 ]; then
    ok 0 "Scan started successfully (202 Accepted)"
else
    cat /tmp/scan_start_$$.json
    ok 1 "Scan failed to start" severity fail expected "202" actual "$HTTP_CODE"
fi

# Test 2: Poll Status until Complete
diag "Test 2: Polling Scan Status..."
scanning="true"
attempts=0
while [ "$scanning" = "true" ] && [ $attempts -lt 40 ]; do
    dilated_sleep 1
    curl -s "$API_URL/api/scanner/status" > /tmp/scan_status_$$.json
    scanning=$(jq -r '.scanning' /tmp/scan_status_$$.json)
    attempts=$((attempts + 1))
done

if [ "$scanning" = "false" ]; then
    ok 0 "Scan completed"
else
    ok 1 "Scan timed out" severity fail
fi

# Test 3: Verify Scan Result (Network Summary)
diag "Test 3: Verify Result Summary"
curl -s "$API_URL/api/scanner/result" > /tmp/scan_result_$$.json
HOST_COUNT=$(jq '.hosts | length' /tmp/scan_result_$$.json)

# We expect at least the WAN peer (192.168.100.2).
if [ "$HOST_COUNT" -ge 1 ]; then
    ok 0 "Found $HOST_COUNT hosts (Expected >= 1)"
else
    cat /tmp/scan_result_$$.json
    ok 1 "Network scan found 0 hosts" severity fail
fi

# Test 4: Verify Host Details (Services)
diag "Test 4: Verify Peer Services (Port 8080)"
# Find the entry for 10.200.200.2
PEER_DATA=$(jq '.hosts[] | select(.ip == "10.200.200.2")' /tmp/scan_result_$$.json)

if [ -z "$PEER_DATA" ]; then
    ok 1 "Peer 10.200.200.2 not found in result" severity fail
else
    PORTS=$(echo "$PEER_DATA" | jq -r '.open_ports[].port')
    if echo "$PORTS" | grep -q "8080"; then
        ok 0 "Found open port 8080 on peer"
    else
        echo "Ports found: $PORTS"
        ok 1 "Missing expected tcp/8080 on peer" severity fail
    fi
fi

# Test 5: Single Host Scan Endpoint
diag "Test 5: Scan Single Host"
HTTP_CODE=$(curl -s -o /tmp/host_scan_$$.json -w "%{http_code}" -X POST "$API_URL/api/scanner/host" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-key-bypass-csrf" \
    -d '{"ip":"10.200.200.2"}')

if [ "$HTTP_CODE" -eq 200 ]; then
    PORTS=$(jq -r '.open_ports[].port' /tmp/host_scan_$$.json)
    if echo "$PORTS" | grep -q "8080"; then
        ok 0 "Single host scan successful (Found port 8080)"
    else
        ok 1 "Single host scan missing port 8080" severity fail
    fi
else
    ok 1 "Single host scan failed" severity fail expected "200" actual "$HTTP_CODE"
fi

# Test 6: Common Ports Endpoint
diag "Test 6: Get Common Ports"
HTTP_CODE=$(curl -s -o /tmp/ports_$$.json -w "%{http_code}" "$API_URL/api/scanner/ports")

if [ "$HTTP_CODE" -eq 200 ]; then
    COUNT=$(jq '. | length' /tmp/ports_$$.json)
    if [ "$COUNT" -gt 0 ]; then
        ok 0 "Common ports returned ($COUNT ports)"
    else
        ok 1 "Common ports list empty" severity fail
    fi
else
    ok 1 "Failed to get common ports" severity fail expected "200" actual "$HTTP_CODE"
fi
