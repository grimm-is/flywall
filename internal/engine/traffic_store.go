// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package engine

import (
	"net"
	"sync"
	"time"
)

// MemoryTrafficStore implements TrafficStore in memory
type MemoryTrafficStore struct {
	flows      []TrafficFlow
	flowMap    map[string]*TrafficFlow // For quick lookups and updates
	maxFlows   int
	mu         sync.RWMutex
	cleanup    time.Duration
	lastCleanup time.Time
}

// NewMemoryTrafficStore creates a new in-memory traffic store
func NewMemoryTrafficStore(maxFlows int, cleanupInterval time.Duration) *MemoryTrafficStore {
	store := &MemoryTrafficStore{
		flows:      make([]TrafficFlow, 0, maxFlows),
		flowMap:    make(map[string]*TrafficFlow),
		maxFlows:   maxFlows,
		cleanup:    cleanupInterval,
		lastCleanup: time.Now(),
	}
	
	// Start cleanup goroutine
	go store.cleanupLoop()
	
	return store
}

// StoreFlow stores a traffic flow
func (m *MemoryTrafficStore) StoreFlow(flow TrafficFlow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.flowKey(flow)
	
	// Check if flow exists
	if existing, exists := m.flowMap[key]; exists {
		// Update existing flow
		existing.Bytes += flow.Bytes
		existing.Packets += flow.Packets
		existing.Timestamp = flow.Timestamp
		existing.State = flow.State
		return nil
	}
	
	// Add new flow
	m.flows = append(m.flows, flow)
	m.flowMap[key] = &m.flows[len(m.flows)-1]
	
	// Check if we need to trim
	if len(m.flows) > m.maxFlows {
		m.trimOldestFlows()
	}
	
	return nil
}

// GetRecentFlows returns flows from the specified duration
func (m *MemoryTrafficStore) GetRecentFlows(duration time.Duration) ([]TrafficFlow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	cutoff := time.Now().Add(-duration)
	result := make([]TrafficFlow, 0)
	
	for _, flow := range m.flows {
		if flow.Timestamp.After(cutoff) {
			result = append(result, flow)
		}
	}
	
	return result, nil
}

// GetActiveFlows returns currently active flows
func (m *MemoryTrafficStore) GetActiveFlows() ([]TrafficFlow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]TrafficFlow, 0)
	
	for _, flow := range m.flows {
		// Consider flows active if seen in last 5 minutes
		if flow.Timestamp.After(time.Now().Add(-5 * time.Minute)) {
			result = append(result, flow)
		}
	}
	
	return result, nil
}

// flowKey creates a unique key for a flow
func (m *MemoryTrafficStore) flowKey(flow TrafficFlow) string {
	return string(flow.SrcIP) + ":" + string(flow.DstIP) + ":" + 
		   string(flow.Protocol) + ":" + string(flow.Interface)
}

// trimOldestFlows removes oldest flows to maintain max size
func (m *MemoryTrafficStore) trimOldestFlows() {
	if len(m.flows) <= m.maxFlows {
		return
	}
	
	// Sort by timestamp (simple approach - in production use a more efficient method)
	// For now, just remove the first quarter
	trimCount := m.maxFlows / 4
	
	for i := 0; i < trimCount; i++ {
		if len(m.flows) == 0 {
			break
		}
		
		flow := m.flows[0]
		key := m.flowKey(flow)
		delete(m.flowMap, key)
		m.flows = m.flows[1:]
	}
}

// cleanupLoop periodically cleans up old flows
func (m *MemoryTrafficStore) cleanupLoop() {
	ticker := time.NewTicker(m.cleanup)
	defer ticker.Stop()
	
	for range ticker.C {
		m.mu.Lock()
		cutoff := time.Now().Add(-m.cleanup)
		
		// Remove old flows
		activeFlows := make([]TrafficFlow, 0)
		for _, flow := range m.flows {
			if flow.Timestamp.After(cutoff) {
				activeFlows = append(activeFlows, flow)
			} else {
				key := m.flowKey(flow)
				delete(m.flowMap, key)
			}
		}
		
		m.flows = activeFlows
		m.lastCleanup = time.Now()
		m.mu.Unlock()
	}
}

// PrometheusTrafficStore integrates with Prometheus metrics
type PrometheusTrafficStore struct {
	*MemoryTrafficStore
	metrics TrafficMetrics
}

// TrafficMetrics defines the metrics interface
type TrafficMetrics interface {
	RecordFlow(flow TrafficFlow)
	GetFlowStats() FlowStats
}

// FlowStats contains flow statistics
type FlowStats struct {
	TotalFlows    uint64
	ActiveFlows   uint64
	BytesPerSec   float64
	PacketsPerSec float64
}

// NewPrometheusTrafficStore creates a store with Prometheus integration
func NewPrometheusTrafficStore(maxFlows int, cleanupInterval time.Duration, metrics TrafficMetrics) *PrometheusTrafficStore {
	return &PrometheusTrafficStore{
		MemoryTrafficStore: NewMemoryTrafficStore(maxFlows, cleanupInterval),
		metrics:           metrics,
	}
}

