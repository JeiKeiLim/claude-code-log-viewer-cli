# Story 6.3: Statistics Display

Status: done

## Story

As a **developer reviewing logs**,
I want **to see token counts in the viewer**,
So that **I understand token consumption patterns**.

## Acceptance Criteria

1. **Given** an entry has `usage` data in the log **When** displayed **Then** shows "Tokens: 1,234 (from log)"

2. **Given** an entry has no `usage` data **When** displayed **Then** calculates tokens and shows "Tokens: ~1,200 (estimated)"

3. **Given** a conversation is loaded **When** viewing the status bar **Then** total tokens for conversation is displayed (e.g., "Tokens: 12,345" or "Tokens: ~12,345")

## Tasks / Subtasks

- [x] Task 1: Pass token service from AppModel to ViewerModel (AC: #1, #2, #3)
  - [x] Add `tokenService *token.Service` field to ViewerModel struct
  - [x] Add `conversationTokens int` and `tokensEstimated bool` fields to ViewerModel struct
  - [x] Modify `NewViewerModel()` to accept `tokenSvc *token.Service` as final parameter
  - [x] In `NewViewerModel()`, calculate conversation totals using `tokenSvc.CalculateConversation()` if service is non-nil
  - [x] Modify `NewViewerModelWithBack()` to accept `tokenSvc *token.Service` and pass to `NewViewerModel()`
  - [x] Modify `NewViewerModelWithBackNavigation()` to accept `tokenSvc *token.Service` and pass to `NewViewerModelWithBack()`
  - [x] Update AppModel `conversationLoadedMsg` handler to pass `m.tokenService` when creating ViewerModel
  - [x] Update AppModel `conversationLoadedWithWatchMsg` handler to pass `m.tokenService` when creating ViewerModel
  - [x] Update AppModel `OpenViewerFromDashboardMsg` handler to pass `m.tokenService` when creating ViewerModel
  - [x] Update dashboard's pane viewer creation to pass `nil` (dashboard panes show preview content, not full statistics)

- [x] Task 2: Add helper functions in utils.go (AC: #1, #2)
  - [x] Create `formatWithCommas(n int) string` helper function
    - Return number with thousand separators (e.g., 1234 -> "1,234")
    - Handle numbers < 1000 without separators
  - [x] Create `formatTokenUsage(entry types.LogEntry, svc *token.Service) string` helper function
    - If `entry.Type == types.EntryTypeFileHistorySnapshot`, return `""`
    - If `!entry.Usage.IsEmpty()`, return `fmt.Sprintf("Tokens: %s (from log)", formatWithCommas(entry.Usage.Total()))`
    - If `svc == nil`, return `""` (graceful degradation)
    - Otherwise return `fmt.Sprintf("Tokens: ~%s (estimated)", formatWithCommas(svc.CalculateEntry(entry)))`

- [x] Task 3: Add TokenInfo style to styles.go (AC: #1, #2)
  - [x] Add `TokenInfo lipgloss.Style` field to Styles struct
  - [x] Initialize with muted/italic styling: `Foreground(dimColor).Italic(true)`
  - [x] Add `Tokens lipgloss.Style` field to `StatusBarSegment` struct
  - [x] Initialize StatusBarSegment.Tokens with same styling as Position segment (purple background)

- [x] Task 4: Add per-entry token display in viewer.go (AC: #1, #2)
  - [x] In `renderUserMessage()`: After timestamp, add token info if non-empty
    - Get token info: `tokenInfo := formatTokenUsage(entry, m.tokenService)`
    - If `tokenInfo != ""`, append to header: `header = fmt.Sprintf("%s  %s", header, Styles.TokenInfo.Render(tokenInfo))`
  - [x] In `renderAssistantMessage()`: After timestamp, add token info if non-empty (same pattern)

- [x] Task 5: Add conversation-level token summary to status bar (AC: #3)
  - [x] Create `buildTokensSegment() string` method on ViewerModel
    - If `m.tokenService == nil`, return `""`
    - Format: `"Tokens: {prefix}{formatted}"` where prefix is "~" if `m.tokensEstimated`
    - Use `Styles.StatusBarSegment.Tokens.Render()`
  - [x] In `View()`: Add tokens segment to footer between position and shortcuts segments
  - [x] Update width calculation to account for tokens segment width

- [x] Task 6: Handle watch mode token recalculation (AC: #3)
  - [x] In ViewerModel `Update()`, `watcher.NewEntriesMsg` case:
    - After appending new entries, recalculate conversation tokens
    - `if m.tokenService != nil { m.conversationTokens, m.tokensEstimated = m.tokenService.CalculateConversation(m.entries) }`

- [x] Task 7: Update static rendering functions for async bulk load (AC: #1, #2)
  - [x] Update `renderEntryStatic()` signature to add `tokenSvc *token.Service` parameter
  - [x] Update `renderUserMessageStatic()` signature to add `tokenSvc *token.Service` parameter
  - [x] Update `renderUserMessageStatic()` to include token info in header (same pattern as instance method)
  - [x] Update `renderAssistantMessageStatic()` signature to add `tokenSvc *token.Service` parameter
  - [x] Update `renderAssistantMessageStatic()` to include token info in header
  - [x] Update `markAllMessagesLoadedCmd()` to capture `m.tokenService` and pass to `renderEntryStatic()`
  - [x] **Note**: Token service is thread-safe (uses sync.RWMutex) so safe to use in goroutine

- [x] Task 8: Write comprehensive tests (AC: #1, #2, #3)
  - [x] Test `formatWithCommas()`:
    - 0 -> "0"
    - 999 -> "999"
    - 1000 -> "1,000"
    - 1234567 -> "1,234,567"
  - [x] Test `formatTokenUsage()`:
    - Entry with log usage data -> "Tokens: X,XXX (from log)"
    - Entry without log usage data (with service) -> "Tokens: ~X,XXX (estimated)"
    - Entry without log usage data (nil service) -> ""
    - FileHistorySnapshot entry -> ""
  - [x] Test `buildTokensSegment()`:
    - With token service and estimated=false -> "Tokens: 12,345"
    - With token service and estimated=true -> "Tokens: ~12,345"
    - Without token service -> ""
  - [x] Test watch mode token recalculation (ViewerModel Update with NewEntriesMsg)

- [x] Task 9: Build and validation (AC: #1, #2, #3)
  - [x] Run `make test` - all tests pass
  - [x] Run `make lint` - no linting errors
  - [x] Run `make ci` - full CI validation passes
  - [x] Manual verification: check token display in TUI for entries with and without usage data

## Dev Notes

### Existing Token Service (Story 6.1 + 6.2)

The token service is already implemented and integrated into AppModel:

```go
// internal/token/service.go - EXISTING
type Service struct {
    encoder *tiktoken.Tiktoken
    cache   map[string]int
    mu      sync.RWMutex  // Thread-safe for concurrent access
}

func (s *Service) Calculate(text string) int           // Single text
func (s *Service) CalculateBatch(texts []string) []int // Batch
func (s *Service) CalculateEntry(entry types.LogEntry) int  // Per entry
func (s *Service) CalculateConversation(entries []types.LogEntry) (int, bool) // Total + estimated flag
func (s *Service) ClearCache()
```

```go
// internal/tui/app.go - EXISTING
type AppModel struct {
    // ...
    tokenService *token.Service  // Already initialized with soft-fail
}
```

### TokenUsage Type (types/entry.go)

```go
type TokenUsage struct {
    InputTokens              int
    OutputTokens             int
    CacheCreationInputTokens int
    CacheReadInputTokens     int
}

func (u TokenUsage) Total() int      // Returns sum of all tokens
func (u TokenUsage) IsEmpty() bool   // Returns true if all fields are 0
```

### Implementation Approach

**1. Modify ViewerModel to receive token service:**

```go
type ViewerModel struct {
    // ... existing fields
    tokenService       *token.Service  // NEW: for token calculations
    conversationTokens int             // NEW: total tokens for conversation
    tokensEstimated    bool            // NEW: true if any token was estimated
}

// Update constructor - tokenSvc is the FINAL parameter
func NewViewerModel(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions, tokenSvc *token.Service) ViewerModel {
    m := ViewerModel{
        // ... existing initialization
        tokenService: tokenSvc,
    }

    // Calculate conversation totals if service available
    if tokenSvc != nil {
        m.conversationTokens, m.tokensEstimated = tokenSvc.CalculateConversation(entries)
    }

    return m
}

// Update wrapper functions - all accept tokenSvc as FINAL parameter
func NewViewerModelWithBack(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions, tokenSvc *token.Service) ViewerModel {
    m := NewViewerModel(entries, parseErrors, title, opts, tokenSvc)
    m.canGoBack = true
    return m
}

func NewViewerModelWithBackNavigation(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions, tokenSvc *token.Service) ViewerModel {
    return NewViewerModelWithBack(entries, parseErrors, title, opts, tokenSvc)
}
```

**2. Token formatting helpers (in utils.go):**

```go
// formatWithCommas formats an integer with thousand separators.
// Examples: 999 -> "999", 1000 -> "1,000", 1234567 -> "1,234,567"
func formatWithCommas(n int) string {
    str := strconv.Itoa(n)
    if n < 1000 {
        return str
    }

    var result strings.Builder
    offset := len(str) % 3
    if offset > 0 {
        result.WriteString(str[:offset])
        if offset < len(str) {
            result.WriteString(",")
        }
    }
    for i := offset; i < len(str); i += 3 {
        if i > offset {
            result.WriteString(",")
        }
        result.WriteString(str[i : i+3])
    }
    return result.String()
}

// formatTokenUsage returns a formatted token string for display.
// Returns empty string for FileHistorySnapshot entries or if token service is nil.
func formatTokenUsage(entry types.LogEntry, svc *token.Service) string {
    // Skip file-history-snapshot entries (no user-facing text)
    if entry.Type == types.EntryTypeFileHistorySnapshot {
        return ""
    }

    // Use actual usage from log if available
    if !entry.Usage.IsEmpty() {
        return fmt.Sprintf("Tokens: %s (from log)", formatWithCommas(entry.Usage.Total()))
    }

    // Service unavailable - graceful degradation
    if svc == nil {
        return ""
    }

    // Calculate token estimate
    calculated := svc.CalculateEntry(entry)
    return fmt.Sprintf("Tokens: ~%s (estimated)", formatWithCommas(calculated))
}
```

**3. Update message rendering to include tokens:**

```go
func (m *ViewerModel) renderUserMessage(entry types.LogEntry) string {
    timestamp := formatTimestamp(entry.Timestamp)
    tokenInfo := formatTokenUsage(entry, m.tokenService)

    // Build header with optional token info
    header := fmt.Sprintf("%s %s  %s",
        UserIcon,
        Styles.UserHeader.Render("User"),
        Styles.Timestamp.Render(timestamp),
    )
    if tokenInfo != "" {
        header = fmt.Sprintf("%s  %s", header, Styles.TokenInfo.Render(tokenInfo))
    }
    // ... rest of rendering
}

func (m *ViewerModel) renderAssistantMessage(entry types.LogEntry) string {
    timestamp := formatTimestamp(entry.Timestamp)
    tokenInfo := formatTokenUsage(entry, m.tokenService)

    header := fmt.Sprintf("%s %s  %s",
        AssistantIcon,
        Styles.AssistantHeader.Render("Assistant"),
        Styles.Timestamp.Render(timestamp),
    )
    if tokenInfo != "" {
        header = fmt.Sprintf("%s  %s", header, Styles.TokenInfo.Render(tokenInfo))
    }
    // ... rest of rendering
}
```

**4. Update static rendering functions (thread-safe):**

```go
// Note: token.Service uses sync.RWMutex internally, safe for concurrent goroutine access

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

func renderUserMessageStatic(entry types.LogEntry, width int, gutterWidth int, tokenSvc *token.Service) string {
    timestamp := formatTimestamp(entry.Timestamp)
    tokenInfo := formatTokenUsage(entry, tokenSvc)

    header := fmt.Sprintf("%s %s  %s",
        UserIcon,
        Styles.UserHeader.Render("User"),
        Styles.Timestamp.Render(timestamp),
    )
    if tokenInfo != "" {
        header = fmt.Sprintf("%s  %s", header, Styles.TokenInfo.Render(tokenInfo))
    }
    // ... rest of rendering
}

func renderAssistantMessageStatic(entry types.LogEntry, width int, showThinking, showToolInputs bool, opts RenderOptions, mdRenderer *MarkdownRenderer, gutterWidth int, tokenSvc *token.Service) string {
    timestamp := formatTimestamp(entry.Timestamp)
    tokenInfo := formatTokenUsage(entry, tokenSvc)

    header := fmt.Sprintf("%s %s  %s",
        AssistantIcon,
        Styles.AssistantHeader.Render("Assistant"),
        Styles.Timestamp.Render(timestamp),
    )
    if tokenInfo != "" {
        header = fmt.Sprintf("%s  %s", header, Styles.TokenInfo.Render(tokenInfo))
    }
    // ... rest of rendering
}

// Update markAllMessagesLoadedCmd to capture and pass tokenService
func (m *ViewerModel) markAllMessagesLoadedCmd() tea.Cmd {
    // Capture values needed for rendering
    entries := m.entries
    total := len(entries)
    width := m.width
    showThinking := m.showThinking
    showToolInputs := m.showToolInputs
    opts := m.renderOpts
    mdRenderer := m.markdownRenderer
    showLineNumbers := m.showLineNumbers
    gutterWidth := m.gutterWidth
    tokenSvc := m.tokenService  // NEW: capture for async rendering

    return func() tea.Msg {
        var content strings.Builder
        for i := 0; i < total; i++ {
            rendered := renderEntryStatic(entries[i], width, showThinking, showToolInputs, opts, mdRenderer, gutterWidth, tokenSvc)
            // ... rest of rendering
        }
        // ...
    }
}
```

**5. Status bar segment for conversation total:**

```go
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
```

**6. Add styles for token info (in styles.go):**

```go
// In Styles struct definition - add TokenInfo field
var Styles = struct {
    // ... existing styles
    TokenInfo lipgloss.Style
    StatusBarSegment struct {
        Mode      lipgloss.Style
        Position  lipgloss.Style
        Shortcuts lipgloss.Style
        Tokens    lipgloss.Style  // NEW
    }
}{
    // ... existing initializations
    TokenInfo: lipgloss.NewStyle().
        Foreground(dimColor).
        Italic(true),
    StatusBarSegment: struct {
        Mode      lipgloss.Style
        Position  lipgloss.Style
        Shortcuts lipgloss.Style
        Tokens    lipgloss.Style
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
        Tokens: lipgloss.NewStyle().  // NEW: matches Position style
            Background(primaryColor).
            Foreground(whiteColor).
            Padding(0, 1),
    },
}
```

### Update Points in app.go

```go
// In conversationLoadedMsg handler
case conversationLoadedMsg:
    // ...
    m.viewerModel = NewViewerModelWithBackNavigation(
        msg.entries, msg.parseErrors, title, opts,
        m.tokenService,  // NEW: pass token service
    )

// In conversationLoadedWithWatchMsg handler
case conversationLoadedWithWatchMsg:
    // ...
    m.viewerModel = NewViewerModelWithBackNavigation(
        msg.entries, msg.parseErrors, title, opts,
        m.tokenService,  // NEW: pass token service
    )

// In OpenViewerFromDashboardMsg handler (when viewer is opened from dashboard)
case OpenViewerFromDashboardMsg:
    // When conversation finishes loading, pass token service
    // (OpenViewerFromDashboardMsg triggers loadConversation which returns conversationLoadedMsg)
    // So the conversationLoadedMsg handler above handles this case
```

### Watch Mode Token Update (in viewer.go)

```go
case watcher.NewEntriesMsg:
    // Exit raw mode on file change (Story 4.3)
    if m.rawMode {
        m.rawMode = false
    }

    // ... existing handling ...

    // Append new entries from file watcher
    m.entries = append(m.entries, msg.Entries...)
    m.loadedCount = len(m.entries)

    // NEW: Recalculate conversation tokens with new entries
    if m.tokenService != nil {
        m.conversationTokens, m.tokensEstimated = m.tokenService.CalculateConversation(m.entries)
    }

    // ... rest of existing handling ...
```

### View() Footer Update (in viewer.go)

```go
func (m ViewerModel) View() string {
    // ... existing code ...

    // Build segmented footer
    modeSegment := m.buildModeSegment()
    newEntriesSegment := m.buildNewEntriesSegment()
    posSegment := m.buildPositionSegment()
    tokensSegment := m.buildTokensSegment()  // NEW
    shortcutsText := m.buildShortcutsSegment()

    // Calculate width for shortcuts segment (fills remaining space)
    modeWidth := lipgloss.Width(modeSegment)
    newEntriesWidth := lipgloss.Width(newEntriesSegment)
    posWidth := lipgloss.Width(posSegment)
    tokensWidth := lipgloss.Width(tokensSegment)  // NEW
    shortcutsWidth := m.width - modeWidth - newEntriesWidth - posWidth - tokensWidth  // UPDATED

    // ... toast handling updated similarly ...

    // Join all segments (NEW: add tokensSegment between position and shortcuts)
    footer = lipgloss.JoinHorizontal(lipgloss.Top,
        modeSegment, newEntriesSegment, posSegment, tokensSegment, shortcutsSegment)

    // ... rest of view ...
}
```

### Dashboard Pane Note

Dashboard panes pass `nil` for token service because:
- Panes show preview content (truncated entries)
- Token statistics are per-conversation, not per-pane
- Reduces computation overhead in multi-pane view
- Users can open full viewer (Enter key) to see token statistics

### Files to Modify

| File | Changes |
|------|---------|
| `internal/tui/viewer.go` | Add tokenService/conversationTokens/tokensEstimated fields, update constructors, update render methods, add buildTokensSegment, update View() footer, handle watch mode recalculation |
| `internal/tui/app.go` | Pass tokenService to NewViewerModel calls (3 locations) |
| `internal/tui/styles.go` | Add TokenInfo style, add StatusBarSegment.Tokens style |
| `internal/tui/utils.go` | Add formatWithCommas and formatTokenUsage helpers |
| `internal/tui/utils_test.go` | Add tests for formatWithCommas |
| `internal/tui/viewer_test.go` | Add tests for formatTokenUsage, buildTokensSegment, watch mode recalculation |

### Thread Safety Note

The `token.Service` uses `sync.RWMutex` for cache access, making it safe to:
- Pass to goroutines in `markAllMessagesLoadedCmd()`
- Call `Calculate()` and `CalculateEntry()` concurrently
- No additional synchronization needed in viewer code

### References

- [internal/token/service.go - CalculateEntry, CalculateConversation methods from Story 6.2]
- [internal/types/entry.go - TokenUsage struct with Total(), IsEmpty() methods]
- [internal/tui/app.go - AppModel.tokenService field, cache clear triggers]
- [architecture-phase3.md Decision #7 - token package design]
- [project-context.md - testing rules, coverage targets]
- [epics-phase3.md#Story-6.3 - acceptance criteria]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- Implemented token service integration in ViewerModel
- Added formatWithCommas and formatTokenUsage helper functions
- Added TokenInfo and StatusBarSegment.Tokens styles
- Updated renderUserMessage/renderAssistantMessage for per-entry token display
- Added buildTokensSegment for conversation-level token summary in status bar
- Implemented watch mode token recalculation on NewEntriesMsg
- Updated static rendering functions with token service parameter (CR fix)
- Added comprehensive tests for all token display functionality

### File List

- internal/tui/viewer.go - Token service integration, buildTokensSegment, render methods with token info
- internal/tui/app.go - Pass tokenService to NewViewerModel calls
- internal/tui/styles.go - TokenInfo style, StatusBarSegment.Tokens style
- internal/tui/utils.go - formatWithCommas, formatTokenUsage helpers
- internal/tui/utils_test.go - Tests for formatWithCommas, formatTokenUsage
- internal/tui/viewer_test.go - Tests for buildTokensSegment, watch mode token recalculation
- cmd/cclv/main.go - Pass nil for tokenService in pipeline mode

