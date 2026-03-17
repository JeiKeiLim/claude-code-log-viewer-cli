package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/usage"
)

// Story 5.5 Tests: App.go Integration for Navigation Source

func TestNavigationSourceDefault(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Default should be FromConversationList (zero value)
	if model.viewerSource != FromConversationList {
		t.Errorf("Default viewerSource = %d, want %d (FromConversationList)",
			model.viewerSource, FromConversationList)
	}
}

func TestGoBackMsgFromConversationList(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.state = viewViewer
	model.viewerSource = FromConversationList

	// Handle GoBackMsg
	newModel, _ := model.Update(GoBackMsg{})
	updatedModel := newModel.(AppModel)

	// Should return to conversations
	if updatedModel.state != viewConversations {
		t.Errorf("GoBackMsg from conversation list: state = %d, want %d (viewConversations)",
			updatedModel.state, viewConversations)
	}
	// viewerSource should be reset
	if updatedModel.viewerSource != FromConversationList {
		t.Errorf("viewerSource should be reset to FromConversationList")
	}
}

func TestGoBackMsgFromDashboard(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.state = viewViewer
	model.viewerSource = FromDashboard

	// Handle GoBackMsg
	newModel, _ := model.Update(GoBackMsg{})
	updatedModel := newModel.(AppModel)

	// Should return to dashboard
	if updatedModel.state != viewDashboard {
		t.Errorf("GoBackMsg from dashboard: state = %d, want %d (viewDashboard)",
			updatedModel.state, viewDashboard)
	}
	// viewerSource should be reset
	if updatedModel.viewerSource != FromConversationList {
		t.Errorf("viewerSource should be reset to FromConversationList")
	}
}

func TestOpenViewerFromDashboardMsgHandler(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model := NewAppModel(projects)
	model.state = viewDashboard
	model.width = 80
	model.height = 40

	// Create the message
	msg := OpenViewerFromDashboardMsg{
		FilePath: "/tmp/test/conv.jsonl",
		Project:  projects[0],
	}

	// Handle the message
	newModel, cmd := model.Update(msg)
	updatedModel := newModel.(AppModel)

	// Should set loading state
	if !updatedModel.loading {
		t.Error("OpenViewerFromDashboardMsg should set loading = true")
	}

	// Should set viewerSource to FromDashboard
	if updatedModel.viewerSource != FromDashboard {
		t.Errorf("viewerSource = %d, want %d (FromDashboard)",
			updatedModel.viewerSource, FromDashboard)
	}

	// Should set selectedConversation
	if updatedModel.selectedConversation.FilePath != "/tmp/test/conv.jsonl" {
		t.Errorf("selectedConversation.FilePath = %q, want %q",
			updatedModel.selectedConversation.FilePath, "/tmp/test/conv.jsonl")
	}

	// Should set selectedProject
	if updatedModel.selectedProject.DisplayName != "proj1" {
		t.Errorf("selectedProject.DisplayName = %q, want %q",
			updatedModel.selectedProject.DisplayName, "proj1")
	}

	// Should return a command (batch of spinner tick + load)
	if cmd == nil {
		t.Error("OpenViewerFromDashboardMsg should return a command")
	}
}

func TestNavigationSourceEnum(t *testing.T) {
	// Verify enum values
	if FromConversationList != 0 {
		t.Errorf("FromConversationList = %d, want 0", FromConversationList)
	}
	if FromDashboard != 1 {
		t.Errorf("FromDashboard = %d, want 1", FromDashboard)
	}
}

func TestAppModelViewerSourceField(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set and verify
	model.viewerSource = FromDashboard
	if model.viewerSource != FromDashboard {
		t.Error("viewerSource field should be settable")
	}
}

func TestWindowSizeMsgForwarded(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Test that WindowSizeMsg updates dimensions
	msg := tea.WindowSizeMsg{Width: 120, Height: 60}
	newModel, _ := model.Update(msg)
	updatedModel := newModel.(AppModel)

	if updatedModel.width != 120 {
		t.Errorf("width = %d, want 120", updatedModel.width)
	}
	if updatedModel.height != 60 {
		t.Errorf("height = %d, want 60", updatedModel.height)
	}
}

// Story 5.6 Tests: Dashboard Navigation Hierarchy

func TestGoBackToProjectsFromDashboardMsgHandler(t *testing.T) {
	projects := []types.Project{
		{DisplayName: "proj1", DirPath: "/tmp/proj1"},
		{DisplayName: "proj2", DirPath: "/tmp/proj2"},
	}
	model := NewAppModel(projects)
	model.state = viewDashboard

	// Handle GoBackToProjectsFromDashboardMsg
	newModel, _ := model.Update(GoBackToProjectsFromDashboardMsg{})
	updatedModel := newModel.(AppModel)

	// Should return to projects view
	if updatedModel.state != viewProjects {
		t.Errorf("GoBackToProjectsFromDashboardMsg: state = %d, want %d (viewProjects)",
			updatedModel.state, viewProjects)
	}
}

func TestDashboardEscKeyEmitsGoBackToProjects(t *testing.T) {
	projects := []types.Project{
		{DisplayName: "proj1", DirPath: "/tmp/proj1"},
	}
	model, _ := NewDashboardModel(projects)
	model.SetSize(80, 40)

	// Simulate esc key press
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	_ = newModel.(DashboardModel)

	// Should return a command that produces GoBackToProjectsFromDashboardMsg
	if cmd == nil {
		t.Fatal("esc key should return a command")
	}

	// Execute the command and check the message type
	msg := cmd()
	if _, ok := msg.(GoBackToProjectsFromDashboardMsg); !ok {
		t.Errorf("esc key should emit GoBackToProjectsFromDashboardMsg, got %T", msg)
	}
}

