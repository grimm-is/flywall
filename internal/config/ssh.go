// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// SSHConfig configures the internal SSH server
type SSHConfig struct {
	Enabled       bool   `hcl:"enabled,optional" json:"enabled"`
	ListenAddress string `hcl:"listen_address,optional" json:"listen_address,omitempty"` // Default: ":2222"
	Port          int    `hcl:"port,optional" json:"port,omitempty"`                     // Default: 2222
	// HostKeyPath is the path to the SSH host key (PEM or OpenSSH format)
	// If empty, the server will generate one or use defaults.
	HostKeyPath string `hcl:"host_key_path,optional" json:"host_key_path,omitempty"`

	// AuthorizedKeysPath is the path to authorized_keys file (optional)
	// If empty, simple password auth or no auth might be used (default: no auth/allow all for now? Or specific user?)
	AuthorizedKeysPath string `hcl:"authorized_keys_path,optional" json:"authorized_keys_path,omitempty"`
}
