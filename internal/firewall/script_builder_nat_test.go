package firewall

import (
	"strings"
	"testing"

	"grimm.is/flywall/internal/config"
)

func TestNATTableGeneration(t *testing.T) {
	cfg := &Config{
		NAT: []config.NATRule{
			{
				Name:         "masq",
				Type:         "masquerade",
				OutInterface: "eth0",
			},
			{
				Name:     "dnat-service",
				Type:     "dnat",
				Protocol: "tcp",
				DestPort: "8080",
				ToIP:     "10.0.0.10",
				ToPort:   "80",
			},
		},
	}

	sb, err := BuildNATTableScript(cfg, "flywall")
	if err != nil {
		t.Fatalf("BuildNATTableScript error: %v", err)
	}
	script := sb.Build()

	// Verify Masquerade
	if !strings.Contains(script, `oifname "eth0" masquerade`) {
		t.Errorf("Masquerade rule missing. Got:\n%s", script)
	}

	// Verify DNAT
	if !strings.Contains(script, `tcp dport 8080 dnat to 10.0.0.10:80`) {
		t.Errorf("DNAT rule missing. Got:\n%s", script)
	}
}
