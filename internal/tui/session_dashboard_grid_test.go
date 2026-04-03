package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
)

// testKeyMsg creates a tea.KeyMsg for a single rune key (e.g., "c", "]", "[").
func testKeyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// TestGridMode_TwoSessions_ShowsGridView verifies AC3: when 2+ active sessions
// are detected, the dashboard displays the 3x3 grid view.
func TestGridMode_TwoSessions_ShowsGridView(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")
	makeJSONLFile(t, projectDir, "sess-2")

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}},
		},
		ScanTime: time.Now(),
	}

	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", updated.PaneCount())
	}

	if updated.ViewMode() != DashboardViewGrid {
		t.Errorf("expected DashboardViewGrid mode, got %v", updated.ViewMode())
	}
}

// TestGridMode_ThreeSessions_ShowsGridView verifies 3 sessions also use grid mode.
func TestGridMode_ThreeSessions_ShowsGridView(t *testing.T) {
	checker := newTestPIDChecker(100, 200, 300)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-test-3"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	for i := 1; i <= 3; i++ {
		makeJSONLFile(t, projectDir, fmt.Sprintf("sess-%d", i))
	}

	sessions := []session.ActiveSession{
		{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
		{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}},
		{Meta: session.SessionMeta{PID: 300, SessionID: "sess-3", CWD: projectPath}},
	}

	scanResult := session.ScanResult{Sessions: sessions, ScanTime: time.Now()}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 3 {
		t.Fatalf("expected 3 panes, got %d", updated.PaneCount())
	}

	if updated.ViewMode() != DashboardViewGrid {
		t.Errorf("expected DashboardViewGrid, got %v", updated.ViewMode())
	}
}

// TestGridMode_NineSessions_FullGrid verifies 9 sessions fill the full 3x3 grid.
func TestGridMode_NineSessions_FullGrid(t *testing.T) {
	pids := make([]int, 9)
	for i := range pids {
		pids[i] = 100 + i
	}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-full-9"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(180, 50)

	var sessions []session.ActiveSession
	for i := 0; i < 9; i++ {
		sid := fmt.Sprintf("sess-%d", i)
		makeJSONLFile(t, projectDir, sid)
		sessions = append(sessions, session.ActiveSession{
			Meta: session.SessionMeta{PID: 100 + i, SessionID: sid, CWD: projectPath},
		})
	}

	scanResult := session.ScanResult{Sessions: sessions, ScanTime: time.Now()}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 9 {
		t.Fatalf("expected 9 panes, got %d", updated.PaneCount())
	}

	if updated.ViewMode() != DashboardViewGrid {
		t.Errorf("expected DashboardViewGrid, got %v", updated.ViewMode())
	}

	// All 9 panes should be on one page
	if updated.TotalPages() != 1 {
		t.Errorf("expected 1 page for 9 panes, got %d", updated.TotalPages())
	}

	visiblePanes := updated.CurrentPagePanes()
	if len(visiblePanes) != 9 {
		t.Errorf("expected 9 visible panes, got %d", len(visiblePanes))
	}
}

// TestGridMode_TenSessions_Pagination verifies pagination with more than 9 sessions.
func TestGridMode_TenSessions_Pagination(t *testing.T) {
	pids := make([]int, 10)
	for i := range pids {
		pids[i] = 100 + i
	}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-10"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(180, 50)

	var sessions []session.ActiveSession
	for i := 0; i < 10; i++ {
		sid := fmt.Sprintf("sess-%d", i)
		makeJSONLFile(t, projectDir, sid)
		sessions = append(sessions, session.ActiveSession{
			Meta: session.SessionMeta{PID: 100 + i, SessionID: sid, CWD: projectPath},
		})
	}

	scanResult := session.ScanResult{Sessions: sessions, ScanTime: time.Now()}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.ViewMode() != DashboardViewGrid {
		t.Errorf("expected DashboardViewGrid, got %v", updated.ViewMode())
	}

	// 10 panes need 2 pages
	if updated.TotalPages() != 2 {
		t.Errorf("expected 2 pages for 10 panes, got %d", updated.TotalPages())
	}

	// First page has 9
	page1 := updated.CurrentPagePanes()
	if len(page1) != 9 {
		t.Errorf("expected 9 panes on page 1, got %d", len(page1))
	}
}