// StoreFlow stores a flow and records metrics
func (p *PrometheusTrafficStore) StoreFlow(flow TrafficFlow) error {
	// Record metrics
	if p.metrics != nil {
		p.metrics.RecordFlow(flow)
	}
	
	// Store in memory
	return p.MemoryTrafficStore.StoreFlow(flow)
}

// EbpfTrafficStore integrates with eBPF for real-time flow collection
type EbpfTrafficStore struct {
	*MemoryTrafficStore
	ebpfCollector EbpfCollector
}

// EbpfCollector defines the eBPF collection interface
type EbpfCollector interface {
	Start() error
	Stop() error
	GetFlows() ([]TrafficFlow, error)
	FlowEvents() <-chan TrafficFlow
}

// NewEbpfTrafficStore creates a store with eBPF integration
func NewEbpfTrafficStore(maxFlows int, cleanupInterval time.Duration, collector EbpfCollector) *EbpfTrafficStore {
	store := &EbpfTrafficStore{
		MemoryTrafficStore: NewMemoryTrafficStore(maxFlows, cleanupInterval),
		ebpfCollector:      collector,
	}
	
	// Start collecting flows
	go store.collectFlows()
	
	return store
}

// collectFlows collects flows from eBPF
func (e *EbpfTrafficStore) collectFlows() {
	if e.ebpfCollector == nil {
		return
	}
	
	// Start the collector
	if err := e.ebpfCollector.Start(); err != nil {
		return
	}
	
	defer e.ebpfCollector.Stop()
	
	// Listen for flow events
	flowChan := e.ebpfCollector.FlowEvents()
	
	for flow := range flowChan {
		e.StoreFlow(flow)
	}
}

// MockTrafficStore for testing
type MockTrafficStore struct {
	flows []TrafficFlow
}

// NewMockTrafficStore creates a mock store with predefined flows
func NewMockTrafficStore(flows []TrafficFlow) *MockTrafficStore {
	return &MockTrafficStore{
		flows: flows,
	}
}

// StoreFlow implements TrafficStore
func (m *MockTrafficStore) StoreFlow(flow TrafficFlow) error {
	m.flows = append(m.flows, flow)
	return nil
}

// GetRecentFlows implements TrafficStore
func (m *MockTrafficStore) GetRecentFlows(duration time.Duration) ([]TrafficFlow, error) {
	return m.flows, nil
}

// GetActiveFlows implements TrafficStore
func (m *MockTrafficStore) GetActiveFlows() ([]TrafficFlow, error) {
	return m.flows, nil
}

// CreateTestFlows creates test flows for demonstration
func CreateTestFlows() []TrafficFlow {
	now := time.Now()
	return []TrafficFlow{
		{
			Timestamp:   now.Add(-5 * time.Minute),
			SrcIP:       net.ParseIP("192.168.1.100"),
			DstIP:       net.ParseIP("10.0.0.10"),
			SrcPort:     54321,
			DstPort:     443,
			Protocol:    "tcp",
			Interface:  "eth0",
			Zone:        "lan",
			Bytes:       1024000,
			Packets:     1000,
			State:       "ESTABLISHED",
			Action:      "ACCEPT",
			MatchedRule: "allow-https-outbound",
		},
		{
			Timestamp:   now.Add(-2 * time.Minute),
			SrcIP:       net.ParseIP("192.168.1.101"),
			DstIP:       net.ParseIP("8.8.8.8"),
			SrcPort:     54322,
			DstPort:     53,
			Protocol:    "udp",
			Interface:  "eth0",
			Zone:        "lan",
			Bytes:       512,
			Packets:     4,
			State:       "NEW",
			Action:      "ACCEPT",
			MatchedRule: "allow-dns",
		},
		{
			Timestamp:   now.Add(-1 * time.Minute),
			SrcIP:       net.ParseIP("10.0.0.50"),
			DstIP:       net.ParseIP("192.168.1.100"),
			SrcPort:     22,
			DstPort:     22,
			Protocol:    "tcp",
			Interface:  "eth1",
			Zone:        "wan",
			Bytes:       2048,
			Packets:     20,
			State:       "NEW",
			Action:      "DROP",
			MatchedRule: "block-ssh-from-wan",
		},
		{
			Timestamp:   now.Add(-30 * time.Second),
			SrcIP:       net.ParseIP("192.168.1.200"),
			DstIP:       net.ParseIP("192.168.1.10"),
			SrcPort:     54323,
			DstPort:     3306,
			Protocol:    "tcp",
			Interface:  "eth0",
			Zone:        "lan",
			Bytes:       8192,
			Packets:     50,
			State:       "ESTABLISHED",
			Action:      "ACCEPT",
			MatchedRule: "allow-mysql-internal",
		},
	}
}
