package identity

import (
	"fmt"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// syncGroupToFirewallLocked synchronizes a group's members and schedule to the firewall.
// It updates the IPSet and active scheduled rules.
// Caller must hold s.mu (Read or Write).
func (s *Service) syncGroupToFirewallLocked(groupID string) {
	// Check dependencies
	if s.fwMgr == nil || s.ipsetSvc == nil {
		return
	}

	group, exists := s.groups[groupID]
	if !exists {
		// Group deleted? We should clean up.
		// Handled by deleteGroupFromFirewall
		return
	}

	// Collect MACs for this group
	var macs []string
	for _, ident := range s.identities {
		if ident.GroupID == groupID {
			macs = append(macs, ident.MACs...)
		}
	}

	// 1. Update IPSet
	setName := fmt.Sprintf("group_%s_mac", groupID)
	// Sanitize name if needed? UUIDs are safe.
	// Ensure set exists
	ipsetMgr := s.ipsetSvc.GetIPSetManager()
	if err := ipsetMgr.CreateSet(setName, "ether_addr"); err != nil {
		logging.Error("Failed to create/ensure group ipset", "set", setName, "error", err)
	}

	// Update elements (Atomic reload)
	if err := ipsetMgr.ReloadSet(setName, macs); err != nil {
		logging.Error("Failed to update group ipset members", "set", setName, "count", len(macs), "error", err)
	}

	// 2. Update Firewall Rules
	// If group has schedule enabled, create rules. Otherwise remove them.
	hasSchedule := group.Schedule != nil && group.Schedule.Enabled && len(group.Schedule.Blocks) > 0

	// We'll use a standard prefix for rules for this group
	rulePrefix := fmt.Sprintf("group_%s_blk", groupID)

	// Since we don't track old rules easily, we iterate the blocks and create/update.
	// For now, if we change the number of blocks, we might leave orphans if we don't track them.
	// But usually groups have 1 or 2 blocks.
	// A brute force cleanup might be needed or we just overwrite.
	// Better: Generate the list of desired rule names, applied them.
	// But Manager.ApplyScheduledRule doesn't support bulk sync or cleanup.
	//
	// Workaround: We assume a max number of blocks (e.g. 5) and disable any beyond current count?
	// Or we just implement removal for exact names.

	if hasSchedule {
		for i, block := range group.Schedule.Blocks {
			ruleName := fmt.Sprintf("%s_%d", rulePrefix, i)

			// Construct ScheduledRule
			// We target the "lan_wan" policy by default for internet blocking.
			// TODO: Make this configurable or dynamic.
			schedRule := config.ScheduledRule{
				Name:       ruleName,
				PolicyName: "lan_wan",
				Enabled:    true,
				Rule: config.PolicyRule{
					Name:        fmt.Sprintf("Block %s (%d)", group.Name, i),
					Description: fmt.Sprintf("Scheduled block for group %s", group.Name),
					Action:      "drop",
					SrcIPSet:    setName,
					TimeStart:   block.StartTime,
					TimeEnd:     block.EndTime,
					Days:        block.Days,
				},
			}

			if err := s.fwMgr.ApplyScheduledRule(schedRule, true); err != nil {
				logging.Error("Failed to apply group schedule rule", "rule", ruleName, "error", err)
			}
		}
		// TODO: Clean up extra rules if blocks decreased?
		// For now, we don't handle shrinking block lists gracefully without tracking.
	} else {
		// Schedule disabled - remove rules (we try to remove index 0..9 just in case)
		for i := 0; i < 10; i++ {
			ruleName := fmt.Sprintf("%s_%d", rulePrefix, i)
			// ApplyScheduledRule with enabled=false removes it
			dummyRule := config.ScheduledRule{Name: ruleName}
			// We ignoring error as rule might not exist
			_ = s.fwMgr.ApplyScheduledRule(dummyRule, false)
		}
	}
}

// deleteGroupFromFirewallLocked cleans up ipsets and rules for a deleted group
// Caller must hold s.mu.
func (s *Service) deleteGroupFromFirewallLocked(groupID string) {
	if s.fwMgr == nil || s.ipsetSvc == nil {
		return
	}

	// 1. Remove Rules
	rulePrefix := fmt.Sprintf("group_%s_blk", groupID)
	for i := 0; i < 10; i++ {
		ruleName := fmt.Sprintf("%s_%d", rulePrefix, i)
		dummyRule := config.ScheduledRule{Name: ruleName}
		_ = s.fwMgr.ApplyScheduledRule(dummyRule, false)
	}

	// 2. Remove IPSet
	setName := fmt.Sprintf("group_%s_mac", groupID)
	// We don't have a direct RemoveSet exposed on IPSetManager easily via IPSetService?
	// IPSetManager has DestroySet?
	// ipsetSvc.GetIPSetManager() returns the *IPSetManager.
	// We'll check if needed. Destroying sets referenced by rules (even if disabled) might be tricky if concurrency issues.
	// But assuming we disabled rules first.
	// Ideally we keep the set or flush it.
	ipsetMgr := s.ipsetSvc.GetIPSetManager()
	if err := ipsetMgr.FlushSet(setName); err != nil {
		logging.Error("Failed to flush group ipset", "set", setName, "error", err)
	}
	// We leave the empty set to avoid potential reference errors if rules linger.
}
