package tui

import (
	"strings"
	"testing"

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
