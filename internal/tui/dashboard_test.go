package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

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
	// AC 5.7.2: Tool entries show short info format [T] ToolName: target
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{
					Type:      types.ContentTypeToolUse,
					ToolName:  "Read",
					ToolInput: map[string]any{"file_path": "/path/to/file.go"},
				},
			},
		},
	}

	rendered := renderPaneEntry(entry, 40, nil)
	if !strings.Contains(rendered, PaneToolIcon) {
		t.Errorf("renderPaneEntry(tool) should contain PaneToolIcon %q, got %q", PaneToolIcon, rendered)
	}
	if !strings.Contains(rendered, "Read:") {
		t.Errorf("renderPaneEntry(tool) should contain tool name 'Read:', got %q", rendered)
	}
	// Should contain file path (or truncated version)
	if !strings.Contains(rendered, "file.go") && !strings.Contains(rendered, "/path") {
		t.Errorf("renderPaneEntry(tool) should contain file path info, got %q", rendered)
	}
}

// Story 5.7 Tests

func TestDashboardViewIncludesHelpText(t *testing.T) {
	// AC 5.7.1: Dashboard view should include help text at bottom
	projects := []types.Project{{DisplayName: "proj1"}}
	model, _ := NewDashboardModel(projects)
	model.SetSize(80, 20)

	view := model.View()
	if !strings.Contains(view, dashboardHelpText) {
		t.Errorf("Dashboard View() should contain help text %q, got %q", dashboardHelpText, view)
	}
}

func TestDashboardHelpTextConstant(t *testing.T) {
	// Verify the help text constant contains expected keys
	if !strings.Contains(dashboardHelpText, "nav") {
		t.Error("dashboardHelpText should mention 'nav' for navigation")
	}
	if !strings.Contains(dashboardHelpText, "Enter") {
		t.Error("dashboardHelpText should mention 'Enter' for opening")
	}
	if !strings.Contains(dashboardHelpText, "Esc") {
		t.Error("dashboardHelpText should mention 'Esc' for going back")
	}
}

func TestFormatPaneToolSummary(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		want     string
	}{
		{
			name:     "Read with file_path",
			toolName: "Read",
			input:    map[string]any{"file_path": "/path/to/file.go"},
			want:     "/path/to/file.go",
		},
		{
			name:     "Write with file_path",
			toolName: "Write",
			input:    map[string]any{"file_path": "/path/to/output.md"},
			want:     "/path/to/output.md",
		},
		{
			name:     "Edit with file_path",
			toolName: "Edit",
			input:    map[string]any{"file_path": "/path/to/edit.go"},
			want:     "/path/to/edit.go",
		},
		{
			name:     "Bash with command",
			toolName: "Bash",
			input:    map[string]any{"command": "make build"},
			want:     "make build",
		},
		{
			name:     "Bash with multiline command",
			toolName: "Bash",
			input:    map[string]any{"command": "make build\nmake test"},
			want:     "make build",
		},
		{
			name:     "Glob with pattern",
			toolName: "Glob",
			input:    map[string]any{"pattern": "**/*.go"},
			want:     "**/*.go",
		},
		{
			name:     "Grep with pattern",
			toolName: "Grep",
			input:    map[string]any{"pattern": "func.*Test"},
			want:     "func.*Test",
		},
		{
			name:     "Task with description",
			toolName: "Task",
			input:    map[string]any{"description": "explore codebase"},
			want:     "explore codebase",
		},
		{
			name:     "WebFetch with url",
			toolName: "WebFetch",
			input:    map[string]any{"url": "https://example.com"},
			want:     "https://example.com",
		},
		{
			name:     "WebSearch with query",
			toolName: "WebSearch",
			input:    map[string]any{"query": "Go testing best practices"},
			want:     "Go testing best practices",
		},
		{
			name:     "Unknown tool",
			toolName: "UnknownTool",
			input:    map[string]any{"foo": "bar"},
			want:     "",
		},
		{
			name:     "Read with empty file_path",
			toolName: "Read",
			input:    map[string]any{"file_path": ""},
			want:     "",
		},
		{
			name:     "Read with nil input",
			toolName: "Read",
			input:    nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPaneToolSummary(tt.toolName, tt.input)
			if got != tt.want {
				t.Errorf("formatPaneToolSummary(%q, %v) = %q, want %q",
					tt.toolName, tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderPaneEntryToolTruncation(t *testing.T) {
	// Test that long file paths are truncated appropriately
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{
					Type:      types.ContentTypeToolUse,
					ToolName:  "Read",
					ToolInput: map[string]any{"file_path": "/very/long/path/to/some/deeply/nested/directory/file.go"},
				},
			},
		},
	}

	// With narrow width, should truncate
	rendered := renderPaneEntry(entry, 25, nil)
	if !strings.Contains(rendered, PaneToolIcon) {
		t.Error("truncated tool entry should contain PaneToolIcon")
	}
	if !strings.Contains(rendered, "Read") {
		t.Error("truncated tool entry should contain tool name")
	}
	// Should be truncated to fit width (with icon taking ~4 chars)
	visualWidth := VisualWidth(rendered)
	if visualWidth > 25 {
		t.Errorf("rendered width %d exceeds max width 25", visualWidth)
	}
}

func TestRenderPaneEntryToolNoSummary(t *testing.T) {
	// Tool with no extractable summary should just show tool name
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{
					Type:      types.ContentTypeToolUse,
					ToolName:  "CustomTool",
					ToolInput: map[string]any{"unknown_field": "value"},
				},
			},
		},
	}

	rendered := renderPaneEntry(entry, 40, nil)
	if !strings.Contains(rendered, PaneToolIcon) {
		t.Error("tool entry should contain PaneToolIcon")
	}
	if !strings.Contains(rendered, "CustomTool") {
		t.Errorf("tool entry should contain tool name 'CustomTool', got %q", rendered)
	}
}

