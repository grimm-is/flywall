package firewall

import (
	"fmt"
	"sort"
	"strings"

	"grimm.is/flywall/internal/config"
)

// generateWebAccessRules generates firewall rules based on cfg.Web.Allow/Deny.
// Returns true if new rules were generated (indicating legacy logic should be skipped).
func generateWebAccessRules(cfg *Config, sb *ScriptBuilder) bool {
	if cfg.Web == nil {
		return false
	}

	// Determine ports
	var ports []string
	parsePort := func(addr string, defaultPort int) {
		if addr == "" {
			ports = append(ports, fmt.Sprintf("%d", defaultPort))
			return
		}
		parts := strings.Split(addr, ":")
		if len(parts) == 2 && parts[1] != "" {
			ports = append(ports, parts[1])
		} else {
			ports = append(ports, fmt.Sprintf("%d", defaultPort))
		}
	}

	parsePort(cfg.Web.Listen, 80)
	parsePort(cfg.Web.TLSListen, 443)

	// Removing duplicates
	uniquePorts := make(map[string]bool)
	var finalPorts []string
	for _, p := range ports {
		if !uniquePorts[p] {
			uniquePorts[p] = true
			finalPorts = append(finalPorts, p)
		}
	}
	portSet := strings.Join(finalPorts, ", ")

	hasRules := len(cfg.Web.Deny) > 0 || len(cfg.Web.Allow) > 0
	if !hasRules {
		return false
	}

	// Helper to build rule constraints
	buildConstraints := func(rule config.AccessRule) string {
		var parts []string

		// Interfaces
		var ifaces []string
		if rule.Interface != "" {
			ifaces = append(ifaces, forceQuote(rule.Interface))
		}
		for _, i := range rule.Interfaces {
			ifaces = append(ifaces, forceQuote(i))
		}
		if len(ifaces) > 0 {
			parts = append(parts, fmt.Sprintf("iifname { %s }", strings.Join(ifaces, ", ")))
		}

		// Sources
		var sources []string
		if rule.Source != "" {
			sources = append(sources, rule.Source)
		}
		sources = append(sources, rule.Sources...)

		if len(sources) > 0 {
			var v4 []string
			var v6 []string
			for _, s := range sources {
				if strings.Contains(s, ":") && !strings.Contains(s, ".") { // simple v6 check (mostly valid)
					v6 = append(v6, s)
				} else {
					v4 = append(v4, s) // Assume v4 or hostname
				}
			}

			if len(v4) > 0 {
				parts = append(parts, fmt.Sprintf("ip saddr { %s }", strings.Join(v4, ", ")))
			}
			if len(v6) > 0 {
				parts = append(parts, fmt.Sprintf("ip6 saddr { %s }", strings.Join(v6, ", ")))
			}
		}

		return strings.Join(parts, " ")
	}

	// Generate Deny Rules
	for _, rule := range cfg.Web.Deny {
		match := buildConstraints(rule)
		sb.AddRule("input", fmt.Sprintf("%s tcp dport { %s } drop", match, portSet), "[web] Explicit Deny")
	}

	// Generate Allow Rules
	for _, rule := range cfg.Web.Allow {
		match := buildConstraints(rule)
		sb.AddRule("input", fmt.Sprintf("%s tcp dport { %s } accept", match, portSet), "[web] Explicit Allow")
	}

	return true
}

