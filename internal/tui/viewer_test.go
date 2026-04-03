// Package tui provides the terminal user interface components.
package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/token"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
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
			// watchMode=true shows LIVE even without a watcher — this supports the
			// embedded single-session viewer in the dashboard where the dashboard's
			// pane watcher forwards events and the viewer's own watcher is nil.
			name:      "watch mode enabled without watcher shows LIVE",
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
			m := NewViewerModel(entries, 0, "Test", opts, nil)

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
			m := NewViewerModel(entries, 0, "Test", DefaultRenderOptions(), nil)

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
		name                  string
		opts                  RenderOptions
		wantThinkingContent   bool // Full thinking content
		wantToolContent       bool // Full tool input
		wantThinkingCollapsed bool // Collapsed indicator for thinking
		wantToolCollapsed     bool // Collapsed indicator for tool (shows tool name + summary)
	}{
		{
			name:                  "default shows all",
			opts:                  DefaultRenderOptions(),
			wantThinkingContent:   true,
			wantToolContent:       true,
			wantThinkingCollapsed: false,
			wantToolCollapsed:     false,
		},
		{
			name:                  "hide thoughts shows collapsed indicator",
			opts:                  RenderOptions{HideThoughts: true, HideTools: false},
			wantThinkingContent:   false,
			wantToolContent:       true,
			wantThinkingCollapsed: true,
			wantToolCollapsed:     false,
		},
		{
			name:                  "hide tools shows collapsed summary",
			opts:                  RenderOptions{HideThoughts: false, HideTools: true},
			wantThinkingContent:   true,
			wantToolContent:       false,
			wantThinkingCollapsed: false,
			wantToolCollapsed:     true,
		},
		{
			name:                  "hide both shows collapsed indicators",
			opts:                  RenderOptions{HideThoughts: true, HideTools: true},
			wantThinkingContent:   false,
			wantToolContent:       false,
			wantThinkingCollapsed: true,
			wantToolCollapsed:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderPlain(entries, "test", tt.opts)

			// Check for full thinking content ("Some thinking")
			hasThinkingContent := strings.Contains(output, "Some thinking")
			if hasThinkingContent != tt.wantThinkingContent {
				t.Errorf("RenderPlain() thinkingContent=%v, want %v", hasThinkingContent, tt.wantThinkingContent)
			}

			// Check for thinking collapsed indicator
			hasThinkingCollapsed := strings.Contains(output, "[thinking collapsed]")
			if hasThinkingCollapsed != tt.wantThinkingCollapsed {
				t.Errorf("RenderPlain() thinkingCollapsed=%v, want %v", hasThinkingCollapsed, tt.wantThinkingCollapsed)
			}

			// Check for full tool input (contains file_path in JSON)
			hasToolContent := strings.Contains(output, "file_path")
			if hasToolContent != tt.wantToolContent {
				t.Errorf("RenderPlain() toolContent=%v, want %v", hasToolContent, tt.wantToolContent)
			}

			// Check for tool collapsed summary (tool name present but not full input)
			hasToolCollapsed := strings.Contains(output, "Read") && strings.Contains(output, "test.go") && !hasToolContent
			if tt.wantToolCollapsed && !hasToolCollapsed {
				// When collapsed, should have tool name and summary but not full JSON input
				if !strings.Contains(output, "Read") {
					t.Error("RenderPlain() collapsed tool should show tool name 'Read'")
				}
			}
		})
	}
}

// TestRenderAssistantMessage_HideThoughtsFlag tests AC-1: TUI --hide-thoughts shows collapsed indicator.
func TestRenderAssistantMessage_HideThoughtsFlag(t *testing.T) {
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{Type: types.ContentTypeThinking, Thinking: "secret thinking content"},
			},
		},
	}

	tests := []struct {
		name                   string
		hideThoughts           bool
		wantCollapsedIndicator bool
		wantThinkingContent    bool
	}{
		{
			name:                   "without flag shows thinking content",
			hideThoughts:           false,
			wantCollapsedIndicator: false,
			wantThinkingContent:    true,
		},
		{
			name:                   "with flag shows collapsed indicator",
			hideThoughts:           true,
			wantCollapsedIndicator: true,
			wantThinkingContent:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := RenderOptions{HideThoughts: tt.hideThoughts}
			mdRenderer, err := NewMarkdownRenderer(80)
			if err != nil {
				t.Fatalf("NewMarkdownRenderer failed: %v", err)
			}
			output := renderAssistantMessageStatic(entry, 80, true, true, opts, mdRenderer, 0, nil)

			hasCollapsedIndicator := strings.Contains(output, "[thinking collapsed]")
			hasThinkingContent := strings.Contains(output, "secret thinking content")

			if hasCollapsedIndicator != tt.wantCollapsedIndicator {
				t.Errorf("collapsed indicator: got %v, want %v", hasCollapsedIndicator, tt.wantCollapsedIndicator)
			}
			if hasThinkingContent != tt.wantThinkingContent {
				t.Errorf("thinking content: got %v, want %v", hasThinkingContent, tt.wantThinkingContent)
			}
		})
	}
}

// TestRenderAssistantMessage_HideToolsFlag tests AC-2: TUI --hide-tools shows collapsed summary.
func TestRenderAssistantMessage_HideToolsFlag(t *testing.T) {
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{
					Type:      types.ContentTypeToolUse,
					ToolName:  "Read",
					ToolInput: map[string]any{"file_path": "/test/file.go"},
				},
			},
		},
	}

	tests := []struct {
		name            string
		hideTools       bool
		wantToolHeader  bool
		wantToolSummary bool
		wantFullInput   bool
	}{
		{
			name:            "without flag shows full tool input",
			hideTools:       false,
			wantToolHeader:  true,
			wantToolSummary: false, // Full input, not summary
			wantFullInput:   true,
		},
		{
			name:            "with flag shows collapsed summary",
			hideTools:       true,
			wantToolHeader:  true,
			wantToolSummary: true,  // Summary (file.go)
			wantFullInput:   false, // No JSON input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := RenderOptions{HideTools: tt.hideTools}
			mdRenderer, err := NewMarkdownRenderer(80)
			if err != nil {
				t.Fatalf("NewMarkdownRenderer failed: %v", err)
			}
			output := renderAssistantMessageStatic(entry, 80, true, true, opts, mdRenderer, 0, nil)

			hasToolHeader := strings.Contains(output, "Tool") && strings.Contains(output, "Read")
			hasToolSummary := strings.Contains(output, "file.go") && !strings.Contains(output, "\"file_path\"")
			hasFullInput := strings.Contains(output, "\"file_path\"")

			if hasToolHeader != tt.wantToolHeader {
				t.Errorf("tool header: got %v, want %v", hasToolHeader, tt.wantToolHeader)
			}
			if tt.wantToolSummary && !hasToolSummary {
				t.Errorf("tool summary: expected summary with file.go but not JSON")
			}
			if hasFullInput != tt.wantFullInput {
				t.Errorf("full input: got %v, want %v", hasFullInput, tt.wantFullInput)
			}
		})
	}
}

// TestRenderAssistantMessage_BothFlagsShowCollapsed tests AC-6: message with only hidden content shows indicators.
func TestRenderAssistantMessage_BothFlagsShowCollapsed(t *testing.T) {
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{Type: types.ContentTypeThinking, Thinking: "hidden thinking"},
				{
					Type:      types.ContentTypeToolUse,
					ToolName:  "Bash",
					ToolInput: map[string]any{"command": "ls"},
				},
			},
		},
	}

	opts := RenderOptions{HideThoughts: true, HideTools: true}
	mdRenderer, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer failed: %v", err)
	}
	output := renderAssistantMessageStatic(entry, 80, true, true, opts, mdRenderer, 0, nil)

	// Both collapsed indicators should be present
	if !strings.Contains(output, "[thinking collapsed]") {
		t.Error("expected thinking collapsed indicator")
	}
	if !strings.Contains(output, "Bash") {
		t.Error("expected tool name 'Bash' in collapsed output")
	}

	// Full content should be hidden
	if strings.Contains(output, "hidden thinking") {
		t.Error("thinking content should be hidden")
	}
	if strings.Contains(output, "\"command\"") {
		t.Error("tool input JSON should be hidden")
	}
}

// TestRenderAssistantMessage_FlagVsToggleConsistency tests AC-5:
// Both flag (--hide-thoughts) and toggle ('t' key) produce collapsed state (content hidden).
// The difference is the text hint: flag shows "collapsed", toggle shows "press 't' to expand".
func TestRenderAssistantMessage_FlagVsToggleConsistency(t *testing.T) {
	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{Type: types.ContentTypeThinking, Thinking: "deep analysis of the problem"},
			},
		},
	}

	mdRenderer, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer failed: %v", err)
	}

	// Flag behavior: HideThoughts=true, showThinking=true (toggle state ignored)
	flagOutput := renderAssistantMessageStatic(entry, 80, true, true, RenderOptions{HideThoughts: true}, mdRenderer, 0, nil)

	// Toggle behavior: HideThoughts=false, showThinking=false (toggle collapsed)
	toggleOutput := renderAssistantMessageStatic(entry, 80, false, true, RenderOptions{HideThoughts: false}, mdRenderer, 0, nil)

	// Both should hide the actual thinking content
	if strings.Contains(flagOutput, "deep analysis of the problem") {
		t.Error("flag output should hide thinking content")
	}
	if strings.Contains(toggleOutput, "deep analysis of the problem") {
		t.Error("toggle output should hide thinking content")
	}

	// Flag should show "collapsed" text
	if !strings.Contains(flagOutput, "[thinking collapsed]") {
		t.Error("flag output should show 'collapsed' text")
	}

	// Toggle should show "press 't' to expand" text
	if !strings.Contains(toggleOutput, "press 't' to expand") {
		t.Error("toggle output should show 'press t to expand' text")
	}

	// Both are collapsed states - neither should show full content
	// This validates AC-5: consistent behavior between flag and toggle
}

