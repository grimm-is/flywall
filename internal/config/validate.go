// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"fmt"
	"log"
	"net"
	"path/filepath"
	"regexp"
	"strings"
)

// isWildcardZone checks if a zone name is a wildcard pattern.
// Patterns include "*" (any zone) or glob patterns like "vpn*".
func isWildcardZone(zone string) bool {
	return zone == "*" || strings.ContainsAny(zone, "*?[]")
}

// matchesZone checks if a zone name matches a pattern (supports glob).
func matchesZone(pattern, zoneName string) bool {
	if pattern == "*" {
		return true
	}
	matched, err := filepath.Match(pattern, zoneName)
	if err != nil {
		// Malformed pattern - log and treat as no match
		log.Printf("[CONFIG] Warning: malformed zone pattern %q: %v", pattern, err)
		return false
	}
	return matched
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field    string
	Message  string
	Severity string // "error" (default), "warning"
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// HasErrors returns true if there are any validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validate validates the entire configuration.
func (c *Config) Validate() ValidationErrors {
	var errs ValidationErrors

	// Validate zones
	errs = append(errs, c.validateZones()...)

	// Validate VRFs
	errs = append(errs, c.validateVRFs()...)

	// Validate interfaces
	errs = append(errs, c.validateInterfaces()...)

	// Validate interface overlaps (The "Overlapping Subnets" Foot-Gun Fix)
	errs = append(errs, c.validateInterfaceOverlaps()...)

	// Validate IPSets
	errs = append(errs, c.validateIPSets()...)

	// Validate policies
	errs = append(errs, c.validatePolicies()...)

	// Validate NAT rules
	errs = append(errs, c.validateNAT()...)

	// Validate routes
	errs = append(errs, c.validateRoutes()...)

	// Validate QoS policies
	errs = append(errs, c.validateQoS()...)

	return errs
}

func (c *Config) validateZones() ValidationErrors {
	var errs ValidationErrors

	for i, zone := range c.Zones {
		field := fmt.Sprintf("zones[%s]", zone.Name)
		if zone.Name == "" {
			field = fmt.Sprintf("zones[%d]", i)
		}

		// Validate top-level Src
		if zone.Src != "" && !isValidIPOrCIDR(zone.Src) {
			errs = append(errs, ValidationError{
				Field:   field + ".src",
				Message: fmt.Sprintf("invalid IP or CIDR: %s", zone.Src),
			})
		}

		// Validate top-level Dst
		if zone.Dst != "" && !isValidIPOrCIDR(zone.Dst) {
			errs = append(errs, ValidationError{
				Field:   field + ".dst",
				Message: fmt.Sprintf("invalid IP or CIDR: %s", zone.Dst),
			})
		}

		// Validate top-level VLAN (1-4094, 4095 is reserved)
		if zone.VLAN != 0 && (zone.VLAN < 1 || zone.VLAN > 4094) {
			errs = append(errs, ValidationError{
				Field:   field + ".vlan",
				Message: fmt.Sprintf("VLAN must be between 1 and 4094, got %d", zone.VLAN),
			})
		}

		// Validate match blocks
		for j, match := range zone.Matches {
			matchField := fmt.Sprintf("%s.match[%d]", field, j)

			if match.Src != "" && !isValidIPOrCIDR(match.Src) {
				errs = append(errs, ValidationError{
					Field:   matchField + ".src",
					Message: fmt.Sprintf("invalid IP or CIDR: %s", match.Src),
				})
			}

			if match.Dst != "" && !isValidIPOrCIDR(match.Dst) {
				errs = append(errs, ValidationError{
					Field:   matchField + ".dst",
					Message: fmt.Sprintf("invalid IP or CIDR: %s", match.Dst),
				})
			}

			if match.VLAN != 0 && (match.VLAN < 1 || match.VLAN > 4094) {
				errs = append(errs, ValidationError{
					Field:   matchField + ".vlan",
					Message: fmt.Sprintf("VLAN must be between 1 and 4094, got %d", match.VLAN),
				})
			}
		}
	}

	return errs
}

func (c *Config) validateInterfaces() ValidationErrors {
	var errs ValidationErrors
	seen := make(map[string]bool)

	for i, iface := range c.Interfaces {
		field := fmt.Sprintf("interfaces[%d]", i)

		// Check for duplicate names
		if seen[iface.Name] {
			errs = append(errs, ValidationError{
				Field:   field + ".name",
				Message: fmt.Sprintf("duplicate interface name: %s", iface.Name),
			})
		}
		seen[iface.Name] = true

		// Validate interface name format
		if !isValidInterfaceName(iface.Name) {
			errs = append(errs, ValidationError{
				Field:   field + ".name",
				Message: fmt.Sprintf("invalid interface name: %s", iface.Name),
			})
		}

		// Validate IPv4 addresses
		for j, ip := range iface.IPv4 {
			if !isValidCIDR(ip) {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("%s.ipv4[%d]", field, j),
					Message: fmt.Sprintf("invalid IPv4 CIDR: %s", ip),
				})
			}
		}

		// Validate MTU
		if iface.MTU != 0 && (iface.MTU < 576 || iface.MTU > 65535) {
			errs = append(errs, ValidationError{
				Field:   field + ".mtu",
				Message: fmt.Sprintf("MTU must be between 576 and 65535, got %d", iface.MTU),
			})
		}

		// Validate VLANs
		for j, vlan := range iface.VLANs {
			vlanField := fmt.Sprintf("%s.vlans[%d]", field, j)
			vid := 0
			fmt.Sscanf(vlan.ID, "%d", &vid)
			if vid < 1 || vid > 4094 {
				errs = append(errs, ValidationError{
					Field:   vlanField + ".id",
					Message: fmt.Sprintf("VLAN ID must be between 1 and 4094, got %s", vlan.ID),
				})
			}
		}
	}

	return errs
}

