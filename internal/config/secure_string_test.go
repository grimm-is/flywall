// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

func TestSecureStringDecoding(t *testing.T) {
	hcl := `
		private_key = "secret_value"
		listen_port = 51820
	`

	type TestConfig struct {
		PrivateKey SecureString `hcl:"private_key"`
		ListenPort int          `hcl:"listen_port"`
	}

	var cfg TestConfig
	err := hclsimple.Decode("test.hcl", []byte(hcl), nil, &cfg)
	if err != nil {
		t.Fatalf("Failed to decode HCL: %v", err)
	}

	if cfg.PrivateKey != "secret_value" {
		t.Errorf("Expected PrivateKey 'secret_value', got '%s'", cfg.PrivateKey)
	}
	if cfg.ListenPort != 51820 {
		t.Errorf("Expected ListenPort 51820, got %d", cfg.ListenPort)
	}
}

func TestWireGuardBlockDecoding(t *testing.T) {
	hcl := `
		wireguard "wg0" {
			enabled = true
			interface = "wg0"
			private_key = "secret_wg_key"
			listen_port = 51820
		}
	`

	type WGConfig struct {
		Name       string       `hcl:"name,label"`
		Enabled    bool         `hcl:"enabled,optional"`
		Interface  string       `hcl:"interface,optional"`
		PrivateKey SecureString `hcl:"private_key,optional"`
		ListenPort int          `hcl:"listen_port,optional"`
	}

	type Config struct {
		WG []WGConfig `hcl:"wireguard,block"`
	}

	var cfg Config
	err := hclsimple.Decode("test.hcl", []byte(hcl), nil, &cfg)
	if err != nil {
		t.Fatalf("Failed to decode HCL: %v", err)
	}

	if len(cfg.WG) != 1 {
		t.Fatalf("Expected 1 WG config, got %d", len(cfg.WG))
	}
	wg := cfg.WG[0]
	if wg.Name != "wg0" {
		t.Errorf("Expected Name 'wg0', got '%s'", wg.Name)
	}
	if !wg.Enabled {
		t.Error("Expected Enabled true")
	}
	if wg.PrivateKey != "secret_wg_key" {
		t.Errorf("Expected PrivateKey 'secret_wg_key', got '%s'", wg.PrivateKey)
	}
	if wg.ListenPort != 51820 {
		t.Errorf("Expected ListenPort 51820, got %d", wg.ListenPort)
	}
}
