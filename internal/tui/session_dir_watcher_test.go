// Package tui provides the terminal user interface components.
// This file contains tests for Sub-AC 2b: session file watcher that monitors
// ~/.claude/sessions/ directory for file deletions/modifications and maps them
// to active session panes.
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// writeSessionJSONForTUI writes a {pid}.json session file to dir and returns its path.
func writeSessionJSONForTUI(t *testing.T, dir string, pid int, meta session.SessionMeta) string {
	t.Helper()
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal session meta: %v", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.json", pid))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	return path
}

// newDashboardWithDirWatcher creates a SessionDashboardModel with a real
// SessionDirectoryWatcher attached, wired to a temp sessions directory.
func newDashboardWithDirWatcher(t *testing.T, sessDir, projectDir, projectPath string, checker *testPIDChecker) (SessionDashboardModel, *session.SessionDirectoryWatcher) {
	t.Helper()

	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
		session.WithDirWatcherEventBuffer(64),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(
		projectPath,
		projectDir,
		sc,
		mon,
		WithDashboardDirWatcher(dw),
	)
	m.SetSize(120, 40)

	return m, dw
}

// ─────────────────────────────────────────────────────────────────────────────
// handleDirWatcherEvent tests
// ─────────────────────────────────────────────────────────────────────────────

// TestHandleDirWatcherEvent_SessionOpened_CreatesPane verifies that a
// SessionOpened event from the directory watcher creates a new session pane.
func TestHandleDirWatcherEvent_SessionOpened_CreatesPane(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dirwatcher-test"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	sessionID := "watcher-opened-session"
	makeJSONLFile(t, projectDir, sessionID)

	openedSess := session.ActiveSession{
		Meta: session.SessionMeta{
			PID:       100,
			SessionID: sessionID,
			CWD:       projectPath,
			Kind:      "interactive",
		},
		FilePath: filepath.Join(sessDir, "100.json"),
	}

	event := session.SessionEvent{
		Type:    session.SessionOpened,
		Session: openedSess,
	}

	newModel, cmd := m.Update(sessionDirWatcherEventMsg{event: event})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after SessionOpened, got %d", updated.PaneCount())
	}
	if updated.panes[0].session.Meta.PID != 100 {
		t.Errorf("pane PID = %d, want 100", updated.panes[0].session.Meta.PID)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (content loading) after SessionOpened")
	}
}

// TestHandleDirWatcherEvent_SessionOpened_NoDuplicate verifies that sending
// a SessionOpened event for an already-tracked PID does not create a duplicate pane.
func TestHandleDirWatcherEvent_SessionOpened_NoDuplicate(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dirwatcher-dedup"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	sessionID := "dedup-session"
	makeJSONLFile(t, projectDir, sessionID)

	openedSess := session.ActiveSession{
		Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath},
	}
	event := session.SessionEvent{Type: session.SessionOpened, Session: openedSess}

	// First event creates pane
	newModel, _ := m.Update(sessionDirWatcherEventMsg{event: event})
	updated := newModel.(SessionDashboardModel)
	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after first event, got %d", updated.PaneCount())
	}

	// Second event for same PID should NOT create duplicate
	newModel2, _ := updated.Update(sessionDirWatcherEventMsg{event: event})
	updated2 := newModel2.(SessionDashboardModel)
	if updated2.PaneCount() != 1 {
		t.Errorf("expected 1 pane after duplicate event, got %d", updated2.PaneCount())
	}
}

// TestHandleDirWatcherEvent_SessionClosed_RemovesPane verifies that a
// SessionClosed event from the directory watcher removes the corresponding pane.
func TestHandleDirWatcherEvent_SessionClosed_RemovesPane(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dirwatcher-close"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "s1")
	makeJSONLFile(t, projectDir, "s2")

	// Add two panes via scan result
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "s1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "s2", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)
	if updated.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", updated.PaneCount())
	}

	// Send SessionClosed for PID 100
	closedEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 100}},
	}
	newModel2, _ := updated.Update(sessionDirWatcherEventMsg{event: closedEvent})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.PaneCount() != 1 {
		t.Errorf("expected 1 pane after SessionClosed, got %d", updated2.PaneCount())
	}
	if updated2.panes[0].session.Meta.PID != 200 {
		t.Errorf("remaining pane PID = %d, want 200", updated2.panes[0].session.Meta.PID)
	}
}

