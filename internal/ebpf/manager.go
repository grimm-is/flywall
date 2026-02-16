// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ebpf

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cilium/ebpf"

	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/ebpf/coordinator"
	"grimm.is/flywall/internal/ebpf/flow"
	"grimm.is/flywall/internal/ebpf/hooks"
	"grimm.is/flywall/internal/ebpf/interfaces"
	"grimm.is/flywall/internal/ebpf/ips"
	"grimm.is/flywall/internal/ebpf/loader"
	"grimm.is/flywall/internal/ebpf/maps"
	"grimm.is/flywall/internal/ebpf/programs"
	"grimm.is/flywall/internal/ebpf/qos"
	"grimm.is/flywall/internal/ebpf/socket"
	"grimm.is/flywall/internal/ebpf/stats"
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/learning"
	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/services/ebpf/dns_blocklist"
)

// Manager is the main eBPF manager that coordinates all components
type Manager struct {
	config      *Config
	loader      interfaces.Loader
	mapManager  *maps.Manager
	hookManager *hooks.Manager
	coordinator *coordinator.Coordinator

	// Feature-specific managers
	tcProgram      *programs.TCOffloadProgram
	flowManager    *flow.Manager
	qosManager     *qos.Manager
	socketManager  *socket.Manager
	ipsIntegration *ips.Integration

	// Embedded programs
	programs map[string]*ebpf.Program

	// Feature services
	dnsBlocklist *dns_blocklist.Service

	// State
	loaded  bool
	running bool
	mutex   sync.RWMutex

	// Event handling
	eventChan chan Event

	// Statistics
	stats          *Statistics
	statsLock      sync.RWMutex
	statsCollector *stats.Collector
	statsExporter  *stats.Exporter
	exporterCancel context.CancelFunc

	alerts *alerting.Engine
}

// NewManager creates a new eBPF manager
func NewManager(config *Config, alerts *alerting.Engine) (*Manager, error) {
	// Verify kernel support
	if err := loader.VerifyKernelSupport(); err != nil {
		return nil, fmt.Errorf("kernel support verification failed: %w", err)
	}

	// Enable JIT if configured
	if config.Performance.MaxCPUPercent > 50 {
		if err := loader.EnableJIT(); err != nil {
			return nil, fmt.Errorf("failed to enable JIT: %w", err)
		}
	}

	// Create components
	m := &Manager{
		config:    config,
		loader:    loader.NewLoader(),
		programs:  make(map[string]*ebpf.Program),
		eventChan: make(chan Event, 1000),
		stats:     &Statistics{},
		alerts:    alerts,
	}

	// Initialize statistics collector
	m.statsCollector = stats.NewCollector()

	// Initialize statistics exporter with default config
	exportConfig := stats.DefaultExportConfig()
	if config.StatsExport != nil {
		exportConfig = *config.StatsExport
	}
	m.statsExporter = stats.NewExporter(m.statsCollector, exportConfig)

	// Create coordinator
	coordConfig := &coordinator.CoordinatorConfig{
		Adaptive:           config.Adaptive.Enabled,
		ScaleBackThreshold: config.Adaptive.ScaleBackThreshold,
		ScaleBackRate:      config.Adaptive.ScaleBackRate,
		MinimumFeatures:    config.Adaptive.MinimumFeatures,
		SamplingConfig: coordinator.SamplingConfig{
			Enabled:       config.Adaptive.SamplingConfig.Enabled,
			MinSampleRate: config.Adaptive.SamplingConfig.MinSampleRate,
			MaxSampleRate: config.Adaptive.SamplingConfig.MaxSampleRate,
			AdaptiveRate:  config.Adaptive.SamplingConfig.AdaptiveRate,
		},
		Performance: coordinator.PerformanceConfig{
			MaxCPUPercent:   config.Performance.MaxCPUPercent,
			MaxMemoryMB:     config.Performance.MaxMemoryMB,
			MaxEventsPerSec: config.Performance.MaxEventsPerSec,
			MaxPPS:          float64(config.Performance.MaxPPS),
		},
	}
	m.coordinator = coordinator.NewCoordinator(coordConfig)

	return m, nil
}

