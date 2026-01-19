// Package tui provides the terminal user interface components.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

// GoBackToProjectsFromDashboardMsg signals return to project list from dashboard.
type GoBackToProjectsFromDashboardMsg struct{}

// DashboardModel represents the multi-project dashboard view.
type DashboardModel struct {
	panes      []PaneModel
	focusIndex int
	width      int
	height     int
}

// PaneModel represents a single pane in the dashboard grid.
type PaneModel struct {
	project      types.Project
	conversation types.Conversation  // Latest conversation for this project (Story 5.3)
	entries      []types.LogEntry    // Parsed log entries (Story 5.3)
	content      string              // Pre-rendered view content (Story 5.3)
	parseErrors  int                 // Parse error count (Story 5.3)
	watcher      *watcher.Watcher    // File watcher for live updates (Story 5.3)
	mdRenderer   *MarkdownRenderer   // Markdown renderer for content (Story 5.3)
	width        int
	height       int
	loading      bool   // Loading state indicator (Story 5.3)
	errMsg       string // Error message if content failed to load (Story 5.3)
}

// paneContentLoadedMsg signals content has been loaded for a specific pane.
type paneContentLoadedMsg struct {
	paneIndex   int
	entries     []types.LogEntry
	parseErrors int
	filePath    string
	err         error
}

// paneWatcherEventMsg wraps watcher events with pane index for routing.
type paneWatcherEventMsg struct {
	paneIndex int
	event     tea.Msg
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
		// Find latest conversation
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
		return paneContentLoadedMsg{
			paneIndex:   paneIndex,
			entries:     result.Entries,
			parseErrors: result.ParseErrors,
			filePath:    conv.FilePath,
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
		// [A] <rendered markdown, first few lines>
		text := extractPaneTextContent(entry)
		if text == "" {
			// Check for tool use or thinking only
			for _, c := range entry.Message.Content {
				if c.Type == types.ContentTypeToolUse {
					return AssistantIcon + " [tool: " + c.ToolName + "]"
				}
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

	model := DashboardModel{
		panes:      panes,
		focusIndex: 0,
	}

	if len(cmds) > 0 {
		return model, tea.Batch(cmds...)
	}
	return model, nil
}

// Init implements tea.Model.
func (m DashboardModel) Init() tea.Cmd {
	// Content loading is initiated from NewDashboardModel
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
		}

	case paneContentLoadedMsg:
		// Handle content loaded for a specific pane
		if msg.paneIndex >= 0 && msg.paneIndex < len(m.panes) {
			pane := &m.panes[msg.paneIndex]
			pane.loading = false

			if msg.err != nil {
				pane.errMsg = msg.err.Error()
				return m, nil
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

			// Start file watcher if we have a file path
			if msg.filePath != "" {
				w, err := watcher.New(msg.filePath)
				if err == nil {
					pane.watcher = w
					pane.conversation = types.Conversation{FilePath: msg.filePath}
					return m, m.waitForPaneWatcher(msg.paneIndex)
				}
				// Watcher creation failed - continue without live updates
			}
		}
		return m, nil

	case paneWatcherEventMsg:
		// Route watcher event to correct pane
		if msg.paneIndex >= 0 && msg.paneIndex < len(m.panes) {
			pane := &m.panes[msg.paneIndex]

			switch event := msg.event.(type) {
			case watcher.NewEntriesMsg:
				// Append new entries
				pane.entries = append(pane.entries, event.Entries...)
				pane.content = pane.renderPaneContent()
				// Chain next watcher wait
				return m, m.waitForPaneWatcher(msg.paneIndex)

			case watcher.FileResetMsg:
				// File was truncated - reload full content
				return m, loadPaneContentCmd(msg.paneIndex, pane.project.DirPath)

			case watcher.WatcherErrorMsg:
				// Continue watching on error (graceful degradation)
				return m, m.waitForPaneWatcher(msg.paneIndex)
			}
		}
		return m, nil
	}
	return m, nil
}

// waitForPaneWatcher returns a command that waits for the next watcher event.
// Wraps the event with pane index for routing.
func (m *DashboardModel) waitForPaneWatcher(paneIndex int) tea.Cmd {
	if paneIndex < 0 || paneIndex >= len(m.panes) {
		return nil
	}
	w := m.panes[paneIndex].watcher
	if w == nil {
		return nil
	}
	return func() tea.Msg {
		cmd := w.WaitForEvent()
		event := cmd() // Execute the blocking wait
		if event == nil {
			return nil // Watcher closed
		}
		return paneWatcherEventMsg{paneIndex: paneIndex, event: event}
	}
}

// closeAllWatchers closes all pane watchers to prevent resource leaks.
func (m *DashboardModel) closeAllWatchers() {
	for i := range m.panes {
		if m.panes[i].watcher != nil {
			_ = m.panes[i].watcher.Close()
			m.panes[i].watcher = nil
		}
	}
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

	paneWidth, paneHeight := calculatePaneDimensions(m.width, m.height, rows, cols)

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
				colViews = append(colViews, pane.View())
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

	return lipgloss.JoinVertical(lipgloss.Left, rowViews...)
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

// View renders a single pane with border and project name header.
// Uses manual border drawing (addBorder) instead of lipgloss.Height() which is unreliable.
// See docs/lessons-learned.md for details.
func (p PaneModel) View() string {
	// Guard against invalid dimensions
	if p.width < 4 || p.height < 3 {
		return ""
	}

	// Inner content width (account for left+right border = 2 chars)
	innerWidth := p.width - 2

	// Truncate project name if too long (leave room for padding)
	displayName := p.project.DisplayName
	maxNameLen := innerWidth - 2 // padding
	if maxNameLen > 3 && len(displayName) > maxNameLen {
		displayName = displayName[:maxNameLen-3] + "..."
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

	// Use manual border drawing for reliable height control
	return addBorder(innerContent, p.width)
}
