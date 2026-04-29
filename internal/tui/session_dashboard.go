// Package tui provides the terminal user interface components.
package tui

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

// sessionDashboardHelpText is the keyboard shortcut guide for session dashboard.
const sessionDashboardHelpText = "arrows/hjkl:nav • [/]:page • Enter:open • c:conversations • Esc:back • auto-detecting sessions"

// MaxSessionPanes is the maximum number of concurrent session panes (3x3 grid).
const MaxSessionPanes = 9

// scanMissThreshold is the number of consecutive scan misses required before
// a pane is removed. This grace period prevents removal due to transient
// file-read races with Claude Code (~6 seconds at 2s scan interval).
const scanMissThreshold = 3

// maxContentLoadRetries is the maximum number of retries when loading JSONL
// content fails (e.g., file not yet created when session JSON appears first).
const maxContentLoadRetries = 5

// contentLoadRetryBaseDelay is the base delay for exponential backoff retries.
const contentLoadRetryBaseDelay = 500 * time.Millisecond

// SessionDashboardModel represents the multi-session dashboard view.
// It auto-detects active Claude Code sessions and displays each in a pane.
type SessionDashboardModel struct {
	panes      []SessionPaneModel
	focusIndex int
	width      int
	height     int

	// Session detection
	scanner     *session.SessionScanner
	monitor     *session.Monitor
	dirWatcher  *session.SessionDirectoryWatcher // Optional fsnotify-based watcher
	projectPath string                           // Project path to filter sessions for
	projectDir  string                           // Claude project dir (encoded path)

	// Lifecycle
	ctx                 context.Context
	cancel              context.CancelFunc
	subscriptionsActive bool
	wg                  *sync.WaitGroup // Tracks all spawned goroutines for leak-free shutdown

	// Channels for session events
	scanResultChan chan session.ScanResult
	monitorChan    chan session.SessionEvent
	dirWatcherChan chan session.SessionEvent // Events from SessionDirectoryWatcher

	// View mode: switches between grid, single-session viewer, and zero-session viewer.
	viewMode             DashboardViewMode
	latestViewer         *ViewerModel // ViewerModel for zero-session mode (latest conversation)
	latestLoading        bool         // True while loading the latest conversation for zero-session mode
	singleSessionViewer  *ViewerModel // ViewerModel for single-session mode (full-screen viewer)
	singleSessionPaneIdx int          // Global pane index backing the single-session viewer (-1 = none)

	// Dirty-region rendering: only re-render panes that changed.
	// gridDirty is set when grid layout changes (resize, pane add/remove)
	// and cleared after View() renders. Since View() uses a value receiver,
	// gridDirty is cleared by Update() after returning from handlers that set it.
	gridDirty bool

	// Pagination: currentPage is zero-indexed page number for the 3x3 grid.
	currentPage int

	// Frame-rate governor: tracks frame timing and skips non-essential
	// redraws when rendering exceeds the 16ms frame budget.
	frameGovernor *FrameRateGovernor

	// Self-healing: timestamp of last subscription tick processed.
	// If the frame tick detects the subscription tick chain has been silent
	// for too long, it restarts it.
	lastSubscriptionTick time.Time
}

// SessionDashboardModelOption configures a SessionDashboardModel.
type SessionDashboardModelOption func(*SessionDashboardModel)

// WithDashboardDirWatcher sets a SessionDirectoryWatcher for real-time
// session detection via fsnotify. When set, file creation/deletion events
// in ~/.claude/sessions/ are mapped directly to pane open/close operations,
// providing sub-second detection latency complementing the polling scanner.
func WithDashboardDirWatcher(w *session.SessionDirectoryWatcher) SessionDashboardModelOption {
	return func(m *SessionDashboardModel) {
		m.dirWatcher = w
		m.dirWatcherChan = make(chan session.SessionEvent, 32)
	}
}

// SessionPaneModel represents a single session pane in the session dashboard.
type SessionPaneModel struct {
	session       session.ActiveSession
	entries       []types.LogEntry
	content       string
	parseErrors   int
	watcher       *watcher.Watcher
	mdRenderer    *MarkdownRenderer
	width         int
	height        int
	loading       bool
	errMsg        string
	jsonlPath     string // Full path to the session's JSONL file
	loadRetries   int    // Number of content load retries attempted
	fileEventChan chan sessionPaneWatcherEventMsg

	// Dirty-region rendering: per-pane caching to avoid expensive re-renders.
	// dirty is set when pane content changes (new entries, content loaded, error).
	// cachedView stores the last ViewWithFocus output.
	// lastFocused tracks whether the pane was focused in the last render.
	dirty       bool   // True when pane content has changed
	cachedView  string // Cached ViewWithFocus output
	lastFocused bool   // Whether pane was focused in last render

	// scanMissCount tracks consecutive full scans where this pane's PID was
	// absent from the results. A transient file-read failure (race with Claude
	// writing to {pid}.json) can cause a single miss; requiring multiple
	// consecutive misses before removal avoids destroying the viewer and
	// losing follow-mode state. Reset to 0 whenever the PID reappears.
	scanMissCount int
}

// Message types for session dashboard

// sessionScanResultMsg delivers scan results to the dashboard.
type sessionScanResultMsg struct {
	result session.ScanResult
}

// sessionClosedMsg signals a session PID has exited.
type sessionClosedMsg struct {
	event session.SessionEvent
}

// sessionPaneContentLoadedMsg signals content has been loaded for a session pane.
type sessionPaneContentLoadedMsg struct {
	sessionID   string
	entries     []types.LogEntry
	parseErrors int
	filePath    string
	err         error
}

// sessionPaneWatcherEventMsg wraps file watcher events with session ID.
type sessionPaneWatcherEventMsg struct {
	sessionID string
	event     tea.Msg
}

// sessionSubscriptionTickMsg is sent periodically to poll channels.
type sessionSubscriptionTickMsg struct{}

// sessionDirWatcherEventMsg delivers a SessionDirectoryWatcher lifecycle event
// (SessionOpened or SessionClosed) to the dashboard for pane management.
type sessionDirWatcherEventMsg struct {
	event session.SessionEvent
}

// GoBackFromSessionDashboardMsg signals return from session dashboard.
type GoBackFromSessionDashboardMsg struct{}

// OpenViewerFromSessionDashboardMsg signals request to open viewer from session dashboard.
type OpenViewerFromSessionDashboardMsg struct {
	FilePath string
	Project  types.Project
}

// OpenConversationsFromSessionDashboardMsg signals request to open the conversation
// list for the current project from the session dashboard. This allows users who
// reached the session dashboard via normal project selection (AC 8) to navigate
// to the conversation list without going back to the project browser first.
type OpenConversationsFromSessionDashboardMsg struct {
	Project types.Project
}