// validateInterfaceOverlaps checks for overlapping subnets across interfaces, respecting VRFs.
func (c *Config) validateInterfaceOverlaps() ValidationErrors {
	var errs ValidationErrors

	type ifaceNet struct {
		Name  string
		IPNet *net.IPNet
		CIDR  string
		VRF   string
	}

	var networks []ifaceNet

	// Collect all networks and their assigned VRFs
	for _, iface := range c.Interfaces {
		effectiveVRF := iface.VRF // Default to "" (global/main)

		for _, addr := range iface.IPv4 {
			_, ipnet, err := net.ParseCIDR(addr)
			if err == nil {
				networks = append(networks, ifaceNet{
					Name:  iface.Name,
					IPNet: ipnet,
					CIDR:  addr,
					VRF:   effectiveVRF,
				})
			}
		}
		for _, vlan := range iface.VLANs {
			// VLANs typically inherit parent's VRF unless we support per-VLAN VRF later.
			// Current plan: Interface-level VRF applies to all VLANs on it too?
			// Probably safer to assume yes for L3 domain consistency, or explicitly allow overriding.
			// For now, assume inherited.
			for _, addr := range vlan.IPv4 {
				_, ipnet, err := net.ParseCIDR(addr)
				if err == nil {
					networks = append(networks, ifaceNet{
						Name:  fmt.Sprintf("%s.vlan%s", iface.Name, vlan.ID),
						IPNet: ipnet,
						CIDR:  addr,
						VRF:   effectiveVRF,
					})
				}
			}
		}
	}

	// Compare all pairs
	for i := 0; i < len(networks); i++ {
		for j := i + 1; j < len(networks); j++ {
			n1 := networks[i]
			n2 := networks[j]

			// Skip if different VRFs (this is the key change for overlapping support)
			if n1.VRF != n2.VRF {
				continue
			}

			// Skip if same interface
			if n1.Name == n2.Name {
				if n1.IPNet.String() == n2.IPNet.String() {
					errs = append(errs, ValidationError{
						Field:   fmt.Sprintf("interfaces[%s]", n1.Name),
						Message: fmt.Sprintf("duplicate subnet %s on same interface (VRF: %s)", n1.CIDR, n1.VRF),
					})
				}
				continue
			}

			// Check overlap
			if netsOverlap(n1.IPNet, n2.IPNet) {
				vrfMsg := n1.VRF
				if vrfMsg == "" {
					vrfMsg = "default"
				}
				errs = append(errs, ValidationError{
					Field:    "interfaces",
					Message:  fmt.Sprintf("overlapping subnets detected in VRF '%s': %s (%s) and %s (%s)", vrfMsg, n1.Name, n1.CIDR, n2.Name, n2.CIDR),
					Severity: "error",
				})
			}
		}
	}

	return errs
}

