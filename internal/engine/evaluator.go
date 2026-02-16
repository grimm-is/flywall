// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package engine

import (
	"fmt"
	"strings"

	"grimm.is/flywall/internal/config"
)

type Verdict string

const (
	VerdictAccept Verdict = "accept"
	VerdictDrop   Verdict = "drop"
	VerdictReject Verdict = "reject" // Treated as drop usually
)

// RuleEngine evaluates packets against a configuration
type RuleEngine struct {
	Config *config.Config

	// Fast lookup maps (optional optimization, unnecessary for small simulations)
	// For simulator, we need to map Interfaces -> Zones to know which Policy chain to start.
	InterfaceToZone map[string]string
}

// NewRuleEngine creates a new evaluator
func NewRuleEngine(cfg *config.Config) *RuleEngine {
	e := &RuleEngine{
		Config:          cfg,
		InterfaceToZone: make(map[string]string),
	}
	e.buildLookups()
	return e
}

func (e *RuleEngine) buildLookups() {
	if e.Config == nil {
		return
	}
	for _, zone := range e.Config.Zones {
		for _, m := range zone.Matches {
			if m.Interface != "" {
				e.InterfaceToZone[m.Interface] = zone.Name
			}
		}
	}
}

// Evaluate determines the fate of a packet
// Returns verdict and the RuleID (or PolicyName if default action) that decided it
func (e *RuleEngine) Evaluate(pkt Packet) (Verdict, string) {
	if e.Config == nil {
		return VerdictAccept, "no-config" // Open by default if no config? Or Drop? SafeMode=Open usually
	}

	// 1. Identify Source Zone
	srcZone := e.InterfaceToZone[pkt.InInterface]
	if srcZone == "" {
		// If interface not in any zone, default to "drop" or "wan" behavior?
		// Flywall usually requires interfaces assigned to zones.
		// For simulation, if InInterface is empty (e.g. locally generated?), maybe "mgmt"?
		// Let's assume assume "wan" if unknown or handle as strict drop.
		// Actually, default policy is implicit drop usually.
		return VerdictDrop, "unknown-zone"
	}

	// 2. Identify Destination Zone
	// This is TRICKY in simulation without a routing table.
	// We know DstIP but not which Interface/Zone it belongs to.
	// We need a helper to resolve IP -> Zone.
	// For now, we simulate "Forwarding" based on Policies.
	// We iterate ALL policies matching From=srcZone.
	// If rule matches, we assume destination is valid (or we implicitly discovered dstZone).

	// Alternative: Identify DstZone by checking if DstIP belongs to any interface subnet.
	// This requires Config to have subnets, which it might not have explicitly (it uses Interfaces).
	// Simulator might need to know "Local IPs".

	// Simplified Approach: Iterate ALL policies where From == srcZone.
	// This matches how nftables chains work (chain zone_lan_forward { ... }).

	for _, policy := range e.Config.Policies {
		if policy.From != srcZone {
			continue
		}

		// Evaluate Rules in this policy
		// We do NOT check policy.To yet, because in nftables we jump to policy chains based on direction.
		// Actually, in `script_builder.go`, we dispatch by Input vs Forward.
		// If DstIP is local, it's Input. If not, it's Forward.

		// For simpler simulation, we just match rules. If rule matches, we take action.
		// Note: Config rules usually enforce Source AND Dest zone.
		// But in PolicyRule, SrcZone/DestZone are optional overrides.
		// The Policy itself defines From/To.

		// If logic strictly requires knowing DestZone, we are stuck without routing.
		// BUT usually rules match on DstIP/Port.
		// If a rule matches IP/Port, we obey it regardless of precise zone boundary in this simple sim.
		// Exception: If we have multiple policies from same zone (e.g. LAN->WAN vs LAN->DMZ).
		// Rules in LAN->WAN shouldn't match LAN->DMZ traffic if defined by IP.

		// Heuristic: If we match the rule's criteria, we follow it.
		// If Policy has "To" zone, ideally we check if packet is destined there.
		// But we match Rules. Rules often specify DestIP.
		// So iterating all policies from SrcZone is reasonable. order by Priority?
		// Config.Policies are a flat list. Order matters if priority used?
		// Flywall `policy.go`: "Rules are evaluated in order - first match wins" (within policy).
		// But between policies?
		// `manager_linux.go` sorts/groups them.

		// For MVP: We just iterate all rules in all matching source-policies.
		// (This is inexact but better than nothing).

		effectiveRules := policy.GetEffectiveRules(e.Config.Policies)
		for _, rule := range effectiveRules {
			if Match(rule, pkt) {
				return Verdict(strings.ToLower(rule.Action)), fmt.Sprintf("rule:%s:%s", policy.Name, rule.Name)
			}
		}

		// If no rule matches, check Policy Default Action?
		// Only if we determine the packet strictly belongs to this Policy.
		// But since we can't be sure of DestZone, applying Policy Action is risky.
		// E.g. LAN->WAN (allow) vs LAN->LAN (allow).
		// If we are iterating policies, and fall through rules, do we accept?
		// Usually we fallback to global drop.
	}

	return VerdictDrop, "default-drop"
}
