// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ebpf

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zclconf/go-cty/cty"
	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ebpf/interfaces"
	"grimm.is/flywall/internal/ebpf/stats"
	"grimm.is/flywall/internal/logging"
)

// Integration handles eBPF integration with the control plane
type Integration struct {
	manager *Manager
	logger  logging.Logger
	config  *config.Config
	alerts  *alerting.Engine
	mutex   sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewIntegration creates a new eBPF integration
func NewIntegration(cfg *config.Config, logger logging.Logger, alerts *alerting.Engine) (*Integration, error) {
	// Convert config to eBPF config
	ebpfConfig := convertConfig(cfg)

	// Create eBPF manager
	manager, err := NewManager(ebpfConfig, alerts)
	if err != nil {
		return nil, fmt.Errorf("failed to create eBPF manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Integration{
		manager: manager,
		logger:  logger,
		config:  cfg,
		alerts:  alerts,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Start starts the eBPF integration
func (i *Integration) Start() error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.logger.Info("Starting eBPF integration")

	// Load eBPF programs
	if err := i.manager.Load(); err != nil {
		if err.Error() != "already loaded" {
			return fmt.Errorf("failed to load eBPF programs: %w", err)
		}
	}

	// Start eBPF manager
	if err := i.manager.Start(); err != nil {
		return fmt.Errorf("failed to start eBPF manager: %w", err)
	}

	i.logger.Info("eBPF integration started successfully")
	return nil
}

// Stop stops the eBPF integration
func (i *Integration) Stop() error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.logger.Info("Stopping eBPF integration")

	// Cancel context
	i.cancel()

	// Stop eBPF manager
	if err := i.manager.Stop(); err != nil {
		i.logger.Error("Failed to stop eBPF manager", "error", err)
		return err
	}

	i.logger.Info("eBPF integration stopped")
	return nil
}

// Close closes the eBPF integration
func (i *Integration) Close() error {
	if i.manager != nil {
		return i.manager.Close()
	}
	return nil
}

// GetStatistics returns eBPF statistics
func (i *Integration) GetStatistics() *interfaces.Statistics {
	if i.manager != nil {
		return i.manager.GetStatistics()
	}
	return &interfaces.Statistics{}
}

// GetFeatureStatus returns the status of eBPF features
func (i *Integration) GetFeatureStatus() map[string]interfaces.FeatureStatus {
	if i.manager != nil {
		return i.manager.GetFeatureStatus()
	}
	return nil
}

// EnableFeature enables an eBPF feature
func (i *Integration) EnableFeature(name string) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if i.manager == nil {
		return fmt.Errorf("eBPF manager not initialized")
	}

	i.logger.Info("Enabling eBPF feature", "feature", name)

	if err := i.manager.EnableFeature(name); err != nil {
		i.logger.Error("Failed to enable eBPF feature", "feature", name, "error", err)
		return err
	}

	i.logger.Info("eBPF feature enabled", "feature", name)
	return nil
}

// DisableFeature disables an eBPF feature
func (i *Integration) DisableFeature(name string) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if i.manager == nil {
		return fmt.Errorf("eBPF manager not initialized")
	}

	i.logger.Info("Disabling eBPF feature", "feature", name)

	if err := i.manager.DisableFeature(name); err != nil {
		i.logger.Error("Failed to disable eBPF feature", "feature", name, "error", err)
		return err
	}

	i.logger.Info("eBPF feature disabled", "feature", name)
	return nil
}

// UpdateConfig updates the eBPF configuration
func (i *Integration) UpdateConfig(cfg *config.Config) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.logger.Info("Updating eBPF configuration")

	// Convert new config
	ebpfConfig := convertConfig(cfg)

	// For now, we would need to restart the manager
	// In the future, implement hot reloading
	if i.manager != nil {
		if err := i.manager.Close(); err != nil {
			i.logger.Error("Failed to close eBPF manager", "error", err)
			return err
		}
	}

	// Create new manager
	manager, err := NewManager(ebpfConfig, i.alerts)
	if err != nil {
		return fmt.Errorf("failed to create new eBPF manager: %w", err)
	}

	i.manager = manager
	i.config = cfg

	// Restart if we were running
	if err := i.manager.Load(); err != nil {
		return fmt.Errorf("failed to load eBPF programs: %w", err)
	}

	if err := i.manager.Start(); err != nil {
		return fmt.Errorf("failed to start eBPF manager: %w", err)
	}

	i.logger.Info("eBPF configuration updated")
	return nil
}