func (c *Config) validateVRFs() ValidationErrors {
	var errs ValidationErrors
	seen := make(map[string]bool)
	seenIDs := make(map[int]bool)

	for i, vrf := range c.VRFs {
		field := fmt.Sprintf("vrfs[%d]", i)

		if vrf.Name == "" {
			errs = append(errs, ValidationError{
				Field:   field + ".name",
				Message: "VRF name is required",
			})
		}

		if seen[vrf.Name] {
			errs = append(errs, ValidationError{
				Field:   field + ".name",
				Message: fmt.Sprintf("duplicate VRF name: %s", vrf.Name),
			})
		}
		seen[vrf.Name] = true

		if vrf.TableID <= 0 || vrf.TableID > 4294967295 {
			errs = append(errs, ValidationError{
				Field:   field + ".table_id",
				Message: fmt.Sprintf("invalid VRF table ID: %d", vrf.TableID),
			})
		}

		if seenIDs[vrf.TableID] {
			errs = append(errs, ValidationError{
				Field:   field + ".table_id",
				Message: fmt.Sprintf("duplicate VRF table ID: %d", vrf.TableID),
			})
		}
		seenIDs[vrf.TableID] = true
	}

	// Check that interfaces reference valid VRFs
	for i, iface := range c.Interfaces {
		if iface.VRF != "" && !seen[iface.VRF] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("interfaces[%d].vrf", i),
				Message: fmt.Sprintf("unknown VRF: %s", iface.VRF),
			})
		}
	}

	return errs
}

// netsOverlap returns true if two subnets overlap
func netsOverlap(n1, n2 *net.IPNet) bool {
	// Check if n1 contains n2's network address OR n2 contains n1's network address
	// Note: This covers exact match, containment, etc.
	return n1.Contains(n2.IP) || n2.Contains(n1.IP)
}