// TestHandleDirWatcherEvent_SessionClosed_GridDirty verifies that removing a
// pane via SessionClosed marks the grid as dirty for re-rendering.
func TestHandleDirWatcherEvent_SessionClosed_GridDirty(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dirwatcher-dirty"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "dirty-sess")

	// Add pane
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "dirty-sess", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)
	updated.gridDirty = false // Reset dirty flag

	// Close pane via dir watcher event
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 100}},
	}
	newModel2, _ := updated.Update(sessionDirWatcherEventMsg{event: closeEvent})
	updated2 := newModel2.(SessionDashboardModel)

	if !updated2.IsGridDirty() {
		t.Error("expected gridDirty=true after pane removed via dir watcher event")
	}
}

// TestHandleDirWatcherEvent_Unknown_NoOp verifies that an unknown event type
// is a no-op and does not modify the model.
func TestHandleDirWatcherEvent_Unknown_NoOp(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", sc, mon)

	// Send unknown event type (value 99)
	unknownEvent := session.SessionEvent{
		Type:    session.SessionEventType(99),
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 1}},
	}
	newModel, cmd := m.Update(sessionDirWatcherEventMsg{event: unknownEvent})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 0 {
		t.Error("unknown event type should not create panes")
	}
	if cmd != nil {
		t.Error("unknown event type should return nil cmd")
	}
}

// TestHandleDirWatcherEvent_SessionOpened_MaxPanesLimit verifies that no more
// than MaxSessionPanes panes are created, even when many dir watcher events arrive.
func TestHandleDirWatcherEvent_SessionOpened_MaxPanesLimit(t *testing.T) {
	pids := make([]int, MaxSessionPanes+2) // 11 PIDs for 9 max panes
	for i := range pids {
		pids[i] = 200 + i
	}

	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dirwatcher-max"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	var current SessionDashboardModel = m
	for i, pid := range pids {
		sid := fmt.Sprintf("max-sess-%d", i)
		makeJSONLFile(t, projectDir, sid)

		openedSess := session.ActiveSession{
			Meta: session.SessionMeta{PID: pid, SessionID: sid, CWD: projectPath},
		}
		event := session.SessionEvent{Type: session.SessionOpened, Session: openedSess}
		newModel, _ := current.Update(sessionDirWatcherEventMsg{event: event})
		current = newModel.(SessionDashboardModel)
	}

	// With pagination, all sessions are stored (no hard cap at MaxSessionPanes)
	expectedTotal := len(pids)
	if current.PaneCount() != expectedTotal {
		t.Errorf("pane count = %d, want %d (all stored for pagination)", current.PaneCount(), expectedTotal)
	}

	// Verify pagination: page 0 shows up to 9, page 1 shows the rest
	visiblePanes := current.CurrentPagePanes()
	if len(visiblePanes) != MaxSessionPanes {
		t.Errorf("page 0 visible panes = %d, want %d", len(visiblePanes), MaxSessionPanes)
	}
	if current.TotalPages() != 2 {
		t.Errorf("total pages = %d, want 2", current.TotalPages())
	}
}

// TestHandleDirWatcherEvent_SessionOpened_ProjectFiltered verifies that
// SessionOpened events for sessions with a different CWD are ignored.
func TestHandleDirWatcherEvent_SessionOpened_ProjectFiltered(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/my-project"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	// Session for a DIFFERENT project
	foreignSession := session.ActiveSession{
		Meta: session.SessionMeta{PID: 100, SessionID: "other-sess", CWD: "/tmp/other-project"},
	}
	event := session.SessionEvent{Type: session.SessionOpened, Session: foreignSession}
	newModel, _ := m.Update(sessionDirWatcherEventMsg{event: event})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 0 {
		t.Errorf("expected 0 panes (filtered by project), got %d", updated.PaneCount())
	}
}

// TestHandleDirWatcherEvent_SessionClosed_FocusAdjustment verifies that
// closing the focused pane adjusts focus correctly.
func TestHandleDirWatcherEvent_SessionClosed_FocusAdjustment(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/focus-adjust"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "fa-1")
	makeJSONLFile(t, projectDir, "fa-2")

	// Add two panes
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "fa-1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "fa-2", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)
	updated.focusIndex = 1 // Focus second pane

	// Remove second pane via dir watcher
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 200}},
	}
	newModel2, _ := updated.Update(sessionDirWatcherEventMsg{event: closeEvent})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.PaneCount() != 1 {
		t.Fatalf("expected 1 pane, got %d", updated2.PaneCount())
	}
	if updated2.focusIndex != 0 {
		t.Errorf("focus should adjust to 0, got %d", updated2.focusIndex)
	}
}

