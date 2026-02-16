// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package interfaces

import (
	"time"

	"github.com/cilium/ebpf"
)

// Program represents an eBPF program
type Program interface {
	Info() (ProgramInfo, error)
}

// Map represents an eBPF map
type Map interface {
	Info() (MapInfo, error)
	GetMap() *ebpf.Map
}

// ProgramInfo represents information about a program
type ProgramInfo struct {
	Name       string
	Type       string
	Tag        string
	ID         uint32
	AttachedTo []string
	LoadedAt   time.Time
	RunCount   uint64
	LastRun    time.Time
}

// MapInfo represents information about a map
type MapInfo struct {
	Name         string
	Type         string
	KeySize      uint32
	ValueSize    uint32
	MaxEntries   uint32
	Flags        uint32
	CreatedAt    time.Time
	LastAccessed time.Time
}

// Loader interface for eBPF loading operations
type Loader interface {
	LoadSpec(data []byte) (*ebpf.CollectionSpec, error)
	LoadCollection(spec *ebpf.CollectionSpec) error
	LoadProgram(name, progType, attachTo string) error
	GetProgram(name string) (Program, error)
	GetMap(name string) (Map, error)
	GetProgramInfo(name string) (ProgramInfo, error)
	GetMapInfo(name string) (MapInfo, error)
	GetCollection() *ebpf.Collection
	Close() error
}
