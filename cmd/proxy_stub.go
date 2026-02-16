// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux

package cmd

// setupProxyChroot is a no-op on non-Linux systems
func setupProxyChroot(jailPath, hostSocketPath string) error {
	return nil
}
