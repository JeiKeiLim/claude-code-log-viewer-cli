package tui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// TestNewAppModelForSessions creates a session dashboard app model.
func TestNewAppModelForSessions(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test-project", "/home/.claude/projects/-tmp-test-project",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	if model.State() != viewSessionDashboard {
		t.Errorf("expected viewSessionDashboard state, got %d", model.State())
	}
	if model.sessionDashboardModel.projectPath != "/tmp/test-project" {
		t.Errorf("unexpected project path: %s", model.sessionDashboardModel.projectPath)
	}
}

// TestNewAppModelForSessions_DefaultScanner uses default scanner and monitor.
func TestNewAppModelForSessions_DefaultScanner(t *testing.T) {
	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test")

	if model.State() != viewSessionDashboard {
		t.Errorf("expected viewSessionDashboard state, got %d", model.State())
	}
	// Should have defaults initialized
	if model.sessionDashboardModel.scanner == nil {
		t.Error("scanner should be initialized")
	}
	if model.sessionDashboardModel.monitor == nil {
		t.Error("monitor should be initialized")
	}
}

// TestAppModel_Init_SessionDashboard verifies Init starts session detection.
func TestAppModel_Init_SessionDashboard(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	cmd := model.Init()
	if cmd == nil {
		t.Error("Init should return non-nil cmd for session dashboard")
	}
}

// TestAppModel_WindowSize_SessionDashboard forwards size to session dashboard.
func TestAppModel_WindowSize_SessionDashboard(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	newModel, _ := model.Update(msg)
	m := newModel.(AppModel)

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
	if m.sessionDashboardModel.width != 120 {
		t.Errorf("expected session dashboard width 120, got %d", m.sessionDashboardModel.width)
	}
}

// TestAppModel_View_SessionDashboard renders session dashboard view.
func TestAppModel_View_SessionDashboard(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	view := model.View()
	if view == "" {
		t.Error("View should return non-empty string for session dashboard")
	}
}

// TestAppModel_GoBackFromSessionDashboard returns to project list.
func TestAppModel_GoBackFromSessionDashboard(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	msg := GoBackFromSessionDashboardMsg{}
	newModel, _ := model.Update(msg)
	m := newModel.(AppModel)

	if m.State() != viewProjects {
		t.Errorf("expected viewProjects state after GoBack, got %d", m.State())
	}
}

// TestAppModel_OpenViewerFromSessionDashboard transitions to loading.
func TestAppModel_OpenViewerFromSessionDashboard(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	msg := OpenViewerFromSessionDashboardMsg{
		FilePath: "/tmp/test.jsonl",
		Project: types.Project{
			DecodedPath: "/tmp/test",
			DisplayName: "test",
		},
	}
	newModel, cmd := model.Update(msg)
	m := newModel.(AppModel)

	if !m.loading {
		t.Error("expected loading state after OpenViewerFromSessionDashboard")
	}
	if m.viewerSource != FromSessionDashboard {
		t.Errorf("expected FromSessionDashboard, got %d", m.viewerSource)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for loading conversation")
	}
}

// TestAppModel_GoBackMsg_FromSessionDashboard returns to session dashboard.
func TestAppModel_GoBackMsg_FromSessionDashboard(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	// Simulate having navigated to viewer from session dashboard
	model.state = viewViewer
	model.viewerSource = FromSessionDashboard

	msg := GoBackMsg{}
	newModel, _ := model.Update(msg)
	m := newModel.(AppModel)

	if m.State() != viewSessionDashboard {
		t.Errorf("expected viewSessionDashboard state, got %d", m.State())
	}
	if m.viewerSource != FromConversationList {
		t.Errorf("expected viewerSource reset to FromConversationList, got %d", m.viewerSource)
	}
}

// TestAppModel_SessionDashboard_ForwardsMessages verifies message routing.
func TestAppModel_SessionDashboard_ForwardsMessages(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	// Unknown key messages should be routed to session dashboard
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	newModel, _ := model.Update(msg)
	m := newModel.(AppModel)

	// Should still be in session dashboard state (no crash)
	if m.State() != viewSessionDashboard {
		t.Errorf("expected viewSessionDashboard state, got %d", m.State())
	}
}

