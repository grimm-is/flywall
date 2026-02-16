// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux
// +build !linux

package qos

import (
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// Manager handles QoS traffic shaping configuration (Stub).
type Manager struct{}

// NewManager creates a new QoS manager (Stub).
func NewManager(logger *logging.Logger) *Manager {
	return &Manager{}
}

// ApplyConfig applies QoS configuration to interfaces (Stub).
func (m *Manager) ApplyConfig(cfg *config.Config) error {
	return nil
}
