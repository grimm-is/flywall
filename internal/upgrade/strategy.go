// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package upgrade

import "context"

// UpgradeStrategy defines how the system handles binary updates.
// Different environments (Linux, BSD, Read-Only Appliance) require different strategies.
type UpgradeStrategy interface {
	// Stage prepares the new binary for execution.
	// sourcePath: The path to the new binary provided by the user.
	// Returns the path to the staged binary (e.g., /usr/sbin/flywall_new) and an error if any.
	Stage(ctx context.Context, sourcePath string) (stagedPath string, err error)

	// Finalize commits the upgrade, typically by moving the staged binary to the final location.
	// This is called by the NEW process when it starts in standby mode.
	Finalize(ctx context.Context) error
}
