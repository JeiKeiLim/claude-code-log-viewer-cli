// Package session provides multi-session detection and lifecycle management
// for Claude Code sessions (Phase 5a).
package session

import (
	"path/filepath"
	"time"
)

// SessionMeta represents the metadata from a ~/.claude/sessions/{pid}.json file.
type SessionMeta struct {
	PID        int    `json:"pid"`
	SessionID  string `json:"sessionId"`
	CWD        string `json:"cwd"`
	StartedAt  int64  `json:"startedAt"`
	Kind       string `json:"kind"`
	Entrypoint string `json:"entrypoint"`
}

// StartedAtTime returns the StartedAt timestamp as a time.Time.
func (s SessionMeta) StartedAtTime() time.Time {
	return time.UnixMilli(s.StartedAt)
}

// ProjectName returns the human-readable project name derived from the CWD.
// It returns the last path component of CWD (e.g., "my-project" from
// "/Users/user/projects/my-project"). Returns an empty string if CWD is empty.
func (s SessionMeta) ProjectName() string {
	if s.CWD == "" {
		return ""
	}
	return filepath.Base(s.CWD)
}

// ActiveSession represents a detected active Claude Code session.
type ActiveSession struct {
	Meta     SessionMeta
	FilePath string // Path to the session JSON file
	JSONLDir string // Project directory containing the JSONL file
}

// SessionEvent represents a lifecycle event for a session.
type SessionEvent struct {
	Type    SessionEventType
	Session ActiveSession
}

// SessionEventType describes what happened to a session.
type SessionEventType int

const (
	// SessionOpened means a new session was detected.
	SessionOpened SessionEventType = iota
	// SessionClosed means a session's PID is no longer alive.
	SessionClosed
)