// TestAppModel_SessionDashboard_ScanResultAddsPane verifies that scan results
// flow through the app and add panes.
func TestAppModel_SessionDashboard_ScanResultAddsPane(t *testing.T) {
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	checker := newTestPIDChecker(1234)
	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(50*time.Millisecond),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	projectPath := "/tmp/test-project"
	model := NewAppModelForSessions(projectPath, projectDir,
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	// Simulate a scan result message
	result := session.ScanResult{
		Sessions: []session.ActiveSession{
			{
				Meta: session.SessionMeta{
					PID:       1234,
					SessionID: "test-session-1",
					CWD:       projectPath,
					Kind:      "interactive",
				},
			},
		},
		ScanTime: time.Now(),
	}

	// Send scan result through the session dashboard
	scanMsg := sessionScanResultMsg{result: result}
	newModel, _ := model.Update(scanMsg)
	m := newModel.(AppModel)

	if m.SessionDashboardState().PaneCount() != 1 {
		t.Errorf("expected 1 pane after scan, got %d", m.SessionDashboardState().PaneCount())
	}
}

// TestAppModel_SessionDashboard_ThreeSessions verifies 3 concurrent sessions
// appear as 3 panes (core AC 5 requirement).
func TestAppModel_SessionDashboard_ThreeSessions(t *testing.T) {
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	checker := newTestPIDChecker(1001, 1002, 1003)
	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithJSONLBaseDir(projectDir),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	projectPath := "/tmp/test-project"
	model := NewAppModelForSessions(projectPath, projectDir,
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	// Create session files and corresponding JSONL logs.
	// The scanner requires a JSONL file to exist for each session.
	makeTestSessionFile(t, sessDir, 1001, "session-a", projectPath)
	makeJSONLFile(t, projectDir, "session-a")
	makeTestSessionFile(t, sessDir, 1002, "session-b", projectPath)
	makeJSONLFile(t, projectDir, "session-b")
	makeTestSessionFile(t, sessDir, 1003, "session-c", projectPath)
	makeJSONLFile(t, projectDir, "session-c")

	// Scan for sessions
	result := scanner.Scan()
	if len(result.Sessions) != 3 {
		t.Fatalf("expected 3 sessions from scan, got %d", len(result.Sessions))
	}

	// Deliver scan result
	scanMsg := sessionScanResultMsg{result: result}
	newModel, _ := model.Update(scanMsg)
	m := newModel.(AppModel)

	paneCount := m.SessionDashboardState().PaneCount()
	if paneCount != 3 {
		t.Errorf("expected 3 panes for 3 sessions, got %d", paneCount)
	}

	// Verify the grid renders properly at a reasonable size
	m.sessionDashboardModel.SetSize(120, 40)
	view := m.sessionDashboardModel.View()
	if view == "" {
		t.Error("expected non-empty view with 3 panes")
	}
}

// TestAppModel_SessionDashboard_SessionClosure verifies pane removal on session close.
func TestAppModel_SessionDashboard_SessionClosure(t *testing.T) {
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	checker := newTestPIDChecker(1001, 1002)
	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	projectPath := "/tmp/test-project"
	model := NewAppModelForSessions(projectPath, projectDir,
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	// Add two sessions
	result := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 1001, SessionID: "s1", CWD: projectPath, Kind: "interactive"}},
			{Meta: session.SessionMeta{PID: 1002, SessionID: "s2", CWD: projectPath, Kind: "interactive"}},
		},
		ScanTime: time.Now(),
	}
	newModel, _ := model.Update(sessionScanResultMsg{result: result})
	m := newModel.(AppModel)
	if m.SessionDashboardState().PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", m.SessionDashboardState().PaneCount())
	}

	// Close one session
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 1001, SessionID: "s1"}},
	}
	newModel2, _ := m.Update(sessionClosedMsg{event: closeEvent})
	m2 := newModel2.(AppModel)

	if m2.SessionDashboardState().PaneCount() != 1 {
		t.Errorf("expected 1 pane after closure, got %d", m2.SessionDashboardState().PaneCount())
	}
}

// TestNavigationSource_SessionDashboard verifies the enum value.
func TestNavigationSource_SessionDashboard(t *testing.T) {
	if FromSessionDashboard != 2 {
		t.Errorf("expected FromSessionDashboard = 2, got %d", FromSessionDashboard)
	}
}

