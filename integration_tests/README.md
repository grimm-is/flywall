# Integration Testing Guidelines

## Fundamental Architecture
**ALL integration tests run inside Virtual Machines.**

Do NOT try to run tests directly on the host (e.g., `./integration_tests/linux/foo.sh`).
ALWAYS use the project wrapper:
```bash
./flywall.sh test int [test_name]
```
This ensures:
1.  Root privileges (inside VM)
2.  Isolated network namespace
3.  Correct kernel modules (WireGuard, NFTables)
4.  No pollution of the developer's machine

## Running Tests
-   **Single Test**: `./flywall.sh test int persistence_test`
-   **Suite**: `./flywall.sh test int`
-   **Debug Mode**: Use `wait_for_log_entry` instead of `sleep` where possible.
