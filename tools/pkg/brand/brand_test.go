// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package brand

import (
	"testing"
)

func TestGet(t *testing.T) {
	b := Get()
	if b.Name == "" {
		t.Error("Brand name should not be empty")
	}
	// Version is a global variable, not in the struct
	if Version == "" {
		t.Error("Global Version should be initialized (to dev default)")
	}
	if Name == "" {
		t.Error("Global Name should be initialized")
	}
}

func TestUserAgent(t *testing.T) {
	ua := UserAgent("1.0.0")
	if ua == "" {
		t.Error("UserAgent should not be empty")
	}

	uaDefault := UserAgent("")
	if uaDefault == "" {
		t.Error("UserAgent default should not be empty")
	}
}
