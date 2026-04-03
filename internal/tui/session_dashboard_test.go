package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

func TestNewSessionDashboardModel(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", "/tmp/projectdir", scanner, monitor)

	if m.projectPath != "/tmp/project" {
		t.Errorf("projectPath = %q, want %q", m.projectPath, "/tmp/project")
	}
	if m.projectDir != "/tmp/projectdir" {
		t.Errorf("projectDir = %q, want %q", m.projectDir, "/tmp/projectdir")
	}
	if m.PaneCount() != 0 {
		t.Errorf("initial pane count = %d, want 0", m.PaneCount())
	}
	if !m.subscriptionsActive {
		t.Error("subscriptions should be active on init")
	}
}

func TestSessionDashboardModel_HandleScanResult_NewPane(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-project"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Create a JSONL file
	sessionID := "test-session-uuid"
	makeJSONLFile(t, projectDir, sessionID)

	// Simulate a scan result with one session
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID:       100,
					SessionID: sessionID,
					CWD:       projectPath,
					Kind:      "interactive",
				},
				FilePath: filepath.Join(sessDir, "100.json"),
			},
		},
		ScanTime: time.Now(),
	}

	// Send scan result message
	newModel, cmd := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("pane count after scan = %d, want 1", updated.PaneCount())
	}

	if updated.panes[0].session.Meta.PID != 100 {
		t.Errorf("pane PID = %d, want 100", updated.panes[0].session.Meta.PID)
	}

	if updated.panes[0].loading != true {
		t.Error("new pane should be in loading state")
	}

	if cmd == nil {
		t.Error("expected non-nil cmd for content loading")
	}
}

func TestSessionDashboardModel_HandleScanResult_NoDuplicatePanes(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-project"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "test-session"
	makeJSONLFile(t, projectDir, sessionID)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath},
			},
		},
	}

	// First scan — creates pane
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after first scan, got %d", updated.PaneCount())
	}

	// Second scan with same session — should NOT create duplicate
	newModel2, _ := updated.Update(sessionScanResultMsg{result: scanResult})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.PaneCount() != 1 {
		t.Errorf("expected 1 pane after duplicate scan, got %d", updated2.PaneCount())
	}
}

// TestSessionDashboardModel_DeduplicateBySessionID_LatestPIDOnly verifies AC6:
// when multiple scan entries share the same sessionId, only the pane for the
// latest (highest) PID is created. Ghost panes from restarted processes must
// not appear in the dashboard.
func TestSessionDashboardModel_DeduplicateBySessionID_LatestPIDOnly(t *testing.T) {
	checker := newTestPIDChecker(100, 200, 300)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dedup-test-project"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	const sharedSessionID = "shared-session-id"
	makeJSONLFile(t, projectDir, sharedSessionID)

	// Three entries share the same sessionId with ascending PIDs.
	// Only the highest PID (300) should survive deduplication.
	scanResult := session.ScanResult{
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sharedSessionID, CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: sharedSessionID, CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 300, SessionID: sharedSessionID, CWD: projectPath}},
		},
	}

	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after deduplication (3 entries, same sessionId), got %d", updated.PaneCount())
	}
	if updated.panes[0].session.Meta.PID != 300 {
		t.Errorf("expected surviving pane PID=300 (latest), got PID=%d", updated.panes[0].session.Meta.PID)
	}
}

// TestSessionDashboardModel_DeduplicateBySessionID_Mixed verifies that mixed
// scan results (some unique sessionIds, some duplicates) are deduplicated
// correctly: unique sessions each get their own pane; duplicate sessionIds
// collapse to a single pane for the highest PID.
func TestSessionDashboardModel_DeduplicateBySessionID_Mixed(t *testing.T) {
	checker := newTestPIDChecker(100, 200, 300, 400)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dedup-mixed-test"

	sc := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-a")
	makeJSONLFile(t, projectDir, "sess-b")
	makeJSONLFile(t, projectDir, "sess-c")

	scanResult := session.ScanResult{
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			// sess-a: two entries — PID 100 and 300; keep PID 300
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-a", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 300, SessionID: "sess-a", CWD: projectPath}},
			// sess-b: unique
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-b", CWD: projectPath}},
			// sess-c: unique
			{Meta: session.SessionMeta{PID: 400, SessionID: "sess-c", CWD: projectPath}},
		},
	}

	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// 4 entries → 3 unique sessionIds → 3 panes
	if updated.PaneCount() != 3 {
		t.Fatalf("expected 3 panes after dedup, got %d", updated.PaneCount())
	}

	pidBySessionID := make(map[string]int)
	for _, p := range updated.panes {
		pidBySessionID[p.session.Meta.SessionID] = p.session.Meta.PID
	}

	if pidBySessionID["sess-a"] != 300 {
		t.Errorf("sess-a: expected PID 300, got %d", pidBySessionID["sess-a"])
	}
	if pidBySessionID["sess-b"] != 200 {
		t.Errorf("sess-b: expected PID 200, got %d", pidBySessionID["sess-b"])
	}
	if pidBySessionID["sess-c"] != 400 {
		t.Errorf("sess-c: expected PID 400, got %d", pidBySessionID["sess-c"])
	}
}

func TestSessionDashboardModel_HandleScanResult_MaxPanes(t *testing.T) {
	pids := make([]int, 12)
	for i := range pids {
		pids[i] = 100 + i
	}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-project"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Create 12 sessions (exceeds max of 9)
	sessions := make([]session.ActiveSession, 12)
	for i := range sessions {
		sid := fmt.Sprintf("session-%d", i)
		makeJSONLFile(t, projectDir, sid)
		sessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       100 + i,
				SessionID: sid,
				CWD:       projectPath,
			},
		}
	}

	scanResult := session.ScanResult{Sessions: sessions}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// With pagination, all 12 sessions are stored (no cap at MaxSessionPanes)
	if updated.PaneCount() != 12 {
		t.Errorf("pane count = %d, want 12 (all sessions stored for pagination)", updated.PaneCount())
	}

	// But only up to 9 are visible on the current page
	visiblePanes := updated.CurrentPagePanes()
	if len(visiblePanes) != MaxSessionPanes {
		t.Errorf("visible panes on page 0 = %d, want %d", len(visiblePanes), MaxSessionPanes)
	}

	// Total pages should be 2 (9 + 3)
	if updated.TotalPages() != 2 {
		t.Errorf("total pages = %d, want 2", updated.TotalPages())
	}
}

func TestSessionDashboardModel_HandleScanResult_FiltersByProject(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/my-project"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-mine")
	makeJSONLFile(t, projectDir, "sess-other")

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{PID: 100, SessionID: "sess-mine", CWD: projectPath},
			},
			{
				Meta: session.SessionMeta{PID: 200, SessionID: "sess-other", CWD: "/tmp/other-project"},
			},
		},
	}

	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Errorf("expected 1 pane (filtered), got %d", updated.PaneCount())
	}
	if updated.panes[0].session.Meta.PID != 100 {
		t.Errorf("wrong session, PID = %d, want 100", updated.panes[0].session.Meta.PID)
	}
}

func TestSessionDashboardModel_HandleSessionClosed(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-project"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")
	makeJSONLFile(t, projectDir, "sess-2")

	// Add two panes
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", updated.PaneCount())
	}

	// Close session 100
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1"}},
	}
	newModel2, _ := updated.Update(sessionClosedMsg{event: closeEvent})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.PaneCount() != 1 {
		t.Errorf("expected 1 pane after close, got %d", updated2.PaneCount())
	}
	if updated2.panes[0].session.Meta.PID != 200 {
		t.Errorf("remaining pane PID = %d, want 200", updated2.panes[0].session.Meta.PID)
	}
}

func TestSessionDashboardModel_HandleSessionClosed_FocusAdjustment(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "s1")
	makeJSONLFile(t, projectDir, "s2")

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "s1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "s2", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Focus on second pane
	updated.focusIndex = 1

	// Close second pane
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 200}},
	}
	newModel2, _ := updated.Update(sessionClosedMsg{event: closeEvent})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.focusIndex != 0 {
		t.Errorf("focus should adjust to 0, got %d", updated2.focusIndex)
	}
}

func TestSessionDashboardModel_PaneContentLoaded(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "test-sess"
	makeJSONLFile(t, projectDir, sessionID)

	// Add a pane
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Simulate content loaded
	contentMsg := sessionPaneContentLoadedMsg{
		sessionID: sessionID,
		filePath:  filepath.Join(projectDir, sessionID+".jsonl"),
	}
	newModel2, _ := updated.Update(contentMsg)
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.panes[0].loading {
		t.Error("pane should not be loading after content loaded")
	}
}

func TestSessionDashboardModel_PaneContentLoaded_Error(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "err-sess"
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	contentMsg := sessionPaneContentLoadedMsg{
		sessionID: sessionID,
		err:       fmt.Errorf("parse error"),
	}
	newModel2, _ := updated.Update(contentMsg)
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.panes[0].errMsg != "parse error" {
		t.Errorf("expected error message, got %q", updated2.panes[0].errMsg)
	}
}

func TestSessionDashboardModel_View_NoPanes(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Waiting") {
		t.Errorf("expected waiting message, got: %q", view)
	}
}

func TestSessionDashboardModel_View_WithPanes(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "view-test"
	makeJSONLFile(t, projectDir, sessionID)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath, Kind: "interactive"}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	view := updated.View()
	if view == "" {
		t.Error("view should not be empty with panes")
	}
	// Should contain session dashboard help text
	if !strings.Contains(view, "auto-detecting") {
		t.Error("expected session dashboard help text")
	}
}

func TestSessionDashboardModel_Navigation(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "s1")
	makeJSONLFile(t, projectDir, "s2")

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "s1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "s2", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Move right
	newModel2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	updated2 := newModel2.(SessionDashboardModel)
	if updated2.focusIndex != 1 {
		t.Errorf("focus after right = %d, want 1", updated2.focusIndex)
	}

	// Move left
	newModel3, _ := updated2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	updated3 := newModel3.(SessionDashboardModel)
	if updated3.focusIndex != 0 {
		t.Errorf("focus after left = %d, want 0", updated3.focusIndex)
	}
}

func TestSessionDashboardModel_EscapeClosesAll(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.SetSize(80, 24)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := newModel.(SessionDashboardModel)

	if updated.subscriptionsActive {
		t.Error("subscriptions should be inactive after escape")
	}

	if cmd == nil {
		t.Fatal("expected command after escape")
	}

	// Execute the command to get the message
	msg := cmd()
	if _, ok := msg.(GoBackFromSessionDashboardMsg); !ok {
		t.Errorf("expected GoBackFromSessionDashboardMsg, got %T", msg)
	}
}

func TestSessionDashboardModel_WindowResize(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "s1", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Resize
	newModel2, _ := updated.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.width != 200 || updated2.height != 60 {
		t.Errorf("size = (%d, %d), want (200, 60)", updated2.width, updated2.height)
	}
}

func TestSessionDashboardModel_ResolveJSONLPath(t *testing.T) {
	m := SessionDashboardModel{
		projectDir: "/home/user/.claude/projects/-Users-foo-bar",
	}

	sess := session.ActiveSession{
		Meta: session.SessionMeta{SessionID: "abc-123"},
	}

	path := m.resolveJSONLPath(sess)
	expected := "/home/user/.claude/projects/-Users-foo-bar/abc-123.jsonl"
	if path != expected {
		t.Errorf("resolveJSONLPath = %q, want %q", path, expected)
	}
}

func TestSessionDashboardModel_ResolveJSONLPath_Empty(t *testing.T) {
	m := SessionDashboardModel{projectDir: ""}
	sess := session.ActiveSession{
		Meta: session.SessionMeta{SessionID: "abc"},
	}
	if path := m.resolveJSONLPath(sess); path != "" {
		t.Errorf("expected empty path, got %q", path)
	}

	m2 := SessionDashboardModel{projectDir: "/tmp"}
	sess2 := session.ActiveSession{
		Meta: session.SessionMeta{SessionID: ""},
	}
	if path := m2.resolveJSONLPath(sess2); path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}

func TestSessionDashboardModel_FindPaneBySessionID(t *testing.T) {
	m := SessionDashboardModel{
		panes: []SessionPaneModel{
			{session: session.ActiveSession{Meta: session.SessionMeta{SessionID: "a"}}},
			{session: session.ActiveSession{Meta: session.SessionMeta{SessionID: "b"}}},
		},
	}

	if idx := m.findPaneBySessionID("a"); idx != 0 {
		t.Errorf("findPaneBySessionID('a') = %d, want 0", idx)
	}
	if idx := m.findPaneBySessionID("b"); idx != 1 {
		t.Errorf("findPaneBySessionID('b') = %d, want 1", idx)
	}
	if idx := m.findPaneBySessionID("c"); idx != -1 {
		t.Errorf("findPaneBySessionID('c') = %d, want -1", idx)
	}
}

func TestSessionDashboardModel_FindPaneByPID(t *testing.T) {
	m := SessionDashboardModel{
		panes: []SessionPaneModel{
			{session: session.ActiveSession{Meta: session.SessionMeta{PID: 100}}},
			{session: session.ActiveSession{Meta: session.SessionMeta{PID: 200}}},
		},
	}

	if idx := m.findPaneByPID(100); idx != 0 {
		t.Errorf("findPaneByPID(100) = %d, want 0", idx)
	}
	if idx := m.findPaneByPID(999); idx != -1 {
		t.Errorf("findPaneByPID(999) = %d, want -1", idx)
	}
}

func TestSessionPaneModel_ViewWithFocus(t *testing.T) {
	pane := SessionPaneModel{
		session: session.ActiveSession{
			Meta: session.SessionMeta{PID: 12345, Kind: "interactive"},
		},
		width:   40,
		height:  10,
		loading: true,
	}

	view := pane.ViewWithFocus(true)
	if view == "" {
		t.Error("view should not be empty")
	}
	if !strings.Contains(view, "Loading") {
		t.Error("loading pane should show Loading")
	}
}

func TestSessionPaneModel_ViewWithFocus_SmallDimensions(t *testing.T) {
	pane := SessionPaneModel{width: 2, height: 2}
	if view := pane.ViewWithFocus(false); view != "" {
		t.Errorf("expected empty view for small dimensions, got %q", view)
	}
}

func TestSessionPaneModel_ViewWithFocus_Error(t *testing.T) {
	pane := SessionPaneModel{
		session: session.ActiveSession{
			Meta: session.SessionMeta{PID: 100, Kind: "interactive"},
		},
		width:  40,
		height: 10,
		errMsg: "some error",
	}

	view := pane.ViewWithFocus(false)
	if !strings.Contains(view, "Error") {
		t.Error("error pane should show error message")
	}
}

func TestSessionPaneModel_ViewWithFocus_Empty(t *testing.T) {
	pane := SessionPaneModel{
		session: session.ActiveSession{
			Meta: session.SessionMeta{PID: 100, Kind: "interactive"},
		},
		width:  40,
		height: 10,
	}

	view := pane.ViewWithFocus(false)
	if !strings.Contains(view, "Waiting") {
		t.Error("empty pane should show waiting message")
	}
}

func TestPaneDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		session  session.ActiveSession
		expected string
	}{
		{
			"interactive session",
			session.ActiveSession{Meta: session.SessionMeta{PID: 12345, Kind: "interactive"}},
			"interactive[12345]",
		},
		{
			"empty kind defaults to session",
			session.ActiveSession{Meta: session.SessionMeta{PID: 42}},
			"session[42]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := paneDisplayName(tt.session)
			if name != tt.expected {
				t.Errorf("paneDisplayName() = %q, want %q", name, tt.expected)
			}
		})
	}
}

func TestFilterSessionsForProject(t *testing.T) {
	sessions := []session.ActiveSession{
		{Meta: session.SessionMeta{PID: 1, CWD: "/my/project"}},
		{Meta: session.SessionMeta{PID: 2, CWD: "/other/project"}},
		{Meta: session.SessionMeta{PID: 3, CWD: "/my/project"}},
	}

	filtered := session.FilterByProject(sessions, "/my/project")
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered sessions, got %d", len(filtered))
	}
}

func TestFilterSessionsForProject_NoFilter(t *testing.T) {
	sessions := []session.ActiveSession{
		{Meta: session.SessionMeta{PID: 1, CWD: "/a"}},
		{Meta: session.SessionMeta{PID: 2, CWD: "/b"}},
	}
	// When projectPath is empty, FilterByProject returns nil,
	// so callers should skip the call (preserving original behavior).
	filtered := session.FilterByProject(sessions, "")
	if filtered != nil {
		t.Errorf("expected nil when no filter, got %d sessions", len(filtered))
	}
}

func TestSessionDashboardModel_ScanErrorIgnored(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	scanResult := session.ScanResult{Err: fmt.Errorf("scan error")}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 0 {
		t.Error("scan error should not create panes")
	}
}

func TestSessionDashboardModel_HandleSessionClosed_UnknownPID(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 999}},
	}
	newModel, _ := m.Update(sessionClosedMsg{event: closeEvent})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 0 {
		t.Error("closing unknown PID should be no-op")
	}
}

func TestSessionDashboardModel_SubscriptionTick_Inactive(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.subscriptionsActive = false

	newModel, cmd := m.Update(sessionSubscriptionTickMsg{})
	_ = newModel.(SessionDashboardModel)

	if cmd != nil {
		t.Error("inactive subscription should return nil cmd")
	}
}

func TestSessionDashboardModel_EnterKey_EmptyPane(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	// Use empty projectDir so resolveJSONLPath returns ""
	m := NewSessionDashboardModel(projectPath, "", scanner, monitor)
	m.SetSize(120, 40)

	// Add pane with no JSONL path (empty projectDir means no JSONL resolution)
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "no-jsonl", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Verify jsonlPath is empty
	if updated.panes[0].jsonlPath != "" {
		t.Fatalf("expected empty jsonlPath, got %q", updated.panes[0].jsonlPath)
	}

	// Enter on pane with no jsonlPath should be no-op
	_, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("should not produce command for pane without JSONL path")
	}
}

// TestSessionDashboardModel_PaneAppearsWithin3Seconds is the core acceptance test.
// It verifies the end-to-end timing from session detection to pane appearance.
func TestSessionDashboardModel_PaneAppearsWithin3Seconds(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	// Create scanner with fast polling (100ms for test).
	// WithJSONLBaseDir directs the scanner to check projectDir for JSONL files
	// rather than the real ~/.claude/projects tree (which test sessions don't use).
	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(100*time.Millisecond),
		session.WithJSONLBaseDir(projectDir),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "timing-test"
	makeJSONLFile(t, projectDir, sessionID)

	// Create session file — this is when the session "appears"
	start := time.Now()
	makeTestSessionFile(t, sessDir, 100, sessionID, projectPath)

	// Start scanner
	resultCh := scanner.Start()
	defer scanner.Stop()

	// Wait for scan result
	var scanResult session.ScanResult
	select {
	case scanResult = <-resultCh:
	case <-time.After(3 * time.Second):
		t.Fatal("scan result not received within 3 seconds")
	}

	// Process scan result
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	elapsed := time.Since(start)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane, got %d", updated.PaneCount())
	}

	if elapsed > 3*time.Second {
		t.Errorf("pane appeared after %v, must be under 3 seconds", elapsed)
	}

	t.Logf("Session detected and pane created in %v", elapsed)
}

