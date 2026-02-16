// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"strings"
	"testing"
)

func TestHCLRoundTripComments(t *testing.T) {
	hclSource := `
// System settings
ip_forwarding = true # enable routing
mss_clamping  = false

/*
  Network interfaces
*/
interface "eth0" {
	// WAN interface
	zone = "wan"
	dhcp = true
}

interface "eth1" {
	zone = "lan"
	ipv4 = ["192.168.1.1/24"]
}
`
	cf, err := LoadConfigFromBytes("test.hcl", []byte(hclSource))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 1. Update a top-level attribute
	err = cf.SetAttribute("ip_forwarding", false)
	if err != nil {
		t.Fatalf("Failed to set attribute: %v", err)
	}

	// 2. Update a section
	newInterfaceHCL := `
interface "eth0" {
	// Updated WAN interface
	zone = "wan"
	dhcp = false
}
`
	err = cf.SetSectionByLabel("interface", []string{"eth0"}, newInterfaceHCL)
	if err != nil {
		t.Fatalf("Failed to set section: %v", err)
	}

	output := cf.GetRawHCL()

	// Check if comments are preserved
	expectedComments := []string{
		"// System settings",
		"# enable routing",
		"/*",
		"Network interfaces",
		"*/",
		"// Updated WAN interface",
	}

	for _, comment := range expectedComments {
		if !strings.Contains(output, comment) {
			t.Errorf("Comment %q not found in output:\n%s", comment, output)
		}
	}

	// Check if eth1 (untouched) still has its comments/structure
	if !strings.Contains(output, `interface "eth1"`) {
		t.Errorf("interface eth1 not found in output")
	}
}
