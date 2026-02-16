// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux
// +build linux

package performance

import (
	"fmt"
	"net"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/safchain/ethtool"
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// HardwareOffload manages hardware offload capabilities
type HardwareOffload struct {
	// Configuration
	config *HardwareOffloadConfig

	// State
	mutex   sync.RWMutex
	enabled bool

	// Hardware capabilities
	capabilities *HardwareCapabilities

	// Offloaded flows
	offloadedFlows map[uint64]*OffloadedFlow

	// Statistics
	stats *HardwareOffloadStats

	// Logger
	logger *logging.Logger
}

// NewHardwareOffload creates a new hardware offload manager
func NewHardwareOffload(logger *logging.Logger, config *HardwareOffloadConfig) *HardwareOffload {
	if config == nil {
		config = DefaultHardwareOffloadConfig()
	}

	ho := &HardwareOffload{
		config:         config,
		offloadedFlows: make(map[uint64]*OffloadedFlow),
		stats:          &HardwareOffloadStats{},
		logger:         logger,
	}

	// Detect hardware capabilities
	if config.AutoDetect {
		ho.detectCapabilities()
	}

	return ho
}

// Start starts the hardware offload manager
func (ho *HardwareOffload) Start() error {
	ho.mutex.Lock()
	defer ho.mutex.Unlock()

	if !ho.config.Enabled {
		ho.logger.Info("Hardware offload disabled")
		return nil
	}

	// Check if hardware supports offload
	if !ho.hasHardwareSupport() {
		ho.logger.Warn("Hardware offload not supported by hardware")
		ho.enabled = false
		return nil
	}

	ho.enabled = true

	// Start background tasks
	go ho.syncWorker()
	go ho.cleanupWorker()

	ho.logger.Info("Hardware offload started",
		"tc_offload", ho.capabilities.TCOffload,
		"flow_offload", ho.capabilities.FlowOffload,
		"max_flows", ho.capabilities.MaxFlows)

	return nil
}

// Stop stops the hardware offload manager
func (ho *HardwareOffload) Stop() {
	ho.mutex.Lock()
	defer ho.mutex.Unlock()

	if !ho.enabled {
		return
	}

	ho.enabled = false

	// Remove all offloaded flows
	for _, flow := range ho.offloadedFlows {
		ho.removeOffloadedFlow(flow)
	}

	ho.logger.Info("Hardware offload stopped")
}

// ShouldOffload determines if a flow should be offloaded
func (ho *HardwareOffload) ShouldOffload(key types.FlowKey, flowState *types.FlowState, packetRate uint64) bool {
	ho.mutex.RLock()
	defer ho.mutex.RUnlock()

	if !ho.enabled || !ho.hasHardwareSupport() {
		return false
	}

	// Check if already offloaded
	if _, exists := ho.offloadedFlows[key.Hash()]; exists {
		atomic.AddUint64(&ho.stats.OffloadHits, 1)
		return true
	}

	atomic.AddUint64(&ho.stats.OffloadMisses, 1)

	// Check flow rate threshold
	if packetRate < ho.config.MinFlowRate {
		return false
	}

	// Check flow state
	if flowState.Verdict != types.VerdictTrusted {
		return false
	}

	// Check if flow is marked for offload
	if flowState.Flags&types.FlowFlagOffloaded == 0 {
		return false
	}

	// Check maximum offloaded flows
	if len(ho.offloadedFlows) >= ho.config.MaxOffloadedFlows {
		return false
	}

	return true
}

// OffloadFlow offloads a flow to hardware
func (ho *HardwareOffload) OffloadFlow(key types.FlowKey, flowState *types.FlowState, device string) error {
	start := time.Now()

	ho.mutex.Lock()
	defer ho.mutex.Unlock()

	if !ho.enabled {
		return fmt.Errorf("hardware offload not enabled")
	}

	// Check if already offloaded
	if _, exists := ho.offloadedFlows[key.Hash()]; exists {
		return fmt.Errorf("flow already offloaded")
	}

	// Create offloaded flow
	flow := &OffloadedFlow{
		Key:         key,
		Device:      device,
		Handle:      ho.generateHandle(),
		OffloadTime: time.Now(),
		LastSeen:    time.Now(),
		Actions:     ho.buildOffloadActions(flowState),
	}

	// Install flow in hardware
	if err := ho.installHardwareFlow(flow); err != nil {
		atomic.AddUint64(&ho.stats.FailedOffloads, 1)
		return fmt.Errorf("failed to install hardware flow: %w", err)
	}

	// Track offloaded flow
	ho.offloadedFlows[key.Hash()] = flow
	atomic.AddUint64(&ho.stats.SuccessfulOffloads, 1)
	atomic.AddUint64(&ho.stats.TotalOffloads, 1)
	atomic.AddUint64(&ho.stats.OffloadedFlows, 1)

	// Update statistics
	offloadTime := time.Since(start)
	ho.updateAvgOffloadTime(offloadTime)

	ho.logger.Info("Flow offloaded to hardware",
		"flow", key.String(),
		"device", device,
		"handle", flow.Handle,
		"time", offloadTime)

	return nil
}

// UpdateFlow updates an offloaded flow
func (ho *HardwareOffload) UpdateFlow(key types.FlowKey, flowState *types.FlowState) error {
	ho.mutex.Lock()
	defer ho.mutex.Unlock()

	flow, exists := ho.offloadedFlows[key.Hash()]; if !exists {
		return fmt.Errorf("flow not offloaded")
	}

	// Update flow statistics
	flow.LastSeen = time.Now()
	flow.Actions = ho.buildOffloadActions(flowState)

	// Update hardware flow
	if err := ho.updateHardwareFlow(flow); err != nil {
		return fmt.Errorf("failed to update hardware flow: %w", err)
	}

	return nil
}

// RemoveFlow removes an offloaded flow
func (ho *HardwareOffload) RemoveFlow(key types.FlowKey) error {
	ho.mutex.Lock()
	defer ho.mutex.Unlock()

	flow, exists := ho.offloadedFlows[key.Hash()]
	if !exists {
		return nil
	}

	return ho.removeOffloadedFlow(flow)
}

// removeOffloadedFlow removes an offloaded flow (internal, assumes lock held)
func (ho *HardwareOffload) removeOffloadedFlow(flow *OffloadedFlow) error {
	// Remove from hardware
	if err := ho.removeHardwareFlow(flow); err != nil {
		ho.logger.Warn("Failed to remove hardware flow",
			"flow", flow.Key.String(),
			"error", err)
	}

	// Remove from tracking
	delete(ho.offloadedFlows, flow.Key.Hash())
	atomic.AddUint64(&ho.stats.OffloadedFlows, ^uint64(0))

	return nil
}

// detectCapabilities detects hardware offload capabilities
func (ho *HardwareOffload) detectCapabilities() {
	ho.capabilities = &HardwareCapabilities{
		TCOffload:     ho.detectTCOffload(),
		FlowOffload:   ho.detectFlowOffload(),
		EncapOffload:  ho.detectEncapOffload(),
		DecapOffload:  ho.detectDecapOffload(),
		VXLANOffload:  ho.detectVXLANOffload(),
		GeneveOffload: ho.detectGeneveOffload(),
		MaxFlows:      ho.detectMaxFlows(),
		MaxActions:    ho.detectMaxActions(),
		EncapTypes:    ho.detectEncapTypes(),
		DecapTypes:    ho.detectDecapTypes(),
	}

	ho.logger.Info("Hardware capabilities detected",
		"tc_offload", ho.capabilities.TCOffload,
		"flow_offload", ho.capabilities.FlowOffload,
		"max_flows", ho.capabilities.MaxFlows)
}

// detectTCOffload detects TC offload capability across all interfaces
func (ho *HardwareOffload) detectTCOffload() bool {
	eth, err := ethtool.NewEthtool()
	if err != nil {
		ho.logger.Warn("Failed to create ethtool handle", "error", err)
		return false
	}
	defer eth.Close()

	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	for _, iface := range ifaces {
		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		features, err := eth.Features(iface.Name)
		if err != nil {
			continue
		}

		// Look for hw-tc-offload
		if enabled, ok := features["hw-tc-offload"]; ok && enabled {
			return true
		}
	}

	return false
}

// detectFlowOffload detects flow offload capability
func (ho *HardwareOffload) detectFlowOffload() bool {
	return ho.detectTCOffload()
}

// detectEncapOffload detects encapsulation offload capability
func (ho *HardwareOffload) detectEncapOffload() bool {
	eth, err := ethtool.NewEthtool()
	if err != nil {
		return false
	}
	defer eth.Close()

	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		features, err := eth.Features(iface.Name)
		if err != nil {
			continue
		}
		// Check for common encapsulation offload features
		if (features["tx-udp_tnl-segmentation"] || features["tx-gre-segmentation"]) {
			return true
		}
	}
	return false
}