// NewSessionDashboardModel creates a new session dashboard for a specific project.
// projectPath is the decoded filesystem path (e.g., /Users/foo/project).
// projectDir is the Claude encoded project directory (e.g., ~/.claude/projects/-Users-foo-project).
// Optional SessionDashboardModelOption values configure additional components such as
// a SessionDirectoryWatcher for real-time fsnotify-based detection.
func NewSessionDashboardModel(projectPath, projectDir string, scannerInst *session.SessionScanner, monitorInst *session.Monitor, opts ...SessionDashboardModelOption) SessionDashboardModel {
	ctx, cancel := context.WithCancel(context.Background())

	m := SessionDashboardModel{
		projectPath:          projectPath,
		projectDir:           projectDir,
		scanner:              scannerInst,
		monitor:              monitorInst,
		ctx:                  ctx,
		cancel:               cancel,
		subscriptionsActive:  true,
		wg:                   &sync.WaitGroup{},
		scanResultChan:       make(chan session.ScanResult, 4),
		monitorChan:          make(chan session.SessionEvent, 16),
		gridDirty:            true, // Initial render needed
		frameGovernor:        NewFrameRateGovernor(),
		viewMode:             DashboardViewZeroSessions, // Start in zero-session mode (no panes yet)
		singleSessionPaneIdx: -1,                        // No single-session pane
		latestLoading:        projectDir != "",          // Will load latest conversation in Init()
		lastSubscriptionTick: time.Now(),                // Initialize for self-healing check
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// TotalPages returns the total number of pages needed to display all panes.
// Each page holds up to MaxSessionPanes (9) panes in a 3x3 grid.
func (m *SessionDashboardModel) TotalPages() int {
	if len(m.panes) == 0 {
		return 1
	}
	return (len(m.panes) + MaxSessionPanes - 1) / MaxSessionPanes
}

// CurrentPagePanes returns the slice of panes for the current page.
func (m *SessionDashboardModel) CurrentPagePanes() []SessionPaneModel {
	total := len(m.panes)
	if total == 0 {
		return nil
	}
	start := m.currentPage * MaxSessionPanes
	if start >= total {
		return nil
	}
	end := start + MaxSessionPanes
	if end > total {
		end = total
	}
	return m.panes[start:end]
}

// adjustAfterPaneRemoval clamps the current page, adjusts focus within the
// visible page, and recalculates the grid layout after one or more panes have
// been removed. This is shared by handleScanResult and handleSessionClosed.
func (m *SessionDashboardModel) adjustAfterPaneRemoval() {
	m.clampCurrentPage()

	visibleCount := m.currentPagePaneCount()
	if visibleCount == 0 {
		m.focusIndex = 0
	} else if m.focusIndex >= visibleCount {
		m.focusIndex = visibleCount - 1
	}

	m.markGridDirty()
	m.recalcPaneSizes()
}

// clampCurrentPage ensures currentPage is within valid bounds.
func (m *SessionDashboardModel) clampCurrentPage() {
	totalPages := m.TotalPages()
	if m.currentPage >= totalPages {
		m.currentPage = totalPages - 1
	}
	if m.currentPage < 0 {
		m.currentPage = 0
	}
}

// navigatePageForward moves to the next page if available.
// Returns true if the page changed.
func (m *SessionDashboardModel) navigatePageForward() bool {
	if m.currentPage < m.TotalPages()-1 {
		m.currentPage++
		m.focusIndex = 0
		m.markGridDirty()
		m.markAllPanesDirty()
		return true
	}
	return false
}

// navigatePageBack moves to the previous page if available.
// Returns true if the page changed.
func (m *SessionDashboardModel) navigatePageBack() bool {
	if m.currentPage > 0 {
		m.currentPage--
		m.focusIndex = 0
		m.markGridDirty()
		m.markAllPanesDirty()
		return true
	}
	return false
}

// markPaneDirty marks a specific pane as needing re-rendering.
func (m *SessionDashboardModel) markPaneDirty(idx int) {
	if idx >= 0 && idx < len(m.panes) {
		m.panes[idx].dirty = true
	}
}

// markAllPanesDirty marks all panes as needing re-rendering.
func (m *SessionDashboardModel) markAllPanesDirty() {
	for i := range m.panes {
		m.panes[i].dirty = true
	}
}

// markGridDirty marks the grid layout as needing full recompute.
// This also invalidates all pane caches since positions may change.
func (m *SessionDashboardModel) markGridDirty() {
	m.gridDirty = true
	m.markAllPanesDirty()
}

// anyPaneDirty returns true if any pane needs re-rendering.
func (m *SessionDashboardModel) anyPaneDirty() bool {
	for _, p := range m.panes {
		if p.dirty {
			return true
		}
	}
	return false
}

// IsPaneDirty returns whether a specific pane is marked dirty.
// Exported for testing.
func (m *SessionDashboardModel) IsPaneDirty(idx int) bool {
	if idx >= 0 && idx < len(m.panes) {
		return m.panes[idx].dirty
	}
	return false
}

// clearGridDirty resets the grid dirty flag after rendering.
// Called from Update() on the subscription tick to ensure gridDirty
// is cleared after View() has had a chance to use it.
func (m *SessionDashboardModel) clearGridDirty() {
	m.gridDirty = false
}

// IsGridDirty returns whether the grid layout needs recomputing.
// Exported for testing.
func (m *SessionDashboardModel) IsGridDirty() bool {
	return m.gridDirty
}

// PaneCachedView returns the cached view of a specific pane. Exported for testing.
func (m *SessionDashboardModel) PaneCachedView(idx int) string {
	if idx >= 0 && idx < len(m.panes) {
		return m.panes[idx].cachedView
	}
	return ""
}

// Init starts the session detection pipeline.
func (m SessionDashboardModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.startScannerCmd(),
		m.startMonitorCmd(),
		m.sessionSubscriptionTickCmd(),
		frameTickCmd(),
	}
	if m.dirWatcher != nil {
		cmds = append(cmds, m.startDirWatcherCmd())
	}
	// Start with zero-session mode: load the latest conversation immediately
	// (latestLoading is set in the constructor since Init() is a value receiver)
	if m.projectDir != "" {
		cmds = append(cmds, loadLatestConversationCmd(m.projectDir))
	}
	return tea.Batch(cmds...)
}

// startScannerCmd starts the session scanner polling loop and bridges results.
func (m SessionDashboardModel) startScannerCmd() tea.Cmd {
	return func() tea.Msg {
		resultCh := m.scanner.Start()
		if resultCh == nil {
			return nil // Already running
		}

		// Bridge scanner results to our channel
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			for {
				select {
				case <-m.ctx.Done():
					m.scanner.Stop()
					return
				case result, ok := <-resultCh:
					if !ok {
						return
					}
					select {
					case m.scanResultChan <- result:
					case <-m.ctx.Done():
						return
					default:
						// Channel full, skip
					}
				}
			}
		}()

		return nil
	}
}

