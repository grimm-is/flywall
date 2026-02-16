// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package upgrade

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"grimm.is/flywall/internal/brand"
)

func TestInPlaceStrategy_Stage(t *testing.T) {
	// Create temp directory simulating install dir
	tmpDir, err := os.MkdirTemp("", "upgrade-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy "current" binary
	currentExe := filepath.Join(tmpDir, brand.BinaryName)
	if err := os.WriteFile(currentExe, []byte("old binary content"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a dummy "new" binary
	newBinaryPath := filepath.Join(tmpDir, "upload_temp_file")
	newContent := []byte("new binary content")
	if err := os.WriteFile(newBinaryPath, newContent, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize strategy with mocked executable path
	strategy := NewInPlaceStrategy()
	strategy.getExecutablePath = func() (string, error) {
		return currentExe, nil
	}

	// Test Stage
	stagedPath, err := strategy.Stage(context.Background(), newBinaryPath)
	if err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// Verify staged path is correct
	expectedStaged := filepath.Join(tmpDir, brand.BinaryName+"_new")
	if stagedPath != expectedStaged {
		t.Errorf("expected staged path %s, got %s", expectedStaged, stagedPath)
	}

	// Verify content was copied
	content, err := os.ReadFile(stagedPath)
	if err != nil {
		t.Fatalf("failed to read staged file: %v", err)
	}
	if string(content) != string(newContent) {
		t.Errorf("content mismatch")
	}

	// Verify permissions
	info, err := os.Stat(stagedPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0700 != 0700 {
		t.Errorf("expected executable permissions, got %v", info.Mode())
	}
}

func TestInPlaceStrategy_Finalize(t *testing.T) {
	// Create temp directory simulating install dir
	tmpDir, err := os.MkdirTemp("", "upgrade-finalize-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// In the real world, the "new" binary is running as {brand}_new (or brand, but let's simulate the _new start case)
	// Wait, Finalize relies on getting filepath.Dir(executable) and renaming executable to target.

	// Case 1: Running as flywall_new, need to rename to flywall

	// Setup: Binary running as flywall_new
	runningExe := filepath.Join(tmpDir, brand.BinaryName+"_new")
	if err := os.WriteFile(runningExe, []byte("new content"), 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-existing old binary (should be overwritten)
	targetExe := filepath.Join(tmpDir, brand.BinaryName)
	if err := os.WriteFile(targetExe, []byte("old content"), 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize strategy
	strategy := NewInPlaceStrategy()
	strategy.getExecutablePath = func() (string, error) {
		return runningExe, nil
	}

	// Test Finalize
	if err := strategy.Finalize(context.Background()); err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// Verify: runningExe (flywall_new) should be gone (renamed)
	if _, err := os.Stat(runningExe); !os.IsNotExist(err) {
		t.Errorf("expected %s to be gone (renamed)", runningExe)
	}

	// Verify: targetExe (flywall) should contain new content
	content, err := os.ReadFile(targetExe)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new content" {
		t.Errorf("target file does not contain new content")
	}
}
