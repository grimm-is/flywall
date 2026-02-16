// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

// Command flywall-sim replays PCAPs through Flywall's learning and discovery engines.
// It evaluates anomaly detection by comparing baseline traffic against attack captures.
package main

import (
	"flag"
	"log"
	"os"
	"time"

	"grimm.is/flywall/internal/clock"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/kernel"
	"grimm.is/flywall/internal/learning"
)

func main() {
	configPath := flag.String("config", "", "Path to HCL config file")
	serverMode := flag.Bool("server", false, "Run in server mode (default if no subcommand)")
	flag.Parse()

	args := flag.Args()
	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
	}

	// Client Mode: Replay command
	if subcmd == "replay" {
		if len(args) < 2 {
			log.Fatal("Usage: flywall-sim replay <pcap-file>")
		}
		pcapFile := args[1]
		if err := SendReplayCommand(pcapFile); err != nil {
			log.Fatalf("Replay command failed: %v", err)
		}
		return
	}

	// Server Mode (Default)
	if subcmd == "server" || *serverMode || subcmd == "" {
		runServer(*configPath)
		return
	}

	log.Fatalf("Unknown command: %s", subcmd)
}

func runServer(configPath string) {
	// Initialize Simulation Components
	clk := clock.NewMockClock(time.Now())
	simKernel := kernel.NewSimKernel(clk)

	// Load Config if provided
	var cfg *config.Config
	if configPath != "" {
		log.Printf("Loading config from %s...", configPath)
		var err error
		cfg, err = config.LoadFile(configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	} else {
		// Default config
		cfg = &config.Config{
			API: &config.APIConfig{
				Listen: ":8080",
			},
		}
	}

	// Setup Learning Engine
	// Use a temp file for flow DB so we can inspect it? Or memory?
	// FlowDB supports sqlite. Use a temp file.
	dbPath := "sim_flow.db"

	// Ensure cleanup of old DB
	os.Remove(dbPath)

	engineConfig := learning.EngineConfig{
		DBPath:       dbPath,
		Logger:       nil,  // Will use default logger
		LearningMode: true, // Start in Learning Mode
		Config:       cfg.RuleLearning,
	}

	// Create engine
	engine, err := learning.NewEngine(engineConfig)
	if err != nil {
		log.Fatalf("Failed to create learning engine: %v", err)
	}
	engine.SetLearningMode(true)

	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}
	defer engine.Stop()

	// Start Server
	if err := StartServer(cfg, simKernel, engine, clk); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