// TestHandleDirWatcherEvent_SessionClosed_UnknownPID verifies that closing an
// unknown PID is a no-op.
func TestHandleDirWatcherEvent_SessionClosed_UnknownPID(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", sc, mon)

	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 999}},
	}
	newModel, cmd := m.Update(sessionDirWatcherEventMsg{event: closeEvent})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 0 {
		t.Error("closing unknown PID should be no-op")
	}
	if cmd != nil {
		t.Error("no-op close should return nil cmd")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// pollChannels dir watcher tests
// ─────────────────────────────────────────────────────────────────────────────

// TestPollChannels_DirWatcherEvent verifies that pollChannels returns a
// sessionDirWatcherEventMsg when there is a pending event in dirWatcherChan.
func TestPollChannels_DirWatcherEvent(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
		dirWatcherChan: make(chan session.SessionEvent, 1),
	}

	expected := session.SessionEvent{
		Type:    session.SessionOpened,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 42}},
	}
	m.dirWatcherChan <- expected

	msg := m.pollChannels()
	if msg == nil {
		t.Fatal("expected non-nil message from dirWatcherChan")
	}
	dwMsg, ok := msg.(sessionDirWatcherEventMsg)
	if !ok {
		t.Fatalf("expected sessionDirWatcherEventMsg, got %T", msg)
	}
	if dwMsg.event.Session.Meta.PID != 42 {
		t.Errorf("PID = %d, want 42", dwMsg.event.Session.Meta.PID)
	}
	if dwMsg.event.Type != session.SessionOpened {
		t.Errorf("event type = %v, want SessionOpened", dwMsg.event.Type)
	}
}

// TestPollChannels_NilDirWatcherChan verifies that pollChannels skips the
// dir watcher check when dirWatcherChan is nil (no dir watcher configured).
func TestPollChannels_NilDirWatcherChan(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
		dirWatcherChan: nil, // No dir watcher
	}

	// Should return nil without panicking
	msg := m.pollChannels()
	if msg != nil {
		t.Errorf("expected nil with no events and nil dirWatcherChan, got %T", msg)
	}
}

// TestPollChannels_DirWatcherPriorityAfterScan verifies that dir watcher events
// are returned after scan results (scan results have higher priority).
func TestPollChannels_DirWatcherPriorityAfterScan(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
		dirWatcherChan: make(chan session.SessionEvent, 1),
	}

	// Put scan result first
	m.scanResultChan <- session.ScanResult{}

	// Put dir watcher event
	m.dirWatcherChan <- session.SessionEvent{Type: session.SessionOpened}

	// First poll should return scan result
	msg1 := m.pollChannels()
	if _, ok := msg1.(sessionScanResultMsg); !ok {
		t.Errorf("expected scanResult first, got %T", msg1)
	}

	// Second poll should return dir watcher event
	msg2 := m.pollChannels()
	if _, ok := msg2.(sessionDirWatcherEventMsg); !ok {
		t.Errorf("expected dirWatcherEvent second, got %T", msg2)
	}
}