func TestDashboardQKeyEmitsGoBackToProjects(t *testing.T) {
	projects := []types.Project{
		{DisplayName: "proj1", DirPath: "/tmp/proj1"},
	}
	model, _ := NewDashboardModel(projects)
	model.SetSize(80, 40)

	// Simulate q key press
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_ = newModel.(DashboardModel)

	// Should return a command that produces GoBackToProjectsFromDashboardMsg
	if cmd == nil {
		t.Fatal("q key should return a command")
	}

	// Execute the command and check the message type
	msg := cmd()
	if _, ok := msg.(GoBackToProjectsFromDashboardMsg); !ok {
		t.Errorf("q key should emit GoBackToProjectsFromDashboardMsg, got %T", msg)
	}
}

// Story 7.4 Tests: App Model Wrapper for Usage Bar

func TestAppModel_InitialUsageBarState(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Initial state should be StateLoading
	if model.UsageBarState() != usage.StateLoading {
		t.Errorf("Initial UsageBarState = %v, want StateLoading", model.UsageBarState())
	}
}

func TestAppModel_UsageFetchedMsg_Success(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
		SevenDay: &usage.UsageWindow{Utilization: 12.0},
	}

	newModel, _ := model.Update(usageFetchedMsg{limits: limits, stale: false})
	m := newModel.(AppModel)

	if m.UsageBarState() != usage.StateNormal {
		t.Errorf("UsageBarState = %v, want StateNormal", m.UsageBarState())
	}
}

func TestAppModel_UsageFetchedMsg_Stale(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}

	newModel, _ := model.Update(usageFetchedMsg{limits: limits, stale: true})
	m := newModel.(AppModel)

	if m.UsageBarState() != usage.StateStale {
		t.Errorf("UsageBarState = %v, want StateStale", m.UsageBarState())
	}
}

func TestAppModel_UsageFetchedMsg_NotLoggedIn(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrNoCredentials", usage.ErrNoCredentials},
		{"ErrKeychainNotFound", usage.ErrKeychainNotFound},
		{"ErrKeychainTimeout", usage.ErrKeychainTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projects := []types.Project{{DisplayName: "proj1"}}
			model := NewAppModel(projects)

			newModel, _ := model.Update(usageFetchedMsg{err: tt.err})
			m := newModel.(AppModel)

			if m.UsageBarState() != usage.StateNotLoggedIn {
				t.Errorf("UsageBarState for %s = %v, want StateNotLoggedIn", tt.name, m.UsageBarState())
			}
		})
	}
}

func TestAppModel_UsageFetchedMsg_TokenExpired(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	newModel, _ := model.Update(usageFetchedMsg{err: usage.ErrTokenExpired})
	m := newModel.(AppModel)

	if m.UsageBarState() != usage.StateError {
		t.Errorf("UsageBarState = %v, want StateError", m.UsageBarState())
	}
}

func TestAppModel_UsageFetchedMsg_ErrorWithStaleData(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}

	// Error with stale data should show stale state
	newModel, _ := model.Update(usageFetchedMsg{limits: limits, stale: true, err: usage.ErrAPITimeout})
	m := newModel.(AppModel)

	if m.UsageBarState() != usage.StateStale {
		t.Errorf("UsageBarState = %v, want StateStale for error with stale data", m.UsageBarState())
	}
}

func TestAppModel_UsageFetchedMsg_ErrorWithoutData(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Error without any data should show error state
	newModel, _ := model.Update(usageFetchedMsg{err: usage.ErrAPIError})
	m := newModel.(AppModel)

	if m.UsageBarState() != usage.StateError {
		t.Errorf("UsageBarState = %v, want StateError", m.UsageBarState())
	}
}

func TestAppModel_View_IncludesUsageBar(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set dimensions via WindowSizeMsg
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := newModel.(AppModel)

	view := m.View()

	// Usage bar should be at top (in loading state initially)
	if !strings.Contains(view, "Loading") {
		t.Error("View should include usage bar loading state")
	}
}

func TestAppModel_View_UsageBarAtTop(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set dimensions
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := newModel.(AppModel)

	// Update with usage data
	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
		SevenDay: &usage.UsageWindow{Utilization: 12.0},
	}
	newModel2, _ := m.Update(usageFetchedMsg{limits: limits, stale: false})
	m2 := newModel2.(AppModel)

	view := m2.View()
	lines := strings.Split(view, "\n")

	// First line should contain usage info (5h or 7d indicator)
	if len(lines) == 0 {
		t.Fatal("View should not be empty")
	}
	firstLine := lines[0]
	if !strings.Contains(firstLine, "5h") && !strings.Contains(firstLine, "7d") {
		t.Errorf("First line should contain usage info, got: %q", firstLine)
	}
}

func TestAppModel_WindowSizeMsg_AdjustsChildHeight(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Send WindowSizeMsg
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := newModel.(AppModel)

	// After update, internal dimensions should be stored
	if m.width != 80 {
		t.Errorf("width = %d, want 80", m.width)
	}
	if m.height != 24 {
		t.Errorf("height = %d, want 24", m.height)
	}

	// View should render without panic (implicit test of height adjustment)
	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestAppModel_WindowSizeMsg_UpdatesUsageBarWidth(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Send WindowSizeMsg with specific width
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := newModel.(AppModel)

	// The usageBar should have the width set (verify by rendering)
	view := m.View()
	if view == "" {
		t.Error("View should not be empty after WindowSizeMsg")
	}
}

func TestAppModel_LoadingView_IncludesUsageBar(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.loading = true

	// Set dimensions
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := newModel.(AppModel)
	m.loading = true

	view := m.View()

	// Should contain both loading indicator and usage bar
	if !strings.Contains(view, "Loading") {
		t.Error("Loading view should contain 'Loading' text")
	}
}

func TestAppModel_Init_IncludesFetchUsage(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Init should return a batch command
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init() should return a command")
	}

	// Execute the command - it should be a batch
	// We can't easily test the batch contents, but we verify it doesn't panic
	// and that the usage bar starts in loading state
	if model.UsageBarState() != usage.StateLoading {
		t.Errorf("UsageBarState after Init = %v, want StateLoading", model.UsageBarState())
	}
}