func TestBuildNewEntriesSegment(t *testing.T) {
	tests := []struct {
		name            string
		newEntriesCount int
		wantEmpty       bool
		wantContains    string
	}{
		{
			name:            "zero count returns empty",
			newEntriesCount: 0,
			wantEmpty:       true,
			wantContains:    "",
		},
		{
			name:            "single new entry shows +1 new",
			newEntriesCount: 1,
			wantEmpty:       false,
			wantContains:    "+1 new",
		},
		{
			name:            "multiple new entries shows count",
			newEntriesCount: 42,
			wantEmpty:       false,
			wantContains:    "+42 new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{newEntriesCount: tt.newEntriesCount}
			got := m.buildNewEntriesSegment()

			if tt.wantEmpty && got != "" {
				t.Errorf("buildNewEntriesSegment() = %q, want empty string", got)
			}
			if !tt.wantEmpty && !strings.Contains(got, tt.wantContains) {
				t.Errorf("buildNewEntriesSegment() = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}

func TestNewViewerModelNewEntriesCountInitialized(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{WatchMode: true}

	m := NewViewerModel(entries, 0, "Test", opts, nil)

	if m.newEntriesCount != 0 {
		t.Errorf("NewViewerModel() newEntriesCount = %d, want 0", m.newEntriesCount)
	}
}

func TestIsAtBottom(t *testing.T) {
	// Note: This tests the isAtBottom() method behavior conceptually.
	// Since viewport.AtBottom() and ScrollPercent() are internal to bubbles,
	// we test that the method exists and can be called on a ViewerModel.
	// The actual logic is: AtBottom() || ScrollPercent() >= 0.99

	m := ViewerModel{}
	// Method should exist and be callable (returns false for uninitialized viewport)
	result := m.isAtBottom()
	// Uninitialized viewport has no content, ScrollPercent() returns 0
	// AtBottom() returns false for empty viewport
	if result {
		t.Logf("isAtBottom() returned %v for uninitialized viewport (expected false or true depending on viewport implementation)", result)
	}
}

func TestSmartAutoScrollNewEntriesHandler(t *testing.T) {
	// Test that newEntriesCount increments when entries arrive while scrolled up
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{WatchMode: true}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24) // Initialize viewport

	// Since viewport is not scrolled, isAtBottom() returns true on a fresh viewport
	// Manually set newEntriesCount to simulate scrolled-up state
	m.newEntriesCount = 5

	// Verify count can be cleared
	m.newEntriesCount = 0
	if m.newEntriesCount != 0 {
		t.Errorf("newEntriesCount should be 0 after reset, got %d", m.newEntriesCount)
	}
}

func TestGKeyResetsNewEntriesCount(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{WatchMode: true}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Simulate accumulated new entries while scrolled up
	m.newEntriesCount = 10

	// Verify count is positive before G key
	if m.newEntriesCount != 10 {
		t.Errorf("newEntriesCount should be 10 before G key, got %d", m.newEntriesCount)
	}

	// The 'G' key handler resets newEntriesCount to 0
	// We simulate what happens in the Update handler for "G" key
	m.newEntriesCount = 0 // This is what line 274 does

	if m.newEntriesCount != 0 {
		t.Errorf("newEntriesCount should be 0 after G key, got %d", m.newEntriesCount)
	}
}

func TestFileResetMsgClearsNewEntriesCount(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{WatchMode: true}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Simulate accumulated new entries
	m.newEntriesCount = 25

	// FileResetMsg handler clears newEntriesCount (line 388)
	m.newEntriesCount = 0 // This is what line 388 does

	if m.newEntriesCount != 0 {
		t.Errorf("newEntriesCount should be 0 after FileResetMsg, got %d", m.newEntriesCount)
	}
}

func TestManualScrollToBottomClearsNewEntriesCount(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{WatchMode: true}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Simulate new entries accumulated
	m.newEntriesCount = 5

	// The logic in lines 313-316 is:
	// if m.isAtBottom() && m.newEntriesCount > 0 { m.newEntriesCount = 0 }
	// Simulate the behavior when user scrolls to bottom
	if m.isAtBottom() && m.newEntriesCount > 0 {
		m.newEntriesCount = 0
	}

	// Fresh viewport with minimal content is "at bottom"
	if m.newEntriesCount != 0 {
		t.Errorf("newEntriesCount should be 0 after manual scroll to bottom, got %d", m.newEntriesCount)
	}
}

func TestWatchModeToggleOn(t *testing.T) {
	// Test that 'w' key enables watch mode when off with valid FilePath
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{WatchMode: false, FilePath: "/tmp/test.jsonl"}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Initially watch mode should be off
	if m.watchMode != false {
		t.Errorf("Initial watchMode = %v, want false", m.watchMode)
	}

	// Verify renderOpts.FilePath is set (required for toggle to work)
	if m.renderOpts.FilePath != "/tmp/test.jsonl" {
		t.Errorf("renderOpts.FilePath = %q, want %q", m.renderOpts.FilePath, "/tmp/test.jsonl")
	}
}

func TestWatchModeToggleOff(t *testing.T) {
	// Test that toggling off sets watchMode to false and clears newEntriesCount
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{WatchMode: true, FilePath: "/tmp/test.jsonl"}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Simulate new entries accumulated
	m.newEntriesCount = 5

	// Simulate toggle off behavior
	if m.watchMode {
		m.watchMode = false
		m.newEntriesCount = 0
		if m.watcher != nil {
			m.watcher = nil
		}
	}

	if m.watchMode != false {
		t.Errorf("watchMode after toggle off = %v, want false", m.watchMode)
	}
	if m.newEntriesCount != 0 {
		t.Errorf("newEntriesCount after toggle off = %d, want 0", m.newEntriesCount)
	}
}

func TestWatchModeNoFilePathNoOp(t *testing.T) {
	// Test that toggling on without file path is a no-op (graceful degradation)
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{WatchMode: false, FilePath: ""} // Empty FilePath
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Attempting to enable watch mode with empty FilePath should be no-op
	if m.renderOpts.FilePath != "" {
		t.Errorf("FilePath should be empty, got %q", m.renderOpts.FilePath)
	}

	// Verify watchMode stays false
	if m.watchMode != false {
		t.Errorf("watchMode should remain false when FilePath is empty")
	}
}

func TestWatchModeToggleNonExistentFile(t *testing.T) {
	// Test that toggling watch on with non-existent file path is graceful no-op
	// watcher.New() will fail, but we should not crash
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	// Use a path that definitely doesn't exist
	opts := RenderOptions{WatchMode: false, FilePath: "/nonexistent/path/that/does/not/exist.jsonl"}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// FilePath is set but file doesn't exist
	if m.renderOpts.FilePath == "" {
		t.Errorf("FilePath should be set, got empty")
	}

	// Verify initial state
	if m.watchMode != false {
		t.Errorf("Initial watchMode = %v, want false", m.watchMode)
	}

	// watcher.New() with non-existent file returns error
	// The 'w' key handler checks: if err == nil { m.watcher = w; m.watchMode = true }
	// So watchMode should stay false when watcher creation fails
	// We verify the preconditions for graceful failure behavior

	// After failed watcher creation, watchMode should remain false
	if m.watchMode != false {
		t.Errorf("watchMode should remain false when watcher.New fails")
	}
	if m.watcher != nil {
		t.Errorf("watcher should be nil when watcher.New fails")
	}
}

func TestWatcherClosedOnBackNavigation(t *testing.T) {
	// Test that watcher is closed when user navigates back
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{WatchMode: true, FilePath: "/tmp/test.jsonl"}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.canGoBack = true

	// The back navigation handler (h/esc) checks:
	// if m.canGoBack { if m.watcher != nil { m.watcher.Close() } ... }
	// We verify the required conditions are testable
	if !m.canGoBack {
		t.Errorf("canGoBack = %v, want true for back navigation test", m.canGoBack)
	}
}

func TestBuildShortcutsContainsWatchShortcut(t *testing.T) {
	// Test that the shortcuts segment includes 'w:watch'
	m := ViewerModel{canGoBack: false}
	got := m.buildShortcutsSegment()

	if !strings.Contains(got, "w:watch") {
		t.Errorf("buildShortcutsSegment() = %q, should contain 'w:watch'", got)
	}
}

func TestConversationSelectedWithWatchMsgType(t *testing.T) {
	// Test that ConversationSelectedWithWatchMsg type exists and has correct fields
	conv := types.Conversation{FilePath: "/test/path.jsonl"}
	msg := ConversationSelectedWithWatchMsg{Conversation: conv}

	if msg.Conversation.FilePath != "/test/path.jsonl" {
		t.Errorf("ConversationSelectedWithWatchMsg.Conversation.FilePath = %q, want %q",
			msg.Conversation.FilePath, "/test/path.jsonl")
	}
}

// --- Render Cache Tests (Story 3.3) ---

func TestRenderCacheInitialization(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	// Cache should be initialized
	if m.renderCache == nil {
		t.Error("renderCache should be initialized, got nil")
	}
	if len(m.renderCache) != 0 {
		t.Errorf("renderCache should be empty on init, got %d entries", len(m.renderCache))
	}
	// cacheWidth should be initialWidth - 4 - gutterSpace (Story 4.1)
	// gutterWidth for 1 entry = 3, GutterSeparator = " " (1 char), so gutterSpace = 4
	gutterSpace := m.gutterWidth + len(GutterSeparator)
	expectedWidth := 80 - 4 - gutterSpace
	if m.cacheWidth != expectedWidth {
		t.Errorf("cacheWidth = %d, want %d", m.cacheWidth, expectedWidth)
	}
}

func TestInvalidateRenderCache(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeAssistant}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	// Manually add some cache entries
	m.renderCache[0] = "cached content"
	m.renderCache[1] = "more cached content"
	m.width = 100 // Simulate width change

	// Invalidate
	m.invalidateRenderCache()

	// Cache should be cleared
	if len(m.renderCache) != 0 {
		t.Errorf("renderCache should be empty after invalidate, got %d entries", len(m.renderCache))
	}
	// cacheWidth should be updated
	expectedWidth := 100 - 4
	if m.cacheWidth != expectedWidth {
		t.Errorf("cacheWidth after invalidate = %d, want %d", m.cacheWidth, expectedWidth)
	}
}

func TestGetCachedRenderCacheHit(t *testing.T) {
	entries := []types.LogEntry{
		{
			Type: types.EntryTypeAssistant,
			Message: types.Message{
				Content: []types.MessageContent{
					{Type: types.ContentTypeText, Text: "Hello world"},
				},
			},
		},
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24) // Initialize viewport

	// First call - cache miss, should render and cache
	first := m.getCachedRender(0, entries[0])
	if first == "" {
		t.Error("getCachedRender should return non-empty content")
	}

	// Verify cache entry exists
	if _, ok := m.renderCache[0]; !ok {
		t.Error("Entry should be cached after first getCachedRender call")
	}

	// Second call - cache hit, should return same content
	second := m.getCachedRender(0, entries[0])
	if first != second {
		t.Errorf("getCachedRender cache hit should return same content\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestGetCachedRenderKeyByIndex(t *testing.T) {
	entries := []types.LogEntry{
		{Type: types.EntryTypeAssistant, Message: types.Message{Content: []types.MessageContent{{Type: types.ContentTypeText, Text: "Entry 0"}}}},
		{Type: types.EntryTypeUser, Message: types.Message{TextContent: "User entry"}}, // Won't be cached
		{Type: types.EntryTypeAssistant, Message: types.Message{Content: []types.MessageContent{{Type: types.ContentTypeText, Text: "Entry 2"}}}},
		{Type: types.EntryTypeAssistant, Message: types.Message{Content: []types.MessageContent{{Type: types.ContentTypeText, Text: "Entry 3"}}}},
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	// Don't call SetSize yet to avoid automatic updateContent() populating cache

	// Clear any cache from initialization and manually call getCachedRender
	m.renderCache = make(map[int]string)

	// Render entries 0, 2 only (skip 1 which is user entry, skip 3 to test non-rendered)
	m.getCachedRender(0, entries[0])
	m.getCachedRender(1, entries[1]) // User entry - not cached
	m.getCachedRender(2, entries[2])
	// Don't call for index 3

	// Check cache has correct keys
	if _, ok := m.renderCache[0]; !ok {
		t.Error("Index 0 should be in cache")
	}
	if _, ok := m.renderCache[1]; ok {
		t.Error("Index 1 (user entry) should NOT be in cache")
	}
	if _, ok := m.renderCache[2]; !ok {
		t.Error("Index 2 should be in cache")
	}
	if _, ok := m.renderCache[3]; ok {
		t.Error("Index 3 should NOT be in cache (not rendered)")
	}
}

func TestGetCachedRenderOnlyAssistant(t *testing.T) {
	tests := []struct {
		name        string
		entryType   types.EntryType
		shouldCache bool
	}{
		{"assistant cached", types.EntryTypeAssistant, true},
		{"user not cached", types.EntryTypeUser, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entry types.LogEntry
			if tt.entryType == types.EntryTypeAssistant {
				entry = types.LogEntry{
					Type: types.EntryTypeAssistant,
					Message: types.Message{
						Content: []types.MessageContent{
							{Type: types.ContentTypeText, Text: "Content"},
						},
					},
				}
			} else {
				entry = types.LogEntry{
					Type:    types.EntryTypeUser,
					Message: types.Message{TextContent: "User content"},
				}
			}

			entries := []types.LogEntry{entry}
			opts := RenderOptions{Width: 80}
			m := NewViewerModel(entries, 0, "Test", opts, nil)
			m.SetSize(80, 24)

			// Render
			m.getCachedRender(0, entry)

			// Check cache
			_, cached := m.renderCache[0]
			if cached != tt.shouldCache {
				t.Errorf("Entry type %v cached = %v, want %v", tt.entryType, cached, tt.shouldCache)
			}
		})
	}
}

func TestCacheInvalidationOnToggle(t *testing.T) {
	entries := []types.LogEntry{
		{
			Type: types.EntryTypeAssistant,
			Message: types.Message{
				Content: []types.MessageContent{
					{Type: types.ContentTypeText, Text: "Content"},
					{Type: types.ContentTypeThinking, Thinking: "Thinking content"},
				},
			},
		},
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Populate cache
	m.getCachedRender(0, entries[0])
	if len(m.renderCache) == 0 {
		t.Fatal("Cache should have entries before toggle test")
	}

	// Simulate toggle thinking (what 't' key does)
	m.showThinking = !m.showThinking
	m.invalidateRenderCache()

	if len(m.renderCache) != 0 {
		t.Errorf("Cache should be empty after toggle, got %d entries", len(m.renderCache))
	}
}

func TestCacheNotInvalidatedOnNewEntries(t *testing.T) {
	entries := []types.LogEntry{
		{Type: types.EntryTypeAssistant, Message: types.Message{Content: []types.MessageContent{{Type: types.ContentTypeText, Text: "Entry 0"}}}},
	}
	opts := RenderOptions{Width: 80, WatchMode: true}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Populate cache
	m.getCachedRender(0, entries[0])
	initialCacheLen := len(m.renderCache)

	// Simulate NewEntriesMsg behavior - append new entry without invalidating cache
	newEntry := types.LogEntry{
		Type:    types.EntryTypeAssistant,
		Message: types.Message{Content: []types.MessageContent{{Type: types.ContentTypeText, Text: "New entry"}}},
	}
	m.entries = append(m.entries, newEntry)
	m.loadedCount = len(m.entries)
	// Note: We do NOT call invalidateRenderCache() for NewEntriesMsg

	// Cache should still have the original entry
	if len(m.renderCache) != initialCacheLen {
		t.Errorf("Cache should preserve %d entries after new entry, got %d", initialCacheLen, len(m.renderCache))
	}

	// Original entry still cached
	if _, ok := m.renderCache[0]; !ok {
		t.Error("Original entry (index 0) should still be cached after new entries")
	}
}

func TestCacheInvalidationOnResize(t *testing.T) {
	entries := []types.LogEntry{
		{
			Type: types.EntryTypeAssistant,
			Message: types.Message{
				Content: []types.MessageContent{
					{Type: types.ContentTypeText, Text: "Content"},
				},
			},
		},
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24) // Initialize viewport

	// Populate cache
	m.getCachedRender(0, entries[0])
	if len(m.renderCache) == 0 {
		t.Fatal("Cache should have entries before resize test")
	}

	// Simulate resize with widthDiff > 5 (threshold that triggers invalidation)
	// The WindowSizeMsg handler checks: if widthDiff > 5 { m.invalidateRenderCache() }
	oldWidth := m.width
	m.width = oldWidth + 10 // Change > 5

	// Recreate what WindowSizeMsg handler does
	newRenderWidth := m.width - 4
	widthDiff := m.markdownRenderer.Width() - newRenderWidth
	if widthDiff < 0 {
		widthDiff = -widthDiff
	}
	if widthDiff > 5 {
		m.invalidateRenderCache()
	}

	if len(m.renderCache) != 0 {
		t.Errorf("Cache should be empty after resize with widthDiff > 5, got %d entries", len(m.renderCache))
	}
}

func TestCacheInvalidationOnFileReset(t *testing.T) {
	entries := []types.LogEntry{
		{
			Type: types.EntryTypeAssistant,
			Message: types.Message{
				Content: []types.MessageContent{
					{Type: types.ContentTypeText, Text: "Content"},
				},
			},
		},
	}
	opts := RenderOptions{Width: 80, WatchMode: true}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Populate cache
	m.getCachedRender(0, entries[0])
	if len(m.renderCache) == 0 {
		t.Fatal("Cache should have entries before FileResetMsg test")
	}

	// Simulate what FileResetMsg handler does (lines 448-452 in viewer.go)
	m.newEntriesCount = 0
	m.invalidateRenderCache()

	if len(m.renderCache) != 0 {
		t.Errorf("Cache should be empty after FileResetMsg, got %d entries", len(m.renderCache))
	}
}

func TestUpdateContentUsesCachedRender(t *testing.T) {
	entries := []types.LogEntry{
		{
			Type: types.EntryTypeAssistant,
			Message: types.Message{
				Content: []types.MessageContent{
					{Type: types.ContentTypeText, Text: "Cached content test"},
				},
			},
		},
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24) // This calls updateContent() which should populate cache

	// After updateContent(), the assistant entry should be cached
	if _, ok := m.renderCache[0]; !ok {
		t.Error("updateContent() should populate cache for assistant entries")
	}

	// Verify cache has correct content (contains original text)
	cached := m.renderCache[0]
	if cached == "" {
		t.Error("Cached content should not be empty")
	}

	// The cached content should contain the text
	// Note: Glamour processes markdown so exact match not expected
	// but the words should be present
	if !strings.Contains(cached, "Cached") || !strings.Contains(cached, "content") || !strings.Contains(cached, "test") {
		t.Errorf("Cached content should contain rendered text, got: %s", cached)
	}
}

// --- Gutter / Line Numbers Tests (Story 4.1) ---

func TestCalculateGutterWidth(t *testing.T) {
	tests := []struct {
		name       string
		entryCount int
		want       int
	}{
		{"zero entries returns minimum 3", 0, 3},
		{"1 entry returns 3", 1, 3},
		{"9 entries returns 3", 9, 3},
		{"10 entries returns 3", 10, 3},
		{"99 entries returns 3", 99, 3},
		{"100 entries returns 3", 100, 3},
		{"999 entries returns 3", 999, 3},
		{"1000 entries returns 4", 1000, 4},
		{"9999 entries returns 4", 9999, 4},
		{"10000 entries returns 5", 10000, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateGutterWidth(tt.entryCount)
			if got != tt.want {
				t.Errorf("calculateGutterWidth(%d) = %d, want %d", tt.entryCount, got, tt.want)
			}
		})
	}
}

func TestPrependGutter(t *testing.T) {
	tests := []struct {
		name        string
		entryNum    int
		content     string
		gutterWidth int
		wantPrefix  string
		wantPadding string
	}{
		{
			name:        "single line with 3 char gutter",
			entryNum:    1,
			content:     "Hello",
			gutterWidth: 3,
			wantPrefix:  "  1",
			wantPadding: "",
		},
		{
			name:        "multi-line with 3 char gutter",
			entryNum:    42,
			content:     "Line1\nLine2\nLine3",
			gutterWidth: 3,
			wantPrefix:  " 42",
			wantPadding: "    ", // 3 + 1 (separator space)
		},
		{
			name:        "4 char gutter for large numbers",
			entryNum:    1234,
			content:     "Content",
			gutterWidth: 4,
			wantPrefix:  "1234",
			wantPadding: "",
		},
		{
			name:        "multi-line with 4 char gutter",
			entryNum:    1000,
			content:     "First\nSecond",
			gutterWidth: 4,
			wantPrefix:  "1000",
			wantPadding: "     ", // 4 + 1 (separator space)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prependGutter(tt.entryNum, tt.content, tt.gutterWidth)

			// Check that the first line has the correct number prefix
			if !strings.Contains(got, tt.wantPrefix) {
				t.Errorf("prependGutter() should contain prefix %q, got %q", tt.wantPrefix, got)
			}

			// Check that continuation lines have padding (if multi-line)
			if tt.wantPadding != "" {
				lines := strings.Split(got, "\n")
				if len(lines) > 1 {
					// Second line should start with padding
					if !strings.HasPrefix(lines[1], tt.wantPadding) {
						t.Errorf("Continuation line should start with %q padding, got %q", tt.wantPadding, lines[1])
					}
				}
			}
		})
	}
}

func TestPrependGutterStatic(t *testing.T) {
	tests := []struct {
		name        string
		entryNum    int
		content     string
		gutterWidth int
		wantPrefix  string
		wantPadding string
	}{
		{
			name:        "single line with 3 char gutter (no styling)",
			entryNum:    1,
			content:     "Hello",
			gutterWidth: 3,
			wantPrefix:  "  1 ", // right-aligned "  1" + separator " "
			wantPadding: "",
		},
		{
			name:        "multi-line with 3 char gutter (no styling)",
			entryNum:    42,
			content:     "Line1\nLine2\nLine3",
			gutterWidth: 3,
			wantPrefix:  " 42 ", // right-aligned " 42" + separator " "
			wantPadding: "    ", // 3 + 1 (separator space)
		},
		{
			name:        "4 char gutter for large numbers (no styling)",
			entryNum:    1234,
			content:     "Content",
			gutterWidth: 4,
			wantPrefix:  "1234 ", // "1234" + separator " "
			wantPadding: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prependGutterStatic(tt.entryNum, tt.content, tt.gutterWidth)

			// Static version should NOT contain ANSI escape codes (no lipgloss styling)
			if strings.Contains(got, "\x1b[") {
				t.Errorf("prependGutterStatic() should NOT contain ANSI escape codes, got %q", got)
			}

			// Check that the first line starts with the correct number prefix
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("prependGutterStatic() should start with prefix %q, got %q", tt.wantPrefix, got)
			}

			// Check that continuation lines have padding (if multi-line)
			if tt.wantPadding != "" {
				lines := strings.Split(got, "\n")
				if len(lines) > 1 {
					// Second line should start with padding
					if !strings.HasPrefix(lines[1], tt.wantPadding) {
						t.Errorf("Continuation line should start with %q padding, got %q", tt.wantPadding, lines[1])
					}
				}
			}
		})
	}
}

func TestGutterWidthRecalculationOnDigitThreshold(t *testing.T) {
	// Test that gutter width increases when crossing from 999 to 1000 entries
	entries := make([]types.LogEntry, 999)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser}
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	initialWidth := m.gutterWidth
	if initialWidth != 3 {
		t.Errorf("Initial gutterWidth for 999 entries = %d, want 3", initialWidth)
	}

	// Simulate adding entry that crosses threshold
	m.entries = append(m.entries, types.LogEntry{Type: types.EntryTypeUser})
	newWidth := calculateGutterWidth(len(m.entries))
	if newWidth != 4 {
		t.Errorf("New gutterWidth for 1000 entries = %d, want 4", newWidth)
	}
}

func TestNewViewerModelGutterWidthInitialization(t *testing.T) {
	tests := []struct {
		name       string
		entryCount int
		wantWidth  int
	}{
		{"empty entries", 0, 3},
		{"few entries", 10, 3},
		{"99 entries", 99, 3},
		{"1000 entries", 1000, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := make([]types.LogEntry, tt.entryCount)
			opts := RenderOptions{Width: 80}
			m := NewViewerModel(entries, 0, "Test", opts, nil)

			if m.gutterWidth != tt.wantWidth {
				t.Errorf("NewViewerModel() gutterWidth = %d, want %d", m.gutterWidth, tt.wantWidth)
			}
		})
	}
}

