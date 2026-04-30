package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestFindLatestJSONLByMtime(t *testing.T) {
	dir := t.TempDir()

	// Empty directory
	result := findLatestJSONLByMtime(dir)
	if result != "" {
		t.Errorf("expected empty string for empty dir, got %q", result)
	}

	// Create two JSONL files with different modification times
	older := filepath.Join(dir, "older.jsonl")
	newer := filepath.Join(dir, "newer.jsonl")

	if err := os.WriteFile(older, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Ensure different mtime
	past := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(newer, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result = findLatestJSONLByMtime(dir)
	if result != newer {
		t.Errorf("expected %q, got %q", newer, result)
	}

	// Non-JSONL files should be ignored
	txtFile := filepath.Join(dir, "notjsonl.txt")
	if err := os.WriteFile(txtFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	result = findLatestJSONLByMtime(dir)
	if result != newer {
		t.Errorf("expected %q after adding txt file, got %q", newer, result)
	}
}

func TestFindLatestJSONLByMtime_NonexistentDir(t *testing.T) {
	result := findLatestJSONLByMtime("/nonexistent/directory/path")
	if result != "" {
		t.Errorf("expected empty string for nonexistent dir, got %q", result)
	}
}

func TestDetectViewMode(t *testing.T) {
	tests := []struct {
		count    int
		expected DashboardViewMode
	}{
		{0, DashboardViewZeroSessions},
		{1, DashboardViewSingleSession},
		{2, DashboardViewGrid},
		{3, DashboardViewGrid},
		{9, DashboardViewGrid},
	}
	for _, tt := range tests {
		got := detectViewMode(tt.count)
		if got != tt.expected {
			t.Errorf("detectViewMode(%d) = %d, want %d", tt.count, got, tt.expected)
		}
	}
}

func TestLoadLatestConversationCmd_NoFiles(t *testing.T) {
	dir := t.TempDir()
	cmd := loadLatestConversationCmd(dir)
	msg := cmd()

	loaded, ok := msg.(latestConversationLoadedMsg)
	if !ok {
		t.Fatalf("expected latestConversationLoadedMsg, got %T", msg)
	}
	if loaded.err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestLoadLatestConversationCmd_WithJSONL(t *testing.T) {
	dir := t.TempDir()

	// Create a valid JSONL file
	content := `{"type":"user","message":{"role":"user","content":"hello"},"timestamp":"2026-03-31T00:00:00Z"}` + "\n"
	filePath := filepath.Join(dir, "test-session.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := loadLatestConversationCmd(dir)
	msg := cmd()

	loaded, ok := msg.(latestConversationLoadedMsg)
	if !ok {
		t.Fatalf("expected latestConversationLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Errorf("unexpected error: %v", loaded.err)
	}
	if loaded.filePath != filePath {
		t.Errorf("filePath = %q, want %q", loaded.filePath, filePath)
	}
	if len(loaded.entries) != 1 {
		t.Errorf("entries count = %d, want 1", len(loaded.entries))
	}
}

func TestCreateLatestViewer(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}
	viewer := createLatestViewer(entries, 0, "/tmp/test-session.jsonl", 120, 40)

	if viewer.canGoBack {
		t.Error("expected canGoBack=false for zero-session viewer")
	}
	if len(viewer.entries) != 1 {
		t.Errorf("entries count = %d, want 1", len(viewer.entries))
	}
}

func TestHandleLatestConversationLoaded_Success(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.latestLoading = true

	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}

	msg := latestConversationLoadedMsg{
		entries:     entries,
		parseErrors: 0,
		filePath:    filepath.Join(projectDir, "test.jsonl"),
	}

	result, _ := m.handleLatestConversationLoaded(msg)
	updated := result.(SessionDashboardModel)

	if updated.latestLoading {
		t.Error("expected latestLoading=false after load")
	}
	if updated.latestViewer == nil {
		t.Error("expected latestViewer to be set")
	}
}

func TestHandleLatestConversationLoaded_Error(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.latestLoading = true

	msg := latestConversationLoadedMsg{
		err: os.ErrNotExist,
	}

	result, _ := m.handleLatestConversationLoaded(msg)
	updated := result.(SessionDashboardModel)

	if updated.latestLoading {
		t.Error("expected latestLoading=false after error")
	}
	if updated.latestViewer != nil {
		t.Error("expected latestViewer to be nil on error")
	}
}

func TestZeroSessionViewMode_View(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewZeroSessions

	// While loading, show loading message
	m.latestLoading = true
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view while loading")
	}

	// After loading with no viewer (error case), show fallback
	m.latestLoading = false
	m.latestViewer = nil
	view = m.View()
	if view == "" {
		t.Error("expected non-empty fallback view")
	}

	// After loading with viewer, show viewer's View()
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}
	viewer := createLatestViewer(entries, 0, filepath.Join(projectDir, "test.jsonl"), 120, 40)
	m.latestViewer = &viewer
	view = m.View()
	if view == "" {
		t.Error("expected non-empty viewer view")
	}
}

func TestTransitionToZeroSessionMode(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.viewMode = DashboardViewGrid

	cmd := m.transitionToZeroSessionMode()

	if m.viewMode != DashboardViewZeroSessions {
		t.Errorf("viewMode = %d, want DashboardViewZeroSessions", m.viewMode)
	}
	if !m.latestLoading {
		t.Error("expected latestLoading=true during transition")
	}
	if m.latestViewer != nil {
		t.Error("expected latestViewer=nil during transition (reset)")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to load latest conversation")
	}
}

func TestUpdateViewMode_ZeroToGrid(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewZeroSessions

	// Set up a fake latest viewer to verify it gets cleaned up
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}
	viewer := createLatestViewer(entries, 0, filepath.Join(projectDir, "test.jsonl"), 120, 40)
	m.latestViewer = &viewer

	// Add 2 panes to trigger grid mode
	m.panes = []SessionPaneModel{
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 100}}},
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 101}}},
	}

	m.updateViewMode()

	if m.viewMode != DashboardViewGrid {
		t.Errorf("viewMode = %d, want DashboardViewGrid", m.viewMode)
	}
	if m.latestViewer != nil {
		t.Error("expected latestViewer=nil after transition to grid mode")
	}
}

