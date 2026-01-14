//go:build linux
// +build linux

package metrics

import (
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

// NFTStats holds nftables statistics collected via native netlink.
type NFTStats struct {
	// Counters maps counter name -> packets
	Counters map[string]uint64
	// Sets maps set name -> element count
	Sets map[string]int
	// RuleStats maps chain:action -> packets
	RuleStats map[string]uint64
	// PolicyStats maps policy name -> packets/bytes
	PolicyStats map[string]PolicyCounters
}

// PolicyCounters holds packet and byte counts for a policy.
type PolicyCounters struct {
	Packets uint64
	Bytes   uint64
}

// collectNFTablesNative gathers nftables statistics using google/nftables library.
// This replaces the exec.Command("nft", ...) calls with native netlink.
func collectNFTablesNative(tableName string) (*NFTStats, error) {
	conn, err := nftables.New()
	if err != nil {
		return nil, err
	}

	stats := &NFTStats{
		Counters:    make(map[string]uint64),
		Sets:        make(map[string]int),
		RuleStats:   make(map[string]uint64),
		PolicyStats: make(map[string]PolicyCounters),
	}

	// Find our table
	tables, err := conn.ListTables()
	if err != nil {
		return stats, nil // Return empty stats, non-fatal
	}

	var targetTable *nftables.Table
	for _, t := range tables {
		if t.Name == tableName {
			targetTable = t
			break
		}
	}

	if targetTable == nil {
		return stats, nil
	}

	// Collect set statistics
	sets, err := conn.GetSets(targetTable)
	if err == nil {
		for _, set := range sets {
			elements, err := conn.GetSetElements(set)
			if err == nil {
				stats.Sets[set.Name] = len(elements)
			}
		}
	}

	// Collect chain and rule statistics
	chains, err := conn.ListChains()
	if err != nil {
		return stats, nil
	}

	for _, chain := range chains {
		if chain.Table.Name != tableName {
			continue
		}

		rules, err := conn.GetRules(targetTable, chain)
		if err != nil {
			continue
		}

		for _, rule := range rules {
			var packets, bytes uint64
			var action string
			var counterName string

			for _, e := range rule.Exprs {
				switch ex := e.(type) {
				case *expr.Counter:
					packets = ex.Packets
					bytes = ex.Bytes
				case *expr.Verdict:
					switch ex.Kind {
					case expr.VerdictAccept:
						action = "accept"
					case expr.VerdictDrop:
						action = "drop"
					case expr.VerdictReturn:
						action = "return"
					}
				}
			}

			// Check for named counter reference in UserData
			if len(rule.UserData) > 0 {
				userData := string(rule.UserData)
				// Check if this is a stats counter comment
				if strings.HasPrefix(userData, "cnt_") || strings.Contains(userData, "cnt_") {
					counterName = userData
				}
				// Check if this is a policy counter
				if strings.HasPrefix(userData, "policy-") {
					stats.PolicyStats[userData] = PolicyCounters{
						Packets: packets,
						Bytes:   bytes,
					}
				}
			}

			// Named counters from flywall_stats chain
			if counterName != "" {
				stats.Counters[counterName] = packets
			}

			// Aggregate by chain:action
			if action != "" {
				key := chain.Name + ":" + action
				stats.RuleStats[key] += packets
				_ = bytes // bytes available if needed
			}
		}
	}

	return stats, nil
}

// collectInterfaceStatsNative collects interface counters from nftables using native netlink.
func collectInterfaceStatsNative(tableName string) (map[string]InterfaceCounters, error) {
	conn, err := nftables.New()
	if err != nil {
		return nil, err
	}

	results := make(map[string]InterfaceCounters)

	tables, err := conn.ListTables()
	if err != nil {
		return results, nil
	}

	var targetTable *nftables.Table
	for _, t := range tables {
		if t.Name == tableName {
			targetTable = t
			break
		}
	}

	if targetTable == nil {
		return results, nil
	}

	chains, err := conn.ListChains()
	if err != nil {
		return results, nil
	}

	for _, chain := range chains {
		if chain.Table.Name != tableName {
			continue
		}

		rules, err := conn.GetRules(targetTable, chain)
		if err != nil {
			continue
		}

		for _, rule := range rules {
			if len(rule.UserData) == 0 {
				continue
			}

			userData := string(rule.UserData)
			// Parse format: "interface-eth0-in" or "interface-eth0-out"
			if !strings.HasPrefix(userData, "interface-") {
				continue
			}

			parts := strings.Split(userData, "-")
			if len(parts) < 3 {
				continue
			}

			ifaceName := parts[1]
			direction := parts[2]

			var packets, bytes uint64
			for _, e := range rule.Exprs {
				if counter, ok := e.(*expr.Counter); ok {
					packets = counter.Packets
					bytes = counter.Bytes
					break
				}
			}

			ic := results[ifaceName]
			if direction == "in" {
				ic.RxPackets = packets
				ic.RxBytes = bytes
			} else if direction == "out" {
				ic.TxPackets = packets
				ic.TxBytes = bytes
			}
			results[ifaceName] = ic
		}
	}

	return results, nil
}

// InterfaceCounters holds per-interface traffic counters.
type InterfaceCounters struct {
	RxPackets uint64
	RxBytes   uint64
	TxPackets uint64
	TxBytes   uint64
}
