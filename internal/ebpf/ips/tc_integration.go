// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ips

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"grimm.is/flywall/internal/ebpf/flow"
	"grimm.is/flywall/internal/ebpf/programs"
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// TCIPSIntegration enhances TCOffloadProgram with IPS capabilities
type TCIPSIntegration struct {
	tcProgram   *programs.TCOffloadProgram
	flowManager *flow.Manager
	ips         *Integration
	logger      *logging.Logger

	// Configuration
	config *IPSConfig

	// State
	mutex   sync.RWMutex
	enabled bool

	// Packet processing
	processing sync.WaitGroup
	stopCh     chan struct{}
}

// IPSConfig for TC IPS integration
type IPSConfig struct {
	Enabled          bool          `json:"enabled"`
	InspectionWindow int           `json:"inspection_window"`
	OffloadThreshold int           `json:"offload_threshold"`
	MaxPendingFlows  int           `json:"max_pending_flows"`
	CleanupInterval  time.Duration `json:"cleanup_interval"`
}

// DefaultIPSConfig returns default IPS configuration
func DefaultIPSConfig() *IPSConfig {
	return &IPSConfig{
		Enabled:          true,
		InspectionWindow: 10,
		OffloadThreshold: 5,
		MaxPendingFlows:  10000,
		CleanupInterval:  5 * time.Minute,
	}
}

// NewTCIPSIntegration creates a new TC IPS integration
func NewTCIPSIntegration(
	tcProgram *programs.TCOffloadProgram,
	flowManager *flow.Manager,
	logger *logging.Logger,
	config *IPSConfig,
) *TCIPSIntegration {
	if config == nil {
		config = DefaultIPSConfig()
	}

	return &TCIPSIntegration{
		tcProgram:   tcProgram,
		flowManager: flowManager,
		logger:      logger,
		config:      config,
		stopCh:      make(chan struct{}),
	}
}

// SetIPS sets the IPS integration module
func (t *TCIPSIntegration) SetIPS(ips *Integration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.ips = ips
}

// Start starts the TC IPS integration
func (t *TCIPSIntegration) Start() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.config.Enabled {
		t.logger.Info("TC IPS integration disabled")
		return nil
	}

	if t.tcProgram == nil {
		return fmt.Errorf("TC program not available")
	}

	if t.flowManager == nil {
		return fmt.Errorf("flow manager not available")
	}

	if t.ips == nil {
		return fmt.Errorf("IPS integration not available")
	}

	t.enabled = true
	t.logger.Info("TC IPS integration started")

	return nil
}

// Stop stops the TC IPS integration
func (t *TCIPSIntegration) Stop() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.enabled {
		return
	}

	t.enabled = false
	close(t.stopCh)
	t.processing.Wait()

	t.logger.Info("TC IPS integration stopped")
}

// ProcessPacket processes a packet through IPS if needed
func (t *TCIPSIntegration) ProcessPacket(key types.FlowKey, packetInfo *SKBInfo) (types.FlowState, error) {
	t.mutex.RLock()
	enabled := t.enabled
	ipsModule := t.ips
	t.mutex.RUnlock()

	if !enabled || ipsModule == nil {
		// Default to trusted if IPS is not available
		return types.FlowState{
			Verdict: uint8(types.VerdictTrusted),
		}, nil
	}

	// Process through IPS
	return ipsModule.ProcessPacket(key, packetInfo)
}

// EnhancedTCProgram wraps TCOffloadProgram with IPS capabilities
type EnhancedTCProgram struct {
	*programs.TCOffloadProgram
	ips    *TCIPSIntegration
	logger *logging.Logger
}

// NewEnhancedTCProgram creates a TC program with IPS integration
func NewEnhancedTCProgram(
	logger *logging.Logger,
	flowManager *flow.Manager,
	ipsConfig *IPSConfig,
) (*EnhancedTCProgram, error) {
	// Create base TC program
	tcProgram, err := programs.NewTCOffloadProgram(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create TC program: %w", err)
	}

	// Create IPS integration
	ipsIntegration := NewTCIPSIntegration(tcProgram, flowManager, logger, ipsConfig)

	enhanced := &EnhancedTCProgram{
		TCOffloadProgram: tcProgram,
		ips:              ipsIntegration,
		logger:           logger,
	}

	return enhanced, nil
}

// SetIPS sets the IPS integration module
func (e *EnhancedTCProgram) SetIPS(ips *Integration) {
	e.ips.SetIPS(ips)
}

// Start starts the enhanced TC program
func (e *EnhancedTCProgram) Start() error {
	// Start IPS integration
	return e.ips.Start()
}

// Stop stops the enhanced TC program
func (e *EnhancedTCProgram) Stop() error {
	// Stop IPS integration
	e.ips.Stop()

	return nil
}

// ProcessPacketWithIPS processes a packet with IPS inspection
func (e *EnhancedTCProgram) ProcessPacketWithIPS(key types.FlowKey, packetInfo *SKBInfo) (types.FlowState, error) {
	return e.ips.ProcessPacket(key, packetInfo)
}

// AttachWithIPS attaches the TC program and enables IPS processing
func (e *EnhancedTCProgram) AttachWithIPS(ifaceName string) error {
	// For non-Linux platforms, just log and return
	if runtime.GOOS != "linux" {
		e.logger.Info("Skipping TC IPS attachment on non-Linux platform", "interface", ifaceName)
		return nil
	}

	// Attach base TC program
	return e.TCOffloadProgram.Attach(ifaceName)
}

// GetIPSStatistics returns IPS integration statistics
func (e *EnhancedTCProgram) GetIPSStatistics() *Statistics {
	if e.ips.ips != nil {
		return e.ips.ips.GetStatistics()
	}
	return nil
}