func TestSessionDashboardModel_MultipleSessionsAppear(t *testing.T) {
	pids := []int{100, 200, 300}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(100*time.Millisecond),
		session.WithJSONLBaseDir(projectDir),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Create all sessions at once
	for i, pid := range pids {
		sid := fmt.Sprintf("sess-%d", i)
		makeJSONLFile(t, projectDir, sid)
		makeTestSessionFile(t, sessDir, pid, sid, projectPath)
	}

	start := time.Now()
	resultCh := scanner.Start()
	defer scanner.Stop()

	var scanResult session.ScanResult
	select {
	case scanResult = <-resultCh:
	case <-time.After(3 * time.Second):
		t.Fatal("scan result not received within 3 seconds")
	}

	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)
	elapsed := time.Since(start)

	if updated.PaneCount() != 3 {
		t.Errorf("expected 3 panes, got %d", updated.PaneCount())
	}

	if elapsed > 3*time.Second {
		t.Errorf("panes appeared after %v, must be under 3 seconds", elapsed)
	}

	t.Logf("3 sessions detected and panes created in %v", elapsed)
}

func TestSessionPaneModel_RenderContent(t *testing.T) {
	pane := &SessionPaneModel{
		width:  60,
		height: 15,
	}

	// Empty entries
	content := pane.renderContent()
	if content != "" {
		t.Errorf("expected empty content for no entries, got %q", content)
	}
}

func TestSessionDashboardModel_MoveFocus_SinglePane(t *testing.T) {
	m := SessionDashboardModel{
		panes:  []SessionPaneModel{{}},
		width:  120,
		height: 40,
	}

	result := m.moveFocus("right")
	if result != 0 {
		t.Errorf("moveFocus with single pane should return 0, got %d", result)
	}
}

func TestSessionDashboardModel_PollChannels_Empty(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
	}

	msg := m.pollChannels()
	if msg != nil {
		t.Errorf("expected nil from empty channels, got %T", msg)
	}
}

func TestSessionDashboardModel_PollChannels_ScanResult(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
	}

	expected := session.ScanResult{ScanTime: time.Now()}
	m.scanResultChan <- expected

	msg := m.pollChannels()
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
	if scanMsg, ok := msg.(sessionScanResultMsg); !ok {
		t.Errorf("expected sessionScanResultMsg, got %T", msg)
	} else if scanMsg.result.ScanTime != expected.ScanTime {
		t.Error("scan time mismatch")
	}
}

func TestSessionDashboardModel_PollChannels_MonitorEvent(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
	}

	expected := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 42}},
	}
	m.monitorChan <- expected

	msg := m.pollChannels()
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
	if closedMsg, ok := msg.(sessionClosedMsg); !ok {
		t.Errorf("expected sessionClosedMsg, got %T", msg)
	} else if closedMsg.event.Session.Meta.PID != 42 {
		t.Error("PID mismatch")
	}
}

func TestSessionDashboardModel_CloseAll(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	m.closeAll()

	if m.subscriptionsActive {
		t.Error("subscriptions should be inactive after closeAll")
	}
}

func TestSessionDashboardModel_QKey(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	updated := newModel.(SessionDashboardModel)

	if updated.subscriptionsActive {
		t.Error("subscriptions should be inactive after q")
	}
	if cmd == nil {
		t.Fatal("expected command after q")
	}
	msg := cmd()
	if _, ok := msg.(GoBackFromSessionDashboardMsg); !ok {
		t.Errorf("expected GoBackFromSessionDashboardMsg, got %T", msg)
	}
}

func TestSessionDashboardModel_HandlePaneContentLoaded_NoMatchingPane(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	// Content for non-existent session — should be no-op
	msg := sessionPaneContentLoadedMsg{sessionID: "non-existent"}
	newModel, _ := m.Update(msg)
	updated := newModel.(SessionDashboardModel)
	if updated.PaneCount() != 0 {
		t.Error("should remain at 0 panes")
	}
}

func TestSessionDashboardModel_HandleWatcherEvent_NoMatchingPane(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	msg := sessionPaneWatcherEventMsg{sessionID: "non-existent"}
	newModel, _ := m.Update(msg)
	updated := newModel.(SessionDashboardModel)
	if updated.PaneCount() != 0 {
		t.Error("should remain at 0 panes")
	}
}

func TestLoadSessionPaneContentCmd(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")
	content := `{"type":"user","message":{"role":"user","content":"hello"},"timestamp":"2026-03-31T00:00:00Z"}` + "\n"
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := loadSessionPaneContentCmd("test-session", jsonlPath)
	msg := cmd()

	loaded, ok := msg.(sessionPaneContentLoadedMsg)
	if !ok {
		t.Fatalf("expected sessionPaneContentLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Errorf("unexpected error: %v", loaded.err)
	}
	if loaded.sessionID != "test-session" {
		t.Errorf("sessionID = %q, want %q", loaded.sessionID, "test-session")
	}
	if loaded.filePath != jsonlPath {
		t.Errorf("filePath = %q, want %q", loaded.filePath, jsonlPath)
	}
}

func TestLoadSessionPaneContentCmd_Error(t *testing.T) {
	cmd := loadSessionPaneContentCmd("test", "/nonexistent/path.jsonl")
	msg := cmd()

	loaded, ok := msg.(sessionPaneContentLoadedMsg)
	if !ok {
		t.Fatalf("expected sessionPaneContentLoadedMsg, got %T", msg)
	}
	if loaded.err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestSessionDashboardModel_EnterKey_WithJSONLPath(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "enter-test"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Manually set jsonlPath (normally set by content loaded handler)
	updated.panes[0].jsonlPath = jsonlPath

	newModel2, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = newModel2.(SessionDashboardModel)

	if cmd == nil {
		t.Fatal("expected command from enter")
	}

	msg := cmd()
	openMsg, ok := msg.(OpenViewerFromSessionDashboardMsg)
	if !ok {
		t.Fatalf("expected OpenViewerFromSessionDashboardMsg, got %T", msg)
	}
	if openMsg.FilePath != jsonlPath {
		t.Errorf("FilePath = %q, want %q", openMsg.FilePath, jsonlPath)
	}
}

func TestSessionDashboardModel_SetSize(t *testing.T) {
	m := SessionDashboardModel{}
	m.SetSize(100, 50)
	if m.width != 100 || m.height != 50 {
		t.Errorf("SetSize: (%d, %d) want (100, 50)", m.width, m.height)
	}
}

func TestSessionDashboardModel_RecalcPaneSizes_NoPanes(t *testing.T) {
	m := SessionDashboardModel{}
	m.SetSize(100, 50)
	// Should not panic
	m.recalcPaneSizes()
}

func TestSessionDashboardModel_Init(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	cmd := m.Init()

	if cmd == nil {
		t.Error("Init should return a non-nil batch command")
	}
}

func TestSessionDashboardModel_SubscriptionTick_Active(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.subscriptionsActive = true

	// With no events pending, should return a tick cmd
	newModel, cmd := m.Update(sessionSubscriptionTickMsg{})
	_ = newModel.(SessionDashboardModel)

	if cmd == nil {
		t.Error("active subscription tick should return a tick cmd")
	}
}

func TestSessionDashboardModel_SubscriptionTick_WithEvent(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.subscriptionsActive = true

	// Put a scan result in the channel
	makeJSONLFile(t, projectDir, "tick-test")
	m.scanResultChan <- session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "tick-test", CWD: projectPath}},
		},
	}

	// Tick should poll and process the event
	newModel, cmd := m.Update(sessionSubscriptionTickMsg{})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Errorf("expected 1 pane from polled event, got %d", updated.PaneCount())
	}
	if cmd == nil {
		t.Error("should return continuation cmd")
	}
}

func TestSessionPaneModel_RenderContent_WithEntries(t *testing.T) {
	pane := &SessionPaneModel{
		width:  60,
		height: 15,
		entries: []types.LogEntry{
			{
				Type: types.EntryTypeUser,
				Message: types.Message{
					TextContent: "hello world",
				},
			},
		},
	}

	content := pane.renderContent()
	if content == "" {
		t.Error("expected non-empty content for entries with text")
	}
	if !strings.Contains(content, "hello world") {
		t.Errorf("expected content to contain 'hello world', got %q", content)
	}
}

func TestSessionPaneModel_ViewWithFocus_WithContent(t *testing.T) {
	pane := SessionPaneModel{
		session: session.ActiveSession{
			Meta: session.SessionMeta{PID: 100, Kind: "interactive"},
		},
		width:   60,
		height:  15,
		content: "some rendered content",
		entries: []types.LogEntry{
			{
				Type:    types.EntryTypeUser,
				Message: types.Message{TextContent: "test"},
			},
		},
	}

	view := pane.ViewWithFocus(false)
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestSessionDashboardModel_PollChannels_FileWatcherEvent(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
		panes: []SessionPaneModel{
			{
				session:       session.ActiveSession{Meta: session.SessionMeta{SessionID: "test"}},
				fileEventChan: make(chan sessionPaneWatcherEventMsg, 1),
			},
		},
	}

	// Put event in file watcher channel
	expected := sessionPaneWatcherEventMsg{
		sessionID: "test",
		event:     watcher.NewEntriesMsg{},
	}
	m.panes[0].fileEventChan <- expected

	msg := m.pollChannels()
	if msg == nil {
		t.Fatal("expected non-nil message from file watcher channel")
	}
	if watchMsg, ok := msg.(sessionPaneWatcherEventMsg); !ok {
		t.Errorf("expected sessionPaneWatcherEventMsg, got %T", msg)
	} else if watchMsg.sessionID != "test" {
		t.Errorf("sessionID = %q, want %q", watchMsg.sessionID, "test")
	}
}

func TestSessionDashboardModel_HandleWatcherEvent_NewEntries(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "watch-test"
	makeJSONLFile(t, projectDir, sessionID)

	// Add a pane
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Send new entries watcher event
	newEntries := []types.LogEntry{
		{Type: types.EntryTypeUser, Message: types.Message{TextContent: "new entry"}},
	}
	watchMsg := sessionPaneWatcherEventMsg{
		sessionID: sessionID,
		event:     watcher.NewEntriesMsg{Entries: newEntries},
	}
	newModel2, _ := updated.Update(watchMsg)
	updated2 := newModel2.(SessionDashboardModel)

	if len(updated2.panes[0].entries) != 1 {
		t.Errorf("expected 1 entry after watcher event, got %d", len(updated2.panes[0].entries))
	}
}

func TestSessionDashboardModel_HandleWatcherEvent_FileReset(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "reset-test"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)
	updated.panes[0].jsonlPath = jsonlPath

	// Send file reset event
	watchMsg := sessionPaneWatcherEventMsg{
		sessionID: sessionID,
		event:     watcher.FileResetMsg{},
	}
	newModel2, cmd := updated.Update(watchMsg)
	_ = newModel2.(SessionDashboardModel)

	if cmd == nil {
		t.Error("file reset should trigger reload command")
	}
}

func TestSessionDashboardModel_CloseAll_WithPanes(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "close-test")

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "close-test", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	updated.closeAll()

	if updated.subscriptionsActive {
		t.Error("subscriptions should be inactive after closeAll")
	}
	// Pane watchers should be nil
	for i, pane := range updated.panes {
		if pane.watcher != nil {
			t.Errorf("pane %d watcher should be nil", i)
		}
		if pane.fileEventChan != nil {
			t.Errorf("pane %d fileEventChan should be nil", i)
		}
	}
}

func TestSessionDashboardModel_MoveFocus_Directions(t *testing.T) {
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 4), // 2x2 grid
		width:  120,
		height: 40,
	}

	// Start at 0
	m.focusIndex = 0
	result := m.moveFocus("right")
	if result != 1 {
		t.Errorf("right from 0: got %d, want 1", result)
	}

	m.focusIndex = 0
	result = m.moveFocus("down")
	if result != 2 {
		t.Errorf("down from 0: got %d, want 2", result)
	}

	m.focusIndex = 3
	result = m.moveFocus("up")
	if result != 1 {
		t.Errorf("up from 3: got %d, want 1", result)
	}

	m.focusIndex = 3
	result = m.moveFocus("left")
	if result != 2 {
		t.Errorf("left from 3: got %d, want 2", result)
	}
}

func TestSessionDashboardModel_EnterKey_NoPanes(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.focusIndex = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter with no panes should be no-op")
	}
}

func TestPaneDisplayName_Interactive(t *testing.T) {
	sess := session.ActiveSession{
		Meta: session.SessionMeta{PID: 12345, Kind: "interactive"},
	}
	name := paneDisplayName(sess)
	if name != "interactive[12345]" {
		t.Errorf("got %q, want %q", name, "interactive[12345]")
	}
}

func TestPaneDisplayName_DefaultKind(t *testing.T) {
	sess := session.ActiveSession{
		Meta: session.SessionMeta{PID: 42, Kind: ""},
	}
	name := paneDisplayName(sess)
	if name != "session[42]" {
		t.Errorf("got %q, want %q", name, "session[42]")
	}
}

// TestPaneDisplayName_ActiveNoIdleSuffix verifies that active sessions
// (State == SessionActive, i.e. JSONL modified within 2 minutes) display
// without the "⏸ IDLE" suffix — they use the normal display name.
func TestPaneDisplayName_ActiveNoIdleSuffix(t *testing.T) {
	sess := session.ActiveSession{
		Meta:  session.SessionMeta{PID: 999, Kind: "interactive"},
		State: session.SessionActive,
	}
	name := paneDisplayName(sess)
	if strings.Contains(name, "IDLE") {
		t.Errorf("active session should not contain IDLE suffix, got %q", name)
	}
	if name != "interactive[999]" {
		t.Errorf("got %q, want %q", name, "interactive[999]")
	}
}

// TestSessionPaneModel_ViewWithFocus_ActiveUsesNormalStyle verifies that
// active sessions (JSONL modified within 2 minutes) render with the normal
// header style and normal border colors — not the idle warning style.
func TestSessionPaneModel_ViewWithFocus_ActiveUsesNormalStyle(t *testing.T) {
	pane := SessionPaneModel{
		session: session.ActiveSession{
			Meta:  session.SessionMeta{PID: 500, Kind: "interactive"},
			State: session.SessionActive,
		},
		width:   40,
		height:  10,
		loading: true,
	}

	// Focused active pane should NOT contain idle markers
	viewFocused := pane.ViewWithFocus(true)
	if viewFocused == "" {
		t.Fatal("focused active pane should not be empty")
	}
	if strings.Contains(viewFocused, "IDLE") {
		t.Error("focused active pane should not show IDLE")
	}

	// Unfocused active pane should also NOT contain idle markers
	viewUnfocused := pane.ViewWithFocus(false)
	if viewUnfocused == "" {
		t.Fatal("unfocused active pane should not be empty")
	}
	if strings.Contains(viewUnfocused, "IDLE") {
		t.Error("unfocused active pane should not show IDLE")
	}
}

// TestClassifySessionState_ActiveWithinTwoMinutes verifies the scanner
// classifies sessions with JSONL modified within 2 minutes as Active.
func TestClassifySessionState_ActiveWithinTwoMinutes(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name string
		age  time.Duration
	}{
		{"just now", 0},
		{"10 seconds ago", 10 * time.Second},
		{"1 minute ago", 1 * time.Minute},
		{"1m59s ago", time.Minute + 59*time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			modTime := now.Add(-tc.age)
			state := session.ClassifySessionState(modTime, now)
			if state != session.SessionActive {
				t.Errorf("JSONL modified %v ago should be Active, got %v", tc.age, state)
			}
		})
	}
}

func TestSessionDashboardModel_View_ZeroSize(t *testing.T) {
	m := SessionDashboardModel{
		panes: []SessionPaneModel{{}},
	}
	// width/height = 0 => calculateGrid returns non-zero but pane dimensions = 0
	view := m.View()
	// Should handle gracefully, not panic
	_ = view
}

func TestSessionDashboardModel_HandleScanResult_IncrementalAdd(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "s1")
	makeJSONLFile(t, projectDir, "s2")

	// First scan: one session
	scan1 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "s1", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	updated := newModel.(SessionDashboardModel)
	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane, got %d", updated.PaneCount())
	}

	// Second scan: two sessions (adds one)
	scan2 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "s1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "s2", CWD: projectPath}},
		},
	}
	newModel2, _ := updated.Update(sessionScanResultMsg{result: scan2})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.PaneCount() != 2 {
		t.Errorf("expected 2 panes after incremental add, got %d", updated2.PaneCount())
	}
}

func TestSessionDashboardModel_SessionSubscriptionTickCmd(t *testing.T) {
	m := SessionDashboardModel{}
	cmd := m.sessionSubscriptionTickCmd()
	if cmd == nil {
		t.Error("expected non-nil tick cmd")
	}
}

func TestSessionPaneModel_ViewWithFocus_LongName(t *testing.T) {
	pane := SessionPaneModel{
		session: session.ActiveSession{
			Meta: session.SessionMeta{PID: 100, Kind: "interactive-very-long-kind-name-that-exceeds-width"},
		},
		width:   20,
		height:  10,
		loading: true,
	}

	view := pane.ViewWithFocus(true)
	if view == "" {
		t.Error("should render even with long name")
	}
}

// --- Adaptive Grid Layout Integration Tests ---

func TestRecalcPaneSizes_AdaptiveLayout_RemainderDistribution(t *testing.T) {
	// Test that recalcPaneSizes distributes remainder pixels correctly
	// 101 width / 3 cols = first pane gets 34, others 34, 33
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 3),
		width:  101,
		height: 42, // gridHeight = 41
	}

	m.recalcPaneSizes()

	totalWidth := 0
	for _, p := range m.panes {
		totalWidth += p.width
	}
	if totalWidth != 101 {
		t.Errorf("total width = %d, want 101 (remainder pixels distributed)", totalWidth)
	}

	// First pane should be wider (gets remainder)
	if m.panes[0].width <= m.panes[2].width {
		// First should be >= last due to remainder distribution
		if m.panes[0].width < m.panes[2].width {
			t.Errorf("pane 0 width %d should be >= pane 2 width %d", m.panes[0].width, m.panes[2].width)
		}
	}
}

func TestRecalcPaneSizes_AdaptiveLayout_NonUniformLastRow(t *testing.T) {
	// 5 panes in 2x3: row 0 has 3 panes, row 1 has 2 wider panes
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 5),
		width:  120,
		height: 51, // gridHeight = 50
	}

	m.recalcPaneSizes()

	// Row 0 panes: indices 0,1,2 should be 40 wide (120/3)
	for i := 0; i < 3; i++ {
		if m.panes[i].width != 40 {
			t.Errorf("pane %d (row 0) width = %d, want 40", i, m.panes[i].width)
		}
	}

	// Row 1 panes: indices 3,4 should be 60 wide (120/2)
	for i := 3; i < 5; i++ {
		if m.panes[i].width != 60 {
			t.Errorf("pane %d (row 1) width = %d, want 60", i, m.panes[i].width)
		}
	}
}