// detectDecapOffload detects decapsulation offload capability
func (ho *HardwareOffload) detectDecapOffload() bool {
	// Similar to encap but for receive side
	return ho.detectEncapOffload() 
}

// detectVXLANOffload detects VXLAN offload capability
func (ho *HardwareOffload) detectVXLANOffload() bool {
	eth, err := ethtool.NewEthtool()
	if err != nil {
		return false
	}
	defer eth.Close()

	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		features, err := eth.Features(iface.Name)
		if err != nil {
			continue
		}
		if features["tx-udp_tnl-segmentation"] || features["rx-udp_tnl-l4-csum"] {
			return true
		}
	}
	return false
}

// detectGeneveOffload detects Geneve offload capability
func (ho *HardwareOffload) detectGeneveOffload() bool {
	// Often tied to the same hardware features as VXLAN
	return ho.detectVXLANOffload()
}

// detectMaxFlows detects maximum number of offloaded flows
func (ho *HardwareOffload) detectMaxFlows() int {
	// Ideally we'd query via devlink or similar, but for now 
	// we use a safe default based on typical NIC capacities.
	return 100000 
}

// detectMaxActions detects maximum number of actions per flow
func (ho *HardwareOffload) detectMaxActions() int {
	return 32
}

// detectEncapTypes detects supported encapsulation types
func (ho *HardwareOffload) detectEncapTypes() []string {
	types := []string{"vxlan"}
	if ho.capabilities != nil && ho.capabilities.GeneveOffload {
		types = append(types, "geneve")
	}
	return types
}

