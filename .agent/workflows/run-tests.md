---
description: How to run and debug integration tests
---

# Running Integration Tests

Integration tests run in a QEMU VM with a real Linux kernel.

## Quick Start

```bash
# Full Integration Test Run (Recommended before commit)
# This runs all tests in parallel and takes < 1 minute.
./flywall.sh test int
```

# Run a specific test file
./flywall.sh test int integration_tests/linux/10-api/api_auth_test.sh

# Run tests matching a pattern
./flywall.sh test int 30-firewall
```

## Test Organization

Tests are in `integration_tests/linux/` directory, organized by category:

| Directory | Purpose |
|-----------|---------|
| `00-sanity/` | Basic connectivity, sanity checks |
| `05-golang/` | Unit tests running in VM environment |
| `10-api/` | API authentication, CRUD operations |
| `12-profiling/` | Performance and resource profiling |
| `20-dhcp/` | DHCP server functionality |
| `25-dns/` | DNS server functionality |
| `30-firewall/` | Firewall rules, zones, policies |
| `40-network/` | VLANs, bonds, NAT, routing |
| `50-security/` | IPSets, learning engine, threat intel |
| `60-vpn/` | WireGuard, Tailscale |
| `65-qos/` | Traffic shaping and QoS policies |
| `70-system/` | Config, upgrade, lifecycle |
| `80-monitoring/` | Metrics, nflog |
| `80-upgrade/` | Dedicated upgrade and migration tests |
| `90-cli/` | CLI commands |
| `99-enforcement/` | Strict enforcement validation |
| `99-scenarios/` | End-to-end user scenarios |

## Writing a New Test

Tests are shell scripts using TAP-style output with helpers from `common.sh`:

```bash
#!/bin/sh

# Test description (used by orchestrator)
# TEST_TIMEOUT: Override default timeout if needed
TEST_TIMEOUT=60

. "$(dirname "$0")/../common.sh"

require_root
require_binary
cleanup_on_exit

log() { echo "[TEST] $1"; }

# --- Setup ---
CONFIG_FILE="/tmp/test.hcl"
cat > "$CONFIG_FILE" <<EOF
schema_version = "1.0"
# ... test config ...
EOF

# Start services
log "Starting Control Plane..."
$APP_BIN ctl "$CONFIG_FILE" > /tmp/ctl.log 2>&1 &
CTL_PID=$!
track_pid $CTL_PID

wait_for_file $CTL_SOCKET 5 || fail "Control plane socket not created"

# --- Test Cases ---
log "Testing feature X..."
result=$(some_command)
if [ "$result" = "expected" ]; then
    log "PASS: Feature X works"
else
    fail "FAIL: Feature X returned $result"
fi

# --- Cleanup (automatic via cleanup_on_exit) ---
rm -f "$CONFIG_FILE"
exit 0
```

## Common Test Helpers

From `integration_tests/linux/common.sh`:

```bash
# Setup
require_root           # Fail if not root
require_binary         # Ensure flywall binary exists
cleanup_on_exit        # Auto-cleanup on exit

# Process management
track_pid $PID         # Track for cleanup
wait_for_file PATH SEC # Wait for file to exist
wait_for_port PORT SEC # Wait for port to open
wait_for_api           # Wait for API to be ready

# Assertions
fail "message"         # Log failure and exit 1
log "message"          # Log test progress

# API helpers
api_get "/path"                    # GET with auth
api_post "/path" '{"json":true}'   # POST with auth
```

## Debugging Failed Tests

### 1. Run with Verbose Output

```bash
./flywall.sh test int integration_tests/linux/10-api/api_auth_test.sh --verbose
```

### 2. Keep VM Running

```bash
# Start VM manually
./flywall.sh vm start

# SSH into VM
ssh -p 2222 root@localhost

# Run test manually inside VM
/mnt/flywall/integration_tests/linux/10-api/api_auth_test.sh
```

### 3. Check Logs

Inside VM:
```bash
# Flywall logs
journalctl -u flywall

# Check firewall rules
nft list ruleset

# Check interfaces
ip addr
```

### 4. Test API Manually

```bash
# Get API key
cat /var/lib/flywall/auth.json

# Call API
curl -s http://localhost:8080/api/status \
    -H "Authorization: Bearer <api-key>"
```

## Test Environment Variables

| Variable | Purpose |
|----------|---------|
| `API_URL` | API base URL (default: http://localhost:8080) |
| `API_KEY` | Authentication key |
| `VERBOSE` | Enable verbose output |
| `KEEP_VM` | Don't stop VM after tests |

## Common Issues

### "API server unreachable"

```bash
# Check if ctl and api are running
ps aux | grep flywall

# Check socket
ls -la /run/flywall/

# Wait longer for startup
sleep 10
```

### "Permission denied"

```bash
# Tests must run as root in VM
sudo -i
```

### "Test timeout"

- Increase timeout in test
- Check if command is hanging (use `timeout` wrapper)
