// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestModel_Update_TabSwitching(t *testing.T) {
	backend := &MockBackend{}
	m := NewModel(backend)

	// Initial state
	assert.Equal(t, ViewDashboard, m.ActiveView)

	// Simulate Tab key press
	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Should switch to next view (Flows)
	assert.Equal(t, ViewFlows, m.ActiveView)

	// Tab again -> Policy
	newModel, _ = m.Update(msg)
	m = newModel.(Model)
	assert.Equal(t, ViewPolicy, m.ActiveView)
}

func TestModel_Update_BackendError(t *testing.T) {
	backend := &MockBackend{}
	m := NewModel(backend)

	// Simulate BackendError
	err := errors.New("connection lost")
	msg := BackendError{Err: err}

	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// ConnectionError should be set
	assert.Equal(t, "connection lost", m.ConnectionError)

	// View should render error message
	view := m.View()
	assert.Contains(t, view, "Connection Lost")
	assert.Contains(t, view, "connection lost")
}

func TestModel_Update_WindowSize(t *testing.T) {
	backend := &MockBackend{}
	m := NewModel(backend)

	// Simulate Window Resize
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Dimensions should be updated
	assert.Equal(t, 100, m.Width)
	assert.Equal(t, 50, m.Height)

	// Sub-models should also be updated (checked via Dashboard Width)
	assert.Equal(t, 100, m.Dashboard.Width)
	assert.Equal(t, 50, m.Dashboard.Height)
}