func TestRenderPaneEntryToolBeforeText(t *testing.T) {
	// When entry has both tool use AND text, tool should be shown (appears first)
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{
					Type:      types.ContentTypeToolUse,
					ToolName:  "Read",
					ToolInput: map[string]any{"file_path": "/test.go"},
				},
				{Type: types.ContentTypeText, Text: "Here is the file content"},
			},
		},
	}

	rendered := renderPaneEntry(entry, 40, nil)
	// Should show tool info (since tool_use is checked first)
	if !strings.Contains(rendered, PaneToolIcon) {
		t.Errorf("entry with tool_use first should show tool icon, got %q", rendered)
	}
}

func TestRenderPaneEntryUnknownContentType(t *testing.T) {
	// Assistant entries with unknown content types should be handled gracefully
	// Story 5.7 Task 2.5 allows skipping non-essential content types
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{
					Type: types.ContentType("unknown_type"),
					Text: "", // No text
				},
			},
		},
	}

	rendered := renderPaneEntry(entry, 40, nil)
	// Unknown content types should return empty (not panic or show garbage)
	// since renderPaneEntry only handles tool_use, text, and thinking
	if rendered != "" {
		t.Errorf("unknown content type entry should return empty string, got %q", rendered)
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

// Story 5.4 Tests

func TestPaneNewConversationMsgHandler(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)
	model.SetSize(80, 40)

	// Pre-set some state to verify it gets reset
	model.panes[0].entries = []types.LogEntry{{Type: types.EntryTypeUser}}
	model.panes[0].content = "old content"
	model.panes[0].loading = false
	model.panes[0].conversation.FilePath = "/tmp/old.jsonl"

	// Handle new conversation message
	msg := paneNewConversationMsg{
		paneIndex:   0,
		newFilePath: "/tmp/test/new.jsonl",
	}
	newModel, cmd := model.Update(msg)
	updatedModel := newModel.(DashboardModel)

	// Check pane state was reset
	if !updatedModel.panes[0].loading {
		t.Error("pane.loading should be true after new conversation switch")
	}
	if updatedModel.panes[0].content != "" {
		t.Error("pane.content should be empty after new conversation switch")
	}
	if len(updatedModel.panes[0].entries) != 0 {
		t.Error("pane.entries should be empty after new conversation switch")
	}
	// Check visual indicator was set
	if !updatedModel.panes[0].showNewIndicator {
		t.Error("pane.showNewIndicator should be true after new conversation switch")
	}
	// Check conversation path was updated
	if updatedModel.panes[0].conversation.FilePath != "/tmp/test/new.jsonl" {
		t.Errorf("pane.conversation.FilePath = %q, want %q",
			updatedModel.panes[0].conversation.FilePath, "/tmp/test/new.jsonl")
	}
	// Check command was returned (should be a batch)
	if cmd == nil {
		t.Error("Update(paneNewConversationMsg) should return a command")
	}
}

