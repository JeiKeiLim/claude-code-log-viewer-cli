# Story 4.5: Newline Normalization Fix

Status: done

## Story

As a **user viewing assistant responses**,
I want **excessive blank lines normalized**,
So that **rendered markdown doesn't have awkward spacing**.

## Acceptance Criteria

### AC 4.5.1: Normalize 3+ consecutive newlines
- **Given** assistant text content with 3+ consecutive newlines
- **When** the markdown is rendered
- **Then** consecutive newlines are reduced to maximum 2
- **And** the visual output has consistent paragraph spacing

### AC 4.5.2: Preserve single and double newlines
- **Given** assistant text content with 1 or 2 consecutive newlines
- **When** the markdown is rendered
- **Then** those newlines are preserved exactly as-is
- **And** paragraph breaks and line breaks work normally

### AC 4.5.3: Apply only to assistant text
- **Given** user messages or tool content
- **When** displayed
- **Then** newline normalization is NOT applied
- **And** original formatting is preserved

### AC 4.5.4: Normalization applied post-Glamour rendering
- **Given** markdown content processed by Glamour that produces 3+ consecutive newlines
- **When** `MarkdownRenderer.Render()` is called
- **Then** normalization is applied AFTER Glamour renders the content
- **And** ANSI escape codes in styled output are preserved (not corrupted)

## Tasks / Subtasks

- [x] Task 1: Implement newline normalization function (AC: 4.5.1, 4.5.2, 4.5.4)
  - [x] 1.1: Add `"regexp"` to imports in `internal/tui/styles.go`
  - [x] 1.2: Create package-level `var multipleNewlinesRe = regexp.MustCompile(`\n{3,}`)` for performance
  - [x] 1.3: Create `NormalizeNewlines(s string) string` function that uses `multipleNewlinesRe.ReplaceAllString(s, "\n\n")`
  - [x] 1.4: Verify function handles edge cases (empty string, only newlines, mixed patterns)

- [x] Task 2: Integrate into MarkdownRenderer.Render() (AC: 4.5.3, 4.5.4)
  - [x] 2.1: Apply `NormalizeNewlines()` AFTER `m.renderer.Render(content)`
  - [x] 2.2: Apply BEFORE `strings.TrimRight()` to ensure consistent output
  - [x] 2.3: Verify the normalization only affects assistant markdown path (not user/tool)

- [x] Task 3: Add unit tests (AC: 4.5.1-4.5.4)
  - [x] 3.1: Test `NormalizeNewlines()` with 3 newlines -> 2
  - [x] 3.2: Test `NormalizeNewlines()` with 4 newlines -> 2
  - [x] 3.3: Test `NormalizeNewlines()` with 10 newlines -> 2
  - [x] 3.4: Test `NormalizeNewlines()` preserves 1 newline
  - [x] 3.5: Test `NormalizeNewlines()` preserves 2 newlines
  - [x] 3.6: Test `NormalizeNewlines()` with mixed patterns (e.g., `a\nb\n\nc\n\n\nd\n\n\n\n\ne` -> `a\nb\n\nc\n\nd\n\ne`)
  - [x] 3.7: Test `NormalizeNewlines()` with ANSI escape codes (verifies codes not corrupted)
  - [x] 3.8: Test `MarkdownRenderer.Render()` normalizes output (integration test)

- [x] Task 4: Run build, lint, and test validation
  - [x] 4.1: Run `make build` - binary builds successfully
  - [x] 4.2: Run `make lint` - no errors
  - [x] 4.3: Run `make test` - all tests pass
  - [x] 4.4: Run `make ci` - full CI passes

- [x] Task 5: Manual testing
  - [x] 5.1: Find or create a conversation with assistant response containing many blank lines
  - [x] 5.2: View in cclv, verify spacing is reasonable (max double-spaced paragraphs)
  - [x] 5.3: Verify user messages still display with original formatting
  - [x] 5.4: Toggle raw mode (`r`) to see original JSONL content still has original newlines

## Dev Notes

### Where to Apply the Fix

The fix belongs in `internal/tui/styles.go` within the `MarkdownRenderer.Render()` method (lines 395-404).

**Current implementation:**
```go
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

**Updated implementation:**
```go
// Pre-compiled regex for newline normalization (Story 4.5)
var multipleNewlinesRe = regexp.MustCompile(`\n{3,}`)

