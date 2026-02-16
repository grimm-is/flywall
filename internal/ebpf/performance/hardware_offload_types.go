// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package performance

import (
	"time"

	"grimm.is/flywall/internal/ebpf/types"
)

// HardwareOffloadConfig for hardware offload configuration
type HardwareOffloadConfig struct {
	Enabled             bool          `json:"enabled"`
	AutoDetect          bool          `json:"auto_detect"`
	ForceOffload        bool          `json:"force_offload"`
	MinFlowRate         uint64        `json:"min_flow_rate"`
	OffloadThreshold    int           `json:"offload_threshold"`
	MaxOffloadedFlows   int           `json:"max_offloaded_flows"`
	OffloadTimeout      time.Duration `json:"offload_timeout"`
	SyncInterval        time.Duration `json:"sync_interval"`
	EnableEncapOffload  bool          `json:"enable_encap_offload"`
	EnableDecapOffload  bool          `json:"enable_decap_offload"`
	EnableVXLANOffload  bool          `json:"enable_vxlan_offload"`
	EnableGeneveOffload bool          `json:"enable_geneve_offload"`
}

// DefaultHardwareOffloadConfig returns default hardware offload configuration
func DefaultHardwareOffloadConfig() *HardwareOffloadConfig {
	return &HardwareOffloadConfig{
		Enabled:             true,
		AutoDetect:          true,
		ForceOffload:        false,
		MinFlowRate:         1000, // packets per second
		OffloadThreshold:    100,
		MaxOffloadedFlows:   10000,
		OffloadTimeout:      30 * time.Second,
		SyncInterval:        5 * time.Second,
		EnableEncapOffload:  true,
		EnableDecapOffload:  true,
		EnableVXLANOffload:  true,
		EnableGeneveOffload: false,
	}
}

// HardwareCapabilities represents hardware offload capabilities
type HardwareCapabilities struct {
	TCOffload     bool     `json:"tc_offload"`
	FlowOffload   bool     `json:"flow_offload"`
	EncapOffload  bool     `json:"encap_offload"`
	DecapOffload  bool     `json:"decap_offload"`
	VXLANOffload  bool     `json:"vxlan_offload"`
	GeneveOffload bool     `json:"geneve_offload"`
	MaxFlows      int      `json:"max_flows"`
	MaxActions    int      `json:"max_actions"`
	EncapTypes    []string `json:"encap_types"`
	DecapTypes    []string `json:"decap_types"`
}

// OffloadedFlow represents an offloaded flow
type OffloadedFlow struct {
	Key         types.FlowKey   `json:"key"`
	Device      string          `json:"device"`
	Handle      uint32          `json:"handle"`
	OffloadTime time.Time       `json:"offload_time"`
	LastSeen    time.Time       `json:"last_seen"`
	PacketCount uint64          `json:"packet_count"`
	ByteCount   uint64          `json:"byte_count"`
	Actions     []OffloadAction `json:"actions"`
}

// OffloadAction represents an offload action
type OffloadAction struct {
	Type   string      `json:"type"`
	Params interface{} `json:"params"`
}

// HardwareOffloadStats tracks hardware offload statistics
type HardwareOffloadStats struct {
	TotalOffloads      uint64        `json:"total_offloads"`
	SuccessfulOffloads uint64        `json:"successful_offloads"`
	FailedOffloads     uint64        `json:"failed_offloads"`
	OffloadedFlows     uint64        `json:"offloaded_flows"`
	OffloadedPackets   uint64        `json:"offloaded_packets"`
	OffloadedBytes     uint64        `json:"offloaded_bytes"`
	OffloadHits        uint64        `json:"offload_hits"`
	OffloadMisses      uint64        `json:"offload_misses"`
	AvgOffloadTime     time.Duration `json:"avg_offload_time"`
	LastUpdate         time.Time     `json:"last_update"`
}
