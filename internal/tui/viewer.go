// Package tui provides the terminal user interface components.
package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
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

	// Lazy loading state
	lazyLoadState LoadingState
	loadedCount   int  // Number of entries rendered
	lazyEnabled   bool // Whether lazy loading is enabled (>100 entries)

	// Overlay spinner for bulk loading operations
	overlaySpinner     spinner.Model
	showOverlaySpinner bool

	// Watch mode (for future Story 2.1)
	watchMode bool
}

// NewViewerModel creates a new viewer model with the given entries.
func NewViewerModel(entries []types.LogEntry, parseErrors int, title string) ViewerModel {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100

	config := DefaultLazyLoadConfig()
	lazyEnabled := len(entries) > config.MessageThreshold

	// For lazy loading, initially render only the first batch
	loadedCount := len(entries)
	state := LoadingStateComplete
	if lazyEnabled {
		loadedCount = min(config.BatchSize*2, len(entries)) // Load 2 batches initially (40 entries)
		if loadedCount < len(entries) {
			state = LoadingStateIdle
		}
	}

	// Initialize spinner with Dot style and Loading style
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ListStyles.Loading

	return ViewerModel{
		entries:        entries,
		parseErrors:    parseErrors,
		title:          title,
		showThinking:   false, // Collapsed by default
		showToolInputs: false, // Collapsed by default
		canGoBack:      false,
		searchInput:    ti,
		lazyEnabled:    lazyEnabled,
		loadedCount:    loadedCount,
		lazyLoadState:  state,
		overlaySpinner: s,
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
	case spinner.TickMsg:
		var spinnerCmd tea.Cmd
		m.overlaySpinner, spinnerCmd = m.overlaySpinner.Update(msg)
		// Only return tick command if overlay spinner is shown
		if m.showOverlaySpinner {
			return m, spinnerCmd
		}
		return m, nil

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
			m.viewport.ScrollDown(1)

		case "k", "up":
			m.viewport.ScrollUp(1)

		case "d", "ctrl+d":
			m.viewport.HalfPageDown()

		case "u", "ctrl+u":
			m.viewport.HalfPageUp()

		case "pgdown", " ":
			m.viewport.PageDown()

		case "pgup":
			m.viewport.PageUp()

		case "home":
			m.viewport.GotoTop()

		case "end":
			m.viewport.GotoBottom()

		case "G":
			// Go to bottom with async loading if lazy loading is enabled
			if m.lazyEnabled && m.loadedCount < len(m.entries) {
				m.showOverlaySpinner = true
				return m, tea.Batch(m.overlaySpinner.Tick, m.markAllMessagesLoadedCmd())
			}
			// If all messages loaded, just go to bottom
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

	case viewerMessagesLoadedMsg:
		// Update loaded count
		m.loadedCount = msg.loadedCount
		wasOverlayShown := m.showOverlaySpinner
		m.showOverlaySpinner = false // Clear overlay spinner
		if m.loadedCount >= len(m.entries) {
			m.lazyLoadState = LoadingStateComplete
		} else {
			m.lazyLoadState = LoadingStateIdle
		}
		// Use pre-rendered content if available (from bulk loading)
		if msg.renderedContent != "" {
			m.viewport.SetContent(msg.renderedContent)
		} else {
			m.updateContent()
		}
		// If overlay was shown (bulk load via 'G'), go to bottom after loading
		if wasOverlayShown {
			m.viewport.GotoBottom()
		}
		return m, nil
	}

	m.viewport, cmd = m.viewport.Update(msg)

	// Check if we need to load more messages (when scrolling near bottom)
	if m.lazyEnabled && m.lazyLoadState == LoadingStateIdle {
		// Check if we're near the bottom of the currently rendered content
		scrollPercent := m.viewport.ScrollPercent()
		if scrollPercent > 0.8 && m.loadedCount < len(m.entries) {
			m.lazyLoadState = LoadingStateLoading
			return m, m.loadMoreMessages()
		}
	}

	return m, cmd
}

// loadMoreMessages loads the next batch of messages.
func (m *ViewerModel) loadMoreMessages() tea.Cmd {
	config := DefaultLazyLoadConfig()
	newLoadedCount := m.loadedCount + config.BatchSize
	if newLoadedCount > len(m.entries) {
		newLoadedCount = len(m.entries)
	}

	return func() tea.Msg {
		return viewerMessagesLoadedMsg{
			loadedCount: newLoadedCount,
		}
	}
}

