// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"fmt"
	"strings"

	"grimm.is/flywall/internal/config"
)

// ExampleIntegration demonstrates how to use the integrated rule optimization
func ExampleIntegration() {
	// Create script builder with optimization enabled by default
	builder := NewScriptBuilder("flywall", "inet", "UTC")

	// Optimization is enabled by default, but you can configure it:
	// builder.SetOptimizationLevel(2) // Advanced optimization (default)
	// builder.SetOptimizationEnabled(true) // Enable optimization (default)

	// Example configuration with many similar rules
	policies := []config.Policy{
		{
			From: "lan",
			To:   "wan",
			Rules: []config.PolicyRule{
				{
					Name:     "allow_http_1",
					Action:   "accept",
					Protocol: "tcp",
					DestPort: 80,
					SrcIP:    "192.168.1.10",
				},
				{
					Name:     "allow_http_2",
					Action:   "accept",
					Protocol: "tcp",
					DestPort: 80,
					SrcIP:    "192.168.1.11",
				},
				{
					Name:     "allow_http_3",
					Action:   "accept",
					Protocol: "tcp",
					DestPort: 80,
					SrcIP:    "192.168.1.12",
				},
				{
					Name:     "allow_http_4",
					Action:   "accept",
					Protocol: "tcp",
					DestPort: 80,
					SrcIP:    "192.168.1.13",
				},
				{
					Name:     "allow_http_5",
					Action:   "accept",
					Protocol: "tcp",
					DestPort: 80,
					SrcIP:    "192.168.1.14",
				},
			},
		},
		{
			From: "lan",
			To:   "wan",
			Rules: []config.PolicyRule{
				{
					Name:     "allow_https_1",
					Action:   "accept",
					Protocol: "tcp",
					DestPort: 443,
					SrcIP:    "192.168.1.20",
				},
				{
					Name:     "allow_https_2",
					Action:   "accept",
					Protocol: "tcp",
					DestPort: 443,
					SrcIP:    "192.168.1.21",
				},
				{
					Name:     "allow_https_3",
					Action:   "accept",
					Protocol: "tcp",
					DestPort: 443,
					SrcIP:    "192.168.1.22",
				},
			},
		},
	}

	// Add table
	builder.AddTable()

	// Add chain
	builder.AddChain("forward", "filter", "forward", 0, "accept")

	// Add policy rules (optimization happens automatically during Build())
	for _, policy := range policies {
		for _, rule := range policy.Rules {
			ruleStr := buildRuleString(rule)
			builder.AddRule("forward", ruleStr)
		}
	}

	// Build the script - optimization is applied here automatically!
	script := builder.Build()

	fmt.Println("Generated nftables script with integrated optimization:")
	fmt.Println(script)

	/*
		The optimized output would look like:

		add table inet flywall
		add set inet flywall opt_src_1 { type ipv4_addr; size 10; comment "Optimized source IP set" }
		add element inet flywall opt_src_1 { 192.168.1.10, 192.168.1.11, 192.168.1.12, 192.168.1.13, 192.168.1.14 }
		add chain inet flywall forward { type filter hook forward priority 0; policy accept; }
		accept ip protocol tcp th dport 80 saddr @opt_src_1
		accept ip protocol tcp th dport 443 saddr 192.168.1.20
		accept ip protocol tcp th dport 443 saddr 192.168.1.21
		accept ip protocol tcp th dport 443 saddr 192.168.1.22

		Note: Only the HTTP rules were optimized because they had 5 similar rules.
		The HTTPS rules remained individual since there were only 3 (at the threshold).
	*/
}