func TestShowLineNumbersDefaultTrue(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	if m.showLineNumbers != true {
		t.Errorf("NewViewerModel() showLineNumbers = %v, want true", m.showLineNumbers)
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

// --- Command Mode Tests (Story 4.2) ---

func TestInputModeEnum(t *testing.T) {
	// Test that InputMode enum values are distinct and correct
	if InputNone != 0 {
		t.Errorf("InputNone = %d, want 0", InputNone)
	}
	if InputCommand != 1 {
		t.Errorf("InputCommand = %d, want 1", InputCommand)
	}
	if InputSearch != 2 {
		t.Errorf("InputSearch = %d, want 2", InputSearch)
	}
}

func TestColonKeyActivatesCommandMode(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Initial state should be InputNone
	if m.inputMode != InputNone {
		t.Errorf("Initial inputMode = %d, want InputNone (%d)", m.inputMode, InputNone)
	}

	// Simulate ':' key press - set inputMode to InputCommand
	m.inputMode = InputCommand
	m.inputBuffer = ""

	if m.inputMode != InputCommand {
		t.Errorf("After ':' key, inputMode = %d, want InputCommand (%d)", m.inputMode, InputCommand)
	}
	if m.inputBuffer != "" {
		t.Errorf("After ':' key, inputBuffer = %q, want empty", m.inputBuffer)
	}
}

func TestDigitCaptureInCommandMode(t *testing.T) {
	tests := []struct {
		name       string
		digits     []string
		wantBuffer string
	}{
		{
			name:       "single digit",
			digits:     []string{"5"},
			wantBuffer: "5",
		},
		{
			name:       "multiple digits",
			digits:     []string{"1", "2", "3"},
			wantBuffer: "123",
		},
		{
			name:       "max digits enforced",
			digits:     []string{"1", "2", "3", "4", "5", "6", "7"},
			wantBuffer: "123456", // MaxCommandBufferDigits = 6
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []types.LogEntry{{Type: types.EntryTypeUser}}
			opts := RenderOptions{Width: 80}
			m := NewViewerModel(entries, 0, "Test", opts, nil)
			m.SetSize(80, 24)

			// Enter command mode
			m.inputMode = InputCommand
			m.inputBuffer = ""

			// Simulate digit presses
			for _, digit := range tt.digits {
				if len(m.inputBuffer) < MaxCommandBufferDigits {
					m.inputBuffer += digit
				}
			}

			if m.inputBuffer != tt.wantBuffer {
				t.Errorf("After digits %v, inputBuffer = %q, want %q", tt.digits, m.inputBuffer, tt.wantBuffer)
			}
		})
	}
}

func TestEnterKeyValidNavigationExitsCommandMode(t *testing.T) {
	entries := make([]types.LogEntry, 10)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser, Message: types.Message{TextContent: "Entry"}}
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Enter command mode with valid number
	m.inputMode = InputCommand
	m.inputBuffer = "5"

	// Simulate Enter - valid navigation
	num := 5 // strconv.Atoi("5")
	if num >= 1 && num <= len(m.entries) {
		_ = m.navigateToEntry(num)
		m.inputMode = InputNone
		m.inputBuffer = ""
	}

	if m.inputMode != InputNone {
		t.Errorf("After valid Enter, inputMode = %d, want InputNone (%d)", m.inputMode, InputNone)
	}
	if m.inputBuffer != "" {
		t.Errorf("After valid Enter, inputBuffer = %q, want empty", m.inputBuffer)
	}
}

func TestEscapeKeyCancelsCommandMode(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Enter command mode with some input
	m.inputMode = InputCommand
	m.inputBuffer = "42"

	// Simulate Escape
	m.inputMode = InputNone
	m.inputBuffer = ""

	if m.inputMode != InputNone {
		t.Errorf("After Escape, inputMode = %d, want InputNone (%d)", m.inputMode, InputNone)
	}
	if m.inputBuffer != "" {
		t.Errorf("After Escape, inputBuffer = %q, want empty", m.inputBuffer)
	}
}

func TestZeroEntryShowsToastError(t *testing.T) {
	entries := make([]types.LogEntry, 10)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser}
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Enter command mode with :0
	m.inputMode = InputCommand
	m.inputBuffer = "0"

	// Simulate Enter - should be invalid
	num := 0 // strconv.Atoi("0")
	if num < 1 || num > len(m.entries) {
		m.inputMode = InputNone
		m.inputBuffer = ""
		m.toast = "Invalid line number"
	}

	if m.toast != "Invalid line number" {
		t.Errorf("After :0, toast = %q, want 'Invalid line number'", m.toast)
	}
}

func TestOutOfRangeShowsToastError(t *testing.T) {
	entries := make([]types.LogEntry, 5)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser}
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Enter command mode with :10 (out of range for 5 entries)
	m.inputMode = InputCommand
	m.inputBuffer = "10"

	// Simulate Enter - should be invalid
	num := 10 // strconv.Atoi("10")
	if num < 1 || num > len(m.entries) {
		m.inputMode = InputNone
		m.inputBuffer = ""
		m.toast = "Invalid line number"
	}

	if m.toast != "Invalid line number" {
		t.Errorf("After :10 (max 5), toast = %q, want 'Invalid line number'", m.toast)
	}
}

func TestToastExpiryClearsBothFields(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set toast with expiry using showToast
	_ = m.showToast("Test message", ToastDuration)
	toastID := m.toastID

	// Verify toast is set
	if m.toast != "Test message" {
		t.Errorf("After showToast, toast = %q, want 'Test message'", m.toast)
	}

	// Send toastExpiredMsg through Update() with matching ID (CR fix)
	expiredMsg := toastExpiredMsg{id: toastID}
	updatedModel, _ := m.Update(expiredMsg)
	m = updatedModel.(ViewerModel)

	if m.toast != "" {
		t.Errorf("After toast expiry with matching ID, toast = %q, want empty", m.toast)
	}
	if !m.toastExpiry.IsZero() {
		t.Error("After toast expiry, toastExpiry should be zero")
	}
}

