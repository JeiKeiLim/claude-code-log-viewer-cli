package tui

import "testing"

func TestVisualWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		{"korean", "안녕", 4},
		{"japanese", "日本語", 6},
		{"mixed ascii and korean", "Hello안녕", 9},
		{"single space", " ", 1},
		{"tabs", "\t", 0}, // tab has no visual width in lipgloss
		{"numbers", "12345", 5},
		{"punctuation", "!@#$%", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VisualWidth(tt.input)
			if got != tt.want {
				t.Errorf("VisualWidth(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{"empty string", "", 10, ""},
		{"zero width", "hello", 0, ""},
		{"negative width", "hello", -1, ""},
		{"width 1", "hello", 1, "h"},
		{"width 2", "hello", 2, "he"},
		{"width 3", "hello", 3, "hel"},
		{"fits exactly", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"korean fits", "안녕", 4, "안녕"},
		{"korean truncate", "안녕하세요", 8, "안녕..."},
		{"mixed truncate", "Hello안녕World", 10, "Hello안..."},
		{"very long", "This is a very long string that needs truncation", 20, "This is a very lo..."},
		{"width exactly 4 for ellipsis", "hello", 4, "h..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateToWidth(tt.input, tt.maxWidth)
			gotWidth := VisualWidth(got)
			if gotWidth > tt.maxWidth && tt.maxWidth > 0 {
				t.Errorf("TruncateToWidth(%q, %d) = %q (width %d), exceeds maxWidth",
					tt.input, tt.maxWidth, got, gotWidth)
			}
			// For deterministic cases, check exact output
			if tt.want != "" && got != tt.want {
				t.Errorf("TruncateToWidth(%q, %d) = %q, want %q",
					tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestTruncateFromLeftToWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{"empty string", "", 10, ""},
		{"zero width", "hello", 0, ""},
		{"negative width", "hello", -1, ""},
		{"width 1", "hello", 1, "."},
		{"width 2", "hello", 2, ".."},
		{"width 3", "hello", 3, "..."},
		{"fits exactly", "hello", 5, "hello"},
		{"needs truncation", "/path/to/file", 10, "...to/file"},
		{"korean path fits", "/경로/파일", 10, "/경로/파일"}, // 1+2+2+1+2+2=10, fits exactly
		{"very long path", "/Users/john/projects/myapp/src/file.go", 20, "...myapp/src/file.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateFromLeftToWidth(tt.input, tt.maxWidth)
			gotWidth := VisualWidth(got)
			if gotWidth > tt.maxWidth && tt.maxWidth > 0 {
				t.Errorf("TruncateFromLeftToWidth(%q, %d) = %q (width %d), exceeds maxWidth",
					tt.input, tt.maxWidth, got, gotWidth)
			}
			// Verify exact output for deterministic cases
			if tt.want != "" && got != tt.want {
				t.Errorf("TruncateFromLeftToWidth(%q, %d) = %q, want %q",
					tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestPadToWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  int // expected visual width
	}{
		{"empty to 5", "", 5, 5},
		{"short to 10", "hello", 10, 10},
		{"exact width", "hello", 5, 5},
		{"longer than width", "hello world", 5, 11}, // no truncation, just returns as-is
		{"korean", "안녕", 10, 10},
		{"mixed", "Hi안녕", 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadToWidth(tt.input, tt.width)
			gotWidth := VisualWidth(got)
			if gotWidth != tt.want {
				t.Errorf("PadToWidth(%q, %d) has width %d, want %d",
					tt.input, tt.width, gotWidth, tt.want)
			}
		})
	}
}

func TestTruncateToWidthEdgeCases(t *testing.T) {
	// Test that CJK character at boundary is handled correctly
	t.Run("CJK at boundary", func(t *testing.T) {
		// "A안녕" = 1 + 2 + 2 = 5 columns
		input := "A안녕"
		got := TruncateToWidth(input, 4)
		// Should truncate to fit, can't include second 안 (width 2) + ... (width 3)
		gotWidth := VisualWidth(got)
		if gotWidth > 4 {
			t.Errorf("Width %d exceeds max 4: %q", gotWidth, got)
		}
	})

	t.Run("only ellipsis fits", func(t *testing.T) {
		got := TruncateToWidth("안녕하세요", 3)
		// Width 3 can only fit "..."
		if got != "..." {
			// Or a single-width char if available
			if VisualWidth(got) > 3 {
				t.Errorf("Width exceeds 3: %q (width %d)", got, VisualWidth(got))
			}
		}
	})
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		wantMax  int // max width per line
	}{
		{"empty string", "", 20, 0},
		{"zero width", "hello world", 0, 11}, // returns as-is
		{"negative width", "hello world", -1, 11},
		{"fits in width", "hello", 20, 5},
		{"needs wrap", "hello world test", 10, 10},
		{"preserves newlines", "hello\nworld", 20, 5},
		{"wraps long lines", "this is a very long line that needs wrapping", 15, 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WrapText(tt.input, tt.maxWidth)
			// Check that no line exceeds maxWidth (for positive maxWidth)
			if tt.maxWidth > 0 {
				lines := splitLines(got)
				for _, line := range lines {
					lineWidth := VisualWidth(line)
					if lineWidth > tt.maxWidth {
						t.Errorf("WrapText(%q, %d) produced line %q with width %d > %d",
							tt.input, tt.maxWidth, line, lineWidth, tt.maxWidth)
					}
				}
			}
		})
	}
}

func TestWrapTextWithCJK(t *testing.T) {
	t.Run("korean text wrapping", func(t *testing.T) {
		// "안녕하세요" = 2+2+2+2+2 = 10 columns
		input := "안녕하세요 테스트입니다"
		got := WrapText(input, 12)
		lines := splitLines(got)
		for _, line := range lines {
			if VisualWidth(line) > 12 {
				t.Errorf("Line width %d exceeds 12: %q", VisualWidth(line), line)
			}
		}
	})

	t.Run("mixed content wrapping", func(t *testing.T) {
		input := "Hello 안녕 World 세계"
		got := WrapText(input, 10)
		lines := splitLines(got)
		for _, line := range lines {
			if VisualWidth(line) > 10 {
				t.Errorf("Line width %d exceeds 10: %q", VisualWidth(line), line)
			}
		}
	})
}

func TestBreakLongWord(t *testing.T) {
	t.Run("breaks long ascii word", func(t *testing.T) {
		input := "supercalifragilisticexpialidocious"
		got := breakLongWord(input, 10)
		lines := splitLines(got)
		for _, line := range lines {
			if VisualWidth(line) > 10 {
				t.Errorf("Line width %d exceeds 10: %q", VisualWidth(line), line)
			}
		}
	})

	t.Run("breaks long korean word", func(t *testing.T) {
		input := "안녕하세요감사합니다"
		got := breakLongWord(input, 8)
		lines := splitLines(got)
		for _, line := range lines {
			if VisualWidth(line) > 8 {
				t.Errorf("Line width %d exceeds 8: %q", VisualWidth(line), line)
			}
		}
	})
}

// splitLines is a helper to split text into lines for testing
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
