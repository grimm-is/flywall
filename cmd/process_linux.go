// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux
// +build linux

package cmd

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// SetProcessName sets the process name using prctl (Linux only).
func SetProcessName(name string) error {
	bytes := append([]byte(name), 0)
	return unix.Prctl(unix.PR_SET_NAME, uintptr(unsafe.Pointer(&bytes[0])), 0, 0, 0)
}