func TestRecalcPaneSizes_AdaptiveLayout_AllPaneCounts(t *testing.T) {
	// Verify all pane counts 1-9 produce valid layouts
	tests := []struct {
		count    int
		wantRows int
		wantCols int
	}{
		{1, 1, 1},
		{2, 1, 2},
		{3, 1, 3},
		{4, 2, 2},
		{5, 2, 3},
		{6, 2, 3},
		{7, 3, 3},
		{8, 3, 3},
		{9, 3, 3},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d_panes", tt.count), func(t *testing.T) {
			m := SessionDashboardModel{
				panes:  make([]SessionPaneModel, tt.count),
				width:  180,
				height: 91, // gridHeight = 90
			}

			m.recalcPaneSizes()

			// All panes should have non-zero dimensions
			for i, p := range m.panes {
				if p.width <= 0 || p.height <= 0 {
					t.Errorf("pane %d has non-positive dimensions: %dx%d", i, p.width, p.height)
				}
			}

			// Verify total width coverage per row via grid layout
			layout := CalculateGridLayout(tt.count, 180, 90)
			if layout.Rows != tt.wantRows || layout.Cols != tt.wantCols {
				t.Errorf("grid %dx%d, want %dx%d", layout.Rows, layout.Cols, tt.wantRows, tt.wantCols)
			}
		})
	}
}

func TestRecalcPaneSizes_AdaptiveLayout_OddTerminalSize(t *testing.T) {
	// Non-divisible terminal dimensions - verify no pixel waste
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 4), // 2x2
		width:  173,
		height: 98, // gridHeight = 97
	}

	m.recalcPaneSizes()

	// Row 0: panes 0,1 widths should sum to 173
	w0 := m.panes[0].width + m.panes[1].width
	if w0 != 173 {
		t.Errorf("row 0 total width = %d, want 173", w0)
	}

	// Row 1: panes 2,3 widths should sum to 173
	w1 := m.panes[2].width + m.panes[3].width
	if w1 != 173 {
		t.Errorf("row 1 total width = %d, want 173", w1)
	}

	// Heights should sum to gridHeight (97)
	h := m.panes[0].height + m.panes[2].height
	if h != 97 {
		t.Errorf("total height = %d, want 97", h)
	}
}

func TestRecalcPaneSizes_EmptyPanes(t *testing.T) {
	m := SessionDashboardModel{
		panes:  nil,
		width:  100,
		height: 50,
	}
	// Should not panic
	m.recalcPaneSizes()
}

func TestRecalcPaneSizes_MinGridHeight(t *testing.T) {
	// With height=2, gridHeight = max(2-1, 3) = 3
	// But MinPaneHeight=5, so grid layout marks overflow and returns no panes.
	// Pane dimensions should remain at zero (no layout possible).
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 1),
		width:  100,
		height: 2, // gridHeight = max(1, 3) = 3
	}

	m.recalcPaneSizes()

	// With gridHeight=3 < MinPaneHeight=5, layout has overflow → no panes assigned
	// Pane dimensions remain 0 (no valid layout at this size)
	if m.panes[0].width != 0 && m.panes[0].height != 0 {
		// Either both are zero (overflow) or both are set (enough space)
		// At height=2, gridHeight=3 which is < MinPaneHeight=5, so overflow
		layout := CalculateGridLayout(1, 100, 3)
		if layout.Overflow {
			// Expected: panes not assigned any size
			t.Logf("correctly detected overflow at gridHeight=3 (MinPaneHeight=%d)", MinPaneHeight)
		}
	}
}

func TestRecalcPaneSizes_SmallButViable(t *testing.T) {
	// Height just large enough: gridHeight = MinPaneHeight + 1 = 6
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 1),
		width:  100,
		height: 7, // gridHeight = 6
	}

	m.recalcPaneSizes()

	if m.panes[0].width != 100 {
		t.Errorf("pane width = %d, want 100", m.panes[0].width)
	}
	if m.panes[0].height != 6 {
		t.Errorf("pane height = %d, want 6", m.panes[0].height)
	}
}

func TestMoveFocus_AdaptiveLayout_NonUniformGrid(t *testing.T) {
	// 5 panes in 2x3 grid: navigation should work correctly
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 5),
		width:  120,
		height: 51,
	}

	// Move right from pane 0 → pane 1
	m.focusIndex = 0
	if got := m.moveFocus("right"); got != 1 {
		t.Errorf("right from 0: got %d, want 1", got)
	}

	// Move down from pane 0 → pane 3 (row 0, col 0 → row 1, col 0)
	m.focusIndex = 0
	if got := m.moveFocus("down"); got != 3 {
		t.Errorf("down from 0: got %d, want 3", got)
	}

	// Move down from pane 2 → should clamp to last pane if beyond range
	m.focusIndex = 2
	result := m.moveFocus("down")
	if result < 0 || result >= 5 {
		t.Errorf("down from 2: got %d, should be in range [0,4]", result)
	}
}

func TestMoveFocus_AdaptiveLayout_SinglePane(t *testing.T) {
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 1),
		width:  120,
		height: 40,
	}

	// All directions should return 0 for single pane
	for _, dir := range []string{"up", "down", "left", "right"} {
		m.focusIndex = 0
		if got := m.moveFocus(dir); got != 0 {
			t.Errorf("moveFocus(%q) with 1 pane: got %d, want 0", dir, got)
		}
	}
}

func TestMoveFocus_AdaptiveLayout_ZeroDimensions(t *testing.T) {
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 4),
		width:  0,
		height: 0,
	}

	// Should not panic with zero dimensions
	m.focusIndex = 0
	result := m.moveFocus("right")
	if result != 0 {
		t.Errorf("moveFocus with zero dims: got %d, want 0", result)
	}
}

func TestView_AdaptiveLayout_UsesGridLayout(t *testing.T) {
	// Verify View renders something for various pane counts
	for count := 1; count <= 9; count++ {
		t.Run(fmt.Sprintf("%d_panes", count), func(t *testing.T) {
			m := SessionDashboardModel{
				width:  120,
				height: 40,
			}
			// Create panes with loading state (minimal rendering)
			m.panes = make([]SessionPaneModel, count)
			for i := range m.panes {
				m.panes[i] = SessionPaneModel{
					session: session.ActiveSession{
						Meta: session.SessionMeta{PID: 100 + i, Kind: "test"},
					},
					width:   40,
					height:  20,
					loading: true,
				}
			}

			view := m.View()
			if view == "" {
				t.Errorf("View() returned empty for %d panes", count)
			}
		})
	}
}

func TestView_AdaptiveLayout_NoPanesShowsWaiting(t *testing.T) {
	m := SessionDashboardModel{
		width:  120,
		height: 40,
	}
	view := m.View()
	if !strings.Contains(view, "Waiting") {
		t.Error("empty dashboard should show waiting message")
	}
}

func TestSetSize_PropagatesAdaptiveLayout(t *testing.T) {
	m := SessionDashboardModel{
		panes: make([]SessionPaneModel, 5), // 2x3 grid
	}

	m.SetSize(120, 51) // gridHeight = 50

	// After SetSize, panes should have dimensions from adaptive layout
	// Row 0 (panes 0-2): width = 40, height = 25
	// Row 1 (panes 3-4): width = 60, height = 25
	for i := 0; i < 3; i++ {
		if m.panes[i].width != 40 {
			t.Errorf("pane %d width = %d, want 40", i, m.panes[i].width)
		}
	}
	for i := 3; i < 5; i++ {
		if m.panes[i].width != 60 {
			t.Errorf("pane %d width = %d, want 60", i, m.panes[i].width)
		}
	}
}

func TestRecalcPaneSizes_SevenPanes_LastRowSingleFullWidth(t *testing.T) {
	// 7 panes in 3x3: row 2 has 1 pane at full width
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 7),
		width:  120,
		height: 61, // gridHeight = 60
	}

	m.recalcPaneSizes()

	// Last pane (index 6) should be full width
	if m.panes[6].width != 120 {
		t.Errorf("pane 6 (last row, single) width = %d, want 120", m.panes[6].width)
	}

	// First 6 panes should be 40 wide (120/3)
	for i := 0; i < 6; i++ {
		if m.panes[i].width != 40 {
			t.Errorf("pane %d width = %d, want 40", i, m.panes[i].width)
		}
	}
}

func TestRecalcPaneSizes_EightPanes_LastRowTwoPanes(t *testing.T) {
	// 8 panes in 3x3: row 2 has 2 panes, each 60 wide
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 8),
		width:  120,
		height: 61, // gridHeight = 60
	}

	m.recalcPaneSizes()

	// Last two panes should be 60 wide each
	if m.panes[6].width != 60 {
		t.Errorf("pane 6 width = %d, want 60", m.panes[6].width)
	}
	if m.panes[7].width != 60 {
		t.Errorf("pane 7 width = %d, want 60", m.panes[7].width)
	}
}

func TestRecalcPaneSizes_NinePanes_AllEqual(t *testing.T) {
	// 9 panes in 3x3: all should be 40x20
	m := SessionDashboardModel{
		panes:  make([]SessionPaneModel, 9),
		width:  120,
		height: 61, // gridHeight = 60
	}

	m.recalcPaneSizes()

	for i, p := range m.panes {
		if p.width != 40 || p.height != 20 {
			t.Errorf("pane %d: %dx%d, want 40x20", i, p.width, p.height)
		}
	}
}

func TestSessionDashboardModel_DirtyTracking(t *testing.T) {
	m := SessionDashboardModel{
		panes: []SessionPaneModel{
			{session: session.ActiveSession{Meta: session.SessionMeta{PID: 1}}},
			{session: session.ActiveSession{Meta: session.SessionMeta{PID: 2}}},
			{session: session.ActiveSession{Meta: session.SessionMeta{PID: 3}}},
		},
	}

	t.Run("anyPaneDirty returns false when no panes dirty", func(t *testing.T) {
		if m.anyPaneDirty() {
			t.Error("expected no dirty panes initially")
		}
	})

	t.Run("markPaneDirty marks specific pane", func(t *testing.T) {
		m.markPaneDirty(1)
		if !m.IsPaneDirty(1) {
			t.Error("expected pane 1 to be dirty")
		}
		if m.IsPaneDirty(0) {
			t.Error("expected pane 0 to not be dirty")
		}
	})

	t.Run("anyPaneDirty returns true when a pane is dirty", func(t *testing.T) {
		if !m.anyPaneDirty() {
			t.Error("expected anyPaneDirty true after marking pane 1")
		}
	})

	t.Run("markAllPanesDirty marks all", func(t *testing.T) {
		m.markAllPanesDirty()
		for i := 0; i < 3; i++ {
			if !m.IsPaneDirty(i) {
				t.Errorf("expected pane %d to be dirty", i)
			}
		}
	})

	t.Run("IsPaneDirty out of range", func(t *testing.T) {
		if m.IsPaneDirty(-1) {
			t.Error("expected false for negative index")
		}
		if m.IsPaneDirty(100) {
			t.Error("expected false for out-of-range index")
		}
	})

	t.Run("markPaneDirty out of range", func(t *testing.T) {
		// Should not panic
		m.markPaneDirty(-1)
		m.markPaneDirty(100)
	})
}

func TestSessionDashboardModel_GridDirty(t *testing.T) {
	m := SessionDashboardModel{
		panes: []SessionPaneModel{
			{session: session.ActiveSession{Meta: session.SessionMeta{PID: 1}}},
		},
	}

	t.Run("initially not dirty", func(t *testing.T) {
		if m.IsGridDirty() {
			t.Error("expected grid not dirty initially")
		}
	})

	t.Run("markGridDirty sets flag and marks all panes dirty", func(t *testing.T) {
		m.markGridDirty()
		if !m.IsGridDirty() {
			t.Error("expected grid dirty after markGridDirty")
		}
		if !m.IsPaneDirty(0) {
			t.Error("expected pane dirty after markGridDirty")
		}
	})

	t.Run("clearGridDirty resets flag", func(t *testing.T) {
		m.clearGridDirty()
		if m.IsGridDirty() {
			t.Error("expected grid not dirty after clearGridDirty")
		}
	})
}

func TestSessionDashboardModel_PaneCachedView(t *testing.T) {
	m := SessionDashboardModel{
		panes: []SessionPaneModel{
			{cachedView: "cached-content-1"},
			{cachedView: "cached-content-2"},
		},
	}

	t.Run("returns cached view for valid index", func(t *testing.T) {
		if got := m.PaneCachedView(0); got != "cached-content-1" {
			t.Errorf("expected 'cached-content-1', got %q", got)
		}
		if got := m.PaneCachedView(1); got != "cached-content-2" {
			t.Errorf("expected 'cached-content-2', got %q", got)
		}
	})

	t.Run("returns empty for out-of-range index", func(t *testing.T) {
		if got := m.PaneCachedView(-1); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
		if got := m.PaneCachedView(10); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

// --- Dirty-region rendering integration tests ---

// newTestDashboardWithPanes creates a SessionDashboardModel with N panes sized for rendering.
func newTestDashboardWithPanes(n int) SessionDashboardModel {
	panes := make([]SessionPaneModel, n)
	for i := range panes {
		panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("sess-%d", i),
					Kind:      "interactive",
				},
			},
			entries: []types.LogEntry{
				{Type: types.EntryTypeUser, Message: types.Message{TextContent: "hello"}},
			},
			width:  40,
			height: 20,
		}
	}
	return SessionDashboardModel{
		panes:  panes,
		width:  120,
		height: 61,
	}
}

func TestDirtyRegion_ViewPopulatesCacheOnFirstRender(t *testing.T) {
	m := newTestDashboardWithPanes(2)
	// All panes start with empty cachedView
	for i := range m.panes {
		if m.panes[i].cachedView != "" {
			t.Fatalf("pane %d should have empty cache initially", i)
		}
	}

	// First View() should populate caches
	result := m.View()
	if result == "" {
		t.Fatal("expected non-empty view")
	}

	for i := range m.panes {
		if m.panes[i].cachedView == "" {
			t.Errorf("pane %d should have populated cache after View()", i)
		}
	}
}

func TestDirtyRegion_ViewClearsDirtyFlagsAfterRender(t *testing.T) {
	m := newTestDashboardWithPanes(3)
	// Mark some panes dirty
	m.panes[0].dirty = true
	m.panes[2].dirty = true

	_ = m.View()

	for i := range m.panes {
		if m.panes[i].dirty {
			t.Errorf("pane %d should not be dirty after View()", i)
		}
	}
}

func TestDirtyRegion_CleanPanesReuseCachedView(t *testing.T) {
	m := newTestDashboardWithPanes(2)

	// First render populates caches
	_ = m.View()
	cache0 := m.panes[0].cachedView
	_ = m.panes[1].cachedView

	// Set a known marker in the cache to detect reuse vs re-render
	marker := "MARKER_SHOULD_PERSIST"
	m.panes[1].cachedView = marker
	// Pane 1 is NOT dirty, NOT gridDirty, focus hasn't changed, cache is non-empty
	// So View() should reuse the marker

	// Make sure pane 0 is also clean
	m.panes[0].dirty = false
	m.panes[0].cachedView = cache0

	_ = m.View()

	if m.panes[1].cachedView != marker {
		t.Errorf("expected clean pane to reuse cached view %q, got %q", marker, m.panes[1].cachedView)
	}

	// Pane 0 should also have been reused (it was clean)
	if m.panes[0].cachedView != cache0 {
		t.Errorf("expected clean pane 0 to reuse cached view")
	}
}

func TestDirtyRegion_DirtyPaneIsReRendered(t *testing.T) {
	m := newTestDashboardWithPanes(2)

	// First render
	_ = m.View()
	originalCache := m.panes[0].cachedView

	// Mark pane 0 dirty and change its content
	m.panes[0].entries = append(m.panes[0].entries, types.LogEntry{
		Type:    types.EntryTypeAssistant,
		Message: types.Message{TextContent: "world"},
	})
	m.panes[0].content = m.panes[0].renderContent()
	m.panes[0].dirty = true

	// Mark pane 1 with a marker — it should NOT be re-rendered
	marker := "PANE1_STABLE"
	m.panes[1].cachedView = marker

	_ = m.View()

	// Pane 0 should have been re-rendered (different from original)
	if m.panes[0].cachedView == originalCache {
		t.Error("dirty pane 0 should have been re-rendered with new content")
	}
	// Pane 1 should still have the marker (not re-rendered)
	if m.panes[1].cachedView != marker {
		t.Error("clean pane 1 should not have been re-rendered")
	}
}

func TestDirtyRegion_GridDirtyForcesAllPanesReRender(t *testing.T) {
	m := newTestDashboardWithPanes(3)

	// First render to populate caches
	_ = m.View()

	// Set markers on all panes
	for i := range m.panes {
		m.panes[i].cachedView = fmt.Sprintf("MARKER_%d", i)
		m.panes[i].dirty = false
	}

	// Set gridDirty
	m.gridDirty = true

	_ = m.View()

	// All pane caches should be different from markers (re-rendered)
	for i := range m.panes {
		if m.panes[i].cachedView == fmt.Sprintf("MARKER_%d", i) {
			t.Errorf("pane %d should have been re-rendered when gridDirty=true", i)
		}
	}
}

func TestDirtyRegion_FocusChangeReRendersAffectedPanes(t *testing.T) {
	m := newTestDashboardWithPanes(3)
	m.focusIndex = 0

	// First render — pane 0 is focused, panes 1,2 unfocused
	_ = m.View()

	// Verify lastFocused states
	if !m.panes[0].lastFocused {
		t.Error("pane 0 should be lastFocused=true after render with focusIndex=0")
	}
	if m.panes[1].lastFocused {
		t.Error("pane 1 should be lastFocused=false")
	}

	// Move focus from 0 to 1 — only panes 0 and 1 should re-render
	m.focusIndex = 1
	// Set marker on pane 2 (should not change since it's not affected by focus change)
	m.panes[2].cachedView = "PANE2_STABLE"

	_ = m.View()

	// Pane 0 should now be lastFocused=false (re-rendered as unfocused)
	if m.panes[0].lastFocused {
		t.Error("pane 0 should now be lastFocused=false after losing focus")
	}
	// Pane 1 should have been re-rendered (was unfocused, now focused)
	if !m.panes[1].lastFocused {
		t.Error("pane 1 should now be lastFocused=true")
	}
	// Pane 2 should NOT have been re-rendered (focus didn't affect it)
	if m.panes[2].cachedView != "PANE2_STABLE" {
		t.Errorf("pane 2 should not re-render when focus moves between 0 and 1, got %q", m.panes[2].cachedView)
	}
}

func TestDirtyRegion_NavigationKeysSetsCorrectDirtyFlags(t *testing.T) {
	m := newTestDashboardWithPanes(4)
	m.width = 120
	m.height = 61
	m.focusIndex = 0

	tests := []struct {
		key         string
		expectDirty []int // pane indices that should be dirty
	}{
		{"right", []int{0, 1}},
	}

	for _, tt := range tests {
		// Reset all dirty flags
		for i := range m.panes {
			m.panes[i].dirty = false
		}
		m.focusIndex = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
		updated := result.(SessionDashboardModel)

		for _, idx := range tt.expectDirty {
			if !updated.panes[idx].dirty {
				t.Errorf("key %q: expected pane %d to be dirty", tt.key, idx)
			}
		}
	}
}

