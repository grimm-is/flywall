// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux
// +build linux

package qos

import (
	"fmt"
	"os/exec"
	"strings"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"

	"github.com/vishvananda/netlink"
)

// Manager handles QoS traffic shaping configuration.
type Manager struct {
	logger *logging.Logger
}

// NewManager creates a new QoS manager.
func NewManager(logger *logging.Logger) *Manager {
	if logger == nil {
		logger = logging.New(logging.DefaultConfig())
	}
	return &Manager{
		logger: logger,
	}
}

// ApplyConfig applies QoS configuration to interfaces.
func (m *Manager) ApplyConfig(cfg *config.Config) error {
	for i, policy := range cfg.QoSPolicies {
		if !policy.Enabled {
			continue
		}

		if err := m.applyPolicy(policy, i); err != nil {
			return fmt.Errorf("failed to apply QoS policy %s: %w", policy.Name, err)
		}
	}
	return nil
}

func (m *Manager) applyPolicy(pol config.QoSPolicy, policyIdx int) error {
	link, err := netlink.LinkByName(pol.Interface)
	if err != nil {
		return fmt.Errorf("interface %s not found: %w", pol.Interface, err)
	}

	// 1. Clear existing qdiscs (root)
	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		return fmt.Errorf("failed to list qdiscs: %w", err)
	}
	for _, q := range qdiscs {
		if q.Attrs().Parent == netlink.HANDLE_ROOT {
			netlink.QdiscDel(q)
		}
	}

	// 2. Create Root HTB Qdisc
	// Handle 1:
	rootQdisc := netlink.NewHtb(netlink.QdiscAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    netlink.HANDLE_ROOT,
		Handle:    netlink.MakeHandle(1, 0),
	})
	// Set default class to 0 (unclassified) or specific default
	// rootQdisc.Defcls = 0

	if err := netlink.QdiscAdd(rootQdisc); err != nil {
		return fmt.Errorf("failed to add root HTB qdisc: %w", err)
	}

	// 3. Create Root Class (Total Bandwidth)
	// Class 1:1
	rate := parseRate(pol.UploadMbps) // Assume upload shaping for egress interface
	if pol.DownloadMbps > 0 && pol.UploadMbps == 0 {
		// If only download set, maybe we rely on ingress qdisc (which is harder)
		// For now, let's assume this policy applies to the interface's EGRESS
		rate = parseRate(pol.DownloadMbps)
	}

	rootClass := netlink.NewHtbClass(netlink.ClassAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    netlink.MakeHandle(1, 0),
		Handle:    netlink.MakeHandle(1, 1),
	}, netlink.HtbClassAttrs{
		Rate:    rate,
		Ceil:    rate,
		Buffer:  1514, // Reasonable default
		Cbuffer: 1514,
	})

	if err := netlink.ClassAdd(rootClass); err != nil {
		return fmt.Errorf("failed to add root HTB class: %w", err)
	}

	// 4. Create Child Classes
	classIDMap := make(map[string]uint16) // Map name to minor handle

	for i, class := range pol.Classes {
		minorID := uint16(10 + i) // Start at 1:10
		classIDMap[class.Name] = minorID

		classRate := parseRateStr(class.Rate, rate) // Convert % or unit to uint64
		classCeil := parseRateStr(class.Ceil, rate)
		if classCeil == 0 {
			classCeil = rate // Default ceil to max
		}

		prio := 0
		if class.Priority > 0 {
			prio = class.Priority
		}

		childClass := netlink.NewHtbClass(netlink.ClassAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    netlink.MakeHandle(1, 1),
			Handle:    netlink.MakeHandle(1, minorID),
		}, netlink.HtbClassAttrs{
			// Convert to bytes/s for netlink
			Rate:    classRate,
			Ceil:    classCeil,
			Prio:    uint32(prio),
			Buffer:  1514,
			Cbuffer: 1514,
		})

		if err := netlink.ClassAdd(childClass); err != nil {
			return fmt.Errorf("failed to add child class %s: %w", class.Name, err)
		}

		// Add Leaf Qdisc (fq_codel or sfq)
		leafParent := netlink.MakeHandle(1, minorID)

		// Default to fq_codel
		fq := netlink.NewFqCodel(netlink.QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    leafParent,
			Handle:    netlink.MakeHandle(100+uint16(i), 0),
		})

		if err := netlink.QdiscAdd(fq); err != nil {
			return fmt.Errorf("failed to add leaf qdisc for class %s: %w", class.Name, err)
		}
	}

	// 5. Apply Classification Rules (Filters) using FWMark
	for j, rule := range pol.Rules {
		classMinor, ok := classIDMap[rule.Class]
		if !ok {
			continue
		}

		classIdx := -1
		for idx, class := range pol.Classes {
			if class.Name == rule.Class {
				classIdx = idx
				break
			}
		}
		if classIdx == -1 {
			continue
		}

		mark := CalculateFWMark(policyIdx, classIdx)

		// FWMark filter using raw tc command (netlink lib limitations)
		//
		// CRITICAL IMPLEMENTATION NOTE:
		// We are intentionally executing the raw `tc` command here instead of using the `vishvananda/netlink` library.
		// As of v1.3.0, the library's `FilterAdd` implementation for the `fw` filter type has serialization issues
		// where the `handle` (fwmark) and `classid` attributes are sometimes omitted or incorrectly encoded.
		//
		// This caused integration tests to fail because `tc filter show` verified the filter existed but
		// had no handle (mark) associated, rendering the QoS classification ineffective.
		//
		// We attempted to:
		// 1. Use `netlink.Fw` struct (missing in older vendored versions).
		// 2. Define a local `Fw` struct (library type assertion fails).
		//
		// DECISION:
		// Until the upstream library is updated/patched, we use `os/exec` for reliability on this critical path.
		// Do not revert to `netlink.FilterAdd` for `fw` type without verifying `tc filter show` contains
		// correct handles (e.g. 0xf000) and classids.
		cmd := exec.Command("tc", "filter", "add", "dev", pol.Interface,
			"parent", "1:0",
			"protocol", "ip",
			"prio", fmt.Sprintf("%d", 100+j),
			"handle", fmt.Sprintf("0x%x", mark),
			"fw",
			"classid", fmt.Sprintf("1:%x", classMinor),
		)

		if out, err := cmd.CombinedOutput(); err != nil {
			m.logger.Warn("failed to add fwmark filter", "mark", mark, "error", err, "output", string(out))
		}
	}
	return nil
}

// Helpers

func parseRate(mbps int) uint64 {
	// Mbps to Bytes/s
	// 1 Mbps = 1000 * 1000 bits / 8 = 125,000 bytes/s
	return uint64(mbps) * 125000
}

func parseRateStr(rateStr string, parentRate uint64) uint64 {
	if rateStr == "" {
		return 0
	}
	// Handle percentages
	if strings.HasSuffix(rateStr, "%") {
		var percent float64
		fmt.Sscanf(rateStr, "%f%%", &percent)
		return uint64(float64(parentRate) * percent / 100.0)
	}
	// Handle raw numbers (assume mbit)
	var rate int
	_, err := fmt.Sscanf(rateStr, "%dmbit", &rate)
	if err == nil {
		return parseRate(rate)
	}

	return 0 // Fallback
}