// --- AC5: View state resets to fresh defaults on each auto-transition ---
// These tests verify that every transition path creates fresh view state
// with no scroll position, search state, or stale viewer references preserved.

func TestTransitionToSingleSessionMode_ResetsState(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewGrid

	// Simulate a pre-existing singleSessionViewer from a prior transition
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "old viewer"}},
	}
	oldViewer := createSingleSessionViewer(entries, 0, "/old/path.jsonl", "old-session", 120, 40)
	m.singleSessionViewer = &oldViewer
	m.singleSessionPaneIdx = 5 // Stale index

	// Set stale grid navigation state
	m.focusIndex = 3
	m.currentPage = 1

	// Set up 1 pane with content to trigger single-session mode
	m.panes = []SessionPaneModel{
		{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "new-session"}},
			entries: []types.LogEntry{
				{Type: "user", Message: types.Message{Role: "user", TextContent: "new content"}},
			},
			jsonlPath: "/new/path.jsonl",
		},
	}

	cmd := m.transitionToSingleSessionMode()

	if m.viewMode != DashboardViewSingleSession {
		t.Errorf("viewMode = %d, want DashboardViewSingleSession", m.viewMode)
	}
	if m.singleSessionPaneIdx != 0 {
		t.Errorf("singleSessionPaneIdx = %d, want 0 (fresh)", m.singleSessionPaneIdx)
	}
	if m.singleSessionViewer == nil {
		t.Fatal("expected singleSessionViewer to be created")
	}
	// The viewer should be a NEW instance, not the old one
	if m.singleSessionViewer == &oldViewer {
		t.Error("expected a fresh ViewerModel, got the old one")
	}
	// AC5: grid navigation state must reset to defaults
	if m.focusIndex != 0 {
		t.Errorf("focusIndex = %d, want 0 (fresh state)", m.focusIndex)
	}
	if m.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0 (fresh state)", m.currentPage)
	}
	// cmd may be nil if viewer has no file watcher (Init returns nil without watchers)
	_ = cmd
}

