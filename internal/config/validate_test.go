// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"testing"
)

// TestValidateZones tests zone validation
func TestValidateZones(t *testing.T) {
	tests := []struct {
		name     string
		zones    []Zone
		wantErrs int
	}{
		{
			name: "valid zone with interface",
			zones: []Zone{
				{Name: "lan", Interface: "eth0"},
			},
			wantErrs: 0,
		},
		{
			name: "valid zone with src CIDR",
			zones: []Zone{
				{Name: "guest", Interface: "eth1", Src: "192.168.10.0/24"},
			},
			wantErrs: 0,
		},
		{
			name: "valid zone with VLAN",
			zones: []Zone{
				{Name: "vlan100", Interface: "eth0", VLAN: 100},
			},
			wantErrs: 0,
		},
		{
			name: "invalid src CIDR",
			zones: []Zone{
				{Name: "test", Src: "not-a-cidr"},
			},
			wantErrs: 1,
		},
		{
			name: "invalid dst IP",
			zones: []Zone{
				{Name: "test", Dst: "invalid-ip"},
			},
			wantErrs: 1,
		},
		{
			name: "VLAN too low",
			zones: []Zone{
				{Name: "test", VLAN: 0}, // 0 is ignored (unset)
			},
			wantErrs: 0,
		},
		{
			name: "VLAN too high",
			zones: []Zone{
				{Name: "test", VLAN: 4095},
			},
			wantErrs: 1,
		},
		{
			name: "VLAN negative",
			zones: []Zone{
				{Name: "test", VLAN: -1},
			},
			wantErrs: 1,
		},
		{
			name: "valid match block",
			zones: []Zone{
				{Name: "dmz", Matches: []RuleMatch{
					{Interface: "eth2", Src: "10.0.0.0/8", VLAN: 200},
				}},
			},
			wantErrs: 0,
		},
		{
			name: "invalid match block src",
			zones: []Zone{
				{Name: "dmz", Matches: []RuleMatch{
					{Interface: "eth2", Src: "bad-cidr"},
				}},
			},
			wantErrs: 1,
		},
		{
			name: "invalid match block VLAN",
			zones: []Zone{
				{Name: "dmz", Matches: []RuleMatch{
					{Interface: "eth2", VLAN: 5000},
				}},
			},
			wantErrs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Zones: tt.zones}
			errs := cfg.validateZones()
			if len(errs) != tt.wantErrs {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
		})
	}
}

// TestValidateInterfaces tests interface validation
func TestValidateInterfaces(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []Interface
		wantErrs   int
	}{
		{
			name: "valid interfaces",
			interfaces: []Interface{
				{Name: "eth0", Zone: "wan", IPv4: []string{"192.168.1.1/24"}},
				{Name: "eth1", Zone: "lan", IPv4: []string{"10.0.0.1/24"}},
			},
			wantErrs: 0,
		},
		{
			name: "duplicate interface names",
			interfaces: []Interface{
				{Name: "eth0", Zone: "wan"},
				{Name: "eth0", Zone: "lan"},
			},
			wantErrs: 1,
		},
		{
			name: "invalid interface name",
			interfaces: []Interface{
				{Name: "123invalid"},
			},
			wantErrs: 1,
		},
		{
			name: "invalid IPv4 CIDR",
			interfaces: []Interface{
				{Name: "eth0", IPv4: []string{"not-an-ip"}},
			},
			wantErrs: 1,
		},
		{
			name: "invalid MTU",
			interfaces: []Interface{
				{Name: "eth0", MTU: 100},
			},
			wantErrs: 1,
		},
		{
			name: "invalid VLAN ID",
			interfaces: []Interface{
				{Name: "eth0", VLANs: []VLAN{{ID: "9999"}}},
			},
			wantErrs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Interfaces: tt.interfaces}
			errs := cfg.validateInterfaces()
			if len(errs) != tt.wantErrs {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
		})
	}
}

