// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// LoadOptions controls how configs are loaded
type LoadOptions struct {
	// AutoMigrate automatically migrates old configs to current version
	AutoMigrate bool

	// StrictVersion fails if config version doesn't match current
	StrictVersion bool

	// AllowUnknownFields ignores unknown HCL fields (useful for forward compat)
	AllowUnknownFields bool
}

// DefaultLoadOptions returns sensible defaults for loading configs
func DefaultLoadOptions() LoadOptions {
	return LoadOptions{
		AutoMigrate:        true,
		StrictVersion:      false,
		AllowUnknownFields: false,
	}
}

// LoadResult contains the loaded config and metadata about the load
type LoadResult struct {
	Config          *Config
	OriginalVersion SchemaVersion
	CurrentVersion  SchemaVersion
	WasMigrated     bool
	MigrationPath   []string // List of migrations applied
	Warnings        []string
}

// LoadFile loads a config file (HCL or JSON) with version handling
func LoadFile(path string) (*Config, error) {
	result, err := LoadFileWithOptions(path, DefaultLoadOptions())
	if err != nil {
		return nil, err
	}
	return result.Config, nil
}

// LoadWithDefaults loads a config file with default options
func LoadWithDefaults(path string) (*Config, error) {
	return LoadFile(path)
}

// LoadFileWithOptions loads a config file with explicit options
func LoadFileWithOptions(path string, opts LoadOptions) (*LoadResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".hcl":
		return LoadHCLWithOptions(data, path, opts)
	case ".json":
		return LoadJSONWithOptions(data, opts)
	default:
		// Try HCL first
		hclResult, hclErr := LoadHCLWithOptions(data, path, opts)
		if hclErr == nil {
			return hclResult, nil
		}

		// Fall back to JSON
		jsonResult, jsonErr := LoadJSONWithOptions(data, opts)
		if jsonErr == nil {
			return jsonResult, nil
		}

		// If both fail, return a combined error or just the HCL one if it looks like HCL
		if strings.Contains(string(data), "{") && strings.Contains(string(data), "\"") && !strings.Contains(string(data), "=") {
			return nil, fmt.Errorf("failed to parse config as HCL or JSON. JSON error: %w", jsonErr)
		}
		return nil, fmt.Errorf("failed to parse config as HCL: %w (JSON fallback error: %v)", hclErr, jsonErr)
	}
}

// LoadHCL loads config from HCL bytes
func LoadHCL(data []byte, filename string) (*Config, error) {
	result, err := LoadHCLWithOptions(data, filename, DefaultLoadOptions())
	if err != nil {
		return nil, err
	}
	return result.Config, nil
}

// LoadHCLWithOptions loads config from HCL bytes with options
func LoadHCLWithOptions(data []byte, filename string, opts LoadOptions) (*LoadResult, error) {
	// Apply pre-parse legacy transformations if needed
	var legacyWarnings []string
	if opts.AutoMigrate && IsLegacyConfig(data) {
		var applied []string
		data, applied = TransformLegacyHCL(data)
		for _, change := range applied {
			legacyWarnings = append(legacyWarnings, fmt.Sprintf("Applied legacy transformation: %s", change))
		}
	}

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(data, filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL: %w", diags)
	}

	// Decode into config
	var config Config
	diags = gohcl.DecodeBody(file.Body, nil, &config)
	if diags.HasErrors() {
		// Check for unknown fields
		if !opts.AllowUnknownFields {
			for _, diag := range diags {
				if diag.Severity == hcl.DiagError {
					return nil, fmt.Errorf("failed to decode HCL: %w", diags)
				}
			}
		}
	}

	// Handle version migration if needed
	result := &LoadResult{
		Config:   &config,
		Warnings: legacyWarnings,
	}

	// Parse versions
	originalVersion, _ := ParseVersion(config.SchemaVersion)
	currentVersion, _ := ParseVersion(CurrentSchemaVersion)
	result.OriginalVersion = originalVersion
	result.CurrentVersion = currentVersion

	if originalVersion.Compare(currentVersion) > 0 {
		return nil, fmt.Errorf("config version %s is newer than supported version %s",
			config.SchemaVersion, CurrentSchemaVersion)
	}

	if opts.AutoMigrate && config.SchemaVersion != CurrentSchemaVersion {
		err := MigrateConfig(&config, currentVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate config from version %s to %s: %w",
				config.SchemaVersion, CurrentSchemaVersion, err)
		}
		result.Config = &config
		result.WasMigrated = true
		result.Warnings = []string{fmt.Sprintf("Migrated from version %s to %s", config.SchemaVersion, CurrentSchemaVersion)}
	} else if opts.StrictVersion && config.SchemaVersion != CurrentSchemaVersion {
		return nil, fmt.Errorf("config version %s does not match current version %s (set AutoMigrate=true to auto-migrate)",
			config.SchemaVersion, CurrentSchemaVersion)
	} else {
		// Even if no schema migration is needed, we must always canonicalize the config
		// to ensure internal consistency (e.g. migrating deprecated fields to new internal structures).
		if err := config.Canonicalize(); err != nil {
			return nil, fmt.Errorf("canonicalization failed: %w", err)
		}
	}

	return result, nil
}