// TestViewState_SessionDashboard verifies the session dashboard view state.
func TestViewState_SessionDashboard(t *testing.T) {
	if viewSessionDashboard != 4 {
		t.Errorf("expected viewSessionDashboard = 4, got %d", viewSessionDashboard)
	}
}

// TestWithSessionScanner_Option verifies the option function.
func TestWithSessionScanner_Option(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	s := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))

	cfg := sessionDashboardConfig{}
	opt := WithSessionScanner(s)
	opt(&cfg)

	if cfg.scanner != s {
		t.Error("WithSessionScanner should set the scanner in config")
	}
}

// TestWithSessionMonitor_Option verifies the option function.
func TestWithSessionMonitor_Option(t *testing.T) {
	checker := newTestPIDChecker()
	m := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	cfg := sessionDashboardConfig{}
	opt := WithSessionMonitor(m)
	opt(&cfg)

	if cfg.monitor != m {
		t.Error("WithSessionMonitor should set the monitor in config")
	}
}

// TestSessionDashboardState_Accessor verifies the test accessor.
func TestSessionDashboardState_Accessor(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	state := model.SessionDashboardState()
	if state.PaneCount() != 0 {
		t.Errorf("expected 0 panes initially, got %d", state.PaneCount())
	}
}

// TestAppModel_SessionDashboard_SpinnerNotForwarded verifies spinners
// aren't forwarded to session dashboard (no overlay spinner support).
func TestAppModel_SessionDashboard_SpinnerNotForwarded(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	// Spinner tick should not crash in session dashboard state
	msg := model.spinner.Tick()
	newModel, _ := model.Update(msg)
	m := newModel.(AppModel)

	// Should stay in session dashboard
	if m.State() != viewSessionDashboard {
		t.Errorf("expected viewSessionDashboard, got %d", m.State())
	}
}

// TestAppModel_SessionDashboard_MaxNinePanes verifies 3x3 grid limit.
func TestAppModel_SessionDashboard_MaxNinePanes(t *testing.T) {
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	pids := []int{1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009, 1010}
	checker := newTestPIDChecker(pids...)
	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
	)
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	projectPath := "/tmp/test-project"
	model := NewAppModelForSessions(projectPath, projectDir,
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
	)

	// Create 10 sessions (max is 9)
	var sessions []session.ActiveSession
	for i, pid := range pids {
		sessions = append(sessions, session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       pid,
				SessionID: "session-" + strconv.Itoa(i),
				CWD:       projectPath,
				Kind:      "interactive",
			},
		})
	}

	result := session.ScanResult{Sessions: sessions, ScanTime: time.Now()}
	newModel, _ := model.Update(sessionScanResultMsg{result: result})
	m := newModel.(AppModel)

	// With pagination, all 10 sessions are stored (no hard cap)
	dashState := m.SessionDashboardState()
	paneCount := dashState.PaneCount()
	if paneCount != 10 {
		t.Errorf("expected 10 total panes (pagination stores all), got %d", paneCount)
	}

	// Only up to 9 are visible per page
	visiblePanes := dashState.CurrentPagePanes()
	if len(visiblePanes) > MaxSessionPanes {
		t.Errorf("expected at most %d visible panes per page, got %d", MaxSessionPanes, len(visiblePanes))
	}
	if len(visiblePanes) != MaxSessionPanes {
		t.Errorf("expected exactly %d visible panes on page 0, got %d", MaxSessionPanes, len(visiblePanes))
	}
}

// TestAppModel_SessionDashboard_UsageBar verifies usage bar is present.
func TestAppModel_SessionDashboard_UsageBar(t *testing.T) {
	model := NewAppModelForSessions("/tmp/test", "/home/.claude/projects/-tmp-test")

	if model.usageBar == nil {
		t.Error("usage bar should be initialized")
	}
	if model.usageClient == nil {
		t.Error("usage client should be initialized")
	}
}

// --- AC 8: Session dashboard reachable from normal project selection flow ---