// TestValidateIPSets tests IPSet validation
func TestValidateIPSets(t *testing.T) {
	tests := []struct {
		name     string
		ipsets   []IPSet
		wantErrs int
	}{
		{
			name: "valid ipset",
			ipsets: []IPSet{
				{Name: "blocklist", Type: "ipv4_addr", Entries: []string{"1.2.3.4", "10.0.0.0/8"}},
			},
			wantErrs: 0,
		},
		{
			name: "duplicate names",
			ipsets: []IPSet{
				{Name: "test"},
				{Name: "test"},
			},
			wantErrs: 1,
		},
		{
			name: "invalid name format",
			ipsets: []IPSet{
				{Name: "123-invalid"},
			},
			wantErrs: 1,
		},
		{
			name: "invalid type",
			ipsets: []IPSet{
				{Name: "test", Type: "invalid_type"},
			},
			wantErrs: 1,
		},
		{
			name: "invalid entry",
			ipsets: []IPSet{
				{Name: "test", Entries: []string{"not-an-ip"}},
			},
			wantErrs: 1,
		},

		{
			name: "invalid URL",
			ipsets: []IPSet{
				{Name: "test", URL: "not-a-url"},
			},
			wantErrs: 1,
		},
		{
			name: "valid URL",
			ipsets: []IPSet{
				{Name: "test", URL: "https://example.com/list.txt"},
			},
			wantErrs: 0,
		},
		{
			name: "negative refresh hours",
			ipsets: []IPSet{
				{Name: "test", RefreshHours: -1},
			},
			wantErrs: 1,
		},
		{
			name: "invalid action",
			ipsets: []IPSet{
				{Name: "test", Action: "invalid"},
			},
			wantErrs: 1,
		},
		{
			name: "invalid apply_to",
			ipsets: []IPSet{
				{Name: "test", ApplyTo: "invalid"},
			},
			wantErrs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{IPSets: tt.ipsets}
			errs := cfg.validateIPSets()
			if len(errs) != tt.wantErrs {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
		})
	}
}

// TestValidatePolicies tests policy validation
func TestValidatePolicies(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []Interface
		ipsets     []IPSet
		policies   []Policy
		wantErrs   int
	}{
		{
			name:       "valid policy",
			interfaces: []Interface{{Name: "eth0", Zone: "wan"}, {Name: "eth1", Zone: "lan"}},
			policies: []Policy{
				{From: "lan", To: "wan", Rules: []PolicyRule{{Action: "accept"}}},
			},
			wantErrs: 0,
		},
		{
			name:       "unknown from zone",
			interfaces: []Interface{{Name: "eth0", Zone: "wan"}},
			policies: []Policy{
				{From: "unknown", To: "wan"},
			},
			wantErrs: 1,
		},
		{
			name:       "firewall zone is allowed",
			interfaces: []Interface{{Name: "eth0", Zone: "wan"}},
			policies: []Policy{
				{From: "wan", To: "firewall"},
			},
			wantErrs: 0,
		},
		{
			name:       "invalid action in rule",
			interfaces: []Interface{{Name: "eth0", Zone: "wan"}},
			policies: []Policy{
				{From: "wan", To: "firewall", Rules: []PolicyRule{{Action: "invalid"}}},
			},
			wantErrs: 1,
		},
		{
			name:       "invalid protocol",
			interfaces: []Interface{{Name: "eth0", Zone: "wan"}},
			policies: []Policy{
				{From: "wan", To: "firewall", Rules: []PolicyRule{{Action: "accept", Protocol: "invalid"}}},
			},
			wantErrs: 1,
		},
		{
			name:       "invalid port",
			interfaces: []Interface{{Name: "eth0", Zone: "wan"}},
			policies: []Policy{
				{From: "wan", To: "firewall", Rules: []PolicyRule{{Action: "accept", DestPort: 99999}}},
			},
			wantErrs: 1,
		},
		{
			name:       "unknown IPSet reference",
			interfaces: []Interface{{Name: "eth0", Zone: "wan"}},
			policies: []Policy{
				{From: "wan", To: "firewall", Rules: []PolicyRule{{Action: "accept", SrcIPSet: "nonexistent"}}},
			},
			wantErrs: 1,
		},
		{
			name:       "valid IPSet reference",
			interfaces: []Interface{{Name: "eth0", Zone: "wan"}},
			ipsets:     []IPSet{{Name: "blocklist"}},
			policies: []Policy{
				{From: "wan", To: "firewall", Rules: []PolicyRule{{Action: "drop", SrcIPSet: "blocklist"}}},
			},
			wantErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Interfaces: tt.interfaces, IPSets: tt.ipsets, Policies: tt.policies}
			errs := cfg.validatePolicies()
			if len(errs) != tt.wantErrs {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
		})
	}
}

