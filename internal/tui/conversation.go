// Package tui provides the terminal user interface components.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// ConversationItem implements ListItem for the conversation list.
type ConversationItem struct {
	conversation types.Conversation
}

// Render renders the conversation item for display.
func (i ConversationItem) Render(width int, selected bool) string {
	// Selection indicator and styling
	var prefixStyled string
	var titleStyle, descStyle lipgloss.Style

	if selected {
		prefixStyled = Styles.ListItem.GutterSelected.Render(GutterSelected)
		titleStyle = Styles.ListItem.TitleSelected
		descStyle = Styles.ListItem.DescSelected
	} else {
		prefixStyled = GutterNormal // No styling needed for normal gutter
		titleStyle = Styles.ListItem.TitleNormal
		descStyle = Styles.ListItem.DescNormal
	}

	// Calculate available width using shared helper
	availWidth := listItemAvailWidth(width)

	timestamp := formatTimestamp(i.conversation.LastModified)
	title := titleStyle.Render(timestamp)

	// Build description with message count, duration, and preview
	preview := i.conversation.FirstUserMessage
	if preview == "" {
		preview = "(no preview)"
	}

	// Format duration
	durationStr := formatDuration(i.conversation.Duration)
	if i.conversation.Duration == 0 {
		durationStr = "<1m"
	}

	countStr := fmt.Sprintf("%d msgs", i.conversation.MessageCount)
	metaPrefix := fmt.Sprintf("%s • %s • ", countStr, durationStr)

	// Calculate how much space left for preview after metadata prefix
	metaWidth := VisualWidth(metaPrefix)
	previewMaxWidth := availWidth - metaWidth
	if previewMaxWidth < 10 {
		previewMaxWidth = 10
	}
	preview = TruncateToWidth(preview, previewMaxWidth)

	// Pad description to fill width for consistent selection background
	descContent := metaPrefix + preview
	paddedDesc := PadToWidth(descContent, availWidth)
	desc := descStyle.Render(paddedDesc)

	// Description line also gets gutter alignment (normal gutter for visual alignment)
	return fmt.Sprintf("%s%s\n%s%s", prefixStyled, title, GutterNormal, desc)
}

// FilterValue returns the value used for filtering.
func (i ConversationItem) FilterValue() string {
	return i.conversation.FirstUserMessage
}

// ConversationModel is the Bubbletea model for the conversation browser.
type ConversationModel struct {
	listViewport  ListViewport[ConversationItem]
	conversations []types.Conversation
	projectName   string
	width         int
	height        int
	ready         bool // Set to true after first WindowSizeMsg

	// Lazy loading state
	lazyLoadState LoadingState
	loadedCount   int  // Number of conversations with metadata loaded
	lazyEnabled   bool // Whether lazy loading is enabled (>50 conversations)
}

// NewConversationModel creates a new conversation browser model.
func NewConversationModel(conversations []types.Conversation, projectName string) ConversationModel {
	return NewConversationModelWithLazyLoad(conversations, projectName, false, len(conversations))
}

// NewConversationModelWithLazyLoad creates a conversation browser with lazy loading support.
func NewConversationModelWithLazyLoad(conversations []types.Conversation, projectName string, lazyEnabled bool, loadedCount int) ConversationModel {
	items := make([]ConversationItem, len(conversations))
	for i, c := range conversations {
		items[i] = ConversationItem{conversation: c}
	}

	// Create viewport-based list with 2 lines per item
	listViewport := NewListViewport[ConversationItem](items, 2)

	state := LoadingStateComplete
	if lazyEnabled && loadedCount < len(conversations) {
		state = LoadingStateIdle
	}

	return ConversationModel{
		listViewport:  listViewport,
		conversations: conversations,
		projectName:   projectName,
		lazyEnabled:   lazyEnabled,
		loadedCount:   loadedCount,
		lazyLoadState: state,
	}
}

// Init implements tea.Model.
func (m ConversationModel) Init() tea.Cmd {
	return nil
}

// SetSize sets the list size and marks the model as ready.
func (m *ConversationModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Calculate actual list height: total - header - footer - border (2 lines for border top/bottom)
	listHeight := height - 4
	if listHeight < 4 {
		listHeight = 4
	}
	// Width for list is total width minus 4 (2 for outer margins, 2 for border chars)
	listWidth := width - 4
	if listWidth < 10 {
		listWidth = 10
	}
	m.listViewport.SetSize(listWidth, listHeight)
	m.ready = true
}

