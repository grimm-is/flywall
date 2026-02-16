// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ctlplane"
	"grimm.is/flywall/internal/install"
	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/tsnet"
)

// RunTsNet implements the 'fw tsnet' command
func RunTsNet(args []string) error {
	fs := flag.NewFlagSet("tsnet", flag.ExitOnError)
	var configPath string
	var targetAddr string

	fs.StringVar(&configPath, "c", "/etc/flywall/flywall.hcl", "Path to configuration file")
	fs.StringVar(&targetAddr, "target", "localhost:8080", "Target API address to proxy to")

	// Parse flags
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Determine if config file was explicitly set
	explicitConfig := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "c" {
			explicitConfig = true
		}
	})

	var cfg *config.Config
	var err error

	// If not explicit, try to fetch from running daemon first
	if !explicitConfig {
		// Try to connect to daemon
		client, err := ctlplane.NewClient()
		if err == nil {
			defer client.Close()
			if runningCfg, err := client.GetRunningConfig(); err == nil {
				cfg = runningCfg
				logging.Info("Loaded configuration from running daemon")
			} else {
				// We connected but failed to get config
				logging.Debug(fmt.Sprintf("Connected to daemon but failed to get config: %v", err))
			}
		} else {
			// Failed to connect
			// Log to debug/stderr but don't fail yet, as we might fall back to file
			// However, since fallback to file is likely to fail in demo environment, this info is crucial.
			// Given user complaint, I'll print a warning if this fails AND file load fails later.
			// But I can't know future failure here.
			// I'll stick to Debug logging, but maybe user needs to see it?
			// The user sees "failed to load config from ...".
			// If I add a log here, it might help.
			// "TsNet failed: ..." is printed by main.go catch.
			logging.Debug(fmt.Sprintf("Failed to connect to daemon at %s: %v", ctlplane.GetSocketPath(), err))
		}
	}

	// Fallback to file if failed or explicit
	if cfg == nil {
		cfg, err = config.LoadWithDefaults(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config from %s: %w", configPath, err)
		}
		logging.Info(fmt.Sprintf("Loaded configuration from %s", configPath))
	}

	// Ensure StateDir is set (vital for tsnet to store tailscale state)
	if cfg.StateDir == "" {
		// Use GetStateDir() to respect environment variables (e.g. FLYWALL_STATE_DIR)
		cfg.StateDir = install.GetStateDir()
		logging.Debug(fmt.Sprintf("StateDir not set in config, defaulting to %s", cfg.StateDir))
	}

	if cfg.API == nil || cfg.API.TsNet == nil || !cfg.API.TsNet.Enabled {
		// Allow running even if not enabled in file? Maybe via flag?
		// For now, respect config (as per plan "TsNet Config struct")
		// Or maybe we treat `fw tsnet` as an explicit override?
		// "Minimal memory footprint if not enabled" implies we don't start it in main daemon.
		// So `fw tsnet` IS the enabler.
		// But it needs config to know AuthKey/Hostname.
		if cfg.API == nil {
			fmt.Println("Error: API configuration missing in flywall.hcl")
			return nil
		}
		if cfg.API.TsNet == nil {
			fmt.Println("Error: tsnet configuration block missing in flywall.hcl")
			return nil
		}
	}

	// Setup Logging
	// Check env for log level override
	logLevel := logging.LevelInfo
	if lvl := os.Getenv("FLYWALL_LOG_LEVEL"); lvl != "" {
		switch lvl {
		case "debug":
			logLevel = logging.LevelDebug
		case "warn":
			logLevel = logging.LevelWarn
		case "error":
			logLevel = logging.LevelError
		}
	} else if os.Getenv("DEBUG") != "" {
		logLevel = logging.LevelDebug
	}

	logging.SetDefault(logging.New(logging.Config{
		Level:  logLevel,
		Output: os.Stdout,
		JSON:   false,
	}))

	// Create and Start Service
	srv := tsnet.NewServer(cfg.API.TsNet, cfg.StateDir, targetAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logging.Info("Stopping tsnet...")
		cancel()
	}()

	logging.Info("Starting Flywall Tailscale Proxy...")
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("tsnet service error: %w", err)
	}

	return nil
}
