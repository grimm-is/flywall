// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"os"
	"testing"

	"grimm.is/flywall/internal/host"
	"grimm.is/flywall/internal/logging"
)

// TestSystemRequirements checks if system meets minimum requirements
func TestSystemRequirements(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("System requirements check requires root privileges")
	}

	// Initialize logger (was unused but might be needed if VerifyBPFSupport uses it indirectly or for future expansion)
	_ = logging.New(logging.Config{Level: logging.LevelError})

	issues := host.VerifyBPFSupport()
	if len(issues) == 0 {
		t.Log("✓ All eBPF system requirements met")
	}

	for _, issue := range issues {
		if issue.Fatal {
			t.Skipf("Skipping eBPF tests due to missing requirement: %s: %s", issue.Feature, issue.Message)
		} else {
			t.Errorf("WARNING: %s: %s", issue.Feature, issue.Message)
		}
	}

	// Summary
	t.Log("System verification complete")
}

// BenchmarkBaseline provides baseline performance metrics
func BenchmarkBaseline(b *testing.B) {
	// Baseline test without eBPF to compare overhead
	b.Run("NoeBPF", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Simulate basic packet processing
			_ = i
		}
	})
}