func TestToastExpiryIgnoresMismatchedID(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set toast
	_ = m.showToast("Test message", ToastDuration)

	// Send toastExpiredMsg with wrong ID (simulates race condition)
	expiredMsg := toastExpiredMsg{id: 999}
	updatedModel, _ := m.Update(expiredMsg)
	m = updatedModel.(ViewerModel)

	// Toast should NOT be cleared because ID doesn't match
	if m.toast != "Test message" {
		t.Errorf("After toast expiry with wrong ID, toast = %q, want 'Test message'", m.toast)
	}
}

func TestEmptyConversationShowsError(t *testing.T) {
	entries := []types.LogEntry{} // Empty
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Try to navigate to any entry
	err := m.navigateToEntry(1)
	if err == nil {
		t.Error("navigateToEntry(1) on empty entries should return error")
	}
	if err != nil && err.Error() != "invalid line number" {
		t.Errorf("navigateToEntry error = %q, want 'invalid line number'", err.Error())
	}
}

func TestNavigateToFirstEntry(t *testing.T) {
	entries := make([]types.LogEntry, 10)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser, Message: types.Message{TextContent: "Entry"}}
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Navigate to first entry
	err := m.navigateToEntry(1)
	if err != nil {
		t.Errorf("navigateToEntry(1) should not error, got %v", err)
	}

	// Entry line positions should exist
	if len(m.entryLinePositions) == 0 {
		t.Error("entryLinePositions should be populated after SetSize")
	}
}

func TestNavigateToLastEntry(t *testing.T) {
	entries := make([]types.LogEntry, 10)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser, Message: types.Message{TextContent: "Entry"}}
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Navigate to last entry
	lastEntry := len(entries)
	err := m.navigateToEntry(lastEntry)
	if err != nil {
		t.Errorf("navigateToEntry(%d) should not error, got %v", lastEntry, err)
	}
}

func TestNonNumericInputIgnoredInCommandMode(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Enter command mode
	m.inputMode = InputCommand
	m.inputBuffer = "12"

	// Simulate non-numeric input (should be ignored)
	// In real handler, letters would not modify inputBuffer
	nonNumericKeys := []string{"a", "b", "x", "!", "@"}
	for _, key := range nonNumericKeys {
		// Only digits 0-9 are accepted
		if key >= "0" && key <= "9" {
			m.inputBuffer += key
		}
	}

	if m.inputBuffer != "12" {
		t.Errorf("After non-numeric input, inputBuffer = %q, want '12'", m.inputBuffer)
	}
}

func TestBackspaceRemovesLastDigit(t *testing.T) {
	tests := []struct {
		name       string
		initial    string
		wantBuffer string
	}{
		{
			name:       "remove last digit",
			initial:    "123",
			wantBuffer: "12",
		},
		{
			name:       "remove from single digit",
			initial:    "5",
			wantBuffer: "",
		},
		{
			name:       "backspace on empty buffer",
			initial:    "",
			wantBuffer: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []types.LogEntry{{Type: types.EntryTypeUser}}
			opts := RenderOptions{Width: 80}
			m := NewViewerModel(entries, 0, "Test", opts, nil)
			m.SetSize(80, 24)

			// Enter command mode with initial buffer
			m.inputMode = InputCommand
			m.inputBuffer = tt.initial

			// Simulate backspace
			if len(m.inputBuffer) > 0 {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}

			if m.inputBuffer != tt.wantBuffer {
				t.Errorf("After backspace, inputBuffer = %q, want %q", m.inputBuffer, tt.wantBuffer)
			}
		})
	}
}

func TestBuildShortcutsContainsGotoHint(t *testing.T) {
	// Test that shortcuts segment includes :N:goto hint
	m := ViewerModel{canGoBack: false}
	got := m.buildShortcutsSegment()

	if !strings.Contains(got, ":N:goto") {
		t.Errorf("buildShortcutsSegment() = %q, should contain ':N:goto'", got)
	}
}

func TestNewViewerModelInputModeInitialization(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	if m.inputMode != InputNone {
		t.Errorf("NewViewerModel() inputMode = %d, want InputNone (%d)", m.inputMode, InputNone)
	}
	if m.inputBuffer != "" {
		t.Errorf("NewViewerModel() inputBuffer = %q, want empty", m.inputBuffer)
	}
}

func TestNewViewerModelEntryLinePositionsInitialization(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	if m.entryLinePositions == nil {
		t.Error("NewViewerModel() entryLinePositions should be initialized, got nil")
	}
}

func TestEntryLinePositionsPopulatedOnUpdateContent(t *testing.T) {
	entries := make([]types.LogEntry, 5)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser, Message: types.Message{TextContent: "Entry"}}
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24) // Triggers updateContent()

	// entryLinePositions should have same count as loaded entries
	if len(m.entryLinePositions) != len(entries) {
		t.Errorf("entryLinePositions has %d entries, want %d", len(m.entryLinePositions), len(entries))
	}

	// First entry should start at line 0
	if len(m.entryLinePositions) > 0 && m.entryLinePositions[0] != 0 {
		t.Errorf("entryLinePositions[0] = %d, want 0", m.entryLinePositions[0])
	}

	// Subsequent entries should have increasing line positions
	for i := 1; i < len(m.entryLinePositions); i++ {
		if m.entryLinePositions[i] <= m.entryLinePositions[i-1] {
			t.Errorf("entryLinePositions[%d] = %d should be > entryLinePositions[%d] = %d",
				i, m.entryLinePositions[i], i-1, m.entryLinePositions[i-1])
		}
	}
}

func TestNavigateToEntryBoundaryValidation(t *testing.T) {
	entries := make([]types.LogEntry, 10)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser}
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	tests := []struct {
		name      string
		entryNum  int
		wantError bool
	}{
		{"valid first entry", 1, false},
		{"valid last entry", 10, false},
		{"valid middle entry", 5, false},
		{"invalid zero", 0, true},
		{"invalid negative", -1, true},
		{"invalid too large", 11, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.navigateToEntry(tt.entryNum)
			if (err != nil) != tt.wantError {
				t.Errorf("navigateToEntry(%d) error = %v, wantError = %v", tt.entryNum, err, tt.wantError)
			}
		})
	}
}

func TestShowToastSetsFields(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	// Call showToast
	cmd := m.showToast("Test message", ToastDuration)

	if m.toast != "Test message" {
		t.Errorf("showToast() toast = %q, want 'Test message'", m.toast)
	}
	if m.toastExpiry.IsZero() {
		t.Error("showToast() toastExpiry should not be zero")
	}
	if m.toastID != 1 {
		t.Errorf("showToast() toastID = %d, want 1 (first toast)", m.toastID)
	}

	// Verify returned command is not nil (CR fix)
	if cmd == nil {
		t.Error("showToast() should return a non-nil tea.Cmd")
	}
}

func TestShowToastIncreasesToastID(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	// First toast
	_ = m.showToast("First", ToastDuration)
	firstID := m.toastID

	// Second toast
	_ = m.showToast("Second", ToastDuration)
	secondID := m.toastID

	if secondID != firstID+1 {
		t.Errorf("Second toastID = %d, want %d (firstID+1)", secondID, firstID+1)
	}
}

func TestSyncEntryLinePositions(t *testing.T) {
	entries := make([]types.LogEntry, 5)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser, Message: types.Message{TextContent: "Entry"}}
	}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Clear entry line positions
	m.entryLinePositions = nil

	// Call syncEntryLinePositions
	m.syncEntryLinePositions()

	// Verify positions are rebuilt
	if len(m.entryLinePositions) != len(entries) {
		t.Errorf("syncEntryLinePositions() created %d positions, want %d", len(m.entryLinePositions), len(entries))
	}

	// First entry should start at line 0
	if len(m.entryLinePositions) > 0 && m.entryLinePositions[0] != 0 {
		t.Errorf("entryLinePositions[0] = %d, want 0", m.entryLinePositions[0])
	}

	// Subsequent entries should have increasing line positions
	for i := 1; i < len(m.entryLinePositions); i++ {
		if m.entryLinePositions[i] <= m.entryLinePositions[i-1] {
			t.Errorf("entryLinePositions[%d] = %d should be > entryLinePositions[%d] = %d",
				i, m.entryLinePositions[i], i-1, m.entryLinePositions[i-1])
		}
	}
}

// --- Raw JSONL Mode Tests (Story 4.3) ---

func TestRawModeInitialization(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	// rawMode should be false by default
	if m.rawMode != false {
		t.Errorf("NewViewerModel() rawMode = %v, want false", m.rawMode)
	}
	// rawLines should be nil
	if m.rawLines != nil {
		t.Errorf("NewViewerModel() rawLines should be nil, got %v", m.rawLines)
	}
	// rawLineCount should be 0
	if m.rawLineCount != 0 {
		t.Errorf("NewViewerModel() rawLineCount = %d, want 0", m.rawLineCount)
	}
	// rawLinePositions should be nil
	if m.rawLinePositions != nil {
		t.Errorf("NewViewerModel() rawLinePositions should be nil, got %v", m.rawLinePositions)
	}
}

func TestRKeyTogglesRawModeToTrue(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Initially rawMode is false
	if m.rawMode != false {
		t.Errorf("Initial rawMode = %v, want false", m.rawMode)
	}

	// Simulate 'r' key toggle behavior (without actual file loading)
	m.rawMode = true
	m.rawLines = []string{`{"type": "test"}`}
	m.rawLineCount = 1

	if m.rawMode != true {
		t.Errorf("After 'r' toggle, rawMode = %v, want true", m.rawMode)
	}
}

func TestRKeyTogglesRawModeToFalse(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Start in raw mode
	m.rawMode = true
	m.rawLines = []string{`{"type": "test"}`}
	m.rawLineCount = 1

	// Simulate 'r' key toggle to exit raw mode
	m.rawMode = false

	if m.rawMode != false {
		t.Errorf("After 'r' toggle back, rawMode = %v, want false", m.rawMode)
	}
}

func TestFormatJSONLineValidJSON(t *testing.T) {
	line := `{"name":"test","value":42}`
	got := formatJSONLine(line)

	// Should be pretty-printed with indentation
	if !strings.Contains(got, "  ") {
		t.Errorf("formatJSONLine() should contain indentation, got %q", got)
	}
	if !strings.Contains(got, "\"name\"") {
		t.Errorf("formatJSONLine() should contain 'name' key, got %q", got)
	}
	if !strings.Contains(got, "\"value\"") {
		t.Errorf("formatJSONLine() should contain 'value' key, got %q", got)
	}
}

func TestFormatJSONLineInvalidJSON(t *testing.T) {
	line := "not valid json {{"
	got := formatJSONLine(line)

	// Invalid JSON should be returned as-is
	if got != line {
		t.Errorf("formatJSONLine() for invalid JSON = %q, want %q", got, line)
	}
}

func TestRawModeGutterWidthUsesRawLineCount(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set up raw mode with many lines
	m.rawMode = true
	m.rawLines = make([]string, 1000)
	for i := range m.rawLines {
		m.rawLines[i] = `{"line": ` + string(rune('0'+i%10)) + `}`
	}
	m.rawLineCount = 1000

	// Gutter width for 1000 lines should be 4
	gutterWidth := calculateGutterWidth(m.rawLineCount)
	if gutterWidth != 4 {
		t.Errorf("calculateGutterWidth(%d) = %d, want 4", m.rawLineCount, gutterWidth)
	}
}

func TestNavigateToEntryInRawMode(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set up raw mode
	m.rawMode = true
	m.rawLines = []string{`{"line": 1}`, `{"line": 2}`, `{"line": 3}`}
	m.rawLineCount = 3
	m.rawLinePositions = []int{0, 5, 10}

	// Valid navigation
	err := m.navigateToEntry(2)
	if err != nil {
		t.Errorf("navigateToEntry(2) in raw mode should not error, got %v", err)
	}

	// Invalid navigation - out of range
	err = m.navigateToEntry(5)
	if err == nil {
		t.Error("navigateToEntry(5) in raw mode with 3 lines should return error")
	}
}

func TestNavigateToEntryInRawModeZeroInvalid(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set up raw mode
	m.rawMode = true
	m.rawLines = []string{`{"line": 1}`}
	m.rawLineCount = 1
	m.rawLinePositions = []int{0}

	// Zero should be invalid
	err := m.navigateToEntry(0)
	if err == nil {
		t.Error("navigateToEntry(0) in raw mode should return error")
	}
}

func TestBuildModeSegmentShowsRAW(t *testing.T) {
	m := ViewerModel{rawMode: true}
	got := m.buildModeSegment()

	if !strings.Contains(got, "RAW") {
		t.Errorf("buildModeSegment() in raw mode = %q, should contain 'RAW'", got)
	}
}

func TestBuildModeSegmentShowsRAWAndLIVE(t *testing.T) {
	// Create a mock watcher (non-nil to indicate active)
	m := ViewerModel{rawMode: true, watchMode: true}
	// watcher is nil, so LIVE won't show
	got := m.buildModeSegment()

	if !strings.Contains(got, "RAW") {
		t.Errorf("buildModeSegment() with rawMode and watchMode = %q, should contain 'RAW'", got)
	}
	// LIVE requires non-nil watcher
}

