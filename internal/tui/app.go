// Package tui provides the terminal user interface components.
package tui

import (
	"fmt"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	tea "github.com/charmbracelet/bubbletea"
)

// viewState represents the current view in the application.
type viewState int

const (
	viewProjects viewState = iota
	viewConversations
	viewViewer
)

// AppModel is the root Bubbletea model for the interactive mode.
type AppModel struct {
	state                viewState
	projectModel         ProjectModel
	conversationModel    ConversationModel
	viewerModel          ViewerModel
	selectedProject      types.Project
	selectedConversation types.Conversation
	width                int
	height               int
}

// NewAppModel creates a new application model with the project browser.
func NewAppModel(projects []types.Project) AppModel {
	return AppModel{
		state:        viewProjects,
		projectModel: NewProjectModel(projects),
	}
}

// NewAppModelWithError creates an app model showing an error.
func NewAppModelWithError(err error) AppModel {
	return AppModel{
		state:        viewProjects,
		projectModel: NewProjectModelWithError(err),
	}
}

// Init implements tea.Model.
func (m AppModel) Init() tea.Cmd {
	// Request window size to properly initialize the list dimensions
	return tea.Batch(m.projectModel.Init(), tea.WindowSize())
}

// Update implements tea.Model.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Forward to current view
		switch m.state {
		case viewProjects:
			var cmd tea.Cmd
			newModel, cmd := m.projectModel.Update(msg)
			m.projectModel = newModel.(ProjectModel)
			return m, cmd
		case viewConversations:
			var cmd tea.Cmd
			newModel, cmd := m.conversationModel.Update(msg)
			m.conversationModel = newModel.(ConversationModel)
			return m, cmd
		case viewViewer:
			var cmd tea.Cmd
			newModel, cmd := m.viewerModel.Update(msg)
			m.viewerModel = newModel.(ViewerModel)
			return m, cmd
		}

	case ProjectSelectedMsg:
		// User selected a project, load its conversations
		m.selectedProject = msg.Project
		return m, m.loadConversations()

	case conversationsLoadedMsg:
		// Conversations loaded, show the conversation list
		if msg.err != nil {
			// Handle error - go back to projects
			return m, nil
		}

		if len(msg.conversations) == 0 {
			// No conversations - show empty conversation list
			m.conversationModel = NewConversationModel(msg.conversations, m.selectedProject.DisplayName)
			m.conversationModel.SetSize(m.width, m.height)
			m.state = viewConversations
			return m, nil
		}

		// Show conversation list with lazy loading info
		m.conversationModel = NewConversationModelWithLazyLoad(
			msg.conversations,
			m.selectedProject.DisplayName,
			msg.lazyEnabled,
			msg.loadedCount,
		)
		m.conversationModel.SetSize(m.width, m.height)
		m.state = viewConversations
		return m, nil

	case ConversationSelectedMsg:
		// User selected a conversation, load it
		m.selectedConversation = msg.Conversation
		return m, m.loadConversation(msg.Conversation.FilePath)

	case conversationLoadedMsg:
		// Conversation loaded, switch to viewer
		if msg.err != nil {
			// Handle error - stay on conversation list
			return m, nil
		}
		// Build title: "project-name - 2026-01-12 09:00 (model)"
		title := fmt.Sprintf("%s - %s", m.selectedProject.DisplayName, formatTimestamp(m.selectedConversation.LastModified))
		if m.selectedConversation.Model != "" {
			// Shorten model name for display (e.g., "claude-3-5-sonnet-20241022" -> "sonnet-20241022")
			modelShort := m.selectedConversation.Model
			if VisualWidth(modelShort) > 20 {
				// Find last hyphen-separated segment that starts with the model version
				parts := []string{}
				for i := len(modelShort) - 1; i >= 0; i-- {
					if modelShort[i] == '-' {
						parts = append([]string{modelShort[i+1:]}, parts...)
						if len(parts) >= 2 {
							modelShort = parts[0] + "-" + parts[1]
							break
						}
					}
				}
			}
			title = fmt.Sprintf("%s (%s)", title, modelShort)
		}
		m.viewerModel = NewViewerModelWithBackNavigation(msg.entries, msg.parseErrors, title)
		m.viewerModel.SetSize(m.width, m.height)
		m.state = viewViewer
		return m, nil

	case BackToProjectsFromConversationsMsg:
		// User pressed escape in conversation list, go back to projects
		m.state = viewProjects
		return m, nil

	case GoBackMsg:
		// User pressed escape in viewer, go back to conversation list
		m.state = viewConversations
		return m, nil
	}

	// Route updates to current view
	switch m.state {
	case viewProjects:
		var cmd tea.Cmd
		newModel, cmd := m.projectModel.Update(msg)
		m.projectModel = newModel.(ProjectModel)
		return m, cmd

	case viewConversations:
		var cmd tea.Cmd
		newModel, cmd := m.conversationModel.Update(msg)
		m.conversationModel = newModel.(ConversationModel)
		return m, cmd

	case viewViewer:
		var cmd tea.Cmd
		newModel, cmd := m.viewerModel.Update(msg)
		m.viewerModel = newModel.(ViewerModel)
		return m, cmd
	}

	return m, nil
}

// View implements tea.Model.
func (m AppModel) View() string {
	switch m.state {
	case viewProjects:
		return m.projectModel.View()
	case viewConversations:
		return m.conversationModel.View()
	case viewViewer:
		return m.viewerModel.View()
	default:
		return m.projectModel.View()
	}
}

// Message types for async operations

type conversationsLoadedMsg struct {
	conversations []types.Conversation
	err           error
	lazyEnabled   bool
	loadedCount   int
}

type conversationLoadedMsg struct {
	entries     []types.LogEntry
	parseErrors int
	err         error
}

// loadConversations loads conversations for the selected project.
// Uses lazy loading for projects with >50 conversations.
func (m AppModel) loadConversations() tea.Cmd {
	return func() tea.Msg {
		config := DefaultLazyLoadConfig()

		// First, scan without metadata (fast)
		conversations, err := scanner.ScanConversationsLazy(m.selectedProject.DirPath)
		if err != nil {
			return conversationsLoadedMsg{err: err}
		}

		// If small number of conversations, load all metadata immediately
		if len(conversations) <= config.ConversationThreshold {
			for i := range conversations {
				scanner.ExtractConversationMetadataBatch(conversations, i, 1)
			}
			return conversationsLoadedMsg{
				conversations: conversations,
				lazyEnabled:   false,
				loadedCount:   len(conversations),
			}
		}

		// Large project: load first batch of metadata
		batchSize := config.BatchSize
		scanner.ExtractConversationMetadataBatch(conversations, 0, batchSize)

		return conversationsLoadedMsg{
			conversations: conversations,
			lazyEnabled:   true,
			loadedCount:   min(batchSize, len(conversations)),
		}
	}
}

// loadConversation loads a conversation file.
func (m AppModel) loadConversation(filePath string) tea.Cmd {
	return func() tea.Msg {
		result, err := parser.ParseJSONLFile(filePath)
		if err != nil {
			return conversationLoadedMsg{err: err}
		}
		return conversationLoadedMsg{
			entries:     result.Entries,
			parseErrors: result.ParseErrors,
		}
	}
}
