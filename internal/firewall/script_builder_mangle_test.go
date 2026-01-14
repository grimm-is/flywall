package firewall

import (
	"strings"
	"testing"

	"grimm.is/flywall/internal/config"
)

func TestMangleTableGeneration(t *testing.T) {
	cfg := &Config{
		MarkRules: []config.MarkRule{
			{
				Name:    "mark-test",
				Enabled: true,
				SrcIP:   "192.168.1.100",
				Mark:    "0x10",
			},
		},
	}

	sb, err := BuildMangleTableScript(cfg, "flywall")
	if err != nil {
		t.Fatalf("BuildMangleTableScript error: %v", err)
	}
	script := sb.Build()

	// Verify Mark Rule
	if !strings.Contains(script, `ip saddr 192.168.1.100 meta mark set 0x10`) {
		t.Errorf("Mark rule missing. Got:\n%s", script)
	}
}
