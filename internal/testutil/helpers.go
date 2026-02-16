// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package testutil

import (
	"os"
	"testing"
)

// RequireVM skips the test if the FLYWALL_VM_TEST environment variable is not set.
// This ensures that tests requiring real kernel capabilities (nftables, interfaces)
// are only run in the proper environment.
func RequireVM(t *testing.T) {
	t.Helper()
	if os.Getenv("FLYWALL_VM_TEST") == "" {
		t.Skip("Skipping test: requires FLYWALL_VM_TEST environment")
	}
}