// startMonitorCmd starts the monitor and bridges events to our channel.
func (m SessionDashboardModel) startMonitorCmd() tea.Cmd {
	return func() tea.Msg {
		m.monitor.Start(m.ctx)

		// Bridge monitor events to our channel
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			for {
				select {
				case <-m.ctx.Done():
					return
				case event, ok := <-m.monitor.Events():
					if !ok {
						return
					}
					select {
					case m.monitorChan <- event:
					case <-m.ctx.Done():
						return
					default:
						// Channel full, skip
					}
				}
			}
		}()

		return nil
	}
}

// startDirWatcherCmd starts the SessionDirectoryWatcher and bridges its events
// to dirWatcherChan. File creation events map to SessionOpened (new pane) and
// file deletion events map to SessionClosed (pane removal).
// This provides sub-second detection latency for Claude >= v2.1.81.
func (m SessionDashboardModel) startDirWatcherCmd() tea.Cmd {
	return func() tea.Msg {
		if m.dirWatcher == nil || m.dirWatcherChan == nil {
			return nil
		}

		if err := m.dirWatcher.Start(); err != nil {
			// Non-fatal: scanner fallback still provides polling detection.
			return nil
		}

		// Bridge dir watcher events to dirWatcherChan.
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			events := m.dirWatcher.Events()
			for {
				select {
				case <-m.ctx.Done():
					return
				case event, ok := <-events:
					if !ok {
						return
					}
					select {
					case m.dirWatcherChan <- event:
					case <-m.ctx.Done():
						return
					default:
						// Channel full — drop to avoid blocking watcher goroutine.
					}
				}
			}
		}()

		return nil
	}
}

// Update implements tea.Model.
// forwardToViewer sends msg to the given embedded ViewerModel pointer,
// updates it in place, and returns the resulting tea.Cmd.
func forwardToViewer(viewer **ViewerModel, msg tea.Msg) tea.Cmd {
	newViewer, cmd := (*viewer).Update(msg)
	v := newViewer.(ViewerModel)
	*viewer = &v
	return cmd
}

func (m SessionDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		// Forward resize to embedded viewers if active
		if m.latestViewer != nil {
			m.latestViewer.SetSize(msg.Width, msg.Height)
		}
		if m.singleSessionViewer != nil {
			m.singleSessionViewer.SetSize(msg.Width, msg.Height)
		}
		m.markGridDirty() // Resize invalidates entire grid
		for i := range m.panes {
			if len(m.panes[i].entries) > 0 {
				renderWidth := m.panes[i].width - 6
				if renderWidth < 20 {
					renderWidth = 20
				}
				m.panes[i].mdRenderer, _ = NewMarkdownRenderer(renderWidth)
				m.panes[i].content = m.panes[i].renderContent()
			}
		}
		return m, nil

	case spinner.TickMsg:
		// Forward spinner tick messages to the active embedded viewer so that
		// overlay loading animations animate correctly in single-session and
		// zero-session modes (matching the normal app.go viewer path).
		if m.viewMode == DashboardViewSingleSession && m.singleSessionViewer != nil {
			return m, forwardToViewer(&m.singleSessionViewer, msg)
		}
		if m.viewMode == DashboardViewZeroSessions && m.latestViewer != nil {
			return m, forwardToViewer(&m.latestViewer, msg)
		}
		return m, nil

	case tea.KeyMsg:
		// In zero-session mode, forward all keys to the embedded viewer
		// except esc/q which should exit the dashboard.
		if m.viewMode == DashboardViewZeroSessions && m.latestViewer != nil {
			if msg.String() == "esc" || msg.String() == "q" {
				m.closeAll()
				return m, func() tea.Msg { return GoBackFromSessionDashboardMsg{} }
			}
			return m, forwardToViewer(&m.latestViewer, msg)
		}

		// In single-session mode, forward all keys to the embedded viewer
		// except esc/q which should exit the dashboard.
		if m.viewMode == DashboardViewSingleSession && m.singleSessionViewer != nil {
			if msg.String() == "esc" || msg.String() == "q" {
				m.closeAll()
				return m, func() tea.Msg { return GoBackFromSessionDashboardMsg{} }
			}
			return m, forwardToViewer(&m.singleSessionViewer, msg)
		}

		switch msg.String() {
		case "esc", "q":
			m.closeAll()
			return m, func() tea.Msg { return GoBackFromSessionDashboardMsg{} }
		case "up", "k":
			return m.handleFocusMove("up")
		case "down", "j":
			return m.handleFocusMove("down")
		case "left", "h":
			return m.handleFocusMove("left")
		case "right", "l":
			return m.handleFocusMove("right")
		case "enter":
			globalIdx := m.currentPage*MaxSessionPanes + m.focusIndex
			if globalIdx >= 0 && globalIdx < len(m.panes) {
				pane := m.panes[globalIdx]
				if pane.jsonlPath == "" {
					return m, nil
				}
				return m, func() tea.Msg {
					return OpenViewerFromSessionDashboardMsg{
						FilePath: pane.jsonlPath,
						Project: types.Project{
							DecodedPath: m.projectPath,
							DisplayName: filepath.Base(m.projectPath),
							DirPath:     m.projectDir,
						},
					}
				}
			}
			return m, nil

		case "[":
			m.navigatePageBack()
			return m, nil
		case "]":
			m.navigatePageForward()
			return m, nil

		case "c":
			// Open the conversation list for the current project.
			// This is used when the session dashboard was opened via normal project
			// selection (AC 8) and the user wants to browse historical conversations.
			project := types.Project{
				DecodedPath: m.projectPath,
				DisplayName: filepath.Base(m.projectPath),
				DirPath:     m.projectDir,
			}
			m.closeAll()
			return m, func() tea.Msg {
				return OpenConversationsFromSessionDashboardMsg{Project: project}
			}
		}

	case latestConversationLoadedMsg:
		return m.handleLatestConversationLoaded(msg)

	case sessionScanResultMsg:
		return m.handleScanResult(msg.result)

	case sessionDirWatcherEventMsg:
		return m.handleDirWatcherEvent(msg.event)

	case sessionClosedMsg:
		return m.handleSessionClosed(msg.event)

	case sessionPaneContentLoadedMsg:
		return m.handlePaneContentLoaded(msg)

	case sessionPaneWatcherEventMsg:
		return m.handleWatcherEvent(msg)

	case frameTickMsg:
		if !m.subscriptionsActive {
			return m, nil
		}
		// Self-healing: if subscription tick hasn't fired in 2 seconds, restart it.
		// This recovers from any edge case where the tick chain silently breaks.
		if m.subscriptionsActive && !m.lastSubscriptionTick.IsZero() &&
			time.Since(m.lastSubscriptionTick) > 2*time.Second {
			m.lastSubscriptionTick = time.Now()
			return m, tea.Batch(frameTickCmd(), m.sessionSubscriptionTickCmd())
		}
		// Frame-rate governor: check if we should skip non-essential redraws.
		// Essential updates (grid changes, focused pane) always proceed.
		// Non-essential updates (unfocused pane content changes) are deferred.
		if m.frameGovernor != nil && m.frameGovernor.ShouldSkipNonEssential() {
			// Under budget pressure: record skips for unfocused dirty panes
			// but do NOT clear their dirty flags — the content change must
			// persist until the pane is actually re-rendered in View().
			globalFocusIdx := m.currentPage*MaxSessionPanes + m.focusIndex
			for i := range m.panes {
				if i != globalFocusIdx && m.panes[i].dirty && !m.gridDirty {
					m.frameGovernor.RecordSkip()
				}
			}
		}
		return m, frameTickCmd()

	case sessionSubscriptionTickMsg:
		if !m.subscriptionsActive {
			return m, nil
		}
		m.lastSubscriptionTick = time.Now()

		// Clear grid dirty flag after View() has consumed it.
		// View() runs between Update() calls, so by the next tick,
		// the gridDirty=true state has been rendered.
		m.gridDirty = false

		if polledMsg := m.pollChannels(); polledMsg != nil {
			newModel, eventCmd := m.Update(polledMsg)
			updatedModel := newModel.(SessionDashboardModel)
			if updatedModel.subscriptionsActive {
				return updatedModel, tea.Batch(eventCmd, updatedModel.sessionSubscriptionTickCmd())
			}
			return updatedModel, eventCmd
		}

		return m, m.sessionSubscriptionTickCmd()
	}

	// Forward any unhandled messages to the embedded viewer (e.g. viewerMessagesLoadedMsg,
	// rawLinesLoadedMsg, spinner.TickMsg for overlay animation, gotoTopMsg, etc.).
	// These messages originate from cmds returned by the embedded viewer and must be
	// routed back to it so lazy loading, the overlay spinner, and G-key navigation
	// all function identically to the standalone viewer path.
	if m.viewMode == DashboardViewSingleSession && m.singleSessionViewer != nil {
		return m, forwardToViewer(&m.singleSessionViewer, msg)
	}
	if m.viewMode == DashboardViewZeroSessions && m.latestViewer != nil {
		return m, forwardToViewer(&m.latestViewer, msg)
	}

	return m, nil
}

