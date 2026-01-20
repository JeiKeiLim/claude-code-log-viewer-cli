// Package tui provides the terminal user interface components.
package tui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/token"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

// InputMode represents the current input mode for the viewer.
type InputMode int

const (
	InputNone    InputMode = iota
	InputCommand           // :N navigation
	InputSearch            // / search
)

// RenderOptions controls visibility of content types during rendering.
type RenderOptions struct {
	HideThoughts bool   // Hide thinking blocks
	HideTools    bool   // Hide tool use blocks
	Width        int    // Width override for rendering (0=auto-detect)
	WatchMode    bool   // Enable file watching mode
	FilePath     string // Full path for file watching
}

// DefaultRenderOptions returns options that show all content types with auto-detect width.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		HideThoughts: false,
		HideTools:    false,
		Width:        0,
		WatchMode:    false,
	}
}

// ViewerModel is the Bubbletea model for viewing log entries.
type ViewerModel struct {
	entries     []types.LogEntry
	viewport    viewport.Model
	ready       bool
	parseErrors int
	width       int
	height      int
	title       string // Display title based on source context

	// Toggle states (collapsed by default per spec)
	showThinking   bool
	showToolInputs bool

	// For returning to previous view
	canGoBack bool

	// Input mode state (Story 4.2)
	inputMode   InputMode // Current input mode: InputNone, InputCommand, InputSearch
	inputBuffer string    // Buffer for command input (:42)

	// Search state
	searchInput   textinput.Model
	searchQuery   string
	searchMatches []int // Line numbers with matches
	currentMatch  int   // Index into searchMatches
	noResults     bool

	// Toast notification state (Story 4.2)
	toast       string    // Toast message to display
	toastExpiry time.Time // When toast should disappear
	toastID     int       // Unique ID for toast race condition prevention

	// Line position tracking for navigation (Story 4.2)
	entryLinePositions []int // Y offset where each entry starts in viewport

	// gg key detection
	lastKeyG     bool
	lastKeyGTime time.Time

	// Lazy loading state
	lazyLoadState LoadingState
	loadedCount   int  // Number of entries rendered
	lazyEnabled   bool // Whether lazy loading is enabled (>100 entries)

	// Overlay spinner for bulk loading operations
	overlaySpinner     spinner.Model
	showOverlaySpinner bool

	// Watch mode and watcher
	watchMode       bool
	watcher         *watcher.Watcher
	newEntriesCount int // Track unseen entries when scrolled up in watch mode

	// Render options for visibility control (includes FilePath for reload on truncation)
	renderOpts RenderOptions

	// Markdown renderer for assistant text
	markdownRenderer *MarkdownRenderer

	// Render cache - keyed by entry index for O(1) lookup
	renderCache map[int]string // entry index -> rendered markdown string
	cacheWidth  int            // width when cache was built

	// Line numbers display (Story 4.1)
	showLineNumbers bool // default true
	gutterWidth     int  // calculated from entry count

	// Raw JSONL mode (Story 4.3)
	rawMode          bool     // Toggle between parsed and raw view (default: false)
	rawLines         []string // Cached raw JSONL lines
	rawLineCount     int      // Total raw lines for gutter width
	rawLinePositions []int    // Y offset for each raw line for navigation

	// Token service for statistics display (Story 6.3)
	tokenService       *token.Service // For token calculations
	conversationTokens int            // Total tokens for conversation
	tokensEstimated    bool           // True if any token was estimated
}

// calculateGutterWidth returns the width needed for line numbers.
// Minimum width is 3 (for numbers 1-99), increases for larger counts.
func calculateGutterWidth(entryCount int) int {
	if entryCount == 0 {
		return 3
	}
	width := len(fmt.Sprintf("%d", entryCount))
	if width < 3 {
		width = 3
	}
	return width
}

// prependGutter adds line number to first line, padding to subsequent lines.
func prependGutter(entryNum int, content string, gutterWidth int) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder

	// Format the line number with right-alignment
	numStr := fmt.Sprintf("%*d", gutterWidth, entryNum)
	gutterFormatted := Styles.Gutter.Render(numStr) + GutterSeparator
	result.WriteString(gutterFormatted + lines[0])

	// Add padding to continuation lines
	padding := strings.Repeat(" ", gutterWidth+len(GutterSeparator))
	for i := 1; i < len(lines); i++ {
		result.WriteString("\n" + padding + lines[i])
	}
	return result.String()
}

// prependGutterStatic adds line number without lipgloss styling (for async rendering).
func prependGutterStatic(entryNum int, content string, gutterWidth int) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder

	// Format the line number with right-alignment (no styling for static version)
	numStr := fmt.Sprintf("%*d", gutterWidth, entryNum)
	result.WriteString(numStr + GutterSeparator + lines[0])

	// Add padding to continuation lines
	padding := strings.Repeat(" ", gutterWidth+len(GutterSeparator))
	for i := 1; i < len(lines); i++ {
		result.WriteString("\n" + padding + lines[i])
	}
	return result.String()
}

// NewViewerModel creates a new viewer model with the given entries.
// tokenSvc is optional; if nil, token statistics will not be displayed.
func NewViewerModel(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions, tokenSvc *token.Service) ViewerModel {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100

	config := DefaultLazyLoadConfig()
	lazyEnabled := len(entries) > config.MessageThreshold

	// For lazy loading, initially render only the first batch
	loadedCount := len(entries)
	state := LoadingStateComplete
	if lazyEnabled {
		loadedCount = min(config.BatchSize*2, len(entries)) // Load 2 batches initially (40 entries)
		if loadedCount < len(entries) {
			state = LoadingStateIdle
		}
	}

	// Initialize spinner with Dot style and Loading style
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ListStyles.Loading

	// Initialize markdown renderer with default width
	initialWidth := opts.Width
	if initialWidth <= 0 {
		initialWidth = 80
	}
	// Account for gutter width in markdown renderer (Story 4.1)
	gutterWidth := calculateGutterWidth(len(entries))
	gutterSpace := gutterWidth + len(GutterSeparator) // showLineNumbers defaults to true
	mdRenderWidth := initialWidth - 4 - gutterSpace
	if mdRenderWidth < 20 {
		mdRenderWidth = 20
	}
	mdRenderer, _ := NewMarkdownRenderer(mdRenderWidth)

	m := ViewerModel{
		entries:            entries,
		parseErrors:        parseErrors,
		title:              title,
		showThinking:       false, // Collapsed by default
		showToolInputs:     false, // Collapsed by default
		canGoBack:          false,
		inputMode:          InputNone, // Story 4.2: default to no input mode
		inputBuffer:        "",        // Story 4.2: empty command buffer
		searchInput:        ti,
		lazyEnabled:        lazyEnabled,
		loadedCount:        loadedCount,
		lazyLoadState:      state,
		overlaySpinner:     s,
		watchMode:          opts.WatchMode,
		newEntriesCount:    0, // Explicitly initialize for watch mode tracking
		renderOpts:         opts,
		markdownRenderer:   mdRenderer,
		renderCache:        make(map[int]string),
		cacheWidth:         mdRenderWidth,
		showLineNumbers:    true, // Default true (Story 4.1)
		gutterWidth:        gutterWidth,
		entryLinePositions: make([]int, 0, len(entries)), // Story 4.2: navigation tracking
		// Raw JSONL mode (Story 4.3)
		rawMode:          false,
		rawLines:         nil,
		rawLineCount:     0,
		rawLinePositions: nil,
		// Token service (Story 6.3)
		tokenService: tokenSvc,
	}

	// Calculate conversation totals if service available (Story 6.3)
	if tokenSvc != nil {
		m.conversationTokens, m.tokensEstimated = tokenSvc.CalculateConversation(entries)
	}

	// Apply width override if specified
	if opts.Width > 0 {
		m.width = opts.Width
	}

	// Create watcher if watch mode enabled and file path provided
	if opts.WatchMode && opts.FilePath != "" {
		w, err := watcher.New(opts.FilePath)
		if err == nil {
			m.watcher = w
		}
		// If watcher creation fails, continue without it (graceful degradation)
	}

	return m
}

