// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package loader

import (
	"time"

	"github.com/cilium/ebpf"
	"grimm.is/flywall/internal/ebpf/interfaces"
)

// ProgramWrapper wraps an eBPF program to implement the interfaces.Program interface
type ProgramWrapper struct {
	program *ebpf.Program
}

// NewProgramWrapper creates a new program wrapper
func NewProgramWrapper(prog *ebpf.Program) *ProgramWrapper {
	return &ProgramWrapper{program: prog}
}

// Info returns information about the program
func (p *ProgramWrapper) Info() (interfaces.ProgramInfo, error) {
	info, err := p.program.Info()
	if err != nil {
		return interfaces.ProgramInfo{}, err
	}

	id, _ := info.ID()

	return interfaces.ProgramInfo{
		Name:       info.Name,
		Type:       info.Type.String(),
		Tag:        info.Tag,
		ID:         uint32(id),
		AttachedTo: []string{}, // Would need to track attachments
		LoadedAt:   time.Now(), // Would need to track actual load time
		RunCount:   0,          // Would need to track runs
		LastRun:    time.Time{},
	}, nil
}

// GetProgram returns the underlying eBPF program
func (p *ProgramWrapper) GetProgram() *ebpf.Program {
	return p.program
}
