// Package session provides multi-session detection and lifecycle management
// for Claude Code sessions (Phase 5a).
package session

import (
	"context"
	"sync"
	"time"
)

// Registry maintains the authoritative set of active Claude Code sessions.
// It aggregates detection from SessionScanner (polling, 2s default interval)
// and an optional SessionDirectoryWatcher (event-driven fsnotify, ~1s latency)
// to ensure sessions are discovered within the 2-second detection window.
// A Monitor polls PID liveness to detect session closures.
//
// All three sources feed a single unified events channel that callers can
// consume to react to session open/close events.
//
// Usage:
//
//	r := NewRegistry("") // defaults to ~/.claude/sessions/
//	r.Start(ctx)
//	defer r.Stop()
//	for event := range r.Events() { ... }
type Registry struct {
	scanner    *SessionScanner
	dirWatcher *SessionDirectoryWatcher // nil when not available
	monitor    *Monitor

	events chan SessionEvent

	mu       sync.RWMutex
	sessions map[int]ActiveSession // keyed by PID

	running bool // guards against double-Start
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// registryConfig holds configuration for building a Registry.
// It is populated by RegistryOption functions before any component is created.
type registryConfig struct {
	sessionsDir     string
	scanInterval    time.Duration
	monitorInterval time.Duration
	pidChecker      PIDChecker
	eventBuffer     int
	dirWatcher      *SessionDirectoryWatcher // pre-provided watcher (for tests)
	noDirWatcher    bool                     // disable auto-creation of dir watcher
}

// RegistryOption configures a Registry.
type RegistryOption func(*registryConfig)

// WithRegistryScanInterval sets the polling interval for the session scanner.
// The interval is passed directly to SessionScanner; values outside the
// scanner's accepted range are handled by the scanner itself.
func WithRegistryScanInterval(d time.Duration) RegistryOption {
	return func(cfg *registryConfig) {
		cfg.scanInterval = d
	}
}

// WithRegistryMonitorInterval sets the PID polling interval for the Monitor.
// Values are clamped to [MinPollInterval, MaxPollInterval] by the Monitor.
func WithRegistryMonitorInterval(d time.Duration) RegistryOption {
	return func(cfg *registryConfig) {
		cfg.monitorInterval = d
	}
}

// WithRegistryPIDChecker sets the PID checker for all internal detection
// components (scanner, dir watcher, and monitor). Primarily useful for testing.
func WithRegistryPIDChecker(checker PIDChecker) RegistryOption {
	return func(cfg *registryConfig) {
		cfg.pidChecker = checker
	}
}

// WithRegistryEventBuffer sets the capacity of the unified events channel.
func WithRegistryEventBuffer(size int) RegistryOption {
	return func(cfg *registryConfig) {
		if size > 0 {
			cfg.eventBuffer = size
		}
	}
}

// WithRegistryDirWatcher provides a pre-created SessionDirectoryWatcher.
// When set, the Registry skips auto-creating its own watcher. Useful for
// testing and for callers that need to share a single watcher instance.
func WithRegistryDirWatcher(w *SessionDirectoryWatcher) RegistryOption {
	return func(cfg *registryConfig) {
		cfg.dirWatcher = w
	}
}

// WithRegistryNoDirWatcher disables automatic creation of the fsnotify-based
// directory watcher. The registry will rely solely on polling for session
// detection. Primarily useful in test environments where fsnotify setup is
// undesirable.
func WithRegistryNoDirWatcher() RegistryOption {
	return func(cfg *registryConfig) {
		cfg.noDirWatcher = true
	}
}

// NewRegistry creates a new Registry configured to watch the given sessions
// directory. If sessionsDir is empty, it defaults to ~/.claude/sessions/.
//
// The registry does not start automatically; call Start() to begin detection.
// Call Stop() to terminate all background goroutines and release resources.
func NewRegistry(sessionsDir string, opts ...RegistryOption) *Registry {
	if sessionsDir == "" {
		sessionsDir = DefaultSessionsPath()
	}

	cfg := &registryConfig{
		sessionsDir:     sessionsDir,
		scanInterval:    2 * time.Second,
		monitorInterval: DefaultPollInterval,
		eventBuffer:     32,
		pidChecker:      NewPIDChecker(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Build Scanner
	scanner := NewSessionScanner(
		cfg.sessionsDir,
		WithScanInterval(cfg.scanInterval),
		WithScannerPIDChecker(cfg.pidChecker),
	)

	// Build Monitor
	monitor := NewMonitor(
		WithMonitorPIDChecker(cfg.pidChecker),
		WithPollInterval(cfg.monitorInterval),
	)

	r := &Registry{
		scanner:  scanner,
		monitor:  monitor,
		events:   make(chan SessionEvent, cfg.eventBuffer),
		sessions: make(map[int]ActiveSession),
	}

	// Attach dir watcher: use provided one, auto-create, or disable.
	switch {
	case cfg.dirWatcher != nil:
		r.dirWatcher = cfg.dirWatcher
	case !cfg.noDirWatcher:
		w, err := NewSessionDirectoryWatcher(
			cfg.sessionsDir,
			WithDirWatcherPIDChecker(cfg.pidChecker),
		)
		if err == nil {
			r.dirWatcher = w
		}
		// If creation fails, continue in polling-only mode (degraded gracefully).
	}

	return r
}

// Start begins all background session detection goroutines.
// It is safe to call multiple times; subsequent calls are no-ops.
// The provided context controls the lifetime of the background goroutines:
// cancelling ctx is equivalent to calling Stop().
func (r *Registry) Start(ctx context.Context) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	r.ctx, r.cancel = context.WithCancel(ctx)

	// Start the fsnotify directory watcher first (if available) so it is
	// ready to emit events before the initial scan completes.
	if r.dirWatcher != nil {
		_ = r.dirWatcher.Start() // Errors are silently ignored; scanner covers the fallback.
	}

	// Start monitor (PID closure polling).
	r.monitor.Start(r.ctx)

	// Start scanner (polling loop); collect the result channel.
	scanCh := r.scanner.Start()

	// Launch the fan-in goroutine.
	r.wg.Add(1)
	go r.run(scanCh)
}

// Stop terminates all background goroutines and releases resources.
// It blocks until all goroutines have exited. It is safe to call Stop
// multiple times; subsequent calls are no-ops.
func (r *Registry) Stop() {
	if r.cancel != nil {
		r.cancel()
	}

	// Stop components in reverse start order.
	r.scanner.Stop()
	r.monitor.Stop()
	if r.dirWatcher != nil {
		_ = r.dirWatcher.Close()
	}

	r.wg.Wait()
}

// Events returns the read-only channel on which unified session lifecycle
// events are delivered. Both SessionOpened and SessionClosed events are
// emitted here, deduplicated across all detection sources.
//
// The channel is buffered (default 32). If the caller does not drain the
// channel promptly, events may be dropped silently to avoid blocking the
// detection pipeline.
//
// The channel is never closed by the Registry; callers should use context
// cancellation or Stop() to signal the end of the session.
func (r *Registry) Events() <-chan SessionEvent {
	return r.events
}

// Sessions returns a point-in-time snapshot of all currently active sessions.
// The returned slice is a copy; modifying it does not affect the Registry.
// Returns nil (not an empty slice) when no sessions are active.
func (r *Registry) Sessions() []ActiveSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.sessions) == 0 {
		return nil
	}
	out := make([]ActiveSession, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, s)
	}
	return out
}

