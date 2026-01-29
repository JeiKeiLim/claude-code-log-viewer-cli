// Package tui provides the terminal user interface components.
package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

// dashboardHelpText is the keyboard shortcut guide displayed at the bottom of the dashboard.
const dashboardHelpText = "arrows/hjkl:nav • Enter:open • r:refresh • R:all • Esc:back"

// GoBackToProjectsFromDashboardMsg signals return to project list from dashboard.
type GoBackToProjectsFromDashboardMsg struct{}

// DashboardModel represents the multi-project dashboard view.
type DashboardModel struct {
	panes               []PaneModel
	focusIndex          int
	width               int
	height              int
	ctx                 context.Context    // Story 9.2: Context for subscription goroutine cancellation
	cancel              context.CancelFunc // Story 9.2: Cancel function to stop subscription goroutines
	subscriptionsActive bool               // Story 9.2: Controls polling termination
}

// PaneModel represents a single pane in the dashboard grid.
type PaneModel struct {
	project      types.Project
	conversation types.Conversation // Latest conversation for this project (Story 5.3)
	entries      []types.LogEntry   // Parsed log entries (Story 5.3)
	content      string             // Pre-rendered view content (Story 5.3)
	parseErrors  int                // Parse error count (Story 5.3)
	watcher      *watcher.Watcher   // File watcher for live updates (Story 5.3)
	mdRenderer   *MarkdownRenderer  // Markdown renderer for content (Story 5.3)
	width        int
	height       int
	loading      bool   // Loading state indicator (Story 5.3)
	errMsg       string // Error message if content failed to load (Story 5.3)

	// Story 5.4: New conversation detection
	dirWatcher       *fsnotify.Watcher // Directory watcher for new conversations
	watchingDir      string            // Directory path being watched
	showNewIndicator bool              // Visual indicator for new conversation

	// Story 9.2: Subscription channels for event delivery
	fileEventChan chan paneWatcherEventMsg    // Channel for file watcher events
	dirEventChan  chan paneDirWatcherEventMsg // Channel for directory watcher events
}

// paneContentLoadedMsg signals content has been loaded for a specific pane.
type paneContentLoadedMsg struct {
	paneIndex    int
	entries      []types.LogEntry
	parseErrors  int
	filePath     string
	lastModified time.Time // Story 10.1: Timestamp from ScanConversationsLazy for rescan comparison
	err          error
}

// paneWatcherEventMsg wraps file content watcher events with pane index for routing.
// This is distinct from paneDirWatcherEventMsg which handles directory-level events.
type paneWatcherEventMsg struct {
	paneIndex int
	event     tea.Msg
}

// Story 5.4: Directory watcher message types

// paneDirWatcherInitMsg signals successful directory watcher initialization.
type paneDirWatcherInitMsg struct {
	paneIndex int
	watcher   *fsnotify.Watcher
	watchDir  string
}

// paneDirWatcherEventMsg signals a new file was created in the watched directory.
type paneDirWatcherEventMsg struct {
	paneIndex   int
	newFilePath string
}

// paneDirWatcherErrorMsg signals an error in the directory watcher.
type paneDirWatcherErrorMsg struct {
	paneIndex int
	err       error
}

// paneNewConversationMsg signals that a pane should switch to a new conversation.
type paneNewConversationMsg struct {
	paneIndex   int
	newFilePath string
}

// OpenViewerFromDashboardMsg signals request to open viewer from dashboard.
// Handled by AppModel to load conversation and transition to viewer.
type OpenViewerFromDashboardMsg struct {
	FilePath string        // Full path to conversation JSONL file
	Project  types.Project // Project for building viewer title
}

// subscriptionTickMsg is sent periodically to poll subscription channels.
// Story 9.2: Bubbletea has no native subscription support, so we poll channels.
type subscriptionTickMsg struct{}

// subscriptionChannelBuffer is the buffer size for subscription channels.
// Sufficient for ~10 events/sec; larger wastes memory.
const subscriptionChannelBuffer = 10

// paneIndicatorExpiredMsg signals that the new conversation indicator should be cleared.
type paneIndicatorExpiredMsg struct {
	paneIndex int
}

// paneRescanResultMsg signals result of re-scanning for latest conversation.
// Story 10.1: Used to catch conversations created during initialization race window.
type paneRescanResultMsg struct {
	paneIndex  int
	latestConv types.Conversation
	err        error
}

// rescanLatestCmd returns a command that re-scans for the latest conversation.
// Story 10.1: CRITICAL - projectPath must be passed as parameter (not read from m.panes
// inside closure) because the closure executes asynchronously and m.panes may have changed.
func rescanLatestCmd(paneIndex int, projectPath string) tea.Cmd {
	return func() tea.Msg {
		conversations, err := scanner.ScanConversationsLazy(projectPath)
		if err != nil {
			return paneRescanResultMsg{paneIndex: paneIndex, err: err}
		}
		if len(conversations) == 0 {
			return paneRescanResultMsg{paneIndex: paneIndex, err: nil} // Empty project
		}
		return paneRescanResultMsg{paneIndex: paneIndex, latestConv: conversations[0]}
	}
}

