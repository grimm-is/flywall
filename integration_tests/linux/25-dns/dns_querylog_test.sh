#!/bin/sh
set -x

# DNS Query Logging Test
# Tests that DNS queries are correctly logged to the database:
# - Queries are resolved (regression test)
# - Queries appear in the query log via RPC/API

TEST_TIMEOUT=60

# Source common functions
. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

if ! command -v dig >/dev/null 2>&1; then
    echo "1..0 # SKIP dig command not found"
    exit 0
fi

if ! command -v curl >/dev/null 2>&1; then
    echo "1..0 # SKIP curl command not found"
    exit 0
fi

# --- Test Suite ---
plan 5

diag "================================================"
diag "DNS Query Logging Test"
diag "Tests persistence of queries to SQLite & RPC retrieval"
diag "================================================"

# --- Setup ---
TEST_CONFIG=$(mktemp_compatible "dns_querylog.hcl")
cat > "$TEST_CONFIG" << 'EOF'
schema_version = "1.1"

interface "lo" {
  zone = "local"
  ipv4 = ["127.0.0.1/8"]
}

zone "local" {
  interfaces = ["lo"]
  services {
    dns = true
  }
}

dns {
  mode = "forward"
  forwarders = ["8.8.8.8"]

  serve "local" {
    local_domain = "local"
    host "10.0.0.1" {
      hostnames = ["logtest.local"]
    }
  }
}

api {
  enabled = true
  listen = "0.0.0.0:8080"
  require_auth = false
}
EOF

ok 0 "Created DNS config"

# Test 1: Start system
diag "Starting system..."
start_ctl "$TEST_CONFIG"
start_api -listen :8080
ok 0 "System started with API"

# Wait for DNS binding
dilated_sleep 2

# Test 2: Perform a DNS query
diag "Performing DNS query for logtest.local..."
QUERY_RESULT=$(dig @127.0.0.1 -p 53 logtest.local A +short +timeout=5 2>&1)

if echo "$QUERY_RESULT" | grep -qE "^10\.0\.0\.1$"; then
    ok 0 "DNS query succeeded"
else
    _log=$(tail -n 20 "$CTL_LOG")
    ok 1 "DNS query succeeded" severity fail expected "10.0.0.1" actual "$QUERY_RESULT" log_tail "$_log"
fi

# Allow some time for async logging
dilated_sleep 2

# Test 3: Query the API for logs
diag "Querying API for DNS history..."
curl -s "http://127.0.0.1:8080/api/dns/queries?limit=10" > /tmp/dns_querylog_api.json

# Check if our query is in the log
if grep -q "logtest.local" /tmp/dns_querylog_api.json; then
    ok 0 "Query found in API logs"
else
    cat /tmp/dns_querylog_api.json
    ok 1 "Query found in API logs" severity fail error "logtest.local not found in logs"
fi

# Test 4: Check stats
diag "Querying API for DNS stats..."
curl -s "http://127.0.0.1:8080/api/dns/stats" > /tmp/dns_stats_api.json

# Should have at least 1 total query
TOTAL_QUERIES=$(grep -o '"total_queries":[0-9]*' /tmp/dns_stats_api.json | cut -d: -f2)
if [ -n "$TOTAL_QUERIES" ] && [ "$TOTAL_QUERIES" -ge 1 ]; then
    ok 0 "Stats reflect queries (Count: $TOTAL_QUERIES)"
else
    cat /tmp/dns_stats_api.json
    ok 1 "Stats reflect queries" severity fail
fi

diag "All tests completed"