// TestGridMode_ViewRendersGridLayout verifies that the View() output contains
// the grid layout help text when in grid mode.
func TestGridMode_ViewRendersGridLayout(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-view-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")
	makeJSONLFile(t, projectDir, "sess-2")

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}},
		},
		ScanTime: time.Now(),
	}

	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	view := updated.View()

	// Grid mode should display the help text for the dashboard
	if !strings.Contains(view, "nav") {
		t.Errorf("grid view should contain navigation help text, got:\n%s", view)
	}

	// Grid mode should NOT show "Loading latest conversation" (zero-session mode)
	if strings.Contains(view, "Loading latest conversation") {
		t.Error("grid view should not show zero-session loading message")
	}

	// Grid mode should NOT show "Loading session conversation" (single-session mode)
	if strings.Contains(view, "Loading session conversation") {
		t.Error("grid view should not show single-session loading message")
	}
}

// TestGridMode_GridNavigationPreserved verifies arrow key navigation works in grid mode.
func TestGridMode_GridNavigationPreserved(t *testing.T) {
	checker := newTestPIDChecker(100, 200, 300, 400)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-nav-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	for i := 1; i <= 4; i++ {
		makeJSONLFile(t, projectDir, fmt.Sprintf("sess-%d", i))
	}

	sessions := []session.ActiveSession{
		{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
		{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}},
		{Meta: session.SessionMeta{PID: 300, SessionID: "sess-3", CWD: projectPath}},
		{Meta: session.SessionMeta{PID: 400, SessionID: "sess-4", CWD: projectPath}},
	}

	scanResult := session.ScanResult{Sessions: sessions, ScanTime: time.Now()}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.ViewMode() != DashboardViewGrid {
		t.Fatalf("expected grid mode, got %v", updated.ViewMode())
	}

	// Focus should start at 0
	if updated.focusIndex != 0 {
		t.Errorf("initial focus = %d, want 0", updated.focusIndex)
	}

	// Navigate right (should work in grid mode)
	newModel2, _ := updated.Update(testKeyMsg("right"))
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.focusIndex != 1 {
		t.Errorf("focus after right = %d, want 1", updated2.focusIndex)
	}

	// Navigate down
	newModel3, _ := updated2.Update(testKeyMsg("down"))
	updated3 := newModel3.(SessionDashboardModel)

	// With 4 panes in a 2x2 grid, down from index 1 should go to index 3
	if updated3.focusIndex < 2 {
		t.Errorf("focus after down from row 0 = %d, should be >= 2", updated3.focusIndex)
	}
}

// TestGridMode_PageNavigationPreserved verifies page navigation with [/] keys.
func TestGridMode_PageNavigationPreserved(t *testing.T) {
	pids := make([]int, 12)
	for i := range pids {
		pids[i] = 100 + i
	}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-page-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(180, 50)

	var sessions []session.ActiveSession
	for i := 0; i < 12; i++ {
		sid := fmt.Sprintf("sess-%d", i)
		makeJSONLFile(t, projectDir, sid)
		sessions = append(sessions, session.ActiveSession{
			Meta: session.SessionMeta{PID: 100 + i, SessionID: sid, CWD: projectPath},
		})
	}

	scanResult := session.ScanResult{Sessions: sessions, ScanTime: time.Now()}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Should be on page 0
	if updated.CurrentPage() != 0 {
		t.Fatalf("expected page 0, got %d", updated.CurrentPage())
	}

	// Navigate to next page with ]
	newModel2, _ := updated.Update(testKeyMsg("]"))
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.CurrentPage() != 1 {
		t.Errorf("expected page 1 after ], got %d", updated2.CurrentPage())
	}

	// Navigate back with [
	newModel3, _ := updated2.Update(testKeyMsg("["))
	updated3 := newModel3.(SessionDashboardModel)

	if updated3.CurrentPage() != 0 {
		t.Errorf("expected page 0 after [, got %d", updated3.CurrentPage())
	}
}

// TestGridMode_EnterOpensViewer verifies Enter key opens viewer from grid mode.
func TestGridMode_EnterOpensViewer(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-enter-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	jsonlPath := makeJSONLFile(t, projectDir, "sess-1")
	makeJSONLFile(t, projectDir, "sess-2")

	sessions := []session.ActiveSession{
		{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
		{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}},
	}

	scanResult := session.ScanResult{Sessions: sessions, ScanTime: time.Now()}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Set the JSONL path on the first pane (normally done by content loading)
	updated.panes[0].jsonlPath = jsonlPath

	// Press Enter on the focused pane
	newModel2, cmd := updated.Update(testKeyMsg("enter"))
	_ = newModel2.(SessionDashboardModel)

	if cmd == nil {
		t.Fatal("expected non-nil cmd from Enter key")
	}

	// Execute the command and check the message
	msg := cmd()
	openMsg, ok := msg.(OpenViewerFromSessionDashboardMsg)
	if !ok {
		t.Fatalf("expected OpenViewerFromSessionDashboardMsg, got %T", msg)
	}

	if openMsg.FilePath != jsonlPath {
		t.Errorf("expected file path %q, got %q", jsonlPath, openMsg.FilePath)
	}
}

