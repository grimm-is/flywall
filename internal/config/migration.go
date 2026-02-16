// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
)

// Migration represents a config schema migration from one version to another
type Migration struct {
	FromVersion SchemaVersion
	ToVersion   SchemaVersion
	Description string
	Migrate     func(*Config) error
	MigrateHCL  func(*hclwrite.File) error // AST-based migration (preserves comments)
}

// MigrationRegistry holds all registered migrations
type MigrationRegistry struct {
	migrations []Migration
}

// DefaultMigrations is the global migration registry
var DefaultMigrations = &MigrationRegistry{}

// Register adds a migration to the registry
func (r *MigrationRegistry) Register(m Migration) {
	r.migrations = append(r.migrations, m)
}

// GetMigrationPath returns the sequence of migrations needed to go from 'from' to 'to'
func (r *MigrationRegistry) GetMigrationPath(from, to SchemaVersion) ([]Migration, error) {
	cmp := from.Compare(to)
	if cmp == 0 {
		return nil, nil // No migration needed
	}

	isUpgrade := cmp < 0

	// Filter migrations that fall within the range [from, to] or [to, from]
	var applicable []Migration
	for _, m := range r.migrations {
		mIsUpgrade := m.FromVersion.Compare(m.ToVersion) < 0
		if isUpgrade && mIsUpgrade {
			// Upgrade path: m.From >= from && m.To <= to
			if m.FromVersion.Compare(from) >= 0 && m.ToVersion.Compare(to) <= 0 {
				applicable = append(applicable, m)
			}
		} else if !isUpgrade && !mIsUpgrade {
			// Downgrade path: m.From <= from && m.To >= to
			if m.FromVersion.Compare(from) <= 0 && m.ToVersion.Compare(to) >= 0 {
				applicable = append(applicable, m)
			}
		}
	}

	// Sort migrations
	if isUpgrade {
		// Sort upgrades ascending by FromVersion
		sort.Slice(applicable, func(i, j int) bool {
			return applicable[i].FromVersion.Compare(applicable[j].FromVersion) < 0
		})
	} else {
		// Sort downgrades descending by FromVersion
		sort.Slice(applicable, func(i, j int) bool {
			return applicable[i].FromVersion.Compare(applicable[j].FromVersion) > 0
		})
	}

	// Validate the path: no overlaps
	for i := 0; i < len(applicable)-1; i++ {
		if isUpgrade {
			if applicable[i].ToVersion.Compare(applicable[i+1].FromVersion) > 0 {
				return nil, fmt.Errorf("overlapping upgrade migrations: %s->%s and %s->%s",
					applicable[i].FromVersion, applicable[i].ToVersion,
					applicable[i+1].FromVersion, applicable[i+1].ToVersion)
			}
		} else {
			if applicable[i].ToVersion.Compare(applicable[i+1].FromVersion) < 0 {
				return nil, fmt.Errorf("overlapping downgrade migrations: %s->%s and %s->%s",
					applicable[i].FromVersion, applicable[i].ToVersion,
					applicable[i+1].FromVersion, applicable[i+1].ToVersion)
			}
		}
	}

	return applicable, nil
}

// MigrateConfig applies all necessary migrations to bring config to target version
func MigrateConfig(cfg *Config, targetVersion SchemaVersion) error {
	return DefaultMigrations.MigrateConfig(cfg, targetVersion)
}

// MigrateConfig applies all necessary migrations to bring config to target version
func (r *MigrationRegistry) MigrateConfig(cfg *Config, targetVersion SchemaVersion) error {
	currentVersion, err := ParseVersion(cfg.SchemaVersion)
	if err != nil {
		return fmt.Errorf("invalid config schema version: %w", err)
	}

	if currentVersion.Compare(targetVersion) == 0 {
		// Even if no migration is needed, ensure the version string is set if it was empty
		if cfg.SchemaVersion == "" {
			cfg.SchemaVersion = currentVersion.String()
		}
		return nil // Already at target version
	}

	path, err := r.GetMigrationPath(currentVersion, targetVersion)
	if err != nil {
		return err
	}

	for _, migration := range path {
		if err := migration.Migrate(cfg); err != nil {
			return fmt.Errorf("migration %s -> %s failed: %w",
				migration.FromVersion, migration.ToVersion, err)
		}
		cfg.SchemaVersion = migration.ToVersion.String()
	}

	// Final version update (covers jumps or cases with no migrations)
	cfg.SchemaVersion = targetVersion.String()

	// Canonicalize config (clean up deprecated fields even within same version)
	if err := cfg.Canonicalize(); err != nil {
		return fmt.Errorf("canonicalization failed: %w", err)
	}

	return nil
}

// MigrateToLatest migrates config to the current schema version
func MigrateToLatest(cfg *Config) error {
	target, _ := ParseVersion(CurrentSchemaVersion)
	return MigrateConfig(cfg, target)
}

// When a new schema version requires migration logic, add it here.
func init() {
	// Declarative migrations are loaded at runtime or via LoadDeclarativeMigrations
}

// LoadDeclarativeMigrations loads migrations from an HCL file and registers them.
// It registers both forward defined migrations and inferred reverse migrations.
func (r *MigrationRegistry) LoadDeclarativeMigrations(path string, hclBytes []byte) error {
	engine, err := NewMigrationEngine(hclBytes, path)
	if err != nil {
		return err
	}

	// Helper to register a migration from the engine
	register := func(def MigrationDef) {
		r.Register(Migration{
			FromVersion: MustParseVersion(def.FromVersion),
			ToVersion:   MustParseVersion(def.ToVersion),
			Description: def.Description,
			Migrate: func(cfg *Config) error {
				return engine.Apply(cfg, def)
			},
		})
	}

	// 1. Register all forward migrations defined in the file
	for _, m := range engine.migrations {
		register(m)

		// 2. Try to infer and register the reverse migration
		if reverse, ok := engine.GetReverseMigration(m.ToVersion, m.FromVersion); ok {
			register(reverse)
		}
	}

	return nil
}

// MustParseVersion parses a version string or panics.
func MustParseVersion(v string) SchemaVersion {
	ver, err := ParseVersion(v)
	if err != nil {
		panic(fmt.Sprintf("invalid version in migration: %s", v))
	}
	return ver
}
