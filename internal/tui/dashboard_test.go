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
		wantCmd       bool // Story 5.3: expects batch command for loading
	}{
		{"no projects", nil, 0, false},
		{"empty slice", []types.Project{}, 0, false},
		{"one project", []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}, 1, true},
		{"three projects", []types.Project{
			{DisplayName: "proj1", DirPath: "/tmp/test1"},
			{DisplayName: "proj2", DirPath: "/tmp/test2"},
			{DisplayName: "proj3", DirPath: "/tmp/test3"},
		}, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, cmd := NewDashboardModel(tt.projects)
			if len(model.panes) != tt.wantPaneCount {
				t.Errorf("NewDashboardModel() created %d panes, want %d", len(model.panes), tt.wantPaneCount)
			}
			if model.focusIndex != 0 {
				t.Errorf("NewDashboardModel() focusIndex = %d, want 0", model.focusIndex)
			}
			// Story 5.3: Check command is returned for content loading
			if tt.wantCmd && cmd == nil {
				t.Error("NewDashboardModel() should return command for loading pane content")
			}
			if !tt.wantCmd && cmd != nil {
				t.Error("NewDashboardModel() should return nil command when no projects")
			}
			// Story 5.3: Panes should start in loading state
			for i, pane := range model.panes {
				if !pane.loading {
					t.Errorf("pane[%d].loading = false, want true (initial state)", i)
				}
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
	model, _ := NewDashboardModel(projects)
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
	model, _ := NewDashboardModel(nil)
	// Should not panic
	model.SetSize(100, 50)
	if model.width != 100 || model.height != 50 {
		t.Errorf("SetSize() on empty model should still set dimensions")
	}
}

func TestDashboardModelUpdateEscapeKey(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model, _ := NewDashboardModel(projects)

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
	model, _ := NewDashboardModel(projects)

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
	model, _ := NewDashboardModel(projects)

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
	model, _ := NewDashboardModel(nil)
	view := model.View()
	if view != "" {
		t.Errorf("View() with no panes should return empty string, got %q", view)
	}
}

func TestDashboardModelViewSinglePane(t *testing.T) {
	projects := []types.Project{{DisplayName: "TestProject"}}
	model, _ := NewDashboardModel(projects)
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

	model, _ := NewDashboardModel(projects)
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

// Story 5.3 Tests

func TestTruncateFromTop(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		maxLines int
		want     string
	}{
		{"empty content", "", 5, ""},
		{"content shorter than max", "line1\nline2", 5, "line1\nline2"},
		{"content equal to max", "line1\nline2\nline3", 3, "line1\nline2\nline3"},
		{"content longer than max", "line1\nline2\nline3\nline4\nline5", 3, "line3\nline4\nline5"},
		{"single line", "line1", 5, "line1"},
		{"max zero", "line1\nline2", 0, ""},
		{"max negative", "line1\nline2", -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateFromTop(tt.content, tt.maxLines)
			if got != tt.want {
				t.Errorf("truncateFromTop(%q, %d) = %q, want %q",
					tt.content, tt.maxLines, got, tt.want)
			}
		})
	}
}

func TestRenderPaneEntryUser(t *testing.T) {
	entry := types.LogEntry{
		Type: types.EntryTypeUser,
		Message: types.Message{
			TextContent: "Hello, this is a test message",
		},
	}

	rendered := renderPaneEntry(entry, 40, nil)
	if !strings.Contains(rendered, UserIcon) {
		t.Errorf("renderPaneEntry(user) should contain UserIcon %q, got %q", UserIcon, rendered)
	}
	if !strings.Contains(rendered, "Hello") {
		t.Error("renderPaneEntry(user) should contain message text")
	}
}

func TestRenderPaneEntryAssistant(t *testing.T) {
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{Type: types.ContentTypeText, Text: "This is the assistant response"},
			},
		},
	}

	rendered := renderPaneEntry(entry, 40, nil)
	if !strings.Contains(rendered, AssistantIcon) {
		t.Errorf("renderPaneEntry(assistant) should contain AssistantIcon %q, got %q", AssistantIcon, rendered)
	}
	if !strings.Contains(rendered, "assistant response") {
		t.Error("renderPaneEntry(assistant) should contain message text")
	}
}

func TestRenderPaneEntryTool(t *testing.T) {
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{Type: types.ContentTypeToolUse, ToolName: "Read"},
			},
		},
	}

	rendered := renderPaneEntry(entry, 40, nil)
	if !strings.Contains(rendered, AssistantIcon) {
		t.Errorf("renderPaneEntry(tool) should contain AssistantIcon %q, got %q", AssistantIcon, rendered)
	}
	if !strings.Contains(rendered, "[tool: Read]") {
		t.Errorf("renderPaneEntry(tool) should show tool indicator, got %q", rendered)
	}
}

func TestPaneContentLoadedMsgHandler(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)
	model.SetSize(80, 40)

	// Simulate content loaded message
	entries := []types.LogEntry{
		{Type: types.EntryTypeUser, Message: types.Message{TextContent: "Hello"}},
	}
	msg := paneContentLoadedMsg{
		paneIndex: 0,
		entries:   entries,
		filePath:  "/tmp/test/conv.jsonl",
	}

	newModel, _ := model.Update(msg)
	updatedModel := newModel.(DashboardModel)

	// Check pane state updated
	if updatedModel.panes[0].loading {
		t.Error("pane.loading should be false after content loaded")
	}
	if len(updatedModel.panes[0].entries) != 1 {
		t.Errorf("pane.entries should have 1 entry, got %d", len(updatedModel.panes[0].entries))
	}
}

