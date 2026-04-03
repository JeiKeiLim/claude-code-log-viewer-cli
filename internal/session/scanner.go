// Package session provides multi-session detection and lifecycle management
// for Claude Code sessions (Phase 5a).
package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultSessionsPath returns the default Claude sessions directory path.
func DefaultSessionsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "sessions")
}

// ScanResult holds the results of a single session directory scan.
type ScanResult struct {
	Sessions   []ActiveSession
	ScanTime   time.Time
	Err        error
	IsFullScan bool // True when this result represents a complete directory scan (not a synthetic single-session event)
}

// SessionScanner polls the sessions directory for {pid}.json files
// and returns detected sessions with metadata.
type SessionScanner struct {
	sessionsDir  string
	pidChecker   PIDChecker
	interval     time.Duration
	jsonlBaseDir string // Optional override for JSONL file lookup directory (primarily for tests)

	mu       sync.Mutex
	stopCh   chan struct{}
	running  bool
	lastScan ScanResult
}

// ScannerOption configures a SessionScanner.
type ScannerOption func(*SessionScanner)

// WithScanInterval sets the polling interval (default 2s).
func WithScanInterval(d time.Duration) ScannerOption {
	return func(s *SessionScanner) {
		if d > 0 {
			s.interval = d
		}
	}
}

// WithScannerPIDChecker sets the PID checker implementation for the scanner.
func WithScannerPIDChecker(checker PIDChecker) ScannerOption {
	return func(s *SessionScanner) {
		s.pidChecker = checker
	}
}

// WithJSONLBaseDir overrides the directory in which the scanner looks for
// JSONL log files when checking session activity. By default the scanner
// derives the lookup directory from each session's CWD using CWDToProjectDir.
// Supplying a non-empty dir causes the scanner to look in that directory
// instead, which is useful in tests where JSONL files live in a temp directory
// rather than the real ~/.claude/projects tree.
func WithJSONLBaseDir(dir string) ScannerOption {
	return func(s *SessionScanner) {
		s.jsonlBaseDir = dir
	}
}

// NewSessionScanner creates a new SessionScanner that polls the given directory.
// If sessionsDir is empty, it defaults to ~/.claude/sessions/.
func NewSessionScanner(sessionsDir string, opts ...ScannerOption) *SessionScanner {
	if sessionsDir == "" {
		sessionsDir = DefaultSessionsPath()
	}

	s := &SessionScanner{
		sessionsDir: sessionsDir,
		pidChecker:  NewPIDChecker(),
		interval:    2 * time.Second,
		stopCh:      make(chan struct{}),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// SessionsDir returns the configured sessions directory path.
func (s *SessionScanner) SessionsDir() string {
	return s.sessionsDir
}

// Interval returns the configured polling interval.
func (s *SessionScanner) Interval() time.Duration {
	return s.interval
}

// Scan performs a single scan of the sessions directory and returns
// all detected session files with their parsed metadata. Only sessions
// whose PIDs are alive and whose JSONL log file exists are included.
// Sessions without a JSONL log file are skipped entirely — PID liveness
// alone is never sufficient.
func (s *SessionScanner) Scan() ScanResult {
	result := ScanResult{
		ScanTime:   time.Now(),
		IsFullScan: true,
	}

	entries, err := os.ReadDir(s.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist yet — not an error, just no sessions
			return result
		}
		result.Err = fmt.Errorf("reading sessions directory: %w", err)
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		pid, ok := parsePIDFromFilename(name)
		if !ok {
			continue
		}

		// Check PID liveness
		if !s.pidChecker.IsAlive(pid) {
			continue
		}

		filePath := filepath.Join(s.sessionsDir, name)
		session, err := ParseSessionFile(filePath, pid)
		if err != nil {
			continue // Skip unreadable/corrupt/invalid files
		}

		// Skip sessions without a JSONL log file.
		// The lookup directory is either the override (jsonlBaseDir, used in tests)
		// or the project directory derived from the session's CWD.
		jsonlLookupDir := s.jsonlBaseDir
		if jsonlLookupDir == "" {
			// Default: derive from CWD — requires a non-empty JSONLDir.
			if session.JSONLDir == "" {
				continue
			}
			jsonlLookupDir = session.JSONLDir
		}
		jsonlPath := filepath.Join(jsonlLookupDir, session.Meta.SessionID+".jsonl")
		jsonlStat, err := os.Stat(jsonlPath)
		if err != nil {
			continue // JSONL file does not exist or is inaccessible
		}

		// Use JSONL file last-modified time as the primary activity signal.
		modTime := jsonlStat.ModTime()
		session.JSONLLastModified = modTime
		session.State = ClassifySessionState(modTime, result.ScanTime)

		result.Sessions = append(result.Sessions, session)
	}

	return result
}

// JSONLPath returns the full path to the JSONL log file for a session.
// Returns an empty string if JSONLDir is empty.
func JSONLPath(sess ActiveSession) string {
	if sess.JSONLDir == "" {
		return ""
	}
	return filepath.Join(sess.JSONLDir, sess.Meta.SessionID+".jsonl")
}

// ClassifySessionState determines the lifecycle state of a session based on
// the JSONL file's last modification time relative to the current time.
//
// State transitions:
//   - Active: JSONL modified within IdleThreshold (2 minutes)
//   - Idle: JSONL not modified for IdleThreshold..RemovalThreshold (2–5 minutes)
//   - Removed: JSONL not modified for longer than RemovalThreshold (5+ minutes)
func ClassifySessionState(jsonlModTime, now time.Time) SessionState {
	age := now.Sub(jsonlModTime)
	if age >= RemovalThreshold {
		return SessionRemoved
	}
	if age >= IdleThreshold {
		return SessionIdle
	}
	return SessionActive
}

// ScanOnce performs a scan and caches the result. Thread-safe.
func (s *SessionScanner) ScanOnce() ScanResult {
	result := s.Scan()
	s.mu.Lock()
	s.lastScan = result
	s.mu.Unlock()
	return result
}

// LastScan returns the most recent cached scan result.
func (s *SessionScanner) LastScan() ScanResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastScan
}

