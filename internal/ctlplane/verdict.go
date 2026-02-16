// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ctlplane

// VerdictType represents the type of verdict for a packet
type VerdictType int

const (
	// VerdictDrop drops the packet
	VerdictDrop VerdictType = iota
	// VerdictAccept accepts the packet
	VerdictAccept
	// VerdictAcceptWithMark accepts the packet and sets a conntrack mark
	VerdictAcceptWithMark
)

// Verdict represents the verdict for a packet, including optional conntrack mark
type Verdict struct {
	Type VerdictType
	Mark uint32 // Only used when Type is VerdictAcceptWithMark
}