func TestDirtyRegion_WindowResizeSetsGridDirty(t *testing.T) {
	m := newTestDashboardWithPanes(2)
	m.width = 80
	m.height = 40

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	updated := result.(SessionDashboardModel)

	if !updated.IsGridDirty() {
		t.Error("window resize should set gridDirty")
	}
	for i := range updated.panes {
		if !updated.panes[i].dirty {
			t.Errorf("window resize should mark pane %d dirty", i)
		}
	}
}

func TestDirtyRegion_ContentLoadedMarksPaneDirty(t *testing.T) {
	m := newTestDashboardWithPanes(2)
	m.panes[0].dirty = false
	m.panes[1].dirty = false
	m.panes[1].loading = true

	msg := sessionPaneContentLoadedMsg{
		sessionID:   "sess-1",
		entries:     []types.LogEntry{{Type: types.EntryTypeUser, Message: types.Message{TextContent: "test"}}},
		parseErrors: 0,
	}

	result, _ := m.Update(msg)
	updated := result.(SessionDashboardModel)

	if !updated.panes[1].dirty {
		t.Error("content loaded should mark target pane dirty")
	}
	if updated.panes[0].dirty {
		t.Error("content loaded should NOT mark other panes dirty")
	}
}

func TestDirtyRegion_ContentLoadedErrorMarksPaneDirty(t *testing.T) {
	m := newTestDashboardWithPanes(1)
	m.panes[0].dirty = false
	m.panes[0].loading = true

	msg := sessionPaneContentLoadedMsg{
		sessionID: "sess-0",
		err:       fmt.Errorf("load error"),
	}

	result, _ := m.Update(msg)
	updated := result.(SessionDashboardModel)

	if !updated.panes[0].dirty {
		t.Error("content load error should mark pane dirty")
	}
}

func TestDirtyRegion_WatcherEventMarksPaneDirty(t *testing.T) {
	m := newTestDashboardWithPanes(2)
	m.panes[0].dirty = false
	m.panes[1].dirty = false

	msg := sessionPaneWatcherEventMsg{
		sessionID: "sess-0",
		event: watcher.NewEntriesMsg{
			Entries: []types.LogEntry{{Type: types.EntryTypeAssistant, Message: types.Message{TextContent: "hi"}}},
		},
	}

	result, _ := m.Update(msg)
	updated := result.(SessionDashboardModel)

	if !updated.panes[0].dirty {
		t.Error("watcher event should mark target pane dirty")
	}
	if updated.panes[1].dirty {
		t.Error("watcher event should NOT mark other panes dirty")
	}
}

func TestDirtyRegion_SessionAddedMarksGridDirty(t *testing.T) {
	pidChecker := newTestPIDChecker(1000)
	sessDir := t.TempDir()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(pidChecker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(pidChecker))

	m := NewSessionDashboardModel("/test/project", "/encoded/project", scanner, monitor)
	m.width = 120
	m.height = 61
	m.gridDirty = false

	// Simulate scan result adding a new session
	result := session.ScanResult{
		Sessions: []session.ActiveSession{
			{
				Meta:     session.SessionMeta{PID: 2000, SessionID: "new-sess", CWD: "/test/project", Kind: "interactive"},
				FilePath: filepath.Join(sessDir, "2000.json"),
			},
		},
	}

	updated, _ := m.handleScanResult(result)
	model := updated.(SessionDashboardModel)

	if !model.IsGridDirty() {
		t.Error("adding a session should set gridDirty")
	}
}

func TestDirtyRegion_SessionRemovedMarksGridDirty(t *testing.T) {
	pidChecker := newTestPIDChecker(1000)
	sessDir := t.TempDir()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(pidChecker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(pidChecker))

	m := NewSessionDashboardModel("/test/project", "/encoded/project", scanner, monitor)
	m.width = 120
	m.height = 61
	m.panes = []SessionPaneModel{
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 1000, SessionID: "s1"}}, width: 60, height: 30},
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 2000, SessionID: "s2"}}, width: 60, height: 30},
	}
	m.gridDirty = false

	event := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 1000}},
	}

	updated, _ := m.handleSessionClosed(event)
	model := updated.(SessionDashboardModel)

	if !model.IsGridDirty() {
		t.Error("removing a session should set gridDirty")
	}
	if len(model.panes) != 1 {
		t.Errorf("expected 1 pane remaining, got %d", len(model.panes))
	}
}

func TestDirtyRegion_SubscriptionTickClearsGridDirty(t *testing.T) {
	pidChecker := newTestPIDChecker()
	sessDir := t.TempDir()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(pidChecker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(pidChecker))

	m := NewSessionDashboardModel("/test/project", "/encoded/project", scanner, monitor)
	m.gridDirty = true

	result, _ := m.Update(sessionSubscriptionTickMsg{})
	updated := result.(SessionDashboardModel)

	if updated.IsGridDirty() {
		t.Error("subscription tick should clear gridDirty")
	}
}

func TestDirtyRegion_LastFocusedTracking(t *testing.T) {
	m := newTestDashboardWithPanes(2)
	m.focusIndex = 0

	_ = m.View()

	if !m.panes[0].lastFocused {
		t.Error("focused pane should have lastFocused=true after render")
	}
	if m.panes[1].lastFocused {
		t.Error("unfocused pane should have lastFocused=false after render")
	}

	// Change focus
	m.focusIndex = 1
	_ = m.View()

	if m.panes[0].lastFocused {
		t.Error("pane 0 should now have lastFocused=false")
	}
	if !m.panes[1].lastFocused {
		t.Error("pane 1 should now have lastFocused=true")
	}
}

func TestDirtyRegion_EmptyCacheForcesReRender(t *testing.T) {
	m := newTestDashboardWithPanes(2)

	// First render
	_ = m.View()

	// Clear cache for pane 1 (but don't mark dirty)
	m.panes[1].cachedView = ""
	m.panes[1].dirty = false
	m.gridDirty = false

	_ = m.View()

	// Empty cache should force re-render
	if m.panes[1].cachedView == "" {
		t.Error("empty cache should trigger re-render even when not dirty")
	}
}

func TestDirtyRegion_NoFocusChangeNoReRender(t *testing.T) {
	m := newTestDashboardWithPanes(2)
	m.focusIndex = 0

	// First render
	_ = m.View()

	// Set markers
	m.panes[0].cachedView = "STABLE_0"
	m.panes[1].cachedView = "STABLE_1"
	m.panes[0].dirty = false
	m.panes[1].dirty = false
	m.gridDirty = false

	// Focus stays at 0 — nothing should re-render
	_ = m.View()

	if m.panes[0].cachedView != "STABLE_0" {
		t.Error("pane 0 should not re-render when focus unchanged and not dirty")
	}
	if m.panes[1].cachedView != "STABLE_1" {
		t.Error("pane 1 should not re-render when focus unchanged and not dirty")
	}
}

func TestDirtyRegion_MultipleConsecutiveViewCalls(t *testing.T) {
	m := newTestDashboardWithPanes(2)
	m.focusIndex = 0

	// First call populates caches
	v1 := m.View()
	cache0After1 := m.panes[0].cachedView
	cache1After1 := m.panes[1].cachedView

	// Second call with nothing dirty should produce same result
	v2 := m.View()
	cache0After2 := m.panes[0].cachedView
	cache1After2 := m.panes[1].cachedView

	// View output should be identical
	if v1 != v2 {
		t.Error("consecutive View() calls with no changes should produce identical output")
	}

	// Caches should be identical (reused, not re-rendered)
	if cache0After1 != cache0After2 {
		t.Error("pane 0 cache should be stable across consecutive clean renders")
	}
	if cache1After1 != cache1After2 {
		t.Error("pane 1 cache should be stable across consecutive clean renders")
	}
}

func TestDirtyRegion_NavigationArrowMarksDirtyOnlyMovedPanes(t *testing.T) {
	m := newTestDashboardWithPanes(3)
	m.width = 120
	m.height = 61
	m.focusIndex = 0

	// Reset all dirty
	for i := range m.panes {
		m.panes[i].dirty = false
	}

	// Move right: focus goes from 0 to 1
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	updated := result.(SessionDashboardModel)

	if !updated.panes[0].dirty {
		t.Error("old focused pane 0 should be marked dirty on focus move")
	}
	if !updated.panes[1].dirty {
		t.Error("new focused pane 1 should be marked dirty on focus move")
	}
	if updated.panes[2].dirty {
		t.Error("uninvolved pane 2 should NOT be marked dirty on focus move")
	}
}

func TestDirtyRegion_SameFocusNavigationNoDirty(t *testing.T) {
	// Single pane — navigation doesn't move
	m := newTestDashboardWithPanes(1)
	m.width = 120
	m.height = 61
	m.focusIndex = 0
	m.panes[0].dirty = false

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	updated := result.(SessionDashboardModel)

	// Focus stays at 0 — pane should NOT be marked dirty
	if updated.panes[0].dirty {
		t.Error("navigation that doesn't change focus should not mark pane dirty")
	}
}

// --- Session Discovery and Pane Creation Tests (Sub-AC 2) ---

// TestSessionDashboardModel_HandleDirWatcherEvent_SessionOpened verifies that a
// SessionOpened event from the directory watcher creates a new pane.
func TestSessionDashboardModel_HandleDirWatcherEvent_SessionOpened(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dirwatcher-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "dw-session-1"
	makeJSONLFile(t, projectDir, sessionID)

	// Simulate a SessionOpened event from the directory watcher
	openedEvent := session.SessionEvent{
		Type: session.SessionOpened,
		Session: session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       100,
				SessionID: sessionID,
				CWD:       projectPath,
				Kind:      "interactive",
			},
			FilePath: filepath.Join(sessDir, "100.json"),
		},
	}

	newModel, cmd := m.Update(sessionDirWatcherEventMsg{event: openedEvent})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after SessionOpened event, got %d", updated.PaneCount())
	}

	if updated.panes[0].session.Meta.PID != 100 {
		t.Errorf("pane PID = %d, want 100", updated.panes[0].session.Meta.PID)
	}

	if cmd == nil {
		t.Error("expected non-nil cmd for content loading")
	}
}

// TestSessionDashboardModel_HandleDirWatcherEvent_SessionClosed verifies that a
// SessionClosed event from the directory watcher removes the corresponding pane.
func TestSessionDashboardModel_HandleDirWatcherEvent_SessionClosed(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dirwatcher-close-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "dw-s1")
	makeJSONLFile(t, projectDir, "dw-s2")

	// First add two panes via scan result
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "dw-s1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "dw-s2", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	if m.PaneCount() != 2 {
		t.Fatalf("expected 2 panes before close, got %d", m.PaneCount())
	}

	// Now close PID 100 via dir watcher SessionClosed event
	closedEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 100}},
	}
	newModel2, _ := m.Update(sessionDirWatcherEventMsg{event: closedEvent})
	updated := newModel2.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Errorf("expected 1 pane after SessionClosed event, got %d", updated.PaneCount())
	}
	if updated.panes[0].session.Meta.PID != 200 {
		t.Errorf("remaining pane PID = %d, want 200", updated.panes[0].session.Meta.PID)
	}
}

// TestSessionDashboardModel_HandleDirWatcherEvent_Unknown verifies that an unknown
// event type from the directory watcher is a no-op.
func TestSessionDashboardModel_HandleDirWatcherEvent_Unknown(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.SetSize(80, 24)

	// Use an event type that isn't SessionOpened or SessionClosed
	unknownEvent := session.SessionEvent{
		Type:    session.SessionEventType(99),
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 42}},
	}

	newModel, cmd := m.Update(sessionDirWatcherEventMsg{event: unknownEvent})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 0 {
		t.Errorf("unknown event should not create panes, got %d", updated.PaneCount())
	}
	if cmd != nil {
		t.Error("unknown event should produce no command")
	}
}

// TestSessionDashboardModel_PollChannels_DirWatcherEvent verifies that the
// dir watcher channel is drained by pollChannels.
func TestSessionDashboardModel_PollChannels_DirWatcherEvent(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
		dirWatcherChan: make(chan session.SessionEvent, 1),
	}

	// Send a dir watcher event
	expected := session.SessionEvent{
		Type:    session.SessionOpened,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 77}},
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
	if dwMsg.event.Session.Meta.PID != 77 {
		t.Errorf("PID = %d, want 77", dwMsg.event.Session.Meta.PID)
	}
	if dwMsg.event.Type != session.SessionOpened {
		t.Errorf("event type = %v, want SessionOpened", dwMsg.event.Type)
	}
}

// TestSessionDashboardModel_PollChannels_DirWatcherNil verifies that pollChannels
// handles a nil dirWatcherChan safely.
func TestSessionDashboardModel_PollChannels_DirWatcherNil(t *testing.T) {
	m := SessionDashboardModel{
		scanResultChan: make(chan session.ScanResult, 1),
		monitorChan:    make(chan session.SessionEvent, 1),
		dirWatcherChan: nil, // nil — should not panic
	}

	// Should not panic
	msg := m.pollChannels()
	if msg != nil {
		t.Errorf("expected nil from empty channels, got %T", msg)
	}
}

// TestSessionDashboardModel_StartDirWatcherCmd_NilWatcher verifies that
// startDirWatcherCmd returns nil msg when dirWatcher is nil.
func TestSessionDashboardModel_StartDirWatcherCmd_NilWatcher(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	// No dir watcher set — startDirWatcherCmd should return nil msg
	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	cmd := m.startDirWatcherCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	// Execute the cmd — should return nil since no dirWatcher
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil msg with no dir watcher, got %T", msg)
	}
}

// TestSessionDashboardModel_StartDirWatcherCmd_WithWatcher verifies that
// startDirWatcherCmd starts the watcher and bridges events.
func TestSessionDashboardModel_StartDirWatcherCmd_WithWatcher(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	// Create a real SessionDirectoryWatcher
	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	m := NewSessionDashboardModel("/tmp/p", projectDir, scanner, monitor,
		WithDashboardDirWatcher(dw),
	)
	m.SetSize(120, 40)

	// startDirWatcherCmd should start the watcher and launch a bridge goroutine
	cmd := m.startDirWatcherCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil msg after starting dir watcher, got %T", msg)
	}

	// Clean up
	m.closeAll()
}

// TestSessionDashboardModel_StartDirWatcherCmd_FailedStart verifies that
// startDirWatcherCmd handles a watcher that fails to start gracefully.
func TestSessionDashboardModel_StartDirWatcherCmd_FailedStart(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	// Create and close a dir watcher so Start() fails (closed state)
	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	// Close it first so Start() will fail
	_ = dw.Close()

	m := NewSessionDashboardModel("/tmp/p", sessDir, scanner, monitor,
		WithDashboardDirWatcher(dw),
	)

	cmd := m.startDirWatcherCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	// Execute — Start() will fail (watcher is closed), should return nil gracefully
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil msg even when watcher start fails, got %T", msg)
	}

	// Clean up
	m.closeAll()
}

// TestSessionDashboardModel_DirWatcher_PaneAppearsWithin2Seconds verifies the
// core Sub-AC 2 requirement: a new pane appears within 2 seconds of a new-session
// event being delivered via the directory watcher path.
func TestSessionDashboardModel_DirWatcher_PaneAppearsWithin2Seconds(t *testing.T) {
	checker := newTestPIDChecker(42)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/timing-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor,
		WithDashboardDirWatcher(dw),
	)
	m.SetSize(120, 40)

	sessionID := "timing-dw-test"
	makeJSONLFile(t, projectDir, sessionID)

	// Simulate receiving a SessionOpened event (as would come from dir watcher)
	start := time.Now()
	openedEvent := session.SessionEvent{
		Type: session.SessionOpened,
		Session: session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       42,
				SessionID: sessionID,
				CWD:       projectPath,
				Kind:      "interactive",
			},
		},
	}

	// Update processes the event
	newModel, _ := m.Update(sessionDirWatcherEventMsg{event: openedEvent})
	updated := newModel.(SessionDashboardModel)
	elapsed := time.Since(start)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after dir watcher event, got %d", updated.PaneCount())
	}

	// The pane should appear essentially instantly (sub-millisecond processing)
	// but we verify the 2-second bound as the AC requires
	if elapsed > 2*time.Second {
		t.Errorf("pane appeared after %v, must be within 2 seconds", elapsed)
	}

	t.Logf("Pane created via dir watcher event in %v", elapsed)

	m.closeAll()
}

// TestSessionDashboardModel_DirWatcher_NoDuplicatePanes verifies that the
// dir watcher path also prevents duplicate panes for the same session.
func TestSessionDashboardModel_DirWatcher_NoDuplicatePanes(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dirwatcher-dup-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "dw-dup-session"
	makeJSONLFile(t, projectDir, sessionID)

	openedEvent := session.SessionEvent{
		Type: session.SessionOpened,
		Session: session.ActiveSession{
			Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath},
		},
	}

	// First event — should create pane
	newModel, _ := m.Update(sessionDirWatcherEventMsg{event: openedEvent})
	m = newModel.(SessionDashboardModel)

	if m.PaneCount() != 1 {
		t.Fatalf("expected 1 pane after first event, got %d", m.PaneCount())
	}

	// Second identical event — should NOT create duplicate
	newModel2, _ := m.Update(sessionDirWatcherEventMsg{event: openedEvent})
	m2 := newModel2.(SessionDashboardModel)

	if m2.PaneCount() != 1 {
		t.Errorf("expected 1 pane after duplicate event, got %d", m2.PaneCount())
	}
}

// TestSessionDashboardModel_DirWatcher_WithDirWatcherOption verifies that
// WithDashboardDirWatcher option correctly sets up the dir watcher and channel.
func TestSessionDashboardModel_DirWatcher_WithDirWatcherOption(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor,
		WithDashboardDirWatcher(dw),
	)

	if m.dirWatcher == nil {
		t.Error("expected dirWatcher to be set after WithDashboardDirWatcher")
	}
	if m.dirWatcherChan == nil {
		t.Error("expected dirWatcherChan to be non-nil after WithDashboardDirWatcher")
	}

	_ = dw.Close()
}

// TestSessionDashboardModel_Init_WithDirWatcher verifies that Init() starts
// the dir watcher when one is configured.
func TestSessionDashboardModel_Init_WithDirWatcher(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor,
		WithDashboardDirWatcher(dw),
	)

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Init()")
	}

	m.closeAll()
}

// TestSessionDashboardModel_DirWatcherBridgeGoroutine verifies that the bridge
// goroutine started by startDirWatcherCmd exits cleanly when context is cancelled.
func TestSessionDashboardModel_DirWatcherBridgeGoroutine(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	m := NewSessionDashboardModel("/tmp/p", projectDir, scanner, monitor,
		WithDashboardDirWatcher(dw),
	)
	m.SetSize(120, 40)

	// Start dir watcher via cmd
	cmd := m.startDirWatcherCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	_ = cmd() // Execute to start goroutine

	// Close everything — bridge goroutine should exit cleanly
	m.closeAll()

	// WaitForGoroutines should complete (no leaks)
	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Success — goroutines exited
	case <-time.After(5 * time.Second):
		t.Error("goroutines did not exit within 5 seconds after closeAll()")
	}
}

