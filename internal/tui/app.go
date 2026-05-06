// Package tui provides the terminal user interface components.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/token"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/usage"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

// viewState represents the current view in the application.
type viewState int

const (
	viewProjects viewState = iota
	viewConversations
	viewViewer
	viewDashboard
	viewSessionDashboard // Phase 5a: Multi-agent session dashboard
	viewAgentSelector    // Agent selector entry point
)

// Usage refresh constants (Story 7.5)
const (
	refreshInterval = 60 * time.Second
	refreshDebounce = 5 * time.Second
)

// NavigationSource tracks where the viewer was opened from.
// Used by GoBackMsg handler to return to correct parent view.
type NavigationSource int

const (
	FromConversationList NavigationSource = iota // Default: viewer opened from conversation list
	FromDashboard                                // Viewer opened from dashboard pane
	FromSessionDashboard                         // Viewer opened from session dashboard pane (Phase 5a)
)

// AppModel is the root Bubbletea model for the interactive mode.
type AppModel struct {
	state                 viewState
	projectModel          ProjectModel
	conversationModel     ConversationModel
	viewerModel           ViewerModel
	dashboardModel        DashboardModel        // Dashboard view (Story 5.2)
	sessionDashboardModel SessionDashboardModel // Session dashboard view (Phase 5a)
	agentSelectorModel    AgentSelectorModel    // Agent selector entry point
	selectedProject       types.Project
	selectedConversation  types.Conversation
	selectedProjects      []types.Project  // For dashboard view (Story 5.1)
	viewerSource          NavigationSource // Tracks where viewer was opened from (Story 5.5)
	width                 int
	height                int
	spinner               spinner.Model
	loading               bool
	tokenService          *token.Service

	// Agent provider integration
	usingProviders   bool                // True when initialized via NewAppModelWithProviders
	selectedProvider agent.AgentProvider // Currently selected agent provider (nil for claude-code only mode)

	// Usage monitoring (Story 7.4)
	usageBar    *usage.UsageBarModel
	usageClient *usage.Client

	// Usage refresh state (Story 7.5)
	lastRefreshTime   time.Time
	refreshInProgress bool

	// Auth retry state (Story 11.1)
	authExpired bool

	// Rate limit backoff: skip periodic ticks until this time
	rateLimitUntil time.Time

	// noMultiSession disables the session dashboard; project selection goes
	// directly to the conversation list (pre-Phase-5a behavior).
	noMultiSession bool
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

// SetNoMultiSession disables the session dashboard so that project selection
// goes directly to the conversation list (pre-Phase-5a behavior).
func (m *AppModel) SetNoMultiSession(v bool) {
	m.noMultiSession = v
}

// NewAppModel creates a new application model with the project browser.
func NewAppModel(projects []types.Project) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ListStyles.Loading

	// Initialize token service with soft-fail
	tokenSvc, _ := token.New()

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

// NewAppModelWithProviders creates an app model that starts with the agent selector.
// The user picks a provider, then browses that provider's projects and sessions.
func NewAppModelWithProviders(providers []agent.AgentProvider) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ListStyles.Loading

	tokenSvc, _ := token.New()

	return AppModel{
		state:              viewAgentSelector,
		agentSelectorModel: NewAgentSelectorModelLoading(providers),
		spinner:            s,
		loading:            false,
		tokenService:       tokenSvc,
		usageBar:           usage.NewUsageBarModel(newUsageBarStyles()),
		usageClient:        usage.NewClient(),
		usingProviders:     true,
	}
}

