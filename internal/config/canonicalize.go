// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// Canonicalize cleans up the configuration by migrating deprecated fields
// to their canonical representations.
//
// This method delegates to ApplyPostLoadMigrations() which runs all registered
// post-load migrations including zone canonicalization.
func (c *Config) Canonicalize() error {
	return ApplyPostLoadMigrations(c)
}