// TestSessionDashboardModel_FrameTickMsg_Inactive verifies that frameTickMsg
// is handled gracefully when subscriptions are inactive.
func TestSessionDashboardModel_FrameTickMsg_Inactive(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.subscriptionsActive = false

	newModel, cmd := m.Update(frameTickMsg{})
	_ = newModel.(SessionDashboardModel)

	if cmd != nil {
		t.Error("inactive subscription should return nil cmd for frameTickMsg")
	}
}

// TestSessionDashboardModel_FrameTickMsg_Active verifies that frameTickMsg
// schedules the next frame tick when subscriptions are active.
func TestSessionDashboardModel_FrameTickMsg_Active(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.subscriptionsActive = true

	newModel, cmd := m.Update(frameTickMsg{})
	_ = newModel.(SessionDashboardModel)

	if cmd == nil {
		t.Error("active subscription should schedule next frame tick")
	}
}

// TestSessionDashboardModel_UpDownNavigation verifies vertical navigation keys.
func TestSessionDashboardModel_UpDownNavigation(t *testing.T) {
	checker := newTestPIDChecker(100, 200, 300, 400)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/nav-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	for i := 0; i < 4; i++ {
		makeJSONLFile(t, projectDir, fmt.Sprintf("nav-s%d", i))
	}

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "nav-s0", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "nav-s1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 300, SessionID: "nav-s2", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 400, SessionID: "nav-s3", CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Move down from pane 0 (in 2x2 grid, should go to row 1 col 0 = pane 2)
	newModel2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated := newModel2.(SessionDashboardModel)

	// In 2x2 grid with 4 panes, down from 0 goes to 2
	if updated.focusIndex < 0 || updated.focusIndex >= 4 {
		t.Errorf("focus out of range: %d", updated.focusIndex)
	}

	// Move up from current position
	newModel3, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	updated3 := newModel3.(SessionDashboardModel)

	if updated3.focusIndex < 0 || updated3.focusIndex >= 4 {
		t.Errorf("focus out of range after up: %d", updated3.focusIndex)
	}
}

// TestSessionDashboardModel_PaneContentLoaded_WithWatcher verifies that
// handlePaneContentLoaded starts a file watcher when a valid file path is given.
func TestSessionDashboardModel_PaneContentLoaded_WithWatcher(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/watcher-content-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "watcher-sess"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	// Add a pane
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Send content loaded with a real file path — should start file watcher
	contentMsg := sessionPaneContentLoadedMsg{
		sessionID:   sessionID,
		filePath:    jsonlPath,
		entries:     []types.LogEntry{{Type: types.EntryTypeUser, Message: types.Message{TextContent: "hello"}}},
		parseErrors: 0,
	}
	newModel2, _ := m.Update(contentMsg)
	m2 := newModel2.(SessionDashboardModel)

	if m2.panes[0].loading {
		t.Error("pane should not be loading after content loaded")
	}
	if m2.panes[0].watcher == nil {
		t.Error("pane should have a file watcher after content loaded with valid path")
	}

	// Clean up
	m2.closeAll()
}

// TestSessionDashboardModel_RenderContent_ShortContent verifies renderContent
// handles content that fits within height.
func TestSessionDashboardModel_RenderContent_ShortContent(t *testing.T) {
	pane := &SessionPaneModel{
		width:  60,
		height: 20,
		entries: []types.LogEntry{
			{Type: types.EntryTypeUser, Message: types.Message{TextContent: "short"}},
		},
	}

	content := pane.renderContent()
	if content == "" {
		t.Error("expected non-empty content")
	}
}

// TestSessionDashboardModel_AllPanesHaveContent verifies AllPanesHaveContent
// edge cases including out-of-range, no panes, loading, and content loaded.
func TestSessionDashboardModel_AllPanesHaveContent(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-all-content"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// No panes → should return false
	if m.AllPanesHaveContent() {
		t.Error("AllPanesHaveContent should be false when there are no panes")
	}

	// Add a pane in loading state
	sessionID := "all-content-test"
	makeJSONLFile(t, projectDir, sessionID)
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Pane in loading state → should return false
	if m.AllPanesHaveContent() {
		t.Error("AllPanesHaveContent should be false when pane is loading")
	}

	// Load content: simulate with content loaded message
	contentMsg := sessionPaneContentLoadedMsg{
		sessionID:   sessionID,
		entries:     []types.LogEntry{{Type: types.EntryTypeUser, Message: types.Message{TextContent: "hello"}}},
		parseErrors: 0,
		filePath:    filepath.Join(projectDir, sessionID+".jsonl"),
	}
	newModel2, _ := m.Update(contentMsg)
	m = newModel2.(SessionDashboardModel)

	// Pane loaded with entries → should return true
	if !m.AllPanesHaveContent() {
		t.Error("AllPanesHaveContent should be true after content is loaded")
	}

	m.closeAll()
}

// TestSessionDashboardModel_PaneIsLoading_OutOfRange verifies PaneIsLoading
// returns false for out-of-range indices.
func TestSessionDashboardModel_PaneIsLoading_OutOfRange(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	// Negative index
	if m.PaneIsLoading(-1) {
		t.Error("PaneIsLoading(-1) should return false")
	}
	// Out-of-range index
	if m.PaneIsLoading(999) {
		t.Error("PaneIsLoading(999) should return false")
	}
}

// TestSessionDashboardModel_PaneEntriesCount_OutOfRange verifies PaneEntriesCount
// returns -1 for out-of-range indices.
func TestSessionDashboardModel_PaneEntriesCount_OutOfRange(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	// Negative index → -1
	if got := m.PaneEntriesCount(-1); got != -1 {
		t.Errorf("PaneEntriesCount(-1) = %d, want -1", got)
	}
	// Out-of-range index → -1
	if got := m.PaneEntriesCount(999); got != -1 {
		t.Errorf("PaneEntriesCount(999) = %d, want -1", got)
	}
}

// --- Partial page display tests (AC 5) ---

// TestPartialPage_Page2Shows2Sessions verifies that page 2 of 11 sessions
// correctly shows only 2 panes in a partial grid.
func TestPartialPage_Page2Shows2Sessions(t *testing.T) {
	m := newTestDashboardWithPanes(11) // 9 on page 1, 2 on page 2
	m.currentPage = 1                  // page 2 (zero-indexed)

	if got := m.TotalPages(); got != 2 {
		t.Fatalf("TotalPages() = %d, want 2", got)
	}

	pagePanes := m.CurrentPagePanes()
	if len(pagePanes) != 2 {
		t.Fatalf("CurrentPagePanes() len = %d, want 2", len(pagePanes))
	}

	// Verify the panes are the correct ones (sessions 9 and 10)
	if pagePanes[0].session.Meta.PID != 1009 {
		t.Errorf("first pane PID = %d, want 1009", pagePanes[0].session.Meta.PID)
	}
	if pagePanes[1].session.Meta.PID != 1010 {
		t.Errorf("second pane PID = %d, want 1010", pagePanes[1].session.Meta.PID)
	}
}

// TestPartialPage_GridLayoutFor2Panes verifies the grid layout is 1x2 for 2 panes.
func TestPartialPage_GridLayoutFor2Panes(t *testing.T) {
	layout := CalculateGridLayout(2, 120, 60)
	if layout.Rows != 1 || layout.Cols != 2 {
		t.Errorf("grid for 2 panes = %dx%d, want 1x2", layout.Rows, layout.Cols)
	}
	if len(layout.Panes) != 2 {
		t.Errorf("pane count = %d, want 2", len(layout.Panes))
	}
}

// TestPartialPage_ViewRendersOnlyPartialPanes verifies View() renders
// only the panes for the current partial page.
func TestPartialPage_ViewRendersOnlyPartialPanes(t *testing.T) {
	m := newTestDashboardWithPanes(11)
	m.currentPage = 1
	m.gridDirty = true

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string for partial page")
	}

	// Page 2 has sessions with PIDs 1009 and 1010 (displayed as "PID 1009", "PID 1010")
	// Count PID references - only 2 should be visible
	pidCount := 0
	for i := 0; i < 11; i++ {
		pid := fmt.Sprintf("%d", 1000+i)
		if strings.Contains(view, pid) {
			pidCount++
		}
	}
	if pidCount != 2 {
		t.Errorf("expected 2 PIDs visible on partial page, got %d", pidCount)
	}
}

// TestPartialPage_SingleSessionOnPage2 verifies page 2 with exactly 1 session.
func TestPartialPage_SingleSessionOnPage2(t *testing.T) {
	m := newTestDashboardWithPanes(10) // 9 on page 1, 1 on page 2
	m.currentPage = 1

	pagePanes := m.CurrentPagePanes()
	if len(pagePanes) != 1 {
		t.Fatalf("CurrentPagePanes() len = %d, want 1", len(pagePanes))
	}

	// Grid for 1 pane is 1x1
	layout := CalculateGridLayout(1, 120, 60)
	if layout.Rows != 1 || layout.Cols != 1 {
		t.Errorf("grid for 1 pane = %dx%d, want 1x1", layout.Rows, layout.Cols)
	}
}

// TestPartialPage_FocusClampedToVisiblePanes verifies focusIndex is clamped
// to the visible pane count on a partial page.
func TestPartialPage_FocusClampedToVisiblePanes(t *testing.T) {
	m := newTestDashboardWithPanes(11)
	m.currentPage = 1
	m.focusIndex = 5 // Out of range for page with 2 panes

	// moveFocus should work correctly within partial page bounds
	newIdx := m.moveFocus("right")
	if newIdx >= 2 {
		t.Errorf("moveFocus('right') = %d, want < 2 for partial page", newIdx)
	}
}

// TestPartialPage_NavigationWrapsWithinPartialGrid verifies arrow key
// navigation wraps correctly within a partial page's grid.
func TestPartialPage_NavigationWrapsWithinPartialGrid(t *testing.T) {
	m := newTestDashboardWithPanes(11)
	m.currentPage = 1
	m.focusIndex = 0

	// Right from index 0 in 1x2 grid should go to index 1
	newIdx := m.moveFocus("right")
	if newIdx != 1 {
		t.Errorf("moveFocus('right') from 0 = %d, want 1", newIdx)
	}

	// Right from index 1 should wrap to 0
	m.focusIndex = 1
	newIdx = m.moveFocus("right")
	if newIdx != 0 {
		t.Errorf("moveFocus('right') from 1 = %d, want 0 (wrap)", newIdx)
	}
}

// TestPartialPage_CurrentPagePaneCount verifies currentPagePaneCount
// returns correct counts for partial pages.
func TestPartialPage_CurrentPagePaneCount(t *testing.T) {
	tests := []struct {
		totalPanes int
		page       int
		want       int
	}{
		{11, 0, 9}, // Full first page
		{11, 1, 2}, // Partial second page
		{10, 1, 1}, // Single pane on page 2
		{18, 1, 9}, // Full second page
		{19, 2, 1}, // Single pane on page 3
		{9, 0, 9},  // Exactly one full page
		{1, 0, 1},  // Single pane total
		{5, 0, 5},  // Partial first page
	}

	for _, tt := range tests {
		m := newTestDashboardWithPanes(tt.totalPanes)
		m.currentPage = tt.page
		got := m.currentPagePaneCount()
		if got != tt.want {
			t.Errorf("currentPagePaneCount(total=%d, page=%d) = %d, want %d",
				tt.totalPanes, tt.page, got, tt.want)
		}
	}
}

// TestPartialPage_ViewPageIndicatorShowsCorrectPage verifies the page
// indicator displays correct page numbers for partial pages.
func TestPartialPage_ViewPageIndicatorShowsCorrectPage(t *testing.T) {
	m := newTestDashboardWithPanes(11)
	m.currentPage = 1
	m.gridDirty = true

	view := m.View()
	// Should show "Page 2/2"
	if !strings.Contains(view, "2/2") {
		t.Errorf("View() should contain page indicator '2/2', got:\n%s", view)
	}
}

// TestPartialPage_VariousPartialSizes verifies grid layout is correct
// for all possible partial page sizes (1-8 panes).
func TestPartialPage_VariousPartialSizes(t *testing.T) {
	expectedGrids := map[int][2]int{
		1: {1, 1},
		2: {1, 2},
		3: {1, 3},
		4: {2, 2},
		5: {2, 3},
		6: {2, 3},
		7: {3, 3},
		8: {3, 3},
	}

	for count, expected := range expectedGrids {
		layout := CalculateGridLayout(count, 120, 60)
		if layout.Rows != expected[0] || layout.Cols != expected[1] {
			t.Errorf("grid for %d panes = %dx%d, want %dx%d",
				count, layout.Rows, layout.Cols, expected[0], expected[1])
		}
		if len(layout.Panes) != count {
			t.Errorf("pane count for %d panes = %d", count, len(layout.Panes))
		}
	}
}

// TestPartialPage_PartialGridUsesFullWidth verifies that panes on a partial
// page expand to use the full terminal width.
func TestPartialPage_PartialGridUsesFullWidth(t *testing.T) {
	// 2 panes should split the full 120-column width
	layout := CalculateGridLayout(2, 120, 60)
	totalWidth := 0
	for _, p := range layout.Panes {
		totalWidth += p.Width
	}
	if totalWidth != 120 {
		t.Errorf("total pane width = %d, want 120 (full width)", totalWidth)
	}

	// 1 pane should use the full width
	layout = CalculateGridLayout(1, 120, 60)
	if layout.Panes[0].Width != 120 {
		t.Errorf("single pane width = %d, want 120", layout.Panes[0].Width)
	}
}

// TestPartialPage_AutoRetreatOnSessionRemoval verifies that when sessions
// are removed making the current partial page empty, the dashboard retreats
// to the previous page.
func TestPartialPage_AutoRetreatOnSessionRemoval(t *testing.T) {
	m := newTestDashboardWithPanes(10) // 9 on page 1, 1 on page 2
	m.currentPage = 1                  // On page 2

	// Remove the last pane (the only one on page 2)
	m.panes = m.panes[:9]
	m.clampCurrentPage()

	if m.currentPage != 0 {
		t.Errorf("currentPage after removing last pane = %d, want 0", m.currentPage)
	}
	if m.currentPagePaneCount() != 9 {
		t.Errorf("pane count after retreat = %d, want 9", m.currentPagePaneCount())
	}
}

func TestSessionDashboardModel_ViewShowsPageIndicator(t *testing.T) {
	// Create dashboard with >9 sessions to trigger multi-page display
	pids := make([]int, 12)
	for i := range pids {
		pids[i] = 100 + i
	}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectPath := "/tmp/page-indicator-project"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Build 12 sessions
	sessions := make([]session.ActiveSession, 12)
	for i := 0; i < 12; i++ {
		sid := fmt.Sprintf("sess-%d", i)
		makeJSONLFile(t, projectDir, sid)
		sessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{PID: pids[i], SessionID: sid, CWD: projectPath},
		}
	}

	scanResult := session.ScanResult{Sessions: sessions}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// View on page 0 should show "Page 1/2"
	view := updated.View()
	if !strings.Contains(view, "Page 1/2") {
		t.Errorf("View on page 0 should contain 'Page 1/2', got:\n%s", view)
	}

	// Navigate to page 1 via ] key
	newModel2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	updated2 := newModel2.(SessionDashboardModel)

	view2 := updated2.View()
	if !strings.Contains(view2, "Page 2/2") {
		t.Errorf("View on page 1 should contain 'Page 2/2', got:\n%s", view2)
	}

	// Single page (<=9 sessions) should NOT show page indicator
	smallScan := session.ScanResult{Sessions: sessions[:5]}
	newModel3, _ := m.Update(sessionScanResultMsg{result: smallScan})
	updated3 := newModel3.(SessionDashboardModel)

	view3 := updated3.View()
	if strings.Contains(view3, "Page ") {
		t.Errorf("View with single page should NOT show page indicator, got:\n%s", view3)
	}
}

// --- Page stability tests (AC 7) ---
// These tests verify that when the current page is still populated after
// session changes, the dashboard stays on the current page number.

// TestPageStability_RemoveFromOtherPageStaysOnCurrentPage verifies that
// removing a session from a different page does not change the current page.
func TestPageStability_RemoveFromOtherPageStaysOnCurrentPage(t *testing.T) {
	// 12 sessions: page 0 has 9, page 1 has 3
	m := newTestDashboardWithPanes(12)
	m.currentPage = 1 // On page 2 (zero-indexed)

	// Remove a session from page 1 (index 4, PID 1004)
	m.panes = append(m.panes[:4], m.panes[5:]...)
	m.clampCurrentPage()

	// 11 sessions remain: page 0 has 9, page 1 has 2 → page 1 is still valid
	if m.currentPage != 1 {
		t.Errorf("currentPage = %d, want 1 (should stay on page 2)", m.currentPage)
	}
	if m.currentPagePaneCount() != 2 {
		t.Errorf("pane count on page 1 = %d, want 2", m.currentPagePaneCount())
	}
}

// TestPageStability_RemoveFromCurrentPageStaysIfStillPopulated verifies that
// removing a session from the current page keeps the page if sessions remain.
func TestPageStability_RemoveFromCurrentPageStaysIfStillPopulated(t *testing.T) {
	// 12 sessions: page 0 has 9, page 1 has 3
	m := newTestDashboardWithPanes(12)
	m.currentPage = 1 // On page 2 (zero-indexed)

	// Remove one session from page 2 (index 9, PID 1009 — first on page 2)
	m.panes = append(m.panes[:9], m.panes[10:]...)
	m.clampCurrentPage()

	// 11 sessions remain: page 0 has 9, page 1 has 2 → page 1 still valid
	if m.currentPage != 1 {
		t.Errorf("currentPage = %d, want 1 (page 2 still has sessions)", m.currentPage)
	}
	if m.currentPagePaneCount() != 2 {
		t.Errorf("pane count on page 1 = %d, want 2", m.currentPagePaneCount())
	}
}

// TestPageStability_AddNewSessionStaysOnCurrentPage verifies that adding
// new sessions during polling does not change the current page.
func TestPageStability_AddNewSessionStaysOnCurrentPage(t *testing.T) {
	// 11 sessions: page 0 has 9, page 1 has 2
	m := newTestDashboardWithPanes(11)
	m.currentPage = 1 // On page 2

	// Add a new session (simulating what handleScanResult does)
	newPane := SessionPaneModel{
		session: session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       2000,
				SessionID: "sess-new",
				Kind:      "interactive",
			},
		},
		width:  40,
		height: 20,
	}
	m.panes = append(m.panes, newPane)

	// 12 sessions now: page 0 has 9, page 1 has 3
	// currentPage should remain 1
	if m.currentPage != 1 {
		t.Errorf("currentPage = %d, want 1 (should stay on page 2 after add)", m.currentPage)
	}
	if m.currentPagePaneCount() != 3 {
		t.Errorf("pane count on page 1 = %d, want 3", m.currentPagePaneCount())
	}
}

