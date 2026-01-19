// Package tui provides the terminal user interface components.
package tui

import (
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ToastDuration is the duration for toast notifications (Story 4.2).
const ToastDuration = 3 * time.Second

// MaxCommandBufferDigits is the maximum digits allowed in command mode input (Story 4.2).
const MaxCommandBufferDigits = 6

// Theme defines adaptive colors for light and dark terminal backgrounds.
type Theme struct {
	// Core colors
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor
	Accent    lipgloss.AdaptiveColor

	// Text colors
	Text  lipgloss.AdaptiveColor
	Muted lipgloss.AdaptiveColor
	Dim   lipgloss.AdaptiveColor

	// Background colors
	Background lipgloss.AdaptiveColor
	BgAlt      lipgloss.AdaptiveColor
	SelectedBg lipgloss.AdaptiveColor // Subtle selection background
	CheckedBg  lipgloss.AdaptiveColor // Subtle checked selection background (multi-select)

	// Role colors (semantic)
	User      lipgloss.AdaptiveColor
	Assistant lipgloss.AdaptiveColor
	Thinking  lipgloss.AdaptiveColor
	Tool      lipgloss.AdaptiveColor

	// Constant colors (non-adaptive, for text on colored backgrounds)
	White lipgloss.AdaptiveColor // Constant white (e.g., text on dark accent)
	Black lipgloss.AdaptiveColor // Constant black (e.g., text on light accent)
}

// DefaultTheme provides the default color theme with light/dark adaptations.
var DefaultTheme = Theme{
	// Core colors
	Primary:   lipgloss.AdaptiveColor{Light: "#5B21B6", Dark: "#7C3AED"}, // Purple
	Secondary: lipgloss.AdaptiveColor{Light: "#059669", Dark: "#10B981"}, // Green
	Accent:    lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#F59E0B"}, // Amber

	// Text colors
	Text:  lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"}, // Dark/Light gray
	Muted: lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}, // Muted gray
	Dim:   lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}, // Dim gray

	// Background colors
	Background: lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1F2937"}, // White/Dark
	BgAlt:      lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#374151"}, // Light/Dark alt

	// Role colors
	User:      lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#3B82F6"}, // Blue
	Assistant: lipgloss.AdaptiveColor{Light: "#059669", Dark: "#10B981"}, // Green
	Thinking:  lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#8B5CF6"}, // Purple
	Tool:      lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#F59E0B"}, // Amber

	// Constant colors (same in light and dark modes)
	White: lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"}, // Constant white
	Black: lipgloss.AdaptiveColor{Light: "#000000", Dark: "#000000"}, // Constant black

	// Selection background (subtle tint for list selection)
	SelectedBg: lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#2D1B4E"}, // Subtle purple
	// Checked selection background (for multi-select checkbox state)
	CheckedBg: lipgloss.AdaptiveColor{Light: "#D1FAE5", Dark: "#064E3B"}, // Subtle green
}

// Color palette (using Theme for adaptive colors)
var (
	// Primary colors
	primaryColor   = DefaultTheme.Primary
	secondaryColor = DefaultTheme.Secondary
	accentColor    = DefaultTheme.Accent

	// Text colors
	textColor  = DefaultTheme.Text
	mutedColor = DefaultTheme.Muted
	dimColor   = DefaultTheme.Dim

	// Background colors
	bgAltColor = DefaultTheme.BgAlt

	// Role colors
	userColor      = DefaultTheme.User
	assistantColor = DefaultTheme.Assistant
	thinkingColor  = DefaultTheme.Thinking
	toolColor      = DefaultTheme.Tool

	// Constant colors
	whiteColor = DefaultTheme.White
	blackColor = DefaultTheme.Black
)