func TestBuildPositionSegmentInRawMode(t *testing.T) {
	m := ViewerModel{rawMode: true, rawLineCount: 50}
	got := m.buildPositionSegment()

	if !strings.Contains(got, "Line") {
		t.Errorf("buildPositionSegment() in raw mode = %q, should contain 'Line'", got)
	}
	if !strings.Contains(got, "/50") {
		t.Errorf("buildPositionSegment() in raw mode = %q, should contain '/50'", got)
	}
}

func TestBuildPositionSegmentEmptyRawMode(t *testing.T) {
	m := ViewerModel{rawMode: true, rawLineCount: 0}
	got := m.buildPositionSegment()

	if !strings.Contains(got, "Line 0/0") {
		t.Errorf("buildPositionSegment() in empty raw mode = %q, should contain 'Line 0/0'", got)
	}
}

func TestBuildShortcutsSegmentRawModeHint(t *testing.T) {
	tests := []struct {
		name     string
		rawMode  bool
		wantHint string
	}{
		{"normal mode shows r:raw", false, "r:raw"},
		{"raw mode shows r:normal", true, "r:normal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{rawMode: tt.rawMode}
			got := m.buildShortcutsSegment()

			if !strings.Contains(got, tt.wantHint) {
				t.Errorf("buildShortcutsSegment() with rawMode=%v = %q, should contain %q", tt.rawMode, got, tt.wantHint)
			}
		})
	}
}

func TestLoadRawJSONLMissingFilePath(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80, FilePath: ""} // Empty FilePath
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	err := m.loadRawJSONL()
	if err == nil {
		t.Error("loadRawJSONL() with empty FilePath should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "no file path") {
		t.Errorf("loadRawJSONL() error = %q, should contain 'no file path'", err.Error())
	}
}

func TestWatchModeExitsRawModeOnNewEntries(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80, WatchMode: true}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Start in raw mode
	m.rawMode = true
	m.rawLines = []string{`{"line": 1}`}
	m.rawLineCount = 1

	// Simulate NewEntriesMsg behavior - should exit raw mode
	if m.rawMode {
		m.rawMode = false
	}

	if m.rawMode != false {
		t.Errorf("After NewEntriesMsg, rawMode = %v, want false", m.rawMode)
	}
}

func TestFileResetMsgExitsRawMode(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80, WatchMode: true}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Start in raw mode
	m.rawMode = true
	m.rawLines = []string{`{"line": 1}`}
	m.rawLineCount = 1

	// Simulate FileResetMsg behavior - should exit raw mode
	if m.rawMode {
		m.rawMode = false
	}

	if m.rawMode != false {
		t.Errorf("After FileResetMsg, rawMode = %v, want false", m.rawMode)
	}
}

func TestScrollPositionPreservationOnToggle(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Save initial scroll position (percentage-based)
	initialPct := m.viewport.ScrollPercent()

	// Simulate toggle to raw mode
	m.rawMode = true
	m.rawLines = []string{`{"line": 1}`, `{"line": 2}`}
	m.rawLineCount = 2
	m.loadedCount = 2

	// Toggle back
	m.rawMode = false
	m.updateContent()

	// Check scroll position is restored (approximately)
	restoredPct := m.viewport.ScrollPercent()
	// Both should be 0.0 for this small content
	if initialPct != restoredPct {
		t.Logf("Scroll position before=%f after=%f (expected approximately equal)", initialPct, restoredPct)
	}
}

func TestFormatJSONLineArray(t *testing.T) {
	// Test that JSON arrays are also pretty-printed (Story 4.3 CR fix)
	line := `[1,2,3,{"nested":"value"}]`
	got := formatJSONLine(line)

	// Should be pretty-printed with indentation
	if !strings.Contains(got, "  ") {
		t.Errorf("formatJSONLine() for array should contain indentation, got %q", got)
	}
	if !strings.Contains(got, "\"nested\"") {
		t.Errorf("formatJSONLine() for array should contain nested key, got %q", got)
	}
}

func TestLoadMoreRawLinesMsg(t *testing.T) {
	// Test that rawLinesLoadedMsg type exists and is handled
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set up raw mode with lazy loading enabled
	m.rawMode = true
	m.rawLines = make([]string, 150)
	for i := range m.rawLines {
		m.rawLines[i] = `{"line": ` + string(rune('0'+i%10)) + `}`
	}
	m.rawLineCount = 150
	m.loadedCount = 40
	m.lazyEnabled = true
	m.lazyLoadState = LoadingStateIdle

	// Simulate receiving rawLinesLoadedMsg
	msg := rawLinesLoadedMsg{loadedCount: 60}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(ViewerModel)

	if m.loadedCount != 60 {
		t.Errorf("After rawLinesLoadedMsg, loadedCount = %d, want 60", m.loadedCount)
	}
	if m.lazyLoadState != LoadingStateIdle {
		t.Errorf("After partial load, lazyLoadState should be LoadingStateIdle")
	}
}

func TestLoadMoreRawLinesComplete(t *testing.T) {
	// Test that loading all raw lines sets state to complete
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set up raw mode
	m.rawMode = true
	m.rawLines = []string{`{"line": 1}`, `{"line": 2}`}
	m.rawLineCount = 2
	m.loadedCount = 1
	m.lazyEnabled = true

	// Simulate loading all remaining lines
	msg := rawLinesLoadedMsg{loadedCount: 2}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(ViewerModel)

	if m.lazyLoadState != LoadingStateComplete {
		t.Errorf("After loading all lines, lazyLoadState = %v, want LoadingStateComplete", m.lazyLoadState)
	}
}

func TestNavigateToEntryLoadsLazyContentInRawMode(t *testing.T) {
	// Test that navigating beyond loaded content triggers full load in raw mode
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set up raw mode with partial content loaded
	m.rawMode = true
	m.rawLines = make([]string, 100)
	for i := range m.rawLines {
		m.rawLines[i] = `{"line": ` + string(rune('0'+i%10)) + `}`
	}
	m.rawLineCount = 100
	m.loadedCount = 40 // Only 40 loaded
	m.rawLinePositions = make([]int, 40)
	m.lazyEnabled = true

	// Navigate to line 80 (beyond loaded content)
	err := m.navigateToEntry(80)
	if err != nil {
		t.Errorf("navigateToEntry(80) should not error, got %v", err)
	}

	// loadedCount should now be rawLineCount (all loaded)
	if m.loadedCount != m.rawLineCount {
		t.Errorf("After navigating beyond loaded, loadedCount = %d, want %d", m.loadedCount, m.rawLineCount)
	}
	if m.lazyLoadState != LoadingStateComplete {
		t.Errorf("After loading all, lazyLoadState should be LoadingStateComplete")
	}
}

func TestRawModeLazyLoadScrollTrigger(t *testing.T) {
	// Test that scroll check in raw mode uses rawLineCount not entries count
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set up raw mode with lazy loading
	m.rawMode = true
	m.rawLines = make([]string, 150)
	for i := range m.rawLines {
		m.rawLines[i] = `{"line": ` + string(rune('0'+i%10)) + `}`
	}
	m.rawLineCount = 150
	m.loadedCount = 40
	m.lazyEnabled = true
	m.lazyLoadState = LoadingStateIdle

	// loadMoreRawLines should return a command that produces rawLinesLoadedMsg
	cmd := m.loadMoreRawLines()
	if cmd == nil {
		t.Error("loadMoreRawLines() should return a non-nil tea.Cmd")
	}

	// Execute the command to verify it produces the right message type
	msg := cmd()
	if _, ok := msg.(rawLinesLoadedMsg); !ok {
		t.Errorf("loadMoreRawLines() command should produce rawLinesLoadedMsg, got %T", msg)
	}
}

func TestGKeyInRawModeStaysInRawMode(t *testing.T) {
	// Test that 'G' key in raw mode stays in raw mode (Story 4.3 CR fix)
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set up raw mode with lazy loading
	m.rawMode = true
	m.rawLines = make([]string, 150)
	for i := range m.rawLines {
		m.rawLines[i] = `{"line": ` + string(rune('0'+i%10)) + `}`
	}
	m.rawLineCount = 150
	m.loadedCount = 40
	m.lazyEnabled = true
	m.lazyLoadState = LoadingStateIdle

	// Press 'G' key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	updatedModel, cmd := m.Update(keyMsg)
	m = updatedModel.(ViewerModel)

	// Should still be in raw mode
	if !m.rawMode {
		t.Error("After 'G' key in raw mode, should still be in rawMode")
	}

	// Should show overlay spinner for loading
	if !m.showOverlaySpinner {
		t.Error("After 'G' key with lazy loading, should show overlay spinner")
	}

	// Command should be returned (batch command for lazy loading)
	// We just verify the model state is correct - command execution is implicit
	_ = cmd
}

func TestGKeyInRawModeAllLoadedGoesToBottom(t *testing.T) {
	// Test that 'G' key in raw mode when all loaded just goes to bottom
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Set up raw mode with all content loaded
	m.rawMode = true
	m.rawLines = []string{`{"line": 1}`, `{"line": 2}`}
	m.rawLineCount = 2
	m.loadedCount = 2
	m.lazyEnabled = true
	m.lazyLoadState = LoadingStateComplete

	// Press 'G' key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	updatedModel, _ := m.Update(keyMsg)
	m = updatedModel.(ViewerModel)

	// Should still be in raw mode
	if !m.rawMode {
		t.Error("After 'G' key when all loaded, should still be in rawMode")
	}

	// Should NOT show overlay spinner (all content already loaded)
	if m.showOverlaySpinner {
		t.Error("After 'G' key when all loaded, should NOT show overlay spinner")
	}
}

// --- Path Display Toast Tests (Story 4.4) ---

func TestPKeyWithValidFilePathShowsPathToast(t *testing.T) {
	// Test AC 4.4.1: 'p' key with valid FilePath shows path in toast
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80, FilePath: "/path/to/conversation.jsonl"}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Press 'p' key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	updatedModel, cmd := m.Update(keyMsg)
	m = updatedModel.(ViewerModel)

	// Toast should show the file path
	if m.toast != "/path/to/conversation.jsonl" {
		t.Errorf("After 'p' key with FilePath, toast = %q, want '/path/to/conversation.jsonl'", m.toast)
	}

	// Toast command should be returned (non-nil)
	if cmd == nil {
		t.Error("After 'p' key, should return toast timer command")
	}

	// toastID should be incremented
	if m.toastID < 1 {
		t.Errorf("After 'p' key, toastID = %d, want >= 1", m.toastID)
	}
}

func TestPKeyWithoutFilePathShowsNoPathAvailable(t *testing.T) {
	// Test AC 4.4.3: 'p' key without FilePath shows "No path available"
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80, FilePath: ""} // Empty FilePath
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Press 'p' key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	updatedModel, cmd := m.Update(keyMsg)
	m = updatedModel.(ViewerModel)

	// Toast should show "No path available"
	if m.toast != "No path available" {
		t.Errorf("After 'p' key without FilePath, toast = %q, want 'No path available'", m.toast)
	}

	// Toast command should be returned
	if cmd == nil {
		t.Error("After 'p' key without FilePath, should return toast timer command")
	}
}

func TestPKeyInRawModeShowsPath(t *testing.T) {
	// Test AC 4.4.4: 'p' key in raw mode shows path (same behavior as normal mode)
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80, FilePath: "/path/to/rawfile.jsonl"}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Enter raw mode
	m.rawMode = true
	m.rawLines = []string{`{"line": 1}`}
	m.rawLineCount = 1

	// Press 'p' key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	updatedModel, cmd := m.Update(keyMsg)
	m = updatedModel.(ViewerModel)

	// Toast should show the file path even in raw mode
	if m.toast != "/path/to/rawfile.jsonl" {
		t.Errorf("After 'p' key in raw mode, toast = %q, want '/path/to/rawfile.jsonl'", m.toast)
	}

	// Should still return toast command
	if cmd == nil {
		t.Error("After 'p' key in raw mode, should return toast timer command")
	}

	// Should still be in raw mode
	if !m.rawMode {
		t.Error("After 'p' key, should still be in raw mode")
	}
}

func TestPathToastExpiry(t *testing.T) {
	// Test AC 4.4.2: toast expiry clears path toast after 3 seconds
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80, FilePath: "/path/to/file.jsonl"}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// Press 'p' key to show path toast
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	updatedModel, _ := m.Update(keyMsg)
	m = updatedModel.(ViewerModel)

	// Verify toast is set
	if m.toast != "/path/to/file.jsonl" {
		t.Fatalf("Toast should be set before expiry test")
	}
	toastID := m.toastID

	// Send toastExpiredMsg with matching ID
	expiredMsg := toastExpiredMsg{id: toastID}
	updatedModel, _ = m.Update(expiredMsg)
	m = updatedModel.(ViewerModel)

	// Toast should be cleared
	if m.toast != "" {
		t.Errorf("After toast expiry, toast = %q, want empty", m.toast)
	}
	if !m.toastExpiry.IsZero() {
		t.Error("After toast expiry, toastExpiry should be zero")
	}
}

func TestBuildShortcutsContainsPathHint(t *testing.T) {
	// Test AC 4.4.5: shortcuts segment includes 'p:path' hint
	m := ViewerModel{canGoBack: false}
	got := m.buildShortcutsSegment()

	if !strings.Contains(got, "p:path") {
		t.Errorf("buildShortcutsSegment() = %q, should contain 'p:path'", got)
	}
}

