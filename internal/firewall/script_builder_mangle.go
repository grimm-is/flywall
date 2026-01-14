package firewall

import (
	"fmt"
)

// BuildMangleTableScript builds the mangle table script (management routing).
func BuildMangleTableScript(cfg *Config, tableName string) (*ScriptBuilder, error) {
	// Only build mangle table if we have interfaces with Table > 0 (Multi-WAN / Policy Routing)
	// OR if we have VPN configs (which imply potential uplink routing needs)
	// OR if we have specific policies that need connection options (e.g. MSS clamping sometimes uses mangle)
	//
	// Actually, we use mangle for restoring connection marks on input to properly route return traffic
	// back out the same interface it came in on (when using multiple routing tables).
	needsMangle := false

	// Check interfaces
	if cfg.Interfaces != nil {
		for _, iface := range cfg.Interfaces {
			if iface.Table > 0 {
				needsMangle = true
				break
			}
		}
	}

	// Check VPNs (WireGuard/Tailscale often need marking for return path)
	if !needsMangle && cfg.VPN != nil {
		if len(cfg.VPN.WireGuard) > 0 || len(cfg.VPN.Tailscale) > 0 {
			needsMangle = true
		}
	}

	if len(cfg.MarkRules) > 0 {
		needsMangle = true
	}

	if !needsMangle {
		return nil, nil
	}
	timezone := "UTC"
	if cfg.System != nil && cfg.System.Timezone != "" {
		timezone = cfg.System.Timezone
	}
	sb := NewScriptBuilder(tableName, "ip", timezone)
	sb.AddTable()

	// Chains
	sb.AddChain("prerouting", "filter", "prerouting", -150, "accept")
	sb.AddChain("output", "filter", "output", -150, "accept")
	// input/forward/postrouting usually not needed for basic connmark restore

	// 1. Restore Connection Mark to Packet Mark (Input/Prerouting)
	// This ensures that packets belonging to an established stream carry the mark
	// initially set on the first packet (e.g. by the inbound interface rule below).
	sb.AddRule("prerouting", "ct state established,related meta mark set ct mark", "[routing] Restore mark")
	sb.AddRule("output", "ct state established,related meta mark set ct mark", "[routing] Restore mark")

	// 2. Mark New Incoming Connections (based on ingress interface)
	// If a packet comes in on a WAN interface (with a specific routing table), extract/set a mark.
	// We map Interface -> Mark.
	// Mark Scheme:
	// 0x01XX : Physical Interfaces (e.g. eth0 -> 0x0100, eth1 -> 0x0101)
	// 0x02XX : VPN Interfaces
	// 0x00XX : Local/System (default)
	if cfg.Interfaces != nil {
		for i, iface := range cfg.Interfaces {
			if iface.Table > 0 {
				mark := 0x0100 + i
				// Match new/related incoming packets on this interface
				// "ct state new" isn't enough? "ct mark set" persists to conntrack entry.
				// "meta mark set" sets on current packet (used for routing this packet).
				// We need BOTH: set packet mark (for routing reply if local dest? no, prerouting is for forwarding too)
				// AND save to ct mark.
				sb.AddRule("prerouting", fmt.Sprintf(
					"iifname \"%s\" meta mark set 0x%x ct mark set 0x%x",
					iface.Name, mark, mark), fmt.Sprintf("[routing] Mark Ingress %s", iface.Name))
			}
		}
	}

	// 3. VPN Marking (if needed for policy routing override)
	if cfg.VPN != nil {
		for i, wg := range cfg.VPN.WireGuard {
			// If WG interface participates in multi-wan/policy routing
			mark := 0x0200 + i
			ifaceName := wg.Interface
			if ifaceName == "" {
				ifaceName = "wg0"
			}
			// Assuming we want to track connections on WG too?
			sb.AddRule("prerouting", fmt.Sprintf(
				"iifname \"%s\" meta mark set 0x%x ct mark set 0x%x",
				ifaceName, mark, mark), fmt.Sprintf("[routing] Mark VPN %s", ifaceName))
		}
		// Tailscale?
		for i, ts := range cfg.VPN.Tailscale {
			mark := 0x0220 + i
			ifaceName := ts.Interface
			if ifaceName == "" {
				ifaceName = "tailscale0"
			}
			sb.AddRule("prerouting", fmt.Sprintf(
				"iifname \"%s\" meta mark set 0x%x ct mark set 0x%x",
				ifaceName, mark, mark), fmt.Sprintf("[routing] Mark VPN %s", ifaceName))
		}
	}

	// 4. Custom Mark Rules
	for _, rule := range cfg.MarkRules {
		if !rule.Enabled {
			continue
		}

		// Build match expression
		match := ""
		if rule.SrcIP != "" {
			match += fmt.Sprintf("ip saddr %s ", rule.SrcIP)
		}
		if rule.DstIP != "" {
			match += fmt.Sprintf("ip daddr %s ", rule.DstIP)
		}
		if rule.Protocol != "" && rule.Protocol != "any" {
			match += fmt.Sprintf("meta l4proto %s ", rule.Protocol)
		}
		if rule.SrcPort > 0 {
			match += fmt.Sprintf("sport %d ", rule.SrcPort)
		}
		if rule.DstPort > 0 {
			match += fmt.Sprintf("dport %d ", rule.DstPort)
		}
		if rule.InInterface != "" {
			match += fmt.Sprintf("iifname %s ", forceQuote(rule.InInterface))
		}

		// Build action
		action := fmt.Sprintf("meta mark set %s", rule.Mark)
		if rule.SaveMark {
			action += fmt.Sprintf(" ct mark set %s", rule.Mark)
		}

		// Add rule to Prerouting (and Output if src is local? usually explicit rules imply inbound or routed)
		// Default to Prerouting for now
		if match != "" || action != "" {
			sb.AddRule("prerouting", fmt.Sprintf("%s%s", match, action), rule.Name)
		}
	}

	return sb, nil
}
