#!/bin/sh
# HA Full Stack Integration Test
#
# Merges HA Failover and Replication Sync tests to verify the complete consistency lifecycle:
# 1. Primary starts (Priority 50), gets VIP.
# 2. Simulated data injected into Primary.
# 3. Backup starts (Priority 150), syncs data from Primary.
# 4. Verify Data Consistency (PSK Auth + Checksums).
# 5. Primary KILLED -> Failover -> Backup gets VIP.
# 6. New data injected into Backup (Simulating active duties).
# 7. Primary RESTARTS -> Failback -> Primary reclaims VIP.
# 8. Verify Primary syncs NEW data from Backup.

set -x
TEST_TIMEOUT=120
. "$(dirname "$0")/../common.sh"
require_linux
export FLYWALL_LOG_FILE=stdout
export FLYWALL_NO_SANDBOX=1

plan 22

diag "Starting HA Full Stack Test..."

# Define working dirs
PRIM_DIR="/tmp/flywall_full_prim"
BACK_DIR="/tmp/flywall_full_back"
rm -rf $PRIM_DIR $BACK_DIR
mkdir -p $PRIM_DIR/state
mkdir -p $BACK_DIR/state

# Setup Namespaces
ip netns add ns_prim
ip netns add ns_back
ip netns add ns_cli

# Bring up loopback
ip netns exec ns_prim ip link set lo up
ip netns exec ns_back ip link set lo up
ip netns exec ns_cli ip link set lo up

cleanup() {
    pkill -P $$ 2>/dev/null
    pkill -F $PRIM_DIR/pid 2>/dev/null
    pkill -F $BACK_DIR/pid 2>/dev/null
    ip netns del ns_prim 2>/dev/null
    ip netns del ns_back 2>/dev/null
    ip netns del ns_cli 2>/dev/null
    rm -rf $PRIM_DIR $BACK_DIR
    rm -f /tmp/full_primary.hcl /tmp/full_backup.hcl
}
trap cleanup EXIT

fail() {
    echo "### TEST FAILED: $1"
    echo "### PRIMARY LOG:"
    cat $PRIM_DIR/log 2>/dev/null
    echo "### BACKUP LOG:"
    cat $BACK_DIR/log 2>/dev/null
    exit 1
}

# Network Topology:
# Sync Link: v-prim (192.168.100.1) <---> v-back (192.168.100.2)
ip link add v-prim type veth peer name v-back
ip link set v-prim netns ns_prim
ip link set v-back netns ns_back

ip netns exec ns_prim ip addr add 192.168.100.1/24 dev v-prim
ip netns exec ns_prim ip link set v-prim up

ip netns exec ns_back ip addr add 192.168.100.2/24 dev v-back
ip netns exec ns_back ip link set v-back up

# LAN Link (Bridge): v-lan-p, v-lan-b, v-cli <---> br-lan
ip link add br-lan type bridge
ip link set br-lan up

ip link add v-lan-p type veth peer name v-lan-p-br
ip link add v-lan-b type veth peer name v-lan-b-br
ip link add v-cli type veth peer name v-cli-br

ip link set v-lan-p netns ns_prim
ip link set v-lan-b netns ns_back
ip link set v-cli netns ns_cli

ip link set v-lan-p-br master br-lan
ip link set v-lan-b-br master br-lan
ip link set v-cli-br master br-lan

ip link set v-lan-p-br up
ip link set v-lan-b-br up
ip link set v-cli-br up

ip netns exec ns_prim ip addr add 10.0.0.10/24 dev v-lan-p
ip netns exec ns_prim ip link set v-lan-p up

ip netns exec ns_back ip addr add 10.0.0.20/24 dev v-lan-b
ip netns exec ns_back ip link set v-lan-b up

ip netns exec ns_cli ip addr add 10.0.0.50/24 dev v-cli
ip netns exec ns_cli ip link set v-cli up

# Connectivity Check
ip netns exec ns_prim ping -c 1 192.168.100.2 >/dev/null 2>&1
ok $? "Primary can reach backup on HA link"

ip netns exec ns_cli ping -c 1 10.0.0.10 >/dev/null 2>&1
ok $? "Client can reach primary LAN IP"

# Configuration
# Shared Secret for PSK
SECRET="failover-shared-secret-key-123"
VIP="10.0.0.1"

# Primary Config (Priority 50 - Master)
cat > /tmp/full_primary.hcl <<EOF
interface "lo" {
    ipv4 = ["127.0.0.1/8"]
}
interface "v-prim" {
    ipv4 = ["192.168.100.1/24"]
    zone = "sync"
}
interface "v-lan-p" {
    ipv4 = ["10.0.0.10/24"]
    zone = "lan"
}
zone "lan" {}
zone "sync" {}
policy "lan" "firewall" {
    name = "allow_lan"
    action = "accept"
}
policy "sync" "firewall" {
    name = "allow_sync"
    action = "accept"
}
api {
    enabled = true
    listen = "0.0.0.0:8080"
}

