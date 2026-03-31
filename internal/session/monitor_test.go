package session

import (
	"context"
	"sync"
	"testing"
	"time"
)

// monitorMockPIDChecker is a test double for PIDChecker that allows controlling
// which PIDs are alive or dead.
type monitorMockPIDChecker struct {
	mu    sync.RWMutex
	alive map[int]bool
	calls int
}

func newMonitorMockPIDChecker() *monitorMockPIDChecker {
	return &monitorMockPIDChecker{alive: make(map[int]bool)}
}

func (m *monitorMockPIDChecker) SetAlive(pid int, alive bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alive[pid] = alive
}

func (m *monitorMockPIDChecker) IsAlive(pid int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.alive[pid]
}

func (m *monitorMockPIDChecker) CallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls
}

// helper to create a test session
func testSession(pid int, sessionID string) ActiveSession {
	return ActiveSession{
		Meta: SessionMeta{
			PID:       pid,
			SessionID: sessionID,
			CWD:       "/test/project",
			StartedAt: time.Now().UnixMilli(),
			Kind:      "interactive",
		},
		FilePath: "/home/user/.claude/sessions/" + sessionID + ".json",
		JSONLDir: "/home/user/.claude/projects/test",
	}
}

func TestNewMonitor_Defaults(t *testing.T) {
	m := NewMonitor()
	if m.pollInterval != DefaultPollInterval {
		t.Errorf("expected default poll interval %v, got %v", DefaultPollInterval, m.pollInterval)
	}
	if m.checker == nil {
		t.Error("expected non-nil PID checker")
	}
	if m.events == nil {
		t.Error("expected non-nil events channel")
	}
	if m.sessions == nil {
		t.Error("expected non-nil sessions map")
	}
}

func TestNewMonitor_WithOptions(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithPollInterval(2*time.Second),
		WithMonitorPIDChecker(checker),
		WithEventBufferSize(32),
	)
	if m.pollInterval != 2*time.Second {
		t.Errorf("expected 2s poll interval, got %v", m.pollInterval)
	}
	if m.checker != checker {
		t.Error("expected custom PID checker")
	}
}

func TestWithPollInterval_Clamping(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{"too small", 100 * time.Millisecond, MinPollInterval},
		{"minimum", MinPollInterval, MinPollInterval},
		{"normal", 3 * time.Second, 3 * time.Second},
		{"maximum", MaxPollInterval, MaxPollInterval},
		{"too large", 30 * time.Second, MaxPollInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMonitor(WithPollInterval(tt.input))
			if m.pollInterval != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, m.pollInterval)
			}
		})
	}
}

func TestWithEventBufferSize_MinimumOne(t *testing.T) {
	m := NewMonitor(WithEventBufferSize(0))
	if cap(m.events) != 1 {
		t.Errorf("expected buffer size 1, got %d", cap(m.events))
	}
}

func TestTrackSession(t *testing.T) {
	m := NewMonitor()
	s := testSession(1234, "test-session-1")

	m.TrackSession(s)

	if !m.IsTracking(1234) {
		t.Error("expected PID 1234 to be tracked")
	}
	if m.TrackedCount() != 1 {
		t.Errorf("expected 1 tracked session, got %d", m.TrackedCount())
	}
}

func TestTrackSession_UpdateExisting(t *testing.T) {
	m := NewMonitor()
	s1 := testSession(1234, "session-1")
	s2 := testSession(1234, "session-2") // same PID, different session

	m.TrackSession(s1)
	m.TrackSession(s2)

	if m.TrackedCount() != 1 {
		t.Errorf("expected 1 tracked session after update, got %d", m.TrackedCount())
	}
	sessions := m.TrackedSessions()
	if len(sessions) != 1 || sessions[0].Meta.SessionID != "session-2" {
		t.Error("expected session to be updated to session-2")
	}
}

func TestUntrackSession(t *testing.T) {
	m := NewMonitor()
	m.TrackSession(testSession(1234, "session-1"))
	m.TrackSession(testSession(5678, "session-2"))

	m.UntrackSession(1234)

	if m.IsTracking(1234) {
		t.Error("expected PID 1234 to be untracked")
	}
	if !m.IsTracking(5678) {
		t.Error("expected PID 5678 to still be tracked")
	}
	if m.TrackedCount() != 1 {
		t.Errorf("expected 1 tracked session, got %d", m.TrackedCount())
	}
}

func TestUntrackSession_NonExistent(t *testing.T) {
	m := NewMonitor()
	// Should not panic
	m.UntrackSession(9999)
}

