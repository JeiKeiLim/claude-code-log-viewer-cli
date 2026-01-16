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

// listItemAvailWidth calculates the available width for list item content.
// It accounts for the gutter prefix (2 chars) and border padding (2 chars).
// Returns a minimum of 10 to ensure content is always readable.
func listItemAvailWidth(totalWidth int) int {
	const gutterPrefixWidth = 2 // "│ " or "  "
	const borderPadding = 2     // Left and right border chars
	availWidth := totalWidth - gutterPrefixWidth - borderPadding
	if availWidth < 10 {
		availWidth = 10
	}
	return availWidth
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

// overlaySpinnerView creates an overlay with a spinner centered on the background.
// The spinner box uses BgAlt background with accent foreground styling.
// Background lines above/below the spinner remain visible; spinner lines replace center rows.
// This approach avoids corrupting ANSI escape sequences by replacing entire lines.
func overlaySpinnerView(background string, spinnerView string, spinnerText string, width, height int) string {
	// Create a styled box for the spinner
	spinnerContent := spinnerView + " " + ListStyles.Loading.Render(spinnerText)
	spinnerBox := lipgloss.NewStyle().
		Background(DefaultTheme.BgAlt).
		Foreground(accentColor).
		Padding(1, 3).
		Render(spinnerContent)

	// Split background into lines
	bgLines := strings.Split(background, "\n")

	// Calculate spinner box dimensions
	boxLines := strings.Split(spinnerBox, "\n")
	boxHeight := len(boxLines)

	// Calculate vertical centering
	startRow := (height - boxHeight) / 2
	if startRow < 0 {
		startRow = 0
	}

	// Build result: background lines with spinner box lines replacing center rows
	result := make([]string, height)
	for i := 0; i < height; i++ {
		if i >= startRow && i < startRow+boxHeight {
			// This row gets the spinner box line (centered horizontally)
			boxIdx := i - startRow
			if boxIdx < len(boxLines) {
				// Center the box line horizontally with padding
				centeredLine := lipgloss.NewStyle().
					Width(width).
					Align(lipgloss.Center).
					Render(boxLines[boxIdx])
				result[i] = centeredLine
			} else {
				result[i] = strings.Repeat(" ", width)
			}
		} else if i < len(bgLines) {
			// Keep background line
			result[i] = bgLines[i]
		} else {
			result[i] = strings.Repeat(" ", width)
		}
	}

	return strings.Join(result, "\n")
}
