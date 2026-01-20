// Package tui provides the terminal user interface components.
package tui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/token"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/usage"
)

// viewState represents the current view in the application.
type viewState int

const (
	viewProjects viewState = iota
	viewConversations
	viewViewer
	viewDashboard
)

// NavigationSource tracks where the viewer was opened from.
// Used by GoBackMsg handler to return to correct parent view.
type NavigationSource int

const (
	FromConversationList NavigationSource = iota // Default: viewer opened from conversation list
	FromDashboard                                // Viewer opened from dashboard pane
)

// AppModel is the root Bubbletea model for the interactive mode.
type AppModel struct {
	state                viewState
	projectModel         ProjectModel
	conversationModel    ConversationModel
	viewerModel          ViewerModel
	dashboardModel       DashboardModel // Dashboard view (Story 5.2)
	selectedProject      types.Project
	selectedConversation types.Conversation
	selectedProjects     []types.Project  // For dashboard view (Story 5.1)
	viewerSource         NavigationSource // Tracks where viewer was opened from (Story 5.5)
	width                int
	height               int
	spinner              spinner.Model
	loading              bool
	tokenService         *token.Service

	// Usage monitoring (Story 7.4)
	usageBar    *usage.UsageBarModel
	usageClient *usage.Client
}

// newUsageBarStyles creates the usage bar styles from the TUI style exports (Story 7.4).
func newUsageBarStyles() usage.UsageBarStyles {
	stylesExport := GetUsageBarStyles()
	return usage.UsageBarStyles{
		Container: stylesExport.Container,
		Label:     stylesExport.Label,
		Normal:    stylesExport.Normal,
		Warning:   stylesExport.Warning,
		Critical:  stylesExport.Critical,
		Dimmed:    stylesExport.Dimmed,
		Stale:     stylesExport.Stale,
	}
}

// NewAppModel creates a new application model with the project browser.
func NewAppModel(projects []types.Project) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ListStyles.Loading

	// Initialize token service with soft-fail
	tokenSvc, err := token.New()
	if err != nil {
		log.Printf("Warning: token service initialization failed: %v", err)
	}

	return AppModel{
		state:        viewProjects,
		projectModel: NewProjectModel(projects),
		spinner:      s,
		loading:      false,
		tokenService: tokenSvc,
		usageBar:     usage.NewUsageBarModel(newUsageBarStyles()),
		usageClient:  usage.NewClient(),
	}
}

// NewAppModelWithError creates an app model showing an error.
func NewAppModelWithError(err error) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ListStyles.Loading

	// Initialize token service with soft-fail
	tokenSvc, tokenErr := token.New()
	if tokenErr != nil {
		log.Printf("Warning: token service initialization failed: %v", tokenErr)
	}

	return AppModel{
		state:        viewProjects,
		projectModel: NewProjectModelWithError(err),
		spinner:      s,
		loading:      false,
		tokenService: tokenSvc,
		usageBar:     usage.NewUsageBarModel(newUsageBarStyles()),
		usageClient:  usage.NewClient(),
	}
}

// Init implements tea.Model.
func (m AppModel) Init() tea.Cmd {
	// Request window size to properly initialize the list dimensions
	// Add usage fetch on startup (Story 7.4 - async, non-blocking)
	return tea.Batch(m.projectModel.Init(), tea.WindowSize(), m.fetchUsage())
}