func TestTrackedSessions_Snapshot(t *testing.T) {
	m := NewMonitor()
	m.TrackSession(testSession(1, "s1"))
	m.TrackSession(testSession(2, "s2"))
	m.TrackSession(testSession(3, "s3"))

	sessions := m.TrackedSessions()
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}

	// Verify it's a snapshot (modifying returned slice doesn't affect monitor)
	sessions = sessions[:0]
	if m.TrackedCount() != 3 {
		t.Error("modifying snapshot should not affect monitor")
	}
}

func TestIsTracking_NotTracked(t *testing.T) {
	m := NewMonitor()
	if m.IsTracking(9999) {
		t.Error("expected PID 9999 to not be tracked")
	}
}

func TestCheckNow_DetectsDeadPID(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithEventBufferSize(4),
	)

	// Start context for event emission
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.ctx, m.cancel = ctx, cancel

	// Track two sessions
	checker.SetAlive(1234, true)
	checker.SetAlive(5678, true)
	m.TrackSession(testSession(1234, "alive-session"))
	m.TrackSession(testSession(5678, "dying-session"))

	// Kill one PID
	checker.SetAlive(5678, false)

	// Run immediate check
	m.CheckNow()

	// Should get exactly one closure event
	select {
	case event := <-m.Events():
		if event.Type != SessionClosed {
			t.Errorf("expected SessionClosed event, got %d", event.Type)
		}
		if event.Session.Meta.PID != 5678 {
			t.Errorf("expected PID 5678, got %d", event.Session.Meta.PID)
		}
		if event.Session.Meta.SessionID != "dying-session" {
			t.Errorf("expected session ID 'dying-session', got %s", event.Session.Meta.SessionID)
		}
	default:
		t.Error("expected a closure event but channel was empty")
	}

	// Dead PID should be removed from tracking
	if m.IsTracking(5678) {
		t.Error("dead PID should be untracked")
	}
	if !m.IsTracking(1234) {
		t.Error("alive PID should still be tracked")
	}
}

func TestCheckNow_AllAlive(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithEventBufferSize(4),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.ctx, m.cancel = ctx, cancel

	checker.SetAlive(1234, true)
	checker.SetAlive(5678, true)
	m.TrackSession(testSession(1234, "s1"))
	m.TrackSession(testSession(5678, "s2"))

	m.CheckNow()

	select {
	case event := <-m.Events():
		t.Errorf("expected no events, got %v", event)
	default:
		// Good - no events
	}

	if m.TrackedCount() != 2 {
		t.Errorf("expected 2 tracked sessions, got %d", m.TrackedCount())
	}
}

func TestCheckNow_MultipleDeadPIDs(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithEventBufferSize(16),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.ctx, m.cancel = ctx, cancel

	// Track 5 sessions, kill 3
	for i := 1; i <= 5; i++ {
		checker.SetAlive(i*1000, i <= 2) // PIDs 1000, 2000 alive; 3000, 4000, 5000 dead
		m.TrackSession(testSession(i*1000, "session-"+string(rune('0'+i))))
	}

	m.CheckNow()

	// Collect all events
	closedPIDs := make(map[int]bool)
	for i := 0; i < 3; i++ {
		select {
		case event := <-m.Events():
			if event.Type != SessionClosed {
				t.Errorf("expected SessionClosed, got %d", event.Type)
			}
			closedPIDs[event.Session.Meta.PID] = true
		default:
			t.Fatalf("expected 3 events, got %d", i)
		}
	}

	if !closedPIDs[3000] || !closedPIDs[4000] || !closedPIDs[5000] {
		t.Errorf("expected PIDs 3000, 4000, 5000 to be closed, got %v", closedPIDs)
	}

	if m.TrackedCount() != 2 {
		t.Errorf("expected 2 alive sessions remaining, got %d", m.TrackedCount())
	}
}

func TestCheckNow_EmptySessions(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithEventBufferSize(4),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.ctx, m.cancel = ctx, cancel

	// No sessions tracked - should not panic
	m.CheckNow()

	select {
	case event := <-m.Events():
		t.Errorf("expected no events, got %v", event)
	default:
		// Good
	}
}

