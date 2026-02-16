// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ctlplane

import (
	"fmt"
	"log"
	"sync"

	"grimm.is/flywall/internal/config"
)

// ApplyHook defines the signature for functions that are called when a configuration is applied.
type ApplyHook func(*config.Config) error

// ConfigManager handles the separation of staged (candidate) and running configurations.
type ConfigManager struct {
	mu         sync.RWMutex
	running    *config.Config
	staged     *config.Config
	hcl        *config.ConfigFile
	configFile string
	nm         *NetworkManager
	hooks      []ApplyHook
}

// NewConfigManager creates a new configuration manager.
func NewConfigManager(cfg *config.Config, configFile string, hcl *config.ConfigFile, nm *NetworkManager) *ConfigManager {
	// Clone both configs to ensure clean serializable copies
	running := cfg.Clone()
	if running == nil {
		log.Printf("[CM] CRITICAL: Initial running config clone failed")
		running = cfg // Fallback to original if clone fails
	}
	staged := cfg.Clone()
	if staged == nil {
		log.Printf("[CM] CRITICAL: Initial staged config clone failed")
		staged = cfg // Fallback to original if clone fails
	}
	return &ConfigManager{
		running:    running,
		staged:     staged,
		configFile: configFile,
		hcl:        hcl,
		nm:         nm,
	}
}

// GetStaged returns a read-only view of the staged configuration.
func (cm *ConfigManager) GetStaged() *config.Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if cm.staged == nil {
		log.Printf("[CM] CRITICAL: Staged config is nil!")
		return nil
	}
	clone := cm.staged.Clone()
	if clone == nil {
		log.Printf("[CM] CRITICAL: Failed to clone staged config!")
		return nil
	}
	return clone
}

// GetRunning returns a read-only view of the running configuration.
func (cm *ConfigManager) GetRunning() *config.Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if cm.running == nil {
		log.Printf("[CM] CRITICAL: Running config is nil!")
		return nil
	}
	clone := cm.running.Clone()
	if clone == nil {
		log.Printf("[CM] CRITICAL: Failed to clone running config!")
		return nil
	}
	return clone
}

// Stage executes a mutation function on the staged configuration.
func (cm *ConfigManager) Stage(fn func(*config.Config) error) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Apply mutation to a clone first for atomicity
	candidate := cm.staged.Clone()
	if err := fn(candidate); err != nil {
		return err
	}

	// Update staged config
	cm.staged = candidate

	// Sync to HCL persisted state if available
	if cm.hcl != nil {
		cm.hcl.Config = cm.staged
		if err := cm.hcl.SyncConfigToHCL(); err != nil {
			return fmt.Errorf("failed to sync HCL: %w", err)
		}
		if err := cm.hcl.Save(); err != nil {
			return fmt.Errorf("failed to persist staged changes: %w", err)
		}
	}

	return nil
}

// RegisterApplyHook registers a function to be called when configuration is applied.
func (cm *ConfigManager) RegisterApplyHook(fn ApplyHook) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.hooks = append(cm.hooks, fn)
}

// Apply promotes staged config to running and triggers hooks.
func (cm *ConfigManager) Apply() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	log.Printf("[CM] Applying staged configuration...")

	// 1. Network Changes (Atomic)
	if cm.nm != nil {
		if err := cm.nm.ApplyConfig(cm.staged); err != nil {
			return fmt.Errorf("failed to apply network configuration: %w", err)
		}
	}

	// 2. Call registered hooks (Firewall, DNS, DHCP, etc.)
	for _, hook := range cm.hooks {
		if err := hook(cm.staged); err != nil {
			log.Printf("[CM] Warning: apply hook failed: %v", err)
			// Decide if hook failure should stop application?
			// Usually hooks reload services, so we might want to continue.
		}
	}

	// 3. Update running config
	cm.running = cm.staged.Clone()

	// 4. Persistence
	if cm.hcl != nil {
		cm.hcl.Config = cm.running
		if err := cm.hcl.SyncConfigToHCL(); err != nil {
			log.Printf("[CM] Warning: failed to sync HCL: %v", err)
		}
		if err := cm.hcl.Save(); err != nil {
			log.Printf("[CM] Warning: failed to save HCL: %v", err)
		}
	}

	log.Printf("[CM] Configuration applied successfully")
	return nil
}

// Rollback discards staged changes and reverts to running state.
func (cm *ConfigManager) Rollback() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	log.Printf("[CM] Rolling back staged configuration...")
	cm.staged = cm.running.Clone()
	if cm.staged == nil {
		return fmt.Errorf("failed to clone running config for rollback")
	}

	if cm.hcl != nil {
		cm.hcl.Config = cm.staged
		if err := cm.hcl.SyncConfigToHCL(); err != nil {
			return fmt.Errorf("failed to sync HCL for rollback: %w", err)
		}
		if err := cm.hcl.Save(); err != nil {
			return fmt.Errorf("failed to persist rollback: %w", err)
		}
	}
	log.Printf("[CM] Rollback complete")

	return nil
}