// handleScanResult processes a scan result and adds/removes panes.
// On each scan tick, panes whose PIDs are no longer in the scan results
// (i.e., dead/ended sessions) are removed from the dashboard.
func (m SessionDashboardModel) handleScanResult(result session.ScanResult) (tea.Model, tea.Cmd) {
	if result.Err != nil {
		return m, nil
	}

	// Filter sessions for our project
	projectSessions := result.Sessions
	if m.projectPath != "" {
		projectSessions = session.FilterByProject(result.Sessions, m.projectPath)
	}

	// Deduplicate sessions sharing the same sessionId, keeping only the
	// latest PID. This prevents ghost panes when a Claude Code process
	// restarts and reuses the same sessionId with a new PID.
	projectSessions = session.DeduplicateBySessionID(projectSessions)

	// Build set of live PIDs from scan results for dead session filtering.
	// The scanner already filters out dead PIDs (PID liveness check in Scan()),
	// so any pane whose PID is absent from a full scan has exited.
	// Sessions in the SessionRemoved state (5+ minutes of no JSONL writes) are
	// excluded from the live set so their panes are removed from the dashboard.
	livePIDs := make(map[int]bool, len(projectSessions))
	sessionByPID := make(map[int]session.ActiveSession, len(projectSessions))
	for _, sess := range projectSessions {
		if sess.State != session.SessionRemoved {
			livePIDs[sess.Meta.PID] = true
		}
		sessionByPID[sess.Meta.PID] = sess
	}

	// Update session state on existing panes so lifecycle transitions
	// (Active → Idle → Removed) are reflected in the dashboard.
	for i := range m.panes {
		pid := m.panes[i].session.Meta.PID
		if updated, ok := sessionByPID[pid]; ok {
			m.panes[i].session.State = updated.State
			m.panes[i].session.JSONLLastModified = updated.JSONLLastModified
		}
	}

	// Use the IsFullScan flag from the scanner to determine whether this result
	// represents a complete directory scan. Full scans enumerate ALL alive sessions,
	// so any pane whose PID is absent has truly exited (dead PID via syscall.Kill).
	// This ensures dead PIDs are removed immediately regardless of JSONL timing,
	// even when ALL sessions die simultaneously.
	// Non-full-scan results (e.g., synthetic single-session events) only add panes
	// and never trigger removal, to avoid false positives.
	isFullScan := result.IsFullScan && len(m.panes) > 0

	// Remove panes for dead/ended sessions whose PIDs are no longer alive.
	// Use a grace period (scanMissThreshold consecutive misses) to avoid
	// removing panes due to transient file-read races with Claude Code.
	removedAny := false
	if isFullScan {
		for i := len(m.panes) - 1; i >= 0; i-- {
			pid := m.panes[i].session.Meta.PID
			if !livePIDs[pid] {
				m.panes[i].scanMissCount++
				if m.panes[i].scanMissCount < scanMissThreshold {
					continue // Grace period: wait for more consecutive misses
				}
				// Close watcher for dead session
				if m.panes[i].watcher != nil {
					_ = m.panes[i].watcher.Close()
				}
				// Untrack from monitor
				m.monitor.UntrackSession(pid)
				// Remove pane
				m.panes = append(m.panes[:i], m.panes[i+1:]...)
				removedAny = true
			} else {
				// PID reappeared — reset miss counter
				m.panes[i].scanMissCount = 0
			}
		}
	}

	if removedAny {
		m.adjustAfterPaneRemoval()
	}

	// Find new sessions (not yet displayed as panes).
	// Sessions in SessionRemoved state are not added as new panes — they have
	// already exceeded the 5-minute inactivity removal threshold.
	var cmds []tea.Cmd
	existingPIDs := make(map[int]bool)
	for _, pane := range m.panes {
		existingPIDs[pane.session.Meta.PID] = true
	}

	for _, sess := range projectSessions {
		if existingPIDs[sess.Meta.PID] {
			continue // Already have pane for this session
		}

		// Do not add panes for sessions already past the removal threshold.
		if sess.State == session.SessionRemoved {
			continue
		}

		// Track in monitor for PID liveness checking
		m.monitor.TrackSession(sess)

		// Create new pane
		pane := SessionPaneModel{
			session: sess,
			loading: true,
		}

		// Determine JSONL path from session metadata
		jsonlPath := m.resolveJSONLPath(sess)
		pane.jsonlPath = jsonlPath

		m.panes = append(m.panes, pane)

		// Recalculate dimensions for new pane count — grid layout changed
		m.markGridDirty()
		m.recalcPaneSizes()

		// Load content for this pane
		if jsonlPath != "" {
			cmds = append(cmds, loadSessionPaneContentCmd(sess.Meta.SessionID, jsonlPath))
		}
	}

	// Detect view mode transitions based on current session count
	if transitionCmd := m.updateViewMode(); transitionCmd != nil {
		cmds = append(cmds, transitionCmd)
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// handleDirWatcherEvent processes a SessionDirectoryWatcher lifecycle event.
// SessionOpened maps to pane creation (same as handleScanResult for a single session).
// SessionClosed maps to pane removal (same as handleSessionClosed).
// This provides real-time session tracking via fsnotify file events in
// ~/.claude/sessions/ without waiting for the 2-second polling cycle.
func (m SessionDashboardModel) handleDirWatcherEvent(event session.SessionEvent) (tea.Model, tea.Cmd) {
	switch event.Type {
	case session.SessionOpened:
		// Add a single new session pane without triggering dead-session removal.
		// handleScanResult performs full reconciliation (removing panes not in
		// the scan result), which would incorrectly remove all other panes when
		// called with a synthetic single-session result. Instead, use the
		// dedicated addSessionPane helper for incremental pane addition.
		return m.addSessionPane(event.Session)

	case session.SessionClosed:
		return m.handleSessionClosed(event)
	}
	return m, nil
}

// addSessionPane adds a single new session pane if it passes the project filter
// and isn't already displayed. This is used for incremental session additions
// (e.g., from dir watcher events) that should not trigger dead-session removal.
func (m SessionDashboardModel) addSessionPane(sess session.ActiveSession) (tea.Model, tea.Cmd) {
	// Filter for our project
	if m.projectPath != "" {
		projectSessions := session.FilterByProject([]session.ActiveSession{sess}, m.projectPath)
		if len(projectSessions) == 0 {
			return m, nil
		}
		sess = projectSessions[0]
	}

	// Check if already displayed (by PID)
	for _, pane := range m.panes {
		if pane.session.Meta.PID == sess.Meta.PID {
			return m, nil // Already have pane for this session
		}
	}

	// Deduplicate by sessionId: if a pane with the same sessionId already
	// exists but with an older (lower) PID, remove the old pane so the new
	// one replaces it. This handles process restarts that reuse a sessionId.
	if sess.Meta.SessionID != "" {
		for i := len(m.panes) - 1; i >= 0; i-- {
			if m.panes[i].session.Meta.SessionID == sess.Meta.SessionID &&
				m.panes[i].session.Meta.PID < sess.Meta.PID {
				// Close watcher for the superseded pane
				if m.panes[i].watcher != nil {
					_ = m.panes[i].watcher.Close()
				}
				m.monitor.UntrackSession(m.panes[i].session.Meta.PID)
				m.panes = append(m.panes[:i], m.panes[i+1:]...)
			}
		}
	}

	// Track in monitor for PID liveness checking
	m.monitor.TrackSession(sess)

	// Create new pane
	pane := SessionPaneModel{
		session: sess,
		loading: true,
	}

	// Determine JSONL path from session metadata
	jsonlPath := m.resolveJSONLPath(sess)
	pane.jsonlPath = jsonlPath

	m.panes = append(m.panes, pane)

	// Recalculate dimensions for new pane count — grid layout changed
	m.markGridDirty()
	m.recalcPaneSizes()

	// Check for view mode transition after pane addition
	var cmds []tea.Cmd
	if transitionCmd := m.updateViewMode(); transitionCmd != nil {
		cmds = append(cmds, transitionCmd)
	}

	// Load content for this pane
	if jsonlPath != "" {
		cmds = append(cmds, loadSessionPaneContentCmd(sess.Meta.SessionID, jsonlPath))
	}
	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// handleSessionClosed removes the pane for a closed session.
func (m SessionDashboardModel) handleSessionClosed(event session.SessionEvent) (tea.Model, tea.Cmd) {
	pid := event.Session.Meta.PID

	for i, pane := range m.panes {
		if pane.session.Meta.PID == pid {
			// Close watcher
			if pane.watcher != nil {
				_ = pane.watcher.Close()
			}
			// Remove pane
			m.panes = append(m.panes[:i], m.panes[i+1:]...)

			m.adjustAfterPaneRemoval()
			break
		}
	}

	// Check for view mode transition after pane removal
	if transitionCmd := m.updateViewMode(); transitionCmd != nil {
		return m, transitionCmd
	}

	return m, nil
}

// handlePaneContentLoaded processes loaded content for a session pane.
func (m SessionDashboardModel) handlePaneContentLoaded(msg sessionPaneContentLoadedMsg) (tea.Model, tea.Cmd) {
	idx := m.findPaneBySessionID(msg.sessionID)
	if idx < 0 {
		return m, nil
	}

	pane := &m.panes[idx]
	pane.loading = false

	if msg.err != nil {
		// JSONL file may not exist yet (session JSON appears before JSONL).
		// Retry up to maxContentLoadRetries with exponential backoff.
		if pane.loadRetries < maxContentLoadRetries && pane.jsonlPath != "" {
			pane.loadRetries++
			delay := contentLoadRetryBaseDelay * time.Duration(1<<(pane.loadRetries-1))
			return m, retryLoadSessionPaneContentCmd(msg.sessionID, pane.jsonlPath, delay)
		}
		pane.errMsg = msg.err.Error()
		pane.dirty = true
		return m, nil
	}

	// Successful load — clear any previous error and reset retry counter
	pane.errMsg = ""
	pane.loadRetries = 0

	pane.entries = msg.entries
	pane.parseErrors = msg.parseErrors

	if pane.mdRenderer == nil && pane.width > 0 {
		renderWidth := pane.width - 6
		if renderWidth < 20 {
			renderWidth = 20
		}
		pane.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
	}

	pane.content = pane.renderContent()
	pane.dirty = true

	// Start file watcher for live updates
	if msg.filePath != "" {
		if pane.watcher != nil {
			_ = pane.watcher.Close()
			pane.watcher = nil
		}
		pane.fileEventChan = nil

		w, err := watcher.New(msg.filePath)
		if err == nil {
			pane.watcher = w
			pane.jsonlPath = msg.filePath
			m.startSessionFileWatcherSubscription(m.ctx, idx)
		}
	}

	// If in single-session mode and the viewer hasn't been created yet
	// (because the pane was still loading during transition), create it now.
	if m.viewMode == DashboardViewSingleSession && m.singleSessionViewer == nil && idx == m.singleSessionPaneIdx {
		viewer := createSingleSessionViewer(pane.entries, pane.parseErrors, pane.jsonlPath, pane.session.Meta.SessionID, m.width, m.height)
		m.singleSessionViewer = &viewer
		return m, m.singleSessionViewer.Init()
	}

	return m, nil
}

// handleWatcherEvent processes file watcher events for session panes.
func (m SessionDashboardModel) handleWatcherEvent(msg sessionPaneWatcherEventMsg) (tea.Model, tea.Cmd) {
	idx := m.findPaneBySessionID(msg.sessionID)
	if idx < 0 {
		return m, nil
	}

	pane := &m.panes[idx]

	switch event := msg.event.(type) {
	case watcher.NewEntriesMsg:
		pane.entries = append(pane.entries, event.Entries...)
		pane.content = pane.renderContent()
		pane.dirty = true

		// In single-session mode, forward new entries to the embedded viewer
		// so it stays in sync with the backing pane's live data.
		if m.viewMode == DashboardViewSingleSession && m.singleSessionViewer != nil && idx == m.singleSessionPaneIdx {
			return m, forwardToViewer(&m.singleSessionViewer, event)
		}
	case watcher.FileResetMsg:
		// Reload — pane will be marked dirty when content loads
		return m, loadSessionPaneContentCmd(pane.session.Meta.SessionID, pane.jsonlPath)
	}

	return m, nil
}

// resolveJSONLPath returns the JSONL file path for a session.
// SessionID maps directly to JSONL filename in the project directory.
func (m SessionDashboardModel) resolveJSONLPath(sess session.ActiveSession) string {
	if sess.Meta.SessionID == "" || m.projectDir == "" {
		return ""
	}
	return filepath.Join(m.projectDir, sess.Meta.SessionID+".jsonl")
}

// findPaneBySessionID returns the index of the pane with the given session ID, or -1.
func (m SessionDashboardModel) findPaneBySessionID(sessionID string) int {
	for i, pane := range m.panes {
		if pane.session.Meta.SessionID == sessionID {
			return i
		}
	}
	return -1
}

// findPaneByPID returns the index of the pane with the given PID, or -1.
func (m SessionDashboardModel) findPaneByPID(pid int) int {
	for i, pane := range m.panes {
		if pane.session.Meta.PID == pid {
			return i
		}
	}
	return -1
}

// loadSessionPaneContentCmd returns a command that loads JSONL content for a session.
func loadSessionPaneContentCmd(sessionID, jsonlPath string) tea.Cmd {
	return func() tea.Msg {
		result, err := parser.ParseJSONLFile(jsonlPath)
		if err != nil {
			return sessionPaneContentLoadedMsg{
				sessionID: sessionID,
				err:       err,
			}
		}
		return sessionPaneContentLoadedMsg{
			sessionID:   sessionID,
			entries:     result.Entries,
			parseErrors: result.ParseErrors,
			filePath:    jsonlPath,
		}
	}
}

// retryLoadSessionPaneContentCmd returns a command that retries loading after a delay.
func retryLoadSessionPaneContentCmd(sessionID, jsonlPath string, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(_ time.Time) tea.Msg {
		result, err := parser.ParseJSONLFile(jsonlPath)
		if err != nil {
			return sessionPaneContentLoadedMsg{
				sessionID: sessionID,
				err:       err,
			}
		}
		return sessionPaneContentLoadedMsg{
			sessionID:   sessionID,
			entries:     result.Entries,
			parseErrors: result.ParseErrors,
			filePath:    jsonlPath,
		}
	})
}

// startSessionFileWatcherSubscription starts a goroutine to relay file watcher events.
func (m *SessionDashboardModel) startSessionFileWatcherSubscription(ctx context.Context, paneIndex int) {
	if paneIndex < 0 || paneIndex >= len(m.panes) {
		return
	}
	pane := &m.panes[paneIndex]
	if pane.watcher == nil {
		return
	}

	sessionID := pane.session.Meta.SessionID
	ch := make(chan sessionPaneWatcherEventMsg, subscriptionChannelBuffer)
	pane.fileEventChan = ch

	// Capture references before goroutine to avoid race with closeAll() and struct reassignment
	w := pane.watcher
	events := w.EventsChan()
	errors := w.ErrorsChan()
	wg := m.wg

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					entries, err := w.ReadNewEntries()
					var msg sessionPaneWatcherEventMsg
					if err == watcher.ErrFileTruncated {
						msg = sessionPaneWatcherEventMsg{sessionID: sessionID, event: watcher.FileResetMsg{}}
					} else if err != nil {
						msg = sessionPaneWatcherEventMsg{sessionID: sessionID, event: watcher.WatcherErrorMsg{Err: err}}
					} else if len(entries) > 0 {
						msg = sessionPaneWatcherEventMsg{sessionID: sessionID, event: watcher.NewEntriesMsg{Entries: entries}}
					} else {
						continue
					}
					select {
					case ch <- msg:
					case <-ctx.Done():
						return
					}
				}
			case _, ok := <-errors:
				if !ok {
					return
				}
			}
		}
	}()
}

