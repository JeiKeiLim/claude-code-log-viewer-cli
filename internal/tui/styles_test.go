package tui

import (
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

// TestThemeAllFieldsPopulated ensures all 14 color fields are non-zero (12 adaptive + 2 constant)
func TestThemeAllFieldsPopulated(t *testing.T) {
	// Count populated fields
	populated := 0
	expected := 14

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