// TestGridMode_TransitionFromSingleToGrid verifies transition from 1 to 2 sessions
// switches to grid mode.
func TestGridMode_TransitionFromSingleToGrid(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-transition-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")

	// First scan: 1 session
	scan1 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
		},
		ScanTime: time.Now(),
	}

	newModel, _ := m.Update(sessionScanResultMsg{result: scan1})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 1 {
		t.Fatalf("expected 1 pane, got %d", updated.PaneCount())
	}

	// Should be in single-session mode
	if updated.ViewMode() != DashboardViewSingleSession {
		t.Errorf("expected single-session mode with 1 pane, got %v", updated.ViewMode())
	}

	// Second scan: add another session → should transition to grid mode
	makeJSONLFile(t, projectDir, "sess-2")
	scan2 := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}},
		},
		ScanTime: time.Now(),
	}

	newModel2, _ := updated.Update(sessionScanResultMsg{result: scan2})
	updated2 := newModel2.(SessionDashboardModel)

	if updated2.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", updated2.PaneCount())
	}

	if updated2.ViewMode() != DashboardViewGrid {
		t.Errorf("expected grid mode with 2 panes, got %v", updated2.ViewMode())
	}

	// Grid dirty should be set for re-render
	if !updated2.IsGridDirty() {
		t.Error("grid should be dirty after transition to grid mode")
	}
}

// TestGridMode_TransitionFromZeroToGrid verifies transition from 0 to 2+ sessions
// switches directly to grid mode.
func TestGridMode_TransitionFromZeroToGrid(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-zero-to-grid"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Initially in zero-session mode (default with 0 panes triggers transition)
	// Send scan with 2 sessions directly
	makeJSONLFile(t, projectDir, "sess-1")
	makeJSONLFile(t, projectDir, "sess-2")

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}},
		},
		ScanTime: time.Now(),
	}

	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	if updated.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", updated.PaneCount())
	}

	if updated.ViewMode() != DashboardViewGrid {
		t.Errorf("expected grid mode, got %v", updated.ViewMode())
	}

	// latestViewer should be cleared on transition to grid
	if updated.LatestViewer() != nil {
		t.Error("latest viewer should be nil in grid mode")
	}
}

// TestGridMode_GridDirtyOnTransition verifies that transitioning to grid mode
// marks the grid as dirty for full re-render.
func TestGridMode_GridDirtyOnTransition(t *testing.T) {
	m := SessionDashboardModel{
		viewMode: DashboardViewSingleSession,
	}
	m.transitionToGridMode()

	if m.viewMode != DashboardViewGrid {
		t.Errorf("expected DashboardViewGrid after transition, got %v", m.viewMode)
	}

	if !m.gridDirty {
		t.Error("grid should be dirty after transition to grid mode")
	}

	if m.singleSessionViewer != nil {
		t.Error("singleSessionViewer should be nil after transition to grid mode")
	}

	if m.latestViewer != nil {
		t.Error("latestViewer should be nil after transition to grid mode")
	}
}

// TestDetectViewMode_GridFor2Plus verifies detectViewMode returns grid for 2+ sessions.
func TestDetectViewMode_GridFor2Plus(t *testing.T) {
	tests := []struct {
		count int
		want  DashboardViewMode
	}{
		{0, DashboardViewZeroSessions},
		{1, DashboardViewSingleSession},
		{2, DashboardViewGrid},
		{3, DashboardViewGrid},
		{9, DashboardViewGrid},
		{12, DashboardViewGrid},
		{100, DashboardViewGrid},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("count=%d", tt.count), func(t *testing.T) {
			got := detectViewMode(tt.count)
			if got != tt.want {
				t.Errorf("detectViewMode(%d) = %v, want %v", tt.count, got, tt.want)
			}
		})
	}
}

// TestGridMode_CKeyOpensConversations verifies 'c' key works in grid mode.
func TestGridMode_CKeyOpensConversations(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/grid-c-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	makeJSONLFile(t, projectDir, "sess-1")
	makeJSONLFile(t, projectDir, "sess-2")

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: "sess-1", CWD: projectPath}},
			{Meta: session.SessionMeta{PID: 200, SessionID: "sess-2", CWD: projectPath}},
		},
		ScanTime: time.Now(),
	}

	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	updated := newModel.(SessionDashboardModel)

	// Press 'c' to open conversations
	_, cmd := updated.Update(testKeyMsg("c"))
	if cmd == nil {
		t.Fatal("expected non-nil cmd from 'c' key")
	}

	msg := cmd()
	_, ok := msg.(OpenConversationsFromSessionDashboardMsg)
	if !ok {
		t.Fatalf("expected OpenConversationsFromSessionDashboardMsg, got %T", msg)
	}
}
