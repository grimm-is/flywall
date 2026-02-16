// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux
// +build !linux

package socket

import (
	"time"
)

func (dhcp *DHCPFilter) getKtime() uint64 {
	// Fallback to wall clock on non-linux platforms for stubs
	return uint64(time.Now().UnixNano())
}
