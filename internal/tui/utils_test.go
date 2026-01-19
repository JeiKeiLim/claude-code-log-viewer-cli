// Package tui provides the terminal user interface components.
package tui

import (
	"testing"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/token"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestFormatWithCommas(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{"zero", 0, "0"},
		{"single digit", 5, "5"},
		{"two digits", 42, "42"},
		{"three digits", 999, "999"},
		{"one thousand", 1000, "1,000"},
		{"four digits", 1234, "1,234"},
		{"five digits", 12345, "12,345"},
		{"six digits", 123456, "123,456"},
		{"seven digits", 1234567, "1,234,567"},
		{"eight digits", 12345678, "12,345,678"},
		{"million", 1000000, "1,000,000"},
		{"near million", 999999, "999,999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatWithCommas(tt.n)
			if got != tt.want {
				t.Errorf("formatWithCommas(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestFormatTokenUsage(t *testing.T) {
	// Create a real token service for testing
	tokenSvc, err := token.New()
	if err != nil {
		t.Skipf("Skipping token tests - service init failed: %v", err)
	}

	tests := []struct {
		name      string
		entry     types.LogEntry
		svc       *token.Service
		wantEmpty bool
		contains  string // Substring to check for if not empty
	}{
		{
			name: "file-history-snapshot returns empty",
			entry: types.LogEntry{
				Type: types.EntryTypeFileHistorySnapshot,
			},
			svc:       tokenSvc,
			wantEmpty: true,
		},
		{
			name: "nil service returns empty",
			entry: types.LogEntry{
				Type: types.EntryTypeUser,
				Message: types.Message{
					TextContent: "Hello world",
				},
			},
			svc:       nil,
			wantEmpty: true,
		},
		{
			name: "user message with actual usage",
			entry: types.LogEntry{
				Type: types.EntryTypeUser,
				Message: types.Message{
					TextContent: "Hello world",
				},
				Usage: types.TokenUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
			svc:       tokenSvc,
			wantEmpty: false,
			contains:  "(from log)",
		},
		{
			name: "user message with estimated usage",
			entry: types.LogEntry{
				Type: types.EntryTypeUser,
				Message: types.Message{
					TextContent: "Hello world",
				},
			},
			svc:       tokenSvc,
			wantEmpty: false,
			contains:  "(estimated)",
		},
		{
			name: "assistant message with actual usage",
			entry: types.LogEntry{
				Type: types.EntryTypeAssistant,
				Message: types.Message{
					Content: []types.MessageContent{
						{Type: types.ContentTypeText, Text: "Response text"},
					},
				},
				Usage: types.TokenUsage{
					InputTokens:  200,
					OutputTokens: 100,
				},
			},
			svc:       tokenSvc,
			wantEmpty: false,
			contains:  "300",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTokenUsage(tt.entry, tt.svc)
			if tt.wantEmpty && got != "" {
				t.Errorf("formatTokenUsage() = %q, want empty", got)
			}
			if !tt.wantEmpty {
				if got == "" {
					t.Errorf("formatTokenUsage() = empty, want non-empty containing %q", tt.contains)
				} else if tt.contains != "" && !contains(got, tt.contains) {
					t.Errorf("formatTokenUsage() = %q, want to contain %q", got, tt.contains)
				}
			}
		})
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