// initDirectoryWatcher returns a command that creates and stores the directory watcher.
// Called from paneContentLoadedMsg handler, NOT from constructor.
// Watches the project directory directly where .jsonl files are stored.
func (m *DashboardModel) initDirectoryWatcher(paneIndex int, projectPath string) tea.Cmd {
	return func() tea.Msg {
		// Watch project directory directly - Claude Code stores .jsonl files at root level
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return paneDirWatcherErrorMsg{paneIndex: paneIndex, err: err}
		}

		if err := w.Add(projectPath); err != nil {
			_ = w.Close()
			return paneDirWatcherErrorMsg{paneIndex: paneIndex, err: err}
		}

		// Return init success message with watcher reference
		return paneDirWatcherInitMsg{paneIndex: paneIndex, watcher: w, watchDir: projectPath}
	}
}

// findLatestConversation returns the most recent conversation for a project.
// Uses ScanConversationsLazy which returns conversations sorted by LastModified descending.
func findLatestConversation(projectPath string) (types.Conversation, error) {
	convs, err := scanner.ScanConversationsLazy(projectPath)
	if err != nil {
		return types.Conversation{}, err
	}
	if len(convs) == 0 {
		return types.Conversation{}, nil // Empty is valid - no conversations
	}
	// ScanConversationsLazy already sorts by LastModified descending
	return convs[0], nil
}

// loadPaneContentCmd returns a command that loads content for a specific pane.
func loadPaneContentCmd(paneIndex int, projectPath string) tea.Cmd {
	return func() tea.Msg {
		// Find latest conversation (includes LastModified from ScanConversationsLazy)
		conv, err := findLatestConversation(projectPath)
		if err != nil {
			return paneContentLoadedMsg{paneIndex: paneIndex, err: err}
		}
		// Handle empty project (no conversations)
		if conv.FilePath == "" {
			return paneContentLoadedMsg{paneIndex: paneIndex, entries: nil, filePath: ""}
		}
		// Parse the conversation file
		result, err := parser.ParseJSONLFile(conv.FilePath)
		if err != nil {
			return paneContentLoadedMsg{paneIndex: paneIndex, err: err}
		}
		// Story 10.1: Use LastModified from ScanConversationsLazy (already statted during scan)
		// This avoids redundant os.Stat call and ensures consistency with rescan comparisons
		return paneContentLoadedMsg{
			paneIndex:    paneIndex,
			entries:      result.Entries,
			parseErrors:  result.ParseErrors,
			filePath:     conv.FilePath,
			lastModified: conv.LastModified,
		}
	}
}