// Start begins periodic scanning in a background goroutine.
// Detected session changes are sent to the returned channel.
// Call Stop() to terminate scanning.
func (s *SessionScanner) Start() <-chan ScanResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		// Already running; return nil to signal caller
		return nil
	}

	s.running = true
	s.stopCh = make(chan struct{})
	resultCh := make(chan ScanResult, 1)

	go s.pollLoop(resultCh)

	return resultCh
}

// Stop terminates the background polling goroutine.
// It is safe to call Stop multiple times.
func (s *SessionScanner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
}

// IsRunning reports whether the scanner is actively polling.
func (s *SessionScanner) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// pollLoop runs periodic scans and sends results to the channel.
func (s *SessionScanner) pollLoop(resultCh chan<- ScanResult) {
	defer close(resultCh)

	// Perform initial scan immediately
	result := s.ScanOnce()
	select {
	case resultCh <- result:
	case <-s.stopCh:
		return
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			result := s.ScanOnce()
			select {
			case resultCh <- result:
			case <-s.stopCh:
				return
			default:
				// Channel full, skip this result to avoid blocking
			}
		}
	}
}

// parsePIDFromFilename extracts a PID from a filename like "12345.json".
// Returns the PID and true if the filename is valid, otherwise 0 and false.
func parsePIDFromFilename(name string) (int, bool) {
	if !strings.HasSuffix(name, ".json") {
		return 0, false
	}
	base := strings.TrimSuffix(name, ".json")
	if base == "" {
		return 0, false
	}
	pid, err := strconv.Atoi(base)
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// CWDToProjectDir converts a CWD path to the Claude projects directory path.
// Claude encodes project paths: /Users/foo/bar -> -Users-foo-bar
func CWDToProjectDir(cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// Encode the CWD as Claude does: replace path separators with hyphens
	encoded := strings.ReplaceAll(cwd, string(os.PathSeparator), "-")
	// On Unix, paths start with / which becomes a leading -
	// On Windows, this would need different handling
	return filepath.Join(home, ".claude", "projects", encoded)
}

// DeduplicateBySessionID removes duplicate sessions that share the same sessionId,
// keeping only the one with the highest (latest) PID. When multiple PIDs reference
// the same sessionId (e.g., after a Claude Code process restart), only the most
// recent incarnation should be shown to avoid ghost panes.
//
// The relative order of surviving sessions is preserved (matches their first
// winning appearance in the input slice). The input slice is not modified;
// a new slice is returned. Returns nil when the input is empty.
func DeduplicateBySessionID(sessions []ActiveSession) []ActiveSession {
	if len(sessions) == 0 {
		return nil
	}

	// First pass: determine the highest PID for each sessionId.
	bestPID := make(map[string]int, len(sessions))
	for _, s := range sessions {
		id := s.Meta.SessionID
		if id == "" {
			id = s.FilePath
		}
		if s.Meta.PID > bestPID[id] {
			bestPID[id] = s.Meta.PID
		}
	}

	// Second pass: iterate in original order, keeping only the session whose
	// PID equals the best PID for its sessionId. This preserves insertion order
	// and ensures stable, deterministic output even when no duplicates exist.
	seen := make(map[string]bool, len(bestPID))
	out := make([]ActiveSession, 0, len(bestPID))
	for _, s := range sessions {
		id := s.Meta.SessionID
		if id == "" {
			id = s.FilePath
		}
		if !seen[id] && s.Meta.PID == bestPID[id] {
			out = append(out, s)
			seen[id] = true
		}
	}
	return out
}

// FilterByProject returns the subset of sessions whose CWD matches the given
// projectCWD path. Comparison is done after filepath.Clean on both sides to
// normalise trailing slashes and redundant separators.
//
// Use this to narrow a full scan result to sessions belonging to one project:
//
//	active := FilterByProject(result.Sessions, "/Users/me/my-project")
//
// Returns nil (not an empty slice) when no sessions match.
func FilterByProject(sessions []ActiveSession, projectCWD string) []ActiveSession {
	if len(sessions) == 0 || projectCWD == "" {
		return nil
	}
	clean := filepath.Clean(projectCWD)
	var out []ActiveSession
	for _, s := range sessions {
		if s.Meta.CWD != "" && filepath.Clean(s.Meta.CWD) == clean {
			out = append(out, s)
		}
	}
	return out
}
