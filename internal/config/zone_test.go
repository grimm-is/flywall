// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"testing"
)

func TestZone_SimpleInterface(t *testing.T) {
	hcl := `
schema_version = "1.0"

zone "wan" {
  interface = "eth0"
  dhcp = true
}
`
	cfg, err := LoadHCL([]byte(hcl), "test.hcl")
	if err != nil {
		t.Fatalf("LoadHCL() error = %v", err)
	}

	if len(cfg.Zones) != 1 {
		t.Fatalf("len(Zones) = %d, want 1", len(cfg.Zones))
	}

	zone := cfg.Zones[0]
	// Canonicalization moves 'interface' to Matches
	if len(zone.Matches) != 1 {
		t.Fatalf("len(Matches) = %d, want 1", len(zone.Matches))
	}
	if zone.Matches[0].Interface != "eth0" {
		t.Errorf("Matches[0].Interface = %q, want %q", zone.Matches[0].Interface, "eth0")
	}
	if !zone.DHCP {
		t.Error("DHCP = false, want true")
	}
}

func TestZone_InterfaceWildcard(t *testing.T) {
	hcl := `
schema_version = "1.0"

zone "vpn" {
  interface = "wg+"
  management {
    web = true
  }
}
`
	cfg, err := LoadHCL([]byte(hcl), "test.hcl")
	if err != nil {
		t.Fatalf("LoadHCL() error = %v", err)
	}

	if len(cfg.Zones) != 1 {
		t.Fatalf("len(Zones) = %d, want 1", len(cfg.Zones))
	}

	zone := cfg.Zones[0]
	// Canonicalization moves 'interface' to Matches
	if len(zone.Matches) != 1 {
		t.Fatalf("len(Matches) = %d, want 1", len(zone.Matches))
	}
	if zone.Matches[0].Interface != "wg+" {
		t.Errorf("Matches[0].Interface = %q, want %q", zone.Matches[0].Interface, "wg+")
	}
}

func TestZone_SourceMatch(t *testing.T) {
	hcl := `
schema_version = "1.0"

zone "guest" {
  interface = "eth1"
  src = "192.168.10.0/24"
  vlan = 100
  services {
    dns = true
  }
}
`
	cfg, err := LoadHCL([]byte(hcl), "test.hcl")
	if err != nil {
		t.Fatalf("LoadHCL() error = %v", err)
	}

	zone := cfg.Zones[0]
	// Canonicalization moves 'interface' to Matches
	if len(zone.Matches) != 1 {
		t.Fatalf("len(Matches) = %d, want 1", len(zone.Matches))
	}
	if zone.Matches[0].Interface != "eth1" {
		t.Errorf("Matches[0].Interface = %q, want %q", zone.Matches[0].Interface, "eth1")
	}
	// Src and VLAN stay global defaults?
	// RuleMatch doesn't have VLAN? No, wait. RuleMatch HAS Src/VLAN.
	// But Zone also has them.
	// If `src` is top-level, it might NOT be cleared if it applies to all matches?
	// `validate.go` shows `zone.Src` validation.
	// `migrate_zones.go` ONLY handles `Interface`, `Interfaces`, `iface.Zone`.
	// It does NOT appear to migrate `Src` or `VLAN` into matches.
	// So checking `zone.Src` should be fine.
	if zone.Src != "192.168.10.0/24" {
		t.Errorf("Src = %q, want %q", zone.Src, "192.168.10.0/24")
	}
	if zone.VLAN != 100 {
		t.Errorf("VLAN = %d, want %d", zone.VLAN, 100)
	}
}

func TestZone_MatchBlocks(t *testing.T) {
	hcl := `
schema_version = "1.0"

zone "dmz" {
  match {
    interface = "eth2"
  }
  match {
    interface = "eth3"
  }
  management {
    web = true
    ssh = true
  }
}
`
	cfg, err := LoadHCL([]byte(hcl), "test.hcl")
	if err != nil {
		t.Fatalf("LoadHCL() error = %v", err)
	}

	zone := cfg.Zones[0]
	if len(zone.Matches) != 2 {
		t.Fatalf("len(Matches) = %d, want 2", len(zone.Matches))
	}
	if zone.Matches[0].Interface != "eth2" {
		t.Errorf("Matches[0].Interface = %q, want %q", zone.Matches[0].Interface, "eth2")
	}
	if zone.Matches[1].Interface != "eth3" {
		t.Errorf("Matches[1].Interface = %q, want %q", zone.Matches[1].Interface, "eth3")
	}
}

func TestZone_AdditiveInheritance(t *testing.T) {
	hcl := `
schema_version = "1.0"

zone "guest" {
  src = "192.168.10.0/24"

  match {
    interface = "eth1"
  }
  match {
    interface = "wlan0"
  }
}
`
	cfg, err := LoadHCL([]byte(hcl), "test.hcl")
	if err != nil {
		t.Fatalf("LoadHCL() error = %v", err)
	}

	zone := cfg.Zones[0]
	// Global src should be present
	if zone.Src != "192.168.10.0/24" {
		t.Errorf("Src = %q, want %q", zone.Src, "192.168.10.0/24")
	}
	// Matches should be present
	if len(zone.Matches) != 2 {
		t.Fatalf("len(Matches) = %d, want 2", len(zone.Matches))
	}
}

func TestZone_InterfacesAttributeRejected(t *testing.T) {
	hcl := `
schema_version = "1.0"

zone "lan" {
  interfaces = ["eth1"]
}
`
	// The interfaces attribute was removed and should now be rejected
	_, err := LoadHCLWithOptions([]byte(hcl), "test.hcl", DefaultLoadOptions())
	if err == nil {
		t.Fatal("Expected error for removed 'interfaces' attribute, but got none")
	}

	// Verify the error message references the interfaces attribute
	errStr := err.Error()
	if !(len(errStr) >= 10 && (errStr[0:10] == "interfaces" || hasSubstr(errStr, "interfaces"))) {
		t.Errorf("Error message should reference 'interfaces', got: %v", err)
	}
}

// hasSubstr checks if s contains substr (simple inline check to avoid conflicts)
func hasSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestZone_WithIPAssignment(t *testing.T) {
	hcl := `
schema_version = "1.0"

zone "lan" {
  interface = "eth1"
  ipv4 = ["192.168.1.1/24"]
  management {
    web = true
  }
}
`
	cfg, err := LoadHCL([]byte(hcl), "test.hcl")
	if err != nil {
		t.Fatalf("LoadHCL() error = %v", err)
	}

	zone := cfg.Zones[0]
	// Normalized
	if len(zone.Matches) != 1 {
		t.Fatalf("len(Matches) = %d, want 1", len(zone.Matches))
	}
	if zone.Matches[0].Interface != "eth1" {
		t.Errorf("Matches[0].Interface = %q, want %q", zone.Matches[0].Interface, "eth1")
	}
	if len(zone.IPv4) != 1 || zone.IPv4[0] != "192.168.1.1/24" {
		t.Errorf("IPv4 = %v, want [192.168.1.1/24]", zone.IPv4)
	}
}
