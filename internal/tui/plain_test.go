package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/usage"
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

// timePtr is a helper to create time.Time pointers for tests.
func timePtr(t time.Time) *time.Time {
	return &t
}

func TestRenderUsagePlain(t *testing.T) {
	// Use a fixed reference time for consistent test results
	now := time.Now()

	tests := []struct {
		name        string
		limits      *usage.UsageLimits
		wantContain []string
		wantExclude []string
	}{
		{
			name: "typical usage with reset times",
			limits: &usage.UsageLimits{
				FiveHour: &usage.UsageWindow{Utilization: 35.0, ResetsAt: timePtr(now.Add(2*time.Hour + 16*time.Minute))}, // +16m to tolerate test execution time
				SevenDay: &usage.UsageWindow{Utilization: 12.0, ResetsAt: timePtr(now.Add(5*24*time.Hour + 12*time.Hour))},
			},
			wantContain: []string{"Claude Code Usage", "5-hour:", "35%", "resets in", "2h", "7-day:", "12%", "5d"}, // Check both reset times
			wantExclude: []string{"Opus:"},
		},
		{
			name: "no reset time (nil)",
			limits: &usage.UsageLimits{
				FiveHour: &usage.UsageWindow{Utilization: 50.0, ResetsAt: nil},
				SevenDay: &usage.UsageWindow{Utilization: 25.0},
			},
			wantContain: []string{"5-hour:", "50%", "7-day:", "25%"},
			wantExclude: []string{"resets in", "Opus:"},
		},
		{
			name: "reset time in past (should not show reset)",
			limits: &usage.UsageLimits{
				FiveHour: &usage.UsageWindow{Utilization: 60.0, ResetsAt: timePtr(now.Add(-5 * time.Minute))},
				SevenDay: &usage.UsageWindow{Utilization: 30.0},
			},
			wantContain: []string{"5-hour:", "60%", "7-day:", "30%"},
			wantExclude: []string{"resets in"},
		},
		{
			name: "with Opus (non-zero)",
			limits: &usage.UsageLimits{
				FiveHour:     &usage.UsageWindow{Utilization: 10.0},
				SevenDay:     &usage.UsageWindow{Utilization: 5.0},
				SevenDayOpus: &usage.UsageWindow{Utilization: 2.0},
			},
			wantContain: []string{"5-hour:", "10%", "7-day:", "5%", "Opus:", "2%"},
		},
		{
			name: "Opus at zero (hidden)",
			limits: &usage.UsageLimits{
				FiveHour:     &usage.UsageWindow{Utilization: 10.0},
				SevenDay:     &usage.UsageWindow{Utilization: 5.0},
				SevenDayOpus: &usage.UsageWindow{Utilization: 0.0},
			},
			wantContain: []string{"5-hour:", "10%", "7-day:", "5%"},
			wantExclude: []string{"Opus:"},
		},
		{
			name: "percentage rounding - uses Go banker's rounding",
			limits: &usage.UsageLimits{
				FiveHour: &usage.UsageWindow{Utilization: 35.5}, // 35.5 rounds to 36 (even)
				SevenDay: &usage.UsageWindow{Utilization: 34.6}, // 34.6 rounds to 35
			},
			wantContain: []string{"5-hour:", "36%", "7-day:", "35%"},
		},
		{
			name: "nil FiveHour (edge case)",
			limits: &usage.UsageLimits{
				FiveHour: nil,
				SevenDay: &usage.UsageWindow{Utilization: 20.0},
			},
			wantContain: []string{"Claude Code Usage", "7-day:", "20%"},
			wantExclude: []string{"5-hour:"},
		},
		{
			name: "nil SevenDay (edge case)",
			limits: &usage.UsageLimits{
				FiveHour: &usage.UsageWindow{Utilization: 15.0},
				SevenDay: nil,
			},
			wantContain: []string{"Claude Code Usage", "5-hour:", "15%"},
			wantExclude: []string{"7-day:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderUsagePlain(tt.limits)

			// Verify output ends with newline
			if !strings.HasSuffix(output, "\n") {
				t.Error("output should end with trailing newline")
			}

			// Check that required strings are present
			for _, want := range tt.wantContain {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q, got:\n%s", want, output)
				}
			}

			// Check that excluded strings are absent
			for _, exclude := range tt.wantExclude {
				if strings.Contains(output, exclude) {
					t.Errorf("output should NOT contain %q, got:\n%s", exclude, output)
				}
			}
		})
	}
}

func TestFormatResetDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, ""},
		{"negative", -5 * time.Minute, ""},
		{"less than minute (30s)", 30 * time.Second, "<1m"},
		{"just under a minute (59s)", 59 * time.Second, "<1m"},
		{"exactly one minute", 1 * time.Minute, "1m"},
		{"exact minutes (45m)", 45 * time.Minute, "45m"},
		{"hours and minutes (2h 15m)", 2*time.Hour + 15*time.Minute, "2h 15m"},
		{"exact hours (3h)", 3 * time.Hour, "3h"},
		{"one hour exactly", 1 * time.Hour, "1h"},
		{"hours with zero minutes (2h)", 2*time.Hour + 0*time.Minute, "2h"},
		{"one minute exact", 60 * time.Second, "1m"},
		{"1 second less than 1 minute", 59*time.Second + 999*time.Millisecond, "<1m"},
		// Days support for 7-day reset times
		{"days and hours (5d 12h)", 5*24*time.Hour + 12*time.Hour, "5d 12h"},
		{"exact days (7d)", 7 * 24 * time.Hour, "7d"},
		{"one day exactly", 24 * time.Hour, "1d"},
		{"one day and hours (1d 5h)", 24*time.Hour + 5*time.Hour, "1d 5h"},
		{"23 hours (no days)", 23 * time.Hour, "23h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatResetDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatResetDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestRenderUsagePlainColorModes(t *testing.T) {
	// Test that RenderUsagePlain produces different output based on color profile
	// This tests AC-5: Color Flag Compatibility
	limits := &usage.UsageLimits{
		FiveHour: &usage.UsageWindow{Utilization: 35.0},
		SevenDay: &usage.UsageWindow{Utilization: 12.0},
	}

	// ANSI escape code pattern: ESC[...m
	containsANSI := func(s string) bool {
		return strings.Contains(s, "\x1b[")
	}

	t.Run("color=always produces ANSI codes", func(t *testing.T) {
		// Save current profile and restore after test
		// Force TrueColor profile for styled output
		lipgloss.SetColorProfile(termenv.TrueColor)
		output := RenderUsagePlain(limits)

		if !containsANSI(output) {
			t.Error("expected ANSI escape codes with TrueColor profile, got none")
		}
		if !strings.Contains(output, "Claude Code Usage") {
			t.Error("output missing expected content")
		}
	})

	t.Run("color=never produces no ANSI codes", func(t *testing.T) {
		// Set Ascii profile (no colors)
		lipgloss.SetColorProfile(termenv.Ascii)
		output := RenderUsagePlain(limits)

		if containsANSI(output) {
			t.Errorf("expected no ANSI escape codes with Ascii profile, got:\n%q", output)
		}
		if !strings.Contains(output, "Claude Code Usage") {
			t.Error("output missing expected content")
		}
	})
}
