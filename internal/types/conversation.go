// Package types defines the core data structures for Claude Code log entries.
package types

import "time"

// Conversation represents a single JSONL conversation file.
type Conversation struct {
	// FilePath is the full path to the .jsonl file
	FilePath string
	// LastModified is the file modification timestamp (for sorting)
	LastModified time.Time
	// MessageCount is the number of log entries
	MessageCount int
	// FirstUserMessage is a preview of the first user message (truncated to 80 chars)
	FirstUserMessage string
}