func TestNewAppModelWithError_InitializesUsageBar(t *testing.T) {
	model := NewAppModelWithError(nil)

	// Should have usage bar initialized
	if model.usageBar == nil {
		t.Fatal("usageBar should be initialized")
	}
	if model.usageClient == nil {
		t.Fatal("usageClient should be initialized")
	}

	// Initial state should be loading
	if model.UsageBarState() != usage.StateLoading {
		t.Errorf("UsageBarState = %v, want StateLoading", model.UsageBarState())
	}
}

func TestAppModel_AllViewStates_IncludeUsageBar(t *testing.T) {
	tests := []struct {
		name  string
		state viewState
	}{
		{"viewProjects", viewProjects},
		{"viewConversations", viewConversations},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projects := []types.Project{{DisplayName: "proj1"}}
			model := NewAppModel(projects)
			model.state = tt.state

			// Set dimensions
			newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m := newModel.(AppModel)
			m.state = tt.state

			view := m.View()

			// Should contain usage bar in loading state
			if !strings.Contains(view, "Loading") {
				t.Errorf("View in %s state should include usage bar", tt.name)
			}
		})
	}
}

func TestAppModel_ViewerState_IncludesUsageBar(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set dimensions
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := newModel.(AppModel)

	// Set up viewer model with minimal data
	m.viewerModel = NewViewerModel([]types.LogEntry{}, 0, "Test", RenderOptions{}, nil)
	m.viewerModel.SetSize(80, 23) // height - UsageBarHeight
	m.state = viewViewer

	view := m.View()

	// Should contain usage bar in loading state
	if !strings.Contains(view, "Loading") {
		t.Error("View in viewViewer state should include usage bar")
	}
}

func TestAppModel_DashboardState_IncludesUsageBar(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model := NewAppModel(projects)

	// Set dimensions
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := newModel.(AppModel)

	// Set up dashboard model
	m.dashboardModel, _ = NewDashboardModel(projects)
	m.dashboardModel.SetSize(80, 23) // height - UsageBarHeight
	m.state = viewDashboard

	view := m.View()

	// Should contain usage bar in loading state
	if !strings.Contains(view, "Loading") {
		t.Error("View in viewDashboard state should include usage bar")
	}
}

func TestAppModel_ConversationLoadedMsg_AdjustsHeight(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set dimensions first
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := newModel.(AppModel)

	// Simulate conversation loaded - verifies height adjustment doesn't panic
	msg := conversationLoadedMsg{
		entries:     []types.LogEntry{},
		parseErrors: 0,
		filePath:    "/tmp/test.jsonl",
	}
	newModel2, _ := m.Update(msg)
	m2 := newModel2.(AppModel)

	// Verify state changed to viewer
	if m2.state != viewViewer {
		t.Errorf("state = %d, want viewViewer", m2.state)
	}
}

func TestAppModel_DashboardSelectedMsg_AdjustsHeight(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model := NewAppModel(projects)

	// Set dimensions first
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := newModel.(AppModel)

	// Simulate dashboard selection
	msg := DashboardSelectedMsg{Projects: projects}
	newModel2, _ := m.Update(msg)
	m2 := newModel2.(AppModel)

	// Verify state changed to dashboard
	if m2.state != viewDashboard {
		t.Errorf("state = %d, want viewDashboard", m2.state)
	}
}

// Story 7.5 Tests: Usage Bar Refresh

func TestAppModel_RefreshStateFields(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Initial state: refreshInProgress should be false
	if model.refreshInProgress {
		t.Error("Initial refreshInProgress should be false")
	}

	// lastRefreshTime should be zero value
	if !model.lastRefreshTime.IsZero() {
		t.Error("Initial lastRefreshTime should be zero")
	}
}

func TestRefreshConstants(t *testing.T) {
	// Verify constants are defined with expected values
	if refreshInterval.Seconds() != 60 {
		t.Errorf("refreshInterval = %v, want 60s", refreshInterval)
	}
	if refreshDebounce.Seconds() != 5 {
		t.Errorf("refreshDebounce = %v, want 5s", refreshDebounce)
	}
}

func TestAppModel_UsageTickMsg_TriggersRefresh(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set usageBar to normal state (not loading) to allow refresh
	model.usageBar.SetLimits(&usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}, false)
	model.refreshInProgress = false

	newModel, cmd := model.Update(usageTickMsg{})
	m := newModel.(AppModel)

	if !m.refreshInProgress {
		t.Error("expected refreshInProgress to be true after tick")
	}
	if cmd == nil {
		t.Error("expected cmd to include fetchUsage and reschedule")
	}
}

func TestAppModel_UsageTickMsg_ReschedulesEvenWhenSkipped(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.refreshInProgress = true // Already refreshing

	newModel, cmd := model.Update(usageTickMsg{})
	m := newModel.(AppModel)

	// refreshInProgress should remain true (not changed)
	if !m.refreshInProgress {
		t.Error("refreshInProgress should still be true")
	}

	// Should still return a command (the reschedule tick)
	if cmd == nil {
		t.Error("expected reschedule tick command even when refresh is skipped")
	}
}

func TestAppModel_UsageTickMsg_SkippedDuringLoading(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	// Initial state is loading - should skip refresh

	newModel, cmd := model.Update(usageTickMsg{})
	m := newModel.(AppModel)

	// Should not set refreshInProgress since we're in loading state
	if m.refreshInProgress {
		t.Error("should not start refresh during loading state")
	}

	// Should still reschedule
	if cmd == nil {
		t.Error("expected reschedule tick command")
	}
}

func TestAppModel_ManualRefresh_R_Key(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Simulate successful initial fetch
	model.usageBar.SetLimits(&usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}, false)
	// Wait for debounce (set lastRefreshTime to 10s ago)
	model.lastRefreshTime = time.Now().Add(-10 * time.Second)

	// Press R key (shift+r)
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newModel, cmd := model.Update(keyMsg)
	m := newModel.(AppModel)

	if !m.refreshInProgress {
		t.Error("expected manual refresh to trigger")
	}
	if cmd == nil {
		t.Error("expected fetchUsage command")
	}
}

