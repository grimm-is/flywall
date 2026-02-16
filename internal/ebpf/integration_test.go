// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ebpf

import (
	"os"
	"testing"
	"time"

	"github.com/zclconf/go-cty/cty"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ebpf/loader"
	"grimm.is/flywall/internal/logging"
)

// TestE2EIntegration tests end-to-end eBPF integration with control plane
func TestE2EIntegration(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("End-to-end integration test requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	// Create test configuration
	cfg := createTestConfig(t)
	logger := logging.New(logging.Config{Level: logging.LevelInfo})

	// Test 1: Create eBPF integration
	t.Run("CreateIntegration", func(t *testing.T) {
		integration, err := NewIntegration(cfg, *logger, nil)
		if err != nil {
			t.Fatalf("Failed to create eBPF integration: %v", err)
		}
		defer integration.Close()

		if integration == nil {
			t.Fatal("Integration is nil")
		}

		t.Log("✓ eBPF integration created successfully")
	})

	// Test 2: Start eBPF integration
	t.Run("StartIntegration", func(t *testing.T) {
		integration, err := NewIntegration(cfg, *logger, nil)
		if err != nil {
			t.Fatalf("Failed to create eBPF integration: %v", err)
		}
		defer integration.Close()

		// Start integration
		if err := integration.Start(); err != nil {
			t.Fatalf("Failed to start eBPF integration: %v", err)
		}

		// Give it time to initialize (increased for CI stability)
		time.Sleep(5 * time.Second)

		// Stop integration
		if err := integration.Stop(); err != nil {
			t.Errorf("Failed to stop eBPF integration: %v", err)
		}

		t.Log("✓ eBPF integration started and stopped successfully")
	})

	// Test 3: Statistics collection
	t.Run("StatisticsCollection", func(t *testing.T) {
		integration, err := NewIntegration(cfg, *logger, nil)
		if err != nil {
			t.Fatalf("Failed to create eBPF integration: %v", err)
		}
		defer integration.Close()

		// Start integration
		if err := integration.Start(); err != nil {
			t.Fatalf("Failed to start eBPF integration: %v", err)
		}
		defer integration.Stop()

		// Wait for programs to load
		time.Sleep(2 * time.Second)

		// Get statistics
		stats := integration.GetStatistics()
		if stats == nil {
			t.Fatal("Statistics is nil")
		}

		// Verify statistics structure
		if stats.Features == nil {
			t.Error("Features map is nil")
		}
		if stats.Maps == nil {
			t.Error("Maps map is nil")
		}
		if stats.Programs == nil {
			t.Error("Programs map is nil")
		}

		t.Logf("✓ Statistics collected: %d programs, %d maps, %d features",
			len(stats.Programs), len(stats.Maps), len(stats.Features))
	})

	// Test 4: Configuration updates
	t.Run("ConfigurationUpdates", func(t *testing.T) {
		integration, err := NewIntegration(cfg, *logger, nil)
		if err != nil {
			t.Fatalf("Failed to create eBPF integration: %v", err)
		}
		defer integration.Close()

		// Start integration
		if err := integration.Start(); err != nil {
			t.Fatalf("Failed to start eBPF integration: %v", err)
		}
		defer integration.Stop()

		// Test configuration update (if implemented)
		// This would test dynamic reconfiguration of eBPF programs
		t.Log("✓ Configuration updates (placeholder)")
	})

	// Test 5: Error handling
	t.Run("ErrorHandling", func(t *testing.T) {
		// Test with invalid configuration (empty eBPF config)
		invalidCfg := &config.Config{}
		invalidLogger := logging.New(logging.Config{Level: logging.LevelError})

		integration, err := NewIntegration(invalidCfg, *invalidLogger, nil)
		if err != nil {
			t.Fatalf("Unexpected error with empty config: %v", err)
		}
		defer integration.Close()

		if integration.IsEnabled() {
			t.Error("Expected integration to be disabled with empty config, but it is enabled")
		} else {
			t.Log("✓ Empty config correctly results in disabled integration")
		}
	})
}

// TestControlPlaneAPI tests integration with control plane APIs
func TestControlPlaneAPI(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Control plane API test requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	cfg := createTestConfig(t)
	logger := logging.New(logging.Config{Level: logging.LevelInfo})

	integration, err := NewIntegration(cfg, *logger, nil)
	if err != nil {
		t.Fatalf("Failed to create eBPF integration: %v", err)
	}
	defer integration.Close()

	// Start integration
	if err := integration.Start(); err != nil {
		t.Fatalf("Failed to start eBPF integration: %v", err)
	}
	defer integration.Stop()

	// Wait for initialization
	time.Sleep(2 * time.Second)

	// Test API endpoints that would be exposed by the control plane
	t.Run("StatisticsAPI", func(t *testing.T) {
		stats := integration.GetStatistics()
		if stats == nil {
			t.Fatal("Failed to get statistics")
		}

		// These would be exposed via HTTP API in production
		t.Logf("Statistics API would return:")
		t.Logf("  Packets processed: %d", stats.PacketsProcessed)
		t.Logf("  Packets dropped: %d", stats.PacketsDropped)
		t.Logf("  Active programs: %d", len(stats.Programs))
	})

	t.Run("HealthCheck", func(t *testing.T) {
		// Health check would verify:
		// - eBPF programs are loaded
		// - Maps are accessible
		// - Statistics are being collected
		stats := integration.GetStatistics()

		healthy := stats != nil && len(stats.Programs) > 0
		if !healthy {
			t.Error("Health check failed")
		} else {
			t.Log("✓ Health check passed")
		}
	})
}

// TestFirewallIntegration tests integration with firewall rules
func TestFirewallIntegration(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Firewall integration test requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	cfg := createTestConfig(t)
	logger := logging.New(logging.Config{Level: logging.LevelInfo})

	integration, err := NewIntegration(cfg, *logger, nil)
	if err != nil {
		t.Fatalf("Failed to create eBPF integration: %v", err)
	}
	defer integration.Close()

	// Start integration
	if err := integration.Start(); err != nil {
		t.Fatalf("Failed to start eBPF integration: %v", err)
	}
	defer integration.Stop()

	// Wait for initialization
	time.Sleep(2 * time.Second)

	// Test firewall rule application
	t.Run("ApplyRules", func(t *testing.T) {
		// This would test:
		// - Converting firewall rules to eBPF programs
		// - Loading and applying the programs
		// - Verifying they take effect
		t.Log("✓ Firewall rule application (placeholder)")
	})

	t.Run("BlockListIntegration", func(t *testing.T) {
		// This would test:
		// - Adding IPs to blocklist
		// - Verifying they are blocked by eBPF
		// - Removing from blocklist
		t.Log("✓ Block list integration (placeholder)")
	})
}

// createTestConfig creates a test configuration for eBPF
func createTestConfig(t *testing.T) *config.Config {
	cfg := &config.Config{
		// Basic configuration
	}

	// Enable eBPF features
	cfg.EBPF = &config.EBPFConfig{
		Enabled: true,
		Features: []*config.EBPFFeatureConfig{
			{
				Name:    "tc_offload",
				Enabled: true,
				Config: cty.ObjectVal(map[string]cty.Value{
					"max_flows":    cty.NumberIntVal(100000),
					"flow_timeout": cty.NumberIntVal(300),
					"enable_qos":   cty.BoolVal(true),
					"qos_mark":     cty.NumberIntVal(0x20),
				}),
			},
			{
				Name:    "xdp_blocklist",
				Enabled: true,
				Config: cty.ObjectVal(map[string]cty.Value{
					"max_entries": cty.NumberIntVal(1000000),
				}),
			},
			{
				Name:    "dns_filter",
				Enabled: true,
				Config: cty.ObjectVal(map[string]cty.Value{
					"log_queries": cty.BoolVal(true),
				}),
			},
		},
		Performance: &config.EBPFPerformanceConfig{
			MaxCPUPercent: 80,
			MaxMemoryMB:   500,
			MaxPPS:        1000000,
		},
		Adaptive: &config.EBPFAdaptiveConfig{
			Enabled: false,
		},
		StatsExport: &config.StatsExportConfig{
			EnablePrometheus: false,
			EnableJSON:       false,
		},
	}

	return cfg
}

// BenchmarkE2EIntegration benchmarks the end-to-end integration
func BenchmarkE2EIntegration(b *testing.B) {
	if os.Getuid() != 0 {
		b.Skip("Benchmark requires root privileges")
	}

	cfg := createTestConfig(nil)
	logger := logging.New(logging.Config{Level: logging.LevelError})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		integration, err := NewIntegration(cfg, *logger, nil)
		if err != nil {
			b.Fatalf("Failed to create integration: %v", err)
		}

		if err := integration.Start(); err != nil {
			integration.Close()
			b.Fatalf("Failed to start integration: %v", err)
		}

		// Simulate some work
		stats := integration.GetStatistics()
		_ = stats

		if err := integration.Stop(); err != nil {
			b.Errorf("Failed to stop integration: %v", err)
		}

		integration.Close()
	}
}
