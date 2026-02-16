// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux

package agent

import (
	"fmt"
	"os"
	"syscall"
)

func ensureDevPts() error {
	// 1. Create mount point
	if err := os.MkdirAll("/dev/pts", 0755); err != nil {
		return fmt.Errorf("mkdir /dev/pts: %w", err)
	}

	// 2. Unmount existing mounts (clean slate)
	// We loop until unmount fails to clear stacked mounts
	for {
		if err := syscall.Unmount("/dev/pts", 0); err != nil {
			break
		}
	}

	// 3. Mount devpts with proper options
	// mode=620 (rw-w----), gid=5 (tty), ptmxmode=666 (rw-rw-rw-)
	err := syscall.Mount("devpts", "/dev/pts", "devpts", 0, "mode=620,ptmxmode=666,gid=5")
	if err != nil {
		return fmt.Errorf("mount devpts: %w", err)

	}

	// 4. Ensure /dev/ptmx
	// Some systems (like Alpine in container/VM) might lack /dev/ptmx or rely on /dev/pts/ptmx
	if _, err := os.Stat("/dev/ptmx"); os.IsNotExist(err) {
		if err := os.Symlink("/dev/pts/ptmx", "/dev/ptmx"); err != nil {
			// Fallback to mknod (c 5 2)
			// Mode 0666 | S_IFCHR
			if err := syscall.Mknod("/dev/ptmx", 0666|syscall.S_IFCHR, 5<<8|2); err != nil {
				fmt.Fprintf(os.Stderr, "[Agent] Warning: failed to create /dev/ptmx: %v\n", err)
			}
		}
	}

	return nil
}
