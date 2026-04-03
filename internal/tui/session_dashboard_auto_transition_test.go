package tui

import (
	"testing"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
)

// TestAutoTransition_GridToSingle_ViaScanResult verifies that when a scan result
// removes sessions from 2 to 1, the view mode automatically transitions from
// Grid to SingleSession without user intervention.
func TestAutoTransition_GridToSingle_ViaScanResult(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-grid-to-single"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")
	makeJSONLFile(t, projectDir, "sess-2")

	// First scan: 2 sessions → grid mode
	scan1 := session.ScanResult{
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	updated := newModel.(SessionDashboardModel)

	if updated.ViewMode() != DashboardViewGrid {
		t.Fatalf("expected grid mode with 2 sessions, got %v", updated.ViewMode())
	}
	if updated.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", updated.PaneCount())
	}

	// Second scan: PID 200 gone → 1 session → auto-transition to single-session mode
	// Grace period requires testScanMissThreshold consecutive misses before removal.
	checker.SetAlive(200, false)
	scan2 := session.ScanResult{
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	updated2 := applyScanResultNTimes(t, updated, sessionScanResultMsg{result: scan2}, testScanMissThreshold)

	if updated2.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after session removal, got %d", updated2.PaneCount())
	}
	if updated2.ViewMode() != DashboardViewSingleSession {
		t.Errorf("expected single-session mode after dropping from 2→1, got %v", updated2.ViewMode())
	}
}

// TestAutoTransition_GridToZero_ViaScanResult verifies that when a scan result
// removes all sessions, the view mode automatically transitions from Grid to
// ZeroSessions without user intervention.
func TestAutoTransition_GridToZero_ViaScanResult(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-grid-to-zero"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")
	makeJSONLFile(t, projectDir, "sess-2")

	// First scan: 2 sessions → grid mode
	scan1 := session.ScanResult{
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	updated := newModel.(SessionDashboardModel)

	if updated.ViewMode() != DashboardViewGrid {
		t.Fatalf("expected grid mode, got %v", updated.ViewMode())
	}

	// Second scan: both PIDs gone → 0 sessions → auto-transition to zero-session mode
	// Grace period requires testScanMissThreshold consecutive misses before removal.
	checker.SetAlive(100, false)
	checker.SetAlive(200, false)
	scan2 := session.ScanResult{
		IsFullScan: true,
		Sessions:   []session.ActiveSession{},
		ScanTime:   time.Now(),
	}
	updated2 := applyScanResultNTimes(t, updated, sessionScanResultMsg{result: scan2}, testScanMissThreshold)

	if updated2.PaneCount() != 0 {
		t.Fatalf("expected 0 panes after all sessions removed, got %d", updated2.PaneCount())
	}
	if updated2.ViewMode() != DashboardViewZeroSessions {
		t.Errorf("expected zero-session mode after dropping from 2→0, got %v", updated2.ViewMode())
	}
	if !updated2.LatestLoading() {
		t.Error("expected latestLoading=true during zero-session transition")
	}
}

// TestAutoTransition_SingleToZero_ViaSessionClosed verifies that when the last
// session closes, the view mode automatically transitions from SingleSession
// to ZeroSessions without user intervention.
func TestAutoTransition_SingleToZero_ViaSessionClosed(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-single-to-zero"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")

	// First scan: 1 session → single-session mode
	scan1 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	updated := newModel.(SessionDashboardModel)

	if updated.ViewMode() != DashboardViewSingleSession {
		t.Fatalf("expected single-session mode, got %v", updated.ViewMode())
	}

	// Session closes via sessionClosedMsg → auto-transition to zero-session mode
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1"}},
	}
	newModel2, _ := updated.Update(sessionClosedMsg{event: closeEvent})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.PaneCount() != 0 {
		t.Fatalf("expected 0 panes after last session closed, got %d", updated2.PaneCount())
	}
	if updated2.ViewMode() != DashboardViewZeroSessions {
		t.Errorf("expected zero-session mode after last session closed, got %v", updated2.ViewMode())
	}
}

// TestAutoTransition_ZeroToSingle_ViaScanResult verifies that when a new session
// appears while in zero-session mode, the view auto-transitions to single-session mode.
func TestAutoTransition_ZeroToSingle_ViaScanResult(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-zero-to-single"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Initially in zero-session mode (default)
	if m.ViewMode() != DashboardViewZeroSessions {
		t.Fatalf("expected initial zero-session mode, got %v", m.ViewMode())
	}

	// Scan detects 1 session → auto-transition to single-session mode
	makeJSONLFile(t, projectDir, "sess-1")
	scan1 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane, got %d", updated.PaneCount())
	}
	if updated.ViewMode() != DashboardViewSingleSession {
		t.Errorf("expected single-session mode, got %v", updated.ViewMode())
	}
	// Latest viewer should be cleaned up
	if updated.LatestViewer() != nil {
		t.Error("latestViewer should be nil after transition away from zero-session mode")
	}
}