func TestAppModel_ManualRefresh_Debounce(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	model.usageBar.SetLimits(&usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}, false)
	model.lastRefreshTime = time.Now() // Just refreshed

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newModel, cmd := model.Update(keyMsg)
	m := newModel.(AppModel)

	if m.refreshInProgress {
		t.Error("expected refresh to be blocked by debounce")
	}
	if cmd != nil {
		t.Error("expected no command when debounced")
	}
}

func TestAppModel_ManualRefresh_IgnoredDuringLoading(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	// Initial state is loading (no usageFetchedMsg received)

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newModel, cmd := model.Update(keyMsg)
	m := newModel.(AppModel)

	if m.refreshInProgress {
		t.Error("expected no refresh during loading state")
	}
	if cmd != nil {
		t.Error("expected no command during loading state")
	}
}

func TestAppModel_ManualRefresh_LowercaseR_Ignored(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	model.usageBar.SetLimits(&usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}, false)
	model.lastRefreshTime = time.Now().Add(-10 * time.Second)

	// lowercase 'r' should NOT trigger refresh (only 'R' does)
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	newModel, _ := model.Update(keyMsg)
	m := newModel.(AppModel)

	if m.refreshInProgress {
		t.Error("lowercase 'r' should not trigger refresh")
	}
	// cmd may be non-nil due to forwarding to child views, so we check refreshInProgress instead
}

func TestAppModel_ManualRefresh_IgnoredWhenInProgress(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	model.usageBar.SetLimits(&usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}, false)
	model.refreshInProgress = true
	model.lastRefreshTime = time.Now().Add(-10 * time.Second)

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newModel, cmd := model.Update(keyMsg)
	m := newModel.(AppModel)

	// refreshInProgress should still be true (unchanged)
	if !m.refreshInProgress {
		t.Error("refreshInProgress should remain true")
	}
	if cmd != nil {
		t.Error("expected no additional refresh when already in progress")
	}
}

func TestAppModel_UsageFetchedMsg_UpdatesRefreshState(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.refreshInProgress = true

	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}

	newModel, _ := model.Update(usageFetchedMsg{limits: limits, stale: false})
	m := newModel.(AppModel)

	// refreshInProgress should be reset to false
	if m.refreshInProgress {
		t.Error("refreshInProgress should be false after successful fetch")
	}

	// lastRefreshTime should be updated
	if m.lastRefreshTime.IsZero() {
		t.Error("lastRefreshTime should be set after successful fetch")
	}
}

func TestAppModel_UsageFetchedMsg_ResetsRefreshOnError(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.refreshInProgress = true

	// Simulate error without stale data
	newModel, _ := model.Update(usageFetchedMsg{err: usage.ErrAPIError})
	m := newModel.(AppModel)

	// refreshInProgress should be reset to false EVEN on error
	if m.refreshInProgress {
		t.Error("refreshInProgress should be false even after error")
	}

	// lastRefreshTime should NOT be updated on error
	if !m.lastRefreshTime.IsZero() {
		t.Error("lastRefreshTime should not be updated on error")
	}
}

func TestAppModel_UsageFetchedMsg_UpdatesLastRefreshTimeOnSuccess(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.refreshInProgress = true

	before := time.Now()
	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}

	newModel, _ := model.Update(usageFetchedMsg{limits: limits, stale: false})
	m := newModel.(AppModel)
	after := time.Now()

	// lastRefreshTime should be between before and after
	if m.lastRefreshTime.Before(before) || m.lastRefreshTime.After(after) {
		t.Errorf("lastRefreshTime = %v, expected between %v and %v",
			m.lastRefreshTime, before, after)
	}
}

func TestAppModel_ManualRefresh_SetsRefreshingState(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set initial state with limits
	model.usageBar.SetLimits(&usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}, false)
	model.lastRefreshTime = time.Now().Add(-10 * time.Second)

	// Press R key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newModel, _ := model.Update(keyMsg)
	m := newModel.(AppModel)

	// Should be in refreshing state
	if m.UsageBarState() != usage.StateRefreshing {
		t.Errorf("UsageBarState = %v, want StateRefreshing", m.UsageBarState())
	}
}

func TestAppModel_StateRefreshing_ToNormal_OnSuccess(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set to refreshing state
	model.usageBar.SetLimits(&usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}, false)
	model.usageBar.SetRefreshing()
	model.refreshInProgress = true

	// Simulate successful fetch
	newModel, _ := model.Update(usageFetchedMsg{
		limits: &usage.UsageLimits{FiveHour: &usage.UsageWindow{Utilization: 40.0}},
		stale:  false,
	})
	m := newModel.(AppModel)

	if m.UsageBarState() != usage.StateNormal {
		t.Errorf("UsageBarState = %v, want StateNormal after successful fetch", m.UsageBarState())
	}
	if m.refreshInProgress {
		t.Error("refreshInProgress should be false after fetch")
	}
}

func TestAppModel_StateRefreshing_ToStale_OnError(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set to refreshing state with existing limits
	existingLimits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}
	model.usageBar.SetLimits(existingLimits, false)
	model.usageBar.SetRefreshing()
	model.refreshInProgress = true

	// Simulate error with stale cached data
	newModel, _ := model.Update(usageFetchedMsg{
		limits: existingLimits,
		stale:  true,
		err:    usage.ErrAPITimeout,
	})
	m := newModel.(AppModel)

	if m.UsageBarState() != usage.StateStale {
		t.Errorf("UsageBarState = %v, want StateStale after error with cached data", m.UsageBarState())
	}
}

func TestAppModel_Init_SchedulesUsageTick(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	cmd := model.Init()

	// Init should return a batch command
	if cmd == nil {
		t.Fatal("Init() should return a command")
	}

	// Verify that a batch was returned (indirect test)
	// The batch includes scheduleUsageTick which will produce usageTickMsg after 60s
	// We can't easily test timing, but verify the command exists
}

func TestScheduleUsageTick_ReturnsCmd(t *testing.T) {
	cmd := scheduleUsageTick()
	if cmd == nil {
		t.Fatal("scheduleUsageTick() should return a non-nil command")
	}
	// Note: We can't easily test the timing behavior, but we verify it returns a cmd
}

