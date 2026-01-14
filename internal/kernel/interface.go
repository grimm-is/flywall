// Package kernel provides an abstraction over the Linux kernel's network subsystems.
// On Linux, it wraps real netlink/nftables calls.
// In simulation mode, it provides a stateful in-memory implementation for PCAP replay.
package kernel

import "time"

// Kernel abstracts the OS network subsystem.
// Components interact with this interface instead of making direct syscalls.
type Kernel interface {
	// Conntrack operations
	DumpFlows() ([]Flow, error)
	GetFlow(id string) (Flow, bool)
	KillFlow(id string) error

	// Blocklist operations
	AddBlock(ip string) error
	RemoveBlock(ip string) error
	IsBlocked(ip string) bool

	// Time abstraction (for synchronized simulation)
	Now() time.Time

	// Stats: Named counters for anomaly detection
	GetCounters() (map[string]uint64, error)
}
