// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package kernel

import "time"

// FlowState represents the connection tracking state.
type FlowState string

const (
	FlowStateNew         FlowState = "NEW"
	FlowStateEstablished FlowState = "ESTABLISHED"
	FlowStateClosed      FlowState = "CLOSED"
)

// Flow represents a connection tracking entry.
type Flow struct {
	ID        string    // 5-tuple hash key
	SrcIP     string    // Source IP address
	DstIP     string    // Destination IP address
	SrcPort   uint16    // Source port
	DstPort   uint16    // Destination port
	Protocol  string    // "tcp", "udp", "icmp"
	State     FlowState // Connection state
	Packets   uint64    // Packet count
	Bytes     uint64    // Byte count
	StartTime time.Time // Connection start time
	LastSeen  time.Time // Last packet timestamp
}

// Counter holds nftables rule statistics.
type Counter struct {
	Packets uint64
	Bytes   uint64
}
