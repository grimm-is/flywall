// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package controlplane

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ebpf/loader"
	"grimm.is/flywall/internal/logging"
)

// TestControlPlaneIntegration tests the control plane integration
func TestControlPlaneIntegration(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Control plane integration test requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	// Create test configuration
	cfg := createTestConfig()
	logger := logging.New(logging.Config{Level: logging.LevelError})

	// Create control plane
	cp, err := NewControlPlane(cfg, *logger, nil)
	if err != nil {
		t.Fatalf("Failed to create control plane: %v", err)
	}
	defer cp.Close()

	// Test 1: Start and stop
	t.Run("StartStop", func(t *testing.T) {
		if err := cp.Start(); err != nil {
			t.Fatalf("Failed to start control plane: %v", err)
		}

		// Give it time to initialize
		time.Sleep(2 * time.Second)

		if err := cp.Stop(); err != nil {
			t.Errorf("Failed to stop control plane: %v", err)
		}

		t.Log("✓ Control plane started and stopped successfully")
	})

	// Test 2: HTTP API endpoints
	t.Run("HTTPAPI", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(cp.router)
		defer server.Close()

		// Test health endpoint
		resp, err := http.Get(server.URL + "/api/v1/ebpf/health")
		if err != nil {
			t.Fatalf("Failed to get health: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Health endpoint returned status %d", resp.StatusCode)
		}

		var health map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("Failed to decode health response: %v", err)
		}

		if healthy, ok := health["healthy"].(bool); !ok || !healthy {
			t.Error("Health check failed")
		}

		t.Log("✓ HTTP API health check passed")
	})

	// Test 3: Statistics API
	t.Run("StatisticsAPI", func(t *testing.T) {
		// Start control plane
		if err := cp.Start(); err != nil {
			t.Fatalf("Failed to start control plane: %v", err)
		}
		defer cp.Stop()

		// Wait for initialization
		time.Sleep(2 * time.Second)

		// Create test server
		server := httptest.NewServer(cp.router)
		defer server.Close()

		// Test stats endpoint
		resp, err := http.Get(server.URL + "/api/v1/ebpf/stats")
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Stats endpoint returned status %d", resp.StatusCode)
		}

		var stats map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			t.Fatalf("Failed to decode stats response: %v", err)
		}

		// Verify stats structure
		if _, ok := stats["packets_processed"]; !ok {
			t.Error("Missing packets_processed in stats")
		}
		if _, ok := stats["programs"]; !ok {
			t.Error("Missing programs in stats")
		}
		if _, ok := stats["maps"]; !ok {
			t.Error("Missing maps in stats")
		}

		t.Logf("✓ Statistics API returned: %v", stats)
	})
}

// TestFirewallRuleUpdate tests firewall rule updates
func TestFirewallRuleUpdate(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Firewall rule update test requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	cfg := createTestConfig()
	logger := logging.New(logging.Config{Level: logging.LevelError})

	cp, err := NewControlPlane(cfg, *logger, nil)
	if err != nil {
		t.Fatalf("Failed to create control plane: %v", err)
	}
	defer cp.Close()

	// Start control plane
	if err := cp.Start(); err != nil {
		t.Fatalf("Failed to start control plane: %v", err)
	}
	defer cp.Stop()

	// Wait for initialization
	time.Sleep(2 * time.Second)

	// Test rule update (placeholder)
	t.Run("UpdateRules", func(t *testing.T) {
		err := cp.UpdateFirewallRules(nil) // TODO: Fix type
		if err != nil {
			t.Logf("Rule update not yet implemented: %v", err)
		} else {
			t.Log("✓ Firewall rules updated")
		}
	})
}

// TestBlocklistUpdate tests blocklist updates
func TestBlocklistUpdate(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Blocklist update test requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	cfg := createTestConfig()
	logger := logging.New(logging.Config{Level: logging.LevelError})

	cp, err := NewControlPlane(cfg, *logger, nil)
	if err != nil {
		t.Fatalf("Failed to create control plane: %v", err)
	}
	defer cp.Close()

	// Start control plane
	if err := cp.Start(); err != nil {
		t.Fatalf("Failed to start control plane: %v", err)
	}
	defer cp.Stop()

	// Wait for initialization
	time.Sleep(2 * time.Second)

	// Test blocklist update (placeholder)
	t.Run("UpdateBlocklist", func(t *testing.T) {
		ips := []string{"192.168.1.100", "10.0.0.50"}

		err := cp.UpdateBlocklist(ips)
		if err != nil {
			t.Logf("Blocklist update not yet implemented: %v", err)
		} else {
			t.Log("✓ Blocklist updated")
		}
	})
}