// TestValidateNAT tests NAT validation
func TestValidateNAT(t *testing.T) {
	tests := []struct {
		name     string
		nat      []NATRule
		wantErrs int
	}{
		{
			name:     "valid masquerade",
			nat:      []NATRule{{Type: "masquerade"}},
			wantErrs: 0,
		},
		{
			name:     "valid snat",
			nat:      []NATRule{{Type: "snat"}},
			wantErrs: 0,
		},
		{
			name:     "valid dnat",
			nat:      []NATRule{{Type: "dnat"}},
			wantErrs: 0,
		},
		{
			name:     "invalid type",
			nat:      []NATRule{{Type: "invalid"}},
			wantErrs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{NAT: tt.nat}
			errs := cfg.validateNAT()
			if len(errs) != tt.wantErrs {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
		})
	}
}

// TestValidateRoutes tests route validation
func TestValidateRoutes(t *testing.T) {
	tests := []struct {
		name     string
		routes   []Route
		wantErrs int
	}{
		{
			name:     "valid route",
			routes:   []Route{{Destination: "10.0.0.0/8", Gateway: "192.168.1.1"}},
			wantErrs: 0,
		},
		{
			name:     "invalid destination",
			routes:   []Route{{Destination: "not-a-cidr"}},
			wantErrs: 1,
		},
		{
			name:     "invalid gateway",
			routes:   []Route{{Destination: "10.0.0.0/8", Gateway: "not-an-ip"}},
			wantErrs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Routes: tt.routes}
			errs := cfg.validateRoutes()
			if len(errs) != tt.wantErrs {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
		})
	}
}

// TestValidationHelpers tests helper functions
func TestValidationHelpers(t *testing.T) {
	// isValidInterfaceName
	t.Run("isValidInterfaceName", func(t *testing.T) {
		valid := []string{"eth0", "enp0s3", "wlan0", "br-lan", "eth0.100"}
		for _, name := range valid {
			if !isValidInterfaceName(name) {
				t.Errorf("%s should be valid", name)
			}
		}
		invalid := []string{"", "123eth", "a-very-long-interface-name-over-15-chars"}
		for _, name := range invalid {
			if isValidInterfaceName(name) {
				t.Errorf("%s should be invalid", name)
			}
		}
	})

	// isValidSetName
	t.Run("isValidSetName", func(t *testing.T) {
		valid := []string{"blocklist", "allow_list", "test-set"}
		for _, name := range valid {
			if !isValidSetName(name) {
				t.Errorf("%s should be valid", name)
			}
		}
		invalid := []string{"", "123set"}
		for _, name := range invalid {
			if isValidSetName(name) {
				t.Errorf("%s should be invalid", name)
			}
		}
	})

	// isValidCIDR
	t.Run("isValidCIDR", func(t *testing.T) {
		if !isValidCIDR("192.168.1.0/24") {
			t.Error("192.168.1.0/24 should be valid")
		}
		if isValidCIDR("192.168.1.1") {
			t.Error("192.168.1.1 (no mask) should be invalid for CIDR")
		}
	})

	// isValidIPOrCIDR
	t.Run("isValidIPOrCIDR", func(t *testing.T) {
		valid := []string{"192.168.1.1", "10.0.0.0/8", "2001:db8::1"}
		for _, ip := range valid {
			if !isValidIPOrCIDR(ip) {
				t.Errorf("%s should be valid", ip)
			}
		}
		if isValidIPOrCIDR("not-an-ip") {
			t.Error("not-an-ip should be invalid")
		}
	})

	// isValidURL
	t.Run("isValidURL", func(t *testing.T) {
		if !isValidURL("https://example.com") {
			t.Error("https URL should be valid")
		}
		if !isValidURL("http://example.com") {
			t.Error("http URL should be valid")
		}
		if isValidURL("ftp://example.com") {
			t.Error("ftp URL should be invalid")
		}
	})
}

