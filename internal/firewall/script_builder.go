// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

// ScriptBuilder builds nftables scripts for atomic application.
// It manages the construction of tables, chains, rules, sets, maps, and other
// objects, ensuring they are output in the correct order for `nft -f`.
// This is crucial because nftables requires objects to be defined before
// they are referenced.
type ScriptBuilder struct {
	tableName  string
	family     string
	timezone   string              // Timezone for time-based rules
	lines      []string            // Raw lines (comments, flush commands)
	tables     []string            // Table definitions
	chains     []string            // Chain definitions
	flowtables []string            // Flowtable definitions
	rules      map[string][]string // Rules keyed by chain name (to keep them grouped)
	sets       []string            // Set definitions
	maps       []string            // Map definitions
	counters   []string            // Counter definitions
	chainOrder []string            // Order of chains to output (preserving addition order)

	// Optimization settings
	optimizeEnabled bool
	optimizeLevel   int // 0=none, 1=basic, 2=advanced
	setCounter      int
}

// NewScriptBuilder creates a new script builder for a specific table and family.
// Common families are "inet" (IPv4+IPv6), "ip" (IPv4), "ip6" (IPv6), and "netdev" (ingress).
func NewScriptBuilder(tableName, family, timezone string) *ScriptBuilder {
	return &ScriptBuilder{
		tableName:       tableName,
		family:          family,
		timezone:        timezone,
		rules:           make(map[string][]string),
		optimizeEnabled: true,
		optimizeLevel:   2, // Advanced optimization by default
	}
}

// SetOptimizationLevel configures the optimization level (0=none, 1=basic, 2=advanced)
func (sb *ScriptBuilder) SetOptimizationLevel(level int) {
	sb.optimizeLevel = level
}

// SetOptimizationEnabled enables or disables rule optimization
func (sb *ScriptBuilder) SetOptimizationEnabled(enabled bool) {
	sb.optimizeEnabled = enabled
}

func (sb *ScriptBuilder) AddLine(line string) {
	sb.lines = append(sb.lines, line)
}

func (sb *ScriptBuilder) AddTable() {
	sb.tables = append(sb.tables, fmt.Sprintf("add table %s %s", sb.family, quote(sb.tableName)))
}

func (sb *ScriptBuilder) AddTableWithComment(comment string) {
	sb.tables = append(sb.tables,
		fmt.Sprintf("add table %s %s { comment %q; }",
			sb.family, quote(sb.tableName), comment))
}

func (sb *ScriptBuilder) AddChain(name, typeName, hook string, priority int, policy string, comment ...string) {
	var cmd string
	if typeName != "" {
		cmd = fmt.Sprintf("add chain %s %s %s { type %s hook %s priority %d; policy %s;",
			sb.family, quote(sb.tableName), quote(name), typeName, hook, priority, policy)
	} else {
		cmd = fmt.Sprintf("add chain %s %s %s {", sb.family, quote(sb.tableName), quote(name))
	}

	if len(comment) > 0 {
		cmd += fmt.Sprintf(" comment %q;", comment[0])
	}
	cmd += " }"
	sb.chains = append(sb.chains, cmd)
	sb.chainOrder = append(sb.chainOrder, name)
}

func (sb *ScriptBuilder) AddRule(chain, rule string, comment ...string) {
	// If rule already has a comment, don't add another one
	if (len(comment) > 0 && comment[0] != "") && !strings.Contains(rule, "comment \"") {
		rule += fmt.Sprintf(" comment %q", comment[0])
	}
	cmd := fmt.Sprintf("add rule %s %s %s %s",
		sb.family, quote(sb.tableName), quote(chain), rule)
	sb.rules[chain] = append(sb.rules[chain], cmd)
}

func (sb *ScriptBuilder) AddSet(name, setType, comment string, size int, flags ...string) {
	typeKeyword := "type"
	if strings.Contains(setType, " ") || strings.Contains(setType, ".") {
		typeKeyword = "typeof"
	}

	def := fmt.Sprintf("add set %s %s %s { %s %s;",
		sb.family, sb.tableName, quote(name), typeKeyword, setType)
	if len(flags) > 0 {
		def += fmt.Sprintf(" flags %s;", strings.Join(flags, ","))
	}
	if size > 0 {
		def += fmt.Sprintf(" size %d;", size)
	}
	if comment != "" {
		def += fmt.Sprintf(" comment %q;", comment)
	}
	def += " }"
	sb.sets = append(sb.sets, def)
}

func (sb *ScriptBuilder) AddSetElements(setName string, elements []string) {
	// Add elements 100 at a time to avoid huge command lines
	batchSize := 100
	for i := 0; i < len(elements); i += batchSize {
		end := i + batchSize
		if end > len(elements) {
			end = len(elements)
		}

		chunk := elements[i:end]
		// Quote elements if necessary? Usually passed correctly formatted.
		// If elements contain spaces (comments?), nft might parse weirdly.
		// Assuming caller handles basic formatting, but we might verify.
		sb.lines = append(sb.lines, fmt.Sprintf("add element %s %s %s { %s }",
			sb.family, sb.tableName, quote(setName), strings.Join(chunk, ", ")))
	}
}

