// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux
// +build !linux

package performance

import (
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// TCOptimizer optimizes TC program performance (Stub)
type TCOptimizer struct {
	config *TCOptimizerConfig
	logger *logging.Logger
}

func NewTCOptimizer(logger *logging.Logger, config *TCOptimizerConfig) *TCOptimizer {
	return &TCOptimizer{config: config, logger: logger}
}

func (opt *TCOptimizer) Start() error {
	opt.logger.Debug("TC optimizer not supported on this platform")
	return nil
}

func (opt *TCOptimizer) Stop() {}

func (opt *TCOptimizer) ProcessPacket(packet []byte, key types.FlowKey) *PacketResult {
	return &PacketResult{Action: "bypass"}
}

func (opt *TCOptimizer) ProcessBatch(packets []*PacketTask) *TCBatchResult {
	return &TCBatchResult{}
}

func (opt *TCOptimizer) GetMetrics() *TCMetrics {
	return &TCMetrics{}
}

func (opt *TCOptimizer) GetStatistics() *OptimizerStats {
	return &OptimizerStats{}
}