// NewViewerModelWithBack creates a new viewer that can return to a previous view.
// tokenSvc is optional; if nil, token statistics will not be displayed.
func NewViewerModelWithBack(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions, tokenSvc *token.Service) ViewerModel {
	m := NewViewerModel(entries, parseErrors, title, opts, tokenSvc)
	m.canGoBack = true
	return m
}

// NewViewerModelWithBackNavigation is an alias for NewViewerModelWithBack.
// tokenSvc is optional; if nil, token statistics will not be displayed.
func NewViewerModelWithBackNavigation(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions, tokenSvc *token.Service) ViewerModel {
	return NewViewerModelWithBack(entries, parseErrors, title, opts, tokenSvc)
}

// SetSize sets the viewport size.
func (m *ViewerModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Header: 1 line for title
	// Footer: 1 line for help/status
	headerHeight := 1
	footerHeight := 1
	verticalMargins := headerHeight + footerHeight

	if !m.ready {
		m.viewport = viewport.New(width, height-verticalMargins)
		m.viewport.YPosition = headerHeight
		m.ready = true
		m.updateContent()
	} else {
		m.viewport.Width = width
		m.viewport.Height = height - verticalMargins
		m.updateContent()
	}
}

// Init implements tea.Model.
func (m ViewerModel) Init() tea.Cmd {
	if m.watcher != nil {
		return m.watcher.WaitForEvent()
	}
	return nil
}

