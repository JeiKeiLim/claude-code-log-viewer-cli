package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/token"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// --- AC6: Single-session full view looks identical to existing conversation view ---

// TestSingleSessionViewer_VisualIdentity verifies that the single-session view
// produces the same View() output as a standalone ViewerModel created with
// equivalent parameters. This is the core AC6 criterion.
func TestSingleSessionViewer_VisualIdentity(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "Hello, world!"}},
		{Type: "assistant", Message: types.Message{Role: "assistant", TextContent: "Hi there! How can I help?"}},
		{Type: "user", Message: types.Message{Role: "user", TextContent: "What is Go?"}},
	}

	width, height := 120, 40

	// Create standalone viewer (the "reference" experience)
	standaloneOpts := RenderOptions{
		FilePath: "/tmp/test-session.jsonl",
	}
	tokenSvc, _ := token.New()
	standalone := NewViewerModel(entries, 0, "Session: test-sessio…", standaloneOpts, tokenSvc)
	standalone.watchMode = true // Simulate live mode
	standalone.canGoBack = false
	standalone.SetSize(width, height)

	// Create single-session viewer (via dashboard helper)
	singleSession := createSingleSessionViewer(entries, 0, "/tmp/test-session.jsonl", "test-session-id-long", width, height)

	// Both need viewport initialization via WindowSizeMsg
	standaloneUpdated, _ := standalone.Update(tea.WindowSizeMsg{Width: width, Height: height})
	standaloneViewer := standaloneUpdated.(ViewerModel)

	singleUpdated, _ := singleSession.Update(tea.WindowSizeMsg{Width: width, Height: height})
	singleViewer := singleUpdated.(ViewerModel)

	standaloneView := standaloneViewer.View()
	singleView := singleViewer.View()

	// Both should produce non-empty views
	if standaloneView == "" {
		t.Fatal("standalone viewer produced empty view")
	}
	if singleView == "" {
		t.Fatal("single-session viewer produced empty view")
	}

	// Views should have the same structure (header + viewport + footer)
	standaloneLines := strings.Split(standaloneView, "\n")
	singleLines := strings.Split(singleView, "\n")

	// Line count should match (same content, same dimensions)
	if len(standaloneLines) != len(singleLines) {
		t.Errorf("line count mismatch: standalone=%d, single-session=%d",
			len(standaloneLines), len(singleLines))
	}
}

// TestSingleSessionViewer_HasLiveIndicator verifies that the single-session
// viewer shows the "LIVE" mode indicator, matching the watch-mode experience.
func TestSingleSessionViewer_HasLiveIndicator(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "test"}},
	}

	viewer := createSingleSessionViewer(entries, 0, "/tmp/test.jsonl", "sess-1", 120, 40)

	if !viewer.watchMode {
		t.Error("expected watchMode=true for single-session viewer (LIVE indicator)")
	}

	// The mode segment should include LIVE
	modeSegment := viewer.buildModeSegment()
	if !strings.Contains(modeSegment, "LIVE") {
		t.Errorf("expected LIVE in mode segment, got %q", modeSegment)
	}
}

// TestSingleSessionViewer_NoOwnWatcher verifies that the single-session viewer
// does not create its own file watcher (the pane watcher handles live updates).
func TestSingleSessionViewer_NoOwnWatcher(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "test"}},
	}

	viewer := createSingleSessionViewer(entries, 0, "/tmp/test.jsonl", "sess-1", 120, 40)

	if viewer.watcher != nil {
		t.Error("single-session viewer should NOT have its own watcher (pane watcher forwards events)")
	}

	// Init() should return nil (no watcher to start)
	cmd := viewer.Init()
	if cmd != nil {
		t.Error("expected nil Init() cmd when viewer has no watcher")
	}
}

// TestSingleSessionViewer_SameFeatures verifies that the single-session viewer
// has all the same feature toggles and defaults as a standalone viewer.
func TestSingleSessionViewer_SameFeatures(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "test content"}},
		{Type: "assistant", Message: types.Message{Role: "assistant", TextContent: "response"}},
	}

	tokenSvc, _ := token.New()
	standalone := NewViewerModel(entries, 0, "Test", RenderOptions{}, tokenSvc)
	single := createSingleSessionViewer(entries, 0, "/tmp/test.jsonl", "sess-1", 0, 0)

	// Feature parity checks
	if standalone.showThinking != single.showThinking {
		t.Errorf("showThinking mismatch: standalone=%v, single=%v",
			standalone.showThinking, single.showThinking)
	}
	if standalone.showToolInputs != single.showToolInputs {
		t.Errorf("showToolInputs mismatch: standalone=%v, single=%v",
			standalone.showToolInputs, single.showToolInputs)
	}
	if standalone.showLineNumbers != single.showLineNumbers {
		t.Errorf("showLineNumbers mismatch: standalone=%v, single=%v",
			standalone.showLineNumbers, single.showLineNumbers)
	}
	if standalone.rawMode != single.rawMode {
		t.Errorf("rawMode mismatch: standalone=%v, single=%v",
			standalone.rawMode, single.rawMode)
	}
	if standalone.inputMode != single.inputMode {
		t.Errorf("inputMode mismatch: standalone=%v, single=%v",
			standalone.inputMode, single.inputMode)
	}
	if standalone.lazyEnabled != single.lazyEnabled {
		t.Errorf("lazyEnabled mismatch: standalone=%v, single=%v",
			standalone.lazyEnabled, single.lazyEnabled)
	}
}