// convertConfig converts control plane config to eBPF config
func convertConfig(cfg *config.Config) *Config {
	ebpfConfig := &Config{
		Features: make(map[string]FeatureConfig),
	}

	// Check if EBPF config exists
	if cfg.EBPF != nil {
		ebpfConfig.Enabled = cfg.EBPF.Enabled

		// Copy performance config if it exists
		if cfg.EBPF.Performance != nil {
			ebpfConfig.Performance = PerformanceConfig{
				MaxCPUPercent:   cfg.EBPF.Performance.MaxCPUPercent,
				MaxMemoryMB:     cfg.EBPF.Performance.MaxMemoryMB,
				MaxEventsPerSec: cfg.EBPF.Performance.MaxEventsPerSec,
				MaxPPS:          cfg.EBPF.Performance.MaxPPS,
			}
		}

		// Copy adaptive config if it exists
		if cfg.EBPF.Adaptive != nil {
			ebpfConfig.Adaptive = AdaptiveConfig{
				Enabled:            cfg.EBPF.Adaptive.Enabled,
				ScaleBackThreshold: cfg.EBPF.Adaptive.ScaleBackThreshold,
				ScaleBackRate:      cfg.EBPF.Adaptive.ScaleBackRate,
				MinimumFeatures:    cfg.EBPF.Adaptive.MinimumFeatures,
				SamplingConfig: SamplingConfig{
					Enabled:       cfg.EBPF.Adaptive.Sampling != nil && cfg.EBPF.Adaptive.Sampling.Enabled,
					MinSampleRate: getSamplingRate(cfg.EBPF.Adaptive.Sampling, true),
					MaxSampleRate: getSamplingRate(cfg.EBPF.Adaptive.Sampling, false),
					AdaptiveRate:  cfg.EBPF.Adaptive.Sampling != nil && cfg.EBPF.Adaptive.Sampling.AdaptiveRate,
				},
			}
		}

		// Copy maps config if it exists
		if cfg.EBPF.Maps != nil {
			ebpfConfig.Maps = MapConfig{
				MaxMaps:       cfg.EBPF.Maps.MaxMaps,
				MaxMapEntries: cfg.EBPF.Maps.MaxMapEntries,
				MaxMapMemory:  cfg.EBPF.Maps.MaxMapMemory,
				CacheSize:     cfg.EBPF.Maps.CacheSize,
			}
		}

		// Copy programs config if it exists
		if cfg.EBPF.Programs != nil {
			ebpfConfig.Programs = ProgramConfig{
				XDPBlocklist: cfg.EBPF.Programs.XDPBlocklist,
				TCClassifier: cfg.EBPF.Programs.TCClassifier,
				SocketDNS:    cfg.EBPF.Programs.SocketDNS,
				SocketTLS:    cfg.EBPF.Programs.SocketTLS,
				SocketDHCP:   cfg.EBPF.Programs.SocketDHCP,
			}
		}

		// Copy stats export config if it exists
		if cfg.EBPF.StatsExport != nil {
			ebpfConfig.StatsExport = &stats.ExportConfig{
				EnablePrometheus: cfg.EBPF.StatsExport.EnablePrometheus,
				PrometheusPort:   cfg.EBPF.StatsExport.PrometheusPort,
				EnableJSON:       cfg.EBPF.StatsExport.EnableJSON,
				JSONEndpoint:     cfg.EBPF.StatsExport.JSONEndpoint,
				ExportInterval:   10 * time.Second,
			}
		}
	}

	// Convert feature configurations if EBPF config exists
	if cfg.EBPF != nil && cfg.EBPF.Features != nil {
		for _, feature := range cfg.EBPF.Features {
			configMap := make(map[string]interface{})
			if !feature.Config.IsNull() {
				// Convert cty.Value to map[string]interface{}
				if err := cty.Walk(feature.Config, func(path cty.Path, value cty.Value) (bool, error) {
					if len(path) == 1 {
						if name, ok := path[0].(cty.GetAttrStep); ok {
							switch value.Type() {
							case cty.String:
								configMap[name.Name] = value.AsString()
							case cty.Number:
								f, _ := value.AsBigFloat().Float64()
								configMap[name.Name] = f
							case cty.Bool:
								configMap[name.Name] = value.True()
							}
						}
					}
					return true, nil
				}); err != nil {
					fmt.Printf("Failed to convert feature config: %v\n", err)
				}
			}

			ebpfConfig.Features[feature.Name] = FeatureConfig{
				Enabled:  feature.Enabled,
				Priority: feature.Priority,
				Config:   configMap,
			}
		}
	}

	return ebpfConfig
}

// getSamplingRate extracts a sampling rate from config
func getSamplingRate(cfg *config.EBPFSamplingConfig, isMin bool) float64 {
	if cfg == nil {
		if isMin {
			return 0.1
		}
		return 1.0
	}
	if isMin {
		return cfg.MinSampleRate
	}
	return cfg.MaxSampleRate
}

// IsEnabled returns true if eBPF is enabled
func (i *Integration) IsEnabled() bool {
	if i.config != nil && i.config.EBPF != nil {
		return i.config.EBPF.Enabled
	}
	return false
}

// GetMapInfo returns information about eBPF maps
func (i *Integration) GetMapInfo() map[string]interfaces.MapInfo {
	if i.manager != nil {
		return i.manager.GetMapInfo()
	}
	return nil
}

// GetHookInfo returns information about eBPF hooks
func (i *Integration) GetHookInfo() map[string]interface{} {
	if i.manager != nil {
		return i.manager.GetHookInfo()
	}
	return nil
}

// HealthCheck performs a health check on the eBPF integration
func (i *Integration) HealthCheck() error {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	if i.manager == nil {
		return fmt.Errorf("eBPF manager not initialized")
	}

	// Check if manager is loaded and running
	// This would need to be implemented in the manager
	return nil
}

// UpdateBlocklist updates the IP blocklist
func (i *Integration) UpdateBlocklist(ips []string) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if i.manager == nil {
		return fmt.Errorf("eBPF manager not initialized")
	}

	i.logger.Info("Updating blocklist", "count", len(ips))

	if err := i.manager.UpdateBlocklist(ips); err != nil {
		i.logger.Error("Failed to update blocklist", "error", err)
		return err
	}

	return nil
}