// Load loads all eBPF programs and maps
func (m *Manager) Load() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.loaded {
		return fmt.Errorf("already loaded")
	}

	// Load embedded programs
	if err := m.loadEmbeddedPrograms(); err != nil {
		return fmt.Errorf("failed to load embedded programs: %w", err)
	}

	// Create map manager
	collection, err := m.getCollection()
	if err != nil {
		return err
	}
	m.mapManager = maps.NewManager(collection)

	// Register maps
	if err := m.registerMaps(); err != nil {
		return fmt.Errorf("failed to register maps: %w", err)
	}

	// Initialize feature services
	if err := m.initializeServices(); err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}

	// Create hook manager
	m.hookManager = hooks.NewManager()

	// Register programs with hook manager
	for name, program := range m.programs {
		m.hookManager.RegisterProgram(name, program)
	}

	// Register features with coordinator
	if err := m.registerFeatures(); err != nil {
		return fmt.Errorf("failed to register features: %w", err)
	}

	// Validate configuration
	if err := m.coordinator.ValidateConfiguration(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	m.loaded = true
	return nil
}

// Start activates all enabled features
func (m *Manager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.loaded {
		return fmt.Errorf("not loaded")
	}

	if m.running {
		return fmt.Errorf("already running")
	}

	// Seed flow map from kernel conntrack
	if m.flowManager != nil {
		if err := m.syncConntrackFlows(); err != nil {
			logging.Default().Warn("Failed to seed flow map from conntrack", "error", err)
		}
	}

	// Start feature services
	if m.dnsBlocklist != nil {
		if err := m.dnsBlocklist.Start(context.Background()); err != nil {
			return fmt.Errorf("failed to start DNS blocklist service: %w", err)
		}
	}

	// Start TC fast path
	if m.tcProgram != nil {
		// Attach to all available interfaces
		interfaces, err := net.Interfaces()
		if err != nil {
			return fmt.Errorf("failed to get network interfaces: %w", err)
		}

		for _, iface := range interfaces {
			// Skip loopback and non-up interfaces
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}

			if err := m.tcProgram.Attach(iface.Name); err != nil {
				logging.Default().Warn("Failed to attach TC program",
					"interface", iface.Name,
					"error", err)
			}
		}
	}

	// Start flow manager
	if m.flowManager != nil {
		if err := m.flowManager.Start(); err != nil {
			return fmt.Errorf("failed to start flow manager: %w", err)
		}
	}

	// Start socket manager
	if m.socketManager != nil {
		if err := m.socketManager.Start(); err != nil {
			return fmt.Errorf("failed to start socket manager: %w", err)
		}
	}

	// Start IPS integration
	if m.ipsIntegration != nil {
		if err := m.ipsIntegration.Start(); err != nil {
			return fmt.Errorf("failed to start IPS integration: %w", err)
		}
	}

	// Update interface list
	if err := m.hookManager.UpdateInterfaces(); err != nil {
		return fmt.Errorf("failed to update interfaces: %w", err)
	}

	// Attach hooks based on enabled features
	if err := m.attachHooks(); err != nil {
		return fmt.Errorf("failed to attach hooks: %w", err)
	}

	// Start event processor
	m.eventChan = make(chan Event, 1000)
	go m.processEvents()

	// Start statistics collector
	go m.collectStatistics()

	// Start statistics exporter
	exporterCtx, exporterCancel := context.WithCancel(context.Background())
	m.exporterCancel = exporterCancel
	if err := m.statsExporter.Start(exporterCtx); err != nil {
		logging.Default().Error("Failed to start statistics exporter", "error", err)
	}

	// Start adaptive controller if enabled
	if m.config.Adaptive.Enabled {
		go m.runAdaptiveController()
	}

	m.running = true
	return nil
}