// TestValidateFullConfig tests the full Validate method
func TestValidateFullConfig(t *testing.T) {
	cfg := &Config{
		Interfaces: []Interface{{Name: "eth0", Zone: "wan", IPv4: []string{"192.168.1.1/24"}}},
		IPSets:     []IPSet{{Name: "blocklist", Type: "ipv4_addr"}},
		Policies:   []Policy{{From: "wan", To: "firewall", Rules: []PolicyRule{{Action: "accept"}}}},
		NAT:        []NATRule{{Type: "masquerade"}},
		Routes:     []Route{{Destination: "10.0.0.0/8", Gateway: "192.168.1.254"}},
	}

	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid config, got: %v", errs)
	}
}

// TestValidationErrorMethods tests ValidationError and ValidationErrors methods
func TestValidationErrorMethods(t *testing.T) {
	err := ValidationError{Field: "test.field", Message: "is invalid"}
	if err.Error() != "test.field: is invalid" {
		t.Errorf("unexpected error string: %s", err.Error())
	}

	errs := ValidationErrors{}
	if errs.Error() != "" {
		t.Error("empty errors should return empty string")
	}
	if errs.HasErrors() {
		t.Error("empty errors should not have errors")
	}

	errs = append(errs, err)
	if !errs.HasErrors() {
		t.Error("should have errors")
	}
}

// TestIntentValidator_ValidateIntent tests the intent validation functionality
func TestIntentValidator_ValidateIntent(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: &Config{
				Interfaces: []Interface{
					{Name: "eth0", IPv4: []string{"192.168.1.1/24"}, Zone: "lan"},
				},
				Zones: []Zone{
					{Name: "lan"},
				},
				Policies: []Policy{
					{
						From: "lan",
						To:   "wan",
						Rules: []PolicyRule{
							{Action: "accept", Protocol: "tcp", DestPort: 80},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "conflicting rules",
			config: &Config{
				Interfaces: []Interface{
					{Name: "eth0", IPv4: []string{"192.168.1.1/24"}, Zone: "lan"},
				},
				Zones: []Zone{
					{Name: "lan"},
				},
				Policies: []Policy{
					{
						From: "lan",
						To:   "wan",
						Rules: []PolicyRule{
							{Action: "accept", Protocol: "tcp", DestPort: 80},
							{Action: "drop", Protocol: "tcp", DestPort: 80},
						},
					},
				},
			},
			expectError: true,
			errorField:  "policies",
		},
		{
			name: "interface in multiple zones",
			config: &Config{
				Interfaces: []Interface{
					{Name: "eth0", IPv4: []string{"192.168.1.1/24"}, Zone: "lan"},
					{Name: "eth0", IPv4: []string{"10.0.0.1/24"}, Zone: "dmz"},
				},
				Zones: []Zone{
					{Name: "lan"},
					{Name: "dmz"},
				},
			},
			expectError: true,
			errorField:  "interfaces",
		},
		{
			name: "overly permissive rule",
			config: &Config{
				Interfaces: []Interface{
					{Name: "eth0", IPv4: []string{"192.168.1.1/24"}, Zone: "lan"},
				},
				Zones: []Zone{
					{Name: "lan"},
				},
				Policies: []Policy{
					{
						From: "lan",
						To:   "wan",
						Rules: []PolicyRule{
							{Action: "accept"},
						},
					},
				},
			},
			expectError: true,
			errorField:  "policies",
		},
		{
			name: "undefined zone reference",
			config: &Config{
				Interfaces: []Interface{
					{Name: "eth0", IPv4: []string{"192.168.1.1/24"}, Zone: "nonexistent"},
				},
				Zones: []Zone{
					{Name: "lan"},
				},
			},
			expectError: true,
			errorField:  "interfaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewIntentValidator(tt.config)
			errors := validator.ValidateIntent()

			hasError := len(errors) > 0
			if hasError != tt.expectError {
				t.Errorf("ValidateIntent() error = %v, expectError %v", hasError, tt.expectError)
				return
			}

			if tt.expectError && len(errors) > 0 {
				found := false
				for _, err := range errors {
					if err.Field == tt.errorField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error in field %s, got errors in fields: %v", tt.errorField, getFieldNames(errors))
				}
			}
		})
	}
}

func getFieldNames(errors []ValidationError) []string {
	names := make([]string, len(errors))
	for i, err := range errors {
		names[i] = err.Field
	}
	return names
}

// TestIntentValidator_rulesConflict tests rule conflict detection
func TestIntentValidator_rulesConflict(t *testing.T) {
	validator := NewIntentValidator(&Config{})

	tests := []struct {
		name     string
		rule1    PolicyRule
		rule2    PolicyRule
		conflict bool
	}{
		{
			name:     "same action no conflict",
			rule1:    PolicyRule{Action: "accept", Protocol: "tcp", DestPort: 80},
			rule2:    PolicyRule{Action: "accept", Protocol: "tcp", DestPort: 80},
			conflict: false,
		},
		{
			name:     "different actions same criteria",
			rule1:    PolicyRule{Action: "accept", Protocol: "tcp", DestPort: 80},
			rule2:    PolicyRule{Action: "drop", Protocol: "tcp", DestPort: 80},
			conflict: true,
		},
		{
			name:     "different protocols no conflict",
			rule1:    PolicyRule{Action: "accept", Protocol: "tcp", DestPort: 80},
			rule2:    PolicyRule{Action: "drop", Protocol: "udp", DestPort: 80},
			conflict: false,
		},
		{
			name:     "different ports no conflict",
			rule1:    PolicyRule{Action: "accept", Protocol: "tcp", DestPort: 80},
			rule2:    PolicyRule{Action: "drop", Protocol: "tcp", DestPort: 443},
			conflict: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.rulesConflict(tt.rule1, tt.rule2)
			if result != tt.conflict {
				t.Errorf("rulesConflict() = %v, want %v", result, tt.conflict)
			}
		})
	}
}

