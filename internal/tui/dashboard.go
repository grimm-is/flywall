// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"fmt"
	"strings"
	"time"

	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/ctlplane"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DashboardModel is the main HUD view
type DashboardModel struct {
	Backend     Backend
	Status      *EnrichedStatus
	Stats       *ctlplane.SystemStats
	Bandwidth   []ctlplane.BandwidthPoint
	Alerts      []alerting.AlertEvent
	LastUpdated time.Time
	Width       int
	Height      int
}

func NewDashboardModel(backend Backend) DashboardModel {
	return DashboardModel{
		Backend: backend,
	}
}

type TickMsg time.Time

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.refreshAll(),
		m.tick(),
	)
}

func (m DashboardModel) tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m DashboardModel) refreshAll() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			status, err := m.Backend.GetStatus()
			if err != nil {
				return BackendError{Err: err}
			}
			return status
		},
		func() tea.Msg {
			stats, err := m.Backend.GetSystemStats()
			if err != nil {
				return BackendError{Err: err}
			}
			return stats
		},
		func() tea.Msg {
			bw, err := m.Backend.GetBandwidth("1h")
			if err != nil {
				return BackendError{Err: err}
			}
			return bw
		},
		func() tea.Msg {
			alerts, err := m.Backend.GetAlerts(5)
			if err != nil {
				return BackendError{Err: err}
			}
			return alerts
		},
	)
}

func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case *EnrichedStatus:
		m.Status = msg
	case *ctlplane.SystemStats:
		m.Stats = msg
	case []ctlplane.BandwidthPoint:
		m.Bandwidth = msg
	case []alerting.AlertEvent:
		m.Alerts = msg
	case TickMsg:
		m.LastUpdated = time.Time(msg)
		return m, tea.Batch(m.refreshAll(), m.tick())
	case tea.KeyMsg:
		switch msg.String() {
		case "B":
			// Reboot
			return m, func() tea.Msg {
				err := m.Backend.Reboot()
				if err != nil {
					return BackendError{Err: err}
				}
				return nil
			}
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m DashboardModel) View() string {
	if m.Status == nil {
		return "Loading Dashboard..."
	}

	// Layout:
	// [ Header / Status ]
	// [ Sparklines ]
	// [ Alerts ]

	// 1. Status Block
	statusIcon := "✅"
	statusText := StyleStatusGood.Render("ONLINE")
	if !m.Status.Running {
		statusIcon = "❌"
		statusText = StyleStatusBad.Render("OFFLINE")
	}

	statusBlock := StyleCard.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			StyleTitle.Render("System Status"),
			fmt.Sprintf("%s %s", statusIcon, statusText),
			StyleSubtitle.Render(fmt.Sprintf("Uptime: %s", m.Status.Uptime)),
		),
	)

	// 2. Metrics Block (Real Stats)
	cpuVal, ramVal := 0.0, 0.0
	if m.Stats != nil {
		cpuVal = m.Stats.CPUUsage / 100.0
		if m.Stats.MemoryTotal > 0 {
			ramVal = float64(m.Stats.MemoryUsed) / float64(m.Stats.MemoryTotal)
		}
	}

	metricsBlock := StyleCard.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			StyleTitle.Render("Resource Usage"),
			fmt.Sprintf("CPU: %s", progressBar(cpuVal)),
			fmt.Sprintf("RAM: %s", progressBar(ramVal)),
		),
	)

	// 3. Throughput Block (Real Bandwidth)
	trafficLine := "Traffic: (waiting for data)"

	if len(m.Bandwidth) > 0 {
		var data []float64
		var lastBytes int64
		for _, p := range m.Bandwidth {
			data = append(data, float64(p.Bytes))
			lastBytes = p.Bytes
		}
		trafficLine = fmt.Sprintf("Total: %s (%s)", sparkline(data), formatBits(lastBytes*8))
	}

	throughputBlock := StyleCard.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			StyleTitle.Render("Network Traffic (1h)"),
			trafficLine,
		),
	)

	// Top Row
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, statusBlock, metricsBlock, throughputBlock)

	// 4. Alert Ticker
	var alertItems []string
	alertItems = append(alertItems, StyleTitle.Render("System Alerts"))

	if len(m.Alerts) == 0 {
		alertItems = append(alertItems, StyleSubtitle.Render("No recent alerts"))
	} else {
		for _, a := range m.Alerts {
			ts := a.Timestamp.Format("15:04")
			line := fmt.Sprintf("• [%s] %s", ts, a.Message)
			switch a.Severity {
			case alerting.LevelCritical:
				alertItems = append(alertItems, StyleStatusBad.Render(line))
			case alerting.LevelWarning:
				alertItems = append(alertItems, StyleStatusWarn.Render(line))
			default:
				alertItems = append(alertItems, StyleStatusGood.Render(line))
			}
		}
	}

	alertsBlock := StyleCard.Width(m.Width - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			alertItems...,
		),
	)

	footer := StyleSubtitle.Render(fmt.Sprintf("Last updated: %s", m.LastUpdated.Format("15:04:05")))

	return lipgloss.JoinVertical(lipgloss.Left,
		topRow,
		alertsBlock,
		footer,
	)
}

// Simple text-based progress bar helper
func progressBar(percent float64) string {
	w := 20
	filled := int(float64(w) * percent)
	if filled < 0 {
		filled = 0
	}
	if filled > w {
		filled = w
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", w-filled)
	return fmt.Sprintf("[%s] %.0f%%", bar, percent*100)
}

func sparkline(data []float64) string {
	if len(data) == 0 {
		return ""
	}
	// chars := []string{" ", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	chars := []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	max := 0.0
	for _, v := range data {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1
	}

	// limit to last 20 points
	start := 0
	if len(data) > 20 {
		start = len(data) - 20
	}

	var sb strings.Builder
	for i := start; i < len(data); i++ {
		val := data[i]
		idx := int((val / max) * float64(len(chars)-1))
		sb.WriteRune(chars[idx])
	}
	return sb.String()
}

func formatBits(bits int64) string {
	const unit = 1000
	if bits < unit {
		return fmt.Sprintf("%d bps", bits)
	}
	div, exp := int64(unit), 0
	for n := bits / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cbps", float64(bits)/float64(div), "kMGTPE"[exp])
}
