# ADR-001: Hybrid eBPF/NFQUEUE Architecture

**Status:** Accepted
**Date:** February 2, 2026
**Authors:** Architecture Review

## Context

Flywall currently uses NFQUEUE for inline packet inspection, providing full payload access in userspace Go code. The eBPF implementation plan proposed migrating to XDP/TC for "10x performance gains."

External review identified critical limitations that make a pure eBPF approach unsuitable for deep packet inspection and IPS workloads.

## Decision

**We will NOT replace NFQUEUE with eBPF.** Instead, we adopt a **Hybrid Offload Model**:

| Layer | Technology | Use Case |
|-------|------------|----------|
| L3/L4 Fast Path | XDP | DDoS mitigation, IP blocklists, rate limiting |
| First-N Packets | NFQUEUE | TLS fingerprinting, DPI, learning engine |
| Trusted Flow Fast Path | TC (egress) | Offload established "safe" connections |
| Fallback | NFQUEUE | All flows when eBPF unavailable |

## Problem Analysis

### 1. Deep Packet Inspection Limitations

**eBPF Verifier Constraints:**
- Strict instruction count limits (~1M instructions)
- No unbounded loops (cannot "scan payload for regex")
- Stack size limited to 512 bytes
- Complex payload parsing exceeds limits

**Fragmentation Blind Spot:**
- XDP runs BEFORE kernel defragmentation
- Malicious payloads split across IP fragments bypass XDP inspection
- Reassembly in eBPF is memory-intensive and unreliable
- NFQUEUE benefits from kernel conntrack defragmentation

### 2. State Synchronization ("Split Brain")

**The Problem:**
```
Userspace (Go)          Kernel (eBPF)
     │                       │
     │  "Flow X is bad"      │
     │  ───────────────►     │  (map update syscall)
     │                       │
     │     ◄─── latency ───► │  (packets still passing)
     │                       │
```

- Truth about flow state lives in kernel maps
- Detection logic lives in userspace
- Ring buffer sync introduces latency gap
- During high-rate attacks, map updates lag behind

**Map Management:**
- No garbage collection in eBPF
- Map fills during DDoS → fail-open or fail-closed
- Manual eviction policies required

### 3. Development & Debugging Complexity

**Verifier Hell:**
- Code that compiles may be rejected at load time
- "Potentially out of bounds" false positives
- Weeks spent fighting verifier, not building features

**Debugging:**
- No `fmt.Println` or Delve debugger
- Limited to `bpf_trace_printk` (slow, limited)
- Map inspection via external tools

**Kernel Compatibility:**
- CO-RE helps but doesn't solve everything
- Requires BTF (BPF Type Format) in kernel
- Older kernels or missing CONFIG flags = total failure

### 4. Fail-Open vs Fail-Closed Safety

**NFQUEUE:**
```hcl
queue num 100 bypass  # If userspace crashes, traffic flows
```

**XDP/TC:**
- Logic bug drops packets at line rate
- Silent blackhole with incredible efficiency
- Unloading buggy program harder to automate
- No equivalent "bypass" flag

### 5. TLS/Encrypted Traffic

**The Challenge:**
- Client Hello may span multiple TCP segments
- XDP/TC sees individual packets, not streams
- Cannot "wait" for next packet to reconstruct
- JA3 extraction extremely brittle in eBPF

**NFQUEUE Advantage:**
- Buffer handshake packets
- Parse in Go with full string manipulation
- Make informed decision with complete data

## Architecture

```
                    ┌─────────────────────────────────────┐
                    │           Incoming Traffic          │
                    └─────────────────────────────────────┘
                                      │
                                      ▼
                    ┌─────────────────────────────────────┐
                    │         XDP (L3/L4 Fast Path)       │
                    │  • IP Blocklist lookup              │
                    │  • Rate limiting (token bucket)     │
                    │  • DDoS mitigation                  │
                    │  • Bogon filtering                  │
                    └─────────────────────────────────────┘
                           │                    │
                      (blocked)            (passed)
                           │                    │
                           ▼                    ▼
                        DROP         ┌─────────────────────┐
                                     │   TC Classifier     │
                                     │  • Check flow map   │
                                     │  • Trusted? → PASS  │
                                     │  • Unknown? → QUEUE │
                                     └─────────────────────┘
                                            │         │
                                       (trusted)  (unknown)
                                            │         │
                                            ▼         ▼
                                         PASS    ┌───────────┐
                                                 │  NFQUEUE  │
                                                 │  (Go IPS) │
                                                 └───────────┘
                                                       │
                                              ┌────────┴────────┐
                                              │                 │
                                        (safe flow)      (malicious)
                                              │                 │
                                              ▼                 ▼
                                    ┌─────────────────┐      DROP
                                    │ Update flow map │
                                    │ (offload to TC) │
                                    └─────────────────┘
```

## Implementation Guidelines

### XDP Layer (Stateless, L3/L4 Only)
```c
// ONLY these operations in XDP:
- IP blocklist lookup (hash map)
- Rate limiting per-IP (LRU map + token bucket)
- Bogon/martian filtering (static)
- Protocol validation (basic sanity)

// NEVER in XDP:
- Payload inspection beyond headers
- Fragmented packet reassembly
- TLS/application protocol parsing
- Complex stateful decisions
```

### NFQUEUE Layer (Stateful, DPI)
```go
// Full capability:
- TLS fingerprinting (JA3/JA4)
- HTTP/DNS payload inspection
- Learning engine training
- Pattern matching (regex, Aho-Corasick)
- Defragmented packet access
```

### TC Fast Path (Offload)
```c
// After Go approves a flow:
- Lookup 5-tuple in "trusted_flows" map
- If found and not expired → PASS (skip NFQUEUE)
- Counters for statistics
```

## Implementation Considerations

### 1. XDP Fragment Handling
**Critical:** XDP programs must immediately pass fragmented packets to avoid parsing issues.

```c
// In XDP program
if (iph->frag_off & htons(IP_MF | IP_OFFSET)) {
    // Fragmented packet - let kernel reassemble
    return XDP_PASS;
}
```

### 2. Flow Offload Race Conditions
**Acceptable Failure:** Race between Go approval and eBPF map update is safe.
- Packet may go to NFQUEUE again (no harm)
- Go logic must be idempotent
- Consider re-asserting offload on duplicate packets

### 3. TCP State Cleanup
**Recommended Approach:** Use timeout/LRU for flow map cleanup.
- Option A: Timeout-based expiration (recommended)
- Option B: TC detects FIN/RST (complex, race-prone)

## Consequences

### Positive
- Maintains NFQUEUE's full inspection capability
- Gains XDP performance for simple L3/L4 filtering
- Graceful fallback when eBPF unavailable
- Safer failure modes (NFQUEUE bypass)

### Negative
- More complex than pure-eBPF approach
- Two codebases to maintain (C + Go)
- Latency for first-N packets (NFQUEUE overhead)

### Neutral
- Aligns with industry practice (Suricata, Cilium)
- Matches original "offload trusted flows" concept

## References

- `inline-ips-implementation.md` - Original NFQUEUE design
- `ebpf-feature-mapping.md` - Hybrid approach mention
- External architecture review (Feb 2, 2026)