// markAllMessagesLoadedCmd returns a command that pre-renders all messages.
// The expensive rendering happens in the command (goroutine) so the spinner can animate.
func (m *ViewerModel) markAllMessagesLoadedCmd() tea.Cmd {
	// Capture values needed for rendering
	entries := m.entries
	total := len(entries)
	width := m.width
	showThinking := m.showThinking
	showToolInputs := m.showToolInputs

	return func() tea.Msg {
		// Pre-render all content in the goroutine (expensive operation)
		var content strings.Builder
		for i := 0; i < total; i++ {
			rendered := renderEntryStatic(entries[i], width, showThinking, showToolInputs)
			content.WriteString(rendered)
			content.WriteString("\n")
		}
		return viewerMessagesLoadedMsg{
			loadedCount:     total,
			renderedContent: content.String(),
		}
	}
}

// GoBackMsg signals the parent to go back to the previous view.
type GoBackMsg struct{}

// viewerMessagesLoadedMsg is sent when more messages are rendered.
type viewerMessagesLoadedMsg struct {
	loadedCount     int
	renderedContent string // Pre-rendered content for bulk loading (optional)
}

// buildModeSegment returns the mode indicator segment (for watch mode).
func (m ViewerModel) buildModeSegment() string {
	if !m.watchMode {
		return "" // Empty when not in watch mode
	}
	return Styles.StatusBarSegment.Mode.Render("LIVE")
}

// buildPositionSegment returns the position indicator segment.
func (m ViewerModel) buildPositionSegment() string {
	total := len(m.entries)
	if total == 0 {
		return Styles.StatusBarSegment.Position.Render("0/0")
	}
	// Approximate position from scroll percentage
	// scrollPct=0.0 → pos 1, scrollPct=1.0 → pos total
	scrollPct := m.viewport.ScrollPercent()
	pos := int(float64(total-1)*scrollPct) + 1
	return Styles.StatusBarSegment.Position.Render(fmt.Sprintf("Entry %d/%d", pos, total))
}

// buildShortcutsSegment returns the keyboard shortcuts segment.
func (m ViewerModel) buildShortcutsSegment() string {
	var parts []string
	parts = append(parts, "j/k:scroll", "gg/G:top/bottom", "/:search")
	if len(m.searchMatches) > 0 {
		parts = append(parts, "n/N:next/prev")
	}
	if m.canGoBack {
		parts = append(parts, "h/esc:back")
	}
	parts = append(parts, "t:thinking", "i:inputs", "q:quit")
	return strings.Join(parts, " • ")
}

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

	// Build segmented footer
	modeSegment := m.buildModeSegment()
	posSegment := m.buildPositionSegment()
	shortcutsText := m.buildShortcutsSegment()

	// Add search/status info to shortcuts text if applicable
	var statusSuffix string
	if m.noResults && m.searchQuery != "" {
		statusSuffix = fmt.Sprintf(" | No results for '%s'", m.searchQuery)
	} else if len(m.searchMatches) > 0 {
		statusSuffix = fmt.Sprintf(" | Match %d/%d for '%s'", m.currentMatch+1, len(m.searchMatches), m.searchQuery)
	} else if m.lazyEnabled && m.loadedCount < len(m.entries) {
		statusSuffix = fmt.Sprintf(" | %d/%d loaded", m.loadedCount, len(m.entries))
	}
	if m.parseErrors > 0 {
		statusSuffix += fmt.Sprintf(" (%d skipped)", m.parseErrors)
	}

	// Calculate width for shortcuts segment (fills remaining space)
	modeWidth := lipgloss.Width(modeSegment)
	posWidth := lipgloss.Width(posSegment)
	shortcutsWidth := m.width - modeWidth - posWidth
	if shortcutsWidth < 0 {
		shortcutsWidth = 0
	}

	shortcutsSegment := Styles.StatusBarSegment.Shortcuts.
		Width(shortcutsWidth).
		Render(shortcutsText + statusSuffix)

	footer := lipgloss.JoinHorizontal(lipgloss.Top, modeSegment, posSegment, shortcutsSegment)

	normalView := fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)

	// If overlay spinner is shown, overlay it on top of the normal view
	if m.showOverlaySpinner {
		return overlaySpinnerView(normalView, m.overlaySpinner.View(), "Loading...", m.width, m.height)
	}

	return normalView
}