// ConversationSelectedMsg is sent when a conversation is selected.
type ConversationSelectedMsg struct {
	Conversation types.Conversation
}

// BackToProjectsFromConversationsMsg is sent when the user wants to go back to projects.
type BackToProjectsFromConversationsMsg struct{}

// conversationMetadataLoadedMsg is sent when a batch of conversation metadata is loaded.
type conversationMetadataLoadedMsg struct {
	loadedCount int
}

// Update implements tea.Model.
func (m ConversationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "enter", "l":
			if item, ok := m.listViewport.SelectedItem(); ok {
				return m, func() tea.Msg {
					return ConversationSelectedMsg{Conversation: item.conversation}
				}
			}

		case "h", "esc":
			return m, func() tea.Msg {
				return BackToProjectsFromConversationsMsg{}
			}
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case conversationMetadataLoadedMsg:
		// Update the loaded count and refresh the list items
		m.loadedCount = msg.loadedCount
		if m.loadedCount >= len(m.conversations) {
			m.lazyLoadState = LoadingStateComplete
		} else {
			m.lazyLoadState = LoadingStateIdle
		}
		// Refresh list items with updated metadata
		m.refreshListItems()
		return m, nil
	}

	// Delegate navigation to ListViewport
	var cmd tea.Cmd
	m.listViewport, cmd = m.listViewport.Update(msg)

	// Lazy loading: if cursor moved beyond loaded boundary, load more metadata
	if m.lazyEnabled && m.loadedCount < len(m.conversations) {
		currentIdx := m.listViewport.Cursor()
		// Load if cursor is within 5 items of the loaded boundary
		if currentIdx >= m.loadedCount-5 {
			// Load metadata up to cursor position + buffer
			targetLoad := currentIdx + 10
			if targetLoad > len(m.conversations) {
				targetLoad = len(m.conversations)
			}
			if targetLoad > m.loadedCount {
				// Load synchronously to avoid navigation issues
				scanner.ExtractConversationMetadataBatch(m.conversations, m.loadedCount, targetLoad-m.loadedCount)
				m.loadedCount = targetLoad
				m.refreshListItems()
			}
		}
	}

	return m, cmd
}

// refreshListItems refreshes the list items with current conversation data.
// Preserves the current selection index to avoid cursor jump issues.
func (m *ConversationModel) refreshListItems() {
	currentIndex := m.listViewport.Cursor()
	items := make([]ConversationItem, len(m.conversations))
	for i, c := range m.conversations {
		items[i] = ConversationItem{conversation: c}
	}
	m.listViewport.SetItems(items)
	m.listViewport.SetCursor(currentIndex)
}

// View implements tea.Model.
func (m ConversationModel) View() string {
	if len(m.conversations) == 0 {
		return m.renderEmpty()
	}

	if !m.ready {
		return "Loading..."
	}

	// Header with conversation count and loading indicator
	convCount := m.listViewport.ItemCount()
	headerText := fmt.Sprintf("Conversations: %s %s", m.projectName, ListStyles.Counter.Render(fmt.Sprintf("(%d)", convCount)))
	if m.lazyLoadState == LoadingStateLoading {
		headerText += " " + ListStyles.Loading.Render("loading...")
	} else if m.lazyEnabled && m.loadedCount < len(m.conversations) {
		headerText += " " + Styles.Muted.Render(fmt.Sprintf("[%d/%d loaded]", m.loadedCount, len(m.conversations)))
	}
	header := Styles.Title.Render(headerText)

	// Footer
	help := "j/k:nav • enter/l:open • h/esc:back • g/G:top/bottom • q:quit"
	footer := Styles.HelpText.Render(help)

	// Viewport already respects height strictly
	listView := m.listViewport.View()

	// Add manual border
	boxed := addBorder(listView, m.width-2)

	return fmt.Sprintf("%s\n%s\n%s", header, boxed, footer)
}

// renderEmpty renders the empty state when no conversations exist.
func (m ConversationModel) renderEmpty() string {
	title := Styles.Title.Render(fmt.Sprintf("Conversations: %s", m.projectName))
	msg := Styles.Muted.Render("No conversations found in this project")
	help := "Press 'h' or Escape to go back"

	return fmt.Sprintf("%s\n\n%s\n\n%s", title, msg, Styles.HelpText.Render(help))
}

// SelectedConversation returns the currently selected conversation.
func (m ConversationModel) SelectedConversation() (types.Conversation, bool) {
	if item, ok := m.listViewport.SelectedItem(); ok {
		return item.conversation, true
	}
	return types.Conversation{}, false
}