func TestTransitionToGridMode_ResetsAllViewerState(t *testing.T) {
	checker := newTestPIDChecker(100, 101)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewSingleSession

	// Set up stale single-session viewer state
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}
	singleViewer := createSingleSessionViewer(entries, 0, "/tmp/sess.jsonl", "sess-1", 120, 40)
	m.singleSessionViewer = &singleViewer
	m.singleSessionPaneIdx = 0

	// Also set stale zero-session viewer state (shouldn't exist in single mode, but be defensive)
	latestViewer := createLatestViewer(entries, 0, "/tmp/latest.jsonl", 120, 40)
	m.latestViewer = &latestViewer
	m.latestLoading = true

	m.panes = []SessionPaneModel{
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 100}}},
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 101}}},
	}

	// Set stale grid navigation state
	m.focusIndex = 5
	m.currentPage = 2

	m.transitionToGridMode()

	if m.viewMode != DashboardViewGrid {
		t.Errorf("viewMode = %d, want DashboardViewGrid", m.viewMode)
	}
	if m.singleSessionViewer != nil {
		t.Error("expected singleSessionViewer=nil after grid transition")
	}
	if m.singleSessionPaneIdx != -1 {
		t.Errorf("singleSessionPaneIdx = %d, want -1", m.singleSessionPaneIdx)
	}
	if m.latestViewer != nil {
		t.Error("expected latestViewer=nil after grid transition")
	}
	if m.latestLoading {
		t.Error("expected latestLoading=false after grid transition")
	}
	// AC5: grid navigation state must reset to defaults
	if m.focusIndex != 0 {
		t.Errorf("focusIndex = %d, want 0 (fresh state)", m.focusIndex)
	}
	if m.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0 (fresh state)", m.currentPage)
	}
}

func TestUpdateViewMode_SingleToZero_FreshState(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewSingleSession

	// Set up stale single-session state
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "active session"}},
	}
	singleViewer := createSingleSessionViewer(entries, 0, "/tmp/sess.jsonl", "sess-1", 120, 40)
	m.singleSessionViewer = &singleViewer
	m.singleSessionPaneIdx = 0

	// Set stale grid navigation state
	m.focusIndex = 4
	m.currentPage = 2

	// Remove all panes → should transition to zero-session mode
	m.panes = nil

	cmd := m.updateViewMode()

	if m.viewMode != DashboardViewZeroSessions {
		t.Errorf("viewMode = %d, want DashboardViewZeroSessions", m.viewMode)
	}
	// Single session viewer must be cleaned up by the zero transition
	if m.latestViewer != nil {
		t.Error("expected latestViewer=nil (not yet loaded)")
	}
	if !m.latestLoading {
		t.Error("expected latestLoading=true during zero-session transition")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to load latest conversation")
	}
	// AC5: grid navigation state must reset to defaults
	if m.focusIndex != 0 {
		t.Errorf("focusIndex = %d, want 0 (fresh state)", m.focusIndex)
	}
	if m.currentPage != 0 {
		t.Errorf("currentPage = %d, want 0 (fresh state)", m.currentPage)
	}
}

func TestUpdateViewMode_GridToSingle_FreshState(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewGrid

	// Set 1 pane with content
	m.panes = []SessionPaneModel{
		{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "the-session"}},
			entries: []types.LogEntry{
				{Type: "user", Message: types.Message{Role: "user", TextContent: "content"}},
			},
			jsonlPath: "/tmp/path.jsonl",
		},
	}

	cmd := m.updateViewMode()

	if m.viewMode != DashboardViewSingleSession {
		t.Errorf("viewMode = %d, want DashboardViewSingleSession", m.viewMode)
	}
	if m.singleSessionPaneIdx != 0 {
		t.Errorf("singleSessionPaneIdx = %d, want 0", m.singleSessionPaneIdx)
	}
	if m.singleSessionViewer == nil {
		t.Fatal("expected fresh singleSessionViewer")
	}
	// cmd may be nil if viewer has no file watcher
	_ = cmd
}

func TestUpdateViewMode_ZeroToSingle_FreshState(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewZeroSessions

	// Set up stale zero-session viewer
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "old latest"}},
	}
	latestViewer := createLatestViewer(entries, 0, "/tmp/latest.jsonl", 120, 40)
	m.latestViewer = &latestViewer
	m.latestLoading = false

	// Add 1 pane with content
	m.panes = []SessionPaneModel{
		{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "new-session"}},
			entries: []types.LogEntry{
				{Type: "user", Message: types.Message{Role: "user", TextContent: "new content"}},
			},
			jsonlPath: "/tmp/new.jsonl",
		},
	}

	cmd := m.updateViewMode()

	if m.viewMode != DashboardViewSingleSession {
		t.Errorf("viewMode = %d, want DashboardViewSingleSession", m.viewMode)
	}
	if m.singleSessionViewer == nil {
		t.Fatal("expected fresh singleSessionViewer")
	}
	// cmd may be nil if viewer has no file watcher
	_ = cmd
}

