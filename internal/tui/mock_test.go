// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ctlplane"
)

// MockBackend implements Backend for testing purposes
type MockBackend struct {
	Status               *EnrichedStatus
	SystemStats          *ctlplane.SystemStats
	Flows                []Flow
	Config               *config.Config
	Backups              []ctlplane.BackupInfo
	Err                  error
	ApplyCalled          bool
	RebootCalled         bool
	RestartServiceCalled bool
	Bandwidth            []ctlplane.BandwidthPoint
	Alerts               []alerting.AlertEvent
}

func (m *MockBackend) GetStatus() (*EnrichedStatus, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Status == nil {
		return &EnrichedStatus{Running: true, Uptime: "1h"}, nil
	}
	return m.Status, nil
}

func (m *MockBackend) GetSystemStats() (*ctlplane.SystemStats, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.SystemStats == nil {
		return &ctlplane.SystemStats{}, nil
	}
	return m.SystemStats, nil
}

func (m *MockBackend) GetFlows(filter string) ([]Flow, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Flows, nil
}

func (m *MockBackend) GetConfig() (*config.Config, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Config == nil {
		return &config.Config{}, nil
	}
	return m.Config, nil
}

func (m *MockBackend) ApplyConfig(cfg *config.Config) error {
	m.ApplyCalled = true
	return m.Err
}

func (m *MockBackend) ReloadConfig() error {
	m.RestartServiceCalled = true // Reuse flag or add new one
	return m.Err
}

func (m *MockBackend) ListBackups() ([]ctlplane.BackupInfo, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Backups, nil
}

func (m *MockBackend) RestoreBackup(version int) error {
	return m.Err
}

func (m *MockBackend) Reboot() error {
	m.RebootCalled = true
	return m.Err
}

func (m *MockBackend) RestartService(name string) error {
	m.RestartServiceCalled = true
	return m.Err
}

func (m *MockBackend) GetBandwidth(window string) ([]ctlplane.BandwidthPoint, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Bandwidth, nil
}

func (m *MockBackend) GetAlerts(limit int) ([]alerting.AlertEvent, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Alerts, nil
}

func (m *MockBackend) ApproveFlow(id int64) error {
	return m.Err
}

func (m *MockBackend) DenyFlow(id int64) error {
	return m.Err
}

func (m *MockBackend) GetServices() ([]ctlplane.ServiceStatus, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return nil, nil
}
