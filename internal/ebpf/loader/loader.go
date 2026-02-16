// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package loader

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"grimm.is/flywall/internal/ebpf/interfaces"
	"grimm.is/flywall/internal/host"
)

// Loader handles loading and managing eBPF programs
type Loader struct {
	collection *ebpf.Collection
	links      []link.Link
	programs   map[string]*ebpf.Program
	maps       map[string]*ebpf.Map
	loaded     bool
	mutex      sync.Mutex
}

// NewLoader creates a new eBPF loader
func NewLoader() *Loader {
	return &Loader{
		programs: make(map[string]*ebpf.Program),
		maps:     make(map[string]*ebpf.Map),
		links:    make([]link.Link, 0),
	}
}

// LoadSpec loads an eBPF collection spec from embedded data
func (l *Loader) LoadSpec(data []byte) (*ebpf.CollectionSpec, error) {
	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to load collection spec: %w", err)
	}

	return spec, nil
}

// LoadCollection loads an eBPF collection from a spec
func (l *Loader) LoadCollection(spec *ebpf.CollectionSpec) error {
	if l.loaded {
		return fmt.Errorf("collection already loaded")
	}

	collection, err := ebpf.NewCollection(spec)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	l.collection = collection

	// Store references to programs and maps
	for name, program := range collection.Programs {
		l.programs[name] = program
	}

	for name, m := range collection.Maps {
		l.maps[name] = m
	}

	l.loaded = true
	return nil
}

// LoadProgram loads and attaches a specific program
func (l *Loader) LoadProgram(name string, progType string, attachTo string) error {
	if !l.loaded {
		return fmt.Errorf("no collection loaded")
	}

	prog, exists := l.collection.Programs[name]
	if !exists {
		return fmt.Errorf("program %s not found in collection", name)
	}

	// Store program reference
	l.programs[name] = prog

	// Attach based on program type
	var lnk link.Link
	var err error

	switch progType {
	case "xdp":
		lnk, err = l.attachXDP(prog, attachTo)
	case "tc":
		lnk, err = l.attachTC(prog, attachTo)
	case "socket_filter":
		lnk, err = l.attachSocketFilter(prog, attachTo)
	default:
		return fmt.Errorf("unsupported program type: %s", progType)
	}

	if err != nil {
		return fmt.Errorf("failed to attach program %s: %w", name, err)
	}

	l.links = append(l.links, lnk)
	return nil
}

// GetProgram returns a loaded program
func (l *Loader) GetProgram(name string) (interfaces.Program, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	prog, exists := l.programs[name]
	if !exists {
		return nil, fmt.Errorf("program %s not found", name)
	}

	return NewProgramWrapper(prog), nil
}

// GetMap returns a loaded eBPF map
func (l *Loader) GetMap(name string) (interfaces.Map, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	m, exists := l.maps[name]
	if !exists {
		return nil, fmt.Errorf("map %s not found", name)
	}
	return NewMapWrapper(m), nil
}

// Close closes all loaded programs and maps
func (l *Loader) Close() error {
	var firstErr error

	// Close all links
	for _, lnk := range l.links {
		if err := lnk.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Close collection
	if l.collection != nil {
		l.collection.Close()
	}

	l.loaded = false
	l.programs = make(map[string]*ebpf.Program)
	l.maps = make(map[string]*ebpf.Map)
	l.links = make([]link.Link, 0)

	return firstErr
}

// IsLoaded returns true if the collection is loaded
func (l *Loader) IsLoaded() bool {
	return l.loaded
}

// GetCollection returns the eBPF collection
func (l *Loader) GetCollection() *ebpf.Collection {
	return l.collection
}

// GetProgramInfo returns information about a program
func (l *Loader) GetProgramInfo(name string) (interfaces.ProgramInfo, error) {
	prog, err := l.GetProgram(name)
	if err != nil {
		return interfaces.ProgramInfo{}, err
	}

	return prog.Info()
}

// GetMapInfo returns information about a map
func (l *Loader) GetMapInfo(name string) (interfaces.MapInfo, error) {
	m, err := l.GetMap(name)
	if err != nil {
		return interfaces.MapInfo{}, err
	}

	return m.Info()
}

// attachXDP attaches an XDP program
func (l *Loader) attachXDP(prog *ebpf.Program, iface string) (link.Link, error) {
	// Find interface index
	ifaceObj, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", iface, err)
	}

	return link.AttachXDP(link.XDPOptions{
		Program:   prog,
		Interface: ifaceObj.Index,
	})
}

// attachTC attaches a TC program
func (l *Loader) attachTC(prog *ebpf.Program, iface string) (link.Link, error) {
	// Find interface index
	ifaceObj, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", iface, err)
	}

	return link.AttachTCX(link.TCXOptions{
		Program:   prog,
		Interface: ifaceObj.Index,
		Attach:    ebpf.AttachTCXIngress,
	})
}

// attachSocketFilter attaches a socket filter
func (l *Loader) attachSocketFilter(prog *ebpf.Program, socket string) (link.Link, error) {
	fd, err := strconv.Atoi(socket)
	if err != nil {
		return nil, fmt.Errorf("invalid socket descriptor: %w", err)
	}

	socketFile := os.NewFile(uintptr(fd), "socket")
	defer socketFile.Close()

	// AttachSocketFilter doesn't return a link in the new API
	err = link.AttachSocketFilter(socketFile, prog)
	return nil, err
}

// getAttachPoints returns the attach points for a program
func (l *Loader) getAttachPoints(name string) []string {
	// This would need to be tracked during attachment
	// For now, return empty slice
	return []string{}
}

// getMapInfoForProgram returns map info for maps used by a program
func (l *Loader) getMapInfoForProgram(name string) map[string]interfaces.MapInfo {
	result := make(map[string]interfaces.MapInfo)

	// This would need to track which maps are used by which program
	// For now, return all maps
	for name := range l.maps {
		info, err := l.GetMapInfo(name)
		if err == nil {
			result[name] = info
		}
	}

	return result
}

// VerifyKernelSupport checks if the kernel supports required eBPF features
func VerifyKernelSupport() error {
	issues := host.VerifyBPFSupport()
	for _, issue := range issues {
		if issue.Fatal {
			return fmt.Errorf("kernel support verification failed: %s", issue.Message)
		}
	}
	return nil
}

// EnableJIT enables eBPF JIT compilation
func EnableJIT() error {
	return os.WriteFile("/proc/sys/net/core/bpf_jit_enable", []byte("1"), 0644)
}

// GetMemoryLimit returns the current memory limit for maps
func GetMemoryLimit() (uint64, error) {
	limitMB, err := host.GetBPFJITLimit()
	return uint64(limitMB * 1024 * 1024), err
}

// SetMemoryLimit sets the memory limit for maps
func SetMemoryLimit(limit uint64) error {
	return host.SetBPFJITLimit(int64(limit / 1024 / 1024))
}