func TestPaneContentLoadedMsgHandlerError(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Simulate error message
	msg := paneContentLoadedMsg{
		paneIndex: 0,
		err:       fmt.Errorf("test error"),
	}

	newModel, _ := model.Update(msg)
	updatedModel := newModel.(DashboardModel)

	// Check error state
	if updatedModel.panes[0].loading {
		t.Error("pane.loading should be false after error")
	}
	if updatedModel.panes[0].errMsg == "" {
		t.Error("pane.errMsg should be set on error")
	}
}

func TestPaneViewLoadingState(t *testing.T) {
	pane := PaneModel{
		project: types.Project{DisplayName: "Test"},
		width:   30,
		height:  10,
		loading: true,
	}

	view := pane.View()
	if !strings.Contains(view, "Loading...") {
		t.Error("PaneModel.View() should show 'Loading...' when loading=true")
	}
}

func TestPaneViewErrorState(t *testing.T) {
	pane := PaneModel{
		project: types.Project{DisplayName: "Test"},
		width:   30,
		height:  10,
		loading: false,
		errMsg:  "test error",
	}

	view := pane.View()
	if !strings.Contains(view, "Error:") {
		t.Error("PaneModel.View() should show error message when errMsg is set")
	}
}

func TestPaneViewNoConversations(t *testing.T) {
	pane := PaneModel{
		project: types.Project{DisplayName: "Test"},
		width:   30,
		height:  10,
		loading: false,
		entries: []types.LogEntry{}, // Empty entries
	}

	view := pane.View()
	if !strings.Contains(view, "No conversations") {
		t.Error("PaneModel.View() should show 'No conversations' when entries is empty")
	}
}

func TestExtractPaneTextContent(t *testing.T) {
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{Type: types.ContentTypeThinking, Thinking: "thinking..."},
				{Type: types.ContentTypeText, Text: "actual response"},
			},
		},
	}

	text := extractPaneTextContent(entry)
	if text != "actual response" {
		t.Errorf("extractPaneTextContent() = %q, want %q", text, "actual response")
	}
}

func TestCloseAllWatchers(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model, _ := NewDashboardModel(projects)

	// Manually test closeAllWatchers doesn't panic with nil watchers
	model.closeAllWatchers() // Should not panic
}

func TestRenderPaneEntryUserWithCJK(t *testing.T) {
	// Test that CJK characters are properly truncated using visual width
	entry := types.LogEntry{
		Type: types.EntryTypeUser,
		Message: types.Message{
			TextContent: "这是一个很长的中文消息，应该被正确截断而不会破坏字符",
		},
	}

	// Width 20 should truncate properly without corrupting multi-byte chars
	rendered := renderPaneEntry(entry, 20, nil)
	if !strings.Contains(rendered, UserIcon) {
		t.Error("renderPaneEntry should contain UserIcon")
	}
	// Must produce non-empty output (didn't panic)
	if len(rendered) < 5 {
		t.Error("renderPaneEntry should produce meaningful output")
	}
	// Verify truncation happened (original text is longer than width)
	if !strings.Contains(rendered, "...") {
		t.Error("long CJK text should be truncated with ...")
	}
}

func TestRenderPaneEntryUserWithEmoji(t *testing.T) {
	// Test that emoji characters are properly handled
	entry := types.LogEntry{
		Type: types.EntryTypeUser,
		Message: types.Message{
			TextContent: "Hello 👋 this is a message with emoji 🎉",
		},
	}

	rendered := renderPaneEntry(entry, 25, nil)
	if !strings.Contains(rendered, UserIcon) {
		t.Error("renderPaneEntry should contain UserIcon")
	}
	// Should produce valid output without panicking
	if len(rendered) == 0 {
		t.Error("renderPaneEntry should produce non-empty output")
	}
}

func TestWindowSizeMsgUpdatesMarkdownRenderer(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)
	model.SetSize(80, 40)

	// Simulate content loaded with entries
	entries := []types.LogEntry{
		{Type: types.EntryTypeUser, Message: types.Message{TextContent: "Hello"}},
	}
	msg := paneContentLoadedMsg{
		paneIndex: 0,
		entries:   entries,
		filePath:  "/tmp/test/conv.jsonl",
	}
	newModel, _ := model.Update(msg)
	model = newModel.(DashboardModel)

	// Now resize
	newModel, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	updatedModel := newModel.(DashboardModel)

	// Verify dimensions updated
	if updatedModel.width != 120 || updatedModel.height != 60 {
		t.Errorf("dimensions not updated: got (%d, %d), want (120, 60)",
			updatedModel.width, updatedModel.height)
	}

	// Verify markdown renderer was updated (it should exist after resize)
	if updatedModel.panes[0].mdRenderer == nil {
		t.Error("mdRenderer should be initialized after resize with content")
	}

	// Verify content was re-rendered (not empty)
	if updatedModel.panes[0].content == "" && len(updatedModel.panes[0].entries) > 0 {
		t.Error("content should be re-rendered after resize")
	}
}