// Update implements tea.Model.
func (m ViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		var spinnerCmd tea.Cmd
		m.overlaySpinner, spinnerCmd = m.overlaySpinner.Update(msg)
		// Only return tick command if overlay spinner is shown
		if m.showOverlaySpinner {
			return m, spinnerCmd
		}
		return m, nil

	case toastExpiredMsg:
		// Clear toast when expired (Story 4.2) - only if ID matches to prevent race conditions
		if msg.id == m.toastID {
			m.toast = ""
			m.toastExpiry = time.Time{}
		}
		return m, nil

	case tea.KeyMsg:
		// Handle command mode FIRST (Story 4.2, 4.3)
		if m.inputMode == InputCommand {
			switch msg.String() {
			case "enter":
				num, err := strconv.Atoi(m.inputBuffer)
				// Validate against rawLineCount in raw mode, entries count otherwise (Story 4.3)
				maxNum := len(m.entries)
				if m.rawMode {
					maxNum = m.rawLineCount
				}
				if err != nil || num < 1 || num > maxNum {
					m.inputMode = InputNone
					m.inputBuffer = ""
					return m, m.showToast("Invalid line number", ToastDuration)
				}
				_ = m.navigateToEntry(num) // Validation already done above
				m.inputMode = InputNone
				m.inputBuffer = ""
				return m, nil
			case "esc":
				m.inputMode = InputNone
				m.inputBuffer = ""
				return m, nil
			case "backspace":
				if len(m.inputBuffer) > 0 {
					m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
				}
				return m, nil
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				// Limit buffer to MaxCommandBufferDigits (supports up to 999,999 entries)
				if len(m.inputBuffer) < MaxCommandBufferDigits {
					m.inputBuffer += msg.String()
				}
				return m, nil
			default:
				// Ignore all other keys in command mode (letters, symbols, etc.)
				return m, nil
			}
		}

		// Handle search mode
		if m.inputMode == InputSearch {
			switch msg.String() {
			case "enter":
				m.inputMode = InputNone
				m.searchQuery = m.searchInput.Value()
				m.performSearch()
				return m, nil
			case "esc":
				m.inputMode = InputNone
				m.searchInput.SetValue("")
				m.searchQuery = ""
				m.searchMatches = nil
				m.noResults = false
				m.updateContent()
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}

		keyStr := msg.String()

		// Check for gg sequence
		if keyStr == "g" {
			if m.lastKeyG && time.Since(m.lastKeyGTime) < 500*time.Millisecond {
				// gg detected - go to top
				m.viewport.GotoTop()
				m.lastKeyG = false
				return m, nil
			}
			m.lastKeyG = true
			m.lastKeyGTime = time.Now()
			return m, nil
		}
		m.lastKeyG = false

		switch keyStr {
		case "q", "ctrl+c":
			if m.watcher != nil {
				_ = m.watcher.Close()
			}
			return m, tea.Quit

		case "j", "down":
			m.viewport.ScrollDown(1)

		case "k", "up":
			m.viewport.ScrollUp(1)

		case "d", "ctrl+d":
			m.viewport.HalfPageDown()

		case "u", "ctrl+u":
			m.viewport.HalfPageUp()

		case "pgdown", " ":
			m.viewport.PageDown()

		case "pgup":
			m.viewport.PageUp()

		case "home":
			m.viewport.GotoTop()

		case "end":
			m.viewport.GotoBottom()

		case "G":
			// Clear new entries indicator when jumping to bottom
			m.newEntriesCount = 0

			// Raw mode: check against rawLineCount (Story 4.3)
			if m.rawMode {
				if m.lazyEnabled && m.loadedCount < m.rawLineCount {
					m.showOverlaySpinner = true
					return m, tea.Batch(m.overlaySpinner.Tick, m.markAllRawLinesLoadedCmd())
				}
				m.viewport.GotoBottom()
				return m, nil
			}

			// Normal mode: go to bottom with async loading if lazy loading is enabled
			if m.lazyEnabled && m.loadedCount < len(m.entries) {
				m.showOverlaySpinner = true
				return m, tea.Batch(m.overlaySpinner.Tick, m.markAllMessagesLoadedCmd())
			}
			// If all messages loaded, just go to bottom
			m.viewport.GotoBottom()

		case "h", "esc":
			if m.canGoBack {
				// Close watcher before navigating back to prevent resource leak
				if m.watcher != nil {
					_ = m.watcher.Close()
				}
				// Signal to go back - handled by parent
				return m, func() tea.Msg { return GoBackMsg{} }
			}

		case ":":
			// Enter command mode (Story 4.2)
			m.inputMode = InputCommand
			m.inputBuffer = ""
			return m, nil

		case "/":
			// Open search
			m.inputMode = InputSearch
			m.searchInput.Focus()
			return m, textinput.Blink

		case "n":
			// Next match
			m.nextMatch()

		case "N":
			// Previous match
			m.prevMatch()

		case "t":
			m.showThinking = !m.showThinking
			m.invalidateRenderCache() // Toggle affects rendering, clear cache
			m.updateContent()

		case "i":
			m.showToolInputs = !m.showToolInputs
			m.invalidateRenderCache() // Toggle affects rendering, clear cache
			m.updateContent()

		case "w":
			// Toggle watch mode
			if m.watchMode {
				// Disable watch mode
				if m.watcher != nil {
					_ = m.watcher.Close()
					m.watcher = nil
				}
				m.watchMode = false
				m.newEntriesCount = 0
				return m, nil
			}
			// Enable watch mode if file path is available
			if m.renderOpts.FilePath != "" {
				w, err := watcher.New(m.renderOpts.FilePath)
				if err == nil {
					m.watcher = w
					m.watchMode = true
					return m, m.watcher.WaitForEvent()
				}
			}
			return m, nil

		case "r":
			// Toggle raw JSONL mode (Story 4.3)
			if m.rawMode {
				// Exit raw mode - restore scroll position best-effort
				scrollPct := m.viewport.ScrollPercent()
				m.rawMode = false
				m.updateContent()
				// Restore approximate scroll position
				maxOffset := m.viewport.TotalLineCount() - m.viewport.Height
				if maxOffset > 0 {
					m.viewport.SetYOffset(int(float64(maxOffset) * scrollPct))
				}
			} else {
				// Enter raw mode
				if err := m.loadRawJSONL(); err != nil {
					return m, m.showToast("Cannot load raw file", ToastDuration)
				}
				m.rawMode = true
				// Update gutter width for raw line count
				m.gutterWidth = calculateGutterWidth(m.rawLineCount)
				// Disable lazy loading for raw mode (fixes first line clipping issue)
				// Raw JSONL lines are small, so loading all is fast
				m.loadedCount = m.rawLineCount
				m.lazyEnabled = false
				m.lazyLoadState = LoadingStateComplete
				// Recreate viewport to ensure clean state
				m.viewport = viewport.New(m.viewport.Width, m.viewport.Height)
				m.viewport.YPosition = 0
				m.updateRawContent()
			}
			return m, nil

		case "p":
			// Show file path as toast (Story 4.4)
			if m.renderOpts.FilePath != "" {
				return m, m.showToast(m.renderOpts.FilePath, ToastDuration)
			}
			return m, m.showToast("No path available", ToastDuration)
		}

		// Clear new entries indicator when user manually scrolls to bottom
		if m.isAtBottom() && m.newEntriesCount > 0 {
			m.newEntriesCount = 0
		}

	case tea.WindowSizeMsg:
		// Only update width from terminal if not overridden
		if m.renderOpts.Width == 0 {
			m.width = msg.Width
		}
		m.height = msg.Height

		// Recreate markdown renderer if nil or width changed significantly
		// Account for gutter width (Story 4.1)
		gutterSpace := 0
		if m.showLineNumbers {
			gutterSpace = m.gutterWidth + len(GutterSeparator)
		}
		newRenderWidth := m.width - 4 - gutterSpace
		if newRenderWidth < 20 {
			newRenderWidth = 20
		}
		if m.markdownRenderer == nil {
			m.markdownRenderer, _ = NewMarkdownRenderer(newRenderWidth)
		} else {
			widthDiff := m.markdownRenderer.Width() - newRenderWidth
			if widthDiff < 0 {
				widthDiff = -widthDiff
			}
			if widthDiff > 5 {
				m.markdownRenderer, _ = NewMarkdownRenderer(newRenderWidth)
				m.invalidateRenderCache() // Clear cache when width changes significantly
			}
		}

		// Header: 1 line for title
		// Footer: 1 line for help/status
		headerHeight := 1
		footerHeight := 1
		verticalMargins := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(m.width, msg.Height-verticalMargins)
			m.viewport.YPosition = headerHeight
			m.ready = true
			m.updateContent()
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = msg.Height - verticalMargins
			m.updateContent()
		}

	case viewerMessagesLoadedMsg:
		// Update loaded count
		m.loadedCount = msg.loadedCount
		wasOverlayShown := m.showOverlaySpinner
		m.showOverlaySpinner = false // Clear overlay spinner
		if m.loadedCount >= len(m.entries) {
			m.lazyLoadState = LoadingStateComplete
		} else {
			m.lazyLoadState = LoadingStateIdle
		}
		// Use pre-rendered content if available (from bulk loading)
		if msg.renderedContent != "" {
			m.viewport.SetContent(msg.renderedContent)
			// Sync entryLinePositions for navigation after bulk load (CR fix)
			m.syncEntryLinePositions()
		} else {
			m.updateContent()
		}
		// If overlay was shown (bulk load via 'G'), go to bottom after loading
		if wasOverlayShown {
			m.viewport.GotoBottom()
		}
		return m, nil

	case gotoTopMsg:
		// Handle deferred go-to-top after content change (fixes viewport clipping)
		m.viewport.SetYOffset(0)
		return m, nil

	case rawLinesLoadedMsg:
		// Update loaded count for raw mode lazy loading (Story 4.3)
		m.loadedCount = msg.loadedCount
		wasOverlayShown := m.showOverlaySpinner
		m.showOverlaySpinner = false
		if m.loadedCount >= m.rawLineCount {
			m.lazyLoadState = LoadingStateComplete
		} else {
			m.lazyLoadState = LoadingStateIdle
		}
		// Use pre-rendered content if available (from bulk loading via 'G')
		if msg.renderedContent != "" {
			m.viewport.SetContent(msg.renderedContent)
			m.rawLinePositions = msg.linePositions
		} else {
			m.updateRawContent()
		}
		// If overlay was shown (bulk load via 'G'), go to bottom after loading
		if wasOverlayShown {
			m.viewport.GotoBottom()
		}
		return m, nil

	case watcher.NewEntriesMsg:
		// Exit raw mode on file change (Story 4.3)
		if m.rawMode {
			m.rawMode = false
		}

		// Capture scroll position BEFORE modifying state
		wasAtBottom := m.isAtBottom()

		// Append new entries from file watcher
		m.entries = append(m.entries, msg.Entries...)
		m.loadedCount = len(m.entries)

		// Recalculate conversation tokens for new entries (Story 6.3)
		if m.tokenService != nil {
			for _, entry := range msg.Entries {
				if !entry.Usage.IsEmpty() {
					m.conversationTokens += entry.Usage.Total()
				} else if entry.Type != types.EntryTypeFileHistorySnapshot {
					m.conversationTokens += m.tokenService.CalculateEntry(entry)
					m.tokensEstimated = true
				}
			}
		}

		// Recalculate gutterWidth if digit threshold crossed (Story 4.1)
		newGutterWidth := calculateGutterWidth(len(m.entries))
		if newGutterWidth != m.gutterWidth {
			m.gutterWidth = newGutterWidth
			m.invalidateRenderCache() // Width change affects all rendered content
		}

		m.updateContent()

		// Smart scroll decision
		if wasAtBottom {
			m.viewport.GotoBottom() // Auto-scroll
		} else {
			m.newEntriesCount += len(msg.Entries) // Track for indicator
		}

		// Chain next wait
		if m.watcher != nil {
			return m, m.watcher.WaitForEvent()
		}
		return m, nil

	case watcher.FileResetMsg:
		// Exit raw mode on file reset (Story 4.3)
		if m.rawMode {
			m.rawMode = false
		}

		// File was truncated - reset everything
		m.newEntriesCount = 0     // Clear indicator
		m.invalidateRenderCache() // Clear cache on file reset

		// Reload from beginning
		if m.renderOpts.FilePath != "" {
			result, err := parser.ParseJSONLFile(m.renderOpts.FilePath)
			if err == nil {
				m.entries = result.Entries
				m.loadedCount = len(m.entries)
				m.parseErrors = result.ParseErrors
				m.gutterWidth = calculateGutterWidth(len(m.entries)) // Recalculate gutter width (Story 4.1)

				// Recalculate conversation totals on file reset (Story 6.3)
				if m.tokenService != nil {
					m.conversationTokens, m.tokensEstimated = m.tokenService.CalculateConversation(m.entries)
				}

				m.updateContent()
			}
		}
		// Chain next wait
		if m.watcher != nil {
			return m, m.watcher.WaitForEvent()
		}
		return m, nil

	case watcher.WatcherErrorMsg:
		// On watcher error, continue waiting (graceful degradation)
		if m.watcher != nil {
			return m, m.watcher.WaitForEvent()
		}
		return m, nil
	}

	m.viewport, cmd = m.viewport.Update(msg)

	// Check if we need to load more messages (when scrolling near bottom)
	if m.lazyEnabled && m.lazyLoadState == LoadingStateIdle {
		scrollPercent := m.viewport.ScrollPercent()
		if scrollPercent > 0.8 {
			// Raw mode: check against rawLineCount (Story 4.3)
			if m.rawMode && m.loadedCount < m.rawLineCount {
				m.lazyLoadState = LoadingStateLoading
				return m, m.loadMoreRawLines()
			}
			// Normal mode: check against entries count
			if !m.rawMode && m.loadedCount < len(m.entries) {
				m.lazyLoadState = LoadingStateLoading
				return m, m.loadMoreMessages()
			}
		}
	}

	return m, cmd
}

