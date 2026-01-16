# Story 3.1: Integrate Glamour Markdown Renderer

Status: complete

## Story

As a **developer viewing assistant responses**,
I want **markdown to be rendered with formatting**,
So that **code blocks, headers, and lists are readable**.

## Acceptance Criteria

### AC 3.1.1: Glamour dependency
- **Given** the go.mod file
- **When** I add Glamour
- **Then** `go mod tidy` succeeds
- **And** `make build` produces working binary

### AC 3.1.2: Renderer initialization
- **Given** the Bubbletea model
- **When** it initializes
- **Then** a Glamour renderer is created with WithAutoStyle()
- **And** renderer adapts to terminal theme

### AC 3.1.3: Assistant text rendering
- **Given** a log entry with type "assistant"
- **When** the text content is displayed
- **Then** it passes through Glamour renderer
- **And** markdown formatting is applied

### AC 3.1.4: Code block syntax highlighting
- **Given** assistant text with code blocks
- **When** rendered
- **Then** code has syntax highlighting
- **And** language-specific coloring when specified

### AC 3.1.5: Non-assistant content unchanged
- **Given** user messages or tool calls
- **When** displayed
- **Then** they do NOT pass through Glamour
- **And** render as plain text with card styling

## Tasks / Subtasks

- [x] Task 1: Add Glamour dependency (AC: 3.1.1)
  - [x] 1.1: Add `github.com/charmbracelet/glamour` to go.mod requires section
  - [x] 1.2: Run `go mod tidy` to resolve version
  - [x] 1.3: Verify `make build` succeeds

- [x] Task 2: Create markdown renderer in styles.go (AC: 3.1.2)
  - [x] 2.1: Add `glamour` import to styles.go and ensure `strings` is imported for `TrimRight`
  - [x] 2.2: Create `MarkdownRenderer` struct with `renderer *glamour.TermRenderer` and `width int`
  - [x] 2.3: Create `NewMarkdownRenderer(width int) (*MarkdownRenderer, error)` function using `glamour.WithAutoStyle()` and `glamour.WithWordWrap(width)`
  - [x] 2.4: Create `Render(content string) string` method that handles errors gracefully (return raw content on error)

- [x] Task 3: Integrate renderer into ViewerModel (AC: 3.1.2, 3.1.3)
  - [x] 3.1: Add `markdownRenderer *MarkdownRenderer` field to `ViewerModel` struct in viewer.go
  - [x] 3.2: Initialize renderer in `NewViewerModel()` with initial width (use 80 as default if width=0)
  - [x] 3.3: Recreate renderer on `tea.WindowSizeMsg` when width changes significantly (>5 chars difference). Note: implement `abs(x int) int` helper or use conditional

- [x] Task 4: Render assistant text through Glamour (AC: 3.1.3, 3.1.4)
  - [x] 4.1: In `renderAssistantMessage()`, for `ContentTypeText` blocks, call `m.markdownRenderer.Render(content.Text)` instead of `WrapText()`
  - [x] 4.2: In `renderAssistantMessageStatic()`, accept `*MarkdownRenderer` parameter and use it for text rendering
  - [x] 4.3: Update `renderEntryStatic()` signature to accept `*MarkdownRenderer` parameter
  - [x] 4.4: Update `markAllMessagesLoadedCmd()` to capture renderer and pass to `renderEntryStatic()`

- [x] Task 5: Ensure non-assistant content unchanged (AC: 3.1.5)
  - [x] 5.1: Verify `renderUserMessage()` still uses `WrapText()` - do NOT modify
  - [x] 5.2: Verify `renderThinkingBlock()` still uses `WrapText()` - do NOT modify
  - [x] 5.3: Verify `renderToolUseBlock()` still uses existing formatting - do NOT modify

- [x] Task 6: Handle Glamour output trailing newlines
  - [x] 6.1: Glamour adds trailing newlines - use `strings.TrimRight(rendered, "\n")` before returning
  - [x] 6.2: This prevents extra blank lines between content blocks

- [x] Task 7: Adjust styling for Glamour output
  - [x] 7.1: Remove `Styles.MessageContent.Render()` wrapper from Glamour-rendered content (Glamour handles styling)
  - [x] 7.2: Glamour already applies its own styling - double-styling causes issues

- [x] Task 8: Add unit tests for markdown rendering
  - [x] 8.1: Test `NewMarkdownRenderer()` creates valid renderer
  - [x] 8.2: Test `Render()` converts markdown to styled output
  - [x] 8.3: Test code block rendering includes language indication
  - [x] 8.4: Test graceful fallback on render error
  - [x] 8.5: Test width is respected (no horizontal overflow)

