// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package hooks

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"grimm.is/flywall/internal/ebpf/types"
)

// Manager manages eBPF program attachments (hooks)
type Manager struct {
	links      map[string]*AttachedHook
	programs   map[string]*ebpf.Program
	mutex      sync.RWMutex
	interfaces map[string]bool // Track interface availability
}

// AttachedHook represents an attached eBPF program
type AttachedHook struct {
	Name        string
	Program     *ebpf.Program
	Link        link.Link
	Type        types.ProgramType
	AttachPoint string
	AttachedAt  int64
	Active      bool
	mutex       sync.RWMutex
}

// NewManager creates a new hook manager
func NewManager() *Manager {
	return &Manager{
		links:      make(map[string]*AttachedHook),
		programs:   make(map[string]*ebpf.Program),
		interfaces: make(map[string]bool),
	}
}

// RegisterProgram registers a program with the hook manager
func (hm *Manager) RegisterProgram(name string, program *ebpf.Program) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	hm.programs[name] = program
}

// Attach attaches an eBPF program to a hook point
func (hm *Manager) Attach(config *types.HookConfig) error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	// Check if program is registered
	program, exists := hm.programs[config.ProgramName]
	if !exists {
		return fmt.Errorf("program %s not registered", config.ProgramName)
	}

	// Check if already attached
	if hook, exists := hm.links[config.ProgramName]; exists && hook.Active {
		if !config.AutoReplace {
			return fmt.Errorf("program %s already attached", config.ProgramName)
		}

		// Detach existing hook
		if err := hm.detachHook(hook); err != nil {
			return fmt.Errorf("failed to detach existing hook: %w", err)
		}
	}

	// Attach based on program type
	var lnk link.Link
	var err error

	switch config.ProgramType {
	case types.ProgramTypeXDP:
		lnk, err = hm.attachXDP(program, config.AttachPoint)
	case types.ProgramTypeTC:
		lnk, err = hm.attachTC(program, config.AttachPoint)
	case types.ProgramTypeSocketFilter:
		lnk, err = hm.attachSocketFilter(program, config.AttachPoint)
	default:
		return fmt.Errorf("unsupported program type: %v", config.ProgramType)
	}

	if err != nil {
		return fmt.Errorf("failed to attach program: %w", err)
	}

	// Create attached hook record
	hook := &AttachedHook{
		Name:        config.ProgramName,
		Program:     program,
		Link:        lnk,
		Type:        config.ProgramType,
		AttachPoint: config.AttachPoint,
		AttachedAt:  time.Now().Unix(),
		Active:      true,
	}

	hm.links[config.ProgramName] = hook

	return nil
}

// attachXDP attaches an XDP program
func (hm *Manager) attachXDP(program *ebpf.Program, iface string) (link.Link, error) {
	if !hm.interfaceExists(iface) {
		return nil, fmt.Errorf("interface %s does not exist", iface)
	}

	// Find interface index
	ifaceObj, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", iface, err)
	}

	return link.AttachXDP(link.XDPOptions{
		Program:   program,
		Interface: ifaceObj.Index,
	})
}

// attachTC attaches a TC program
func (hm *Manager) attachTC(program *ebpf.Program, iface string) (link.Link, error) {
	if !hm.interfaceExists(iface) {
		return nil, fmt.Errorf("interface %s does not exist", iface)
	}

	// Find interface index
	ifaceObj, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", iface, err)
	}

	// Use the new TCX API
	return link.AttachTCX(link.TCXOptions{
		Program:   program,
		Interface: ifaceObj.Index,
		Attach:    ebpf.AttachTCXIngress,
	})
}

// attachSocketFilter attaches a socket filter
func (hm *Manager) attachSocketFilter(program *ebpf.Program, socket string) (link.Link, error) {
	// For socket filters, the attach point is a file descriptor
	// In practice, this would be passed as an FD
	fd, err := strconv.Atoi(socket)
	if err != nil {
		return nil, fmt.Errorf("invalid socket descriptor: %w", err)
	}

	socketFile := os.NewFile(uintptr(fd), "socket")
	// We don't close socketFile here because os.NewFile doesn't dup the FD,
	// and closing the *File would close the underlying FD which may be 
	// managed/needed by the caller (e.g. systemd or another service).

	// AttachSocketFilter doesn't return a link in the new API
	err = link.AttachSocketFilter(socketFile, program)
	return nil, err
}

// Detach detaches an eBPF program
func (hm *Manager) Detach(programName string) error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	hook, exists := hm.links[programName]
	if !exists {
		return fmt.Errorf("program %s not attached", programName)
	}

	return hm.detachHook(hook)
}

// detachHook detaches a hook (internal method, assumes lock held)
func (hm *Manager) detachHook(hook *AttachedHook) error {
	hook.mutex.Lock()
	defer hook.mutex.Unlock()

	if !hook.Active {
		return nil
	}

	if err := hook.Link.Close(); err != nil {
		return fmt.Errorf("failed to close link: %w", err)
	}

	hook.Active = false
	return nil
}

