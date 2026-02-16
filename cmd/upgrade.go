// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cmd

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"grimm.is/flywall/internal/install"

	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/ctlplane"
	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/upgrade"
)

// RunUpgrade initiates a seamless upgrade to a new binary.
func RunUpgrade(newBinaryPath, configPath string) {
	logger := logging.New(logging.DefaultConfig())
	logger.Info("Starting seamless upgrade process")

	// Verify new binary exists and is executable
	info, err := os.Stat(newBinaryPath)
	if err != nil {
		logger.Error("New binary not found", "path", newBinaryPath, "error", err)
		os.Exit(1)
	}
	if info.Mode()&0111 == 0 {
		logger.Error("New binary is not executable", "path", newBinaryPath)
		os.Exit(1)
	}

	// Connect to control plane
	client, err := ctlplane.NewClient()
	if err != nil {
		logger.Error("Failed to connect to control plane (is the daemon running?)", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	// Initialize upgrade strategy
	strategy := upgrade.NewInPlaceStrategy()

	// Stage the binary (copy to secure location)
	securePath, err := strategy.Stage(context.Background(), newBinaryPath)
	if err != nil {
		logger.Error("Failed to stage binary", "error", err)
		os.Exit(1)
	}
	logger.Info("Binary staged successfully", "path", securePath)

	// Calculate checksum of the binary we just copied
	// We re-open the file at securePath to ensure we hash exactly what the server will see
	hashFile, err := os.Open(securePath)
	if err != nil {
		logger.Error("Failed to read back staged binary", "error", err)
		os.Exit(1)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, hashFile); err != nil {
		hashFile.Close() // Close on error
		logger.Error("Failed to calculate checksum", "error", err)
		os.Exit(1)
	}
	// Explicitly close before upgrading (avoid "text file busy")
	hashFile.Close()

	checksum := hex.EncodeToString(hasher.Sum(nil))
	logger.Info("Calculated checksum", "hash", checksum)

	// Call Upgrade RPC
	if err := client.Upgrade(checksum); err != nil {
		logger.Error("Upgrade failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Upgrade initiated successfully. The daemon is restarting with the new binary.")
	os.Exit(0)
}

// RunUpgradeStandby runs the new process in standby mode during upgrade.
func RunUpgradeStandby(configPath string, uiAssets embed.FS) {
	// Set process name to "flywall" immediately to hide "flywall_new" origin from ps
	if err := SetProcessName("flywall"); err != nil {
		// Ignore error, purely cosmetic
	}

	logger := logging.New(logging.DefaultConfig())
	logger.Info("Starting in upgrade standby mode")

	// Manage PID file with Watchdog (Self-Healing)
	// Key component for upgrade stability: ensures identity is preserved
	runDir := install.GetRunDir()
	pidFile := filepath.Join(runDir, brand.LowerName+".pid")

	writePID := func() error {
		return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	}

	if err := writePID(); err != nil {
		logger.Error("Failed to write PID file", "error", err)
		// Continue anyway?
	}

	// Start Keeper/Watchdog
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			// Check if file exists and is correct
			data, err := os.ReadFile(pidFile)
			if err != nil || strings.TrimSpace(string(data)) != fmt.Sprintf("%d", os.Getpid()) {
				// Restore it
				logger.Info("Restoring PID file (detected missing or invalid)")
				_ = writePID()
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// Create upgrade manager
	mgr := upgrade.NewManager(logger)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Received shutdown signal")
		cancel()
	}()

	// Run standby mode - this will:
	// 1. Load state from old process
	// 2. Validate configuration
	// 3. Signal ready to old process
	// 4. Receive listener file descriptors
	if err := mgr.RunStandby(ctx, configPath); err != nil {
		logger.Error("Standby mode failed", "error", err)
		os.Exit(1)
	}

	// Load configuration
	// RunCtl will load the config itself, so we don't strictly need to load it here
	// unless we wanted to validate it before calling RunCtl.
	// But RunCtl does its own validation/loading.

	// Now we have the listeners, start the services
	logger.Info("Taking over from old process (RunUpgradeStandby -> RunCtl)")

	// Prepare listeners map
	listeners := make(map[string]interface{})
	if ctlListener, ok := mgr.GetListener("ctl"); ok {
		logger.Info("Using handed-off Control Plane listener", "addr", ctlListener.Addr())
		listeners["ctl"] = ctlListener
	}
	if apiListener, ok := mgr.GetListener("api"); ok {
		logger.Info("Using handed-off API listener", "addr", apiListener.Addr())
		listeners["api"] = apiListener
		// Note: RunCtl doesn't use listeners["api"] directly for Start(),
		// but uses it in spawnAPI to extract the file descriptor.
	}

	// Cleanup phase: Ensure no orphaned processes from previous run are holding the lock
	// This is required because older versions might leak the 'ip' process or its children.
	// We query the PIDs inside the namespace and SIGKILL them.
	// This ensures the lock file is released and we can start fresh.
	cleanupCmd := exec.Command("ip", "netns", "pids", brand.LowerName+"-api")
	if out, err := cleanupCmd.Output(); err == nil && len(out) > 0 {
		pids := strings.Fields(string(out))
		if len(pids) > 0 {
			logger.Info("Cleaning up orphaned API processes from previous instance", "count", len(pids))
			for _, pidStr := range pids {
				if pid, err := strconv.Atoi(pidStr); err == nil {
					p, _ := os.FindProcess(pid)
					p.Signal(syscall.SIGKILL)
				}
			}
			// Give them a moment to die and for 'ip' wrapper to exit/release lock
			time.Sleep(1 * time.Second)
		}
	}

	// Finalization: Rename binary BEFORE calling RunCtl
	// This ensures restart/crash loops use the correct (new) binary.
	strategy := upgrade.NewInPlaceStrategy()
	if err := strategy.Finalize(context.Background()); err != nil {
		logger.Error("Failed to finalize upgrade (rename failed)", "error", err)
		// We continue anyway; running as flywall_new is acceptable but not ideal.
	} else {
		logger.Info("Upgrade finalized: binary renamed to standard name")
	}

	// Debug: Log listener count
	logger.Info("Calling RunCtl with injected listeners", "count", len(listeners), "hasCtl", listeners["ctl"] != nil, "hasApi", listeners["api"] != nil)

	// Call RunCtl with injected listeners
	// This unifies the code path, ensuring full functionality (Network Manager, Watchdog, etc.)
	if err := RunCtl(configPath, false, "", "", "", "", false, listeners); err != nil {
		logger.Error("Control plane failed", "error", err)
		os.Exit(1)
	}
}
