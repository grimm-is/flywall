// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

// Package config provides HCL configuration handling with comment preservation.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	"grimm.is/flywall/internal/errors"
)

// ConfigDiff represents a structured configuration difference
type ConfigDiff struct {
	Added    []Change
	Modified []Change
	Removed  []Change
	Moved    []Change // For reordered items
	Summary  DiffSummary
}

// Change represents a single configuration change
type Change struct {
	Path     string      // e.g., "interfaces[0].ipv4[1]"
	Old      interface{} // Previous value
	New      interface{} // New value
	Type     ChangeType
	Section  string // Top-level section (interfaces, policies, etc.)
	Severity string // "critical", "warning", "info"
}

// ChangeType represents the type of change
type ChangeType string

const (
	Added    ChangeType = "added"
	Modified ChangeType = "modified"
	Removed  ChangeType = "removed"
	Moved    ChangeType = "moved"
)

// DiffSummary provides a high-level summary of changes
type DiffSummary struct {
	TotalChanges     int
	CriticalChanges  int
	WarningChanges   int
	AffectedSections []string
	HasConnectivity  bool // Changes that might affect connectivity
	HasSecurity      bool // Changes that affect security rules
}

// ConfigFile represents an HCL configuration file with preserved source.
// This allows round-trip editing while preserving comments and formatting.
type ConfigFile struct {
	Path     string
	Config   *Config
	hclFile  *hclwrite.File
	original []byte
}

// LoadConfigFile loads an HCL config file, preserving the original source
// for round-trip editing with comments.
func LoadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, errors.KindInternal, "failed to read config file")
	}

	return LoadConfigFromBytes(path, data)
}

// LoadConfigFromBytes loads config from bytes, preserving source for round-trip.
func LoadConfigFromBytes(filename string, data []byte) (*ConfigFile, error) {
	// Parse for writing (preserves comments and formatting)
	hclFile, diags := hclwrite.ParseConfig(data, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, errors.Errorf(errors.KindValidation, "failed to parse HCL for writing: %s", diags.Error())
	}

	// Parse for reading (into Go struct)
	var cfg Config

	if err := hclsimple.Decode(filename, data, nil, &cfg); err != nil {
		return nil, errors.Wrap(err, errors.KindValidation, "failed to decode config")
	}

	return &ConfigFile{
		Path:     filename,
		Config:   &cfg,
		hclFile:  hclFile,
		original: data,
	}, nil
}

// Save writes the config back to disk, preserving comments where possible.
// If the config was modified via the structured API, it merges changes
// while trying to preserve original formatting and comments.
func (cf *ConfigFile) Save() error {
	return cf.SaveTo(cf.Path)
}

// SaveTo writes the config to a specific path.
func (cf *ConfigFile) SaveTo(path string) error {
	// Create backup of original file
	if _, err := os.Stat(path); err == nil {
		backupPath := path + ".bak"
		if err := copyFile(path, backupPath); err != nil {
			return errors.Wrap(err, errors.KindInternal, "failed to create backup")
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, errors.KindInternal, "failed to create directory")
	}

	// Write the HCL file
	data := cf.hclFile.Bytes()
	if err := SecureWriteFile(path, data); err != nil {
		return errors.Wrap(err, errors.KindInternal, "failed to write config")
	}

	cf.Path = path
	cf.original = data
	return nil
}

// GetRawHCL returns the current HCL source as a string.
func (cf *ConfigFile) GetRawHCL() string {
	return string(cf.hclFile.Bytes())
}

// SetRawHCL replaces the entire config with new HCL source.
// Returns an error if the HCL is invalid.
func (cf *ConfigFile) SetRawHCL(hclSource string) error {
	data := []byte(hclSource)

	// Validate by parsing
	newFile, diags := hclwrite.ParseConfig(data, cf.Path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return errors.Errorf(errors.KindValidation, "invalid HCL: %s", diags.Error())
	}

	// Also validate it decodes to our config struct
	var cfg Config
	if err := hclsimple.Decode(cf.Path, data, nil, &cfg); err != nil {
		return errors.Wrap(err, errors.KindValidation, "HCL does not match config schema")
	}

	cf.hclFile = newFile
	cf.Config = &cfg
	return nil
}

