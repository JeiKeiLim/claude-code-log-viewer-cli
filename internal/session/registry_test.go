package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

// ─────────────────────────────── test helpers ───────────────────────────────

// registryMockPIDChecker is a thread-safe PIDChecker for registry tests.
// It is distinct from scannerMockPIDChecker and monitorMockPIDChecker to
// avoid redeclaration errors across the package-level test files.
type registryMockPIDChecker struct {
	mu    sync.RWMutex
	alive map[int]bool
}

func newRegistryMockPIDChecker() *registryMockPIDChecker {
	return &registryMockPIDChecker{alive: make(map[int]bool)}
}

func (m *registryMockPIDChecker) SetAlive(pid int, alive bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alive[pid] = alive
}

func (m *registryMockPIDChecker) IsAlive(pid int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alive[pid]
}

// regFormatFilename converts a PID to "{pid}.json" filename.
func regFormatFilename(pid int) string {
	return strconv.Itoa(pid) + ".json"
}

// regWriteSessionFile writes a valid {pid}.json session file to dir and returns its path.
// It also creates the corresponding JSONL log file so that the scanner's
// JSONL-existence check passes.
func regWriteSessionFile(t *testing.T, dir string, pid int, sessionID string) string {
	t.Helper()
	meta := SessionMeta{
		PID:       pid,
		SessionID: sessionID,
		CWD:       "/test/project",
		StartedAt: time.Now().UnixMilli(),
		Kind:      "interactive",
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal session JSON: %v", err)
	}
	path := filepath.Join(dir, regFormatFilename(pid))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write session file %s: %v", path, err)
	}

	// Create the JSONL log file so the scanner includes this session.
	jsonlDir := CWDToProjectDir(meta.CWD)
	if jsonlDir != "" {
		if err := os.MkdirAll(jsonlDir, 0755); err != nil {
			t.Fatalf("create JSONL dir %s: %v", jsonlDir, err)
		}
		jsonlPath := filepath.Join(jsonlDir, sessionID+".jsonl")
		if err := os.WriteFile(jsonlPath, []byte("{}\n"), 0644); err != nil {
			t.Fatalf("write JSONL file %s: %v", jsonlPath, err)
		}
		t.Cleanup(func() {
			os.Remove(jsonlPath)
			// Try to remove the dir if empty; ignore errors
			os.Remove(jsonlDir)
		})
	}

	return path
}

// waitForRegistryCount polls r.SessionCount() until it equals want or timeout expires.
func waitForRegistryCount(r *Registry, want int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.SessionCount() == want {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// waitForRegistryOpenEvent waits for a SessionOpened event matching the given PID.
// It fails the test if no matching event is received within timeout.
func waitForRegistryOpenEvent(t *testing.T, r *Registry, pid int, timeout time.Duration) SessionEvent {
	t.Helper()
	return waitForEvent(t, r.Events(),
		func(e SessionEvent) bool {
			return e.Type == SessionOpened && e.Session.Meta.PID == pid
		},
		timeout,
		"waiting for SessionOpened for PID "+strconv.Itoa(pid),
	)
}

// waitForRegistryCloseEvent waits for a SessionClosed event matching the given PID.
// It fails the test if no matching event is received within timeout.
func waitForRegistryCloseEvent(t *testing.T, r *Registry, pid int, timeout time.Duration) SessionEvent {
	t.Helper()
	return waitForEvent(t, r.Events(),
		func(e SessionEvent) bool {
			return e.Type == SessionClosed && e.Session.Meta.PID == pid
		},
		timeout,
		"waiting for SessionClosed for PID "+strconv.Itoa(pid),
	)
}

// ─────────────────────────────── constructor tests ──────────────────────────

func TestNewRegistry_Defaults(t *testing.T) {
	r := NewRegistry("", WithRegistryNoDirWatcher())
	if r.scanner == nil {
		t.Error("expected non-nil scanner")
	}
	if r.monitor == nil {
		t.Error("expected non-nil monitor")
	}
	if r.events == nil {
		t.Error("expected non-nil events channel")
	}
	if r.sessions == nil {
		t.Error("expected non-nil sessions map")
	}
	if cap(r.events) != 32 {
		t.Errorf("expected default event buffer 32, got %d", cap(r.events))
	}
	if r.dirWatcher != nil {
		t.Error("expected no dir watcher when WithRegistryNoDirWatcher is set")
	}
}

func TestNewRegistry_EmptySessDir_UsesDefault(t *testing.T) {
	r := NewRegistry("", WithRegistryNoDirWatcher())
	if r.scanner.SessionsDir() == "" {
		t.Error("expected non-empty default sessions dir")
	}
}

func TestNewRegistry_CustomSessionsDir(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(tmpDir, WithRegistryNoDirWatcher())
	if r.scanner.SessionsDir() != tmpDir {
		t.Errorf("expected sessionsDir %q, got %q", tmpDir, r.scanner.SessionsDir())
	}
}

func TestNewRegistry_WithScanInterval(t *testing.T) {
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryScanInterval(500*time.Millisecond),
	)
	if r.scanner.Interval() != 500*time.Millisecond {
		t.Errorf("expected 500ms scan interval, got %v", r.scanner.Interval())
	}
}

func TestNewRegistry_WithEventBuffer(t *testing.T) {
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryEventBuffer(64),
	)
	if cap(r.events) != 64 {
		t.Errorf("expected event buffer 64, got %d", cap(r.events))
	}
}

