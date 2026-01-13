// Package tui provides the terminal user interface components.
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	// Primary colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	accentColor    = lipgloss.Color("#F59E0B") // Amber

	// Text colors
	textColor  = lipgloss.Color("#E5E7EB") // Light gray
	mutedColor = lipgloss.Color("#9CA3AF") // Muted gray
	dimColor   = lipgloss.Color("#6B7280") // Dim gray

	// Background colors
	bgColor    = lipgloss.Color("#1F2937") // Dark background
	bgAltColor = lipgloss.Color("#374151") // Alternate background

	// Role colors
	userColor      = lipgloss.Color("#3B82F6") // Blue for user
	assistantColor = lipgloss.Color("#10B981") // Green for assistant
	thinkingColor  = lipgloss.Color("#8B5CF6") // Purple for thinking
	toolColor      = lipgloss.Color("#F59E0B") // Amber for tool use
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
		Foreground(lipgloss.Color("#FFFFFF")).
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
		Foreground(lipgloss.Color("#000000")),

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
