#!/bin/bash
set -u

TRACE_WAS_ON=0
. "$(dirname "$0")/../common.sh"
require_binary

test_count=0
failed_count=0

run_subtest() {
    local test_name="$1"
    local config="$2"
    local patterns="$3"
    
    test_count=$((test_count + 1))
    local config_file=$(mktemp)
    cat > "$config_file" <<EOF
schema_version = "1.0"
$config
EOF
    
    local _output
    if ! _output=$($APP_BIN show "$config_file" 2>&1); then
        echo "not ok $test_count - $test_name"
        echo "  ---"
        echo "  message: \"flywall show failed\""
        printf '%s\n' "$_output" | sed 's/^/    /'
        echo "  ..."
        rm -f "$config_file"
        failed_count=$((failed_count + 1))
        return 1
    fi
    rm -f "$config_file"
    
    # Filter out DEBUG lines
    _output=$(printf '%s\n' "$_output" | grep -v "DEBUG:")
    
    local failed=0
    local fail_pat=""
    
    # Check each pattern line robustly as regex (case-insensitive)
    printf '%s\n' "$patterns" | while IFS= read -r pat || [ -n "$pat" ]; do
        [ -z "$pat" ] && continue
        if ! printf '%s\n' "$_output" | grep -qiE "$pat"; then
            printf '%s' "$pat" > /tmp/fail_pat.tmp
            exit 1
        fi
    done
    if [ $? -ne 0 ]; then
        failed=1
        fail_pat=$(cat /tmp/fail_pat.tmp 2>/dev/null || echo "unknown")
    fi
    
    if [ $failed -eq 0 ]; then
        echo "ok $test_count - $test_name"
        return 0
    else
        echo "not ok $test_count - $test_name"
        echo "  ---"
        echo "  message: \"Missing expected pattern\""
        echo "  pattern: \"$fail_pat\""
        echo "  output: |"
        printf '%s\n' "$_output" | head -n 500 | sed 's/^/    /'
        echo "  ..."
        failed_count=$((failed_count + 1))
        return 1
    fi
}

test_zone_match_src_network() {
    run_subtest "zone match - src network" \
'zone "lan" {
  interfaces = ["eth1"]
}
interface "eth1" {
  zone = "lan"
  ipv4 = ["192.168.1.1/24"]
}
policy "lan" "firewall" {
  action = "accept"
}' \
'policy_lan_firewall'
}

test_stats_counters_chain() {
    run_subtest "stats counters chain" \
'zone "lan" {
  interfaces = ["eth1"]
}
interface "eth1" {
  zone = "lan"
  ipv4 = ["192.168.1.1/24"]
}
policy "lan" "firewall" {
  action = "accept"
}' \
'flywall_stats
cnt_syn'
}

test_scheduled_rule_time() {
    run_subtest "scheduled rule - time matching" \
'zone "lan" {
  interfaces=["eth1"]
}
interface "eth1" {
  zone="lan"
  ipv4=["192.168.1.1/24"]
}
policy "lan" "firewall" {
  action = "accept"
  rule "late" {
    time_start = "22:00"
    time_end = "01:00"
    action = "accept"
  }
  rule "work" {
    time_start = "14:00"
    time_end = "21:00"
    action = "accept"
  }
  rule "night" {
    time_start = "04:00"
    time_end = "05:00"
    days = ["Tuesday"]
    action = "accept"
  }
  rule "week" {
     days = ["Monday", "Tuesday"]
     action = "drop"
     dest_port = 8888
  }
}' \
'hour.*0
hour.*22-23
hour.*14-20
hour.*4
dport 8888.*1, 2' 
}

test_scheduled_rule_timezone() {
    # Asia/Tokyo is UTC+9 (No DST)
    # 1. "afternoon": 14:00-15:00 Mon (JST) -> 05:00-06:00 Mon (UTC)
    # 2. "morning": 05:00-06:00 Wed (JST) -> 20:00-21:00 Tue (UTC) (Day Wrap)
    run_subtest "scheduled rule - timezone (Asia/Tokyo)" \
'system {
  timezone = "Asia/Tokyo"
}
zone "lan" {
  interfaces=["eth1"]
}
interface "eth1" {
  zone="lan"
  ipv4=["192.168.1.1/24"]
}
policy "lan" "firewall" {
  action = "accept"
  rule "afternoon" {
    time_start = "14:00"
    time_end = "15:00"
    days = ["Monday"]
    action = "accept"
  }
  rule "morning" {
    time_start = "05:00"
    time_end = "06:00"
    days = ["Wednesday"]
    action = "accept"
  }
}' \
'meta day . meta hour .* 1 . 5
meta day . meta hour .* 2 . 20'
}

# Run tests
echo "TAP version 13"
test_zone_match_src_network
test_stats_counters_chain
test_scheduled_rule_time
test_scheduled_rule_timezone

if [ $failed_count -eq 0 ]; then
    echo "All tests passed!"
    exit 0
else
    echo "$failed_count tests failed"
    exit 1
fi