// loadMoreMessages loads the next batch of messages.
func (m *ViewerModel) loadMoreMessages() tea.Cmd {
	config := DefaultLazyLoadConfig()
	newLoadedCount := m.loadedCount + config.BatchSize
	if newLoadedCount > len(m.entries) {
		newLoadedCount = len(m.entries)
	}

	return func() tea.Msg {
		return viewerMessagesLoadedMsg{
			loadedCount: newLoadedCount,
		}
	}
}

// loadMoreRawLines loads the next batch of raw JSONL lines (Story 4.3).
func (m *ViewerModel) loadMoreRawLines() tea.Cmd {
	config := DefaultLazyLoadConfig()
	newLoadedCount := m.loadedCount + config.BatchSize
	if newLoadedCount > m.rawLineCount {
		newLoadedCount = m.rawLineCount
	}

	return func() tea.Msg {
		return rawLinesLoadedMsg{
			loadedCount: newLoadedCount,
		}
	}
}

// rawLinesLoadedMsg is sent when more raw JSONL lines are loaded (Story 4.3).
type rawLinesLoadedMsg struct {
	loadedCount     int
	renderedContent string // Pre-rendered content (for bulk load via 'G')
	linePositions   []int  // Pre-computed line positions (for bulk load)
}

// markAllMessagesLoadedCmd returns a command that pre-renders all messages.
// The expensive rendering happens in the command (goroutine) so the spinner can animate.
// Note: token.Service is thread-safe (uses sync.RWMutex) so safe to use in goroutine (Story 6.3).
func (m *ViewerModel) markAllMessagesLoadedCmd() tea.Cmd {
	// Capture values needed for rendering
	entries := m.entries
	total := len(entries)
	width := m.width
	showThinking := m.showThinking
	showToolInputs := m.showToolInputs
	opts := m.renderOpts
	mdRenderer := m.markdownRenderer // Capture for async rendering
	showLineNumbers := m.showLineNumbers
	gutterWidth := m.gutterWidth
	tokenSvc := m.tokenService // Capture for async token info rendering (Story 6.3)

	return func() tea.Msg {
		// Pre-render all content in the goroutine (expensive operation)
		var content strings.Builder
		for i := 0; i < total; i++ {
			rendered := renderEntryStatic(entries[i], width, showThinking, showToolInputs, opts, mdRenderer, gutterWidth, tokenSvc)
			// Prepend line numbers if enabled (Story 4.1)
			if showLineNumbers {
				rendered = prependGutterStatic(i+1, rendered, gutterWidth) // 1-indexed line numbers
			}
			content.WriteString(rendered)
			content.WriteString("\n")
		}
		return viewerMessagesLoadedMsg{
			loadedCount:     total,
			renderedContent: content.String(),
		}
	}
}

// markAllRawLinesLoadedCmd pre-renders all raw JSONL lines and returns them (Story 4.3).
// Similar to markAllMessagesLoadedCmd but for raw mode.
func (m *ViewerModel) markAllRawLinesLoadedCmd() tea.Cmd {
	rawLines := m.rawLines
	total := len(rawLines)
	showLineNumbers := m.showLineNumbers
	gutterWidth := m.gutterWidth

	return func() tea.Msg {
		var content strings.Builder
		linePositions := make([]int, 0, total)
		currentLine := 0

		for i := 0; i < total; i++ {
			linePositions = append(linePositions, currentLine)
			formatted := formatJSONLine(rawLines[i])
			if showLineNumbers {
				formatted = prependGutterStatic(i+1, formatted, gutterWidth)
			}
			content.WriteString(formatted)
			content.WriteString("\n")
			currentLine += strings.Count(formatted, "\n") + 1
		}

		return rawLinesLoadedMsg{
			loadedCount:     total,
			renderedContent: content.String(),
			linePositions:   linePositions,
		}
	}
}

// GoBackMsg signals the parent to go back to the previous view.
type GoBackMsg struct{}

// gotoTopMsg signals the viewport to go to top (used after content changes)
type gotoTopMsg struct{}

// viewerMessagesLoadedMsg is sent when more messages are rendered.
type viewerMessagesLoadedMsg struct {
	loadedCount     int
	renderedContent string // Pre-rendered content for bulk loading (optional)
}

// toastExpiredMsg signals that the toast should be cleared (Story 4.2).
type toastExpiredMsg struct {
	id int // ID to match against current toast to prevent race conditions
}

// buildModeSegment returns the mode indicator segment (Story 4.3: RAW + LIVE).
func (m ViewerModel) buildModeSegment() string {
	var modes []string
	if m.rawMode {
		modes = append(modes, "RAW")
	}
	if m.watchMode && m.watcher != nil {
		modes = append(modes, "LIVE")
	}
	if len(modes) == 0 {
		return ""
	}
	return Styles.StatusBarSegment.Mode.Render(strings.Join(modes, " "))
}

// buildNewEntriesSegment returns the new entries indicator segment (watch mode only).
func (m ViewerModel) buildNewEntriesSegment() string {
	if m.newEntriesCount == 0 {
		return "" // Empty when no new entries pending
	}
	indicator := fmt.Sprintf("+%d new", m.newEntriesCount)
	return Styles.StatusBarSegment.Mode.Render(indicator) // Reuse accent style
}

// buildPositionSegment returns the position indicator segment (Story 4.3: raw mode support, Story 4.6: accurate position).
// Uses binary search on position arrays for accurate entry/line tracking.
func (m ViewerModel) buildPositionSegment() string {
	// In raw mode, show line position (Story 4.3)
	if m.rawMode {
		total := m.rawLineCount
		if total == 0 {
			return Styles.StatusBarSegment.Position.Render("Line 0/0")
		}
		// Use binary search for accurate position (Story 4.6)
		pos := m.findVisibleRawLine(m.viewport.YOffset)
		// During lazy loading, show position within loaded range
		if m.lazyEnabled && m.loadedCount < total {
			return Styles.StatusBarSegment.Position.Render(fmt.Sprintf("Line %d/%d (of %d)", pos, m.loadedCount, total))
		}
		return Styles.StatusBarSegment.Position.Render(fmt.Sprintf("Line %d/%d", pos, total))
	}

	// Normal mode: show entry position
	total := len(m.entries)
	if total == 0 {
		return Styles.StatusBarSegment.Position.Render("Entry 0/0")
	}
	// Use binary search for accurate position (Story 4.6)
	pos := m.findVisibleEntry(m.viewport.YOffset)
	// During lazy loading, show position within loaded range
	if m.lazyEnabled && m.loadedCount < total {
		return Styles.StatusBarSegment.Position.Render(fmt.Sprintf("Entry %d/%d (of %d)", pos, m.loadedCount, total))
	}
	return Styles.StatusBarSegment.Position.Render(fmt.Sprintf("Entry %d/%d", pos, total))
}

// buildShortcutsSegment returns the keyboard shortcuts segment (Story 4.3: r toggle).
func (m ViewerModel) buildShortcutsSegment() string {
	var parts []string
	parts = append(parts, "j/k:scroll", "gg/G:top/bottom", ":N:goto", "/:search")
	if len(m.searchMatches) > 0 {
		parts = append(parts, "n/N:next/prev")
	}
	if m.canGoBack {
		parts = append(parts, "h/esc:back")
	}
	// Raw mode toggle hint (Story 4.3)
	if m.rawMode {
		parts = append(parts, "r:normal")
	} else {
		parts = append(parts, "r:raw")
	}
	parts = append(parts, "p:path", "t:thinking", "i:inputs", "w:watch", "q:quit")
	return strings.Join(parts, " • ")
}

// buildTokensSegment returns the conversation total tokens segment (Story 6.3).
func (m ViewerModel) buildTokensSegment() string {
	if m.tokenService == nil {
		return "" // No token service, skip segment
	}

	var prefix string
	if m.tokensEstimated {
		prefix = "~"
	}
	formatted := formatWithCommas(m.conversationTokens)
	return Styles.StatusBarSegment.Tokens.Render(fmt.Sprintf("Tokens: %s%s", prefix, formatted))
}