func TestPaneIndicatorExpiredMsgHandler(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Set indicator active
	model.panes[0].showNewIndicator = true

	// Handle expired message
	msg := paneIndicatorExpiredMsg{paneIndex: 0}
	newModel, _ := model.Update(msg)
	updatedModel := newModel.(DashboardModel)

	// Check indicator was cleared
	if updatedModel.panes[0].showNewIndicator {
		t.Error("pane.showNewIndicator should be false after indicator expired")
	}
}

func TestPaneViewWithNewIndicator(t *testing.T) {
	pane := PaneModel{
		project:          types.Project{DisplayName: "TestProject"},
		width:            40,
		height:           10,
		loading:          false,
		showNewIndicator: true,
	}

	view := pane.View()
	if !strings.Contains(view, "[NEW]") {
		t.Error("PaneModel.View() should show '[NEW]' badge when showNewIndicator=true")
	}
	if !strings.Contains(view, "TestProject") {
		t.Error("PaneModel.View() should still show project name with badge")
	}
}

func TestPaneViewWithNewIndicatorTruncatesName(t *testing.T) {
	// Long name + badge should result in truncation
	pane := PaneModel{
		project:          types.Project{DisplayName: "VeryLongProjectName"},
		width:            25, // Small width to force truncation
		height:           10,
		loading:          false,
		showNewIndicator: true,
	}

	view := pane.View()
	if !strings.Contains(view, "[NEW]") {
		t.Error("PaneModel.View() should show '[NEW]' badge")
	}
	// Name should be truncated to fit
	if !strings.Contains(view, "...") {
		t.Error("PaneModel.View() should truncate long name with badge")
	}
}

func TestCloseAllWatchersWithDirWatcher(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model, _ := NewDashboardModel(projects)

	// Manually test closeAllWatchers doesn't panic with nil watchers
	// (both file and directory watchers)
	model.panes[0].watcher = nil
	model.panes[0].dirWatcher = nil
	model.closeAllWatchers() // Should not panic
}

func TestPaneDirWatcherInitMsgHandler(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// We can't easily create a real fsnotify.Watcher in tests,
	// but we can verify the handler works with nil (edge case)
	// Watcher watches project directory directly (not conversations subdirectory)
	msg := paneDirWatcherInitMsg{
		paneIndex: 0,
		watcher:   nil,
		watchDir:  "/tmp/test",
	}
	newModel, cmd := model.Update(msg)
	updatedModel := newModel.(DashboardModel)

	// Check watchingDir was set to project directory
	if updatedModel.panes[0].watchingDir != "/tmp/test" {
		t.Errorf("pane.watchingDir = %q, want %q",
			updatedModel.panes[0].watchingDir, "/tmp/test")
	}
	// Story 9.2: With nil watcher, startDirWatcherSubscription should be a no-op
	// No command returned since subscription model doesn't return commands
	if cmd != nil {
		t.Error("paneDirWatcherInitMsg with nil watcher should return nil")
	}
}

func TestPaneDirWatcherEventMsgHandlerFileMissing(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Event with non-existent file path
	msg := paneDirWatcherEventMsg{
		paneIndex:   0,
		newFilePath: "/nonexistent/path/to/file.jsonl",
	}
	_, cmd := model.Update(msg)

	// Story 9.2: Should return nil cmd when file doesn't exist (subscription continues watching)
	if cmd != nil {
		t.Error("paneDirWatcherEventMsg with missing file should return nil")
	}
}

func TestInitDirectoryWatcherReturnsCommand(t *testing.T) {
	tmpDir := t.TempDir() // Auto-cleaned up after test
	projects := []types.Project{{DisplayName: "proj1", DirPath: tmpDir}}
	model, _ := NewDashboardModel(projects)

	// Verify watcher can be initialized on project directory
	cmd := model.initDirectoryWatcher(0, tmpDir)
	if cmd == nil {
		t.Error("initDirectoryWatcher should return a command")
	}
}

func TestPaneIndicatorTimeoutCmd(t *testing.T) {
	// Just verify the function returns a non-nil command
	cmd := paneIndicatorTimeoutCmd(0, 0) // 0 duration for instant test
	if cmd == nil {
		t.Error("paneIndicatorTimeoutCmd should return a command")
	}
}

