// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux

package firewall

import (
	"context"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/services"
)

// Name returns the service name.
func (m *Manager) Name() string {
	return "Firewall"
}

// Start starts the firewall service.
// For nftables, the connection is already established, so we just return nil.
func (m *Manager) Start(ctx context.Context) error {
	return nil
}

// Stop stops the firewall service.
// We don't close the nftables connection as it's not supported by the library,
// and we generally want the firewall to persist even if the control plane stops.
//
// Restart safety: On daemon restart, ApplyConfig() performs a full table flush
// before re-applying rules, preventing duplicate or stale rules.
func (m *Manager) Stop(ctx context.Context) error {
	return nil
}

// Reload reloads the firewall configuration.
// It returns false because we support hot reloading (no restart required).
func (m *Manager) Reload(cfg *config.Config) (bool, error) {
	if cfg == nil {
		return false, nil
	}
	err := m.ApplyConfig(FromGlobalConfig(cfg))
	return false, err
}

// Status returns the current status of the service.
func (m *Manager) Status() services.ServiceStatus {
	return services.ServiceStatus{
		Name:    m.Name(),
		Running: m.IsRunning(),
	}
}

// IsRunning returns true if the firewall manager is active.
func (m *Manager) IsRunning() bool {
	return m.conn != nil
}