// NewAppModelForSessions creates an app model that starts directly in session dashboard mode.
// projectPath is the decoded filesystem path (e.g., /Users/foo/project).
// projectDir is the Claude encoded project directory (e.g., ~/.claude/projects/-Users-foo-project).
func NewAppModelForSessions(projectPath, projectDir string, opts ...SessionDashboardOption) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ListStyles.Loading

	// Initialize token service with soft-fail
	tokenSvc, _ := token.New()

	// Create session scanner and monitor with defaults
	scannerInst := session.NewSessionScanner("")
	monitorInst := session.NewMonitor()

	// Apply options for testing or custom configuration
	cfg := sessionDashboardConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.scanner != nil {
		scannerInst = cfg.scanner
	}
	if cfg.monitor != nil {
		monitorInst = cfg.monitor
	}

	// Build dashboard model options: add dir watcher when available.
	var dashOpts []SessionDashboardModelOption
	dirWatcherInst := cfg.dirWatcher
	if dirWatcherInst == nil {
		// Create a default dir watcher for real-time session detection.
		// Errors are non-fatal — the polling scanner provides fallback detection.
		if dw, err := session.NewSessionDirectoryWatcher(""); err == nil {
			dirWatcherInst = dw
		}
	}
	if dirWatcherInst != nil {
		dashOpts = append(dashOpts, WithDashboardDirWatcher(dirWatcherInst))
	}

	sessionDash := NewSessionDashboardModel(projectPath, projectDir, scannerInst, monitorInst, dashOpts...)

	return AppModel{
		state:                 viewSessionDashboard,
		sessionDashboardModel: sessionDash,
		spinner:               s,
		loading:               false,
		tokenService:          tokenSvc,
		usageBar:              usage.NewUsageBarModel(newUsageBarStyles()),
		usageClient:           usage.NewClient(),
	}
}

// SessionDashboardOption configures the session dashboard.
type SessionDashboardOption func(*sessionDashboardConfig)

type sessionDashboardConfig struct {
	scanner    *session.SessionScanner
	monitor    *session.Monitor
	dirWatcher *session.SessionDirectoryWatcher
}

// WithSessionScanner sets a custom session scanner (useful for testing).
func WithSessionScanner(s *session.SessionScanner) SessionDashboardOption {
	return func(cfg *sessionDashboardConfig) {
		cfg.scanner = s
	}
}

// WithSessionMonitor sets a custom session monitor (useful for testing).
func WithSessionMonitor(m *session.Monitor) SessionDashboardOption {
	return func(cfg *sessionDashboardConfig) {
		cfg.monitor = m
	}
}

// WithSessionDirWatcher sets a custom SessionDirectoryWatcher for real-time
// file-system event detection. When provided, session file creations and
// deletions in ~/.claude/sessions/ are immediately mapped to pane lifecycle
// events without waiting for the polling scanner cycle.
func WithSessionDirWatcher(w *session.SessionDirectoryWatcher) SessionDashboardOption {
	return func(cfg *sessionDashboardConfig) {
		cfg.dirWatcher = w
	}
}

