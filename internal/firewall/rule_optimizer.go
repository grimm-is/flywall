// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"grimm.is/flywall/internal/config"
)

// RuleOptimizer optimizes nftables rules for better performance
type RuleOptimizer struct {
	maxRulesPerChain int
	mergeThreshold   int // Minimum number of IPs to consider merging
}

// OptimizedRuleSet contains optimized rules, sets, and maps
type OptimizedRuleSet struct {
	Chains map[string][]string
	Sets   map[string]*OptimizedSet
	Maps   map[string]string
}

// OptimizedSet represents an optimized IP set with merged ranges
type OptimizedSet struct {
	Name     string
	Type     string
	Elements []string
	Comment  string
	Size     int
}

// NewRuleOptimizer creates a new rule optimizer
func NewRuleOptimizer() *RuleOptimizer {
	return &RuleOptimizer{
		maxRulesPerChain: 1000,
		mergeThreshold:   4, // Merge at least 4 IPs into ranges
	}
}

// OptimizePolicyRules optimizes policy rules for better nftables performance
func (ro *RuleOptimizer) OptimizePolicyRules(policies []config.Policy) (*OptimizedRuleSet, error) {
	optimized := &OptimizedRuleSet{
		Chains: make(map[string][]string),
		Sets:   make(map[string]*OptimizedSet),
		Maps:   make(map[string]string),
	}

	// Group rules by common patterns for optimization
	ruleGroups := ro.groupRulesByPattern(policies)

	// Process each group
	for groupKey, group := range ruleGroups {
		if len(group.rules) >= ro.mergeThreshold {
			// Create optimized set for this group
			set := ro.createOptimizedSet(groupKey, group)
			optimized.Sets[groupKey] = set

			// Create single rule using the set
			rule := ro.createSetBasedRule(group, set.Name)
			chainName := ro.getChainName(group.rules[0])
			optimized.Chains[chainName] = append(optimized.Chains[chainName], rule)
		} else {
			// Add rules as-is
			for _, rule := range group.rules {
				nftRule := ro.convertToNFTRule(rule)
				chainName := ro.getChainName(rule)
				optimized.Chains[chainName] = append(optimized.Chains[chainName], nftRule)
			}
		}
	}

	return optimized, nil
}

// RuleGroup represents a group of similar rules
type RuleGroup struct {
	rules   []config.PolicyRule
	action  string
	proto   string
	dstPort int
}

// groupRulesByPattern groups rules that can be optimized together
func (ro *RuleOptimizer) groupRulesByPattern(policies []config.Policy) map[string]*RuleGroup {
	groups := make(map[string]*RuleGroup)
	setCounter := 0

	for _, policy := range policies {
		for _, rule := range policy.Rules {
			// Skip rules that don't fit optimization patterns
			if !ro.canOptimize(rule) {
				continue
			}

			// Create group key based on optimizable patterns
			key := ro.createGroupKey(rule, &setCounter)

			if _, exists := groups[key]; !exists {
				groups[key] = &RuleGroup{
					action:  rule.Action,
					proto:   rule.Protocol,
					dstPort: rule.DestPort,
				}
			}

			groups[key].rules = append(groups[key].rules, rule)
		}
	}

	return groups
}

// canOptimize checks if a rule can be optimized
func (ro *RuleOptimizer) canOptimize(rule config.PolicyRule) bool {
	// Can optimize if:
	// - Has source IPs or IP sets
	// - Has consistent action and protocol
	// - Doesn't have complex match conditions
	hasSrcMatch := rule.SrcIP != "" || rule.SrcIPSet != ""
	hasSimpleMatch := rule.ConnState == "" && rule.SourceCountry == "" &&
		rule.TCPFlags == "" && rule.TimeStart == ""

	return hasSrcMatch && hasSimpleMatch && rule.Action != ""
}

// createGroupKey creates a unique key for grouping similar rules
func (ro *RuleOptimizer) createGroupKey(rule config.PolicyRule, counter *int) string {
	// Use action, protocol, and port as grouping criteria
	key := fmt.Sprintf("%s_%s_%d", rule.Action, rule.Protocol, rule.DestPort)

	return key
}

