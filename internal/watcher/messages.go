// Package watcher provides file watching capabilities for live log updates.
package watcher

import "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"

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
