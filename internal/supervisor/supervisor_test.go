// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package supervisor

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestCrashEvent_IsCrash(t *testing.T) {
	tests := []struct {
		name     string
		event    CrashEvent
		expected bool
	}{
		{
			name:     "clean exit",
			event:    CrashEvent{ExitCode: 0},
			expected: false,
		},
		{
			name:     "SIGTERM",
			event:    CrashEvent{Signal: syscall.SIGTERM},
			expected: false,
		},
		{
			name:     "SIGINT",
			event:    CrashEvent{Signal: syscall.SIGINT},
			expected: false,
		},
		{
			name:     "SIGKILL",
			event:    CrashEvent{Signal: syscall.SIGKILL},
			expected: true,
		},
		{
			name:     "SIGSEGV",
			event:    CrashEvent{Signal: syscall.SIGSEGV},
			expected: true,
		},
		{
			name:     "panic",
			event:    CrashEvent{WasPanic: true},
			expected: true,
		},
		{
			name:     "non-zero exit",
			event:    CrashEvent{ExitCode: 1},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.IsCrash(); got != tt.expected {
				t.Errorf("IsCrash() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSupervisor_ShouldEnterSafeMode(t *testing.T) {
	dir := t.TempDir()
	sup := New(dir, Config{Threshold: 3, Window: time.Minute})

	// No crashes yet
	if sup.ShouldEnterSafeMode() {
		t.Error("ShouldEnterSafeMode() should be false with no crashes")
	}

	// Add 2 crashes - still under threshold
	_ = sup.RecordExit(0, syscall.SIGKILL, false)
	_ = sup.RecordExit(0, syscall.SIGSEGV, false)
	if sup.ShouldEnterSafeMode() {
		t.Error("ShouldEnterSafeMode() should be false with 2 crashes")
	}

	// Add a clean exit - should not count
	_ = sup.RecordExit(0, 0, false)
	if sup.ShouldEnterSafeMode() {
		t.Error("Clean exit should not trigger safe mode")
	}

	// Add 3rd crash - at threshold
	_ = sup.RecordExit(0, syscall.SIGKILL, false)
	if !sup.ShouldEnterSafeMode() {
		t.Error("ShouldEnterSafeMode() should be true at threshold")
	}
}

func TestSupervisor_Reset(t *testing.T) {
	dir := t.TempDir()
	sup := New(dir, Config{Threshold: 3, Window: time.Minute})

	// Add crashes
	_ = sup.RecordExit(0, syscall.SIGKILL, false)
	_ = sup.RecordExit(0, syscall.SIGKILL, false)
	_ = sup.RecordExit(0, syscall.SIGKILL, false)

	if !sup.ShouldEnterSafeMode() {
		t.Fatal("Should be in safe mode before reset")
	}

	_ = sup.Reset()

	if sup.ShouldEnterSafeMode() {
		t.Error("Should not be in safe mode after reset")
	}
}

func TestSupervisor_StatePersistence(t *testing.T) {
	dir := t.TempDir()

	// Create supervisor and record crash
	sup1 := New(dir, DefaultConfig())
	_ = sup1.RecordExit(0, syscall.SIGKILL, false)

	// Create new supervisor - should load state
	sup2 := New(dir, DefaultConfig())
	if len(sup2.state.Events) != 1 {
		t.Errorf("Expected 1 event after reload, got %d", len(sup2.state.Events))
	}
}

func TestSupervisor_PruneOldEvents(t *testing.T) {
	dir := t.TempDir()
	window := 100 * time.Millisecond
	sup := New(dir, Config{Threshold: 3, Window: window})

	// Add crash
	_ = sup.RecordExit(0, syscall.SIGKILL, false)

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Add another event to trigger prune
	_ = sup.RecordExit(0, 0, false)

	// First crash should be pruned
	crashCount := 0
	for _, e := range sup.state.Events {
		if e.IsCrash() {
			crashCount++
		}
	}
	if crashCount != 0 {
		t.Errorf("Expected 0 crashes after prune, got %d", crashCount)
	}
}

func TestShouldSkipDetection_TestMode(t *testing.T) {
	// Set test mode
	os.Setenv("FLYWALL_TEST_MODE", "1")
	defer os.Unsetenv("FLYWALL_TEST_MODE")

	if !ShouldSkipDetection() {
		t.Error("Should skip detection in test mode")
	}
}