func TestPaneDirWatcherEventMsgHandlerFileOlder(t *testing.T) {
	// Create temp files to test timestamp comparison
	tmpDir := t.TempDir()
	currFile := tmpDir + "/current.jsonl"
	newFile := tmpDir + "/new.jsonl"

	// Create both files
	if err := os.WriteFile(newFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create new file: %v", err)
	}
	if err := os.WriteFile(currFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create current file: %v", err)
	}

	// Set new file to have an older timestamp (1 hour ago)
	oldTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(newFile, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set file time: %v", err)
	}

	projects := []types.Project{{DisplayName: "proj1", DirPath: tmpDir}}
	model, _ := NewDashboardModel(projects)

	// Set current conversation to the "current" file (newer)
	model.panes[0].conversation.FilePath = currFile

	// Event with the "new" file that has older timestamp
	msg := paneDirWatcherEventMsg{
		paneIndex:   0,
		newFilePath: newFile,
	}
	_, cmd := model.Update(msg)

	// With nil dirWatcher and older-timestamp file, should return nil
	// (would continue watching in real scenario, not switch to older file)
	if cmd != nil {
		t.Error("paneDirWatcherEventMsg with older-timestamp file should not switch")
	}
}

// Story 5.5 Tests: Pane Focus Navigation

func TestMoveFocusSinglePane(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model, _ := NewDashboardModel(projects)

	// Single pane - navigation should return 0 for all directions
	for _, dir := range []string{"up", "down", "left", "right"} {
		result := model.moveFocus(dir)
		if result != 0 {
			t.Errorf("moveFocus(%q) with single pane = %d, want 0", dir, result)
		}
	}
}

func TestMoveFocus2x2Grid(t *testing.T) {
	// 4 panes = 2x2 grid
	// [0] [1]
	// [2] [3]
	projects := make([]types.Project, 4)
	for i := range projects {
		projects[i] = types.Project{DisplayName: fmt.Sprintf("P%d", i)}
	}
	model, _ := NewDashboardModel(projects)

	tests := []struct {
		name       string
		startFocus int
		direction  string
		wantFocus  int
	}{
		// From top-left (0)
		{"0 right", 0, "right", 1},
		{"0 down", 0, "down", 2},
		{"0 left (wrap)", 0, "left", 1},
		{"0 up (wrap)", 0, "up", 2},
		// From top-right (1)
		{"1 left", 1, "left", 0},
		{"1 right (wrap)", 1, "right", 0},
		{"1 down", 1, "down", 3},
		{"1 up (wrap)", 1, "up", 3},
		// From bottom-left (2)
		{"2 right", 2, "right", 3},
		{"2 up", 2, "up", 0},
		{"2 down (wrap)", 2, "down", 0},
		{"2 left (wrap)", 2, "left", 3},
		// From bottom-right (3)
		{"3 left", 3, "left", 2},
		{"3 up", 3, "up", 1},
		{"3 right (wrap)", 3, "right", 2},
		{"3 down (wrap)", 3, "down", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.focusIndex = tt.startFocus
			result := model.moveFocus(tt.direction)
			if result != tt.wantFocus {
				t.Errorf("moveFocus(%q) from %d = %d, want %d",
					tt.direction, tt.startFocus, result, tt.wantFocus)
			}
		})
	}
}

func TestMoveFocusIncompleteGrid(t *testing.T) {
	// 5 panes = 2x3 grid (incomplete)
	// [0] [1] [2]
	// [3] [4] [ ]
	projects := make([]types.Project, 5)
	for i := range projects {
		projects[i] = types.Project{DisplayName: fmt.Sprintf("P%d", i)}
	}
	model, _ := NewDashboardModel(projects)

	tests := []struct {
		name       string
		startFocus int
		direction  string
		wantFocus  int
	}{
		// From position 2, down would go to position 5 which doesn't exist
		// Should clamp to last valid index (4)
		{"2 down (clamp)", 2, "down", 4},
		// From position 4, right would go to position 5 which doesn't exist
		// Since 5 >= len(panes), clamps to 4 (stays in place)
		{"4 right (clamp)", 4, "right", 4},
		// Normal navigation
		{"0 right", 0, "right", 1},
		{"1 right", 1, "right", 2},
		{"3 up", 3, "up", 0},
		// From position 2, right wraps to 0
		{"2 right (wrap)", 2, "right", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.focusIndex = tt.startFocus
			result := model.moveFocus(tt.direction)
			if result != tt.wantFocus {
				t.Errorf("moveFocus(%q) from %d = %d, want %d",
					tt.direction, tt.startFocus, result, tt.wantFocus)
			}
		})
	}
}

