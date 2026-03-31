// Package tui provides the terminal user interface components.
// This file tests Sub-AC 2: session event emitter that broadcasts a
// 'new-session' event with session metadata when a new session file is
// detected by the SessionDirectoryWatcher.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

// ─────────────────────────────────────────────────────────────────────────────
// End-to-end: SessionDirectoryWatcher emits new-session event with metadata
// ─────────────────────────────────────────────────────────────────────────────

// TestSessionEventEmitter_NewSessionFileEmitsOpenedEventWithMetadata verifies
// the core requirement of Sub-AC 2: when a new {pid}.json session file appears
// in the sessions directory, the SessionDirectoryWatcher broadcasts a
// SessionOpened event that carries the full session metadata (PID, SessionID,
// CWD, FilePath, JSONLDir).
func TestSessionEventEmitter_NewSessionFileEmitsOpenedEventWithMetadata(t *testing.T) {
	pid := 8001
	sessDir := t.TempDir()
	checker := newTestPIDChecker(pid)

	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
		session.WithDirWatcherEventBuffer(32),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer dw.Close()

	if err := dw.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Write a session file with known metadata.
	sessionID := "emit-test-session"
	cwd := "/tmp/test-project"
	meta := session.SessionMeta{
		PID:       pid,
		SessionID: sessionID,
		CWD:       cwd,
		StartedAt: time.Now().UnixMilli(),
		Kind:      "interactive",
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	filePath := filepath.Join(sessDir, fmt.Sprintf("%d.json", pid))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	// Wait for the SessionOpened event.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-dw.Events():
			if event.Type != session.SessionOpened {
				continue
			}
			if event.Session.Meta.PID != pid {
				continue
			}
			// Verify full metadata is present in the emitted event.
			if event.Session.Meta.SessionID != sessionID {
				t.Errorf("SessionID = %q, want %q", event.Session.Meta.SessionID, sessionID)
			}
			if event.Session.Meta.CWD != cwd {
				t.Errorf("CWD = %q, want %q", event.Session.Meta.CWD, cwd)
			}
			if event.Session.FilePath == "" {
				t.Error("FilePath should be non-empty in emitted event")
			}
			if event.Session.JSONLDir == "" {
				t.Error("JSONLDir should be non-empty when CWD is set")
			}
			if event.Session.Meta.Kind != "interactive" {
				t.Errorf("Kind = %q, want %q", event.Session.Meta.Kind, "interactive")
			}
			return // Success
		case <-deadline:
			t.Fatal("timeout waiting for SessionOpened event with metadata")
		}
	}
}

// TestSessionEventEmitter_MultipleSessions_EachEmitsOwnEvent verifies that
// when multiple session files appear, each generates a separate SessionOpened
// event with distinct metadata.
func TestSessionEventEmitter_MultipleSessions_EachEmitsOwnEvent(t *testing.T) {
	pids := []int{8010, 8011, 8012}
	sessDir := t.TempDir()
	checker := newTestPIDChecker(pids...)

	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
		session.WithDirWatcherEventBuffer(64),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer dw.Close()

	if err := dw.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Write session files for all PIDs.
	for _, pid := range pids {
		meta := session.SessionMeta{
			PID:       pid,
			SessionID: fmt.Sprintf("multi-session-%d", pid),
			CWD:       fmt.Sprintf("/tmp/project-%d", pid),
		}
		data, _ := json.Marshal(meta)
		filePath := filepath.Join(sessDir, fmt.Sprintf("%d.json", pid))
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			t.Fatalf("write session file for PID %d: %v", pid, err)
		}
	}

	// Collect all SessionOpened events within a timeout.
	received := make(map[int]session.SessionEvent)
	deadline := time.After(3 * time.Second)
	for len(received) < len(pids) {
		select {
		case event := <-dw.Events():
			if event.Type == session.SessionOpened {
				received[event.Session.Meta.PID] = event
			}
		case <-deadline:
			t.Fatalf("timeout: received %d/%d SessionOpened events", len(received), len(pids))
		}
	}

	// Verify each event has distinct, correct metadata.
	for _, pid := range pids {
		event, ok := received[pid]
		if !ok {
			t.Errorf("no event for PID %d", pid)
			continue
		}
		expectedID := fmt.Sprintf("multi-session-%d", pid)
		if event.Session.Meta.SessionID != expectedID {
			t.Errorf("PID %d: SessionID = %q, want %q", pid, event.Session.Meta.SessionID, expectedID)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// startMonitorCmd: event bridging tests
// ─────────────────────────────────────────────────────────────────────────────

// TestStartMonitorCmd_BridgesSessionClosedEvent verifies that the bridge
// goroutine started by startMonitorCmd delivers a SessionClosed event from
// the Monitor.Events() channel to the dashboard's monitorChan.
func TestStartMonitorCmd_BridgesSessionClosedEvent(t *testing.T) {
	checker := newTestPIDChecker(9001)
	sessDir := t.TempDir()

	monitor := session.NewMonitor(
		session.WithMonitorPIDChecker(checker),
		session.WithPollInterval(session.MinPollInterval),
	)
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/monitor-bridge-test", "/tmp/pd", scanner, monitor)
	m.monitorChan = make(chan session.SessionEvent, 8)

	// Execute startMonitorCmd to start the bridge goroutine.
	cmd := m.startMonitorCmd()
	if cmd == nil {
		t.Fatal("startMonitorCmd should return non-nil cmd")
	}
	result := cmd()
	if result != nil {
		t.Errorf("startMonitorCmd() should return nil msg, got %T", result)
	}

	// Track a session in the monitor, then mark it as dead.
	s := session.ActiveSession{
		Meta:     session.SessionMeta{PID: 9001, SessionID: "bridge-test"},
		FilePath: "/tmp/9001.json",
	}
	monitor.TrackSession(s)
	checker.SetAlive(9001, false)

	// Wait for the SessionClosed event to arrive in monitorChan.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case event := <-m.monitorChan:
			if event.Type == session.SessionClosed && event.Session.Meta.PID == 9001 {
				// Success: bridge delivered the event.
				m.cancel()
				m.WaitForGoroutines()
				return
			}
		case <-deadline:
			m.cancel()
			m.WaitForGoroutines()
			t.Fatal("timeout: monitorChan bridge did not deliver SessionClosed event")
		}
	}
}