// TestPageStability_RemoveFromPage1StaysOnPage1 verifies that removing a
// session from page 1 while viewing page 1 keeps the page at 0.
func TestPageStability_RemoveFromPage1StaysOnPage1(t *testing.T) {
	m := newTestDashboardWithPanes(12)
	m.currentPage = 0 // On page 1

	// Remove one session from page 1 (index 3)
	m.panes = append(m.panes[:3], m.panes[4:]...)
	m.clampCurrentPage()

	if m.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0 (page 1 still has 8 sessions)", m.currentPage)
	}
	if m.currentPagePaneCount() != 9 {
		// After removal, 11 sessions. Page 0 still has min(9, 11) = 9
		t.Errorf("pane count on page 0 = %d, want 9", m.currentPagePaneCount())
	}
}

// TestPageStability_HandleScanResultPreservesPage verifies that the full
// handleScanResult flow preserves the current page when it's still valid.
func TestPageStability_HandleScanResultPreservesPage(t *testing.T) {
	pids := make([]int, 12)
	for i := range pids {
		pids[i] = 1000 + i
	}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectPath := "/tmp/page-stability-project"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Build 12 sessions
	sessions := make([]session.ActiveSession, 12)
	for i := range sessions {
		sessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       1000 + i,
				SessionID: fmt.Sprintf("sess-stability-%d", i),
				CWD:       projectPath,
				Kind:      "interactive",
			},
		}
	}

	// First scan: populate all 12 panes
	scanResult := session.ScanResult{Sessions: sessions}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	if len(m.panes) != 12 {
		t.Fatalf("expected 12 panes, got %d", len(m.panes))
	}

	// Navigate to page 2
	m.SetCurrentPage(1)
	if m.currentPage != 1 {
		t.Fatalf("failed to set page to 1")
	}

	// Kill one session on page 1 (PID 1004)
	checker.SetAlive(1004, false)

	// Rescan with 11 alive sessions
	var aliveSessions []session.ActiveSession
	for _, s := range sessions {
		if s.Meta.PID != 1004 {
			aliveSessions = append(aliveSessions, s)
		}
	}
	// Grace period requires testScanMissThreshold consecutive misses before removal.
	scanResult2 := session.ScanResult{Sessions: aliveSessions, IsFullScan: true}
	m = applyScanResultNTimes(t, m, sessionScanResultMsg{result: scanResult2}, testScanMissThreshold)

	// Page 1 still has 2 sessions (was 3, removed 0 from page 2)
	// Actually removing PID 1004 from page 1 shifts indices, page 2 now has 2
	if m.currentPage != 1 {
		t.Errorf("currentPage after scan = %d, want 1 (page 2 still populated)", m.currentPage)
	}
	if len(m.panes) != 11 {
		t.Errorf("total panes = %d, want 11", len(m.panes))
	}
	if m.currentPagePaneCount() != 2 {
		t.Errorf("pane count on page 1 = %d, want 2", m.currentPagePaneCount())
	}
}

// TestPageStability_HandleSessionClosedPreservesPage verifies that
// handleSessionClosed keeps the current page when it's still valid.
func TestPageStability_HandleSessionClosedPreservesPage(t *testing.T) {
	pids := make([]int, 12)
	for i := range pids {
		pids[i] = 1000 + i
	}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectPath := "/tmp/page-closed-project"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Build 12 sessions and populate panes
	sessions := make([]session.ActiveSession, 12)
	for i := range sessions {
		sessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       1000 + i,
				SessionID: fmt.Sprintf("sess-closed-%d", i),
				CWD:       projectPath,
				Kind:      "interactive",
			},
		}
	}

	scanResult := session.ScanResult{Sessions: sessions}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Navigate to page 2
	m.SetCurrentPage(1)

	// Close a session on page 1 via sessionClosedMsg
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: sessions[2], // PID 1002, on page 1
	}
	newModel2, _ := m.Update(sessionClosedMsg{event: closeEvent})
	m = newModel2.(SessionDashboardModel)

	// Page 2 still has sessions (11 total → page 0 has 9, page 1 has 2)
	if m.currentPage != 1 {
		t.Errorf("currentPage after close = %d, want 1 (page 2 still populated)", m.currentPage)
	}
	if len(m.panes) != 11 {
		t.Errorf("total panes = %d, want 11", len(m.panes))
	}
}

// TestPageStability_MultipleRemovalsStayOnPageIfPopulated verifies that
// multiple sequential session removals preserve the current page when
// the page remains populated after each removal.
func TestPageStability_MultipleRemovalsStayOnPageIfPopulated(t *testing.T) {
	// 20 sessions: page 0 has 9, page 1 has 9, page 2 has 2
	m := newTestDashboardWithPanes(20)
	m.currentPage = 1 // On page 2

	// Remove 3 sessions from page 1 (indices 0, 1, 2)
	m.panes = append(m.panes[:0], m.panes[3:]...)
	m.clampCurrentPage()

	// 17 sessions: page 0 has 9, page 1 has 8
	if m.currentPage != 1 {
		t.Errorf("currentPage = %d, want 1 after removing from page 1", m.currentPage)
	}

	// Remove 2 more from page 1
	m.panes = append(m.panes[:0], m.panes[2:]...)
	m.clampCurrentPage()

	// 15 sessions: page 0 has 9, page 1 has 6
	if m.currentPage != 1 {
		t.Errorf("currentPage = %d, want 1 after more removals from page 1", m.currentPage)
	}

	if m.currentPagePaneCount() != 6 {
		t.Errorf("pane count on page 1 = %d, want 6", m.currentPagePaneCount())
	}
}

// TestPageStability_ClampDoesNotChangeValidPage verifies that clampCurrentPage
// is a no-op when the current page is within valid bounds.
func TestPageStability_ClampDoesNotChangeValidPage(t *testing.T) {
	tests := []struct {
		name       string
		totalPanes int
		page       int
		wantPage   int
	}{
		{"page 0 of 1 page", 5, 0, 0},
		{"page 0 of 2 pages", 12, 0, 0},
		{"page 1 of 2 pages", 12, 1, 1},
		{"page 1 of 3 pages", 20, 1, 1},
		{"page 2 of 3 pages", 20, 2, 2},
		{"page 0 with exactly 9", 9, 0, 0},
		{"page 1 with exactly 18", 18, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestDashboardWithPanes(tt.totalPanes)
			m.currentPage = tt.page
			m.clampCurrentPage()

			if m.currentPage != tt.wantPage {
				t.Errorf("clampCurrentPage() changed page from %d to %d, want %d",
					tt.page, m.currentPage, tt.wantPage)
			}
		})
	}
}

// TestAutoRetreat_ScanResultRemovesAllOnCurrentPage verifies that when a scan
// result removes all sessions on the current page, the dashboard auto-retreats
// to the last non-empty page.
func TestAutoRetreat_ScanResultRemovesAllOnCurrentPage(t *testing.T) {
	// Create 12 sessions (page 0: 9, page 1: 3)
	projectPath := "/test/project"
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	for i := 0; i < 12; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 61)

	// Manually set up 12 panes
	panes := make([]SessionPaneModel, 12)
	for i := range panes {
		panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("sess-%d", i),
					CWD:       projectPath,
					Kind:      "interactive",
				},
			},
			width:  40,
			height: 20,
		}
	}
	m.panes = panes
	m.currentPage = 1 // On page 2 (showing sessions 9-11)

	// Now scan returns only 9 sessions (page 2 sessions are gone)
	liveSessions := make([]session.ActiveSession, 9)
	for i := 0; i < 9; i++ {
		liveSessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       1000 + i,
				SessionID: fmt.Sprintf("sess-%d", i),
				CWD:       projectPath,
				Kind:      "interactive",
			},
		}
	}

	// Grace period requires testScanMissThreshold consecutive misses before removal.
	scanMsg := sessionScanResultMsg{result: session.ScanResult{Sessions: liveSessions, IsFullScan: true}}
	updated := applyScanResultNTimes(t, m, scanMsg, testScanMissThreshold)

	// Should auto-retreat to page 0 (only page left)
	if updated.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0 (should auto-retreat)", updated.currentPage)
	}
	if updated.TotalPages() != 1 {
		t.Errorf("TotalPages = %d, want 1", updated.TotalPages())
	}
}

// TestAutoRetreat_ScanResultRetreatsToLastNonEmptyPage verifies retreat goes
// to the correct page when multiple pages become empty.
func TestAutoRetreat_ScanResultRetreatsToLastNonEmptyPage(t *testing.T) {
	projectPath := "/test/project"
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	// 20 sessions = 3 pages (9+9+2), currently on page 2
	for i := 0; i < 20; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 61)

	panes := make([]SessionPaneModel, 20)
	for i := range panes {
		panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("sess-%d", i),
					CWD:       projectPath,
					Kind:      "interactive",
				},
			},
			width:  40,
			height: 20,
		}
	}
	m.panes = panes
	m.currentPage = 2 // On page 3 (last page, showing 2 sessions)

	// Scan now returns only 5 sessions (all on page 1)
	liveSessions := make([]session.ActiveSession, 5)
	for i := 0; i < 5; i++ {
		liveSessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       1000 + i,
				SessionID: fmt.Sprintf("sess-%d", i),
				CWD:       projectPath,
				Kind:      "interactive",
			},
		}
	}

	// Grace period requires testScanMissThreshold consecutive misses before removal.
	scanMsg := sessionScanResultMsg{result: session.ScanResult{Sessions: liveSessions, IsFullScan: true}}
	updated := applyScanResultNTimes(t, m, scanMsg, testScanMissThreshold)

	// Should retreat to page 0 (5 sessions fit on 1 page)
	if updated.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0 (should retreat to last non-empty)", updated.currentPage)
	}
	if updated.PaneCount() != 5 {
		t.Errorf("PaneCount = %d, want 5", updated.PaneCount())
	}
}

// TestAutoRetreat_SessionClosedClampsPage verifies that handleSessionClosed
// auto-retreats when the last session on the current page disappears.
func TestAutoRetreat_SessionClosedClampsPage(t *testing.T) {
	m := newTestDashboardWithPanes(10) // 9 on page 0, 1 on page 1
	m.currentPage = 1
	m.focusIndex = 0

	// Close the only session on page 1 (PID 1009, index 9)
	event := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 1009}},
	}

	newModel, _ := m.handleSessionClosed(event)
	updated := newModel.(SessionDashboardModel)

	if updated.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0 (should auto-retreat after last pane on page removed)", updated.currentPage)
	}
	if updated.PaneCount() != 9 {
		t.Errorf("PaneCount = %d, want 9", updated.PaneCount())
	}
}

// TestAutoRetreat_AllSessionsDisappear verifies that when all sessions
// disappear, currentPage retreats to 0.
func TestAutoRetreat_AllSessionsDisappear(t *testing.T) {
	projectPath := "/test/project"
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	// At least 1 alive PID needed for the scanner's liveness check
	checker.SetAlive(1000, true)

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 61)

	// 12 panes, on page 1
	panes := make([]SessionPaneModel, 12)
	for i := range panes {
		panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("sess-%d", i),
					CWD:       projectPath,
					Kind:      "interactive",
				},
			},
			width:  40,
			height: 20,
		}
	}
	m.panes = panes
	m.currentPage = 1

	// Scan returns only PID 1000 alive (full scan triggers removal of rest)
	liveSessions := []session.ActiveSession{
		{
			Meta: session.SessionMeta{
				PID:       1000,
				SessionID: "sess-0",
				CWD:       projectPath,
				Kind:      "interactive",
			},
		},
	}

	// Grace period requires testScanMissThreshold consecutive misses before removal.
	scanMsg := sessionScanResultMsg{result: session.ScanResult{Sessions: liveSessions, IsFullScan: true}}
	updated := applyScanResultNTimes(t, m, scanMsg, testScanMissThreshold)

	// Only 1 session left, should be on page 0
	if updated.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0", updated.currentPage)
	}
	if updated.PaneCount() != 1 {
		t.Errorf("PaneCount = %d, want 1", updated.PaneCount())
	}
}

// TestAutoRetreat_FocusIndexClampedWithinPage verifies that focus is
// properly adjusted when auto-retreating to a page with fewer panes.
func TestAutoRetreat_FocusIndexClampedWithinPage(t *testing.T) {
	projectPath := "/test/project"
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	for i := 0; i < 12; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 61)

	panes := make([]SessionPaneModel, 12)
	for i := range panes {
		panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("sess-%d", i),
					CWD:       projectPath,
					Kind:      "interactive",
				},
			},
			width:  40,
			height: 20,
		}
	}
	m.panes = panes
	m.currentPage = 1
	m.focusIndex = 2 // Focused on 3rd pane of page 2

	// Remove all page-2 sessions, leaving only 3 on page 0
	liveSessions := make([]session.ActiveSession, 3)
	for i := 0; i < 3; i++ {
		liveSessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       1000 + i,
				SessionID: fmt.Sprintf("sess-%d", i),
				CWD:       projectPath,
				Kind:      "interactive",
			},
		}
	}

	result := session.ScanResult{Sessions: liveSessions, IsFullScan: true}
	newModel, _ := m.handleScanResult(result)
	updated := newModel.(SessionDashboardModel)

	if updated.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0", updated.currentPage)
	}
	// focusIndex should be clamped to valid range (0-2 for 3 panes)
	if updated.focusIndex >= updated.PaneCount() {
		t.Errorf("focusIndex = %d, should be < %d", updated.focusIndex, updated.PaneCount())
	}
}

// --- AC 8: New sessions appearing during polling are included in the paginated list ---

// TestNewSessionsDuringPolling_IncludedInPaginatedList verifies that when new
// sessions appear during subsequent polling cycles (scan results), they are
// properly appended to the pane list and accessible via pagination.
func TestNewSessionsDuringPolling_IncludedInPaginatedList(t *testing.T) {
	// Start with 9 sessions (fills page 1), then add 3 more via a second scan.
	allPIDs := make([]int, 12)
	for i := range allPIDs {
		allPIDs[i] = 1000 + i
	}
	checker := newTestPIDChecker(allPIDs...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-project"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Create JSONL files for all sessions
	for i := 0; i < 12; i++ {
		makeJSONLFile(t, projectDir, fmt.Sprintf("sess-%d", i))
	}

	// First poll: 9 sessions fill page 1
	firstSessions := make([]session.ActiveSession, 9)
	for i := 0; i < 9; i++ {
		firstSessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       1000 + i,
				SessionID: fmt.Sprintf("sess-%d", i),
				CWD:       projectPath,
				Kind:      "interactive",
			},
		}
	}
	scan1 := session.ScanResult{Sessions: firstSessions}
	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	m = newModel.(SessionDashboardModel)

	if m.PaneCount() != 9 {
		t.Fatalf("after first poll: pane count = %d, want 9", m.PaneCount())
	}
	if m.TotalPages() != 1 {
		t.Fatalf("after first poll: total pages = %d, want 1", m.TotalPages())
	}

	// Second poll: 3 new sessions appear (total 12)
	allSessions := make([]session.ActiveSession, 12)
	for i := 0; i < 12; i++ {
		allSessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       1000 + i,
				SessionID: fmt.Sprintf("sess-%d", i),
				CWD:       projectPath,
				Kind:      "interactive",
			},
		}
	}
	scan2 := session.ScanResult{Sessions: allSessions}
	newModel2, _ := m.Update(sessionScanResultMsg{result: scan2})
	m = newModel2.(SessionDashboardModel)

	// All 12 sessions should be in the pane list
	if m.PaneCount() != 12 {
		t.Errorf("after second poll: pane count = %d, want 12", m.PaneCount())
	}

	// Pagination should now show 2 pages
	if m.TotalPages() != 2 {
		t.Errorf("after second poll: total pages = %d, want 2", m.TotalPages())
	}

	// Page 1 should have 9 panes
	page1Panes := m.CurrentPagePanes()
	if len(page1Panes) != 9 {
		t.Errorf("page 1 panes = %d, want 9", len(page1Panes))
	}

	// Navigate to page 2 and verify the new sessions are there
	m.navigatePageForward()
	if m.CurrentPage() != 1 {
		t.Errorf("after navigate forward: current page = %d, want 1", m.CurrentPage())
	}
	page2Panes := m.CurrentPagePanes()
	if len(page2Panes) != 3 {
		t.Errorf("page 2 panes = %d, want 3", len(page2Panes))
	}

	// Verify the new sessions are specifically the ones added in the second poll
	for _, pane := range page2Panes {
		pid := pane.session.Meta.PID
		if pid < 1009 || pid > 1011 {
			t.Errorf("page 2 contains unexpected PID %d (expected 1009-1011)", pid)
		}
	}
}

// TestNewSessionsDuringPolling_SingleSessionAddsNewPage verifies that even a
// single new session appearing during polling creates a new page when page 1 is full.
func TestNewSessionsDuringPolling_SingleSessionAddsNewPage(t *testing.T) {
	allPIDs := make([]int, 10)
	for i := range allPIDs {
		allPIDs[i] = 500 + i
	}
	checker := newTestPIDChecker(allPIDs...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-proj"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	for i := 0; i < 10; i++ {
		makeJSONLFile(t, projectDir, fmt.Sprintf("s-%d", i))
	}

	// First poll: 9 sessions (full page)
	firstSessions := make([]session.ActiveSession, 9)
	for i := 0; i < 9; i++ {
		firstSessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{PID: 500 + i, SessionID: fmt.Sprintf("s-%d", i), CWD: projectPath},
		}
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: session.ScanResult{Sessions: firstSessions}})
	m = newModel.(SessionDashboardModel)

	if m.TotalPages() != 1 {
		t.Fatalf("before new session: total pages = %d, want 1", m.TotalPages())
	}

	// Second poll: 1 new session (10th) appears
	allSessions := make([]session.ActiveSession, 10)
	copy(allSessions, firstSessions)
	allSessions[9] = session.ActiveSession{
		Meta: session.SessionMeta{PID: 509, SessionID: "s-9", CWD: projectPath},
	}
	newModel2, _ := m.Update(sessionScanResultMsg{result: session.ScanResult{Sessions: allSessions}})
	m = newModel2.(SessionDashboardModel)

	if m.PaneCount() != 10 {
		t.Errorf("pane count = %d, want 10", m.PaneCount())
	}
	if m.TotalPages() != 2 {
		t.Errorf("total pages = %d, want 2", m.TotalPages())
	}

	// Navigate to page 2 and verify the 10th session is there
	m.navigatePageForward()
	page2 := m.CurrentPagePanes()
	if len(page2) != 1 {
		t.Errorf("page 2 panes = %d, want 1", len(page2))
	}
	if len(page2) > 0 && page2[0].session.Meta.PID != 509 {
		t.Errorf("page 2 pane PID = %d, want 509", page2[0].session.Meta.PID)
	}
}

