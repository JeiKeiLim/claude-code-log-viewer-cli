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
	// Model is the Claude model used (e.g., "claude-opus-4-5-20251101")
	// Only populated for assistant entries
	Model string
	// Usage contains token usage statistics
	// Only populated for assistant entries
	Usage TokenUsage
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
	Model   string              `json:"model,omitempty"`
	Usage   *RawTokenUsage      `json:"usage,omitempty"`
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

// TokenUsage represents token consumption for a message or conversation.
type TokenUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// Total returns the total token count (input + output + cache).
func (u TokenUsage) Total() int {
	return u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// TotalInput returns total input tokens including cache.
func (u TokenUsage) TotalInput() int {
	return u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// IsEmpty returns true if all token counts are zero.
func (u TokenUsage) IsEmpty() bool {
	return u.InputTokens == 0 && u.OutputTokens == 0 &&
		u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0
}

// RawTokenUsage is the raw JSON structure for token usage.
type RawTokenUsage struct {
	InputTokens              int    `json:"input_tokens"`
	OutputTokens             int    `json:"output_tokens"`
	CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
	ServiceTier              string `json:"service_tier,omitempty"`
}