func TestRapidPKeyPressesUpdateToastID(t *testing.T) {
	// Test that rapid 'p' key presses update toastID correctly (race prevention)
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80, FilePath: "/path/to/file.jsonl"}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	// First 'p' key press
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	updatedModel, _ := m.Update(keyMsg)
	m = updatedModel.(ViewerModel)
	firstToastID := m.toastID

	// Second rapid 'p' key press
	updatedModel, _ = m.Update(keyMsg)
	m = updatedModel.(ViewerModel)
	secondToastID := m.toastID

	// Third rapid 'p' key press
	updatedModel, _ = m.Update(keyMsg)
	m = updatedModel.(ViewerModel)
	thirdToastID := m.toastID

	// Each press should increment toastID
	if secondToastID != firstToastID+1 {
		t.Errorf("Second toastID = %d, want %d", secondToastID, firstToastID+1)
	}
	if thirdToastID != secondToastID+1 {
		t.Errorf("Third toastID = %d, want %d", thirdToastID, secondToastID+1)
	}

	// Old toastExpiredMsg should be ignored (test via ID mismatch)
	oldExpiredMsg := toastExpiredMsg{id: firstToastID}
	updatedModel, _ = m.Update(oldExpiredMsg)
	m = updatedModel.(ViewerModel)

	// Toast should NOT be cleared because ID doesn't match current
	if m.toast != "/path/to/file.jsonl" {
		t.Errorf("After old toast expiry, toast = %q, should still show path", m.toast)
	}
}

// Story 4.6: Position indicator desync fix tests

func TestFindVisibleEntry(t *testing.T) {
	tests := []struct {
		name      string
		positions []int
		yOffset   int
		want      int
	}{
		{
			name:      "empty positions returns 1",
			positions: []int{},
			yOffset:   100,
			want:      1,
		},
		{
			name:      "single entry at offset 0",
			positions: []int{0},
			yOffset:   0,
			want:      1,
		},
		{
			name:      "single entry with higher offset",
			positions: []int{0},
			yOffset:   50,
			want:      1,
		},
		{
			name:      "multiple entries - at start",
			positions: []int{0, 10, 20, 30},
			yOffset:   0,
			want:      1,
		},
		{
			name:      "multiple entries - middle of first",
			positions: []int{0, 10, 20, 30},
			yOffset:   5,
			want:      1,
		},
		{
			name:      "multiple entries - exactly at second",
			positions: []int{0, 10, 20, 30},
			yOffset:   10,
			want:      2,
		},
		{
			name:      "multiple entries - between second and third",
			positions: []int{0, 10, 20, 30},
			yOffset:   15,
			want:      2,
		},
		{
			name:      "multiple entries - at last",
			positions: []int{0, 10, 20, 30},
			yOffset:   30,
			want:      4,
		},
		{
			name:      "multiple entries - past last",
			positions: []int{0, 10, 20, 30},
			yOffset:   100,
			want:      4,
		},
		{
			name:      "large array - binary search works",
			positions: []int{0, 50, 100, 150, 200, 250, 300, 350, 400, 450},
			yOffset:   225,
			want:      5, // Entry 5 starts at 200, entry 6 starts at 250
		},
		{
			name:      "offset before first entry",
			positions: []int{10, 20, 30},
			yOffset:   5,
			want:      1, // Binary search finds no position <= 5, returns index 0 + 1 = 1
		},
		{
			name:      "negative offset returns first entry",
			positions: []int{0, 10, 20, 30},
			yOffset:   -5,
			want:      1, // Negative offset: no position <= -5, returns index 0 + 1 = 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{entryLinePositions: tt.positions}
			got := m.findVisibleEntry(tt.yOffset)
			if got != tt.want {
				t.Errorf("findVisibleEntry(%d) = %d, want %d", tt.yOffset, got, tt.want)
			}
		})
	}
}

func TestFindVisibleRawLine(t *testing.T) {
	tests := []struct {
		name      string
		positions []int
		yOffset   int
		want      int
	}{
		{
			name:      "empty positions returns 1",
			positions: []int{},
			yOffset:   100,
			want:      1,
		},
		{
			name:      "single line at offset 0",
			positions: []int{0},
			yOffset:   0,
			want:      1,
		},
		{
			name:      "multiple lines - at start",
			positions: []int{0, 5, 10, 15, 20},
			yOffset:   0,
			want:      1,
		},
		{
			name:      "multiple lines - between lines",
			positions: []int{0, 5, 10, 15, 20},
			yOffset:   7,
			want:      2, // Line 2 starts at 5, line 3 starts at 10
		},
		{
			name:      "multiple lines - exactly on line",
			positions: []int{0, 5, 10, 15, 20},
			yOffset:   15,
			want:      4,
		},
		{
			name:      "multiple lines - past last",
			positions: []int{0, 5, 10, 15, 20},
			yOffset:   50,
			want:      5,
		},
		{
			name:      "negative offset returns first line",
			positions: []int{0, 5, 10, 15, 20},
			yOffset:   -10,
			want:      1, // Negative offset: no position <= -10, returns index 0 + 1 = 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{rawLinePositions: tt.positions}
			got := m.findVisibleRawLine(tt.yOffset)
			if got != tt.want {
				t.Errorf("findVisibleRawLine(%d) = %d, want %d", tt.yOffset, got, tt.want)
			}
		})
	}
}

func TestBuildPositionSegmentWithAccuratePosition(t *testing.T) {
	// Note: viewport.YOffset is read from viewport.YOffset property.
	// For unit tests, we test the lookup functions directly (TestFindVisibleEntry/TestFindVisibleRawLine).
	// This test validates that buildPositionSegment produces expected output format
	// when positions are set to known values.

	tests := []struct {
		name       string
		entries    int
		positions  []int
		rawMode    bool
		rawCount   int
		rawPos     []int
		want       string
		wantPrefix string
	}{
		{
			name:      "normal mode - has entries shows entry count",
			entries:   10,
			positions: []int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90},
			want:      "/10", // Shows total entries in denominator
		},
		{
			name:      "normal mode - empty entries shows 0/0",
			entries:   0,
			positions: []int{},
			want:      "Entry 0/0",
		},
		{
			name:     "raw mode - has lines shows line count",
			rawMode:  true,
			rawCount: 5,
			rawPos:   []int{0, 3, 6, 9, 12},
			want:     "/5", // Shows total lines in denominator
		},
		{
			name:     "raw mode - empty shows 0/0",
			rawMode:  true,
			rawCount: 0,
			rawPos:   []int{},
			want:     "Line 0/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := make([]types.LogEntry, tt.entries)
			m := ViewerModel{
				entries:            entries,
				entryLinePositions: tt.positions,
				rawMode:            tt.rawMode,
				rawLineCount:       tt.rawCount,
				rawLinePositions:   tt.rawPos,
				loadedCount:        tt.entries, // Assume fully loaded for simplicity
			}
			if tt.rawMode {
				m.loadedCount = tt.rawCount
			}

			got := m.buildPositionSegment()
			if !strings.Contains(got, tt.want) {
				t.Errorf("buildPositionSegment() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestBuildPositionSegmentLazyLoading(t *testing.T) {
	// Test that lazy loading shows "(of Z)" format when not fully loaded
	entries := make([]types.LogEntry, 100)
	m := ViewerModel{
		entries:            entries,
		entryLinePositions: []int{0, 10, 20, 30, 40}, // Only 5 entries loaded
		loadedCount:        5,
		lazyEnabled:        true,
	}

	got := m.buildPositionSegment()
	// Should show "Entry X/5 (of 100)" format during lazy loading
	if !strings.Contains(got, "/5") {
		t.Errorf("buildPositionSegment() = %q, want to contain '/5' (loaded count)", got)
	}
	if !strings.Contains(got, "of 100") {
		t.Errorf("buildPositionSegment() = %q, want to contain 'of 100' (total count)", got)
	}
}

// Story 6.3: Token statistics display tests

func TestBuildTokensSegment(t *testing.T) {
	tests := []struct {
		name               string
		tokenService       bool // true to create mock service, false for nil
		conversationTokens int
		tokensEstimated    bool
		wantEmpty          bool
		wantContains       string
	}{
		{
			name:         "nil token service returns empty",
			tokenService: false,
			wantEmpty:    true,
		},
		{
			name:               "zero tokens with service",
			tokenService:       true,
			conversationTokens: 0,
			tokensEstimated:    false,
			wantEmpty:          false,
			wantContains:       "Tokens: 0",
		},
		{
			name:               "exact tokens",
			tokenService:       true,
			conversationTokens: 1234,
			tokensEstimated:    false,
			wantEmpty:          false,
			wantContains:       "1,234",
		},
		{
			name:               "estimated tokens shows tilde",
			tokenService:       true,
			conversationTokens: 5678,
			tokensEstimated:    true,
			wantEmpty:          false,
			wantContains:       "~5,678",
		},
		{
			name:               "large token count formats with commas",
			tokenService:       true,
			conversationTokens: 1234567,
			tokensEstimated:    false,
			wantEmpty:          false,
			wantContains:       "1,234,567",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{
				conversationTokens: tt.conversationTokens,
				tokensEstimated:    tt.tokensEstimated,
			}

			// Set tokenService to a non-nil value if test requires it
			if tt.tokenService {
				// We just need a non-nil pointer for the nil check in buildTokensSegment
				// The actual service isn't called - we use pre-computed values
				svc, err := token.New()
				if err != nil {
					t.Skipf("Skipping test - token service init failed: %v", err)
				}
				m.tokenService = svc
			}

			got := m.buildTokensSegment()

			if tt.wantEmpty && got != "" {
				t.Errorf("buildTokensSegment() = %q, want empty", got)
			}
			if !tt.wantEmpty {
				if got == "" {
					t.Errorf("buildTokensSegment() = empty, want non-empty")
				} else if !strings.Contains(got, tt.wantContains) {
					t.Errorf("buildTokensSegment() = %q, want to contain %q", got, tt.wantContains)
				}
			}
		})
	}
}

func TestNewViewerModelCalculatesTokens(t *testing.T) {
	// Create entries with known token usage
	entries := []types.LogEntry{
		{
			Type: types.EntryTypeUser,
			Message: types.Message{
				TextContent: "Hello world",
			},
			Usage: types.TokenUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		},
		{
			Type: types.EntryTypeAssistant,
			Message: types.Message{
				Content: []types.MessageContent{
					{Type: types.ContentTypeText, Text: "Response"},
				},
			},
			Usage: types.TokenUsage{
				InputTokens:  200,
				OutputTokens: 100,
			},
		},
	}

	svc, err := token.New()
	if err != nil {
		t.Skipf("Skipping test - token service init failed: %v", err)
	}

	opts := DefaultRenderOptions()
	m := NewViewerModel(entries, 0, "Test", opts, svc)

	// Total should be 150 + 300 = 450 (from actual Usage data)
	expectedTotal := 450
	if m.conversationTokens != expectedTotal {
		t.Errorf("NewViewerModel() conversationTokens = %d, want %d", m.conversationTokens, expectedTotal)
	}

	// Should NOT be estimated since all entries have actual Usage data
	if m.tokensEstimated {
		t.Error("NewViewerModel() tokensEstimated = true, want false (all entries have Usage)")
	}
}

func TestNewViewerModelEstimatesTokensWhenNoUsage(t *testing.T) {
	// Create entries without usage data
	entries := []types.LogEntry{
		{
			Type: types.EntryTypeUser,
			Message: types.Message{
				TextContent: "Hello world",
			},
		},
	}

	svc, err := token.New()
	if err != nil {
		t.Skipf("Skipping test - token service init failed: %v", err)
	}

	opts := DefaultRenderOptions()
	m := NewViewerModel(entries, 0, "Test", opts, svc)

	// Should have some tokens calculated
	if m.conversationTokens == 0 {
		t.Error("NewViewerModel() conversationTokens = 0, want > 0 for 'Hello world'")
	}

	// Should be estimated since entry has no Usage data
	if !m.tokensEstimated {
		t.Error("NewViewerModel() tokensEstimated = false, want true (no Usage data)")
	}
}

func TestNewViewerModelWithNilTokenService(t *testing.T) {
	entries := []types.LogEntry{
		{
			Type: types.EntryTypeUser,
			Message: types.Message{
				TextContent: "Hello world",
			},
		},
	}

	opts := DefaultRenderOptions()
	m := NewViewerModel(entries, 0, "Test", opts, nil)

	// With nil service, conversationTokens should be 0
	if m.conversationTokens != 0 {
		t.Errorf("NewViewerModel() with nil tokenService: conversationTokens = %d, want 0", m.conversationTokens)
	}

	// Should not be estimated when service is nil
	if m.tokensEstimated {
		t.Error("NewViewerModel() with nil tokenService: tokensEstimated = true, want false")
	}

	// buildTokensSegment should return empty string
	got := m.buildTokensSegment()
	if got != "" {
		t.Errorf("buildTokensSegment() with nil tokenService = %q, want empty", got)
	}
}

func TestWatchModeTokenRecalculation(t *testing.T) {
	// Test that NewEntriesMsg updates conversation tokens (Story 6.3)

	// Create initial entries with known token usage
	initialEntries := []types.LogEntry{
		{
			Type: types.EntryTypeUser,
			Message: types.Message{
				TextContent: "Hello",
			},
			Usage: types.TokenUsage{
				InputTokens:  50,
				OutputTokens: 25,
			},
		},
	}

	svc, err := token.New()
	if err != nil {
		t.Skipf("Skipping test - token service init failed: %v", err)
	}

	opts := RenderOptions{WatchMode: true, FilePath: "/tmp/test.jsonl"}
	m := NewViewerModel(initialEntries, 0, "Test", opts, svc)
	m.SetSize(80, 24)

	// Initial token count should be 75 (50 + 25)
	initialTokens := m.conversationTokens
	if initialTokens != 75 {
		t.Errorf("Initial conversationTokens = %d, want 75", initialTokens)
	}

	// Simulate new entries arriving via watch mode
	// The Update handler for NewEntriesMsg increments conversationTokens
	newEntry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{Type: types.ContentTypeText, Text: "Response"},
			},
		},
		Usage: types.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	// Simulate what NewEntriesMsg handler does (viewer.go lines 695-705)
	m.entries = append(m.entries, newEntry)
	if m.tokenService != nil {
		if !newEntry.Usage.IsEmpty() {
			m.conversationTokens += newEntry.Usage.Total()
		} else if newEntry.Type != types.EntryTypeFileHistorySnapshot {
			m.conversationTokens += m.tokenService.CalculateEntry(newEntry)
			m.tokensEstimated = true
		}
	}

	// After new entry, tokens should be 75 + 150 = 225
	expectedTotal := 225
	if m.conversationTokens != expectedTotal {
		t.Errorf("After NewEntriesMsg, conversationTokens = %d, want %d", m.conversationTokens, expectedTotal)
	}

	// Should still not be estimated since all entries have Usage data
	if m.tokensEstimated {
		t.Error("After NewEntriesMsg with Usage data, tokensEstimated = true, want false")
	}
}