func TestPollLoop_DetectsClosure(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithPollInterval(100*time.Millisecond), // Will be clamped to MinPollInterval
		WithEventBufferSize(4),
	)

	checker.SetAlive(1234, true)
	m.TrackSession(testSession(1234, "test-session"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	// Wait for at least one poll cycle to verify it's alive
	time.Sleep(600 * time.Millisecond)

	// Kill the PID
	checker.SetAlive(1234, false)

	// Wait for detection (should be within poll interval)
	select {
	case event := <-m.Events():
		if event.Type != SessionClosed {
			t.Errorf("expected SessionClosed, got %d", event.Type)
		}
		if event.Session.Meta.PID != 1234 {
			t.Errorf("expected PID 1234, got %d", event.Session.Meta.PID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for closure event")
	}
}

func TestPollLoop_DetectsClosureWithin5Seconds(t *testing.T) {
	// This test validates the core AC: session closure detected within 5 seconds
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithPollInterval(MinPollInterval), // 500ms for fast test
		WithEventBufferSize(4),
	)

	checker.SetAlive(1234, true)
	m.TrackSession(testSession(1234, "test-session"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	// Kill the PID and start timing
	checker.SetAlive(1234, false)
	start := time.Now()

	select {
	case event := <-m.Events():
		elapsed := time.Since(start)
		if event.Type != SessionClosed {
			t.Errorf("expected SessionClosed, got %d", event.Type)
		}
		if elapsed > 5*time.Second {
			t.Errorf("closure detection took %v, exceeding 5s SLA", elapsed)
		}
		t.Logf("closure detected in %v", elapsed)
	case <-time.After(5 * time.Second):
		t.Fatal("closure not detected within 5 seconds")
	}
}

func TestPollLoop_DefaultIntervalMeetsSLA(t *testing.T) {
	// Verify that default poll interval (2s) guarantees sub-5s detection.
	// Worst case: PID dies immediately after a poll check.
	// Next check is 2s later, event emission is near-instant.
	// Total worst case: ~2s < 5s ✓
	if DefaultPollInterval >= 5*time.Second {
		t.Errorf("default poll interval %v is too large for 5s SLA", DefaultPollInterval)
	}
	// Allow for some overhead (scheduling, syscall time)
	if DefaultPollInterval > 4*time.Second {
		t.Errorf("default poll interval %v leaves insufficient margin for 5s SLA", DefaultPollInterval)
	}
}

func TestPollLoop_DefaultIntervalMeetsSubAC1Constraint(t *testing.T) {
	// Sub-AC 1 requires the configurable poll interval default to be ≤2s.
	// This test explicitly validates that constraint.
	if DefaultPollInterval > 2*time.Second {
		t.Errorf("DefaultPollInterval %v exceeds the ≤2s sub-AC 1 requirement", DefaultPollInterval)
	}
	// Also verify the minimum interval supports fast detection (well under 2s).
	if MinPollInterval > 2*time.Second {
		t.Errorf("MinPollInterval %v exceeds the ≤2s requirement", MinPollInterval)
	}
}

func TestStop_GracefulShutdown(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithPollInterval(MinPollInterval),
	)

	checker.SetAlive(1234, true)
	m.TrackSession(testSession(1234, "test"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// Stop should return promptly
	done := make(chan struct{})
	go func() {
		m.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2 seconds")
	}
}

func TestStop_Idempotent(t *testing.T) {
	m := NewMonitor()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// Multiple stops should not panic
	m.Stop()
	m.Stop()
	m.Stop()
}

func TestContextCancellation_StopsPolling(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithPollInterval(MinPollInterval),
	)

	checker.SetAlive(1234, true)
	m.TrackSession(testSession(1234, "test"))

	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)

	// Cancel context
	cancel()

	// Wait for goroutine to exit
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good - goroutine exited
	case <-time.After(2 * time.Second):
		t.Fatal("polling goroutine did not exit after context cancellation")
	}
}

func TestConcurrentAccess(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithPollInterval(MinPollInterval),
		WithEventBufferSize(64),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.ctx, m.cancel = ctx, cancel

	var wg sync.WaitGroup
	// Concurrently add, remove, check, and poll sessions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pid := id * 100
			checker.SetAlive(pid, true)
			m.TrackSession(testSession(pid, "session"))
			m.IsTracking(pid)
			m.TrackedCount()
			m.TrackedSessions()
			m.CheckNow()
			m.UntrackSession(pid)
		}(i)
	}

	wg.Wait()
}

func TestPollLoop_NoGoroutineLeak(t *testing.T) {
	checker := newMonitorMockPIDChecker()

	for i := 0; i < 5; i++ {
		m := NewMonitor(
			WithMonitorPIDChecker(checker),
			WithPollInterval(MinPollInterval),
		)
		checker.SetAlive(i+1000, true)
		m.TrackSession(testSession(i+1000, "session"))

		ctx, cancel := context.WithCancel(context.Background())
		m.Start(ctx)

		// Immediately stop
		cancel()
		m.Stop()
	}
	// If we get here without hanging, no goroutine leaks
}

func TestCheckNow_RaceConditionPIDDiesWhileChecking(t *testing.T) {
	// Simulate a PID that dies between the snapshot read and the check
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithEventBufferSize(4),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.ctx, m.cancel = ctx, cancel

	checker.SetAlive(1234, true)
	m.TrackSession(testSession(1234, "session"))

	// First CheckNow - alive
	m.CheckNow()
	select {
	case <-m.Events():
		t.Error("unexpected event when PID is alive")
	default:
	}

	// PID dies
	checker.SetAlive(1234, false)
	m.CheckNow()

	select {
	case event := <-m.Events():
		if event.Type != SessionClosed {
			t.Error("expected SessionClosed")
		}
	default:
		t.Error("expected closure event")
	}

	// Second CheckNow after removal - should not emit again
	m.CheckNow()
	select {
	case event := <-m.Events():
		t.Errorf("unexpected duplicate event: %v", event)
	default:
		// Good - no duplicate events
	}
}