// Stop deactivates all features
func (m *Manager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return nil
	}

	// Stop IPS integration
	if m.ipsIntegration != nil {
		m.ipsIntegration.Stop()
	}

	// Stop flow manager
	if m.flowManager != nil {
		m.flowManager.Stop()
	}

	// Stop socket manager
	if m.socketManager != nil {
		m.socketManager.Stop()
	}

	// Detach TC programs
	if m.tcProgram != nil {
		if err := m.tcProgram.Detach(); err != nil {
			logging.Default().Error("Error detaching TC program", "error", err)
		}
	}

	// Stop feature services
	if m.dnsBlocklist != nil {
		m.dnsBlocklist.Stop(context.Background())
	}

	// Stop statistics exporter
	if m.exporterCancel != nil {
		m.exporterCancel()
	}
	if m.statsExporter != nil {
		m.statsExporter.Stop()
	}

	// Detach hooks
	if err := m.hookManager.DetachAll(); err != nil {
		return fmt.Errorf("failed to detach hooks: %w", err)
	}

	// Close event channel
	close(m.eventChan)

	m.running = false
	return nil
}

// Close closes the eBPF manager and cleans up resources
func (m *Manager) Close() error {
	if err := m.Stop(); err != nil {
		return err
	}

	var firstErr error

	// Close components
	if m.loader != nil {
		if err := m.loader.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if m.hookManager != nil {
		if err := m.hookManager.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if m.coordinator != nil {
		if err := m.coordinator.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	m.loaded = false
	return firstErr
}

// loadEmbeddedPrograms loads embedded eBPF programs
func (m *Manager) loadEmbeddedPrograms() error {
	// Load TC offload program if enabled
	if m.config.Features["tc_offload"].Enabled {
		spec, err := programs.LoadTcOffload()
		if err != nil {
			return fmt.Errorf("failed to load TC offload spec: %w", err)
		}

		// Disable pinning for all maps
		for _, mapSpec := range spec.Maps {
			mapSpec.Pinning = ebpf.PinNone
		}

		// Workaround for tc_stats_map issues on some kernels/environments
		// This matches logic in NewTCOffloadProgram
		if m, ok := spec.Maps["tc_stats_map"]; ok {
			m.Type = ebpf.Array
		}

		// Load the collection via the loader
		if err := m.loader.LoadCollection(spec); err != nil {
			return fmt.Errorf("failed to load TC offload collection: %w", err)
		}

		return nil
	}

	// Load XDP blocklist program if enabled
	if m.config.Features["ddos_protection"].Enabled {
		spec, err := programs.LoadXdpBlocklist()
		if err != nil {
			return fmt.Errorf("failed to load XDP blocklist spec: %w", err)
		}

		// Disable pinning for all maps
		for _, mapSpec := range spec.Maps {
			mapSpec.Pinning = ebpf.PinNone
		}

		// Load the collection via the loader
		if err := m.loader.LoadCollection(spec); err != nil {
			return fmt.Errorf("failed to load XDP blocklist collection: %w", err)
		}
	}

	// Load XDP blocklist program if enabled
	if m.config.Features["ddos_protection"].Enabled {
		spec, err := programs.LoadXdpBlocklist()
		if err != nil {
			return fmt.Errorf("failed to load XDP blocklist spec: %w", err)
		}

		// Disable pinning for all maps
		for _, mapSpec := range spec.Maps {
			mapSpec.Pinning = ebpf.PinNone
		}

		// Load the collection via the loader
		if err := m.loader.LoadCollection(spec); err != nil {
			return fmt.Errorf("failed to load XDP blocklist collection: %w", err)
		}
	}

	// For other features, we would load their programs here
	// For now, if no features are enabled, create an empty collection
	// to avoid "collection not available" errors
	if m.loader.GetCollection() == nil {
		// Create a minimal spec with just a stats map
		spec := &ebpf.CollectionSpec{
			Maps: map[string]*ebpf.MapSpec{
				"statistics": {
					Type:       ebpf.Array,
					KeySize:    4,
					ValueSize:  8,
					MaxEntries: 10,
				},
			},
		}
		if err := m.loader.LoadCollection(spec); err != nil {
			return fmt.Errorf("failed to load minimal collection: %w", err)
		}
	}

	return nil
}

// getCollection returns the eBPF collection
func (m *Manager) getCollection() (*ebpf.Collection, error) {
	collection := m.loader.GetCollection()
	if collection == nil {
		return nil, fmt.Errorf("collection not loaded")
	}
	return collection, nil
}

// initializeServices initializes feature-specific services
func (m *Manager) initializeServices() error {
	// Initialize DNS blocklist service if enabled
	if m.config.Features["dns_blocklist"].Enabled {
		config := &dns_blocklist.Config{
			BloomSize:       131072, // 1MB default
			HashCount:       7,
			Sources:         []string{},
			UpdateInterval:  time.Hour,
			MaxDomains:      100000,
			CleanupInterval: time.Hour,
		}

		// Extract configuration from feature config
		if featureConfig, ok := m.config.Features["dns_blocklist"]; ok {
			if bloomSize, ok := featureConfig.Config["bloom_size"].(float64); ok {
				config.BloomSize = uint32(bloomSize)
			}
			if hashCount, ok := featureConfig.Config["hash_count"].(float64); ok {
				config.HashCount = uint32(hashCount)
			}
			if sources, ok := featureConfig.Config["sources"].([]interface{}); ok {
				for _, s := range sources {
					if source, ok := s.(string); ok {
						config.Sources = append(config.Sources, source)
					}
				}
			}
		}

		service := dns_blocklist.NewService(m.loader, logging.Default(), config)
		m.dnsBlocklist = service
	}

	// Initialize socket filters if enabled
	if m.config.Features["socket_filters"].Enabled {
		// Prepare socket filter config
		socketConfig := socket.DefaultSocketFilterConfig()
		socketConfig.Enabled = true

		// We could populate more details from m.config.Features["socket_filters"].Config if needed

		m.socketManager = socket.NewManager(logging.Default(), socketConfig, m.alerts)
	}

	// Initialize TC fast path if enabled
	if m.config.Features["tc_offload"].Enabled {
		tcProgram, err := programs.NewTCOffloadProgram(logging.Default())
		if err != nil {
			return fmt.Errorf("failed to create TC offload program: %w", err)
		}
		m.tcProgram = tcProgram

		// Register collection with statistics collector
		if collection := tcProgram.GetCollection(); collection != nil {
			m.statsCollector.RegisterCollection("tc_offload", collection)
		}

		// Get flow map from TC program
		flowMap := tcProgram.FlowMap()
		if flowMap != nil {
			// Create flow manager
			flowConfig := flow.DefaultConfig()
			m.flowManager = flow.NewManager(flowMap, logging.Default(), flowConfig)
		}

		// Create QoS manager
		qosConfig := qos.DefaultConfig()
		m.qosManager = qos.NewManager(tcProgram, logging.Default(), qosConfig)

		// Create IPS integration if enabled
		if m.config.Features["ips_integration"].Enabled {
			// Get learning engine from control plane
			// This will be injected later via SetLearningEngine
			ipsConfig := ips.DefaultConfig()

			// Extract configuration from feature config
			if featureConfig, ok := m.config.Features["ips_integration"]; ok {
				if inspectionWindow, ok := featureConfig.Config["inspection_window"].(float64); ok {
					ipsConfig.InspectionWindow = int(inspectionWindow)
				}
				if offloadThreshold, ok := featureConfig.Config["offload_threshold"].(float64); ok {
					ipsConfig.OffloadThreshold = int(offloadThreshold)
				}
			}

			// IPS integration will be created after learning engine is available
			// See SetLearningEngine method
			m.ipsIntegration = ips.NewIntegration(
				tcProgram,
				m.flowManager,
				nil, // Will be set later
				logging.Default(),
				ipsConfig,
			)
		}
	}

	return nil
}

// registerMaps registers all maps with the map manager
func (m *Manager) registerMaps() error {
	// Register maps
	flowMap, err := m.loader.GetMap("flow_map")
	if err == nil {
		m.mapManager.RegisterMap("flow_map", flowMap.GetMap())
	}

	statsMap, err := m.loader.GetMap("stats_map")
	if err == nil {
		m.mapManager.RegisterMap("stats_map", statsMap.GetMap())
	}

	dnsMap, err := m.loader.GetMap("dns_blocklist")
	if err == nil {
		m.mapManager.RegisterMap("dns_blocklist", dnsMap.GetMap())
	}

	if ipBlocklist, err := m.loader.GetMap("ip_blocklist"); err == nil {
		if err := m.mapManager.RegisterMap("ip_blocklist", ipBlocklist.GetMap()); err != nil {
			return err
		}
	}

	if ipBlocklist, err := m.loader.GetMap("ip_blocklist"); err == nil {
		if err := m.mapManager.RegisterMap("ip_blocklist", ipBlocklist.GetMap()); err != nil {
			return err
		}
	}

	if dnsBloom, err := m.loader.GetMap("dns_bloom"); err == nil {
		if err := m.mapManager.RegisterMap("dns_bloom", dnsBloom.GetMap()); err != nil {
			return err
		}
	}

	return nil
}

// registerFeatures registers all features with the coordinator
func (m *Manager) registerFeatures() error {
	for name, featureConfig := range m.config.Features {
		if !featureConfig.Enabled {
			continue
		}

		feature := &coordinator.Feature{
			Name:         name,
			Enabled:      featureConfig.Enabled,
			Priority:     featureConfig.Priority,
			Dependencies: m.getFeatureDependencies(name),
			Cost:         m.getFeatureCost(name),
			Status: coordinator.FeatureStatus{
				Active:       false,
				SamplingRate: 1.0,
			},
		}

		if err := m.coordinator.RegisterFeature(feature); err != nil {
			return err
		}
	}

	return nil
}

// getFeatureDependencies returns the dependencies for a feature
func (m *Manager) getFeatureDependencies(name string) []string {
	switch name {
	case "inline_ips":
		return []string{"flow_monitoring"}
	case "qos":
		return []string{"flow_monitoring"}
	case "learning_engine":
		return []string{"flow_monitoring", "statistics"}
	case "tls_fingerprinting":
		return []string{"socket_filters"}
	default:
		return nil
	}
}

// getFeatureCost returns the resource cost for a feature
func (m *Manager) getFeatureCost(name string) coordinator.ResourceCost {
	// These would be determined empirically
	costs := map[string]coordinator.ResourceCost{
		"ddos_protection": {
			CPU:          5.0,
			Memory:       10,
			MapLookups:   2.0,
			EventsPerSec: 100,
			MaxPPS:       10.0,
		},
		"dns_blocklist": {
			CPU:          2.0,
			Memory:       5,
			MapLookups:   1.0,
			EventsPerSec: 50,
			MaxPPS:       5.0,
		},
		"inline_ips": {
			CPU:          15.0,
			Memory:       50,
			MapLookups:   5.0,
			EventsPerSec: 500,
			MaxPPS:       2.0,
		},
		"flow_monitoring": {
			CPU:          10.0,
			Memory:       30,
			MapLookups:   3.0,
			EventsPerSec: 200,
			MaxPPS:       5.0,
		},
		"qos": {
			CPU:          5.0,
			Memory:       20,
			MapLookups:   2.0,
			EventsPerSec: 100,
			MaxPPS:       5.0,
		},
		"learning_engine": {
			CPU:          20.0,
			Memory:       100,
			MapLookups:   1.0,
			EventsPerSec: 1000,
			MaxPPS:       1.0,
		},
		"tls_fingerprinting": {
			CPU:          8.0,
			Memory:       15,
			MapLookups:   2.0,
			EventsPerSec: 150,
			MaxPPS:       3.0,
		},
		"device_discovery": {
			CPU:          3.0,
			Memory:       10,
			MapLookups:   1.0,
			EventsPerSec: 25,
			MaxPPS:       1.0,
		},
		"statistics": {
			CPU:          5.0,
			Memory:       25,
			MapLookups:   1.0,
			EventsPerSec: 0,
			MaxPPS:       10.0,
		},
	}

	return costs[name]
}

// attachHooks attaches all necessary hooks
func (m *Manager) attachHooks() error {
	// Attach XDP blocklist
	if m.config.Features["ddos_protection"].Enabled {
		config := &types.HookConfig{
			ProgramName: "xdp_blocklist",
			ProgramType: types.ProgramTypeXDP,
			AttachPoint: "eth0",
			AutoReplace: true,
		}

		if err := m.hookManager.Attach(config); err != nil {
			return fmt.Errorf("failed to attach XDP blocklist: %w", err)
		}
	}

	// Attach TC classifier
	if m.config.Features["inline_ips"].Enabled {
		config := &types.HookConfig{
			ProgramName: "tc_classifier",
			ProgramType: types.ProgramTypeTC,
			AttachPoint: "eth0",
			AutoReplace: true,
		}

		if err := m.hookManager.Attach(config); err != nil {
			return fmt.Errorf("failed to attach TC classifier: %w", err)
		}
	}

	// Attach socket filters
	if m.config.Features["dns_blocklist"].Enabled {
		config := &types.HookConfig{
			ProgramName: "socket_dns",
			ProgramType: types.ProgramTypeSocketFilter,
			AttachPoint: "3", // stdin
			AutoReplace: true,
		}

		if err := m.hookManager.Attach(config); err != nil {
			return fmt.Errorf("failed to attach DNS socket filter: %w", err)
		}
	}

	return nil
}

// processEvents processes events from eBPF programs
func (m *Manager) processEvents() {
	for event := range m.eventChan {
		// Handle event
		switch event.Type {
		case types.EventTypeFlowCreated:
			m.handleFlowCreated(event)
		case types.EventTypeFlowExpired:
			m.handleFlowExpired(event)
		case types.EventTypeDNSQuery:
			m.handleDNSQuery(event)
		case types.EventTypeTLSHandshake:
			m.handleTLSHandshake(event)
		case types.EventTypeAlert:
			m.handleAlert(event)
		}
	}
}

// collectStatistics collects statistics from eBPF maps
func (m *Manager) collectStatistics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.updateStatistics()
	}
}

// updateStatistics updates the statistics from maps
func (m *Manager) updateStatistics() {
	m.statsLock.Lock()
	defer m.statsLock.Unlock()

	// Update from statistics map
	if statsMap, err := m.mapManager.GetMap("statistics"); err == nil {
		var key uint32
		var value uint64

		// Packets processed
		key = 0
		if err := statsMap.Lookup(&key, &value); err == nil {
			m.stats.PacketsProcessed = value
		}

		// Packets dropped
		key = 1
		if err := statsMap.Lookup(&key, &value); err == nil {
			m.stats.PacketsDropped = value
		}

		// Packets passed
		key = 2
		if err := statsMap.Lookup(&key, &value); err == nil {
			m.stats.PacketsPassed = value
		}
	}
}

// runAdaptiveController runs the adaptive performance controller
func (m *Manager) runAdaptiveController() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Check if we need to scale back
		if err := m.coordinator.ScaleBack(); err != nil {
			// Log error
			continue
		}

		// Check if we can scale up
		if err := m.coordinator.ScaleUp(); err != nil {
			// Log error
			continue
		}
	}
}

