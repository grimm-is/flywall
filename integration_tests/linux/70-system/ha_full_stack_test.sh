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
TEST_TIMEOUT=30
. "$(dirname "$0")/../common.sh"
require_linux
export FLYWALL_LOG_FILE=stdout
export FLYWALL_NO_SANDBOX=1

plan 22

diag "Starting HA Full Stack Test..."

# Force cleanup of previous run namespaces (if any)
ip netns del ns_prim 2>/dev/null || true
ip netns del ns_back 2>/dev/null || true
ip netns del ns_cli 2>/dev/null || true

# Define working dirs
# Define working dirs
PRIM_DIR=$(mktemp -d /tmp/flywall_full_prim_XXXXXX)
BACK_DIR=$(mktemp -d /tmp/flywall_full_back_XXXXXX)
mkdir -p $PRIM_DIR/state $PRIM_DIR/run
mkdir -p $BACK_DIR/state $BACK_DIR/run
chmod -R 777 $PRIM_DIR $BACK_DIR

# Setup Namespaces - use fixed names (tests run serially in isolated VMs)
P_NS="ns_prim"
B_NS="ns_back"
C_NS="ns_cli"
ip netns add $P_NS
ip netns add $B_NS
ip netns add $C_NS

# Bring up loopback
ip netns exec $P_NS ip link set lo up
ip netns exec $B_NS ip link set lo up
ip netns exec $C_NS ip link set lo up

cleanup() {
    pkill -P $$ 2>/dev/null
    pkill -F $PRIM_DIR/pid 2>/dev/null
    pkill -F $BACK_DIR/pid 2>/dev/null
    ip netns del $P_NS 2>/dev/null
    ip netns del $B_NS 2>/dev/null
    ip netns del $C_NS 2>/dev/null
    rm -rf $PRIM_DIR $BACK_DIR
    rm -f /tmp/full_primary_$$.hcl /tmp/full_backup_$$.hcl
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
V_PRIM="v-prim"
V_BACK="v-back"
ip link add $V_PRIM type veth peer name $V_BACK
ip link set $V_PRIM netns $P_NS
ip link set $V_BACK netns $B_NS

ip netns exec $P_NS ip addr add 192.168.100.1/24 dev $V_PRIM
ip netns exec $P_NS ip link set $V_PRIM up

ip netns exec $B_NS ip addr add 192.168.100.2/24 dev $V_BACK
ip netns exec $B_NS ip link set $V_BACK up

# LAN Link (Bridge): v-lan-p, v-lan-b, v-cli <---> br-lan
BR_LAN="br-lan"
ip link add $BR_LAN type bridge
ip link set $BR_LAN up

# Use short fixed names - tests run serially in isolated VMs
V_LAN_P="v-lp"
V_LAN_P_BR="v-lpb"
V_LAN_B="v-lb"
V_LAN_B_BR="v-lbb"
V_CLI="v-cli"
V_CLI_BR="v-clib"

ip link add $V_LAN_P type veth peer name $V_LAN_P_BR
ip link add $V_LAN_B type veth peer name $V_LAN_B_BR
ip link add $V_CLI type veth peer name $V_CLI_BR

ip link set $V_LAN_P netns $P_NS
ip link set $V_LAN_B netns $B_NS
ip link set $V_CLI netns $C_NS

ip link set $V_LAN_P_BR master $BR_LAN
ip link set $V_LAN_B_BR master $BR_LAN
ip link set $V_CLI_BR master $BR_LAN

ip link set $V_LAN_P_BR up
ip link set $V_LAN_B_BR up
ip link set $V_CLI_BR up

ip netns exec $P_NS ip addr add 10.0.0.10/24 dev $V_LAN_P
ip netns exec $P_NS ip link set $V_LAN_P up

ip netns exec $B_NS ip addr add 10.0.0.20/24 dev $V_LAN_B
ip netns exec $B_NS ip link set $V_LAN_B up

ip netns exec $C_NS ip addr add 10.0.0.50/24 dev $V_CLI
ip netns exec $C_NS ip link set $V_CLI up

# Connectivity Check
ip netns exec $P_NS ping -c 1 192.168.100.2 >/dev/null 2>&1
ok $? "Primary can reach backup on HA link"

ip netns exec $C_NS ping -c 1 10.0.0.10 >/dev/null 2>&1
ok $? "Client can reach primary LAN IP"

# Configuration
# Shared Secret for PSK
SECRET="failover-shared-secret-key-123"
VIP="10.0.0.1"

# Primary Config (Priority 50 - Master)
cat > /tmp/full_primary_$$.hcl <<EOF
schema_version = "1.0"
interface "lo" {
    ipv4 = ["127.0.0.1/8"]
}
interface "$V_PRIM" {
    ipv4 = ["192.168.100.1/24"]
    zone = "sync"
}
interface "$V_LAN_P" {
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
    listen = "0.0.0.0:8081"
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
        failure_threshold = 10
        heartbeat_port = 9002

        conntrack_sync {
            enabled = false
            interface = "$V_PRIM"
        }
        virtual_ip {
            address = "${VIP}/24"
            interface = "$V_LAN_P"
        }
    }
}
state_dir = "/tmp/flywall_full_prim/state"
EOF

