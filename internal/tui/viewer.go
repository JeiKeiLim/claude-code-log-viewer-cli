// Package tui provides the terminal user interface components.
package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// ViewerModel is the Bubbletea model for viewing log entries.
type ViewerModel struct {
	entries     []types.LogEntry
	viewport    viewport.Model
	ready       bool
	parseErrors int
	width       int
	height      int
	title       string // Display title based on source context

	// Toggle states (collapsed by default per spec)
	showThinking   bool
	showToolInputs bool

	// For returning to previous view
	canGoBack bool

	// Search state
	searching     bool
	searchInput   textinput.Model
	searchQuery   string
	searchMatches []int // Line numbers with matches
	currentMatch  int   // Index into searchMatches
	noResults     bool

	// gg key detection
	lastKeyG     bool
	lastKeyGTime time.Time
}

// NewViewerModel creates a new viewer model with the given entries.
func NewViewerModel(entries []types.LogEntry, parseErrors int, title string) ViewerModel {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100

	return ViewerModel{
		entries:        entries,
		parseErrors:    parseErrors,
		title:          title,
		showThinking:   false, // Collapsed by default
		showToolInputs: false, // Collapsed by default
		canGoBack:      false,
		searchInput:    ti,
	}
}

// NewViewerModelWithBack creates a new viewer that can return to a previous view.
func NewViewerModelWithBack(entries []types.LogEntry, parseErrors int, title string) ViewerModel {
	m := NewViewerModel(entries, parseErrors, title)
	m.canGoBack = true
	return m
}

// NewViewerModelWithBackNavigation is an alias for NewViewerModelWithBack.
func NewViewerModelWithBackNavigation(entries []types.LogEntry, parseErrors int, title string) ViewerModel {
	return NewViewerModelWithBack(entries, parseErrors, title)
}

// SetSize sets the viewport size.
func (m *ViewerModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Header: 1 line for title
	// Footer: 1 line for help/status
	headerHeight := 1
	footerHeight := 1
	verticalMargins := headerHeight + footerHeight

	if !m.ready {
		m.viewport = viewport.New(width, height-verticalMargins)
		m.viewport.YPosition = headerHeight
		m.ready = true
		m.updateContent()
	} else {
		m.viewport.Width = width
		m.viewport.Height = height - verticalMargins
		m.updateContent()
	}
}