// UpdateBlocklist updates the IP blocklist map
// UpdateBlocklist updates the IP blocklist map
// Note: Implementation moved to end of file to avoid duplication

func (m *Manager) syncConntrackFlows() error {
	// This would typically use internal/kernel/provider_linux.go
	// Since we are inside the manager, we'll assume a Linux provider is available
	// or passed in. For now, we'll implement a basic sync logic.

	logging.Default().Info("Synchronizing flow state with kernel conntrack")

	// Implementation placeholder - in a real system we would:
	// 1. Get flows from kernel provider
	// 2. Map them to types.FlowKey and types.FlowState
	// 3. Call m.flowManager.CreateFlow for each

	return nil
}

// Event handlers
func (m *Manager) handleFlowCreated(event Event) {
	// Update flow statistics
	m.statsLock.Lock()
	m.stats.EventsGenerated++
	m.statsLock.Unlock()
}

func (m *Manager) handleFlowExpired(event Event) {
	// Clean up flow
}

func (m *Manager) handleDNSQuery(event Event) {
	// Process DNS query
	m.statsLock.Lock()
	m.stats.EventsGenerated++
	m.statsLock.Unlock()
}

func (m *Manager) handleTLSHandshake(event Event) {
	// Process TLS handshake
	m.statsLock.Lock()
	m.stats.EventsGenerated++
	m.statsLock.Unlock()
}

