// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"strings"
	"testing"

	"grimm.is/flywall/internal/config"
)

// TestFilterTableGeneration tests high-level script generation.
func TestFilterTableGeneration(t *testing.T) {
	cfg := &config.Config{
		Zones: []config.Zone{
			{Name: "LAN", Matches: []config.RuleMatch{{Interface: "eth1"}}},
			{Name: "WAN", Matches: []config.RuleMatch{{Interface: "eth0"}}},
		},
		Policies: []config.Policy{
			{
				From: "LAN", To: "WAN", Action: "accept",
				Rules: []config.PolicyRule{
					{Protocol: "tcp", DestPort: 443, Action: "accept"},
				},
			},
			{
				From: "LAN", To: "Firewall", Action: "accept",
				Rules: []config.PolicyRule{
					{Protocol: "tcp", DestPort: 22, Action: "accept"},
				},
			},
		},
	}

	sb, err := BuildFilterTableScript(FromGlobalConfig(cfg), nil, "test_table", "", nil)
	if err != nil {
		t.Fatalf("BuildFilterTableScript() error = %v", err)
	}
	script := sb.Build()

	// Verify Chain Creation
	if !strings.Contains(script, "add chain inet test_table policy_lan_wan") {
		t.Error("Missing policy chain creation")
	}

	// Verify Verdict Map Usage (Replacing linear jumps)
	if !strings.Contains(script, "add map inet test_table input_vmap") {
		t.Error("Missing input_vmap creation")
	}
	if !strings.Contains(script, "iifname vmap @input_vmap") {
		t.Error("Missing input vmap rule")
	}

	if !strings.Contains(script, "add map inet test_table forward_vmap") {
		t.Error("Missing forward_vmap creation")
	}
	if !strings.Contains(script, `meta iifname . meta oifname vmap @forward_vmap`) {
		t.Error("Missing forward vmap rule")
	}

	// Verify Rule Content
	if !strings.Contains(script, "meta l4proto tcp tcp dport 443 counter accept") {
		t.Error("Missing policy rule content")
	}
}

func TestFlowOffloadGeneration(t *testing.T) {
	cfg := &config.Config{
		EnableFlowOffload: true,
		Interfaces: []config.Interface{
			{Name: "eth0"},
			{Name: "eth1"},
		},
	}

	sb, err := BuildFilterTableScript(FromGlobalConfig(cfg), nil, "test_table", "", nil)
	if err != nil {
		t.Fatalf("BuildFilterTableScript() error = %v", err)
	}
	script := sb.Build()

	// Verify Flowtable Creation
	if !strings.Contains(script, "add flowtable inet test_table ft") {
		t.Error("Missing flowtable definition")
	}

	// Verify Flowtable Rule
	if !strings.Contains(script, "ip protocol { tcp, udp } flow add @ft") {
		t.Error("Missing flow offload rule")
	}
}

func TestConcatenatedSetsGeneration(t *testing.T) {
	cfg := &config.Config{
		Interfaces: []config.Interface{
			{
				Name:       "eth0",
				Management: &config.ZoneManagement{SSH: true, Web: true},
			},
			{
				Name:       "eth1",
				Management: &config.ZoneManagement{SSH: true},
			},
		},
	}

	sb, err := BuildFilterTableScript(FromGlobalConfig(cfg), nil, "test_table", "", nil)
	if err != nil {
		t.Fatalf("BuildFilterTableScript() error = %v", err)
	}
	script := sb.Build()

	// Verify Rule Existence
	// We expect: iifname . tcp dport { ... } accept
	if !strings.Contains(script, "iifname . tcp dport {") {
		t.Error("Missing concatenated set rule for TCP")
	}

	// Verify Elements
	// ssh on eth0, eth1 -> "eth0" . 22, "eth1" . 22
	// web on eth0 -> "eth0" . 80, "eth0" . 443
	expectedElements := []string{
		`"eth0" . 22`,
		`"eth1" . 22`,
		`"eth0" . 80`,
		`"eth0" . 443`,
	}

	for _, elem := range expectedElements {
		if !strings.Contains(script, elem) {
			t.Errorf("Missing element %s in script\nScript:\n%s", elem, script)
		}
	}
}

// TestBuildFilterTableScriptComments verifies that BuildFilterTableScript generates expected comments.
func TestBuildFilterTableScriptComments(t *testing.T) {
	cfg := &Config{
		Zones: []config.Zone{
			{Name: "LAN"},
			{Name: "WAN"},
		},
		Interfaces: []config.Interface{
			{Name: "eth0", Zone: "LAN"},
			{Name: "eth1", Zone: "WAN"},
		},
		Policies: []config.Policy{
			{From: "LAN", To: "WAN", Action: "accept"},
		},
	}

	sb, err := BuildFilterTableScript(cfg, nil, "flywall", "abc123", nil)
	if err != nil {
		t.Fatalf("BuildFilterTableScript error: %v", err)
	}
	script := sb.Build()

	// Verify base chain comments
	expectedComments := []string{
		`[base] Incoming traffic`,
		`[base] Routed traffic`,
		`[base] Outgoing traffic`,
		`[base] Loopback`,
		`[base] Stateful`,
	}

	for _, comment := range expectedComments {
		if !strings.Contains(script, comment) {
			t.Errorf("Expected comment %q not found in script", comment)
		}
	}

	// Verify policy chain comment
	if !strings.Contains(script, `[policy:lan->wan]`) {
		t.Errorf("Policy chain comment not found in script")
	}
}

func TestDNSSetFilterGeneration(t *testing.T) {
	cfg := &config.Config{
		IPSets: []config.IPSet{
			{Name: "google_dns", Type: "dns", Domains: []string{"google.com"}, Size: 100},
			{Name: "yahoo_dns", Type: "dns", Domains: []string{"yahoo.com"}}, // Default size
		},
	}

	sb, err := BuildFilterTableScript(FromGlobalConfig(cfg), nil, "test_table", "", nil)
	if err != nil {
		t.Fatalf("BuildFilterTableScript() error = %v", err)
	}
	script := sb.Build()

	// Verify standard DNS set (ipv4_addr, size explicitly set)
	if !strings.Contains(script, `add set inet test_table google_dns { type ipv4_addr; size 100; comment "[ipset:google_dns]"; }`) {
		t.Errorf("Missing or incorrect google_dns set definition:\n%s", script)
	}

	// Verify default size optimization for DNS set
	if !strings.Contains(script, `add set inet test_table yahoo_dns { type ipv4_addr; size 65535; comment "[ipset:yahoo_dns]"; }`) {
		t.Errorf("Missing or incorrect yahoo_dns set definition (optimization check):\n%s", script)
	}
}
