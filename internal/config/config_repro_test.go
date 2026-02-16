// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"testing"
)

func TestLoadProtectionConfig(t *testing.T) {
	hcl := `
schema_version = "1.0"
protection "wan_protection" {
  interface = "veth-wan"
  anti_spoofing = true
  bogon_filtering = true
}
`
	cfg, err := LoadHCL([]byte(hcl), "test.hcl")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.Protections) != 1 {
		t.Errorf("Expected 1 protection block, got %d", len(cfg.Protections))
	} else {
		p := cfg.Protections[0]
		if p.Name != "wan_protection" {
			t.Errorf("Expected name 'wan_protection', got '%s'", p.Name)
		}
		if p.Interface != "veth-wan" {
			t.Errorf("Expected interface 'veth-wan', got '%s'", p.Interface)
		}
		if !p.AntiSpoofing {
			t.Errorf("Expected AntiSpoofing to be true")
		}
		if !p.BogonFiltering {
			t.Errorf("Expected BogonFiltering to be true")
		}
	}
}
