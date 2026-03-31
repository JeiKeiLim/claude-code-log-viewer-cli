// Package session provides multi-session detection and lifecycle management
// for Claude Code sessions (Phase 5a).
package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// SessionDirectoryWatcher monitors the sessions directory using fsnotify for
// new {pid}.json files, emitting SessionOpened events within 1 second of file
// creation. Unlike SessionScanner (which polls every 2s), this watcher is
// event-driven and provides near-instant session detection.
//
// Usage:
//
//	w, err := NewSessionDirectoryWatcher("")
//	if err != nil { ... }
//	defer w.Close()
//	if err := w.Start(); err != nil { ... }
//	for event := range w.Events() { ... }
type SessionDirectoryWatcher struct {
	sessionsDir string
	pidChecker  PIDChecker
	fsWatcher   *fsnotify.Watcher
	events      chan SessionEvent

	mu       sync.Mutex
	closed   bool
	seenPIDs map[int]struct{} // PIDs for which SessionOpened has been emitted

	wg sync.WaitGroup
}

// DirWatcherOption configures a SessionDirectoryWatcher.
type DirWatcherOption func(*SessionDirectoryWatcher)

// WithDirWatcherPIDChecker sets the PID checker implementation (useful for testing).
func WithDirWatcherPIDChecker(checker PIDChecker) DirWatcherOption {
	return func(w *SessionDirectoryWatcher) {
		w.pidChecker = checker
	}
}

// WithDirWatcherEventBuffer sets the event channel buffer size (default 16).
func WithDirWatcherEventBuffer(size int) DirWatcherOption {
	return func(w *SessionDirectoryWatcher) {
		if size > 0 {
			w.events = make(chan SessionEvent, size)
		}
	}
}

// NewSessionDirectoryWatcher creates a new fsnotify-based directory watcher.
// If sessionsDir is empty, it defaults to ~/.claude/sessions/.
// Call Start() to begin monitoring and Close() to release all resources.
func NewSessionDirectoryWatcher(sessionsDir string, opts ...DirWatcherOption) (*SessionDirectoryWatcher, error) {
	if sessionsDir == "" {
		sessionsDir = DefaultSessionsPath()
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	w := &SessionDirectoryWatcher{
		sessionsDir: sessionsDir,
		pidChecker:  NewPIDChecker(),
		fsWatcher:   fsw,
		events:      make(chan SessionEvent, 16),
		seenPIDs:    make(map[int]struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// Start begins watching the sessions directory. It creates the directory if
// it does not exist (Claude Code may not have run yet). Returns an error if
// the directory cannot be registered with fsnotify.
// Start may only be called once per watcher instance.
func (w *SessionDirectoryWatcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return errors.New("session directory watcher is closed")
	}

	// Create the sessions directory if it doesn't exist; Claude Code creates
	// it on first run, so it may be absent on a fresh system.
	if err := os.MkdirAll(w.sessionsDir, 0755); err != nil {
		return fmt.Errorf("creating sessions directory %s: %w", w.sessionsDir, err)
	}

	if err := w.fsWatcher.Add(w.sessionsDir); err != nil {
		return fmt.Errorf("watching sessions directory %s: %w", w.sessionsDir, err)
	}

	w.wg.Add(1)
	go w.watchLoop()

	return nil
}

// Close stops watching and releases all resources. It is idempotent and safe
// to call multiple times. Close blocks until the internal goroutine exits.
//
// IMPORTANT (macOS kqueue): We must remove all watched paths before calling
// fsWatcher.Close() to prevent file descriptor leaks. This matches the pattern
// used by the existing watcher.Watcher implementation.
func (w *SessionDirectoryWatcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	// Remove all watched paths before closing (macOS kqueue FD leak fix).
	for _, path := range w.fsWatcher.WatchList() {
		_ = w.fsWatcher.Remove(path) // Ignore errors — path may be deleted
	}
	err := w.fsWatcher.Close()

	w.wg.Wait() // Wait for watchLoop to exit cleanly
	return err
}

// Events returns the read-only channel on which session lifecycle events are sent.
// The channel is buffered (default 16); events may be dropped if the caller
// does not drain it. The channel is never closed by the watcher itself.
func (w *SessionDirectoryWatcher) Events() <-chan SessionEvent {
	return w.events
}

// SessionsDir returns the configured sessions directory path.
func (w *SessionDirectoryWatcher) SessionsDir() string {
	return w.sessionsDir
}

// IsClosed reports whether the watcher has been closed.
func (w *SessionDirectoryWatcher) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

// watchLoop is the main goroutine that processes fsnotify events.
// It exits when the fsWatcher's channels are closed (after Close() is called).
func (w *SessionDirectoryWatcher) watchLoop() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return // fsWatcher closed
			}
			w.handleFSEvent(event)
		case _, ok := <-w.fsWatcher.Errors:
			if !ok {
				return // fsWatcher closed
			}
			// Errors are silently ignored; session detection continues.
			// The polling SessionScanner provides a fallback for missed events.
		}
	}
}

