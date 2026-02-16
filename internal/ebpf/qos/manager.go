// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package qos

import (
	"fmt"
	"sync"

	"grimm.is/flywall/internal/ebpf/programs"
	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// Manager manages QoS profiles and flow assignments
type Manager struct {
	tcProgram *programs.TCOffloadProgram
	logger    *logging.Logger
	profiles  map[uint8]*types.QoSProfile
	mutex     sync.RWMutex
}

// Config for the QoS manager
type Config struct {
	DefaultProfiles bool `json:"default_profiles"`
}

// NewManager creates a new QoS manager
func NewManager(tcProgram *programs.TCOffloadProgram, logger *logging.Logger, config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Manager{
		tcProgram: tcProgram,
		logger:    logger,
		profiles:  make(map[uint8]*types.QoSProfile),
	}

	// Initialize default profiles if enabled
	if config.DefaultProfiles {
		m.initDefaultProfiles()
	}

	return m
}

// DefaultConfig returns the default QoS configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultProfiles: true,
	}
}

// initDefaultProfiles initializes default QoS profiles
func (m *Manager) initDefaultProfiles() {
	defaultProfiles := map[uint8]*types.QoSProfile{
		types.QoSProfileDefault: {
			RateLimit:  0, // Unlimited
			BurstLimit: 0,
			Priority:   0,
			AppClass:   0,
		},
		types.QoSProfileBulk: {
			RateLimit:  10000000, // 10 Mbps
			BurstLimit: 1000000,  // 1 MB
			Priority:   1,
			AppClass:   1,
		},
		types.QoSProfileInteractive: {
			RateLimit:  5000000, // 5 Mbps
			BurstLimit: 500000,  // 500 KB
			Priority:   3,
			AppClass:   2,
		},
		types.QoSProfileVideo: {
			RateLimit:  20000000, // 20 Mbps
			BurstLimit: 2000000,  // 2 MB
			Priority:   4,
			AppClass:   3,
		},
		types.QoSProfileVoice: {
			RateLimit:  1000000, // 1 Mbps
			BurstLimit: 100000,  // 100 KB
			Priority:   5,
			AppClass:   4,
		},
		types.QoSProfileCritical: {
			RateLimit:  0, // Unlimited
			BurstLimit: 0,
			Priority:   7,
			AppClass:   5,
		},
	}

	for id, profile := range defaultProfiles {
		if err := m.UpdateProfile(id, *profile); err != nil {
			m.logger.Error("Failed to initialize default QoS profile",
				"profile_id", id, "error", err)
		} else {
			m.profiles[id] = profile
		}
	}
}

// UpdateProfile updates or creates a QoS profile
func (m *Manager) UpdateProfile(profileID uint8, profile types.QoSProfile) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.tcProgram == nil {
		return fmt.Errorf("TC program not available")
	}

	if err := m.tcProgram.UpdateQoSProfile(uint32(profileID), profile); err != nil {
		return fmt.Errorf("failed to update QoS profile: %w", err)
	}

	m.profiles[profileID] = &profile
	m.logger.Info("Updated QoS profile", "profile_id", profileID,
		"rate_limit", profile.RateLimit, "priority", profile.Priority)

	return nil
}

// GetProfile retrieves a QoS profile
func (m *Manager) GetProfile(profileID uint8) (*types.QoSProfile, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if profile, exists := m.profiles[profileID]; exists {
		// Return a copy to prevent modification
		p := *profile
		return &p, nil
	}

	return nil, fmt.Errorf("QoS profile %d not found", profileID)
}

// ListProfiles returns all configured QoS profiles
func (m *Manager) ListProfiles() map[uint8]*types.QoSProfile {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[uint8]*types.QoSProfile, len(m.profiles))
	for id, profile := range m.profiles {
		p := *profile
		result[id] = &p
	}

	return result
}

// AssignFlowProfile assigns a QoS profile to a flow
func (m *Manager) AssignFlowProfile(flowKey types.FlowKey, profileID uint8) error {
	m.mutex.RLock()
	_, exists := m.profiles[profileID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("QoS profile %d not found", profileID)
	}

	if m.tcProgram == nil {
		return fmt.Errorf("TC program not available")
	}

	if err := m.tcProgram.SetQoSProfile(flowKey, profileID); err != nil {
		return fmt.Errorf("failed to assign QoS profile to flow: %w", err)
	}

	m.logger.Debug("Assigned QoS profile to flow",
		"flow", flowKey.String(), "profile_id", profileID)

	return nil
}

// RemoveFlowProfile removes QoS profile assignment from a flow
func (m *Manager) RemoveFlowProfile(flowKey types.FlowKey) error {
	return m.AssignFlowProfile(flowKey, types.QoSProfileDefault)
}

// GetFlowProfile retrieves the QoS profile assigned to a flow
func (m *Manager) GetFlowProfile(flowKey types.FlowKey) (uint8, error) {
	if m.tcProgram == nil {
		return 0, fmt.Errorf("TC program not available")
	}

	flow, err := m.tcProgram.GetFlow(flowKey)
	if err != nil {
		return 0, fmt.Errorf("failed to get flow: %w", err)
	}

	return flow.QoSProfile, nil
}

// DeleteProfile removes a QoS profile
func (m *Manager) DeleteProfile(profileID uint8) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if profileID == types.QoSProfileDefault {
		return fmt.Errorf("cannot delete default QoS profile")
	}

	delete(m.profiles, profileID)
	m.logger.Info("Deleted QoS profile", "profile_id", profileID)

	return nil
}
