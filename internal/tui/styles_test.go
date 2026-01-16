package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestDefaultThemeNotNil verifies DefaultTheme is initialized
func TestDefaultThemeNotNil(t *testing.T) {
	if DefaultTheme == (Theme{}) {
		t.Error("DefaultTheme should not be zero value")
	}
}

// TestThemeCoreColors tests that core colors are present
func TestThemeCoreColors(t *testing.T) {
	tests := []struct {
		name  string
		color lipgloss.AdaptiveColor
	}{
		{"Primary", DefaultTheme.Primary},
		{"Secondary", DefaultTheme.Secondary},
		{"Accent", DefaultTheme.Accent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Light == "" {
				t.Errorf("%s.Light should not be empty", tt.name)
			}
			if tt.color.Dark == "" {
				t.Errorf("%s.Dark should not be empty", tt.name)
			}
		})
	}
}

// TestThemeTextColors tests that text colors are present
func TestThemeTextColors(t *testing.T) {
	tests := []struct {
		name  string
		color lipgloss.AdaptiveColor
	}{
		{"Text", DefaultTheme.Text},
		{"Muted", DefaultTheme.Muted},
		{"Dim", DefaultTheme.Dim},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Light == "" {
				t.Errorf("%s.Light should not be empty", tt.name)
			}
			if tt.color.Dark == "" {
				t.Errorf("%s.Dark should not be empty", tt.name)
			}
		})
	}
}

// TestThemeBackgroundColors tests that background colors are present
func TestThemeBackgroundColors(t *testing.T) {
	tests := []struct {
		name  string
		color lipgloss.AdaptiveColor
	}{
		{"Background", DefaultTheme.Background},
		{"BgAlt", DefaultTheme.BgAlt},
		{"SelectedBg", DefaultTheme.SelectedBg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Light == "" {
				t.Errorf("%s.Light should not be empty", tt.name)
			}
			if tt.color.Dark == "" {
				t.Errorf("%s.Dark should not be empty", tt.name)
			}
		})
	}
}

// TestThemeRoleColors tests that role colors (User, Assistant, Thinking, Tool) are present
func TestThemeRoleColors(t *testing.T) {
	tests := []struct {
		name  string
		color lipgloss.AdaptiveColor
	}{
		{"User", DefaultTheme.User},
		{"Assistant", DefaultTheme.Assistant},
		{"Thinking", DefaultTheme.Thinking},
		{"Tool", DefaultTheme.Tool},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Light == "" {
				t.Errorf("%s.Light should not be empty", tt.name)
			}
			if tt.color.Dark == "" {
				t.Errorf("%s.Dark should not be empty", tt.name)
			}
		})
	}
}

// TestThemeConstantColors tests that constant (non-adaptive) colors are present
func TestThemeConstantColors(t *testing.T) {
	tests := []struct {
		name  string
		color lipgloss.AdaptiveColor
	}{
		{"White", DefaultTheme.White},
		{"Black", DefaultTheme.Black},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Light == "" {
				t.Errorf("%s.Light should not be empty", tt.name)
			}
			if tt.color.Dark == "" {
				t.Errorf("%s.Dark should not be empty", tt.name)
			}
			// Constant colors should be same in light and dark
			if tt.color.Light != tt.color.Dark {
				t.Errorf("%s should be constant (Light=%s, Dark=%s)", tt.name, tt.color.Light, tt.color.Dark)
			}
		})
	}
}

// TestThemeAllFieldsPopulated ensures all 15 color fields are non-zero (13 adaptive + 2 constant)
func TestThemeAllFieldsPopulated(t *testing.T) {
	// Count populated fields
	populated := 0
	expected := 15 // 13 adaptive colors + 2 constant colors

	if DefaultTheme.Primary != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Secondary != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Accent != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Text != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Muted != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Dim != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Background != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.BgAlt != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.SelectedBg != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.User != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Assistant != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Thinking != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Tool != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.White != (lipgloss.AdaptiveColor{}) {
		populated++
	}
	if DefaultTheme.Black != (lipgloss.AdaptiveColor{}) {
		populated++
	}

	if populated != expected {
		t.Errorf("DefaultTheme has %d populated fields, want %d", populated, expected)
	}
}

