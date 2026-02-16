// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"grimm.is/flywall/internal/install"

	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/config"
)

// RunStart starts the control plane daemon in the background
func RunStart(configFile string) error {
	// 0. Pre-flight check: verify config file exists before forking
	// This gives immediate feedback rather than failing in background
	if configFile == "" {
		configFile = filepath.Join(install.DefaultConfigDir, brand.ConfigFileName)
	}
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("configuration file not found: %s\n\n"+
			"To create a configuration file, run:\n"+
			"  %s setup\n\n"+
			"Or create a minimal config manually:\n"+
			"Or create a minimal config manually:\n"+
			"  mkdir -p "+install.DefaultConfigDir+"\n"+
			"  echo 'schema_version = \"1.0\"' > "+filepath.Join(install.DefaultConfigDir, brand.ConfigFileName),
			configFile, brand.BinaryName)
	}

	// 0b. Pre-flight: validate config before forking
	// This catches config errors early and displays them to the user
	if err := validateConfigFile(configFile); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// 1. Check for existing PID file
	runDir := install.GetRunDir()
	pidFile := filepath.Join(runDir, brand.LowerName+".pid")

	if _, err := os.Stat(pidFile); err == nil {
		// File exists, check if process is actually running
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pid, err := strconv.Atoi(string(data))
			if err == nil {
				// Check if process exists by sending signal 0
				process, err := os.FindProcess(pid)
				if err == nil {
					if err := process.Signal(syscall.Signal(0)); err == nil {
						return fmt.Errorf("process already running (PID: %d)", pid)
					}
				}
			}
		}
		// If we get here, PID file exists but process is dead. Warn and cleanup.
		Printer.Printf("Warning: Removing stale PID file %s\n", pidFile)
		os.Remove(pidFile)
	}

	// 2. Prepare command
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Parse config to check for custom log_dir and state_dir
	var logDir = install.GetLogDir()
	var stateDirOverride string

	if configFile != "" {
		if res, err := config.LoadFileWithOptions(configFile, config.DefaultLoadOptions()); err == nil {
			if res.Config.LogDir != "" {
				logDir = res.Config.LogDir
			}
			if res.Config.StateDir != "" {
				stateDirOverride = res.Config.StateDir
			}
		}
	}

	// We start 'ctl' command. 'ctl' will handle spawning 'api'.
	args := []string{"ctl"}
	if stateDirOverride != "" {
		args = append(args, "-state-dir", stateDirOverride)
	}
	if configFile != "" {
		args = append(args, configFile)
	}

	cmd := exec.Command(exe, args...)

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	logFile := filepath.Join(logDir, brand.LowerName+".log")

	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logF.Close()

	cmd.Stdout = logF
	cmd.Stderr = logF

	// 4. Detach process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// 5. Start
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	pid := cmd.Process.Pid
	Printer.Printf("Started %s (PID: %d)\n", brand.Name, pid)
	Printer.Printf("Logs: %s\n", logFile)

	// 6. Wait briefly to detect immediate failures
	// Use a channel to detect if the process exits quickly
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		// Process exited quickly - this is a startup failure
		Printer.Fprintf(os.Stderr, "\nError: Daemon exited immediately.\n")
		// Try to show recent log lines for context
		if content, readErr := os.ReadFile(logFile); readErr == nil {
			lines := strings.Split(string(content), "\n")
			// Show last 10 lines
			start := len(lines) - 10
			if start < 0 {
				start = 0
			}
			Printer.Fprintf(os.Stderr, "Log output:\n")
			for _, line := range lines[start:] {
				if line != "" {
					Printer.Fprintf(os.Stderr, "  %s\n", line)
				}
			}
		}
		// The provided change was syntactically incorrect and could not be applied as-is.
		// It seems to be attempting to introduce a 'foreground' flag and a call to 'RunCtl'.
		// Since 'RunCtl' is not defined in this file and the snippet was malformed,
		// I'm preserving the original logic here.
		// If you intended to add a new 'RunCtl' call or modify the flow based on a 'foreground' flag,
		// please provide a complete and syntactically correct code block.
		if err != nil {
			return fmt.Errorf("daemon failed to start: %w", err)
		}
		return fmt.Errorf("daemon exited unexpectedly")

	case <-time.After(500 * time.Millisecond):
		// Process is still running after 500ms - likely successful
		// Verify it's still alive
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return fmt.Errorf("daemon died during startup (check logs: %s)", logFile)
		}
		return nil
	}
}

// tailLogFile returns the last n lines of a log file
func tailLogFile(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return lines
}

// validateConfigFile loads and validates the config file without starting services
func validateConfigFile(configFile string) error {
	_, err := config.LoadFileWithOptions(configFile, config.DefaultLoadOptions())
	if err != nil {
		return err
	}
	return nil
}