// detectDecapTypes detects supported decapsulation types
func (ho *HardwareOffload) detectDecapTypes() []string {
	return []string{"vxlan", "geneve"}
}

// hasHardwareSupport checks if hardware supports offload
func (ho *HardwareOffload) hasHardwareSupport() bool {
	if ho.capabilities == nil {
		return false
	}
	return ho.capabilities.TCOffload || ho.capabilities.FlowOffload
}

var staticHandleCounter uint32 = 1000

// generateHandle generates a unique handle for offloaded flow
func (ho *HardwareOffload) generateHandle() uint32 {
	return atomic.AddUint32(&staticHandleCounter, 1)
}

// buildOffloadActions builds offload actions from flow state
func (ho *HardwareOffload) buildOffloadActions(flowState *types.FlowState) []OffloadAction {
	actions := make([]OffloadAction, 0)

	// Add QoS action
	if flowState.Verdict != 0 { // Just an example check
		actions = append(actions, OffloadAction{
			Type: "qos",
			Params: map[string]interface{}{
				"verdict": flowState.Verdict,
			},
		})
	}

	// Add offload flag action
	if flowState.Flags&types.FlowFlagOffloaded != 0 {
		actions = append(actions, OffloadAction{
			Type: "mark",
			Params: map[string]interface{}{
				"flags": flowState.Flags,
			},
		})
	}

	return actions
}

// installHardwareFlow installs a flow in hardware
func (ho *HardwareOffload) installHardwareFlow(flow *OffloadedFlow) error {
	srcIP := flow.Key.SrcIP
	dstIP := flow.Key.DstIP
	
	srcStr := fmt.Sprintf("%d.%d.%d.%d", byte(srcIP>>24), byte(srcIP>>16), byte(srcIP>>8), byte(srcIP))
	dstStr := fmt.Sprintf("%d.%d.%d.%d", byte(dstIP>>24), byte(dstIP>>16), byte(dstIP>>8), byte(dstIP))

	cmdArgs := []string{
		"filter", "add", "dev", flow.Device, "ingress",
		"protocol", "ip", "pref", "1", "flower",
		"src_ip", srcStr,
		"dst_ip", dstStr,
		"skip_sw",
		"action", "drop",
	}

	if flow.Key.SrcPort != 0 {
		cmdArgs = append(cmdArgs, "src_port", fmt.Sprintf("%d", flow.Key.SrcPort))
	}
	if flow.Key.DstPort != 0 {
		cmdArgs = append(cmdArgs, "dst_port", fmt.Sprintf("%d", flow.Key.DstPort))
	}

	cmd := exec.Command("tc", cmdArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		ho.logger.Error("tc add failed", "cmd", cmd.String(), "output", string(output), "error", err)
		return fmt.Errorf("tc command failed: %w (output: %s)", err, string(output))
	}

	ho.logger.Debug("Installed hardware flow via tc",
		"flow", flow.Key.String(),
		"handle", flow.Handle)

	return nil
}

// updateHardwareFlow updates a flow in hardware
func (ho *HardwareOffload) updateHardwareFlow(flow *OffloadedFlow) error {
	// tc flower usually requires replace for updates
	srcIP := flow.Key.SrcIP
	dstIP := flow.Key.DstIP
	
	srcStr := fmt.Sprintf("%d.%d.%d.%d", byte(srcIP>>24), byte(srcIP>>16), byte(srcIP>>8), byte(srcIP))
	dstStr := fmt.Sprintf("%d.%d.%d.%d", byte(dstIP>>24), byte(dstIP>>16), byte(dstIP>>8), byte(dstIP))

	cmdArgs := []string{
		"filter", "replace", "dev", flow.Device, "ingress",
		"protocol", "ip", "pref", "1", "handle", fmt.Sprintf("%d", flow.Handle), "flower",
		"src_ip", srcStr,
		"dst_ip", dstStr,
		"skip_sw",
		"action", "drop",
	}

	cmd := exec.Command("tc", cmdArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tc replace failed: %w (output: %s)", err, string(output))
	}
	return nil
}