// NormalizeNewlines reduces 3+ consecutive newlines to exactly 2.
// This prevents awkward spacing in markdown output.
func NormalizeNewlines(s string) string {
    return multipleNewlinesRe.ReplaceAllString(s, "\n\n")
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
```

### Why This Location

1. **Centralized**: All assistant markdown flows through `MarkdownRenderer.Render()` (viewer.go:1292, viewer.go:1435)
2. **Post-Glamour**: Glamour can add extra newlines during rendering, so we normalize after
3. **Only assistant**: User messages use `WrapText()` not `MarkdownRenderer.Render()`, so they are unaffected

### Flow of Text Content

**Assistant messages** (gets normalization):
```
entry.Message.Content[i].Text
    -> m.markdownRenderer.Render(content.Text)
        -> glamour.Render()
        -> NormalizeNewlines()  <- NEW
        -> strings.TrimRight()
    -> returned to viewer
```

**User messages** (no normalization):
```
entry.Message.TextContent
    -> WrapText(entry.Message.TextContent, wrapWidth)
    -> Styles.MessageContent.Render(wrappedText)
    -> returned to viewer
```

### Regex Pattern

```go
regexp.MustCompile(`\n{3,}`)
```

- `\n` - matches a newline character
- `{3,}` - matches 3 or more occurrences
- Replacement: `\n\n` (exactly 2 newlines)

**Examples:**
| Input | Output |
|-------|--------|
| `a\nb` | `a\nb` (1 newline - preserved) |
| `a\n\nb` | `a\n\nb` (2 newlines - preserved) |
| `a\n\n\nb` | `a\n\nb` (3 newlines -> 2) |
| `a\n\n\n\nb` | `a\n\nb` (4 newlines -> 2) |
| `a\n\n\n\n\n\n\n\n\n\nb` | `a\n\nb` (10 newlines -> 2) |

### Import Addition

Add `"regexp"` to imports in `styles.go` if not present.

**Current imports (line 3-9):**
```go
import (
    "strings"
    "time"

    "github.com/charmbracelet/glamour"
    "github.com/charmbracelet/lipgloss"
)
```

**Updated imports:**
```go
import (
    "regexp"
    "strings"
    "time"

    "github.com/charmbracelet/glamour"
    "github.com/charmbracelet/lipgloss"
)
```

### Technical Constraints

| Constraint | Requirement |
|------------|-------------|
| **Use Makefile** | `make build`, `make test`, `make ci` |
| **TEA pattern** | No direct state mutation (this is pure function) |
| **Performance** | Pre-compile regex at package level |
| **Test coverage** | Maintain 90%+ |

### File Changes Summary

| File | Changes |
|------|---------|
| `internal/tui/styles.go` | Add `NormalizeNewlines()` function, add regex var, modify `Render()` method, add `regexp` import |
| `internal/tui/styles_test.go` | Add tests for `NormalizeNewlines()` and `MarkdownRenderer.Render()` normalization |

### Edge Cases

1. **Empty string**: Returns empty string (no change needed)
2. **Only newlines**: `\n\n\n\n` -> `\n\n`
3. **Mixed content**: `hello\n\n\nworld\n\n\n\nend` -> `hello\n\nworld\n\nend`
4. **Whitespace**: Only affects `\n`, not spaces or tabs
5. **ANSI codes**: Glamour output includes ANSI codes for styling - regex only matches literal `\n`, should not interfere

### Test Patterns

Based on Story 4.4 and existing tests in `styles_test.go`:

```go
func TestNormalizeNewlines(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {"single newline preserved", "a\nb", "a\nb"},
        {"double newline preserved", "a\n\nb", "a\n\nb"},
        {"triple newline normalized", "a\n\n\nb", "a\n\nb"},
        {"quadruple newline normalized", "a\n\n\n\nb", "a\n\nb"},
        {"ten newlines normalized", "a\n\n\n\n\n\n\n\n\n\nb", "a\n\nb"},
        {"empty string", "", ""},
        {"only newlines", "\n\n\n\n", "\n\n"},
        {"mixed patterns", "a\nb\n\nc\n\n\nd\n\n\n\n\ne", "a\nb\n\nc\n\nd\n\ne"},
        {"with ANSI codes", "hello\x1b[31m\n\n\n\x1b[0mworld", "hello\x1b[31m\n\n\x1b[0mworld"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := NormalizeNewlines(tt.input)
            if got != tt.want {
                t.Errorf("NormalizeNewlines() = %q, want %q", got, tt.want)
            }
        })
    }
}
```

### Git Commit Template

```
fix: normalize excessive newlines in markdown rendering