// updateContent renders entries and updates the viewport content.
// With lazy loading, only renders up to loadedCount entries.
func (m *ViewerModel) updateContent() {
	var content strings.Builder

	// Render only loadedCount entries for lazy loading
	renderCount := m.loadedCount
	if renderCount > len(m.entries) {
		renderCount = len(m.entries)
	}

	for i := 0; i < renderCount; i++ {
		rendered := m.renderEntry(m.entries[i])
		content.WriteString(rendered)
		content.WriteString("\n")
	}

	// Add loading indicator at the bottom if more content is available
	if m.lazyEnabled && renderCount < len(m.entries) {
		if m.lazyLoadState == LoadingStateLoading {
			content.WriteString(ListStyles.Loading.Render("Loading more messages..."))
		} else {
			content.WriteString(Styles.Muted.Render(fmt.Sprintf("-- %d more entries (scroll down to load) --", len(m.entries)-renderCount)))
		}
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

	// Wrap content to fit viewport width (with margin for styling)
	wrapWidth := m.width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrappedText := WrapText(entry.Message.TextContent, wrapWidth)
	content := Styles.MessageContent.Render(wrappedText)

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

	// Calculate wrap width for content
	wrapWidth := m.width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	var parts []string
	parts = append(parts, header)

	for _, content := range entry.Message.Content {
		switch content.Type {
		case types.ContentTypeText:
			if content.Text != "" {
				wrappedText := WrapText(content.Text, wrapWidth)
				parts = append(parts, Styles.MessageContent.Render(wrappedText))
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

	// Wrap thinking content to viewport width
	wrapWidth := m.width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrappedThinking := WrapText(content.Thinking, wrapWidth)

	header := fmt.Sprintf("%s %s", ThinkingIcon, Styles.ThinkingHeader.Render("Thinking"))
	return Styles.ThinkingBlock.Render(header + "\n" + wrappedThinking)
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

	// Calculate wrap width for tool input content
	wrapWidth := m.width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	// Render tool inputs with wrapping
	inputStr := formatToolInput(content.ToolInput)
	wrappedInput := WrapText(inputStr, wrapWidth)

	return Styles.ToolBlock.Render(header + "\n" + wrappedInput)
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

// renderEntryStatic renders a single log entry without model state (for async rendering).
func renderEntryStatic(entry types.LogEntry, width int, showThinking, showToolInputs bool) string {
	switch entry.Type {
	case types.EntryTypeUser:
		return renderUserMessageStatic(entry, width)
	case types.EntryTypeAssistant:
		return renderAssistantMessageStatic(entry, width, showThinking, showToolInputs)
	default:
		return ""
	}
}

// renderUserMessageStatic renders a user message entry without model state.
func renderUserMessageStatic(entry types.LogEntry, width int) string {
	timestamp := formatTimestamp(entry.Timestamp)
	header := fmt.Sprintf("%s %s  %s",
		UserIcon,
		Styles.UserHeader.Render("User"),
		Styles.Timestamp.Render(timestamp),
	)

	wrapWidth := width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrappedText := WrapText(entry.Message.TextContent, wrapWidth)
	content := Styles.MessageContent.Render(wrappedText)

	return Styles.UserMessage.Render(header + "\n" + content)
}

// renderAssistantMessageStatic renders an assistant message entry without model state.
func renderAssistantMessageStatic(entry types.LogEntry, width int, showThinking, showToolInputs bool) string {
	timestamp := formatTimestamp(entry.Timestamp)
	header := fmt.Sprintf("%s %s  %s",
		AssistantIcon,
		Styles.AssistantHeader.Render("Assistant"),
		Styles.Timestamp.Render(timestamp),
	)

	wrapWidth := width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	var parts []string
	parts = append(parts, header)

	for _, content := range entry.Message.Content {
		switch content.Type {
		case types.ContentTypeText:
			if content.Text != "" {
				wrappedText := WrapText(content.Text, wrapWidth)
				parts = append(parts, Styles.MessageContent.Render(wrappedText))
			}

		case types.ContentTypeThinking:
			parts = append(parts, renderThinkingBlockStatic(content, width, showThinking))

		case types.ContentTypeToolUse:
			parts = append(parts, renderToolUseBlockStatic(content, width, showToolInputs))
		}
	}

	return Styles.AssistantMessage.Render(strings.Join(parts, "\n"))
}

// renderThinkingBlockStatic renders a thinking content block without model state.
func renderThinkingBlockStatic(content types.MessageContent, width int, showThinking bool) string {
	if !showThinking {
		return Styles.CollapsedIndicator.Render(
			fmt.Sprintf("%s [thinking - press 't' to expand]", ThinkingIcon),
		)
	}

	wrapWidth := width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrappedThinking := WrapText(content.Thinking, wrapWidth)

	header := fmt.Sprintf("%s %s", ThinkingIcon, Styles.ThinkingHeader.Render("Thinking"))
	return Styles.ThinkingBlock.Render(header + "\n" + wrappedThinking)
}

// renderToolUseBlockStatic renders a tool use content block without model state.
func renderToolUseBlockStatic(content types.MessageContent, width int, showToolInputs bool) string {
	header := fmt.Sprintf("%s %s: %s",
		ToolIcon,
		Styles.ToolHeader.Render("Tool"),
		content.ToolName,
	)

	if !showToolInputs {
		return Styles.ToolBlock.Render(
			header + " " + Styles.CollapsedIndicator.Render("[inputs - press 'i' to expand]"),
		)
	}

	wrapWidth := width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	inputStr := formatToolInput(content.ToolInput)
	wrappedInput := WrapText(inputStr, wrapWidth)

	return Styles.ToolBlock.Render(header + "\n" + wrappedInput)
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
