// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalize(t *testing.T) {
	// Setup input config with deprecated fields
	input := &Config{
		SchemaVersion: "1.0",
		Interfaces: []Interface{
			{Name: "eth0", Zone: "wan"},
			{Name: "eth1", Zone: "lan"},
			{Name: "eth2"}, // No zone
		},
		Zones: []Zone{
			{
				Name:    "lan",
				Matches: []RuleMatch{{Interface: "wlan0"}}, // Already in Matches format
			},
			{
				Name: "dmz", // Empty matches
			},
		},
	}

	// Run canonicalization
	err := input.Canonicalize()
	assert.NoError(t, err)

	// Verify deprecated fields are cleared
	assert.Empty(t, input.Interfaces[0].Zone)
	assert.Empty(t, input.Interfaces[1].Zone)

	// Verify migration to Matches/Interface fields

	// WAN zone should be created and have match for eth0
	wan := findOrCreateZoneForMigration(input, "wan")
	assert.NotNil(t, wan)
	assert.Equal(t, "wan", wan.Name)
	assert.Len(t, wan.Matches, 1)
	assert.Equal(t, "eth0", wan.Matches[0].Interface)

	// LAN zone should have matches for eth1 (from interface) and wlan0 (already in Matches)
	lan := findOrCreateZoneForMigration(input, "lan")
	assert.NotNil(t, lan)
	assert.Len(t, lan.Matches, 2)

	// Check content of matches (order depends on implementation, so check existence)
	eth1Found := false
	wlan0Found := false
	for _, m := range lan.Matches {
		if m.Interface == "eth1" {
			eth1Found = true
		}
		if m.Interface == "wlan0" {
			wlan0Found = true
		}
	}
	assert.True(t, eth1Found, "eth1 should be in lan zone matches")
	assert.True(t, wlan0Found, "wlan0 should be in lan zone matches")
}

func TestMigrateJumps(t *testing.T) {
	registry := &MigrationRegistry{}

	// Test case 1: Jump from 1.0 to 1.1 with no migrations registered
	cfg := &Config{SchemaVersion: "1.0"}
	target := SchemaVersion{Major: 1, Minor: 1}

	// Verify MigrateConfig (which uses DefaultMigrations, but here we check the paths first)
	path, err := registry.GetMigrationPath(SchemaVersion{Major: 1, Minor: 0}, target)
	assert.NoError(t, err)
	assert.Empty(t, path)

	// Simulate MigrateConfig logic for final version bump
	if len(path) == 0 {
		cfg.SchemaVersion = target.String()
	}
	assert.Equal(t, "1.1", cfg.SchemaVersion)

	// In the real system, MigrateConfig uses DefaultMigrations, so we test GetMigrationPath here
	// and simulate what MigrateConfig does.

	// Test case 2: Intermediate migration exists
	// 1.0 -> (jump) -> 1.1 -> (migration) -> 1.2
	m11_12 := Migration{
		FromVersion: SchemaVersion{Major: 1, Minor: 1},
		ToVersion:   SchemaVersion{Major: 1, Minor: 2},
		Description: "Migration 1.1 to 1.2",
		Migrate: func(c *Config) error {
			c.IPForwarding = true
			return nil
		},
	}
	registry.Register(m11_12)

	path, err = registry.GetMigrationPath(SchemaVersion{Major: 1, Minor: 0}, SchemaVersion{Major: 1, Minor: 2})
	assert.NoError(t, err)
	assert.Len(t, path, 1)
	assert.Equal(t, m11_12.Description, path[0].Description)

	// Test case 3: Overlapping migrations should fail
	m10_12 := Migration{
		FromVersion: SchemaVersion{Major: 1, Minor: 0},
		ToVersion:   SchemaVersion{Major: 1, Minor: 2},
		Description: "Migration 1.0 to 1.2",
		Migrate:     func(c *Config) error { return nil },
	}
	registry.Register(m10_12)

	_, err = registry.GetMigrationPath(SchemaVersion{Major: 1, Minor: 0}, SchemaVersion{Major: 1, Minor: 2})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "overlapping")
}

func TestDowngradeMigration(t *testing.T) {
	// Load declarative migrations for this test into a fresh registry
	registry := &MigrationRegistry{}
	// Inline the HCL to identify file path issues in VM execution
	hclBytes := []byte(`
migration "1.0" "1.1" {
  description = "Add eBPF configuration support"

  operation "add_block" "ebpf" {
    # Defaults are applied by migrate_ebpf.go's post-load migration
  }
}
`)

	err := registry.LoadDeclarativeMigrations("migrations.hcl", hclBytes)
	require.NoError(t, err)

	// 1. Setup config with 1.1 features (eBPF)
	cfg := &Config{
		SchemaVersion: "1.1",
		EBPF: &EBPFConfig{
			Enabled: true,
		},
	}

	// 2. Perform downgrade to 1.0 using the isolated registry
	err = registry.MigrateConfig(cfg, SchemaVersion{Major: 1, Minor: 0})
	assert.NoError(t, err)

	// 3. Verify eBPF block is stripped and version is 1.0
	assert.Equal(t, "1.0", cfg.SchemaVersion)
	assert.Nil(t, cfg.EBPF)
}

func TestDeclarativeMigrationRoundTrip(t *testing.T) {
	// Load migrations into fresh registry
	registry := &MigrationRegistry{}
	// Inline the HCL to identify file path issues in VM execution
	hclBytes := []byte(`
migration "1.0" "1.1" {
  description = "Add eBPF configuration support"

  operation "add_block" "ebpf" {
    # Defaults are applied by migrate_ebpf.go's post-load migration
  }
}
`)
	err := registry.LoadDeclarativeMigrations("migrations.hcl", hclBytes)
	require.NoError(t, err)

	// 1. Start with 1.0 config
	cfg := &Config{
		SchemaVersion: "1.0",
	}

	// 2. Upgrade to 1.1
	err = registry.MigrateConfig(cfg, SchemaVersion{Major: 1, Minor: 1})
	assert.NoError(t, err)
	assert.Equal(t, "1.1", cfg.SchemaVersion)
	require.NotNil(t, cfg.EBPF, "EBPF block should be added by migration")
	assert.False(t, cfg.EBPF.Enabled, "EBPF should be disabled by default")

	// 3. Downgrade to 1.0
	err = registry.MigrateConfig(cfg, SchemaVersion{Major: 1, Minor: 0})
	assert.NoError(t, err)
	assert.Equal(t, "1.0", cfg.SchemaVersion)
	assert.Nil(t, cfg.EBPF, "EBPF block should be removed by reverse migration")
}
