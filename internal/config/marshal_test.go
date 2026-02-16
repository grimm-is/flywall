// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecureString_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    SecureString
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: `""`,
		},
		{
			name:     "Non-empty string",
			input:    "secret-value",
			expected: `"(hidden)"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(bytes))
		})
	}
}

func TestConfig_MarshalJSON(t *testing.T) {
	cfg := &Config{
		SchemaVersion: "1.0",
		IPForwarding:  true,
		VPN: &VPNConfig{
			WireGuard: []WireGuardConfig{
				{
					Name:       "wg0",
					PrivateKey: "secret-key",
					Peers: []WireGuardPeerConfig{
						{
							PublicKey:    "public-key",
							PresharedKey: "preshared-secret",
						},
					},
				},
			},
		},
	}

	// Set a sensitive field in Security (if we moved generic auth there)
	// Example: cfg.Security... but Security fields are usually nested.
	// Let's assume we use one of the updated fields in security.go, e.g.
	// However, Config struct in config.go doesn't embed security.go structs directly at top level
	// except via blocks.
	// Let's verify via WireGuard which definitely uses SecureString now.

	bytes, err := json.Marshal(cfg)
	assert.NoError(t, err)

	type checkType struct {
		IPForwarding bool `json:"ip_forwarding"`
		VPN          *struct {
			WireGuard []struct {
				PrivateKey string `json:"private_key"`
				Peers      []struct {
					PresharedKey string `json:"preshared_key"`
				} `json:"peer"`
			} `json:"wireguard"`
		} `json:"vpn"`
	}

	var result checkType
	err = json.Unmarshal(bytes, &result)
	assert.NoError(t, err)

	assert.True(t, result.IPForwarding)
	assert.Equal(t, "(hidden)", result.VPN.WireGuard[0].PrivateKey)
	assert.Equal(t, "(hidden)", result.VPN.WireGuard[0].Peers[0].PresharedKey)
}