// View implements tea.Model.
func (m ViewerModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Header with contextual title (truncated to prevent wrapping)
	headerText := m.title
	if headerText == "" {
		headerText = "Claude Code Log Viewer"
	}
	header := Styles.Title.Render(headerText)
	if m.width > 0 {
		header = lipgloss.NewStyle().MaxWidth(m.width).Render(header)
	}

	// Command mode bar (Story 4.2) - show toast alongside command bar if active
	if m.inputMode == InputCommand {
		commandBar := Styles.SearchInput.Render(":" + m.inputBuffer + "_")
		// Show toast in command mode if active (CR fix)
		if m.toast != "" && time.Now().Before(m.toastExpiry) {
			toastStyle := lipgloss.NewStyle().
				Background(accentColor).
				Foreground(whiteColor).
				Padding(0, 1)
			toastSegment := toastStyle.Render(m.toast)
			commandBar = lipgloss.JoinHorizontal(lipgloss.Top, commandBar, " ", toastSegment)
		}
		if m.width > 0 {
			commandBar = lipgloss.NewStyle().MaxWidth(m.width).Render(commandBar)
		}
		return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), commandBar)
	}

	// Search bar (if searching)
	if m.inputMode == InputSearch {
		searchBar := Styles.SearchInput.Render("/" + m.searchInput.View())
		if m.width > 0 {
			searchBar = lipgloss.NewStyle().MaxWidth(m.width).Render(searchBar)
		}
		return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), searchBar)
	}

	// Build segmented footer with progressive hiding for narrow terminals
	modeSegment := m.buildModeSegment()
	posSegment := m.buildPositionSegment()

	// Calculate fixed segment widths (mode and position are always shown)
	modeWidth := lipgloss.Width(modeSegment)
	posWidth := lipgloss.Width(posSegment)
	fixedWidth := modeWidth + posWidth

	// Optional segments - only include if there's room
	var tokensSegment, newEntriesSegment string
	tokensWidth, newEntriesWidth := 0, 0

	// Add tokens segment if there's room (priority 1 for optional)
	if m.width > fixedWidth+20 { // Need at least 20 chars for shortcuts
		tokensSegment = m.buildTokensSegment()
		tokensWidth = lipgloss.Width(tokensSegment)
	}

	// Add new entries segment if there's room (priority 2 for optional)
	if m.width > fixedWidth+tokensWidth+30 {
		newEntriesSegment = m.buildNewEntriesSegment()
		newEntriesWidth = lipgloss.Width(newEntriesSegment)
	}

	// Build shortcuts text with optional status suffix
	shortcutsText := m.buildShortcutsSegment()
	var statusSuffix string
	if m.noResults && m.searchQuery != "" {
		statusSuffix = fmt.Sprintf(" | No results for '%s'", m.searchQuery)
	} else if len(m.searchMatches) > 0 {
		statusSuffix = fmt.Sprintf(" | Match %d/%d for '%s'", m.currentMatch+1, len(m.searchMatches), m.searchQuery)
	} else if m.lazyEnabled && m.loadedCount < len(m.entries) {
		statusSuffix = fmt.Sprintf(" | %d/%d loaded", m.loadedCount, len(m.entries))
	}
	if m.parseErrors > 0 {
		statusSuffix += fmt.Sprintf(" (%d skipped)", m.parseErrors)
	}

	// Calculate remaining width for shortcuts segment
	// Shortcuts style has Padding(0, 1) = 2 chars of padding
	usedWidth := fixedWidth + tokensWidth + newEntriesWidth
	shortcutsWidth := m.width - usedWidth
	if shortcutsWidth < 0 {
		shortcutsWidth = 0
	}
	shortcutsContentWidth := shortcutsWidth - 2 // Account for padding
	if shortcutsContentWidth < 0 {
		shortcutsContentWidth = 0
	}

	// Build footer segments list
	var segments []string
	segments = append(segments, modeSegment)
	if newEntriesSegment != "" {
		segments = append(segments, newEntriesSegment)
	}
	segments = append(segments, posSegment)
	if tokensSegment != "" {
		segments = append(segments, tokensSegment)
	}

	// Toast rendering (Story 4.2) - replaces shortcuts segment when active
	var footer string
	if m.toast != "" && time.Now().Before(m.toastExpiry) {
		toastStyle := lipgloss.NewStyle().
			Background(accentColor).
			Foreground(whiteColor).
			Padding(0, 1)
		toastSegment := toastStyle.Render(m.toast)
		toastWidth := lipgloss.Width(toastSegment)
		remainingWidth := shortcutsContentWidth - toastWidth
		if remainingWidth < 0 {
			remainingWidth = 0
		}
		// Truncate shortcuts text to fit available space
		truncatedShortcuts := shortcutsText
		if len(truncatedShortcuts) > remainingWidth {
			if remainingWidth > 0 {
				truncatedShortcuts = truncatedShortcuts[:remainingWidth]
			} else {
				truncatedShortcuts = ""
			}
		}
		shortcutsSegment := Styles.StatusBarSegment.Shortcuts.
			Width(shortcutsWidth - toastWidth).
			Render(truncatedShortcuts)
		segments = append(segments, toastSegment, shortcutsSegment)
	} else {
		// Truncate shortcuts text + suffix to fit available space
		fullText := shortcutsText + statusSuffix
		if len(fullText) > shortcutsContentWidth {
			if shortcutsContentWidth > 0 {
				fullText = fullText[:shortcutsContentWidth]
			} else {
				fullText = ""
			}
		}
		shortcutsSegment := Styles.StatusBarSegment.Shortcuts.
			Width(shortcutsWidth).
			Render(fullText)
		segments = append(segments, shortcutsSegment)
	}

	footer = lipgloss.JoinHorizontal(lipgloss.Top, segments...)

	normalView := fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)

	// If overlay spinner is shown, overlay it on top of the normal view
	if m.showOverlaySpinner {
		return overlaySpinnerView(normalView, m.overlaySpinner.View(), "Loading...", m.width, m.height)
	}

	return normalView
}

// updateContent renders entries and updates the viewport content.
// With lazy loading, only renders up to loadedCount entries.
func (m *ViewerModel) updateContent() {
	var content strings.Builder

	// Render only loadedCount entries for lazy loading
	renderCount := m.loadedCount
	if renderCount > len(m.entries) {
		renderCount = len(m.entries)
	}

	// Reset and track entry positions for navigation (Story 4.2)
	m.entryLinePositions = make([]int, 0, renderCount)
	currentLine := 0

	for i := 0; i < renderCount; i++ {
		// Track position BEFORE rendering this entry (Story 4.2)
		m.entryLinePositions = append(m.entryLinePositions, currentLine)

		rendered := m.getCachedRender(i, m.entries[i])
		// Prepend line numbers if enabled (Story 4.1)
		if m.showLineNumbers {
			rendered = prependGutter(i+1, rendered, m.gutterWidth) // 1-indexed line numbers
		}
		content.WriteString(rendered)
		content.WriteString("\n")

		// Count lines in rendered content (including the trailing newline) (Story 4.2)
		currentLine += strings.Count(rendered, "\n") + 1
	}

	// Add loading indicator at the bottom if more content is available
	// Note: Lazy loading indicator is NOT a real entry, so no line number prepended
	if m.lazyEnabled && renderCount < len(m.entries) {
		if m.lazyLoadState == LoadingStateLoading {
			content.WriteString(ListStyles.Loading.Render("Loading more messages..."))
		} else {
			content.WriteString(Styles.Muted.Render(fmt.Sprintf("-- %d more entries (scroll down to load) --", len(m.entries)-renderCount)))
		}
		content.WriteString("\n")
	}

	m.viewport.SetContent(content.String())
}

