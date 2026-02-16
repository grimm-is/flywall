# Flywall eBPF Implementation Roadmap

This roadmap outlines critical tasks required to bring Flywall's eBPF layer to production readiness. Tasks are categorized by layer and designed for delegation to engineering agents.

---

## 1. Socket Layer (Monitoring & Discovery)

### Task 1.1: DHCP Transaction Lifecycle Management
*   **Goal**: Prevent memory leakage and ensure accurate transaction tracking in `DHCPFilter`.
*   **Affected Files**: `internal/ebpf/socket/dhcp_filter.go`, `internal/ebpf/programs/c/dhcp_socket.c`
*   **Requirements**:
    *   Implement `cleanupExpiredTransactions` in Go.
    *   Iterate over `dhcp_discovers`, `dhcp_offers`, `dhcp_requests`, and `dhcp_acks` maps.
    *   Remove entries older than `config.TransactionTimeout`.
    *   Unify `dhcp_key` usage between C and Go (ensure padding compatibility).
*   **DoD**: Integration test showing transactions are removed from maps after timeout.

### Task 1.2: Regex-based DNS Filtering
*   **Goal**: Support advanced domain blocking patterns.
*   **Affected Files**: `internal/ebpf/socket/response_filter.go`
*   **Requirements**:
    *   Implement `RegexMatch` logic in `FilterResponse`.
    *   Add `RegexPatterns` field to `ResponseFilterConfig`.
    *   Pre-compile regex patterns on startup/config-reload for performance.
*   **DoD**: Unit tests verifying that `malicious-.*\.com` blocks `malicious-ads.com`.

### Task 1.3: JA3 Hashing Implementation
*   **Goal**: Enable TLS fingerprinting for security monitoring.
*   **Affected Files**: `internal/ebpf/programs/c/socket_tls.c`, `internal/ebpf/socket/tls_filter.go`
*   **Requirements**:
    *   In C: Parse the ClientHello to extract: Version, Accepted Ciphers, List of Extensions, Elliptic Curves, and Elliptic Curve Formats.
    *   In C: Produce a MD5 hash (or simplified 128-bit hash) of these fields in the standard JA3 format.
    *   In Go: Update `tlsEvent` struct to correctly receive the 16-byte hash.
*   **DoD**: Logs showing valid JA3 hashes for incoming HTTPS connections.

---

## 2. Performance Layer (Acceleration)

### Task 2.1: Robust Hardware Offload Probing
*   **Goal**: Replace stubs with actual hardware capability detection.
*   **Affected Files**: `internal/ebpf/performance/hardware_offload.go`
*   **Requirements**:
    *   Implement `detectEncapOffload`, `detectDecapOffload`, and `detectVXLANOffload` using `ethtool` or `netlink`.
    *   Implement `detectMaxFlows` by querying driver limits if available, or using a safe default based on NIC model.
    *   Replace `exec.Command("tc", ...)` with a library-based approach (e.g., `github.com/florianl/go-tc`) for atomic flow installation.
*   **DoD**: Hardware offload successfully enables only on supported NICs (e.g., Mellanox ConnectX, Intel 700 series).

### Task 2.2: Adaptive Optimization Engine
*   **Goal**: Implement the "Auto-Tuning" feedback loop.
*   **Affected Files**: `internal/ebpf/performance/manager.go`
*   **Requirements**:
    *   Implement `tuneMemoryPool`: Adjust pool size based on `PacketPoolMisses`.
    *   Implement `tuneCache`: Dynamically increase `FlowCacheSize` if `FlowHitRate < 0.9`.
    *   Integrate with Prometheus: Export all metrics in `PerformanceStats`.
*   **DoD**: Simulation showing worker counts and cache sizes adjusting under varying synthetic load.

---

## 3. Control Plane (Management)

### Task 3.1: Feature Lifecycle API
*   **Goal**: Enable dynamic runtime control of eBPF components.
*   **Affected Files**: `internal/ebpf/controlplane/controlplane.go`
*   **Requirements**:
    *   Implement `handleEnableFeature` and `handleDisableFeature`.
    *   Implement `handleUpdateConfig`: Must validate new config and apply it without dropping the Unix socket connection.
    *   Implement `UpdateFirewallRules`: Hook into the `Manager` to update `xdp_blocklist` maps atomically.
*   **DoD**: `curl -X POST /api/v1/ebpf/features/dns_blocklist/enable` successfully toggles the feature.

---

## 4. Kernel Layer (Integration)

### Task 4.1: Conntrack Table Synchronization
*   **Goal**: Synchronize eBPF flow state with Linux kernel conntrack.
*   **Affected Files**: `internal/kernel/provider_linux.go`, `internal/ebpf/manager.go`
*   **Requirements**:
    *   Implement `DumpFlows`: Parse `/proc/net/nf_conntrack` or use `conntlink` to read the state table.
    *   In `Manager`: Create a sync routine that seeds the `flow_map` from conntrack on startup.
*   **DoD**: New eBPF instances inherit existing connection states from the kernel.

---

## 5. Cross-Cutting Technical Debt

### Task 5.1: Unified Struct Definition Audit
*   **Goal**: Ensure binary compatibility between C and Go across all programs.
*   **Requirements**:
    *   Audit `common.h`, `xdp_blocklist.c`, `dhcp_socket.c`, and `socket_tls.c`.
    *   Ensure `struct event` and `struct statistics` are defined ONCE in `common.h` and used everywhere.
    *   Verify Go `types.go` definitions match C memory layout (including explicit padding).
*   **DoD**: `go generate` runs without warnings and integration tests pass with `DEBUG_EBPF=1`.
