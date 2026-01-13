// Package tui provides terminal user interface components.
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// VisualWidth returns the visual column width of a string.
// CJK characters count as 2 columns, ASCII as 1.
func VisualWidth(s string) int {
	return lipgloss.Width(s)
}

// TruncateToWidth truncates a string to fit within maxWidth visual columns.
// Adds "..." suffix if truncated.
func TruncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if maxWidth <= 3 {
		runes := []rune(s)
		width := 0
		for i, r := range runes {
			runeWidth := lipgloss.Width(string(r))
			if width+runeWidth > maxWidth {
				return string(runes[:i])
			}
			width += runeWidth
		}
		return s
	}
	if VisualWidth(s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	for i := len(runes) - 1; i >= 0; i-- {
		candidate := string(runes[:i]) + "..."
		if VisualWidth(candidate) <= maxWidth {
			return candidate
		}
	}
	return "..."
}

// TruncateFromLeftToWidth truncates from the left, keeping the right portion.
// Adds "..." prefix if truncated. Useful for paths.
func TruncateFromLeftToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if maxWidth <= 3 {
		return "..."[:min(maxWidth, 3)]
	}
	if VisualWidth(s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	for i := 1; i < len(runes); i++ {
		candidate := "..." + string(runes[i:])
		if VisualWidth(candidate) <= maxWidth {
			return candidate
		}
	}
	return "..."
}

// PadToWidth pads a string with spaces to reach exact visual width.
func PadToWidth(s string, width int) string {
	currentWidth := VisualWidth(s)
	if currentWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-currentWidth)
}

// WrapText wraps text to fit within maxWidth visual columns.
// Preserves existing newlines and wraps long lines at word boundaries.
func WrapText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}

	var result strings.Builder
	lines := strings.Split(s, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// If line fits, use as-is
		if VisualWidth(line) <= maxWidth {
			result.WriteString(line)
			continue
		}

		// Wrap the line
		result.WriteString(wrapLine(line, maxWidth))
	}

	return result.String()
}

// wrapLine wraps a single line to fit within maxWidth.
func wrapLine(line string, maxWidth int) string {
	if maxWidth <= 0 || VisualWidth(line) <= maxWidth {
		return line
	}

	var result strings.Builder
	words := strings.Fields(line)
	currentLineWidth := 0

	for i, word := range words {
		wordWidth := VisualWidth(word)

		// If single word is longer than maxWidth, break it
		if wordWidth > maxWidth {
			if currentLineWidth > 0 {
				result.WriteString("\n")
				currentLineWidth = 0
			}
			result.WriteString(breakLongWord(word, maxWidth))
			currentLineWidth = VisualWidth(lastLine(result.String()))
			continue
		}

		// Check if word fits on current line
		spaceWidth := 0
		if currentLineWidth > 0 {
			spaceWidth = 1
		}

		if currentLineWidth+spaceWidth+wordWidth <= maxWidth {
			if i > 0 && currentLineWidth > 0 {
				result.WriteString(" ")
				currentLineWidth++
			}
			result.WriteString(word)
			currentLineWidth += wordWidth
		} else {
			// Start new line
			result.WriteString("\n")
			result.WriteString(word)
			currentLineWidth = wordWidth
		}
	}

	return result.String()
}

// breakLongWord breaks a word that's longer than maxWidth into multiple lines.
func breakLongWord(word string, maxWidth int) string {
	var result strings.Builder
	runes := []rune(word)
	currentWidth := 0
	start := 0

	for i, r := range runes {
		runeWidth := VisualWidth(string(r))
		if currentWidth+runeWidth > maxWidth && currentWidth > 0 {
			if result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString(string(runes[start:i]))
			start = i
			currentWidth = 0
		}
		currentWidth += runeWidth
	}

	// Write remaining
	if start < len(runes) {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(string(runes[start:]))
	}

	return result.String()
}

// lastLine returns the last line of a multi-line string.
func lastLine(s string) string {
	idx := strings.LastIndex(s, "\n")
	if idx == -1 {
		return s
	}
	return s[idx+1:]
}
