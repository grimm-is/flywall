// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"grimm.is/flywall/internal/config"
)

func TestConfigModel_Update_LoadConfig(t *testing.T) {
	backend := &MockBackend{}
	m := NewConfigModel(backend)

	cfg := &config.Config{SchemaVersion: "1.0"}
	m, _ = m.Update(cfg)

	assert.Equal(t, cfg, m.Config)
	assert.Nil(t, m.LastError)
}

func TestConfigModel_Update_SaveSuccess(t *testing.T) {
	backend := &MockBackend{}
	m := NewConfigModel(backend)

	// Simulate save success
	msg := ConfigSaveSuccess{}
	m, _ = m.Update(msg)

	assert.Nil(t, m.LastError)
	// Ideally check for a toast message or status update if implemented
}