// GetSection returns the raw HCL for a specific section (e.g., "dhcp", "dns_server").
func (cf *ConfigFile) GetSection(sectionType string) (string, error) {
	body := cf.hclFile.Body()

	for _, block := range body.Blocks() {
		if block.Type() == sectionType {
			return formatBlock(block), nil
		}
	}

	return "", errors.Errorf(errors.KindNotFound, "section %q not found", sectionType)
}

// GetSectionByLabel returns raw HCL for a labeled block (e.g., interface "eth0").
func (cf *ConfigFile) GetSectionByLabel(sectionType string, labels []string) (string, error) {
	body := cf.hclFile.Body()

	for _, block := range body.Blocks() {
		if block.Type() == sectionType {
			blockLabels := block.Labels()

			// Match by all labels (exact count match)
			matchAll := true
			if len(labels) > 0 && len(blockLabels) == len(labels) {
				for i, l := range labels {
					if blockLabels[i] != l {
						matchAll = false
						break
					}
				}
				if matchAll {
					return formatBlock(block), nil
				}
			}

			// Fallback: match by name attribute if single label provided
			if len(labels) == 1 {
				attr := block.Body().GetAttribute("name")
				if attr != nil {
					val := strings.Trim(string(attr.Expr().BuildTokens(nil).Bytes()), "\" ")
					if val == labels[0] {
						return formatBlock(block), nil
					}
				}
			}
		}
	}

	return "", errors.Errorf(errors.KindNotFound, "section %s with labels %v not found", sectionType, labels)
}

// SetSection replaces a section with new HCL content.
// The sectionHCL should be a complete block definition.
func (cf *ConfigFile) SetSection(sectionType string, sectionHCL string) error {
	// Parse the new section
	newBlock, err := parseBlock(sectionHCL, cf.Path)
	if err != nil {
		return errors.Wrap(err, errors.KindValidation, "invalid section HCL")
	}

	if newBlock.Type() != sectionType {
		return errors.Errorf(errors.KindValidation, "section type mismatch: expected %q, got %q", sectionType, newBlock.Type())
	}

	body := cf.hclFile.Body()

	// Find existing block to replace in-place
	found := false
	for _, block := range body.Blocks() {
		if block.Type() == sectionType {
			// Replace body content to preserve block header comments/position
			dstBody := block.Body()
			clearBody(dstBody)
			copyBody(dstBody, newBlock.Body())
			found = true
			break
		}
	}

	if !found {
		// Append new block
		body.AppendNewline()
		appendBlock(body, newBlock)
	}

	// Re-decode to update Config struct
	return cf.reloadConfig()
}

