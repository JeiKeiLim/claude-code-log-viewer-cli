// Package tui provides the terminal user interface components.
package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/usage"
)

// RenderPlain renders log entries as plain text with ANSI colors for terminal output.
// This is used for pipeline mode when stdout is not a TTY or --plain flag is used.
func RenderPlain(entries []types.LogEntry, source string, opts RenderOptions) string {
	width := opts.Width
	if width == 0 {
		width = DefaultPlainModeWidth // Default for plain mode when no terminal
	}

	var b strings.Builder

	// Header showing source
	header := fmt.Sprintf("=== %s ===\n\n", source)
	b.WriteString(Styles.Title.Render(header))

	for _, entry := range entries {
		rendered := renderEntryPlain(entry, opts, width)
		b.WriteString(rendered)
		b.WriteString("\n")
	}

	return b.String()
}

// renderEntryPlain renders a single log entry for plain text output.
func renderEntryPlain(entry types.LogEntry, opts RenderOptions, width int) string {
	switch entry.Type {
	case types.EntryTypeUser:
		return renderUserMessagePlain(entry, width)
	case types.EntryTypeAssistant:
		return renderAssistantMessagePlain(entry, opts, width)
	default:
		return ""
	}
}

// renderUserMessagePlain renders a user message entry for plain text output.
func renderUserMessagePlain(entry types.LogEntry, width int) string {
	timestamp := formatTimestamp(entry.Timestamp)
	header := fmt.Sprintf("%s %s  %s",
		UserIcon,
		Styles.UserHeader.Render("User"),
		Styles.Timestamp.Render(timestamp),
	)

	// Wrap content to fit specified width (with margin for styling)
	wrapWidth := width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrappedText := WrapText(entry.Message.TextContent, wrapWidth)
	content := Styles.MessageContent.Render(wrappedText)

	return Styles.UserMessage.Render(header + "\n" + content)
}

// renderAssistantMessagePlain renders an assistant message entry for plain text output.
func renderAssistantMessagePlain(entry types.LogEntry, opts RenderOptions, width int) string {
	timestamp := formatTimestamp(entry.Timestamp)
	header := fmt.Sprintf("%s %s  %s",
		AssistantIcon,
		Styles.AssistantHeader.Render("Assistant"),
		Styles.Timestamp.Render(timestamp),
	)

	// Calculate wrap width for content
	wrapWidth := width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	var parts []string
	parts = append(parts, header)

	for _, content := range entry.Message.Content {
		switch content.Type {
		case types.ContentTypeText:
			if content.Text != "" {
				wrappedText := WrapText(content.Text, wrapWidth)
				parts = append(parts, Styles.MessageContent.Render(wrappedText))
			}

		case types.ContentTypeThinking:
			if opts.HideThoughts {
				continue // Skip thinking blocks when hidden
			}
			// Show thinking content expanded in plain mode
			thinkingHeader := fmt.Sprintf("%s %s", ThinkingIcon, Styles.ThinkingHeader.Render("Thinking"))
			wrappedThinking := WrapText(content.Thinking, wrapWidth)
			parts = append(parts, Styles.ThinkingBlock.Render(thinkingHeader+"\n"+wrappedThinking))

		case types.ContentTypeToolUse:
			if opts.HideTools {
				continue // Skip tool blocks when hidden
			}
			// Show tool use with inputs in plain mode
			toolHeader := fmt.Sprintf("%s %s: %s",
				ToolIcon,
				Styles.ToolHeader.Render("Tool"),
				content.ToolName,
			)
			inputStr := formatToolInputPlain(content.ToolInput)
			wrappedInput := WrapText(inputStr, wrapWidth)
			parts = append(parts, Styles.ToolBlock.Render(toolHeader+"\n"+wrappedInput))
		}
	}

	return Styles.AssistantMessage.Render(strings.Join(parts, "\n"))
}

// formatToolInputPlain formats tool input as a readable string for plain output.
func formatToolInputPlain(input map[string]any) string {
	if len(input) == 0 {
		return "(no input)"
	}

	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "(error formatting input)"
	}

	return string(data)
}

// RenderUsagePlain renders usage limits as plain text for CLI output.
// It respects the color settings configured via configureColorOutput().
func RenderUsagePlain(limits *usage.UsageLimits) string {
	var b strings.Builder

	b.WriteString(Styles.Title.Render("Claude Code Usage") + "\n")

	if limits.FiveHour != nil {
		resetStr := ""
		if limits.FiveHour.ResetsAt != nil {
			remaining := time.Until(*limits.FiveHour.ResetsAt)
			if remaining > 0 {
				resetStr = fmt.Sprintf(" (resets in %s)", formatResetDuration(remaining))
			}
		}
		b.WriteString(fmt.Sprintf("  5-hour:  %.0f%%%s\n",
			limits.FiveHour.Utilization, resetStr))
	}

	if limits.SevenDay != nil {
		b.WriteString(fmt.Sprintf("  7-day:   %.0f%%\n",
			limits.SevenDay.Utilization))
	}

	// Only show Opus if utilization > 0 (most users don't have Opus quota)
	if limits.SevenDayOpus != nil && limits.SevenDayOpus.Utilization > 0 {
		b.WriteString(fmt.Sprintf("  Opus:    %.0f%%\n",
			limits.SevenDayOpus.Utilization))
	}

	return b.String()
}

// formatResetDuration converts duration to human-readable format for usage reset times.
// Input examples and expected outputs:
//   - 0 or negative → "" (empty string)
//   - 30s → "<1m"
//   - 45*time.Minute → "45m"
//   - 2*time.Hour + 15*time.Minute → "2h 15m"
//   - 3*time.Hour → "3h"
func formatResetDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d < time.Minute {
		return "<1m"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}