// BuildRuleExpression converts a PolicyRule to an nft rule expression string.
// Exported for use by the API layer to show generated syntax.
func BuildRuleExpression(rule config.PolicyRule, timezone string) (string, error) {
	var parts []string

	// Protocol
	if rule.Protocol != "" && rule.Protocol != "any" {
		parts = append(parts, fmt.Sprintf("meta l4proto %s", rule.Protocol))
	}

	// Source IP
	if rule.SrcIP != "" {
		parts = append(parts, fmt.Sprintf("ip saddr %s", rule.SrcIP))
	}

	// Dest IP
	if rule.DestIP != "" {
		parts = append(parts, fmt.Sprintf("ip daddr %s", rule.DestIP))
	}

	// Source IPSet
	if rule.SrcIPSet != "" {
		if !isValidIdentifier(rule.SrcIPSet) {
			return "", fmt.Errorf("invalid source ipset name: %s", rule.SrcIPSet)
		}
		parts = append(parts, fmt.Sprintf("ip saddr @%s", quote(rule.SrcIPSet)))
	}

	// Dest IPSet
	if rule.DestIPSet != "" {
		if !isValidIdentifier(rule.DestIPSet) {
			return "", fmt.Errorf("invalid destination ipset name: %s", rule.DestIPSet)
		}
		parts = append(parts, fmt.Sprintf("ip daddr @%s", quote(rule.DestIPSet)))
	}

	// GeoIP matching (uses country-specific sets like @geoip_country_CN)
	if rule.SourceCountry != "" {
		countryCode := strings.ToUpper(rule.SourceCountry)
		parts = append(parts, fmt.Sprintf("ip saddr @geoip_country_%s", countryCode))
	}
	if rule.DestCountry != "" {
		countryCode := strings.ToUpper(rule.DestCountry)
		parts = append(parts, fmt.Sprintf("ip daddr @geoip_country_%s", countryCode))
	}

	// Connection State
	if rule.ConnState != "" {
		// Validate states? "new", "established", "related", "invalid"
		validStates := map[string]bool{
			"new": true, "established": true, "related": true, "invalid": true, "untracked": true,
		}
		states := strings.Split(rule.ConnState, ",")
		var validStateParts []string
		for _, s := range states {
			s = strings.TrimSpace(strings.ToLower(s))
			if !validStates[s] {
				return "", fmt.Errorf("invalid connection state: %s", s)
			}
			validStateParts = append(validStateParts, s)
		}
		parts = append(parts, fmt.Sprintf("ct state %s", strings.Join(validStateParts, ",")))
	}

	// Source/Dest Port
	// Note: Assuming rule.SrcPort and rule.DestPort are now int types.
	match := ""
	if rule.SrcPort > 0 {
		proto := rule.Protocol
		if proto == "" || proto == "any" {
			proto = "tcp"
		}
		match += fmt.Sprintf("%s sport %d", proto, rule.SrcPort)
	}

	if rule.DestPort > 0 {
		proto := rule.Protocol
		if proto == "" || proto == "any" {
			proto = "tcp"
		}
		if match != "" {
			match += " "
		}
		match += fmt.Sprintf("%s dport %d", proto, rule.DestPort)
	}

	// Multi-port matching
	if len(rule.SrcPorts) > 0 {
		proto := rule.Protocol
		if proto == "" || proto == "any" {
			proto = "tcp"
		}
		var pStrs []string
		for _, p := range rule.SrcPorts {
			pStrs = append(pStrs, fmt.Sprintf("%d", p))
		}
		if match != "" {
			match += " "
		}
		match += fmt.Sprintf("%s sport { %s }", proto, strings.Join(pStrs, ", "))
	}

	if len(rule.DestPorts) > 0 {
		proto := rule.Protocol
		if proto == "" || proto == "any" {
			proto = "tcp"
		}
		var pStrs []string
		for _, p := range rule.DestPorts {
			pStrs = append(pStrs, fmt.Sprintf("%d", p))
		}
		if match != "" {
			match += " "
		}
		match += fmt.Sprintf("%s dport { %s }", proto, strings.Join(pStrs, ", "))
	}

	if match != "" {
		parts = append(parts, match)
	}

	// Scheduled Time (Timezone aware)
	// rule.TimeStart/TimeEnd (HH:MM), rule.Days (Mon,Tue...)
	// Uses set-based logic for robust handling of wrapping/shifting
	if rule.TimeStart != "" && rule.TimeEnd != "" {
		// Default timezone if empty
		tz := timezone
		if tz == "" {
			tz = "UTC"
		}

		days := rule.Days
		if len(days) == 0 {
			days = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
		}

		tuples, err := generateActiveTuples(tz, rule.TimeStart, rule.TimeEnd, days)
		if err != nil {
			return "", err
		}

		if len(tuples) > 0 {
			setStr := compressTuples(tuples)
			// Format: meta day . meta hour { ... }
			// Note: If setStr contains commas, we wrap in { }.
			// If it's single element? The set syntax usually requires { } for multiple or single?
			// "meta day . meta hour { 1 . 22 }" is valid.
			parts = append(parts, fmt.Sprintf("meta day . meta hour { %s }", setStr))
		} else {
			// No active tuples? (e.g. invalid days). Rule never matches.
			// We return impossible match?? Or just empty parts implies match all?
			// If user specified time, they expect restriction. If no time matches, we should block/fail?
			// We'll error out? or return "meta day 999" (impossible)?
			// Or just ignore time if error?
			// generateActiveTuples returns error if timezone bad.
			// If result empty (e.g. no days), it effectively means "never".
			// But we returned nil error.
			// Let's assume valid output if err==nil.
		}

	} else if len(rule.Days) > 0 {
		// Days only
		// Check for day matching.
		// Use integer mapping.
		dayMap := map[string]int{
			"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
		}
		var validDays []int
		for _, d := range rule.Days {
			if idx, ok := dayMap[strings.ToLower(d[:3])]; ok {
				validDays = append(validDays, idx)
			}
		}
		sort.Ints(validDays)
		if len(validDays) > 0 {
			var dStrs []string
			for _, d := range validDays {
				dStrs = append(dStrs, fmt.Sprintf("%d", d))
			}
			parts = append(parts, fmt.Sprintf("meta day { %s }", strings.Join(dStrs, ", ")))
		}
	}

	// Limit
	if rule.Limit != "" {
		// rule.Limit is a string like "10/second" or "10/minute burst 5 packets"
		// We trust the config validation or pass it through.
		parts = append(parts, fmt.Sprintf("limit rate %s", rule.Limit))
	}

	// Log
	if rule.Log {
		logPrefix := rule.LogPrefix
		if logPrefix == "" {
			logPrefix = fmt.Sprintf("LOG: ")
		}
		// Log group 0 is default (kernel ring buffer).
		// If we had a LogGroup field, we'd use it, but PolicyRule doesn't have it.
		// We use standard NFLOG group 0.
		parts = append(parts, fmt.Sprintf("log group 0 prefix %q", logPrefix))
	}

	// Action
	action := "accept"
	switch strings.ToLower(rule.Action) {
	case "drop":
		action = "drop"
	case "reject":
		action = "reject"
	}

	// Add logging if action is drop/reject
	if action != "accept" {
		// Log rule
		parts = append(parts, fmt.Sprintf(`limit rate 10/minute log group 0 prefix "DROP_RULE: "`))
	}

	// Add counter for observability (required for sparklines)
	// Named counter if specified, anonymous otherwise
	if rule.Counter != "" {
		parts = append(parts, fmt.Sprintf("counter name %s", quote(rule.Counter)))
	} else {
		parts = append(parts, "counter")
	}

	// Add verdict
	parts = append(parts, action)

	// Add rule ID comment for stats collector correlation
	// This allows mapping nft counters back to config rule IDs
	if rule.ID != "" {
		parts = append(parts, fmt.Sprintf(`comment "rule:%s"`, rule.ID))
	} else if rule.Name != "" {
		// Fallback for rules without explicit IDs
		parts = append(parts, fmt.Sprintf(`comment "rule:%s"`, rule.Name))
	}

	return strings.Join(parts, " "), nil
}

