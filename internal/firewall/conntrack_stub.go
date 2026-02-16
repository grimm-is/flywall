// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux

package firewall

import (
	"fmt"
)

// GetConntrackEntries is a stub for non-Linux platforms.
// Connection tracking is only supported on Linux via netlink.
func GetConntrackEntries() ([]ConntrackEntry, error) {
	return nil, fmt.Errorf("conntrack not supported on this platform")
}