// GetAttachedHook returns information about an attached hook
func (hm *Manager) GetAttachedHook(programName string) (*AttachedHook, error) {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	hook, exists := hm.links[programName]
	if !exists {
		return nil, fmt.Errorf("program %s not attached", programName)
	}

	return hook, nil
}

// ListAttached returns all attached hooks
func (hm *Manager) ListAttached() map[string]*AttachedHook {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	result := make(map[string]*AttachedHook)
	for name, hook := range hm.links {
		if hook.Active {
			result[name] = hook
		}
	}

	return result
}

// UpdateInterfaces updates the list of available interfaces
func (hm *Manager) UpdateInterfaces() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get interfaces: %w", err)
	}

	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	// Clear existing list
	hm.interfaces = make(map[string]bool)

	// Add current interfaces
	for _, iface := range interfaces {
		hm.interfaces[iface.Name] = true
	}

	return nil
}

// interfaceExists checks if an interface exists
func (hm *Manager) interfaceExists(name string) bool {
	exists, _ := hm.interfaces[name]
	return exists
}

// DetachAll detaches all hooks
func (hm *Manager) DetachAll() error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	var firstErr error

	for name, hook := range hm.links {
		if err := hm.detachHook(hook); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to detach %s: %w", name, err)
		}
	}

	// Clear all links
	hm.links = make(map[string]*AttachedHook)

	return firstErr
}

// Close closes the hook manager and detaches all hooks
func (hm *Manager) Close() error {
	return hm.DetachAll()
}

// GetHookStats returns statistics about attached hooks
func (hm *Manager) GetHookStats() map[string]types.HookStats {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	stats := make(map[string]types.HookStats)

	for name, hook := range hm.links {
		if !hook.Active {
			continue
		}

		stats[name] = types.HookStats{
			Name:        name,
			Type:        hook.Type.String(),
			AttachPoint: hook.AttachPoint,
			AttachedAt:  hook.AttachedAt,
			RunCount:    hm.getRunCount(hook),
		}
	}

	return stats
}

// getRunCount gets the run count for a hook
func (hm *Manager) getRunCount(hook *AttachedHook) uint64 {
	// This would need to be tracked separately
	// For now, return 0
	return 0
}

// ValidateAttachment validates if a program can be attached
func (hm *Manager) ValidateAttachment(config *types.HookConfig) error {
	// Check if program exists
	_, exists := hm.programs[config.ProgramName]
	if !exists {
		return fmt.Errorf("program %s not registered", config.ProgramName)
	}

	// Validate attach point based on type
	switch config.ProgramType {
	case types.ProgramTypeXDP, types.ProgramTypeTC:
		if !hm.interfaceExists(config.AttachPoint) {
			return fmt.Errorf("interface %s does not exist", config.AttachPoint)
		}
	case types.ProgramTypeSocketFilter:
		if _, err := strconv.Atoi(config.AttachPoint); err != nil {
			return fmt.Errorf("invalid socket descriptor: %s", config.AttachPoint)
		}
	}

	return nil
}

// Replace replaces an attached hook with a new program
func (hm *Manager) Replace(programName string, newProgram *ebpf.Program) error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	hook, exists := hm.links[programName]
	if !exists {
		return fmt.Errorf("program %s not attached", programName)
	}

	// Detach old hook
	if err := hm.detachHook(hook); err != nil {
		return fmt.Errorf("failed to detach old hook: %w", err)
	}

	// Attach new program with same configuration
	config := &types.HookConfig{
		ProgramName: programName,
		ProgramType: hook.Type,
		AttachPoint: hook.AttachPoint,
		AutoReplace: true,
	}

	// Temporarily register new program
	oldProgram := hm.programs[programName]
	hm.programs[programName] = newProgram

	// Attach new program
	err := hm.Attach(config)
	if err != nil {
		// Restore old program on failure
		hm.programs[programName] = oldProgram
		return fmt.Errorf("failed to attach new program: %w", err)
	}

	return nil
}

// GetProgramType returns the program type for a program
func (hm *Manager) GetProgramType(programName string) (types.ProgramType, error) {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	program, exists := hm.programs[programName]
	if !exists {
		return types.ProgramTypeUnspec, fmt.Errorf("program %s not found", programName)
	}

	info, err := program.Info()
	if err != nil {
		return types.ProgramTypeUnspec, err
	}

	// Convert ebpf.ProgramType to types.ProgramType
	switch info.Type {
	case ebpf.XDP:
		return types.ProgramTypeXDP, nil
	case ebpf.SchedCLS:
		return types.ProgramTypeTC, nil
	case ebpf.SocketFilter:
		return types.ProgramTypeSocketFilter, nil
	default:
		return types.ProgramTypeUnspec, fmt.Errorf("unsupported program type: %v", info.Type)
	}
}

// IsAttached returns true if a program is attached
func (hm *Manager) IsAttached(programName string) bool {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	hook, exists := hm.links[programName]
	return exists && hook.Active
}
