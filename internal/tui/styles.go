// Package tui provides the terminal user interface components.
package tui

import "github.com/charmbracelet/lipgloss"

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
}{
	// Message container styles
	UserMessage: lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(userColor).
		PaddingLeft(1).
		MarginBottom(1),

	AssistantMessage: lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(assistantColor).
		PaddingLeft(1).
		MarginBottom(1),

	ThinkingBlock: lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(thinkingColor).
		PaddingLeft(1).
		Foreground(mutedColor),

	ToolBlock: lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(toolColor).
		PaddingLeft(1),

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
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1),
}

// Icons for different message types (text-based, no emoji per FR-017)
const (
	UserIcon      = "[U]"
	AssistantIcon = "[A]"
	ThinkingIcon  = "[T]"
	ToolIcon      = "[>]"
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