// SetSectionByLabel replaces a labeled section with new HCL content.
func (cf *ConfigFile) SetSectionByLabel(sectionType string, labels []string, sectionHCL string) error {
	newBlock, err := parseBlock(sectionHCL, cf.Path)
	if err != nil {
		return errors.Wrap(err, errors.KindValidation, "invalid section HCL")
	}

	if newBlock.Type() != sectionType {
		return errors.Errorf(errors.KindValidation, "section type mismatch: expected %q, got %q", sectionType, newBlock.Type())
	}

	body := cf.hclFile.Body()

	// Find and replace existing block in-place
	found := false
	for _, block := range body.Blocks() {
		if block.Type() == sectionType {
			blockLabels := block.Labels()

			// Match by all labels (exact count match)
			matchAll := true
			if len(labels) > 0 && len(blockLabels) == len(labels) {
				for i, l := range labels {
					if blockLabels[i] != l {
						matchAll = false
						break
					}
				}
				if matchAll {
					dstBody := block.Body()
					clearBody(dstBody)
					copyBody(dstBody, newBlock.Body())
					found = true
					break
				}
			}

			// Fallback: match by name attribute if single label provided
			if !found && len(labels) == 1 {
				attr := block.Body().GetAttribute("name")
				if attr != nil {
					val := strings.Trim(string(attr.Expr().BuildTokens(nil).Bytes()), "\" ")
					if val == labels[0] {
						dstBody := block.Body()
						clearBody(dstBody)
						copyBody(dstBody, newBlock.Body())
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		// Append new block
		body.AppendNewline()
		appendBlock(body, newBlock)
	}

	return cf.reloadConfig()
}

// AddSection adds a new section to the config.
func (cf *ConfigFile) AddSection(sectionHCL string) error {
	newBlock, err := parseBlock(sectionHCL, cf.Path)
	if err != nil {
		return errors.Wrap(err, errors.KindValidation, "invalid section HCL")
	}

	body := cf.hclFile.Body()
	body.AppendNewline()
	appendBlock(body, newBlock)

	return cf.reloadConfig()
}

// RemoveSection removes a section by type.
func (cf *ConfigFile) RemoveSection(sectionType string) error {
	body := cf.hclFile.Body()

	for _, block := range body.Blocks() {
		if block.Type() == sectionType {
			body.RemoveBlock(block)
			return cf.reloadConfig()
		}
	}

	return errors.Errorf(errors.KindNotFound, "section %q not found", sectionType)
}

// RemoveSectionByLabel removes a labeled section.
// It matches if all provided labels match, OR if only one label is provided
// and it matches the 'name' attribute inside the block.
func (cf *ConfigFile) RemoveSectionByLabel(sectionType string, labels []string) error {
	body := cf.hclFile.Body()

	for _, block := range body.Blocks() {
		if block.Type() == sectionType {
			blockLabels := block.Labels()

			// Match by all labels (exact count match)
			matchAll := true
			if len(labels) > 0 && len(blockLabels) == len(labels) {
				for i, l := range labels {
					if blockLabels[i] != l {
						matchAll = false
						break
					}
				}
				if matchAll {
					body.RemoveBlock(block)
					return cf.reloadConfig()
				}
			}

			// Fallback: match by name attribute if single label provided
			if len(labels) == 1 {
				attr := block.Body().GetAttribute("name")
				if attr != nil {
					// Extract string value from tokens
					val := strings.Trim(string(attr.Expr().BuildTokens(nil).Bytes()), "\" ")
					if val == labels[0] {
						body.RemoveBlock(block)
						return cf.reloadConfig()
					}
				}
			}
		}
	}

	return errors.Errorf(errors.KindNotFound, "section %s with labels %v not found", sectionType, labels)
}

// ListSections returns all top-level section types and their labels.
func (cf *ConfigFile) ListSections() []SectionInfo {
	var sections []SectionInfo
	body := cf.hclFile.Body()

	for _, block := range body.Blocks() {
		info := SectionInfo{
			Type: block.Type(),
		}
		if labels := block.Labels(); len(labels) > 0 {
			info.Labels = labels
			info.Label = strings.Join(labels, " ")
		}
		sections = append(sections, info)
	}

	return sections
}

// SectionInfo describes a config section.
type SectionInfo struct {
	Type   string   `json:"type"`
	Labels []string `json:"labels,omitempty"`
	Label  string   `json:"label,omitempty"` // For backward compatibility, joined by space
}

// ValidateHCL validates HCL source without modifying the config.
func ValidateHCL(hclSource string) error {
	data := []byte(hclSource)

	// Check syntax
	_, diags := hclwrite.ParseConfig(data, "validate.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return errors.Errorf(errors.KindValidation, "syntax error: %s", diags.Error())
	}

	// Check schema
	var cfg Config
	if err := hclsimple.Decode("validate.hcl", data, nil, &cfg); err != nil {
		return errors.Wrap(err, errors.KindValidation, "schema error")
	}

	return nil
}

// ValidateSection validates a single section's HCL.
func ValidateSection(sectionType, sectionHCL string) error {
	_, err := parseBlock(sectionHCL, "validate.hcl")
	return err
}

// FormatHCL formats HCL source code.
func FormatHCL(hclSource string) (string, error) {
	data := []byte(hclSource)

	file, diags := hclwrite.ParseConfig(data, "format.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return "", errors.Errorf(errors.KindValidation, "invalid HCL: %s", diags.Error())
	}

	return string(file.Bytes()), nil
}

// reloadConfig re-decodes the HCL into the Config struct.
func (cf *ConfigFile) reloadConfig() error {
	data := cf.hclFile.Bytes()
	var cfg Config
	if err := hclsimple.Decode(cf.Path, data, nil, &cfg); err != nil {
		return errors.Wrap(err, errors.KindInternal, "failed to reload config")
	}
	cf.Config = &cfg
	return nil
}

// MigrateToLatest migrates the config file to the latest schema version,
// preserving comments and formatting.
func (cf *ConfigFile) MigrateToLatest() error {
	target, _ := ParseVersion(CurrentSchemaVersion)
	return cf.MigrateTo(target)
}

// MigrateTo migrates the config file to a specific schema version,
// preserving comments and formatting.
func (cf *ConfigFile) MigrateTo(targetVersion SchemaVersion) error {
	currentVersion, err := ParseVersion(cf.Config.SchemaVersion)
	if err != nil {
		return errors.Wrap(err, errors.KindValidation, "invalid config schema version")
	}

	if currentVersion.Compare(targetVersion) >= 0 {
		return nil // Already at or above target version
	}

	path, err := DefaultMigrations.GetMigrationPath(currentVersion, targetVersion)
	if err != nil {
		return err
	}

	for _, migration := range path {
		// Run AST migration if defined
		if migration.MigrateHCL != nil {
			if err := migration.MigrateHCL(cf.hclFile); err != nil {
				return fmt.Errorf("HCL migration %s -> %s failed: %w",
					migration.FromVersion, migration.ToVersion, err)
			}
		}

		// Always update schema_version attribute
		// We do this via AST to preserve formatting
		cf.hclFile.Body().SetAttributeValue("schema_version", cty.StringVal(migration.ToVersion.String()))

		// Update internal struct state (re-decode)
		// This is less efficient but safer to ensure struct matches AST
		if err := cf.reloadConfig(); err != nil {
			return fmt.Errorf("failed to reload config after migration %s -> %s: %w",
				migration.FromVersion, migration.ToVersion, err)
		}
	}

	return nil
}

// Helper functions

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return SecureWriteFile(dst, data)
}

func formatBlock(block *hclwrite.Block) string {
	f := hclwrite.NewEmptyFile()
	appendBlock(f.Body(), block)
	return string(f.Bytes())
}

func parseBlock(hclSource, filename string) (*hclwrite.Block, error) {
	data := []byte(hclSource)

	file, diags := hclwrite.ParseConfig(data, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, errors.Errorf(errors.KindValidation, "parse error: %s", diags.Error())
	}

	blocks := file.Body().Blocks()
	if len(blocks) == 0 {
		return nil, errors.New(errors.KindValidation, "no block found in HCL")
	}
	if len(blocks) > 1 {
		return nil, errors.Errorf(errors.KindValidation, "expected single block, got %d", len(blocks))
	}

	return blocks[0], nil
}

func appendBlock(body *hclwrite.Body, src *hclwrite.Block) {
	body.AppendBlock(src)
}

// clearBody removes all attributes and blocks from a body.
func clearBody(body *hclwrite.Body) {
	for name := range body.Attributes() {
		body.RemoveAttribute(name)
	}
	for _, block := range body.Blocks() {
		body.RemoveBlock(block)
	}
}

// copyBody copies attributes and blocks from src to dst, attempt to preserve comments.
func copyBody(dst, src *hclwrite.Body) {
	// hclwrite's Body doesn't have an easy way to copy with comments.
	// But we can get the tokens of the source body and append them.
	// Note: BuildTokens includes the opening and closing braces if it's a block body?
	// Actually, src is a Body. src.BuildTokens(nil) returns tokens of the content.
	tokens := src.BuildTokens(nil)
	dst.AppendUnstructuredTokens(tokens)
}

// NewConfigFile creates a new empty config file.
func NewConfigFile(path string) *ConfigFile {
	return &ConfigFile{
		Path:    path,
		Config:  &Config{},
		hclFile: hclwrite.NewEmptyFile(),
	}
}

// SetAttribute sets a top-level attribute (e.g., ip_forwarding = true).
func (cf *ConfigFile) SetAttribute(name string, value interface{}) error {
	body := cf.hclFile.Body()

	ctyVal, err := toCtyValue(value)
	if err != nil {
		return errors.Wrapf(err, errors.KindValidation, "invalid value for %s", name)
	}

	body.SetAttributeValue(name, ctyVal)
	return cf.reloadConfig()
}

// toCtyValue converts a Go value to a cty.Value for HCL writing.
func toCtyValue(v interface{}) (cty.Value, error) {
	switch val := v.(type) {
	case bool:
		return cty.BoolVal(val), nil
	case int:
		return cty.NumberIntVal(int64(val)), nil
	case int64:
		return cty.NumberIntVal(val), nil
	case float64:
		return cty.NumberFloatVal(val), nil
	case string:
		return cty.StringVal(val), nil
	case []string:
		if len(val) == 0 {
			return cty.ListValEmpty(cty.String), nil
		}
		vals := make([]cty.Value, len(val))
		for i, s := range val {
			vals[i] = cty.StringVal(s)
		}
		return cty.ListVal(vals), nil
	default:
		return cty.NilVal, errors.Errorf(errors.KindValidation, "unsupported type: %T", v)
	}
}

// GetConfigWithMetadata returns the config along with file metadata.
type ConfigMetadata struct {
	Path         string        `json:"path"`
	LastModified time.Time     `json:"last_modified"`
	Size         int64         `json:"size"`
	Sections     []SectionInfo `json:"sections"`
}

func (cf *ConfigFile) GetMetadata() ConfigMetadata {
	meta := ConfigMetadata{
		Path:     cf.Path,
		Sections: cf.ListSections(),
	}

	if info, err := os.Stat(cf.Path); err == nil {
		meta.LastModified = info.ModTime()
		meta.Size = info.Size()
	}

	return meta
}

// Diff returns a diff between original and current HCL.
// If structured is true, returns a semantic diff; otherwise returns simple line-by-line diff.
func (cf *ConfigFile) Diff(structured ...bool) string {
	current := cf.hclFile.Bytes()
	if bytes.Equal(cf.original, current) {
		return ""
	}

	// If structured diff requested and we have parsed configs
	if len(structured) > 0 && structured[0] && cf.Config != nil {
		// Load original config
		originalCfg, err := LoadConfigFromBytes(cf.Path, cf.original)
		if err == nil {
			// Perform structured diff
			diff := DiffConfigs(originalCfg.Config, cf.Config)
			if diff.HasChanges() {
				return diff.String()
			}
		}
		// Fall back to simple diff if structured fails
	}

	// Simple line-by-line diff (original behavior)
	origLines := strings.Split(string(cf.original), "\n")
	currLines := strings.Split(string(current), "\n")

	var diff strings.Builder
	diff.WriteString("--- original\n")
	diff.WriteString("+++ modified\n")

	// Very simple diff - just show changed lines
	maxLines := len(origLines)
	if len(currLines) > maxLines {
		maxLines = len(currLines)
	}

	for i := 0; i < maxLines; i++ {
		origLine := ""
		currLine := ""
		if i < len(origLines) {
			origLine = origLines[i]
		}
		if i < len(currLines) {
			currLine = currLines[i]
		}

		if origLine != currLine {
			if origLine != "" {
				diff.WriteString(fmt.Sprintf("-%s\n", origLine))
			}
			if currLine != "" {
				diff.WriteString(fmt.Sprintf("+%s\n", currLine))
			}
		}
	}

	return diff.String()
}

// DiffStructured returns a structured semantic diff between original and current configs
func (cf *ConfigFile) DiffStructured() (*ConfigDiff, error) {
	if cf.Config == nil {
		return nil, fmt.Errorf("no parsed config available")
	}

	originalCfg, err := LoadConfigFromBytes(cf.Path, cf.original)
	if err != nil {
		return nil, fmt.Errorf("failed to parse original config: %w", err)
	}

	diff := DiffConfigs(originalCfg.Config, cf.Config)
	return diff, nil
}

// HasChanges returns true if the config has been modified since loading.
func (cf *ConfigFile) HasChanges() bool {
	return !bytes.Equal(cf.original, cf.hclFile.Bytes())
}

// Reload discards changes and reloads from disk.
func (cf *ConfigFile) Reload() error {
	newCf, err := LoadConfigFile(cf.Path)
	if err != nil {
		return err
	}
	*cf = *newCf
	return nil
}

// ParseHCLDiagnostics parses HCL and returns detailed diagnostics.
type HCLDiagnostic struct {
	Severity string `json:"severity"` // "error" or "warning"
	Summary  string `json:"summary"`
	Detail   string `json:"detail,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

func ParseHCLWithDiagnostics(hclSource string) ([]HCLDiagnostic, error) {
	data := []byte(hclSource)
	parser := hclparse.NewParser()

	_, diags := parser.ParseHCL(data, "input.hcl")

	var result []HCLDiagnostic
	for _, d := range diags {
		diag := HCLDiagnostic{
			Summary: d.Summary,
			Detail:  d.Detail,
		}
		if d.Severity == hcl.DiagError {
			diag.Severity = "error"
		} else {
			diag.Severity = "warning"
		}
		if d.Subject != nil {
			diag.Line = d.Subject.Start.Line
			diag.Column = d.Subject.Start.Column
		}
		result = append(result, diag)
	}

	if diags.HasErrors() {
		return result, errors.New(errors.KindValidation, "HCL has errors")
	}
	return result, nil
}

// DiffConfigs performs a structured diff between two configurations
func DiffConfigs(oldConfig, newConfig *Config) *ConfigDiff {
	diff := &ConfigDiff{
		Added:    make([]Change, 0),
		Modified: make([]Change, 0),
		Removed:  make([]Change, 0),
		Moved:    make([]Change, 0),
	}

	// Convert to maps for comparison
	oldMap := configToMap(oldConfig)
	newMap := configToMap(newConfig)

	// Compare each section
	compareSections(oldMap, newMap, "", diff)

	// Calculate summary
	diff.calculateSummary()

	return diff
}

// configToMap converts a Config to a map for comparison
func configToMap(cfg *Config) map[string]interface{} {
	data, _ := json.Marshal(cfg)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}

// compareSections recursively compares configuration sections
func compareSections(old, new map[string]interface{}, basePath string, diff *ConfigDiff) {
	// Track all keys
	allKeys := make(map[string]bool)
	for k := range old {
		allKeys[k] = true
	}
	for k := range new {
		allKeys[k] = true
	}

	for key := range allKeys {
		currentPath := joinPath(basePath, key)
		oldValue, oldExists := old[key]
		newValue, newExists := new[key]

		if !oldExists && newExists {
			// Added
			change := Change{
				Path:    currentPath,
				New:     newValue,
				Type:    Added,
				Section: getSection(currentPath),
			}
			change.Severity = assessChangeSeverity(change)
			diff.Added = append(diff.Added, change)
		} else if oldExists && !newExists {
			// Removed
			change := Change{
				Path:    currentPath,
				Old:     oldValue,
				Type:    Removed,
				Section: getSection(currentPath),
			}
			change.Severity = assessChangeSeverity(change)
			diff.Removed = append(diff.Removed, change)
		} else if oldExists && newExists {
			// Compare values
			compareValues(oldValue, newValue, currentPath, diff)
		}
	}
}

// compareValues compares two configuration values
func compareValues(old, new interface{}, path string, diff *ConfigDiff) {
	oldType := reflect.TypeOf(old)
	newType := reflect.TypeOf(new)

	if oldType != newType {
		// Type changed
		change := Change{
			Path:    path,
			Old:     old,
			New:     new,
			Type:    Modified,
			Section: getSection(path),
		}
		change.Severity = assessChangeSeverity(change)
		diff.Modified = append(diff.Modified, change)
		return
	}

	switch oldTyped := old.(type) {
	case map[string]interface{}:
		if newTyped, ok := new.(map[string]interface{}); ok {
			compareSections(oldTyped, newTyped, path, diff)
		}
	case []interface{}:
		if newTyped, ok := new.([]interface{}); ok {
			compareArrays(oldTyped, newTyped, path, diff)
		}
	default:
		if !reflect.DeepEqual(old, new) {
			change := Change{
				Path:    path,
				Old:     old,
				New:     new,
				Type:    Modified,
				Section: getSection(path),
			}
			change.Severity = assessChangeSeverity(change)
			diff.Modified = append(diff.Modified, change)
		}
	}
}

// compareArrays compares arrays with special handling for reordering
func compareArrays(old, new []interface{}, basePath string, diff *ConfigDiff) {
	// Try to match items by key if they have one
	oldByKey := indexArrayByKey(old)
	newByKey := indexArrayByKey(new)

	// Find added, removed, and modified items
	for key, oldValue := range oldByKey {
		if newValue, exists := newByKey[key]; exists {
			// Compare the items
			itemPath := fmt.Sprintf("%s[%s]", basePath, key)
			compareValues(oldValue, newValue, itemPath, diff)
		} else {
			// Removed
			change := Change{
				Path:    fmt.Sprintf("%s[%s]", basePath, key),
				Old:     oldValue,
				Type:    Removed,
				Section: getSection(basePath),
			}
			change.Severity = assessChangeSeverity(change)
			diff.Removed = append(diff.Removed, change)
		}
	}

	for key, newValue := range newByKey {
		if _, exists := oldByKey[key]; !exists {
			// Added
			change := Change{
				Path:    fmt.Sprintf("%s[%s]", basePath, key),
				New:     newValue,
				Type:    Added,
				Section: getSection(basePath),
			}
			change.Severity = assessChangeSeverity(change)
			diff.Added = append(diff.Added, change)
		}
	}
}

// indexArrayByKey creates a map from array items to their keys
func indexArrayByKey(arr []interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for i, item := range arr {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Try to find a unique key
			if name, ok := itemMap["name"].(string); ok {
				result[name] = item
				continue
			}
		}
		// Fallback to index
		result[fmt.Sprintf("%d", i)] = item
	}

	return result
}

// assessChangeSeverity determines the severity of a change
func assessChangeSeverity(change Change) string {
	path := strings.ToLower(change.Path)

	// Critical changes
	if strings.Contains(path, "interfaces") &&
		(strings.Contains(path, "zone") || strings.Contains(path, "gateway")) {
		return "critical"
	}
	if strings.Contains(path, "policies") && change.Type == Removed {
		return "critical"
	}
	if strings.Contains(path, "schema_version") {
		return "critical"
	}

	// Warning changes
	if strings.Contains(path, "policies") && change.Type == Modified {
		return "warning"
	}
	if strings.Contains(path, "ipset") {
		return "warning"
	}
	if strings.Contains(path, "nat") {
		return "warning"
	}

	// Default to info
	return "info"
}

// getSection extracts the top-level section from a path
func getSection(path string) string {
	parts := strings.SplitN(path, ".", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// joinPath joins path parts
func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// calculateSummary calculates the diff summary
func (cd *ConfigDiff) calculateSummary() {
	sections := make(map[string]bool)

	for _, change := range append(append(cd.Added, cd.Modified...), cd.Removed...) {
		cd.Summary.TotalChanges++

		sections[change.Section] = true

		switch change.Severity {
		case "critical":
			cd.Summary.CriticalChanges++
		case "warning":
			cd.Summary.WarningChanges++
		}

		// Check for connectivity impact
		if strings.Contains(change.Path, "interfaces") ||
			strings.Contains(change.Path, "zones") ||
			strings.Contains(change.Path, "routes") {
			cd.Summary.HasConnectivity = true
		}

		// Check for security impact
		if strings.Contains(change.Path, "policies") ||
			strings.Contains(change.Path, "ipset") ||
			strings.Contains(change.Path, "security") {
			cd.Summary.HasSecurity = true
		}
	}

	// Convert sections map to sorted slice
	for section := range sections {
		cd.Summary.AffectedSections = append(cd.Summary.AffectedSections, section)
	}
	sort.Strings(cd.Summary.AffectedSections)
}

// HasChanges returns true if there are any changes
func (cd *ConfigDiff) HasChanges() bool {
	return len(cd.Added) > 0 || len(cd.Modified) > 0 ||
		len(cd.Removed) > 0 || len(cd.Moved) > 0
}

// GetChangesBySection returns changes grouped by section
func (cd *ConfigDiff) GetChangesBySection() map[string][]Change {
	sections := make(map[string][]Change)

	for _, change := range cd.Added {
		sections[change.Section] = append(sections[change.Section], change)
	}
	for _, change := range cd.Modified {
		sections[change.Section] = append(sections[change.Section], change)
	}
	for _, change := range cd.Removed {
		sections[change.Section] = append(sections[change.Section], change)
	}
	for _, change := range cd.Moved {
		sections[change.Section] = append(sections[change.Section], change)
	}

	return sections
}

// String returns a human-readable summary of the diff
func (cd *ConfigDiff) String() string {
	var parts []string

	if len(cd.Added) > 0 {
		parts = append(parts, fmt.Sprintf("Added: %d", len(cd.Added)))
	}
	if len(cd.Modified) > 0 {
		parts = append(parts, fmt.Sprintf("Modified: %d", len(cd.Modified)))
	}
	if len(cd.Removed) > 0 {
		parts = append(parts, fmt.Sprintf("Removed: %d", len(cd.Removed)))
	}
	if len(cd.Moved) > 0 {
		parts = append(parts, fmt.Sprintf("Moved: %d", len(cd.Moved)))
	}

	if cd.Summary.CriticalChanges > 0 {
		parts = append(parts, fmt.Sprintf("Critical: %d", cd.Summary.CriticalChanges))
	}

	result := strings.Join(parts, ", ")

	if cd.Summary.HasConnectivity {
		result += " [Connectivity Impact]"
	}
	if cd.Summary.HasSecurity {
		result += " [Security Impact]"
	}

	return result
}

// ToJSON converts the diff to JSON for API responses
func (cd *ConfigDiff) ToJSON() (string, error) {
	data, err := json.MarshalIndent(cd, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