// syncEntryLinePositions rebuilds entry line positions from rendered content (CR fix).
// Called after bulk load when pre-rendered content skips updateContent().
func (m *ViewerModel) syncEntryLinePositions() {
	renderCount := m.loadedCount
	if renderCount > len(m.entries) {
		renderCount = len(m.entries)
	}

	m.entryLinePositions = make([]int, 0, renderCount)
	currentLine := 0

	for i := 0; i < renderCount; i++ {
		m.entryLinePositions = append(m.entryLinePositions, currentLine)

		// Estimate line count - get cached render if available
		rendered := ""
		if cached, ok := m.renderCache[i]; ok {
			rendered = cached
		} else {
			rendered = m.renderEntry(m.entries[i])
		}
		// Prepend gutter for accurate line counting
		if m.showLineNumbers {
			rendered = prependGutter(i+1, rendered, m.gutterWidth)
		}
		// Count lines including trailing newline
		currentLine += strings.Count(rendered, "\n") + 1
	}
}

// navigateToEntry jumps viewport to the specified entry (1-indexed) (Story 4.2, 4.3).
// In raw mode, navigates to raw JSONL line number instead of entry number.
// If target line is not yet loaded (lazy loading), loads all content first.
func (m *ViewerModel) navigateToEntry(entryNum int) error {
	if m.rawMode {
		// Raw mode: navigate to raw line (Story 4.3)
		if m.rawLineCount == 0 || entryNum < 1 || entryNum > m.rawLineCount {
			return fmt.Errorf("invalid line number")
		}
		// If target line not yet loaded, load all (lazy loading case)
		if entryNum > m.loadedCount {
			m.loadedCount = m.rawLineCount
			m.lazyLoadState = LoadingStateComplete
			m.updateRawContent()
		}
		if entryNum <= len(m.rawLinePositions) {
			m.viewport.SetYOffset(m.rawLinePositions[entryNum-1])
		}
	} else {
		// Normal mode: navigate to entry (Story 4.2)
		if len(m.entries) == 0 || entryNum < 1 || entryNum > len(m.entries) {
			return fmt.Errorf("invalid line number")
		}
		// If target entry not yet loaded, load all (lazy loading case)
		if entryNum > m.loadedCount {
			m.loadedCount = len(m.entries)
			m.lazyLoadState = LoadingStateComplete
			m.updateContent()
		}
		if entryNum <= len(m.entryLinePositions) {
			m.viewport.SetYOffset(m.entryLinePositions[entryNum-1])
		}
	}
	return nil
}

// loadRawJSONL reads the JSONL file and stores raw lines (Story 4.3).
// Uses same 1MB buffer as parser (project-context.md).
func (m *ViewerModel) loadRawJSONL() error {
	if m.renderOpts.FilePath == "" {
		return fmt.Errorf("no file path available")
	}
	file, err := os.Open(m.renderOpts.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	m.rawLines = make([]string, 0)
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // Max 1MB per line (matches parser)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 { // Skip empty lines
			m.rawLines = append(m.rawLines, line)
		}
	}
	m.rawLineCount = len(m.rawLines)
	return scanner.Err()
}

// formatJSONLine pretty-prints a JSON line with 2-space indentation (Story 4.3).
// Handles both JSON objects and arrays. Invalid JSON is returned as-is (graceful degradation).
func formatJSONLine(line string) string {
	// Use json.Indent for direct byte transformation (handles any valid JSON type)
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(line), "", "  "); err != nil {
		return line // Not valid JSON, return as-is
	}
	return buf.String()
}

// updateRawContent renders raw JSONL with pretty-print formatting (Story 4.3).
// Mirrors updateContent() pattern with lazy loading support.
func (m *ViewerModel) updateRawContent() {
	var content strings.Builder
	gutterWidth := calculateGutterWidth(m.rawLineCount)

	m.rawLinePositions = make([]int, 0, m.rawLineCount)
	currentLine := 0

	// LazyThreshold = 100 per project-context.md lazy loading rules
	renderCount := m.loadedCount
	if renderCount > m.rawLineCount {
		renderCount = m.rawLineCount
	}

	for i := 0; i < renderCount; i++ {
		m.rawLinePositions = append(m.rawLinePositions, currentLine)
		formatted := formatJSONLine(m.rawLines[i])

		if m.showLineNumbers {
			formatted = prependGutter(i+1, formatted, gutterWidth)
		}
		content.WriteString(formatted)
		content.WriteString("\n")
		currentLine += strings.Count(formatted, "\n") + 1
	}

	if m.lazyEnabled && renderCount < m.rawLineCount {
		content.WriteString(Styles.Muted.Render(
			fmt.Sprintf("-- %d more lines (scroll down to load) --",
				m.rawLineCount-renderCount)))
		content.WriteString("\n")
	}
	m.viewport.SetContent(content.String())
}

// showToast displays a temporary toast message (Story 4.2).
func (m *ViewerModel) showToast(message string, duration time.Duration) tea.Cmd {
	m.toastID++ // Increment ID to invalidate any pending expiry timers
	currentID := m.toastID
	m.toast = message
	m.toastExpiry = time.Now().Add(duration)
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return toastExpiredMsg{id: currentID}
	})
}

// renderEntry renders a single log entry.
func (m *ViewerModel) renderEntry(entry types.LogEntry) string {
	switch entry.Type {
	case types.EntryTypeUser:
		return m.renderUserMessage(entry)
	case types.EntryTypeAssistant:
		return m.renderAssistantMessage(entry)
	default:
		return ""
	}
}

// renderUserMessage renders a user message entry.
func (m *ViewerModel) renderUserMessage(entry types.LogEntry) string {
	timestamp := formatTimestamp(entry.Timestamp)
	tokenInfo := formatTokenUsage(entry, m.tokenService)

	header := fmt.Sprintf("%s %s  %s",
		UserIcon,
		Styles.UserHeader.Render("User"),
		Styles.Timestamp.Render(timestamp),
	)
	// Add token info to header if available (Story 6.3)
	if tokenInfo != "" {
		header = fmt.Sprintf("%s  %s", header, Styles.TokenInfo.Render(tokenInfo))
	}

	// Wrap content to fit viewport width (with margin for styling and gutter)
	gutterSpace := 0
	if m.showLineNumbers {
		gutterSpace = m.gutterWidth + len(GutterSeparator) // Story 4.1
	}
	wrapWidth := m.width - 4 - gutterSpace
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrappedText := WrapText(entry.Message.TextContent, wrapWidth)
	content := Styles.MessageContent.Render(wrappedText)

	return Styles.UserMessage.Render(header + "\n" + content)
}

// renderAssistantMessage renders an assistant message entry.
func (m *ViewerModel) renderAssistantMessage(entry types.LogEntry) string {
	timestamp := formatTimestamp(entry.Timestamp)
	tokenInfo := formatTokenUsage(entry, m.tokenService)

	header := fmt.Sprintf("%s %s  %s",
		AssistantIcon,
		Styles.AssistantHeader.Render("Assistant"),
		Styles.Timestamp.Render(timestamp),
	)
	// Add token info to header if available (Story 6.3)
	if tokenInfo != "" {
		header = fmt.Sprintf("%s  %s", header, Styles.TokenInfo.Render(tokenInfo))
	}

	var parts []string
	parts = append(parts, header)

	for _, content := range entry.Message.Content {
		switch content.Type {
		case types.ContentTypeText:
			if content.Text != "" {
				// Use Glamour for markdown rendering
				rendered := m.markdownRenderer.Render(content.Text)
				parts = append(parts, rendered) // No extra styling - Glamour handles it
			}

		case types.ContentTypeThinking:
			if m.renderOpts.HideThoughts {
				continue // Skip thinking blocks when hidden
			}
			parts = append(parts, m.renderThinkingBlock(content))

		case types.ContentTypeToolUse:
			if m.renderOpts.HideTools {
				continue // Skip tool blocks when hidden
			}
			parts = append(parts, m.renderToolUseBlock(content))
		}
	}

	return Styles.AssistantMessage.Render(strings.Join(parts, "\n"))
}

