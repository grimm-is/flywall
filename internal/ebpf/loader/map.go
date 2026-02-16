// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package loader

import (
    "time"

    "github.com/cilium/ebpf"
    "grimm.is/flywall/internal/ebpf/interfaces"
)

// MapWrapper wraps an eBPF map to implement the interfaces.Map interface
type MapWrapper struct {
    ebpfMap *ebpf.Map
}

// NewMapWrapper creates a new map wrapper
func NewMapWrapper(m *ebpf.Map) *MapWrapper {
    return &MapWrapper{ebpfMap: m}
}

// Info returns information about the map
func (m *MapWrapper) Info() (interfaces.MapInfo, error) {
    info, err := m.ebpfMap.Info()
    if err != nil {
        return interfaces.MapInfo{}, err
    }

    return interfaces.MapInfo{
        Name:         info.Name,
        Type:         info.Type.String(),
        KeySize:      uint32(info.KeySize),
        ValueSize:    uint32(info.ValueSize),
        MaxEntries:   info.MaxEntries,
        Flags:        uint32(info.Flags),
        CreatedAt:    time.Now(), // Would need to track actual creation time
        LastAccessed: time.Time{},
    }, nil
}

// GetMap returns the underlying eBPF map
func (m *MapWrapper) GetMap() *ebpf.Map {
    return m.ebpfMap
}
