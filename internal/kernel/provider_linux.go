// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux
// +build linux

package kernel

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

// DumpFlows returns all active conntrack flows by parsing /proc/net/nf_conntrack.
func (k *LinuxKernel) DumpFlows() ([]Flow, error) {
	file, err := os.Open("/proc/net/nf_conntrack")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Conntrack not available or no flows
		}
		return nil, fmt.Errorf("failed to open conntrack table: %w", err)
	}
	defer file.Close()

	var flows []Flow
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		flow, err := k.parseConntrackLine(line)
		if err == nil {
			flows = append(flows, flow)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading conntrack table: %w", err)
	}

	return flows, nil
}

// parseConntrackLine parses a single line from /proc/net/nf_conntrack
func (k *LinuxKernel) parseConntrackLine(line string) (Flow, error) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return Flow{}, fmt.Errorf("invalid conntrack line")
	}

	flow := Flow{
		Protocol: fields[0],
		LastSeen: time.Now(),
	}

	// Protocol number is fields[1]
	// Timeout is fields[2]

	// Find state for TCP
	stateIdx := 3
	if flow.Protocol == "tcp" {
		flow.State = FlowState(fields[3])
		stateIdx = 4
	} else {
		flow.State = FlowStateEstablished // UDP/ICMP are effectively ESTABLISHED if in table
	}

	// Extract src, dst, sport, dport from the first tuple
	for i := stateIdx; i < len(fields); i++ {
		field := fields[i]
		if strings.HasPrefix(field, "src=") {
			flow.SrcIP = field[4:]
		} else if strings.HasPrefix(field, "dst=") {
			flow.DstIP = field[4:]
		} else if strings.HasPrefix(field, "sport=") {
			port, _ := strconv.ParseUint(field[6:], 10, 16)
			flow.SrcPort = uint16(port)
		} else if strings.HasPrefix(field, "dport=") {
			port, _ := strconv.ParseUint(field[6:], 10, 16)
			flow.DstPort = uint16(port)
		} else if field == "[ASSURED]" {
			// Flow is established and bidirectional
		}
	}

	// Generate a unique ID based on the 5-tuple
	idInput := fmt.Sprintf("%s-%s-%s-%d-%d", flow.Protocol, flow.SrcIP, flow.DstIP, flow.SrcPort, flow.DstPort)
	flow.ID = fmt.Sprintf("%x", md5.Sum([]byte(idInput)))

	return flow, nil
}

// GetFlow retrieves a specific flow by ID.
func (k *LinuxKernel) GetFlow(id string) (Flow, bool) {
	flows, err := k.DumpFlows()
	if err != nil {
		return Flow{}, false
	}

	for _, f := range flows {
		if f.ID == id {
			return f, true
		}
	}

	return Flow{}, false
}

// KillFlow removes a flow from conntrack using the conntrack CLI.
func (k *LinuxKernel) KillFlow(id string) error {
	flow, ok := k.GetFlow(id)
	if !ok {
		return fmt.Errorf("flow not found: %s", id)
	}

	// conntrack -D -p [proto] -s [src] -d [dst] --sport [sport] --dport [dport]
	cmd := exec.Command("conntrack", "-D",
		"-p", flow.Protocol,
		"-s", flow.SrcIP,
		"-d", flow.DstIP,
		"--sport", fmt.Sprintf("%d", flow.SrcPort),
		"--dport", fmt.Sprintf("%d", flow.DstPort),
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		// If the error is "No such file or directory", conntrack tool might be missing
		// If output contains "0 flow entries", it might already be gone (which is fine)
		if strings.Contains(string(out), "0 flow entries") {
			return nil
		}
		return fmt.Errorf("failed to kill flow: %w, output: %s", err, string(out))
	}

	return nil
}

// AddBlock adds an IP to a blocklist set.
func (k *LinuxKernel) AddBlock(ip string) error {
	conn, err := nftables.New()
	if err != nil {
		return err
	}

	table := &nftables.Table{Name: k.tableName, Family: nftables.TableFamilyINet}
	set := &nftables.Set{
		Table: table,
		Name:  "blocked_ips",
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP: %s", ip)
	}

	err = conn.SetAddElements(set, []nftables.SetElement{
		{Key: parsedIP.To4()},
	})
	if err != nil {
		return err
	}

	return conn.Flush()
}

// RemoveBlock removes an IP from a blocklist set.
func (k *LinuxKernel) RemoveBlock(ip string) error {
	conn, err := nftables.New()
	if err != nil {
		return err
	}

	table := &nftables.Table{Name: k.tableName, Family: nftables.TableFamilyINet}
	set := &nftables.Set{
		Table: table,
		Name:  "blocked_ips",
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP: %s", ip)
	}

	err = conn.SetDeleteElements(set, []nftables.SetElement{
		{Key: parsedIP.To4()},
	})
	if err != nil {
		return err
	}

	return conn.Flush()
}

// IsBlocked checks if an IP is in the blocklist.
func (k *LinuxKernel) IsBlocked(ip string) bool {
	conn, err := nftables.New()
	if err != nil {
		return false
	}

	table := &nftables.Table{Name: k.tableName, Family: nftables.TableFamilyINet}
	set, err := conn.GetSetByName(table, "blocked_ips")
	if err != nil {
		return false
	}

	elements, err := conn.GetSetElements(set)
	if err != nil {
		return false
	}

	parsedIP := net.ParseIP(ip).To4()
	for _, el := range elements {
		if bytes.Equal(el.Key, parsedIP) {
			return true
		}
	}

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