// pollChannels checks all channels for pending events (non-blocking).
func (m *SessionDashboardModel) pollChannels() tea.Msg {
	// Check scan result channel
	select {
	case result, ok := <-m.scanResultChan:
		if ok {
			return sessionScanResultMsg{result: result}
		}
	default:
	}

	// Check monitor event channel
	select {
	case event, ok := <-m.monitorChan:
		if ok {
			return sessionClosedMsg{event: event}
		}
	default:
	}

	// Check dir watcher channel: file creation → pane add; deletion → pane remove.
	if m.dirWatcherChan != nil {
		select {
		case event, ok := <-m.dirWatcherChan:
			if ok {
				return sessionDirWatcherEventMsg{event: event}
			}
		default:
		}
	}

	// Check file watcher channels for each pane
	for i := range m.panes {
		if ch := m.panes[i].fileEventChan; ch != nil {
			select {
			case msg, ok := <-ch:
				if ok {
					return msg
				}
			default:
			}
		}
	}

	return nil
}

// sessionSubscriptionTickCmd schedules the next polling tick (100ms).
func (m SessionDashboardModel) sessionSubscriptionTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return sessionSubscriptionTickMsg{}
	})
}

// closeAll shuts down all watchers and goroutines.
func (m *SessionDashboardModel) closeAll() {
	m.subscriptionsActive = false

	if m.cancel != nil {
		m.cancel()
	}

	for i := range m.panes {
		if m.panes[i].watcher != nil {
			_ = m.panes[i].watcher.Close()
			m.panes[i].watcher = nil
		}
		m.panes[i].fileEventChan = nil
	}

	m.scanner.Stop()
	m.monitor.Stop()

	// Close the directory watcher after cancelling the context so the bridge
	// goroutine can exit cleanly before the fsnotify channels are closed.
	if m.dirWatcher != nil {
		_ = m.dirWatcher.Close()
	}

	// Stop frame-rate governor
	if m.frameGovernor != nil {
		m.frameGovernor.Stop()
	}

	// Wait for all bridge/subscription goroutines to complete
	if m.wg != nil {
		m.wg.Wait()
	}
}

