// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"grimm.is/flywall/internal/config"
)

// OptimizedScriptBuilder extends ScriptBuilder with rule optimization
type OptimizedScriptBuilder struct {
	*ScriptBuilder
	optimizationEnabled bool
	optimizationLevel   int // 0=none, 1=basic, 2=advanced
	setCounter          int
}

// NewOptimizedScriptBuilder creates a new optimized script builder
func NewOptimizedScriptBuilder(builder *ScriptBuilder) *OptimizedScriptBuilder {
	return &OptimizedScriptBuilder{
		ScriptBuilder:       builder,
		optimizationEnabled: true,
		optimizationLevel:   2, // Advanced optimization by default
		setCounter:          0,
	}
}

// SetOptimizationLevel configures the optimization level
func (osb *OptimizedScriptBuilder) SetOptimizationLevel(level int) {
	osb.optimizationLevel = level
}

// AddOptimizedPolicyRules adds policy rules with optimization
func (osb *OptimizedScriptBuilder) AddOptimizedPolicyRules(policies []config.Policy) error {
	if !osb.optimizationEnabled {
		// Fall back to non-optimized addition
		for _, policy := range policies {
			for _, rule := range policy.Rules {
				if err := osb.addPolicyRule(policy, rule); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Group rules across all policies for optimization
	ruleGroups := osb.groupRulesByPattern(policies)

	// Process each group
	for _, group := range ruleGroups {
		if len(group.rules) >= 3 {
			// Create optimized set for this group
			if err := osb.createOptimizedRuleGroup(group); err != nil {
				return err
			}
		} else {
			// Add rules as-is
			for _, ruleAndPolicy := range group.rules {
				if err := osb.addPolicyRule(ruleAndPolicy.policy, ruleAndPolicy.rule); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// ruleAndPolicy pairs a rule with its parent policy
type ruleAndPolicy struct {
	rule   config.PolicyRule
	policy config.Policy
}

// ruleGroup represents a group of similar rules
type ruleGroup struct {
	rules   []ruleAndPolicy
	action  string
	proto   string
	dstPort int
}

// groupRulesByPattern groups rules that can be optimized together
func (osb *OptimizedScriptBuilder) groupRulesByPattern(policies []config.Policy) map[string]*ruleGroup {
	groups := make(map[string]*ruleGroup)

	for _, policy := range policies {
		for _, rule := range policy.Rules {
			// Skip rules that don't fit optimization patterns
			if !osb.canOptimize(rule) {
				continue
			}

			// Create group key
			key := osb.createGroupKey(rule)

			if _, exists := groups[key]; !exists {
				groups[key] = &ruleGroup{
					action:  rule.Action,
					proto:   rule.Protocol,
					dstPort: rule.DestPort,
				}
			}

			groups[key].rules = append(groups[key].rules, ruleAndPolicy{
				rule:   rule,
				policy: policy,
			})
		}
	}

	return groups
}

// canOptimize checks if a rule can be optimized
func (osb *OptimizedScriptBuilder) canOptimize(rule config.PolicyRule) bool {
	// Can optimize if it has source IPs and simple match conditions
	hasSrcMatch := rule.SrcIP != "" || rule.SrcIPSet != ""
	hasSimpleMatch := rule.ConnState == "" && rule.SourceCountry == "" &&
		rule.TCPFlags == "" && rule.TimeStart == "" && rule.DestIP == ""

	return hasSrcMatch && hasSimpleMatch && rule.Action != ""
}

// createGroupKey creates a unique key for grouping similar rules
func (osb *OptimizedScriptBuilder) createGroupKey(rule config.PolicyRule) string {
	return fmt.Sprintf("%s_%s_%d", rule.Action, rule.Protocol, rule.DestPort)
}

// createOptimizedRuleGroup creates an optimized rule for a group of similar rules
func (osb *OptimizedScriptBuilder) createOptimizedRuleGroup(group *ruleGroup) error {
	// Collect all source IPs from the group
	var srcIPs []string
	for _, ruleAndPolicy := range group.rules {
		if ruleAndPolicy.rule.SrcIP != "" {
			srcIPs = append(srcIPs, ruleAndPolicy.rule.SrcIP)
		}
	}

	// Remove duplicates
	srcIPs = osb.removeDuplicateStrings(srcIPs)

	// Only optimize if we have enough IPs
	if len(srcIPs) < 3 {
		// Add rules individually
		for _, ruleAndPolicy := range group.rules {
			if err := osb.addPolicyRule(ruleAndPolicy.policy, ruleAndPolicy.rule); err != nil {
				return err
			}
		}
		return nil
	}

	// Optimize IP addresses by merging into CIDR blocks
	if osb.optimizationLevel >= 2 {
		srcIPs = osb.mergeAdjacentIPs(srcIPs)
	}

	// Create IP set
	setName := osb.createIPSet("src", srcIPs)

	// Create single rule using the set
	chainName := osb.getChainName(group.rules[0].policy, group.rules[0].rule)
	ruleStr := osb.buildRuleFromGroup(group, setName)
	
	osb.AddRule(chainName, ruleStr)

	return nil
}

// createIPSet creates an optimized IP set
func (osb *OptimizedScriptBuilder) createIPSet(prefix string, ips []string) string {
	if len(ips) == 0 {
		return ""
	}

	// Sort IPs for better optimization
	sortedIPs := osb.sortIPs(ips)

	// Create set name
	osb.setCounter++
	setName := fmt.Sprintf("opt_%s_%d", prefix, osb.setCounter)

	// Add set creation to script
	osb.AddSet(setName, "ipv4_addr", fmt.Sprintf("Optimized %s IP set", prefix), len(sortedIPs)*2)
	
	// Add elements in batches
	if len(sortedIPs) > 0 {
		osb.AddSetElements(setName, sortedIPs)
	}

	return setName
}

// mergeAdjacentIPs merges adjacent IP ranges for optimization
func (osb *OptimizedScriptBuilder) mergeAdjacentIPs(ips []string) []string {
	if len(ips) < 2 {
		return ips
	}

	// Parse and sort IP addresses
	parsedIPs := make([]net.IP, 0, len(ips))
	for _, ipStr := range ips {
		if ip := net.ParseIP(ipStr); ip != nil && ip.To4() != nil {
			parsedIPs = append(parsedIPs, ip.To4())
		}
	}

	if len(parsedIPs) < 2 {
		return ips
	}

	// Sort IPs
	sort.Slice(parsedIPs, func(i, j int) bool {
		return osb.compareIPs(parsedIPs[i], parsedIPs[j]) < 0
	})

	// Merge adjacent IPs into CIDR ranges
	merged := osb.mergeToCIDRs(parsedIPs)

	// Convert back to strings
	result := make([]string, len(merged))
	for i, cidr := range merged {
		result[i] = cidr
	}

	return result
}

// mergeToCIDRs converts adjacent IPs to optimal CIDR ranges
func (osb *OptimizedScriptBuilder) mergeToCIDRs(ips []net.IP) []string {
	if len(ips) == 0 {
		return nil
	}

	var result []string
	i := 0

	for i < len(ips) {
		// Try to create the largest possible CIDR block starting at i
		maxSize := 32
		for maxSize > 0 {
			mask := net.CIDRMask(32-maxSize, 32)
			network := &net.IPNet{
				IP:   ips[i],
				Mask: mask,
			}

			// Check if all consecutive IPs fit in this network
			fits := true
			end := i
			for end < len(ips) && end-i < (1<<maxSize) {
				if !network.Contains(ips[end]) {
					fits = false
					break
				}
				end++
			}

			if fits && end-i > 1 {
				// Found a suitable CIDR block
				result = append(result, network.String())
				i = end
				break
			}
			maxSize--
		}

		if maxSize == 0 {
			// No CIDR optimization possible, use individual IP
			result = append(result, ips[i].String())
			i++
		}
	}

	return result
}

// buildRuleFromGroup builds a rule string from a rule group
func (osb *OptimizedScriptBuilder) buildRuleFromGroup(group *ruleGroup, setName string) string {
	var conditions []string

	// Add protocol match
	if group.proto != "" {
		conditions = append(conditions, fmt.Sprintf("ip protocol %s", group.proto))
	}

	// Add destination port match
	if group.dstPort > 0 {
		conditions = append(conditions, fmt.Sprintf("th dport %d", group.dstPort))
	}

	// Add source set match
	conditions = append(conditions, fmt.Sprintf("ip saddr @%s", setName))

	// Build complete rule
	action := group.action
	if action == "" {
		action = "accept"
	}

	return fmt.Sprintf("%s %s", action, strings.Join(conditions, " "))
}

// addPolicyRule adds a single policy rule to the script
func (osb *OptimizedScriptBuilder) addPolicyRule(policy config.Policy, rule config.PolicyRule) error {
	chainName := osb.getChainName(policy, rule)
	ruleStr := osb.buildRuleString(rule)
	osb.AddRule(chainName, ruleStr)
	return nil
}

// buildRuleString converts a rule to nftables format
func (osb *OptimizedScriptBuilder) buildRuleString(rule config.PolicyRule) string {
	var conditions []string

	if rule.Protocol != "" {
		conditions = append(conditions, fmt.Sprintf("ip protocol %s", rule.Protocol))
	}
	if rule.SrcIP != "" {
		conditions = append(conditions, fmt.Sprintf("ip saddr %s", rule.SrcIP))
	}
	if rule.SrcIPSet != "" {
		conditions = append(conditions, fmt.Sprintf("ip saddr @%s", rule.SrcIPSet))
	}
	if rule.DestIP != "" {
		conditions = append(conditions, fmt.Sprintf("ip daddr %s", rule.DestIP))
	}
	if rule.DestPort > 0 {
		conditions = append(conditions, fmt.Sprintf("th dport %d", rule.DestPort))
	}

	action := rule.Action
	if action == "" {
		action = "accept"
	}

	if len(conditions) > 0 {
		return fmt.Sprintf("%s %s", action, strings.Join(conditions, " "))
	}
	return action
}

// getChainName determines the chain name for a rule
func (osb *OptimizedScriptBuilder) getChainName(policy config.Policy, rule config.PolicyRule) string {
	// Create chain name based on policy direction
	// This is a simplified implementation - would need to match actual chain naming logic
	if policy.From == "internet" {
		return "input"
	} else if policy.To == "internet" {
		return "output"
	}
	return "forward"
}

// Helper functions

func (osb *OptimizedScriptBuilder) compareIPs(ip1, ip2 net.IP) int {
	ip1Int := osb.ipToUint32(ip1)
	ip2Int := osb.ipToUint32(ip2)

	if ip1Int < ip2Int {
		return -1
	} else if ip1Int > ip2Int {
		return 1
	}
	return 0
}

func (osb *OptimizedScriptBuilder) ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func (osb *OptimizedScriptBuilder) sortIPs(ips []string) []string {
	parsedIPs := make([]net.IP, 0, len(ips))
	for _, ipStr := range ips {
		if ip := net.ParseIP(ipStr); ip != nil && ip.To4() != nil {
			parsedIPs = append(parsedIPs, ip.To4())
		}
	}

	sort.Slice(parsedIPs, func(i, j int) bool {
		return osb.compareIPs(parsedIPs[i], parsedIPs[j]) < 0
	})

	result := make([]string, len(parsedIPs))
	for i, ip := range parsedIPs {
		result[i] = ip.String()
	}

	return result
}

func (osb *OptimizedScriptBuilder) removeDuplicateStrings(items []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
