# 01-sanity: VM Environment Validation

These tests verify that the test VM environment has the required capabilities before running Flywall application tests. They do **not** test Flywall functionality.

## Purpose

Sanity tests check:
- **Kernel modules**: nftables, wireguard, etc.
- **Network interfaces**: eth0, eth1, eth2 present
- **Essential tools**: ip, curl/wget, netstat, nc, ss
- **Service infrastructure**: DHCP server/client binaries available
- **DNS tooling**: dig/nslookup available

## When to Run

These should run:
1. **First**, before any Flywall tests (hence the `01-` prefix)
2. **After VM rebuild**, to validate the new image

If these tests fail, fix the VM image—not Flywall code.

## Naming Convention

Tests here are named for the *capability* they check (e.g., `dhcp_test.sh` checks DHCP infrastructure availability), not Flywall features. This differs from tests in `20-dhcp/` which test Flywall's DHCP service.
