package firewall

import (
	"strings"
	"testing"
)

// TestScriptBuilderComments verifies that comments are correctly added to nftables commands.
func TestScriptBuilderComments(t *testing.T) {
	t.Run("ChainWithComment", func(t *testing.T) {
		sb := NewScriptBuilder("test", "inet", "UTC")
		sb.AddChain("input", "filter", "input", 0, "drop", "[base] Incoming traffic")
		script := sb.Build()

		if !strings.Contains(script, `comment "[base] Incoming traffic"`) {
			t.Errorf("Chain comment not found in output:\n%s", script)
		}
	})

	t.Run("ChainWithoutComment", func(t *testing.T) {
		sb := NewScriptBuilder("test", "inet", "UTC")
		sb.AddChain("input", "filter", "input", 0, "drop")
		script := sb.Build()

		if strings.Contains(script, "comment") {
			t.Errorf("Unexpected comment in output:\n%s", script)
		}
	})

	t.Run("RuleWithComment", func(t *testing.T) {
		sb := NewScriptBuilder("test", "inet", "UTC")
		sb.AddTable()
		sb.AddChain("input", "filter", "input", 0, "drop")
		sb.AddRule("input", "ct state established accept", "[base] Stateful")
		script := sb.Build()

		if !strings.Contains(script, `comment "[base] Stateful"`) {
			t.Errorf("Rule comment not found in output:\n%s", script)
		}
	})

	t.Run("RuleWithoutComment", func(t *testing.T) {
		sb := NewScriptBuilder("test", "inet", "UTC")
		sb.AddTable()
		sb.AddChain("input", "filter", "input", 0, "drop")
		sb.AddRule("input", "ct state established accept")
		script := sb.Build()

		// Count occurrences of "comment" - should be 0
		if strings.Contains(script, "comment") {
			t.Errorf("Unexpected comment in output:\n%s", script)
		}
	})

	t.Run("RuleSkipsDuplicateComment", func(t *testing.T) {
		sb := NewScriptBuilder("test", "inet", "UTC")
		sb.AddTable()
		sb.AddChain("input", "filter", "input", 0, "drop")
		// Rule expression already has a comment
		sb.AddRule("input", `tcp dport 22 accept comment "user-defined"`, "[source] should be skipped")
		script := sb.Build()

		// Should only have the original comment, not the added one
		if strings.Contains(script, "[source] should be skipped") {
			t.Errorf("Duplicate comment was incorrectly added:\n%s", script)
		}
		if !strings.Contains(script, `comment "user-defined"`) {
			t.Errorf("Original comment missing:\n%s", script)
		}
	})

	t.Run("SetWithComment", func(t *testing.T) {
		sb := NewScriptBuilder("test", "inet", "UTC")
		sb.AddTable()
		sb.AddSet("blocklist", "ipv4_addr", "[ipset:blocklist]", 0, "interval")
		script := sb.Build()

		if !strings.Contains(script, `comment "[ipset:blocklist]"`) {
			t.Errorf("Set comment not found in output:\n%s", script)
		}
	})

	t.Run("MapWithComment", func(t *testing.T) {
		sb := NewScriptBuilder("test", "inet", "UTC")
		sb.AddTable()
		sb.AddMap("input_vmap", "ifname", "verdict", "[base] Input dispatch", nil, nil)
		script := sb.Build()

		if !strings.Contains(script, `comment "[base] Input dispatch"`) {
			t.Errorf("Map comment not found in output:\n%s", script)
		}
	})

	t.Run("FlowtableWithComment", func(t *testing.T) {
		sb := NewScriptBuilder("test", "inet", "UTC")
		sb.AddTable()
		sb.AddFlowtable("ft", []string{"eth0"}, "[feature] Flow offload")
		script := sb.Build()

		if !strings.Contains(script, `comment "[feature] Flow offload"`) {
			t.Errorf("Flowtable comment not found in output:\n%s", script)
		}
	})

	t.Run("RuleWithQuotesIntegrity", func(t *testing.T) {
		sb := NewScriptBuilder("flywall", "inet", "UTC")
		sb.AddTable()
		sb.AddChain("input", "filter", "input", 0, "drop")
		sb.AddRule("input", `log prefix "TEST: " counter accept`, "[test] Log check")
		script := sb.Build()

		expected := `add rule inet flywall input log prefix "TEST: " counter accept comment "[test] Log check"`
		if !strings.Contains(script, expected) {
			t.Errorf("Output corruption detected.\nExpected: %s\nGot:      %s", expected, script)
		}
	})
}
