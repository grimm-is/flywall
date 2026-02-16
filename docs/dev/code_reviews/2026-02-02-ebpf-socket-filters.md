# eBPF Socket Filters Code Review

**Scope**: Changeset `49dc6ab6..HEAD` (including working tree)
**Date**: 2026-02-02
**Reviewer**: Antigravity

## Executive Summary
The review covers the Phase 4 ("Socket Filters") implementation. While the C code structure for DHCP and DNS monitoring is present, the implementation contains **critical architectural flaws** that render the usage of eBPF maps for state tracking impossible. Furthermore, the Go userspace implementation is currently a skeleton, contradicting status reports claiming completion. 

**UPDATE**: The TLS socket filter (`tls_socket.c`) has been removed as it violates ADR-001 (Hybrid Architecture), which assigns TLS fingerprinting to the NFQUEUE layer, not eBPF socket filters.

## Critical Issues 🔴

### 1. Map Key Design Flaw (Append-Only Keys)
**Affected Files**: `dhcp_socket.c`
**Description**: The eBPF maps use keys that include a nanosecond-precision timestamp:
```c
struct dhcp_key {
    __u64 timestamp; // <--- Critical Flaw
    __u32 pid;
    ...
};
key.timestamp = bpf_ktime_get_ns();
```
**Impact**:
- **Lookup Impossibility**: Userspace (or other eBPF programs) cannot look up any transaction (e.g., "Find the DHCP Discover for XID 1234") because the key requires knowing the *exact* nanosecond timestamp when it was inserted.
- **State Disconnection**: The maps function solely as a ring buffer/log, not as a lookup table. The logic attempting to update state (e.g., matching a DHCP Request to a Discover) fails because the key for the original entry cannot be reconstructed.
**Recommendation**: Remove `timestamp` from the key. Use `xid` (DHCP) or `5-tuple` (TLS/DNS) as the unique key. Move timestamps to the `value` struct.

### 2. DNS Response Correlation Logic Failure
**Affected File**: `dns_socket.c`
**Description**: The logic to calculate DNS response time attempts to look up the original query using the response packet's flow tuple *without* reversing it.
```c
// Current (Broken):
struct dns_key lookup_key = {};
lookup_key.src_ip = ip->saddr; // Should be ip->daddr
lookup_key.dst_ip = ip->daddr; // Should be ip->saddr
// ...
```
**Impact**: Response time calculation will always be zero/invalid as the lookup will never match the query key.
**Recommendation**: Implement 5-tuple swapping (Source <-> Dest, Port <-> Port) when constructing the lookup key for responses.

### 3. TLS Implementation Removed (ADR-001 Compliance)
**Affected Component**: TLS Socket Filter
**Description**: The TLS socket filter has been removed to comply with ADR-001 (Hybrid Architecture).
**Impact**: TLS fingerprinting is now handled by the NFQUEUE layer, which is appropriate for complex payload parsing and avoids fragmentation blind spots.

### 4. Stubbed Userspace Implementation
**Affected Files**: `internal/ebpf/socket/*.go`
**Description**: The Go implementation consists almost entirely of `// TODO` stubs.
- `dns_filter.go`: `processQueryEvents` is empty.
- `dhcp_filter.go`: Event processing loops are empty.
- `manager.go`: Initialization references placeholder programs.
**Impact**: The system collects no data and performs no filtering in userspace. The "Completed" status in `ebpf-progress.md` is inaccurate.

## Major Issues 🟠

### 5. C/Go Definition Mismatch
**Description**: The Go code manually defines map specifications (`ebpf.NewMap`) with hardcoded key/value sizes that do not match the C structs.
- **Example**: `dns_filter.go` defines `KeySize: 16`, but `dns_socket.c` `struct dns_key` is significantly larger (includes `src_ip`, `dst_ip`, etc.).
**Impact**: Manually loading these maps will likely fail or cause memory corruption due to size mismatches.
**Recommendation**: Use `cilium/ebpf`'s `LoadCollectionSpec` to automatically load map definitions from the compiled ELF, ensuring synchronization between C and Go.

## Minor Issues 🟡

### 6. DNS Fail-Open
**Description**: `dns_socket.c` returns `0` (Pass) on all parsing errors.
**Impact**: Malformed or malicious DNS packets designed to evade the parser will bypass the filter.
**Recommendation**: Consider a "default drop" or "alert on parse error" mode for high-security configurations.

## Conclusion
The eBPF socket filters are not production-ready. 

**Status Update**: 
- TLS filter removed for ADR-001 compliance ✓
- Map key design already fixed (uses xid/mac, not timestamp) ✓
- DNS response correlation already fixed ✓
- Go userspace partially implemented (basic structure in place)
- C/Go struct mismatches fixed using LoadCollectionSpec ✓

I recommend marking Phase 4 as **In Progress** with DHCP and DNS monitoring components, while TLS fingerprinting should be tracked under the NFQUEUE implementation.
