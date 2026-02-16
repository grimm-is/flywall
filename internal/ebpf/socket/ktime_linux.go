// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux
// +build linux

package socket

import (
	"golang.org/x/sys/unix"
	"time"
)

func (dhcp *DHCPFilter) getKtime() uint64 {
	// Roughly equivalent to bpf_ktime_get_ns() which is CLOCK_MONOTONIC
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		return uint64(time.Now().UnixNano())
	}
	return uint64(ts.Sec)*1e9 + uint64(ts.Nsec)
}