func TestAppModel_ManualRefresh_CacheInvalidation(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set up state to allow refresh
	model.usageBar.SetLimits(&usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}, false)
	model.lastRefreshTime = time.Now().Add(-10 * time.Second)

	// Trigger manual refresh
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newModel, cmd := model.Update(keyMsg)
	m := newModel.(AppModel)

	// Verify refresh was triggered
	if !m.refreshInProgress {
		t.Error("refreshInProgress should be true")
	}
	if cmd == nil {
		t.Error("should return fetchUsage command")
	}

	// Note: Cache invalidation happens via m.usageClient.InvalidateCache()
	// which we can't directly verify without mocking, but the test
	// verifies the code path is executed
}

func TestAppModel_RefreshBehavior_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		setupModel          func(*AppModel)
		expectRefresh       bool
		expectRefreshState  bool
		checkRefreshBlocked bool
	}{
		{
			name: "normal state allows refresh",
			setupModel: func(m *AppModel) {
				m.usageBar.SetLimits(&usage.UsageLimits{
					FiveHour: &usage.UsageWindow{Utilization: 35.0},
				}, false)
				m.lastRefreshTime = time.Now().Add(-10 * time.Second)
			},
			expectRefresh:      true,
			expectRefreshState: true,
		},
		{
			name: "loading state blocks refresh",
			setupModel: func(m *AppModel) {
				// Initial state is loading
			},
			expectRefresh:       false,
			checkRefreshBlocked: true,
		},
		{
			name: "debounce window blocks refresh",
			setupModel: func(m *AppModel) {
				m.usageBar.SetLimits(&usage.UsageLimits{
					FiveHour: &usage.UsageWindow{Utilization: 35.0},
				}, false)
				m.lastRefreshTime = time.Now() // Just refreshed
			},
			expectRefresh:       false,
			checkRefreshBlocked: true,
		},
		{
			name: "in-progress blocks refresh",
			setupModel: func(m *AppModel) {
				m.usageBar.SetLimits(&usage.UsageLimits{
					FiveHour: &usage.UsageWindow{Utilization: 35.0},
				}, false)
				m.refreshInProgress = true
				m.lastRefreshTime = time.Now().Add(-10 * time.Second)
			},
			expectRefresh:       false,
			checkRefreshBlocked: true,
		},
		{
			name: "stale state allows refresh",
			setupModel: func(m *AppModel) {
				m.usageBar.SetLimits(&usage.UsageLimits{
					FiveHour: &usage.UsageWindow{Utilization: 35.0},
				}, true) // stale
				m.lastRefreshTime = time.Now().Add(-10 * time.Second)
			},
			expectRefresh:      true,
			expectRefreshState: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projects := []types.Project{{DisplayName: "proj1"}}
			model := NewAppModel(projects)
			tt.setupModel(&model)

			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
			newModel, cmd := model.Update(keyMsg)
			m := newModel.(AppModel)

			if tt.expectRefresh {
				if cmd == nil {
					t.Error("expected refresh command")
				}
				if tt.expectRefreshState && m.UsageBarState() != usage.StateRefreshing {
					t.Errorf("UsageBarState = %v, want StateRefreshing", m.UsageBarState())
				}
			}

			if tt.checkRefreshBlocked {
				if cmd != nil {
					t.Error("expected no command when refresh blocked")
				}
			}
		})
	}
}

// Story 7.7 Tests: Graceful Degradation

func TestGracefulDegradation(t *testing.T) {
	tests := []struct {
		name        string
		fetchErr    error
		staleLimits *usage.UsageLimits
		wantState   usage.UsageBarState
		wantErrMsg  string
	}{
		{
			name:      "no credentials",
			fetchErr:  usage.ErrNoCredentials,
			wantState: usage.StateNotLoggedIn,
		},
		{
			name:      "keychain not found",
			fetchErr:  usage.ErrKeychainNotFound,
			wantState: usage.StateNotLoggedIn,
		},
		{
			name:      "keychain timeout",
			fetchErr:  usage.ErrKeychainTimeout,
			wantState: usage.StateNotLoggedIn,
		},
		{
			name:      "invalid credentials",
			fetchErr:  usage.ErrInvalidCredentials,
			wantState: usage.StateNotLoggedIn,
		},
		{
			name:      "empty token",
			fetchErr:  usage.ErrEmptyToken,
			wantState: usage.StateNotLoggedIn,
		},
		{
			name:       "token expired",
			fetchErr:   usage.ErrTokenExpired,
			wantState:  usage.StateError,
			wantErrMsg: "Run 'claude' to refresh",
		},
		{
			name:        "api timeout with stale data",
			fetchErr:    usage.ErrAPITimeout,
			staleLimits: &usage.UsageLimits{FiveHour: &usage.UsageWindow{Utilization: 50}},
			wantState:   usage.StateStale,
		},
		{
			name:       "api timeout without stale data",
			fetchErr:   usage.ErrAPITimeout,
			wantState:  usage.StateError,
			wantErrMsg: "Unknown",
		},
		{
			name:        "api error with stale data",
			fetchErr:    usage.ErrAPIError,
			staleLimits: &usage.UsageLimits{FiveHour: &usage.UsageWindow{Utilization: 25}},
			wantState:   usage.StateStale,
		},
		{
			name:       "api error without stale data",
			fetchErr:   usage.ErrAPIError,
			wantState:  usage.StateError,
			wantErrMsg: "Unknown",
		},
		{
			name:        "network error with stale data",
			fetchErr:    errors.New("dial tcp: connect: network unreachable"),
			staleLimits: &usage.UsageLimits{FiveHour: &usage.UsageWindow{Utilization: 30}},
			wantState:   usage.StateStale,
		},
		{
			name:       "network error without stale data",
			fetchErr:   errors.New("dial tcp: connect: network unreachable"),
			wantState:  usage.StateError,
			wantErrMsg: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create app model with empty projects
			m := NewAppModel([]types.Project{})

			// Simulate usageFetchedMsg
			msg := usageFetchedMsg{
				err:    tt.fetchErr,
				limits: tt.staleLimits,
				stale:  tt.staleLimits != nil,
			}

			result, _ := m.Update(msg)
			newModel := result.(AppModel)

			if newModel.UsageBarState() != tt.wantState {
				t.Errorf("got state %v, want %v", newModel.UsageBarState(), tt.wantState)
			}

			// Verify error message if applicable
			if tt.wantErrMsg != "" {
				// Check bar contains expected message
				view := newModel.usageBar.View()
				if !strings.Contains(view, tt.wantErrMsg) {
					t.Errorf("bar view %q does not contain %q", view, tt.wantErrMsg)
				}
			}
		})
	}
}

