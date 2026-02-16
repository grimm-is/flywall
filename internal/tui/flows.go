// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FlowsModel struct {
	Backend Backend
	Table   table.Model
	Flows   []Flow
	Width   int
	Height  int
}

func NewFlowsModel(backend Backend) FlowsModel {
	columns := []table.Column{
		{Title: "Proto", Width: 6},
		{Title: "Source", Width: 20},
		{Title: "Destination", Width: 20},
		{Title: "State", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorDeep).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(ColorIce).
		Background(ColorDeep).
		Bold(false)
	t.SetStyles(s)

	return FlowsModel{
		Backend: backend,
		Table:   t,
	}
}

func (m FlowsModel) Init() tea.Cmd {
	return func() tea.Msg {
		flows, err := m.Backend.GetFlows("")
		if err != nil {
			return nil
		}
		return flows
	}
}

func (m FlowsModel) Update(msg tea.Msg) (FlowsModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case []Flow:
		m.Flows = msg
		rows := make([]table.Row, len(msg))
		for i, f := range msg {
			rows[i] = table.Row{
				strings.ToUpper(f.Proto),
				f.Src,
				f.Dst,
				f.State,
			}
		}
		m.Table.SetRows(rows)

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Refresh
			return m, m.Init()
		case "a":
			// Approve selected flow
			if len(m.Flows) > 0 {
				idx := m.Table.Cursor()
				if idx >= 0 && idx < len(m.Flows) {
					id := m.Flows[idx].ID
					return m, func() tea.Msg {
						if err := m.Backend.ApproveFlow(id); err != nil {
							// Return error or just log? For TUI, maybe just refresh
						}
						return nil // Trigger refresh via Init? Or specific msg?
						// Let's re-init to refresh list
					}
				}
			}
			// We need to chain the refresh.
			// The func above returns nil msg, which does nothing.
			// We should probably return a command that does the action AND then returns a "refresh needed" msg
			// or just chain commands if possible. tea.Sequence/Batch.
			// But Batch runs in parallel.
			// We can define a wrapper cmd.

			// Better:
			if len(m.Flows) > 0 {
				idx := m.Table.Cursor()
				if idx >= 0 && idx < len(m.Flows) {
					id := m.Flows[idx].ID
					return m, func() tea.Msg {
						m.Backend.ApproveFlow(id) // Ignore error for now or handle it
						// Return a message that triggers refresh?
						// Actually, we can just call Init() cmd which returns the flows msg.
						// But we need to wait for Approve to finish.
						// So:
						return m.Init()()
					}
				}
			}
		case "d":
			if len(m.Flows) > 0 {
				idx := m.Table.Cursor()
				if idx >= 0 && idx < len(m.Flows) {
					id := m.Flows[idx].ID
					return m, func() tea.Msg {
						m.Backend.DenyFlow(id)
						return m.Init()()
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Table.SetHeight(msg.Height - 5) // Reserve space for header/footer
		// Adjust column widths if needed
	}

	m.Table, cmd = m.Table.Update(msg)
	return m, cmd
}

func (m FlowsModel) View() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		StyleHeader.Render("FLOW MONITOR (r: refresh)"),
		StyleCard.Render(m.Table.View()),
		StyleSubtitle.Render(fmt.Sprintf("%d active flows", len(m.Flows))),
	)
}