- [x] Task 9: Run build, lint, and test validation
  - [x] 9.1: Run `make build` - verify binary size increase is reasonable
  - [x] 9.2: Run `make lint` - no errors
  - [x] 9.3: Run `make test` - all tests pass, coverage maintained

- [x] Task 10: Manual testing
  - [x] 10.1: Open a conversation with code blocks - verify syntax highlighting
  - [x] 10.2: Open a conversation with markdown headers - verify formatting
  - [x] 10.3: Open a conversation with bullet lists - verify list rendering
  - [x] 10.4: Verify user messages are NOT styled with markdown
  - [x] 10.5: Resize terminal - verify rewrap works (sets up for Story 3.2)

## Dev Notes

### Glamour Package Details

**Package:** `github.com/charmbracelet/glamour`

**Key Functions:**
```go
// Create renderer with auto theme detection and word wrap
renderer, err := glamour.NewTermRenderer(
    glamour.WithAutoStyle(),      // Detects light/dark theme
    glamour.WithWordWrap(width),  // Wraps to terminal width
)

// Render markdown to styled string
rendered, err := renderer.Render(markdownText)
```

**Glamour Characteristics:**
- Adds ANSI escape codes for syntax highlighting
- Adds trailing newlines to output - MUST trim
- Code blocks get language-specific highlighting when language is specified
- Headers become bold with different sizes
- Lists get proper indentation and bullets
- Links show as `[text](url)` style

### Integration Pattern - MarkdownRenderer Struct

**Location:** `internal/tui/styles.go` (after `ListStyles` definition, around line 348)

**Required imports:** Add to styles.go imports:
```go
import (
    "strings"  // For TrimRight in Render()

    "github.com/charmbracelet/glamour"
    // ... other imports
)
```

```go
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
    return strings.TrimRight(rendered, "\n")
}
```

### ViewerModel Changes

**Location:** `internal/tui/viewer.go`

**Add to struct (around line 42):**
```go
type ViewerModel struct {
    // ... existing fields ...

    // Markdown renderer for assistant text
    markdownRenderer *MarkdownRenderer
}
```

**Initialization in NewViewerModel (around line 89):**
```go
func NewViewerModel(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions) ViewerModel {
    // ... existing code ...

    // Initialize markdown renderer with default width
    initialWidth := opts.Width
    if initialWidth <= 0 {
        initialWidth = 80
    }
    mdRenderer, _ := NewMarkdownRenderer(initialWidth - 4) // Account for padding

    m := ViewerModel{
        // ... existing fields ...
        markdownRenderer: mdRenderer,
    }
    // ...
}
```

**Resize handling in Update() (around line 346):**
```go
case tea.WindowSizeMsg:
    // ... existing code ...

    // Recreate markdown renderer if width changed significantly
    newRenderWidth := m.width - 4
    // Note: Use conditional instead of abs() since Go has no built-in abs for int
    widthDiff := m.markdownRenderer.width - newRenderWidth
    if widthDiff < 0 {
        widthDiff = -widthDiff
    }
    if m.markdownRenderer == nil || widthDiff > 5 {
        m.markdownRenderer, _ = NewMarkdownRenderer(newRenderWidth)
    }
```

### Assistant Text Rendering Changes

**Location:** `internal/tui/viewer.go`, `renderAssistantMessage()` (around line 676)

**Current code:**
```go
case types.ContentTypeText:
    if content.Text != "" {
        wrappedText := WrapText(content.Text, wrapWidth)
        parts = append(parts, Styles.MessageContent.Render(wrappedText))
    }
```

**Change to:**
```go
case types.ContentTypeText:
    if content.Text != "" {
        // Use Glamour for markdown rendering
        rendered := m.markdownRenderer.Render(content.Text)
        parts = append(parts, rendered) // No extra styling - Glamour handles it
    }
```

### Static Rendering Changes (for async/bulk loading)

**Location:** `internal/tui/viewer.go`

**renderEntryStatic()** (line 781) - Update signature:
```go
func renderEntryStatic(entry types.LogEntry, width int, showThinking, showToolInputs bool, opts RenderOptions, mdRenderer *MarkdownRenderer) string
```

**renderAssistantMessageStatic()** (line 811) - Update signature and use renderer:
```go
func renderAssistantMessageStatic(entry types.LogEntry, width int, showThinking, showToolInputs bool, opts RenderOptions, mdRenderer *MarkdownRenderer) string {
    // ... existing code ...

    // In ContentTypeText case, replace:
    // wrappedText := WrapText(content.Text, wrapWidth)
    // parts = append(parts, Styles.MessageContent.Render(wrappedText))
    // With:
    rendered := mdRenderer.Render(content.Text)
    parts = append(parts, rendered)  // No extra styling
}
```

