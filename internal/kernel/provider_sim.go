// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build darwin || simulator
// +build darwin simulator

package kernel

import (
	"fmt"
	"sync"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"grimm.is/flywall/internal/clock"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/engine"
)

// SimKernel is a stateful in-memory kernel simulator for PCAP replay.
// It maintains conntrack-like flow tables and blocklists without requiring Linux.
type SimKernel struct {
	mu sync.RWMutex

	// Clock for time-synchronized simulation
	Clock *clock.MockClock

	// State tables
	FlowTable  map[string]*Flow    // 5-tuple key -> Flow
	RuleStats  map[string]*Counter // RuleID -> Counter
	BlockedIPs map[string]bool     // IP -> blocked

	// Rule Engine
	Engine *engine.RuleEngine

	// Configurable timeouts
	TCPTimeout time.Duration
	UDPTimeout time.Duration
}

// NewSimKernel creates a new simulation kernel.
func NewSimKernel(clk *clock.MockClock) *SimKernel {
	return &SimKernel{
		Clock:      clk,
		FlowTable:  make(map[string]*Flow),
		RuleStats:  make(map[string]*Counter),
		BlockedIPs: make(map[string]bool),
		TCPTimeout: 2 * time.Hour,
		UDPTimeout: 30 * time.Second,
	}
}

// Now returns the simulated time.
func (s *SimKernel) Now() time.Time {
	return s.Clock.Now()
}

// LoadConfig initializes the rule engine with the given config.
func (s *SimKernel) LoadConfig(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Engine = engine.NewRuleEngine(cfg)
}

// DumpFlows returns all active flows, excluding expired ones.
func (s *SimKernel) DumpFlows() ([]Flow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := s.Clock.Now()
	var active []Flow

	for _, f := range s.FlowTable {
		timeout := s.UDPTimeout
		if f.Protocol == "tcp" {
			timeout = s.TCPTimeout
		}

		if now.Sub(f.LastSeen) < timeout && f.State != FlowStateClosed {
			active = append(active, *f)
		}
	}
	return active, nil
}

// GetFlow retrieves a specific flow by ID.
func (s *SimKernel) GetFlow(id string) (Flow, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if f, ok := s.FlowTable[id]; ok {
		return *f, true
	}
	return Flow{}, false
}

// KillFlow removes a flow from the table.
func (s *SimKernel) KillFlow(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.FlowTable, id)
	return nil
}

// AddBlock adds an IP to the blocklist.
func (s *SimKernel) AddBlock(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.BlockedIPs[ip] = true
	return nil
}

// RemoveBlock removes an IP from the blocklist.
func (s *SimKernel) RemoveBlock(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.BlockedIPs, ip)
	return nil
}

// IsBlocked checks if an IP is in the blocklist.
func (s *SimKernel) IsBlocked(ip string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.BlockedIPs[ip]
}

// GetCounters returns named counter statistics.
// In simulation mode, this returns the SYN/RST/FIN counts tracked during packet injection.
func (s *SimKernel) GetCounters() (map[string]uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counters := make(map[string]uint64)
	for name, counter := range s.RuleStats {
		counters[name] = counter.Packets
	}
	return counters, nil
}

