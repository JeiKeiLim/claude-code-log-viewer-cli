// Package tui provides the terminal user interface components.
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// createTestProjects creates a slice of test projects.
func createTestProjects(count int) []types.Project {
	projects := make([]types.Project, count)
	for i := 0; i < count; i++ {
		projects[i] = types.Project{
			DirPath:           "/test/path",
			DisplayName:       "test-project-" + string(rune('a'+i)),
			DecodedPath:       "/decoded/path",
			ConversationCount: i + 1,
		}
	}
	return projects
}

func TestProjectModel_ToggleSelection(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50) // Initialize size

	// Test adding selection
	if !model.ToggleSelection(0) {
		t.Error("ToggleSelection should return true for first selection")
	}
	if model.SelectedCount() != 1 {
		t.Errorf("SelectedCount = %d, want 1", model.SelectedCount())
	}
	if !model.selectionMode {
		t.Error("selectionMode should be true after selection")
	}

	// Test removing selection
	if !model.ToggleSelection(0) {
		t.Error("ToggleSelection should return true for deselection")
	}
	if model.SelectedCount() != 0 {
		t.Errorf("SelectedCount = %d, want 0", model.SelectedCount())
	}
	if model.selectionMode {
		t.Error("selectionMode should be false after deselection")
	}
}

func TestProjectModel_SelectedCount(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	if model.SelectedCount() != 0 {
		t.Errorf("Initial SelectedCount = %d, want 0", model.SelectedCount())
	}

	model.ToggleSelection(0)
	model.ToggleSelection(2)
	model.ToggleSelection(4)

	if model.SelectedCount() != 3 {
		t.Errorf("SelectedCount = %d, want 3", model.SelectedCount())
	}
}

func TestProjectModel_SelectedProjects(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	model.ToggleSelection(1)
	model.ToggleSelection(3)

	selected := model.SelectedProjects()
	if len(selected) != 2 {
		t.Fatalf("len(SelectedProjects) = %d, want 2", len(selected))
	}

	// Should be in index order
	if selected[0].DisplayName != "test-project-b" {
		t.Errorf("First selected = %s, want test-project-b", selected[0].DisplayName)
	}
	if selected[1].DisplayName != "test-project-d" {
		t.Errorf("Second selected = %s, want test-project-d", selected[1].DisplayName)
	}
}

func TestProjectModel_MaxSelectionLimit(t *testing.T) {
	projects := createTestProjects(12)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	// Select 9 projects (the max)
	for i := 0; i < 9; i++ {
		if !model.ToggleSelection(i) {
			t.Errorf("Selection %d should succeed", i)
		}
	}

	if model.SelectedCount() != 9 {
		t.Errorf("SelectedCount = %d, want 9", model.SelectedCount())
	}

	// Try to select 10th - should fail
	if model.ToggleSelection(9) {
		t.Error("10th selection should be blocked")
	}

	if model.SelectedCount() != 9 {
		t.Errorf("SelectedCount after blocked = %d, want 9", model.SelectedCount())
	}

	// Deselecting should still work
	if !model.ToggleSelection(0) {
		t.Error("Deselection should still work at limit")
	}
	if model.SelectedCount() != 8 {
		t.Errorf("SelectedCount after deselect = %d, want 8", model.SelectedCount())
	}
}

func TestProjectModel_ClearSelections(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	model.ToggleSelection(0)
	model.ToggleSelection(1)
	model.ToggleSelection(2)

	if model.SelectedCount() != 3 {
		t.Errorf("SelectedCount before clear = %d, want 3", model.SelectedCount())
	}
	if !model.selectionMode {
		t.Error("selectionMode should be true before clear")
	}

	model.ClearSelections()

	if model.SelectedCount() != 0 {
		t.Errorf("SelectedCount after clear = %d, want 0", model.SelectedCount())
	}
	if model.selectionMode {
		t.Error("selectionMode should be false after clear")
	}
}

func TestProjectModel_SpaceKeyToggle(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	// Simulate Space key press
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = newModel.(ProjectModel)

	if model.SelectedCount() != 1 {
		t.Errorf("After Space key, SelectedCount = %d, want 1", model.SelectedCount())
	}
	if cmd != nil {
		t.Error("Space key toggle should not emit command on success")
	}

	// Toggle off
	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = newModel.(ProjectModel)

	if model.SelectedCount() != 0 {
		t.Errorf("After second Space key, SelectedCount = %d, want 0", model.SelectedCount())
	}
}

func TestProjectModel_EnterWithSelections(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	// Select some projects
	model.ToggleSelection(0)
	model.ToggleSelection(2)
	model.updateItemsWithSelection()

	// Press Enter
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should emit DashboardSelectedMsg
	if cmd == nil {
		t.Fatal("Enter with selections should emit command")
	}

	msg := cmd()
	dashMsg, ok := msg.(DashboardSelectedMsg)
	if !ok {
		t.Fatalf("Expected DashboardSelectedMsg, got %T", msg)
	}

	if len(dashMsg.Projects) != 2 {
		t.Errorf("DashboardSelectedMsg.Projects len = %d, want 2", len(dashMsg.Projects))
	}
}

func TestProjectModel_EnterWithoutSelections(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	// Press Enter without selecting
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Enter should emit command")
	}

	msg := cmd()
	_, ok := msg.(ProjectSelectedMsg)
	if !ok {
		t.Fatalf("Expected ProjectSelectedMsg, got %T", msg)
	}
}

func TestProjectModel_EscapeClearsSelections(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	// Select some projects
	model.ToggleSelection(0)
	model.ToggleSelection(1)
	model.updateItemsWithSelection()

	if model.SelectedCount() != 2 {
		t.Errorf("Before Escape, SelectedCount = %d, want 2", model.SelectedCount())
	}

	// Press Escape
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = newModel.(ProjectModel)

	if model.SelectedCount() != 0 {
		t.Errorf("After Escape, SelectedCount = %d, want 0", model.SelectedCount())
	}
	if model.selectionMode {
		t.Error("After Escape, selectionMode should be false")
	}
}

func TestProjectModel_EscapeDoesNothingWithoutSelections(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	// Press Escape without selections
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = newModel.(ProjectModel)

	// Should not emit any command and state unchanged
	if cmd != nil {
		t.Error("Escape without selections should not emit command")
	}
	if model.selectionMode {
		t.Error("selectionMode should remain false")
	}
}

func TestProjectModel_FilterPreservesSelections(t *testing.T) {
	projects := createTestProjects(5)
	model := NewProjectModel(projects)
	model.SetSize(100, 50)

	// Select projects at indices 0 and 2
	model.ToggleSelection(0)
	model.ToggleSelection(2)
	model.updateItemsWithSelection()

	if model.SelectedCount() != 2 {
		t.Errorf("Before filter, SelectedCount = %d, want 2", model.SelectedCount())
	}

	// Apply filter (simulating typing)
	model.applyFilter("test-project-a")

	// Verify selection state is preserved in model
	if model.SelectedCount() != 2 {
		t.Errorf("After filter, SelectedCount = %d, want 2", model.SelectedCount())
	}

	// Reset filter
	model.resetFilter()

	// Verify selections still intact after reset
	if model.SelectedCount() != 2 {
		t.Errorf("After reset, SelectedCount = %d, want 2", model.SelectedCount())
	}
	if !model.selectionMode {
		t.Error("selectionMode should remain true after filter cycle")
	}

	// Verify the correct projects are still selected
	selected := model.SelectedProjects()
	if len(selected) != 2 {
		t.Fatalf("len(SelectedProjects) = %d, want 2", len(selected))
	}
}
