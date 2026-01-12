// Package tui provides the terminal user interface components.
package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// RenderPlain renders log entries as plain text with ANSI colors for terminal output.
// This is used for pipeline mode when stdout is not a TTY or --plain flag is used.
func RenderPlain(entries []types.LogEntry, source string) string {
	var b strings.Builder

	// Header showing source
	header := fmt.Sprintf("=== %s ===\n\n", source)
	b.WriteString(Styles.Title.Render(header))

	for _, entry := range entries {
		rendered := renderEntryPlain(entry)
		b.WriteString(rendered)
		b.WriteString("\n")
	}

	return b.String()
}

// renderEntryPlain renders a single log entry for plain text output.
func renderEntryPlain(entry types.LogEntry) string {
	switch entry.Type {
	case types.EntryTypeUser:
		return renderUserMessagePlain(entry)
	case types.EntryTypeAssistant:
		return renderAssistantMessagePlain(entry)
	default:
		return ""
	}
}

// renderUserMessagePlain renders a user message entry for plain text output.
func renderUserMessagePlain(entry types.LogEntry) string {
	timestamp := formatTimestamp(entry.Timestamp)
	header := fmt.Sprintf("%s %s  %s",
		UserIcon,
		Styles.UserHeader.Render("User"),
		Styles.Timestamp.Render(timestamp),
	)

	content := Styles.MessageContent.Render(entry.Message.TextContent)

	return Styles.UserMessage.Render(header + "\n" + content)
}

// renderAssistantMessagePlain renders an assistant message entry for plain text output.
func renderAssistantMessagePlain(entry types.LogEntry) string {
	timestamp := formatTimestamp(entry.Timestamp)
	header := fmt.Sprintf("%s %s  %s",
		AssistantIcon,
		Styles.AssistantHeader.Render("Assistant"),
		Styles.Timestamp.Render(timestamp),
	)

	var parts []string
	parts = append(parts, header)

	for _, content := range entry.Message.Content {
		switch content.Type {
		case types.ContentTypeText:
			if content.Text != "" {
				parts = append(parts, Styles.MessageContent.Render(content.Text))
			}

		case types.ContentTypeThinking:
			// Show thinking content expanded in plain mode
			thinkingHeader := fmt.Sprintf("%s %s", ThinkingIcon, Styles.ThinkingHeader.Render("Thinking"))
			parts = append(parts, Styles.ThinkingBlock.Render(thinkingHeader+"\n"+content.Thinking))

		case types.ContentTypeToolUse:
			// Show tool use with inputs in plain mode
			toolHeader := fmt.Sprintf("%s %s: %s",
				ToolIcon,
				Styles.ToolHeader.Render("Tool"),
				content.ToolName,
			)
			inputStr := formatToolInputPlain(content.ToolInput)
			parts = append(parts, Styles.ToolBlock.Render(toolHeader+"\n"+inputStr))
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