// removeHardwareFlow removes a flow from hardware
func (ho *HardwareOffload) removeHardwareFlow(flow *OffloadedFlow) error {
	cmdArgs := []string{
		"filter", "del", "dev", flow.Device, "ingress",
		"pref", "1", "handle", fmt.Sprintf("%d", flow.Handle), "flower",
	}
	cmd := exec.Command("tc", cmdArgs...)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Ignore if already deleted
		return nil
	}
	return nil
}

// syncWorker synchronizes offloaded flows with hardware
func (ho *HardwareOffload) syncWorker() {
	ticker := time.NewTicker(ho.config.SyncInterval)
	defer ticker.Stop()

	for range ticker.C {
		ho.syncFlows()
	}
}

// cleanupWorker cleans up expired offloaded flows
func (ho *HardwareOffload) cleanupWorker() {
	ticker := time.NewTicker(ho.config.OffloadTimeout)
	defer ticker.Stop()

	for range ticker.C {
		ho.cleanupExpiredFlows()
	}
}

// syncFlows synchronizes offloaded flows
func (ho *HardwareOffload) syncFlows() {
	ho.mutex.RLock()
	devices := make(map[string]bool)
	for _, flow := range ho.offloadedFlows {
		devices[flow.Device] = true
	}
	ho.mutex.RUnlock()

	for dev := range devices {
		// Query tc for filters on this device
		cmd := exec.Command("tc", "-s", "filter", "show", "dev", dev, "ingress")
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		// Basic parsing of tc output to update stats
		// In a real implementation, we would parse packet/byte counts
		ho.logger.Debug("Syncing flows for device", "device", dev, "output_len", len(output))
	}
}

// cleanupExpiredFlows cleans up expired offloaded flows
func (ho *HardwareOffload) cleanupExpiredFlows() {
	ho.mutex.Lock()
	defer ho.mutex.Unlock()

	now := time.Now()
	expired := make([]*OffloadedFlow, 0)

	for _, flow := range ho.offloadedFlows {
		if now.Sub(flow.LastSeen) > ho.config.OffloadTimeout {
			expired = append(expired, flow)
		}
	}

	for _, flow := range expired {
		ho.logger.Info("Removing expired offloaded flow",
			"flow", flow.Key.String(),
			"age", now.Sub(flow.LastSeen))
		ho.removeOffloadedFlow(flow)
	}
}

// updateAvgOffloadTime updates average offload time
func (ho *HardwareOffload) updateAvgOffloadTime(offloadTime time.Duration) {
	if ho.stats.AvgOffloadTime == 0 {
		ho.stats.AvgOffloadTime = offloadTime
	} else {
		alpha := 0.1
		ho.stats.AvgOffloadTime = time.Duration(
			float64(ho.stats.AvgOffloadTime)*(1-alpha) + float64(offloadTime)*alpha,
		)
	}
}

// GetStatistics returns hardware offload statistics
func (ho *HardwareOffload) GetStatistics() *HardwareOffloadStats {
	ho.mutex.RLock()
	defer ho.mutex.RUnlock()

	stats := *ho.stats
	stats.LastUpdate = time.Now()
	return &stats
}

// GetCapabilities returns hardware capabilities
func (ho *HardwareOffload) GetCapabilities() *HardwareCapabilities {
	ho.mutex.RLock()
	defer ho.mutex.RUnlock()
	return ho.capabilities
}

// GetOffloadedFlows returns all offloaded flows
func (ho *HardwareOffload) GetOffloadedFlows() []*OffloadedFlow {
	ho.mutex.RLock()
	defer ho.mutex.RUnlock()

	flows := make([]*OffloadedFlow, 0, len(ho.offloadedFlows))
	for _, flow := range ho.offloadedFlows {
		flows = append(flows, flow)
	}
	return flows
}

// SetEnabled enables or disables hardware offload
func (ho *HardwareOffload) SetEnabled(enabled bool) {
	ho.mutex.Lock()
	defer ho.mutex.Unlock()

	if ho.enabled == enabled {
		return
	}

	ho.enabled = enabled
	ho.logger.Info("Hardware offload", "enabled", enabled)

	if !enabled {
		// Remove all offloaded flows
		for _, flow := range ho.offloadedFlows {
			ho.removeOffloadedFlow(flow)
		}
	}
}
