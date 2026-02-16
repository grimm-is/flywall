// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/config"
)

// canonicalZoneName normalizes zone name aliases to their canonical form.
// "firewall", "router", and "self" all refer to the router itself.
func canonicalZoneName(name string) string {
	lower := strings.ToLower(name)
	switch lower {
	case "firewall", "router", "self", brand.LowerName:
		return brand.LowerName
	default:
		return lower
	}
}

// BuildFilterTableScript builds the main filter table script from config.
// This is the core logic for firewall rule generation. It constructs the "inet"
// table which handles IPv4 and IPv6 filtering.
//
// Key components generated:
//  1. Base Chains: input, forward, output with default drop policies.
//  2. Stats Chain: A dedicated chain for counting packet types (SYN, UDP, etc.).
//  3. Service Rules: Allow rules for local services (SSH, DNS, DHCP) based on zone config.
//  4. Policy Chains: Per-policy chains (e.g. "policy_lan_wan") containing user rules.
//  5. Verdict Maps: O(1) dispatch using vmaps to jump to the correct policy chain
//     based on input/output interfaces.
//  6. Protection: Anti-spoofing and invalid packet drops.
//  7. NAT/Masquerade: Handled in separate NAT table, but referenced here for context.
func BuildFilterTableScript(
	cfg *Config,
	vpn *config.VPNConfig,
	tableName string,
	configHash string,
	resolvedIPSets map[string][]string,
) (*ScriptBuilder, error) {
	timezone := "UTC"
	if cfg.System != nil && cfg.System.Timezone != "" {
		timezone = cfg.System.Timezone
	}
	sb := NewScriptBuilder(tableName, "inet", timezone)

	// Create table with metadata comment
	applyCount := GetNextApplyCount(tableName, "inet")
	comment := BuildMetadataComment(applyCount, configHash)
	sb.AddTableWithComment(comment)

	// Flowtables (Performance Optimization)
	// Hardware/Software Offload for established connections.
	if cfg.EnableFlowOffload {
		var flowDevices []string
		for _, iface := range cfg.Interfaces {
			// Only add interfaces that exist and are managed.
			// Ideally we verify they support offload, but we can just try adding them.
			flowDevices = append(flowDevices, iface.Name)
		}
		if len(flowDevices) > 0 {
			sb.AddFlowtable("ft", flowDevices)
		}
	}

	// Add Protection Chain (Raw Prerouting)
	addProtectionRules(cfg, sb)

	// Define GeoIP sets if referenced by any policy rule
	// We scan all policies to find used identifiers like "US", "CN", etc.
	usedCountries := make(map[string]bool)
	for _, pol := range cfg.Policies {
		if pol.Disabled {
			continue
		}
		for _, rule := range pol.Rules {
			if rule.Disabled {
				continue
			}
			if rule.SourceCountry != "" {
				usedCountries[strings.ToUpper(rule.SourceCountry)] = true
			}
			if rule.DestCountry != "" {
				usedCountries[strings.ToUpper(rule.DestCountry)] = true
			}
		}
	}

	for code := range usedCountries {
		if len(code) != 2 {
			return nil, fmt.Errorf("invalid country code: %s", code)
		}
		setName := fmt.Sprintf("geoip_country_%s", code)
		// Add set definition: type ipv4_addr; flags interval;
		// note: Currently only supporting IPv4 for GeoIP as per BuildRuleExpression
		sb.AddSet(setName, "ipv4_addr", fmt.Sprintf("[geoip] %s", code), 0, "interval")
	}

	// CRITICAL: Define IPSets BEFORE rules that reference them
	for _, ipset := range cfg.IPSets {
		setType := ipset.Type
		if setType == "" {
			setType = "ipv4_addr"
		}

		isDynamic := setType == "dns" || setType == "dynamic"

		// Infer interval flag: needed if entries contain CIDR notation
		var flags []string
		for _, entry := range ipset.Entries {
			if strings.Contains(entry, "/") || strings.Contains(entry, "-") {
				flags = append(flags, "interval")
				break
			}
		}

		// Handle DNS type sets
		size := ipset.Size
		if setType == "dns" {
			setType = "ipv4_addr"
			// Default size for Dynamic sets if not specified (optimization)
			if size == 0 {
				size = 65535
			}
		}

		if !isValidIdentifier(ipset.Name) {
			return nil, fmt.Errorf("invalid ipset name: %s", ipset.Name)
		}
		sb.AddSet(ipset.Name, setType, fmt.Sprintf("[ipset:%s]", ipset.Name), size, flags...)

		// Smart Flush: Flush sets to ensure they match config.
		// For resolved sets, we always flush because we rebuild the content.
		// For purely dynamic sets (without resolved content), we might
		// preserve them if needed, but atomic apply usually rebuilds.
		// Given we have resolutions, we can flush safely.
		if !isDynamic {
			sb.AddLine(fmt.Sprintf("flush set %s %s %s", sb.family, sb.tableName, quote(ipset.Name)))
		}

		// Add elements: Prefer resolved download results, fallback to static if not in map
		// (though resolved map includes static if call was successful)
		var elements []string
		if resolved, ok := resolvedIPSets[ipset.Name]; ok {
			elements = resolved
		} else {
			elements = ipset.Entries
		}

		if len(elements) > 0 {
			sb.AddSetElements(ipset.Name, elements)
		}
	}

	// Define DNS Egress Control Sets (Dynamic, Persistent)
	// We handle v4 and v6 separately because nftables sets are typed.
	if cfg.DNS != nil && cfg.DNS.EgressFilter {
		// size 65535, timeout 3600 (default fallback, individual elements override)
		// We add "timeout" flag to support element timeouts.
		sb.AddSet("dns_allowed_v4", "ipv4_addr", "[dns-wall] Allowed IPv4", 65535, "timeout")
		sb.AddSet("dns_allowed_v6", "ipv6_addr", "[dns-wall] Allowed IPv6", 65535, "timeout")
		// Do NOT flush these sets. They are persistent.
	}

	// Define blocked_ips set for fail2ban-style IP blocking
	// This set is managed dynamically via RPC from the API server
	sb.AddSet("blocked_ips", "ipv4_addr", "[ipset:blocked_ips]", 0)

	// Create base chains with default drop policy
	sb.AddChain("input", "filter", "input", 0, "drop", "[base] Incoming traffic")
	sb.AddChain("forward", "filter", "forward", 0, "drop", "[base] Routed traffic")
	sb.AddChain("output", "filter", "output", 0, "drop", "[base] Outgoing traffic")

	// Create mangle chain for UplinkManager policy routing marks
	// Priority -150 (mangle) ensures marks are set before routing decision
	sb.AddChain("mark_prerouting", "filter", "prerouting", -150, "accept", "[base] Policy routing marks")

	// ========================================
	// Stats Chain - Named counters for anomaly detection
	// Tracks SYN/RST/FIN/UDP/ICMP for flood and scan detection
	// ========================================

	// Define named counters (queryable via nft list counters)
	sb.AddCounter("cnt_syn", "[stats] TCP SYN packets")
	sb.AddCounter("cnt_rst", "[stats] TCP RST packets")
	sb.AddCounter("cnt_fin", "[stats] TCP FIN packets")
	sb.AddCounter("cnt_udp", "[stats] UDP packets")
	sb.AddCounter("cnt_icmp", "[stats] ICMP packets")

	// Create stats chain (regular chain, no hook - called via jump)
	sb.AddChain("flywall_stats", "", "", 0, "", "[stats] Packet counters")

	// Add counter rules to stats chain
	// SYN: tcp flags & (syn|ack) == syn (new connection attempts)
	sb.AddRule("flywall_stats", "tcp flags & (syn|ack) == syn counter name cnt_syn", "[stats] Count SYN")
	// RST: connection reset/rejection
	sb.AddRule("flywall_stats", "tcp flags & rst == rst counter name cnt_rst", "[stats] Count RST")
	// FIN: connection closure
	sb.AddRule("flywall_stats", "tcp flags & fin == fin counter name cnt_fin", "[stats] Count FIN")
	// UDP: all UDP traffic
	sb.AddRule("flywall_stats", "meta l4proto udp counter name cnt_udp", "[stats] Count UDP")
	// ICMP: all ICMP traffic
	sb.AddRule("flywall_stats", "meta l4proto { icmp, icmpv6 } counter name cnt_icmp", "[stats] Count ICMP")

	// Add base rules (loopback, established/related)
	sb.AddRule("input", "iifname \"lo\" accept", "[base] Loopback")
	sb.AddRule("output", "oifname \"lo\" accept", "[base] Loopback")

	// Jump to stats chain early to count ALL packets (before accept shortcuts)
	sb.AddRule("input", "jump flywall_stats", "[stats] Collect metrics")
	sb.AddRule("forward", "jump flywall_stats", "[stats] Collect metrics")

	// Drop traffic from blocked IPs (fail2ban-style blocking)
	// This runs early to block malicious IPs before any accept rules
	sb.AddRule("input", "ip saddr @blocked_ips drop", "[security] Blocked IPs")
	sb.AddRule("forward", "ip saddr @blocked_ips drop", "[security] Blocked IPs")

	sb.AddRule("input", "ct state established,related accept", "[base] Stateful")
	sb.AddRule("forward", "ct state established,related accept", "[base] Stateful")
	sb.AddRule("output", "ct state established,related accept", "[base] Stateful")

	// Drop invalid packets (malformed, out-of-window, or spoofed)
	// Rate-limit logging to prevent log spam
	sb.AddRule("input", `ct state invalid limit rate 10/minute log group 0 prefix "DROP_INVALID: " counter drop`, "[base] Invalid drop")
	sb.AddRule("forward", `ct state invalid limit rate 10/minute log group 0 prefix "DROP_INVALID: " counter drop`, "[base] Invalid drop")
	sb.AddRule("output", "ct state invalid drop", "[base] Invalid drop")

	// Device Discovery Logging (using configured log group)
	logGroup := 100 // Default
	if cfg.RuleLearning != nil {
		logGroup = cfg.RuleLearning.LogGroup
	}

	// Log NEW connections early in the chain for device tracking.
	// Rate limited to prevent flooding the collector on busy networks.
	// This happens BEFORE accept/drop decisions so we see all devices.
	sb.AddRule("input", fmt.Sprintf("ct state new limit rate 100/second burst 50 packets log group %d prefix \"DISCOVER: \"", logGroup), "[feature] Device discovery")
	sb.AddRule("forward", fmt.Sprintf("ct state new limit rate 100/second burst 50 packets log group %d prefix \"DISCOVER: \"", logGroup), "[feature] Device discovery")

	// Explicitly log mDNS for device discovery (multicast often bypasses state checking)
	sb.AddRule("input", fmt.Sprintf("udp dport 5353 limit rate 100/second burst 50 packets log group %d prefix \"DISCOVER_MDNS: \"", logGroup), "[feature] mDNS discovery")

	// HA and Replication Rules
	if cfg.Replication != nil {
		// Replication State Sync (TCP)
		// Parse port from ListenAddr (default 9001 if parsing fails)
		repPort := 9001
		if parts := strings.Split(cfg.Replication.ListenAddr, ":"); len(parts) == 2 {
			if p, err := strconv.Atoi(parts[1]); err == nil {
				repPort = p
			}
		}
		sb.AddRule("input", fmt.Sprintf("tcp dport %d accept", repPort), "[ha] Replication state sync input")
		sb.AddRule("output", fmt.Sprintf("tcp dport %d accept", repPort), "[ha] Replication state sync output")

		// HA Heartbeat and Conntrack Sync
		if cfg.Replication.HA != nil && cfg.Replication.HA.Enabled {
			// Heartbeat
			hbPort := 9002
			if cfg.Replication.HA.HeartbeatPort > 0 {
				hbPort = cfg.Replication.HA.HeartbeatPort
			}
			sb.AddRule("input", fmt.Sprintf("udp dport %d accept", hbPort), "[ha] Heartbeat input")
			sb.AddRule("output", fmt.Sprintf("udp dport %d accept", hbPort), "[ha] Heartbeat output")

			// Conntrack Sync
			if cfg.Replication.HA.ConntrackSync != nil && cfg.Replication.HA.ConntrackSync.Enabled {
				ctPort := 3780
				if cfg.Replication.HA.ConntrackSync.Port > 0 {
					ctPort = cfg.Replication.HA.ConntrackSync.Port
				}
				sb.AddRule("input", fmt.Sprintf("udp dport %d accept", ctPort), "[ha] Conntrack sync input")
				sb.AddRule("output", fmt.Sprintf("udp dport %d accept", ctPort), "[ha] Conntrack sync output")
			}
		}
	}

	// Add essential service rules (DHCP, DNS for router itself)
	// DHCP Client (WAN) and Server (LAN) - Must be before DROP_INVALID
	sb.AddRule("input", "udp dport 67-68 accept", "[svc:dhcp] DHCP server/client")
	sb.AddRule("output", "udp dport 67-68 accept", "[svc:dhcp] DHCP client")

	// VPN Lockout Protection Rules (ManagementAccess = true)
	// These rules ensure VPN traffic is ALWAYS accepted, even if other rules fail
	// Added BEFORE invalid packet drops and other rules
	if vpn != nil {
		// Tailscale lockout protection
		for _, ts := range vpn.Tailscale {
			if ts.ManagementAccess {
				iface := "tailscale0"
				if ts.Interface != "" {
					iface = ts.Interface
				}
				sb.AddRule("input", fmt.Sprintf("iifname %q accept comment \"tailscale-lockout-protection\"", iface))
				sb.AddRule("output", fmt.Sprintf("oifname %q accept comment \"tailscale-lockout-protection\"", iface))
				sb.AddRule("forward", fmt.Sprintf("iifname %q accept comment \"tailscale-lockout-protection\"", iface))
				sb.AddRule("forward", fmt.Sprintf("oifname %q accept comment \"tailscale-lockout-protection\"", iface))
			}
		}

		// WireGuard lockout protection
		for _, wg := range vpn.WireGuard {
			if wg.Enabled && wg.ManagementAccess {
				iface := wg.Interface
				if iface == "" {
					iface = "wg0"
				}
				sb.AddRule("input", fmt.Sprintf("iifname %q accept comment \"wireguard-lockout-protection\"", iface))
				sb.AddRule("output", fmt.Sprintf("oifname %q accept comment \"wireguard-lockout-protection\"", iface))
				sb.AddRule("forward", fmt.Sprintf("iifname %q accept comment \"wireguard-lockout-protection\"", iface))
				sb.AddRule("forward", fmt.Sprintf("oifname %q accept comment \"wireguard-lockout-protection\"", iface))
			}
		}
	}

	// MSS Clamping to PMTU (Forward Chain)
	// Keeps TCP connections healthy across links with different MTUs (e.g. PPPoE, VPNs)
	if cfg.MSSClamping {
		sb.AddRule("forward", "tcp flags syn tcp option maxseg size set rt mtu", "[feature] MSS clamping")
	}

	// Flowtable Offload Rule (Forward Chain)
	// Bypasses the rest of the ruleset for established connections in the flowtable.
	if cfg.EnableFlowOffload {
		sb.AddRule("forward", "ip protocol { tcp, udp } flow add @ft", "[feature] Flow offload")
	}

	// Add ICMP accept rules
	sb.AddRule("input", "meta l4proto icmp accept", "[base] ICMP")
	sb.AddRule("input", "meta l4proto icmpv6 accept", "[base] ICMPv6")
	// IPv6 Neighbor Discovery (Vital for IPv6 connectivity)
	sb.AddRule("input", "icmpv6 type { nd-neighbor-solicit, nd-neighbor-advert, nd-router-solicit, nd-router-advert } accept", "[base] IPv6 ND")
	sb.AddRule("output", "meta l4proto icmp accept", "[base] ICMP")
	sb.AddRule("output", "meta l4proto icmpv6 accept", "[base] ICMPv6")
	sb.AddRule("output", "icmpv6 type { nd-neighbor-solicit, nd-neighbor-advert, nd-router-solicit, nd-router-advert } accept", "[base] IPv6 ND")

	// mDNS Reflector Rules (Must be before generic drops)
	if cfg.MDNS != nil && cfg.MDNS.Enabled && len(cfg.MDNS.Interfaces) > 0 {
		var mdnsIfaces []string
		for _, iface := range cfg.MDNS.Interfaces {
			mdnsIfaces = append(mdnsIfaces, forceQuote(iface))
		}
		ifacesStr := strings.Join(mdnsIfaces, ", ")

		// Allow INPUT multicast (query/response) on enabled interfaces
		sb.AddRule("input", fmt.Sprintf("iifname { %s } ip daddr 224.0.0.251 udp dport 5353 accept", ifacesStr), "[svc:mdns] Reflector input")
		sb.AddRule("input", fmt.Sprintf("iifname { %s } ip6 daddr ff02::fb udp dport 5353 accept", ifacesStr), "[svc:mdns] Reflector input v6")

		// Allow OUTPUT multicast (reflector sending)
		sb.AddRule("output", fmt.Sprintf("oifname { %s } ip daddr 224.0.0.251 udp dport 5353 accept", ifacesStr), "[svc:mdns] Reflector output")
		sb.AddRule("output", fmt.Sprintf("oifname { %s } ip6 daddr ff02::fb udp dport 5353 accept", ifacesStr), "[svc:mdns] Reflector output v6")
	}

	// NTP Service Rules
	if cfg.NTP != nil && cfg.NTP.Enabled {
		// Allow OUTPUT (Client syncing from upstream)
		sb.AddRule("output", "udp dport 123 accept", "[svc:ntp] Client sync")

		// Allow INPUT on Internal Zones (LAN Clients)
		zoneMap := buildZoneMapForScript(cfg)
		for _, zone := range cfg.Zones {
			ifaces := resolveZoneInterfaces(zone.Name, zoneMap)
			if isZoneInternal(&zone, ifaces) {
				for _, ifname := range ifaces {
					sb.AddRule("input", fmt.Sprintf("iifname %s udp dport 123 accept", forceQuote(ifname)), fmt.Sprintf("[svc:ntp] zone:%s", zone.Name))
				}
			}
		}
	}

	// UPnP Service Rules
	if cfg.UPnP != nil && cfg.UPnP.Enabled {
		if len(cfg.UPnP.InternalIntfs) > 0 {
			var upnpIfaces []string
			for _, iface := range cfg.UPnP.InternalIntfs {
				upnpIfaces = append(upnpIfaces, forceQuote(iface))
			}
			ifacesStr := strings.Join(upnpIfaces, ", ")

			// Allow SSDP (UDP 1900) on internal interfaces
			// Standard multicast address 239.255.255.250, but we allow port generic for simplicity
			sb.AddRule("input", fmt.Sprintf("iifname { %s } udp dport 1900 accept", ifacesStr), "[svc:upnp] SSDP discovery")
		}
	}

	// Drop mDNS noise on VPN interfaces (unless explicitly enabled above?)
	sb.AddRule("input", "iifname \"wg*\" udp dport 5353 limit rate 5/minute log group 0 prefix \"DROP_MDNS: \" counter drop", "[vpn] mDNS noise drop")
	sb.AddRule("input", "iifname \"tun*\" udp dport 5353 limit rate 5/minute log group 0 prefix \"DROP_MDNS: \" counter drop", "[vpn] mDNS noise drop")

	// Network Learning Rules
	// Log TLS traffic for SNI inspection.
	sb.AddRule("forward", fmt.Sprintf(`tcp dport 443 ct state established limit rate 50/second burst 20 packets log group %d prefix "TLS_SNI: "`, logGroup), "[feature] TLS SNI learning")

	// VPN Transport Rules (WireGuard)
	if vpn != nil {
		for _, wg := range vpn.WireGuard {
			if wg.Enabled {
				// Allow incoming handshake/data on ListenPort
				if wg.ListenPort > 0 {
					sb.AddRule("input", fmt.Sprintf("udp dport %d accept", wg.ListenPort), fmt.Sprintf("[vpn:wg] %s listen", wg.Interface))
				}
				// Allow outgoing handshake/data to Peers
				for _, peer := range wg.Peers {
					parts := strings.Split(peer.Endpoint, ":")
					if len(parts) == 2 {
						port := parts[1]
						sb.AddRule("output", fmt.Sprintf("udp dport %s accept", port), fmt.Sprintf("[vpn:wg] peer %s", peer.Endpoint))
					}
				}
			}
		}
	}

	// DNS and API Access (Zone-Aware / Interface-Aware)

	// Global Output enabled for DNS (router resolving)
	sb.AddRule("output", "udp dport 53 accept", "[svc:dns] Router DNS client")
	sb.AddRule("output", "tcp dport 53 accept", "[svc:dns] Router DNS client")

	// Allow DNS/API/SSH/etc based on Zone and Interface config

	// Web Access Control (New Config)
	useNewWebRules := generateWebAccessRules(cfg, sb)

	// DNS Egress Control (DNS Wall) - Forward Chain
	// Block forwarding to IPs not in the allowed sets.
	// Only affects Forward chain (LAN Clients). Router Output is unrestricted.
	if cfg.DNS != nil && cfg.DNS.EgressFilter {
		// Allow if in set.
		// We rely on "return" or "accept"?
		// If we use "return", we continue to other checks (e.g. invalid drop, logging).
		// But this is a positive security model: "Allow only if resolved".
		// Actually, we usually want to BLOCK if NOT in set.
		sb.AddRule("forward", "ip daddr != @dns_allowed_v4 ct state new reject with icmp type admin-prohibited", "[dns-wall] Block unknown IPv4")
		sb.AddRule("forward", "ip6 daddr != @dns_allowed_v6 ct state new reject with icmpv6 type no-route", "[dns-wall] Block unknown IPv6")
	}

	// Consolidate services into TCP and UDP sets for concatenation
	// Format: "iifname . port"
	var tcpElements []string
	var udpElements []string
	var icmpElements []string

	// Helper to add a service for an interface
	addService := func(ifaceName, serviceName string) {
		svc, ok := BuiltinServices[serviceName]
		if !ok {
			return // Should not happen for known builtins
		}

		// Handle TCP
		if svc.Protocol&ProtoTCP != 0 {
			if len(svc.Ports) > 0 {
				for _, p := range svc.Ports {
					tcpElements = append(tcpElements, fmt.Sprintf("%s . %d", ifaceName, p))
				}
			} else if svc.Port > 0 {
				tcpElements = append(tcpElements, fmt.Sprintf("%s . %d", ifaceName, svc.Port))
			}
		}

		// Handle UDP
		if svc.Protocol&ProtoUDP != 0 {
			if len(svc.Ports) > 0 {
				for _, p := range svc.Ports {
					udpElements = append(udpElements, fmt.Sprintf("%s . %d", ifaceName, p))
				}
			} else if svc.Port > 0 {
				udpElements = append(udpElements, fmt.Sprintf("%s . %d", ifaceName, svc.Port))
			}
		}

		// Handle ICMP (Special Case)
		if svc.Protocol&ProtoICMP != 0 {
			icmpElements = append(icmpElements, ifaceName)
		}
	}

	for _, iface := range cfg.Interfaces {
		// Consolidate logic for this interface
		allowSSH := false
		allowWeb := false
		allowAPI := false
		allowICMP := false
		allowSNMP := false
		allowSyslog := false

		// Logic for checking if Management block is present or using legacy
		// Assuming we parsed this correctly.
		// Note: The logic in original script_builder.go checked both structure and legacy fields.
		// We simplified here slightly but should respect original if possible.
		if iface.Management != nil { // Struct pointer check
			// Management override logic
			allowSSH = iface.Management.SSH
			allowWeb = iface.Management.Web || iface.Management.WebUI
			allowAPI = iface.Management.API
			allowICMP = iface.Management.ICMP
			allowSNMP = iface.Management.SNMP
			allowSyslog = iface.Management.Syslog
		} else {
			// Legacy Interface-level fallback
			if iface.AccessWebUI {
				allowWeb = true
				allowAPI = true
			}
		}

		// Quote interface name for nftables
		qIface := forceQuote(iface.Name)

		// Collect services
		if allowSSH {
			addService(qIface, "ssh")
		}
		if allowICMP {
			addService(qIface, "icmp")
		}
		if allowSNMP {
			addService(qIface, "snmp")
		}
		if allowSyslog {
			addService(qIface, "syslog")
		}

		if !useNewWebRules && (allowWeb || allowAPI) {
			addService(qIface, "http")
			addService(qIface, "https")

			if allowAPI || allowWeb {
				// Explicitly allow default API ports 8443 and 8080 (WebUI/API)
				tcpElements = append(tcpElements, fmt.Sprintf("%s . %d", qIface, 8443))
				tcpElements = append(tcpElements, fmt.Sprintf("%s . %d", qIface, 8080))
			}

			// Custom port if legacy field used
			if iface.WebUIPort > 0 && iface.WebUIPort != 80 && iface.WebUIPort != 443 && iface.WebUIPort != 8080 && iface.WebUIPort != 8443 {
				tcpElements = append(tcpElements, fmt.Sprintf("%s . %d", qIface, iface.WebUIPort))
			}
		}
	}

	// Process zones directly (for zones using match blocks instead of interface.zone)
	// This ensures zones with management/services blocks are processed even without explicit interface config
	zoneMap := buildZoneMapForScript(cfg)
	for _, zone := range cfg.Zones {
		if zone.Management == nil && zone.Services == nil {
			continue // No rules to generate
		}

		// Get interfaces for this zone
		cName := canonicalZoneName(zone.Name)
		zoneIfaces, ok := zoneMap[cName]
		if !ok || len(zoneIfaces) == 0 {
			continue
		}

		for _, ifaceName := range zoneIfaces {
			qIface := forceQuote(ifaceName)

			// Zone Services
			if zone.Services != nil {
				if zone.Services.DNS {
					addService(qIface, "dns")
				}
				if zone.Services.NTP {
					addService(qIface, "ntp")
				}
				// Custom Ports
				for _, svc := range zone.Services.CustomPorts {
					if strings.EqualFold(svc.Protocol, "tcp") {
						tcpElements = append(tcpElements, fmt.Sprintf("%s . %d", qIface, svc.Port))
					} else if strings.EqualFold(svc.Protocol, "udp") {
						udpElements = append(udpElements, fmt.Sprintf("%s . %d", qIface, svc.Port))
					}
				}
			}

			// Zone Management
			if zone.Management != nil {
				if zone.Management.SSH {
					addService(qIface, "ssh")
				}
				if zone.Management.Web || zone.Management.WebUI || zone.Management.API {
					addService(qIface, "http")
					addService(qIface, "https")

					if zone.Management.API || zone.Management.Web || zone.Management.WebUI {
						tcpElements = append(tcpElements, fmt.Sprintf("%s . %d", qIface, 8443))
						tcpElements = append(tcpElements, fmt.Sprintf("%s . %d", qIface, 8080))
					}

				}
				if zone.Management.ICMP {
					addService(qIface, "icmp")
				}
				if zone.Management.SNMP {
					addService(qIface, "snmp")
				}
				if zone.Management.Syslog {
					addService(qIface, "syslog")
				}
			}
		}
	}

	// Apply Consolidated Rules
	if len(tcpElements) > 0 {
		sb.AddRule("input", fmt.Sprintf("iifname . tcp dport { %s } accept", strings.Join(tcpElements, ", ")), "[svc] Consolidated TCP services")
	}

	if len(udpElements) > 0 {
		sb.AddRule("input", fmt.Sprintf("iifname . udp dport { %s } accept", strings.Join(udpElements, ", ")), "[svc] Consolidated UDP services")
	}

	// ICMP (Keep separate as it doesn't match dport)
	if len(icmpElements) > 0 {
		sb.AddRule("input", fmt.Sprintf("iifname { %s } meta l4proto icmp accept", strings.Join(icmpElements, ", ")), "[svc] Consolidated ICMP")
	}

	// Add auto-generated IPSet block rules
	for _, ipset := range cfg.IPSets {
		if ipset.Action == "" {
			continue
		}

		// Determine action
		action := "drop"
		switch strings.ToLower(ipset.Action) {
		case "accept":
			action = "accept"
		case "reject":
			action = "reject"
		}

		// Determine chains
		applyInput := ipset.ApplyTo == "input" || ipset.ApplyTo == "both" || ipset.ApplyTo == ""
		applyForward := ipset.ApplyTo == "forward" || ipset.ApplyTo == "both"

		// Default match on source if not specified
		matchSource := ipset.MatchOnSource || (!ipset.MatchOnSource && !ipset.MatchOnDest)
		matchDest := ipset.MatchOnDest

		// Determine address family
		addrFamily := "ip"
		if ipset.Type == "ipv6_addr" {
			addrFamily = "ip6"
		}

		ipsetComment := fmt.Sprintf("[ipset:%s] %s", ipset.Name, action)
		if applyInput {
			if matchSource {
				sb.AddRule("input", fmt.Sprintf("%s saddr @%s limit rate 10/minute log group 0 prefix \"DROP_IPSET: \" counter %s", addrFamily, quote(ipset.Name), action), ipsetComment)
			}
			if matchDest {
				sb.AddRule("input", fmt.Sprintf("%s daddr @%s limit rate 10/minute log group 0 prefix \"DROP_IPSET: \" counter %s", addrFamily, quote(ipset.Name), action), ipsetComment)
			}
		}

		if applyForward {
			if matchSource {
				sb.AddRule("forward", fmt.Sprintf("%s saddr @%s limit rate 10/minute log group 0 prefix \"DROP_IPSET: \" counter %s", addrFamily, ipset.Name, action), ipsetComment)
			}
			if matchDest {
				sb.AddRule("forward", fmt.Sprintf("%s daddr @%s limit rate 10/minute log group 0 prefix \"DROP_IPSET: \" counter %s", addrFamily, ipset.Name, action), ipsetComment)
			}
		}
	}

	// Build zone map for policy rules
	zoneMap = buildZoneMapForScript(cfg)

	// =========================================================================
	// Policy Processing & Merging
	// =========================================================================
	// We aggregate policies by their canonical From->To pair to handle aliases
	// (e.g. "firewall" == "local") and multiple policy blocks.

	type AggregatedPolicy struct {
		From    string
		To      string
		Rules   []config.PolicyRule
		Action  string   // "accept", "drop", or "reject"
		Sources []string // Names of source policies for comments
	}

	// Use map for aggregation: key = "canonical(From)->canonical(To)"
	policyMap := make(map[string]*AggregatedPolicy)

	// 1. Process Explicit Policies
	for _, pol := range cfg.Policies {
		if pol.Disabled {
			continue
		}
		cFrom := canonicalZoneName(pol.From)
		cTo := canonicalZoneName(pol.To)
		key := fmt.Sprintf("%s->%s", cFrom, cTo)

		agg, exists := policyMap[key]
		if !exists {
			agg = &AggregatedPolicy{
				From:   cFrom,
				To:     cTo,
				Action: "drop", // Default safe fallback if not specified
			}
			policyMap[key] = agg
		}

		// Append rules
		agg.Rules = append(agg.Rules, pol.Rules...)

		// Update Action (Last one matches standard imperative config behavior)
		if pol.Action != "" {
			agg.Action = strings.ToLower(pol.Action)
		}

		// Track source
		if pol.Name != "" {
			agg.Sources = append(agg.Sources, pol.Name)
		} else {
			agg.Sources = append(agg.Sources, "unnamed")
		}
	}

	// 2. Inject Implicit Policies (if missing)
	for _, zone := range cfg.Zones {
		// key for flywall -> zone
		key := fmt.Sprintf("%s->%s", brand.LowerName, canonicalZoneName(zone.Name))

		if _, exists := policyMap[key]; !exists {
			// Create implicit policy
			policyMap[key] = &AggregatedPolicy{
				From:    brand.LowerName,
				To:      canonicalZoneName(zone.Name),
				Action:  "accept",
				Rules:   []config.PolicyRule{}, // No explicit rules, just default action
				Sources: []string{"implicit_default"},
			}
		}
	}

	// Add policy chains and rules
	// Initialize maps for O(1) dispatch

	// Initialize maps for O(1) dispatch
	inputMap := make(map[string]string)   // iifname -> verdict
	outputMap := make(map[string]string)  // oifname -> verdict
	forwardMap := make(map[string]string) // iifname . oifname -> verdict

	// 3. Generate Chain Definitions (Sorted for determinism)
	// We need keys
	var keys []string
	for k := range policyMap {
		keys = append(keys, k)
	}
	// Sort by key
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	for _, key := range keys {
		pol := policyMap[key]

		chainName := fmt.Sprintf("policy_%s_%s", pol.From, pol.To)
		if !isValidIdentifier(chainName) {
			return nil, fmt.Errorf("invalid policy chain name derived from zones: %s", chainName)
		}

		sourcesStr := strings.Join(pol.Sources, ", ")
		chainComment := fmt.Sprintf("[policy:%s->%s] sources: %s", pol.From, pol.To, sourcesStr)
		sb.AddChain(chainName, "", "", 0, "", chainComment)

		// Add rules to policy chain
		for i, rule := range pol.Rules {
			if rule.Disabled {
				continue
			}
			ruleExpr, err := BuildRuleExpression(rule, timezone)
			if err != nil {
				return nil, err
			}
			if ruleExpr != "" {
				ruleComment := ""
				if rule.Name != "" {
					ruleComment = fmt.Sprintf("[policy:%s->%s] %s", pol.From, pol.To, rule.Name)
				} else {
					ruleComment = fmt.Sprintf("[policy:%s->%s] rule#%d", pol.From, pol.To, i+1)
				}
				sb.AddRule(chainName, ruleExpr, ruleComment)
			}
		}

		// Add default action
		defaultAction := "drop"
		if strings.ToLower(pol.Action) == "accept" {
			defaultAction = "accept"
		}

		// If default is drop/reject, log it
		if defaultAction != "accept" {
			prefix := fmt.Sprintf("DROP_POL_%s_%s: ", pol.From, pol.To)
			if len(prefix) > 28 {
				prefix = prefix[:28] + ": "
			}
			sb.AddRule(chainName, fmt.Sprintf("limit rate 10/minute log group 0 prefix %q counter %s", prefix, defaultAction), fmt.Sprintf("[policy:%s->%s] default", pol.From, pol.To))
		} else {
			sb.AddRule(chainName, "counter "+defaultAction, fmt.Sprintf("[policy:%s->%s] default", pol.From, pol.To))
		}

		// Map Collection for Verdict Maps (Optimization)
		fromIfaces := zoneMap[pol.From]
		toIfaces := zoneMap[pol.To]

		isInput := pol.To == brand.LowerName
		isOutput := pol.From == brand.LowerName

		if isInput {
			// Input Chain Dispatch
			for _, iface := range fromIfaces {
				if _, exists := inputMap[iface]; !exists {
					inputMap[iface] = fmt.Sprintf("jump %s", chainName)
				}
			}
		} else if isOutput {
			// Output Chain Dispatch
			for _, iface := range toIfaces {
				if _, exists := outputMap[iface]; !exists {
					outputMap[iface] = fmt.Sprintf("jump %s", chainName)
				}
			}
		} else {
			// Forward Chain Dispatch
			for _, src := range fromIfaces {
				for _, dst := range toIfaces {
					key := fmt.Sprintf("%s . %s", quote(src), quote(dst))
					if _, exists := forwardMap[key]; !exists {
						forwardMap[key] = fmt.Sprintf("jump %s", chainName)
					}
				}
			}
		}
	}

	// Generate Verdict Maps and Dispatch Rules
	// Input Map
	if len(inputMap) > 0 {
		var elements []string
		for iface, verdict := range inputMap {
			elements = append(elements, fmt.Sprintf("%s : %s", quote(iface), verdict))
		}
		sb.AddMap("input_vmap", "ifname", "verdict", "[base] Input dispatch map", nil, elements)
		sb.AddRule("input", "iifname vmap @input_vmap", "[base] Policy dispatch")
	}

	// Output Map
	if len(outputMap) > 0 {
		var elements []string
		for iface, verdict := range outputMap {
			elements = append(elements, fmt.Sprintf("%s : %s", quote(iface), verdict))
		}
		sb.AddMap("output_vmap", "ifname", "verdict", "[base] Output dispatch map", nil, elements)
		sb.AddRule("output", "oifname vmap @output_vmap", "[base] Policy dispatch")
	}

	// Forward Map
	if len(forwardMap) > 0 {
		var elements []string
		for key, verdict := range forwardMap {
			elements = append(elements, fmt.Sprintf("%s : %s", key, verdict))
		}
		sb.AddMap("forward_vmap", "ifname . ifname", "verdict", "[base] Forward dispatch map", nil, elements)
		sb.AddRule("forward", "meta iifname . meta oifname vmap @forward_vmap", "[base] Policy dispatch")
	}

	// Add final drop rules (already in chain policy, but explicit for logging)
	// Add final drop rules with rate-limited logging to prevent "Log Spam Death Spiral"
	//
	// When inline learning mode is enabled, we use nfqueue instead of drop.
	// This holds packets until the learning engine returns a verdict, fixing the
	// "first packet" problem where the first packet of a new flow would be dropped
	// before the allow rule could be added.

	// DEBUG: Log the inline mode check
	if cfg.RuleLearning != nil {
		log.Printf("[FIREWALL] DEBUG: RuleLearning exists - Enabled=%v, InlineMode=%v", cfg.RuleLearning.Enabled, cfg.RuleLearning.InlineMode)
	} else {
		log.Printf("[FIREWALL] DEBUG: RuleLearning is NIL")
	}

	if cfg.RuleLearning != nil && cfg.RuleLearning.Enabled && cfg.RuleLearning.InlineMode {
		// Get offload mark from config (default 0x200000)
		offloadMarkStr := cfg.RuleLearning.OffloadMark
		offloadMark, err := config.ParseOffloadMark(offloadMarkStr)
		if err != nil {
			log.Printf("[FIREWALL] Invalid offload mark '%s': %v, using default", offloadMarkStr, err)
			offloadMark = 0x200000
		}

		// Rule 1: Bypass rule for offloaded flows (high priority)
		// Flows with the offload mark are accepted before reaching nfqueue
		log.Printf("[FIREWALL] DEBUG: Adding bypass rules for offload mark 0x%x", offloadMark)
		sb.AddRule("input", fmt.Sprintf("ct mark 0x%x accept", offloadMark), "[feature] Inline IPS bypass (offloaded flows)")
		sb.AddRule("forward", fmt.Sprintf("ct mark 0x%x accept", offloadMark), "[feature] Inline IPS bypass (offloaded flows)")

		// Rule 2: Queue new/unmarked packets for inspection
		// 'bypass' flag = accept if queue full (fail-open safety)
		queueGroup := cfg.RuleLearning.LogGroup
		log.Printf("[FIREWALL] DEBUG: Adding queue rules for group %d", queueGroup)
		sb.AddRule("input", fmt.Sprintf("queue num %d bypass", queueGroup), "[feature] Inline IPS queue")
		sb.AddRule("forward", fmt.Sprintf("queue num %d bypass", queueGroup), "[feature] Inline IPS queue")
		log.Printf("[FIREWALL] DEBUG: Queue rules added successfully")
	} else {
		// Standard async mode with nflog
		log.Printf("[FIREWALL] DEBUG: Using standard async mode (not inline)")
		sb.AddRule("input", `limit rate 10/minute burst 5 packets log group 0 prefix "DROP_INPUT: " counter drop`, "[base] Final drop")
		sb.AddRule("forward", `limit rate 10/minute burst 5 packets log group 0 prefix "DROP_FWD: " counter drop`, "[base] Final drop")
	}

	return sb, nil
}
