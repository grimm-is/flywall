// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ctlplane

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"grimm.is/flywall/internal/config"
)

func TestConfigManager_Stage(t *testing.T) {
	initial := &config.Config{
		Interfaces: []config.Interface{
			{Name: "eth0"},
		},
	}
	cm := NewConfigManager(initial, "", nil, nil)

	// Success case
	err := cm.Stage(func(cfg *config.Config) error {
		cfg.Interfaces = append(cfg.Interfaces, config.Interface{Name: "eth1"})
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cm.staged.Interfaces))
	assert.Equal(t, 1, len(cm.running.Interfaces)) // Running stays same

	// Failure case (should not modify staged)
	err = cm.Stage(func(cfg *config.Config) error {
		cfg.Interfaces = append(cfg.Interfaces, config.Interface{Name: "eth2"})
		return assert.AnError
	})
	assert.Error(t, err)
	assert.Equal(t, 2, len(cm.staged.Interfaces)) // Still 2, eth2 not added
}

func TestConfigManager_Apply(t *testing.T) {
	initial := &config.Config{
		Interfaces: []config.Interface{
			{Name: "eth0"},
		},
	}
	cm := NewConfigManager(initial, "", nil, nil)

	cm.Stage(func(cfg *config.Config) error {
		cfg.Interfaces = append(cfg.Interfaces, config.Interface{Name: "eth1"})
		return nil
	})

	hookCalled := false
	cm.RegisterApplyHook(func(cfg *config.Config) error {
		hookCalled = true
		assert.Equal(t, 2, len(cfg.Interfaces))
		return nil
	})

	err := cm.Apply()
	assert.NoError(t, err)
	assert.True(t, hookCalled)
	assert.Equal(t, 2, len(cm.running.Interfaces))
}

func TestConfigManager_Rollback(t *testing.T) {
	initial := &config.Config{
		Interfaces: []config.Interface{
			{Name: "eth0"},
		},
	}
	cm := NewConfigManager(initial, "", nil, nil)

	cm.Stage(func(cfg *config.Config) error {
		cfg.Interfaces = append(cfg.Interfaces, config.Interface{Name: "eth1"})
		return nil
	})
	assert.Equal(t, 2, len(cm.staged.Interfaces))

	err := cm.Rollback()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cm.staged.Interfaces))
	assert.Equal(t, "eth0", cm.staged.Interfaces[0].Name)
}
