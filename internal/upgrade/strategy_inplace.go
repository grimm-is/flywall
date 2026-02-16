// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"grimm.is/flywall/internal/brand"
)

// InPlaceStrategy implements the standard upgrade strategy:
// 1. Stage: Copy new binary to {current_dir}/{brand}_new
// 2. Finalize: Rename {current_dir}/{brand}_new to {current_dir}/{brand}
type InPlaceStrategy struct {
	// getExecutablePath returns the path to the currently running executable.
	// Can be mocked for testing.
	getExecutablePath func() (string, error)
}

// NewInPlaceStrategy creates a new in-place upgrade strategy.
func NewInPlaceStrategy() *InPlaceStrategy {
	return &InPlaceStrategy{
		getExecutablePath: os.Executable,
	}
}

// Stage copies the source binary to the staging location alongside the current executable.
func (s *InPlaceStrategy) Stage(ctx context.Context, sourcePath string) (string, error) {
	// Determines where we are running now
	currentExe, err := s.getExecutablePath()
	if err != nil {
		return "", fmt.Errorf("failed to determine current executable path: %w", err)
	}
	installDir := filepath.Dir(currentExe)

	// Target staged path: e.g. /usr/sbin/flywall_new
	stagedPath := filepath.Join(installDir, brand.BinaryName+"_new")

	// Ensure source exists
	srcInfo, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("source binary not found: %w", err)
	}
	if srcInfo.Mode()&0111 == 0 {
		return "", fmt.Errorf("source binary is not executable")
	}

	// Copy binary
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source binary: %w", err)
	}
	defer srcFile.Close()

	// Open destination with executable permissions
	dstFile, err := os.OpenFile(stagedPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to open staging path '%s' (check write permissions): %w", stagedPath, err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}
	if err := dstFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close staged binary: %w", err)
	}

	return stagedPath, nil
}

// Finalize renames the currently running executable to the final target name.
// This is intended to be called by the *new* process when it starts up.
func (s *InPlaceStrategy) Finalize(ctx context.Context) error {
	executable, err := s.getExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Calculate target path: e.g. /usr/sbin/flywall
	targetPath := filepath.Join(filepath.Dir(executable), brand.BinaryName)

	// If we are already named correctly, do nothing
	if executable == targetPath {
		return nil
	}

	// Remove old binary first (handles ETXTBSY if old process still has it mapped)
	// We proceed even if remove fails (e.g. it doesn't exist)
	_ = os.Remove(targetPath)

	// Rename current (new) binary to target
	if err := os.Rename(executable, targetPath); err != nil {
		return fmt.Errorf("failed to rename binary from %s to %s: %w", executable, targetPath, err)
	}

	return nil
}

// CalculateChecksum computes the SHA256 hash of a file.
// Helper for verification.
func CalculateChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