func (m *Manager) handleAlert(event Event) {
	// Handle alert
}

// GetStatistics returns current statistics
func (m *Manager) GetStatistics() *interfaces.Statistics {
	// Get statistics from collector
	stats := m.statsCollector.Collect()

	// Update with local counters
	m.statsLock.RLock()
	stats.PacketsProcessed = m.stats.PacketsProcessed
	stats.PacketsDropped = m.stats.PacketsDropped
	stats.PacketsPassed = m.stats.PacketsPassed
	stats.BytesProcessed = m.stats.BytesProcessed
	m.statsLock.RUnlock()

	return stats
}

// GetFeatureStatus returns the status of all features
func (m *Manager) GetFeatureStatus() map[string]interfaces.FeatureStatus {
	coordStatus := m.coordinator.GetFeatureStatus()
	result := make(map[string]interfaces.FeatureStatus)

	for name, status := range coordStatus {
		result[name] = interfaces.FeatureStatus{
			Enabled:      true, // If it's in the coordinator, it's enabled
			Active:       status.Active,
			SamplingRate: status.SamplingRate,
			PacketCount:  0, // Not tracked in coordinator
			DropCount:    0, // Not tracked in coordinator
			ErrorCount:   0, // Not tracked in coordinator
			LastError:    status.Error,
		}
	}

	return result
}

