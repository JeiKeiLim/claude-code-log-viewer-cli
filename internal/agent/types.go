// Package agent defines provider interfaces for multi-agent log viewing.
package agent

import "time"

// AgentType identifies which agent produced a log session.
type AgentType string

const (
	AgentClaudeCode AgentType = "claude-code"
	AgentCodex      AgentType = "codex"
	AgentOpenCode   AgentType = "opencode"
)

// String returns the human-readable agent name.
func (a AgentType) String() string {
	switch a {
	case AgentClaudeCode:
		return "Claude Code"
	case AgentCodex:
		return "Codex"
	case AgentOpenCode:
		return "OpenCode"
	default:
		return string(a)
	}
}

// Badge returns the text badge for display in the TUI.
func (a AgentType) Badge() string {
	switch a {
	case AgentClaudeCode:
		return "[C]"
	case AgentCodex:
		return "[X]"
	case AgentOpenCode:
		return "[O]"
	default:
		return "[?]"
	}
}

// Project represents an agent-specific project directory.
type Project struct {
	// Path is the decoded filesystem path (e.g., "/Users/alice/myproject").
	Path string
	// Directory is the working directory of the project (alias for Path, used by providers).
	Directory string
	// DisplayName is the short name for the TUI (e.g., "myproject").
	DisplayName string
	// AgentType identifies which agent this project belongs to.
	AgentType AgentType
	// SessionCount is the number of sessions in this project.
	SessionCount int
}

// Session represents a single conversation session within a project.
type Session struct {
	// ID is the unique session identifier (filename stem, hash, or DB row ID).
	ID string
	// ProjectPath is the filesystem path of the parent project.
	ProjectPath string
	// FilePath is the full path to the session data (JSONL file or DB path).
	FilePath string
	// AgentType identifies which agent produced this session.
	AgentType AgentType
	// CreatedAt is the session creation timestamp.
	CreatedAt time.Time
	// LastModified is when the session was last updated.
	LastModified time.Time
	// MessageCount is the number of conversation entries.
	MessageCount int
	// FirstUserMessage is a preview of the first user message (truncated).
	FirstUserMessage string
	// Model is the primary model used in the session.
	Model string
	// Tokens is the total token usage for the session.
	Tokens TokenStats
	// TurnCount is the number of user-assistant turn pairs.
	TurnCount int
}