// SessionCount returns the number of currently active sessions.
func (r *Registry) SessionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

// LookupSession returns the ActiveSession for the given PID and a boolean
// indicating whether the session was found.
func (r *Registry) LookupSession(pid int) (ActiveSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[pid]
	return s, ok
}

// run is the main fan-in goroutine. It reads from the scanner, dirWatcher,
// and monitor channels and updates the authoritative sessions map.
// It exits when the context is cancelled.
func (r *Registry) run(scanCh <-chan ScanResult) {
	defer r.wg.Done()

	differ := NewSessionDiffer()

	// A nil channel blocks forever in a select; this is the correct way to
	// disable a case when the dir watcher is absent or its channel closes.
	var dirWatcherCh <-chan SessionEvent
	if r.dirWatcher != nil {
		dirWatcherCh = r.dirWatcher.Events()
	}

	monitorCh := r.monitor.Events()

	for {
		select {
		case <-r.ctx.Done():
			return

		case result, ok := <-scanCh:
			if !ok {
				// Scanner stopped (its channel is closed when Stop() is called).
				// Continue processing dir watcher and monitor events until ctx
				// is cancelled.
				scanCh = nil // disable this case
				continue
			}
			diff := differ.Update(result)
			for _, s := range diff.Opened {
				r.addSession(s)
			}
			for _, s := range diff.Closed {
				r.removeSession(s.Meta.PID)
			}

		case event, ok := <-dirWatcherCh:
			if !ok {
				dirWatcherCh = nil // channel closed; disable this case
				continue
			}
			switch event.Type {
			case SessionOpened:
				r.addSession(event.Session)
			case SessionClosed:
				r.removeSession(event.Session.Meta.PID)
			}

		case event, ok := <-monitorCh:
			if !ok {
				monitorCh = nil // channel closed; disable this case
				continue
			}
			if event.Type == SessionClosed {
				r.removeSession(event.Session.Meta.PID)
			}
		}
	}
}

// addSession adds a session to the registry if it is not already present.
// If the session is new, it is added to the monitor for closure detection
// and a SessionOpened event is emitted. Duplicate sessions (same PID) are
// silently ignored, providing deduplication between the scanner and dir watcher.
func (r *Registry) addSession(s ActiveSession) {
	r.mu.Lock()
	_, exists := r.sessions[s.Meta.PID]
	if !exists {
		r.sessions[s.Meta.PID] = s
	}
	r.mu.Unlock()

	if exists {
		return // Already tracked; deduplication complete.
	}

	// Track in monitor for PID closure detection.
	r.monitor.TrackSession(s)

	// Emit event (non-blocking; drop if buffer full to avoid pipeline stalls).
	select {
	case r.events <- SessionEvent{Type: SessionOpened, Session: s}:
	default:
	}
}

// removeSession removes a session from the registry by PID. If the session
// was tracked, it is untracked from the monitor and a SessionClosed event is
// emitted. Idempotent: calling with an unknown PID is a no-op.
//
// The full session metadata from the registry is used in the emitted event,
// even when the closure signal (from monitor or dir watcher) carries only
// partial information (e.g., just a PID).
func (r *Registry) removeSession(pid int) {
	r.mu.Lock()
	s, exists := r.sessions[pid]
	if exists {
		delete(r.sessions, pid)
	}
	r.mu.Unlock()

	if !exists {
		return // Already removed; idempotent.
	}

	// Untrack from monitor so it stops polling this PID.
	r.monitor.UntrackSession(pid)

	// Emit closure event with full session metadata.
	select {
	case r.events <- SessionEvent{Type: SessionClosed, Session: s}:
	default:
	}
}