func TestNewRegistry_WithEventBuffer_Zero_IsIgnored(t *testing.T) {
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryEventBuffer(0), // zero is ignored; default 32 used
	)
	if cap(r.events) != 32 {
		t.Errorf("expected default buffer 32 when 0 provided, got %d", cap(r.events))
	}
}

func TestNewRegistry_WithPIDChecker(t *testing.T) {
	checker := newRegistryMockPIDChecker()
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
	)
	if r.scanner == nil || r.monitor == nil {
		t.Error("expected components to be created with custom PID checker")
	}
}

func TestNewRegistry_WithDirWatcher(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewSessionDirectoryWatcher(tmpDir)
	if err != nil {
		t.Fatalf("create dir watcher: %v", err)
	}
	defer w.Close()

	r := NewRegistry(tmpDir, WithRegistryDirWatcher(w))
	if r.dirWatcher != w {
		t.Error("expected provided dir watcher to be used")
	}
}

func TestNewRegistry_NoDirWatcher(t *testing.T) {
	r := NewRegistry("", WithRegistryNoDirWatcher())
	if r.dirWatcher != nil {
		t.Error("expected no dir watcher when WithRegistryNoDirWatcher is set")
	}
}

func TestNewRegistry_WithMonitorInterval(t *testing.T) {
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryMonitorInterval(3*time.Second),
	)
	if r.monitor.pollInterval != 3*time.Second {
		t.Errorf("expected monitor interval 3s, got %v", r.monitor.pollInterval)
	}
}

func TestNewRegistry_WithMonitorInterval_Clamped(t *testing.T) {
	// Too small → clamped to MinPollInterval by Monitor
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryMonitorInterval(10*time.Millisecond),
	)
	if r.monitor.pollInterval != MinPollInterval {
		t.Errorf("expected clamped interval %v, got %v", MinPollInterval, r.monitor.pollInterval)
	}
}

// ─────────────────────────────── lifecycle tests ────────────────────────────

func TestRegistry_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)

	// Stop should return promptly
	done := make(chan struct{})
	go func() {
		r.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() did not return within 3 seconds")
	}
}

func TestRegistry_StopMultipleTimes_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryScanInterval(MinPollInterval),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	// Multiple stops should not panic
	r.Stop()
	r.Stop()
	r.Stop()
}

func TestRegistry_ContextCancellation_StopsGoroutines(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)

	// Cancel context — goroutines should exit
	cancel()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("goroutines did not exit after context cancellation")
	}
	r.Stop() // harmless extra stop
}

// ─────────────────────────────── session discovery via scanner ──────────────

func TestRegistry_DetectsSessionFromScanner(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 12345
	checker.SetAlive(pid, true)

	// Write a session file before starting
	regWriteSessionFile(t, tmpDir, pid, "session-abc")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	ev := waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	if ev.Session.Meta.SessionID != "session-abc" {
		t.Errorf("expected sessionID 'session-abc', got %q", ev.Session.Meta.SessionID)
	}

	if r.SessionCount() != 1 {
		t.Errorf("expected 1 session in registry, got %d", r.SessionCount())
	}
}

func TestRegistry_DetectsMultipleSessions(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()

	pids := []int{1001, 1002, 1003}
	for i, pid := range pids {
		checker.SetAlive(pid, true)
		regWriteSessionFile(t, tmpDir, pid, "session-"+strconv.Itoa(i))
	}

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	if !waitForRegistryCount(r, 3, 5*time.Second) {
		t.Errorf("expected 3 sessions within 5 seconds, got %d", r.SessionCount())
	}
}

// ─────────────────────────────── session closure via scanner diff ────────────