// Styles defines all the lipgloss styles used in the application.
var Styles = struct {
	// Message styles
	UserMessage      lipgloss.Style
	AssistantMessage lipgloss.Style
	ThinkingBlock    lipgloss.Style
	ToolBlock        lipgloss.Style

	// Header styles
	UserHeader      lipgloss.Style
	AssistantHeader lipgloss.Style
	ThinkingHeader  lipgloss.Style
	ToolHeader      lipgloss.Style

	// Content styles
	MessageContent lipgloss.Style
	Timestamp      lipgloss.Style
	Muted          lipgloss.Style

	// UI elements
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	StatusBar lipgloss.Style
	HelpText  lipgloss.Style
	Selected  lipgloss.Style
	Normal    lipgloss.Style

	// Collapsed indicators
	CollapsedIndicator lipgloss.Style

	// Search
	SearchMatch lipgloss.Style
	SearchInput lipgloss.Style

	// Line number gutter (Story 4.1)
	Gutter lipgloss.Style

	// Status bar segments
	StatusBarSegment struct {
		Mode      lipgloss.Style
		Position  lipgloss.Style
		Shortcuts lipgloss.Style
	}

	// List item styles (for project/conversation lists)
	ListItem struct {
		GutterSelected lipgloss.Style
		TitleSelected  lipgloss.Style
		TitleNormal    lipgloss.Style
		TitleChecked   lipgloss.Style // For multi-select checked items
		DescSelected   lipgloss.Style
		DescNormal     lipgloss.Style
		DescChecked    lipgloss.Style // For multi-select checked items
	}

	// Selection style for multi-select checkbox indicator (Story 5.1)
	SelectionIndicator lipgloss.Style
}{
	// Message container styles
	UserMessage: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(userColor).
		Padding(0, 1),

	AssistantMessage: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(assistantColor).
		Padding(0, 1),

	ThinkingBlock: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(thinkingColor).
		Padding(0, 1).
		Foreground(mutedColor),

	ToolBlock: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(toolColor).
		Padding(0, 1),

	// Header styles
	UserHeader: lipgloss.NewStyle().
		Foreground(userColor).
		Bold(true),

	AssistantHeader: lipgloss.NewStyle().
		Foreground(assistantColor).
		Bold(true),

	ThinkingHeader: lipgloss.NewStyle().
		Foreground(thinkingColor).
		Bold(true),

	ToolHeader: lipgloss.NewStyle().
		Foreground(toolColor).
		Bold(true),

	// Content styles
	MessageContent: lipgloss.NewStyle().
		Foreground(textColor),

	Timestamp: lipgloss.NewStyle().
		Foreground(dimColor).
		Italic(true),

	Muted: lipgloss.NewStyle().
		Foreground(mutedColor),

	// UI elements
	Title: lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true),

	Subtitle: lipgloss.NewStyle().
		Foreground(secondaryColor),

	StatusBar: lipgloss.NewStyle().
		Background(bgAltColor).
		Foreground(textColor).
		Padding(0, 1),

	HelpText: lipgloss.NewStyle().
		Foreground(dimColor),

	Selected: lipgloss.NewStyle().
		Background(primaryColor).
		Foreground(whiteColor).
		Bold(true),

	Normal: lipgloss.NewStyle().
		Foreground(textColor),

	// Collapsed indicator
	CollapsedIndicator: lipgloss.NewStyle().
		Foreground(dimColor).
		Italic(true),

	// Search styles
	SearchMatch: lipgloss.NewStyle().
		Background(accentColor).
		Foreground(blackColor),

	SearchInput: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1),

	// Line number gutter style (Story 4.1) - dimmed, right-aligned
	Gutter: lipgloss.NewStyle().
		Foreground(dimColor),

	StatusBarSegment: struct {
		Mode      lipgloss.Style
		Position  lipgloss.Style
		Shortcuts lipgloss.Style
	}{
		Mode: lipgloss.NewStyle().
			Background(accentColor).
			Foreground(whiteColor).
			Padding(0, 1).
			Bold(true),
		Position: lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(whiteColor).
			Padding(0, 1),
		Shortcuts: lipgloss.NewStyle().
			Background(bgAltColor).
			Foreground(textColor).
			Padding(0, 1),
	},

	ListItem: struct {
		GutterSelected lipgloss.Style
		TitleSelected  lipgloss.Style
		TitleNormal    lipgloss.Style
		TitleChecked   lipgloss.Style
		DescSelected   lipgloss.Style
		DescNormal     lipgloss.Style
		DescChecked    lipgloss.Style
	}{
		GutterSelected: lipgloss.NewStyle().
			Foreground(primaryColor),
		TitleSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Background(DefaultTheme.SelectedBg),
		TitleNormal: lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor),
		TitleChecked: lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor).
			Background(DefaultTheme.CheckedBg),
		DescSelected: lipgloss.NewStyle().
			Foreground(textColor).
			Background(DefaultTheme.SelectedBg),
		DescNormal: lipgloss.NewStyle().
			Foreground(mutedColor),
		DescChecked: lipgloss.NewStyle().
			Foreground(textColor).
			Background(DefaultTheme.CheckedBg),
	},

	// Selection indicator style (green foreground for [x])
	SelectionIndicator: lipgloss.NewStyle().
		Foreground(secondaryColor).
		Bold(true),
}