// Update implements tea.Model.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		// If app is loading, update app's spinner
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		// Otherwise, forward to active child model for their overlay spinners
		switch m.state {
		case viewConversations:
			newModel, cmd := m.conversationModel.Update(msg)
			m.conversationModel = newModel.(ConversationModel)
			return m, cmd
		case viewViewer:
			newModel, cmd := m.viewerModel.Update(msg)
			m.viewerModel = newModel.(ViewerModel)
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update usage bar width (Story 7.4)
		m.usageBar.SetWidth(msg.Width)

		// Calculate available height for child views (Story 7.4)
		viewHeight := msg.Height - UsageBarHeight
		childMsg := tea.WindowSizeMsg{Width: msg.Width, Height: viewHeight}

		// Forward adjusted size to current view
		switch m.state {
		case viewProjects:
			var cmd tea.Cmd
			newModel, cmd := m.projectModel.Update(childMsg)
			m.projectModel = newModel.(ProjectModel)
			return m, cmd
		case viewConversations:
			var cmd tea.Cmd
			newModel, cmd := m.conversationModel.Update(childMsg)
			m.conversationModel = newModel.(ConversationModel)
			return m, cmd
		case viewViewer:
			var cmd tea.Cmd
			newModel, cmd := m.viewerModel.Update(childMsg)
			m.viewerModel = newModel.(ViewerModel)
			return m, cmd
		case viewDashboard:
			var cmd tea.Cmd
			newModel, cmd := m.dashboardModel.Update(childMsg)
			m.dashboardModel = newModel.(DashboardModel)
			return m, cmd
		}

	case ProjectSelectedMsg:
		// User selected a project, load its conversations
		m.loading = true
		m.selectedProject = msg.Project
		return m, tea.Batch(m.spinner.Tick, m.loadConversations())

	case conversationsLoadedMsg:
		// Conversations loaded, show the conversation list
		m.loading = false // Stop spinner
		if msg.err != nil {
			// Handle error - go back to projects
			return m, nil
		}

		// Calculate adjusted height for child views (Story 7.4)
		viewHeight := m.height - UsageBarHeight

		if len(msg.conversations) == 0 {
			// No conversations - show empty conversation list
			m.conversationModel = NewConversationModel(msg.conversations, m.selectedProject.DisplayName)
			m.conversationModel.SetSize(m.width, viewHeight)
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
		m.conversationModel.SetSize(m.width, viewHeight)
		m.state = viewConversations
		return m, nil

	case ConversationSelectedMsg:
		// User selected a conversation, load it
		// Clear token cache before loading new conversation
		if m.tokenService != nil {
			m.tokenService.ClearCache()
		}
		m.loading = true
		m.selectedConversation = msg.Conversation
		return m, tea.Batch(m.spinner.Tick, m.loadConversation(msg.Conversation.FilePath))

	case ConversationSelectedWithWatchMsg:
		// User selected a conversation with watch mode enabled
		// Clear token cache before loading new conversation
		if m.tokenService != nil {
			m.tokenService.ClearCache()
		}
		m.loading = true
		m.selectedConversation = msg.Conversation
		return m, tea.Batch(m.spinner.Tick, m.loadConversationWithWatch(msg.Conversation.FilePath))

	case conversationLoadedMsg:
		// Conversation loaded, switch to viewer
		m.loading = false // Stop spinner
		if msg.err != nil {
			// Handle error - stay on conversation list
			return m, nil
		}
		// Calculate adjusted height for child views (Story 7.4)
		viewHeight := m.height - UsageBarHeight
		title := m.buildConversationTitle()
		opts := RenderOptions{FilePath: msg.filePath}
		m.viewerModel = NewViewerModelWithBackNavigation(msg.entries, msg.parseErrors, title, opts, m.tokenService)
		m.viewerModel.SetSize(m.width, viewHeight)
		m.state = viewViewer
		return m, nil

	case conversationLoadedWithWatchMsg:
		// Conversation loaded with watch mode, switch to viewer
		m.loading = false // Stop spinner
		if msg.err != nil {
			// Handle error - stay on conversation list
			return m, nil
		}
		// Calculate adjusted height for child views (Story 7.4)
		viewHeight := m.height - UsageBarHeight
		title := m.buildConversationTitle()
		opts := RenderOptions{WatchMode: true, FilePath: msg.filePath}
		m.viewerModel = NewViewerModelWithBackNavigation(msg.entries, msg.parseErrors, title, opts, m.tokenService)
		m.viewerModel.SetSize(m.width, viewHeight)
		m.state = viewViewer
		return m, m.viewerModel.Init() // Return Init() to start watcher

	case BackToProjectsFromConversationsMsg:
		// User pressed escape in conversation list, go back to projects
		m.state = viewProjects
		return m, nil

	case GoBackMsg:
		// User pressed escape in viewer, return to source view (Story 5.5)
		// Clear token cache when leaving viewer
		if m.tokenService != nil {
			m.tokenService.ClearCache()
		}
		if m.viewerSource == FromDashboard {
			m.state = viewDashboard
			m.viewerSource = FromConversationList // Reset for next navigation
			// Resume dashboard watchers that were suspended during viewer navigation
			return m, m.dashboardModel.ResumeWatchers()
		}
		m.state = viewConversations
		m.viewerSource = FromConversationList // Reset for next navigation
		return m, nil

	case DashboardSelectedMsg:
		// User selected multiple projects for dashboard view (Story 5.1, 5.2, 5.3)
		m.selectedProjects = msg.Projects
		var cmd tea.Cmd
		m.dashboardModel, cmd = NewDashboardModel(msg.Projects)
		// Calculate adjusted height for child views (Story 7.4)
		viewHeight := m.height - UsageBarHeight
		m.dashboardModel.SetSize(m.width, viewHeight)
		m.state = viewDashboard
		return m, cmd

	case GoBackToProjectsFromDashboardMsg:
		// User pressed escape in dashboard, go back to projects (Story 5.2, 5.3)
		// Watchers are already closed in DashboardModel.Update() before sending this msg
		m.projectModel.ClearSelections()
		m.projectModel.updateItemsWithSelection()
		m.state = viewProjects
		return m, nil

	case OpenViewerFromDashboardMsg:
		// User pressed Enter on a pane in dashboard - open viewer (Story 5.5)
		m.loading = true
		m.selectedConversation = types.Conversation{FilePath: msg.FilePath}
		m.selectedProject = msg.Project
		m.viewerSource = FromDashboard
		return m, tea.Batch(m.spinner.Tick, m.loadConversation(msg.FilePath))

	case ShowToastMsg:
		// For now, we just ignore toast messages since project view doesn't have toast UI
		// TODO: Add toast UI to project view or AppModel in future stories
		return m, nil

	case usageFetchedMsg:
		// Handle usage fetch result (Story 7.4)
		if msg.err != nil {
			// Handle specific error types
			if errors.Is(msg.err, usage.ErrNoCredentials) ||
				errors.Is(msg.err, usage.ErrKeychainNotFound) ||
				errors.Is(msg.err, usage.ErrKeychainTimeout) {
				m.usageBar.SetNotLoggedIn()
			} else if errors.Is(msg.err, usage.ErrTokenExpired) {
				m.usageBar.SetError("Session expired")
			} else if msg.limits != nil {
				// Error but have stale data
				m.usageBar.SetLimits(msg.limits, true)
			} else {
				m.usageBar.SetError("Usage unavailable")
			}
		} else {
			m.usageBar.SetLimits(msg.limits, msg.stale)
		}
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

	case viewDashboard:
		// Forward messages to dashboard model (Story 5.2)
		var cmd tea.Cmd
		newModel, cmd := m.dashboardModel.Update(msg)
		m.dashboardModel = newModel.(DashboardModel)
		return m, cmd
	}

	return m, nil
}

