// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"grimm.is/flywall/internal/ctlplane"
)

func TestDashboardModel_Update_Status(t *testing.T) {
	backend := &MockBackend{}
	m := NewDashboardModel(backend)

	// Simulate receiving EnrichedStatus
	status := &EnrichedStatus{
		Running: true,
		Uptime:  "24h",
	}
	m, _ = m.Update(status)

	assert.Equal(t, status, m.Status)
}

func TestDashboardModel_Update_Stats(t *testing.T) {
	backend := &MockBackend{}
	m := NewDashboardModel(backend)

	// Simulate receiving SystemStats
	stats := &ctlplane.SystemStats{
		CPUUsage:    50.0,
		MemoryTotal: 1000,
		MemoryUsed:  500,
	}
	m, _ = m.Update(stats)

	assert.Equal(t, stats, m.Stats)
}

func TestDashboardModel_View_Render(t *testing.T) {
	backend := &MockBackend{}
	m := NewDashboardModel(backend)

	// 1. Loading State
	assert.Contains(t, m.View(), "Loading Dashboard")

	// 2. Data Loaded
	m.Status = &EnrichedStatus{Running: true, Uptime: "10m"}
	m.Stats = &ctlplane.SystemStats{CPUUsage: 10.0}

	view := m.View()
	assert.Contains(t, view, "ONLINE")
	assert.Contains(t, view, "Uptime: 10m")
	assert.Contains(t, view, "Resource Usage")
}