// GetMapInfo returns information about all maps
func (m *Manager) GetMapInfo() map[string]interfaces.MapInfo {
	mapStats := m.mapManager.GetStats()
	result := make(map[string]interfaces.MapInfo)

	for name, stats := range mapStats {
		result[name] = interfaces.MapInfo{
			Name:         stats.Name,
			Type:         stats.Type,
			KeySize:      stats.KeySize,
			ValueSize:    stats.ValueSize,
			MaxEntries:   stats.MaxEntries,
			Flags:        0, // Not available in maps.MapInfo
			CreatedAt:    stats.CreatedAt,
			LastAccessed: stats.LastAccessed,
		}
	}

	return result
}

// GetHookInfo returns information about all hooks
func (m *Manager) GetHookInfo() map[string]interface{} {
	info := make(map[string]interface{})

	if m.hookManager != nil {
		attached := m.hookManager.ListAttached()
		for name, hook := range attached {
			info[name] = map[string]interface{}{
				"attached":    hook.Active,
				"iface":       hook.AttachPoint,
				"type":        hook.Type.String(),
				"attached_at": hook.AttachedAt,
			}
		}
	}

	return info
}

// EnableFeature enables a feature
func (m *Manager) EnableFeature(name string) error {
	if err := m.coordinator.EnableFeature(name); err != nil {
		return err
	}

	// Attach hook if needed
	return m.attachFeatureHook(name)
}

