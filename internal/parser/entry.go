// Package parser handles parsing of Claude Code JSONL log files.
package parser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// ParseEntry parses a single JSONL line into a LogEntry.
func ParseEntry(data []byte) (types.LogEntry, error) {
	var raw types.RawLogEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return types.LogEntry{}, fmt.Errorf("failed to parse raw entry: %w", err)
	}

	entry := types.LogEntry{
		Type:      types.EntryType(raw.Type),
		UUID:      raw.UUID,
		SessionID: raw.SessionID,
	}

	// Parse parent UUID
	if raw.ParentUUID != nil {
		entry.ParentUUID = *raw.ParentUUID
	}

	// Parse timestamp
	if raw.Timestamp != "" {
		t, err := time.Parse(time.RFC3339, raw.Timestamp)
		if err != nil {
			// Try alternative format
			t, err = time.Parse("2006-01-02T15:04:05.000Z", raw.Timestamp)
			if err != nil {
				// Use zero time if parsing fails
				entry.Timestamp = time.Time{}
			} else {
				entry.Timestamp = t
			}
		} else {
			entry.Timestamp = t
		}
	}

	// Parse message based on entry type
	if len(raw.Message) > 0 {
		switch entry.Type {
		case types.EntryTypeUser:
			msg, err := parseUserMessage(raw.Message)
			if err != nil {
				return types.LogEntry{}, fmt.Errorf("failed to parse user message: %w", err)
			}
			entry.Message = msg

		case types.EntryTypeAssistant:
			msg, err := parseAssistantMessage(raw.Message)
			if err != nil {
				return types.LogEntry{}, fmt.Errorf("failed to parse assistant message: %w", err)
			}
			entry.Message = msg
		}
	}

	return entry, nil
}

// parseUserMessage parses a user message from raw JSON.
func parseUserMessage(data json.RawMessage) (types.Message, error) {
	var raw types.RawUserMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return types.Message{}, err
	}

	return types.Message{
		Role:        raw.Role,
		TextContent: raw.Content,
	}, nil
}

// parseAssistantMessage parses an assistant message from raw JSON.
func parseAssistantMessage(data json.RawMessage) (types.Message, error) {
	var raw types.RawAssistantMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return types.Message{}, err
	}

	msg := types.Message{
		Role:    raw.Role,
		Content: make([]types.MessageContent, 0, len(raw.Content)),
	}

	for _, c := range raw.Content {
		content := types.MessageContent{
			Type:      types.ContentType(c.Type),
			Text:      c.Text,
			Thinking:  c.Thinking,
			ToolName:  c.Name,
			ToolInput: c.Input,
			ToolID:    c.ID,
		}
		msg.Content = append(msg.Content, content)
	}

	return msg, nil
}