// handleFSEvent processes a single fsnotify event from the sessions directory.
// It ignores non-{pid}.json files and dispatches to the appropriate handler.
func (w *SessionDirectoryWatcher) handleFSEvent(event fsnotify.Event) {
	filename := filepath.Base(event.Name)
	pid, ok := parsePIDFromFilename(filename)
	if !ok {
		return // Not a {pid}.json file — ignore
	}

	switch {
	case event.Has(fsnotify.Create), event.Has(fsnotify.Write):
		// On Create: the file may not be fully written yet; the subsequent
		// Write event (if any) will retry via the same handler.
		w.handleSessionAppeared(pid, event.Name)
	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		w.handleSessionRemoved(pid, event.Name)
	}
}

// handleSessionAppeared processes a Create or Write event for a {pid}.json file.
// It validates PID liveness, reads session metadata, and emits a SessionOpened
// event. Duplicate events for the same PID are suppressed via seenPIDs.
func (w *SessionDirectoryWatcher) handleSessionAppeared(pid int, filePath string) {
	// Fast path: check if already seen (avoids syscall for duplicates).
	w.mu.Lock()
	if _, seen := w.seenPIDs[pid]; seen {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	// Check PID liveness before reading metadata.
	if !w.pidChecker.IsAlive(pid) {
		return
	}

	// Read and validate session metadata. On Create events the file may not
	// be fully written yet — ParseSessionFile will return an error in that
	// case, and the subsequent Write event will trigger another attempt.
	sess, err := ParseSessionFile(filePath, pid)
	if err != nil {
		return
	}

	// Double-checked lock: mark as seen and emit event atomically.
	w.mu.Lock()
	if _, seen := w.seenPIDs[pid]; seen {
		w.mu.Unlock()
		return // Another goroutine raced us
	}
	w.seenPIDs[pid] = struct{}{}
	w.mu.Unlock()

	select {
	case w.events <- SessionEvent{Type: SessionOpened, Session: sess}:
	default:
		// Channel full — drop event to avoid blocking the watcher goroutine.
		// The caller should ensure the events channel is drained promptly.
		// Remove from seenPIDs so the next Write event can retry.
		w.mu.Lock()
		delete(w.seenPIDs, pid)
		w.mu.Unlock()
	}
}

// handleSessionRemoved processes a Remove or Rename event for a {pid}.json file.
// It emits a SessionClosed event only if a corresponding SessionOpened was emitted.
func (w *SessionDirectoryWatcher) handleSessionRemoved(pid int, filePath string) {
	w.mu.Lock()
	_, wasSeen := w.seenPIDs[pid]
	if wasSeen {
		delete(w.seenPIDs, pid)
	}
	w.mu.Unlock()

	// Only emit SessionClosed if we previously emitted SessionOpened for this PID.
	if !wasSeen {
		return
	}

	sess := ActiveSession{
		Meta:     SessionMeta{PID: pid},
		FilePath: filePath,
	}

	select {
	case w.events <- SessionEvent{Type: SessionClosed, Session: sess}:
	default:
		// Channel full — drop event.
	}
}