// DisableFeature disables a feature
func (m *Manager) DisableFeature(name string) error {
	// Detach hook
	if err := m.detachFeatureHook(name); err != nil {
		return err
	}

	return m.coordinator.DisableFeature(name, false)
}

// attachFeatureHook attaches the hook for a feature
func (m *Manager) attachFeatureHook(name string) error {
	// This would attach the appropriate hook for the feature
	return nil
}

// detachFeatureHook detaches the hook for a feature
func (m *Manager) detachFeatureHook(name string) error {
	// This would detach the appropriate hook for the feature
	return nil
}

// GetDNSBlocklistService returns the DNS blocklist service
func (m *Manager) GetDNSBlocklistService() interfaces.DNSBlocklistService {
	return m.dnsBlocklist
}

// GetTCProgram returns the TC offload program
func (m *Manager) GetTCProgram() *programs.TCOffloadProgram {
	return m.tcProgram
}

// GetFlowManager returns the flow manager
func (m *Manager) GetFlowManager() *flow.Manager {
	return m.flowManager
}

// GetQoSManager returns the QoS manager
func (m *Manager) GetQoSManager() *qos.Manager {
	return m.qosManager
}

// SetLearningEngine sets the learning engine for IPS integration
func (m *Manager) SetLearningEngine(engine interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.ipsIntegration == nil {
		return nil // IPS integration not enabled
	}

	// Type assert to learning engine
	learningEngine, ok := engine.(*learning.Engine)
	if !ok {
		return fmt.Errorf("invalid learning engine type")
	}

	// Create a new IPS integration with the learning engine
	ipsConfig := ips.DefaultConfig()
	if m.config.Features["ips_integration"].Enabled {
		if featureConfig, ok := m.config.Features["ips_integration"]; ok {
			if inspectionWindow, ok := featureConfig.Config["inspection_window"].(float64); ok {
				ipsConfig.InspectionWindow = int(inspectionWindow)
			}
			if offloadThreshold, ok := featureConfig.Config["offload_threshold"].(float64); ok {
				ipsConfig.OffloadThreshold = int(offloadThreshold)
			}
		}
	}

	// Replace the IPS integration
	m.ipsIntegration = ips.NewIntegration(
		m.tcProgram,
		m.flowManager,
		learningEngine,
		logging.Default(),
		ipsConfig,
	)

	// Start IPS integration if manager is running
	if m.running {
		if err := m.ipsIntegration.Start(); err != nil {
			return fmt.Errorf("failed to start IPS integration: %w", err)
		}
	}

	return nil
}

