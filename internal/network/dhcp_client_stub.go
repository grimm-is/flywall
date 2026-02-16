// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux
// +build !linux

package network

import "log"

// StartNativeDHCPClient is a stub for non-Linux systems.
func (m *Manager) StartNativeDHCPClient(ifaceName string, tableID int) error {
	log.Printf("[network] [DRY RUN] Starting DHCP Client on interface '%s' (Table: %d). (Not supported on non-Linux, simulation only)", ifaceName, tableID)
	return nil
}