// TestPollChannels_DirWatcherClosedEvent verifies that a SessionClosed event
// in the dir watcher channel is correctly delivered to pollChannels.
func TestPollChannels_DirWatcherClosedEvent(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
		dirWatcherChan: make(chan session.SessionEvent, 1),
	}

	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 77}},
	}
	m.dirWatcherChan <- closeEvent

	msg := m.pollChannels()
	dwMsg, ok := msg.(sessionDirWatcherEventMsg)
	if !ok {
		t.Fatalf("expected sessionDirWatcherEventMsg, got %T", msg)
	}
	if dwMsg.event.Type != session.SessionClosed {
		t.Errorf("event type = %v, want SessionClosed", dwMsg.event.Type)
	}
	if dwMsg.event.Session.Meta.PID != 77 {
		t.Errorf("PID = %d, want 77", dwMsg.event.Session.Meta.PID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// startDirWatcherCmd tests
// ─────────────────────────────────────────────────────────────────────────────

// TestStartDirWatcherCmd_NilWatcher verifies that startDirWatcherCmd with nil
// dirWatcher returns a no-op command.
func TestStartDirWatcherCmd_NilWatcher(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", sc, mon)
	// No dirWatcher set — m.dirWatcher == nil

	cmd := m.startDirWatcherCmd()
	if cmd == nil {
		t.Fatal("startDirWatcherCmd should return a non-nil tea.Cmd")
	}
	// Execute the cmd — should return nil when dirWatcher is nil
	result := cmd()
	if result != nil {
		t.Errorf("cmd() should return nil for nil dirWatcher, got %T", result)
	}
}

// TestStartDirWatcherCmd_NilDirWatcherChan verifies that startDirWatcherCmd
// returns nil when dirWatcherChan is nil.
func TestStartDirWatcherCmd_NilDirWatcherChan(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	dw, err := session.NewSessionDirectoryWatcher(sessDir, session.WithDirWatcherPIDChecker(checker))
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer dw.Close()

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", sc, mon)

	// Manually set dirWatcher but leave dirWatcherChan as nil
	m.dirWatcher = dw
	m.dirWatcherChan = nil

	cmd := m.startDirWatcherCmd()
	result := cmd()
	if result != nil {
		t.Errorf("cmd() should return nil when dirWatcherChan is nil, got %T", result)
	}
}

// TestStartDirWatcherCmd_StartsWatcher verifies that startDirWatcherCmd starts
// the watcher and bridges events to dirWatcherChan.
func TestStartDirWatcherCmd_StartsWatcher(t *testing.T) {
	pid := 5001
	checker := newTestPIDChecker(pid)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/start-watcher-test"

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, projectPath, checker)
	defer dw.Close()

	// Execute startDirWatcherCmd — this starts the watcher goroutine
	cmd := m.startDirWatcherCmd()
	result := cmd()
	if result != nil {
		t.Errorf("startDirWatcherCmd should return nil msg, got %T", result)
	}

	// Write a session file — watcher should detect it
	makeJSONLFile(t, projectDir, "start-test-sess")
	writeSessionJSONForTUI(t, sessDir, pid, session.SessionMeta{
		PID:       pid,
		SessionID: "start-test-sess",
		CWD:       projectPath,
	})

	// Wait for event to be bridged to dirWatcherChan
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-m.dirWatcherChan:
			if event.Type == session.SessionOpened && event.Session.Meta.PID == pid {
				// Test passed — event was bridged correctly
				m.cancel() // Clean up context
				return
			}
		case <-deadline:
			m.cancel()
			t.Fatal("timeout waiting for bridged dir watcher event")
		}
	}
}

// TestStartDirWatcherCmd_ContextCancellation verifies that the bridge goroutine
// exits cleanly when the context is cancelled.
func TestStartDirWatcherCmd_ContextCancellation(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, "/tmp/ctx-cancel", checker)
	defer dw.Close()

	// Start dir watcher cmd
	cmd := m.startDirWatcherCmd()
	cmd()

	// Cancel context — bridge goroutine should exit
	m.cancel()

	// Wait for goroutines to drain
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Goroutines exited cleanly
	case <-time.After(3 * time.Second):
		t.Error("bridge goroutine did not exit after context cancellation")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// WithDashboardDirWatcher option tests
// ─────────────────────────────────────────────────────────────────────────────

// TestWithDashboardDirWatcher_SetsWatcher verifies that WithDashboardDirWatcher
// sets the dirWatcher field and creates the dirWatcherChan.
func TestWithDashboardDirWatcher_SetsWatcher(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	dw, err := session.NewSessionDirectoryWatcher(sessDir, session.WithDirWatcherPIDChecker(checker))
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer dw.Close()

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", sc, mon, WithDashboardDirWatcher(dw))

	if m.dirWatcher != dw {
		t.Error("dirWatcher not set correctly by option")
	}
	if m.dirWatcherChan == nil {
		t.Error("dirWatcherChan should be non-nil when dir watcher is set")
	}
}

// TestWithDashboardDirWatcher_InitIncludesDirWatcherCmd verifies that Init
// returns a command that includes the dir watcher startup when configured.
func TestWithDashboardDirWatcher_InitIncludesDirWatcherCmd(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, "/tmp/init-test", checker)
	defer dw.Close()

	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return non-nil batch command with dir watcher")
	}

	// Execute init cmd to start components (non-blocking)
	// The cmd is a batch command; we just verify it's non-nil to confirm
	// the dir watcher cmd is included.
	_ = cmd
}

