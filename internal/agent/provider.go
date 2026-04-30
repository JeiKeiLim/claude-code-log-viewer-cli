package agent

import "io"

// AgentProvider is the interface that each agent backend must implement.
type AgentProvider interface {
	// Type returns the agent type identifier.
	Type() AgentType
	// DisplayName returns the human-readable agent name (e.g., "Claude Code").
	DisplayName() string
	// Badge returns the text badge for the TUI (e.g., "[C]").
	Badge() string
	// IsAvailable returns true if the agent's storage is present and accessible.
	IsAvailable() bool

	// DiscoverProjects returns all projects found for this agent.
	DiscoverProjects() ([]Project, error)
	// DiscoverSessions returns all sessions within the given project.
	DiscoverSessions(project Project) ([]Session, error)

	// ParseSession parses a complete session file into conversation entries.
	ParseSession(session Session) ([]ConversationEntry, error)
	// ParseSessionStream parses a session from a reader (pipeline mode).
	ParseSessionStream(r io.Reader) ([]ConversationEntry, error)
}

// SessionWatcher watches a session for new entries (live mode).
type SessionWatcher interface {
	// NewEntries returns any entries appended since the last call.
	NewEntries() ([]ConversationEntry, error)
	// Close releases any resources held by the watcher.
	Close() error
}