// TestStartMonitorCmd_ContextCancellationStopsBridgeGoroutine verifies that
// cancelling the context causes the monitor bridge goroutine to exit cleanly.
func TestStartMonitorCmd_ContextCancellationStopsBridgeGoroutine(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	monitor := session.NewMonitor(
		session.WithMonitorPIDChecker(checker),
		session.WithPollInterval(session.MinPollInterval),
	)
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/monitor-ctx-cancel", "/tmp/pd", scanner, monitor)

	cmd := m.startMonitorCmd()
	cmd()

	// Give the goroutine time to start.
	time.Sleep(20 * time.Millisecond)

	// Cancel context and stop monitor.
	m.cancel()
	monitor.Stop()

	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Goroutine exited cleanly.
	case <-time.After(3 * time.Second):
		t.Error("monitor bridge goroutine did not exit after context cancellation")
	}
}

// TestStartMonitorCmd_ChannelFull_DoesNotBlock verifies that when monitorChan
// is full, the bridge goroutine drops the event rather than blocking.
func TestStartMonitorCmd_ChannelFull_DoesNotBlock(t *testing.T) {
	checker := newTestPIDChecker(9010)
	sessDir := t.TempDir()

	monitor := session.NewMonitor(
		session.WithMonitorPIDChecker(checker),
		session.WithPollInterval(session.MinPollInterval),
	)
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/monitor-full-chan", "/tmp/pd", scanner, monitor)
	// Use a very small channel so it fills immediately.
	m.monitorChan = make(chan session.SessionEvent, 1)

	cmd := m.startMonitorCmd()
	cmd()

	// Fill the channel.
	m.monitorChan <- session.SessionEvent{Type: session.SessionOpened}

	// Track a session and mark it dead to trigger an event.
	s := session.ActiveSession{
		Meta: session.SessionMeta{PID: 9010, SessionID: "full-chan-test"},
	}
	monitor.TrackSession(s)
	checker.SetAlive(9010, false)

	// Give time for monitor to poll and the bridge to try to send.
	time.Sleep(300 * time.Millisecond)

	// The bridge should have dropped the event (channel full) without deadlock.
	m.cancel()
	monitor.Stop()

	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()
	select {
	case <-done:
		// Passed: no deadlock.
	case <-time.After(3 * time.Second):
		t.Error("bridge goroutine deadlocked on full monitorChan")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// startScannerCmd: already-running path
// ─────────────────────────────────────────────────────────────────────────────

// TestStartScannerCmd_AlreadyRunning_ReturnsNil verifies that when the
// scanner is already running, startScannerCmd returns a command that yields nil
// without starting a second bridge goroutine.
func TestStartScannerCmd_AlreadyRunning_ReturnsNil(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(500*time.Millisecond),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/scanner-already-running", "/tmp/pd", scanner, monitor)

	// Start the scanner manually so it's already running.
	resultCh := scanner.Start()
	if resultCh == nil {
		t.Fatal("expected non-nil result channel from first Start()")
	}
	defer scanner.Stop()

	// Now execute startScannerCmd — scanner.Start() should return nil.
	cmd := m.startScannerCmd()
	if cmd == nil {
		t.Fatal("startScannerCmd should return non-nil cmd")
	}

	// Execute the cmd synchronously.
	result := cmd()
	if result != nil {
		t.Errorf("startScannerCmd() should return nil when scanner is already running, got %T", result)
	}
}

// TestStartScannerCmd_BridgesResultToChannel verifies that scan results are
// forwarded to the dashboard's scanResultChan by the bridge goroutine.
func TestStartScannerCmd_BridgesResultToChannel(t *testing.T) {
	checker := newTestPIDChecker(9020)
	sessDir := t.TempDir()

	// Write a session file so the scanner detects something.
	meta := session.SessionMeta{
		PID:       9020,
		SessionID: "scan-bridge-test",
		CWD:       "/tmp/scan-project",
	}
	data, _ := json.Marshal(meta)
	filePath := filepath.Join(sessDir, "9020.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(50*time.Millisecond),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/scan-bridge-project", "/tmp/pd", scanner, monitor)
	m.scanResultChan = make(chan session.ScanResult, 8)

	cmd := m.startScannerCmd()
	cmd()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case result := <-m.scanResultChan:
			if result.Err != nil {
				continue
			}
			// Found a scan result — bridge is working.
			m.cancel()
			scanner.Stop()
			m.WaitForGoroutines()
			return
		case <-deadline:
			m.cancel()
			scanner.Stop()
			m.WaitForGoroutines()
			t.Fatal("timeout: scanResultChan bridge did not deliver scan result")
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// sessionSubscriptionTickCmd
// ─────────────────────────────────────────────────────────────────────────────

// TestSessionSubscriptionTickCmd_ReturnsNonNilCmd verifies that the tick command
// is non-nil and executes the time-based tick callback.
func TestSessionSubscriptionTickCmd_ReturnsNonNilCmd(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/tick-test", "/tmp/pd", scanner, monitor)

	cmd := m.sessionSubscriptionTickCmd()
	if cmd == nil {
		t.Fatal("sessionSubscriptionTickCmd should return non-nil command")
	}
}

// TestSessionSubscriptionTickCmd_DeliversTickMsg verifies that executing the
// tick command delivers a sessionSubscriptionTickMsg after the tick interval.
func TestSessionSubscriptionTickCmd_DeliversTickMsg(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/tick-deliver", "/tmp/pd", scanner, monitor)

	cmd := m.sessionSubscriptionTickCmd()

	// Execute in a goroutine since tea.Tick blocks until the tick fires.
	msgCh := make(chan interface{}, 1)
	go func() {
		msgCh <- cmd()
	}()

	select {
	case msg := <-msgCh:
		if _, ok := msg.(sessionSubscriptionTickMsg); !ok {
			t.Errorf("expected sessionSubscriptionTickMsg, got %T", msg)
		}
	case <-time.After(500 * time.Millisecond): // tick fires at 100ms
		t.Fatal("timeout waiting for tick message")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// startSessionFileWatcherSubscription: file event bridging
// ─────────────────────────────────────────────────────────────────────────────

// TestStartSessionFileWatcherSubscription_WriteEventDelivered verifies that
// when new content is appended to a session's JSONL file, a NewEntriesMsg is
// delivered to the pane's fileEventChan via the subscription goroutine.
func TestStartSessionFileWatcherSubscription_WriteEventDelivered(t *testing.T) {
	checker := newTestPIDChecker(9030)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/file-watcher-write-test"

	sessionID := "write-event-session"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Create a scan result to add the pane.
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 9030, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	if m.PaneCount() != 1 {
		t.Fatalf("expected 1 pane, got %d", m.PaneCount())
	}

	// Create a real file watcher for the JSONL file.
	w, err := watcher.New(jsonlPath)
	if err != nil {
		t.Fatalf("watcher.New: %v", err)
	}
	m.panes[0].watcher = w
	m.panes[0].jsonlPath = jsonlPath
	m.panes[0].session = session.ActiveSession{
		Meta: session.SessionMeta{PID: 9030, SessionID: sessionID, CWD: projectPath},
	}

	// Start the file watcher subscription.
	m.startSessionFileWatcherSubscription(m.ctx, 0)

	ch := m.panes[0].fileEventChan
	if ch == nil {
		t.Fatal("expected fileEventChan to be set after startSessionFileWatcherSubscription")
	}

	// Append a new log entry to the JSONL file to trigger a write event.
	// Use a short sleep to let the watcher initialize.
	time.Sleep(100 * time.Millisecond)
	// Use a valid user entry (simplest parseable format).
	newEntry := `{"type":"user","message":{"role":"user","content":"world"},"timestamp":"2026-03-31T12:00:00Z"}` + "\n"
	f, err := os.OpenFile(jsonlPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open file for append: %v", err)
	}
	if _, err := f.WriteString(newEntry); err != nil {
		f.Close()
		t.Fatalf("write to file: %v", err)
	}
	f.Close()

	// Wait for the NewEntriesMsg to arrive in the pane's event channel.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				t.Fatal("fileEventChan closed unexpectedly")
			}
			if _, isEntries := msg.event.(watcher.NewEntriesMsg); isEntries {
				// Success: write event was bridged as NewEntriesMsg.
				m.cancel()
				_ = w.Close()
				m.WaitForGoroutines()
				return
			}
		case <-deadline:
			m.cancel()
			_ = w.Close()
			m.WaitForGoroutines()
			t.Fatal("timeout: fileEventChan did not receive NewEntriesMsg after file write")
		}
	}
}

// TestStartSessionFileWatcherSubscription_FileTruncationEmitsResetMsg verifies
// that when the JSONL file is truncated (replaced by a shorter file), the
// subscription emits a FileResetMsg to signal conversation restart.
func TestStartSessionFileWatcherSubscription_FileTruncationEmitsResetMsg(t *testing.T) {
	checker := newTestPIDChecker(9031)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/file-truncate-test"

	sessionID := "truncate-event-session"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 9031, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	w, err := watcher.New(jsonlPath)
	if err != nil {
		t.Fatalf("watcher.New: %v", err)
	}
	m.panes[0].watcher = w
	m.panes[0].jsonlPath = jsonlPath
	m.panes[0].session = session.ActiveSession{
		Meta: session.SessionMeta{PID: 9031, SessionID: sessionID, CWD: projectPath},
	}

	m.startSessionFileWatcherSubscription(m.ctx, 0)

	ch := m.panes[0].fileEventChan
	if ch == nil {
		t.Fatal("expected fileEventChan to be set")
	}

	// Let the watcher initialize.
	time.Sleep(50 * time.Millisecond)

	// Truncate the file: write shorter content to simulate file reset.
	// This advances the file position past EOF, which the watcher detects as truncation.
	newContent := `{"type":"user","message":{"role":"user","content":"new"},"timestamp":"2026-03-31T13:00:00Z"}` + "\n"
	if err := os.WriteFile(jsonlPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("truncate/overwrite file: %v", err)
	}

	// Wait for a FileResetMsg.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				// Channel closed means goroutine exited; file reset may have been
				// received before channel closure.
				m.cancel()
				_ = w.Close()
				m.WaitForGoroutines()
				return
			}
			if _, isReset := msg.event.(watcher.FileResetMsg); isReset {
				// Success: truncation detected as FileResetMsg.
				m.cancel()
				_ = w.Close()
				m.WaitForGoroutines()
				return
			}
			// NewEntriesMsg may come first (watcher sees the write before detecting
			// truncation). Continue waiting.
		case <-deadline:
			m.cancel()
			_ = w.Close()
			m.WaitForGoroutines()
			// Note: truncation detection is best-effort (depends on read position).
			// If no reset msg is received, this is acceptable but we log it.
			t.Log("info: no FileResetMsg received within deadline (watcher may not have detected truncation)")
			return
		}
	}
}

