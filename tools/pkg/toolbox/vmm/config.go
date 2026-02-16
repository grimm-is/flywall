// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package vmm

import "io"

type Config struct {
	KernelPath      string
	InitrdPath      string
	RootfsPath      string
	ProjectRoot     string
	Debug           bool
	ConsoleOutput   bool
	RunSkipped      bool        // Force normally-skipped tests to run
	Verbose         bool        // Show detailed status messages
	StrictIsolation bool        // Kill workers after every test (no reuse)
	Trace           bool        // Log all JSONL protocol messages
	BuildDir        string      // Directory for ephemeral artifacts (sockets, overlays)
	BuildSharePath  string      // Path to share as 'build_share' (defaults to ProjectRoot/build)
	AssetsSharePath string      // Shared writable assets directory (ipsets, geoip, caches)
	ArtifactDir     string      // Directory for persistent test artifacts (logs, state)
	ForwardPorts    map[int]int // Host -> Guest port forwarding (TCP)
	Stdin           io.Reader   // Input stream to attach to VM console (if nil, no stdin)
	InterfaceCount  int         // Number of network interfaces to provision (primary is always WAN)
	DevMode         bool        // Boot with dev_mode=true (full stack) instead of agent_mode=true
	MemoryMB        int         // RAM size in MB (defaults to 256M if 0)
	Stdout          io.Writer   // Custom stdout (overrides default os.Stdout behavior)
	Stderr          io.Writer   // Custom stderr (overrides default os.Stderr behavior)
}
