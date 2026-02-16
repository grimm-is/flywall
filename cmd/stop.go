// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cmd

import (
	"grimm.is/flywall/internal/install"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/health"
)

// RunStop stops the control plane daemon
func RunStop() error {
	runDir := install.GetRunDir()
	pidFile := filepath.Join(runDir, brand.LowerName+".pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no PID file found at %s (is daemon running?)", pidFile)
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return fmt.Errorf("invalid PID in file: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	Printer.Printf("Stopping %s (PID: %d)...\n", brand.Name, pid)
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for PID file to disappear (daemon should remove it)
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(pidFile); os.IsNotExist(err) {
			// Clear crash state on clean shutdown
			stateDir := install.GetStateDir()
			crashFile := filepath.Join(stateDir, health.StateFileName)
			os.Remove(crashFile) // Ignore error - file may not exist

			Printer.Println("Stopped.")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	Printer.Println("Warning: PID file still exists. Process might be stuck or slow to shutdown.")
	return nil
}