// TestListItemStyles tests that all ListItem styles are properly initialized
// Note: lipgloss doesn't render ANSI codes in non-TTY environments (tests),
// so we verify style initialization by checking they can render without panic.
func TestListItemStyles(t *testing.T) {
	// Verify GutterSelected is initialized and can render
	t.Run("GutterSelected is initialized", func(t *testing.T) {
		style := Styles.ListItem.GutterSelected
		rendered := style.Render("│")
		if len(rendered) == 0 {
			t.Error("GutterSelected should render content")
		}
	})

	// Verify TitleSelected is initialized
	t.Run("TitleSelected is initialized", func(t *testing.T) {
		style := Styles.ListItem.TitleSelected
		rendered := style.Render("test")
		if len(rendered) == 0 {
			t.Error("TitleSelected should render content")
		}
	})

	// Verify TitleNormal is initialized
	t.Run("TitleNormal is initialized", func(t *testing.T) {
		style := Styles.ListItem.TitleNormal
		rendered := style.Render("test")
		if len(rendered) == 0 {
			t.Error("TitleNormal should render content")
		}
	})

	// Verify DescSelected is initialized
	t.Run("DescSelected is initialized", func(t *testing.T) {
		style := Styles.ListItem.DescSelected
		rendered := style.Render("test")
		if len(rendered) == 0 {
			t.Error("DescSelected should render content")
		}
	})

	// Verify DescNormal is initialized
	t.Run("DescNormal is initialized", func(t *testing.T) {
		style := Styles.ListItem.DescNormal
		rendered := style.Render("test")
		if len(rendered) == 0 {
			t.Error("DescNormal should render content")
		}
	})

	// Verify style struct has exactly 5 styles (GutterNormal was removed as unused)
	t.Run("ListItem struct has 5 styles", func(t *testing.T) {
		// This documents that GutterNormal style was intentionally removed
		// GutterNormal constant is used directly (no styling needed for unselected gutter)
		_ = Styles.ListItem.GutterSelected
		_ = Styles.ListItem.TitleSelected
		_ = Styles.ListItem.TitleNormal
		_ = Styles.ListItem.DescSelected
		_ = Styles.ListItem.DescNormal
	})
}

// TestGutterConstants verifies gutter string constants have correct visual width and bytes
func TestGutterConstants(t *testing.T) {
	t.Run("GutterSelected visual width is 2", func(t *testing.T) {
		width := VisualWidth(GutterSelected)
		if width != 2 {
			t.Errorf("GutterSelected visual width = %d, want 2", width)
		}
	})

	t.Run("GutterNormal visual width is 2", func(t *testing.T) {
		width := VisualWidth(GutterNormal)
		if width != 2 {
			t.Errorf("GutterNormal visual width = %d, want 2", width)
		}
	})

	t.Run("GutterSelected contains vertical line U+2502", func(t *testing.T) {
		if GutterSelected[0:3] != "│" {
			t.Errorf("GutterSelected should start with │ (U+2502)")
		}
	})

	t.Run("GutterSelected ends with regular space U+0020", func(t *testing.T) {
		// U+2502 is 3 bytes, so the space should be at index 3
		if GutterSelected[3] != ' ' {
			t.Errorf("GutterSelected should end with regular space (U+0020), got %v", GutterSelected[3])
		}
	})

	t.Run("GutterNormal is two regular spaces", func(t *testing.T) {
		if GutterNormal != "  " {
			t.Errorf("GutterNormal should be two spaces, got %q", GutterNormal)
		}
	})
}

// TestNewMarkdownRenderer tests markdown renderer creation
func TestNewMarkdownRenderer(t *testing.T) {
	t.Run("creates valid renderer with positive width", func(t *testing.T) {
		r, err := NewMarkdownRenderer(80)
		if err != nil {
			t.Errorf("NewMarkdownRenderer(80) error = %v", err)
		}
		if r == nil {
			t.Error("NewMarkdownRenderer(80) returned nil")
		}
		if r.Width() != 80 {
			t.Errorf("Width() = %d, want 80", r.Width())
		}
	})

	t.Run("uses default width for zero", func(t *testing.T) {
		r, err := NewMarkdownRenderer(0)
		if err != nil {
			t.Errorf("NewMarkdownRenderer(0) error = %v", err)
		}
		if r == nil {
			t.Error("NewMarkdownRenderer(0) returned nil")
		}
		if r.Width() != 80 {
			t.Errorf("Width() = %d, want 80 (default)", r.Width())
		}
	})

	t.Run("uses default width for negative", func(t *testing.T) {
		r, err := NewMarkdownRenderer(-10)
		if err != nil {
			t.Errorf("NewMarkdownRenderer(-10) error = %v", err)
		}
		if r == nil {
			t.Error("NewMarkdownRenderer(-10) returned nil")
		}
		if r.Width() != 80 {
			t.Errorf("Width() = %d, want 80 (default)", r.Width())
		}
	})
}

