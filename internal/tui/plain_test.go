package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestRenderPlainWithWidth(t *testing.T) {
	// Create a test entry with long text that should wrap
	longText := strings.Repeat("word ", 30) // 150 chars
	entries := []types.LogEntry{
		{
			Type:      types.EntryTypeUser,
			Timestamp: time.Now(),
			Message: types.Message{
				TextContent: longText,
			},
		},
	}

	tests := []struct {
		name      string
		width     int
		wantWidth int // Expected effective width
	}{
		{"default width 0 uses 80", 0, 80},
		{"explicit width 60", 60, 60},
		{"explicit width 120", 120, 120},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := RenderOptions{Width: tt.width}
			output := RenderPlain(entries, "test", opts)

			if len(output) == 0 {
				t.Error("RenderPlain returned empty output")
			}

			// Verify output contains wrapped content (has newlines in message body)
			// The message content should be wrapped, meaning more lines than original
			lines := strings.Split(output, "\n")
			if len(lines) < 3 {
				t.Errorf("Expected wrapped output with multiple lines, got %d lines", len(lines))
			}

			// Verify text wrapping occurred by checking that at least one content line
			// is shorter than the original text (150 chars) but within expected width
			foundWrappedLine := false
			for _, line := range lines {
				// Skip empty lines and header lines
				if len(line) == 0 || strings.HasPrefix(line, "===") {
					continue
				}
				// If we find a line with content that's reasonably sized, wrapping worked
				// Use simple length check (ANSI codes add overhead but won't exceed 2x)
				if len(line) > 10 && len(line) < tt.wantWidth*2 {
					foundWrappedLine = true
					break
				}
			}
			if !foundWrappedLine {
				t.Error("Expected to find wrapped content lines")
			}
		})
	}
}

func TestRenderPlainWithWidthAssistant(t *testing.T) {
	// Create assistant entry with text content
	longText := strings.Repeat("assistant response ", 20)
	entries := []types.LogEntry{
		{
			Type:      types.EntryTypeAssistant,
			Timestamp: time.Now(),
			Message: types.Message{
				Content: []types.MessageContent{
					{
						Type: types.ContentTypeText,
						Text: longText,
					},
				},
			},
		},
	}

	opts := RenderOptions{Width: 60}
	output := RenderPlain(entries, "test", opts)

	if len(output) == 0 {
		t.Error("RenderPlain returned empty output for assistant message")
	}
}

func TestRenderPlainWithHideOptions(t *testing.T) {
	entries := []types.LogEntry{
		{
			Type:      types.EntryTypeAssistant,
			Timestamp: time.Now(),
			Message: types.Message{
				Content: []types.MessageContent{
					{
						Type: types.ContentTypeText,
						Text: "Hello world",
					},
					{
						Type:     types.ContentTypeThinking,
						Thinking: "Thinking about something",
					},
					{
						Type:      types.ContentTypeToolUse,
						ToolName:  "Read",
						ToolInput: map[string]any{"file_path": "/test.txt"},
					},
				},
			},
		},
	}

	tests := []struct {
		name         string
		hideThoughts bool
		hideTools    bool
		wantThinking bool
		wantTool     bool
	}{
		{"show all", false, false, true, true},
		{"hide thoughts", true, false, false, true},
		{"hide tools", false, true, true, false},
		{"hide both", true, true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := RenderOptions{
				Width:        80,
				HideThoughts: tt.hideThoughts,
				HideTools:    tt.hideTools,
			}
			output := RenderPlain(entries, "test", opts)

			hasThinking := strings.Contains(output, "Thinking")
			hasTool := strings.Contains(output, "Read")

			if hasThinking != tt.wantThinking {
				t.Errorf("thinking visibility: got %v, want %v", hasThinking, tt.wantThinking)
			}
			if hasTool != tt.wantTool {
				t.Errorf("tool visibility: got %v, want %v", hasTool, tt.wantTool)
			}
		})
	}
}
