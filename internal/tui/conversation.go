// Package tui provides the terminal user interface components.
package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// ConversationItem implements list.Item for the conversation list.
type ConversationItem struct {
	conversation types.Conversation
}

func (i ConversationItem) Title() string {
	return formatTimestamp(i.conversation.LastModified)
}

func (i ConversationItem) Description() string {
	preview := i.conversation.FirstUserMessage
	if preview == "" {
		preview = "(no preview)"
	}
	return fmt.Sprintf("%d msgs • %s", i.conversation.MessageCount, preview)
}

func (i ConversationItem) FilterValue() string {
	return i.conversation.FirstUserMessage
}

// ConversationItemDelegate is a custom delegate for rendering conversation items.
type ConversationItemDelegate struct{}

func (d ConversationItemDelegate) Height() int                             { return 2 }
func (d ConversationItemDelegate) Spacing() int                            { return 0 }
func (d ConversationItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d ConversationItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(ConversationItem)
	if !ok {
		return
	}

	var style lipgloss.Style
	if index == m.Index() {
		style = Styles.Selected
	} else {
		style = Styles.Normal
	}

	timestamp := formatTimestamp(i.conversation.LastModified)
	title := style.Render(timestamp)

	preview := i.conversation.FirstUserMessage
	if preview == "" {
		preview = "(no preview)"
	}
	countStr := fmt.Sprintf("%d msgs", i.conversation.MessageCount)
	desc := Styles.Muted.Render(fmt.Sprintf("%s • %s", countStr, preview))

	fmt.Fprintf(w, "  %s\n  %s\n", title, desc)
}

// ConversationModel is the Bubbletea model for the conversation browser.
type ConversationModel struct {
	list          list.Model
	conversations []types.Conversation
	projectName   string
	width         int
	height        int
	ready         bool // Set to true after first WindowSizeMsg
}

// NewConversationModel creates a new conversation browser model.
func NewConversationModel(conversations []types.Conversation, projectName string) ConversationModel {
	items := make([]list.Item, len(conversations))
	for i, c := range conversations {
		items[i] = ConversationItem{conversation: c}
	}

	delegate := ConversationItemDelegate{}
	// Use reasonable default size, will be resized on WindowSizeMsg
	l := list.New(items, delegate, 80, 20)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	return ConversationModel{
		list:          l,
		conversations: conversations,
		projectName:   projectName,
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
	// Reserve 2 lines: 1 for newline after list, 1 for help text
	m.list.SetSize(width, height-2)
	m.ready = true
}

// ConversationSelectedMsg is sent when a conversation is selected.
type ConversationSelectedMsg struct {
	Conversation types.Conversation
}

// BackToProjectsFromConversationsMsg is sent when the user wants to go back to projects.
type BackToProjectsFromConversationsMsg struct{}

// Update implements tea.Model.
func (m ConversationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		// j/k/down/up navigation handled by bubbles list component via m.list.Update(msg) below

		case "enter", "l":
			if item, ok := m.list.SelectedItem().(ConversationItem); ok {
				return m, func() tea.Msg {
					return ConversationSelectedMsg{Conversation: item.conversation}
				}
			}

		case "h", "esc":
			return m, func() tea.Msg {
				return BackToProjectsFromConversationsMsg{}
			}

		// g/G navigation handled by bubbles list component via m.list.Update(msg) below
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-2)
		m.ready = true
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m ConversationModel) View() string {
	if len(m.conversations) == 0 {
		return m.renderEmpty()
	}

	if !m.ready {
		return "Loading..."
	}

	// Header
	header := Styles.Title.Render(fmt.Sprintf("Conversations: %s", m.projectName))

	// Footer
	help := "j/k:nav • enter/l:open • h/esc:back • g/G:top/bottom • q:quit"
	footer := Styles.HelpText.Render(help)

	// Truncate list to exact height (total - header - footer = height-2)
	listHeight := m.height - 2
	listView := truncateConvLines(m.list.View(), listHeight)

	return fmt.Sprintf("%s\n%s\n%s", header, listView, footer)
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
	if item, ok := m.list.SelectedItem().(ConversationItem); ok {
		return item.conversation, true
	}
	return types.Conversation{}, false
}

// truncateConvLines truncates a string to at most n lines.
func truncateConvLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}
