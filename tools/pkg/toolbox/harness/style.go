// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package harness

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	StylePass    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // Green
	StyleFail    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // Red
	StyleSkip    = lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // Grey
	StyleInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("63")) // Purple/Blue
	StyleComment = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
	StylePlan    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
)

// HighlightTAP applies coloring to a single line of TAP output
func HighlightTAP(line string) string {
	// Simple prefix checking. TAP is line-based.
	// We check the raw line to preserve indentation, but check prefix on trimmed.

trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "ok") {
		return StylePass.Render(line)
	}
	if strings.HasPrefix(trimmed, "not ok") {
		return StyleFail.Render(line)
	}
	if strings.HasPrefix(trimmed, "#") {
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "skip") || strings.Contains(lower, "todo") {
			return StyleSkip.Render(line)
		}
		return StyleComment.Render(line)
	}
	if strings.HasPrefix(trimmed, "1..") {
		return StylePlan.Render(line)
	}
	if strings.HasPrefix(trimmed, "TAP_") || strings.HasPrefix(trimmed, "###") {
		return StyleInfo.Render(line)
	}

	return line
}

// HighlightTAPBlock applies coloring to a full block of TAP text
func HighlightTAPBlock(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = HighlightTAP(line)
	}
	return strings.Join(lines, "\n")
}