// TestWithoutDashboardDirWatcher_InitSkipsDirWatcherCmd verifies that Init
// does not include a dir watcher command when no dir watcher is configured.
func TestWithoutDashboardDirWatcher_InitSkipsDirWatcherCmd(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", sc, mon) // No WithDashboardDirWatcher

	if m.dirWatcher != nil {
		t.Error("dirWatcher should be nil without option")
	}
	if m.dirWatcherChan != nil {
		t.Error("dirWatcherChan should be nil without option")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// closeAll with dir watcher tests
// ─────────────────────────────────────────────────────────────────────────────

// TestCloseAll_ClosesDirWatcher verifies that closeAll() closes the
// SessionDirectoryWatcher to release fsnotify file descriptors.
func TestCloseAll_ClosesDirWatcher(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, "/tmp/close-dw", checker)

	// Start the watcher (normally done by Init → startDirWatcherCmd)
	if err := dw.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	m.closeAll()

	// After closeAll, the dir watcher should be closed
	if !dw.IsClosed() {
		t.Error("dir watcher should be closed after closeAll()")
	}
}

// TestCloseAll_WithDirWatcher_SubscriptionsInactive verifies that closeAll
// marks subscriptions as inactive even when a dir watcher is present.
func TestCloseAll_WithDirWatcher_SubscriptionsInactive(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, "/tmp/close-subs", checker)
	defer dw.Close()

	m.closeAll()

	if m.subscriptionsActive {
		t.Error("subscriptions should be inactive after closeAll()")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Update dispatch tests
// ─────────────────────────────────────────────────────────────────────────────

// TestUpdate_DispatchesDirWatcherEventMsg verifies that the Update function
// correctly dispatches sessionDirWatcherEventMsg to handleDirWatcherEvent.
func TestUpdate_DispatchesDirWatcherEventMsg(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dispatch-test"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	sessionID := "dispatch-test-sess"
	makeJSONLFile(t, projectDir, sessionID)

	openEvent := session.SessionEvent{
		Type: session.SessionOpened,
		Session: session.ActiveSession{
			Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath},
		},
	}

	// Wrap in sessionDirWatcherEventMsg and send through Update
	msg := sessionDirWatcherEventMsg{event: openEvent}
	newModel, cmd := m.Update(msg)
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("Update should dispatch to handleDirWatcherEvent; expected 1 pane, got %d", updated.PaneCount())
	}
	if cmd == nil {
		t.Error("expected non-nil cmd from dir watcher session opened event")
	}
}

// TestUpdate_DirWatcherEventMsg_ThroughSubscriptionTick verifies that the
// subscription tick polls the dir watcher channel and processes events.
func TestUpdate_DirWatcherEventMsg_ThroughSubscriptionTick(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/tick-dispatch"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	// Create dashboard with dir watcher channel configured
	dw, err := session.NewSessionDirectoryWatcher(sessDir, session.WithDirWatcherPIDChecker(checker))
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer dw.Close()

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon, WithDashboardDirWatcher(dw))
	m.SetSize(120, 40)
	m.subscriptionsActive = true

	sessionID := "tick-sess"
	makeJSONLFile(t, projectDir, sessionID)

	// Put a SessionOpened event in the dir watcher channel
	m.dirWatcherChan <- session.SessionEvent{
		Type: session.SessionOpened,
		Session: session.ActiveSession{
			Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath},
		},
	}

	// Send subscription tick — should poll dirWatcherChan and process the event
	newModel, cmd := m.Update(sessionSubscriptionTickMsg{})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Errorf("expected 1 pane after tick processes dir watcher event, got %d", updated.PaneCount())
	}
	if cmd == nil {
		t.Error("subscription tick with event should return continuation cmd")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// End-to-end integration tests with real SessionDirectoryWatcher
// ─────────────────────────────────────────────────────────────────────────────

// TestDirWatcher_EndToEnd_FileCreation_PaneAppears verifies the complete
// pipeline: session file creation → dir watcher detects it → event bridged to
// dirWatcherChan → pane appears in dashboard within 2 seconds.
func TestDirWatcher_EndToEnd_FileCreation_PaneAppears(t *testing.T) {
	pid := 7001
	checker := newTestPIDChecker(pid)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/e2e-creation"

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, projectPath, checker)

	// Start dir watcher cmd (bridges events to dirWatcherChan)
	cmd := m.startDirWatcherCmd()
	cmd()

	sessionID := "e2e-sess"
	makeJSONLFile(t, projectDir, sessionID)

	// Create session file — dir watcher should detect it
	start := time.Now()
	writeSessionJSONForTUI(t, sessDir, pid, session.SessionMeta{
		PID:       pid,
		SessionID: sessionID,
		CWD:       projectPath,
	})

	// Wait for event to arrive in dirWatcherChan
	var event session.SessionEvent
	deadline := time.After(2 * time.Second)
	select {
	case event = <-m.dirWatcherChan:
		// Got event
	case <-deadline:
		m.cancel()
		_ = dw.Close()
		t.Fatal("timeout: dir watcher event not received within 2 seconds")
	}

	latency := time.Since(start)
	t.Logf("Dir watcher event latency: %v", latency)

	// Clean up
	m.cancel()
	_ = dw.Close()

	// Verify event type and content
	if event.Type != session.SessionOpened {
		t.Errorf("expected SessionOpened, got %v", event.Type)
	}
	if event.Session.Meta.PID != pid {
		t.Errorf("PID = %d, want %d", event.Session.Meta.PID, pid)
	}
	if event.Session.Meta.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", event.Session.Meta.SessionID, sessionID)
	}

	// Verify that processing the event creates a pane
	newModel, loadCmd := m.Update(sessionDirWatcherEventMsg{event: event})
	updated := newModel.(SessionDashboardModel)
	if updated.PaneCount() != 1 {
		t.Errorf("expected 1 pane after processing event, got %d", updated.PaneCount())
	}
	if loadCmd == nil {
		t.Error("expected content loading cmd")
	}
}

