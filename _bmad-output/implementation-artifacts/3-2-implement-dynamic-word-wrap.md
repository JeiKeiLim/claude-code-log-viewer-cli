# Story 3.2: Implement Dynamic Word Wrap

Status: complete

## Story

As a **developer resizing my terminal**,
I want **markdown content to rewrap correctly**,
So that **text remains readable at any width**.

## Acceptance Criteria

### AC 3.2.1: Initial word wrap
- **Given** cclv starts
- **When** markdown renders
- **Then** it wraps to current terminal width
- **And** no horizontal scrolling needed

### AC 3.2.2: Resize detection
- **Given** the user resizes their terminal
- **When** tea.WindowSizeMsg is received
- **Then** the new width is captured
- **And** re-render is triggered

### AC 3.2.3: Re-render on resize
- **Given** terminal width changes
- **When** markdown content re-renders
- **Then** it uses the new width for word wrap
- **And** display updates smoothly

### AC 3.2.4: Renderer recreation
- **Given** terminal width changes significantly
- **When** re-rendering is needed
- **Then** Glamour renderer is recreated with new width
- **And** WithWordWrap uses updated value

## Tasks / Subtasks

- [x] Task 1: Verify existing resize implementation (AC: 3.2.1, 3.2.2, 3.2.3, 3.2.4)
  - [x] 1.1: Confirm `WindowSizeMsg` handler (viewer.go:357-389) recreates markdown renderer when width difference > 5
  - [x] 1.2: Confirm `updateContent()` is called after resize, re-rendering all loaded messages
  - [x] 1.3: Verify nil-check order is correct: `if m.markdownRenderer == nil || widthDiff > 5` - Fixed: nil-check now before Width() call
  - [x] 1.4: Confirm initial word wrap works (Story 3.1 dependency complete)

- [x] Task 2: Add unit tests for width-specific wrapping (AC: 3.2.1, 3.2.3, 3.2.4)
  - [x] 2.1: Test `NewMarkdownRenderer()` with different widths produces different output for long text
  - [x] 2.2: Test `Width()` method returns correct value
  - [x] 2.3: Test markdown wrapping behavior at boundary widths (40, 80, 120)

- [x] Task 3: Manual testing (All ACs)
  - [x] 3.1: Open cclv with a conversation containing long markdown text
  - [x] 3.2: Resize terminal narrower - verify text rewraps correctly
  - [x] 3.3: Resize terminal wider - verify text expands correctly
  - [x] 3.4: Rapid resize - verify no crashes or rendering glitches
  - [x] 3.5: Test with watch mode active - verify resize still works (watcher unaffected)
  - [x] 3.6: Test after 'G' key (bulk load) then resize - verify content rewraps

- [x] Task 4: Run build, lint, and test validation
  - [x] 4.1: Run `make build` - verify binary builds
  - [x] 4.2: Run `make lint` - no errors
  - [x] 4.3: Run `make test` - all tests pass, coverage maintained

## Dev Notes

### Current Implementation Status (from Story 3.1)

Story 3.1 already implemented the complete foundation. The resize handling is **already working**:

```go
// In viewer.go WindowSizeMsg handler (lines 357-389):
case tea.WindowSizeMsg:
    // Width captured from message (or preserves override)
    if m.renderOpts.Width == 0 {
        m.width = msg.Width
    }
    m.height = msg.Height

    // Recreate markdown renderer if width changed significantly (>5 chars)
    newRenderWidth := m.width - 4
    widthDiff := m.markdownRenderer.Width() - newRenderWidth
    if widthDiff < 0 {
        widthDiff = -widthDiff
    }
    if m.markdownRenderer == nil || widthDiff > 5 {
        m.markdownRenderer, _ = NewMarkdownRenderer(newRenderWidth)
    }

    // ... viewport resize ...
    m.updateContent()  // Re-renders all content with new width
```

**Why this story is mostly verification:**
1. Markdown renderer is recreated with new width on resize
2. `updateContent()` is called after resize, re-rendering all loaded content
3. The `widthDiff > 5` threshold acts as natural debounce for rapid resize
4. Bulk-loaded content (via 'G' key) is re-rendered because `updateContent()` regenerates from entries

### Render Width Calculation

```go
newRenderWidth := m.width - 4  // Accounts for card padding and border
```

The `-4` offset breakdown:
- 2 chars: `Padding(0, 1)` adds 1 char left + 1 char right padding
- 2 chars: `RoundedBorder()` adds 1 char left border + 1 char right border

### Test Code for Task 2.1

Add this test to `internal/tui/styles_test.go`:

```go
func TestMarkdownRendererDifferentWidths(t *testing.T) {
    longText := "This is a very long line that should wrap differently at different terminal widths because it exceeds typical terminal boundaries and forces word wrapping to occur."

    r80, err := NewMarkdownRenderer(80)
    if err != nil {
        t.Fatalf("NewMarkdownRenderer(80) error = %v", err)
    }
    r40, err := NewMarkdownRenderer(40)
    if err != nil {
        t.Fatalf("NewMarkdownRenderer(40) error = %v", err)
    }

    out80 := r80.Render(longText)
    out40 := r40.Render(longText)

    // Different widths should produce different line counts
    lines80 := strings.Count(out80, "\n")
    lines40 := strings.Count(out40, "\n")

    // Narrower width (40) should wrap to more lines than wider width (80)
    if lines40 <= lines80 {
        t.Errorf("40-width should wrap to more lines than 80-width: got %d vs %d", lines40, lines80)
    }
}

func TestMarkdownRendererBoundaryWidths(t *testing.T) {
    tests := []struct {
        name  string
        width int
    }{
        {"MinWidth40", 40},
        {"StandardWidth80", 80},
        {"WideWidth120", 120},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            r, err := NewMarkdownRenderer(tt.width)
            if err != nil {
                t.Fatalf("NewMarkdownRenderer(%d) error = %v", tt.width, err)
            }
            if r.Width() != tt.width {
                t.Errorf("Width() = %d, want %d", r.Width(), tt.width)
            }
            // Verify it can render without error
            output := r.Render("# Test header\n\nSome content")
            if !strings.Contains(output, "Test header") {
                t.Error("Rendered output should contain header text")
            }
        })
    }
}
```