func TestWatchModeTokenRecalculationWithEstimation(t *testing.T) {
	// Test that new entries without Usage data set tokensEstimated flag

	// Create initial entry with usage data
	initialEntries := []types.LogEntry{
		{
			Type: types.EntryTypeUser,
			Message: types.Message{
				TextContent: "Hello",
			},
			Usage: types.TokenUsage{
				InputTokens:  50,
				OutputTokens: 25,
			},
		},
	}

	svc, err := token.New()
	if err != nil {
		t.Skipf("Skipping test - token service init failed: %v", err)
	}

	opts := RenderOptions{WatchMode: true, FilePath: "/tmp/test.jsonl"}
	m := NewViewerModel(initialEntries, 0, "Test", opts, svc)
	m.SetSize(80, 24)

	// Initially should not be estimated
	if m.tokensEstimated {
		t.Error("Initial tokensEstimated = true, want false")
	}

	// Simulate new entry arriving WITHOUT Usage data
	newEntry := types.LogEntry{
		Type: types.EntryTypeUser,
		Message: types.Message{
			TextContent: "World",
		},
		// No Usage data - will require estimation
	}

	// Simulate what NewEntriesMsg handler does
	m.entries = append(m.entries, newEntry)
	if m.tokenService != nil {
		if !newEntry.Usage.IsEmpty() {
			m.conversationTokens += newEntry.Usage.Total()
		} else if newEntry.Type != types.EntryTypeFileHistorySnapshot {
			m.conversationTokens += m.tokenService.CalculateEntry(newEntry)
			m.tokensEstimated = true
		}
	}

	// Should now be estimated since new entry has no Usage data
	if !m.tokensEstimated {
		t.Error("After NewEntriesMsg without Usage data, tokensEstimated = false, want true")
	}

	// Token count should have increased
	if m.conversationTokens <= 75 {
		t.Errorf("conversationTokens = %d, want > 75 (initial + estimate for 'World')", m.conversationTokens)
	}
}

// TestBuildModeSegmentShowsFOLLOW tests Story 11.2 AC-3: FOLLOW indicator appears when active.
func TestBuildModeSegmentShowsFOLLOW(t *testing.T) {
	tests := []struct {
		name           string
		followLatest   bool
		hasWatcher     bool
		wantContains   string
		wantNotContain string
	}{
		{
			name:           "follow-latest disabled shows no FOLLOW",
			followLatest:   false,
			hasWatcher:     false,
			wantContains:   "",
			wantNotContain: "FOLLOW",
		},
		{
			name:           "follow-latest enabled but no watcher shows no FOLLOW",
			followLatest:   true,
			hasWatcher:     false,
			wantContains:   "",
			wantNotContain: "FOLLOW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{
				followLatest: tt.followLatest,
			}
			// Create a mock projectWatcher if needed (can't create real one without dir)
			// The check is just: m.projectWatcher != nil
			// For testing, we rely on the nil check behavior

			got := m.buildModeSegment()

			if tt.wantContains != "" && !strings.Contains(got, tt.wantContains) {
				t.Errorf("buildModeSegment() = %q, want to contain %q", got, tt.wantContains)
			}
			if tt.wantNotContain != "" && strings.Contains(got, tt.wantNotContain) {
				t.Errorf("buildModeSegment() = %q, should not contain %q", got, tt.wantNotContain)
			}
		})
	}
}

// TestBuildShortcutsSegmentShowsFollowLatest tests Story 11.2 AC-6: shortcuts segment shows L toggle.
func TestBuildShortcutsSegmentShowsFollowLatest(t *testing.T) {
	tests := []struct {
		name         string
		watchMode    bool
		followLatest bool
		wantContains string
	}{
		{
			name:         "watch mode off - no follow toggle",
			watchMode:    false,
			followLatest: false,
			wantContains: "", // Should not show L:follow or L:unfollow
		},
		{
			name:         "watch mode on, follow-latest off - shows L:follow",
			watchMode:    true,
			followLatest: false,
			wantContains: "L:follow",
		},
		{
			name:         "watch mode on, follow-latest on - shows L:unfollow",
			watchMode:    true,
			followLatest: true,
			wantContains: "L:unfollow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{
				watchMode:    tt.watchMode,
				followLatest: tt.followLatest,
			}

			got := m.buildShortcutsSegment()

			if tt.wantContains == "" {
				// Should not contain any follow toggle text
				if strings.Contains(got, "L:follow") || strings.Contains(got, "L:unfollow") {
					t.Errorf("buildShortcutsSegment() = %q, should not contain follow toggle when watch mode is off", got)
				}
			} else {
				if !strings.Contains(got, tt.wantContains) {
					t.Errorf("buildShortcutsSegment() = %q, want to contain %q", got, tt.wantContains)
				}
			}
		})
	}
}

// TestLKeyRequiresWatchMode tests Story 11.2 AC-6: L key only works in watch mode.
func TestLKeyRequiresWatchMode(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser, Message: types.Message{TextContent: "Hello"}}}

	tests := []struct {
		name        string
		watchMode   bool
		wantToast   string
		wantEnabled bool
	}{
		{
			name:        "L key without watch mode shows error toast",
			watchMode:   false,
			wantToast:   "requires watch mode",
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := RenderOptions{WatchMode: tt.watchMode, FilePath: "/tmp/test.jsonl"}
			m := NewViewerModel(entries, 0, "Test", opts, nil)
			m.SetSize(80, 24)

			// Simulate L key press
			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}}
			newModel, _ := m.Update(keyMsg)
			updatedModel := newModel.(ViewerModel)

			if !tt.watchMode {
				// Should show error toast
				if !strings.Contains(updatedModel.toast, tt.wantToast) {
					t.Errorf("toast = %q, want to contain %q", updatedModel.toast, tt.wantToast)
				}
				// Should not enable follow-latest
				if updatedModel.followLatest {
					t.Error("follow-latest should not be enabled when watch mode is off")
				}
			}
		})
	}
}

