// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HistoryModel struct {
	Backend Backend
	List    list.Model
	Width   int
	Height  int
}

type checkpointItem struct {
	title string
	desc  string
}

func (i checkpointItem) Title() string       { return i.title }
func (i checkpointItem) Description() string { return i.desc }
func (i checkpointItem) FilterValue() string { return i.title }

func NewHistoryModel(backend Backend) HistoryModel {
	items := []list.Item{
		checkpointItem{title: "Loading...", desc: "Fetching history"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Configuration History"
	l.Styles.Title = StyleTitle

	return HistoryModel{
		Backend: backend,
		List:    l,
	}
}

func (m HistoryModel) Init() tea.Cmd {
	return func() tea.Msg {
		backups, err := m.Backend.ListBackups()
		if err != nil {
			return BackendError{Err: err}
		}

		var items []checkpointItem
		for _, b := range backups {
			items = append(items, checkpointItem{
				title: fmt.Sprintf("v%d", b.Version),
				desc:  fmt.Sprintf("%s - %s", b.Timestamp, b.Description),
			})
		}

		if len(items) == 0 {
			items = append(items, checkpointItem{title: "No History", desc: "No configuration backups found"})
		}

		return items
	}
}

func (m HistoryModel) Update(msg tea.Msg) (HistoryModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case BackendError:
		return m, nil

	case []checkpointItem:
		items := make([]list.Item, len(msg))
		for i, it := range msg {
			items[i] = it
		}
		cmd = m.List.SetItems(items)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Restore selected backup
			item := m.List.SelectedItem()
			if item == nil {
				return m, nil
			}
			checkpoint, ok := item.(checkpointItem)
			if !ok {
				return m, nil
			}

			// Extract version from title (e.g. "v45")
			var version int
			fmt.Sscanf(checkpoint.title, "v%d", &version)

			return m, func() tea.Msg {
				err := m.Backend.RestoreBackup(version)
				if err != nil {
					return BackendError{Err: err}
				}
				// Success! Refresh list
				return m.Init()()
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.List.SetSize(msg.Width-4, msg.Height-4)
	}

	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

func (m HistoryModel) View() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		StyleHeader.Render("TIME MACHINE"),
		StyleSubtitle.Render("Select a checkpoint to view diff or rollback"),
		StyleCard.Render(m.List.View()),
	)
}