// View renders the session dashboard using the adaptive grid layout engine.
// Uses dirty-region rendering: only panes with dirty flags are re-rendered.
// Clean panes return their cached ViewWithFocus output, avoiding expensive
// content formatting and border application. The grid composition (string joining)
// is always performed since it's cheap, but per-pane rendering is the primary
// cost that dirty tracking optimizes.
//
// Dirty flags are set by Update() handlers when pane state changes:
//   - gridDirty: resize, pane add/remove → all panes re-render
//   - pane.dirty: content loaded, watcher event → single pane re-renders
//   - Focus change: only old and new focused panes re-render
func (m SessionDashboardModel) View() string {
	// Frame budget tracking: record frame start/end for governor decisions.
	if m.frameGovernor != nil {
		m.frameGovernor.FrameStart()
		defer m.frameGovernor.FrameEnd()
	}

	if m.viewMode == DashboardViewZeroSessions {
		// Zero sessions: show latest conversation via embedded ViewerModel
		if m.latestLoading {
			waiting := Styles.Muted.Render("Loading latest conversation...")
			helpText := Styles.HelpText.Render(sessionDashboardHelpText)
			return lipgloss.JoinVertical(lipgloss.Left, waiting, helpText)
		}
		if m.latestViewer != nil {
			return m.latestViewer.View()
		}
		// No conversations found - show fallback message
		waiting := Styles.Muted.Render("No conversations found. Waiting for active Claude Code sessions...")
		helpText := Styles.HelpText.Render(sessionDashboardHelpText)
		return lipgloss.JoinVertical(lipgloss.Left, waiting, helpText)
	}

	if m.viewMode == DashboardViewSingleSession {
		// Single session: show full-screen ViewerModel
		if m.singleSessionViewer != nil {
			return m.singleSessionViewer.View()
		}
		// Viewer not yet created (pane still loading)
		waiting := Styles.Muted.Render("Loading session conversation...")
		helpText := Styles.HelpText.Render(sessionDashboardHelpText)
		return lipgloss.JoinVertical(lipgloss.Left, waiting, helpText)
	}

	if len(m.panes) == 0 {
		// No active sessions and not in zero-session mode (shouldn't normally reach here)
		waiting := Styles.Muted.Render("Waiting for active Claude Code sessions...")
		helpText := Styles.HelpText.Render(sessionDashboardHelpText)
		return lipgloss.JoinVertical(lipgloss.Left, waiting, helpText)
	}

	// Pagination: determine which panes are visible on the current page
	pageStart := m.currentPage * MaxSessionPanes
	if pageStart >= len(m.panes) {
		pageStart = 0 // Safety fallback
	}
	pageEnd := pageStart + MaxSessionPanes
	if pageEnd > len(m.panes) {
		pageEnd = len(m.panes)
	}
	visiblePanes := m.panes[pageStart:pageEnd]
	visibleCount := len(visiblePanes)

	// Reserve space for help text and optional page indicator
	reservedLines := 1
	totalPages := m.TotalPages()
	if totalPages > 1 {
		reservedLines = 2 // Help text + page indicator
	}
	gridHeight := m.height - reservedLines
	if gridHeight < 3 {
		gridHeight = 3
	}

	layout := CalculateGridLayout(visibleCount, m.width, gridHeight)
	if layout.Rows == 0 || layout.Cols == 0 {
		return ""
	}

	// Group pane layouts by row for rendering
	rowPanes := make(map[int][]PaneLayout)
	for _, pl := range layout.Panes {
		rowPanes[pl.Row] = append(rowPanes[pl.Row], pl)
	}

	var rowViews []string
	for r := 0; r < layout.Rows; r++ {
		panesInRow := rowPanes[r]
		var colViews []string
		for _, pl := range panesInRow {
			if pl.Index < visibleCount {
				// Map page-local index to global pane index
				globalIdx := pageStart + pl.Index
				pane := &m.panes[globalIdx]
				pane.width = pl.Width
				pane.height = pl.Height
				focused := pl.Index == m.focusIndex

				// Only re-render pane if dirty, grid changed, focus state changed, or no cache
				focusChanged := pane.lastFocused != focused
				if pane.dirty || m.gridDirty || pane.cachedView == "" || focusChanged {
					pane.cachedView = pane.ViewWithFocus(focused)
					pane.dirty = false
					pane.lastFocused = focused
				}

				colViews = append(colViews, pane.cachedView)
			}
		}
		if len(colViews) > 0 {
			rowViews = append(rowViews, lipgloss.JoinHorizontal(lipgloss.Top, colViews...))
		}
	}

	grid := lipgloss.JoinVertical(lipgloss.Left, rowViews...)
	helpText := Styles.HelpText.Render(sessionDashboardHelpText)

	// Build page indicator if multiple pages exist
	var parts []string
	parts = append(parts, grid)
	if totalPages > 1 {
		pageIndicator := Styles.Muted.Render("Page " + strconv.Itoa(m.currentPage+1) + "/" + strconv.Itoa(totalPages))
		parts = append(parts, pageIndicator)
	}
	parts = append(parts, helpText)

	result := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Note: pane.cachedView, pane.dirty, and pane.lastFocused updates above persist
	// because slices share the underlying array between value receiver and caller.
	// gridDirty is cleared by clearGridDirty() called from Update() handlers.

	return result
}