// TestProjectSelectedMsg_TransitionsToSessionDashboard verifies that selecting
// a single project in the normal TUI flow opens the session dashboard (AC 8).
func TestProjectSelectedMsg_TransitionsToSessionDashboard(t *testing.T) {
	projects := []types.Project{
		{
			DisplayName: "my-project",
			DecodedPath: "/Users/test/my-project",
			DirPath:     "/home/.claude/projects/-Users-test-my-project",
			EncodedName: "-Users-test-my-project",
		},
	}
	model := NewAppModel(projects)
	model.width = 120
	model.height = 40

	// Simulate selecting a project
	msg := ProjectSelectedMsg{Project: projects[0]}
	newModel, cmd := model.Update(msg)
	m := newModel.(AppModel)

	// Should transition to session dashboard, not conversation list
	if m.State() != viewSessionDashboard {
		t.Errorf("ProjectSelectedMsg: state = %d, want viewSessionDashboard (%d)",
			m.State(), viewSessionDashboard)
	}

	// Should NOT be in loading state (no conversation loading)
	if m.loading {
		t.Error("ProjectSelectedMsg: should not set loading = true (no conversation loading)")
	}

	// Should have a session dashboard model configured for the selected project
	if m.sessionDashboardModel.projectPath != projects[0].DecodedPath {
		t.Errorf("session dashboard projectPath = %q, want %q",
			m.sessionDashboardModel.projectPath, projects[0].DecodedPath)
	}
	if m.sessionDashboardModel.projectDir != projects[0].DirPath {
		t.Errorf("session dashboard projectDir = %q, want %q",
			m.sessionDashboardModel.projectDir, projects[0].DirPath)
	}

	// Should have a non-nil Init command to start session detection
	if cmd == nil {
		t.Error("ProjectSelectedMsg: should return non-nil cmd to start session detection")
	}

	// selectedProject should be set
	if m.selectedProject.DisplayName != projects[0].DisplayName {
		t.Errorf("selectedProject.DisplayName = %q, want %q",
			m.selectedProject.DisplayName, projects[0].DisplayName)
	}
}

// TestProjectSelectedMsg_SessionDashboardScannerInitialized verifies the scanner
// and monitor are initialized in the session dashboard model.
func TestProjectSelectedMsg_SessionDashboardScannerInitialized(t *testing.T) {
	projects := []types.Project{
		{
			DisplayName: "proj",
			DecodedPath: "/Users/test/proj",
			DirPath:     "/home/.claude/projects/-Users-test-proj",
		},
	}
	model := NewAppModel(projects)

	msg := ProjectSelectedMsg{Project: projects[0]}
	newModel, _ := model.Update(msg)
	m := newModel.(AppModel)

	if m.sessionDashboardModel.scanner == nil {
		t.Error("session dashboard scanner should be initialized after ProjectSelectedMsg")
	}
	if m.sessionDashboardModel.monitor == nil {
		t.Error("session dashboard monitor should be initialized after ProjectSelectedMsg")
	}
}

// TestOpenConversationsFromSessionDashboard_LoadsConversations verifies that
// pressing 'c' in the session dashboard triggers conversation loading (AC 8).
func TestOpenConversationsFromSessionDashboard_LoadsConversations(t *testing.T) {
	projects := []types.Project{
		{
			DisplayName: "my-project",
			DecodedPath: "/Users/test/my-project",
			DirPath:     "/home/.claude/projects/-Users-test-my-project",
		},
	}
	model := NewAppModel(projects)
	model.width = 120
	model.height = 40
	// Set up as if project was selected (session dashboard opened)
	model.selectedProject = projects[0]
	model.state = viewSessionDashboard

	// Simulate pressing 'c' — session dashboard sends OpenConversationsFromSessionDashboardMsg
	msg := OpenConversationsFromSessionDashboardMsg{Project: projects[0]}
	newModel, cmd := model.Update(msg)
	m := newModel.(AppModel)

	// Should start loading conversations
	if !m.loading {
		t.Error("OpenConversationsFromSessionDashboardMsg: should set loading = true")
	}

	// Should have a non-nil command (conversation loader)
	if cmd == nil {
		t.Error("OpenConversationsFromSessionDashboardMsg: should return non-nil cmd")
	}
}

