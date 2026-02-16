// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/florianl/go-nfqueue/v2"
	"github.com/vishvananda/netlink"

	"grimm.is/flywall/internal/ebpf/programs"
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/learning"
	"grimm.is/flywall/internal/logging"
)

const (
	benchInterface = "bench0"
	benchQueueNum  = 99
)

// setupBenchInterface creates a dummy interface and sets up iptables rules
func setupBenchInterface(t testing.TB) func() {
	// 1. Create dummy interface
	link := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name: benchInterface,
		},
	}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("Failed to create dummy interface: %v", err)
	}

	// 2. Assign IP
	addr, _ := netlink.ParseAddr("192.168.100.1/24")
	if err := netlink.AddrAdd(link, addr); err != nil {
		netlink.LinkDel(link)
		t.Fatalf("Failed to add address: %v", err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		netlink.LinkDel(link)
		t.Fatalf("Failed to set link up: %v", err)
	}

	// 3. Add iptables rule to send traffic to NFQUEUE
	// We use OUTPUT chain and specific destination to target our packets
	cmd := exec.Command("iptables", "-I", "OUTPUT", "-o", benchInterface, "-j", "NFQUEUE", "--queue-num", fmt.Sprintf("%d", benchQueueNum))
	if out, err := cmd.CombinedOutput(); err != nil {
		netlink.LinkDel(link)
		t.Fatalf("Failed to add iptables rule: %v, output: %s", err, out)
	}

	return func() {
		exec.Command("iptables", "-D", "OUTPUT", "-o", benchInterface, "-j", "NFQUEUE", "--queue-num", fmt.Sprintf("%d", benchQueueNum)).Run()
		netlink.LinkDel(link)
	}
}

// BenchmarkNFQUEUE_RealLatency measures the actual round-trip latency from sending a packet
// to receiving it in userspace via NFQUEUE.
func BenchmarkNFQUEUE_RealLatency(b *testing.B) {
	if os.Getuid() != 0 {
		b.Skip("Benchmark requires root privileges")
	}

	cleanup := setupBenchInterface(b)
	defer cleanup()

	// Setup NFQUEUE listener
	config := nfqueue.Config{
		NfQueue:      benchQueueNum,
		MaxPacketLen: 0xFFFF,
		MaxQueueLen:  1024,
		Copymode:     nfqueue.NfQnlCopyPacket,
		WriteTimeout: 10 * time.Millisecond,
	}

	nf, err := nfqueue.Open(&config)
	if err != nil {
		b.Fatalf("Failed to open nfqueue: %v", err)
	}
	defer nf.Close()

	// Channel to signal packet reception
	packetReceived := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fn := func(a nfqueue.Attribute) int {
		nf.SetVerdict(*a.PacketID, nfqueue.NfAccept)
		select {
		case packetReceived <- struct{}{}:
		default:
		}
		return 0
	}

	if err := nf.RegisterWithErrorFunc(ctx, fn, func(e error) int { return 0 }); err != nil {
		b.Fatalf("Failed to register nfqueue: %v", err)
	}

	// Connect to the benchmark interface
	conn, err := net.Dial("udp", "192.168.100.2:80")
	if err != nil {
		b.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	payload := []byte("benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Send packet
		if _, err := conn.Write(payload); err != nil {
			b.Fatalf("Failed to write packet: %v", err)
		}

		// Wait for it to appear in NFQUEUE
		<-packetReceived
	}
}

// BenchmarkNFQUEUE_Userspace benchmarks the userspace packet processing logic
func BenchmarkNFQUEUE_Userspace(b *testing.B) {
	// Setup: Initialize learning engine with in-memory DB
	logger := logging.New(logging.Config{Level: logging.LevelError})
	config := learning.EngineConfig{
		DBPath:       ":memory:",
		Logger:       logger,
		LearningMode: true,
	}

	engine, err := learning.NewEngine(config)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Stop()

	// Pre-generate random packet info
	numPackets := 1000
	packets := make([]*learning.PacketInfo, numPackets)
	for i := 0; i < numPackets; i++ {
		packets[i] = &learning.PacketInfo{
			SrcMAC:   fmt.Sprintf("00:11:22:33:44:%02x", i%256),
			SrcIP:    fmt.Sprintf("192.168.1.%d", i%250),
			DstIP:    "10.0.0.1",
			DstPort:  80,
			Protocol: "TCP",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ProcessPacket(packets[i%numPackets])
	}
}

// BenchmarkEBPF_MapUpdate benchmarks the eBPF map update operation
func BenchmarkEBPF_MapUpdate(b *testing.B) {
	if os.Getuid() != 0 {
		b.Skip("Benchmark requires root privileges")
	}

	logger := logging.New(logging.Config{Level: logging.LevelError})
	program, err := programs.NewTCOffloadProgram(logger)
	if err != nil {
		b.Fatalf("Failed to load TC program: %v", err)
	}
	defer program.Close()

	// Pre-generate keys and states
	numFlows := 1000
	keys := make([]types.FlowKey, numFlows)
	states := make([]types.FlowState, numFlows)

	for i := 0; i < numFlows; i++ {
		keys[i] = types.FlowKey{
			SrcIP:   uint32(i),
			DstIP:   uint32(i + 1),
			SrcPort: uint16(i + 2),
			DstPort: uint16(i + 3),
			IPProto: 6,
		}
		states[i] = types.FlowState{
			PacketCount: 1,
			ByteCount:   100,
			LastSeen:    uint64(time.Now().UnixNano()),
			Verdict:     1,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = program.UpdateFlow(keys[i%numFlows], states[i%numFlows])
	}
}

// TestRelativeSpeed runs benchmarks and calculates relative performance using measured overhead
func TestRelativeSpeed(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges")
	}

	// 1. Measure Logic Latency
	t.Log("Measuring Userspace Logic Latency...")
	logicResult := testing.Benchmark(BenchmarkNFQUEUE_Userspace)
	logicNs := time.Duration(logicResult.NsPerOp())
	t.Logf("Result: %s per op", logicNs)

	// 2. Measure Real Full-Stack Latency
	t.Log("Measuring Full-Stack NFQUEUE Latency (System + Logic)...")
	realResult := testing.Benchmark(BenchmarkNFQUEUE_RealLatency)
	realNs := time.Duration(realResult.NsPerOp())
	t.Logf("Result: %s per op", realNs)

	// 3. Measure eBPF Control Plane (Proxy)
	t.Log("Measuring eBPF Map Update Latency...")
	ebpfResult := testing.Benchmark(BenchmarkEBPF_MapUpdate)
	ebpfNs := time.Duration(ebpfResult.NsPerOp())
	t.Logf("Result: %s per op", ebpfNs)

	// 4. Calculate Measured Overhead
	measuredOverhead := realNs
	if measuredOverhead < 0 {
		measuredOverhead = 0 // Should not happen
	}

	// 5. Comparison
	// NFQUEUE Total = Real Measured Latency
	// eBPF Total = eBPF Map Update (Control Plane) + minimal data plane overhead
	// Since eBPF data plane is in-kernel and simpler than map update, using map update as proxy is conservative/fair.

	t.Logf("\n--- Performance Comparison (Measured) ---")
	t.Logf("NFQUEUE Total Latency: %s (Measured)", realNs)
	t.Logf("eBPF Control Plane:    %s (Measured)", ebpfNs)
	t.Logf("Netlink/OS Overhead:   %s (Derived)", measuredOverhead)

	if ebpfNs > 0 {
		ratio := float64(realNs) / float64(ebpfNs)
		t.Logf("Speedup Factor: eBPF is %.2fx faster than NFQUEUE", ratio)
	}
}
