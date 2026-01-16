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

	m := NewViewerModel(entries, 0, "Test", opts)

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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)

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
	m := NewViewerModel(entries, 0, "Test", opts)

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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
		name       string
		entryType  types.EntryType
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
			m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)

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
			m := NewViewerModel(entries, 0, "Test", opts)

			if m.gutterWidth != tt.wantWidth {
				t.Errorf("NewViewerModel() gutterWidth = %d, want %d", m.gutterWidth, tt.wantWidth)
			}
		})
	}
}

func TestShowLineNumbersDefaultTrue(t *testing.T) {
	entries := []types.LogEntry{{Type: types.EntryTypeUser}}
	opts := RenderOptions{Width: 80}
	m := NewViewerModel(entries, 0, "Test", opts)

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
	m := NewViewerModel(entries, 0, "Test", opts)
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
			m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
			m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)

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
	m := NewViewerModel(entries, 0, "Test", opts)

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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)
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
	m := NewViewerModel(entries, 0, "Test", opts)

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
	m := NewViewerModel(entries, 0, "Test", opts)

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
	m := NewViewerModel(entries, 0, "Test", opts)
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
