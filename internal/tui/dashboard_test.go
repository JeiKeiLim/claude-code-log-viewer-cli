package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestCalculateGrid(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		wantRows int
		wantCols int
	}{
		{"zero projects", 0, 0, 0},
		{"one project", 1, 1, 1},
		{"two projects", 2, 1, 2},
		{"three projects", 3, 1, 3},
		{"four projects", 4, 2, 2},
		{"five projects", 5, 2, 3},
		{"six projects", 6, 2, 3},
		{"seven projects", 7, 3, 3},
		{"eight projects", 8, 3, 3},
		{"nine projects", 9, 3, 3},
		{"ten projects (exceeds max, still 3x3)", 10, 3, 3},
		{"twelve projects (exceeds max, still 3x3)", 12, 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, cols := calculateGrid(tt.count)
			if rows != tt.wantRows || cols != tt.wantCols {
				t.Errorf("calculateGrid(%d) = (%d, %d), want (%d, %d)",
					tt.count, rows, cols, tt.wantRows, tt.wantCols)
			}
		})
	}
}

func TestCalculatePaneDimensions(t *testing.T) {
	tests := []struct {
		name           string
		totalWidth     int
		totalHeight    int
		rows           int
		cols           int
		wantPaneWidth  int
		wantPaneHeight int
	}{
		{"1x1 grid", 100, 50, 1, 1, 100, 50},
		{"1x2 grid", 100, 50, 1, 2, 50, 50},
		{"2x2 grid", 100, 50, 2, 2, 50, 25},
		{"2x3 grid", 120, 60, 2, 3, 40, 30},
		{"3x3 grid", 90, 90, 3, 3, 30, 30},
		{"zero rows", 100, 50, 0, 2, 0, 0},
		{"zero cols", 100, 50, 2, 0, 0, 0},
		{"both zero", 100, 50, 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paneWidth, paneHeight := calculatePaneDimensions(tt.totalWidth, tt.totalHeight, tt.rows, tt.cols)
			if paneWidth != tt.wantPaneWidth || paneHeight != tt.wantPaneHeight {
				t.Errorf("calculatePaneDimensions(%d, %d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.totalWidth, tt.totalHeight, tt.rows, tt.cols,
					paneWidth, paneHeight, tt.wantPaneWidth, tt.wantPaneHeight)
			}
		})
	}
}

func TestNewDashboardModel(t *testing.T) {
	tests := []struct {
		name          string
		projects      []types.Project
		wantPaneCount int
	}{
		{"no projects", nil, 0},
		{"empty slice", []types.Project{}, 0},
		{"one project", []types.Project{{DisplayName: "proj1"}}, 1},
		{"three projects", []types.Project{
			{DisplayName: "proj1"},
			{DisplayName: "proj2"},
			{DisplayName: "proj3"},
		}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewDashboardModel(tt.projects)
			if len(model.panes) != tt.wantPaneCount {
				t.Errorf("NewDashboardModel() created %d panes, want %d", len(model.panes), tt.wantPaneCount)
			}
			if model.focusIndex != 0 {
				t.Errorf("NewDashboardModel() focusIndex = %d, want 0", model.focusIndex)
			}
		})
	}
}

func TestDashboardModelSetSize(t *testing.T) {
	projects := []types.Project{
		{DisplayName: "proj1"},
		{DisplayName: "proj2"},
		{DisplayName: "proj3"},
		{DisplayName: "proj4"},
	}
	model := NewDashboardModel(projects)
	model.SetSize(100, 50)

	if model.width != 100 {
		t.Errorf("SetSize() width = %d, want 100", model.width)
	}
	if model.height != 50 {
		t.Errorf("SetSize() height = %d, want 50", model.height)
	}

	// 4 projects = 2x2 grid, so panes should be 50x25
	for i, pane := range model.panes {
		if pane.width != 50 {
			t.Errorf("pane[%d].width = %d, want 50", i, pane.width)
		}
		if pane.height != 25 {
			t.Errorf("pane[%d].height = %d, want 25", i, pane.height)
		}
	}
}

func TestDashboardModelSetSizeEmpty(t *testing.T) {
	model := NewDashboardModel(nil)
	// Should not panic
	model.SetSize(100, 50)
	if model.width != 100 || model.height != 50 {
		t.Errorf("SetSize() on empty model should still set dimensions")
	}
}

