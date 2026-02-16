// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"time"

	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ctlplane"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// View represents the currently active screen
type View int

const (
	ViewDashboard View = iota
	ViewFlows
	ViewPolicy
	ViewHistory
	ViewSystem
	ViewConfigTree // Full config graph
)

// Backend defines the interface for data retrieval and actions.
type Backend interface {
	GetStatus() (*EnrichedStatus, error)
	GetSystemStats() (*ctlplane.SystemStats, error)
	GetFlows(filter string) ([]Flow, error)
	GetConfig() (*config.Config, error)
	ApplyConfig(cfg *config.Config) error
	ReloadConfig() error
	ListBackups() ([]ctlplane.BackupInfo, error)
	RestoreBackup(version int) error
	Reboot() error
	RestartService(name string) error
	GetBandwidth(window string) ([]ctlplane.BandwidthPoint, error)
	GetAlerts(limit int) ([]alerting.AlertEvent, error)

	GetServices() ([]ctlplane.ServiceStatus, error)
	ApproveFlow(id int64) error
	DenyFlow(id int64) error
}

// Model is the main application state
type Model struct {
	Backend Backend

	// State
	ActiveView      View
	Width           int
	Height          int
	ConnectionError string // If set, shows disconnected state

	// Views
	Dashboard DashboardModel
	Flows     FlowsModel
	Policy    PolicyModel
	History   HistoryModel
	System    SystemModel
	Config    ConfigModel
}

// NewModel creates a new initial model
func NewModel(backend Backend) Model {
	return Model{
		Backend:    backend,
		ActiveView: ViewDashboard,
		Dashboard:  NewDashboardModel(backend),
		Flows:      NewFlowsModel(backend),
		Policy:     NewPolicyModel(backend),
		History:    NewHistoryModel(backend),
		System:     NewSystemModel(backend),
		Config:     NewConfigModel(backend),
	}
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
	// Init all views that need initial data
	return tea.Batch(
		m.Dashboard.Init(),
		m.Flows.Init(),
		m.Policy.Init(),
		m.History.Init(),
		m.System.Init(),
		m.Config.Init(),
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case BackendError:
		m.ConnectionError = msg.Err.Error()
		// Auto-retry after 5 seconds
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return RetryMsg{}
		})

	case RetryMsg:
		if m.ConnectionError != "" {
			m.ConnectionError = ""
			return m, m.Init()
		}
		return m, nil

	case tea.KeyMsg:
		// If editing form in config view, don't trap global keys if focusing?
		// But let's keep global quit for now unless editing needs q/ctrl+c
		if m.ActiveView != ViewConfigTree {
			switch msg.String() {
			case "r":
				if m.ConnectionError != "" {
					m.ConnectionError = ""
					// Re-init everything
					return m, m.Init()
				}
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		} else {
			// In config view, check if editing
			if m.ActiveView == ViewConfigTree && m.Config.Editing {
				// Don't intercept keys
			} else {
				switch msg.String() {
				case "q", "ctrl+c":
					return m, tea.Quit
				}
			}
		}

		if msg.String() == "tab" {
			// Cycle views
			// If editing config, maybe don't cycle?
			if m.ActiveView == ViewConfigTree && m.Config.Editing {
				// consume
			} else {
				m.ActiveView = (m.ActiveView + 1) % 6
				return m, nil
			}
		}

		// Shortcuts for Top Bar
		if !(m.ActiveView == ViewConfigTree && m.Config.Editing) {
			switch msg.String() {
			case "1":
				m.ActiveView = ViewDashboard
				return m, nil
			case "2":
				m.ActiveView = ViewFlows
				return m, nil
			case "3":
				m.ActiveView = ViewPolicy
				return m, nil
			case "4":
				m.ActiveView = ViewHistory
				return m, nil
			case "5":
				m.ActiveView = ViewSystem
				return m, nil
			case "6":
				m.ActiveView = ViewConfigTree
				return m, nil
			case "R":
				// Global config reload
				return m, func() tea.Msg {
					err := m.Backend.RestartService("firewall")
					// Ideally this would be ReloadConfig but interface varies
					if err != nil {
						return BackendError{Err: err}
					}
					return nil
				}
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		// Propagate resize to all views
		var cmd tea.Cmd
		m.Dashboard, cmd = m.Dashboard.Update(msg)
		cmds = append(cmds, cmd)

		m.Flows, cmd = m.Flows.Update(msg)
		cmds = append(cmds, cmd)

		m.Policy, cmd = m.Policy.Update(msg)
		cmds = append(cmds, cmd)

		m.History, cmd = m.History.Update(msg)
		cmds = append(cmds, cmd)

		m.System, cmd = m.System.Update(msg)
		cmds = append(cmds, cmd)

		m.Config, cmd = m.Config.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Delegate to active view
	var cmd tea.Cmd
	switch m.ActiveView {
	case ViewDashboard:
		m.Dashboard, cmd = m.Dashboard.Update(msg)
	case ViewFlows:
		m.Flows, cmd = m.Flows.Update(msg)
	case ViewPolicy:
		m.Policy, cmd = m.Policy.Update(msg)
	case ViewHistory:
		m.History, cmd = m.History.Update(msg)
	case ViewSystem:
		m.System, cmd = m.System.Update(msg)
	case ViewConfigTree:
		m.Config, cmd = m.Config.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the application
func (m Model) View() string {
	if m.ConnectionError != "" {
		// Centered Error Message
		msg := StyleTitle.Render("⚠ Connection Lost") + "\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(m.ConnectionError) + "\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Attempting to reconnect... (Press q to quit)")

		return lipgloss.Place(m.Width, m.Height,
			lipgloss.Center, lipgloss.Center,
			StyleCard.Render(msg),
		)
	}

	doc := m.ViewTopBar() + "\n"
	// doc := StyleHeader.Render("FLYWALL FIREWALL HUD     [Tab] Next View") + "\n\n"

	switch m.ActiveView {
	case ViewDashboard:
		doc += m.Dashboard.View()
	case ViewFlows:
		doc += m.Flows.View()
	case ViewPolicy:
		doc += m.Policy.View()
	case ViewHistory:
		doc += m.History.View()
	case ViewSystem:
		doc += m.System.View()
	case ViewConfigTree:
		doc += m.Config.View()
	}

	return StyleApp.Render(doc)
}

// ViewTopBar renders the top navigation menu
func (m Model) ViewTopBar() string {
	var items []string

	menus := []struct {
		View  View
		Label string
		Key   string
	}{
		{ViewDashboard, "Dashboard", "1"},
		{ViewFlows, "Flows", "2"},
		{ViewPolicy, "Policy", "3"},
		{ViewHistory, "History", "4"},
		{ViewSystem, "System", "5"},
		{ViewConfigTree, "Config", "6"},
	}

	for _, menu := range menus {
		key := StyleMenuKey.Render("[" + menu.Key + "]")
		label := menu.Label

		if m.ActiveView == menu.View {
			items = append(items, StyleMenuItemActive.Render(key+" "+label))
		} else {
			items = append(items, StyleMenuItem.Render(key+" "+label))
		}
	}

	// Join horizontally
	// Add branding
	brand := StyleTitle.Render("FLYWALL ")

	bar := lipgloss.JoinHorizontal(lipgloss.Top, append([]string{brand}, items...)...)
	return StyleTopBar.Render(bar)
}

// Shared Types
type EnrichedStatus struct {
	Running bool
	Uptime  string
}

type Flow struct {
	ID    int64
	Proto string
	Src   string
	Dst   string
	State string
}

type BackendError struct {
	Err error
}

type RetryMsg struct{}
