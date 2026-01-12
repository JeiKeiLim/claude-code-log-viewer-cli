// Package types defines the core data structures for Claude Code log entries.
package types

import (
	"encoding/json"
	"time"
)

// EntryType represents the type of a log entry.
type EntryType string

const (
	EntryTypeUser                EntryType = "user"
	EntryTypeAssistant           EntryType = "assistant"
	EntryTypeFileHistorySnapshot EntryType = "file-history-snapshot"
)

// ContentType represents the type of content within an assistant message.
type ContentType string

const (
	ContentTypeText     ContentType = "text"
	ContentTypeThinking ContentType = "thinking"
	ContentTypeToolUse  ContentType = "tool_use"
)

// LogEntry represents a parsed log entry from a JSONL file.
type LogEntry struct {
	Type       EntryType
	UUID       string
	ParentUUID string
	Timestamp  time.Time
	SessionID  string
	Message    Message
}

// Message represents the message content of a log entry.
type Message struct {
	Role    string
	Content []MessageContent
	// For user messages, this holds the plain text content
	TextContent string
}

// MessageContent represents a single content block within an assistant message.
type MessageContent struct {
	Type      ContentType
	Text      string
	Thinking  string
	ToolName  string
	ToolInput map[string]any
	ToolID    string
}

// RawLogEntry is the raw JSON structure from Claude Code logs.
type RawLogEntry struct {
	Type       string          `json:"type"`
	UUID       string          `json:"uuid"`
	ParentUUID *string         `json:"parentUuid"`
	Timestamp  string          `json:"timestamp"`
	SessionID  string          `json:"sessionId"`
	Message    json.RawMessage `json:"message"`
}

// RawUserMessage is the raw JSON structure for user messages.
type RawUserMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RawAssistantMessage is the raw JSON structure for assistant messages.
type RawAssistantMessage struct {
	Role    string              `json:"role"`
	Content []RawMessageContent `json:"content"`
}

// RawMessageContent is the raw JSON structure for message content blocks.
type RawMessageContent struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	Thinking string         `json:"thinking,omitempty"`
	Name     string         `json:"name,omitempty"`
	ID       string         `json:"id,omitempty"`
	Input    map[string]any `json:"input,omitempty"`
}