func TestDashboardModelUpdateEscapeKey(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewDashboardModel(projects)

	// Test Escape key
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	_ = newModel // We don't need the model for this test

	if cmd == nil {
		t.Fatal("Update(Escape) should return a command")
	}

	// Execute the command and check the message type
	msg := cmd()
	if _, ok := msg.(GoBackToProjectsFromDashboardMsg); !ok {
		t.Errorf("Update(Escape) command returned %T, want GoBackToProjectsFromDashboardMsg", msg)
	}
}

func TestDashboardModelUpdateQKey(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewDashboardModel(projects)

	// Test q key
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_ = newModel

	if cmd == nil {
		t.Fatal("Update(q) should return a command")
	}

	// Execute the command and check the message type
	msg := cmd()
	if _, ok := msg.(GoBackToProjectsFromDashboardMsg); !ok {
		t.Errorf("Update(q) command returned %T, want GoBackToProjectsFromDashboardMsg", msg)
	}
}

func TestDashboardModelUpdateWindowSizeMsg(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model := NewDashboardModel(projects)

	newModel, cmd := model.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	updatedModel := newModel.(DashboardModel)

	if cmd != nil {
		t.Errorf("Update(WindowSizeMsg) should return nil cmd, got %v", cmd)
	}
	if updatedModel.width != 120 || updatedModel.height != 60 {
		t.Errorf("Update(WindowSizeMsg) dimensions = (%d, %d), want (120, 60)",
			updatedModel.width, updatedModel.height)
	}
}

func TestDashboardModelViewEmpty(t *testing.T) {
	model := NewDashboardModel(nil)
	view := model.View()
	if view != "" {
		t.Errorf("View() with no panes should return empty string, got %q", view)
	}
}

func TestDashboardModelViewSinglePane(t *testing.T) {
	projects := []types.Project{{DisplayName: "TestProject"}}
	model := NewDashboardModel(projects)
	model.SetSize(40, 10)

	view := model.View()
	if view == "" {
		t.Error("View() should return non-empty string for single pane")
	}
	if !strings.Contains(view, "TestProject") {
		t.Error("View() should contain project name")
	}
}

func TestDashboardModelViewIncompleteRow(t *testing.T) {
	// 5 projects in 2x3 grid = incomplete last row (only 2 cells filled instead of 3)
	projects := make([]types.Project, 5)
	for i := range projects {
		projects[i] = types.Project{DisplayName: fmt.Sprintf("P%d", i+1)}
	}

	model := NewDashboardModel(projects)
	model.SetSize(60, 20)

	view := model.View()
	if view == "" {
		t.Error("View() should return non-empty string")
	}
	// Should have 2 rows
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		t.Error("View() should render multiple rows for 5 panes in 2x3 grid")
	}
}

func TestPaneModelViewInvalidDimensions(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"too small width", 3, 10},
		{"too small height", 10, 2},
		{"both too small", 3, 2},
		{"zero width", 0, 10},
		{"zero height", 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pane := PaneModel{
				project: types.Project{DisplayName: "Test"},
				width:   tt.width,
				height:  tt.height,
			}
			view := pane.View()
			if view != "" {
				t.Errorf("PaneModel.View() with invalid dimensions (%d, %d) should return empty string, got %q",
					tt.width, tt.height, view)
			}
		})
	}
}

func TestPaneModelViewTruncatesLongName(t *testing.T) {
	pane := PaneModel{
		project: types.Project{DisplayName: "VeryLongProjectNameThatExceedsPaneWidth"},
		width:   20,
		height:  10,
	}

	view := pane.View()
	// The view should contain "..." indicating truncation
	if !strings.Contains(view, "...") {
		t.Error("PaneModel.View() should truncate long project names with '...'")
	}
}

func TestPaneModelViewValidDimensions(t *testing.T) {
	pane := PaneModel{
		project: types.Project{DisplayName: "Test"},
		width:   20,
		height:  10,
	}

	view := pane.View()
	if view == "" {
		t.Error("PaneModel.View() with valid dimensions should return non-empty string")
	}
	if !strings.Contains(view, "Test") {
		t.Error("PaneModel.View() should contain project name")
	}
}