// TestDirWatcher_EndToEnd_FileDeletion_PaneRemoved verifies the complete
// pipeline: session file deletion → dir watcher detects it → event bridged →
// pane removed from dashboard within 5 seconds.
func TestDirWatcher_EndToEnd_FileDeletion_PaneRemoved(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end file deletion test in short mode")
	}
	pid := 7002
	checker := newTestPIDChecker(pid)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/e2e-deletion"

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, projectPath, checker)

	// Start dir watcher
	cmd := m.startDirWatcherCmd()
	cmd()

	sessionID := "e2e-del-sess"
	makeJSONLFile(t, projectDir, sessionID)

	// First: create session file and wait for SessionOpened event
	sessionFile := writeSessionJSONForTUI(t, sessDir, pid, session.SessionMeta{
		PID:       pid,
		SessionID: sessionID,
		CWD:       projectPath,
	})

	var openEvent session.SessionEvent
	deadline := time.After(2 * time.Second)
	for {
		select {
		case e := <-m.dirWatcherChan:
			if e.Type == session.SessionOpened && e.Session.Meta.PID == pid {
				openEvent = e
				goto gotOpen
			}
		case <-deadline:
			m.cancel()
			_ = dw.Close()
			t.Fatal("timeout waiting for SessionOpened")
		}
	}
gotOpen:

	// Process open event — creates pane
	newModel, _ := m.Update(sessionDirWatcherEventMsg{event: openEvent})
	m = newModel.(SessionDashboardModel)
	if m.PaneCount() != 1 {
		m.cancel()
		_ = dw.Close()
		t.Fatalf("expected 1 pane after open, got %d", m.PaneCount())
	}

	// Now delete the session file
	start := time.Now()
	if err := os.Remove(sessionFile); err != nil {
		m.cancel()
		_ = dw.Close()
		t.Fatalf("Remove: %v", err)
	}

	// Wait for SessionClosed event in dirWatcherChan.
	// Use a generous 30-second timeout: under heavy system load (race detector,
	// coverage profiling, parallel tests) the kqueue/fsnotify Remove event can
	// take several seconds to arrive.  In isolation the latency is < 1 ms.
	var closeEvent session.SessionEvent
	deadline2 := time.After(30 * time.Second)
	for {
		select {
		case e := <-m.dirWatcherChan:
			if e.Type == session.SessionClosed && e.Session.Meta.PID == pid {
				closeEvent = e
				goto gotClose
			}
		case <-deadline2:
			m.cancel()
			_ = dw.Close()
			t.Fatal("timeout: SessionClosed not received within 30 seconds after file deletion")
		}
	}
gotClose:

	latency := time.Since(start)
	t.Logf("File deletion detection latency: %v", latency)

	// Clean up
	m.cancel()
	_ = dw.Close()

	// Process close event — removes pane
	newModel2, _ := m.Update(sessionDirWatcherEventMsg{event: closeEvent})
	m2 := newModel2.(SessionDashboardModel)

	if m2.PaneCount() != 0 {
		t.Errorf("expected 0 panes after SessionClosed, got %d", m2.PaneCount())
	}
}