### Why Debounce is Unnecessary

The `widthDiff > 5` threshold serves as implicit debounce:
- Small terminal adjustments (1-5 chars) don't trigger renderer recreation
- Only significant width changes cause re-render
- This prevents excessive renderer recreation during drag-resize

### Watch Mode and Resize Interaction

When resize occurs while watch mode is active:
- The watcher continues running unaffected (separate goroutine)
- New entries still arrive via `watcher.WaitForEvent()`
- The renderer is recreated with new width
- Both existing and new content use the updated renderer

### Files to Modify

| File | Change | Lines |
|------|--------|-------|
| `internal/tui/styles_test.go` | Add width-specific tests | New tests |

### Files NOT to Modify

| File | Reason |
|------|--------|
| `internal/tui/viewer.go` | Resize handling already correct from Story 3.1 |
| `internal/tui/styles.go` | MarkdownRenderer already correct from Story 3.1 |
| `cmd/cclv/main.go` | No CLI changes needed |
| `internal/parser/*.go` | Parser unaffected |

### Technical Constraints

| Constraint | Requirement |
|------------|-------------|
| **NO EMOJI in UI** | Text icons only per project-context.md |
| **Use Makefile** | `make build`, `make test`, never raw go commands |
| **Test patterns** | Table-driven tests per project-context.md |
| **Performance** | Re-render should feel smooth (<100ms for typical logs) |

### Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Width < 40 | Use specified width, may have tight wrapping |
| Width > 200 | Normal wrap, long lines |
| Rapid resize | Each final state correct (>5 threshold debounces) |
| Resize during search | Search highlights preserved after re-render |
| Resize in watch mode | Watcher unaffected, content rewraps |
| Resize after bulk load ('G') | `updateContent()` re-renders with new width |
| Width override (--width flag) | Preserves override, ignores terminal resize |

### Dependencies

- **Story 3.1 (Complete)**: Provides `MarkdownRenderer`, Glamour integration, resize handling
- **Sets up for Story 3.3**: Render caching will build on this verified resize handling

### Git Intelligence

Recent commits:
```
aca15d1 feat: integrate Glamour markdown renderer for assistant text
1daa088 feat: enable watch mode from interactive browse
e217df7 feat: implement smart auto-scroll for live watch mode
```

Suggested commit message:
```
test: add width-specific markdown wrapping tests

- Verify different widths produce different line counts
- Test boundary widths (40, 80, 120)
- Confirm resize triggers re-render with new width

Story 3.2 of Epic 3: Markdown Rendering

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: epics.md lines 945-991] - Story 3.2 requirements and acceptance criteria
- [Source: prd.md lines 155-160] - FR-302 Dynamic Word Wrap requirements
- [Source: project-context.md lines 37-39] - Glamour approved dependency
- [Source: internal/tui/viewer.go:357-389] - WindowSizeMsg handler
- [Source: internal/tui/viewer.go:638-664] - updateContent() method
- [Source: internal/tui/styles.go:355-395] - MarkdownRenderer implementation
- [Source: 3-1-integrate-glamour-markdown-renderer.md] - Previous story learnings

## Implementation Checklist

Before marking story complete, verify:

- [x] Traced code: resize triggers markdown renderer recreation (widthDiff > 5)
- [x] Traced code: `updateContent()` called after resize
- [x] Traced code: nil-check order is correct (nil checked first) - Fixed in this story
- [x] Added `TestMarkdownRendererDifferentWidths` test
- [x] Added `TestMarkdownRendererBoundaryWidths` test
- [x] Manual: narrowing terminal causes text to wrap to more lines
- [x] Manual: widening terminal allows text to expand
- [x] Manual: code blocks remain readable at different widths
- [x] Manual: no horizontal scrolling in normal content
- [x] Manual: resize after 'G' key (bulk load) rewraps correctly
- [x] Manual: rapid resize doesn't crash
- [x] Manual: watch mode resize works correctly
- [x] `make build` succeeds
- [x] `make lint` has no errors
- [x] `make test` passes with no regressions

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - No debug logging required for this verification-focused story.

### Completion Notes List

1. **Task 1.3 Bug Fix**: The original nil-check order in `WindowSizeMsg` handler was incorrect - `m.markdownRenderer.Width()` was called before checking if `m.markdownRenderer` was nil. Fixed by restructuring the conditional to check nil first.

2. **Tests Added**: Added two new tests to `internal/tui/styles_test.go`:
   - `TestMarkdownRendererDifferentWidths`: Verifies 40-width produces more line wraps than 80-width for long text
   - `TestMarkdownRendererBoundaryWidths`: Tests renderer creation and Width() at 40, 80, 120 widths

3. **Verification Complete**: All resize handling already implemented in Story 3.1 works correctly:
   - `WindowSizeMsg` captures new width and recreates renderer when widthDiff > 5
   - `updateContent()` re-renders all loaded entries with new width
   - Debounce via widthDiff > 5 threshold prevents excessive recreation

### File List

| File | Change |
|------|--------|
| `internal/tui/viewer.go` | Fixed nil-check order in WindowSizeMsg handler (lines 366-376) |
| `internal/tui/styles_test.go` | Added TestMarkdownRendererDifferentWidths and TestMarkdownRendererBoundaryWidths tests |