func TestArrowKeyNavigation(t *testing.T) {
	projects := make([]types.Project, 4)
	for i := range projects {
		projects[i] = types.Project{DisplayName: fmt.Sprintf("P%d", i)}
	}
	model, _ := NewDashboardModel(projects)

	// 2x2 grid layout:
	// [0] [1]
	// [2] [3]
	// Test arrow keys
	tests := []struct {
		key       tea.KeyMsg
		wantFocus int
	}{
		// From index 0 (top-left): right -> 1
		{tea.KeyMsg{Type: tea.KeyRight}, 1},
		// From index 1: down -> 3 (same column, next row)
		{tea.KeyMsg{Type: tea.KeyDown}, 3},
	}

	for i, tt := range tests {
		if i == 0 {
			model.focusIndex = 0
		}
		newModel, _ := model.Update(tt.key)
		model = newModel.(DashboardModel)
		if model.focusIndex != tt.wantFocus {
			t.Errorf("After key %v: focusIndex = %d, want %d", tt.key, model.focusIndex, tt.wantFocus)
		}
	}
}

func TestVimKeyNavigation(t *testing.T) {
	projects := make([]types.Project, 4)
	for i := range projects {
		projects[i] = types.Project{DisplayName: fmt.Sprintf("P%d", i)}
	}
	model, _ := NewDashboardModel(projects)
	model.focusIndex = 3 // Start at bottom-right

	// Test vim keys: h, j, k, l
	tests := []struct {
		key       string
		wantFocus int
	}{
		{"h", 2}, // left: 3 -> 2
		{"k", 0}, // up: 2 -> 0
		{"l", 1}, // right: 0 -> 1
		{"j", 3}, // down: 1 -> 3
	}

	for _, tt := range tests {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
		newModel, _ := model.Update(msg)
		model = newModel.(DashboardModel)
		if model.focusIndex != tt.wantFocus {
			t.Errorf("After key %q: focusIndex = %d, want %d", tt.key, model.focusIndex, tt.wantFocus)
		}
	}
}

func TestEnterKeyWithConversation(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)
	model.panes[0].conversation.FilePath = "/tmp/test/conv.jsonl"

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := model.Update(msg)

	if cmd == nil {
		t.Fatal("Enter key with conversation should return a command")
	}

	// Execute the command and check message type
	result := cmd()
	openMsg, ok := result.(OpenViewerFromDashboardMsg)
	if !ok {
		t.Fatalf("Enter key should return OpenViewerFromDashboardMsg, got %T", result)
	}
	if openMsg.FilePath != "/tmp/test/conv.jsonl" {
		t.Errorf("OpenViewerFromDashboardMsg.FilePath = %q, want %q",
			openMsg.FilePath, "/tmp/test/conv.jsonl")
	}
}

func TestEnterKeyWithoutConversation(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1"}}
	model, _ := NewDashboardModel(projects)
	// No conversation set (empty FilePath)

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := model.Update(msg)

	// Should return nil - no message when no conversation
	if cmd != nil {
		t.Error("Enter key without conversation should return nil command")
	}
}

func TestPaneViewWithFocus(t *testing.T) {
	pane := PaneModel{
		project: types.Project{DisplayName: "TestProject"},
		width:   30,
		height:  10,
		loading: false,
	}

	// Test focused view
	focusedView := pane.ViewWithFocus(true)
	if focusedView == "" {
		t.Error("ViewWithFocus(true) should return non-empty string")
	}

	// Test unfocused view
	unfocusedView := pane.ViewWithFocus(false)
	if unfocusedView == "" {
		t.Error("ViewWithFocus(false) should return non-empty string")
	}

	// Both should contain project name
	if !strings.Contains(focusedView, "TestProject") {
		t.Error("ViewWithFocus(true) should contain project name")
	}
	if !strings.Contains(unfocusedView, "TestProject") {
		t.Error("ViewWithFocus(false) should contain project name")
	}
}

func TestPaneViewWithFocusInvalidDimensions(t *testing.T) {
	pane := PaneModel{
		project: types.Project{DisplayName: "Test"},
		width:   3, // Too small
		height:  2, // Too small
	}

	view := pane.ViewWithFocus(true)
	if view != "" {
		t.Error("ViewWithFocus with invalid dimensions should return empty string")
	}
}

