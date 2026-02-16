// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cmd

import (
	"grimm.is/flywall/internal/config"
)

// CtlRuntimeConfig holds all runtime configuration for the control plane.
type CtlRuntimeConfig struct {
	ConfigFile string
	TestMode   bool
	StateDir   string
	LogDir     string
	RunDir     string
	ShareDir   string
	DryRun     bool
	Listeners  map[string]interface{}
	IsUpgrade  bool

	// Results from Salvage load
	ForgivingResult *config.ForgivingLoadResult
}

// NewCtlRuntimeConfig creates runtime configuration from CLI args.
func NewCtlRuntimeConfig(configFile string, testMode bool, stateDir, logDir, runDir, shareDir string, dryRun bool, listeners map[string]interface{}) *CtlRuntimeConfig {
	return &CtlRuntimeConfig{
		ConfigFile: configFile,
		TestMode:   testMode,
		StateDir:   stateDir,
		LogDir:     logDir,
		RunDir:     runDir,
		ShareDir:   shareDir,
		DryRun:     dryRun,
		Listeners:  listeners,
		IsUpgrade:  len(listeners) > 0,
	}
}
