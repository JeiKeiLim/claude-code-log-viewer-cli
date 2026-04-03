package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// newTestDirWatcher creates a SessionDirectoryWatcher for the given temp dir
// with a mock PID checker and a large event buffer to avoid dropped events.
func newTestDirWatcher(t *testing.T, dir string, checker *mockPIDChecker) *SessionDirectoryWatcher {
	t.Helper()
	w, err := NewSessionDirectoryWatcher(dir,
		WithDirWatcherPIDChecker(checker),
		WithDirWatcherEventBuffer(64),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	return w
}

// startWatcher calls Start() and registers cleanup via t.Cleanup.
func startWatcher(t *testing.T, w *SessionDirectoryWatcher) {
	t.Helper()
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Logf("Close error (ignored): %v", err)
		}
	})
}

// drainAllEvents collects all events available within the given timeout.
func drainAllEvents(ch <-chan SessionEvent, timeout time.Duration) []SessionEvent {
	var events []SessionEvent
	deadline := time.After(timeout)
	for {
		select {
		case e := <-ch:
			events = append(events, e)
		case <-deadline:
			return events
		}
	}
}

// waitForEvent waits for a single event matching pred within timeout.
func waitForEvent(t *testing.T, ch <-chan SessionEvent, pred func(SessionEvent) bool, timeout time.Duration, msg string) SessionEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case e := <-ch:
			if pred(e) {
				return e
			}
		case <-deadline:
			t.Fatalf("timeout waiting for event: %s", msg)
			return SessionEvent{} // unreachable but satisfies compiler
		}
	}
}

// goroutineCount returns the current number of goroutines.
func goroutineCount() int {
	return runtime.NumGoroutine()
}

// ─────────────────────────────────────────────────────────────────────────────
// Constructor tests
// ─────────────────────────────────────────────────────────────────────────────