// renderThinkingBlock renders a thinking content block.
func (m *ViewerModel) renderThinkingBlock(content types.MessageContent) string {
	if !m.showThinking {
		return Styles.CollapsedIndicator.Render(
			fmt.Sprintf("%s [thinking - press 't' to expand]", ThinkingIcon),
		)
	}

	// Wrap thinking content to viewport width (account for gutter)
	gutterSpace := 0
	if m.showLineNumbers {
		gutterSpace = m.gutterWidth + len(GutterSeparator) // Story 4.1
	}
	wrapWidth := m.width - 4 - gutterSpace
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrappedThinking := WrapText(content.Thinking, wrapWidth)

	header := fmt.Sprintf("%s %s", ThinkingIcon, Styles.ThinkingHeader.Render("Thinking"))
	return Styles.ThinkingBlock.Render(header + "\n" + wrappedThinking)
}

// renderToolUseBlock renders a tool use content block.
func (m *ViewerModel) renderToolUseBlock(content types.MessageContent) string {
	header := fmt.Sprintf("%s %s: %s",
		ToolIcon,
		Styles.ToolHeader.Render("Tool"),
		content.ToolName,
	)

	if !m.showToolInputs {
		summary := formatToolSummary(content.ToolName, content.ToolInput)
		return Styles.ToolBlock.Render(
			header + " " + Styles.CollapsedIndicator.Render(summary),
		)
	}

	// Calculate wrap width for tool input content (account for gutter)
	gutterSpace := 0
	if m.showLineNumbers {
		gutterSpace = m.gutterWidth + len(GutterSeparator) // Story 4.1
	}
	wrapWidth := m.width - 4 - gutterSpace
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	// Render tool inputs with wrapping
	inputStr := formatToolInput(content.ToolInput)
	wrappedInput := WrapText(inputStr, wrapWidth)

	return Styles.ToolBlock.Render(header + "\n" + wrappedInput)
}

// formatToolInput formats tool input as a readable string.
func formatToolInput(input map[string]any) string {
	if len(input) == 0 {
		return "(no input)"
	}

	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "(error formatting input)"
	}

	return string(data)
}

// renderEntryStatic renders a single log entry without model state (for async rendering).
// gutterWidth parameter accounts for line number gutter space (Story 4.1).
// tokenSvc parameter enables token info display (Story 6.3) - can be nil for graceful degradation.
func renderEntryStatic(entry types.LogEntry, width int, showThinking, showToolInputs bool, opts RenderOptions, mdRenderer *MarkdownRenderer, gutterWidth int, tokenSvc *token.Service) string {
	switch entry.Type {
	case types.EntryTypeUser:
		return renderUserMessageStatic(entry, width, gutterWidth, tokenSvc)
	case types.EntryTypeAssistant:
		return renderAssistantMessageStatic(entry, width, showThinking, showToolInputs, opts, mdRenderer, gutterWidth, tokenSvc)
	default:
		return ""
	}
}

// renderUserMessageStatic renders a user message entry without model state.
// gutterWidth parameter accounts for line number gutter space (Story 4.1).
// tokenSvc parameter enables token info display (Story 6.3) - can be nil for graceful degradation.
func renderUserMessageStatic(entry types.LogEntry, width int, gutterWidth int, tokenSvc *token.Service) string {
	timestamp := formatTimestamp(entry.Timestamp)
	tokenInfo := formatTokenUsage(entry, tokenSvc)

	header := fmt.Sprintf("%s %s  %s",
		UserIcon,
		Styles.UserHeader.Render("User"),
		Styles.Timestamp.Render(timestamp),
	)
	// Add token info to header if available (Story 6.3)
	if tokenInfo != "" {
		header = fmt.Sprintf("%s  %s", header, Styles.TokenInfo.Render(tokenInfo))
	}

	// Account for gutter width (Story 4.1)
	gutterSpace := gutterWidth + len(GutterSeparator)
	wrapWidth := width - 4 - gutterSpace
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrappedText := WrapText(entry.Message.TextContent, wrapWidth)
	content := Styles.MessageContent.Render(wrappedText)

	return Styles.UserMessage.Render(header + "\n" + content)
}

// renderAssistantMessageStatic renders an assistant message entry without model state.
// gutterWidth parameter accounts for line number gutter space (Story 4.1).
// tokenSvc parameter enables token info display (Story 6.3) - can be nil for graceful degradation.
func renderAssistantMessageStatic(entry types.LogEntry, width int, showThinking, showToolInputs bool, opts RenderOptions, mdRenderer *MarkdownRenderer, gutterWidth int, tokenSvc *token.Service) string {
	timestamp := formatTimestamp(entry.Timestamp)
	tokenInfo := formatTokenUsage(entry, tokenSvc)

	header := fmt.Sprintf("%s %s  %s",
		AssistantIcon,
		Styles.AssistantHeader.Render("Assistant"),
		Styles.Timestamp.Render(timestamp),
	)
	// Add token info to header if available (Story 6.3)
	if tokenInfo != "" {
		header = fmt.Sprintf("%s  %s", header, Styles.TokenInfo.Render(tokenInfo))
	}

	var parts []string
	parts = append(parts, header)

	for _, content := range entry.Message.Content {
		switch content.Type {
		case types.ContentTypeText:
			if content.Text != "" {
				// Use Glamour for markdown rendering
				rendered := mdRenderer.Render(content.Text)
				parts = append(parts, rendered) // No extra styling - Glamour handles it
			}

		case types.ContentTypeThinking:
			if opts.HideThoughts {
				continue // Skip thinking blocks when hidden
			}
			parts = append(parts, renderThinkingBlockStatic(content, width, showThinking, gutterWidth))

		case types.ContentTypeToolUse:
			if opts.HideTools {
				continue // Skip tool blocks when hidden
			}
			parts = append(parts, renderToolUseBlockStatic(content, width, showToolInputs, gutterWidth))
		}
	}

	return Styles.AssistantMessage.Render(strings.Join(parts, "\n"))
}

// renderThinkingBlockStatic renders a thinking content block without model state.
// gutterWidth parameter accounts for line number gutter space (Story 4.1).
func renderThinkingBlockStatic(content types.MessageContent, width int, showThinking bool, gutterWidth int) string {
	if !showThinking {
		return Styles.CollapsedIndicator.Render(
			fmt.Sprintf("%s [thinking - press 't' to expand]", ThinkingIcon),
		)
	}

	// Account for gutter width (Story 4.1)
	gutterSpace := gutterWidth + len(GutterSeparator)
	wrapWidth := width - 4 - gutterSpace
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrappedThinking := WrapText(content.Thinking, wrapWidth)

	header := fmt.Sprintf("%s %s", ThinkingIcon, Styles.ThinkingHeader.Render("Thinking"))
	return Styles.ThinkingBlock.Render(header + "\n" + wrappedThinking)
}

// renderToolUseBlockStatic renders a tool use content block without model state.
// gutterWidth parameter accounts for line number gutter space (Story 4.1).
func renderToolUseBlockStatic(content types.MessageContent, width int, showToolInputs bool, gutterWidth int) string {
	header := fmt.Sprintf("%s %s: %s",
		ToolIcon,
		Styles.ToolHeader.Render("Tool"),
		content.ToolName,
	)

	if !showToolInputs {
		summary := formatToolSummary(content.ToolName, content.ToolInput)
		return Styles.ToolBlock.Render(
			header + " " + Styles.CollapsedIndicator.Render(summary),
		)
	}

	// Account for gutter width (Story 4.1)
	gutterSpace := gutterWidth + len(GutterSeparator)
	wrapWidth := width - 4 - gutterSpace
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	inputStr := formatToolInput(content.ToolInput)
	wrappedInput := WrapText(inputStr, wrapWidth)

	return Styles.ToolBlock.Render(header + "\n" + wrappedInput)
}

