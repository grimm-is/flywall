// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux
// +build !linux

package cmd

// SetProcessName stub.
func SetProcessName(name string) error {
	return nil
}