// InjectPacket simulates kernel packet processing.
// This is the core simulation method called by the PCAP replay loop.
// Returns true if the packet was accepted, false if dropped (blocked).
func (s *SimKernel) InjectPacket(packet gopacket.Packet) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Extract network layer
	var srcIP, dstIP string
	var protocol string

	if ipv4 := packet.Layer(layers.LayerTypeIPv4); ipv4 != nil {
		ip := ipv4.(*layers.IPv4)
		srcIP = ip.SrcIP.String()
		dstIP = ip.DstIP.String()
	} else if ipv6 := packet.Layer(layers.LayerTypeIPv6); ipv6 != nil {
		ip := ipv6.(*layers.IPv6)
		srcIP = ip.SrcIP.String()
		dstIP = ip.DstIP.String()
	} else {
		return true // Non-IP packet, pass through
	}

	// Check blocklist
	if s.BlockedIPs[srcIP] || s.BlockedIPs[dstIP] {
		return false // Dropped
	}

	// Extract transport layer
	var srcPort, dstPort uint16

	if tcp := packet.Layer(layers.LayerTypeTCP); tcp != nil {
		t := tcp.(*layers.TCP)
		srcPort = uint16(t.SrcPort)
		dstPort = uint16(t.DstPort)
		protocol = "tcp"
	} else if udp := packet.Layer(layers.LayerTypeUDP); udp != nil {
		u := udp.(*layers.UDP)
		srcPort = uint16(u.SrcPort)
		dstPort = uint16(u.DstPort)
		protocol = "udp"
	} else if packet.Layer(layers.LayerTypeICMPv4) != nil || packet.Layer(layers.LayerTypeICMPv6) != nil {
		protocol = "icmp"
	} else {
		return true // Unknown protocol, pass through
	}

	// Rule Engine Evaluation
	// We need to guess interfaces. For simulation, we assume:
	// - If DstIP is local (we don't check yet), it's Input.
	// - Else it's Forward.
	// We map Interfaces in LoadConfig via Engine.InterfaceToZone.
	// gopacket doesn't give us Interface.
	// We assume "eth0" (WAN) for now.

	pktInfo := engine.Packet{
		SrcIP:       srcIP,
		DstIP:       dstIP,
		Protocol:    protocol,
		InInterface: "eth0", // Assumption for MVP: Traffic enters WAN
	}

	if protocol == "tcp" || protocol == "udp" {
		pktInfo.SrcPort = int(srcPort)
		pktInfo.DstPort = int(dstPort)
	}

	var verdict engine.Verdict = engine.VerdictAccept
	var ruleID string

	if s.Engine != nil {
		verdict, ruleID = s.Engine.Evaluate(pktInfo)
	}

	// Update Rule Stats
	if ruleID != "" {
		if s.RuleStats[ruleID] == nil {
			s.RuleStats[ruleID] = &Counter{}
		}
		s.RuleStats[ruleID].Packets++
		s.RuleStats[ruleID].Bytes += uint64(len(packet.Data()))
	}

	if verdict == engine.VerdictDrop || verdict == engine.VerdictReject {
		return false
	}

	// Generate flow key
	key := s.flowKey(srcIP, dstIP, srcPort, dstPort, protocol)
	now := s.Clock.Now()
	pktLen := uint64(len(packet.Data()))

	// Update or create flow
	flow, exists := s.FlowTable[key]
	if !exists {
		flow = &Flow{
			ID:        key,
			SrcIP:     srcIP,
			DstIP:     dstIP,
			SrcPort:   srcPort,
			DstPort:   dstPort,
			Protocol:  protocol,
			State:     FlowStateNew,
			StartTime: now,
		}
		s.FlowTable[key] = flow
	}

	// Update counters
	flow.LastSeen = now
	flow.Packets++
	flow.Bytes += pktLen

	// TCP state machine
	if protocol == "tcp" {
		if tcp := packet.Layer(layers.LayerTypeTCP); tcp != nil {
			t := tcp.(*layers.TCP)
			flow.State = s.tcpStateTransition(flow.State, t)
		}
	} else if flow.State == FlowStateNew {
		// UDP/ICMP: move to ESTABLISHED after first packet
		flow.State = FlowStateEstablished
	}

	return true
}

// flowKey generates a unique key for the 5-tuple.
func (s *SimKernel) flowKey(srcIP, dstIP string, srcPort, dstPort uint16, protocol string) string {
	return fmt.Sprintf("%s:%d->%s:%d/%s", srcIP, srcPort, dstIP, dstPort, protocol)
}

// tcpStateTransition implements a simplified TCP state machine.
func (s *SimKernel) tcpStateTransition(current FlowState, tcp *layers.TCP) FlowState {
	switch current {
	case FlowStateNew:
		if tcp.SYN && !tcp.ACK {
			return FlowStateNew // SYN_SENT, stay in NEW
		}
		if tcp.SYN && tcp.ACK {
			return FlowStateEstablished // SYN_RECV -> ESTABLISHED
		}
		return FlowStateEstablished // Data without proper handshake

	case FlowStateEstablished:
		if tcp.FIN || tcp.RST {
			return FlowStateClosed
		}
		return FlowStateEstablished

	case FlowStateClosed:
		return FlowStateClosed // Terminal state
	}

	return current
}

// Stats returns simulation statistics.
func (s *SimKernel) Stats() SimStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SimStats{
		TotalFlows:   len(s.FlowTable),
		BlockedIPs:   len(s.BlockedIPs),
		FlowsByState: make(map[FlowState]int),
	}

	for _, f := range s.FlowTable {
		stats.FlowsByState[f.State]++
	}

	return stats
}

// SimStats holds simulation statistics.
type SimStats struct {
	TotalFlows   int
	BlockedIPs   int
	FlowsByState map[FlowState]int
}
