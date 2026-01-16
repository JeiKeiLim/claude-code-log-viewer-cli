# Story 1.7: Adjust Spacing and Margins

Status: done

## Story

As a **developer viewing conversation logs**,
I want **tighter spacing between message cards**,
So that **screen space is used efficiently while maintaining readability**.

## Acceptance Criteria

### AC 1.7.1: Reduced message margins
- **Given** the viewer with message cards
- **When** messages render
- **Then** MarginBottom is reduced from 1 to 0
- **And** rounded borders provide sufficient visual separation

### AC 1.7.2: Visual separation maintained
- **Given** reduced margins
- **When** viewing consecutive messages
- **Then** each message is still clearly distinct
- **And** readability is not compromised

### AC 1.7.3: Consistent spacing
- **Given** all message types (User, Assistant, Thinking, Tool)
- **When** they render
- **Then** spacing is consistent across all types
- **And** the layout feels balanced

## Tasks / Subtasks

- [x] Task 1: Remove MarginBottom from message styles (AC: 1.7.1)
  - [x] 1.1: In styles.go, change `UserMessage` style (lines 141-145): remove `.MarginBottom(1)` chain
  - [x] 1.2: In styles.go, change `AssistantMessage` style (lines 147-151): remove `.MarginBottom(1)` chain
  - [x] 1.3: Verify `ThinkingBlock` (lines 153-157) and `ToolBlock` (lines 159-162) have no MarginBottom (confirmed - no change needed)

- [x] Task 2: Verify viewer.go separator behavior (AC: 1.7.2)
  - [x] 2.1: Confirm `updateContent()` adds `\n` between entries (line 408) - provides single newline separation
  - [x] 2.2: The `\n` after each entry provides minimal separation; rounded borders provide visual distinction

- [x] Task 3: Visual verification (AC: 1.7.2, 1.7.3)
  - [x] 3.1: Build and run: `make build && ./bin/cclv`
  - [x] 3.2: Test with real log file containing multiple message types
  - [x] 3.3: Verify User messages are visually distinct from each other
  - [x] 3.4: Verify Assistant messages are visually distinct from each other
  - [x] 3.5: Verify mixed message types (User → Assistant → User) are clearly separated
  - [x] 3.6: Verify Thinking blocks (when expanded with 't') don't merge with surrounding content
  - [x] 3.7: Verify Tool blocks (when expanded with 'i') don't merge with surrounding content

- [x] Task 4: Run tests (all ACs)
  - [x] 4.1: Run `make test` - all tests should pass
  - [x] 4.2: Run `make lint` - no lint errors

## Dev Notes

### Implementation Pattern: Simple Style Modification

This is a minimal change - remove two `.MarginBottom(1)` calls from styles.go.

**Current code (styles.go lines 141-151 - verified 2026-01-16):**
```go
UserMessage: lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(userColor).
    Padding(0, 1).
    MarginBottom(1),  // REMOVE THIS LINE

AssistantMessage: lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(assistantColor).
    Padding(0, 1).
    MarginBottom(1),  // REMOVE THIS LINE
```

**Target code:**
```go
UserMessage: lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(userColor).
    Padding(0, 1),

AssistantMessage: lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(assistantColor).
    Padding(0, 1),
```

### Why This Works

1. **Rounded borders provide separation**: The `RoundedBorder()` creates a complete visual box around each message (╭╮╰╯), clearly delineating boundaries.

2. **viewer.go adds newline separator**: In `updateContent()` (line 408), each entry is followed by `content.WriteString("\n")`, which provides a single blank line between entries.

3. **The MarginBottom(1) is redundant**: With both the border AND the newline, adding MarginBottom(1) creates excessive whitespace (2 lines of separation). Removing it leaves exactly 1 line from the `\n`.

### Visual Before/After

**Before (MarginBottom=1):**
```
╭─[U] User──────────╮
│ Hello world       │
╰───────────────────╯
                      <- blank line from \n
                      <- blank line from MarginBottom(1)
╭─[A] Assistant─────╮
│ Hi there          │
╰───────────────────╯
```

**After (MarginBottom=0):**
```
╭─[U] User──────────╮
│ Hello world       │
╰───────────────────╯
                      <- blank line from \n (only)
╭─[A] Assistant─────╮
│ Hi there          │
╰───────────────────╯
```

### Files to Modify

| File | Change |
|------|--------|
| `internal/tui/styles.go` | Remove `.MarginBottom(1)` from UserMessage and AssistantMessage |

**No other files need modification.** The viewer.go logic remains unchanged.

### Testing Approach

No new tests needed - this is a visual-only change. Existing tests verify style initialization. Manual visual verification is sufficient:

1. `make build`
2. `./bin/cclv ~/.claude/projects/*/conversations/*.jsonl` (or any log file)
3. Confirm messages are visually distinct with tighter spacing

### Project Structure Notes

- **Single file change**: `internal/tui/styles.go` only
- **Lines affected**: 2 (remove `.MarginBottom(1)` from lines 145 and 151)
- **No new dependencies**
- **No architectural changes**
- **Follows existing patterns**: Uses lipgloss method chaining

### Build Commands

```bash
make build  # Build binary
make test   # Run tests
make lint   # Run linter
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.7] - Story requirements and ACs (lines 358-390)
- [Source: internal/tui/styles.go:141-162] - Current message styles with MarginBottom (verified)
- [Source: internal/tui/viewer.go:405-409] - updateContent() adds \n separator (verified line 408)
- [Source: _bmad-output/project-context.md] - Project rules and conventions

### Previous Story Intelligence (Story 1.6)

From Story 1.6 completion:
- All tests pass with `make test`
- Build successful with `make build`
- Gutter selection pattern works correctly
- No style-related regressions

### Git Intelligence

Recent commits:
- `23c12f9` - feat: implement list view polish with gutter selection pattern
- `13f4cd9` - feat: add spinner animation during loading operations
- `5f80be1` - feat: implement segmented status bar with position tracking
- `6971e73` - feat: apply rounded border styling to message cards

Pattern: Use `feat:` prefix for feature commits. Commit message style: lowercase, imperative, descriptive.

### Risk Assessment

**Risk: LOW**

- Simple 2-line change
- No logic changes
- No new dependencies
- Easy to revert if spacing feels too tight

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - no debug issues encountered

### Completion Notes List

1. Removed `.MarginBottom(1)` from `UserMessage` style (styles.go:145)
2. Removed `.MarginBottom(1)` from `AssistantMessage` style (styles.go:151)
3. Verified `ThinkingBlock` and `ToolBlock` already have no MarginBottom - no changes needed
4. Verified `viewer.go:408` adds `\n` separator between entries - provides single newline separation
5. All tests pass (`make test`)
6. No lint errors (`make lint`)
7. Build successful (`make build`)

### File List

- `internal/tui/styles.go` - Removed MarginBottom(1) from UserMessage and AssistantMessage styles
