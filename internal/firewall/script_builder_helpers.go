// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"grimm.is/flywall/internal/config"
)

var identifierRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// RFC1918 private address ranges (should not appear on WAN as source)
var protectionPrivateNetworks = []*net.IPNet{
	mustParseCIDR("10.0.0.0/8"),
	mustParseCIDR("172.16.0.0/12"),
	mustParseCIDR("192.168.0.0/16"),
}

// Bogon ranges - reserved/invalid addresses
var protectionBogonNetworks = []*net.IPNet{
	mustParseCIDR("0.0.0.0/8"),       // reserved
	mustParseCIDR("127.0.0.0/8"),     // loopback
	mustParseCIDR("169.254.0.0/16"),  // link-local
	mustParseCIDR("192.0.0.0/24"),    // IETF protocol
	mustParseCIDR("192.0.2.0/24"),    // TEST-NET-1
	mustParseCIDR("198.51.100.0/24"), // TEST-NET-2
	mustParseCIDR("203.0.113.0/24"),  // TEST-NET-3
	mustParseCIDR("224.0.0.0/4"),     // multicast
	mustParseCIDR("240.0.0.0/4"),     // reserved
}

func isValidIdentifier(s string) bool {
	return identifierRegex.MatchString(s)
}

func quote(s string) string {
	if isValidIdentifier(s) {
		return s
	}
	return fmt.Sprintf("%q", s)
}

// forceQuote always quotes a string - needed for interface names in concatenation sets
// where nftables requires quoted identifiers even for valid names.
func forceQuote(s string) string {
	return fmt.Sprintf("%q", s)
}

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// isRFC1918Network checks if a CIDR is in RFC1918 private ranges
func isRFC1918Network(cidr string) bool {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		// Try parsing as plain IP
		ip := net.ParseIP(cidr)
		if ip == nil {
			return false
		}
		for _, rfc1918 := range protectionPrivateNetworks {
			if rfc1918.Contains(ip) {
				return true
			}
		}
		return false
	}

	// Check if the network's first IP falls within RFC1918 ranges
	for _, rfc1918 := range protectionPrivateNetworks {
		if rfc1918.Contains(network.IP) {
			return true
		}
	}
	return false
}

// findZone finds a zone by name in the zones list
func findZone(zones []config.Zone, name string) *config.Zone {
	for i := range zones {
		if zones[i].Name == name {
			return &zones[i]
		}
	}
	return nil
}

// isZoneInternal returns true if the zone contains RFC1918 (internal) networks
func isZoneInternal(zone *config.Zone, zoneIfaces []string) bool {
	// Check zone's explicitly defined networks
	for _, network := range zone.Networks {
		if isRFC1918Network(network) {
			return true
		}
	}
	// Zones with only interfaces are typically internal (LAN)
	// But we can't determine this without checking interface IPs
	// Default: if zone has interfaces but no networks, assume internal
	if len(zoneIfaces) > 0 && len(zone.Networks) == 0 {
		return true
	}
	return false
}

// isZoneExternal returns true if the zone is external (WAN) based on heuristics
func isZoneExternal(zone *config.Zone, zoneIfaces []string, interfaces []config.Interface) bool {
	// 1. Explicit setting
	if zone.External != nil {
		return *zone.External
	}

	// 2. Zone name contains wan/external
	lowerName := strings.ToLower(zone.Name)
	if strings.Contains(lowerName, "wan") || strings.Contains(lowerName, "external") {
		return true
	}

	// 3. Interface has DHCP client enabled (getting address from upstream)
	for _, ifName := range zoneIfaces {
		for _, iface := range interfaces {
			if iface.Name == ifName && iface.DHCP {
				return true
			}
		}
	}

	return false
}

// buildZoneMapForScript builds a zone-to-interfaces map.
// Uses ZoneResolver to handle new match-based zone format and backwards compat.
func buildZoneMapForScript(cfg *Config) map[string][]string {
	zoneMap := make(map[string][]string)

	// Use ZoneResolver for new-style zone definitions
	resolver := config.NewZoneResolver(cfg.Zones)

	// 1. Process zones using ZoneResolver
	for _, zone := range cfg.Zones {
		cName := canonicalZoneName(zone.Name)
		ifaces := resolver.GetZoneInterfaces(zone.Name)
		if len(ifaces) > 0 {
			zoneMap[cName] = ifaces
		} else {
			zoneMap[cName] = []string{}
		}
	}

	// 2. Add interfaces that reference a Zone (interface-level zone assignment)
	// This is the legacy way: interface "eth0" { zone = "WAN" }
	for _, iface := range cfg.Interfaces {
		if iface.Zone != "" {
			cZone := canonicalZoneName(iface.Zone)
			exists := false
			currentList := zoneMap[cZone]
			for _, existing := range currentList {
				if existing == iface.Name {
					exists = true
					break
				}
			}
			if !exists {
				zoneMap[cZone] = append(zoneMap[cZone], iface.Name)
			}
		} else {
			// Implicit Zone: Interface describes its own zone
			cName := canonicalZoneName(iface.Name)
			zoneMap[cName] = []string{iface.Name}
		}
	}

	return zoneMap
}

// resolveUplinkToMark searches VPN config for an uplink by interface name and returns its routing mark.
func resolveUplinkToMark(cfg *config.Config, uplinkName string) uint32 {
	if cfg == nil {
		return 0
	}

	// Search VPN config for WireGuard tunnels matching the name
	if cfg.VPN != nil {
		for i, wg := range cfg.VPN.WireGuard {
			if wg.Interface == uplinkName || wg.Name == uplinkName {
				return uint32(0x0200 + i) // MarkWireGuardBase + index
			}
		}
		for i, ts := range cfg.VPN.Tailscale {
			iface := ts.Interface
			if iface == "" {
				iface = "tailscale0"
			}
			if iface == uplinkName {
				return uint32(0x0220 + i) // MarkTailscaleBase + index
			}
		}
	}

	// Search Interfaces for matching interface with a Table > 0 (Multi-WAN style)
	for i, iface := range cfg.Interfaces {
		if iface.Name == uplinkName && iface.Table > 0 {
			return uint32(0x0100 + i) // MarkWANBase + index
		}
	}

	return 0
}

// resolveZoneInterfaces resolves a zone name or wildcard to a list of interface names.
func resolveZoneInterfaces(pattern string, zoneMap map[string][]string) []string {
	if interfaces, ok := zoneMap[pattern]; ok {
		return interfaces
	}

	// Handle wildcard matching
	var results []string
	for zoneName, interfaces := range zoneMap {
		if matchZoneWildcard(pattern, zoneName) {
			results = append(results, interfaces...)
		}
	}
	return results
}

// matchZoneWildcard checks if a zone pattern matches a zone name.
func matchZoneWildcard(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(name) >= len(prefix) && name[:len(prefix)] == prefix
	}
	return pattern == name
}