func (c *Config) validateIPSets() ValidationErrors {
	var errs ValidationErrors
	seen := make(map[string]bool)

	for i, ipset := range c.IPSets {
		field := fmt.Sprintf("ipsets[%d]", i)

		// Check for duplicate names
		if seen[ipset.Name] {
			errs = append(errs, ValidationError{
				Field:   field + ".name",
				Message: fmt.Sprintf("duplicate IPSet name: %s", ipset.Name),
			})
		}
		seen[ipset.Name] = true

		// Validate name format (alphanumeric, underscore, hyphen)
		if !isValidSetName(ipset.Name) {
			errs = append(errs, ValidationError{
				Field:   field + ".name",
				Message: fmt.Sprintf("invalid IPSet name (use alphanumeric, underscore, hyphen): %s", ipset.Name),
			})
		}

		// Validate type
		validTypes := map[string]bool{
			"":             true, // default
			"ipv4_addr":    true,
			"ipv6_addr":    true,
			"inet_service": true,
		}
		if !validTypes[ipset.Type] {
			errs = append(errs, ValidationError{
				Field:   field + ".type",
				Message: fmt.Sprintf("invalid IPSet type: %s (use ipv4_addr, ipv6_addr, or inet_service)", ipset.Type),
			})
		}

		// Warn if both static entries and external source
		if len(ipset.Entries) > 0 && (ipset.FireHOLList != "" || ipset.URL != "") {
			// This is allowed but worth noting - static entries are merged with downloaded
		}

		// Validate static entries
		for j, entry := range ipset.Entries {
			if !isValidIPOrCIDR(entry) {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("%s.entries[%d]", field, j),
					Message: fmt.Sprintf("invalid IP or CIDR: %s", entry),
				})
			}
		}

		// Validate Managed List (generic)
		if ipset.ManagedList != "" {
			if !isValidSetName(ipset.ManagedList) {
				errs = append(errs, ValidationError{
					Field:   field + ".managed_list",
					Message: fmt.Sprintf("invalid managed list name: %s", ipset.ManagedList),
				})
			}
		}

		// Validate FireHOL list name (legacy validation kept for backward compat)
		if ipset.FireHOLList != "" {
			// We no longer strictly validate against a hardcoded map here
			// because the lists are now dynamic.
			// However, simple format check is good.
			if !isValidSetName(ipset.FireHOLList) {
				errs = append(errs, ValidationError{
					Field:   field + ".firehol_list",
					Message: fmt.Sprintf("invalid FireHOL list name: %s", ipset.FireHOLList),
				})
			}
		}

		// Validate URL format
		if ipset.URL != "" && !isValidURL(ipset.URL) {
			errs = append(errs, ValidationError{
				Field:   field + ".url",
				Message: fmt.Sprintf("invalid URL: %s", ipset.URL),
			})
		}

		// Validate refresh hours
		if ipset.RefreshHours < 0 {
			errs = append(errs, ValidationError{
				Field:   field + ".refresh_hours",
				Message: "refresh_hours cannot be negative",
			})
		}

		// Validate action
		validActions := map[string]bool{"": true, "drop": true, "accept": true, "reject": true, "log": true}
		if !validActions[ipset.Action] {
			errs = append(errs, ValidationError{
				Field:   field + ".action",
				Message: fmt.Sprintf("invalid action: %s (use drop, accept, reject, or log)", ipset.Action),
			})
		}

		// Validate apply_to
		validApplyTo := map[string]bool{"": true, "input": true, "forward": true, "both": true}
		if !validApplyTo[ipset.ApplyTo] {
			errs = append(errs, ValidationError{
				Field:   field + ".apply_to",
				Message: fmt.Sprintf("invalid apply_to: %s (use input, forward, or both)", ipset.ApplyTo),
			})
		}
	}

	return errs
}

