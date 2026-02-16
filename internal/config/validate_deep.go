// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"fmt"
	"net"
)

// DeepValidate performs comprehensive validation including cross-references
func (c *Config) DeepValidate() ValidationErrors {
	var errs ValidationErrors

	// Run existing validations
	errs = append(errs, c.Validate()...)

	// Add cross-validations
	errs = append(errs, c.validateCrossReferences()...)
	errs = append(errs, c.validateSecurityPolicies()...)
	errs = append(errs, c.validateNetworkConsistency()...)
	errs = append(errs, c.validateResourceConstraints()...)

	// Add intent validation
	validator := NewIntentValidator(c)
	intentErrors := validator.ValidateIntent()
	for _, err := range intentErrors {
		errs = append(errs, ValidationError{
			Field:   err.Field,
			Message: err.Message,
		})
	}

	return errs
}

// validateCrossReferences checks for cross-references between config sections
func (c *Config) validateCrossReferences() ValidationErrors {
	var errs ValidationErrors

	// Build lookup maps
	zoneMap := make(map[string]*Zone)
	ifaceMap := make(map[string]*Interface)
	ipsetMap := make(map[string]*IPSet)

	for i := range c.Zones {
		zoneMap[c.Zones[i].Name] = &c.Zones[i]
	}

	for i := range c.Interfaces {
		ifaceMap[c.Interfaces[i].Name] = &c.Interfaces[i]
	}

	for i := range c.IPSets {
		ipsetMap[c.IPSets[i].Name] = &c.IPSets[i]
	}

	// Validate zone references in interfaces
	for i, iface := range c.Interfaces {
		if iface.Zone != "" {
			if _, exists := zoneMap[iface.Zone]; !exists {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("interfaces[%d].zone", i),
					Message: fmt.Sprintf("zone %q does not exist", iface.Zone),
				})
			}
		}
	}

	// Validate interface references in policies
	for i, policy := range c.Policies {
		// Check from/to zones
		if !c.zoneExists(policy.From) {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("policies[%d].from", i),
				Message: fmt.Sprintf("zone %q does not exist", policy.From),
			})
		}
		if !c.zoneExists(policy.To) {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("policies[%d].to", i),
				Message: fmt.Sprintf("zone %q does not exist", policy.To),
			})
		}

		// Check rules
		for j, rule := range policy.Rules {
			// Validate IP set references
			if rule.SrcIPSet != "" {
				if _, exists := ipsetMap[rule.SrcIPSet]; !exists {
					errs = append(errs, ValidationError{
						Field:   fmt.Sprintf("policies[%d].rules[%d].src_ipset", i, j),
						Message: fmt.Sprintf("ipset %q does not exist", rule.SrcIPSet),
					})
				}
			}
			if rule.DestIPSet != "" {
				if _, exists := ipsetMap[rule.DestIPSet]; !exists {
					errs = append(errs, ValidationError{
						Field:   fmt.Sprintf("policies[%d].rules[%d].dest_ipset", i, j),
						Message: fmt.Sprintf("ipset %q does not exist", rule.DestIPSet),
					})
				}
			}
		}
	}

	return errs
}

// validateSecurityPolicies checks for security policy issues
func (c *Config) validateSecurityPolicies() ValidationErrors {
	var errs ValidationErrors

	// Check for policies that allow all traffic
	for i, policy := range c.Policies {
		hasDefaultAllow := false
		for _, rule := range policy.Rules {
			if rule.Action == "accept" &&
				rule.SrcIP == "" && rule.SrcIPSet == "" &&
				rule.DestIP == "" && rule.DestIPSet == "" &&
				rule.Protocol == "" && rule.DestPort == 0 {
				hasDefaultAllow = true
				break
			}
		}

		if hasDefaultAllow {
			errs = append(errs, ValidationError{
				Field:    fmt.Sprintf("policies[%d]", i),
				Message:  "policy contains default allow rule (security risk)",
				Severity: "warning",
			})
		}
	}

	// Check for missing logging on critical policies
	for i, policy := range c.Policies {
		if policy.From == "internet" || policy.To == "internet" {
			hasLogging := false
			for _, rule := range policy.Rules {
				if rule.Log {
					hasLogging = true
					break
				}
			}

			if !hasLogging {
				errs = append(errs, ValidationError{
					Field:    fmt.Sprintf("policies[%d]", i),
					Message:  "internet-facing policy missing logging (security risk)",
					Severity: "warning",
				})
			}
		}
	}

	return errs
}

// validateNetworkConsistency checks for network configuration consistency
func (c *Config) validateNetworkConsistency() ValidationErrors {
	var errs ValidationErrors

	// Check for overlapping IP subnets
	subnets := make(map[string]string) // subnet -> interface
	for _, iface := range c.Interfaces {
		for _, ip := range iface.IPv4 {
			_, ipNet, err := net.ParseCIDR(ip)
			if err != nil {
				continue // Skip invalid CIDRs (caught by basic validation)
			}

			// Check for overlaps
			for existingSubnet, existingIface := range subnets {
				existingIP, existingNet, _ := net.ParseCIDR(existingSubnet)
				if ipNet.Contains(existingIP) || existingNet.Contains(ipNet.IP) {
					errs = append(errs, ValidationError{
						Field: "interfaces",
						Message: fmt.Sprintf("IP subnet overlap: %s (%s) and %s (%s)",
							ip, iface.Name, existingSubnet, existingIface),
						Severity: "warning",
					})
				}
			}
			subnets[ip] = iface.Name
		}
	}

	// Check for routes with invalid interfaces
	for i, route := range c.Routes {
		found := false
		for _, iface := range c.Interfaces {
			if iface.Name == route.Interface {
				found = true
				break
			}
		}

		if !found && route.Interface != "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("routes[%d].interface", i),
				Message: fmt.Sprintf("interface %q does not exist", route.Interface),
			})
		}
	}

	return errs
}

// validateResourceConstraints checks for resource constraint issues
func (c *Config) validateResourceConstraints() ValidationErrors {
	var errs ValidationErrors

	// Count total rules
	totalRules := 0
	for _, policy := range c.Policies {
		totalRules += len(policy.Rules)
	}

	// Warn if too many rules (performance impact)
	if totalRules > 1000 {
		errs = append(errs, ValidationError{
			Field:    "policies",
			Message:  fmt.Sprintf("large number of rules (%d) may impact performance", totalRules),
			Severity: "warning",
		})
	}

	// Count IP sets
	totalIPSets := len(c.IPSets)
	if totalIPSets > 100 {
		errs = append(errs, ValidationError{
			Field:    "ipsets",
			Message:  fmt.Sprintf("large number of IP sets (%d) may impact performance", totalIPSets),
			Severity: "warning",
		})
	}

	return errs
}

// zoneExists checks if a zone exists in the configuration
func (c *Config) zoneExists(zoneName string) bool {
	for _, zone := range c.Zones {
		if zone.Name == zoneName {
			return true
		}
	}
	return false
}