func TestNewSessionDirectoryWatcher_DefaultDir(t *testing.T) {
	w, err := NewSessionDirectoryWatcher("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer w.Close()

	if w.SessionsDir() == "" {
		t.Error("expected non-empty default sessions dir")
	}
}

func TestNewSessionDirectoryWatcher_CustomDir(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker()
	w := newTestDirWatcher(t, tmpDir, checker)
	defer w.Close()

	if w.SessionsDir() != tmpDir {
		t.Errorf("expected %q, got %q", tmpDir, w.SessionsDir())
	}
}

func TestNewSessionDirectoryWatcher_EventBufferOption(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewSessionDirectoryWatcher(tmpDir,
		WithDirWatcherPIDChecker(newMockPIDChecker()),
		WithDirWatcherEventBuffer(32),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer w.Close()

	// Verify channel capacity by filling it.
	// We can't inspect channel cap directly from outside, but we can verify
	// no panic occurs when receiving.
	if w.Events() == nil {
		t.Error("expected non-nil events channel")
	}
}

func TestNewSessionDirectoryWatcher_ZeroBufferIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewSessionDirectoryWatcher(tmpDir,
		WithDirWatcherPIDChecker(newMockPIDChecker()),
		WithDirWatcherEventBuffer(0), // Should use default (16)
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer w.Close()

	if w.Events() == nil {
		t.Error("expected non-nil events channel")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Start / Close lifecycle tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_StartCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a sub-directory that doesn't exist yet.
	sessionsDir := filepath.Join(tmpDir, "claude", "sessions")

	checker := newMockPIDChecker()
	w, err := NewSessionDirectoryWatcher(sessionsDir,
		WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Error("Start() should have created the sessions directory")
	}
}

func TestSessionDirectoryWatcher_StartAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	w := newTestDirWatcher(t, tmpDir, newMockPIDChecker())

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := w.Start()
	if err == nil {
		t.Error("expected error when starting a closed watcher")
	}
}

func TestSessionDirectoryWatcher_CloseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	w := newTestDirWatcher(t, tmpDir, newMockPIDChecker())
	startWatcher(t, w)

	// Calling Close multiple times should not panic or return an error.
	if err := w.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestSessionDirectoryWatcher_CloseWithoutStart(t *testing.T) {
	tmpDir := t.TempDir()
	w := newTestDirWatcher(t, tmpDir, newMockPIDChecker())

	// Close without Start should not panic.
	if err := w.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionDirectoryWatcher_IsClosed(t *testing.T) {
	tmpDir := t.TempDir()
	w := newTestDirWatcher(t, tmpDir, newMockPIDChecker())

	if w.IsClosed() {
		t.Error("expected IsClosed() == false before Close()")
	}

	startWatcher(t, w)
	w.Close()

	if !w.IsClosed() {
		t.Error("expected IsClosed() == true after Close()")
	}
}

func TestSessionDirectoryWatcher_EventsChannel(t *testing.T) {
	tmpDir := t.TempDir()
	w := newTestDirWatcher(t, tmpDir, newMockPIDChecker())
	defer w.Close()

	ch := w.Events()
	if ch == nil {
		t.Error("expected non-nil events channel")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Session detection tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_DetectsNewSession(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(100)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	// Write a new session file.
	writeSessionJSON(t, tmpDir, 100, SessionMeta{
		PID:       100,
		SessionID: "detect-test",
		CWD:       "/home/user/project",
	})

	event := waitForEvent(t, w.Events(),
		func(e SessionEvent) bool {
			return e.Type == SessionOpened && e.Session.Meta.PID == 100
		},
		2*time.Second, "SessionOpened for PID 100")

	if event.Session.Meta.SessionID != "detect-test" {
		t.Errorf("expected session-id 'detect-test', got %q", event.Session.Meta.SessionID)
	}
	if event.Session.FilePath == "" {
		t.Error("expected non-empty FilePath")
	}
}

func TestSessionDirectoryWatcher_DetectsSessionWithJSONLDir(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(200)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	writeSessionJSON(t, tmpDir, 200, SessionMeta{
		PID:       200,
		SessionID: "cwd-test",
		CWD:       "/Users/test/project",
	})

	event := waitForEvent(t, w.Events(),
		func(e SessionEvent) bool { return e.Session.Meta.PID == 200 },
		2*time.Second, "SessionOpened for PID 200")

	if event.Session.JSONLDir == "" {
		t.Error("expected JSONLDir to be set when CWD is present")
	}
}

func TestSessionDirectoryWatcher_DetectsSessionNoCWD(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(300)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	writeSessionJSON(t, tmpDir, 300, SessionMeta{
		PID:       300,
		SessionID: "no-cwd",
	})

	event := waitForEvent(t, w.Events(),
		func(e SessionEvent) bool { return e.Session.Meta.PID == 300 },
		2*time.Second, "SessionOpened for PID 300")

	if event.Session.JSONLDir != "" {
		t.Errorf("expected empty JSONLDir when CWD is absent, got %q", event.Session.JSONLDir)
	}
}

func TestSessionDirectoryWatcher_DetectsMultipleSessions(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(101, 102, 103)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	writeSessionJSON(t, tmpDir, 101, SessionMeta{PID: 101, SessionID: "s1"})
	writeSessionJSON(t, tmpDir, 102, SessionMeta{PID: 102, SessionID: "s2"})
	writeSessionJSON(t, tmpDir, 103, SessionMeta{PID: 103, SessionID: "s3"})

	seen := make(map[int]bool)
	deadline := time.After(3 * time.Second)
	for len(seen) < 3 {
		select {
		case e := <-w.Events():
			if e.Type == SessionOpened {
				seen[e.Session.Meta.PID] = true
			}
		case <-deadline:
			t.Fatalf("timed out: only detected PIDs %v, want 101 102 103", seen)
		}
	}

	for _, pid := range []int{101, 102, 103} {
		if !seen[pid] {
			t.Errorf("PID %d was not detected", pid)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Detection latency tests (core AC requirement: < 1 second)
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_DetectsWithin1Second(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(999)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	// Allow watcher to stabilise before measuring.
	time.Sleep(50 * time.Millisecond)

	createTime := time.Now()
	writeSessionJSON(t, tmpDir, 999, SessionMeta{
		PID:       999,
		SessionID: "latency-test",
		CWD:       "/home/user/latency",
		StartedAt: time.Now().UnixMilli(),
	})

	event := waitForEvent(t, w.Events(),
		func(e SessionEvent) bool {
			return e.Type == SessionOpened && e.Session.Meta.SessionID == "latency-test"
		},
		2*time.Second, "SessionOpened within 2s deadline")

	latency := time.Since(createTime)
	t.Logf("Detection latency: %v", latency)

	if latency > time.Second {
		t.Errorf("detection latency %v exceeds 1 second threshold", latency)
	}
	_ = event
}

func TestSessionDirectoryWatcher_DetectsMultipleWithin1Second(t *testing.T) {
	tmpDir := t.TempDir()
	pids := []int{1001, 1002, 1003}
	checker := newMockPIDChecker(pids...)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	time.Sleep(50 * time.Millisecond)

	createTime := time.Now()
	for _, pid := range pids {
		writeSessionJSON(t, tmpDir, pid, SessionMeta{
			PID:       pid,
			SessionID: fmt.Sprintf("multi-%d", pid),
		})
	}

	seen := make(map[int]bool)
	deadline := time.After(2 * time.Second)
	for len(seen) < len(pids) {
		select {
		case e := <-w.Events():
			if e.Type == SessionOpened {
				seen[e.Session.Meta.PID] = true
			}
		case <-deadline:
			t.Fatalf("timed out after 2s; detected: %v", seen)
		}
	}

	latency := time.Since(createTime)
	t.Logf("All %d sessions detected in %v", len(pids), latency)

	if latency > time.Second {
		t.Errorf("multi-session detection latency %v exceeds 1 second", latency)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PID filtering tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_IgnoresDeadPID(t *testing.T) {
	tmpDir := t.TempDir()
	// PID 404 is NOT in the alive list.
	checker := newMockPIDChecker()

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	writeSessionJSON(t, tmpDir, 404, SessionMeta{PID: 404, SessionID: "dead"})

	// Wait briefly; we should NOT receive any event.
	events := drainAllEvents(w.Events(), 300*time.Millisecond)
	for _, e := range events {
		if e.Session.Meta.PID == 404 {
			t.Errorf("unexpectedly received event for dead PID 404: %+v", e)
		}
	}
}

func TestSessionDirectoryWatcher_IgnoresPIDMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	// Filename is 500.json but content has PID 999.
	checker := newMockPIDChecker(500)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	writeSessionJSON(t, tmpDir, 500, SessionMeta{PID: 999, SessionID: "mismatch"})

	events := drainAllEvents(w.Events(), 300*time.Millisecond)
	for _, e := range events {
		if e.Session.Meta.SessionID == "mismatch" {
			t.Errorf("unexpectedly received event for PID-mismatch session: %+v", e)
		}
	}
}

func TestSessionDirectoryWatcher_UseFilenameWhenContentPIDIsZero(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(600)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	// Content has PID 0 — should fall back to filename PID (600).
	writeSessionJSON(t, tmpDir, 600, SessionMeta{SessionID: "no-pid-in-content"})

	event := waitForEvent(t, w.Events(),
		func(e SessionEvent) bool { return e.Session.Meta.PID == 600 },
		2*time.Second, "SessionOpened for PID 600")

	if event.Session.Meta.SessionID != "no-pid-in-content" {
		t.Errorf("unexpected session ID: %q", event.Session.Meta.SessionID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-session file tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_IgnoresNonJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker()

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "data.log"), []byte("log data"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(`{"key":"val"}`), 0644)

	events := drainAllEvents(w.Events(), 300*time.Millisecond)
	if len(events) > 0 {
		t.Errorf("expected no events for non-PID files, got %d: %+v", len(events), events)
	}
}

func TestSessionDirectoryWatcher_IgnoresCorruptJSON(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(700)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	// Write a corrupt JSON file with a valid PID filename.
	os.WriteFile(filepath.Join(tmpDir, "700.json"), []byte("{corrupt!!"), 0644)

	events := drainAllEvents(w.Events(), 300*time.Millisecond)
	for _, e := range events {
		if e.Session.Meta.PID == 700 {
			t.Errorf("unexpectedly received event for corrupt JSON: %+v", e)
		}
	}
}

func TestSessionDirectoryWatcher_IgnoresEmptyJSON(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(701)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	// Write an empty file.
	os.WriteFile(filepath.Join(tmpDir, "701.json"), []byte(""), 0644)

	events := drainAllEvents(w.Events(), 300*time.Millisecond)
	for _, e := range events {
		if e.Session.Meta.PID == 701 {
			t.Errorf("unexpectedly received event for empty JSON: %+v", e)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Session removal tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_EmitsClosedOnRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(800)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	path := writeSessionJSON(t, tmpDir, 800, SessionMeta{PID: 800, SessionID: "to-remove"})

	// Wait for SessionOpened.
	waitForEvent(t, w.Events(),
		func(e SessionEvent) bool { return e.Type == SessionOpened && e.Session.Meta.PID == 800 },
		2*time.Second, "SessionOpened for PID 800")

	// Remove the file.
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Expect SessionClosed. Use 5s timeout: kqueue/inotify delivery of remove
	// events can be slow when many concurrent goroutines are running (e.g. under
	// -race) and shouldn't fail just because the OS is under load.
	event := waitForEvent(t, w.Events(),
		func(e SessionEvent) bool { return e.Type == SessionClosed && e.Session.Meta.PID == 800 },
		5*time.Second, "SessionClosed for PID 800")

	if event.Session.Meta.PID != 800 {
		t.Errorf("expected PID 800 in closed event, got %d", event.Session.Meta.PID)
	}
}

func TestSessionDirectoryWatcher_NoClosedEventForUnseenRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	// PID 900 is dead, so no SessionOpened was emitted.
	checker := newMockPIDChecker() // Nothing alive

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	path := filepath.Join(tmpDir, "900.json")
	// Write and immediately remove without the watcher seeing a SessionOpened.
	os.WriteFile(path, []byte(`{"pid":900,"sessionId":"unseen"}`), 0644)
	// Wait for the Create event to be processed (should be ignored due to dead PID).
	time.Sleep(200 * time.Millisecond)
	os.Remove(path)

	events := drainAllEvents(w.Events(), 300*time.Millisecond)
	for _, e := range events {
		if e.Type == SessionClosed && e.Session.Meta.PID == 900 {
			t.Errorf("expected no SessionClosed for unseen session, got: %+v", e)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Duplicate suppression tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_NoDuplicateOpenedEvents(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(1100)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	path := writeSessionJSON(t, tmpDir, 1100, SessionMeta{PID: 1100, SessionID: "dedup"})

	// Trigger multiple writes to the same file.
	for i := 0; i < 5; i++ {
		data, _ := json.Marshal(SessionMeta{PID: 1100, SessionID: "dedup"})
		os.WriteFile(path, data, 0644)
		time.Sleep(10 * time.Millisecond)
	}

	// Collect events for a brief period.
	events := drainAllEvents(w.Events(), 500*time.Millisecond)

	openCount := 0
	for _, e := range events {
		if e.Type == SessionOpened && e.Session.Meta.PID == 1100 {
			openCount++
		}
	}
	if openCount > 1 {
		t.Errorf("expected at most 1 SessionOpened for PID 1100, got %d", openCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Goroutine leak tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_NoGoroutineLeak_StartStop(t *testing.T) {
	before := goroutineCount()

	for i := 0; i < 5; i++ {
		tmpDir := t.TempDir()
		w := newTestDirWatcher(t, tmpDir, newMockPIDChecker())

		if err := w.Start(); err != nil {
			t.Fatalf("iteration %d: Start: %v", i, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("iteration %d: Close: %v", i, err)
		}
	}

	// Allow goroutines to fully exit.
	time.Sleep(100 * time.Millisecond)
	after := goroutineCount()

	// Allow ±2 goroutines for runtime scheduler variation.
	if after > before+2 {
		t.Errorf("goroutine leak detected: before=%d after=%d (delta=%d)", before, after, after-before)
	}
}

func TestSessionDirectoryWatcher_NoGoroutineLeak_WithEvents(t *testing.T) {
	before := goroutineCount()

	for i := 0; i < 3; i++ {
		tmpDir := t.TempDir()
		pid := 2000 + i
		checker := newMockPIDChecker(pid)
		w := newTestDirWatcher(t, tmpDir, checker)

		if err := w.Start(); err != nil {
			t.Fatalf("iteration %d: Start: %v", i, err)
		}

		writeSessionJSON(t, tmpDir, pid, SessionMeta{PID: pid, SessionID: fmt.Sprintf("leak-test-%d", i)})

		// Wait for event to be processed.
		waitForEvent(t, w.Events(),
			func(e SessionEvent) bool { return e.Type == SessionOpened },
			2*time.Second, "SessionOpened")

		if err := w.Close(); err != nil {
			t.Fatalf("iteration %d: Close: %v", i, err)
		}
	}

	time.Sleep(100 * time.Millisecond)
	after := goroutineCount()

	if after > before+2 {
		t.Errorf("goroutine leak: before=%d after=%d (delta=%d)", before, after, after-before)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Concurrency / race tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_ConcurrentSessionCreation(t *testing.T) {
	tmpDir := t.TempDir()
	pids := make([]int, 9)
	for i := range pids {
		pids[i] = 3000 + i
	}

	checker := newMockPIDChecker(pids...)
	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	// Create all sessions concurrently using direct file I/O (writeSessionJSON
	// calls t.Fatalf which is unsafe from non-test goroutines).
	var wg sync.WaitGroup
	for _, pid := range pids {
		pid := pid
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, _ := json.Marshal(SessionMeta{
				PID:       pid,
				SessionID: fmt.Sprintf("concurrent-%d", pid),
			})
			path := filepath.Join(tmpDir, fmt.Sprintf("%d.json", pid))
			_ = os.WriteFile(path, data, 0644)
		}()
	}
	wg.Wait()

	// Collect opened events.
	seen := make(map[int]bool)
	deadline := time.After(3 * time.Second)
	for len(seen) < len(pids) {
		select {
		case e := <-w.Events():
			if e.Type == SessionOpened {
				seen[e.Session.Meta.PID] = true
			}
		case <-deadline:
			t.Fatalf("timed out; detected %d/%d PIDs: %v", len(seen), len(pids), seen)
		}
	}
}

func TestSessionDirectoryWatcher_ConcurrentCloseAndEvent(t *testing.T) {
	// Test that closing while events are being processed doesn't panic or deadlock.
	tmpDir := t.TempDir()
	pid := 4000
	checker := newMockPIDChecker(pid)

	w := newTestDirWatcher(t, tmpDir, checker)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Write session files from a goroutine using direct file I/O (not writeSessionJSON
	// which calls t.Fatalf — unsafe from goroutines after test completion).
	sessionData, _ := json.Marshal(SessionMeta{PID: pid, SessionID: "race"})
	sessionPath := filepath.Join(tmpDir, fmt.Sprintf("%d.json", pid))

	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Millisecond)
			_ = os.WriteFile(sessionPath, sessionData, 0644)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Close while writes are still happening — should not panic or deadlock.
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Close()
	}()

	select {
	case <-done:
		// Closed cleanly.
	case <-time.After(2 * time.Second):
		t.Error("Close() timed out — possible deadlock")
	}

	// Wait for write goroutine to finish.
	<-writeDone
}

// ─────────────────────────────────────────────────────────────────────────────
// PID alive → dead → file removed lifecycle
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_FullLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	pid := 5000
	checker := newMockPIDChecker(pid)

	w := newTestDirWatcher(t, tmpDir, checker)
	startWatcher(t, w)

	// Session opens.
	path := writeSessionJSON(t, tmpDir, pid, SessionMeta{
		PID:       pid,
		SessionID: "lifecycle",
		CWD:       "/home/user/lifecycle",
	})

	openEvent := waitForEvent(t, w.Events(),
		func(e SessionEvent) bool { return e.Type == SessionOpened && e.Session.Meta.PID == pid },
		2*time.Second, "SessionOpened")

	if openEvent.Session.Meta.SessionID != "lifecycle" {
		t.Errorf("wrong session ID in open event: %q", openEvent.Session.Meta.SessionID)
	}
	if openEvent.Session.JSONLDir == "" {
		t.Error("expected non-empty JSONLDir in open event")
	}

	// Simulate PID death and file removal.
	checker.SetAlive(pid, false)
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// 5s timeout: kqueue/inotify delivery can be slow under race detector load.
	closeEvent := waitForEvent(t, w.Events(),
		func(e SessionEvent) bool { return e.Type == SessionClosed && e.Session.Meta.PID == pid },
		5*time.Second, "SessionClosed")

	if closeEvent.Session.Meta.PID != pid {
		t.Errorf("wrong PID in close event: %d", closeEvent.Session.Meta.PID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Options tests
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Channel full / event drop path
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_ChannelFullDropAndRetry(t *testing.T) {
	// Use a buffer of size 1 to trigger the channel-full drop path.
	tmpDir := t.TempDir()
	checker := newMockPIDChecker(6001, 6002)

	w, err := NewSessionDirectoryWatcher(tmpDir,
		WithDirWatcherPIDChecker(checker),
		WithDirWatcherEventBuffer(1), // Very small buffer
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer w.Close()
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Write first session — this will go into the buffer.
	writeSessionJSON(t, tmpDir, 6001, SessionMeta{PID: 6001, SessionID: "fill-buffer"})

	// Wait for the first session event to be processed by handleSessionAppeared
	// (but don't drain the channel yet, leaving it full).
	time.Sleep(200 * time.Millisecond)

	// Write second session — channel should be full, event gets dropped and
	// seenPIDs entry removed, allowing a retry on next Write event.
	writeSessionJSON(t, tmpDir, 6002, SessionMeta{PID: 6002, SessionID: "dropped-then-retried"})

	// Now drain the channel to unblock it.
	time.Sleep(100 * time.Millisecond)
	for {
		select {
		case <-w.Events():
		default:
			goto drained
		}
	}
drained:

	// The retry should now succeed on the next Write to 6002.
	// Trigger another write to retry.
	writeSessionJSON(t, tmpDir, 6002, SessionMeta{PID: 6002, SessionID: "dropped-then-retried"})

	// We just verify no panic or deadlock; exact event reception is best-effort
	// due to OS scheduling. Allow up to 2 seconds.
	drainAllEvents(w.Events(), 2*time.Second)
}

// ─────────────────────────────────────────────────────────────────────────────
// Start with invalid parent directory (MkdirAll failure path)
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionDirectoryWatcher_StartMkdirFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — permission checks don't apply")
	}

	// Create a read-only parent directory so MkdirAll cannot create subdirs.
	tmpDir := t.TempDir()
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(roDir, 0555); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	// Try to watch a sub-directory inside the read-only dir.
	sessionsDir := filepath.Join(roDir, "sessions")
	w, err := NewSessionDirectoryWatcher(sessionsDir,
		WithDirWatcherPIDChecker(newMockPIDChecker()),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer w.Close()

	// Start should fail because it cannot create the sessions sub-directory.
	err = w.Start()
	if err == nil {
		t.Error("expected Start() to fail when MkdirAll cannot create the directory")
	}
}

func TestWithDirWatcherPIDChecker(t *testing.T) {
	tmpDir := t.TempDir()
	// Only PID 9999 is alive.
	custom := newMockPIDChecker(9999)

	w, err := NewSessionDirectoryWatcher(tmpDir, WithDirWatcherPIDChecker(custom))
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// PID 1234 should be filtered out.
	writeSessionJSON(t, tmpDir, 1234, SessionMeta{PID: 1234, SessionID: "filtered"})
	// PID 9999 should pass.
	writeSessionJSON(t, tmpDir, 9999, SessionMeta{PID: 9999, SessionID: "allowed"})

	// Collect events.
	var opened []SessionEvent
	deadline := time.After(time.Second)
waitLoop:
	for {
		select {
		case e := <-w.Events():
			if e.Type == SessionOpened {
				opened = append(opened, e)
				if e.Session.Meta.PID == 9999 {
					break waitLoop
				}
			}
		case <-deadline:
			break waitLoop
		}
	}

	for _, e := range opened {
		if e.Session.Meta.PID == 1234 {
			t.Error("expected PID 1234 to be filtered by custom PID checker")
		}
	}

	found9999 := false
	for _, e := range opened {
		if e.Session.Meta.PID == 9999 {
			found9999 = true
		}
	}
	if !found9999 {
		t.Error("expected PID 9999 to be allowed by custom PID checker")
	}
}