func (sb *ScriptBuilder) AddMap(name, keyType, valueType, comment string, flags []string, elements []string) {
	def := fmt.Sprintf("add map %s %s %s { type %s : %s;",
		sb.family, sb.tableName, quote(name), keyType, valueType)
	if len(flags) > 0 {
		def += fmt.Sprintf(" flags %s;", strings.Join(flags, ","))
	}
	if comment != "" {
		def += fmt.Sprintf(" comment %q;", comment)
	}
	if len(elements) > 0 {
		def += fmt.Sprintf(" elements = { %s };", strings.Join(elements, ", "))
	}
	def += " }"
	sb.maps = append(sb.maps, def)
}

func (sb *ScriptBuilder) AddCounter(name, comment string) {
	// Note: nftables counter objects do not support the comment clause.
	// Comments are silently ignored here but kept in the signature for doc purposes.
	_ = comment
	def := fmt.Sprintf("add counter %s %s %s", sb.family, sb.tableName, quote(name))
	sb.counters = append(sb.counters, def)
}

func (sb *ScriptBuilder) AddFlowtable(name string, devices []string, comment ...string) {
	// flowtable ft { hook ingress priority 0; devices = { ... }; }
	// Need to force quote devices
	var qDevices []string
	for _, d := range devices {
		qDevices = append(qDevices, fmt.Sprintf("%q", d))
	}
	def := fmt.Sprintf("add flowtable %s %s %s { hook ingress priority 0; devices = { %s };",
		sb.family, sb.tableName, name, strings.Join(qDevices, ", "))
	if len(comment) > 0 {
		def += fmt.Sprintf(" comment %q;", comment[0])
	}
	def += " }"
	sb.flowtables = append(sb.flowtables, def)
}

// Build assembles the complete nftables script with optional optimization.
// The order of operations is critical for nftables:
// 1. Tables (container)
// 2. Sets (used by rules)
// 3. Counters (used by rules)
// 4. Flowtables (used by rules)
// 5. Chains (contain rules)
// 6. Maps (may reference chains)
// 7. Rules (inside chains) - OPTIMIZED HERE
// 8. Elements (populate sets/maps)
func (sb *ScriptBuilder) Build() string {
	// Apply optimization if enabled before building
	if sb.optimizeEnabled && sb.optimizeLevel > 0 {
		sb.optimizeRules()
	}

	var lines []string

	// Comments/Header provided by caller mostly, but tables first
	lines = append(lines, sb.tables...)

	// Sets
	lines = append(lines, sb.sets...)

	// Counters
	lines = append(lines, sb.counters...)

	// Flowtables
	lines = append(lines, sb.flowtables...)

	// Chains (must be before maps that reference them via jump)
	lines = append(lines, sb.chains...)

	// Flush chains before adding rules (idempotency for Smart Flush)
	// This ensures repeated ApplyConfig calls don't duplicate rules.
	// "add chain" is a no-op for existing chains, but "add rule" appends.
	for _, chain := range sb.chainOrder {
		lines = append(lines, fmt.Sprintf("flush chain %s %s %s",
			sb.family, quote(sb.tableName), quote(chain)))
	}

	// Maps (after chains, since maps may contain jump verdicts)
	lines = append(lines, sb.maps...)

	// Rules (in chain order)
	// We defined specific chain addition order logic: we store chain names in chainOrder.
	// But what if chains added out of order? The caller calls AddChain.
	// We iterate sb.chainOrder to output rules for each chain.
	for _, chain := range sb.chainOrder {
		if rules, ok := sb.rules[chain]; ok {
			lines = append(lines, rules...)
		}
	}

	// Lines (Flush commands, element additions)
	// Usually placed after sets defined.
	// Wait, sets defs are separate from lines?
	// AddLine is generic. AddSetElements appends to AddLine.
	// We should probably interleave correctly.
	// Current impl: AddLine logic is simplistic.
	// If AddSetElements called, it appends to sb.lines.
	// Let's output sb.lines after sets/maps but before chains?
	// Or after chains? Element addition works any time after set exists.
	// Outputting after defines sets is safest.
	lines = append(lines, sb.lines...)

	return strings.Join(lines, "\n") + "\n"
}

// optimizeRules applies rule optimization based on the configured level
func (sb *ScriptBuilder) optimizeRules() {
	for chainName, rules := range sb.rules {
		if len(rules) < 3 {
			continue // Not enough rules to optimize
		}

		// Group rules by pattern for optimization
		groups := sb.groupRulesByPattern(rules)

		var optimizedRules []string

		for _, group := range groups {
			if len(group) >= 3 && sb.canOptimizeGroup(group) {
				// Create optimized rule for this group
				optimizedRule := sb.createOptimizedRule(group)
				optimizedRules = append(optimizedRules, optimizedRule)
			} else {
				// Keep rules as-is
				optimizedRules = append(optimizedRules, group...)
			}
		}

		sb.rules[chainName] = optimizedRules
	}
}