// NewAppModelWithError creates an app model showing an error.
func NewAppModelWithError(err error) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ListStyles.Loading

	// Initialize token service with soft-fail
	tokenSvc, _ := token.New()

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
	// Add periodic refresh scheduling (Story 7.5)
	if m.state == viewAgentSelector {
		return tea.Batch(m.loadAgentProviders(), tea.WindowSize(), m.fetchUsage(), scheduleUsageTick())
	}
	if m.state == viewSessionDashboard {
		// Phase 5a: Session dashboard mode - start session detection pipeline
		return tea.Batch(m.sessionDashboardModel.Init(), tea.WindowSize(), m.fetchUsage(), scheduleUsageTick())
	}
	return tea.Batch(m.projectModel.Init(), tea.WindowSize(), m.fetchUsage(), scheduleUsageTick())
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
		case viewSessionDashboard:
			newModel, cmd := m.sessionDashboardModel.Update(msg)
			m.sessionDashboardModel = newModel.(SessionDashboardModel)
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
		case viewSessionDashboard:
			// Phase 5a: Forward size through Update so embedded viewers are resized too
			var cmd tea.Cmd
			newModel, cmd := m.sessionDashboardModel.Update(childMsg)
			m.sessionDashboardModel = newModel.(SessionDashboardModel)
			return m, cmd
		}

	case AgentSelectedMsg:
		// User selected an agent provider — discover projects and show project list.
		m.selectedProvider = msg.Provider
		agentProjects := msg.Projects
		if len(agentProjects) == 0 {
			discovered, err := msg.Provider.DiscoverProjects()
			if err != nil {
				m.state = viewAgentSelector // Stay on selector
				return m, nil
			}
			agentProjects = discovered
		}
		if len(agentProjects) == 0 {
			m.state = viewAgentSelector // Stay on selector
			return m, nil
		}
		// Convert agent.Project → types.Project
		projects := make([]types.Project, 0, len(agentProjects))
		for _, ap := range agentProjects {
			projects = append(projects, types.Project{
				DecodedPath:       ap.Path,
				DisplayName:       ap.DisplayName,
				DirPath:           ap.Directory,
				ConversationCount: ap.SessionCount,
			})
		}
		viewHeight := m.height - UsageBarHeight
		m.projectModel = NewProjectModelWithBack(projects)
		m.projectModel.SetSize(m.width, viewHeight)
		m.state = viewProjects
		return m, nil

	case BackToAgentSelectorMsg:
		// User pressed esc/h in project list — return to agent selector.
		m.selectedProvider = nil
		m.state = viewAgentSelector
		return m, nil

	case ProjectSelectedMsg:
		m.selectedProject = msg.Project

		// Claude Code keeps the existing concurrent-session dashboard even
		// when reached through the provider selector. Other providers use their
		// provider-native session list until they have dashboard support.
		if m.usingProviders && m.selectedProvider != nil {
			if m.selectedProvider.Type() == agent.AgentClaudeCode && !m.noMultiSession {
				return m.openSessionDashboard(msg.Project)
			}
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadProviderSessions())
		}

		// When multi-session is disabled, skip the session dashboard and go
		// directly to the conversation list (pre-Phase-5a behavior).
		if m.noMultiSession {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadConversations())
		}

		return m.openSessionDashboard(msg.Project)

	case OpenConversationsFromSessionDashboardMsg:
		// User pressed 'c' in the session dashboard — load conversation list for the project.
		// The project was captured when ProjectSelectedMsg was originally handled.
		m.loading = true
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
		if m.usingProviders && m.selectedProvider != nil {
			return m, tea.Batch(m.spinner.Tick, m.loadProviderSession())
		}
		return m, tea.Batch(m.spinner.Tick, m.loadConversation(msg.Conversation.FilePath))

	case ConversationSelectedWithWatchMsg:
		// User selected a conversation with watch mode enabled
		// Clear token cache before loading new conversation
		if m.tokenService != nil {
			m.tokenService.ClearCache()
		}
		m.loading = true
		m.selectedConversation = msg.Conversation
		if m.usingProviders && m.selectedProvider != nil {
			return m, tea.Batch(m.spinner.Tick, m.loadProviderSessionWithWatch())
		}
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
		opts := m.viewerRenderOptions(msg.filePath, false)
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
		opts := m.viewerRenderOptions(msg.filePath, true)
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
		if m.viewerSource == FromSessionDashboard {
			// Phase 5a: Return to session dashboard
			m.state = viewSessionDashboard
			m.viewerSource = FromConversationList // Reset for next navigation
			return m, nil
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
		// Story 9.2 fix: Must call Init() to start subscription polling for live updates
		initCmd := m.dashboardModel.Init()
		return m, tea.Batch(cmd, initCmd)

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

	case GoBackFromSessionDashboardMsg:
		// Phase 5a: User pressed escape in session dashboard, go back to projects
		m.state = viewProjects
		return m, nil

	case OpenViewerFromSessionDashboardMsg:
		// Phase 5a: User pressed Enter on a session pane - open viewer
		m.loading = true
		m.selectedConversation = types.Conversation{FilePath: msg.FilePath}
		m.selectedProject = msg.Project
		m.viewerSource = FromSessionDashboard
		return m, tea.Batch(m.spinner.Tick, m.loadConversation(msg.FilePath))

	case ShowToastMsg:
		// For now, we just ignore toast messages since project view doesn't have toast UI
		// TODO: Add toast UI to project view or AppModel in future stories
		return m, nil

	case usageTickMsg:
		// Periodic refresh trigger (Story 7.5)
		// Skip during rate-limit backoff to avoid re-triggering the limit
		if !m.rateLimitUntil.IsZero() && time.Now().Before(m.rateLimitUntil) {
			return m, scheduleUsageTick()
		}
		// Only refresh if not already in progress and not in loading state
		if !m.refreshInProgress && m.usageBar.State() != usage.StateLoading {
			m.refreshInProgress = true
			return m, tea.Batch(m.fetchUsage(), scheduleUsageTick())
		}
		// Reschedule even if skipped
		return m, scheduleUsageTick()

	case authRetryTickMsg:
		// Story 11.1: Auth retry when expired
		if m.authExpired && !m.refreshInProgress {
			m.refreshInProgress = true
			return m, m.fetchUsage()
		}
		// Recovered or refresh in progress - stop polling
		return m, nil

	case rateLimitRetryMsg:
		// Rate limit backoff expired - retry fetch
		if !m.refreshInProgress {
			m.refreshInProgress = true
			return m, m.fetchUsage()
		}
		return m, nil

	case usageFetchedMsg:
		// Capture for recovery detection (Story 11.1)
		wasExpired := m.authExpired

		// Update refresh state (Story 7.5)
		m.refreshInProgress = false // ALWAYS reset, even on error
		if msg.err == nil {
			m.lastRefreshTime = time.Now() // Only on success
		}

		// Handle usage fetch result (Story 7.4, 7.7, 11.1)
		if msg.err != nil {
			// Handle specific error types (expanded for Story 7.7)
			if errors.Is(msg.err, usage.ErrNoCredentials) ||
				errors.Is(msg.err, usage.ErrKeychainNotFound) ||
				errors.Is(msg.err, usage.ErrKeychainTimeout) ||
				errors.Is(msg.err, usage.ErrInvalidCredentials) ||
				errors.Is(msg.err, usage.ErrEmptyToken) {
				m.usageBar.SetNotLoggedIn()
			} else if errors.Is(msg.err, usage.ErrTokenExpired) {
				// Token expired - show actionable message (no log, expected behavior)
				m.usageBar.SetError("Run 'claude' to refresh")
				// Story 11.1: Track expired state and schedule retry
				if !m.authExpired {
					m.authExpired = true
				}
				return m, scheduleAuthRetryTick()
			} else if errors.Is(msg.err, usage.ErrRateLimited) {
				var rateLimitErr *usage.RateLimitError
				if errors.As(msg.err, &rateLimitErr) {
					// Block periodic ticks during backoff so they don't re-trigger the limit
					m.rateLimitUntil = time.Now().Add(rateLimitErr.RetryAfter)
					if msg.limits != nil {
						m.usageBar.SetLimits(msg.limits, true)
					} else {
						m.usageBar.SetError("Rate limited - retrying")
					}
					return m, scheduleRateLimitRetry(rateLimitErr.RetryAfter)
				}
			} else if errors.Is(msg.err, context.Canceled) {
				// Context canceled (e.g., app shutting down) - silently ignore
				// Keep current state, don't update bar
			} else if m.authExpired {
				// Story 11.1 AC-4: Network error during retry - keep polling silently
				return m, scheduleAuthRetryTick()
			} else if msg.limits != nil {
				// Error but have stale data - show stale values with "(stale)" indicator (AC-2)
				m.usageBar.SetLimits(msg.limits, true)
			} else {
				// No stale data available - show "Unknown" (AC-2)
				m.usageBar.SetError("Unknown")
			}
		} else {
			// Success
			// Clear rate-limit backoff on success
			m.rateLimitUntil = time.Time{}
			m.usageBar.SetLimits(msg.limits, msg.stale)
			// Story 11.1 AC-3: Recovery detection with toast
			if wasExpired {
				m.authExpired = false
				return m, func() tea.Msg {
					return ShowToastMsg{Message: "Usage limits restored"}
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		// Global key handlers (Story 7.5)
		if msg.String() == "R" {
			return m.handleManualRefresh()
		}
		// Fall through to route to child views
	}

	switch msg := msg.(type) {
	case agentProvidersLoadedMsg:
		m.agentSelectorModel = msg.selector
		if m.state != viewAgentSelector {
			return m, nil
		}
		return m, m.agentSelectorModel.Init()
	}

	// Route updates to current view
	switch m.state {
	case viewAgentSelector:
		var cmd tea.Cmd
		newModel, cmd := m.agentSelectorModel.Update(msg)
		m.agentSelectorModel = newModel.(AgentSelectorModel)
		return m, cmd

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

	case viewSessionDashboard:
		// Phase 5a: Forward messages to session dashboard model
		newModel, cmd := m.sessionDashboardModel.Update(msg)
		m.sessionDashboardModel = newModel.(SessionDashboardModel)
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
		case viewAgentSelector:
			contentView = m.agentSelectorModel.View()
		case viewProjects:
			contentView = m.projectModel.View()
		case viewConversations:
			contentView = m.conversationModel.View()
		case viewViewer:
			contentView = m.viewerModel.View()
		case viewDashboard:
			contentView = m.dashboardModel.View()
		case viewSessionDashboard:
			contentView = m.sessionDashboardModel.View()
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

type agentProvidersLoadedMsg struct {
	selector AgentSelectorModel
}

// usageFetchedMsg carries the result of a usage API fetch (Story 7.4).
type usageFetchedMsg struct {
	limits *usage.UsageLimits
	stale  bool
	err    error
}

// Retry/tick message types — three separate mechanisms:
// - usageTickMsg: periodic 60s refresh
// - authRetryTickMsg: 5min auth recovery polling
// - rateLimitRetryMsg: Retry-After based backoff for 429 responses

// usageTickMsg triggers periodic usage refresh (Story 7.5).
type usageTickMsg struct{}

// scheduleUsageTick schedules the next periodic refresh (Story 7.5).
func scheduleUsageTick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return usageTickMsg{}
	})
}

// authRetryTickMsg triggers retry when auth is expired (Story 11.1).
type authRetryTickMsg struct{}

// authRetryInterval is the polling interval when auth is expired (Story 11.1).
const authRetryInterval = 5 * time.Minute

// scheduleAuthRetryTick schedules auth retry (only called when expired) (Story 11.1).
func scheduleAuthRetryTick() tea.Cmd {
	return tea.Tick(authRetryInterval, func(t time.Time) tea.Msg {
		return authRetryTickMsg{}
	})
}

// rateLimitRetryMsg triggers retry after a 429 rate-limit response.
type rateLimitRetryMsg struct{}

// scheduleRateLimitRetry schedules a retry using the Retry-After duration from
// the 429 response, plus random jitter (0-15s) so multiple instances don't all
// retry at the exact same moment and re-trigger the rate limit.
func scheduleRateLimitRetry(delay time.Duration) tea.Cmd {
	jitter := time.Duration(rand.Int63n(int64(15 * time.Second)))
	return tea.Tick(delay+jitter, func(t time.Time) tea.Msg {
		return rateLimitRetryMsg{}
	})
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

func (m AppModel) loadAgentProviders() tea.Cmd {
	providers := append([]agent.AgentProvider(nil), m.agentSelectorModel.providers...)
	return func() tea.Msg {
		return agentProvidersLoadedMsg{selector: NewAgentSelectorModel(providers)}
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

func (m AppModel) openSessionDashboard(project types.Project) (tea.Model, tea.Cmd) {
	// Open the session dashboard for that project.
	// The session dashboard auto-detects active Claude Code sessions (AC 8).
	// Users can press 'c' in the session dashboard to view the conversation list,
	// or ESC to return to the project browser.
	scannerInst := session.NewSessionScanner("")
	monitorInst := session.NewMonitor()

	var dashOpts []SessionDashboardModelOption
	if dw, err := session.NewSessionDirectoryWatcher(""); err == nil {
		dashOpts = append(dashOpts, WithDashboardDirWatcher(dw))
	}

	viewHeight := m.height - UsageBarHeight
	m.sessionDashboardModel = NewSessionDashboardModel(
		project.DecodedPath,
		project.DirPath,
		scannerInst,
		monitorInst,
		dashOpts...,
	)
	m.sessionDashboardModel.SetSize(m.width, viewHeight)
	m.state = viewSessionDashboard
	return m, m.sessionDashboardModel.Init()
}

// loadProviderSessions loads sessions for the selected project using the agent provider.
func (m AppModel) loadProviderSessions() tea.Cmd {
	return func() tea.Msg {
		agentProject := agent.Project{
			Path:      m.selectedProject.DecodedPath,
			Directory: m.selectedProject.DirPath,
		}
		sessions, err := m.selectedProvider.DiscoverSessions(agentProject)
		if err != nil {
			return conversationsLoadedMsg{err: err}
		}
		// Convert agent.Session → types.Conversation
		conversations := make([]types.Conversation, 0, len(sessions))
		for _, s := range sessions {
			conversations = append(conversations, types.Conversation{
				FilePath:         s.FilePath,
				LastModified:     s.LastModified,
				CreationTime:     s.CreatedAt,
				MessageCount:     s.MessageCount,
				FirstUserMessage: s.FirstUserMessage,
				Model:            s.Model,
			})
		}
		return conversationsLoadedMsg{
			conversations: conversations,
			lazyEnabled:   false,
			loadedCount:   len(conversations),
		}
	}
}

// loadProviderSession loads a single session using the agent provider.
func (m AppModel) loadProviderSession() tea.Cmd {
	return func() tea.Msg {
		session := agent.Session{FilePath: m.selectedConversation.FilePath}
		convEntries, err := m.selectedProvider.ParseSession(session)
		if err != nil {
			return conversationLoadedMsg{err: err}
		}
		// Convert ConversationEntry → LogEntry
		entries := make([]types.LogEntry, 0, len(convEntries))
		for _, ce := range convEntries {
			entries = append(entries, convertConversationEntryToLogEntry(ce))
		}
		return conversationLoadedMsg{
			entries:  entries,
			filePath: m.selectedConversation.FilePath,
		}
	}
}

func (m AppModel) loadProviderSessionWithWatch() tea.Cmd {
	return func() tea.Msg {
		session := agent.Session{FilePath: m.selectedConversation.FilePath}
		convEntries, err := m.selectedProvider.ParseSession(session)
		if err != nil {
			return conversationLoadedWithWatchMsg{err: err}
		}
		entries := make([]types.LogEntry, 0, len(convEntries))
		for _, ce := range convEntries {
			entries = append(entries, convertConversationEntryToLogEntry(ce))
		}
		return conversationLoadedWithWatchMsg{
			entries:  entries,
			filePath: m.selectedConversation.FilePath,
		}
	}
}

func (m AppModel) viewerRenderOptions(filePath string, watchMode bool) RenderOptions {
	opts := RenderOptions{FilePath: filePath, WatchMode: watchMode}
	if m.usingProviders && m.selectedProvider != nil {
		if m.selectedProvider.Type() == agent.AgentCodex {
			opts.WatchParser = providerWatchParser(m.selectedProvider)
		}
		if watchable, ok := m.selectedProvider.(agent.WatchableProvider); ok {
			opts.ProviderWatch = providerWatchFactory(watchable, filePath)
		}
	}
	return opts
}

func providerWatchFactory(provider agent.WatchableProvider, sessionID string) ProviderWatchFactory {
	return func() (ProviderEntryWatcher, error) {
		w, err := provider.WatchSession(agent.Session{ID: sessionID, FilePath: sessionID})
		if err != nil {
			return nil, err
		}
		return providerEntryWatcher{watcher: w}, nil
	}
}

type providerEntryWatcher struct {
	watcher agent.SessionWatcher
}

func (w providerEntryWatcher) NewEntries() ([]types.LogEntry, error) {
	convEntries, err := w.watcher.NewEntries()
	if err != nil {
		return nil, err
	}
	entries := make([]types.LogEntry, 0, len(convEntries))
	for _, ce := range convEntries {
		entries = append(entries, convertConversationEntryToLogEntry(ce))
	}
	return entries, nil
}

func (w providerEntryWatcher) Close() error {
	return w.watcher.Close()
}

func providerWatchParser(provider agent.AgentProvider) watcher.EntryParser {
	return func(r io.Reader) ([]types.LogEntry, error) {
		convEntries, err := provider.ParseSessionStream(r)
		if err != nil {
			return nil, err
		}
		entries := make([]types.LogEntry, 0, len(convEntries))
		for _, ce := range convEntries {
			entries = append(entries, convertConversationEntryToLogEntry(ce))
		}
		return entries, nil
	}
}

// convertConversationEntryToLogEntry converts an agent.ConversationEntry to types.LogEntry.
func convertConversationEntryToLogEntry(ce agent.ConversationEntry) types.LogEntry {
	var entryType types.EntryType
	switch ce.Type() {
	case agent.EntryTypeUser:
		entryType = types.EntryTypeUser
	case agent.EntryTypeAssistant:
		entryType = types.EntryTypeAssistant
	default:
		entryType = types.EntryType(ce.Type())
	}

	blocks := ce.ContentBlocks()
	content := make([]types.MessageContent, 0, len(blocks))
	var textContent string

	for _, b := range blocks {
		switch b.ContentType() {
		case agent.ContentBlockText:
			content = append(content, types.MessageContent{
				Type: types.ContentTypeText,
				Text: b.Text(),
			})
		case agent.ContentBlockThinking:
			content = append(content, types.MessageContent{
				Type:     types.ContentTypeThinking,
				Thinking: b.Text(),
			})
		case agent.ContentBlockToolUse:
			content = append(content, types.MessageContent{
				Type:      types.ContentTypeToolUse,
				ToolName:  b.ToolName(),
				ToolInput: b.ToolInput(),
			})
		case agent.ContentBlockReasoning:
			content = append(content, types.MessageContent{
				Type: types.ContentTypeText,
				Text: b.Text(),
			})
		}
	}

	if ce.Type() == agent.EntryTypeUser && len(content) == 1 && content[0].Type == types.ContentTypeText {
		textContent = content[0].Text
	}

	tokens := ce.TokenUsage()

	return types.LogEntry{
		Type:      entryType,
		Timestamp: ce.Timestamp(),
		SessionID: ce.SessionID(),
		Message: types.Message{
			Role:        ce.Role(),
			Content:     content,
			TextContent: textContent,
		},
		Usage: types.TokenUsage{
			InputTokens:              tokens.InputTokens,
			OutputTokens:             tokens.OutputTokens,
			CacheCreationInputTokens: tokens.CachedTokens,
		},
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

// SessionDashboardState returns the current session dashboard model for testing (Phase 5a).
func (m AppModel) SessionDashboardState() SessionDashboardModel {
	return m.sessionDashboardModel
}

// State returns the current view state for testing.
func (m AppModel) State() viewState {
	return m.state
}

// handleManualRefresh handles the R key for manual usage refresh (Story 7.5).
func (m AppModel) handleManualRefresh() (tea.Model, tea.Cmd) {
	// Skip if already refreshing
	if m.refreshInProgress {
		return m, nil
	}

	// Skip if in loading state
	if m.usageBar.State() == usage.StateLoading {
		return m, nil
	}

	// Skip if within debounce window
	if time.Since(m.lastRefreshTime) < refreshDebounce {
		return m, nil
	}

	// Trigger manual refresh
	m.usageClient.InvalidateCache() // Force fresh fetch
	m.rateLimitUntil = time.Time{}  // Clear rate-limit backoff on manual refresh
	m.refreshInProgress = true
	m.usageBar.SetRefreshing() // Show indicator (Story 7.5)
	return m, m.fetchUsage()
}
