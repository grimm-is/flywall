// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// SixToFourConfig configures a 6to4 tunnel.
type SixToFourConfig struct {
	Name      string `hcl:"name,label" json:"name"`
	Interface string `hcl:"interface" json:"interface"` // Physical interface name (usually WAN)
	Enabled   bool   `hcl:"enabled,optional" json:"enabled"`
	Zone      string `hcl:"zone,optional" json:"zone,omitempty"` // Zone for the tunnel interface (tun6to4)
	MTU       int    `hcl:"mtu,optional" json:"mtu,omitempty"`   // Default 1480
}