func (c *Config) validatePolicies() ValidationErrors {
	var errs ValidationErrors
	zones := c.getDefinedZones()

	for i, policy := range c.Policies {
		field := fmt.Sprintf("policies[%d]", i)

		// Validate from zone exists (allow wildcards)
		if policy.From != "" && !isWildcardZone(policy.From) && !zones[policy.From] {
			errs = append(errs, ValidationError{
				Field:   field + ".from",
				Message: fmt.Sprintf("unknown zone: %s", policy.From),
			})
		}

		// Validate to zone exists (allow "firewall"/"self" and wildcards)
		if policy.To != "" && !isWildcardZone(policy.To) && policy.To != "firewall" && policy.To != "Firewall" && policy.To != "self" && !zones[policy.To] {
			errs = append(errs, ValidationError{
				Field:   field + ".to",
				Message: fmt.Sprintf("unknown zone: %s", policy.To),
			})
		}

		// Validate rules
		for j, rule := range policy.Rules {
			ruleField := fmt.Sprintf("%s.rules[%d]", field, j)

			// Validate action
			validActions := map[string]bool{"accept": true, "drop": true, "reject": true}
			if !validActions[strings.ToLower(rule.Action)] {
				errs = append(errs, ValidationError{
					Field:   ruleField + ".action",
					Message: fmt.Sprintf("invalid action: %s", rule.Action),
				})
			}

			// Validate protocol
			if rule.Protocol != "" {
				validProtos := map[string]bool{"tcp": true, "udp": true, "icmp": true, "any": true}
				if !validProtos[strings.ToLower(rule.Protocol)] {
					errs = append(errs, ValidationError{
						Field:   ruleField + ".proto",
						Message: fmt.Sprintf("invalid protocol: %s", rule.Protocol),
					})
				}
			}

			// Validate ports
			if rule.DestPort != 0 && (rule.DestPort < 1 || rule.DestPort > 65535) {
				errs = append(errs, ValidationError{
					Field:   ruleField + ".dest_port",
					Message: fmt.Sprintf("port must be between 1 and 65535, got %d", rule.DestPort),
				})
			}

			// Validate IPSet references
			if rule.SrcIPSet != "" && !c.hasIPSet(rule.SrcIPSet) {
				errs = append(errs, ValidationError{
					Field:   ruleField + ".src_ipset",
					Message: fmt.Sprintf("unknown IPSet: %s", rule.SrcIPSet),
				})
			}
			if rule.DestIPSet != "" && !c.hasIPSet(rule.DestIPSet) {
				errs = append(errs, ValidationError{
					Field:   ruleField + ".dest_ipset",
					Message: fmt.Sprintf("unknown IPSet: %s", rule.DestIPSet),
				})
			}
		}

		// Validate inheritance
		if policy.Inherits != "" {
			// Check that parent policy exists
			parentFound := false
			for j, p := range c.Policies {
				if p.Name == policy.Inherits {
					parentFound = true
					// Prevent self-inheritance
					if i == j {
						errs = append(errs, ValidationError{
							Field:   field + ".inherits",
							Message: "policy cannot inherit from itself",
						})
					}
					break
				}
			}
			if !parentFound {
				errs = append(errs, ValidationError{
					Field:   field + ".inherits",
					Message: fmt.Sprintf("unknown parent policy: %s", policy.Inherits),
				})
			}

			// Check for circular inheritance
			visited := make(map[string]bool)
			current := policy.Inherits
			for current != "" {
				if visited[current] {
					errs = append(errs, ValidationError{
						Field:   field + ".inherits",
						Message: fmt.Sprintf("circular inheritance detected involving: %s", current),
					})
					break
				}
				visited[current] = true

				// Find next parent
				var nextParent string
				for _, p := range c.Policies {
					if p.Name == current {
						nextParent = p.Inherits
						break
					}
				}
				current = nextParent
			}
		}
	}

	// Check for duplicate zone combinations (excluding inherited policies with same zones)
	zoneCombos := make(map[string][]int) // "from->to" -> list of policy indices
	for i, policy := range c.Policies {
		key := policy.From + "->" + policy.To
		zoneCombos[key] = append(zoneCombos[key], i)
	}
	for combo, indices := range zoneCombos {
		if len(indices) > 1 {
			// Multiple policies for same zone combo - check if they're related by inheritance
			for _, idx := range indices {
				policy := c.Policies[idx]
				// If this policy doesn't inherit from another in the same combo, it's a conflict
				hasInheritanceRelation := false
				for _, otherIdx := range indices {
					if idx == otherIdx {
						continue
					}
					otherPolicy := c.Policies[otherIdx]
					if policy.Inherits == otherPolicy.Name || otherPolicy.Inherits == policy.Name {
						hasInheritanceRelation = true
						break
					}
				}
				if !hasInheritanceRelation && policy.Inherits == "" {
					// Only warn for non-inheriting policies - inheriting is intentional
					errs = append(errs, ValidationError{
						Field:    fmt.Sprintf("policies[%d]", idx),
						Message:  fmt.Sprintf("duplicate zone combination %s without inheritance relationship", combo),
						Severity: "warning",
					})
				}
			}
		}
	}

	return errs
}

func (c *Config) validateNAT() ValidationErrors {
	var errs ValidationErrors

	for i, nat := range c.NAT {
		field := fmt.Sprintf("nat[%d]", i)

		validTypes := map[string]bool{"masquerade": true, "snat": true, "dnat": true}
		if !validTypes[strings.ToLower(nat.Type)] {
			errs = append(errs, ValidationError{
				Field:   field + ".type",
				Message: fmt.Sprintf("invalid NAT type: %s", nat.Type),
			})
		}
	}

	return errs
}

func (c *Config) validateRoutes() ValidationErrors {
	var errs ValidationErrors

	for i, route := range c.Routes {
		field := fmt.Sprintf("routes[%d]", i)

		if route.Destination != "" && !isValidCIDR(route.Destination) {
			errs = append(errs, ValidationError{
				Field:   field + ".destination",
				Message: fmt.Sprintf("invalid destination CIDR: %s", route.Destination),
			})
		}

		if route.Gateway != "" && net.ParseIP(route.Gateway) == nil {
			errs = append(errs, ValidationError{
				Field:   field + ".gateway",
				Message: fmt.Sprintf("invalid gateway IP: %s", route.Gateway),
			})
		}
	}

	return errs
}

