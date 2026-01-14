package engine

import (
	"net"
	"strings"
	"log"

	"grimm.is/flywall/internal/config"
)

// Packet represents the metadata needed for rule matching
type Packet struct {
	SrcIP    string
	DstIP    string
	SrcPort  int
	DstPort  int
	Protocol string // "tcp", "udp", "icmp"
	InInterface string
	OutInterface string
}

// Match checks if a packet matches a rule
func Match(rule config.PolicyRule, pkt Packet) bool {
	// 1. Protocol
	if !MatchProtocol(rule.Protocol, pkt.Protocol) {
		return false
	}

	// 2. Source IP
	if rule.SrcIP != "" {
		match := MatchIP(rule.SrcIP, pkt.SrcIP)
		if rule.InvertSrc {
			match = !match
		}
		if !match {
			return false
		}
	}

	// 3. Destination IP
	if rule.DestIP != "" {
		match := MatchIP(rule.DestIP, pkt.DstIP)
		if rule.InvertDest {
			match = !match
		}
		if !match {
			return false
		}
	}

	// 4. Source Port
	if !MatchPort(rule.SrcPort, rule.SrcPorts, pkt.SrcPort) {
		return false
	}

	// 5. Destination Port
	if !MatchPort(rule.DestPort, rule.DestPorts, pkt.DstPort) {
		return false
	}
	
	// 6. Interfaces
	if rule.InInterface != "" && rule.InInterface != pkt.InInterface {
		return false
	}
	// Note: OutInterface matching is complex in simulation as we might not know routing decision yet.
	// For now, if rule requires OutInterface, only match if packet explicitly has it set.
	if rule.OutInterface != "" && rule.OutInterface != pkt.OutInterface {
		return false
	}

	return true
}

// MatchProtocol checks if protocols match (case insensitive)
func MatchProtocol(ruleProto, pktProto string) bool {
	if ruleProto == "" {
		return true // Any protocol
	}
	return strings.EqualFold(ruleProto, pktProto)
}

// MatchIP checks if an IP belongs to a CIDR or equals a specific IP
func MatchIP(ruleIP, pktIP string) bool {
	if ruleIP == "" {
		return true
	}

	// Check if rule is CIDR
	if strings.Contains(ruleIP, "/") {
		_, ipNet, err := net.ParseCIDR(ruleIP)
		if err != nil {
			log.Printf("Invalid rule CIDR %s: %v", ruleIP, err)
			return false
		}
		parsedIP := net.ParseIP(pktIP)
		if parsedIP == nil {
			return false
		}
		return ipNet.Contains(parsedIP)
	}

	// Single IP match
	return ruleIP == pktIP
}

// MatchPort checks if a packet port matches rule port(s)
func MatchPort(single int, multiple []int, packetPort int) bool {
	// If no ports specified, match all
	if single == 0 && len(multiple) == 0 {
		return true
	}

	// Check single
	if single != 0 && single == packetPort {
		return true
	}

	// Check multiple
	for _, p := range multiple {
		if p == packetPort {
			return true
		}
	}

	return false
}
