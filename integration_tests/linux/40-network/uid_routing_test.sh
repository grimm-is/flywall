#!/bin/sh
set -x

# UID Routing Test
# Verifies: UID routing generates correct nftables mark rules

TEST_TIMEOUT=30
. "$(dirname "$0")/../common.sh"

plan 4

require_root
require_linux
require_binary

cleanup_on_exit

diag "Test: UID Routing Configuration"

# Create test config with UID routing
TEST_CONFIG=$(mktemp_compatible uid_route.hcl)
cat > "$TEST_CONFIG" <<EOF
schema_version = "1.0"

interface "lo" {
  zone = "lan"
  ipv4 = ["127.0.0.1/8"]
}

interface "eth0" {
  zone = "wan"
  dhcp = true
  table = 10
}

zone "lan" {
  match {
    interface = "lo"
  }
}

zone "wan" {
  match {
    interface = "eth0"
  }
}

# UID routing: route user 65534 (nobody) via eth0 uplink
uid_routing "test_user" {
  uid = 65534
  uplink = "eth0"
  enabled = true
}
EOF

# Start control plane
diag "Starting control plane..."
start_ctl "$TEST_CONFIG"
ok 0 "Control plane started with UID routing config"

# Wait for rules to be applied
dilated_sleep 2

# Check if UID routing rule appears in ip rules
# Should see: from all uidrange 65534-65534 lookup <table_id>
IP_RULE_OUTPUT=$(ip rule show 2>/dev/null || echo "")
if echo "$IP_RULE_OUTPUT" | grep -q "uidrange 65534-65534"; then
    ok 0 "UID routing rule found in ip rules (uidrange 65534)"
else
    ok 1 "UID routing rule not found in ip rules"
    diag "Expected: from all uidrange 65534-65534 lookup ..."
    diag "IP Rule Output snippet:"
    echo "$IP_RULE_OUTPUT" | grep "uidrange" || echo "(no uid-related rules found)"
    echo "# DEBUG: Full IP Rules:"
    ip rule show
fi

# Verify output chain exists for connmark restore
if nft list chain inet flywall output >/dev/null 2>&1; then
    ok 0 "Output chain present for mark restore"
else
    ok 1 "Output chain missing"
fi

# Verify firewall table created
if nft list table inet flywall >/dev/null 2>&1; then
    ok 0 "Firewall table created"
else
    ok 1 "Firewall table missing"
fi

rm -f "$TEST_CONFIG"
exit 0
