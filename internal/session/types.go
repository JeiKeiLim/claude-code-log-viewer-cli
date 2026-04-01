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

// SessionState represents the lifecycle state of a session.
type SessionState int

const (
	// SessionActive means the session's JSONL file was modified within the idle threshold.
	SessionActive SessionState = iota
	// SessionIdle means the JSONL file has not been modified for longer than the idle threshold
	// but less than the removal threshold.
	SessionIdle
	// SessionRemoved means the JSONL file has not been modified for longer than the removal
	// threshold, and the session should be removed from the dashboard.
	SessionRemoved
)

// String returns a human-readable label for the session state.
func (s SessionState) String() string {
	switch s {
	case SessionActive:
		return "Active"
	case SessionIdle:
		return "Idle"
	case SessionRemoved:
		return "Removed"
	default:
		return "Unknown"
	}
}

// IdleThreshold is the duration of no JSONL writes after which a session is considered idle.
const IdleThreshold = 2 * time.Minute

// RemovalThreshold is the duration of no JSONL writes after which a session is removed.
const RemovalThreshold = 5 * time.Minute

// ActiveSession represents a detected active Claude Code session.
type ActiveSession struct {
	Meta              SessionMeta
	FilePath          string       // Path to the session JSON file
	JSONLDir          string       // Project directory containing the JSONL file
	State             SessionState // Lifecycle state (Active, Idle)
	JSONLLastModified time.Time    // Last modification time of the JSONL log file
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