// View implements tea.Model.
func (m AppModel) View() string {
	// Render usage bar at top (Story 7.4)
	usageBarView := m.usageBar.View()

	var contentView string
	if m.loading {
		contentView = m.loadingView()
	} else {
		switch m.state {
		case viewProjects:
			contentView = m.projectModel.View()
		case viewConversations:
			contentView = m.conversationModel.View()
		case viewViewer:
			contentView = m.viewerModel.View()
		case viewDashboard:
			contentView = m.dashboardModel.View()
		default:
			contentView = m.projectModel.View()
		}
	}

	// Use simple string concatenation to ensure usage bar stays at top
	// (JoinVertical may have issues with viewport ANSI codes)
	return usageBarView + "\n" + contentView
}

// loadingView renders the spinner during loading operations.
func (m AppModel) loadingView() string {
	loadingText := m.spinner.View() + " " + ListStyles.Loading.Render("Loading...")
	// Guard against uninitialized dimensions (before WindowSizeMsg)
	if m.width == 0 || m.height == 0 {
		return loadingText
	}
	// Calculate available height for content (excluding usage bar - Story 7.4)
	viewHeight := m.height - UsageBarHeight
	return lipgloss.Place(
		m.width, viewHeight,
		lipgloss.Center, lipgloss.Center,
		loadingText,
	)
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
	filePath    string // File path for watch mode toggle
}

type conversationLoadedWithWatchMsg struct {
	entries     []types.LogEntry
	parseErrors int
	err         error
	filePath    string
}

// usageFetchedMsg carries the result of a usage API fetch (Story 7.4).
type usageFetchedMsg struct {
	limits *usage.UsageLimits
	stale  bool
	err    error
}

// fetchUsage returns a command that fetches usage asynchronously (Story 7.4).
func (m AppModel) fetchUsage() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		token, err := usage.GetOAuthToken()
		if err != nil {
			return usageFetchedMsg{err: err}
		}

		limits, stale, err := m.usageClient.FetchUsage(ctx, token)
		return usageFetchedMsg{limits: limits, stale: stale, err: err}
	}
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
			filePath:    filePath,
		}
	}
}

// buildConversationTitle builds a display title for the viewer.
// Format: "project-name - timestamp (model-short)" where model is truncated if >20 chars.
func (m AppModel) buildConversationTitle() string {
	title := fmt.Sprintf("%s - %s", m.selectedProject.DisplayName, formatTimestamp(m.selectedConversation.LastModified))
	if m.selectedConversation.Model != "" {
		modelShort := m.selectedConversation.Model
		if VisualWidth(modelShort) > 20 {
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
	return title
}

// loadConversationWithWatch loads a conversation file with watch mode enabled.
func (m AppModel) loadConversationWithWatch(filePath string) tea.Cmd {
	return func() tea.Msg {
		result, err := parser.ParseJSONLFile(filePath)
		if err != nil {
			return conversationLoadedWithWatchMsg{err: err}
		}
		return conversationLoadedWithWatchMsg{
			entries:     result.Entries,
			parseErrors: result.ParseErrors,
			filePath:    filePath,
		}
	}
}

// UsageBarState returns the current state of the usage bar for testing (Story 7.4).
func (m AppModel) UsageBarState() usage.UsageBarState {
	return m.usageBar.State()
}
