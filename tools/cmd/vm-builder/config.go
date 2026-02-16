// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// VMConfig defines the Virtual Machine configuration
// VMConfig defines the Virtual Machine configuration
type VMConfig struct {
	Name        string          `yaml:"name"`
	OS          OSConfig        `yaml:"os"`
	Packages    []string        `yaml:"packages"`
	PostInstall []ProvisionStep `yaml:"post_install"`
}

// OSConfig defines the operating system parameters
type OSConfig struct {
	Distro  string `yaml:"distro"`
	Version string `yaml:"version"`
	Release string `yaml:"release"`
	Mirror  string `yaml:"mirror"`
}

// ProvisionStep represents a single provisioning step
type ProvisionStep struct {
	Name string    `yaml:"name"`
	Run  string    `yaml:"run,omitempty"`  // Shell script to run
	Copy *CopyStep `yaml:"copy,omitempty"` // File copy operation
}

// CopyStep defines a file copy operation
type CopyStep struct {
	Source string `yaml:"source"`
	Dest   string `yaml:"dest"`
	Mode   string `yaml:"mode"` // File mode (e.g., "0755")
}

// LoadConfig reads and parses a VM configuration file
func LoadConfig(path string) (*VMConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config VMConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}