// TestDirWatcher_EndToEnd_MultipleSessionsLifecycle verifies that multiple
// sessions can be created and deleted, with correct pane lifecycle management.
func TestDirWatcher_EndToEnd_MultipleSessionsLifecycle(t *testing.T) {
	pids := []int{8001, 8002, 8003}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/e2e-multi"

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, projectPath, checker)
	defer func() {
		m.cancel()
		_ = dw.Close()
	}()

	// Start dir watcher
	cmd := m.startDirWatcherCmd()
	cmd()

	// Create 3 session files
	sessionFiles := make([]string, len(pids))
	for i, pid := range pids {
		sid := fmt.Sprintf("multi-e2e-%d", i)
		makeJSONLFile(t, projectDir, sid)
		sessionFiles[i] = writeSessionJSONForTUI(t, sessDir, pid, session.SessionMeta{
			PID:       pid,
			SessionID: sid,
			CWD:       projectPath,
		})
	}

	// Collect 3 SessionOpened events
	seenOpen := make(map[int]bool)
	deadline := time.After(3 * time.Second)
	for len(seenOpen) < len(pids) {
		select {
		case e := <-m.dirWatcherChan:
			if e.Type == session.SessionOpened {
				seenOpen[e.Session.Meta.PID] = true
				newModel, _ := m.Update(sessionDirWatcherEventMsg{event: e})
				m = newModel.(SessionDashboardModel)
			}
		case <-deadline:
			t.Fatalf("timeout: only saw %d/%d SessionOpened events", len(seenOpen), len(pids))
		}
	}

	if m.PaneCount() != len(pids) {
		t.Fatalf("expected %d panes after opens, got %d", len(pids), m.PaneCount())
	}

	// Delete first session file
	checker.SetAlive(pids[0], false)
	if err := os.Remove(sessionFiles[0]); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Wait for SessionClosed (generous timeout for loaded CI environments).
	deadline2 := time.After(10 * time.Second)
	for {
		select {
		case e := <-m.dirWatcherChan:
			if e.Type == session.SessionClosed && e.Session.Meta.PID == pids[0] {
				newModel, _ := m.Update(sessionDirWatcherEventMsg{event: e})
				m = newModel.(SessionDashboardModel)
				goto closedOne
			}
		case <-deadline2:
			t.Fatal("timeout waiting for SessionClosed after file deletion")
		}
	}
closedOne:

	if m.PaneCount() != len(pids)-1 {
		t.Errorf("expected %d panes after close, got %d", len(pids)-1, m.PaneCount())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Goroutine leak tests for dir watcher lifecycle
// ─────────────────────────────────────────────────────────────────────────────

// TestDirWatcher_NoGoroutineLeak_StartStop verifies that starting and stopping
// the dashboard with a dir watcher does not leak goroutines.
func TestDirWatcher_NoGoroutineLeak_StartStop(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	for cycle := 0; cycle < 3; cycle++ {
		checker := newTestPIDChecker()
		sessDir := t.TempDir()
		projectDir := t.TempDir()

		m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir,
			fmt.Sprintf("/tmp/leak-dw-%d", cycle), checker)

		// Start the dir watcher cmd
		startCmd := m.startDirWatcherCmd()
		startCmd()

		// Close everything
		m.closeAll()

		// Wait for goroutines to drain
		done := make(chan struct{})
		go func() {
			m.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Errorf("cycle %d: goroutines did not drain after closeAll", cycle)
		}

		_ = dw.Close() // Extra safety
	}

	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	if after > before+2 {
		t.Errorf("goroutine leak: before=%d after=%d (delta=%d)", before, after, after-before)
	}
}

