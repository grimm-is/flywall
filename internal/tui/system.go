// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"grimm.is/flywall/internal/ctlplane"
)

type SystemModel struct {
	Backend  Backend
	List     list.Model
	Width    int
	Height   int
	Message  string // Result message
	Services []ctlplane.ServiceStatus
}

type opItem struct {
	title string
	desc  string
	id    string
}

func (i opItem) Title() string       { return i.title }
func (i opItem) Description() string { return i.desc }
func (i opItem) FilterValue() string { return i.title }

func NewSystemModel(backend Backend) SystemModel {
	items := []list.Item{
		opItem{title: "Reboot System", desc: "Reboot the entire host machine", id: "reboot"},
		opItem{title: "Restart Firewall", desc: "Restart the firewall engine service", id: "restart_firewall"},
		// Add more ops as needed, e.g. "Restart API", "Clear Conntrack"
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "System Operations"
	l.Styles.Title = StyleTitle

	return SystemModel{
		Backend: backend,
		List:    l,
	}
}

type ServicesMsg struct {
	Services []ctlplane.ServiceStatus
}

func (m SystemModel) Init() tea.Cmd {
	return func() tea.Msg {
		services, err := m.Backend.GetServices()
		if err != nil {
			return BackendError{Err: err}
		}
		return ServicesMsg{Services: services}
	}
}

func (m SystemModel) Update(msg tea.Msg) (SystemModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case BackendError:
		m.Message = fmt.Sprintf("Error: %v", msg.Err)
		return m, nil

	case ServicesMsg:
		m.Services = msg.Services
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			item := m.List.SelectedItem()
			if item == nil {
				return m, nil
			}
			op, ok := item.(opItem)
			if !ok {
				return m, nil
			}

			m.Message = fmt.Sprintf("Executing %s...", op.title)

			return m, func() tea.Msg {
				var err error
				switch op.id {
				case "reboot":
					err = m.Backend.Reboot()
				case "restart_firewall":
					err = m.Backend.RestartService("firewall")
				}

				if err != nil {
					return BackendError{Err: err}
				}
				return SystemOpSuccess{Op: op.title}
			}
		case "r":
			// Refresh services (this might be handled globally, but good to have here too if focused)
			// But careful not to conflict with global bindings if they overlap.
			// Global "R" (shift+r) is reload config. Local "r" could be refresh view.
			return m, m.Init()
		}

	case SystemOpSuccess:
		m.Message = fmt.Sprintf("Success: %s executed.", msg.Op)
		return m, nil

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.List.SetSize(msg.Width-4, msg.Height/2) // Use half height for list
	}

	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

type SystemOpSuccess struct {
	Op string
}

func (m SystemModel) View() string {
	// Render Service Status
	var servicesView string
	if len(m.Services) == 0 {
		servicesView = StyleSubtle.Render("No service status available.")
	} else {
		var rows []string
		for _, s := range m.Services {
			status := "RUNNING"
			style := StyleStatusOk
			if !s.Running {
				status = "STOPPED"
				style = StyleStatusErr
				if s.Error != "" {
					status = fmt.Sprintf("ERROR: %s", s.Error)
				}
			}
			rows = append(rows, fmt.Sprintf("%-20s %s", s.Name, style.Render(status)))
		}
		servicesView = lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	servicesBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			StyleSubtitle.Render("Service Status"),
			servicesView,
		))

	return lipgloss.JoinVertical(lipgloss.Left,
		StyleHeader.Render("SYSTEM MANAGEMENT"),
		servicesBox,
		StyleSubtitle.Render("Operations"),
		StyleCard.Render(m.List.View()),
		StyleStatusWarn.Render(m.Message),
	)
}
