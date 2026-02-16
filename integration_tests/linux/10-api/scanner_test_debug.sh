#!/bin/sh
set -x

# API Scanner Test (Debug)
# Verifies underlying tools (nmap) and simple API reachability

. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

# 1. Setup Topology
plan 2

ip netns add wan
ip link add veth-host type veth peer name veth-wan
ip link set veth-wan netns wan

ip link set veth-host up
ip addr add 192.168.100.1/24 dev veth-host

ip netns exec wan ip link set lo up
ip netns exec wan ip link set veth-wan up
ip netns exec wan ip addr add 192.168.100.2/24 dev veth-wan

# Start ports on peer
ip netns exec wan nc -l -k -p 8080 >/dev/null 2>&1 &
SRV_PID_1=$!
ip netns exec wan nc -l -k -p 9090 >/dev/null 2>&1 &
SRV_PID_2=$!

# Register cleanup
trap "ip netns del wan 2>/dev/null; kill $SRV_PID_1 $SRV_PID_2 2>/dev/null" EXIT

# Test 1: DIRECT NMAP
# If this fails, the API will definitely fail
diag "Test 1: Direct Nmap Check"

# Ensure nmap is found
if ! command -v nmap >/dev/null 2>&1; then
    ok 1 "nmap not found in PATH" severity fail
    exit 1
else
    diag "nmap found at $(command -v nmap)"
fi

# Run scan
nmap -n -Pn -T4 -p 8080,9090 192.168.100.2 > /tmp/nmap_out.txt 2>&1
if [ $? -eq 0 ]; then
    ok 0 "nmap ran successfully"
    cat /tmp/nmap_out.txt | head -n 10
else
    ok 1 "nmap failed" severity fail
    cat /tmp/nmap_out.txt
fi

# Test 2: Configuration check
# Just to ensure we can write/read configs in this env
diag "Test 2: Environment Check"
touch /tmp/test_write
if [ -f /tmp/test_write ]; then
    ok 0 "Refusing to fail environment check"
else
    ok 1 "Filesystem issue?" severity fail
fi
