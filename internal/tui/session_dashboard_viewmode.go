// Package tui provides the terminal user interface components.
package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/token"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// DashboardViewMode represents the current view mode of the session dashboard.
type DashboardViewMode int

const (
	// DashboardViewGrid is the default 3x3 grid view for 2+ sessions.
	DashboardViewGrid DashboardViewMode = iota
	// DashboardViewZeroSessions shows the latest conversation when no sessions are active.
	DashboardViewZeroSessions
	// DashboardViewSingleSession shows a full ViewerModel for a single active session.
	DashboardViewSingleSession
)

// latestConversationLoadedMsg signals that the latest conversation has been loaded
// for the zero-session view mode.
type latestConversationLoadedMsg struct {
	entries     []types.LogEntry
	parseErrors int
	filePath    string
	err         error
}

// findLatestJSONLByMtime returns the JSONL file with the most recent modification
// time in the given directory. Returns empty string if no JSONL files exist.
func findLatestJSONLByMtime(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var latestPath string
	var latestMod int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		modTime := info.ModTime().UnixNano()
		if modTime > latestMod {
			latestMod = modTime
			latestPath = filepath.Join(dir, name)
		}
	}

	return latestPath
}

// loadLatestConversationCmd returns a Bubbletea command that loads the latest
// JSONL file by modification time from the project directory.
func loadLatestConversationCmd(projectDir string) tea.Cmd {
	return func() tea.Msg {
		latestPath := findLatestJSONLByMtime(projectDir)
		if latestPath == "" {
			return latestConversationLoadedMsg{
				err: os.ErrNotExist,
			}
		}

		result, err := parser.ParseJSONLFile(latestPath)
		if err != nil {
			return latestConversationLoadedMsg{
				err:      err,
				filePath: latestPath,
			}
		}
		return latestConversationLoadedMsg{
			entries:     result.Entries,
			parseErrors: result.ParseErrors,
			filePath:    latestPath,
		}
	}
}

// createDashboardViewer creates a ViewerModel with common dashboard settings:
// canGoBack=false, soft-fail token service, and optional size.
// Callers supply the title and may further customize the returned viewer.
func createDashboardViewer(entries []types.LogEntry, parseErrors int, title, filePath string, width, height int) ViewerModel {
	opts := RenderOptions{
		FilePath: filePath,
	}

	// Initialize token service with soft-fail (same pattern as app.go)
	tokenSvc, _ := token.New()

	viewer := NewViewerModel(entries, parseErrors, title, opts, tokenSvc)
	viewer.canGoBack = false
	if width > 0 && height > 0 {
		viewer.SetSize(width, height)
	}
	return viewer
}

// createLatestViewer creates a ViewerModel for the zero-session view mode
// displaying the latest conversation.
func createLatestViewer(entries []types.LogEntry, parseErrors int, filePath string, width, height int) ViewerModel {
	title := "Latest Conversation"
	baseName := filepath.Base(filePath)
	if baseName != "" && baseName != "." {
		// Strip .jsonl extension for cleaner display
		name := strings.TrimSuffix(baseName, ".jsonl")
		title = "Latest: " + name
	}

	return createDashboardViewer(entries, parseErrors, title, filePath, width, height)
}

// detectViewMode determines the appropriate view mode based on active session count.
func detectViewMode(sessionCount int) DashboardViewMode {
	switch {
	case sessionCount == 0:
		return DashboardViewZeroSessions
	case sessionCount == 1:
		return DashboardViewSingleSession
	default:
		return DashboardViewGrid
	}
}

// handleLatestConversationLoaded processes the loaded latest conversation for
// zero-session mode. Creates a ViewerModel to display the conversation.
func (m SessionDashboardModel) handleLatestConversationLoaded(msg latestConversationLoadedMsg) (tea.Model, tea.Cmd) {
	m.latestLoading = false

	if msg.err != nil {
		// No conversations available - latestViewer remains nil, fallback message shown
		return m, nil
	}

	viewer := createLatestViewer(msg.entries, msg.parseErrors, msg.filePath, m.width, m.height)
	m.latestViewer = &viewer

	// Initialize the viewer's viewport
	return m, m.latestViewer.Init()
}

// resetViewState clears all mode-specific view state to defaults.
// Called at the start of every view mode transition to ensure fresh state (AC5).
func (m *SessionDashboardModel) resetViewState() {
	m.singleSessionViewer = nil
	m.singleSessionPaneIdx = -1
	m.latestViewer = nil
	m.latestLoading = false
	m.focusIndex = 0
	m.currentPage = 0
}

