// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux
// +build !linux

package ha

import (
	"fmt"
)

// Stub implementations for non-Linux platforms.

func parseAddr(addrStr string) (interface{}, error) {
	return nil, fmt.Errorf("not supported on this platform")
}

func addIPAddress(ifaceName string, addr interface{}, label string) error {
	return fmt.Errorf("not supported on this platform")
}

func removeIPAddress(ifaceName string, addr interface{}) error {
	return fmt.Errorf("not supported on this platform")
}