**renderEntryStatic()** (line 781) - Update to pass renderer to renderAssistantMessageStatic:
```go
func renderEntryStatic(entry types.LogEntry, width int, showThinking, showToolInputs bool, opts RenderOptions, mdRenderer *MarkdownRenderer) string {
    switch entry.EntryType {
    // ...
    case types.EntryTypeAssistant:
        return renderAssistantMessageStatic(entry, width, showThinking, showToolInputs, opts, mdRenderer)  // Pass renderer
    // ...
    }
}
```

**markAllMessagesLoadedCmd()** (line 472) - Update to capture and pass renderer:
```go
func (m *ViewerModel) markAllMessagesLoadedCmd() tea.Cmd {
    // Capture values needed for rendering
    entries := m.entries
    total := len(entries)
    width := m.width
    showThinking := m.showThinking
    showToolInputs := m.showToolInputs
    opts := m.renderOpts
    mdRenderer := m.markdownRenderer  // Add this capture

    return func() tea.Msg {
        var content strings.Builder
        for i := 0; i < total; i++ {
            rendered := renderEntryStatic(entries[i], width, showThinking, showToolInputs, opts, mdRenderer)  // Pass renderer
            content.WriteString(rendered)
            content.WriteString("\n")
        }
        return viewerMessagesLoadedMsg{
            loadedCount:     total,
            renderedContent: content.String(),
        }
    }
}
```

### Critical Constraints

| Constraint | Requirement |
|------------|-------------|
| **NO EMOJI in UI** | Glamour may render some emoji - acceptable in assistant content |
| **Use Makefile** | `make build`, `make test`, never raw go commands |
| **Graceful degradation** | If renderer fails, return raw content - never crash |
| **Import order** | stdlib -> external -> internal |
| **Trim trailing newlines** | Glamour adds `\n` - must trim to prevent layout issues |
| **Don't double-style** | Remove Styles.MessageContent wrapper from Glamour output |

### DO NOT Modify

| Component | Reason |
|-----------|--------|
| `renderUserMessage()` | User messages are plain text, not markdown |
| `renderThinkingBlock()` | Thinking is already muted style |
| `renderToolUseBlock()` | Tool blocks use JSON formatting |
| Existing word wrap | Still used for non-assistant content |

### Edge Cases

| Case | Handling |
|------|----------|
| Invalid markdown | Glamour handles gracefully, renders as-is |
| Empty content | Return empty string |
| Very long code blocks | Glamour respects word wrap width |
| Nested markdown | Glamour handles recursively |
| Renderer creation fails | Continue with nil renderer, graceful fallback |
| Width is 0 or negative | Use default width of 80 |

### Performance Considerations

- Glamour rendering is relatively fast (~1ms per message)
- Renderer creation is O(1) - do once per width change
- Pre-rendered content cached in viewport - no re-render on scroll
- Story 3.3 will add explicit render caching for further optimization

### Testing Strategy

**Unit Tests (styles_test.go):**
```go
func TestNewMarkdownRenderer(t *testing.T) {
    r, err := NewMarkdownRenderer(80)
    assert.NoError(t, err)
    assert.NotNil(t, r)
}

func TestMarkdownRenderCodeBlock(t *testing.T) {
    r, _ := NewMarkdownRenderer(80)
    input := "```go\nfunc main() {}\n```"
    output := r.Render(input)
    // Verify output contains ANSI codes (styled)
    assert.Contains(t, output, "\x1b[")
}

func TestMarkdownRenderNilRenderer(t *testing.T) {
    var r *MarkdownRenderer
    output := r.Render("test")
    assert.Equal(t, "test", output) // Graceful fallback
}

func TestMarkdownRenderError(t *testing.T) {
    // Test graceful handling of render errors
}

func TestMarkdownTrimTrailingNewlines(t *testing.T) {
    r, _ := NewMarkdownRenderer(80)
    output := r.Render("# Header")
    assert.False(t, strings.HasSuffix(output, "\n"))
}
```

### Git Intelligence

Recent commits show established patterns:
```
1daa088 feat: enable watch mode from interactive browse
e217df7 feat: implement smart auto-scroll for live watch mode
bbbf70a feat: implement fsnotify file watching for live log updates
```

All follow pattern: `feat: <verb> <feature description>`