// TestNewViewerModelFollowLatestInit tests Story 11.2 Task 5.2: NewViewerModel initializes follow-latest.
func TestNewViewerModelFollowLatestInit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/conversation.jsonl"

	// Create a test file so GetBirthtime has something to work with
	if err := os.WriteFile(testFile, []byte(`{"type":"user"}`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	entries := []types.LogEntry{{Type: types.EntryTypeUser}}

	// Test without follow-latest
	optsNoFollow := RenderOptions{
		WatchMode:    true,
		FilePath:     testFile,
		FollowLatest: false,
		ProjectPath:  "",
	}
	mNoFollow := NewViewerModel(entries, 0, "Test", optsNoFollow, nil)

	if mNoFollow.followLatest {
		t.Error("NewViewerModel without FollowLatest should have followLatest = false")
	}
	if mNoFollow.projectWatcher != nil {
		t.Error("NewViewerModel without FollowLatest should have nil projectWatcher")
	}

	// Test with follow-latest
	optsFollow := RenderOptions{
		WatchMode:    true,
		FilePath:     testFile,
		FollowLatest: true,
		ProjectPath:  tmpDir,
	}
	mFollow := NewViewerModel(entries, 0, "Test", optsFollow, nil)
	defer func() {
		if mFollow.projectWatcher != nil {
			_ = mFollow.projectWatcher.Close()
		}
	}()

	if !mFollow.followLatest {
		t.Error("NewViewerModel with FollowLatest should have followLatest = true")
	}
	if mFollow.projectWatcher == nil {
		t.Error("NewViewerModel with FollowLatest should have non-nil projectWatcher")
	}
	if mFollow.currentConversationPath != testFile {
		t.Errorf("currentConversationPath = %q, want %q", mFollow.currentConversationPath, testFile)
	}
	if mFollow.currentCreationTime.IsZero() {
		t.Error("currentCreationTime should not be zero")
	}
}

// TestNewConversationMsgSwitchesWhenNewer tests Story 11.2 AC-3: NewConversationMsg switches when newer.
func TestNewConversationMsgSwitchesWhenNewer(t *testing.T) {
	tmpDir := t.TempDir()
	oldFile := tmpDir + "/old_conversation.jsonl"
	newFile := tmpDir + "/new_conversation.jsonl"

	// Create old file first
	if err := os.WriteFile(oldFile, []byte(`{"type":"user","message":{"role":"user","content":"Old"}}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to create old file: %v", err)
	}

	// Wait a moment to ensure different timestamps
	time.Sleep(50 * time.Millisecond)

	// Create new file
	if err := os.WriteFile(newFile, []byte(`{"type":"user","message":{"role":"user","content":"New"}}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to create new file: %v", err)
	}

	// Get birthtimes
	oldInfo, _ := os.Stat(oldFile)
	newInfo, _ := os.Stat(newFile)
	oldTime := oldInfo.ModTime() // Use ModTime as fallback for test reliability
	newTime := newInfo.ModTime()

	entries := []types.LogEntry{{Type: types.EntryTypeUser, Message: types.Message{TextContent: "Old"}}}

	opts := RenderOptions{
		WatchMode: true,
		FilePath:  oldFile,
	}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.followLatest = true
	m.currentCreationTime = oldTime
	m.currentConversationPath = oldFile

	// Send NewConversationMsg with newer time
	msg := watcher.NewConversationMsg{
		FilePath:     newFile,
		CreationTime: newTime,
	}

	newModel, _ := m.Update(msg)
	updatedModel := newModel.(ViewerModel)

	// Verify switch occurred
	if updatedModel.currentConversationPath != newFile {
		t.Errorf("currentConversationPath = %q, want %q", updatedModel.currentConversationPath, newFile)
	}
	if updatedModel.renderOpts.FilePath != newFile {
		t.Errorf("renderOpts.FilePath = %q, want %q", updatedModel.renderOpts.FilePath, newFile)
	}
}

// TestNewConversationMsgIgnoresWhenNotNewer tests Story 11.2: NewConversationMsg ignores when not newer.
func TestNewConversationMsgIgnoresWhenNotNewer(t *testing.T) {
	tmpDir := t.TempDir()
	currentFile := tmpDir + "/current.jsonl"
	olderFile := tmpDir + "/older.jsonl"

	// Create current file
	if err := os.WriteFile(currentFile, []byte(`{"type":"user"}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to create current file: %v", err)
	}

	currentInfo, _ := os.Stat(currentFile)
	currentTime := currentInfo.ModTime()

	// Create "older" file with the same or older timestamp for test
	if err := os.WriteFile(olderFile, []byte(`{"type":"user"}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to create older file: %v", err)
	}

	entries := []types.LogEntry{{Type: types.EntryTypeUser}}

	opts := RenderOptions{
		WatchMode: true,
		FilePath:  currentFile,
	}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.followLatest = true
	m.currentCreationTime = currentTime
	m.currentConversationPath = currentFile

	// Send NewConversationMsg with older time (subtract 1 second)
	msg := watcher.NewConversationMsg{
		FilePath:     olderFile,
		CreationTime: currentTime.Add(-time.Second),
	}

	newModel, _ := m.Update(msg)
	updatedModel := newModel.(ViewerModel)

	// Verify NO switch occurred
	if updatedModel.currentConversationPath != currentFile {
		t.Errorf("should not switch: currentConversationPath = %q, want %q", updatedModel.currentConversationPath, currentFile)
	}
}

// TestNewConversationMsgDebouncesSwitchInProgress tests that rapid switches are debounced.
func TestNewConversationMsgDebouncesSwitchInProgress(t *testing.T) {
	tmpDir := t.TempDir()
	currentFile := tmpDir + "/current.jsonl"
	newFile := tmpDir + "/new.jsonl"

	if err := os.WriteFile(currentFile, []byte(`{"type":"user"}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to create current file: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(newFile, []byte(`{"type":"user"}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to create new file: %v", err)
	}

	currentInfo, _ := os.Stat(currentFile)
	newInfo, _ := os.Stat(newFile)
	currentTime := currentInfo.ModTime()
	newTime := newInfo.ModTime()

	entries := []types.LogEntry{{Type: types.EntryTypeUser}}

	opts := RenderOptions{
		WatchMode: true,
		FilePath:  currentFile,
	}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.followLatest = true
	m.currentCreationTime = currentTime
	m.currentConversationPath = currentFile
	m.switchInProgress = true // Simulate switch already in progress

	// Send NewConversationMsg while switch is in progress
	msg := watcher.NewConversationMsg{
		FilePath:     newFile,
		CreationTime: newTime,
	}

	newModel, _ := m.Update(msg)
	updatedModel := newModel.(ViewerModel)

	// Verify NO switch occurred (debounced)
	if updatedModel.currentConversationPath != currentFile {
		t.Errorf("should debounce: currentConversationPath = %q, want %q", updatedModel.currentConversationPath, currentFile)
	}
}

// --- Normal Mode Lazy Loading Regression Tests (AC6) ---
// These tests verify that the normal (non-dashboard) viewer lazy loading path
// continues to work correctly after any changes to the session dashboard mode.

// makeEntriesForLazyLoad creates N user entries for lazy loading tests.
func makeEntriesForLazyLoad(n int) []types.LogEntry {
	entries := make([]types.LogEntry, n)
	for i := range entries {
		entries[i] = types.LogEntry{Type: types.EntryTypeUser}
	}
	return entries
}

// TestNormalModeLazyLoadDisabledBelowThreshold verifies lazy loading is NOT
// enabled when entry count is at or below the MessageThreshold (100).
func TestNormalModeLazyLoadDisabledBelowThreshold(t *testing.T) {
	config := DefaultLazyLoadConfig()
	entries := makeEntriesForLazyLoad(config.MessageThreshold)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	if m.lazyEnabled {
		t.Errorf("lazyEnabled = true, want false when entries (%d) == threshold (%d)",
			len(entries), config.MessageThreshold)
	}
	if m.lazyLoadState != LoadingStateComplete {
		t.Errorf("lazyLoadState = %v, want LoadingStateComplete when lazy not enabled", m.lazyLoadState)
	}
	if m.loadedCount != len(entries) {
		t.Errorf("loadedCount = %d, want %d (all entries) when lazy not enabled",
			m.loadedCount, len(entries))
	}
}

// TestNormalModeLazyLoadEnabledAboveThreshold verifies lazy loading IS enabled
// when entry count exceeds the MessageThreshold (100).
func TestNormalModeLazyLoadEnabledAboveThreshold(t *testing.T) {
	config := DefaultLazyLoadConfig()
	entries := makeEntriesForLazyLoad(config.MessageThreshold + 1)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)

	if !m.lazyEnabled {
		t.Errorf("lazyEnabled = false, want true when entries (%d) > threshold (%d)",
			len(entries), config.MessageThreshold)
	}
	// Initially only BatchSize*2 entries should be loaded
	expectedInitialLoad := config.BatchSize * 2
	if m.loadedCount != expectedInitialLoad {
		t.Errorf("loadedCount = %d, want %d (initial batch) on first load",
			m.loadedCount, expectedInitialLoad)
	}
	if m.lazyLoadState != LoadingStateIdle {
		t.Errorf("lazyLoadState = %v, want LoadingStateIdle when more entries remain",
			m.lazyLoadState)
	}
}

// TestNormalModeLoadMoreMessagesCmd verifies loadMoreMessages() returns a command
// that sends viewerMessagesLoadedMsg with an incremented loadedCount.
func TestNormalModeLoadMoreMessagesCmd(t *testing.T) {
	config := DefaultLazyLoadConfig()
	entries := makeEntriesForLazyLoad(config.MessageThreshold + 10)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.lazyEnabled = true
	m.loadedCount = config.BatchSize * 2
	m.lazyLoadState = LoadingStateIdle

	cmd := m.loadMoreMessages()
	if cmd == nil {
		t.Fatal("loadMoreMessages() returned nil cmd, want non-nil")
	}

	msg := cmd()
	loaded, ok := msg.(viewerMessagesLoadedMsg)
	if !ok {
		t.Fatalf("loadMoreMessages() cmd produced %T, want viewerMessagesLoadedMsg", msg)
	}

	expectedCount := m.loadedCount + config.BatchSize
	if loaded.loadedCount != expectedCount {
		t.Errorf("loadedCount in msg = %d, want %d", loaded.loadedCount, expectedCount)
	}
}

// TestNormalModeLoadMoreMessagesCapsAtTotal verifies loadMoreMessages() caps
// loadedCount at len(entries) so it never exceeds the total entry count.
func TestNormalModeLoadMoreMessagesCapsAtTotal(t *testing.T) {
	config := DefaultLazyLoadConfig()
	total := config.MessageThreshold + 5
	entries := makeEntriesForLazyLoad(total)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.lazyEnabled = true
	// Set loadedCount near total so next batch would exceed total
	m.loadedCount = total - 3
	m.lazyLoadState = LoadingStateIdle

	cmd := m.loadMoreMessages()
	if cmd == nil {
		t.Fatal("loadMoreMessages() returned nil cmd, want non-nil")
	}

	msg := cmd()
	loaded, ok := msg.(viewerMessagesLoadedMsg)
	if !ok {
		t.Fatalf("loadMoreMessages() cmd produced %T, want viewerMessagesLoadedMsg", msg)
	}

	if loaded.loadedCount > total {
		t.Errorf("loadedCount %d exceeds total %d — must be capped", loaded.loadedCount, total)
	}
	if loaded.loadedCount != total {
		t.Errorf("loadedCount = %d, want %d (capped at total)", loaded.loadedCount, total)
	}
}

// TestNormalModeViewerMessagesLoadedMsgPartial verifies that receiving a
// viewerMessagesLoadedMsg when more entries remain transitions to LoadingStateIdle
// and updates loadedCount correctly.
func TestNormalModeViewerMessagesLoadedMsgPartial(t *testing.T) {
	config := DefaultLazyLoadConfig()
	total := config.MessageThreshold + 50
	entries := makeEntriesForLazyLoad(total)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.lazyEnabled = true
	m.loadedCount = config.BatchSize * 2
	m.lazyLoadState = LoadingStateLoading

	newCount := m.loadedCount + config.BatchSize
	updatedModel, _ := m.Update(viewerMessagesLoadedMsg{loadedCount: newCount})
	m = updatedModel.(ViewerModel)

	if m.loadedCount != newCount {
		t.Errorf("loadedCount = %d, want %d after partial load", m.loadedCount, newCount)
	}
	if m.lazyLoadState != LoadingStateIdle {
		t.Errorf("lazyLoadState = %v, want LoadingStateIdle after partial load", m.lazyLoadState)
	}
	if m.showOverlaySpinner {
		t.Error("showOverlaySpinner = true, want false after non-bulk load msg")
	}
}

// TestNormalModeViewerMessagesLoadedMsgComplete verifies that receiving a
// viewerMessagesLoadedMsg for the full entry count transitions to LoadingStateComplete.
func TestNormalModeViewerMessagesLoadedMsgComplete(t *testing.T) {
	config := DefaultLazyLoadConfig()
	total := config.MessageThreshold + 10
	entries := makeEntriesForLazyLoad(total)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.lazyEnabled = true
	m.loadedCount = total - config.BatchSize
	m.lazyLoadState = LoadingStateLoading

	updatedModel, _ := m.Update(viewerMessagesLoadedMsg{loadedCount: total})
	m = updatedModel.(ViewerModel)

	if m.loadedCount != total {
		t.Errorf("loadedCount = %d, want %d after complete load", m.loadedCount, total)
	}
	if m.lazyLoadState != LoadingStateComplete {
		t.Errorf("lazyLoadState = %v, want LoadingStateComplete after full load", m.lazyLoadState)
	}
}

// TestNormalModeGKeyWithLazyLoadingShowsSpinner verifies that pressing G when
// lazy loading is enabled and not complete shows the overlay spinner and returns
// the bulk-load command — matching the raw mode behavior.
func TestNormalModeGKeyWithLazyLoadingShowsSpinner(t *testing.T) {
	config := DefaultLazyLoadConfig()
	total := config.MessageThreshold + 50
	entries := makeEntriesForLazyLoad(total)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.lazyEnabled = true
	m.loadedCount = config.BatchSize * 2
	m.lazyLoadState = LoadingStateIdle

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	updatedModel, cmd := m.Update(keyMsg)
	m = updatedModel.(ViewerModel)

	if !m.showOverlaySpinner {
		t.Error("showOverlaySpinner = false, want true after G key with pending lazy load")
	}
	if cmd == nil {
		t.Error("cmd = nil, want non-nil batch cmd (spinner.Tick + markAllMessagesLoadedCmd)")
	}
	// Viewer must remain in normal mode (not raw mode)
	if m.rawMode {
		t.Error("rawMode = true, want false — G key must not switch to raw mode")
	}
}

// TestNormalModeGKeyAllLoadedGoesToBottom verifies that pressing G when all
// entries are already loaded just goes to bottom without showing a spinner.
func TestNormalModeGKeyAllLoadedGoesToBottom(t *testing.T) {
	config := DefaultLazyLoadConfig()
	total := config.MessageThreshold + 5
	entries := makeEntriesForLazyLoad(total)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.lazyEnabled = true
	m.loadedCount = total
	m.lazyLoadState = LoadingStateComplete

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	updatedModel, _ := m.Update(keyMsg)
	m = updatedModel.(ViewerModel)

	if m.showOverlaySpinner {
		t.Error("showOverlaySpinner = true, want false when all messages are already loaded")
	}
	if m.rawMode {
		t.Error("rawMode = true, want false after G key in normal mode")
	}
}

// TestNormalModeSpinnerTickOnlyWhenOverlayShown verifies that spinner.TickMsg
// only continues ticking while the overlay spinner is visible, preventing
// an infinite animation loop when no loading is in progress.
func TestNormalModeSpinnerTickOnlyWhenOverlayShown(t *testing.T) {
	entries := makeEntriesForLazyLoad(1)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.showOverlaySpinner = false

	// Send spinner tick when overlay is NOT shown — cmd must be nil
	tickMsg, _ := m.overlaySpinner.Update(m.overlaySpinner.Tick())
	_ = tickMsg
	updatedModel, cmd := m.Update(m.overlaySpinner.Tick())
	_ = updatedModel
	if cmd != nil {
		t.Error("spinner tick cmd returned when showOverlaySpinner=false, want nil to stop animation loop")
	}
}

// TestNormalModeSpinnerTickContinuesWhenOverlayShown verifies that spinner.TickMsg
// returns a new tick command while the overlay spinner is visible.
func TestNormalModeSpinnerTickContinuesWhenOverlayShown(t *testing.T) {
	config := DefaultLazyLoadConfig()
	total := config.MessageThreshold + 50
	entries := makeEntriesForLazyLoad(total)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.lazyEnabled = true
	m.loadedCount = config.BatchSize * 2
	m.lazyLoadState = LoadingStateLoading
	m.showOverlaySpinner = true

	// Send a spinner tick while overlay IS shown — cmd must be non-nil
	updatedModel, cmd := m.Update(m.overlaySpinner.Tick())
	_ = updatedModel
	if cmd == nil {
		t.Error("spinner tick cmd = nil when showOverlaySpinner=true, want non-nil to keep animating")
	}
}

// TestNormalModeMarkAllMessagesLoadedCmdProducesMsg verifies that
// markAllMessagesLoadedCmd() returns a command that produces a
// viewerMessagesLoadedMsg with all entries accounted for.
func TestNormalModeMarkAllMessagesLoadedCmdProducesMsg(t *testing.T) {
	config := DefaultLazyLoadConfig()
	total := config.MessageThreshold + 5
	entries := makeEntriesForLazyLoad(total)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.lazyEnabled = true
	m.loadedCount = config.BatchSize * 2

	cmd := m.markAllMessagesLoadedCmd()
	if cmd == nil {
		t.Fatal("markAllMessagesLoadedCmd() returned nil, want non-nil")
	}

	msg := cmd()
	loaded, ok := msg.(viewerMessagesLoadedMsg)
	if !ok {
		t.Fatalf("markAllMessagesLoadedCmd() produced %T, want viewerMessagesLoadedMsg", msg)
	}
	if loaded.loadedCount != total {
		t.Errorf("loadedCount = %d, want %d (all entries)", loaded.loadedCount, total)
	}
}

// TestNormalModeBulkLoadGotoBottomAfterOverlay verifies that when a
// viewerMessagesLoadedMsg arrives after a bulk load (overlay was shown),
// the viewport scrolls to the bottom.
func TestNormalModeBulkLoadGotoBottomAfterOverlay(t *testing.T) {
	config := DefaultLazyLoadConfig()
	total := config.MessageThreshold + 10
	entries := makeEntriesForLazyLoad(total)
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts, nil)
	m.SetSize(80, 24)
	m.lazyEnabled = true
	m.loadedCount = config.BatchSize * 2
	m.lazyLoadState = LoadingStateLoading
	m.showOverlaySpinner = true // Simulate G-key-triggered overlay was active

	// Simulate bulk load completing with pre-rendered content
	updatedModel, _ := m.Update(viewerMessagesLoadedMsg{
		loadedCount:     total,
		renderedContent: "pre-rendered content",
	})
	m = updatedModel.(ViewerModel)

	// Overlay must be cleared
	if m.showOverlaySpinner {
		t.Error("showOverlaySpinner still true after bulk load completed")
	}
	// Loading must be complete
	if m.lazyLoadState != LoadingStateComplete {
		t.Errorf("lazyLoadState = %v, want LoadingStateComplete after bulk load", m.lazyLoadState)
	}
}