func TestUpdateViewMode_SingleToGrid_FreshState(t *testing.T) {
	checker := newTestPIDChecker(100, 101)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewSingleSession

	// Set up stale single-session viewer
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "single session"}},
	}
	singleViewer := createSingleSessionViewer(entries, 0, "/tmp/sess.jsonl", "sess-1", 120, 40)
	m.singleSessionViewer = &singleViewer
	m.singleSessionPaneIdx = 0

	// Add 2 panes to trigger grid mode
	m.panes = []SessionPaneModel{
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 100}}},
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 101}}},
	}

	m.updateViewMode()

	if m.viewMode != DashboardViewGrid {
		t.Errorf("viewMode = %d, want DashboardViewGrid", m.viewMode)
	}
	if m.singleSessionViewer != nil {
		t.Error("expected singleSessionViewer=nil after grid transition")
	}
	if m.singleSessionPaneIdx != -1 {
		t.Errorf("singleSessionPaneIdx = %d, want -1", m.singleSessionPaneIdx)
	}
}

func TestTransitionToSingleSessionMode_DeferredCreation(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Set up 1 pane that is still loading (no entries yet)
	m.panes = []SessionPaneModel{
		{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "loading-session"}},
			loading: true,
			entries: nil,
		},
	}

	cmd := m.transitionToSingleSessionMode()

	if m.viewMode != DashboardViewSingleSession {
		t.Errorf("viewMode = %d, want DashboardViewSingleSession", m.viewMode)
	}
	// Viewer creation is deferred until content arrives
	if m.singleSessionViewer != nil {
		t.Error("expected singleSessionViewer=nil when pane is still loading")
	}
	if m.singleSessionPaneIdx != 0 {
		t.Errorf("singleSessionPaneIdx = %d, want 0", m.singleSessionPaneIdx)
	}
	if cmd != nil {
		t.Error("expected nil cmd for deferred viewer creation")
	}
}

func TestRepeatedTransitions_AlwaysFreshState(t *testing.T) {
	checker := newTestPIDChecker(100, 101)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)

	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "content"}},
	}

	// Cycle: Grid → SingleSession → Grid → ZeroSessions → Grid
	// Each transition should produce completely fresh state.

	// Start in grid with 2 panes
	m.viewMode = DashboardViewGrid
	m.panes = []SessionPaneModel{
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 100}}},
		{session: session.ActiveSession{Meta: session.SessionMeta{PID: 101}}},
	}

	// Transition to single session
	m.panes = []SessionPaneModel{
		{
			session:   session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "s1"}},
			entries:   entries,
			jsonlPath: "/tmp/s1.jsonl",
		},
	}
	m.updateViewMode()
	viewer1 := m.singleSessionViewer
	if viewer1 == nil {
		t.Fatal("expected viewer after first single-session transition")
	}

	// Back to grid
	m.panes = append(m.panes, SessionPaneModel{
		session: session.ActiveSession{Meta: session.SessionMeta{PID: 101}},
	})
	m.updateViewMode()
	if m.singleSessionViewer != nil {
		t.Error("expected nil singleSessionViewer after grid transition")
	}

	// Back to single session
	m.panes = []SessionPaneModel{
		{
			session:   session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "s1"}},
			entries:   entries,
			jsonlPath: "/tmp/s1.jsonl",
		},
	}
	m.updateViewMode()
	viewer2 := m.singleSessionViewer
	if viewer2 == nil {
		t.Fatal("expected viewer after second single-session transition")
	}
	// Must be a different (fresh) instance
	if viewer1 == viewer2 {
		t.Error("expected different ViewerModel instances across transitions (fresh state)")
	}

	// Transition to zero sessions
	m.panes = nil
	m.updateViewMode()
	if m.viewMode != DashboardViewZeroSessions {
		t.Errorf("viewMode = %d, want DashboardViewZeroSessions", m.viewMode)
	}
	if m.singleSessionViewer != nil {
		t.Error("expected nil singleSessionViewer after zero-session transition")
	}
	if !m.latestLoading {
		t.Error("expected latestLoading=true in zero-session mode")
	}
}