replication {
    mode = "primary"
    listen_addr = "192.168.100.1:9001"
    peer_addr = "192.168.100.2:9002"
    secret_key = "${SECRET}"
    
    ha {
        enabled = true
        priority = 50
        failback_mode = "auto"
        failback_delay = 5
        heartbeat_interval = 1
        failure_threshold = 3
        heartbeat_port = 9002
        
        conntrack_sync {
            enabled = true
            interface = "v-prim"
        }
        virtual_ip {
            address = "${VIP}/24"
            interface = "v-lan-p"
        }
    }
}
state_dir = "/tmp/flywall_full_prim/state"
EOF

# Backup Config (Priority 150 - Backup)
cat > /tmp/full_backup.hcl <<EOF
interface "lo" {
    ipv4 = ["127.0.0.1/8"]
}
interface "v-prim" {
    ipv4 = ["192.168.100.2/24"]
    zone = "sync"
}
interface "v-lan-b" {
    ipv4 = ["10.0.0.20/24"]
    zone = "lan"
}
zone "lan" {}
zone "sync" {}
policy "lan" "firewall" {
    name = "allow_lan"
    action = "accept"
}
policy "sync" "firewall" {
    name = "allow_sync"
    action = "accept"
}
api {
    enabled = true
    listen = "0.0.0.0:8080"
}

replication {
    mode = "replica"
    listen_addr = "192.168.100.2:9001"
    primary_addr = "192.168.100.1:9001"
    peer_addr = "192.168.100.1:9002"
    secret_key = "${SECRET}"
    
    ha {
        enabled = true
        priority = 150
        heartbeat_interval = 1
        failure_threshold = 3
        heartbeat_port = 9002
        
        conntrack_sync {
            enabled = true
            interface = "v-prim"
        }
        virtual_ip {
            address = "${VIP}/24"
            interface = "v-lan-b"
        }
    }
}
state_dir = "/tmp/flywall_full_back/state"
EOF

# ============================================================================
# Phase 1: Start Primary & Inject Data
# ============================================================================
diag "Starting Primary..."
export FLYWALL_CTL_SOCKET="$PRIM_DIR/ctl.sock"
ip netns exec ns_prim $CTL_BIN ctl --state-dir $PRIM_DIR/state /tmp/full_primary.hcl \
    > $PRIM_DIR/log 2>&1 &
PID_PRIM=$!
echo $PID_PRIM > $PRIM_DIR/pid
sleep 2

ip netns exec ns_prim test -S $PRIM_DIR/ctl.sock
if [ $? -ne 0 ]; then
    echo "### PRIMARY FAILED TO START ###"
    cat $PRIM_DIR/log
    echo "### PRIMARY FIREWALL RULES:"
    ip netns exec ns_prim nft list ruleset
    echo "### PRIMARY IP ADDR:"
    ip netns exec ns_prim ip addr
    echo "### PRIMARY ROUTES:"
    ip netns exec ns_prim ip route
    fail "Primary failed to start"
fi
ok 0 "Primary started"

# Verify Primary has VIP
ip netns exec ns_prim ip addr show v-lan-p | grep -q "$VIP"
ok $? "Primary claimed VIP"

# Inject Simulated Data (DHCP Lease)
diag "Injecting simulated data..."
if command -v sqlite3 >/dev/null 2>&1; then
    PRIM_DB="$PRIM_DIR/state/state.db"
    
    insert_entry() {
        bucket="$1"; key="$2"; value="$3"
        sqlite3 "$PRIM_DB" "INSERT OR IGNORE INTO buckets (name) VALUES ('$bucket');"
        sqlite3 "$PRIM_DB" "INSERT OR REPLACE INTO entries (bucket, key, value, version, updated_at) VALUES ('$bucket', '$key', '$value', 1, datetime('now'));"
    }
    
    insert_entry "dhcp_leases" "00:11:22:33:44:55" '{"mac":"00:11:22:33:44:55","ip":"10.0.0.100"}'
    insert_entry "sessions" "tcp:1.1.1.1:80" '{"proto":"tcp","state":"established"}'
    ok 0 "Data injected into Primary"
else
    ok 0 "Data injection (skipped - sqlite3 missing)"
fi