// LoadJSON loads config from JSON bytes
func LoadJSON(data []byte) (*Config, error) {
	result, err := LoadJSONWithOptions(data, DefaultLoadOptions())
	if err != nil {
		return nil, err
	}
	return result.Config, nil
}

// LoadJSONWithOptions loads config from JSON bytes with options
func LoadJSONWithOptions(data []byte, opts LoadOptions) (*LoadResult, error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Handle version migration if needed
	result := &LoadResult{
		Config: &config,
	}

	// Parse versions
	originalVersion, _ := ParseVersion(config.SchemaVersion)
	currentVersion, _ := ParseVersion(CurrentSchemaVersion)
	result.OriginalVersion = originalVersion
	result.CurrentVersion = currentVersion

	if opts.AutoMigrate && config.SchemaVersion != CurrentSchemaVersion {
		err := MigrateConfig(&config, currentVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate config from version %s to %s: %w",
				config.SchemaVersion, CurrentSchemaVersion, err)
		}
		result.Config = &config
		result.WasMigrated = true
		result.Warnings = []string{fmt.Sprintf("Migrated from version %s to %s", config.SchemaVersion, CurrentSchemaVersion)}
	} else if opts.StrictVersion && config.SchemaVersion != CurrentSchemaVersion {
		return nil, fmt.Errorf("config version %s does not match current version %s (set AutoMigrate=true to auto-migrate)",
			config.SchemaVersion, CurrentSchemaVersion)
	}

	return result, nil
}

// SaveFile saves a config file (format based on extension)
func SaveFile(cfg *Config, path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return SaveJSON(cfg, path)
	case ".hcl":
		return SaveHCL(cfg, path)
	default:
		return SaveJSON(cfg, path)
	}
}

// SaveJSON saves config as JSON
func SaveJSON(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return SecureWriteFile(path, data)
}

// SaveHCL saves config as HCL using hclwrite for formatting
func SaveHCL(cfg *Config, path string) error {
	bytes, err := GenerateHCL(cfg)
	if err != nil {
		return err
	}

	// Create parent dir
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	return SecureWriteFile(path, bytes)
}

// GenerateHCL generates HCL bytes from Config
func GenerateHCL(cfg *Config) ([]byte, error) {
	// Use hcl_serializer's SyncConfigToHCL which is more robust than gohcl.EncodeIntoBody
	cf := &ConfigFile{
		Config:  cfg,
		hclFile: hclwrite.NewEmptyFile(),
	}

	if err := cf.SyncConfigToHCL(); err != nil {
		return nil, fmt.Errorf("failed to sync config to HCL: %w", err)
	}

	return cf.hclFile.Bytes(), nil
}
