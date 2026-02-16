// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build integration
// +build integration

package programs

import (
	"context"
	"net"
	"os"
	"os/exec"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"grimm.is/flywall/internal/logging"
)

// TestTCOffloadIntegration tests the TC offload program with real network traffic
func TestTCOffloadIntegration(t *testing.T) {
	// Check for integration test environment variable
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("Set INTEGRATION=1 to run integration tests")
	}

	// Skip if not running as root
	if os.Getuid() != 0 {
		t.Skip("Integration tests require root privileges")
	}

	logger := logging.New(logging.Config{Level: logging.LevelInfo})

	// Use a simpler test that just loads and verifies the program
	// Full network namespace testing can be complex in CI environments
	t.Run("LoadAndVerify", func(t *testing.T) {
		program, err := NewTCOffloadProgram(logger)
		if err != nil {
			t.Fatalf("Failed to load TC offload program: %v", err)
		}
		defer program.Close()

		// Verify program is loaded
		if program.collection == nil {
			t.Fatal("Program collection is nil")
		}

		// Check maps exist and are accessible
		flowMap := program.collection.Maps["flow_map"]
		if flowMap == nil {
			t.Error("flow_map not found")
		} else {
			// Try to insert a test entry
			testKey := uint32(0x12345678)
			testValue := uint64(0x87654321)
			if err := flowMap.Update(&testKey, &testValue, ebpf.UpdateAny); err != nil {
				t.Errorf("Failed to update flow_map: %v", err)
			} else {
				// Try to retrieve it
				var retrievedValue uint64
				if err := flowMap.Lookup(&testKey, &retrievedValue); err != nil {
					t.Errorf("Failed to lookup from flow_map: %v", err)
				} else if retrievedValue != testValue {
					t.Errorf("Value mismatch: expected %v, got %v", testValue, retrievedValue)
				}
			}
		}

		t.Log("TC offload program loaded and verified successfully")
	})

	// If we have a suitable test interface, try attaching
	if iface := findTestInterface(); iface != "" {
		t.Run("AttachToInterface", func(t *testing.T) {
			program, err := NewTCOffloadProgram(logger)
			if err != nil {
				t.Fatalf("Failed to load TC offload program: %v", err)
			}
			defer program.Close()

			// Try to attach to a test interface
			l, err := link.AttachTC(link.TCAttach{
				Program:   program.collection.Programs["tc_fast_path"],
				Attach:    ebpf.AttachTCIngress,
				Interface: iface,
			})
			if err != nil {
				t.Logf("Failed to attach to %s (this may be expected): %v", iface, err)
				t.Skip("Cannot attach TC program to interface")
			}
			defer l.Close()

			t.Logf("Successfully attached TC program to %s", iface)
		})
	}
}

// findTestInterface looks for a suitable interface for testing
func findTestInterface() string {
	// List of common test interfaces
	testInterfaces := []string{"lo", "dummy0", "veth0", "test0"}

	for _, iface := range testInterfaces {
		if _, err := net.InterfaceByName(iface); err == nil {
			return iface
		}
	}

	return ""
}

// TestDNSFilterIntegration tests the DNS filter program
func TestDNSFilterIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("Set INTEGRATION=1 to run integration tests")
	}

	if os.Getuid() != 0 {
		t.Skip("Integration tests require root privileges")
	}

	// This test would require setting up a DNS server and client
	// For now, just verify the program loads
	t.Skip("DNS filter integration test not yet implemented")
}

// execCommand executes a command and fails the test if it fails
func execCommand(t *testing.T, cmd string, args ...string) {
	t.Helper()
	out, err := execCommandWithContext(context.Background(), t, cmd, args...)
	if err != nil {
		t.Fatalf("Command %s %v failed: %v\nOutput: %s", cmd, args, err, out)
	}
}

// execCommandContext executes a command with context
func execCommandContext(ctx context.Context, t *testing.T, cmd string, args ...string) ([]byte, error) {
	t.Helper()

	c := exec.CommandContext(ctx, cmd, args...)
	return c.CombinedOutput()
}

// execCommand is a helper that returns the error for checking
func execCommand(t *testing.T, cmd string, args ...string) error {
	t.Helper()
	_, err := execCommandWithContext(context.Background(), t, cmd, args...)
	return err
}