// SetSize updates dashboard dimensions.
func (m *SessionDashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.recalcPaneSizes()
}

// recalcPaneSizes recalculates pane dimensions using the adaptive grid layout engine.
// Only recalculates for panes on the current page since those are the ones rendered.
// Each pane gets its own size from CalculateGridLayout, which distributes
// remainder pixels and handles non-uniform last rows.
func (m *SessionDashboardModel) recalcPaneSizes() {
	visibleCount := m.currentPagePaneCount()
	if visibleCount == 0 {
		return
	}
	pageStart := m.currentPage * MaxSessionPanes
	// Match View()'s reservedLines logic: 1 line for single page, 2 for multi-page
	reservedLines := 1
	if m.TotalPages() > 1 {
		reservedLines = 2
	}
	gridHeight := m.height - reservedLines
	if gridHeight < 3 {
		gridHeight = 3
	}
	layout := CalculateGridLayout(visibleCount, m.width, gridHeight)
	for _, pl := range layout.Panes {
		globalIdx := pageStart + pl.Index
		if globalIdx < len(m.panes) {
			m.panes[globalIdx].width = pl.Width
			m.panes[globalIdx].height = pl.Height
		}
	}
}

// handleFocusMove updates focusIndex for the given direction ("up","down","left","right"),
// marks the old and new focused panes dirty, and returns (model, nil).
func (m SessionDashboardModel) handleFocusMove(direction string) (SessionDashboardModel, tea.Cmd) {
	oldFocus := m.focusIndex
	m.focusIndex = m.moveFocus(direction)
	if oldFocus != m.focusIndex {
		pageStart := m.currentPage * MaxSessionPanes
		m.markPaneDirty(pageStart + oldFocus)
		m.markPaneDirty(pageStart + m.focusIndex)
	}
	return m, nil
}

