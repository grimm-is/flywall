package vpn

import (
	"strings"
	"testing"
)

func TestParseWireGuardConfig(t *testing.T) {
	configStr := `
[Interface]
PrivateKey = aaaaaa
ListenPort = 51820
Address = 10.0.0.1/24
DNS = 1.1.1.1
MTU = 1300
Table = 123
PostUp = ip rule add
PostDown = ip rule del

[Peer]
PublicKey = bbbbbb
PresharedKey = cccccc
Endpoint = 1.2.3.4:51820
AllowedIPs = 0.0.0.0/0, ::/0
`

	reader := strings.NewReader(configStr)
	cfg, err := ParseWireGuardConfig(reader)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if cfg.PrivateKey != "aaaaaa" {
		t.Errorf("Expected PrivateKey 'aaaaaa', got '%s'", cfg.PrivateKey)
	}
	if cfg.ListenPort != 51820 {
		t.Errorf("Expected ListenPort 51820, got %d", cfg.ListenPort)
	}
	if len(cfg.Address) != 1 || cfg.Address[0] != "10.0.0.1/24" {
		t.Errorf("Unexpected Address: %v", cfg.Address)
	}
	if len(cfg.DNS) != 1 || cfg.DNS[0] != "1.1.1.1" {
		t.Errorf("Unexpected DNS: %v", cfg.DNS)
	}
	if cfg.MTU != 1300 {
		t.Errorf("Expected MTU 1300, got %d", cfg.MTU)
	}
	if cfg.Table != "123" {
		t.Errorf("Expected Table '123', got '%s'", cfg.Table)
	}
	if len(cfg.PostUp) != 1 || cfg.PostUp[0] != "ip rule add" {
		t.Errorf("Unexpected PostUp: %v", cfg.PostUp)
	}
	if len(cfg.PostDown) != 1 || cfg.PostDown[0] != "ip rule del" {
		t.Errorf("Unexpected PostDown: %v", cfg.PostDown)
	}
}