// createOptimizedSet creates an optimized set from a rule group
func (ro *RuleOptimizer) createOptimizedSet(groupKey string, group *RuleGroup) *OptimizedSet {
	// Collect all source IPs from the group
	var ips []string
	for _, rule := range group.rules {
		if rule.SrcIP != "" {
			ips = append(ips, rule.SrcIP)
		}
		// TODO: Handle IP sets - would need to resolve set elements
	}

	// Optimize IP addresses by merging into CIDR ranges
	optimizedIPs := ro.optimizeIPAddresses(ips)

	return &OptimizedSet{
		Name:     fmt.Sprintf("opt_%s", groupKey),
		Type:     "ipv4_addr",
		Elements: optimizedIPs,
		Comment:  fmt.Sprintf("Optimized set for %s/%s port %d", group.action, group.proto, group.dstPort),
		Size:     len(optimizedIPs) * 2, // Estimate size
	}
}

// optimizeIPAddresses merges individual IPs into CIDR blocks
func (ro *RuleOptimizer) optimizeIPAddresses(ips []string) []string {
	if len(ips) < ro.mergeThreshold {
		return ips
	}

	// Parse and sort IPs
	var parsedIPs []net.IP
	for _, ipStr := range ips {
		if ip := net.ParseIP(ipStr); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				parsedIPs = append(parsedIPs, ip4)
			}
		}
	}

	if len(parsedIPs) < 2 {
		return ips
	}

	// Sort IPs
	sort.Slice(parsedIPs, func(i, j int) bool {
		return ipToInt(parsedIPs[i]) < ipToInt(parsedIPs[j])
	})

	// Merge into CIDR blocks
	var result []string
	i := 0

	for i < len(parsedIPs) {
		// Try to create the largest possible CIDR block starting at i
		maxSize := 32
		for maxSize > 0 {
			mask := net.CIDRMask(32-maxSize, 32)
			network := &net.IPNet{
				IP:   parsedIPs[i],
				Mask: mask,
			}

			// Check if all consecutive IPs fit in this network
			fits := true
			end := i
			for end < len(parsedIPs) && end-i < (1<<maxSize) {
				if !network.Contains(parsedIPs[end]) {
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
			result = append(result, parsedIPs[i].String())
			i++
		}
	}

	return result
}

// createSetBasedRule creates a single nftables rule using an optimized set
func (ro *RuleOptimizer) createSetBasedRule(group *RuleGroup, setName string) string {
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
	rule := fmt.Sprintf("%s %s", group.action, strings.Join(conditions, " "))

	return rule
}

// convertToNFTRule converts a single rule to nftables format
func (ro *RuleOptimizer) convertToNFTRule(rule config.PolicyRule) string {
	var conditions []string

	if rule.Protocol != "" {
		conditions = append(conditions, fmt.Sprintf("ip protocol %s", rule.Protocol))
	}
	if rule.SrcIP != "" {
		conditions = append(conditions, fmt.Sprintf("ip saddr %s", rule.SrcIP))
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
func (ro *RuleOptimizer) getChainName(rule config.PolicyRule) string {
	// This would need to be adapted based on actual chain naming logic
	return "forward"
}

// ipToInt converts an IPv4 address to a 32-bit integer
func ipToInt(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// ApplyOptimizations applies the optimized rules to a ScriptBuilder
func (ro *RuleOptimizer) ApplyOptimizations(sb *ScriptBuilder, optimized *OptimizedRuleSet) {
	// Add all optimized sets first
	for _, set := range optimized.Sets {
		sb.AddSet(set.Name, set.Type, set.Comment, set.Size)
		if len(set.Elements) > 0 {
			sb.AddSetElements(set.Name, set.Elements)
		}
	}

	// Add optimized rules to chains
	for chainName, rules := range optimized.Chains {
		for _, rule := range rules {
			sb.AddRule(chainName, rule)
		}
	}
}