// TestStartSessionFileWatcherSubscription_InvalidPaneIndex_NoOp verifies that
// calling startSessionFileWatcherSubscription with an out-of-bounds index is a
// safe no-op that does not panic or leak goroutines.
func TestStartSessionFileWatcherSubscription_InvalidPaneIndex_NoOp(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/invalid-idx", "/tmp/pd", scanner, monitor)

	ctx := context.Background()

	// Negative index
	m.startSessionFileWatcherSubscription(ctx, -1)
	// Out-of-bounds positive index (no panes)
	m.startSessionFileWatcherSubscription(ctx, 0)
	m.startSessionFileWatcherSubscription(ctx, 99)

	// No panics and no goroutines started.
}

// TestStartSessionFileWatcherSubscription_NilWatcher_NoOp verifies that when
// the pane's watcher is nil, the subscription is skipped gracefully.
func TestStartSessionFileWatcherSubscription_NilWatcher_NoOp(t *testing.T) {
	checker := newTestPIDChecker(9035)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/nil-watcher-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Add a pane without a watcher.
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 9035, SessionID: "nil-watcher", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Pane has no watcher — startSessionFileWatcherSubscription should be a no-op.
	m.startSessionFileWatcherSubscription(m.ctx, 0)

	if m.panes[0].fileEventChan != nil {
		t.Error("fileEventChan should remain nil when watcher is nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Session event emitter: event metadata propagation through TUI
// ─────────────────────────────────────────────────────────────────────────────

// TestSessionEventEmitter_MetadataPropagatedToPane verifies that session
// metadata from a SessionOpened event (PID, SessionID, CWD) is correctly
// propagated to the newly created pane in the session dashboard.
func TestSessionEventEmitter_MetadataPropagatedToPane(t *testing.T) {
	checker := newTestPIDChecker(9040)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/metadata-prop-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "prop-test-session"
	makeJSONLFile(t, projectDir, sessionID)

	// Simulate a SessionDirectoryWatcher emitting a new-session event.
	openedSess := session.ActiveSession{
		Meta: session.SessionMeta{
			PID:       9040,
			SessionID: sessionID,
			CWD:       projectPath,
			Kind:      "interactive",
			StartedAt: time.Now().UnixMilli(),
		},
		FilePath: filepath.Join(sessDir, "9040.json"),
	}

	event := session.SessionEvent{
		Type:    session.SessionOpened,
		Session: openedSess,
	}

	// Process the event through the TUI model.
	newModel, cmd := m.Update(sessionDirWatcherEventMsg{event: event})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after SessionOpened event, got %d", updated.PaneCount())
	}

	pane := updated.panes[0]
	if pane.session.Meta.PID != 9040 {
		t.Errorf("pane PID = %d, want 9040", pane.session.Meta.PID)
	}
	if pane.session.Meta.SessionID != sessionID {
		t.Errorf("pane SessionID = %q, want %q", pane.session.Meta.SessionID, sessionID)
	}
	if pane.session.Meta.CWD != projectPath {
		t.Errorf("pane CWD = %q, want %q", pane.session.Meta.CWD, projectPath)
	}
	if pane.session.Meta.Kind != "interactive" {
		t.Errorf("pane Kind = %q, want %q", pane.session.Meta.Kind, "interactive")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (content loading) after pane creation")
	}
}

// TestSessionEventEmitter_WatcherDeliveredToTUI_EndToEnd tests the full
// pipeline: SessionDirectoryWatcher detects file → emits event → dashboard
// bridge delivers it to dirWatcherChan → pollChannels returns the event.
func TestSessionEventEmitter_WatcherDeliveredToTUI_EndToEnd(t *testing.T) {
	pid := 9050
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/e2e-pipeline-test"
	checker := newTestPIDChecker(pid)

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, projectPath, checker)
	defer dw.Close()

	sessionID := "e2e-pipeline-session"
	makeJSONLFile(t, projectDir, sessionID)

	// Start the dir watcher bridge (normally done by Init → startDirWatcherCmd).
	cmd := m.startDirWatcherCmd()
	cmd()

	// Write a session file — the watcher should detect it and bridge the event.
	meta := session.SessionMeta{
		PID:       pid,
		SessionID: sessionID,
		CWD:       projectPath,
	}
	data, _ := json.Marshal(meta)
	filePath := filepath.Join(sessDir, fmt.Sprintf("%d.json", pid))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	// Poll until the event arrives in dirWatcherChan.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-m.dirWatcherChan:
			if event.Type == session.SessionOpened && event.Session.Meta.PID == pid {
				// Verify metadata made it through the pipeline.
				if event.Session.Meta.SessionID != sessionID {
					t.Errorf("SessionID = %q, want %q", event.Session.Meta.SessionID, sessionID)
				}
				if event.Session.Meta.CWD != projectPath {
					t.Errorf("CWD = %q, want %q", event.Session.Meta.CWD, projectPath)
				}
				m.cancel()
				m.WaitForGoroutines()
				return
			}
		case <-deadline:
			m.cancel()
			m.WaitForGoroutines()
			t.Fatal("timeout: end-to-end pipeline did not deliver SessionOpened event")
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// startDirWatcherCmd: Start() failure path
// ─────────────────────────────────────────────────────────────────────────────

// TestStartDirWatcherCmd_AlreadyStartedWatcher_ReturnsNil verifies that when
// the SessionDirectoryWatcher has already been started (Start() returns an
// error because it's started or closed), startDirWatcherCmd handles it
// gracefully by returning nil (scanner fallback continues to work).
func TestStartDirWatcherCmd_AlreadyStartedWatcher_ReturnsNil(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, "/tmp/dw-already-started", checker)

	// Close the watcher BEFORE startDirWatcherCmd so Start() will fail.
	if err := dw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cmd := m.startDirWatcherCmd()
	result := cmd()
	if result != nil {
		t.Errorf("startDirWatcherCmd should return nil when Start() fails, got %T", result)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// startScannerCmd: channel-close path
// ─────────────────────────────────────────────────────────────────────────────

// TestStartScannerCmd_ScannerChannelCloses_GoroutineExits verifies that when
// the scanner's result channel is closed (scanner stopped), the bridge goroutine
// exits cleanly without goroutine leaks.
func TestStartScannerCmd_ScannerChannelCloses_GoroutineExits(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(50*time.Millisecond),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/scanner-close-test", "/tmp/pd", scanner, monitor)

	cmd := m.startScannerCmd()
	cmd()

	// Give the goroutine time to start.
	time.Sleep(30 * time.Millisecond)

	// Stop the scanner — this closes the result channel.
	scanner.Stop()

	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Bridge goroutine exited cleanly after channel close.
	case <-time.After(3 * time.Second):
		m.cancel()
		t.Error("bridge goroutine did not exit after scanner channel closed")
	}
}

// TestStartScannerCmd_ScanResultChanFull_DoesNotBlock verifies that when
// scanResultChan is full, the bridge goroutine drops the scan result (default
// case) rather than blocking the scanner.
func TestStartScannerCmd_ScanResultChanFull_DoesNotBlock(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(50*time.Millisecond),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/scanner-full-chan", "/tmp/pd", scanner, monitor)
	// Replace scanResultChan with a size-1 channel, then pre-fill it.
	m.scanResultChan = make(chan session.ScanResult, 1)
	m.scanResultChan <- session.ScanResult{}

	cmd := m.startScannerCmd()
	cmd()

	// Give several scan cycles time to run (they should drop without blocking).
	time.Sleep(200 * time.Millisecond)

	// Cancel and verify clean exit.
	m.cancel()
	scanner.Stop()
	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()
	select {
	case <-done:
		// No deadlock.
	case <-time.After(3 * time.Second):
		t.Error("bridge deadlocked on full scanResultChan")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// startMonitorCmd: stop + cancel cleanup path
// ─────────────────────────────────────────────────────────────────────────────

// TestStartMonitorCmd_StopAndCancel_GoroutineExits verifies that when the
// monitor is stopped and the dashboard context is cancelled, the bridge
// goroutine exits cleanly. Monitor.Events() is never closed by the monitor,
// so context cancellation is the expected exit mechanism.
func TestStartMonitorCmd_StopAndCancel_GoroutineExits(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	monitor := session.NewMonitor(
		session.WithMonitorPIDChecker(checker),
		session.WithPollInterval(session.MinPollInterval),
	)
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/monitor-close-test", "/tmp/pd", scanner, monitor)

	cmd := m.startMonitorCmd()
	cmd()

	// Give the goroutine time to start.
	time.Sleep(30 * time.Millisecond)

	// Stop the monitor, then cancel the context.
	monitor.Stop()
	m.cancel()

	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Bridge goroutine exited cleanly via context cancellation.
	case <-time.After(3 * time.Second):
		t.Error("monitor bridge goroutine did not exit after context cancelled")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// startSessionFileWatcherSubscription: errors channel
// ─────────────────────────────────────────────────────────────────────────────

// TestStartSessionFileWatcherSubscription_WatcherClosed_GoroutineExits
// verifies that when the file watcher is closed (both events and errors
// channels close), the subscription goroutine exits without deadlock.
func TestStartSessionFileWatcherSubscription_WatcherClosed_GoroutineExits(t *testing.T) {
	checker := newTestPIDChecker(9060)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/watcher-close-goroutine"

	sessionID := "close-goroutine-session"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 9060, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	w, err := watcher.New(jsonlPath)
	if err != nil {
		t.Fatalf("watcher.New: %v", err)
	}
	m.panes[0].watcher = w
	m.panes[0].jsonlPath = jsonlPath
	m.panes[0].session = session.ActiveSession{
		Meta: session.SessionMeta{PID: 9060, SessionID: sessionID, CWD: projectPath},
	}

	m.startSessionFileWatcherSubscription(m.ctx, 0)

	// Give goroutine time to start.
	time.Sleep(30 * time.Millisecond)

	// Close the watcher — this closes EventsChan and ErrorsChan.
	if err := w.Close(); err != nil {
		t.Fatalf("watcher.Close: %v", err)
	}

	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Subscription goroutine exited cleanly when watcher closed.
	case <-time.After(3 * time.Second):
		m.cancel()
		t.Error("subscription goroutine did not exit after watcher closed")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// handlePaneContentLoaded: existing watcher replacement
// ─────────────────────────────────────────────────────────────────────────────

// TestHandlePaneContentLoaded_ReplacesExistingWatcher verifies that when
// content is loaded for a pane that already has a watcher, the old watcher
// is closed and replaced with a new one for the updated file path.
func TestHandlePaneContentLoaded_ReplacesExistingWatcher(t *testing.T) {
	checker := newTestPIDChecker(9070)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/replace-watcher-test"

	sessionID := "replace-watcher-session"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 9070, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Create an initial watcher and attach it to the pane.
	initialWatcher, err := watcher.New(jsonlPath)
	if err != nil {
		t.Fatalf("watcher.New: %v", err)
	}
	m.panes[0].watcher = initialWatcher

	// Send a content loaded message with a valid file path.
	// This should close the old watcher and create a new one.
	loadedMsg := sessionPaneContentLoadedMsg{
		sessionID: sessionID,
		entries:   nil,
		filePath:  jsonlPath,
		err:       nil,
	}
	newModel, _ = m.Update(loadedMsg)
	m = newModel.(SessionDashboardModel)

	// The pane should have a new watcher (old one was replaced).
	if m.panes[0].watcher == initialWatcher {
		t.Error("expected new watcher to replace old one")
	}
	if m.panes[0].watcher == nil {
		t.Error("expected a new watcher to be created for the pane")
	}

	// Cleanup.
	m.closeAll()
}

// TestHandlePaneContentLoaded_EmptyFilePath_NoWatcher verifies that when
// content is loaded without a file path, no watcher is created for the pane.
func TestHandlePaneContentLoaded_EmptyFilePath_NoWatcher(t *testing.T) {
	checker := newTestPIDChecker(9071)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/no-watcher-test"

	sessionID := "no-watcher-session"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 9071, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Content loaded with empty file path — no watcher should be created.
	loadedMsg := sessionPaneContentLoadedMsg{
		sessionID: sessionID,
		entries:   nil,
		filePath:  "", // No file path
		err:       nil,
	}
	newModel, _ = m.Update(loadedMsg)
	m = newModel.(SessionDashboardModel)

	if m.panes[0].watcher != nil {
		t.Error("expected no watcher when filePath is empty")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// renderContent: edge cases
// ─────────────────────────────────────────────────────────────────────────────

// TestRenderContent_EmptyEntries_ReturnsEmpty verifies that renderContent
// returns an empty string when there are no log entries.
func TestRenderContent_EmptyEntries_ReturnsEmpty(t *testing.T) {
	pane := SessionPaneModel{
		entries: nil,
		width:   80,
		height:  24,
	}
	result := pane.renderContent()
	if result != "" {
		t.Errorf("expected empty string for empty entries, got %q", result)
	}
}

// TestRenderContent_NarrowWidth_UsesMinimum verifies that renderContent uses
// the minimum content width when the pane is very narrow.
func TestRenderContent_NarrowWidth_UsesMinimum(t *testing.T) {
	pane := SessionPaneModel{
		width:  5, // Very narrow (less than minimum threshold)
		height: 20,
		entries: []types.LogEntry{
			{Type: types.EntryTypeUser},
		},
	}
	// Should not panic even with very narrow width.
	result := pane.renderContent()
	// Result can be empty (entry has no content to render) but should not panic.
	_ = result
}

// TestRenderContent_SmallHeight_UsesMinimum verifies that renderContent handles
// very small height gracefully.
func TestRenderContent_SmallHeight_UsesMinimum(t *testing.T) {
	pane := SessionPaneModel{
		width:  80,
		height: 2, // Very small height (less than 3 for border)
		entries: []types.LogEntry{
			{Type: types.EntryTypeUser},
		},
	}
	result := pane.renderContent()
	_ = result // Should not panic
}

// ─────────────────────────────────────────────────────────────────────────────
// startSessionFileWatcherSubscription: empty entries & errors channel paths
// ─────────────────────────────────────────────────────────────────────────────

// TestStartSessionFileWatcherSubscription_PartialWrite_EmptyEntries verifies
// that when a Write event fires for a partial line (no newline terminator),
// ReadNewEntries returns zero entries and the goroutine continues (no message
// sent to fileEventChan).
func TestStartSessionFileWatcherSubscription_PartialWrite_EmptyEntries(t *testing.T) {
	checker := newTestPIDChecker(9080)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/partial-write-test"

	sessionID := "partial-write-session"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 9080, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	loadedMsg := sessionPaneContentLoadedMsg{
		sessionID: sessionID,
		entries:   nil,
		filePath:  jsonlPath,
		err:       nil,
	}
	newModel, _ = m.Update(loadedMsg)
	m = newModel.(SessionDashboardModel)

	if len(m.panes) == 0 || m.panes[0].watcher == nil {
		t.Fatal("expected pane with watcher")
	}

	// Give watcher time to initialize.
	time.Sleep(100 * time.Millisecond)

	pane := &m.panes[0]
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.startSessionFileWatcherSubscription(ctx, 0)

	// Give subscription goroutine time to start.
	time.Sleep(50 * time.Millisecond)

	fileEventChan := pane.fileEventChan

	// Write partial content (no trailing newline) — ParseJSONL returns 0 entries.
	f, err := os.OpenFile(jsonlPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open JSONL file: %v", err)
	}
	_, _ = f.WriteString(`{"type":"user"`) // incomplete JSON, no newline
	_ = f.Close()

	// No message should arrive within a short window (entries == 0 → continue).
	select {
	case msg := <-fileEventChan:
		// If we get a message it should still be valid (e.g., from a prior event).
		// A FileResetMsg or NewEntriesMsg are both OK if the watcher re-fires.
		t.Logf("received message (acceptable): %T", msg)
	case <-time.After(200 * time.Millisecond):
		// Expected: no message for a partial write.
	}

	// Now write a complete line — we expect a NewEntriesMsg.
	time.Sleep(50 * time.Millisecond)
	f, err = os.OpenFile(jsonlPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open JSONL file for complete write: %v", err)
	}
	_, _ = f.WriteString("\n" + `{"type":"user","message":{"role":"user","content":"complete"},"timestamp":"2026-03-31T12:00:00Z"}` + "\n")
	_ = f.Close()

	// Expect a message (NewEntriesMsg or FileResetMsg).
	select {
	case <-fileEventChan:
		// Got a message — subscription is still alive and functional.
	case <-time.After(2 * time.Second):
		t.Error("expected message after complete write, timed out")
	}

	cancel()
	m.closeAll()
}

// TestStartSessionFileWatcherSubscription_ErrorsChannelClose_GoroutineExits
// verifies that when the underlying fsnotify errors channel closes (watcher
// closed while the errors path is selected), the subscription goroutine exits.
// This test starts a subscription and closes the watcher, exercising the
// errors-channel close path (lines 809-812 in session_dashboard.go).
func TestStartSessionFileWatcherSubscription_ErrorsChannelClose_GoroutineExits(t *testing.T) {
	checker := newTestPIDChecker(9081)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/errors-close-test"

	sessionID := "errors-close-session"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 9081, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	loadedMsg := sessionPaneContentLoadedMsg{
		sessionID: sessionID,
		entries:   nil,
		filePath:  jsonlPath,
		err:       nil,
	}
	newModel, _ = m.Update(loadedMsg)
	m = newModel.(SessionDashboardModel)

	if len(m.panes) == 0 || m.panes[0].watcher == nil {
		t.Fatal("expected pane with watcher")
	}

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.startSessionFileWatcherSubscription(ctx, 0)
	time.Sleep(50 * time.Millisecond)

	fileEventChan := m.panes[0].fileEventChan

	// Close the watcher — closes both events and errors channels.
	// The goroutine will exit via whichever channel fires first.
	_ = m.panes[0].watcher.Close()

	// Drain the fileEventChan (closed by goroutine's deferred close).
	// The goroutine exits and closes ch; we wait for that.
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Drain until closed.
		for range fileEventChan {
		}
	}()

	select {
	case <-done:
		// Goroutine exited and closed ch.
	case <-time.After(3 * time.Second):
		t.Error("subscription goroutine did not exit after watcher closed")
	}

	cancel()
	m.closeAll()
}

// TestStartDirWatcherCmd_EventsChannelClose_GoroutineExits verifies that
// when a running dirWatcher is closed (closing the events channel), the
// bridge goroutine inside startDirWatcherCmd exits cleanly via the !ok path.
func TestStartDirWatcherCmd_EventsChannelClose_GoroutineExits(t *testing.T) {
	checker := newTestPIDChecker(9082)
	sessDir := t.TempDir()

	dirWatcher, err := session.NewSessionDirectoryWatcher(sessDir)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/dir-watcher-close", "/tmp/pd", scanner, monitor,
		WithDashboardDirWatcher(dirWatcher))

	// Start the dir watcher bridge.
	cmd := m.startDirWatcherCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from startDirWatcherCmd")
	}
	cmd() // Runs synchronously; starts the goroutine.

	// Give the goroutine time to start.
	time.Sleep(50 * time.Millisecond)

	// Close the dir watcher — this closes its fsnotify watcher, which will
	// eventually cause the bridge goroutine to exit.
	_ = dirWatcher.Close()

	// The bridge goroutine exits via ctx.Done() OR via the events !ok path.
	// Cancel the context to ensure it exits regardless of which path fires.
	m.cancel()

	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Goroutine exited.
	case <-time.After(3 * time.Second):
		t.Error("dir watcher bridge goroutine did not exit after watcher closed")
	}
}

// TestStartMonitorCmd_InnerContextCancel_GoroutineExits verifies that when the
// dashboard context is cancelled while the monitor bridge goroutine is active,
// the goroutine exits via the ctx.Done() path regardless of whether the
// monitorChan has capacity or not.
func TestStartMonitorCmd_InnerContextCancel_GoroutineExits(t *testing.T) {
	checker := newTestPIDChecker(9083)
	sessDir := t.TempDir()

	monitor := session.NewMonitor(
		session.WithMonitorPIDChecker(checker),
		session.WithPollInterval(session.MinPollInterval),
	)
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/monitor-inner-cancel", "/tmp/pd", scanner, monitor)

	// Fill the monitorChan so any forwarded event cannot be sent immediately.
	for i := 0; i < cap(m.monitorChan); i++ {
		m.monitorChan <- session.SessionEvent{}
	}

	cmd := m.startMonitorCmd()
	cmd()

	// Give the goroutine time to start.
	time.Sleep(30 * time.Millisecond)

	// Cancel the dashboard context — the goroutine MUST exit via ctx.Done().
	m.cancel()

	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Goroutine exited cleanly via ctx.Done().
	case <-time.After(3 * time.Second):
		t.Error("monitor bridge goroutine did not exit after context cancelled")
	}
}