// transitionToZeroSessionMode switches the dashboard to zero-session mode.
// Loads the latest conversation and creates a fresh ViewerModel.
// All view state from other modes is reset to defaults (AC5: fresh state on every transition).
func (m *SessionDashboardModel) transitionToZeroSessionMode() tea.Cmd {
	m.viewMode = DashboardViewZeroSessions
	m.resetViewState()
	m.latestLoading = true
	if m.projectDir != "" {
		return loadLatestConversationCmd(m.projectDir)
	}
	m.latestLoading = false
	return nil
}

// transitionToSingleSessionMode switches the dashboard to single-session mode.
// Creates a fresh full-screen ViewerModel from the single active pane's entries.
// The pane index is stored so the viewer can be kept in sync with watcher updates.
// All view state from other modes is reset to defaults (AC5: fresh state on every transition).
func (m *SessionDashboardModel) transitionToSingleSessionMode() tea.Cmd {
	m.viewMode = DashboardViewSingleSession
	m.resetViewState()

	// Must have exactly 1 pane
	if len(m.panes) != 1 {
		return nil
	}

	pane := &m.panes[0]
	m.singleSessionPaneIdx = 0

	// If pane is still loading or has no content yet, defer viewer creation
	// until content arrives via handlePaneContentLoaded.
	if pane.loading || len(pane.entries) == 0 {
		return nil
	}

	viewer := createSingleSessionViewer(pane.entries, pane.parseErrors, pane.jsonlPath, pane.session.Meta.SessionID, m.width, m.height)
	m.singleSessionViewer = &viewer
	return m.singleSessionViewer.Init()
}

// transitionToGridMode switches the dashboard to multi-session grid mode.
// Clears any single-session or zero-session viewer state.
// All view state from other modes is reset to defaults (AC5: fresh state on every transition).
func (m *SessionDashboardModel) transitionToGridMode() tea.Cmd {
	m.viewMode = DashboardViewGrid
	m.resetViewState()
	m.markGridDirty()
	return nil
}

// updateViewMode detects if the view mode should change based on the current
// pane count, and performs the transition if needed. Returns a tea.Cmd if
// the transition requires async work (e.g., loading conversations).
// Fresh view state is created on every transition — no scroll position preserved.
func (m *SessionDashboardModel) updateViewMode() tea.Cmd {
	newMode := detectViewMode(len(m.panes))
	if newMode == m.viewMode {
		return nil
	}

	switch newMode {
	case DashboardViewZeroSessions:
		return m.transitionToZeroSessionMode()
	case DashboardViewSingleSession:
		return m.transitionToSingleSessionMode()
	case DashboardViewGrid:
		return m.transitionToGridMode()
	}
	return nil
}

// createSingleSessionViewer creates a ViewerModel for the single-session view mode
// displaying the active session's conversation at full screen size.
// The viewer does NOT create its own file watcher because the session dashboard's
// pane-level watcher already monitors the file and forwards events to the viewer.
// This avoids duplicate watchers, file descriptor leaks, and potential double-entry bugs.
func createSingleSessionViewer(entries []types.LogEntry, parseErrors int, filePath, sessionID string, width, height int) ViewerModel {
	title := "Session: " + sessionID
	if len(sessionID) > 12 {
		title = "Session: " + sessionID[:12] + "…"
	}

	viewer := createDashboardViewer(entries, parseErrors, title, filePath, width, height)
	// Set watchMode directly so the "LIVE" indicator appears in the footer,
	// matching the standalone viewer experience for active sessions.
	// WatchMode on RenderOptions is intentionally false: the viewer must NOT
	// create its own file watcher because the session dashboard's pane watcher
	// already forwards watcher.NewEntriesMsg events to this viewer.
	viewer.watchMode = true
	return viewer
}

// SingleSessionViewer returns the single-session viewer for testing.
func (m SessionDashboardModel) SingleSessionViewer() *ViewerModel {
	return m.singleSessionViewer
}

// ViewMode returns the current view mode for testing.
func (m SessionDashboardModel) ViewMode() DashboardViewMode {
	return m.viewMode
}

// LatestViewer returns the latest viewer for testing.
func (m SessionDashboardModel) LatestViewer() *ViewerModel {
	return m.latestViewer
}

// LatestLoading returns whether the latest conversation is loading for testing.
func (m SessionDashboardModel) LatestLoading() bool {
	return m.latestLoading
}

// ForceUpdateViewMode triggers view mode detection based on current pane count.
// Exported for testing when panes are manipulated directly (bypassing handlers).
func (m *SessionDashboardModel) ForceUpdateViewMode() {
	m.viewMode = detectViewMode(len(m.panes))
}