func TestTrackSession_DuringPolling(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithPollInterval(MinPollInterval),
		WithEventBufferSize(8),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	// Add sessions while polling is active
	for i := 0; i < 5; i++ {
		pid := (i + 1) * 100
		checker.SetAlive(pid, true)
		m.TrackSession(testSession(pid, "session"))
	}

	time.Sleep(100 * time.Millisecond)

	if m.TrackedCount() != 5 {
		t.Errorf("expected 5 tracked sessions, got %d", m.TrackedCount())
	}
}

func TestMonitor_EventChannelNotClosed(t *testing.T) {
	m := NewMonitor(WithPollInterval(MinPollInterval))

	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	cancel()
	m.Stop()

	// Events channel should still be readable (not closed)
	select {
	case _, ok := <-m.Events():
		if !ok {
			t.Error("events channel should not be closed by monitor")
		}
	default:
		// Good - no events but channel is open
	}
}

func TestCheckNow_PIDCheckerCallCount(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithEventBufferSize(4),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.ctx, m.cancel = ctx, cancel

	// Track 3 sessions
	for i := 1; i <= 3; i++ {
		checker.SetAlive(i, true)
		m.TrackSession(testSession(i, "s"))
	}

	m.CheckNow()

	if checker.CallCount() != 3 {
		t.Errorf("expected 3 PID checks, got %d", checker.CallCount())
	}
}

// TestCheckAllPIDs_ContextCancelledDuringEmit covers the ctx.Done() branch in
// checkAllPIDs — specifically the case where the context is cancelled while
// the monitor is attempting to send closure events on a full channel.
func TestCheckAllPIDs_ContextCancelledDuringEmit(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	// Use a buffer of 1 so the second event send blocks and we can cancel.
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithEventBufferSize(1),
	)

	ctx, cancel := context.WithCancel(context.Background())
	m.ctx, m.cancel = ctx, cancel

	// Track two sessions — both dead so two events will be emitted.
	checker.SetAlive(10001, false)
	checker.SetAlive(10002, false)
	m.TrackSession(testSession(10001, "ctx-cancel-1"))
	m.TrackSession(testSession(10002, "ctx-cancel-2"))

	// Cancel the context immediately so the blocked send unblocks via ctx.Done().
	cancel()

	// checkAllPIDs should return without panicking even with cancelled context.
	// No assertion on how many events are emitted — the cancel path may emit
	// 0 or 1 events depending on scheduling.
	done := make(chan struct{})
	go func() {
		m.checkAllPIDs()
		close(done)
	}()

	select {
	case <-done:
		// Good — returned without blocking
	case <-time.After(2 * time.Second):
		t.Fatal("checkAllPIDs blocked longer than expected with cancelled context")
	}
}

// TestSessionCloseCallback_EndToEndLatency is the primary Sub-AC 2 validation:
// a session file is detected, the PID is killed, and a SessionClosed event is
// received within the ≤5 second end-to-end SLA.
func TestSessionCloseCallback_EndToEndLatency(t *testing.T) {
	checker := newMonitorMockPIDChecker()
	m := NewMonitor(
		WithMonitorPIDChecker(checker),
		WithPollInterval(MinPollInterval), // fastest allowed: 500ms
		WithEventBufferSize(4),
	)

	// Session starts alive.
	pid := 55001
	checker.SetAlive(pid, true)
	m.TrackSession(testSession(pid, "e2e-latency-session"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	// Allow at least one poll cycle to confirm session is alive.
	time.Sleep(600 * time.Millisecond)

	// PID exits — start the latency clock.
	checker.SetAlive(pid, false)
	start := time.Now()

	select {
	case event := <-m.Events():
		elapsed := time.Since(start)
		if event.Type != SessionClosed {
			t.Errorf("expected SessionClosed event, got type %d", event.Type)
		}
		if event.Session.Meta.PID != pid {
			t.Errorf("expected PID %d, got %d", pid, event.Session.Meta.PID)
		}
		if elapsed > 5*time.Second {
			t.Errorf("session-close callback latency %v exceeds ≤5s end-to-end SLA", elapsed)
		}
		t.Logf("Sub-AC 2 end-to-end close callback latency: %v", elapsed)
	case <-time.After(5 * time.Second):
		t.Fatal("session-close callback not received within 5s end-to-end SLA")
	}
}