// --- Sub-AC 2: createSingleSessionViewer produces a properly configured embedded viewer ---

func TestCreateSingleSessionViewer_Properties(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
		{Type: "assistant", Message: types.Message{Role: "assistant", TextContent: "world"}},
	}

	viewer := createSingleSessionViewer(entries, 1, "/tmp/session.jsonl", "abc123def456", 120, 40)

	// Verify entries are passed through
	if len(viewer.entries) != 2 {
		t.Errorf("entries count = %d, want 2", len(viewer.entries))
	}

	// canGoBack must be false — no back navigation in embedded mode
	if viewer.canGoBack {
		t.Error("expected canGoBack=false for embedded single-session viewer")
	}

	// watchMode must be true — the LIVE indicator should display
	if !viewer.watchMode {
		t.Error("expected watchMode=true for single-session viewer")
	}

	// The viewer must NOT have its own file watcher (the dashboard pane forwards events)
	if viewer.watcher != nil {
		t.Error("expected watcher=nil (dashboard pane watcher forwards events)")
	}

	// Title should contain truncated session ID
	if viewer.title == "" {
		t.Error("expected non-empty title")
	}
}

func TestCreateSingleSessionViewer_LongSessionID(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}
	longID := "abcdefghijklmnopqrstuvwxyz"
	viewer := createSingleSessionViewer(entries, 0, "/tmp/s.jsonl", longID, 80, 24)

	// Long session IDs should be truncated in the title
	if len(viewer.title) > 30 {
		t.Errorf("title too long (%d chars): %q", len(viewer.title), viewer.title)
	}
}

func TestCreateSingleSessionViewer_ZeroDimensions(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}
	// Zero dimensions should not panic
	viewer := createSingleSessionViewer(entries, 0, "/tmp/s.jsonl", "sess-1", 0, 0)
	if len(viewer.entries) != 1 {
		t.Errorf("entries count = %d, want 1", len(viewer.entries))
	}
}

func TestSingleSessionView_ShowsViewerOutput(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewSingleSession

	// Without viewer, should show loading message
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view for loading state")
	}

	// With viewer, should show the viewer's output
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello world"}},
	}
	viewer := createSingleSessionViewer(entries, 0, "/tmp/sess.jsonl", "sess-1", 120, 40)
	m.singleSessionViewer = &viewer
	view = m.View()
	if view == "" {
		t.Error("expected non-empty view from single-session viewer")
	}
}

func TestSingleSessionMode_KeyForwarding(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewSingleSession

	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}
	viewer := createSingleSessionViewer(entries, 0, "/tmp/sess.jsonl", "sess-1", 120, 40)
	m.singleSessionViewer = &viewer

	// Non-exit keys should be forwarded to the viewer (not cause exit)
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	updated := result.(SessionDashboardModel)
	if updated.singleSessionViewer == nil {
		t.Error("expected viewer to still exist after non-exit key")
	}
}

func TestPaneContentLoaded_CreatesSingleSessionViewer(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)
	m.viewMode = DashboardViewSingleSession
	m.singleSessionPaneIdx = 0
	m.singleSessionViewer = nil // Not yet created (deferred)

	// Set up a loading pane
	m.panes = []SessionPaneModel{
		{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "deferred-session"}},
			loading: true,
		},
	}

	// Simulate content loaded
	msg := sessionPaneContentLoadedMsg{
		sessionID:   "deferred-session",
		entries:     []types.LogEntry{{Type: "user", Message: types.Message{Role: "user", TextContent: "loaded"}}},
		parseErrors: 0,
		filePath:    "/tmp/deferred.jsonl",
	}

	result, _ := m.handlePaneContentLoaded(msg)
	updated := result.(SessionDashboardModel)

	if updated.singleSessionViewer == nil {
		t.Error("expected singleSessionViewer to be created after deferred content load")
	}
	if updated.singleSessionViewer != nil && !updated.singleSessionViewer.watchMode {
		t.Error("expected viewer watchMode=true after deferred creation")
	}
}

func TestInitialViewModeIsZeroSessions(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)

	// Constructor sets viewMode to DashboardViewZeroSessions (no panes yet).
	// Init() later loads the latest conversation but does not change viewMode here.
	if got := m.ViewMode(); got != DashboardViewZeroSessions {
		t.Errorf("initial viewMode = %v, want DashboardViewZeroSessions", got)
	}
}