// GetIPSIntegration returns the IPS integration
func (m *Manager) GetIPSIntegration() *ips.Integration {
	return m.ipsIntegration
}

// UpdateBlocklist updates the IP blocklist map
func (m *Manager) UpdateBlocklist(ips []string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.mapManager == nil {
		return fmt.Errorf("map manager not initialized")
	}

	blocklistMap, err := m.mapManager.GetMap("ip_blocklist")
	if err != nil {
		return fmt.Errorf("ip_blocklist map not found: %w", err)
	}

	// Batch update not supported for hash map, so we update one by one
	now := uint64(time.Now().UnixNano())

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			logging.Default().Warn("Invalid IP in blocklist update", "ip", ipStr)
			continue
		}

		// Convert to uint32 (assuming IPv4 for now as per xdp_blocklist.c)
		ip4 := ip.To4()
		if ip4 == nil {
			logging.Default().Warn("Skipping non-IPv4 IP in blocklist update", "ip", ipStr)
			continue
		}

		// Correct byte order for map key (little endian for xdp_blocklist.c?)
		// See previous reasoning: use LittleEndian to reconstruct the memory layout
		keyVal := uint32(ip4[0]) | uint32(ip4[1])<<8 | uint32(ip4[2])<<16 | uint32(ip4[3])<<24

		if err := blocklistMap.Update(&keyVal, &now); err != nil {
			logging.Default().Warn("Failed to update blocklist for IP", "ip", ipStr, "error", err)
		}
	}

	logging.Default().Info("Updated blocklist", "count", len(ips))
	return nil
}