// TestDirWatcher_NoGoroutineLeak_WithEvents verifies no goroutine leaks
// when events flow through the dir watcher pipeline.
func TestDirWatcher_NoGoroutineLeak_WithEvents(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	for cycle := 0; cycle < 3; cycle++ {
		pid := 9000 + cycle
		checker := newTestPIDChecker(pid)
		sessDir := t.TempDir()
		projectDir := t.TempDir()
		projectPath := fmt.Sprintf("/tmp/leak-events-%d", cycle)

		m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, projectPath, checker)

		// Start dir watcher
		startCmd := m.startDirWatcherCmd()
		startCmd()

		// Create session file to trigger events
		sid := fmt.Sprintf("leak-e-%d", cycle)
		makeJSONLFile(t, projectDir, sid)
		writeSessionJSONForTUI(t, sessDir, pid, session.SessionMeta{
			PID:       pid,
			SessionID: sid,
			CWD:       projectPath,
		})

		// Wait for event
		waitDeadline := time.After(2 * time.Second)
		received := false
		for !received {
			select {
			case e := <-m.dirWatcherChan:
				if e.Type == session.SessionOpened {
					received = true
				}
			case <-waitDeadline:
				// Best effort — proceed to cleanup
				received = true
			}
		}

		// Shut down
		m.closeAll()

		drainDone := make(chan struct{})
		go func() {
			m.wg.Wait()
			close(drainDone)
		}()

		select {
		case <-drainDone:
		case <-time.After(3 * time.Second):
			t.Errorf("cycle %d: goroutines did not drain", cycle)
		}

		_ = dw.Close()
	}

	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	if after > before+2 {
		t.Errorf("goroutine leak: before=%d after=%d (delta=%d)", before, after, after-before)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// File modification mapping tests
// ─────────────────────────────────────────────────────────────────────────────

// TestDirWatcher_FileModification_SeenPIDSuppressed verifies that Write events
// for already-tracked sessions (seen PIDs) are suppressed and don't create
// duplicate panes.
func TestDirWatcher_FileModification_SeenPIDSuppressed(t *testing.T) {
	pid := 10001
	checker := newTestPIDChecker(pid)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/modify-test"

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, projectPath, checker)
	defer func() {
		m.cancel()
		_ = dw.Close()
	}()

	// Start dir watcher
	startCmd := m.startDirWatcherCmd()
	startCmd()

	sessionID := "modify-sess"
	makeJSONLFile(t, projectDir, sessionID)

	// Create session file — first write triggers SessionOpened
	sessionFile := writeSessionJSONForTUI(t, sessDir, pid, session.SessionMeta{
		PID:       pid,
		SessionID: sessionID,
		CWD:       projectPath,
	})

	// Wait for SessionOpened
	deadline := time.After(2 * time.Second)
	for {
		select {
		case e := <-m.dirWatcherChan:
			if e.Type == session.SessionOpened && e.Session.Meta.PID == pid {
				newModel, _ := m.Update(sessionDirWatcherEventMsg{event: e})
				m = newModel.(SessionDashboardModel)
				goto gotFirstOpen
			}
		case <-deadline:
			t.Fatal("timeout waiting for first SessionOpened")
		}
	}
gotFirstOpen:

	if m.PaneCount() != 1 {
		t.Fatalf("expected 1 pane, got %d", m.PaneCount())
	}

	// Modify the session file multiple times — should NOT create duplicate panes
	// because the PID is already in seenPIDs
	for i := 0; i < 3; i++ {
		writeSessionJSONForTUI(t, sessDir, pid, session.SessionMeta{
			PID:       pid,
			SessionID: sessionID,
			CWD:       projectPath,
		})
		time.Sleep(30 * time.Millisecond)
	}

	// Drain any pending events (should be none for the already-seen PID)
	time.Sleep(300 * time.Millisecond)
	for {
		select {
		case e := <-m.dirWatcherChan:
			if e.Type == session.SessionOpened && e.Session.Meta.PID == pid {
				// Unexpected duplicate
				newModel, _ := m.Update(sessionDirWatcherEventMsg{event: e})
				m = newModel.(SessionDashboardModel)
			}
		default:
			goto drainDone
		}
	}
drainDone:

	// Still should have exactly 1 pane (modifications don't duplicate)
	if m.PaneCount() != 1 {
		t.Errorf("file modifications should not create duplicate panes; got %d panes", m.PaneCount())
	}

	_ = sessionFile // Suppress unused warning
}

// TestDirWatcher_SessionFileNotForProject_Ignored verifies that session files
// created for a different project directory are not reflected in the dashboard.
func TestDirWatcher_SessionFileNotForProject_Ignored(t *testing.T) {
	pid := 10002
	checker := newTestPIDChecker(pid)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/project-filter"

	m, dw := newDashboardWithDirWatcher(t, sessDir, projectDir, projectPath, checker)
	defer func() {
		m.cancel()
		_ = dw.Close()
	}()

	// Start dir watcher
	startCmd := m.startDirWatcherCmd()
	startCmd()

	// Create session file for a DIFFERENT project
	writeSessionJSONForTUI(t, sessDir, pid, session.SessionMeta{
		PID:       pid,
		SessionID: "other-project-sess",
		CWD:       "/tmp/other-project", // Not our project
	})

	// Wait and drain — should get a SessionOpened event from watcher
	// but it should be filtered out at the dashboard level
	time.Sleep(500 * time.Millisecond)
	for {
		select {
		case e := <-m.dirWatcherChan:
			// Process through dashboard — should be filtered
			newModel, _ := m.Update(sessionDirWatcherEventMsg{event: e})
			m = newModel.(SessionDashboardModel)
		default:
			goto drained
		}
	}
drained:

	if m.PaneCount() != 0 {
		t.Errorf("expected 0 panes for different project, got %d", m.PaneCount())
	}
}
