//go:build linux
// +build linux

package kernel

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

// LinuxKernel implements Kernel using real Linux system calls and the google/nftables library.
type LinuxKernel struct {
	tableName string
	mu        sync.RWMutex
}

// NewLinuxKernel creates a new Linux kernel provider.
func NewLinuxKernel(tableName string) *LinuxKernel {
	if tableName == "" {
		tableName = "flywall"
	}
	return &LinuxKernel{tableName: tableName}
}

// Now returns the current system time.
func (k *LinuxKernel) Now() time.Time {
	return time.Now()
}

// DumpFlows returns all active conntrack flows.
// TODO: Implement using /proc/net/nf_conntrack or conntrack CLI.
func (k *LinuxKernel) DumpFlows() ([]Flow, error) {
	// Placeholder - would parse conntrack entries
	return nil, nil
}

// GetFlow retrieves a specific flow by ID.
func (k *LinuxKernel) GetFlow(id string) (Flow, bool) {
	// Placeholder - lookup in conntrack
	return Flow{}, false
}

// KillFlow removes a flow from conntrack.
func (k *LinuxKernel) KillFlow(id string) error {
	// Placeholder - would use conntrack -D
	return nil
}

// AddBlock adds an IP to a blocklist set.
func (k *LinuxKernel) AddBlock(ip string) error {
	// Placeholder - would use nft add element
	return nil
}

// RemoveBlock removes an IP from a blocklist set.
func (k *LinuxKernel) RemoveBlock(ip string) error {
	// Placeholder - would use nft delete element
	return nil
}

// IsBlocked checks if an IP is in the blocklist.
func (k *LinuxKernel) IsBlocked(ip string) bool {
	// Placeholder - would query nft set
	return false
}

// GetCounters returns named counter statistics from nftables.
// Uses google/nftables library for native netlink access.
func (k *LinuxKernel) GetCounters() (map[string]uint64, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	conn, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create nftables connection: %w", err)
	}

	// Get all tables to find our target table
	tables, err := conn.ListTables()
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	var targetTable *nftables.Table
	for _, t := range tables {
		if t.Name == k.tableName && t.Family == nftables.TableFamilyINet {
			targetTable = t
			break
		}
	}

	if targetTable == nil {
		// Table doesn't exist yet, return empty counters
		return make(map[string]uint64), nil
	}

	// Get all chains in the table
	chains, err := conn.ListChains()
	if err != nil {
		return nil, fmt.Errorf("failed to list chains: %w", err)
	}

	counters := make(map[string]uint64)

	// Iterate through chains to find counter rules
	for _, chain := range chains {
		if chain.Table.Name != k.tableName || chain.Table.Family != nftables.TableFamilyINet {
			continue
		}

		rules, err := conn.GetRules(targetTable, chain)
		if err != nil {
			continue // Skip chains we can't read
		}

		for _, rule := range rules {
			// Look for named counters in rule expressions
			for _, e := range rule.Exprs {
				if counter, ok := e.(*expr.Counter); ok {
					// Named counters have a name in the counter expression
					// The counter name is stored separately - we need to extract it from rule context
					// For rules with "counter name X", the name is in the rule UserData or comment
					if len(rule.UserData) > 0 {
						counters[string(rule.UserData)] = counter.Packets
					}
				}
			}
		}
	}

	// Also try to get named counters from the flywall_stats chain specifically
	counters = k.getNamedCounters(conn, targetTable, counters)

	return counters, nil
}

// getNamedCounters extracts named counter values from the flywall_stats chain.
// Named counters (cnt_syn, cnt_rst, etc.) are referenced in rules as "counter name X".
func (k *LinuxKernel) getNamedCounters(conn *nftables.Conn, table *nftables.Table, counters map[string]uint64) map[string]uint64 {
	if table == nil {
		return counters
	}

	// Find the flywall_stats chain
	chains, err := conn.ListChains()
	if err != nil {
		return counters
	}

	var statsChain *nftables.Chain
	for _, chain := range chains {
		if chain.Name == "flywall_stats" && chain.Table.Name == k.tableName {
			statsChain = chain
			break
		}
	}

	if statsChain == nil {
		return counters
	}

	rules, err := conn.GetRules(table, statsChain)
	if err != nil {
		return counters
	}

	// Map to associate rule patterns with counter names
	// These correspond to the rules in script_builder.go
	counterNames := []string{"cnt_syn", "cnt_rst", "cnt_fin", "cnt_udp", "cnt_icmp"}

	for i, rule := range rules {
		if i < len(counterNames) {
			for _, e := range rule.Exprs {
				if counter, ok := e.(*expr.Counter); ok {
					counters[counterNames[i]] = counter.Packets
					break
				}
			}
		}
	}

	return counters
}
