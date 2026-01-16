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
// The background view remains visible behind the spinner.
func overlaySpinnerView(background string, spinnerView string, spinnerText string, width, height int) string {
	// Create a styled box for the spinner
	spinnerContent := spinnerView + " " + ListStyles.Loading.Render(spinnerText)
	spinnerBox := lipgloss.NewStyle().
		Background(DefaultTheme.BgAlt).
		Foreground(accentColor).
		Padding(1, 3).
		Render(spinnerContent)

	// Split background into lines for overlay
	bgLines := strings.Split(background, "\n")

	// Calculate center position for spinner box
	boxLines := strings.Split(spinnerBox, "\n")
	boxHeight := len(boxLines)
	boxWidth := 0
	for _, line := range boxLines {
		if w := lipgloss.Width(line); w > boxWidth {
			boxWidth = w
		}
	}

	// Calculate top-left position for centering
	startRow := (height - boxHeight) / 2
	startCol := (width - boxWidth) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	// Overlay spinner box on background
	result := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(bgLines) {
			result[i] = bgLines[i]
		} else {
			result[i] = strings.Repeat(" ", width)
		}
	}

	// Overlay the spinner box lines
	for i, boxLine := range boxLines {
		row := startRow + i
		if row >= 0 && row < height {
			// Get current background line, pad if needed
			bgLine := result[row]
			bgRunes := []rune(bgLine)

			// Build the overlaid line
			var newLine strings.Builder
			col := 0
			runeIdx := 0

			// Copy background up to startCol
			for col < startCol && runeIdx < len(bgRunes) {
				r := bgRunes[runeIdx]
				newLine.WriteRune(r)
				col += runeWidth(r)
				runeIdx++
			}
			// Pad if needed
			for col < startCol {
				newLine.WriteRune(' ')
				col++
			}

			// Insert the spinner box line
			newLine.WriteString(boxLine)
			col += lipgloss.Width(boxLine)

			// Skip background runes covered by the box
			targetCol := startCol + boxWidth
			for col < targetCol && runeIdx < len(bgRunes) {
				r := bgRunes[runeIdx]
				col += runeWidth(r)
				runeIdx++
			}

			// Copy remaining background
			for runeIdx < len(bgRunes) {
				newLine.WriteRune(bgRunes[runeIdx])
				runeIdx++
			}

			result[row] = newLine.String()
		}
	}

	return strings.Join(result, "\n")
}

// runeWidth returns the display width of a rune (2 for CJK, 1 for others).
func runeWidth(r rune) int {
	if r >= 0x1100 &&
		(r <= 0x115F || // Hangul Jamo
			r == 0x2329 || r == 0x232A || // Angle brackets
			(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) || // CJK
			(r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
			(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility
			(r >= 0xFE10 && r <= 0xFE1F) || // Vertical forms
			(r >= 0xFE30 && r <= 0xFE6F) || // CJK Compatibility Forms
			(r >= 0xFF00 && r <= 0xFF60) || // Fullwidth Forms
			(r >= 0xFFE0 && r <= 0xFFE6) || // Fullwidth Forms
			(r >= 0x20000 && r <= 0x2FFFD) || // CJK Extension B
			(r >= 0x30000 && r <= 0x3FFFD)) { // CJK Extension C+
		return 2
	}
	return 1
}