Suggested commit message:
```
feat: integrate Glamour markdown renderer for assistant text

- Add github.com/charmbracelet/glamour dependency
- Create MarkdownRenderer struct in styles.go with graceful fallback
- Render assistant text content through Glamour
- Maintain existing styling for user/tool/thinking content
- Trim trailing newlines from Glamour output

Story 3.1 of Epic 3: Markdown Rendering

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Dependencies

- **None within Epic 3** - This is the first story
- **Glamour depends on:** Already in Charm stack ecosystem, should resolve cleanly
- **Sets up for:** Story 3.2 (Dynamic Word Wrap) and Story 3.3 (Render Caching)

### Project Structure Notes

**Files to modify:**
- `go.mod` - Add Glamour dependency (line 6-11 requires section)
- `internal/tui/styles.go` - Add MarkdownRenderer struct (after line 349)
- `internal/tui/viewer.go` - Add renderer field, initialize, use in rendering (multiple locations)

**Files to create:**
- None - all changes in existing files

**Alignment with unified project structure:**
- Follows established TEA patterns from Epic 1 and 2
- Uses existing styles.go for new struct (pattern from Theme, Styles)
- Import pattern matches existing files

### Existing Infrastructure to Reuse

| Component | Location | Use |
|-----------|----------|-----|
| `WrapText()` | internal/tui/utils.go | Still used for non-markdown content |
| `Styles.MessageContent` | internal/tui/styles.go | NOT used for Glamour output |
| `Styles.AssistantMessage` | internal/tui/styles.go | Still used as container border |
| Width calculation | viewer.go | Same pattern for wrapWidth |

### References

- [Source: epics.md lines 890-942] - Story 3.1 requirements and acceptance criteria
- [Source: project-context.md lines 37-39] - Glamour is approved dependency
- [Source: prd.md lines 144-154] - FR-301 Glamour Integration requirements
- [Source: internal/tui/viewer.go:676-717] - renderAssistantMessage() to modify
- [Source: internal/tui/viewer.go:811-851] - renderAssistantMessageStatic() to modify
- [Source: internal/tui/viewer.go:781-809] - renderEntryStatic() to modify
- [Source: internal/tui/styles.go:324-348] - ListStyles end, place MarkdownRenderer after
- [Source: internal/tui/viewer.go:472-496] - markAllMessagesLoadedCmd() to update

## Implementation Checklist

Before marking story complete, verify:

- [x] `go mod tidy` succeeds after adding Glamour
- [x] `make build` succeeds with Glamour dependency
- [x] `make lint` has no errors
- [x] `make test` passes with no regressions
- [x] Assistant text shows markdown formatting (headers, code, lists)
- [x] Code blocks have syntax highlighting
- [x] User messages are NOT markdown-styled
- [x] Tool blocks are NOT markdown-styled
- [x] Thinking blocks are NOT markdown-styled
- [x] No extra blank lines between content blocks (trailing newlines trimmed)
- [x] Terminal resize doesn't crash (graceful renderer recreation)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

None - implementation was clean with no issues

### Completion Notes List

1. Added Glamour v0.10.0 via `go get github.com/charmbracelet/glamour@latest`
2. Created `MarkdownRenderer` struct in styles.go with `NewMarkdownRenderer()`, `Render()`, and `Width()` methods
3. Integrated renderer into `ViewerModel` with initialization in `NewViewerModel()` and recreation on `WindowSizeMsg`
4. Updated `renderAssistantMessage()` and `renderAssistantMessageStatic()` to use Glamour for text content
5. Updated `renderEntryStatic()` and `markAllMessagesLoadedCmd()` signatures to pass markdown renderer
6. Trailing newlines trimmed in `Render()` method via `strings.TrimRight()`
7. Removed `Styles.MessageContent.Render()` wrapper from Glamour output to prevent double-styling
8. Added comprehensive unit tests for markdown rendering (9 new test functions)
9. All tests pass, lint is clean, build succeeds

### Code Review Fixes Applied

10. Moved Glamour from indirect to require section in go.mod (dependency tracking fix)
11. Enhanced `TestMarkdownRenderCodeBlock` to verify syntax highlighting via ANSI codes or structural changes (AC 3.1.4 validation)
12. Added `TestMarkdownRenderEmptyContent` test for empty content edge case

### File List

| File | Change Type |
|------|-------------|
| go.mod | Modified - added `github.com/charmbracelet/glamour v0.10.0` to require section |
| go.sum | Modified - added Glamour and transitive dependencies |
| internal/tui/styles.go | Modified - added MarkdownRenderer struct and methods |
| internal/tui/viewer.go | Modified - integrated markdown renderer into ViewerModel |
| internal/tui/styles_test.go | Modified - added markdown renderer unit tests (10 new test functions) |