// truncateFromTop removes excess lines from the beginning of content.
// Returns the last maxLines lines of content (tail behavior).
func truncateFromTop(content string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	// Return last maxLines lines
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

// PaneToolIcon is the icon used for tool entries in dashboard panes.
// Uses [T] as specified in AC 5.7.2 (distinct from styles.ToolIcon which is [>]).
const PaneToolIcon = "[T]"

// formatPaneToolSummary returns a compact summary for tool use in panes.
// Format: "target" (e.g., "/path/to/file.go" for Read, "make build" for Bash).
func formatPaneToolSummary(toolName string, input map[string]any) string {
	switch toolName {
	case "Read", "Write", "Edit":
		if filePath, ok := input["file_path"].(string); ok && filePath != "" {
			return filePath
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok && cmd != "" {
			// Take first line of command
			firstLine := strings.Split(cmd, "\n")[0]
			return firstLine
		}
	case "Glob":
		if pattern, ok := input["pattern"].(string); ok && pattern != "" {
			return pattern
		}
	case "Grep":
		if pattern, ok := input["pattern"].(string); ok && pattern != "" {
			return pattern
		}
	case "Task":
		if desc, ok := input["description"].(string); ok && desc != "" {
			return desc
		}
	case "WebFetch":
		if url, ok := input["url"].(string); ok && url != "" {
			return url
		}
	case "WebSearch":
		if query, ok := input["query"].(string); ok && query != "" {
			return query
		}
	}
	return ""
}

// renderPaneEntry renders a single log entry for compact pane display.
// Uses minimal chrome - no line numbers, collapsed blocks only.
func renderPaneEntry(entry types.LogEntry, width int, mdRenderer *MarkdownRenderer) string {
	switch entry.Type {
	case types.EntryTypeUser:
		// [U] <first line of message, wrapped>
		text := entry.Message.TextContent
		if text == "" {
			return ""
		}
		// Take first line only for compact display
		firstLine := strings.Split(text, "\n")[0]
		// Use visual width for proper multi-byte character handling (CJK, emoji)
		maxWidth := width - 4 // Account for icon + space
		if maxWidth < 5 {
			maxWidth = 5
		}
		if VisualWidth(firstLine) > maxWidth {
			firstLine = TruncateToWidth(firstLine, maxWidth)
		}
		return UserIcon + " " + firstLine

	case types.EntryTypeAssistant:
		// Check for tool use first (AC 5.7.2: tool short info)
		for _, c := range entry.Message.Content {
			if c.Type == types.ContentTypeToolUse {
				// Format: [T] ToolName: target/summary
				summary := formatPaneToolSummary(c.ToolName, c.ToolInput)
				maxWidth := width - 4 // Account for icon + space
				if maxWidth < 10 {
					maxWidth = 10
				}
				// Build display string
				display := c.ToolName
				if summary != "" {
					display = c.ToolName + ": " + summary
				}
				// Truncate if too long - use filepath.Base for paths
				if VisualWidth(display) > maxWidth {
					// For file paths, show just the filename
					if strings.Contains(summary, "/") {
						shortSummary := filepath.Base(summary)
						display = c.ToolName + ": " + shortSummary
					}
					// Still too long? Truncate
					if VisualWidth(display) > maxWidth {
						display = TruncateToWidth(display, maxWidth)
					}
				}
				return PaneToolIcon + " " + display
			}
		}

		// [A] <rendered markdown, first few lines>
		text := extractPaneTextContent(entry)
		if text == "" {
			// Check for thinking only
			for _, c := range entry.Message.Content {
				if c.Type == types.ContentTypeThinking {
					return AssistantIcon + " [thinking...]"
				}
			}
			return ""
		}
		// Render markdown if renderer available
		var rendered string
		if mdRenderer != nil {
			rendered = mdRenderer.Render(text)
		} else {
			rendered = text
		}
		// Take first few lines only
		lines := strings.Split(rendered, "\n")
		maxLines := 3
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, "...")
		}
		return AssistantIcon + " " + strings.Join(lines, "\n    ")
	}
	return ""
}

// extractPaneTextContent extracts text content from assistant message for pane display.
func extractPaneTextContent(entry types.LogEntry) string {
	for _, c := range entry.Message.Content {
		if c.Type == types.ContentTypeText && c.Text != "" {
			return c.Text
		}
	}
	return ""
}

// renderPaneContent renders all entries and truncates to fit pane height.
func (p *PaneModel) renderPaneContent() string {
	if len(p.entries) == 0 {
		return ""
	}

	// Calculate available width for content (inside border)
	contentWidth := p.width - 4 // 2 for border, 2 for padding
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

	// Calculate available height for content
	// total height - border (2) - header (1) = contentHeight
	contentHeight := p.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Apply tail truncation (show latest content)
	content := strings.Join(lines, "\n")
	return truncateFromTop(content, contentHeight)
}

// NewDashboardModel creates a new dashboard model with the given projects.
// Returns the model and a batch command to load content for all panes.
func NewDashboardModel(projects []types.Project) (DashboardModel, tea.Cmd) {
	panes := make([]PaneModel, len(projects))
	cmds := make([]tea.Cmd, len(projects))

	for i, p := range projects {
		panes[i] = PaneModel{
			project: p,
			loading: true, // Start in loading state
		}
		// Create command to load content for this pane
		cmds[i] = loadPaneContentCmd(i, p.DirPath)
	}

	// Story 9.2: Initialize context for subscription goroutine management
	ctx, cancel := context.WithCancel(context.Background())

	model := DashboardModel{
		panes:               panes,
		focusIndex:          0,
		ctx:                 ctx,
		cancel:              cancel,
		subscriptionsActive: true, // Story 9.2: Enable polling on init
	}

	if len(cmds) > 0 {
		return model, tea.Batch(cmds...)
	}
	return model, nil
}

// Init implements tea.Model.
func (m DashboardModel) Init() tea.Cmd {
	// Content loading is initiated from NewDashboardModel
	// Story 9.2: Start subscription polling if active
	if m.subscriptionsActive {
		return m.subscriptionTickCmd()
	}
	return nil
}