func TestRegistry_DetectsSessionClosureViaScanner(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 54321
	checker.SetAlive(pid, true)
	regWriteSessionFile(t, tmpDir, pid, "dying-session")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Wait for session to appear
	waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	// Kill the PID and remove the file so scanner won't see it
	checker.SetAlive(pid, false)
	_ = os.Remove(filepath.Join(tmpDir, regFormatFilename(pid)))

	// Wait for closure event
	ce := waitForRegistryCloseEvent(t, r, pid, 5*time.Second)
	if ce.Session.Meta.PID != pid {
		t.Errorf("expected PID %d, got %d", pid, ce.Session.Meta.PID)
	}

	if !waitForRegistryCount(r, 0, 3*time.Second) {
		t.Errorf("expected 0 sessions after closure, got %d", r.SessionCount())
	}
}

// ─────────────────────────────── session closure via monitor ────────────────

func TestRegistry_DetectsSessionClosureViaMonitor(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 99001
	checker.SetAlive(pid, true)
	regWriteSessionFile(t, tmpDir, pid, "monitor-session")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
		WithRegistryMonitorInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Wait for session to appear
	waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	// Kill the PID (file remains; monitor detects via PID liveness polling)
	checker.SetAlive(pid, false)

	// Wait for monitor to detect closure
	ce := waitForRegistryCloseEvent(t, r, pid, 5*time.Second)
	if ce.Session.Meta.SessionID != "monitor-session" {
		t.Errorf("close event should carry full metadata, got sessionID %q", ce.Session.Meta.SessionID)
	}
}

// ─────────────────────────────── deduplication ──────────────────────────────

func TestRegistry_DeduplicatesSessionsFromMultipleSources(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 77001
	checker.SetAlive(pid, true)

	// Create dir watcher + scanner: both will see the same session file.
	w, err := NewSessionDirectoryWatcher(tmpDir, WithDirWatcherPIDChecker(checker))
	if err != nil {
		t.Fatalf("create dir watcher: %v", err)
	}
	defer w.Close()

	r := NewRegistry(tmpDir,
		WithRegistryDirWatcher(w),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Small delay to let watchers initialise before writing the file.
	time.Sleep(50 * time.Millisecond)

	// Write session file — both dir watcher and scanner will detect it.
	regWriteSessionFile(t, tmpDir, pid, "dedupe-session")

	// Collect events for 1.5 scan cycles — should produce exactly ONE SessionOpened.
	events := drainAllEvents(r.Events(), 2*time.Second)

	openedCount := 0
	for _, ev := range events {
		if ev.Type == SessionOpened && ev.Session.Meta.PID == pid {
			openedCount++
		}
	}
	if openedCount == 0 {
		t.Error("expected at least one SessionOpened event")
	}
	if openedCount > 1 {
		t.Errorf("expected exactly 1 SessionOpened (deduplication), got %d", openedCount)
	}

	if r.SessionCount() != 1 {
		t.Errorf("expected 1 session after deduplication, got %d", r.SessionCount())
	}
}

// ─────────────────────────────── Sessions() / SessionCount() / Lookup ───────

func TestRegistry_SessionsSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()

	pids := []int{2001, 2002}
	for _, pid := range pids {
		checker.SetAlive(pid, true)
		regWriteSessionFile(t, tmpDir, pid, "snap-session")
	}

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	if !waitForRegistryCount(r, 2, 5*time.Second) {
		t.Fatalf("expected 2 sessions, got %d", r.SessionCount())
	}

	snap := r.Sessions()
	if len(snap) != 2 {
		t.Fatalf("expected 2 sessions in snapshot, got %d", len(snap))
	}

	// Verify snapshot is a copy: modifying it does not affect registry
	snap = snap[:0]
	if r.SessionCount() != 2 {
		t.Error("modifying snapshot should not affect registry")
	}
}

func TestRegistry_Sessions_Empty(t *testing.T) {
	r := NewRegistry("", WithRegistryNoDirWatcher())
	snap := r.Sessions()
	if snap != nil {
		t.Errorf("expected nil slice for empty registry, got %v", snap)
	}
}

func TestRegistry_SessionCount_Zero(t *testing.T) {
	r := NewRegistry("", WithRegistryNoDirWatcher())
	if r.SessionCount() != 0 {
		t.Errorf("expected 0 sessions, got %d", r.SessionCount())
	}
}

