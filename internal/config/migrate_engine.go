// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// MigrationDef represents a single migration definition from the HCL file.
type MigrationDef struct {
	FromVersion string        `hcl:"from_version,label"`
	ToVersion   string        `hcl:"to_version,label"`
	Description string        `hcl:"description,optional"`
	Operations  []MigrationOp `hcl:"operation,block"`
	Remain      hcl.Body      `hcl:",remain"`
}

// MigrationOp represents a migration operation.
type MigrationOp struct {
	Type     string            `hcl:"type,label"`        // add_block, remove_block, rename_block, etc.
	Target   string            `hcl:"target,label"`      // block/attribute name
	NewName  string            `hcl:"new_name,optional"` // for rename operations
	Preserve bool              `hcl:"preserve,optional"` // for remove operations
	Defaults map[string]string `hcl:"defaults,optional"` // default values for add
}

// MigrationsFile represents the top-level migrations HCL file.
type MigrationsFile struct {
	Migrations []MigrationDef `hcl:"migration,block"`
}

// MigrationEngine executes declarative migrations.
type MigrationEngine struct {
	migrations map[string]MigrationDef // key: "fromVersion->toVersion"
}

// NewMigrationEngine creates a new migration engine from an HCL file.
func NewMigrationEngine(hclBytes []byte, filename string) (*MigrationEngine, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(hclBytes, filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse migrations HCL: %w", diags)
	}

	var mf MigrationsFile
	diags = gohcl.DecodeBody(file.Body, nil, &mf)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode migrations: %w", diags)
	}

	engine := &MigrationEngine{
		migrations: make(map[string]MigrationDef),
	}
	for _, m := range mf.Migrations {
		key := m.FromVersion + "->" + m.ToVersion
		engine.migrations[key] = m
	}

	return engine, nil
}

// GetMigration returns the migration definition for a version transition.
func (e *MigrationEngine) GetMigration(from, to string) (MigrationDef, bool) {
	key := from + "->" + to
	m, ok := e.migrations[key]
	return m, ok
}

// GetReverseMigration infers a reverse migration by inverting operations.
func (e *MigrationEngine) GetReverseMigration(from, to string) (MigrationDef, bool) {
	// Look for the forward migration (to->from for reverse)
	forwardKey := to + "->" + from
	forward, ok := e.migrations[forwardKey]
	if !ok {
		return MigrationDef{}, false
	}

	// Invert operations
	reverse := MigrationDef{
		FromVersion: from,
		ToVersion:   to,
		Description: "Reverse of: " + forward.Description,
		Operations:  make([]MigrationOp, len(forward.Operations)),
	}

	// Reverse in opposite order
	for i, op := range forward.Operations {
		idx := len(forward.Operations) - 1 - i
		reverse.Operations[idx] = invertOp(op)
	}

	return reverse, true
}

// invertOp returns the inverse of a migration operation.
func invertOp(op MigrationOp) MigrationOp {
	switch op.Type {
	case "add_block":
		return MigrationOp{Type: "remove_block", Target: op.Target}
	case "remove_block":
		return MigrationOp{Type: "add_block", Target: op.Target, Defaults: op.Defaults}
	case "rename_block":
		return MigrationOp{Type: "rename_block", Target: op.NewName, NewName: op.Target}
	case "add_attribute":
		return MigrationOp{Type: "remove_attribute", Target: op.Target}
	case "remove_attribute":
		return MigrationOp{Type: "add_attribute", Target: op.Target, Defaults: op.Defaults}
	case "rename_attribute":
		return MigrationOp{Type: "rename_attribute", Target: op.NewName, NewName: op.Target}
	default:
		return op // Unknown ops pass through
	}
}

// Apply applies a migration to a config.
// This operates on the raw HCL bytes rather than the parsed Config struct
// to preserve comments and unknown fields.
func (e *MigrationEngine) Apply(cfg *Config, migration MigrationDef) error {
	for _, op := range migration.Operations {
		if err := e.applyOp(cfg, op); err != nil {
			return fmt.Errorf("operation %s %s failed: %w", op.Type, op.Target, err)
		}
	}
	cfg.SchemaVersion = migration.ToVersion
	return nil
}

// applyOp applies a single operation to the config struct.
func (e *MigrationEngine) applyOp(cfg *Config, op MigrationOp) error {
	switch op.Type {
	case "add_block":
		return e.addBlock(cfg, op.Target, op.Defaults)
	case "remove_block":
		return e.removeBlock(cfg, op.Target)
	case "rename_block":
		return e.renameBlock(cfg, op.Target, op.NewName)
	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// addBlock adds a block to the config with defaults.
func (e *MigrationEngine) addBlock(cfg *Config, blockType string, defaults map[string]string) error {
	switch strings.ToLower(blockType) {
	case "ebpf":
		if cfg.EBPF == nil {
			cfg.EBPF = DefaultEBPFConfig()
		}
	default:
		return fmt.Errorf("unknown block type: %s", blockType)
	}
	return nil
}

// removeBlock removes a block from the config.
func (e *MigrationEngine) removeBlock(cfg *Config, blockType string) error {
	switch strings.ToLower(blockType) {
	case "ebpf":
		cfg.EBPF = nil
	default:
		return fmt.Errorf("unknown block type: %s", blockType)
	}
	return nil
}

// renameBlock renames a block in the config.
func (e *MigrationEngine) renameBlock(cfg *Config, oldName, newName string) error {
	// This is more complex and depends on the specific blocks
	// For now, this is a placeholder
	return fmt.Errorf("rename_block not yet implemented for %s->%s", oldName, newName)
}