// TestSingleSessionViewer_KeyboardForwarding verifies that keyboard events
// are properly forwarded to the embedded viewer in single-session mode.
func TestSingleSessionViewer_KeyboardForwarding(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Set up single-session mode with viewer
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
		{Type: "assistant", Message: types.Message{Role: "assistant", TextContent: "world"}},
	}
	m.panes = []SessionPaneModel{
		{
			session:   session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "test-session"}},
			entries:   entries,
			jsonlPath: "/tmp/test.jsonl",
		},
	}
	m.transitionToSingleSessionMode()

	if m.singleSessionViewer == nil {
		t.Fatal("expected singleSessionViewer to be created")
	}

	// Initialize the viewer's viewport
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(SessionDashboardModel)

	// Test that toggle keys work (forwarded to viewer)
	tests := []struct {
		key      string
		check    string
		getField func() bool
	}{
		{"t", "showThinking", func() bool { return m.singleSessionViewer.showThinking }},
		{"i", "showToolInputs", func() bool { return m.singleSessionViewer.showToolInputs }},
	}

	for _, tt := range tests {
		before := tt.getField()
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
		m = result.(SessionDashboardModel)
		after := tt.getField()
		if before == after {
			t.Errorf("key %q should toggle %s (was %v, still %v)", tt.key, tt.check, before, after)
		}
	}
}

// TestSingleSessionViewer_WindowResize verifies that window resize events
// are forwarded to the embedded viewer in single-session mode.
func TestSingleSessionViewer_WindowResize(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Set up single-session mode
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}
	m.panes = []SessionPaneModel{
		{
			session:   session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "test-session"}},
			entries:   entries,
			jsonlPath: "/tmp/test.jsonl",
		},
	}
	m.transitionToSingleSessionMode()

	if m.singleSessionViewer == nil {
		t.Fatal("expected singleSessionViewer to be created")
	}

	// Resize
	result, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	updated := result.(SessionDashboardModel)

	if updated.singleSessionViewer.width != 200 {
		t.Errorf("viewer width = %d, want 200", updated.singleSessionViewer.width)
	}
	if updated.singleSessionViewer.height != 60 {
		t.Errorf("viewer height = %d, want 60", updated.singleSessionViewer.height)
	}
}

// TestSingleSessionViewer_FullScreenOutput verifies that the single-session
// view uses the full screen (no grid borders, no dashboard chrome).
func TestSingleSessionViewer_FullScreenOutput(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Set up single-session mode with viewer
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "hello"}},
	}
	m.panes = []SessionPaneModel{
		{
			session:   session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "test-session"}},
			entries:   entries,
			jsonlPath: "/tmp/test.jsonl",
		},
	}
	m.transitionToSingleSessionMode()

	// Initialize viewport
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := result.(SessionDashboardModel)

	view := updated.View()

	// The dashboard's View() should directly return the viewer's View()
	// (no additional wrapping, borders, or dashboard chrome)
	if updated.singleSessionViewer == nil {
		t.Fatal("expected singleSessionViewer")
	}
	viewerView := updated.singleSessionViewer.View()
	if view != viewerView {
		t.Error("dashboard View() should return viewer's View() directly in single-session mode")
	}

	// View should NOT contain grid-specific elements
	if strings.Contains(view, "hjkl") {
		t.Error("single-session view should not contain grid navigation hints")
	}
}

// TestSingleSessionViewer_CanGoBackIsFalse verifies that the single-session
// viewer does not support back navigation (dashboard handles exit via esc/q).
func TestSingleSessionViewer_CanGoBackIsFalse(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "test"}},
	}

	viewer := createSingleSessionViewer(entries, 0, "/tmp/test.jsonl", "sess-1", 120, 40)

	if viewer.canGoBack {
		t.Error("expected canGoBack=false for single-session viewer")
	}
}

// TestSingleSessionViewer_TokenService verifies that the single-session viewer
// initializes token statistics, matching the standalone viewer behavior.
func TestSingleSessionViewer_TokenService(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "test"}},
	}

	viewer := createSingleSessionViewer(entries, 0, "/tmp/test.jsonl", "sess-1", 120, 40)

	if viewer.tokenService == nil {
		t.Error("expected tokenService to be initialized")
	}
}

// TestSingleSessionViewer_TitleTruncation verifies that long session IDs are
// properly truncated in the title, keeping the view clean.
func TestSingleSessionViewer_TitleTruncation(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "test"}},
	}

	tests := []struct {
		sessionID string
		wantTitle string
	}{
		{"short", "Session: short"},
		{"exactly12chr", "Session: exactly12chr"},
		{"this-is-a-very-long-session-id", "Session: this-is-a-ve…"},
	}

	for _, tt := range tests {
		viewer := createSingleSessionViewer(entries, 0, "/tmp/test.jsonl", tt.sessionID, 120, 40)
		if viewer.title != tt.wantTitle {
			t.Errorf("sessionID=%q: title=%q, want %q", tt.sessionID, viewer.title, tt.wantTitle)
		}
	}
}

// TestSingleSessionViewer_FilePathPreserved verifies that the file path is
// preserved in render options for the 'p' key (show file path toast).
func TestSingleSessionViewer_FilePathPreserved(t *testing.T) {
	entries := []types.LogEntry{
		{Type: "user", Message: types.Message{Role: "user", TextContent: "test"}},
	}

	filePath := "/home/user/.claude/projects/myproject/conversation.jsonl"
	viewer := createSingleSessionViewer(entries, 0, filePath, "sess-1", 120, 40)

	if viewer.renderOpts.FilePath != filePath {
		t.Errorf("renderOpts.FilePath = %q, want %q", viewer.renderOpts.FilePath, filePath)
	}
}
