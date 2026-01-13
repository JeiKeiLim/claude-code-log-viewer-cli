// Package tui provides the terminal user interface components.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// formatTimestamp formats a timestamp for display in local timezone.
func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatDuration formats a duration for display.
// Short durations: "5m", "30s"
// Long durations: "2h 15m", "1d 3h"
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, hours)
}

// addBorder wraps content in a rounded box border.
// Returns exactly len(lines)+2 lines (top border, content, bottom border).
// Content lines should already be properly sized before calling this.
func addBorder(content string, width int) string {
	if width < 4 {
		width = 4
	}

	lines := strings.Split(content, "\n")
	innerWidth := width - 2 // Account for left and right border chars

	var result strings.Builder

	// Top border: ╭───╮
	result.WriteString("╭")
	result.WriteString(strings.Repeat("─", innerWidth))
	result.WriteString("╮\n")

	// Content lines with side borders
	for _, line := range lines {
		result.WriteString("│")
		// Use lipgloss.Width to get visual width (ignores ANSI escape codes)
		visualWidth := lipgloss.Width(line)
		if visualWidth < innerWidth {
			result.WriteString(line)
			result.WriteString(strings.Repeat(" ", innerWidth-visualWidth))
		} else {
			// Line fits exactly or is longer - just use it
			result.WriteString(line)
		}
		result.WriteString("│\n")
	}

	// Bottom border: ╰───╯
	result.WriteString("╰")
	result.WriteString(strings.Repeat("─", innerWidth))
	result.WriteString("╯")

	return result.String()
}
