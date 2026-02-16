// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux
// +build !linux

package performance

import (
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// HardwareOffload manages hardware offload capabilities (Stub)
type HardwareOffload struct {
	config *HardwareOffloadConfig
	logger *logging.Logger
}

func NewHardwareOffload(logger *logging.Logger, config *HardwareOffloadConfig) *HardwareOffload {
	return &HardwareOffload{config: config, logger: logger}
}

func (ho *HardwareOffload) Start() error {
	ho.logger.Debug("Hardware offload not supported on this platform")
	return nil
}

func (ho *HardwareOffload) Stop() {}

func (ho *HardwareOffload) ShouldOffload(key types.FlowKey, flowState *types.FlowState, packetRate uint64) bool {
	return false
}

func (ho *HardwareOffload) OffloadFlow(key types.FlowKey, flowState *types.FlowState, device string) error {
	return nil
}

func (ho *HardwareOffload) UpdateFlow(key types.FlowKey, flowState *types.FlowState) error {
	return nil
}

func (ho *HardwareOffload) RemoveFlow(key types.FlowKey) error {
	return nil
}

func (ho *HardwareOffload) GetStatistics() *HardwareOffloadStats {
	return &HardwareOffloadStats{}
}

func (ho *HardwareOffload) GetCapabilities() *HardwareCapabilities {
	return &HardwareCapabilities{}
}

func (ho *HardwareOffload) GetOffloadedFlows() []*OffloadedFlow {
	return nil
}

func (ho *HardwareOffload) SetEnabled(enabled bool) {}