func (c *Config) validateQoS() ValidationErrors {
	var errs ValidationErrors

	for i, policy := range c.QoSPolicies {
		field := fmt.Sprintf("qos_policies[%d]", i)

		if policy.Interface == "" {
			errs = append(errs, ValidationError{
				Field:   field + ".interface",
				Message: "interface is required",
			})
		}

		if policy.DownloadMbps < 0 {
			errs = append(errs, ValidationError{
				Field:   field + ".download_mbps",
				Message: fmt.Sprintf("download_mbps cannot be negative: %d", policy.DownloadMbps),
			})
		}

		if policy.UploadMbps < 0 {
			errs = append(errs, ValidationError{
				Field:   field + ".upload_mbps",
				Message: fmt.Sprintf("upload_mbps cannot be negative: %d", policy.UploadMbps),
			})
		}
	}

	return errs
}

// Helper functions

func (c *Config) getDefinedZones() map[string]bool {
	zones := make(map[string]bool)

	// 1. Explicit zone definitions
	for _, zone := range c.Zones {
		zones[zone.Name] = true
	}

	// 2. Interface-referenced zones (legacy style: interface has zone = "wan")
	for _, iface := range c.Interfaces {
		if iface.Zone != "" {
			zones[iface.Zone] = true
		}
		for _, vlan := range iface.VLANs {
			if vlan.Zone != "" {
				zones[vlan.Zone] = true
			}
		}
	}
	return zones
}

// NormalizeZoneMappings is a no-op. Zone-interface normalization is now handled
// by the canonicalizeZones post-load migration which converts all syntaxes to zone.Matches.
func (c *Config) NormalizeZoneMappings() {
	// No-op: migration handles this
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (c *Config) hasIPSet(name string) bool {
	for _, ipset := range c.IPSets {
		if ipset.Name == name {
			return true
		}
	}
	return false
}

func isValidInterfaceName(name string) bool {
	if name == "" || len(name) > 15 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z][a-zA-Z0-9._-]*$`, name)
	return matched
}

func isValidSetName(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z][a-zA-Z0-9_-]*$`, name)
	return matched
}

