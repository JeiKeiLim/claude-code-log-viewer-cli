# Story 8.1: Fix Plain Mode First-Line Empty Spaces

Status: done

<!-- Validation: PASSED (2026-01-20) by SM Agent
     Issues addressed:
     - AC-2: Standardized on `cat -A` (consistent with verification method)
     - Test strategy: Added complete imports and second test function
     - Project structure: Confirmed test file exists (353 lines), added patterns to follow
     - Task 2 subtasks: Added specific test function names
     - Testing requirements: Added CI step
     - References: Added test file reference
-->

## Story

As a **cclv user viewing logs in plain mode**,
I want **clean output without spurious whitespace**,
So that **the output is properly formatted for terminals and pipes**.

## Acceptance Criteria

1. **AC-1: No Empty Space Lines After Header**
   - Given I run `cclv --plain file.jsonl`
   - When output is rendered
   - Then the header line `=== filename ===` is followed by a blank line (just `\n`)
   - And there are no lines filled with spaces

2. **AC-2: Consistent Spacing**
   - Given plain mode output
   - When viewed with `cat -A` (shows `$` for line endings)
   - Then lines contain only intended content (no trailing space padding)

## Tasks / Subtasks

- [x] Task 1: Fix header rendering in `RenderPlain()` (AC: #1, #2)
  - [x] Subtask 1.1: Move `\n\n` outside of `Styles.Title.Render()` call
  - [x] Subtask 1.2: Verify no trailing spaces are rendered

- [x] Task 2: Add tests for plain mode whitespace (AC: #1, #2)
  - [x] Subtask 2.1: Add `TestRenderPlain_NoSpuriousWhitespace` verifying no space-only lines
  - [x] Subtask 2.2: Add `TestRenderPlain_HeaderFollowedByBlankLine` verifying header format

- [x] Task 3: Verify fix across all plain mode output paths
  - [x] Subtask 3.1: Verify `RenderPlain()` output
  - [x] Subtask 3.2: Verify `RenderUsagePlain()` follows same pattern (already correct)

## Dev Notes

### Root Cause Analysis

The bug is in `internal/tui/plain.go:25-26`:

```go
header := fmt.Sprintf("=== %s ===\n\n", source)
b.WriteString(Styles.Title.Render(header))
```

When lipgloss `Render()` is called with a string containing newlines, it pads each line to the full width of the longest line. This causes the `\n\n` to become lines filled with spaces.

**Expected output:**
```
=== /path/to/file.jsonl ===
<blank line>
[U] User  12:34:56
```

**Actual output (bug):**
```
=== /path/to/file.jsonl ===
                             <-- line with 30 spaces
                             <-- line with 30 spaces
[U] User  12:34:56
```

### Fix

Move newlines outside the `Render()` call:

```go
// Before (buggy):
header := fmt.Sprintf("=== %s ===\n\n", source)
b.WriteString(Styles.Title.Render(header))

// After (fixed):
header := fmt.Sprintf("=== %s ===", source)
b.WriteString(Styles.Title.Render(header) + "\n\n")
```

### Verification Method

```bash
# To verify the fix, use cat -A which shows $ for line endings and spaces as spaces:
./bin/cclv --plain test.jsonl | cat -A | head -5

# Expected (fixed):
# === test.jsonl ===$
# $
# [U] User...

# Actual (buggy):
# === test.jsonl ===     $
#                        $
# [U] User...
```

### Similar Patterns to Audit

Review these locations for the same pattern:

1. `RenderUsagePlain()` at line 142 - Already correct: `Styles.Title.Render("Claude Code Usage") + "\n"`
2. Any other `Styles.*.Render()` calls with embedded newlines

### Test Strategy

Add table-driven tests to `internal/tui/plain_test.go` (file already exists with existing tests):

```go
import (
    "strings"
    "testing"
    "time"

    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestRenderPlain_NoSpuriousWhitespace(t *testing.T) {
    // Create minimal test entry
    entries := []types.LogEntry{
        {
            Type:      types.EntryTypeUser,
            Timestamp: time.Now(),
            Message: types.Message{
                TextContent: "Hello world",
            },
        },
    }

    result := RenderPlain(entries, "test.jsonl", RenderOptions{})

    // Check no line is entirely whitespace (except intended blank lines)
    lines := strings.Split(result, "\n")
    for i, line := range lines {
        if strings.TrimSpace(line) == "" && len(line) > 0 {
            t.Errorf("line %d contains only whitespace (%d chars)", i, len(line))
        }
    }
}

func TestRenderPlain_HeaderFollowedByBlankLine(t *testing.T) {
    entries := []types.LogEntry{
        {
            Type:      types.EntryTypeUser,
            Timestamp: time.Now(),
            Message: types.Message{
                TextContent: "Test message",
            },
        },
    }

    result := RenderPlain(entries, "test.jsonl", RenderOptions{})

    // Header should end with === followed by exactly two newlines (one blank line)
    // The pattern is: "=== test.jsonl ===\n\n[U]..."
    if !strings.Contains(result, "===\n\n") {
        t.Error("header should be followed by exactly one blank line (two newlines)")
    }
}
```

### Project Structure Notes

- **File to modify:** `internal/tui/plain.go` (line 25-26)
- **Test file:** `internal/tui/plain_test.go` (exists with 353 lines - add new tests to existing file)
- No new files required
- No new dependencies required

**Existing test patterns to follow:**
- `TestRenderPlainWithWidth` - tests width handling
- `TestRenderPlainWithHideOptions` - tests visibility options
- Follow the same table-driven pattern with `t.Run()` subtests

### Previous Story Learnings (from Story 7.7)

From Story 7.7:
- Keep changes minimal and focused
- Table-driven tests are required
- Use `make test` not raw `go test`
- 90%+ coverage required

From recent commits:
- `dad5cd4` added `RenderUsagePlain()` with correct pattern (newlines outside Render)
- The usage plain rendering already follows the correct pattern

### Git Intelligence

Recent commits show plain mode was touched in:
- `dad5cd4 feat: add plain text usage output (Story 7.6)` - Added `RenderUsagePlain()` with correct pattern

The fix should follow the same pattern used in `RenderUsagePlain()`.

### Architecture Compliance

- **Lipgloss styling:** Use `Styles.*` from `styles.go`
- **No emoji:** Text icons only (`[U]`, `[A]`, etc.)
- **Makefile:** Use `make build`, `make test`

### Files to Modify

| File | Change |
|------|--------|
| `internal/tui/plain.go` | Fix line 25-26: move `\n\n` outside `Render()` |
| `internal/tui/plain_test.go` | Add whitespace validation tests |

### Files to NOT Modify

- `internal/tui/styles.go` - No style changes needed
- Any other files - Single isolated fix

### Testing Requirements

1. **Unit test:** `TestRenderPlain_NoSpuriousWhitespace` - Verify no space-only lines in output
2. **Unit test:** `TestRenderPlain_HeaderFollowedByBlankLine` - Verify header followed by exactly `\n\n`
3. **Manual test:** Run `cclv --plain file.jsonl | cat -A` to verify visually
4. **CI:** Run `make test` to verify all existing tests still pass

### Anti-Patterns to Avoid

1. **DO NOT** remove the blank line entirely - it provides visual separation
2. **DO NOT** change `Styles.Title` definition - the style itself is correct
3. **DO NOT** add width parameters to `Render()` - unnecessary complexity

### Expected Commit Format

```
fix: remove spurious whitespace in plain mode header (Story 8.1)

Move newlines outside lipgloss Render() call to prevent
space padding on empty lines.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4-epic8.md#Story-8.1] - Requirements (FR-801)
- [Source: _bmad-output/project-context.md] - Critical rules
- [Source: internal/tui/plain.go:25-26] - Bug location
- [Source: internal/tui/plain.go:142] - Correct pattern example (RenderUsagePlain)
- [Source: internal/tui/plain_test.go] - Existing test patterns to follow

### Complexity Assessment

**Low complexity** - Single line change with clear fix. The pattern is already established in `RenderUsagePlain()`.

### Critical Rules (from project-context.md)

- NO EMOJI in any output
- Use `make test` not raw `go test`
- Table-driven tests required
- 90%+ test coverage required

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - straightforward fix with clear test results

### Completion Notes List

1. Fixed `RenderPlain()` at `internal/tui/plain.go:25-26` - moved `\n\n` outside `Styles.Title.Render()` call
2. Added `TestRenderPlain_NoSpuriousWhitespace` - verifies no space-only lines in output (AC-2)
3. Added `TestRenderPlain_HeaderFollowedByBlankLine` - verifies header followed by exactly one blank line (AC-1)
4. Verified `RenderUsagePlain()` already uses correct pattern at line 142
5. All tests pass with 94.1% coverage

### File List

| File | Change |
|------|--------|
| `internal/tui/plain.go` | Fixed line 25-26: moved `\n\n` outside `Render()` |
| `internal/tui/plain_test.go` | Added 2 whitespace validation tests |