func TestNonBlockingInit(t *testing.T) {
	// Create app model
	m := NewAppModel([]types.Project{})

	// Init() should return a tea.Batch (multiple commands)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() returned nil, expected tea.Batch with async fetch")
	}

	// Verify the app is immediately usable (not blocked)
	// Project list should be accessible
	if m.state != viewProjects {
		t.Errorf("initial state %v, want viewProjects", m.state)
	}
}

func TestGracefulDegradation_RefreshResetOnError(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.refreshInProgress = true

	// Simulate any error
	newModel, _ := model.Update(usageFetchedMsg{err: usage.ErrAPIError})
	m := newModel.(AppModel)

	// refreshInProgress should be reset to false even on error
	if m.refreshInProgress {
		t.Error("refreshInProgress should be false after error")
	}
}

func TestGracefulDegradation_LastRefreshNotUpdatedOnError(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.refreshInProgress = true

	// Simulate error
	newModel, _ := model.Update(usageFetchedMsg{err: usage.ErrNoCredentials})
	m := newModel.(AppModel)

	// lastRefreshTime should NOT be updated on error
	if !m.lastRefreshTime.IsZero() {
		t.Error("lastRefreshTime should not be updated on error")
	}
}

func TestGracefulDegradation_AppFunctionsWithoutCredentials(t *testing.T) {
	// App should be usable even when credentials are missing
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model := NewAppModel(projects)

	// Simulate no credentials
	newModel, _ := model.Update(usageFetchedMsg{err: usage.ErrNoCredentials})
	m := newModel.(AppModel)

	// App should be in projects view (not crashed)
	if m.state != viewProjects {
		t.Errorf("state = %d, want viewProjects", m.state)
	}

	// Usage bar should show not logged in
	if m.UsageBarState() != usage.StateNotLoggedIn {
		t.Errorf("UsageBarState = %v, want StateNotLoggedIn", m.UsageBarState())
	}

	// View should render without panic
	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestGracefulDegradation_NewErrorsMapToNotLoggedIn(t *testing.T) {
	// Test the newly added error types (Story 7.7)
	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidCredentials", usage.ErrInvalidCredentials},
		{"ErrEmptyToken", usage.ErrEmptyToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projects := []types.Project{{DisplayName: "proj1"}}
			model := NewAppModel(projects)

			newModel, _ := model.Update(usageFetchedMsg{err: tt.err})
			m := newModel.(AppModel)

			if m.UsageBarState() != usage.StateNotLoggedIn {
				t.Errorf("UsageBarState for %s = %v, want StateNotLoggedIn", tt.name, m.UsageBarState())
			}
		})
	}
}

func TestGracefulDegradation_ContextCanceled(t *testing.T) {
	// Test context.Canceled is handled silently (Task 3.3)
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set initial state to normal with some limits
	initialLimits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}
	model.usageBar.SetLimits(initialLimits, false)
	initialState := model.UsageBarState()

	// Simulate context.Canceled error
	newModel, _ := model.Update(usageFetchedMsg{err: context.Canceled})
	m := newModel.(AppModel)

	// State should remain unchanged (not transition to error)
	if m.UsageBarState() != initialState {
		t.Errorf("context.Canceled should not change state, got %v, want %v",
			m.UsageBarState(), initialState)
	}
}

func TestGracefulDegradation_StaleIndicatorPresent(t *testing.T) {
	// Test AC-2: stale data shows "(stale)" indicator
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set dimensions
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := newModel.(AppModel)

	// Simulate stale data
	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 50.0},
	}
	newModel2, _ := m.Update(usageFetchedMsg{limits: limits, stale: true})
	m2 := newModel2.(AppModel)

	// Verify state is stale
	if m2.UsageBarState() != usage.StateStale {
		t.Errorf("UsageBarState = %v, want StateStale", m2.UsageBarState())
	}

	// Verify bar view contains "(stale)" indicator
	view := m2.usageBar.View()
	if !strings.Contains(view, "(stale)") {
		t.Errorf("stale bar view should contain '(stale)' indicator, got: %q", view)
	}
}

// Story 11.1 Tests: Auto-Detect Auth Refresh

func TestAuthRetryInterval(t *testing.T) {
	// Verify auth retry interval constant
	if authRetryInterval != 5*time.Minute {
		t.Errorf("authRetryInterval = %v, want 5m", authRetryInterval)
	}
}

func TestAppModel_AuthExpired_InitiallyFalse(t *testing.T) {
	// AC-2: authExpired should be false initially
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	if model.authExpired {
		t.Error("authExpired should be false initially")
	}
}

func TestAppModel_AuthExpired_TrueOnTokenExpired(t *testing.T) {
	// Task 5.1: Test authExpired becomes true on ErrTokenExpired
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	newModel, cmd := model.Update(usageFetchedMsg{err: usage.ErrTokenExpired})
	m := newModel.(AppModel)

	if !m.authExpired {
		t.Error("authExpired should be true after ErrTokenExpired")
	}

	// Should schedule auth retry tick
	if cmd == nil {
		t.Error("should return scheduleAuthRetryTick command")
	}
}

func TestAppModel_AuthExpired_FalseOnSuccessfulFetch(t *testing.T) {
	// Task 5.2: Test authExpired becomes false on successful fetch
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.authExpired = true // Simulate expired state

	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}
	newModel, _ := model.Update(usageFetchedMsg{limits: limits, stale: false})
	m := newModel.(AppModel)

	if m.authExpired {
		t.Error("authExpired should be false after successful fetch")
	}
}

