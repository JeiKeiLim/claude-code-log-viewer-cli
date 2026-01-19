package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
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
