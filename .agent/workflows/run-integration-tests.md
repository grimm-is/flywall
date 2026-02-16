---
description: How to run integration tests using the VM orchestrator
---

# Running Integration Tests

**NEVER** run integration test scripts directly (e.g., `sudo integration_tests/...`). They must be run through the VM orchestrator using the `flywall.sh` script.

## running specific integration tests

To run specific integration tests, use the `test int` command followed by the test names (fuzzy matched).

```bash
./flywall.sh test int <test_name_1> <test_name_2> ...
```

### Examples

Run mDNS and Protection Traffic tests:
```bash
./flywall.sh test int mdns protection_traffic
```

Run all DHCP related tests:
```bash
./flywall.sh test int dhcp
```

## Running all integration tests

To run the full suite:

```bash
./flywall.sh test int
```