// GutterSeparator is the string between gutter numbers and content (Story 4.1).
const GutterSeparator = " "

// Icons for different message types (text-based, no emoji per FR-017)
const (
	UserIcon      = "[U]"
	AssistantIcon = "[A]"
	ThinkingIcon  = "[T]"
	ToolIcon      = "[>]"
)

// Selection constants for multi-project selection (Story 5.1)
const (
	MaxSelectedProjects  = 9    // Maximum projects that can be selected for dashboard
	SelectionChecked     = "[x]" // Indicator for selected project
	SelectionUnchecked   = "[ ]" // Indicator for unselected project in selection mode
)

// Box-drawing characters for list decoration
const (
	HorizontalLine = "────────────────────────────────────────────────────────────────────────────────"
	VerticalLine   = "│"
	TopLeft        = "┌"
	TopRight       = "┐"
	BottomLeft     = "└"
	BottomRight    = "┘"
)

// List item gutter indicators
const (
	GutterSelected = "│ " // U+2502 + space (2 chars visual width)
	GutterNormal   = "  " // Two spaces (2 chars visual width)
)

// LoadingState represents the state of lazy loading.
type LoadingState int

const (
	LoadingStateIdle LoadingState = iota
	LoadingStateLoading
	LoadingStateComplete
	LoadingStateError
)

// LazyLoadConfig contains configuration for lazy loading.
type LazyLoadConfig struct {
	BatchSize             int // Number of items to load per batch
	ConversationThreshold int // Threshold for lazy loading conversations (50)
	MessageThreshold      int // Threshold for lazy loading messages (100)
}

// DefaultLazyLoadConfig returns the default lazy loading configuration.
func DefaultLazyLoadConfig() LazyLoadConfig {
	return LazyLoadConfig{
		BatchSize:             20,
		ConversationThreshold: 50,
		MessageThreshold:      100,
	}
}

// ListStyles contains styles for decorated lists.
var ListStyles = struct {
	Header    lipgloss.Style
	Separator lipgloss.Style
	Counter   lipgloss.Style
	Loading   lipgloss.Style
}{
	Header: lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(dimColor).
		PaddingBottom(0).
		MarginBottom(0),

	Separator: lipgloss.NewStyle().
		Foreground(dimColor),

	Counter: lipgloss.NewStyle().
		Foreground(mutedColor),

	Loading: lipgloss.NewStyle().
		Foreground(accentColor).
		Italic(true),
}

// Dashboard pane styles (Story 5.2)
var (
	// PaneBorderStyle defines the border style for dashboard panes.
	PaneBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimColor)

	// PaneHeaderStyle defines the header style for dashboard panes.
	PaneHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Align(lipgloss.Center).
			Foreground(primaryColor)

	// Pane focus styles (Story 5.5)
	PaneFocusedBorderColor   = DefaultTheme.Accent // Amber for focused pane
	PaneUnfocusedBorderColor = DefaultTheme.Muted  // Gray for unfocused pane
)

// multipleNewlinesRe matches 3+ consecutive newlines for normalization (Story 4.5).
var multipleNewlinesRe = regexp.MustCompile(`\n{3,}`)

// NormalizeNewlines reduces 3+ consecutive newlines to exactly 2.
// This prevents awkward spacing in markdown output.
func NormalizeNewlines(s string) string {
	return multipleNewlinesRe.ReplaceAllString(s, "\n\n")
}

// MarkdownRenderer handles markdown-to-styled-text conversion.
type MarkdownRenderer struct {
	renderer *glamour.TermRenderer
	width    int
}

// NewMarkdownRenderer creates a new markdown renderer for the given width.
func NewMarkdownRenderer(width int) (*MarkdownRenderer, error) {
	if width <= 0 {
		width = 80 // Default width
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, err
	}
	return &MarkdownRenderer{renderer: r, width: width}, nil
}

// Render converts markdown to styled terminal output.
// Returns raw content on error (graceful degradation).
func (m *MarkdownRenderer) Render(content string) string {
	if m == nil || m.renderer == nil {
		return content // Graceful fallback
	}
	rendered, err := m.renderer.Render(content)
	if err != nil {
		return content // Graceful fallback
	}
	// Normalize excessive newlines (Story 4.5)
	normalized := NormalizeNewlines(rendered)
	return strings.TrimRight(normalized, "\n")
}

// Width returns the current width of the renderer.
func (m *MarkdownRenderer) Width() int {
	if m == nil {
		return 0
	}
	return m.width
}
