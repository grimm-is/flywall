# Inline IPS Implementation Summary

> **Architecture Note:** This NFQUEUE-based implementation remains the primary deep inspection path.
> eBPF (XDP/TC) is used only for L3/L4 fast-path offload, not as a replacement.
> See [ADR-001](../design/ADR-001-hybrid-ebpf-nfqueue.md) for rationale.

## Overview
This document summarizes the implementation of Inline Intrusion Prevention System (IPS) with Kernel Offloading in Flywall. The feature provides high-performance packet inspection by initially inspecting packets in userspace and then offloading trusted flows to the kernel using conntrack marks.

## Architecture

### Key Components
1. **Configuration Layer** - Added `packet_window` and `offload_mark` settings
2. **NFQueue Reader** - Enhanced to support verdicts with conntrack marks
3. **Learning Engine** - Added packet window tracking and offload decisions
4. **Firewall Rules** - Added bypass rule for offloaded flows
5. **Control Plane** - Integrated all components for inline operation

### Data Flow
1. New packets hit nftables queue rule
2. Packets are sent to userspace via NFQueue
3. Learning engine inspects packets and tracks flow state
4. After N packets, trusted flows are marked with conntrack mark
5. Subsequent packets bypass userspace via bypass rule

## Implementation Details

### 1. Configuration Changes
File: `internal/config/security.go`
```go
type RuleLearningConfig struct {
    // ... existing fields ...
    PacketWindow int    `hcl:"packet_window,optional" json:"packet_window"` // default: 10
    OffloadMark uint32 `hcl:"offload_mark,optional" json:"offload_mark"`   // default: 0x200000
}
```

### 2. NFQueue Enhancements
Files: `internal/ctlplane/verdict.go`, `internal/ctlplane/nfqueue_linux.go`
- Added `VerdictType` and `Verdict` struct
- Enhanced `NFQueueReader` to support conntrack marking
- Added `SetVerdictWithConnMark` support

### 3. Learning Engine Updates
File: `internal/learning/engine.go`
- Added `EngineVerdict` type with 4 states:
  - `VerdictInspect` - Continue inspection
  - `VerdictAllow` - Accept packet
  - `VerdictDrop` - Drop packet
  - `VerdictOffload` - Accept and mark for offload
- Added `ProcessPacketInline` method
- Enhanced `FlowCacheEntry` with packet count tracking

### 4. Firewall Rule Generation
File: `internal/firewall/script_builder_filter.go`
```nftables
# Rule 1: Bypass offloaded flows (high priority)
ct mark 0x200000 accept

# Rule 2: Queue new packets for inspection
queue num 100 bypass
```

### 5. Control Plane Integration
File: `internal/ctlplane/server.go`
- Updated `startInlineLearning` to use `ProcessPacketInline`
- Maps engine verdicts to NFQueue verdicts
- Handles offload mark configuration

## Configuration Example

```hcl
rule_learning {
  enabled = true
  inline_mode = true
  packet_window = 10
  offload_mark = 2097152  # Decimal format required (HCL doesn't support hex literals)
  learning_mode = true
}
```

## Performance Characteristics

1. **Initial Packets**: Full userspace inspection
2. **Trusted Flows**: Kernel bypass after packet window
3. **Latency**: Microseconds for initial packets, wire-speed after offload
4. **CPU Usage**: Reduced for long-lived flows

## Safety Features

1. **Fail-Open**: Queue bypass flag allows traffic if userspace is down
2. **Default Allow**: Errors result in accepting packets
3. **Graceful Degradation**: Falls back to async mode on errors

## Testing

### Unit Tests
- `internal/learning/engine_inline_test.go` - Tests packet window and offload logic
- `internal/ctlplane/nfqueue_test.go` - Tests verdict handling

### Integration Test
- `integration_tests/linux/70-system/inline_ips_test.sh` - VM-based end-to-end testing

The integration test verifies:
- Inline IPS mode activation
- Nftables rule generation (bypass + queue)
- Packet window enforcement
- Flow offloading with conntrack marks
- Fail-open safety mechanism
- Flow state transitions
- Learning engine integration

### Running Tests

```bash
# Run the Inline IPS integration test in VM
./flywall.sh test int inline_ips_test

# Or run all integration tests
./flywall.sh test int

# The test will run in a VM with full networking capabilities
```

### Test Reports
The test generates detailed logs in the VM output and verifies:
- Inline IPS mode activation
- Nftables rule generation (bypass + queue)
- Packet window enforcement
- Flow offloading with conntrack marks
- Fail-open safety mechanism
- Flow state transitions
- Learning engine integration

## Monitoring

### Log Messages
```
offloading trusted flow src_mac=aa:bb:cc:dd:ee:ff src_ip=192.168.1.100 protocol=TCP dst_port=443 packet_count=11 mark=0x200000
```

### Conntrack Check
```bash
conntrack -L | grep "mark=0x200000"
```

## Future Enhancements

1. **Adaptive Window**: Dynamic packet window based on flow characteristics
2. **Multiple Marks**: Different marks for different trust levels
3. **Statistics**: Per-flow offload metrics
4. **TC Offload Bypass**: Currently, `conntrack` marks are checked by `nftables`. Future iteration will use **eBPF TC** to apply these marks (or drop malicious flows) at the earliest possible ingress point, reducing `nftables` evaluation overhead for elephant flows.

## Troubleshooting

### Common Issues
1. **No offload occurring**: Check if flows exceed packet window
2. **All traffic dropped**: Verify bypass flag in queue rule
3. **High CPU usage**: Check if flows are being offloaded properly

### Debug Commands
```bash
# Check nftables rules
nft list ruleset | grep -E "(ct mark|queue)"

# Check conntrack marks
conntrack -L | grep mark

# Monitor flywall logs
journalctl -u flywall -f | grep -E "(offload|inline)"
```
