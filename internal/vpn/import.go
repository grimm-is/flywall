package vpn

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseWireGuardConfig parses a standard WireGuard configuration file (INI format).
// It accepts a reader and returns a WireGuardConfig and an error.
func ParseWireGuardConfig(r io.Reader) (*WireGuardConfig, error) {
	scanner := bufio.NewScanner(r)
	config := &WireGuardConfig{
		Enabled: true,
		MTU:     1420, // Default
	}

	var currentSection string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(line[1 : len(line)-1])
			// If starting a new Peer section, append the previous one (confusing in INI parsing without a struct list, but standard practice)
			// Actually, for multiple peers, we need to handle them.
			// Let's create a new peer struct when we hit [Peer]
			if currentSection == "peer" {
				config.Peers = append(config.Peers, WireGuardPeer{})
			}
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch currentSection {
		case "interface":
			switch key {
			case "privatekey":
				config.PrivateKey = value
			case "listenport":
				if port, err := strconv.Atoi(value); err == nil {
					config.ListenPort = port
				}
			case "address":
				// Handle comma-separated addresses
				addrs := strings.Split(value, ",")
				for _, addr := range addrs {
					config.Address = append(config.Address, strings.TrimSpace(addr))
				}
			case "dns":
				// Handle comma-separated DNS
				servers := strings.Split(value, ",")
				for _, server := range servers {
					config.DNS = append(config.DNS, strings.TrimSpace(server))
				}
			case "mtu":
				if mtu, err := strconv.Atoi(value); err == nil {
					config.MTU = mtu
				}
			case "fwmark":
				if mark, err := strconv.Atoi(value); err == nil {
					config.FWMark = mark
				}
			case "table":
				config.Table = value
			case "postup", "post_up":
				config.PostUp = append(config.PostUp, value)
			case "postdown", "post_down":
				config.PostDown = append(config.PostDown, value)
			}

		case "peer":
			if len(config.Peers) == 0 {
				// Should have hit [Peer] section header first
				continue
			}
			// Modify the last peer added
			peerIdx := len(config.Peers) - 1
			peer := &config.Peers[peerIdx]

			switch key {
			case "publickey":
				peer.PublicKey = value
			case "presharedkey":
				peer.PresharedKey = value
			case "endpoint":
				peer.Endpoint = value
			case "allowedips":
				ips := strings.Split(value, ",")
				for _, ip := range ips {
					peer.AllowedIPs = append(peer.AllowedIPs, strings.TrimSpace(ip))
				}
			case "persistentkeepalive":
				if ka, err := strconv.Atoi(value); err == nil {
					peer.PersistentKeepalive = ka
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	// Validation
	if config.PrivateKey == "" {
		return nil, fmt.Errorf("interface private key is required")
	}

	return config, nil
}

// ConvertToFlywallConfig converts a parsed WireGuardConfig to a Flywall WireGuardConfig.
// It mainly handles type conversion and defaults.
func ConvertToFlywallConfig(wg *WireGuardConfig, name string, iface string) *WireGuardConfig {
	// The parsed config is already *vpn.WireGuardConfig (which maps to config.WireGuardConfig keys mostly),
	// but we need to ensure it matches the internal/config types if they differ.
	// Actually, internal/vpn/wireguard.go defines `WireGuardConfig` which mirrors `config.WireGuardConfig`.
	// Let's verify imports. This file is in `package vpn`.
	// The types in `internal/vpn/wireguard.go` are:
	// type WireGuardConfig struct { ... }
	// type WireGuardPeer struct { ... } (Note: config/security.go has WireGuardPeerConfig)

	// Wait, `internal/vpn/wireguard.go` defines `WireGuardConfig` locally?
	// Yes, line 22 of existing `wireguard.go`.

	wg.Name = name
	wg.Interface = iface
	if wg.MTU == 0 {
		wg.MTU = 1420
	}

	return wg
}
