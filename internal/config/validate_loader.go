// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadAndValidate loads a config file with full validation including cross-references
func LoadAndValidate(path string) (*Config, ValidationErrors, error) {
	return LoadAndValidateWithOptions(path, DefaultLoadOptions())
}

// LoadAndValidateWithOptions loads a config file with options and performs deep validation
func LoadAndValidateWithOptions(path string, opts LoadOptions) (*Config, ValidationErrors, error) {
	// Load the config
	result, err := LoadFileWithOptions(path, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	// Perform deep validation including cross-references
	validationErrors := result.Config.DeepValidate()
	
	// If there are error-level validation failures, return them
	var errs ValidationErrors
	for _, ve := range validationErrors {
		if ve.Severity != "warning" && ve.Severity != "info" {
			errs = append(errs, ve)
		}
	}
	
	if len(errs) > 0 {
		return result.Config, validationErrors, fmt.Errorf("configuration validation failed")
	}
	
	return result.Config, validationErrors, nil
}

// ValidateConfigFile validates an existing config file without loading it
func ValidateConfigFile(path string) ValidationErrors {
	// Load config without migration for validation
	data, err := os.ReadFile(path)
	if err != nil {
		return ValidationErrors{ValidationError{
			Field:   "file",
			Message: fmt.Sprintf("failed to read: %v", err),
		}}
	}
	
	ext := strings.ToLower(filepath.Ext(path))
	var cfg *Config
	
	switch ext {
	case ".hcl":
		cfg, err = LoadHCL(data, path)
	case ".json":
		cfg, err = LoadJSON(data)
	default:
		// Try HCL first
		cfg, err = LoadHCL(data, path)
		if err != nil {
			// Try JSON
			cfg, err = LoadJSON(data)
		}
	}
	
	if err != nil {
		return ValidationErrors{ValidationError{
			Field:   "parse",
			Message: fmt.Sprintf("failed to parse: %v", err),
		}}
	}
	
	// Run deep validation
	return cfg.DeepValidate()
}
