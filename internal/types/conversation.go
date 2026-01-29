// Package types defines the core data structures for Claude Code log entries.
package types

import "time"

// Conversation represents a single JSONL conversation file.
type Conversation struct {
	// FilePath is the full path to the .jsonl file
	FilePath string
	// LastModified is the file modification timestamp (for display in conversation list)
	LastModified time.Time
	// CreationTime is the file birth time (used for sorting by "latest created")
	// Story 10.3: Falls back to mtime on platforms without birthtime support
	CreationTime time.Time
	// MessageCount is the number of log entries
	MessageCount int
	// FirstUserMessage is a preview of the first user message (truncated to 80 chars)
	FirstUserMessage string
	// TotalTokens is the sum of all token usage in the conversation
	TotalTokens TokenUsage
	// Model is the primary model used (from first assistant message)
	Model string
	// Duration is the time between first and last message
	Duration time.Duration
	// TurnCount is the number of user-assistant turn pairs (count of user messages)
	TurnCount int
}
