// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

// Package supervisor provides intelligent process crash detection.
// Unlike simple restart counters, it tracks HOW processes exit and
// only counts actual crashes (SIGKILL, SIGSEGV, panics) toward
// the safe mode threshold.
package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/term"
)

const (
	// DefaultThreshold is the number of actual crashes before entering safe mode.
	DefaultThreshold = 3
	// DefaultWindow is the time window for counting crashes.
	DefaultWindow = 5 * time.Minute
	// StateFileName is the crash state persistence file.
	StateFileName = "supervisor.state"
)

// Config holds supervisor configuration.
type Config struct {
	Threshold int
	Window    time.Duration
}

// DefaultConfig returns the default supervisor configuration.
func DefaultConfig() Config {
	return Config{
		Threshold: DefaultThreshold,
		Window:    DefaultWindow,
	}
}

// CrashEvent records a single crash occurrence.
type CrashEvent struct {
	ExitCode  int            `json:"exit_code"`
	Signal    syscall.Signal `json:"signal"`
	Timestamp time.Time      `json:"timestamp"`
	WasPanic  bool           `json:"was_panic"`
}

// IsCrash returns true if this event represents an actual crash
// (as opposed to a clean exit or requested stop).
func (e CrashEvent) IsCrash() bool {
	// Panics are always crashes
	if e.WasPanic {
		return true
	}

	// Fatal signals are crashes
	switch e.Signal {
	case syscall.SIGKILL, syscall.SIGSEGV, syscall.SIGBUS, syscall.SIGABRT:
		return true
	case syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP:
		return false // Requested stop
	}

	// Exit 0 is clean
	if e.ExitCode == 0 {
		return false
	}

	// Non-zero exit without a signal - treat as crash
	// (could be made configurable)
	return true
}

// State holds persisted crash history.
type State struct {
	Events []CrashEvent `json:"events"`
}

// Supervisor manages crash detection and safe mode decisions.
type Supervisor struct {
	config   Config
	stateDir string
	state    State
}

// New creates a new Supervisor.
func New(stateDir string, config Config) *Supervisor {
	s := &Supervisor{
		config:   config,
		stateDir: stateDir,
	}
	_ = s.loadState() // Best-effort load
	return s
}

// ShouldSkipDetection returns true if crash detection should be
// bypassed for this environment (test mode, interactive, non-service).
func ShouldSkipDetection() bool {
	// Explicit test mode
	if os.Getenv("FLYWALL_TEST_MODE") != "" {
		return true
	}

	// Interactive terminal session
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}

	// Not running under init/systemd
	// INVOCATION_ID is set by systemd for service invocations
	if os.Getppid() != 1 && os.Getenv("INVOCATION_ID") == "" {
		return true
	}

	return false
}

// ShouldEnterSafeMode returns true if too many crashes have occurred
// within the configured window.
func (s *Supervisor) ShouldEnterSafeMode() bool {
	s.pruneOldEvents()

	crashCount := 0
	for _, e := range s.state.Events {
		if e.IsCrash() {
			crashCount++
		}
	}

	return crashCount >= s.config.Threshold
}

// RecordExit records a process exit event.
// wasPanic should be true if a panic was recovered before exit.
func (s *Supervisor) RecordExit(exitCode int, signal syscall.Signal, wasPanic bool) error {
	event := CrashEvent{
		ExitCode:  exitCode,
		Signal:    signal,
		Timestamp: time.Now(),
		WasPanic:  wasPanic,
	}

	s.state.Events = append(s.state.Events, event)
	s.pruneOldEvents()

	return s.saveState()
}

// Reset clears the crash history (called after stable uptime).
func (s *Supervisor) Reset() error {
	s.state.Events = nil
	return s.saveState()
}

// StartStabilityTimer resets crash history after the window duration
// of stable operation.
func (s *Supervisor) StartStabilityTimer() {
	go func() {
		time.Sleep(s.config.Window)
		_ = s.Reset()
	}()
}

// pruneOldEvents removes events outside the tracking window.
func (s *Supervisor) pruneOldEvents() {
	cutoff := time.Now().Add(-s.config.Window)
	filtered := make([]CrashEvent, 0, len(s.state.Events))
	for _, e := range s.state.Events {
		if e.Timestamp.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	s.state.Events = filtered
}

func (s *Supervisor) statePath() string {
	return filepath.Join(s.stateDir, StateFileName)
}

func (s *Supervisor) loadState() error {
	data, err := os.ReadFile(s.statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state yet
		}
		return err
	}

	if err := json.Unmarshal(data, &s.state); err != nil {
		// Corrupt state - reset
		s.state = State{}
	}
	return nil
}

func (s *Supervisor) saveState() error {
	if err := os.MkdirAll(s.stateDir, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(s.state)
	if err != nil {
		return err
	}

	return os.WriteFile(s.statePath(), data, 0644)
}