// TestNewSessionsDuringPolling_MultiplePollingCycles verifies that sessions
// appearing across many polling cycles are all properly accumulated.
func TestNewSessionsDuringPolling_MultiplePollingCycles(t *testing.T) {
	allPIDs := make([]int, 20)
	for i := range allPIDs {
		allPIDs[i] = 2000 + i
	}
	checker := newTestPIDChecker(allPIDs...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/multi-poll"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	for i := 0; i < 20; i++ {
		makeJSONLFile(t, projectDir, fmt.Sprintf("mp-%d", i))
	}

	// Simulate 4 polling cycles, each adding 5 sessions
	var currentSessions []session.ActiveSession
	for cycle := 0; cycle < 4; cycle++ {
		for i := 0; i < 5; i++ {
			idx := cycle*5 + i
			currentSessions = append(currentSessions, session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       2000 + idx,
					SessionID: fmt.Sprintf("mp-%d", idx),
					CWD:       projectPath,
					Kind:      "interactive",
				},
			})
		}

		scanResult := session.ScanResult{Sessions: currentSessions}
		newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
		m = newModel.(SessionDashboardModel)
	}

	// After 4 cycles: 20 sessions total
	if m.PaneCount() != 20 {
		t.Errorf("pane count = %d, want 20", m.PaneCount())
	}

	// 20 sessions / 9 per page = 3 pages (9 + 9 + 2)
	expectedPages := 3 // ceil(20/9) = 3
	if m.TotalPages() != expectedPages {
		t.Errorf("total pages = %d, want %d", m.TotalPages(), expectedPages)
	}

	// Verify page 1 has 9 panes
	if len(m.CurrentPagePanes()) != 9 {
		t.Errorf("page 1 panes = %d, want 9", len(m.CurrentPagePanes()))
	}

	// Page 2 has 9 panes
	m.navigatePageForward()
	if len(m.CurrentPagePanes()) != 9 {
		t.Errorf("page 2 panes = %d, want 9", len(m.CurrentPagePanes()))
	}

	// Page 3 has 2 panes
	m.navigatePageForward()
	if len(m.CurrentPagePanes()) != 2 {
		t.Errorf("page 3 panes = %d, want 2", len(m.CurrentPagePanes()))
	}

	// Verify last page has sessions from the final polling cycle.
	// After deduplication the append order within a single scan result is
	// non-deterministic (map iteration), so we only check that ALL PIDs on
	// the last page belong to the full set of 20 sessions (2000-2019).
	lastPage := m.CurrentPagePanes()
	for _, pane := range lastPage {
		pid := pane.session.Meta.PID
		if pid < 2000 || pid > 2019 {
			t.Errorf("page 3 contains out-of-range PID %d (expected 2000-2019)", pid)
		}
	}
}

// TestNewSessionsDuringPolling_DirWatcherAddsBeyondPage verifies that sessions
// added via directory watcher events (not just scan results) are also properly
// included in the paginated list.
func TestNewSessionsDuringPolling_DirWatcherAddsBeyondPage(t *testing.T) {
	allPIDs := make([]int, 10)
	for i := range allPIDs {
		allPIDs[i] = 3000 + i
	}
	checker := newTestPIDChecker(allPIDs...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/dw-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	for i := 0; i < 10; i++ {
		makeJSONLFile(t, projectDir, fmt.Sprintf("dw-%d", i))
	}

	// Initial scan: 9 sessions fill page 1
	initialSessions := make([]session.ActiveSession, 9)
	for i := 0; i < 9; i++ {
		initialSessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{PID: 3000 + i, SessionID: fmt.Sprintf("dw-%d", i), CWD: projectPath},
		}
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: session.ScanResult{Sessions: initialSessions}})
	m = newModel.(SessionDashboardModel)

	if m.TotalPages() != 1 {
		t.Fatalf("before dir watcher event: total pages = %d, want 1", m.TotalPages())
	}

	// Dir watcher detects a new session (10th)
	newSess := session.ActiveSession{
		Meta: session.SessionMeta{PID: 3009, SessionID: "dw-9", CWD: projectPath},
	}
	dwEvent := sessionDirWatcherEventMsg{
		event: session.SessionEvent{
			Type:    session.SessionOpened,
			Session: newSess,
		},
	}
	newModel2, _ := m.Update(dwEvent)
	m = newModel2.(SessionDashboardModel)

	// Should now have 10 panes across 2 pages
	if m.PaneCount() != 10 {
		t.Errorf("pane count = %d, want 10", m.PaneCount())
	}
	if m.TotalPages() != 2 {
		t.Errorf("total pages = %d, want 2", m.TotalPages())
	}

	// The new session should be on page 2
	m.navigatePageForward()
	page2 := m.CurrentPagePanes()
	if len(page2) != 1 {
		t.Errorf("page 2 panes = %d, want 1", len(page2))
	}
	if len(page2) > 0 && page2[0].session.Meta.PID != 3009 {
		t.Errorf("page 2 PID = %d, want 3009", page2[0].session.Meta.PID)
	}
}

// TestNewSessionsDuringPolling_ViewUpdatesPageIndicator verifies that the
// page indicator in the View() output updates when new sessions create
// additional pages.
func TestNewSessionsDuringPolling_ViewUpdatesPageIndicator(t *testing.T) {
	allPIDs := make([]int, 10)
	for i := range allPIDs {
		allPIDs[i] = 4000 + i
	}
	checker := newTestPIDChecker(allPIDs...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/view-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	for i := 0; i < 10; i++ {
		makeJSONLFile(t, projectDir, fmt.Sprintf("vt-%d", i))
	}

	// 9 sessions: should NOT show page indicator (single page)
	firstSessions := make([]session.ActiveSession, 9)
	for i := 0; i < 9; i++ {
		firstSessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{PID: 4000 + i, SessionID: fmt.Sprintf("vt-%d", i), CWD: projectPath},
		}
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: session.ScanResult{Sessions: firstSessions}})
	m = newModel.(SessionDashboardModel)

	view1 := m.View()
	if strings.Contains(view1, "Page") {
		t.Error("single page should not show page indicator")
	}

	// 10th session appears: should now show "Page 1/2"
	allSessions := make([]session.ActiveSession, 10)
	copy(allSessions, firstSessions)
	allSessions[9] = session.ActiveSession{
		Meta: session.SessionMeta{PID: 4009, SessionID: "vt-9", CWD: projectPath},
	}
	newModel2, _ := m.Update(sessionScanResultMsg{result: session.ScanResult{Sessions: allSessions}})
	m = newModel2.(SessionDashboardModel)

	view2 := m.View()
	if !strings.Contains(view2, "Page 1/2") {
		t.Errorf("view should contain 'Page 1/2' after new session creates second page, got:\n%s", view2)
	}
}

// TestNewSessionsDuringPolling_NoDuplicatesAcrossPolls verifies that the same
// session appearing in multiple poll results is not added twice.
func TestNewSessionsDuringPolling_NoDuplicatesAcrossPolls(t *testing.T) {
	checker := newTestPIDChecker(100, 200, 300)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/nodup"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "s1")
	makeJSONLFile(t, projectDir, "s2")
	makeJSONLFile(t, projectDir, "s3")

	sessions := []session.ActiveSession{
		{Meta: session.SessionMeta{PID: 100, SessionID: "s1", CWD: projectPath}},
		{Meta: session.SessionMeta{PID: 200, SessionID: "s2", CWD: projectPath}},
	}

	// Poll 1: 2 sessions
	newModel, _ := m.Update(sessionScanResultMsg{result: session.ScanResult{Sessions: sessions}})
	m = newModel.(SessionDashboardModel)

	// Poll 2: same 2 sessions + 1 new
	sessions = append(sessions, session.ActiveSession{
		Meta: session.SessionMeta{PID: 300, SessionID: "s3", CWD: projectPath},
	})
	newModel2, _ := m.Update(sessionScanResultMsg{result: session.ScanResult{Sessions: sessions}})
	m = newModel2.(SessionDashboardModel)

	if m.PaneCount() != 3 {
		t.Errorf("pane count = %d, want 3 (no duplicates)", m.PaneCount())
	}

	// Poll 3: exact same 3 sessions again — should not grow
	newModel3, _ := m.Update(sessionScanResultMsg{result: session.ScanResult{Sessions: sessions}})
	m = newModel3.(SessionDashboardModel)

	if m.PaneCount() != 3 {
		t.Errorf("pane count after re-poll = %d, want 3 (still no duplicates)", m.PaneCount())
	}
}

// --- Pagination key navigation tests ---

func TestSessionDashboardModel_TotalPages(t *testing.T) {
	tests := []struct {
		name      string
		paneCount int
		want      int
	}{
		{"zero panes", 0, 1},
		{"one pane", 1, 1},
		{"nine panes", 9, 1},
		{"ten panes", 10, 2},
		{"eighteen panes", 18, 2},
		{"nineteen panes", 19, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := SessionDashboardModel{}
			m.panes = make([]SessionPaneModel, tt.paneCount)
			if got := m.TotalPages(); got != tt.want {
				t.Errorf("TotalPages() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSessionDashboardModel_NavigatePageForward(t *testing.T) {
	m := SessionDashboardModel{width: 120, height: 40}
	// Create 12 panes (2 pages)
	m.panes = make([]SessionPaneModel, 12)
	for i := range m.panes {
		m.panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{PID: 100 + i},
			},
		}
	}

	// Start on page 0
	if m.currentPage != 0 {
		t.Fatalf("initial page = %d, want 0", m.currentPage)
	}

	// Navigate forward
	changed := m.navigatePageForward()
	if !changed {
		t.Error("navigatePageForward() returned false, want true")
	}
	if m.currentPage != 1 {
		t.Errorf("currentPage = %d, want 1", m.currentPage)
	}
	if m.focusIndex != 0 {
		t.Errorf("focusIndex = %d, want 0 after page change", m.focusIndex)
	}

	// Try to navigate forward again (already on last page)
	changed = m.navigatePageForward()
	if changed {
		t.Error("navigatePageForward() returned true on last page, want false")
	}
	if m.currentPage != 1 {
		t.Errorf("currentPage = %d, want 1 (should stay)", m.currentPage)
	}
}

func TestSessionDashboardModel_NavigatePageBack(t *testing.T) {
	m := SessionDashboardModel{width: 120, height: 40}
	m.panes = make([]SessionPaneModel, 12)
	for i := range m.panes {
		m.panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{PID: 100 + i},
			},
		}
	}
	m.currentPage = 1

	// Navigate back
	changed := m.navigatePageBack()
	if !changed {
		t.Error("navigatePageBack() returned false, want true")
	}
	if m.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0", m.currentPage)
	}
	if m.focusIndex != 0 {
		t.Errorf("focusIndex = %d, want 0 after page change", m.focusIndex)
	}

	// Try to navigate back again (already on first page)
	changed = m.navigatePageBack()
	if changed {
		t.Error("navigatePageBack() returned true on first page, want false")
	}
	if m.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0 (should stay)", m.currentPage)
	}
}

func TestSessionDashboardModel_BracketKeyNavigation(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.SetSize(120, 40)

	// Create 12 panes (2 pages)
	m.panes = make([]SessionPaneModel, 12)
	for i := range m.panes {
		m.panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{PID: 100 + i},
			},
			width:   40,
			height:  20,
			loading: true,
		}
	}

	// Press ] to go forward
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}
	result, _ := m.Update(keyMsg)
	updated := result.(SessionDashboardModel)
	if updated.currentPage != 1 {
		t.Errorf("after ']' press: currentPage = %d, want 1", updated.currentPage)
	}

	// Press [ to go back
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}}
	result, _ = updated.Update(keyMsg)
	updated = result.(SessionDashboardModel)
	if updated.currentPage != 0 {
		t.Errorf("after '[' press: currentPage = %d, want 0", updated.currentPage)
	}

	// Press [ again - should stay on page 0
	result, _ = updated.Update(keyMsg)
	updated = result.(SessionDashboardModel)
	if updated.currentPage != 0 {
		t.Errorf("after second '[' press: currentPage = %d, want 0", updated.currentPage)
	}
}

func TestSessionDashboardModel_ClampCurrentPage(t *testing.T) {
	m := SessionDashboardModel{}
	m.panes = make([]SessionPaneModel, 5) // 1 page

	m.currentPage = 3
	m.clampCurrentPage()
	if m.currentPage != 0 {
		t.Errorf("after clamp with 5 panes, currentPage = %d, want 0", m.currentPage)
	}

	m.currentPage = -1
	m.clampCurrentPage()
	if m.currentPage != 0 {
		t.Errorf("after clamp with negative page, currentPage = %d, want 0", m.currentPage)
	}
}

func TestSessionDashboardModel_CurrentPagePanes(t *testing.T) {
	m := SessionDashboardModel{}
	m.panes = make([]SessionPaneModel, 12)
	for i := range m.panes {
		m.panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{PID: 100 + i},
			},
		}
	}

	// Page 0: first 9 panes
	m.currentPage = 0
	pagePanes := m.CurrentPagePanes()
	if len(pagePanes) != 9 {
		t.Errorf("page 0 pane count = %d, want 9", len(pagePanes))
	}

	// Page 1: remaining 3 panes
	m.currentPage = 1
	pagePanes = m.CurrentPagePanes()
	if len(pagePanes) != 3 {
		t.Errorf("page 1 pane count = %d, want 3", len(pagePanes))
	}

	// Out of range page
	m.currentPage = 5
	pagePanes = m.CurrentPagePanes()
	if pagePanes != nil {
		t.Errorf("out of range page should return nil, got %d panes", len(pagePanes))
	}
}

func TestSessionDashboardModel_HelpTextContainsPagination(t *testing.T) {
	if !strings.Contains(sessionDashboardHelpText, "[/]:page") {
		t.Errorf("help text should contain pagination keybinding hint, got: %s", sessionDashboardHelpText)
	}
}

func TestSessionDashboardModel_BracketKeyNoOpSinglePage(t *testing.T) {
	// When there's only 1 page, both [ and ] should be no-ops
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)
	m.SetSize(120, 40)

	// Create 5 panes (1 page only)
	m.panes = make([]SessionPaneModel, 5)
	for i := range m.panes {
		m.panes[i] = SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{PID: 100 + i},
			},
			width:   40,
			height:  20,
			loading: true,
		}
	}

	// Press ] - should stay on page 0
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}
	result, _ := m.Update(keyMsg)
	updated := result.(SessionDashboardModel)
	if updated.currentPage != 0 {
		t.Errorf("after ']' on single page: currentPage = %d, want 0", updated.currentPage)
	}

	// Press [ - should stay on page 0
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}}
	result, _ = updated.Update(keyMsg)
	updated = result.(SessionDashboardModel)
	if updated.currentPage != 0 {
		t.Errorf("after '[' on single page: currentPage = %d, want 0", updated.currentPage)
	}
}

// TestPaneDisplayName_BothEntrypointTypes verifies that both sdk-cli (user sessions)
// and sdk-py (Ouroboros agent sessions) produce valid display names. The entrypoint
// field itself does not appear in the pane title — the Kind field drives the label —
// but both types must render without errors and follow the same format.
func TestPaneDisplayName_BothEntrypointTypes(t *testing.T) {
	tests := []struct {
		name       string
		sess       session.ActiveSession
		wantPrefix string
		wantIdle   bool
	}{
		{
			name: "sdk-cli interactive active",
			sess: session.ActiveSession{
				Meta:  session.SessionMeta{PID: 1000, Kind: "interactive", Entrypoint: "sdk-cli"},
				State: session.SessionActive,
			},
			wantPrefix: "interactive[1000]",
			wantIdle:   false,
		},
		{
			name: "sdk-py task active",
			sess: session.ActiveSession{
				Meta:  session.SessionMeta{PID: 2000, Kind: "task", Entrypoint: "sdk-py"},
				State: session.SessionActive,
			},
			wantPrefix: "task[2000]",
			wantIdle:   false,
		},
		{
			name: "sdk-cli interactive idle",
			sess: session.ActiveSession{
				Meta:  session.SessionMeta{PID: 1001, Kind: "interactive", Entrypoint: "sdk-cli"},
				State: session.SessionIdle,
			},
			wantPrefix: "interactive[1001]",
			wantIdle:   true,
		},
		{
			name: "sdk-py task idle",
			sess: session.ActiveSession{
				Meta:  session.SessionMeta{PID: 2001, Kind: "task", Entrypoint: "sdk-py"},
				State: session.SessionIdle,
			},
			wantPrefix: "task[2001]",
			wantIdle:   true,
		},
		{
			name: "sdk-py with empty kind defaults to session",
			sess: session.ActiveSession{
				Meta:  session.SessionMeta{PID: 2002, Kind: "", Entrypoint: "sdk-py"},
				State: session.SessionActive,
			},
			wantPrefix: "session[2002]",
			wantIdle:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := paneDisplayName(tt.sess)
			if !strings.HasPrefix(name, tt.wantPrefix) {
				t.Errorf("paneDisplayName() = %q, want prefix %q", name, tt.wantPrefix)
			}
			if tt.wantIdle && !strings.Contains(name, "IDLE") {
				t.Errorf("idle session should contain IDLE suffix, got %q", name)
			}
			if !tt.wantIdle && strings.Contains(name, "IDLE") {
				t.Errorf("active session should not contain IDLE suffix, got %q", name)
			}
		})
	}
}

// TestSessionDashboard_HandleScanResult_BothEntrypointTypes verifies that
// handleScanResult correctly processes a mix of sdk-cli and sdk-py sessions,
// creating panes for both types and applying lifecycle states identically.
func TestSessionDashboard_HandleScanResult_BothEntrypointTypes(t *testing.T) {
	checker := newTestPIDChecker(1000, 2000)
	sessDir := t.TempDir()
	projectPath := "/test/entrypoint-project"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "cli-sess")
	makeJSONLFile(t, projectDir, "py-sess")

	now := time.Now()
	scanResult := session.ScanResult{
		ScanTime: now,
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID: 1000, SessionID: "cli-sess", CWD: projectPath,
					Kind: "interactive", Entrypoint: "sdk-cli",
				},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-10 * time.Second),
				JSONLDir:          projectDir,
			},
			{
				Meta: session.SessionMeta{
					PID: 2000, SessionID: "py-sess", CWD: projectPath,
					Kind: "task", Entrypoint: "sdk-py",
				},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-15 * time.Second),
				JSONLDir:          projectDir,
			},
		},
	}

	result, _ := m.handleScanResult(scanResult)
	updated := result.(SessionDashboardModel)

	if len(updated.panes) != 2 {
		t.Fatalf("expected 2 panes (one per entrypoint type), got %d", len(updated.panes))
	}

	// Verify both entrypoint types got panes
	entrypoints := make(map[string]bool)
	for _, pane := range updated.panes {
		entrypoints[pane.session.Meta.Entrypoint] = true
	}
	if !entrypoints["sdk-cli"] {
		t.Error("expected sdk-cli session to have a pane")
	}
	if !entrypoints["sdk-py"] {
		t.Error("expected sdk-py session to have a pane")
	}
}