// Update implements tea.Model.
func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		// Re-render content after resize with updated markdown renderer
		for i := range m.panes {
			if len(m.panes[i].entries) > 0 {
				// Recreate markdown renderer with new width for proper word wrap
				renderWidth := m.panes[i].width - 6
				if renderWidth < 20 {
					renderWidth = 20
				}
				m.panes[i].mdRenderer, _ = NewMarkdownRenderer(renderWidth)
				m.panes[i].content = m.panes[i].renderPaneContent()
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			// Close all watchers before exiting
			m.closeAllWatchers()
			return m, func() tea.Msg { return GoBackToProjectsFromDashboardMsg{} }
		case "up", "k":
			m.focusIndex = m.moveFocus("up")
			return m, nil
		case "down", "j":
			m.focusIndex = m.moveFocus("down")
			return m, nil
		case "left", "h":
			m.focusIndex = m.moveFocus("left")
			return m, nil
		case "right", "l":
			m.focusIndex = m.moveFocus("right")
			return m, nil
		case "enter":
			if m.focusIndex >= 0 && m.focusIndex < len(m.panes) {
				pane := m.panes[m.focusIndex]
				if pane.conversation.FilePath == "" {
					return m, nil // No conversation to open
				}
				return m, func() tea.Msg {
					return OpenViewerFromDashboardMsg{
						FilePath: pane.conversation.FilePath,
						Project:  pane.project,
					}
				}
			}
			return m, nil
		case "r":
			// Story 10.2: Refresh focused pane - set state in Update(), not in command
			if m.focusIndex >= 0 && m.focusIndex < len(m.panes) {
				m.panes[m.focusIndex].loading = true
				m.panes[m.focusIndex].showNewIndicator = true
				return m, tea.Batch(
					loadPaneContentCmd(m.focusIndex, m.panes[m.focusIndex].project.DirPath),
					paneIndicatorTimeoutCmd(m.focusIndex, 1*time.Second),
				)
			}
			return m, nil
		case "R":
			// Story 10.2: Refresh all panes
			if len(m.panes) == 0 {
				return m, nil
			}
			cmds := make([]tea.Cmd, 0, len(m.panes)*2)
			for i := range m.panes {
				m.panes[i].loading = true
				m.panes[i].showNewIndicator = true
				cmds = append(cmds, loadPaneContentCmd(i, m.panes[i].project.DirPath))
				cmds = append(cmds, paneIndicatorTimeoutCmd(i, 1*time.Second))
			}
			return m, tea.Batch(cmds...)
		}

	case paneContentLoadedMsg:
		// Handle content loaded for a specific pane
		if msg.paneIndex >= 0 && msg.paneIndex < len(m.panes) {
			pane := &m.panes[msg.paneIndex]
			pane.loading = false

			if msg.err != nil {
				pane.errMsg = msg.err.Error()
				// Still start directory watcher even on error (to detect first conversation)
				return m, m.initDirectoryWatcher(msg.paneIndex, pane.project.DirPath)
			}

			// Store entries and render content
			pane.entries = msg.entries
			pane.parseErrors = msg.parseErrors

			// Initialize markdown renderer if needed
			if pane.mdRenderer == nil && pane.width > 0 {
				renderWidth := pane.width - 6
				if renderWidth < 20 {
					renderWidth = 20
				}
				pane.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
			}

			// Render content
			pane.content = pane.renderPaneContent()

			// Collect commands to batch
			var cmds []tea.Cmd

			// Start file watcher if we have a file path
			if msg.filePath != "" {
				// Story 10.2 fix: Close existing file watcher before creating new one
				// This prevents FD leaks and stale subscription goroutines
				if pane.watcher != nil {
					_ = pane.watcher.Close()
					pane.watcher = nil
				}
				pane.fileEventChan = nil // Signal old subscription goroutine to exit

				w, err := watcher.New(msg.filePath)
				if err == nil {
					pane.watcher = w
					// Story 10.1: Preserve LastModified for race condition fix
					pane.conversation = types.Conversation{
						FilePath:     msg.filePath,
						LastModified: msg.lastModified,
					}
					// Story 9.2: Start subscription goroutine instead of chained commands
					m.startFileWatcherSubscription(m.ctx, msg.paneIndex)
				}
				// Watcher creation failed - continue without live updates
			}

			// Story 10.2 fix: Close existing directory watcher before creating new one
			if pane.dirWatcher != nil {
				for _, path := range pane.dirWatcher.WatchList() {
					_ = pane.dirWatcher.Remove(path)
				}
				_ = pane.dirWatcher.Close()
				pane.dirWatcher = nil
			}
			pane.dirEventChan = nil // Signal old subscription goroutine to exit

			// Start directory watcher (Story 5.4)
			cmds = append(cmds, m.initDirectoryWatcher(msg.paneIndex, pane.project.DirPath))

			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		}
		return m, nil

	case paneDirWatcherInitMsg:
		// Handle directory watcher initialization success
		if msg.paneIndex >= 0 && msg.paneIndex < len(m.panes) {
			pane := &m.panes[msg.paneIndex]
			pane.dirWatcher = msg.watcher
			pane.watchingDir = msg.watchDir
			// Story 9.2: Start subscription goroutine instead of chained commands
			m.startDirWatcherSubscription(m.ctx, msg.paneIndex)

			// Story 10.1: Re-scan for latest conversation after watcher is ready
			// This catches conversations created during the initialization race window
			// CRITICAL: Pass projectPath explicitly to avoid stale closure capture
			return m, rescanLatestCmd(msg.paneIndex, pane.project.DirPath)
		}
		return m, nil

	case paneRescanResultMsg:
		// Story 10.1: Handle re-scan result after watcher initialization
		// Catches conversations created during the initialization race window
		if msg.err != nil || msg.paneIndex < 0 || msg.paneIndex >= len(m.panes) {
			return m, nil
		}
		pane := &m.panes[msg.paneIndex]

		// Empty rescan result (no conversations in project) - no action needed
		// This handles the case where pane has a conversation but project is now empty
		// (zero LastModified is never After any time, so this is implicitly handled,
		// but explicit check clarifies intent)
		if msg.latestConv.FilePath == "" {
			return m, nil
		}

		// Compare: only switch if different and newer
		if msg.latestConv.FilePath != pane.conversation.FilePath {
			// Different file - check if actually newer
			if msg.latestConv.LastModified.After(pane.conversation.LastModified) {
				return m, func() tea.Msg {
					return paneNewConversationMsg{
						paneIndex:   msg.paneIndex,
						newFilePath: msg.latestConv.FilePath,
					}
				}
			}
		}
		// Same file or not newer - no action (prevents flicker per AC-4)
		return m, nil

	case paneDirWatcherEventMsg:
		// Handle new file creation in watched directory
		// Story 9.2: No longer chains another watcher wait - subscription goroutine handles all events
		if msg.paneIndex >= 0 && msg.paneIndex < len(m.panes) {
			pane := &m.panes[msg.paneIndex]

			// Check if this is actually a newer file than current conversation
			newInfo, err := os.Stat(msg.newFilePath)
			if err != nil {
				// File disappeared - subscription continues watching
				return m, nil
			}

			// If we have a current conversation, compare timestamps
			if pane.conversation.FilePath != "" {
				currInfo, err := os.Stat(pane.conversation.FilePath)
				if err == nil && !newInfo.ModTime().After(currInfo.ModTime()) {
					// New file is not newer - subscription continues watching
					return m, nil
				}
			}

			// New file is newer - switch to it
			return m, func() tea.Msg {
				return paneNewConversationMsg(msg)
			}
		}
		return m, nil

	case paneNewConversationMsg:
		// Handle switching to a new conversation
		if msg.paneIndex >= 0 && msg.paneIndex < len(m.panes) {
			pane := &m.panes[msg.paneIndex]

			// Close existing file watcher (subscription goroutine will exit on channel close)
			if pane.watcher != nil {
				_ = pane.watcher.Close()
				pane.watcher = nil
			}
			pane.fileEventChan = nil // Clear channel reference

			// Reset pane state
			pane.entries = nil
			pane.content = ""
			pane.loading = true
			pane.errMsg = ""

			// Set visual indicator (cleared via paneIndicatorTimeoutCmd)
			pane.showNewIndicator = true

			// Update conversation path
			pane.conversation = types.Conversation{FilePath: msg.newFilePath}

			// Story 9.2: No need to restart dir watcher - subscription goroutine continues
			return m, tea.Batch(
				loadPaneContentCmd(msg.paneIndex, pane.project.DirPath),
				paneIndicatorTimeoutCmd(msg.paneIndex, 2*time.Second),
			)
		}
		return m, nil

	case paneIndicatorExpiredMsg:
		// Clear the new conversation indicator
		if msg.paneIndex >= 0 && msg.paneIndex < len(m.panes) {
			m.panes[msg.paneIndex].showNewIndicator = false
		}
		return m, nil

	case subscriptionTickMsg:
		// Story 9.2: Poll subscription channels for events
		if !m.subscriptionsActive {
			return m, nil // Stop polling when inactive
		}

		// Poll all subscription channels (non-blocking)
		if polledMsg := m.pollSubscriptionChannels(); polledMsg != nil {
			// Process the event
			newModel, eventCmd := m.Update(polledMsg)
			updatedModel := newModel.(DashboardModel)
			// CRITICAL: Must continue polling after processing event (H1 fix)
			// Without this, the polling chain breaks after the first event
			if updatedModel.subscriptionsActive {
				return updatedModel, tea.Batch(eventCmd, updatedModel.subscriptionTickCmd())
			}
			return updatedModel, eventCmd
		}

		// No events - schedule next tick to continue polling
		return m, m.subscriptionTickCmd()

	case paneWatcherEventMsg:
		// Route watcher event to correct pane
		// Story 9.2: No longer chains another watcher wait - subscription goroutine handles all events
		if msg.paneIndex >= 0 && msg.paneIndex < len(m.panes) {
			pane := &m.panes[msg.paneIndex]

			switch event := msg.event.(type) {
			case watcher.NewEntriesMsg:
				// Append new entries
				pane.entries = append(pane.entries, event.Entries...)
				pane.content = pane.renderPaneContent()
				return m, nil // Story 9.2: Subscription goroutine continues watching

			case watcher.FileResetMsg:
				// File was truncated - reload full content
				return m, loadPaneContentCmd(msg.paneIndex, pane.project.DirPath)

			case watcher.WatcherErrorMsg:
				// Continue - subscription goroutine handles retries
				return m, nil
			}
		}
		return m, nil
	}
	return m, nil
}