func TestAppModel_AuthRetryTickMsg_TriggersRefreshWhenExpired(t *testing.T) {
	// Task 5.3: Test authRetryTickMsg only triggers fetch when authExpired && !refreshInProgress
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.authExpired = true
	model.refreshInProgress = false

	newModel, cmd := model.Update(authRetryTickMsg{})
	m := newModel.(AppModel)

	if !m.refreshInProgress {
		t.Error("refreshInProgress should be true after authRetryTickMsg when expired")
	}
	if cmd == nil {
		t.Error("should return fetchUsage command")
	}
}

func TestAppModel_AuthRetryTickMsg_NoOpWhenNotExpired(t *testing.T) {
	// authRetryTickMsg should be no-op when not expired
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.authExpired = false
	model.refreshInProgress = false

	newModel, cmd := model.Update(authRetryTickMsg{})
	m := newModel.(AppModel)

	if m.refreshInProgress {
		t.Error("refreshInProgress should remain false when not expired")
	}
	if cmd != nil {
		t.Error("should return nil command when not expired")
	}
}

func TestAppModel_AuthRetryTickMsg_NoOpWhenRefreshInProgress(t *testing.T) {
	// authRetryTickMsg should be no-op when refresh already in progress
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.authExpired = true
	model.refreshInProgress = true

	newModel, cmd := model.Update(authRetryTickMsg{})
	m := newModel.(AppModel)

	// refreshInProgress should remain true (unchanged)
	if !m.refreshInProgress {
		t.Error("refreshInProgress should remain true")
	}
	if cmd != nil {
		t.Error("should return nil command when refresh in progress")
	}
}

func TestAppModel_AuthRecovery_TriggersToast(t *testing.T) {
	// Task 5.4: Test recovery from expired state triggers ShowToastMsg
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.authExpired = true // Simulate expired state

	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}
	newModel, cmd := model.Update(usageFetchedMsg{limits: limits, stale: false})
	m := newModel.(AppModel)

	if m.authExpired {
		t.Error("authExpired should be false after recovery")
	}

	// Should return a command that produces ShowToastMsg
	if cmd == nil {
		t.Fatal("recovery should return a command for toast")
	}

	// Execute the command and check the message type
	msg := cmd()
	toastMsg, ok := msg.(ShowToastMsg)
	if !ok {
		t.Errorf("recovery should emit ShowToastMsg, got %T", msg)
	}
	if toastMsg.Message != "Usage limits restored" {
		t.Errorf("toast message = %q, want 'Usage limits restored'", toastMsg.Message)
	}
}

func TestAppModel_AuthRecovery_NoToastWhenNotPreviouslyExpired(t *testing.T) {
	// No toast when auth wasn't previously expired
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.authExpired = false // Not expired

	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}
	newModel, cmd := model.Update(usageFetchedMsg{limits: limits, stale: false})
	m := newModel.(AppModel)

	if m.authExpired {
		t.Error("authExpired should remain false")
	}

	// Should return nil command (no toast)
	if cmd != nil {
		t.Error("should not return toast command when not previously expired")
	}
}

func TestAppModel_NetworkErrorDuringRetry_KeepPolling(t *testing.T) {
	// Task 5.5: Test network errors during retry don't change authExpired state
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.authExpired = true // Expired state

	// Simulate network error during retry
	newModel, cmd := model.Update(usageFetchedMsg{err: usage.ErrAPITimeout})
	m := newModel.(AppModel)

	// authExpired should remain true
	if !m.authExpired {
		t.Error("authExpired should remain true after network error during retry")
	}

	// Should continue polling
	if cmd == nil {
		t.Error("should return scheduleAuthRetryTick command after network error during retry")
	}
}

func TestAppModel_NetworkErrorDuringRetry_OtherErrors(t *testing.T) {
	// Test various network errors during retry all continue polling
	tests := []struct {
		name string
		err  error
	}{
		{"ErrAPIError", usage.ErrAPIError},
		{"ErrAPITimeout", usage.ErrAPITimeout},
		{"generic network error", errors.New("dial tcp: connect: connection refused")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projects := []types.Project{{DisplayName: "proj1"}}
			model := NewAppModel(projects)
			model.authExpired = true

			newModel, cmd := model.Update(usageFetchedMsg{err: tt.err})
			m := newModel.(AppModel)

			if !m.authExpired {
				t.Error("authExpired should remain true")
			}
			if cmd == nil {
				t.Error("should continue polling")
			}
		})
	}
}

func TestAppModel_ManualRefresh_ClearsAuthExpired(t *testing.T) {
	// Task 5.6: Test manual refresh (R key) success clears authExpired
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	// Set up expired state with valid bar state for manual refresh
	model.authExpired = true
	model.usageBar.SetError("Run 'claude' to refresh")
	model.lastRefreshTime = time.Now().Add(-10 * time.Second)

	// Trigger manual refresh
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newModel, cmd := model.Update(keyMsg)
	m := newModel.(AppModel)

	// Should trigger refresh
	if !m.refreshInProgress {
		t.Error("manual refresh should set refreshInProgress")
	}
	if cmd == nil {
		t.Error("should return fetchUsage command")
	}

	// Simulate successful fetch after manual refresh
	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}
	newModel2, _ := m.Update(usageFetchedMsg{limits: limits, stale: false})
	m2 := newModel2.(AppModel)

	// authExpired should be cleared
	if m2.authExpired {
		t.Error("authExpired should be false after successful manual refresh")
	}
}

func TestAppModel_TokenExpiredAgainDuringRetry(t *testing.T) {
	// When already expired and get ErrTokenExpired again, should keep polling
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.authExpired = true // Already expired

	newModel, cmd := model.Update(usageFetchedMsg{err: usage.ErrTokenExpired})
	m := newModel.(AppModel)

	// authExpired should remain true
	if !m.authExpired {
		t.Error("authExpired should remain true")
	}

	// Should schedule another retry
	if cmd == nil {
		t.Error("should return scheduleAuthRetryTick command")
	}
}