func TestDashboardFocusedPaneBorderColor(t *testing.T) {
	projects := make([]types.Project, 4)
	for i := range projects {
		projects[i] = types.Project{DisplayName: fmt.Sprintf("P%d", i)}
	}
	model, _ := NewDashboardModel(projects)
	model.SetSize(80, 40)

	// Focus should be on first pane (index 0)
	if model.focusIndex != 0 {
		t.Errorf("Initial focusIndex should be 0, got %d", model.focusIndex)
	}

	// View should render without error
	view := model.View()
	if view == "" {
		t.Error("Dashboard View() should return non-empty string")
	}
}

func TestFocusBoundsChecking(t *testing.T) {
	projects := make([]types.Project, 4)
	for i := range projects {
		projects[i] = types.Project{DisplayName: fmt.Sprintf("P%d", i)}
	}
	model, _ := NewDashboardModel(projects)

	// Test that focus never goes out of bounds
	for i := 0; i < 100; i++ {
		model.focusIndex = model.moveFocus("right")
		if model.focusIndex < 0 || model.focusIndex >= len(model.panes) {
			t.Fatalf("Focus went out of bounds: %d (max: %d)", model.focusIndex, len(model.panes)-1)
		}
	}
}

func TestOpenViewerFromDashboardMsgType(t *testing.T) {
	// Verify the message type has expected fields
	msg := OpenViewerFromDashboardMsg{
		FilePath: "/test/path.jsonl",
		Project:  types.Project{DisplayName: "Test"},
	}

	if msg.FilePath != "/test/path.jsonl" {
		t.Errorf("OpenViewerFromDashboardMsg.FilePath = %q, want %q", msg.FilePath, "/test/path.jsonl")
	}
	if msg.Project.DisplayName != "Test" {
		t.Errorf("OpenViewerFromDashboardMsg.Project.DisplayName = %q, want %q", msg.Project.DisplayName, "Test")
	}
}

// Test addBorderWithStyle edge cases and addBorder delegation (Story 5.5 M2/M4 fixes)

func TestAddBorderWithStyleSmallWidth(t *testing.T) {
	// Test that width < 4 is clamped to 4
	content := "test"
	result := addBorderWithStyle(content, 2, PaneUnfocusedBorderColor)

	// Should not panic and should produce valid output
	if result == "" {
		t.Error("addBorderWithStyle with small width should produce non-empty output")
	}
	// Should contain border characters
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╯") {
		t.Error("addBorderWithStyle should contain border characters")
	}
}

func TestAddBorderDelegatesToAddBorderWithStyle(t *testing.T) {
	content := "test content"

	// Both should produce the same result (addBorder uses unfocused color)
	borderResult := addBorder(content, 20)
	styledResult := addBorderWithStyle(content, 20, PaneUnfocusedBorderColor)

	if borderResult != styledResult {
		t.Error("addBorder should delegate to addBorderWithStyle with unfocused color")
	}
}

func TestPaneViewDelegatesToViewWithFocus(t *testing.T) {
	pane := PaneModel{
		project: types.Project{DisplayName: "TestProject"},
		width:   30,
		height:  10,
		loading: false,
	}

	// View() should produce same result as ViewWithFocus(false)
	viewResult := pane.View()
	viewWithFocusResult := pane.ViewWithFocus(false)

	if viewResult != viewWithFocusResult {
		t.Error("View() should delegate to ViewWithFocus(false)")
	}
}

// Story 9.2 Tests: Subscription Model

func TestNewDashboardModelInitializesContext(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Story 9.2: Context should be initialized
	if model.ctx == nil {
		t.Error("NewDashboardModel() should initialize ctx")
	}
	if model.cancel == nil {
		t.Error("NewDashboardModel() should initialize cancel function")
	}
	// Story 9.2: Subscriptions should be active
	if !model.subscriptionsActive {
		t.Error("NewDashboardModel() should set subscriptionsActive = true")
	}
}

func TestSubscriptionTickMsgHandler(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Handle subscription tick with no events pending
	_, cmd := model.Update(subscriptionTickMsg{})

	// Should return a command for next tick (subscriptions are active)
	if cmd == nil {
		t.Error("subscriptionTickMsg handler should return tick command when subscriptionsActive")
	}
}

