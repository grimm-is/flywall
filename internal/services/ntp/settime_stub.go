// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux

package ntp

import (
	"fmt"
	"time"
)

// setSystemTime is a stub for non-Linux platforms.
// NTP time sync only works on Linux where we have CAP_SYS_TIME.
func setSystemTime(t time.Time) error {
	return fmt.Errorf("setSystemTime not implemented on this platform")
}