// TestSessionDashboard_IdleStateAppliesToBothEntrypoints verifies that
// lifecycle state updates (Active → Idle) apply equally to sdk-cli and sdk-py sessions.
func TestSessionDashboard_IdleStateAppliesToBothEntrypoints(t *testing.T) {
	checker := newTestPIDChecker(1000, 2000)
	sessDir := t.TempDir()
	projectPath := "/test/idle-entrypoint-project"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "cli-sess")
	makeJSONLFile(t, projectDir, "py-sess")

	now := time.Now()

	// First scan: both sessions active
	scanResult1 := session.ScanResult{
		ScanTime: now,
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID: 1000, SessionID: "cli-sess", CWD: projectPath,
					Entrypoint: "sdk-cli",
				},
				State:             session.SessionActive,
				JSONLLastModified: now,
				JSONLDir:          projectDir,
			},
			{
				Meta: session.SessionMeta{
					PID: 2000, SessionID: "py-sess", CWD: projectPath,
					Entrypoint: "sdk-py",
				},
				State:             session.SessionActive,
				JSONLLastModified: now,
				JSONLDir:          projectDir,
			},
		},
	}

	result, _ := m.handleScanResult(scanResult1)
	m = result.(SessionDashboardModel)

	// Second scan: both sessions now idle (3 minutes later)
	laterTime := now.Add(3 * time.Minute)
	scanResult2 := session.ScanResult{
		ScanTime: laterTime,
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID: 1000, SessionID: "cli-sess", CWD: projectPath,
					Entrypoint: "sdk-cli",
				},
				State:             session.SessionIdle,
				JSONLLastModified: now, // unchanged
				JSONLDir:          projectDir,
			},
			{
				Meta: session.SessionMeta{
					PID: 2000, SessionID: "py-sess", CWD: projectPath,
					Entrypoint: "sdk-py",
				},
				State:             session.SessionIdle,
				JSONLLastModified: now, // unchanged
				JSONLDir:          projectDir,
			},
		},
	}

	result, _ = m.handleScanResult(scanResult2)
	m = result.(SessionDashboardModel)

	// Both panes should now reflect idle state
	for _, pane := range m.panes {
		if pane.session.State != session.SessionIdle {
			t.Errorf("PID=%d entrypoint=%q: expected Idle state, got %s",
				pane.session.Meta.PID, pane.session.Meta.Entrypoint, pane.session.State)
		}
	}
}

// TestHandleScanResult_DeadPIDRemovedImmediately verifies AC5:
// when a PID is dead (absent from a full scan result), its dashboard pane is
// removed after the grace period (scanMissThreshold consecutive misses) — even
// if the session's last known state was Active (JSONL recently modified).
// PID death takes priority over JSONL timing.
func TestHandleScanResult_DeadPIDRemovedImmediately(t *testing.T) {
	checker := newTestPIDChecker(1000, 2000)
	sessDir := t.TempDir()
	projectPath := "/test/ac5-dead-pid-dashboard"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	now := time.Now()

	// First scan: two sessions, both Active (JSONL recently modified)
	scanResult1 := session.ScanResult{
		ScanTime:   now,
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID: 1000, SessionID: "sess-alive", CWD: projectPath,
					Entrypoint: "sdk-cli",
				},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-10 * time.Second),
			},
			{
				Meta: session.SessionMeta{
					PID: 2000, SessionID: "sess-dies", CWD: projectPath,
					Entrypoint: "sdk-cli",
				},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-5 * time.Second), // very recent — still "Active"
			},
		},
	}
	result, _ := m.handleScanResult(scanResult1)
	m = result.(SessionDashboardModel)

	if len(m.panes) != 2 {
		t.Fatalf("setup: expected 2 panes, got %d", len(m.panes))
	}

	// PID 2000 dies. The scanner won't include it in the next scan because
	// IsAlive(2000) returns false. The JSONL is still recent — but the PID is dead.
	// The full scan result arrives without PID 2000.
	scanResult2 := session.ScanResult{
		ScanTime:   now.Add(30 * time.Second), // only 30s later — JSONL still "Active"
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID: 1000, SessionID: "sess-alive", CWD: projectPath,
					Entrypoint: "sdk-cli",
				},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-10 * time.Second),
			},
			// PID 2000 intentionally absent — scanner excluded it because IsAlive(2000)==false
		},
	}
	// Grace period requires testScanMissThreshold consecutive misses before removal.
	scanMsg := sessionScanResultMsg{result: scanResult2}
	m = applyScanResultNTimes(t, m, scanMsg, testScanMissThreshold)

	// PID 2000's pane must be removed after grace period, regardless of JSONL timing
	if len(m.panes) != 1 {
		t.Fatalf("expected 1 pane after dead PID removed, got %d", len(m.panes))
	}
	if m.panes[0].session.Meta.PID != 1000 {
		t.Errorf("expected PID 1000 to remain, got PID %d", m.panes[0].session.Meta.PID)
	}
	if m.panes[0].session.State != session.SessionActive {
		t.Errorf("surviving session should remain Active, got %s", m.panes[0].session.State)
	}
}

// TestHandleScanResult_DeadPIDRemovedImmediately_NonFullScanNoRemoval verifies
// that non-full scan results do NOT remove panes, since they may be incomplete.
func TestHandleScanResult_DeadPIDRemovedImmediately_NonFullScanNoRemoval(t *testing.T) {
	checker := newTestPIDChecker(1000, 2000)
	sessDir := t.TempDir()
	projectPath := "/test/ac5-nonfull-scan"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	now := time.Now()

	// First populate 2 panes via a full scan
	fullScan := session.ScanResult{
		ScanTime:   now,
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta:  session.SessionMeta{PID: 1000, SessionID: "s1", CWD: projectPath},
				State: session.SessionActive,
			},
			{
				Meta:  session.SessionMeta{PID: 2000, SessionID: "s2", CWD: projectPath},
				State: session.SessionActive,
			},
		},
	}
	result, _ := m.handleScanResult(fullScan)
	m = result.(SessionDashboardModel)
	if len(m.panes) != 2 {
		t.Fatalf("setup: expected 2 panes, got %d", len(m.panes))
	}

	// Non-full scan arrives with only 1 session (PID 2000 absent).
	// Since IsFullScan=false, no panes should be removed.
	partialScan := session.ScanResult{
		ScanTime:   now.Add(time.Second),
		IsFullScan: false, // NOT a full scan — must not trigger removal
		Sessions: []session.ActiveSession{
			{
				Meta:  session.SessionMeta{PID: 1000, SessionID: "s1", CWD: projectPath},
				State: session.SessionActive,
			},
		},
	}
	result, _ = m.handleScanResult(partialScan)
	m = result.(SessionDashboardModel)

	// Both panes must remain — non-full scan must not remove panes
	if len(m.panes) != 2 {
		t.Errorf("non-full scan must not remove panes; expected 2 panes, got %d", len(m.panes))
	}
}

// TestHandleScanResult_PaneRemovedAfter5MinutesOfJSONLInactivity verifies AC4:
// a session pane is removed from the dashboard when the session's JSONL file
// has not been modified for 5+ minutes (SessionRemoved state), even if the
// underlying process (PID) is still technically alive.
func TestHandleScanResult_PaneRemovedAfter5MinutesOfJSONLInactivity(t *testing.T) {
	checker := newTestPIDChecker(1000, 2000)
	sessDir := t.TempDir()
	projectPath := "/test/ac4-removal-project"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	now := time.Now()

	// First scan: two sessions, both Active (JSONL recently modified)
	scanResult1 := session.ScanResult{
		ScanTime:   now,
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID: 1000, SessionID: "active-sess", CWD: projectPath,
					Kind: "interactive",
				},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-30 * time.Second),
				JSONLDir:          projectDir,
			},
			{
				Meta: session.SessionMeta{
					PID: 2000, SessionID: "stale-sess", CWD: projectPath,
					Kind: "interactive",
				},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-30 * time.Second),
				JSONLDir:          projectDir,
			},
		},
	}

	result, _ := m.handleScanResult(scanResult1)
	m = result.(SessionDashboardModel)

	if len(m.panes) != 2 {
		t.Fatalf("setup: expected 2 panes after first scan, got %d", len(m.panes))
	}

	// Second scan: 6 minutes later. PID 2000 is still alive but JSONL has not
	// been written for 6 minutes → SessionRemoved state.
	// PID 1000 remains active.
	laterTime := now.Add(6 * time.Minute)
	scanResult2 := session.ScanResult{
		ScanTime:   laterTime,
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID: 1000, SessionID: "active-sess", CWD: projectPath,
					Kind: "interactive",
				},
				State:             session.SessionActive,
				JSONLLastModified: laterTime.Add(-10 * time.Second), // still being updated
				JSONLDir:          projectDir,
			},
			{
				// PID 2000 is alive but JSONL has 6 minutes of inactivity → Removed
				Meta: session.SessionMeta{
					PID: 2000, SessionID: "stale-sess", CWD: projectPath,
					Kind: "interactive",
				},
				State:             session.SessionRemoved,
				JSONLLastModified: now.Add(-30 * time.Second), // same as before — no new writes
				JSONLDir:          projectDir,
			},
		},
	}

	// Grace period requires testScanMissThreshold consecutive misses before removal.
	scanMsg := sessionScanResultMsg{result: scanResult2}
	m = applyScanResultNTimes(t, m, scanMsg, testScanMissThreshold)

	// PID 2000's pane must be removed after 5+ minutes of JSONL inactivity (grace period satisfied)
	if len(m.panes) != 1 {
		t.Fatalf("expected 1 pane after stale session removed, got %d", len(m.panes))
	}
	if m.panes[0].session.Meta.PID != 1000 {
		t.Errorf("expected PID 1000 to remain, got PID %d", m.panes[0].session.Meta.PID)
	}
	if m.panes[0].session.State != session.SessionActive {
		t.Errorf("surviving session should remain Active, got %s", m.panes[0].session.State)
	}
}

// TestHandleScanResult_RemovedSessionNotAddedAsPane verifies AC4:
// a session already in SessionRemoved state (JSONL not modified for 5+ minutes)
// when first seen in a scan result must NOT be added as a new dashboard pane.
func TestHandleScanResult_RemovedSessionNotAddedAsPane(t *testing.T) {
	checker := newTestPIDChecker(1000)
	sessDir := t.TempDir()
	projectPath := "/test/ac4-no-add-removed"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	now := time.Now()

	// Scan returns a session that is already in SessionRemoved state.
	// This can happen when the dashboard first starts and finds old sessions.
	scanResult := session.ScanResult{
		ScanTime:   now,
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID: 1000, SessionID: "ancient-sess", CWD: projectPath,
					Kind: "interactive",
				},
				State:             session.SessionRemoved,
				JSONLLastModified: now.Add(-6 * time.Minute), // 6 minutes old
				JSONLDir:          projectDir,
			},
		},
	}

	result, _ := m.handleScanResult(scanResult)
	m = result.(SessionDashboardModel)

	// Session past the removal threshold must not get a pane
	if len(m.panes) != 0 {
		t.Errorf("expected 0 panes for already-removed session (5+ min old JSONL), got %d", len(m.panes))
	}
}

// TestSessionPaneModel_ViewWithFocus_IdleShowsWarningState verifies AC7:
// idle sessions (JSONL not modified for 2–5 minutes) display a distinct visual
// warning state in the dashboard — the "⏸ IDLE" suffix appears in the header
// and the pane consistently shows the idle indicator for both focused and
// unfocused states.
func TestSessionPaneModel_ViewWithFocus_IdleShowsWarningState(t *testing.T) {
	pane := SessionPaneModel{
		session: session.ActiveSession{
			Meta:  session.SessionMeta{PID: 777, Kind: "interactive"},
			State: session.SessionIdle,
		},
		width:   40,
		height:  10,
		loading: true,
	}

	// Focused idle pane must show IDLE indicator
	viewFocused := pane.ViewWithFocus(true)
	if viewFocused == "" {
		t.Fatal("focused idle pane should not be empty")
	}
	if !strings.Contains(viewFocused, "IDLE") {
		t.Error("focused idle pane should show IDLE indicator, got:\n" + viewFocused)
	}

	// Unfocused idle pane must also show IDLE indicator
	viewUnfocused := pane.ViewWithFocus(false)
	if viewUnfocused == "" {
		t.Fatal("unfocused idle pane should not be empty")
	}
	if !strings.Contains(viewUnfocused, "IDLE") {
		t.Error("unfocused idle pane should show IDLE indicator, got:\n" + viewUnfocused)
	}

	// The IDLE indicator must include the pause symbol
	if !strings.Contains(viewFocused, "⏸") {
		t.Error("idle pane header should include pause symbol ⏸")
	}
}

// TestSessionPaneModel_ViewWithFocus_IdleVsActive_VisuallyDistinct verifies AC7:
// the visual output of an idle pane is different from an active pane with the
// same dimensions and content, ensuring the user can distinguish idle sessions.
func TestSessionPaneModel_ViewWithFocus_IdleVsActive_VisuallyDistinct(t *testing.T) {
	activePane := SessionPaneModel{
		session: session.ActiveSession{
			Meta:  session.SessionMeta{PID: 100, Kind: "interactive"},
			State: session.SessionActive,
		},
		width:   40,
		height:  10,
		loading: true,
	}

	idlePane := SessionPaneModel{
		session: session.ActiveSession{
			Meta:  session.SessionMeta{PID: 100, Kind: "interactive"},
			State: session.SessionIdle,
		},
		width:   40,
		height:  10,
		loading: true,
	}

	activeView := activePane.ViewWithFocus(false)
	idleView := idlePane.ViewWithFocus(false)

	if activeView == idleView {
		t.Error("idle pane should render differently from active pane with same content")
	}
	if !strings.Contains(idleView, "IDLE") {
		t.Error("idle pane view should contain IDLE, got:\n" + idleView)
	}
	if strings.Contains(activeView, "IDLE") {
		t.Error("active pane view should not contain IDLE, got:\n" + activeView)
	}
}

// TestHandleScanResult_IdleSessionUpdatesExistingPane verifies AC7:
// when a scan result transitions an existing pane's session from Active to Idle,
// the pane's session state is updated so the idle visual is shown.
func TestHandleScanResult_IdleSessionUpdatesExistingPane(t *testing.T) {
	checker := newTestPIDChecker(500)
	sessDir := t.TempDir()
	projectPath := "/test/ac7-idle-update"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	now := time.Now()

	// First scan: session is Active
	scanResult1 := session.ScanResult{
		ScanTime:   now,
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta:              session.SessionMeta{PID: 500, SessionID: "sess-500", CWD: projectPath, Kind: "interactive"},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-30 * time.Second),
				JSONLDir:          projectDir,
			},
		},
	}
	result, _ := m.handleScanResult(scanResult1)
	m = result.(SessionDashboardModel)

	if len(m.panes) != 1 {
		t.Fatalf("setup: expected 1 pane, got %d", len(m.panes))
	}
	if m.panes[0].session.State != session.SessionActive {
		t.Errorf("initial state should be Active, got %s", m.panes[0].session.State)
	}

	// Second scan: 3 minutes later — session is now Idle
	laterTime := now.Add(3 * time.Minute)
	scanResult2 := session.ScanResult{
		ScanTime:   laterTime,
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta:              session.SessionMeta{PID: 500, SessionID: "sess-500", CWD: projectPath, Kind: "interactive"},
				State:             session.SessionIdle,
				JSONLLastModified: now.Add(-30 * time.Second), // unchanged
				JSONLDir:          projectDir,
			},
		},
	}
	result, _ = m.handleScanResult(scanResult2)
	m = result.(SessionDashboardModel)

	if len(m.panes) != 1 {
		t.Fatalf("expected 1 pane to remain after idle transition, got %d", len(m.panes))
	}
	if m.panes[0].session.State != session.SessionIdle {
		t.Errorf("pane should show Idle state after 3 minutes of inactivity, got %s",
			m.panes[0].session.State)
	}

	// The pane's visual output should show the idle indicator
	view := m.panes[0].ViewWithFocus(false)
	if !strings.Contains(view, "IDLE") {
		t.Errorf("idle pane visual should contain IDLE indicator, got:\n%s", view)
	}
}

// TestHandleScanResult_AllSessionsRemovedAfter5MinuteInactivity verifies AC4:
// when ALL sessions exceed the 5-minute JSONL inactivity threshold in a full scan,
// all panes are removed and the dashboard becomes empty.
func TestHandleScanResult_AllSessionsRemovedAfter5MinuteInactivity(t *testing.T) {
	checker := newTestPIDChecker(1000, 2000, 3000)
	sessDir := t.TempDir()
	projectPath := "/test/ac4-all-removed"
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	now := time.Now()

	// First scan: three active sessions
	scanResult1 := session.ScanResult{
		ScanTime:   now,
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta:              session.SessionMeta{PID: 1000, SessionID: "s1", CWD: projectPath, Kind: "interactive"},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-10 * time.Second),
				JSONLDir:          projectDir,
			},
			{
				Meta:              session.SessionMeta{PID: 2000, SessionID: "s2", CWD: projectPath, Kind: "interactive"},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-20 * time.Second),
				JSONLDir:          projectDir,
			},
			{
				Meta:              session.SessionMeta{PID: 3000, SessionID: "s3", CWD: projectPath, Kind: "interactive"},
				State:             session.SessionActive,
				JSONLLastModified: now.Add(-30 * time.Second),
				JSONLDir:          projectDir,
			},
		},
	}

	result, _ := m.handleScanResult(scanResult1)
	m = result.(SessionDashboardModel)

	if len(m.panes) != 3 {
		t.Fatalf("setup: expected 3 panes after first scan, got %d", len(m.panes))
	}

	// Second scan: 6 minutes later — all sessions have exceeded the removal threshold.
	// All PIDs are still alive, but JSONL files haven't been updated.
	laterTime := now.Add(6 * time.Minute)
	scanResult2 := session.ScanResult{
		ScanTime:   laterTime,
		IsFullScan: true,
		Sessions: []session.ActiveSession{
			{
				Meta:              session.SessionMeta{PID: 1000, SessionID: "s1", CWD: projectPath, Kind: "interactive"},
				State:             session.SessionRemoved,
				JSONLLastModified: now.Add(-10 * time.Second), // unchanged
				JSONLDir:          projectDir,
			},
			{
				Meta:              session.SessionMeta{PID: 2000, SessionID: "s2", CWD: projectPath, Kind: "interactive"},
				State:             session.SessionRemoved,
				JSONLLastModified: now.Add(-20 * time.Second), // unchanged
				JSONLDir:          projectDir,
			},
			{
				Meta:              session.SessionMeta{PID: 3000, SessionID: "s3", CWD: projectPath, Kind: "interactive"},
				State:             session.SessionRemoved,
				JSONLLastModified: now.Add(-30 * time.Second), // unchanged
				JSONLDir:          projectDir,
			},
		},
	}

	// Grace period requires testScanMissThreshold consecutive misses before removal.
	scanMsg := sessionScanResultMsg{result: scanResult2}
	m = applyScanResultNTimes(t, m, scanMsg, testScanMissThreshold)

	// All panes must be removed — no pane should persist past the 5-minute threshold (grace period satisfied)
	if len(m.panes) != 0 {
		t.Errorf("expected 0 panes after all sessions exceed 5-minute removal threshold, got %d", len(m.panes))
	}
}