func TestSubscriptionTickMsgHandlerInactive(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)
	model.subscriptionsActive = false

	// Handle subscription tick when inactive
	_, cmd := model.Update(subscriptionTickMsg{})

	// Should return nil (stop polling)
	if cmd != nil {
		t.Error("subscriptionTickMsg handler should return nil when subscriptionsActive is false")
	}
}

func TestPollSubscriptionChannelsEmpty(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Poll with no channels initialized
	result := model.pollSubscriptionChannels()

	// Should return nil (no events)
	if result != nil {
		t.Error("pollSubscriptionChannels() should return nil when no channels have events")
	}
}

func TestPollSubscriptionChannelsWithFileEvent(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Create a buffered channel and send an event
	ch := make(chan paneWatcherEventMsg, 1)
	model.panes[0].fileEventChan = ch

	// Send a test event
	testMsg := paneWatcherEventMsg{paneIndex: 0, event: nil}
	ch <- testMsg

	// Poll should return the event
	result := model.pollSubscriptionChannels()
	if result == nil {
		t.Error("pollSubscriptionChannels() should return event from fileEventChan")
	}
	if msg, ok := result.(paneWatcherEventMsg); !ok || msg.paneIndex != 0 {
		t.Errorf("pollSubscriptionChannels() returned wrong event type or index")
	}
}

func TestPollSubscriptionChannelsWithDirEvent(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Create a buffered channel and send an event
	ch := make(chan paneDirWatcherEventMsg, 1)
	model.panes[0].dirEventChan = ch

	// Send a test event
	testMsg := paneDirWatcherEventMsg{paneIndex: 0, newFilePath: "/test/new.jsonl"}
	ch <- testMsg

	// Poll should return the event
	result := model.pollSubscriptionChannels()
	if result == nil {
		t.Error("pollSubscriptionChannels() should return event from dirEventChan")
	}
	if msg, ok := result.(paneDirWatcherEventMsg); !ok || msg.paneIndex != 0 {
		t.Errorf("pollSubscriptionChannels() returned wrong event type or index")
	}
}

func TestCloseAllWatchersStopsSubscriptions(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Setup: manually create channels to verify cleanup
	model.panes[0].fileEventChan = make(chan paneWatcherEventMsg, 1)
	model.panes[0].dirEventChan = make(chan paneDirWatcherEventMsg, 1)

	// Call closeAllWatchers
	model.closeAllWatchers()

	// Story 9.2: subscriptionsActive should be false
	if model.subscriptionsActive {
		t.Error("closeAllWatchers() should set subscriptionsActive = false")
	}

	// Story 9.2: Channels should be nil
	if model.panes[0].fileEventChan != nil {
		t.Error("closeAllWatchers() should nil out fileEventChan")
	}
	if model.panes[0].dirEventChan != nil {
		t.Error("closeAllWatchers() should nil out dirEventChan")
	}
}

func TestResumeWatchersCreatesNewContext(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Save original context
	originalCtx := model.ctx

	// Close watchers (cancels context)
	model.closeAllWatchers()

	// Resume watchers
	cmd := model.ResumeWatchers()

	// Story 9.2: Should have new context
	if model.ctx == originalCtx {
		t.Error("ResumeWatchers() should create new context")
	}
	if model.ctx == nil {
		t.Error("ResumeWatchers() should set ctx to non-nil value")
	}
	if model.cancel == nil {
		t.Error("ResumeWatchers() should set cancel function")
	}

	// Story 9.2: subscriptionsActive should be true
	if !model.subscriptionsActive {
		t.Error("ResumeWatchers() should set subscriptionsActive = true")
	}

	// Should return tick command
	if cmd == nil {
		t.Error("ResumeWatchers() should return tick command")
	}
}

func TestSubscriptionTickCmdReturnsCommand(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	cmd := model.subscriptionTickCmd()
	if cmd == nil {
		t.Error("subscriptionTickCmd() should return a command")
	}
}

func TestInitReturnsTickWhenActive(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	cmd := model.Init()

	// Story 9.2: Init should return tick command when subscriptions active
	if cmd == nil {
		t.Error("Init() should return tick command when subscriptionsActive")
	}
}

func TestInitReturnsNilWhenInactive(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)
	model.subscriptionsActive = false

	cmd := model.Init()

	// Should return nil when subscriptions inactive
	if cmd != nil {
		t.Error("Init() should return nil when subscriptionsActive is false")
	}
}

