---
name: Flywall Test Debugging
description: Guide for running, debugging, and writing tests in the Flywall repository using the `fw` tool.
---

# Flywall Test Debugging Skill

This skill provides instructions for running and debugging tests in the Flywall repository. The project uses a custom test runner called `orca` wrapped by the `fw` CLI tool, which executes tests inside a Linux VM for isolation.

## Running Tests

### Integration Tests
Run integration tests using the `fw` tool. You can pass multiple test names to run them in a single batch.

```bash
fw test int [test_name_1] [test_name_2] ...
```

**Examples:**
- Run all: `fw test int`
- Run specific: `fw test int port_scan analytics unit`
- Run unit tests (VM-based): `fw test int unit`

### Unit Tests (Go)
Standard Go unit tests can be run locally if they don't require Linux-specific features (netns, iptables).
```bash
go test ./internal/...
```
For Linux-specific unit tests, use the integration test wrapper: `fw test int unit`.

## Test Environment (VM)

Integration tests run in a QEMU VM. The environment provides:
- **Root Access**: Tests run as root.
- **Network Namespaces**: `ip netns` is available.
- **File System**:
  - `/mnt/flywall`: Read-only source.
  - `/mnt/build`: Read-only binaries.
  - `/mnt/worker`: Writable scratch space (mapped to host per-worker).

## Common Pitfalls & Fixes

### 1. Environment Variables
The `common.sh` script sets up the environment. Key variables:
> [!IMPORTANT]
> **Regression Testing**: `common.sh` is critical infrastructure. Any change to it affects ALL tests. You MUST run the full suite (`fw test int`) after modifying `common.sh`.

- `FLYWALL_RUN_DIR`: Directory for sockets/PIDs. **Must be unique per test** to avoid collisions.
  - *Fix*: Use `FLYWALL_RUN_DIR=$RUN_DIR` (set by `common.sh` using `TEST_PID`).
- `FLYWALL_CTL_SOCKET`: Path to control socket.
  - *Fix*: If overriding `RunDir`, EXPLICITLY override `FLYWALL_CTL_SOCKET` too.
  - `export FLYWALL_CTL_SOCKET="$FLYWALL_RUN_DIR/ctl.sock"`
- `FLYWALL_SKIP_API=1`: Prevents the control plane from automatically spawning an API server.
  - *Use Case*: When the test script wants to manage the API server itself (e.g., `port_scan_test.sh`).

### 2. Interface Naming
Linux interfaces have a 15-character limit (`IFNAMSIZ`).
- *Problem*: `v-heartbeat-primary` (19 chars) -> Truncated -> Failure.
- *Fix*: Use short, unique names: `v-hb-p-${TEST_PID}`.

### 3. Build Failures (Linux vs Darwin)
If you see variable errors in `_linux.go` files:
- *Cause*: `cmd/flywall` imports `internal/brand`, but Linux logic might import it in a way that creates unused imports if not careful with build tags.
- *Check*: `GOOS=linux go build ./...` locally to verify fixes.

### 4. Flaky Tests (Race Conditions)
- *Problem*: Tests failing randomly.
- *Fixes*:
  - Use `wait_for_file` or `wait_for_log_entry` instead of `sleep`.
  - Ensure sockets are ready before connecting (`wait_for_file "$SOCK"`).
  - Use `dilated_sleep` for time-sensitive waits (scales with VM load).

## Debugging

1. **Logs**: Test logs are saved to `build/test-results/<test_category>/<test_name>/<timestamp>.log`.
   - Use `view_file` to inspect failure details.
2. **Interactive Debugging**:
   - Currently not supported easily in CI/VM. Rely on extensive logging (`set -x` in shell scripts).

## Creating New Tests

1. Create a `.sh` file in `integration_tests/linux/<category>/`.
2. Source `common.sh`: `. "$(dirname "$0")/../common.sh"`.
3. Call `plan [N]` with number of tests.
4. Use `ok`, `is`, `diag` helpers (TAP protocol).
5. Register cleanup: `cleanup_on_exit`.
