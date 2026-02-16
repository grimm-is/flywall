// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// SecureWriteFile writes a file with secure permissions (0600)
func SecureWriteFile(filename string, data []byte) error {
	// Create directory with secure permissions if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temporary file first
	tempFile := filename + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Verify file permissions
	if err := setSecurePermissions(tempFile); err != nil {
		os.Remove(tempFile) // Clean up on error
		return fmt.Errorf("failed to set secure permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, filename); err != nil {
		os.Remove(tempFile) // Clean up on error
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

func setSecurePermissions(filename string) error {
	// Set ownership to current user only
	uid := os.Getuid()
	gid := os.Getgid()

	if err := os.Chown(filename, uid, gid); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}

	// Set restrictive permissions
	if err := os.Chmod(filename, 0600); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}

// SecureReadFile reads a file with permission validation
func SecureReadFile(filename string) ([]byte, error) {
	// Check file permissions before reading
	info, err := os.Stat(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Verify file is owned by current user and has secure permissions
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if int(stat.Uid) != os.Getuid() {
			return nil, fmt.Errorf("file is not owned by current user")
		}

		// Check that permissions are not too permissive
		mode := info.Mode()
		if mode&0077 != 0 { // Group and others have permissions
			return nil, fmt.Errorf("file has insecure permissions: %s", mode.String())
		}
	}

	return os.ReadFile(filename)
}
