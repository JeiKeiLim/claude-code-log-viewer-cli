// Package session provides multi-session detection and lifecycle management
// for Claude Code sessions (Phase 5a).
package session

import (
	"encoding/json"
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
	Sessions []ActiveSession
	ScanTime time.Time
	Err      error
}

// SessionScanner polls the sessions directory for {pid}.json files
// and returns detected sessions with metadata.
type SessionScanner struct {
	sessionsDir string
	pidChecker  PIDChecker
	interval    time.Duration

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
// whose PIDs are alive are included.
func (s *SessionScanner) Scan() ScanResult {
	result := ScanResult{
		ScanTime: time.Now(),
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

		result.Sessions = append(result.Sessions, session)
	}

	return result
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

// readSessionMeta reads and parses a session JSON file.
func readSessionMeta(filePath string) (SessionMeta, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SessionMeta{}, err
	}

	var meta SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return SessionMeta{}, fmt.Errorf("parsing session file %s: %w", filePath, err)
	}

	return meta, nil
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