// groupRulesByPattern groups rules that can be optimized together
func (sb *ScriptBuilder) groupRulesByPattern(rules []string) [][]string {
	var groups [][]string
	used := make([]bool, len(rules))

	for i, rule := range rules {
		if used[i] {
			continue
		}

		group := []string{rule}
		used[i] = true

		// Find similar rules
		for j := i + 1; j < len(rules); j++ {
			if used[j] {
				continue
			}

			if sb.rulesAreSimilar(rule, rules[j]) {
				group = append(group, rules[j])
				used[j] = true
			}
		}

		groups = append(groups, group)
	}

	return groups
}

// rulesAreSimilar checks if two rules have similar patterns for optimization
func (sb *ScriptBuilder) rulesAreSimilar(rule1, rule2 string) bool {
	// Extract key components from rules
	// This is a simplified check - in practice, you'd parse the rule structure
	r1Parts := strings.Fields(rule1)
	r2Parts := strings.Fields(rule2)

	// Rules are similar if they have the same action and protocol
	if len(r1Parts) < 3 || len(r2Parts) < 3 {
		return false
	}

	return r1Parts[0] == r2Parts[0] && // Same action
		((len(r1Parts) > 3 && len(r2Parts) > 3 && r1Parts[2] == r2Parts[2]) || // Same protocol
			(len(r1Parts) <= 3 && len(r2Parts) <= 3)) // No protocol specified
}

// canOptimizeGroup checks if a group of rules can be optimized
func (sb *ScriptBuilder) canOptimizeGroup(rules []string) bool {
	// Count unique source IPs in the group
	srcIPs := make(map[string]bool)

	for _, rule := range rules {
		parts := strings.Fields(rule)
		for i, part := range parts {
			if part == "saddr" && i+1 < len(parts) {
				ip := strings.Trim(parts[i+1], "\"'")
				if !strings.HasPrefix(ip, "@") { // Not already a set
					srcIPs[ip] = true
				}
				break
			}
		}
	}

	// Can optimize if we have 3+ unique source IPs
	return len(srcIPs) >= 3
}

// createOptimizedRule creates an optimized rule from a group of similar rules
func (sb *ScriptBuilder) createOptimizedRule(rules []string) string {
	// Extract common pattern
	action := strings.Fields(rules[0])[0]

	// Collect all source IPs
	var srcIPs []string
	for _, rule := range rules {
		parts := strings.Fields(rule)
		for i, part := range parts {
			if part == "saddr" && i+1 < len(parts) {
				ip := strings.Trim(parts[i+1], "\"'")
				if !strings.HasPrefix(ip, "@") {
					srcIPs = append(srcIPs, ip)
				}
				break
			}
		}
	}

	// Remove duplicates
	srcIPs = sb.removeDuplicateStrings(srcIPs)

	// Optimize IPs if advanced level
	if sb.optimizeLevel >= 2 {
		srcIPs = sb.mergeAdjacentIPs(srcIPs)
	}

	// Create IP set
	sb.setCounter++
	setName := fmt.Sprintf("opt_src_%d", sb.setCounter)

	// Add set definition
	sb.AddSet(setName, "ipv4_addr", "Optimized source IP set", len(srcIPs)*2)

	// Add elements
	sb.AddSetElements(setName, srcIPs)

	// Build rule with set reference
	var conditions []string
	for _, rule := range rules {
		parts := strings.Fields(rule)
		// Extract non-IP conditions
		for i := 1; i < len(parts); i++ {
			if parts[i] == "saddr" {
				i++ // Skip IP
				continue
			}
			conditions = append(conditions, parts[i])
		}
		break // Use first rule's conditions
	}

	// Add set reference
	conditions = append(conditions, fmt.Sprintf("saddr @%s", setName))

	return fmt.Sprintf("%s %s", action, strings.Join(conditions, " "))
}

// mergeAdjacentIPs merges adjacent IP ranges into CIDR blocks
func (sb *ScriptBuilder) mergeAdjacentIPs(ips []string) []string {
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
		return sb.compareIPs(parsedIPs[i], parsedIPs[j]) < 0
	})

	// Merge into CIDR blocks
	var result []string
	i := 0

	for i < len(parsedIPs) {
		// Try to create the largest possible CIDR block
		maxSize := 32
		for maxSize > 0 {
			mask := net.CIDRMask(32-maxSize, 32)
			network := &net.IPNet{
				IP:   parsedIPs[i],
				Mask: mask,
			}

			// Check if consecutive IPs fit
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
				result = append(result, network.String())
				i = end
				break
			}
			maxSize--
		}

		if maxSize == 0 {
			result = append(result, parsedIPs[i].String())
			i++
		}
	}

	return result
}

// Helper functions for optimization

func (sb *ScriptBuilder) compareIPs(ip1, ip2 net.IP) int {
	ip1Int := sb.ipToUint32(ip1)
	ip2Int := sb.ipToUint32(ip2)

	if ip1Int < ip2Int {
		return -1
	} else if ip1Int > ip2Int {
		return 1
	}
	return 0
}

func (sb *ScriptBuilder) ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func (sb *ScriptBuilder) removeDuplicateStrings(items []string) []string {
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

// String returns the script for debugging.
func (b *ScriptBuilder) String() string {
	return b.Build()
}