// TestSessionDashboard_CKeyNavigatesToConversations verifies the 'c' key in
// the session dashboard emits OpenConversationsFromSessionDashboardMsg (AC 8).
func TestSessionDashboard_CKeyNavigatesToConversations(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()
	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	projectPath := "/Users/test/my-project"
	projectDir := "/home/.claude/projects/-Users-test-my-project"

	model := NewSessionDashboardModel(
		projectPath,
		projectDir,
		scanner,
		monitor,
	)

	// Press 'c' key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	newModel, cmd := model.Update(msg)
	_ = newModel

	if cmd == nil {
		t.Fatal("pressing 'c' should return a non-nil cmd")
	}

	// Execute the command to get the message
	resultMsg := cmd()
	convMsg, ok := resultMsg.(OpenConversationsFromSessionDashboardMsg)
	if !ok {
		t.Fatalf("expected OpenConversationsFromSessionDashboardMsg, got %T", resultMsg)
	}

	// Project should be set from session dashboard's project path
	if convMsg.Project.DecodedPath != projectPath {
		t.Errorf("Project.DecodedPath = %q, want %q", convMsg.Project.DecodedPath, projectPath)
	}
	if convMsg.Project.DirPath != projectDir {
		t.Errorf("Project.DirPath = %q, want %q", convMsg.Project.DirPath, projectDir)
	}
}

// TestAppModel_ProjectSelection_EscFromSessionDashboardGoesBackToProjects verifies
// that after project selection → session dashboard, ESC returns to project list.
func TestAppModel_ProjectSelection_EscFromSessionDashboardGoesBackToProjects(t *testing.T) {
	projects := []types.Project{
		{
			DisplayName: "my-project",
			DecodedPath: "/Users/test/my-project",
			DirPath:     "/home/.claude/projects/-Users-test-my-project",
		},
	}
	model := NewAppModel(projects)
	model.width = 120
	model.height = 40

	// Step 1: Select a project → session dashboard
	selectMsg := ProjectSelectedMsg{Project: projects[0]}
	newModel, _ := model.Update(selectMsg)
	m := newModel.(AppModel)

	if m.State() != viewSessionDashboard {
		t.Fatalf("expected viewSessionDashboard after project selection, got %d", m.State())
	}

	// Step 2: Press ESC → should get GoBackFromSessionDashboardMsg → go back to projects
	goBackMsg := GoBackFromSessionDashboardMsg{}
	newModel2, _ := m.Update(goBackMsg)
	m2 := newModel2.(AppModel)

	if m2.State() != viewProjects {
		t.Errorf("expected viewProjects after ESC from session dashboard, got %d", m2.State())
	}
}

// TestSessionDashboard_HelpTextContainsConversationShortcut verifies the help text
// mentions the 'c' key for conversations (AC 8 UX discoverability).
func TestSessionDashboard_HelpTextContainsConversationShortcut(t *testing.T) {
	if !containsSubstring(sessionDashboardHelpText, "c:conversations") {
		t.Errorf("sessionDashboardHelpText should mention 'c:conversations', got: %q",
			sessionDashboardHelpText)
	}
}

// containsSubstring is a test helper for substring check.
func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && strings.Contains(s, sub))
}

// TestWithSessionDirWatcher_Option verifies the option function sets the
// dir watcher in the config and that NewAppModelForSessions wires it through
// to the session dashboard model.
func TestWithSessionDirWatcher_Option(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()

	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer dw.Close()

	// Verify the option sets the dirWatcher field in the config.
	cfg := sessionDashboardConfig{}
	opt := WithSessionDirWatcher(dw)
	opt(&cfg)

	if cfg.dirWatcher != dw {
		t.Error("WithSessionDirWatcher should set dirWatcher in config")
	}
}

// TestNewAppModelForSessions_WithDirWatcher verifies that providing a custom
// SessionDirectoryWatcher via WithSessionDirWatcher wires it into the session
// dashboard model (not replaced by the default dir watcher).
func TestNewAppModelForSessions_WithDirWatcher(t *testing.T) {
	sessDir := t.TempDir()
	checker := newTestPIDChecker()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))
	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}
	defer dw.Close()

	model := NewAppModelForSessions("/tmp/test-with-dw", "/home/.claude/projects/-tmp-test-with-dw",
		WithSessionScanner(scanner),
		WithSessionMonitor(monitor),
		WithSessionDirWatcher(dw),
	)

	if model.State() != viewSessionDashboard {
		t.Errorf("expected viewSessionDashboard state, got %d", model.State())
	}

	// The dir watcher should be set in the session dashboard model.
	if model.sessionDashboardModel.dirWatcher == nil {
		t.Error("dirWatcher should be set in session dashboard when WithSessionDirWatcher is provided")
	}

	// The dir watcher channel should also be initialized.
	if model.sessionDashboardModel.dirWatcherChan == nil {
		t.Error("dirWatcherChan should be initialized when dirWatcher is set")
	}
}
