// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package programs

import (
	"os"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"grimm.is/flywall/internal/ebpf/loader"
	"grimm.is/flywall/internal/logging"
)

// TestTCOffloadProgram tests loading the TC offload program
func TestTCOffloadProgram(t *testing.T) {
	// Skip if not running as root (eBPF requires root)
	if os.Getuid() != 0 {
		t.Skip("Skipping eBPF test - requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	logger := logging.New(logging.Config{Level: logging.LevelInfo})

	program, err := NewTCOffloadProgram(logger)
	if err != nil {
		t.Fatalf("Failed to load TC offload program: %v", err)
	}
	defer program.Close()

	// Verify program is loaded
	if program.collection == nil {
		t.Fatal("Program collection is nil")
	}

	// Check if maps exist
	flowMap := program.collection.Maps["flow_map"]
	if flowMap == nil {
		t.Error("flow_map not found")
	}

	statsMap := program.collection.Maps["tc_stats_map"]
	if statsMap == nil {
		t.Error("tc_stats_map not found")
	}

	t.Log("TC offload program loaded successfully")
}

// TestTcOffloadGenerated tests the generated bpf2go code
func TestTcOffloadGenerated(t *testing.T) {
	// Test that the generated code can be loaded
	spec, err := LoadTcOffload()
	if err != nil {
		t.Fatalf("Failed to load TC offload spec: %v", err)
	}

	if spec == nil {
		t.Fatal("Spec is nil")
	}

	// Check programs
	if len(spec.Programs) == 0 {
		t.Error("No programs found in spec")
	}

	// Check maps
	if len(spec.Maps) == 0 {
		t.Error("No maps found in spec")
	}

	// Verify expected programs exist
	expectedPrograms := []string{"tc_egress_fast_path", "tc_fast_path"}
	for _, prog := range expectedPrograms {
		if _, exists := spec.Programs[prog]; !exists {
			t.Errorf("Program %s not found", prog)
		}
	}

	// Verify expected maps exist
	expectedMaps := []string{"flow_map", "qos_profiles", "tc_stats_map"}
	for _, m := range expectedMaps {
		if _, exists := spec.Maps[m]; !exists {
			t.Errorf("Map %s not found", m)
		}
	}

	t.Log("Generated TC offload code verified successfully")
}

// TestAllGeneratedPrograms tests all generated eBPF programs
func TestAllGeneratedPrograms(t *testing.T) {
	tests := []struct {
		name     string
		loadFunc func() (*ebpf.CollectionSpec, error)
	}{
		{
			name:     "TC Offload",
			loadFunc: LoadTcOffload,
		},
		{
			name:     "DNS Socket",
			loadFunc: LoadDnsSocket,
		},
		{
			name:     "DHCP Socket",
			loadFunc: LoadDhcpSocket,
		},
		{
			name:     "XDP Blocklist",
			loadFunc: LoadXdpBlocklist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := tt.loadFunc()
			if err != nil {
				t.Fatalf("Failed to load %s: %v", tt.name, err)
			}

			if spec == nil {
				t.Fatalf("Spec is nil for %s", tt.name)
			}

			t.Logf("Successfully loaded %s (%d programs, %d maps)", tt.name, len(spec.Programs), len(spec.Maps))
		})
	}
}

// BenchmarkTCOffloadLoad benchmarks loading the TC offload program
func BenchmarkTCOffloadLoad(b *testing.B) {
	logger := logging.New(logging.Config{Level: logging.LevelInfo})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		program, err := NewTCOffloadProgram(logger)
		if err != nil {
			b.Fatalf("Failed to load TC offload program: %v", err)
		}
		program.Close()
		time.Sleep(10 * time.Millisecond) // Give kernel time to cleanup
	}
}
