// Package types defines the core data structures for Claude Code log entries.
package types

// Project represents a Claude Code project directory.
type Project struct {
	// EncodedName is the directory name as stored (e.g., "-Users-limjk-GitHub-foo")
	EncodedName string
	// DecodedPath is the decoded original path (e.g., "/Users/limjk/GitHub/foo")
	DecodedPath string
	// DisplayName is the short display name (e.g., "foo" or "limjk/foo" if disambiguated)
	DisplayName string
	// DirPath is the full path to the project directory under ~/.claude/projects/
	DirPath string
	// ConversationCount is the number of .jsonl files in the project
	ConversationCount int
}