// formatToolSummary creates a brief summary of tool input for collapsed display.
func formatToolSummary(toolName string, input map[string]any) string {
	switch toolName {
	case "Read":
		filePath, _ := input["file_path"].(string)
		if filePath == "" {
			return "Read: [collapsed]"
		}
		fileName := filepath.Base(filePath)
		if offset, ok := input["offset"].(float64); ok {
			limit, _ := input["limit"].(float64)
			if limit == 0 {
				limit = 100 // default
			}
			return fmt.Sprintf("Read: %s (lines %d-%d)", fileName, int(offset), int(offset+limit))
		}
		return fmt.Sprintf("Read: %s (full file)", fileName)

	case "Edit":
		filePath, _ := input["file_path"].(string)
		if filePath == "" {
			return "Edit: [collapsed]"
		}
		fileName := filepath.Base(filePath)
		oldStr, _ := input["old_string"].(string)
		newStr, _ := input["new_string"].(string)
		oldLines := strings.Count(oldStr, "\n") + 1
		newLines := strings.Count(newStr, "\n") + 1
		if oldLines == 1 && len(oldStr) == 0 {
			oldLines = 0
		}
		if newLines == 1 && len(newStr) == 0 {
			newLines = 0
		}
		return fmt.Sprintf("Edit: %s (+%d/-%d lines)", fileName, newLines, oldLines)

	case "Glob":
		pattern, _ := input["pattern"].(string)
		if pattern == "" {
			return "Glob: [collapsed]"
		}
		return fmt.Sprintf("Glob: %s", TruncateToWidth(pattern, 40))

	case "Grep":
		pattern, _ := input["pattern"].(string)
		path, _ := input["path"].(string)
		if pattern == "" {
			return "Grep: [collapsed]"
		}
		if path == "" {
			path = "./"
		}
		return fmt.Sprintf("Grep: \"%s\" in %s", TruncateToWidth(pattern, 25), path)

	case "Write":
		filePath, _ := input["file_path"].(string)
		if filePath == "" {
			return "Write: [collapsed]"
		}
		return fmt.Sprintf("Write: %s", filepath.Base(filePath))

	case "Bash":
		cmd, _ := input["command"].(string)
		if cmd == "" {
			return "Bash: [collapsed]"
		}
		return fmt.Sprintf("Bash: %s", TruncateToWidth(cmd, 40))

	case "Task":
		desc, _ := input["description"].(string)
		subagent, _ := input["subagent_type"].(string)
		if desc == "" {
			return "Task: [collapsed]"
		}
		if subagent != "" {
			return fmt.Sprintf("Task: %s - \"%s\"", subagent, TruncateToWidth(desc, 30))
		}
		return fmt.Sprintf("Task: %s", TruncateToWidth(desc, 40))

	case "TodoWrite":
		todos, ok := input["todos"].([]any)
		if !ok {
			return "TodoWrite: [collapsed]"
		}
		return fmt.Sprintf("TodoWrite: %d items", len(todos))

	case "WebFetch":
		url, _ := input["url"].(string)
		if url == "" {
			return "WebFetch: [collapsed]"
		}
		return fmt.Sprintf("WebFetch: %s", TruncateToWidth(url, 40))

	case "WebSearch":
		query, _ := input["query"].(string)
		if query == "" {
			return "WebSearch: [collapsed]"
		}
		return fmt.Sprintf("WebSearch: \"%s\"", TruncateToWidth(query, 35))

	case "NotebookEdit":
		notebookPath, _ := input["notebook_path"].(string)
		if notebookPath == "" {
			return "NotebookEdit: [collapsed]"
		}
		fileName := filepath.Base(notebookPath)
		editMode, _ := input["edit_mode"].(string)
		if editMode == "" {
			editMode = "replace"
		}
		return fmt.Sprintf("NotebookEdit: %s (%s)", fileName, editMode)

	default:
		return fmt.Sprintf("%s: [collapsed]", toolName)
	}
}

// invalidateRenderCache clears the render cache and updates the cache width.
// Called on resize, toggle changes, and file reset.
func (m *ViewerModel) invalidateRenderCache() {
	m.renderCache = make(map[int]string)
	m.cacheWidth = m.width - 4
}

// getCachedRender returns cached rendered content for an entry, or renders and caches it.
// Only assistant entries are cached since markdown rendering via Glamour is expensive.
// User/tool/thinking entries render fast without caching.
func (m *ViewerModel) getCachedRender(idx int, entry types.LogEntry) string {
	// Only cache assistant entries (markdown rendering is expensive)
	if entry.Type != types.EntryTypeAssistant {
		return m.renderEntry(entry)
	}

	// Check cache hit
	if cached, ok := m.renderCache[idx]; ok {
		return cached
	}

	// Cache miss - render and store
	rendered := m.renderEntry(entry)
	m.renderCache[idx] = rendered
	return rendered
}

// isAtBottom returns true if the viewport is at or near the bottom.
// This is used for smart auto-scroll behavior in watch mode.
func (m *ViewerModel) isAtBottom() bool {
	// AtBottom() returns true when scrolled to end
	// ScrollPercent() >= 0.99 handles edge cases near bottom
	return m.viewport.AtBottom() || m.viewport.ScrollPercent() >= 0.99
}

// findPositionByOffset performs binary search to find the 1-indexed position
// for a given yOffset within a sorted positions array. Returns 1 if empty.
// This is a shared helper for findVisibleEntry and findVisibleRawLine (Story 4.6).
func findPositionByOffset(positions []int, yOffset int) int {
	if len(positions) == 0 {
		return 1
	}

	// Binary search for largest position <= yOffset
	lo, hi := 0, len(positions)-1
	result := 0

	for lo <= hi {
		mid := (lo + hi) / 2
		if positions[mid] <= yOffset {
			result = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	return result + 1 // 1-indexed for display
}

// findVisibleEntry returns the 1-indexed entry number visible at yOffset.
// Uses binary search on entryLinePositions for O(log n) lookup.
func (m *ViewerModel) findVisibleEntry(yOffset int) int {
	return findPositionByOffset(m.entryLinePositions, yOffset)
}

// findVisibleRawLine returns the 1-indexed raw line number visible at yOffset.
// Uses binary search on rawLinePositions for O(log n) lookup.
func (m *ViewerModel) findVisibleRawLine(yOffset int) int {
	return findPositionByOffset(m.rawLinePositions, yOffset)
}

// performSearch searches through the content and finds matching lines.
func (m *ViewerModel) performSearch() {
	if m.searchQuery == "" {
		m.searchMatches = nil
		m.noResults = false
		m.updateContent()
		return
	}

	query := strings.ToLower(m.searchQuery)
	content := m.viewport.View()
	lines := strings.Split(content, "\n")

	m.searchMatches = nil
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}

	if len(m.searchMatches) == 0 {
		m.noResults = true
		m.currentMatch = 0
	} else {
		m.noResults = false
		m.currentMatch = 0
		// Jump to first match
		m.viewport.SetYOffset(m.searchMatches[0])
	}

	m.updateContent()
}

// nextMatch jumps to the next search match.
func (m *ViewerModel) nextMatch() {
	if len(m.searchMatches) == 0 {
		return
	}

	m.currentMatch = (m.currentMatch + 1) % len(m.searchMatches)
	m.viewport.SetYOffset(m.searchMatches[m.currentMatch])
}

// prevMatch jumps to the previous search match.
func (m *ViewerModel) prevMatch() {
	if len(m.searchMatches) == 0 {
		return
	}

	m.currentMatch--
	if m.currentMatch < 0 {
		m.currentMatch = len(m.searchMatches) - 1
	}
	m.viewport.SetYOffset(m.searchMatches[m.currentMatch])
}
