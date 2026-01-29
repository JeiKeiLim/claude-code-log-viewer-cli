// Package watcher provides file watching capabilities for live log updates.
package watcher

import (
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// NewEntriesMsg signals new entries were appended to the log file.
type NewEntriesMsg struct {
	Entries []types.LogEntry
}

// WatcherErrorMsg signals an error occurred in the watcher.
type WatcherErrorMsg struct {
	Err error
}

// FileResetMsg signals the file was truncated (new session started).
type FileResetMsg struct{}

// NewConversationMsg signals a new conversation file was created (Story 11.2).
// Used by ProjectWatcher to notify the viewer of new .jsonl files.
type NewConversationMsg struct {
	FilePath     string    // Full path to the new conversation file
	CreationTime time.Time // File creation time (birthtime) for comparison
}