func TestScheduleAuthRetryTick_ReturnsCmd(t *testing.T) {
	cmd := scheduleAuthRetryTick()
	if cmd == nil {
		t.Fatal("scheduleAuthRetryTick() should return a non-nil command")
	}
}

func TestAuthRetryTickMsg_Type(t *testing.T) {
	// Verify the message type exists
	msg := authRetryTickMsg{}
	_ = msg // Just verify it compiles and can be created
}

func TestAppModel_CredentialsRemovedDuringRetry(t *testing.T) {
	// Edge case: auth is expired, user logs out (removes credentials), retry fires
	// Should correctly transition to NotLoggedIn state (not keep polling)
	credentialErrors := []error{
		usage.ErrNoCredentials,
		usage.ErrKeychainNotFound,
		usage.ErrKeychainTimeout,
		usage.ErrInvalidCredentials,
		usage.ErrEmptyToken,
	}

	for _, err := range credentialErrors {
		t.Run(err.Error(), func(t *testing.T) {
			projects := []types.Project{{DisplayName: "proj1"}}
			model := NewAppModel(projects)
			model.authExpired = true // Was expired

			newModel, cmd := model.Update(usageFetchedMsg{err: err})
			m := newModel.(AppModel)

			// Should transition to NotLoggedIn state (credential errors take priority)
			if m.UsageBarState() != usage.StateNotLoggedIn {
				t.Errorf("expected StateNotLoggedIn, got %v", m.UsageBarState())
			}

			// Should NOT continue polling (no command returned)
			if cmd != nil {
				t.Error("should not continue polling when credentials are removed")
			}

			// authExpired state doesn't matter anymore - user needs to log in
		})
	}
}

func TestAppModel_AuthExpiredAndRefreshInProgress_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		authExpired         bool
		refreshInProgress   bool
		expectRefreshStart  bool
		expectReturnCommand bool
	}{
		{
			name:                "expired and not refreshing - should start refresh",
			authExpired:         true,
			refreshInProgress:   false,
			expectRefreshStart:  true,
			expectReturnCommand: true,
		},
		{
			name:                "expired but already refreshing - no-op",
			authExpired:         true,
			refreshInProgress:   true,
			expectRefreshStart:  false,
			expectReturnCommand: false,
		},
		{
			name:                "not expired and not refreshing - no-op",
			authExpired:         false,
			refreshInProgress:   false,
			expectRefreshStart:  false,
			expectReturnCommand: false,
		},
		{
			name:                "not expired but refreshing - no-op",
			authExpired:         false,
			refreshInProgress:   true,
			expectRefreshStart:  false,
			expectReturnCommand: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projects := []types.Project{{DisplayName: "proj1"}}
			model := NewAppModel(projects)
			model.authExpired = tt.authExpired
			model.refreshInProgress = tt.refreshInProgress

			newModel, cmd := model.Update(authRetryTickMsg{})
			m := newModel.(AppModel)

			if tt.expectRefreshStart && !m.refreshInProgress {
				t.Error("expected refresh to start")
			}
			if !tt.expectRefreshStart && m.refreshInProgress != tt.refreshInProgress {
				t.Error("refreshInProgress changed unexpectedly")
			}
			if tt.expectReturnCommand && cmd == nil {
				t.Error("expected command")
			}
			if !tt.expectReturnCommand && cmd != nil {
				t.Error("expected no command")
			}
		})
	}
}

// Rate limit handling tests

func TestAppModel_UsageFetchedMsg_RateLimitedWithStaleData(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	staleLimits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
	}
	rateLimitErr := &usage.RateLimitError{RetryAfter: 120 * time.Second}

	newModel, cmd := model.Update(usageFetchedMsg{
		limits: staleLimits,
		stale:  true,
		err:    rateLimitErr,
	})
	m := newModel.(AppModel)

	// Should show stale data
	if m.UsageBarState() != usage.StateStale {
		t.Errorf("UsageBarState = %v, want StateStale", m.UsageBarState())
	}

	// Should schedule retry
	if cmd == nil {
		t.Error("expected scheduleRateLimitRetry command")
	}
}

func TestAppModel_UsageFetchedMsg_RateLimitedWithoutData(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)

	rateLimitErr := &usage.RateLimitError{RetryAfter: 60 * time.Second}

	newModel, cmd := model.Update(usageFetchedMsg{
		err: rateLimitErr,
	})
	m := newModel.(AppModel)

	// Should show error state
	if m.UsageBarState() != usage.StateError {
		t.Errorf("UsageBarState = %v, want StateError", m.UsageBarState())
	}

	// Should schedule retry
	if cmd == nil {
		t.Error("expected scheduleRateLimitRetry command")
	}
}

func TestAppModel_RateLimitRetryMsg_TriggersRefresh(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.refreshInProgress = false

	newModel, cmd := model.Update(rateLimitRetryMsg{})
	m := newModel.(AppModel)

	if !m.refreshInProgress {
		t.Error("refreshInProgress should be true after rateLimitRetryMsg")
	}
	if cmd == nil {
		t.Error("expected fetchUsage command")
	}
}

func TestAppModel_RateLimitRetryMsg_NoOpWhenRefreshing(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.refreshInProgress = true

	_, cmd := model.Update(rateLimitRetryMsg{})

	if cmd != nil {
		t.Error("expected nil command when refreshInProgress is true")
	}
}

func TestAppModel_UsageTickMsg_SkippedDuringRefresh(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewAppModel(projects)
	model.refreshInProgress = true // Simulating active rate-limit retry

	newModel, cmd := model.Update(usageTickMsg{})
	m := newModel.(AppModel)

	// Should still be refreshInProgress (tick didn't start a new refresh)
	if !m.refreshInProgress {
		t.Error("refreshInProgress should remain true")
	}

	// Should reschedule tick even when skipped
	if cmd == nil {
		t.Error("expected reschedule tick command even when skipped")
	}
}