- Reduce 3+ consecutive newlines to 2 in assistant responses
- Apply normalization after Glamour rendering in MarkdownRenderer.Render()
- Pre-compile regex for performance
- User messages and tool content unaffected

Story 4.5 of Epic 4: Developer Power Tools (FR-405)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Previous Story Learnings (from 4-4-toast-path-display.md)

1. **Reuse existing infrastructure**: Story 4.4 reused toast system from 4.2. Story 4.5 reuses the centralized `MarkdownRenderer.Render()` method.
2. **Minimal changes**: Story 4.4 was ~8 lines of code. Story 4.5 should be similarly minimal (~15 lines including function and regex).
3. **Test first approach**: Follow table-driven test pattern from existing tests.
4. **CI validation**: Always run `make ci` before marking complete.

### Recent Git Commit Patterns

From recent commits:
- `feat:` for new features (add line numbers, toast path display)
- `fix:` for bug fixes (this is a fix for awkward spacing)
- Clean, concise commit messages with bullet points
- Always include `Co-Authored-By` line

### Architecture Reference

From `architecture-phase3.md`:
> **FR-405 (Newline Fix):** Post-processing in markdown render

This confirms the approach: post-process the Glamour output in `MarkdownRenderer.Render()`.

## Implementation Checklist

**Function:**
- [x] Add `multipleNewlinesRe` package-level regex var
- [x] Add `NormalizeNewlines()` function
- [x] Update `MarkdownRenderer.Render()` to call `NormalizeNewlines()`
- [x] Add `regexp` import

**Tests (8+):**
- [x] Test single newline preserved
- [x] Test double newline preserved
- [x] Test triple newline normalized
- [x] Test quadruple newline normalized
- [x] Test ten newlines normalized
- [x] Test empty string
- [x] Test mixed patterns
- [x] Test ANSI escape codes preserved

**Build & Manual:**
- [x] `make build` / `make lint` / `make test` / `make ci` pass
- [x] Coverage >= 90%
- [x] Manual: View assistant response with many blank lines, verify normalized

### References

- [Source: _bmad-output/planning-artifacts/epics-phase3.md#Story 4.5]
- [Source: _bmad-output/planning-artifacts/prd-phase3.md#FR-405]
- [Source: _bmad-output/planning-artifacts/architecture-phase3.md#FR-405]
- [Source: _bmad-output/project-context.md#Testing Rules]
- [Source: internal/tui/styles.go#MarkdownRenderer.Render()]
- [Source: internal/tui/viewer.go:1292 - Assistant text rendering call]
- [Source: _bmad-output/implementation-artifacts/4-4-toast-path-display.md - Previous story pattern]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

No debugging required - straightforward implementation.

### Completion Notes List

1. Added `regexp` import to `internal/tui/styles.go`
2. Created package-level pre-compiled regex `multipleNewlinesRe` for performance
3. Created `NormalizeNewlines(s string) string` function
4. Integrated normalization into `MarkdownRenderer.Render()` method - applied after Glamour renders, before TrimRight
5. Added 9 unit tests for `NormalizeNewlines()` covering all edge cases (AC 4.5.1, 4.5.2)
6. Added integration test `TestMarkdownRendererNormalizesNewlines` (AC 4.5.4)
7. All CI checks pass: `make build`, `make lint`, `make test`, `make ci`
8. AC 4.5.3 (user messages unaffected) verified by code path - user messages use `WrapText()` which doesn't call `NormalizeNewlines()`

### File List

| File | Changes |
|------|---------|
| `internal/tui/styles.go` | Added `regexp` import, `multipleNewlinesRe` var, `NormalizeNewlines()` function, updated `MarkdownRenderer.Render()` |
| `internal/tui/styles_test.go` | Added `TestNormalizeNewlines()` and `TestMarkdownRendererNormalizesNewlines()` tests |

