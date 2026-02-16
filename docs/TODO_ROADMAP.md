# Flywall TODO Roadmap

Captured from a codebase-wide audit on 2026-02-10. Includes inline TODO comments and aspirational features from `internal/ebpf/README.md`.

---

## eBPF Programs — Not Yet Implemented (from README)

Source: [README.md](file:///Users/ben/projects/flywall/internal/ebpf/README.md)

- **tc_classifier.c** — TC program for flow classification: deep packet inspection, flow state mgmt, QoS classification, IPS verdicts
- **socket_dns.c** — Socket filter for DNS monitoring: query/response parsing, domain extraction, query statistics
- **socket_tls.c** — Socket filter for TLS monitoring: TLS handshake parsing, JA3 fingerprint extraction, SNI extraction
- **socket_dhcp.c** — Socket filter for DHCP monitoring: DHCP packet parsing, device fingerprinting, ARP tracking

## eBPF Planned Features (from README)

- **Inline IPS**: TC-based packet inspection, pattern matching, verdict application, learning engine integration
- **QoS Management**: Per-flow classification, rate limiting, priority queuing, bandwidth allocation
- **Learning Engine**: Event collection, pattern detection, rule generation, feedback loop
- **Device Discovery**: DHCP monitoring, ARP tracking, device fingerprinting, network topology

## eBPF Future Enhancements (from README)

- Hardware offload — NIC offload support
- Multi-core optimization — better CPU affinity
- Dynamic reloading — hot-swapping eBPF programs
- Distributed eBPF — cross-node coordination
- ML integration — machine learning for threat detection

---

## eBPF Socket Layer (`internal/ebpf/socket/`)

### DHCP eBPF Filter — `dhcp_filter.go`
Implement full DHCP packet parsing and analysis via eBPF:
- DHCP discover/offer/request/ACK message parsing (L370, L381, L394, L405)
- Packet validation (L420)
- Rogue DHCP server detection (L430)
- MAC address extraction from DHCP payloads (L440)
- DHCP option extraction (L447)
- Ring buffer reading via cilium/ebpf (L316)
- Resource cleanup on shutdown (L363)

### Device Discovery — `device_discovery.go`
Complete the device fingerprinting pipeline:
- Match devices by MAC from DHCP ACK events (L291)
- Vendor OUI lookup (L315)
- Device classification/categorization (L427)
- Alert system integration for new device events (L519)

### DNS Response Filter — `response_filter.go`
Enhance DNS filtering capabilities:
- TTL extraction from DNS responses (L320)
- Private/encrypted DNS detection (DoH, DoT) (L393)
- Domain blocklist loading from external sources (L403)
- Regex-based domain matching (L540)

### Socket Manager — `manager.go`
Wire up cross-component integrations:
- Forward DNS events to learning engine (L267)
- Forward suspicious traffic to IPS for blocking (L293, L512)
- Handle blocked response follow-up actions (L335)

### Query Logger — `query_logger.go`
- Cross-reference queries against block decisions (L256)

### DNS Filter — `dns_filter.go`
- Implement proper resource cleanup (L276)

### Device Database — `device_database.go`
- Synchronize with device discovery subsystem (L710)

---

## eBPF Performance Layer (`internal/ebpf/performance/`)

### Hardware Offload — `hardware_offload.go`
Implement NIC hardware offload detection and flow management:
- Offload capability detection: TC, flow, encap/decap, VXLAN, Geneve (L360–L391)
- Hardware limits: max flows, max actions (L397, L403)
- TC handle generation (L431)
- Hardware flow lifecycle: install, update, remove, sync (L464–L512)

### TC Optimizer — `tc_optimizer.go`
Real performance metrics and adaptive tuning:
- Packet processing logic (L392)
- CPU affinity via `runtime.LockOSThread` (L473)
- Replace hardcoded metrics with actual values: CacheHitRate, OffloadRate, DropRate (L525–L527)
- Actual CPU usage collection (L593)
- Dynamic worker pool scaling (L607)

### Performance Manager — `manager.go`
Adaptive tuning knobs:
- Batch size adjustment (L368, L372)
- Worker count adjustment (L378)
- Buffer pool size adjustment (L392)
- Cache size and TTL adjustment (L402, L408)
- Metrics export to monitoring system (L426)

### Batching — `batching.go`
- Adaptive batch size based on load (L299)

### Monitor — `monitor.go`
- Collect actual eBPF program metrics (L175)

---

## Engine Layer (`internal/engine/`) — Deferred Design

> [!NOTE]
> The engine layer TODOs represent **future architectural features** (simulation, compliance checking, dependency analysis). The stub code has been removed from the codebase. These features should be designed and implemented from scratch when prioritized.

### Planned Features
- **Configuration Simulator**: Dry-run firewall changes against traffic models
- **Compliance Checker**: Validate configs against security policies
- **Dependency Analyzer**: Detect rule conflicts and ordering dependencies
- **Traffic Store Integration**: Persist and replay traffic for analysis
- **Configuration Optimization**: Suggest rule consolidation and ordering improvements

---

## Services Layer (`internal/services/`)

### HA Service — `ha/service.go`
- Restore original MAC addresses on HA teardown (L633)

### Host Manager — `hostmanager/service.go`
- Support `ipv6_addr` type nftables sets (L101)

---

## Kernel Layer (`internal/kernel/`)

### Provider — `provider_linux.go`
- Implement conntrack table reading via `/proc/net/nf_conntrack` or `conntrack` CLI (L35)