// TestAutoTransition_SingleToGrid_ViaScanResult verifies that when a second session
// appears while in single-session mode, the view auto-transitions to grid mode.
func TestAutoTransition_SingleToGrid_ViaScanResult(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-single-to-grid"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")

	// First scan: 1 session → single-session mode
	scan1 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	updated := newModel.(SessionDashboardModel)

	if updated.ViewMode() != DashboardViewSingleSession {
		t.Fatalf("expected single-session mode, got %v", updated.ViewMode())
	}

	// Second scan: another session appears → auto-transition to grid mode
	makeJSONLFile(t, projectDir, "sess-2")
	scan2 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel2, _ := updated.Update(sessionScanResultMsg{result: scan2})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", updated2.PaneCount())
	}
	if updated2.ViewMode() != DashboardViewGrid {
		t.Errorf("expected grid mode, got %v", updated2.ViewMode())
	}
	// Single session viewer should be cleaned up
	if updated2.SingleSessionViewer() != nil {
		t.Error("singleSessionViewer should be nil after transition to grid mode")
	}
}

// TestAutoTransition_NoTransitionWhenModeUnchanged verifies that when the polling
// cycle delivers results that don't change the session count category, no transition
// occurs and state is preserved.
func TestAutoTransition_NoTransitionWhenModeUnchanged(t *testing.T) {
	checker := newTestPIDChecker(100, 200, 300)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-no-change"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")
	makeJSONLFile(t, projectDir, "sess-2")

	// First scan: 2 sessions → grid mode
	scan1 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	updated := newModel.(SessionDashboardModel)

	if updated.ViewMode() != DashboardViewGrid {
		t.Fatalf("expected grid mode, got %v", updated.ViewMode())
	}

	// Second scan: add a third session, still grid mode (2+ → 2+)
	makeJSONLFile(t, projectDir, "sess-3")
	scan2 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}, State: session.SessionActive},
			{Meta: session.SessionMeta{PID: 300, SessionID: "sess-3", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel2, _ := updated.Update(sessionScanResultMsg{result: scan2})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.ViewMode() != DashboardViewGrid {
		t.Errorf("expected grid mode to remain unchanged, got %v", updated2.ViewMode())
	}
	if updated2.PaneCount() != 3 {
		t.Errorf("expected 3 panes, got %d", updated2.PaneCount())
	}
}

// TestAutoTransition_DirWatcherEventTriggersTransition verifies that a dir watcher
// event (faster than polling cycle) also triggers view mode transitions.
func TestAutoTransition_DirWatcherEventTriggersTransition(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-dirwatcher"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Initially zero-session mode
	if m.ViewMode() != DashboardViewZeroSessions {
		t.Fatalf("expected initial zero-session mode, got %v", m.ViewMode())
	}

	// Dir watcher event adds a session → auto-transition to single-session mode
	makeJSONLFile(t, projectDir, "sess-1")
	dirEvent := sessionDirWatcherEventMsg{
		event: session.SessionEvent{
			Type: session.SessionOpened,
			Session: session.ActiveSession{
				Meta:  session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath},
				State: session.SessionActive,
			},
		},
	}
	newModel, _ := m.Update(dirEvent)
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after dir watcher event, got %d", updated.PaneCount())
	}
	if updated.ViewMode() != DashboardViewSingleSession {
		t.Errorf("expected single-session mode after dir watcher event, got %v", updated.ViewMode())
	}
}

// TestAutoTransition_DirWatcherCloseTriggersTransition verifies that a dir watcher
// session closed event triggers view mode transition from single to zero.
func TestAutoTransition_DirWatcherCloseTriggersTransition(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-dirwatcher-close"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")

	// First: add session via scan to get into single-session mode
	scan1 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	updated := newModel.(SessionDashboardModel)

	if updated.ViewMode() != DashboardViewSingleSession {
		t.Fatalf("expected single-session mode, got %v", updated.ViewMode())
	}

	// Dir watcher sends close event → auto-transition to zero-session mode
	dirCloseEvent := sessionDirWatcherEventMsg{
		event: session.SessionEvent{
			Type: session.SessionClosed,
			Session: session.ActiveSession{
				Meta: session.SessionMeta{PID: 100, SessionID: "sess-1"},
			},
		},
	}
	newModel2, _ := updated.Update(dirCloseEvent)
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.PaneCount() != 0 {
		t.Fatalf("expected 0 panes after dir watcher close, got %d", updated2.PaneCount())
	}
	if updated2.ViewMode() != DashboardViewZeroSessions {
		t.Errorf("expected zero-session mode after dir watcher close, got %v", updated2.ViewMode())
	}
}

