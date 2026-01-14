//go:build !linux
// +build !linux

package metrics

// NFTStats holds nftables statistics (stub for non-Linux).
type NFTStats struct {
	Counters    map[string]uint64
	Sets        map[string]int
	RuleStats   map[string]uint64
	PolicyStats map[string]PolicyCounters
}

// PolicyCounters holds packet and byte counts for a policy.
type PolicyCounters struct {
	Packets uint64
	Bytes   uint64
}

// collectNFTablesNative is a no-op on non-Linux platforms.
func collectNFTablesNative(tableName string) (*NFTStats, error) {
	return &NFTStats{
		Counters:    make(map[string]uint64),
		Sets:        make(map[string]int),
		RuleStats:   make(map[string]uint64),
		PolicyStats: make(map[string]PolicyCounters),
	}, nil
}

// InterfaceCounters holds per-interface traffic counters.
type InterfaceCounters struct {
	RxPackets uint64
	RxBytes   uint64
	TxPackets uint64
	TxBytes   uint64
}

// collectInterfaceStatsNative is a no-op on non-Linux platforms.
func collectInterfaceStatsNative(tableName string) (map[string]InterfaceCounters, error) {
	return make(map[string]InterfaceCounters), nil
}
