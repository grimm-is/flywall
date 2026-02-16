// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package state

import (
	"os"
	"testing"
)

func TestBaselineAdapter(t *testing.T) {
	// Create temp dir for test DB
	tmpDir, err := os.MkdirTemp("", "baseline_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := NewSQLiteStore(DefaultOptions(":memory:"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Create bucket and adapter
	bucket, err := NewMetricsBaselineBucket(store)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewBaselineAdapter(bucket)

	// Test interface baseline save/load
	t.Run("interface baseline", func(t *testing.T) {
		err := adapter.SaveInterfaceBaseline("eth0", 1000, 500)
		if err != nil {
			t.Fatal(err)
		}

		rx, tx, err := adapter.LoadInterfaceBaseline("eth0")
		if err != nil {
			t.Fatal(err)
		}
		if rx != 1000 || tx != 500 {
			t.Errorf("Expected rx=1000, tx=500, got rx=%d, tx=%d", rx, tx)
		}
	})

	// Test policy baseline save/load
	t.Run("policy baseline", func(t *testing.T) {
		err := adapter.SavePolicyBaseline("lan->wan", 100, 10000)
		if err != nil {
			t.Fatal(err)
		}

		packets, bytes, err := adapter.LoadPolicyBaseline("lan->wan")
		if err != nil {
			t.Fatal(err)
		}
		if packets != 100 || bytes != 10000 {
			t.Errorf("Expected packets=100, bytes=10000, got packets=%d, bytes=%d", packets, bytes)
		}
	})

	// Test not found
	t.Run("not found", func(t *testing.T) {
		_, _, err := adapter.LoadInterfaceBaseline("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent baseline")
		}
	})
}