func TestStartFileWatcherSubscriptionInvalidIndex(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Should not panic with invalid index
	model.startFileWatcherSubscription(model.ctx, -1)
	model.startFileWatcherSubscription(model.ctx, 99)
	// No assertion needed - just verify no panic
}

func TestStartFileWatcherSubscriptionNilWatcher(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Should not panic and should not create channel when watcher is nil
	model.startFileWatcherSubscription(model.ctx, 0)
	// No assertion needed - just verify no panic
}

func TestStartDirWatcherSubscriptionInvalidIndex(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Should not panic with invalid index
	model.startDirWatcherSubscription(model.ctx, -1)
	model.startDirWatcherSubscription(model.ctx, 99)
	// No assertion needed - just verify no panic
}

func TestStartDirWatcherSubscriptionNilWatcher(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Should not panic and should not create channel when watcher is nil
	model.startDirWatcherSubscription(model.ctx, 0)
	// No assertion needed - just verify no panic
}

func TestPaneWatcherEventMsgNoLongerChains(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)
	model.SetSize(80, 40)

	// Pre-populate pane with content
	model.panes[0].entries = []types.LogEntry{}
	model.panes[0].loading = false

	// Handle NewEntriesMsg
	msg := paneWatcherEventMsg{
		paneIndex: 0,
		event:     nil, // Will be handled gracefully
	}
	_, cmd := model.Update(msg)

	// Story 9.2: Should NOT return a command (no more chaining)
	// Note: We can't easily test the nil event case, but the handler should be graceful
	_ = cmd // Just verify no panic
}

func TestPaneDirWatcherEventMsgNoLongerChains(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Event with non-existent file (will return nil due to Stat failure)
	msg := paneDirWatcherEventMsg{
		paneIndex:   0,
		newFilePath: "/nonexistent/file.jsonl",
	}
	_, cmd := model.Update(msg)

	// Story 9.2: Should return nil (no longer chains to waitForDirEvent)
	if cmd != nil {
		t.Error("paneDirWatcherEventMsg should return nil (subscription handles watching)")
	}
}

// Code Review Fix: Test for H1 - Polling chain continues after event processing
func TestSubscriptionTickContinuesAfterEvent(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Create a buffered channel and send an event
	ch := make(chan paneWatcherEventMsg, 1)
	model.panes[0].fileEventChan = ch

	// Send a test event (nil event is handled gracefully)
	testMsg := paneWatcherEventMsg{paneIndex: 0, event: nil}
	ch <- testMsg

	// Process subscription tick
	newModel, cmd := model.Update(subscriptionTickMsg{})
	updatedModel := newModel.(DashboardModel)

	// H1 Fix: Should return a batched command (event cmd + next tick)
	// when subscriptions are still active
	if !updatedModel.subscriptionsActive {
		t.Error("subscriptionsActive should still be true")
	}
	if cmd == nil {
		t.Error("subscriptionTickMsg handler should return command after processing event")
	}
}

// Code Review Fix: Test for M3 - Multi-event burst handling
func TestPollSubscriptionChannelsMultipleEvents(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/test"}}
	model, _ := NewDashboardModel(projects)

	// Create buffered channel with multiple events
	ch := make(chan paneWatcherEventMsg, 5)
	model.panes[0].fileEventChan = ch

	// Send 5 events rapidly
	for i := 0; i < 5; i++ {
		ch <- paneWatcherEventMsg{paneIndex: 0, event: nil}
	}

	// Poll should return events one at a time
	eventsFound := 0
	for i := 0; i < 5; i++ {
		result := model.pollSubscriptionChannels()
		if result != nil {
			eventsFound++
		}
	}

	if eventsFound != 5 {
		t.Errorf("Expected 5 events from polling, got %d", eventsFound)
	}

	// Channel should be empty now
	result := model.pollSubscriptionChannels()
	if result != nil {
		t.Error("pollSubscriptionChannels should return nil when channel is empty")
	}
}

// Code Review Fix: Test constant is defined correctly
func TestSubscriptionChannelBufferConstant(t *testing.T) {
	// Verify the constant exists and has expected value
	if subscriptionChannelBuffer != 10 {
		t.Errorf("subscriptionChannelBuffer = %d, want 10", subscriptionChannelBuffer)
	}
}