// Init implements tea.Model.
func (m ViewerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m ViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle search mode
		if m.searching {
			switch msg.String() {
			case "enter":
				m.searching = false
				m.searchQuery = m.searchInput.Value()
				m.performSearch()
				return m, nil
			case "esc":
				m.searching = false
				m.searchInput.SetValue("")
				m.searchQuery = ""
				m.searchMatches = nil
				m.noResults = false
				m.updateContent()
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}

		keyStr := msg.String()

		// Check for gg sequence
		if keyStr == "g" {
			if m.lastKeyG && time.Since(m.lastKeyGTime) < 500*time.Millisecond {
				// gg detected - go to top
				m.viewport.GotoTop()
				m.lastKeyG = false
				return m, nil
			}
			m.lastKeyG = true
			m.lastKeyGTime = time.Now()
			return m, nil
		}
		m.lastKeyG = false

		switch keyStr {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "j", "down":
			m.viewport.LineDown(1)

		case "k", "up":
			m.viewport.LineUp(1)

		case "d", "ctrl+d":
			m.viewport.HalfViewDown()

		case "u", "ctrl+u":
			m.viewport.HalfViewUp()

		case "pgdown", " ":
			m.viewport.ViewDown()

		case "pgup":
			m.viewport.ViewUp()

		case "home":
			m.viewport.GotoTop()

		case "end":
			m.viewport.GotoBottom()

		case "G":
			// Go to bottom
			m.viewport.GotoBottom()

		case "h", "esc":
			if m.canGoBack {
				// Signal to go back - handled by parent
				return m, func() tea.Msg { return GoBackMsg{} }
			}

		case "/":
			// Open search
			m.searching = true
			m.searchInput.Focus()
			return m, textinput.Blink

		case "n":
			// Next match
			m.nextMatch()

		case "N":
			// Previous match
			m.prevMatch()

		case "t":
			m.showThinking = !m.showThinking
			m.updateContent()

		case "i":
			m.showToolInputs = !m.showToolInputs
			m.updateContent()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Header: 1 line for title
		// Footer: 1 line for help/status
		headerHeight := 1
		footerHeight := 1
		verticalMargins := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargins)
			m.viewport.YPosition = headerHeight
			m.ready = true
			m.updateContent()
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargins
			m.updateContent()
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// GoBackMsg signals the parent to go back to the previous view.
type GoBackMsg struct{}

// View implements tea.Model.
func (m ViewerModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Header with contextual title
	headerText := m.title
	if headerText == "" {
		headerText = "Claude Code Log Viewer"
	}
	header := Styles.Title.Render(headerText)

	// Search bar (if searching)
	if m.searching {
		searchBar := Styles.SearchInput.Render("/" + m.searchInput.View())
		return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), searchBar)
	}

	// Footer with help and status
	var footerParts []string
	footerParts = append(footerParts, "j/k:scroll", "gg/G:top/bottom", "/:search")
	if len(m.searchMatches) > 0 {
		footerParts = append(footerParts, "n/N:next/prev")
	}
	if m.canGoBack {
		footerParts = append(footerParts, "h/esc:back")
	}
	footerParts = append(footerParts, "t:thinking", "i:inputs", "q:quit")

	helpText := Styles.HelpText.Render(strings.Join(footerParts, " • "))

	// Status with search info
	var statusText string
	if m.noResults && m.searchQuery != "" {
		statusText = fmt.Sprintf("No results for '%s'", m.searchQuery)
	} else if len(m.searchMatches) > 0 {
		statusText = fmt.Sprintf("Match %d/%d for '%s'", m.currentMatch+1, len(m.searchMatches), m.searchQuery)
	} else {
		statusText = fmt.Sprintf("%d entries", len(m.entries))
		if m.parseErrors > 0 {
			statusText += fmt.Sprintf(" (%d lines skipped)", m.parseErrors)
		}
	}
	status := Styles.Muted.Render(statusText)

	footer := lipgloss.JoinHorizontal(
		lipgloss.Left,
		helpText,
		strings.Repeat(" ", max(0, m.width-lipgloss.Width(helpText)-lipgloss.Width(status))),
		status,
	)

	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

// updateContent renders all entries and updates the viewport content.
func (m *ViewerModel) updateContent() {
	var content strings.Builder

	for _, entry := range m.entries {
		rendered := m.renderEntry(entry)
		content.WriteString(rendered)
		content.WriteString("\n")
	}

	m.viewport.SetContent(content.String())
}

// renderEntry renders a single log entry.
func (m *ViewerModel) renderEntry(entry types.LogEntry) string {
	switch entry.Type {
	case types.EntryTypeUser:
		return m.renderUserMessage(entry)
	case types.EntryTypeAssistant:
		return m.renderAssistantMessage(entry)
	default:
		return ""
	}
}

// renderUserMessage renders a user message entry.
func (m *ViewerModel) renderUserMessage(entry types.LogEntry) string {
	timestamp := formatTimestamp(entry.Timestamp)
	header := fmt.Sprintf("%s %s  %s",
		UserIcon,
		Styles.UserHeader.Render("User"),
		Styles.Timestamp.Render(timestamp),
	)

	content := Styles.MessageContent.Render(entry.Message.TextContent)

	return Styles.UserMessage.Render(header + "\n" + content)
}

// renderAssistantMessage renders an assistant message entry.
func (m *ViewerModel) renderAssistantMessage(entry types.LogEntry) string {
	timestamp := formatTimestamp(entry.Timestamp)
	header := fmt.Sprintf("%s %s  %s",
		AssistantIcon,
		Styles.AssistantHeader.Render("Assistant"),
		Styles.Timestamp.Render(timestamp),
	)

	var parts []string
	parts = append(parts, header)

	for _, content := range entry.Message.Content {
		switch content.Type {
		case types.ContentTypeText:
			if content.Text != "" {
				parts = append(parts, Styles.MessageContent.Render(content.Text))
			}

		case types.ContentTypeThinking:
			parts = append(parts, m.renderThinkingBlock(content))

		case types.ContentTypeToolUse:
			parts = append(parts, m.renderToolUseBlock(content))
		}
	}

	return Styles.AssistantMessage.Render(strings.Join(parts, "\n"))
}

// renderThinkingBlock renders a thinking content block.
func (m *ViewerModel) renderThinkingBlock(content types.MessageContent) string {
	if !m.showThinking {
		return Styles.CollapsedIndicator.Render(
			fmt.Sprintf("%s [thinking - press 't' to expand]", ThinkingIcon),
		)
	}

	header := fmt.Sprintf("%s %s", ThinkingIcon, Styles.ThinkingHeader.Render("Thinking"))
	return Styles.ThinkingBlock.Render(header + "\n" + content.Thinking)
}

// renderToolUseBlock renders a tool use content block.
func (m *ViewerModel) renderToolUseBlock(content types.MessageContent) string {
	header := fmt.Sprintf("%s %s: %s",
		ToolIcon,
		Styles.ToolHeader.Render("Tool"),
		content.ToolName,
	)

	if !m.showToolInputs {
		return Styles.ToolBlock.Render(
			header + " " + Styles.CollapsedIndicator.Render("[inputs - press 'i' to expand]"),
		)
	}

	// Render tool inputs with truncation
	inputStr := formatToolInput(content.ToolInput)
	if len(inputStr) > 200 {
		inputStr = inputStr[:200] + fmt.Sprintf("... (%d chars total)", len(inputStr))
	}

	return Styles.ToolBlock.Render(header + "\n" + inputStr)
}

// formatToolInput formats tool input as a readable string.
func formatToolInput(input map[string]any) string {
	if len(input) == 0 {
		return "(no input)"
	}

	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "(error formatting input)"
	}

	return string(data)
}

// performSearch searches through the content and finds matching lines.
func (m *ViewerModel) performSearch() {
	if m.searchQuery == "" {
		m.searchMatches = nil
		m.noResults = false
		m.updateContent()
		return
	}

	query := strings.ToLower(m.searchQuery)
	content := m.viewport.View()
	lines := strings.Split(content, "\n")

	m.searchMatches = nil
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}

	if len(m.searchMatches) == 0 {
		m.noResults = true
		m.currentMatch = 0
	} else {
		m.noResults = false
		m.currentMatch = 0
		// Jump to first match
		m.viewport.SetYOffset(m.searchMatches[0])
	}

	m.updateContent()
}

// nextMatch jumps to the next search match.
func (m *ViewerModel) nextMatch() {
	if len(m.searchMatches) == 0 {
		return
	}

	m.currentMatch = (m.currentMatch + 1) % len(m.searchMatches)
	m.viewport.SetYOffset(m.searchMatches[m.currentMatch])
}

// prevMatch jumps to the previous search match.
func (m *ViewerModel) prevMatch() {
	if len(m.searchMatches) == 0 {
		return
	}

	m.currentMatch--
	if m.currentMatch < 0 {
		m.currentMatch = len(m.searchMatches) - 1
	}
	m.viewport.SetYOffset(m.searchMatches[m.currentMatch])
}