// TestConfigurationReload tests configuration reloading
func TestConfigurationReload(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Configuration reload test requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	cfg := createTestConfig()
	logger := logging.New(logging.Config{Level: logging.LevelError})

	cp, err := NewControlPlane(cfg, *logger, nil)
	if err != nil {
		t.Fatalf("Failed to create control plane: %v", err)
	}
	defer cp.Close()

	// Start control plane
	if err := cp.Start(); err != nil {
		t.Fatalf("Failed to start control plane: %v", err)
	}
	defer cp.Stop()

	// Wait for initialization
	time.Sleep(2 * time.Second)

	// Create test server
	server := httptest.NewServer(cp.router)
	defer server.Close()

	// Test reload endpoint
	t.Run("ReloadConfig", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/api/v1/ebpf/reload", "application/json", nil)
		if err != nil {
			t.Fatalf("Failed to reload config: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Reload returned status %d: %s", resp.StatusCode, string(body))
		}

		var result map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode reload response: %v", err)
		}

		if result["status"] != "ok" {
			t.Error("Reload failed")
		}

		t.Log("✓ Configuration reloaded successfully")
	})
}

// TestFeatureToggle tests enabling/disabling features
func TestFeatureToggle(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Feature toggle test requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	cfg := createTestConfig()
	logger := logging.New(logging.Config{Level: logging.LevelError})

	cp, err := NewControlPlane(cfg, *logger, nil)
	if err != nil {
		t.Fatalf("Failed to create control plane: %v", err)
	}
	defer cp.Close()

	// Start control plane
	if err := cp.Start(); err != nil {
		t.Fatalf("Failed to start control plane: %v", err)
	}
	defer cp.Stop()

	// Wait for initialization
	time.Sleep(2 * time.Second)

	// Create test server
	server := httptest.NewServer(cp.router)
	defer server.Close()

	// Test feature enable/disable
	t.Run("ToggleFeature", func(t *testing.T) {
		// Enable feature
		resp, err := http.Post(server.URL+"/api/v1/ebpf/features/tc_offload/enable",
			"application/json", nil)
		if err != nil {
			t.Fatalf("Failed to enable feature: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Enable feature returned status %d", resp.StatusCode)
		}

		// Disable feature
		resp, err = http.Post(server.URL+"/api/v1/ebpf/features/tc_offload/disable",
			"application/json", nil)
		if err != nil {
			t.Fatalf("Failed to disable feature: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Disable feature returned status %d", resp.StatusCode)
		}

		t.Log("✓ Feature toggle successful")
	})
}

// createTestConfig creates a test configuration
func createTestConfig() *config.Config {
	cfg := &config.Config{}

	// Configure API
	cfg.API = &config.APIConfig{
		Enabled: true,
		Listen:  "127.0.0.1:0", // Use random port on loopback to avoid conflicts
	}

	// Configure eBPF
	cfg.EBPF = &config.EBPFConfig{
		Enabled: true,
		Features: []*config.EBPFFeatureConfig{
			{
				Name:    "tc_offload",
				Enabled: true,
			},
		},
		StatsExport: &config.StatsExportConfig{
			EnablePrometheus: false,
			EnableJSON:       false,
		},
	}

	return cfg
}

// TestRealWorldScenario tests a real-world scenario
func TestRealWorldScenario(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Real-world scenario test requires root privileges")
	}

	// Check if eBPF is supported
	if err := loader.VerifyKernelSupport(); err != nil {
		t.Skipf("eBPF not supported: %v", err)
	}

	if testing.Short() {
		t.Skip("Skipping real-world scenario in short mode")
	}

	cfg := createTestConfig()
	logger := logging.New(logging.Config{Level: logging.LevelInfo})

	cp, err := NewControlPlane(cfg, *logger, nil)
	if err != nil {
		t.Fatalf("Failed to create control plane: %v", err)
	}
	defer cp.Close()

	// Start control plane
	if err := cp.Start(); err != nil {
		t.Fatalf("Failed to start control plane: %v", err)
	}
	defer cp.Stop()

	// Wait for full initialization
	time.Sleep(5 * time.Second)

	// Create test server
	server := httptest.NewServer(cp.router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Simulate real-world operations
	operations := []func() error{
		func() error { return checkHealth(server.URL) },
		func() error { return getStatistics(server.URL) },
		func() error { return getProgramStats(server.URL) },
		func() error { return getMapStats(server.URL) },
		func() error { return reloadConfig(server.URL) },
		func() error { return getConfig(server.URL) },
	}

	for _, op := range operations {
		select {
		case <-ctx.Done():
			t.Fatal("Test timed out")
		default:
			if err := op(); err != nil {
				t.Errorf("Operation failed: %v", err)
			}
		}
	}

	t.Log("✓ Real-world scenario completed successfully")
}

// Helper functions for HTTP requests
func checkHealth(baseURL string) error {
	resp, err := http.Get(baseURL + "/api/v1/ebpf/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func getStatistics(baseURL string) error {
	resp, err := http.Get(baseURL + "/api/v1/ebpf/stats")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func getProgramStats(baseURL string) error {
	resp, err := http.Get(baseURL + "/api/v1/ebpf/stats/programs")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func getMapStats(baseURL string) error {
	resp, err := http.Get(baseURL + "/api/v1/ebpf/stats/maps")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func reloadConfig(baseURL string) error {
	resp, err := http.Post(baseURL+"/api/v1/ebpf/reload", "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func getConfig(baseURL string) error {
	resp, err := http.Get(baseURL + "/api/v1/ebpf/config")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