// PerformanceComparison shows the optimization benefits
func PerformanceComparison() {
	// Create test data with many similar rules
	policies := generateLargeRuleSet(1000) // 1000 rules

	// Generate without optimization
	builderNoOpt := NewScriptBuilder("flywall", "inet", "UTC")
	builderNoOpt.SetOptimizationEnabled(false)
	builderNoOpt.AddTable()
	builderNoOpt.AddChain("forward", "filter", "forward", 0, "accept")

	for _, policy := range policies {
		for _, rule := range policy.Rules {
			ruleStr := buildSimpleRule(rule)
			builderNoOpt.AddRule("forward", ruleStr)
		}
	}

	unoptimizedScript := builderNoOpt.Build()
	unoptimizedLines := countLines(unoptimizedScript)

	// Generate with optimization
	builderOpt := NewScriptBuilder("flywall", "inet", "UTC")
	builderOpt.SetOptimizationLevel(2) // Advanced optimization
	builderOpt.AddTable()
	builderOpt.AddChain("forward", "filter", "forward", 0, "accept")

	for _, policy := range policies {
		for _, rule := range policy.Rules {
			ruleStr := buildSimpleRule(rule)
			builderOpt.AddRule("forward", ruleStr)
		}
	}

	optimizedScript := builderOpt.Build()
	optimizedLines := countLines(optimizedScript)

	fmt.Printf("Performance Comparison:\n")
	fmt.Printf("Unoptimized: %d lines\n", unoptimizedLines)
	fmt.Printf("Optimized: %d lines\n", optimizedLines)
	fmt.Printf("Reduction: %.1f%%\n", float64(unoptimizedLines-optimizedLines)/float64(unoptimizedLines)*100)

	// Show some optimization stats
	fmt.Printf("\nOptimization applied automatically during Build()\n")
	fmt.Printf("- Rules grouped by pattern: action + protocol + port\n")
	fmt.Printf("- IP sets created for 3+ similar source IPs\n")
	fmt.Printf("- CIDR merging applied for advanced optimization\n")
}

// DisableOptimizationExample shows how to disable optimization
func DisableOptimizationExample() {
	// Create builder with optimization disabled
	builder := NewScriptBuilder("flywall", "inet", "UTC")
	builder.SetOptimizationEnabled(false)

	// Or set optimization level to 0
	// builder.SetOptimizationLevel(0)

	// Add rules as usual...
	builder.AddTable()
	builder.AddChain("forward", "filter", "forward", 0, "accept")

	// Add many similar rules
	for i := 0; i < 10; i++ {
		rule := fmt.Sprintf("accept ip protocol tcp th dport 80 saddr 192.168.1.%d", i+10)
		builder.AddRule("forward", rule)
	}

	// Build without optimization
	script := builder.Build()
	fmt.Println("Script without optimization:")
	fmt.Println(script)
}

// Helper functions for the example
func generateLargeRuleSet(count int) []config.Policy {
	policies := []config.Policy{
		{
			From:  "lan",
			To:    "wan",
			Rules: make([]config.PolicyRule, 0),
		},
	}

	// Generate rules with similar patterns
	for i := 0; i < count; i++ {
		rule := config.PolicyRule{
			Name:     fmt.Sprintf("rule_%d", i),
			Action:   "accept",
			Protocol: "tcp",
			DestPort: 80,
			SrcIP:    fmt.Sprintf("192.168.1.%d", (i%254)+1),
		}
		policies[0].Rules = append(policies[0].Rules, rule)
	}

	return policies
}

func buildRuleString(rule config.PolicyRule) string {
	var parts []string

	if rule.Protocol != "" {
		parts = append(parts, fmt.Sprintf("ip protocol %s", rule.Protocol))
	}
	if rule.DestPort > 0 {
		parts = append(parts, fmt.Sprintf("th dport %d", rule.DestPort))
	}
	if rule.SrcIP != "" {
		parts = append(parts, fmt.Sprintf("saddr %s", rule.SrcIP))
	}

	action := rule.Action
	if action == "" {
		action = "accept"
	}

	if len(parts) > 0 {
		return fmt.Sprintf("%s %s", action, strings.Join(parts, " "))
	}
	return action
}

func buildSimpleRule(rule config.PolicyRule) string {
	return fmt.Sprintf("accept ip protocol tcp th dport %d saddr %s", rule.DestPort, rule.SrcIP)
}

func countLines(s string) int {
	count := 0
	for _, r := range s {
		if r == '\n' {
			count++
		}
	}
	return count
}