// moveFocus calculates new focus index for given direction.
// Uses CalculateGridLayout to determine grid structure for navigation.
// focusIndex is page-local (0-based within the current page's visible panes).
func (m *SessionDashboardModel) moveFocus(direction string) int {
	visibleCount := m.currentPagePaneCount()
	if visibleCount <= 1 {
		return 0
	}

	gridHeight := m.height - 1
	if gridHeight < 3 {
		gridHeight = 3
	}
	layout := CalculateGridLayout(visibleCount, m.width, gridHeight)
	rows := layout.Rows
	cols := layout.Cols

	if rows == 0 || cols == 0 {
		return 0
	}

	row := m.focusIndex / cols
	col := m.focusIndex % cols

	switch direction {
	case "up":
		row = (row - 1 + rows) % rows
	case "down":
		row = (row + 1) % rows
	case "left":
		col = (col - 1 + cols) % cols
	case "right":
		col = (col + 1) % cols
	}

	newIdx := row*cols + col
	if newIdx >= visibleCount {
		newIdx = visibleCount - 1
	}
	return newIdx
}

// currentPagePaneCount returns the number of panes on the current page.
func (m *SessionDashboardModel) currentPagePaneCount() int {
	total := len(m.panes)
	if total == 0 {
		return 0
	}
	start := m.currentPage * MaxSessionPanes
	if start >= total {
		return 0
	}
	end := start + MaxSessionPanes
	if end > total {
		end = total
	}
	return end - start
}

// PaneCount returns the number of active panes.
func (m SessionDashboardModel) PaneCount() int {
	return len(m.panes)
}

// CurrentPage returns the current page index (zero-based). Exported for testing.
func (m SessionDashboardModel) CurrentPage() int {
	return m.currentPage
}

// SetCurrentPage sets the current page index (zero-based). Exported for testing.
func (m *SessionDashboardModel) SetCurrentPage(page int) {
	m.currentPage = page
	m.clampCurrentPage()
}

// FrameGovernor returns the frame-rate governor for metrics and testing.
func (m SessionDashboardModel) FrameGovernor() *FrameRateGovernor {
	return m.frameGovernor
}

// WaitForGoroutines waits for all spawned goroutines to complete.
// This is primarily used in tests for leak-free shutdown verification.
func (m SessionDashboardModel) WaitForGoroutines() {
	if m.wg != nil {
		m.wg.Wait()
	}
}

// SessionPaneModel rendering methods

// renderContent renders all entries for the session pane.
func (p *SessionPaneModel) renderContent() string {
	if len(p.entries) == 0 {
		return ""
	}

	contentWidth := p.width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	var lines []string
	for _, entry := range p.entries {
		rendered := renderPaneEntry(entry, contentWidth, p.mdRenderer)
		if rendered == "" {
			continue
		}
		entryLines := strings.Split(rendered, "\n")
		lines = append(lines, entryLines...)
	}

	contentHeight := p.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}

	content := strings.Join(lines, "\n")
	return truncateFromTop(content, contentHeight)
}

// ViewWithFocus renders the session pane with border.
func (p SessionPaneModel) ViewWithFocus(focused bool) string {
	if p.width < 4 || p.height < 3 {
		return ""
	}

	innerWidth := p.width - 2

	// Build header: PID + session indicator
	displayName := paneDisplayName(p.session)
	maxNameLen := innerWidth - 2
	if maxNameLen > 3 && VisualWidth(displayName) > maxNameLen {
		displayName = TruncateToWidth(displayName, maxNameLen-3) + "..."
	}

	// Use warning style for idle sessions
	headerStyle := PaneHeaderStyle
	if p.session.State == session.SessionIdle {
		headerStyle = PaneIdleHeaderStyle
	}

	header := headerStyle.
		Width(innerWidth).
		Render(displayName)

	innerHeight := p.height - 2
	contentHeight := innerHeight - 1
	if contentHeight < 0 {
		contentHeight = 0
	}

	var lines []string
	lines = append(lines, header)

	var contentLines []string
	if p.loading {
		contentLines = []string{Styles.Muted.Render("Loading...")}
	} else if p.errMsg != "" {
		contentLines = []string{Styles.Muted.Render("Error: " + p.errMsg)}
	} else if len(p.entries) == 0 {
		contentLines = []string{Styles.Muted.Render("Waiting for content...")}
	} else if p.content != "" {
		contentLines = strings.Split(p.content, "\n")
	}

	for i := 0; i < contentHeight; i++ {
		if i < len(contentLines) {
			line := contentLines[i]
			visualWidth := lipgloss.Width(line)
			if visualWidth > innerWidth {
				line = TruncateToWidth(line, innerWidth)
			}
			padding := innerWidth - lipgloss.Width(line)
			if padding > 0 {
				line = line + strings.Repeat(" ", padding)
			}
			lines = append(lines, line)
		} else {
			lines = append(lines, strings.Repeat(" ", innerWidth))
		}
	}
	innerContent := strings.Join(lines, "\n")

	// Idle sessions always use the warning border color regardless of focus.
	if p.session.State == session.SessionIdle {
		return addBorderWithStyle(innerContent, p.width, PaneIdleBorderColor)
	}
	if focused {
		return addBorderWithStyle(innerContent, p.width, PaneFocusedBorderColor)
	}
	return addBorderWithStyle(innerContent, p.width, PaneUnfocusedBorderColor)
}

// AllPanesHaveContent returns true if all panes have finished loading and have
// at least one log entry (i.e., JSONL content has been streamed into the pane).
// Returns false if there are no panes.
// Exported for testing.
func (m *SessionDashboardModel) AllPanesHaveContent() bool {
	if len(m.panes) == 0 {
		return false
	}
	for _, pane := range m.panes {
		if pane.loading || len(pane.entries) == 0 {
			return false
		}
	}
	return true
}

// PaneIsLoading returns whether the pane at index idx is in loading state.
// Exported for testing.
func (m *SessionDashboardModel) PaneIsLoading(idx int) bool {
	if idx >= 0 && idx < len(m.panes) {
		return m.panes[idx].loading
	}
	return false
}

// PaneEntriesCount returns the number of log entries in the pane at index idx.
// Returns -1 if the index is out of range.
// Exported for testing.
func (m *SessionDashboardModel) PaneEntriesCount(idx int) int {
	if idx >= 0 && idx < len(m.panes) {
		return len(m.panes[idx].entries)
	}
	return -1
}

// paneDisplayName returns a compact display name for a session pane.
// Idle sessions include a visual "⏸ IDLE" suffix to distinguish them from active sessions.
func paneDisplayName(sess session.ActiveSession) string {
	pid := sess.Meta.PID
	kind := sess.Meta.Kind
	if kind == "" {
		kind = "session"
	}
	name := strings.Join([]string{kind, "[", strconv.Itoa(pid), "]"}, "")
	if sess.State == session.SessionIdle {
		name += " ⏸ IDLE"
	}
	return name
}
