// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PolicyModel struct {
	Backend Backend
	List    list.Model
	Width   int
	Height  int
}

type item struct {
	title string
	desc  string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func NewPolicyModel(backend Backend) PolicyModel {
	// Initial items (placeholder until Init)
	items := []list.Item{
		item{title: "Loading...", desc: "Fetching policies"},
	}

	defaultDelegate := list.NewDefaultDelegate()
	defaultDelegate.Styles.SelectedTitle = defaultDelegate.Styles.SelectedTitle.
		Foreground(ColorIce).
		BorderLeft(false).
		BorderLeftForeground(ColorIce)
	defaultDelegate.Styles.SelectedDesc = defaultDelegate.Styles.SelectedDesc.
		Foreground(ColorDeep)

	l := list.New(items, defaultDelegate, 0, 0)
	l.Title = "Firewall Zones"
	l.SetShowHelp(false)
	l.Styles.Title = StyleTitle

	return PolicyModel{
		Backend: backend,
		List:    l,
	}
}

func (m PolicyModel) Init() tea.Cmd {
	return func() tea.Msg {
		cfg, err := m.Backend.GetConfig()
		if err != nil {
			return BackendError{Err: err}
		}

		var items []item
		// Add Zones
		for _, zone := range cfg.Zones {
			desc := zone.Description
			if desc == "" {
				desc = "Network Zone"
			}
			items = append(items, item{
				title: "Zone: " + zone.Name,
				desc:  desc,
			})
		}

		// Add Policies
		for _, policy := range cfg.Policies {
			desc := policy.Description
			if desc == "" {
				desc = fmt.Sprintf("Policy: %s -> %s", policy.From, policy.To)
			}
			items = append(items, item{
				title: "Policy: " + policy.Name,
				desc:  desc,
			})
		}

		// Fallback if no zones or policies
		if len(items) == 0 {
			items = append(items, item{title: "No Data", desc: "No zones or policies defined in configuration"})
		}

		return items
	}
}

func (m PolicyModel) Update(msg tea.Msg) (PolicyModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case BackendError:
		// Root model handles this, but we can stop loading
		return m, nil

	case []item:
		items := make([]list.Item, len(msg))
		for i, it := range msg {
			items[i] = it
		}
		cmd = m.List.SetItems(items)
		return m, cmd

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.List.SetSize(msg.Width-4, msg.Height-4)
	}

	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

func (m PolicyModel) View() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		StyleHeader.Render("POLICY INSPECTOR"),
		StyleCard.Render(m.List.View()),
	)
}