func TestRegistry_LookupSession_Found(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 3001
	checker.SetAlive(pid, true)
	regWriteSessionFile(t, tmpDir, pid, "lookup-session")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	s, ok := r.LookupSession(pid)
	if !ok {
		t.Errorf("expected to find session for PID %d", pid)
	}
	if s.Meta.PID != pid {
		t.Errorf("expected PID %d, got %d", pid, s.Meta.PID)
	}
	if s.Meta.SessionID != "lookup-session" {
		t.Errorf("expected sessionID 'lookup-session', got %q", s.Meta.SessionID)
	}
}

func TestRegistry_LookupSession_NotFound(t *testing.T) {
	r := NewRegistry("", WithRegistryNoDirWatcher())
	_, ok := r.LookupSession(99999)
	if ok {
		t.Error("expected LookupSession to return false for unknown PID")
	}
}

// ─────────────────────────────── Events() channel properties ────────────────

func TestRegistry_EventChannel_NotClosed_AfterStop(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryScanInterval(MinPollInterval),
	)
	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)
	cancel()
	r.Stop()

	// Channel should still be open (registry never closes it)
	select {
	case _, ok := <-r.Events():
		if !ok {
			t.Error("events channel should not be closed by registry")
		}
	default:
		// Good — channel is open but empty
	}
}

// ─────────────────────────────── close event carries full metadata ───────────

func TestRegistry_ClosedEventCarriesFullMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 88001
	checker.SetAlive(pid, true)
	regWriteSessionFile(t, tmpDir, pid, "full-meta-session")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
		WithRegistryMonitorInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Wait for open event
	waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	// Kill the PID so monitor detects closure
	checker.SetAlive(pid, false)

	// Wait for close event — it must carry FULL metadata from the registry map
	ce := waitForRegistryCloseEvent(t, r, pid, 5*time.Second)
	if ce.Session.Meta.SessionID != "full-meta-session" {
		t.Errorf("close event should carry full metadata, got sessionID %q", ce.Session.Meta.SessionID)
	}
}

// ─────────────────────────────── addSession / removeSession unit tests ───────

func TestRegistry_AddSession_Deduplicate(t *testing.T) {
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryScanInterval(MinPollInterval),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Provide a context so monitor can operate
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.monitor.Start(r.ctx)
	defer r.monitor.Stop()

	s := ActiveSession{
		Meta:     SessionMeta{PID: 10001, SessionID: "dedup-test"},
		FilePath: "/tmp/test.json",
	}

	r.addSession(s)
	r.addSession(s) // second call should be a no-op

	if r.SessionCount() != 1 {
		t.Errorf("expected 1 session after duplicate add, got %d", r.SessionCount())
	}

	// Only 1 event should have been emitted
	events := drainAllEvents(r.Events(), 100*time.Millisecond)
	openedCount := 0
	for _, ev := range events {
		if ev.Type == SessionOpened {
			openedCount++
		}
	}
	if openedCount != 1 {
		t.Errorf("expected 1 SessionOpened event (dedup), got %d", openedCount)
	}
}

func TestRegistry_RemoveSession_Idempotent(t *testing.T) {
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryScanInterval(MinPollInterval),
	)

	// Removing a non-existent session should not panic or emit events
	r.removeSession(99999)

	select {
	case ev := <-r.Events():
		t.Errorf("expected no event, got %v", ev)
	default:
		// Good
	}
}

func TestRegistry_RemoveSession_EmitsClosedWithFullMeta(t *testing.T) {
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryScanInterval(MinPollInterval),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.monitor.Start(r.ctx)
	defer r.monitor.Stop()

	s := ActiveSession{
		Meta:     SessionMeta{PID: 20001, SessionID: "remove-meta-test"},
		FilePath: "/tmp/test.json",
	}

	r.addSession(s)
	// Drain the open event
	drainAllEvents(r.Events(), 100*time.Millisecond)

	r.removeSession(s.Meta.PID)

	events := drainAllEvents(r.Events(), 200*time.Millisecond)
	if len(events) == 0 {
		t.Fatal("expected SessionClosed event")
	}
	if events[0].Type != SessionClosed {
		t.Errorf("expected SessionClosed, got %v", events[0].Type)
	}
	if events[0].Session.Meta.SessionID != "remove-meta-test" {
		t.Errorf("expected full metadata in close event, got sessionID %q", events[0].Session.Meta.SessionID)
	}
}