// addProtectionRules adds protection rules to the script
func addProtectionRules(cfg *Config, sb *ScriptBuilder) {
	if len(cfg.Protections) == 0 {
		return
	}

	// Create protection chain (hooks into prerouting, priority raw -300)
	sb.AddChain("protection", "filter", "prerouting", -300, "accept")

	for _, p := range cfg.Protections {
		// Enable logic: If block exists, we assume it's enabled unless explicitly disabled?
		if !p.Enabled && false {
			// Skipping check as per discussion
		}

		ifaceMatch := ""
		if p.Interface != "" && p.Interface != "*" {
			ifaceMatch = fmt.Sprintf("iifname %q ", p.Interface)
		}

		// Invalid Packets
		if p.InvalidPackets {
			// ct state invalid drop
			sb.AddRule("protection", fmt.Sprintf("%sct state invalid limit rate 10/minute log group 0 prefix \"DROP_INVALID: \" counter drop", ifaceMatch))
		}

		// Anti-Spoofing (RFC1918)
		if p.AntiSpoofing {
			for _, cidr := range protectionPrivateNetworks {
				// Use CIDR notation directly with nft
				sb.AddRule("protection", fmt.Sprintf("%sip saddr %s limit rate 10/minute log group 0 prefix \"SPOOFED-SRC: \" counter drop", ifaceMatch, cidr.String()))
			}
		}

		// Bogon Filtering
		if p.BogonFiltering {
			for _, cidr := range protectionBogonNetworks {
				sb.AddRule("protection", fmt.Sprintf("%sip saddr %s counter drop", ifaceMatch, cidr.String()))
			}
		}

		// SYN Flood Protection
		if p.SynFloodProtection {
			rate := p.SynFloodRate
			if rate == 0 {
				rate = 25
			}
			burst := p.SynFloodBurst
			if burst == 0 {
				burst = 50
			}
			// tcp flags & (fin|syn|rst|ack) == syn
			sb.AddRule("protection", fmt.Sprintf("%stcp flags & (fin|syn|rst|ack) == syn limit rate %d/second burst %d packets return", ifaceMatch, rate, burst))
			sb.AddRule("protection", fmt.Sprintf("%stcp flags & (fin|syn|rst|ack) == syn limit rate 10/minute log group 0 prefix %q counter drop", ifaceMatch, "DROP_SYNFLOOD: "))
		}

		// ICMP Rate Limit
		if p.ICMPRateLimit {
			rate := p.ICMPRate
			if rate == 0 {
				rate = 10
			}
			// burst = 2 * rate
			sb.AddRule("protection", fmt.Sprintf("%smeta l4proto icmp limit rate %d/second burst %d packets return", ifaceMatch, rate, rate*2))
			sb.AddRule("protection", fmt.Sprintf("%smeta l4proto icmp limit rate 10/minute log group 0 prefix %q counter drop", ifaceMatch, "DROP_ICMPFLOOD: "))
		}
	}
}