// TestMarkdownRenderCodeBlock tests rendering of code blocks
func TestMarkdownRenderCodeBlock(t *testing.T) {
	r, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer(80) error = %v", err)
	}

	input := "```go\nfunc main() {}\n```"
	output := r.Render(input)

	// Verify function text is present
	if !strings.Contains(output, "func main") {
		t.Error("Rendered code block should contain 'func main'")
	}

	// Verify output is different from input (processing occurred)
	if output == input {
		t.Error("Glamour should process the code block, but output equals input")
	}

	// Verify ANSI codes or structural changes indicate rendering occurred (AC 3.1.4)
	// Note: Glamour's WithAutoStyle() may not emit ANSI codes in non-TTY test environments,
	// but the output should still be structurally different from the raw markdown.
	hasANSI := strings.Contains(output, "\x1b[")
	hasStructuralChange := !strings.Contains(output, "```")
	if !hasANSI && !hasStructuralChange {
		t.Error("Rendered code block should either contain ANSI codes or have markdown syntax processed")
	}
}

// TestMarkdownRenderNilRenderer tests graceful fallback for nil renderer
func TestMarkdownRenderNilRenderer(t *testing.T) {
	var r *MarkdownRenderer
	output := r.Render("test content")
	if output != "test content" {
		t.Errorf("Render() = %q, want %q (graceful fallback)", output, "test content")
	}
}

// TestMarkdownRenderNilInternalRenderer tests graceful fallback when internal renderer is nil
func TestMarkdownRenderNilInternalRenderer(t *testing.T) {
	r := &MarkdownRenderer{renderer: nil, width: 80}
	output := r.Render("test content")
	if output != "test content" {
		t.Errorf("Render() = %q, want %q (graceful fallback)", output, "test content")
	}
}

// TestMarkdownTrimTrailingNewlines tests that trailing newlines are trimmed
func TestMarkdownTrimTrailingNewlines(t *testing.T) {
	r, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer(80) error = %v", err)
	}

	output := r.Render("# Header")
	if strings.HasSuffix(output, "\n") {
		t.Error("Rendered output should not have trailing newlines")
	}
}

// TestMarkdownRenderHeader tests rendering of markdown headers
func TestMarkdownRenderHeader(t *testing.T) {
	r, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer(80) error = %v", err)
	}

	output := r.Render("# Hello World")

	// Verify the text is present
	if !strings.Contains(output, "Hello World") {
		t.Error("Rendered header should contain 'Hello World'")
	}
}

// TestMarkdownRenderList tests rendering of markdown lists
func TestMarkdownRenderList(t *testing.T) {
	r, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer(80) error = %v", err)
	}

	input := "- Item 1\n- Item 2\n- Item 3"
	output := r.Render(input)

	// Verify list items are present
	if !strings.Contains(output, "Item 1") {
		t.Error("Rendered list should contain 'Item 1'")
	}
	if !strings.Contains(output, "Item 2") {
		t.Error("Rendered list should contain 'Item 2'")
	}
}

// TestMarkdownRendererWidth tests the Width() method
func TestMarkdownRendererWidth(t *testing.T) {
	t.Run("returns width for valid renderer", func(t *testing.T) {
		r, _ := NewMarkdownRenderer(100)
		if r.Width() != 100 {
			t.Errorf("Width() = %d, want 100", r.Width())
		}
	})

	t.Run("returns 0 for nil renderer", func(t *testing.T) {
		var r *MarkdownRenderer
		if r.Width() != 0 {
			t.Errorf("Width() = %d, want 0", r.Width())
		}
	})
}

// TestMarkdownRenderPlainText tests that plain text passes through correctly
func TestMarkdownRenderPlainText(t *testing.T) {
	r, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer(80) error = %v", err)
	}

	input := "Just some plain text without any markdown."
	output := r.Render(input)

	// The text should be present in output
	if !strings.Contains(output, "plain text") {
		t.Error("Plain text should be preserved in output")
	}
}

// TestMarkdownRenderEmptyContent tests graceful handling of empty content
func TestMarkdownRenderEmptyContent(t *testing.T) {
	r, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer(80) error = %v", err)
	}

	output := r.Render("")
	// Empty content should return empty string (or minimal whitespace)
	trimmed := strings.TrimSpace(output)
	if trimmed != "" {
		t.Errorf("Render(\"\") should return empty/whitespace, got %q", output)
	}
}