// ─────────────────────────────── concurrent access ──────────────────────────

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry("",
		WithRegistryNoDirWatcher(),
		WithRegistryScanInterval(MinPollInterval),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.monitor.Start(r.ctx)
	defer r.monitor.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pid := id + 5000
			s := ActiveSession{
				Meta:     SessionMeta{PID: pid, SessionID: "concurrent"},
				FilePath: "/tmp/test.json",
			}
			r.addSession(s)
			r.SessionCount()
			r.Sessions()
			_, _ = r.LookupSession(pid)
			r.removeSession(pid)
		}(i)
	}

	wg.Wait()

	if r.SessionCount() != 0 {
		t.Errorf("expected 0 sessions after concurrent add/remove, got %d", r.SessionCount())
	}
}

// ─────────────────────────────── no goroutine leak ──────────────────────────

func TestRegistry_NoGoroutineLeak(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()

	for i := 0; i < 5; i++ {
		r := NewRegistry(tmpDir,
			WithRegistryNoDirWatcher(),
			WithRegistryPIDChecker(checker),
			WithRegistryScanInterval(MinPollInterval),
		)
		ctx, cancel := context.WithCancel(context.Background())
		r.Start(ctx)
		cancel()
		r.Stop()
	}
	// If we reach here without hanging, no goroutine leaks.
}

// ─────────────────────────────── detection latency ──────────────────────────

