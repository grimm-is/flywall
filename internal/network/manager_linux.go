//go:build linux
// +build linux

package network

// DefaultNetlinker is the default RealNetlinker instance.
var DefaultNetlinker Netlinker = &RealNetlinker{}

// manager handles network configuration via netlink.
// defined in manager_logic.go

// DNSUpdater is an interface for updating the DNS service dynamically.
// defined in manager_logic.go

// NewManager creates a new network manager.
func NewManager() *Manager {
	return &Manager{
		nl:  &RealNetlinker{},
		sys: &RealSystemController{},
		cmd: &RealCommandExecutor{},
		// uidRulePriority determines where UID-based routing rules sit in the RPDB.
		// 15000 is chosen to be after local (0) but well before main (32766) and
		// default (32767), allowing UID rules to override main table routes while
		// still respecting local addresses.
		uidRulePriority: 15000,
	}
}

// NewManagerWithDeps creates a new manager with injected dependencies.
// moved to manager_logic.go

// SetIPForwarding enables or disables IP forwarding.
// SetIPForwarding enables or disables IP forwarding.
// moved to manager_logic.go

// SetupLoopback ensures the loopback interface is up.
// moved to manager_logic.go