// TestIntentValidator_isOverlyPermissive tests overly permissive rule detection
func TestIntentValidator_isOverlyPermissive(t *testing.T) {
	validator := NewIntentValidator(&Config{})

	tests := []struct {
		name       string
		rule       PolicyRule
		permissive bool
	}{
		{
			name:       "accept all",
			rule:       PolicyRule{Action: "accept"},
			permissive: true,
		},
		{
			name:       "accept with src IP",
			rule:       PolicyRule{Action: "accept", SrcIP: "192.168.1.0/24"},
			permissive: false,
		},
		{
			name:       "drop all",
			rule:       PolicyRule{Action: "drop"},
			permissive: false,
		},
		{
			name:       "accept with port",
			rule:       PolicyRule{Action: "accept", DestPort: 80},
			permissive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.isOverlyPermissive(tt.rule)
			if result != tt.permissive {
				t.Errorf("isOverlyPermissive() = %v, want %v", result, tt.permissive)
			}
		})
	}
}

// TestDeepValidate tests the comprehensive validation functionality
func TestDeepValidate(t *testing.T) {
	config := &Config{
		Interfaces: []Interface{
			{Name: "eth0", IPv4: []string{"192.168.1.1/24"}, Zone: "lan"},
		},
		Zones: []Zone{
			{Name: "lan"},
			{Name: "wan"},
		},
		Policies: []Policy{
			{
				From: "lan",
				To:   "wan",
				Rules: []PolicyRule{
					{Action: "accept", Protocol: "tcp", DestPort: 80, Log: true},
				},
			},
		},
	}

	errors := config.DeepValidate()
	if len(errors) > 0 {
		t.Errorf("DeepValidate() returned unexpected errors: %v", errors)
	}
}

// TestDeepValidate_WithErrors tests validation with intentional errors
func TestDeepValidate_WithErrors(t *testing.T) {
	config := &Config{
		Interfaces: []Interface{
			{Name: "eth0", IPv4: []string{"invalid-ip"}, Zone: "nonexistent"},
		},
		Zones: []Zone{
			{Name: "lan"},
		},
		Policies: []Policy{
			{
				From: "nonexistent",
				To:   "another-nonexistent",
				Rules: []PolicyRule{
					{Action: "accept"},
				},
			},
		},
	}

	errors := config.DeepValidate()
	if len(errors) == 0 {
		t.Error("DeepValidate() should have returned errors but didn't")
	}

	// Check for expected error types
	found := map[string]bool{
		"interfaces": false,
		"policies":   false,
	}

	for _, err := range errors {
		if err.Field == "interfaces" {
			found["interfaces"] = true
		}
		if err.Field == "policies" {
			found["policies"] = true
		}
	}

	for field, found := range found {
		if !found {
			t.Errorf("Expected error in field %s but none found", field)
		}
	}
}
