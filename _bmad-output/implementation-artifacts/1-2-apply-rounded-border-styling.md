# Story 1.2: Apply Rounded Border Styling

Status: done

## Story

As a **developer viewing conversation logs**,
I want **message cards to have rounded borders**,
So that **the UI feels modern and polished**.

## Acceptance Criteria

### AC 1.2.1: User message styling
- **Given** a log entry with type "human"
- **When** it renders in the viewport
- **Then** it displays with RoundedBorder() style
- **And** uses the Theme's user message colors

### AC 1.2.2: Assistant message styling
- **Given** a log entry with type "assistant"
- **When** it renders in the viewport
- **Then** it displays with RoundedBorder() style
- **And** uses the Theme's assistant message colors

### AC 1.2.3: Tool call styling
- **Given** a log entry with type "tool_use" or "tool_result"
- **When** it renders in the viewport
- **Then** it displays with RoundedBorder() style
- **And** uses the Theme's tool colors (muted/secondary)

### AC 1.2.4: Border characters
- **Given** any bordered message card
- **When** rendered in terminal
- **Then** uses Unicode rounded corners: ╭ ╮ ╰ ╯

## Tasks / Subtasks

- [x] Task 1: Update all message styles to use RoundedBorder (AC: 1.2.1, 1.2.2, 1.2.3)
  - [x] 1.1: UserMessage - Replace `BorderLeft(true).BorderStyle(lipgloss.NormalBorder())` with `Border(lipgloss.RoundedBorder())`
  - [x] 1.2: AssistantMessage - Same transformation as UserMessage
  - [x] 1.3: ThinkingBlock - Same transformation, preserve `Foreground(mutedColor)`
  - [x] 1.4: ToolBlock - Same transformation

- [x] Task 2: Adjust padding for card appearance (AC: 1.2.1, 1.2.2, 1.2.3)
  - [x] 2.1: Change `PaddingLeft(1)` to `Padding(0, 1)` for symmetrical horizontal padding
  - [x] 2.2: Preserve existing MarginBottom for UserMessage and AssistantMessage (MarginBottom(1))
  - [x] 2.3: ThinkingBlock and ToolBlock - NO MarginBottom (preserve current spacing)

- [x] Task 3: Verify Unicode rounded corners render correctly (AC: 1.2.4)
  - [x] 3.1: Run `make build` and test in terminal
  - [x] 3.2: Verify ╭ ╮ ╰ ╯ characters appear at corners
  - [x] 3.3: Test on both light and dark terminal themes

- [x] Task 4: Run tests and ensure build passes
  - [x] 4.1: Run `make test` - all tests must pass
  - [x] 4.2: Run `make build` - must succeed
  - [x] 4.3: Verify no regressions in existing styles

## Dev Notes

### Architecture Compliance

**CRITICAL**: Follow project-context.md rules exactly.

1. **File Location**: All changes in `internal/tui/styles.go` only
2. **No new files**: Modify existing Styles struct only
3. **No emoji**: Text icons `[U]`, `[A]`, `[T]`, `[>]` per FR-017
4. **Build with Make**: `make build` and `make test` - never raw go commands
5. **Box-drawing characters**: `╭`, `╮`, `╰`, `╯`, `│`, `─`

### Technical Implementation

**Current (lines 121-146):**
```go
UserMessage: lipgloss.NewStyle().
    BorderLeft(true).
    BorderStyle(lipgloss.NormalBorder()).
    BorderForeground(userColor).
    PaddingLeft(1).
    MarginBottom(1),
```

**Target:**
```go
UserMessage: lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(userColor).
    Padding(0, 1).
    MarginBottom(1),
```

**Existing RoundedBorder reference:** `SearchInput` style (line 211) already uses `lipgloss.RoundedBorder()` - use as working pattern.

### Style Migration Summary

| Style | Border Change | Padding Change | Margin |
|-------|--------------|----------------|--------|
| UserMessage | BorderLeft → Border(RoundedBorder) | PaddingLeft(1) → Padding(0, 1) | Keep MarginBottom(1) |
| AssistantMessage | BorderLeft → Border(RoundedBorder) | PaddingLeft(1) → Padding(0, 1) | Keep MarginBottom(1) |
| ThinkingBlock | BorderLeft → Border(RoundedBorder) | PaddingLeft(1) → Padding(0, 1) | NO MarginBottom (preserve) |
| ToolBlock | BorderLeft → Border(RoundedBorder) | PaddingLeft(1) → Padding(0, 1) | NO MarginBottom (preserve) |

### Width Handling

**IMPORTANT**: The `viewer.go` applies styles in `renderEntry()` with available width = `m.width - 2` (viewport padding). The bordered cards will naturally fit within this width. Do NOT add explicit `.Width()` constraints unless overflow issues appear during testing.

### Previous Story Intelligence (1.1)

**Key learnings from Story 1.1:**
1. All color variables (`userColor`, `assistantColor`, `thinkingColor`, `toolColor`) are already adaptive via DefaultTheme
2. No changes needed to viewer.go - styles are applied via global Styles struct
3. Existing tests in `styles_test.go` verify Theme completeness - no new tests needed for style changes

**Recent commit:** `7f4e11e feat: implement adaptive color system for light/dark terminals`

### Files to Modify

| File | Changes |
|------|---------|
| `internal/tui/styles.go` | Lines 121-146: Update UserMessage, AssistantMessage, ThinkingBlock, ToolBlock |

### Files NOT to Modify

- `viewer.go` - No changes needed, styles applied via Styles struct
- `app.go`, `project.go`, `conversation.go` - No changes needed
- `styles_test.go` - Existing tests cover Theme; no new logic added

### Common Pitfalls

1. **DON'T** mix `BorderLeft(true)` with `Border()` - they conflict
2. **DON'T** change color references - already correct from Story 1.1
3. **DON'T** add MarginBottom to ThinkingBlock/ToolBlock - preserve current spacing
4. **DON'T** modify any file except `internal/tui/styles.go`

### Testing Strategy

1. **Unit Tests**: `make test` - existing tests pass (no logic changes)
2. **Manual Testing**: Essential - verify rounded corners visually
3. **Verification**:
   - Load conversation with user, assistant, thinking, tool messages
   - Verify all four message types have rounded borders
   - Verify ╭ ╮ ╰ ╯ corners render correctly
   - Test on light and dark terminal themes

### References

- [internal/tui/styles.go:121-146] - Current message styles
- [internal/tui/styles.go:211] - SearchInput RoundedBorder example
- [_bmad-output/project-context.md#Styling Rules] - Box-drawing characters
- [_bmad-output/planning-artifacts/epics.md#Story 1.2] - Acceptance criteria
- [_bmad-output/planning-artifacts/prd.md#FR-102] - Rounded Border requirements

## Dev Agent Record

### Agent Model Used
Claude Opus 4.5

### Debug Log References
- `make test` - All 57 tests passed
- `make build` - Successful compilation

### Completion Notes List
- All four message styles (UserMessage, AssistantMessage, ThinkingBlock, ToolBlock) updated to use `Border(lipgloss.RoundedBorder())`
- Padding changed from `PaddingLeft(1)` to `Padding(0, 1)` for symmetrical horizontal padding
- MarginBottom preserved for UserMessage and AssistantMessage only
- ThinkingBlock Foreground(mutedColor) preserved as required
- No new files created - all changes in `internal/tui/styles.go` only
- lipgloss.RoundedBorder() uses Unicode rounded corners: ╭ ╮ ╰ ╯

### File List
- `internal/tui/styles.go` (modified: UserMessage, AssistantMessage, ThinkingBlock, ToolBlock, SearchInput)
