// Package tui provides the terminal user interface components.
package tui

import (
	"strings"
	"testing"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestDefaultRenderOptions(t *testing.T) {
	opts := DefaultRenderOptions()

	if opts.HideThoughts != false {
		t.Errorf("DefaultRenderOptions().HideThoughts = %v, want false", opts.HideThoughts)
	}
	if opts.HideTools != false {
		t.Errorf("DefaultRenderOptions().HideTools = %v, want false", opts.HideTools)
	}
	if opts.Width != 0 {
		t.Errorf("DefaultRenderOptions().Width = %v, want 0", opts.Width)
	}
	if opts.WatchMode != false {
		t.Errorf("DefaultRenderOptions().WatchMode = %v, want false", opts.WatchMode)
	}
}

func TestBuildModeSegment(t *testing.T) {
	tests := []struct {
		name      string
		watchMode bool
		want      string
	}{
		{
			name:      "watch mode disabled returns empty",
			watchMode: false,
			want:      "",
		},
		{
			name:      "watch mode enabled returns LIVE",
			watchMode: true,
			want:      "LIVE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{watchMode: tt.watchMode}
			got := m.buildModeSegment()

			if tt.want == "" && got != "" {
				t.Errorf("buildModeSegment() = %q, want empty string", got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("buildModeSegment() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestNewViewerModelWatchMode(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}

	tests := []struct {
		name      string
		watchMode bool
	}{
		{"watch mode disabled", false},
		{"watch mode enabled", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := RenderOptions{WatchMode: tt.watchMode}
			m := NewViewerModel(entries, 0, "Test", opts)

			if m.watchMode != tt.watchMode {
				t.Errorf("NewViewerModel() watchMode = %v, want %v", m.watchMode, tt.watchMode)
			}
		})
	}
}

func TestBuildPositionSegment(t *testing.T) {
	tests := []struct {
		name    string
		entries int
		want    string
	}{
		{
			name:    "empty entries shows 0/0",
			entries: 0,
			want:    "0/0",
		},
		{
			name:    "single entry shows 1/1",
			entries: 1,
			want:    "1/1",
		},
		{
			name:    "multiple entries shows Entry format",
			entries: 42,
			want:    "/42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := make([]types.LogEntry, tt.entries)
			m := NewViewerModel(entries, 0, "Test", DefaultRenderOptions())

			got := m.buildPositionSegment()
			if !strings.Contains(got, tt.want) {
				t.Errorf("buildPositionSegment() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestBuildShortcutsSegment(t *testing.T) {
	tests := []struct {
		name          string
		searchMatches []int
		canGoBack     bool
		wantContains  []string
	}{
		{
			name:          "basic shortcuts always present",
			searchMatches: nil,
			canGoBack:     false,
			wantContains:  []string{"j/k:scroll", "q:quit", "t:thinking", "i:inputs"},
		},
		{
			name:          "search navigation when matches exist",
			searchMatches: []int{1, 2, 3},
			canGoBack:     false,
			wantContains:  []string{"n/N:next/prev"},
		},
		{
			name:          "back navigation when canGoBack true",
			searchMatches: nil,
			canGoBack:     true,
			wantContains:  []string{"h/esc:back"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{
				searchMatches: tt.searchMatches,
				canGoBack:     tt.canGoBack,
			}

			got := m.buildShortcutsSegment()
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("buildShortcutsSegment() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestRenderPlainWithOptions(t *testing.T) {
	// Create test entries with thinking and tool blocks
	entries := []types.LogEntry{
		{
			Type: types.EntryTypeUser,
			Message: types.Message{
				TextContent: "Hello",
			},
		},
		{
			Type: types.EntryTypeAssistant,
			Message: types.Message{
				Content: []types.MessageContent{
					{Type: types.ContentTypeText, Text: "Response text"},
					{Type: types.ContentTypeThinking, Thinking: "Some thinking"},
					{Type: types.ContentTypeToolUse, ToolName: "Read", ToolInput: map[string]any{"file_path": "/test.go"}},
				},
			},
		},
	}

	tests := []struct {
		name         string
		opts         RenderOptions
		wantThinking bool
		wantTool     bool
	}{
		{
			name:         "default shows all",
			opts:         DefaultRenderOptions(),
			wantThinking: true,
			wantTool:     true,
		},
		{
			name:         "hide thoughts only",
			opts:         RenderOptions{HideThoughts: true, HideTools: false},
			wantThinking: false,
			wantTool:     true,
		},
		{
			name:         "hide tools only",
			opts:         RenderOptions{HideThoughts: false, HideTools: true},
			wantThinking: true,
			wantTool:     false,
		},
		{
			name:         "hide both",
			opts:         RenderOptions{HideThoughts: true, HideTools: true},
			wantThinking: false,
			wantTool:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderPlain(entries, "test", tt.opts)

			hasThinking := strings.Contains(output, "Thinking")
			hasTool := strings.Contains(output, "Tool")

			if hasThinking != tt.wantThinking {
				t.Errorf("RenderPlain() thinking=%v, want %v", hasThinking, tt.wantThinking)
			}
			if hasTool != tt.wantTool {
				t.Errorf("RenderPlain() tool=%v, want %v", hasTool, tt.wantTool)
			}
		})
	}
}

func TestFormatToolSummary(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		want     string
	}{
		{
			name:     "Read full file",
			toolName: "Read",
			input:    map[string]any{"file_path": "/path/to/file.go"},
			want:     "Read: file.go (full file)",
		},
		{
			name:     "Read with offset",
			toolName: "Read",
			input:    map[string]any{"file_path": "/path/to/file.go", "offset": float64(10), "limit": float64(50)},
			want:     "Read: file.go (lines 10-60)",
		},
		{
			name:     "Read with offset only (default limit)",
			toolName: "Read",
			input:    map[string]any{"file_path": "/path/to/file.go", "offset": float64(100)},
			want:     "Read: file.go (lines 100-200)",
		},
		{
			name:     "Read empty path",
			toolName: "Read",
			input:    map[string]any{},
			want:     "Read: [collapsed]",
		},
		{
			name:     "Edit with changes",
			toolName: "Edit",
			input:    map[string]any{"file_path": "/path/to/file.go", "old_string": "old", "new_string": "new\nline"},
			want:     "Edit: file.go (+2/-1 lines)",
		},
		{
			name:     "Edit empty strings",
			toolName: "Edit",
			input:    map[string]any{"file_path": "/path/to/file.go", "old_string": "", "new_string": ""},
			want:     "Edit: file.go (+0/-0 lines)",
		},
		{
			name:     "Glob pattern",
			toolName: "Glob",
			input:    map[string]any{"pattern": "**/*.go"},
			want:     "Glob: **/*.go",
		},
		{
			name:     "Grep with path",
			toolName: "Grep",
			input:    map[string]any{"pattern": "TODO", "path": "src/"},
			want:     "Grep: \"TODO\" in src/",
		},
		{
			name:     "Grep without path",
			toolName: "Grep",
			input:    map[string]any{"pattern": "TODO"},
			want:     "Grep: \"TODO\" in ./",
		},
		{
			name:     "Write file",
			toolName: "Write",
			input:    map[string]any{"file_path": "/path/to/output.txt"},
			want:     "Write: output.txt",
		},
		{
			name:     "Bash short command",
			toolName: "Bash",
			input:    map[string]any{"command": "make test"},
			want:     "Bash: make test",
		},
		{
			name:     "Bash long command truncated",
			toolName: "Bash",
			input:    map[string]any{"command": "make build && make test && make lint && make install"},
			want:     "Bash: make build && make test && make lint ...",
		},
		{
			name:     "Task with subagent",
			toolName: "Task",
			input:    map[string]any{"description": "Search codebase", "subagent_type": "Explore"},
			want:     "Task: Explore - \"Search codebase\"",
		},
		{
			name:     "Task without subagent",
			toolName: "Task",
			input:    map[string]any{"description": "Do something"},
			want:     "Task: Do something",
		},
		{
			name:     "TodoWrite with items",
			toolName: "TodoWrite",
			input:    map[string]any{"todos": []any{map[string]any{"content": "a"}, map[string]any{"content": "b"}}},
			want:     "TodoWrite: 2 items",
		},
		{
			name:     "WebFetch",
			toolName: "WebFetch",
			input:    map[string]any{"url": "https://example.com"},
			want:     "WebFetch: https://example.com",
		},
		{
			name:     "WebSearch",
			toolName: "WebSearch",
			input:    map[string]any{"query": "golang tutorials"},
			want:     "WebSearch: \"golang tutorials\"",
		},
		{
			name:     "NotebookEdit replace",
			toolName: "NotebookEdit",
			input:    map[string]any{"notebook_path": "/path/to/notebook.ipynb"},
			want:     "NotebookEdit: notebook.ipynb (replace)",
		},
		{
			name:     "NotebookEdit insert",
			toolName: "NotebookEdit",
			input:    map[string]any{"notebook_path": "/path/to/notebook.ipynb", "edit_mode": "insert"},
			want:     "NotebookEdit: notebook.ipynb (insert)",
		},
		{
			name:     "NotebookEdit empty path",
			toolName: "NotebookEdit",
			input:    map[string]any{},
			want:     "NotebookEdit: [collapsed]",
		},
		{
			name:     "Unknown tool",
			toolName: "CustomTool",
			input:    map[string]any{},
			want:     "CustomTool: [collapsed]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatToolSummary(tt.toolName, tt.input)
			if got != tt.want {
				t.Errorf("formatToolSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}
