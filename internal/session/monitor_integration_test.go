package session

// Integration tests for Sub-AC 3: spawn a real child process, register it as a
// tracked session, terminate it, and assert the session-closed event fires within
// 5 seconds using the real SyscallPIDChecker (no mock process management).
//
// These tests exercise the full lifecycle path that production code follows:
//
//   real OS process → SyscallPIDChecker.IsAlive(pid) → Monitor.pollLoop()
//   → SessionClosed event
//
// Test matrix:
//   - Single process: basic lifecycle (spawn → track → kill → close event)
//   - Multiple sessions: only the killed process fires a close event
//   - Immediate detection: CheckNow() detects closure synchronously after reap
//   - No goroutine leaks: repeated monitor start/stop cycles leave no residual goroutines

import (
	"context"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"
)

// sleepCmd returns a platform-appropriate long-running command (30 seconds).
// The process is used as a stand-in for a Claude Code session process.
func sleepCmd() *exec.Cmd {
	if runtime.GOOS == "windows" {
		// "timeout /t 30" on Windows (no sleep built-in)
		return exec.Command("cmd", "/c", "timeout", "/t", "30")
	}
	return exec.Command("sleep", "30")
}

// spawnRealProcess starts a long-running child process for integration testing.
// It returns the exec.Cmd and a terminate function that kills and reaps the
// process exactly once (safe to call multiple times via sync.Once).
// A t.Cleanup handler calls terminate automatically, so tests need not clean up
// explicitly — but they may call terminate early to control timing.
func spawnRealProcess(t *testing.T) (*exec.Cmd, func()) {
	t.Helper()
	cmd := sleepCmd()
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawnRealProcess: failed to start child process: %v", err)
	}
	var once sync.Once
	terminate := func() {
		once.Do(func() {
			// Kill sends SIGKILL; Wait reaps the zombie so Kill(pid,0) → ESRCH.
			// Errors are intentionally ignored here: the process may have already
			// exited or been reaped by the time cleanup runs.
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
	}
	t.Cleanup(terminate)
	return cmd, terminate
}

// makeIntegrationSession builds an ActiveSession for the given real process.
func makeIntegrationSession(pid int, sessionID string) ActiveSession {
	return ActiveSession{
		Meta: SessionMeta{
			PID:       pid,
			SessionID: sessionID,
			CWD:       "/integration/test/project",
			StartedAt: time.Now().UnixMilli(),
			Kind:      "interactive",
		},
		FilePath: "/home/.claude/sessions/" + sessionID + ".json",
		JSONLDir: "/home/.claude/projects/-integration-test-project",
	}
}

// waitForCloseEvent reads from events until a SessionClosed event for the given
// PID arrives or the deadline expires. Returns the event and true on success.
func waitForCloseEvent(events <-chan SessionEvent, pid int, timeout time.Duration) (SessionEvent, bool) {
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-events:
			if ev.Type == SessionClosed && ev.Session.Meta.PID == pid {
				return ev, true
			}
			// Drain unrelated events (e.g., if multiple sessions are tracked).
		case <-deadline:
			return SessionEvent{}, false
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Primary integration test (Sub-AC 3 core requirement)
// ─────────────────────────────────────────────────────────────────────────────

// TestMonitorIntegration_RealProcess_ClosedEventWithin5Seconds is the canonical
// Sub-AC 3 integration test.  It spawns a real OS process, registers its PID
// with the Monitor (using the production SyscallPIDChecker), terminates the
// process, and asserts that a SessionClosed event is received within 5 seconds.
func TestMonitorIntegration_RealProcess_ClosedEventWithin5Seconds(t *testing.T) {
	cmd, terminate := spawnRealProcess(t)
	pid := cmd.Process.Pid

	// Build the monitor with the production-default SyscallPIDChecker.
	// Use MinPollInterval (500ms) so the test completes quickly.
	m := NewMonitor(
		WithPollInterval(MinPollInterval),
		WithEventBufferSize(8),
		// Intentionally omit WithMonitorPIDChecker to exercise real syscall path.
	)

	// Verify the spawned process is alive before we begin tracking.
	realChecker := NewPIDChecker()
	if !realChecker.IsAlive(pid) {
		t.Fatalf("child process PID %d should be alive immediately after spawn", pid)
	}

	// Register the real process as a tracked session.
	m.TrackSession(makeIntegrationSession(pid, "sub-ac3-primary"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	// Allow at least one poll cycle to confirm the session is observed as alive.
	time.Sleep(600 * time.Millisecond)

	if !m.IsTracking(pid) {
		t.Fatalf("PID %d should still be tracked after first poll cycle", pid)
	}

	// Terminate the process and reap the zombie so syscall.Kill(pid,0) → ESRCH.
	terminate()

	// Confirm the process is dead from the OS perspective.
	if realChecker.IsAlive(pid) {
		t.Fatalf("process PID %d still alive after terminate+reap; test precondition violated", pid)
	}

	// Assert the SessionClosed event arrives within the 5-second SLA.
	start := time.Now()
	ev, ok := waitForCloseEvent(m.Events(), pid, 5*time.Second)
	if !ok {
		t.Fatalf("Sub-AC 3: session-closed event not received within 5s for real process PID %d", pid)
	}

	elapsed := time.Since(start)
	if ev.Type != SessionClosed {
		t.Errorf("expected SessionClosed event type, got %d", ev.Type)
	}
	if ev.Session.Meta.PID != pid {
		t.Errorf("expected PID %d in close event, got %d", pid, ev.Session.Meta.PID)
	}
	if elapsed > 5*time.Second {
		t.Errorf("session-closed callback latency %v exceeds ≤5s end-to-end SLA", elapsed)
	}
	t.Logf("Sub-AC 3 primary: real process PID %d → closed event in %v", pid, elapsed)
}

// ─────────────────────────────────────────────────────────────────────────────
// Multiple sessions: only the terminated process fires
// ─────────────────────────────────────────────────────────────────────────────

// TestMonitorIntegration_RealProcesses_OnlyKilledSessionFires verifies that when
// multiple real processes are tracked simultaneously, terminating one does not
// cause false closed events for the surviving sessions.
func TestMonitorIntegration_RealProcesses_OnlyKilledSessionFires(t *testing.T) {
	cmd1, _ := spawnRealProcess(t)          // stays alive throughout
	cmd2, terminate2 := spawnRealProcess(t) // will be killed
	pid1, pid2 := cmd1.Process.Pid, cmd2.Process.Pid

	m := NewMonitor(
		WithPollInterval(MinPollInterval),
		WithEventBufferSize(8),
	)
	m.TrackSession(makeIntegrationSession(pid1, "keep-alive-session"))
	m.TrackSession(makeIntegrationSession(pid2, "target-session"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	// Wait for at least one full poll cycle.
	time.Sleep(600 * time.Millisecond)

	if !m.IsTracking(pid1) || !m.IsTracking(pid2) {
		t.Fatalf("both PIDs should be tracked; pid1=%v pid2=%v",
			m.IsTracking(pid1), m.IsTracking(pid2))
	}

	// Terminate only the second process.
	terminate2()

	// The closed event should name pid2, not pid1.
	ev, ok := waitForCloseEvent(m.Events(), pid2, 5*time.Second)
	if !ok {
		t.Fatalf("session-closed event for PID %d not received within 5s", pid2)
	}
	if ev.Session.Meta.PID != pid2 {
		t.Errorf("expected close event for PID %d, got PID %d", pid2, ev.Session.Meta.PID)
	}
	t.Logf("correctly received close event for killed PID %d", pid2)

	// pid1 must remain tracked.
	if !m.IsTracking(pid1) {
		t.Errorf("PID %d (alive) should still be tracked after killing PID %d", pid1, pid2)
	}
	// pid2 must be removed.
	if m.IsTracking(pid2) {
		t.Errorf("PID %d (killed) should be removed from tracking", pid2)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Immediate detection via CheckNow (no polling latency)
// ─────────────────────────────────────────────────────────────────────────────

// TestMonitorIntegration_RealProcess_ImmediateDetectionViaCheckNow confirms that
// calling CheckNow() after a real process has been killed and reaped immediately
// emits a SessionClosed event without waiting for the next poll cycle.
// This exercises the synchronous detection path that is useful for on-demand
// checks (e.g., triggered by fsnotify file-deletion events).
func TestMonitorIntegration_RealProcess_ImmediateDetectionViaCheckNow(t *testing.T) {
	cmd, terminate := spawnRealProcess(t)
	pid := cmd.Process.Pid

	// Use a very long poll interval so the ticker never fires during the test;
	// all detection must come through the explicit CheckNow call.
	m := NewMonitor(
		WithPollInterval(MaxPollInterval), // 10 seconds — won't fire in test
		WithEventBufferSize(4),
	)
	m.TrackSession(makeIntegrationSession(pid, "checkNow-integration"))

	// Set up context so checkAllPIDs can emit events.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Assign ctx/cancel directly (mirrors monitor_test.go pattern for CheckNow tests).
	m.ctx, m.cancel = ctx, cancel

	// Baseline: process is alive, CheckNow should not emit any event.
	m.CheckNow()
	select {
	case unexpectedEv := <-m.Events():
		t.Fatalf("unexpected event before process was killed: %v", unexpectedEv)
	default:
		// Good — no events while process is alive.
	}

	// Kill + reap the real process.
	terminate()

	// Verify the OS agrees the process is dead.
	realChecker := NewPIDChecker()
	if realChecker.IsAlive(pid) {
		t.Fatalf("process PID %d still alive after terminate; cannot test CheckNow detection", pid)
	}

	// CheckNow should synchronously detect the dead PID and enqueue an event.
	m.CheckNow()

	select {
	case ev := <-m.Events():
		if ev.Type != SessionClosed {
			t.Errorf("expected SessionClosed, got event type %d", ev.Type)
		}
		if ev.Session.Meta.PID != pid {
			t.Errorf("expected PID %d, got %d", pid, ev.Session.Meta.PID)
		}
		t.Logf("CheckNow immediate detection: PID %d closed event received synchronously", pid)
	default:
		t.Errorf("expected session-closed event from CheckNow after process was killed and reaped")
	}

	// The PID should be removed from tracking.
	if m.IsTracking(pid) {
		t.Errorf("PID %d should be untracked after CheckNow detected it as dead", pid)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Goroutine leak check across session open/close cycles
// ─────────────────────────────────────────────────────────────────────────────

// TestMonitorIntegration_RealProcess_NoGoroutineLeak verifies that repeated
// monitor start/stop cycles involving real processes do not leave residual
// goroutines.  This is the Sub-AC 3 goroutine-safety requirement.
func TestMonitorIntegration_RealProcess_NoGoroutineLeak(t *testing.T) {
	// Capture baseline goroutine count before any monitor is started.
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const cycles = 3
	for i := 0; i < cycles; i++ {
		cmd, terminate := spawnRealProcess(t)
		pid := cmd.Process.Pid

		m := NewMonitor(
			WithPollInterval(MinPollInterval),
			WithEventBufferSize(4),
		)
		m.TrackSession(makeIntegrationSession(pid, "leak-test-session"))

		ctx, cancel := context.WithCancel(context.Background())
		m.Start(ctx)

		// Kill + reap the real process.
		terminate()

		// Stop the monitor cleanly.
		cancel()
		m.Stop()
	}

	// Give goroutines a brief window to finalise.
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	after := runtime.NumGoroutine()
	// Allow a small absolute slack for the Go runtime and test framework.
	const goroutineSlack = 5
	if after > baseline+goroutineSlack {
		t.Errorf("possible goroutine leak after %d open/close cycles: "+
			"baseline=%d, after=%d (delta=%d, max allowed=%d)",
			cycles, baseline, after, after-baseline, goroutineSlack)
	}
	t.Logf("goroutine count: baseline=%d after=%d cycles=%d", baseline, after, cycles)
}

// ─────────────────────────────────────────────────────────────────────────────
// Session closed event carries correct metadata
// ─────────────────────────────────────────────────────────────────────────────

// TestMonitorIntegration_RealProcess_EventCarriesSessionMetadata asserts that
// the SessionClosed event contains the same metadata that was registered with
// TrackSession — not zeroed or partially populated fields.
func TestMonitorIntegration_RealProcess_EventCarriesSessionMetadata(t *testing.T) {
	cmd, terminate := spawnRealProcess(t)
	pid := cmd.Process.Pid

	want := makeIntegrationSession(pid, "metadata-check-session")

	m := NewMonitor(
		WithPollInterval(MinPollInterval),
		WithEventBufferSize(4),
	)
	m.TrackSession(want)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	time.Sleep(600 * time.Millisecond)

	terminate()

	ev, ok := waitForCloseEvent(m.Events(), pid, 5*time.Second)
	if !ok {
		t.Fatalf("no closed event for PID %d within 5s", pid)
	}

	got := ev.Session
	if got.Meta.PID != want.Meta.PID {
		t.Errorf("PID: want %d, got %d", want.Meta.PID, got.Meta.PID)
	}
	if got.Meta.SessionID != want.Meta.SessionID {
		t.Errorf("SessionID: want %q, got %q", want.Meta.SessionID, got.Meta.SessionID)
	}
	if got.Meta.CWD != want.Meta.CWD {
		t.Errorf("CWD: want %q, got %q", want.Meta.CWD, got.Meta.CWD)
	}
	if got.Meta.Kind != want.Meta.Kind {
		t.Errorf("Kind: want %q, got %q", want.Meta.Kind, got.Meta.Kind)
	}
	if got.FilePath != want.FilePath {
		t.Errorf("FilePath: want %q, got %q", want.FilePath, got.FilePath)
	}
	if got.JSONLDir != want.JSONLDir {
		t.Errorf("JSONLDir: want %q, got %q", want.JSONLDir, got.JSONLDir)
	}
	t.Logf("metadata verified for closed session PID %d (%s)", pid, got.Meta.SessionID)
}
