// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"strings"
	"testing"
)

func TestConfigFile_Diff(t *testing.T) {
	original := `ip_forwarding = false`
	modified := `ip_forwarding = true`

	cf, err := LoadConfigFromBytes("test.hcl", []byte(original))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Update the config
	if err := cf.SetRawHCL(modified); err != nil {
		t.Fatalf("Failed to set raw HCL: %v", err)
	}

	// Test simple diff (default behavior)
	diff := cf.Diff()

	expectedLines := []string{
		"--- original",
		"+++ modified",
		`-ip_forwarding = false`,
		`+ip_forwarding = true`,
	}

	for _, expected := range expectedLines {
		if !strings.Contains(diff, expected) {
			t.Errorf("Diff missing expected line: %s\nGot:\n%s", expected, diff)
		}
	}
}

func TestConfigFile_DiffStructured(t *testing.T) {
	original := `
schema_version = "1.0"

interface "eth0" {
	ipv4 = ["192.168.1.1/24"]
	zone = "lan"
}

policy "lan" "wan" {
	rule "rule1" {
		action = "accept"
		proto = "tcp"
		dest_port = 80
	}
}
`

	modified := `
schema_version = "1.0"

interface "eth0" {
	ipv4 = ["192.168.1.1/24", "10.0.0.1/24"]
	zone = "lan"
}

policy "lan" "wan" {
	rule "rule1" {
		action = "accept"
		proto = "tcp"
		dest_port = 443
	}
}
`

	cf, err := LoadConfigFromBytes("test.hcl", []byte(original))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Update the config
	if err := cf.SetRawHCL(modified); err != nil {
		t.Fatalf("Failed to set raw HCL: %v", err)
	}

	// Test structured diff
	diff, err := cf.DiffStructured()
	if err != nil {
		t.Fatalf("Failed to get structured diff: %v", err)
	}

	if !diff.HasChanges() {
		t.Error("Expected changes but diff reports none")
	}

	// Check that we have the expected changes
	if len(diff.Modified) == 0 {
		t.Error("Expected modified changes")
	}

	// Test the string representation
	diffStr := diff.String()
	if !strings.Contains(diffStr, "Modified:") {
		t.Errorf("Expected 'Modified:' in diff string, got: %s", diffStr)
	}
}

func TestConfigFile_DiffWithStructuredOption(t *testing.T) {
	original := `state_dir = "/var/lib/flywall"`
	modified := `state_dir = "/tmp/flywall"`

	cf, err := LoadConfigFromBytes("test.hcl", []byte(original))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Update the config
	if err := cf.SetRawHCL(modified); err != nil {
		t.Fatalf("Failed to set raw HCL: %v", err)
	}

	// Test with structured option
	diff := cf.Diff(true)

	// Since structured diff now supports top-level attributes, we expect structured output
	// rather than fallback to unified diff.
	if !strings.Contains(diff, "Modified:") {
		t.Errorf("Expected structured diff output, got:\n%s", diff)
	}
}

func TestConfigFile_Diff_NoChanges(t *testing.T) {
	original := `ip_forwarding = true`

	cf, err := LoadConfigFromBytes("test.hcl", []byte(original))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	diff := cf.Diff()
	if diff != "" {
		t.Errorf("Expected empty diff for no changes, got:\n%s", diff)
	}
}
