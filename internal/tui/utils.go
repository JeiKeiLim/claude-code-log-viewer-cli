// Package tui provides the terminal user interface components.
package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/token"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// DefaultPlainModeWidth is the default rendering width for plain mode when no terminal is detected.
const DefaultPlainModeWidth = 80

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

// addBorderWithStyle wraps content in a rounded box border with configurable color.
// Returns exactly len(lines)+2 lines (top border, content, bottom border).
// Content lines should already be properly sized before calling this.
func addBorderWithStyle(content string, width int, borderColor lipgloss.AdaptiveColor) string {
	if width < 4 {
		width = 4
	}

	lines := strings.Split(content, "\n")
	innerWidth := width - 2 // Account for left and right border chars

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	var result strings.Builder

	// Top border: ╭───╮
	result.WriteString(borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮"))
	result.WriteString("\n")

	// Content lines with side borders
	for _, line := range lines {
		result.WriteString(borderStyle.Render("│"))
		// Use lipgloss.Width to get visual width (ignores ANSI escape codes)
		visualWidth := lipgloss.Width(line)
		if visualWidth < innerWidth {
			result.WriteString(line)
			result.WriteString(strings.Repeat(" ", innerWidth-visualWidth))
		} else {
			// Line fits exactly or is longer - just use it
			result.WriteString(line)
		}
		result.WriteString(borderStyle.Render("│"))
		result.WriteString("\n")
	}

	// Bottom border: ╰───╯
	result.WriteString(borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯"))

	return result.String()
}

// addBorder wraps content in a rounded box border with default (unfocused) color.
// Returns exactly len(lines)+2 lines (top border, content, bottom border).
// Content lines should already be properly sized before calling this.
func addBorder(content string, width int) string {
	return addBorderWithStyle(content, width, PaneUnfocusedBorderColor)
}

// overlaySpinnerView creates an overlay with a spinner centered on the background.
// The spinner box uses BgAlt background with accent foreground styling.
// Background lines above/below the spinner remain visible; spinner lines replace center rows.
// This approach avoids corrupting ANSI escape sequences by replacing entire lines.
func overlaySpinnerView(background string, spinnerView string, spinnerText string, width, height int) string {
	// Create a styled box for the spinner (no background, just centered text)
	spinnerContent := spinnerView + " " + ListStyles.Loading.Render(spinnerText)
	spinnerBox := lipgloss.NewStyle().
		Foreground(accentColor).
		Padding(0, 1).
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

// formatWithCommas formats an integer with thousand separators.
// Examples: 999 -> "999", 1000 -> "1,000", 1234567 -> "1,234,567"
func formatWithCommas(n int) string {
	str := strconv.Itoa(n)
	if n < 1000 {
		return str
	}

	var result strings.Builder
	offset := len(str) % 3
	if offset > 0 {
		result.WriteString(str[:offset])
		if offset < len(str) {
			result.WriteString(",")
		}
	}
	for i := offset; i < len(str); i += 3 {
		if i > offset {
			result.WriteString(",")
		}
		result.WriteString(str[i : i+3])
	}
	return result.String()
}

// formatTokenUsage returns a formatted token string for display.
// Returns empty string for FileHistorySnapshot entries or if token service is nil.
func formatTokenUsage(entry types.LogEntry, svc *token.Service) string {
	// Skip file-history-snapshot entries (no user-facing text)
	if entry.Type == types.EntryTypeFileHistorySnapshot {
		return ""
	}

	// Use actual usage from log if available
	if !entry.Usage.IsEmpty() {
		return fmt.Sprintf("Tokens: %s (from log)", formatWithCommas(entry.Usage.Total()))
	}

	// Service unavailable - graceful degradation
	if svc == nil {
		return ""
	}

	// Calculate token estimate
	calculated := svc.CalculateEntry(entry)
	return fmt.Sprintf("Tokens: ~%s (estimated)", formatWithCommas(calculated))
}