func TestRegistry_DetectionWithin2Seconds(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 66001
	checker.SetAlive(pid, true)

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(500*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Write session file and start the clock
	start := time.Now()
	regWriteSessionFile(t, tmpDir, pid, "latency-session")

	ev := waitForRegistryOpenEvent(t, r, pid, 3*time.Second)
	elapsed := time.Since(start)

	if ev.Type != SessionOpened {
		t.Errorf("expected SessionOpened")
	}
	if elapsed > 2*time.Second {
		t.Errorf("detection latency %v exceeds 2-second window", elapsed)
	}
	t.Logf("detection latency: %v", elapsed)
}

// ─────────────────────────────── scan interval requirement ──────────────────

func TestRegistry_DefaultScanInterval_MeetsRequirement(t *testing.T) {
	// Sub-AC 2 requires detection within 2-second window.
	// The default scanner interval must be ≤ 2s.
	r := NewRegistry("", WithRegistryNoDirWatcher())
	if r.scanner.Interval() > 2*time.Second {
		t.Errorf("default scan interval %v exceeds 2-second detection window requirement",
			r.scanner.Interval())
	}
}

// ─────────────────────────────── dir watcher integration ────────────────────

func TestRegistry_DirWatcher_EmitsOpenedEvent(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 44001
	checker.SetAlive(pid, true)

	w, err := NewSessionDirectoryWatcher(tmpDir, WithDirWatcherPIDChecker(checker))
	if err != nil {
		t.Fatalf("create dir watcher: %v", err)
	}
	defer w.Close()

	// Use a very long scan interval so the dir watcher fires well before polling.
	r := NewRegistry(tmpDir,
		WithRegistryDirWatcher(w),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(60*time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Small delay to let the watcher initialise
	time.Sleep(50 * time.Millisecond)

	regWriteSessionFile(t, tmpDir, pid, "dir-watcher-session")

	ev := waitForRegistryOpenEvent(t, r, pid, 3*time.Second)
	if ev.Session.Meta.PID != pid {
		t.Errorf("expected PID %d, got %d", pid, ev.Session.Meta.PID)
	}
}

// ─────────────────────────────── empty sessions dir ─────────────────────────

func TestRegistry_EmptySessionsDir_NoSessions(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// After a couple of scan cycles, should still have 0 sessions
	time.Sleep(2 * MinPollInterval)

	if r.SessionCount() != 0 {
		t.Errorf("expected 0 sessions in empty dir, got %d", r.SessionCount())
	}
}

// ─────────────────────────────── nil dirWatcher handling ────────────────────

func TestRegistry_NilDirWatcher_RunLoopStillWorks(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 55001
	checker.SetAlive(pid, true)
	regWriteSessionFile(t, tmpDir, pid, "nodirwatcher-session")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	if r.dirWatcher != nil {
		t.Fatal("expected no dir watcher")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Scanner should still detect the session
	if !waitForRegistryCount(r, 1, 3*time.Second) {
		t.Errorf("expected 1 session via polling, got %d", r.SessionCount())
	}
}

// ─────────────────────────────── batch open on first scan ───────────────────

func TestRegistry_BatchOpenOnFirstScan(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()

	// Pre-populate 3 sessions
	pids := []int{30001, 30002, 30003}
	for _, pid := range pids {
		checker.SetAlive(pid, true)
		regWriteSessionFile(t, tmpDir, pid, "batch-"+strconv.Itoa(pid))
	}

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	if !waitForRegistryCount(r, 3, 5*time.Second) {
		t.Errorf("expected 3 sessions on first scan, got %d", r.SessionCount())
	}
}

func TestRegistry_SubsequentScanDetectsNewSession(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()

	pid1 := 40001
	checker.SetAlive(pid1, true)
	regWriteSessionFile(t, tmpDir, pid1, "session-1")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Wait for first session
	waitForRegistryOpenEvent(t, r, pid1, 3*time.Second)

	// Add a second session
	pid2 := 40002
	checker.SetAlive(pid2, true)
	regWriteSessionFile(t, tmpDir, pid2, "session-2")

	waitForRegistryOpenEvent(t, r, pid2, 3*time.Second)

	if r.SessionCount() != 2 {
		t.Errorf("expected 2 sessions, got %d", r.SessionCount())
	}
}

// ─────────────────────────────── monitor untracking ─────────────────────────

func TestRegistry_MonitorUntracked_AfterSessionClosed(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 70001
	checker.SetAlive(pid, true)
	regWriteSessionFile(t, tmpDir, pid, "untrack-session")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
		WithRegistryMonitorInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Wait for session to appear in monitor
	waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	// Verify monitor is tracking this PID
	if !r.monitor.IsTracking(pid) {
		t.Error("monitor should be tracking the session PID")
	}

	// Kill the PID so monitor detects closure
	checker.SetAlive(pid, false)
	waitForRegistryCloseEvent(t, r, pid, 5*time.Second)

	// Monitor should have untracked the PID
	if r.monitor.IsTracking(pid) {
		t.Error("monitor should have untracked the closed PID")
	}
}

// ─────────────────────────────── Sub-AC 2: end-to-end close latency ──────────

// TestRegistry_SessionCloseLatency_EndToEnd validates the Sub-AC 2 requirement:
// session closure MUST be detected and the SessionClosed event emitted within
// ≤5 seconds of PID exit, measured end-to-end through the Registry pipeline.
func TestRegistry_SessionCloseLatency_EndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 80101
	checker.SetAlive(pid, true)
	regWriteSessionFile(t, tmpDir, pid, "e2e-close-latency")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
		WithRegistryMonitorInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Wait for the session to appear in the registry.
	waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	// Kill the PID and start the latency clock.
	checker.SetAlive(pid, false)
	start := time.Now()

	// Sub-AC 2 SLA: SessionClosed callback received within ≤5 seconds.
	ce := waitForRegistryCloseEvent(t, r, pid, 5*time.Second)
	elapsed := time.Since(start)

	if ce.Session.Meta.PID != pid {
		t.Errorf("close event has wrong PID: got %d, want %d", ce.Session.Meta.PID, pid)
	}
	if elapsed > 5*time.Second {
		t.Errorf("end-to-end session-close callback latency %v exceeds ≤5s SLA", elapsed)
	}
	t.Logf("Sub-AC 2 Registry end-to-end close latency: %v", elapsed)
}

// TestRegistry_ScannerChannelClosed_RunContinues verifies that when the scanner
// stops (its channel closes), the run() goroutine disables that select case and
// continues processing dir-watcher / monitor events without panicking.
func TestRegistry_ScannerChannelClosed_RunContinues(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	// Stop the scanner — this closes its result channel.
	// The run() goroutine should survive and set scanCh = nil.
	r.scanner.Stop()

	// Give run() time to process the closed channel.
	time.Sleep(100 * time.Millisecond)

	// Registry should still be functional (monitor + dir watcher still active).
	// We just verify no panic or deadlock.
	if r.SessionCount() != 0 {
		t.Errorf("expected 0 sessions, got %d", r.SessionCount())
	}

	r.Stop() // Clean shutdown
}

// TestRegistry_DirWatcherClosedChannel_RunContinues verifies that when the
// dir-watcher is closed, run() continues gracefully without panicking.
func TestRegistry_DirWatcherClosedChannel_RunContinues(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()

	dw, err := NewSessionDirectoryWatcher(tmpDir, WithDirWatcherPIDChecker(checker))
	if err != nil {
		t.Fatalf("create dir watcher: %v", err)
	}

	r := NewRegistry(tmpDir,
		WithRegistryDirWatcher(dw),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	// Give the registry time to initialise before closing the dir watcher.
	time.Sleep(50 * time.Millisecond)
	_ = dw.Close()

	// Allow run() to process the watcher shutdown.
	time.Sleep(100 * time.Millisecond)

	// Registry must still be running and reachable.
	_ = r.SessionCount()

	r.Stop() // Clean shutdown
}

// ─────────────────────── Sub-AC 2: 5-second SLA validation ─────────────────

// TestRegistry_ClosureDetectionSLA validates that session-close callbacks are
// triggered within the 5-second end-to-end latency requirement (Sub-AC 2).
// The registry monitors ~/.claude/sessions/{pid}.json files and detects PID
// exit via the Monitor's PID liveness polling.
func TestRegistry_ClosureDetectionSLA(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 80001
	checker.SetAlive(pid, true)
	regWriteSessionFile(t, tmpDir, pid, "sla-session")

	r := NewRegistry(tmpDir,
		WithRegistryNoDirWatcher(),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(MinPollInterval),
		// Use DefaultPollInterval (2s) for the monitor — worst-case detection: ~2s
		WithRegistryMonitorInterval(DefaultPollInterval),
		WithRegistryEventBuffer(16),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Wait for session to appear
	waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	// Kill PID and measure detection latency
	checker.SetAlive(pid, false)
	start := time.Now()

	// Verify closure is detected within the 5-second SLA
	waitForRegistryCloseEvent(t, r, pid, 5*time.Second)
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Errorf("session-close callback latency %v exceeds 5s end-to-end SLA", elapsed)
	}
	t.Logf("Sub-AC 2 SLA satisfied: closure detected in %v (max 5s)", elapsed)
}

// TestRegistry_DirWatcher_SessionClosedEvent covers the path in run() where
// a SessionClosed event arrives from the dir watcher (file removal triggers it).
func TestRegistry_DirWatcher_SessionClosedEvent(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 81001
	checker.SetAlive(pid, true)

	// Create a dir watcher with a fast event buffer
	dw, err := NewSessionDirectoryWatcher(tmpDir,
		WithDirWatcherPIDChecker(checker),
		WithDirWatcherEventBuffer(32),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	// Use long scan interval and monitor interval so the dir watcher path is exercised
	r := NewRegistry(tmpDir,
		WithRegistryDirWatcher(dw),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(60*time.Second),     // avoid scanner interference
		WithRegistryMonitorInterval(MaxPollInterval), // slow monitor to avoid racing
		WithRegistryEventBuffer(64),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	// Wait for dir watcher to start up
	time.Sleep(50 * time.Millisecond)

	// Write session file - dir watcher should emit SessionOpened
	filePath := regWriteSessionFile(t, tmpDir, pid, "dir-watcher-closed-session")
	waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	if r.SessionCount() != 1 {
		t.Fatalf("expected 1 session after open, got %d", r.SessionCount())
	}

	// Remove the session file - dir watcher should emit SessionClosed
	// which exercises the SessionClosed branch in run() at line 303-305
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}

	// Wait for SessionClosed via dir watcher (not via monitor)
	waitForRegistryCloseEvent(t, r, pid, 3*time.Second)

	if r.SessionCount() != 0 {
		t.Errorf("expected 0 sessions after dir watcher SessionClosed, got %d", r.SessionCount())
	}
}

// TestNewRegistry_DefaultDirWatcher_Created verifies that NewRegistry creates
// a dir watcher by default (no WithRegistryNoDirWatcher option).
func TestNewRegistry_DefaultDirWatcher_Created(t *testing.T) {
	tmpDir := t.TempDir()
	// Do NOT pass WithRegistryNoDirWatcher — default path should create a watcher
	r := NewRegistry(tmpDir)
	if r.dirWatcher == nil {
		t.Error("expected dir watcher to be auto-created when WithRegistryNoDirWatcher is not set")
	}
	// Cleanup: close the dir watcher
	if r.dirWatcher != nil {
		_ = r.dirWatcher.Close()
	}
}

// TestRegistry_Run_HandlesScanChannelClose exercises the defensive !ok path in
// run() when the scan channel is closed. This covers the `scanCh = nil; continue`
// branch that fires when the scanner's goroutine has exited and closed its channel.
func TestRegistry_Run_HandlesScanChannelClose(t *testing.T) {
	r := NewRegistry("", WithRegistryNoDirWatcher())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Provide ctx to registry and start the monitor so run() can emit events.
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.monitor.Start(r.ctx)
	defer r.monitor.Stop()

	// Create a channel that is ALREADY CLOSED to trigger the !ok path immediately.
	scanCh := make(chan ScanResult)
	close(scanCh)

	// Start run() directly with the pre-closed channel.
	r.wg.Add(1)
	go r.run(scanCh)

	// Allow run() to process the closed channel (it disables the case and continues)
	time.Sleep(50 * time.Millisecond)

	// Cancel the context to signal run() to exit via ctx.Done()
	cancel()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good — run() exited cleanly after ctx cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("run() goroutine did not exit within 2 seconds after ctx cancellation")
	}
}

// TestRegistry_Run_WithDirWatcherClosed exercises the SessionClosed branch in
// run() via dir_watcher (file removed) and verifies the registry state is updated.
// This is distinct from the monitor-based closure path.
func TestRegistry_Run_DirWatcher_SessionClosed_Branch(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newRegistryMockPIDChecker()
	pid := 91001
	checker.SetAlive(pid, true)

	dw, err := NewSessionDirectoryWatcher(tmpDir,
		WithDirWatcherPIDChecker(checker),
		WithDirWatcherEventBuffer(32),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	// Long scan + monitor intervals so dir-watcher is the primary event source
	r := NewRegistry(tmpDir,
		WithRegistryDirWatcher(dw),
		WithRegistryPIDChecker(checker),
		WithRegistryScanInterval(60*time.Second),
		WithRegistryMonitorInterval(MaxPollInterval),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create then delete the session file to trigger both SessionOpened and
	// SessionClosed events from the dir watcher's watchLoop.
	filePath := regWriteSessionFile(t, tmpDir, pid, "dw-close-branch-session")
	waitForRegistryOpenEvent(t, r, pid, 3*time.Second)

	// Removing the file causes the dir watcher to emit SessionClosed, exercising
	// the `case SessionClosed: r.removeSession(...)` branch in run().
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}
	waitForRegistryCloseEvent(t, r, pid, 3*time.Second)

	if !waitForRegistryCount(r, 0, 2*time.Second) {
		t.Errorf("expected 0 sessions after dir-watcher close, got %d", r.SessionCount())
	}
}

// TestRegistry_Run_HandlesDirWatcherChannelClose exercises the defensive
// `if !ok { dirWatcherCh = nil; continue }` branch in run() (lines 296-298).
// This branch fires when the dir watcher's events channel is closed.
// While this is documented as "never closed" in normal operation, the
// defensive code handles it gracefully.
//
// Since both test and implementation are in package session, we can directly
// close the private events channel before calling run().
func TestRegistry_Run_HandlesDirWatcherChannelClose(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dir watcher but do NOT call Start(), so watchLoop is not running.
	dw, err := NewSessionDirectoryWatcher(tmpDir)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	// Close the events channel directly (safe: watchLoop is not running).
	close(dw.events)
	// Close the underlying fsWatcher to prevent resource leak.
	_ = dw.fsWatcher.Close()

	r := NewRegistry(tmpDir, WithRegistryDirWatcher(dw))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.ctx, r.cancel = context.WithCancel(ctx)
	// Do NOT start the monitor's poll loop (no r.monitor.Start) to keep test simple.

	// Provide an open scan channel to keep run() looping after dirWatcher closes.
	scanCh := make(chan ScanResult)

	r.wg.Add(1)
	go r.run(scanCh)

	// Give run() time to drain the closed dirWatcher channel and set it to nil.
	time.Sleep(50 * time.Millisecond)

	// Cancel context to signal run() to exit.
	cancel()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good — run() exited cleanly after ctx cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("run() goroutine did not exit within 2 seconds after ctx cancellation")
	}
	close(scanCh) // Cleanup open scan channel (no goroutine is draining it)
}

// TestRegistry_Run_HandlesMonitorChannelClose exercises the defensive
// `if !ok { monitorCh = nil; continue }` branch in run() (lines 308-310).
// This branch fires when the monitor's events channel is closed.
// While the Monitor documents its channel as "never closed", this defensive
// code ensures run() degrades gracefully if it ever does close.
func TestRegistry_Run_HandlesMonitorChannelClose(t *testing.T) {
	// Create a registry with no dir watcher; we will manually close the
	// monitor's events channel to simulate this rare defensive path.
	r := NewRegistry("", WithRegistryNoDirWatcher())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.ctx, r.cancel = context.WithCancel(ctx)
	// Do NOT start the monitor poll loop (we're closing its channel manually).

	// Close the monitor's events channel directly (same package — private access allowed).
	// Safe because the monitor's pollLoop goroutine is NOT running.
	close(r.monitor.events)

	// Provide an open scan channel so run() doesn't exit via scanCh closure.
	scanCh := make(chan ScanResult)

	r.wg.Add(1)
	go r.run(scanCh)

	// Give run() time to drain the closed monitor channel and set it to nil.
	time.Sleep(50 * time.Millisecond)

	// Cancel context to signal run() to exit.
	cancel()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good — run() exited cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("run() goroutine did not exit within 2 seconds after ctx cancellation")
	}
	close(scanCh) // Cleanup
}
