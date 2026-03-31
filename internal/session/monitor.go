package session

import (
	"context"
	"sync"
	"time"
)

// DefaultPollInterval is the default interval for PID liveness polling.
// Set to 2 seconds to ensure closure detection within the 5-second SLA
// while meeting the ≤2s configurable interval requirement. Worst-case
// detection latency: ~2s (one missed poll cycle) + event emission overhead.
const DefaultPollInterval = 2 * time.Second

// MinPollInterval is the minimum allowed poll interval.
const MinPollInterval = 500 * time.Millisecond

// MaxPollInterval is the maximum allowed poll interval.
const MaxPollInterval = 10 * time.Second

// Monitor watches active sessions and detects when their PIDs exit.
// It polls PID liveness at a configurable interval and emits SessionClosed
// events through the Events channel when a tracked PID is no longer alive.
type Monitor struct {
	checker      PIDChecker
	pollInterval time.Duration
	events       chan SessionEvent

	mu       sync.RWMutex
	sessions map[int]ActiveSession // keyed by PID

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// MonitorOption configures a Monitor.
type MonitorOption func(*Monitor)

// WithPollInterval sets the PID polling interval.
// Values outside [MinPollInterval, MaxPollInterval] are clamped.
func WithPollInterval(d time.Duration) MonitorOption {
	return func(m *Monitor) {
		if d < MinPollInterval {
			d = MinPollInterval
		}
		if d > MaxPollInterval {
			d = MaxPollInterval
		}
		m.pollInterval = d
	}
}

// WithMonitorPIDChecker sets a custom PID checker for the monitor (useful for testing).
func WithMonitorPIDChecker(checker PIDChecker) MonitorOption {
	return func(m *Monitor) {
		m.checker = checker
	}
}

// WithEventBufferSize sets the event channel buffer size.
func WithEventBufferSize(size int) MonitorOption {
	return func(m *Monitor) {
		if size < 1 {
			size = 1
		}
		m.events = make(chan SessionEvent, size)
	}
}

// NewMonitor creates a new session monitor.
// The monitor does not start polling until Start() is called.
func NewMonitor(opts ...MonitorOption) *Monitor {
	m := &Monitor{
		checker:      NewPIDChecker(),
		pollInterval: DefaultPollInterval,
		events:       make(chan SessionEvent, 16),
		sessions:     make(map[int]ActiveSession),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Start begins the PID polling loop. It is safe to call Start only once.
// Use Stop() to terminate the polling loop.
func (m *Monitor) Start(ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.wg.Add(1)
	go m.pollLoop()
}

// Stop terminates the polling loop and waits for it to finish.
// It is safe to call Stop multiple times; subsequent calls are no-ops.
func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

// Events returns the channel on which session closure events are sent.
// The channel is never closed by the monitor; callers should use context
// cancellation or Stop() to signal completion.
func (m *Monitor) Events() <-chan SessionEvent {
	return m.events
}

// TrackSession adds a session to be monitored. If the PID is already
// tracked, the session metadata is updated.
func (m *Monitor) TrackSession(session ActiveSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.Meta.PID] = session
}

// UntrackSession removes a session from monitoring by PID.
func (m *Monitor) UntrackSession(pid int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, pid)
}

// TrackedSessions returns a snapshot of all currently tracked sessions.
func (m *Monitor) TrackedSessions() []ActiveSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ActiveSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// TrackedCount returns the number of currently tracked sessions.
func (m *Monitor) TrackedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// IsTracking returns true if the given PID is being tracked.
func (m *Monitor) IsTracking(pid int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.sessions[pid]
	return ok
}

// pollLoop is the main polling goroutine. It checks all tracked PIDs
// at the configured interval and emits closure events for dead PIDs.
func (m *Monitor) pollLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllPIDs()
		}
	}
}

// checkAllPIDs iterates all tracked sessions and checks PID liveness.
// Dead PIDs are removed from tracking and closure events are emitted.
func (m *Monitor) checkAllPIDs() {
	// Take a snapshot of current sessions under read lock
	m.mu.RLock()
	snapshot := make(map[int]ActiveSession, len(m.sessions))
	for pid, s := range m.sessions {
		snapshot[pid] = s
	}
	m.mu.RUnlock()

	// Check each PID (no lock held during syscall)
	var dead []int
	for pid := range snapshot {
		if !m.checker.IsAlive(pid) {
			dead = append(dead, pid)
		}
	}

	if len(dead) == 0 {
		return
	}

	// Remove dead PIDs under write lock
	m.mu.Lock()
	closedSessions := make([]ActiveSession, 0, len(dead))
	for _, pid := range dead {
		if s, ok := m.sessions[pid]; ok {
			closedSessions = append(closedSessions, s)
			delete(m.sessions, pid)
		}
	}
	m.mu.Unlock()

	// Emit events (outside lock)
	for _, s := range closedSessions {
		event := SessionEvent{
			Type:    SessionClosed,
			Session: s,
		}
		select {
		case m.events <- event:
		case <-m.ctx.Done():
			return
		}
	}
}

// CheckNow performs an immediate PID liveness check for all tracked sessions.
// This is useful for testing or when immediate detection is needed (e.g., on
// user action). It runs synchronously in the caller's goroutine.
func (m *Monitor) CheckNow() {
	m.checkAllPIDs()
}