// startFileWatcherSubscription starts a long-lived goroutine that listens to file watcher events.
// Story 9.2: Replaces waitForPaneWatcher() to prevent goroutine accumulation.
// CRITICAL: Channel must be assigned BEFORE goroutine starts to prevent nil-send race.
func (m *DashboardModel) startFileWatcherSubscription(ctx context.Context, paneIndex int) {
	if paneIndex < 0 || paneIndex >= len(m.panes) {
		return
	}
	pane := &m.panes[paneIndex]
	if pane.watcher == nil {
		return
	}

	// Create buffered channel BEFORE starting goroutine (prevents nil-send race)
	ch := make(chan paneWatcherEventMsg, subscriptionChannelBuffer)
	pane.fileEventChan = ch

	go func() {
		defer close(ch)
		events := pane.watcher.EventsChan()
		errors := pane.watcher.ErrorsChan()

		for {
			select {
			case <-ctx.Done():
				return // Clean exit on context cancel
			case event, ok := <-events:
				if !ok {
					return // Watcher closed
				}
				if event.Has(fsnotify.Write) {
					entries, err := pane.watcher.ReadNewEntries()
					var msg paneWatcherEventMsg
					if err == watcher.ErrFileTruncated {
						msg = paneWatcherEventMsg{paneIndex: paneIndex, event: watcher.FileResetMsg{}}
					} else if err != nil {
						msg = paneWatcherEventMsg{paneIndex: paneIndex, event: watcher.WatcherErrorMsg{Err: err}}
					} else if len(entries) > 0 {
						msg = paneWatcherEventMsg{paneIndex: paneIndex, event: watcher.NewEntriesMsg{Entries: entries}}
					} else {
						continue // No new entries, skip sending
					}
					// Non-blocking send with context check
					select {
					case ch <- msg:
					case <-ctx.Done():
						return
					}
				}
			case err, ok := <-errors:
				if !ok {
					return // Watcher closed
				}
				msg := paneWatcherEventMsg{paneIndex: paneIndex, event: watcher.WatcherErrorMsg{Err: err}}
				select {
				case ch <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
}

// startDirWatcherSubscription starts a long-lived goroutine that listens to directory watcher events.
// Story 9.2: Replaces waitForDirEvent() to prevent goroutine accumulation.
// CRITICAL: Channel must be assigned BEFORE goroutine starts to prevent nil-send race.
func (m *DashboardModel) startDirWatcherSubscription(ctx context.Context, paneIndex int) {
	if paneIndex < 0 || paneIndex >= len(m.panes) {
		return
	}
	pane := &m.panes[paneIndex]
	if pane.dirWatcher == nil {
		return
	}

	// Create buffered channel BEFORE starting goroutine (prevents nil-send race)
	ch := make(chan paneDirWatcherEventMsg, subscriptionChannelBuffer)
	pane.dirEventChan = ch

	go func() {
		defer close(ch)
		w := pane.dirWatcher

		for {
			select {
			case <-ctx.Done():
				return // Clean exit on context cancel
			case event, ok := <-w.Events:
				if !ok {
					return // Watcher closed
				}
				if event.Op&fsnotify.Create != 0 {
					if strings.HasSuffix(event.Name, ".jsonl") {
						// Verify file still exists (Task 10.3)
						if _, err := os.Stat(event.Name); err == nil {
							msg := paneDirWatcherEventMsg{paneIndex: paneIndex, newFilePath: event.Name}
							select {
							case ch <- msg:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			case _, ok := <-w.Errors:
				if !ok {
					return // Watcher closed
				}
				// Continue on errors - graceful degradation
			}
		}
	}()
}

// paneIndicatorTimeoutCmd returns a command that fires after the given duration.
func paneIndicatorTimeoutCmd(paneIndex int, duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return paneIndicatorExpiredMsg{paneIndex: paneIndex}
	})
}

// subscriptionTickCmd returns a command that schedules the next polling tick.
// Story 9.2: Polls every 100ms for events from subscription channels.
func (m *DashboardModel) subscriptionTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return subscriptionTickMsg{}
	})
}

// pollSubscriptionChannels checks all subscription channels for events (non-blocking).
// Story 9.2: Returns the first event found, or nil if no events pending.
func (m *DashboardModel) pollSubscriptionChannels() tea.Msg {
	for i := range m.panes {
		// Check file event channel
		if ch := m.panes[i].fileEventChan; ch != nil {
			select {
			case msg, ok := <-ch:
				if ok {
					return msg
				}
			default:
				// No event pending
			}
		}
		// Check directory event channel
		if ch := m.panes[i].dirEventChan; ch != nil {
			select {
			case msg, ok := <-ch:
				if ok {
					return msg
				}
			default:
				// No event pending
			}
		}
	}
	return nil
}

// closeAllWatchers closes all pane watchers (file and directory) to prevent resource leaks.
// CRITICAL: On macOS, fsnotify uses kqueue which opens a file descriptor for EACH
// watched path. We must call Remove() on each watched path before Close() to properly
// release all file descriptors.
// Story 9.2: Updated for context-aware shutdown sequence.
func (m *DashboardModel) closeAllWatchers() {
	// 1. Stop polling chain
	m.subscriptionsActive = false

	// 2. Signal subscription goroutines to exit
	if m.cancel != nil {
		m.cancel()
	}

	// 3. Close watchers and nil out channels
	// Goroutines will detect ctx.Done() and exit, closing their channels via defer
	for i := range m.panes {
		// Close file content watcher (uses watcher.Watcher which handles cleanup internally)
		if m.panes[i].watcher != nil {
			_ = m.panes[i].watcher.Close()
			m.panes[i].watcher = nil
		}
		// Close directory watcher (raw fsnotify.Watcher - needs explicit Remove)
		if m.panes[i].dirWatcher != nil {
			for _, path := range m.panes[i].dirWatcher.WatchList() {
				_ = m.panes[i].dirWatcher.Remove(path) // Ignore errors - path may be deleted
			}
			_ = m.panes[i].dirWatcher.Close()
			m.panes[i].dirWatcher = nil
		}
		// 4. Nil out channel references (channels closed by goroutines via defer)
		m.panes[i].fileEventChan = nil
		m.panes[i].dirEventChan = nil
	}
}

// ResumeWatchers restarts subscription goroutines and polling for active watchers.
// Story 9.2: Creates new context (old one is cancelled) and restarts subscriptions.
// Called when returning to dashboard from viewer to resume file/directory watching.
func (m *DashboardModel) ResumeWatchers() tea.Cmd {
	// 1. Create new context for resumed watchers (old context is cancelled)
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel

	// 2. Enable subscriptions
	m.subscriptionsActive = true

	// 3. Restart subscription goroutines for all panes with active watchers
	for i := range m.panes {
		// Restart file watcher subscription if watcher is active
		if m.panes[i].watcher != nil {
			m.startFileWatcherSubscription(ctx, i)
		}
		// Restart directory watcher subscription if watcher is active
		if m.panes[i].dirWatcher != nil {
			m.startDirWatcherSubscription(ctx, i)
		}
	}

	// 4. Return initial polling tick command
	return m.subscriptionTickCmd()
}

// moveFocus calculates new focus index for given direction.
// Handles wrap-around and clamping for incomplete grids.
func (m *DashboardModel) moveFocus(direction string) int {
	if len(m.panes) <= 1 {
		return 0 // Single pane - no movement
	}

	rows, cols := calculateGrid(len(m.panes))
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
	// Clamp to valid pane range (handles incomplete last row)
	if newIdx >= len(m.panes) {
		newIdx = len(m.panes) - 1
	}
	return newIdx
}

// View implements tea.Model.
func (m DashboardModel) View() string {
	// Handle edge case: no panes
	if len(m.panes) == 0 {
		return ""
	}

	rows, cols := calculateGrid(len(m.panes))
	if rows == 0 || cols == 0 {
		return ""
	}

	// Reserve 1 line for help text at bottom
	gridHeight := m.height - 1
	if gridHeight < 3 {
		gridHeight = 3
	}
	paneWidth, paneHeight := calculatePaneDimensions(m.width, gridHeight, rows, cols)

	// Build rows
	var rowViews []string
	for r := 0; r < rows; r++ {
		var colViews []string
		for c := 0; c < cols; c++ {
			idx := r*cols + c
			if idx < len(m.panes) {
				// Render pane with calculated dimensions
				pane := m.panes[idx]
				pane.width = paneWidth
				pane.height = paneHeight
				focused := idx == m.focusIndex
				colViews = append(colViews, pane.ViewWithFocus(focused))
			} else {
				// Empty cell for incomplete last row - render blank space matching pane dimensions
				colViews = append(colViews, lipgloss.NewStyle().
					Width(paneWidth).
					Height(paneHeight).
					Render(""))
			}
		}
		rowViews = append(rowViews, lipgloss.JoinHorizontal(lipgloss.Top, colViews...))
	}

	// Render grid
	grid := lipgloss.JoinVertical(lipgloss.Left, rowViews...)

	// Render help text (dimmed, centered)
	helpText := Styles.HelpText.Render(dashboardHelpText)

	return lipgloss.JoinVertical(lipgloss.Left, grid, helpText)
}

// SetSize updates the dashboard dimensions and recalculates pane sizes.
func (m *DashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	if len(m.panes) == 0 {
		return
	}

	rows, cols := calculateGrid(len(m.panes))
	paneWidth, paneHeight := calculatePaneDimensions(width, height, rows, cols)

	for i := range m.panes {
		m.panes[i].width = paneWidth
		m.panes[i].height = paneHeight
	}
}

// calculateGrid returns the grid dimensions (rows, cols) for a given project count.
// Grid mapping per PRD FR-502:
//   - 0: 0x0 (edge case)
//   - 1: 1x1
//   - 2: 1x2
//   - 3: 1x3
//   - 4: 2x2
//   - 5-6: 2x3
//   - 7+: 3x3 (max 9 per Story 5.1)
func calculateGrid(count int) (rows, cols int) {
	switch count {
	case 0:
		return 0, 0
	case 1:
		return 1, 1
	case 2:
		return 1, 2
	case 3:
		return 1, 3
	case 4:
		return 2, 2
	case 5, 6:
		return 2, 3
	default:
		return 3, 3 // 7+ projects (max 9 per Story 5.1)
	}
}

// calculatePaneDimensions returns the dimensions for each pane given total space and grid size.
func calculatePaneDimensions(totalWidth, totalHeight, rows, cols int) (paneWidth, paneHeight int) {
	// Guard against division by zero
	if rows == 0 || cols == 0 {
		return 0, 0
	}

	// Simple division - borders are handled in PaneModel.View()
	paneWidth = totalWidth / cols
	paneHeight = totalHeight / rows
	return paneWidth, paneHeight
}

// View renders a single pane with border and project name header (unfocused).
// Uses manual border drawing (addBorder) instead of lipgloss.Height() which is unreliable.
// See docs/lessons-learned.md for details.
// Delegates to ViewWithFocus(false) for consistent unfocused styling.
func (p PaneModel) View() string {
	return p.ViewWithFocus(false)
}

// ViewWithFocus renders pane with border color based on focus state.
func (p PaneModel) ViewWithFocus(focused bool) string {
	// Guard against invalid dimensions
	if p.width < 4 || p.height < 3 {
		return ""
	}

	// Inner content width (account for left+right border = 2 chars)
	innerWidth := p.width - 2

	// Truncate project name if too long (leave room for padding and [NEW] badge)
	displayName := p.project.DisplayName
	badgeLen := 0
	if p.showNewIndicator {
		badgeLen = 6 // " [NEW]" length
	}
	maxNameLen := innerWidth - 2 - badgeLen // padding + badge
	if maxNameLen > 3 && VisualWidth(displayName) > maxNameLen {
		displayName = TruncateToWidth(displayName, maxNameLen-3) + "..."
	}

	// Append [NEW] badge if indicator is active (Story 5.4)
	if p.showNewIndicator {
		badge := lipgloss.NewStyle().
			Foreground(DefaultTheme.Accent).
			Bold(true).
			Render(" [NEW]")
		displayName = displayName + badge
	}

	// Header with project name (centered)
	header := PaneHeaderStyle.
		Width(innerWidth).
		Render(displayName)

	// Inner height: total - top border (1) - bottom border (1) = total - 2
	// Content lines: inner height - header (1)
	innerHeight := p.height - 2
	contentHeight := innerHeight - 1
	if contentHeight < 0 {
		contentHeight = 0
	}

	// Build inner content: header + content lines
	var lines []string
	lines = append(lines, header)

	// Determine content to display
	var contentLines []string
	if p.loading {
		// Loading state
		contentLines = []string{Styles.Muted.Render("Loading...")}
	} else if p.errMsg != "" {
		// Error state
		contentLines = []string{Styles.Muted.Render("Error: " + p.errMsg)}
	} else if len(p.entries) == 0 {
		// Empty state - no conversations
		contentLines = []string{Styles.Muted.Render("No conversations")}
	} else if p.content != "" {
		// Display pre-rendered content
		contentLines = strings.Split(p.content, "\n")
	}

	// Add content lines up to contentHeight
	for i := 0; i < contentHeight; i++ {
		if i < len(contentLines) {
			line := contentLines[i]
			// Ensure line fits within innerWidth
			visualWidth := lipgloss.Width(line)
			if visualWidth > innerWidth {
				// Truncate line to fit (simple approach)
				line = TruncateToWidth(line, innerWidth)
			}
			// Pad line to innerWidth
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

	// Use focused or unfocused border color
	if focused {
		return addBorderWithStyle(innerContent, p.width, PaneFocusedBorderColor)
	}
	return addBorderWithStyle(innerContent, p.width, PaneUnfocusedBorderColor)
}