func isValidCIDR(s string) bool {
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

func isValidIPOrCIDR(s string) bool {
	if strings.Contains(s, "/") {
		_, _, err := net.ParseCIDR(s)
		return err == nil
	}
	return net.ParseIP(s) != nil
}

func isValidURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// NormalizePolicies sets default names for policies if not specified.
func (c *Config) NormalizePolicies() {
	for i := range c.Policies {
		p := &c.Policies[i]
		if p.Name == "" {
			p.Name = fmt.Sprintf("%s-to-%s", p.From, p.To)
		}
	}
}

// IntentValidator validates configuration intent and detects logical conflicts
type IntentValidator struct {
	config *Config
}

// NewIntentValidator creates a new intent validator
func NewIntentValidator(config *Config) *IntentValidator {
	return &IntentValidator{config: config}
}

// ValidateIntent checks for logical conflicts and unintended consequences
func (iv *IntentValidator) ValidateIntent() []ValidationError {
	var errors []ValidationError

	// Check for conflicting rules
	errors = append(errors, iv.checkRuleConflicts()...)

	// Check for overlapping zones
	errors = append(errors, iv.checkZoneOverlap()...)

	// Check for routing loops
	errors = append(errors, iv.checkRoutingLoops()...)

	// Check for security implications
	errors = append(errors, iv.checkSecurityImplications()...)

	// Check for interface consistency
	errors = append(errors, iv.checkInterfaceConsistency()...)

	return errors
}

// checkRuleConflicts looks for conflicting rules between the same zones
func (iv *IntentValidator) checkRuleConflicts() []ValidationError {
	var errors []ValidationError

	// Build policy lookup
	policyMap := make(map[string]*Policy)
	for i := range iv.config.Policies {
		key := fmt.Sprintf("%s->%s", iv.config.Policies[i].From, iv.config.Policies[i].To)
		policyMap[key] = &iv.config.Policies[i]
	}

	// Check each policy for internal conflicts
	for key, policy := range policyMap {
		for i, rule1 := range policy.Rules {
			for j, rule2 := range policy.Rules {
				if i >= j {
					continue // Avoid duplicate checks
				}

				if iv.rulesConflict(rule1, rule2) {
					errors = append(errors, ValidationError{
						Field:   "policies",
						Message: fmt.Sprintf("conflicting rules in same policy: %s rules[%d] vs rules[%d]", key, i, j),
					})
				}
			}
		}
	}

	return errors
}

// rulesConflict checks if two rules have overlapping criteria but conflicting actions
func (iv *IntentValidator) rulesConflict(rule1, rule2 PolicyRule) bool {
	// Skip if same action
	if rule1.Action == rule2.Action {
		return false
	}

	// Check for overlapping match criteria
	protocolMatch := rule1.Protocol == "" || rule2.Protocol == "" || rule1.Protocol == rule2.Protocol
	portMatch := rule1.DestPort == 0 || rule2.DestPort == 0 || rule1.DestPort == rule2.DestPort
	srcIPMatch := iv.ipSetsOverlap(rule1.SrcIP, rule1.SrcIPSet, rule2.SrcIP, rule2.SrcIPSet)
	dstIPMatch := iv.ipSetsOverlap(rule1.DestIP, rule1.DestIPSet, rule2.DestIP, rule2.DestIPSet)

	// Rules conflict if they match on all criteria
	return protocolMatch && portMatch && srcIPMatch && dstIPMatch
}

func (iv *IntentValidator) ipSetsOverlap(ip1, set1, ip2, set2 string) bool {
	// Handle empty (match all)
	if (ip1 == "" && set1 == "") || (ip2 == "" && set2 == "") {
		return true
	}

	// Simple string match for now - could be enhanced with actual IP set parsing
	return ip1 == ip2 && set1 == set2
}

// checkZoneOverlap checks for interfaces assigned to multiple zones
func (iv *IntentValidator) checkZoneOverlap() []ValidationError {
	var errors []ValidationError

	// Check if interfaces belong to multiple zones
	interfaceZones := make(map[string][]string)
	for _, iface := range iv.config.Interfaces {
		if iface.Zone != "" {
			interfaceZones[iface.Name] = append(interfaceZones[iface.Name], iface.Zone)
		}
	}

	for iface, zones := range interfaceZones {
		if len(zones) > 1 {
			errors = append(errors, ValidationError{
				Field:   "interfaces",
				Message: fmt.Sprintf("interface %s assigned to multiple zones: %v", iface, zones),
			})
		}
	}

	// Check for undefined zones referenced by interfaces
	zoneMap := make(map[string]bool)
	for _, zone := range iv.config.Zones {
		zoneMap[zone.Name] = true
	}

	for _, iface := range iv.config.Interfaces {
		if iface.Zone != "" && !zoneMap[iface.Zone] {
			errors = append(errors, ValidationError{
				Field:   "interfaces",
				Message: fmt.Sprintf("interface %s references undefined zone: %s", iface.Name, iface.Zone),
			})
		}
	}

	return errors
}

// checkRoutingLoops checks for potential routing loops
func (iv *IntentValidator) checkRoutingLoops() []ValidationError {
	var errors []ValidationError

	// Build routing graph
	routes := make(map[string][]string) // interface -> gateways
	for _, route := range iv.config.Routes {
		if route.Gateway != "" {
			routes[route.Interface] = append(routes[route.Interface], route.Gateway)
		}
	}

	// Simple loop detection
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for iface := range routes {
		if !visited[iface] {
			if iv.hasRouteLoop(iface, routes, visited, recStack) {
				errors = append(errors, ValidationError{
					Field:   "routes",
					Message: fmt.Sprintf("potential routing loop detected for interface: %s", iface),
				})
			}
		}
	}

	return errors
}

// hasRouteLoop performs DFS to detect loops in routing
func (iv *IntentValidator) hasRouteLoop(iface string, routes map[string][]string, visited, recStack map[string]bool) bool {
	visited[iface] = true
	recStack[iface] = true

	for _, nextHop := range routes[iface] {
		if !visited[nextHop] {
			if iv.hasRouteLoop(nextHop, routes, visited, recStack) {
				return true
			}
		} else if recStack[nextHop] {
			return true // Found a back edge (loop)
		}
	}

	recStack[iface] = false
	return false
}

// checkSecurityImplications checks for security issues
func (iv *IntentValidator) checkSecurityImplications() []ValidationError {
	var errors []ValidationError

	// Check for overly permissive rules
	for i, policy := range iv.config.Policies {
		for j, rule := range policy.Rules {
			if iv.isOverlyPermissive(rule) {
				errors = append(errors, ValidationError{
					Field:   "policies",
					Message: fmt.Sprintf("overly permissive rule detected in policies[%d].rules[%d]: allows all traffic", i, j),
				})
			}
		}
	}

	// Check for policies from internet to internal zones
	for i, policy := range iv.config.Policies {
		if iv.isInternetZone(policy.From) && !iv.isInternetZone(policy.To) {
			for j, rule := range policy.Rules {
				if rule.Action == "accept" && iv.isBroadRule(rule) {
					errors = append(errors, ValidationError{
						Field:   "policies",
						Message: fmt.Sprintf("internet to internal zone allows broad access in policies[%d].rules[%d]", i, j),
					})
				}
			}
		}
	}

	return errors
}

// checkInterfaceConsistency validates interface configurations
func (iv *IntentValidator) checkInterfaceConsistency() []ValidationError {
	var errors []ValidationError

	// Check for duplicate interface names
	ifaceNames := make(map[string]bool)
	for i, iface := range iv.config.Interfaces {
		if iface.Name == "" {
			errors = append(errors, ValidationError{
				Field:   "interfaces",
				Message: fmt.Sprintf("interfaces[%d]: interface name is required", i),
			})
			continue
		}

		if ifaceNames[iface.Name] {
			errors = append(errors, ValidationError{
				Field:   "interfaces",
				Message: fmt.Sprintf("duplicate interface name: %s", iface.Name),
			})
		}
		ifaceNames[iface.Name] = true

		// Validate IP addresses
		for j, ip := range iface.IPv4 {
			if _, _, err := net.ParseCIDR(ip); err != nil {
				errors = append(errors, ValidationError{
					Field:   "interfaces",
					Message: fmt.Sprintf("%s.ipv4[%d] = %s: invalid IPv4 CIDR format", iface.Name, j, ip),
				})
			}
		}

		// Check VLAN consistency - Interface struct doesn't have VLANParent field
		// This would need to be implemented if VLAN support is added
		// if iface.VLANParent != "" {
		// 	found := false
		// 	for _, parent := range iv.config.Interfaces {
		// 		if parent.Name == iface.VLANParent {
		// 			found = true
		// 			break
		// 		}
		// 	}
		// 	if !found {
		// 		errors = append(errors, ValidationError{
		// 			Field:   "interfaces",
		// 			Message: fmt.Sprintf("%s.vlan_parent = %s: VLAN parent interface not found", iface.Name, iface.VLANParent),
		// 		})
		// 	}
		// }
	}

	return errors
}

func (iv *IntentValidator) isOverlyPermissive(rule PolicyRule) bool {
	return (rule.SrcIP == "" && rule.SrcIPSet == "") &&
		(rule.DestIP == "" && rule.DestIPSet == "") &&
		(rule.DestPort == 0 && len(rule.DestPorts) == 0) &&
		(rule.Protocol == "" || rule.Protocol == "any") &&
		rule.Service == "" && len(rule.Services) == 0 &&
		rule.Action == "accept"
}

// isInternetZone checks if a zone represents the internet
func (iv *IntentValidator) isInternetZone(zone string) bool {
	return zone == "internet" || zone == "wan" || zone == "external"
}

func (iv *IntentValidator) isBroadRule(rule PolicyRule) bool {
	return (rule.SrcIP == "" || rule.SrcIP == "0.0.0.0/0") &&
		(rule.DestIP == "" || rule.DestIP == "0.0.0.0/0") &&
		(rule.DestPort == 0 || rule.DestPort == -1)
}