// TestAutoTransition_FullCycle_ZeroToSingleToGridAndBack verifies the complete
// round-trip of view mode transitions: 0→1→2→1→0 as sessions come and go.
func TestAutoTransition_FullCycle_ZeroToSingleToGridAndBack(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-full-cycle"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Step 0: Initially zero-session mode
	if m.ViewMode() != DashboardViewZeroSessions {
		t.Fatalf("step 0: expected zero-session mode, got %v", m.ViewMode())
	}

	// Step 1: 0→1 (zero → single)
	makeJSONLFile(t, projectDir, "sess-1")
	scan := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan})
	m = newModel.(SessionDashboardModel)

	if m.ViewMode() != DashboardViewSingleSession {
		t.Fatalf("step 1: expected single-session mode, got %v", m.ViewMode())
	}

	// Step 2: 1→2 (single → grid)
	makeJSONLFile(t, projectDir, "sess-2")
	scan = session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ = m.Update(sessionScanResultMsg{result: scan})
	m = newModel.(SessionDashboardModel)

	if m.ViewMode() != DashboardViewGrid {
		t.Fatalf("step 2: expected grid mode, got %v", m.ViewMode())
	}
	if m.PaneCount() != 2 {
		t.Fatalf("step 2: expected 2 panes, got %d", m.PaneCount())
	}

	// Step 3: 2→1 (grid → single) via scan with dead PID
	// Grace period requires testScanMissThreshold consecutive misses before removal.
	checker.SetAlive(200, false)
	scan = session.ScanResult{
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	m = applyScanResultNTimes(t, m, sessionScanResultMsg{result: scan}, testScanMissThreshold)

	if m.ViewMode() != DashboardViewSingleSession {
		t.Fatalf("step 3: expected single-session mode after 2→1, got %v", m.ViewMode())
	}
	if m.PaneCount() != 1 {
		t.Fatalf("step 3: expected 1 pane, got %d", m.PaneCount())
	}

	// Step 4: 1→0 (single → zero) via session closed
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1"}},
	}
	newModel, _ = m.Update(sessionClosedMsg{event: closeEvent})
	m = newModel.(SessionDashboardModel)

	if m.ViewMode() != DashboardViewZeroSessions {
		t.Fatalf("step 4: expected zero-session mode after 1→0, got %v", m.ViewMode())
	}
	if m.PaneCount() != 0 {
		t.Fatalf("step 4: expected 0 panes, got %d", m.PaneCount())
	}
}

// TestAutoTransition_FreshStateOnEveryTransition verifies that each auto-transition
// creates fresh view state (no scroll position preserved from previous mode).
func TestAutoTransition_FreshStateOnEveryTransition(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-fresh-state"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")

	// Get to single-session mode
	scan1 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	m = newModel.(SessionDashboardModel)

	if m.ViewMode() != DashboardViewSingleSession {
		t.Fatalf("expected single-session mode, got %v", m.ViewMode())
	}

	// Transition to grid mode (add second session)
	makeJSONLFile(t, projectDir, "sess-2")
	scan2 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	newModel, _ = m.Update(sessionScanResultMsg{result: scan2})
	m = newModel.(SessionDashboardModel)

	// Verify single session viewer was cleaned up (fresh state)
	if m.SingleSessionViewer() != nil {
		t.Error("singleSessionViewer should be nil after transition to grid mode (fresh state)")
	}
	if m.LatestViewer() != nil {
		t.Error("latestViewer should be nil in grid mode (fresh state)")
	}

	// Go back to single session (remove second)
	// Grace period requires testScanMissThreshold consecutive misses before removal.
	checker.SetAlive(200, false)
	scan3 := session.ScanResult{
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionActive},
		},
		ScanTime: time.Now(),
	}
	m = applyScanResultNTimes(t, m, sessionScanResultMsg{result: scan3}, testScanMissThreshold)

	if m.ViewMode() != DashboardViewSingleSession {
		t.Fatalf("expected single-session mode after returning from grid, got %v", m.ViewMode())
	}

	// Verify latestViewer is cleaned up (fresh state, no leftover from zero-session mode)
	if m.LatestViewer() != nil {
		t.Error("latestViewer should be nil in single-session mode (fresh state)")
	}

	// Go to zero session mode
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1"}},
	}
	newModel, _ = m.Update(sessionClosedMsg{event: closeEvent})
	m = newModel.(SessionDashboardModel)

	if m.ViewMode() != DashboardViewZeroSessions {
		t.Fatalf("expected zero-session mode, got %v", m.ViewMode())
	}

	// Verify single session viewer is cleaned up (fresh state)
	if m.SingleSessionViewer() != nil {
		t.Error("singleSessionViewer should be nil in zero-session mode (fresh state)")
	}
}

// TestAutoTransition_ScanResultWithRemovedStateDoesNotAddPanes verifies that
// sessions in SessionRemoved state do not trigger pane creation or mode transition.
func TestAutoTransition_ScanResultWithRemovedStateDoesNotAddPanes(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/auto-transition-removed-no-add"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Scan with a removed session should NOT create a pane or transition
	scan := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}, State: session.SessionRemoved},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 0 {
		t.Errorf("expected 0 panes (removed session should not be added), got %d", updated.PaneCount())
	}
	if updated.ViewMode() != DashboardViewZeroSessions {
		t.Errorf("expected zero-session mode (removed session doesn't count), got %v", updated.ViewMode())
	}
}