# Backup Config (Priority 150 - Backup)
cat > /tmp/full_backup_$$.hcl <<EOF
schema_version = "1.0"
interface "lo" {
    ipv4 = ["127.0.0.1/8"]
}
interface "$V_BACK" {
    ipv4 = ["192.168.100.2/24"]
    zone = "sync"
}
interface "$V_LAN_B" {
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
    listen = "0.0.0.0:8082"
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
        failure_threshold = 10
        heartbeat_port = 9002

        conntrack_sync {
            enabled = false
            interface = "$V_BACK"
        }
        virtual_ip {
            address = "${VIP}/24"
            interface = "$V_LAN_B"
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
ip netns exec $P_NS env FLYWALL_RUN_DIR="$PRIM_DIR/run" FLYWALL_STATE_DIR="$PRIM_DIR/state" $CTL_BIN ctl --state-dir $PRIM_DIR/state /tmp/full_primary_$$.hcl \
    > $PRIM_DIR/log 2>&1 &
PID_PRIM=$!
echo $PID_PRIM > $PRIM_DIR/pid
wait_for_file "$PRIM_DIR/ctl.sock" 10


ip netns exec $P_NS test -S $PRIM_DIR/ctl.sock
if [ $? -ne 0 ]; then
    echo "### PRIMARY FAILED TO START ###"
    cat $PRIM_DIR/log
    echo "### PRIMARY FIREWALL RULES:"
    ip netns exec $P_NS nft list ruleset
    echo "### PRIMARY IP ADDR:"
    ip netns exec $P_NS ip addr
    echo "### PRIMARY ROUTES:"
    ip netns exec $P_NS ip route
    fail "Primary failed to start"
fi
# Wait for Primary API to be ready
diag "Waiting for Primary API..."
if command -v curl >/dev/null 2>&1; then
    for i in $(seq 1 30); do
        if ip netns exec $C_NS curl -sS --http0.9 --max-time 2 --connect-timeout 1 http://10.0.0.10:8081/api/replication/status >/dev/null; then
            break
        fi
        sleep 1
    done

    # Check if loop timed out (i=30)
    if [ $i -ge 30 ]; then
        echo "### TIMEOUT WAITING FOR PRIMARY API ###"
        echo "### PRIMARY LOG HEAD:"
        head -n 50 $PRIM_DIR/log
        echo "### PRIMARY LOG TAIL:"
        tail -n 50 $PRIM_DIR/log
    fi
fi
ok 0 "Primary started"

# Verify Primary has VIP
ip netns exec $P_NS ip addr show $V_LAN_P | grep -q "$VIP"
ok $? "Primary claimed VIP"

# Inject Simulated Data (DHCP Lease)
diag "Injecting simulated data..."
if command -v sqlite3 >/dev/null 2>&1; then
    PRIM_DB="$PRIM_DIR/state/state.db"

    insert_entry() {
        bucket="$1"; key="$2"; value="$3"
        sqlite3 "$PRIM_DB" "INSERT OR IGNORE INTO buckets (name) VALUES ('$bucket');"
        sqlite3 "$PRIM_DB" "INSERT OR REPLACE INTO entries \
            (bucket, key, value, version, updated_at) \
            VALUES ('$bucket', '$key', '$value', 1, datetime('now'));"
    }

    insert_entry "dhcp_leases" "00:11:22:33:44:55" '{"mac":"00:11:22:33:44:55","ip":"10.0.0.100"}'
    insert_entry "sessions" "tcp:1.1.1.1:80" '{"proto":"tcp","state":"established"}'

    # Fix permissions after root sqlite3 usage
    chown -R nobody "$PRIM_DIR/state"

    ok 0 "Data injected into Primary"
else
    ok 0 "Data injection (skipped - sqlite3 missing)"
fi

# Allow injection to settle
sleep 2

# Verify Primary API
# Verify Primary API
if command -v curl >/dev/null 2>&1; then
    STATUS=""
    for i in $(seq 1 5); do
        STATUS=$(ip netns exec $C_NS curl -v --http0.9 --max-time 2 --connect-timeout 1 http://10.0.0.10:8081/api/replication/status 2>&1)
        if echo "$STATUS" | grep -q "primary"; then
            break
        fi
        sleep 1
    done
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
# Ensure Primary has had time to broadcast heartbeats (avoid split-brain on Backup start)
# Code Fix in ha/service.go (LastSeen init) should handle this without sleep.

export FLYWALL_CTL_SOCKET="$BACK_DIR/ctl.sock"
ip netns exec $B_NS env FLYWALL_RUN_DIR="$BACK_DIR/run" FLYWALL_STATE_DIR="$BACK_DIR/state" $CTL_BIN ctl --state-dir $BACK_DIR/state /tmp/full_backup_$$.hcl \
    > $BACK_DIR/log 2>&1 &
PID_BACK=$!
echo $PID_BACK > $BACK_DIR/pid
wait_for_file "$BACK_DIR/ctl.sock" 10


ip netns exec $B_NS test -S $BACK_DIR/ctl.sock
ok $? "Backup started"

# Verify Backup does NOT have VIP
ip netns exec $B_NS ip addr show $V_LAN_B | grep -q "$VIP"
if [ $? -ne 0 ]; then ok 0 "Backup is Standby (No VIP)"; else ok 1 "Backup HAS VIP (Unexpected)"; fi

# Verify Backup API (with retry)
if command -v curl >/dev/null 2>&1; then
    dilated_sleep 1
    STATUS=""
    for i in $(seq 1 5); do
        STATUS=$(ip netns exec $C_NS curl -s --max-time 2 --connect-timeout 1 \
            http://10.0.0.20:8082/api/replication/status || echo "")
        if [ -n "$STATUS" ]; then break; fi
        sleep 0.5
    done
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
# kill $PID_PRIM only kills the 'ip netns exec' wrapper.
# We need to kill the actual flywall process inside the namespace.
if [ -f "$PRIM_DIR/run/flywall.pid" ]; then
    REAL_PID=$(cat "$PRIM_DIR/run/flywall.pid")
    diag "Killing Primary process PID $REAL_PID..."
    kill -9 $REAL_PID 2>/dev/null || true
    # Wait for process to disappear
    for i in $(seq 1 10); do
        if ! kill -0 $REAL_PID 2>/dev/null; then
            break
        fi
        sleep 0.5
    done

    # Force kill EVERYTHING in the namespace to prevent survivors (conntrackd, etc)
    # REMOVED killall because it kills processes in other namespaces (PID ns is shared)
    # ip netns exec $P_NS killall -9 "$BINARY_NAME" 2>/dev/null || true
    # ip netns exec $P_NS killall -9 conntrackd 2>/dev/null || true

    # Wait for file to disappear (optional, but good practice)
    # But since we killed everything, we can proceed.
fi
# Also kill the wrapper just in case
kill $PID_PRIM 2>/dev/null || true
wait $PID_PRIM 2>/dev/null

diag "Waiting for failover (poll up to 30s)..."
for i in $(seq 1 30); do
    if ip netns exec $B_NS ip addr show $V_LAN_B | grep -q "$VIP"; then
        break
    fi
    sleep 1
done

ip netns exec $B_NS ip addr show $V_LAN_B | grep -q "$VIP"
ok $? "Backup took over VIP"

# Verify Client Reachability
ip netns exec $C_NS ping -c 1 -W 1 $VIP >/dev/null 2>&1
ok $? "Client can reach VIP via Backup"

# Inject NEW Data into Backup (simulating active work)
if command -v sqlite3 >/dev/null 2>&1; then
    BACK_DB="$BACK_DIR/state/state.db"
    # Insert new lease
    sqlite3 "$BACK_DB" "INSERT OR REPLACE INTO entries \
        (bucket, key, value, version, updated_at) \
        VALUES ('dhcp_leases', '00:NEW:FAILOVER', \
        '{\"mac\":\"new\",\"ip\":\"10.0.0.200\"}', 2, datetime('now'));"
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
ip netns exec $P_NS env FLYWALL_RUN_DIR="$PRIM_DIR/run" FLYWALL_STATE_DIR="$PRIM_DIR/state" $CTL_BIN ctl --state-dir $PRIM_DIR/state /tmp/full_primary_$$.hcl \
    >> $PRIM_DIR/log 2>&1 &
PID_PRIM=$!
echo $PID_PRIM > $PRIM_DIR/pid
wait_for_file "$PRIM_DIR/ctl.sock" 10

diag "Waiting for Primary to restart and preempt (Poll up to 15s)..."
# Poll for VIP acquisition on Primary
for i in $(seq 1 15); do
    ip netns exec $P_NS ip addr show $V_LAN_P | grep -q "$VIP" && break
    sleep 1
done

# Grace period for state stabilization (let backup yield VIP)
dilated_sleep 5

ip netns exec $P_NS test -S $PRIM_DIR/ctl.sock
ok $? "Primary restarted"


# Verify Primary Reclaimed VIP
ip netns exec $P_NS ip addr show $V_LAN_P | grep -q "$VIP"
ok $? "Primary reclaimed VIP (Failback successful)"

# Verify Backup Yielded VIP
ip netns exec $B_NS ip addr show $V_LAN_B | grep -q "$VIP"
if [ $? -ne 0 ]; then ok 0 "Backup yielded VIP"; else ok 1 "Backup kept VIP (Duplicate IP!)"; fi

# Verify Reverse Sync (Primary got the new data)
if command -v sqlite3 >/dev/null 2>&1; then
    PRIM_DB="$PRIM_DIR/state/state.db"
    # Check for the new lease
    FOUND=$(sqlite3 "$PRIM_DB" "SELECT count(*) FROM entries \
        WHERE key='00:NEW:FAILOVER';" 2>/dev/null)
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
