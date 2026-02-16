// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux
// +build !linux

package ha

import (
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// ConntrackdManager is a stub for non-Linux platforms.
type ConntrackdManager struct{}

// NewConntrackdManager returns nil on non-Linux platforms.
func NewConntrackdManager(_ *config.ConntrackSyncConfig, _, _ string, _ *logging.Logger) *ConntrackdManager {
	return nil
}

// GenerateConfig is a no-op on non-Linux.
func (m *ConntrackdManager) GenerateConfig() (string, error) {
	return "", nil
}

// Start is a no-op on non-Linux.
func (m *ConntrackdManager) Start() error {
	return nil
}

// Stop is a no-op on non-Linux.
func (m *ConntrackdManager) Stop() {}

// NotifyFailover is a no-op on non-Linux.
func (m *ConntrackdManager) NotifyFailover() error {
	return nil
}

// NotifyPrimary is a no-op on non-Linux.
func (m *ConntrackdManager) NotifyPrimary() error {
	return nil
}