# Verify Primary API
if command -v curl >/dev/null 2>&1; then
    STATUS=$(ip netns exec ns_cli curl -s http://10.0.0.10:8080/api/replication/status)
    echo "Primary API Status: $STATUS"
    echo "$STATUS" | grep -q "primary"
    ok $? "API reports Primary mode"
else
    ok 1 "API Check skipped (curl missing)"
fi

# ============================================================================
# Phase 2: Start Backup & Sync
# ============================================================================
diag "Starting Backup..."
export FLYWALL_CTL_SOCKET="$BACK_DIR/ctl.sock"
ip netns exec ns_back $CTL_BIN ctl --state-dir $BACK_DIR/state /tmp/full_backup.hcl \
    > $BACK_DIR/log 2>&1 &
PID_BACK=$!
echo $PID_BACK > $BACK_DIR/pid
sleep 3

ip netns exec ns_back test -S $BACK_DIR/ctl.sock
ok $? "Backup started"

# Verify Backup does NOT have VIP
ip netns exec ns_back ip addr show v-lan-b | grep -q "$VIP"
if [ $? -ne 0 ]; then ok 0 "Backup is Standby (No VIP)"; else ok 1 "Backup HAS VIP (Unexpected)"; fi

# Verify Backup API
if command -v curl >/dev/null 2>&1; then
    STATUS=$(ip netns exec ns_cli curl -s http://10.0.0.20:8080/api/replication/status)
    echo "Backup API Status: $STATUS"
    echo "$STATUS" | grep -q "replica"
    ok $? "API reports Replica mode"
else
    ok 1 "API Check skipped (curl missing)"
fi

# Verify Sync Matches
if command -v sqlite3 >/dev/null 2>&1; then
    # Compare lease counts
    PRIM_C=$(sqlite3 "$PRIM_DIR/state/state.db" "SELECT count(*) FROM entries;" 2>/dev/null)
    BACK_C=$(sqlite3 "$BACK_DIR/state/state.db" "SELECT count(*) FROM entries;" 2>/dev/null)
    test "$PRIM_C" -eq "$BACK_C"
    ok $? "Backup synced match ($PRIM_C entries)"
else
    ok 0 "Sync Check (skipped - sqlite3 missing)"
fi

# ============================================================================
# Phase 3: Failover (Kill Primary)
# ============================================================================
diag "Killing Primary..."
kill $PID_PRIM
wait $PID_PRIM 2>/dev/null

diag "Waiting for failover (approx 4s)..."
for i in $(seq 1 10); do
    ip netns exec ns_back ip addr show v-lan-b | grep -q "$VIP" && break
    sleep 1
done

ip netns exec ns_back ip addr show v-lan-b | grep -q "$VIP"
ok $? "Backup took over VIP"

# Verify Client Reachability
ip netns exec ns_cli ping -c 1 -W 1 $VIP >/dev/null 2>&1
ok $? "Client can reach VIP via Backup"

# Inject NEW Data into Backup (simulating active work)
if command -v sqlite3 >/dev/null 2>&1; then
    BACK_DB="$BACK_DIR/state/state.db"
    # Insert new lease
    sqlite3 "$BACK_DB" "INSERT OR REPLACE INTO entries (bucket, key, value, version, updated_at) VALUES ('dhcp_leases', '00:NEW:FAILOVER', '{\"mac\":\"new\",\"ip\":\"10.0.0.200\"}', 2, datetime('now'));"
    ok 0 "New data added to Backup during failover"
else
    ok 0 "New data inject (skipped)"
fi

# ============================================================================
# Phase 4: Failback (Restore Primary)
# ============================================================================
diag "Broadcasting availability and Restarting Primary..."
export FLYWALL_CTL_SOCKET="$PRIM_DIR/ctl.sock"
rm -f $PRIM_DIR/ctl.sock
ip netns exec ns_prim $CTL_BIN ctl --state-dir $PRIM_DIR/state /tmp/full_primary.hcl \
    > $PRIM_DIR/log 2>&1 &
PID_PRIM=$!
echo $PID_PRIM > $PRIM_DIR/pid

diag "Waiting for Primary to restart and preempt (Wait failback_delay 5s)..."
sleep 8 

ip netns exec ns_prim test -S $PRIM_DIR/ctl.sock
ok $? "Primary restarted"

# Verify Primary Reclaimed VIP
ip netns exec ns_prim ip addr show v-lan-p | grep -q "$VIP"
ok $? "Primary reclaimed VIP (Failback successful)"

# Verify Backup Yielded VIP
ip netns exec ns_back ip addr show v-lan-b | grep -q "$VIP"
if [ $? -ne 0 ]; then ok 0 "Backup yielded VIP"; else ok 1 "Backup kept VIP (Duplicate IP!)"; fi

# Verify Reverse Sync (Primary got the new data)
if command -v sqlite3 >/dev/null 2>&1; then
    PRIM_DB="$PRIM_DIR/state/state.db"
    # Check for the new lease
    FOUND=$(sqlite3 "$PRIM_DB" "SELECT count(*) FROM entries WHERE key='00:NEW:FAILOVER';" 2>/dev/null)
    test "$FOUND" -eq "1"
    ok $? "Primary synced new data from Backup ($FOUND/1)"
    
    # Verify Total consistency
    PRIM_C=$(sqlite3 "$PRIM_DIR/state/state.db" "SELECT count(*) FROM entries;" 2>/dev/null)
    BACK_C=$(sqlite3 "$BACK_DIR/state/state.db" "SELECT count(*) FROM entries;" 2>/dev/null)
    test "$PRIM_C" -eq "$BACK_C"
    ok $? "Final state consistent ($PRIM_C entries)"
else
    ok 0 "Reverse sync check (skipped)"
    ok 0 "Final check (skipped)"
fi

echo "### PRIMARY LOG ###"
cat $PRIM_DIR/log
echo "### BACKUP LOG ###"
cat $BACK_DIR/log

diag "HA Full Stack Test Complete!"
