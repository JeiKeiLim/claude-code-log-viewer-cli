# Story 8.2: Collapsed Indicators for Visibility Flags

Status: done

## Story

As a **cclv user using `--hide-thoughts` or `--hide-tools`**,
I want **to see collapsed indicators instead of empty boxes**,
So that **I know content exists but is hidden (consistent with TUI toggle behavior)**.

## Acceptance Criteria

1. **AC-1: TUI `--hide-thoughts` Shows Collapsed Indicator**
   - Given I run `cclv --hide-thoughts file.jsonl` (TUI mode)
   - When an assistant message contains thinking blocks
   - Then thinking blocks display as: `[?] [thinking collapsed]`
   - And NOT as empty space or completely removed

2. **AC-2: TUI `--hide-tools` Shows Collapsed Summary**
   - Given I run `cclv --hide-tools file.jsonl` (TUI mode)
   - When an assistant message contains tool use blocks
   - Then tool blocks display as: `[>] Tool: Read (file_path: /path/to/file...)`
   - And uses the same `formatToolSummary()` as the `i` key toggle

3. **AC-3: Plain Mode `--hide-thoughts` Shows Collapsed Indicator**
   - Given I run `cclv --plain --hide-thoughts file.jsonl`
   - When an assistant message contains thinking blocks
   - Then thinking blocks display as collapsed indicator (not empty)

4. **AC-4: Plain Mode `--hide-tools` Shows Collapsed Summary**
   - Given I run `cclv --plain --hide-tools file.jsonl`
   - When an assistant message contains tool use blocks
   - Then tool blocks display as collapsed summary (not empty)

5. **AC-5: Consistent Behavior Between Flag and Toggle**
   - Given `--hide-thoughts` flag is used
   - When compared to pressing `t` key in TUI
   - Then the visual output is identical (both show collapsed indicator)
   - Same applies for `--hide-tools` vs `i` key

6. **AC-6: Message Box Not Empty**
   - Given an assistant message with ONLY thinking/tool content
   - When `--hide-thoughts --hide-tools` is used
   - Then the message box shows collapsed indicators (not empty header-only box)

## Tasks / Subtasks

- [x] Task 1: Fix TUI rendering for `--hide-thoughts` flag (AC: #1, #5)
  - [x] Subtask 1.1: In `viewer.go:renderAssistantMessage()` (line ~1427), replace `continue` with collapsed indicator
  - [x] Subtask 1.2: Use text `[?] [thinking collapsed]` to differentiate from toggle state
  - [x] Subtask 1.3: In `viewer.go:renderAssistantMessageStatic()` (line ~1585), apply same fix for async rendering

- [x] Task 2: Fix TUI rendering for `--hide-tools` flag (AC: #2, #5)
  - [x] Subtask 2.1: In `viewer.go:renderAssistantMessage()` (line ~1433), replace `continue` with collapsed summary
  - [x] Subtask 2.2: Reuse existing `formatToolSummary()` function (line ~1652)
  - [x] Subtask 2.3: In `viewer.go:renderAssistantMessageStatic()` (line ~1591), apply same fix for async rendering

- [x] Task 3: Fix plain mode rendering for both flags (AC: #3, #4)
  - [x] Subtask 3.1: In `plain.go:renderAssistantMessagePlain()` (line ~95), replace `continue` with collapsed indicators
  - [x] Subtask 3.2: Reuse `formatToolSummary()` for tool collapsed state
  - [x] Subtask 3.3: Use same collapsed text format as TUI for consistency

- [x] Task 4: Add tests for visibility flag collapsed indicators (AC: #1-6)
  - [x] Subtask 4.1: Add `TestRenderAssistantMessage_HideThoughtsFlag` in `viewer_test.go`
  - [x] Subtask 4.2: Add `TestRenderAssistantMessage_HideToolsFlag` in `viewer_test.go`
  - [x] Subtask 4.3: Update `TestRenderPlainWithHideOptions` in `plain_test.go` to verify collapsed indicators
  - [x] Subtask 4.4: Add `TestRenderPlain_BothFlagsShowsCollapsedIndicators` for AC-6

## Dev Notes

### Root Cause Analysis

**Current buggy behavior (viewer.go lines 1427-1437):**
```go
case types.ContentTypeThinking:
    if m.renderOpts.HideThoughts {
        continue // BUG: Completely removes block
    }
    parts = append(parts, m.renderThinkingBlock(content))

case types.ContentTypeToolUse:
    if m.renderOpts.HideTools {
        continue // BUG: Completely removes block
    }
    parts = append(parts, m.renderToolUseBlock(content))
```

The `continue` statement completely removes the block from output. This is inconsistent with the `t` and `i` toggle keys which show collapsed indicators.

**Reference: Correct toggle behavior (viewer.go:1445-1449):**
```go
func (m *ViewerModel) renderThinkingBlock(content types.MessageContent) string {
    if !m.showThinking {
        return Styles.CollapsedIndicator.Render(
            fmt.Sprintf("%s [thinking - press 't' to expand]", ThinkingIcon),
        )
    }
    // ... expanded rendering
}
```

### Required Changes

#### File 1: `internal/tui/viewer.go`

**Location 1: renderAssistantMessage() at lines 1427-1437**

Current code:
```go
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
```

Fix for thinking (replace `continue` at line ~1429):
```go
case types.ContentTypeThinking:
    if m.renderOpts.HideThoughts {
        // Show collapsed indicator (flag behavior - not expandable)
        parts = append(parts, Styles.CollapsedIndicator.Render(
            fmt.Sprintf("%s [thinking collapsed]", ThinkingIcon),
        ))
        continue
    }
    parts = append(parts, m.renderThinkingBlock(content))
```

Fix for tools (replace `continue` at line ~1435):
```go
case types.ContentTypeToolUse:
    if m.renderOpts.HideTools {
        // Show collapsed summary (same format as toggle state)
        header := fmt.Sprintf("%s %s: %s",
            ToolIcon,
            Styles.ToolHeader.Render("Tool"),
            content.ToolName,
        )
        summary := formatToolSummary(content.ToolName, content.ToolInput)
        parts = append(parts, Styles.ToolBlock.Render(
            header + " " + Styles.CollapsedIndicator.Render(summary),
        ))
        continue
    }
    parts = append(parts, m.renderToolUseBlock(content))
```

**Location 2: renderAssistantMessageStatic() at lines 1585-1596**

Current code:
```go
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
```

Apply the identical fix pattern for both cases.

#### File 2: `internal/tui/plain.go`

**Location: renderAssistantMessagePlain() at lines 95-117**

Current code:
```go
case types.ContentTypeThinking:
    if opts.HideThoughts {
        continue // Skip thinking blocks when hidden
    }
    // ... existing expanded rendering

case types.ContentTypeToolUse:
    if opts.HideTools {
        continue // Skip tool blocks when hidden
    }
    // ... existing expanded rendering
```

Fix for thinking (replace `continue` at line ~97):
```go
case types.ContentTypeThinking:
    if opts.HideThoughts {
        // Show collapsed indicator instead of skipping
        parts = append(parts, Styles.CollapsedIndicator.Render(
            fmt.Sprintf("%s [thinking collapsed]", ThinkingIcon),
        ))
        continue
    }
    // ... existing expanded rendering
```

Fix for tools (replace `continue` at line ~106):
```go
case types.ContentTypeToolUse:
    if opts.HideTools {
        // Show collapsed summary instead of skipping
        toolHeader := fmt.Sprintf("%s %s: %s",
            ToolIcon,
            Styles.ToolHeader.Render("Tool"),
            content.ToolName,
        )
        summary := formatToolSummary(content.ToolName, content.ToolInput)
        parts = append(parts, Styles.ToolBlock.Render(
            toolHeader + " " + Styles.CollapsedIndicator.Render(summary),
        ))
        continue
    }
    // ... existing expanded rendering
```

### Collapsed Text Differentiation

Use different text to differentiate flag state from toggle state:

| State | Thinking Block Text | Tool Block |
|-------|-------------------|------------|
| `--hide-thoughts` flag | `[T] [thinking collapsed]` | `[>] Tool: Name (summary)` |
| `t` toggle (collapsed) | `[T] [thinking - press 't' to expand]` | `[>] Tool: Name (summary)` |
| `--hide-tools` flag | N/A | `[>] Tool: Name (summary)` |
| `i` toggle (collapsed) | N/A | `[>] Tool: Name (summary)` |

Note: `ThinkingIcon = "[T]"` (not `[?]`) as defined in `styles.go`.

Note: Tool collapsed state is identical for flag and toggle since there's no key hint needed.

### Project Structure Notes

- **Files to modify:** `internal/tui/viewer.go`, `internal/tui/plain.go`
- **Test files:** `internal/tui/viewer_test.go`, `internal/tui/plain_test.go`
- **No new files required**
- **No new dependencies required**

**Existing patterns to follow:**
- `formatToolSummary()` at viewer.go:1652+ - reuse for tool collapsed state
- `Styles.CollapsedIndicator` for collapsed indicator styling
- `ThinkingIcon` = `[?]`, `ToolIcon` = `[>]` (from styles.go)

### Test Strategy

**Existing test to update - plain_test.go:102:**
The existing `TestRenderPlainWithHideOptions` already tests hide flags but verifies content is ABSENT. Update to verify collapsed indicators are PRESENT instead.

Current test expectations (wrong after fix):
```go
{"hide thoughts", true, false, false, true},  // expects Thinking to NOT appear
{"hide tools", false, true, true, false},     // expects Tool to NOT appear
```

After fix, collapsed indicators should appear:
```go
// Verify collapsed indicator appears instead of content
hasCollapsedThinking := strings.Contains(output, "[thinking collapsed]")
hasCollapsedTool := strings.Contains(output, "Read") && !strings.Contains(output, "file_path")
```

**New tests needed:**

**viewer_test.go:**
```go
func TestRenderAssistantMessage_HideThoughtsFlag(t *testing.T) {
    // Test that HideThoughts flag shows collapsed indicator
    // Use renderEntryStatic() for testable rendering
    entry := types.LogEntry{
        Type: types.EntryTypeAssistant,
        Message: types.Message{
            Content: []types.MessageContent{
                {Type: types.ContentTypeThinking, Thinking: "secret thinking content"},
            },
        },
    }

    opts := RenderOptions{HideThoughts: true}
    mdRenderer := NewMarkdownRenderer(80)
    output := renderAssistantMessageStatic(entry, 80, true, true, opts, mdRenderer, 0, nil)

    // Verify collapsed indicator present
    if !strings.Contains(output, "[thinking collapsed]") {
        t.Error("expected collapsed indicator for hidden thoughts")
    }
    // Verify actual content hidden
    if strings.Contains(output, "secret thinking content") {
        t.Error("hidden thinking content should not appear")
    }
}

func TestRenderAssistantMessage_HideToolsFlag(t *testing.T) {
    entry := types.LogEntry{
        Type: types.EntryTypeAssistant,
        Message: types.Message{
            Content: []types.MessageContent{
                {
                    Type:      types.ContentTypeToolUse,
                    ToolName:  "Read",
                    ToolInput: map[string]any{"file_path": "/test/file.go"},
                },
            },
        },
    }

    opts := RenderOptions{HideTools: true}
    mdRenderer := NewMarkdownRenderer(80)
    output := renderAssistantMessageStatic(entry, 80, true, true, opts, mdRenderer, 0, nil)

    // Verify tool header and summary present
    if !strings.Contains(output, "Tool") || !strings.Contains(output, "Read") {
        t.Error("expected tool header in collapsed output")
    }
    // Verify full input not shown (no multi-line JSON)
    if strings.Contains(output, "\"file_path\"") && strings.Contains(output, "\n") {
        t.Error("full tool input should not appear in collapsed state")
    }
}
```

**plain_test.go:**
```go
func TestRenderPlain_BothFlagsShowsCollapsedIndicators(t *testing.T) {
    // AC-6: Message with ONLY thinking/tool content should show indicators, not empty box
    entries := []types.LogEntry{
        {
            Type:      types.EntryTypeAssistant,
            Timestamp: time.Now(),
            Message: types.Message{
                Content: []types.MessageContent{
                    {Type: types.ContentTypeThinking, Thinking: "thinking content"},
                    {
                        Type:      types.ContentTypeToolUse,
                        ToolName:  "Bash",
                        ToolInput: map[string]any{"command": "ls"},
                    },
                },
            },
        },
    }

    opts := RenderOptions{
        Width:        80,
        HideThoughts: true,
        HideTools:    true,
    }
    output := RenderPlain(entries, "test.jsonl", opts)

    // Verify collapsed indicators present (not empty box)
    if !strings.Contains(output, "[thinking collapsed]") {
        t.Error("expected thinking collapsed indicator")
    }
    if !strings.Contains(output, "Bash") {
        t.Error("expected tool name in collapsed output")
    }
    // Verify actual content hidden
    if strings.Contains(output, "thinking content") {
        t.Error("thinking content should be hidden")
    }
    if strings.Contains(output, "\"command\"") {
        t.Error("tool input JSON should be hidden")
    }
}
```

### Previous Story Learnings (from Story 8.1)

From Story 8.1:
- Keep changes minimal and focused on the specific bug
- Table-driven tests are required
- Use `make test` not raw `go test`
- 90%+ coverage required
- Test both the positive case (collapsed shown) and negative case (full content hidden)

### Git Intelligence

Recent commits show:
- `aa4a7a1` Story 8.1 fixed similar lipgloss rendering issue in plain.go
- The pattern of moving content outside `Render()` calls is established
- Test patterns in `plain_test.go` can be followed

### Architecture Compliance

- **Lipgloss styling:** Use `Styles.*` from `styles.go`
- **NO EMOJI:** Text icons only (`[?]`, `[>]`) per FR-017
- **Makefile:** Use `make build`, `make test`
- **formatToolSummary():** Already exists - MUST reuse, DO NOT recreate

### Files to Modify

| File | Change |
|------|--------|
| `internal/tui/viewer.go` | Fix lines ~1427-1437 and ~1585-1596 for HideThoughts/HideTools |
| `internal/tui/plain.go` | Fix lines ~95-117 for HideThoughts/HideTools in plain mode |
| `internal/tui/viewer_test.go` | Add tests for visibility flag collapsed indicators |
| `internal/tui/plain_test.go` | Update existing test and add AC-6 test |

### Files to NOT Modify

- `internal/tui/styles.go` - Styles already exist
- `cmd/cclv/main.go` - Flag parsing already works
- `internal/types/` - No type changes needed

### Testing Requirements

1. **Unit test:** `TestRenderAssistantMessage_HideThoughtsFlag` - Verify collapsed indicator in TUI
2. **Unit test:** `TestRenderAssistantMessage_HideToolsFlag` - Verify collapsed summary in TUI
3. **Unit test:** Update `TestRenderPlainWithHideOptions` - Change expectations from "content absent" to "collapsed indicator present"
4. **Unit test:** `TestRenderPlain_BothFlagsShowsCollapsedIndicators` - AC-6: Message with only hidden content shows indicators
5. **Manual test:** Run `cclv --hide-thoughts file.jsonl` and verify collapsed indicator
6. **Manual test:** Run `cclv --plain --hide-tools file.jsonl` and verify collapsed summary
7. **CI:** Run `make test` to verify all existing tests still pass

### Anti-Patterns to Avoid

1. **DO NOT** create a new function for collapsed rendering - reuse existing patterns
2. **DO NOT** duplicate `formatToolSummary()` - import and use it
3. **DO NOT** change collapsed indicator styling - use `Styles.CollapsedIndicator`
4. **DO NOT** remove the existing toggle behavior - add flag behavior alongside
5. **DO NOT** make the collapsed text identical to toggle text - differentiate with "collapsed" vs "press 't' to expand"

### Expected Commit Format

```
feat: show collapsed indicators for visibility flags (Story 8.2)

--hide-thoughts and --hide-tools now show collapsed indicators
instead of empty output, consistent with toggle key behavior.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4-epic8.md#Story-8.2] - Requirements (FR-802)
- [Source: _bmad-output/project-context.md] - Critical rules
- [Source: internal/tui/viewer.go:1427-1437] - Bug location (renderAssistantMessage)
- [Source: internal/tui/viewer.go:1585-1596] - Bug location (renderAssistantMessageStatic)
- [Source: internal/tui/plain.go:95-117] - Bug location (renderAssistantMessagePlain)
- [Source: internal/tui/viewer.go:1652+] - formatToolSummary() to reuse
- [Source: internal/tui/viewer.go:1445-1449] - Correct collapsed indicator pattern
- [Source: internal/tui/plain_test.go:102] - Existing test to update (TestRenderPlainWithHideOptions)

### Complexity Assessment

**Medium complexity** - Changes to 4 locations in 2 files, plus 3 test updates/additions. The pattern is already established in the toggle behavior.

### Critical Rules (from project-context.md)

- NO EMOJI in any output - use `[?]` and `[>]` text icons
- Use `make test` not raw `go test`
- Table-driven tests required
- 90%+ test coverage required
- Reuse existing `formatToolSummary()` - DO NOT recreate

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

1. Fixed `viewer.go:renderAssistantMessage()` to show `[?] [thinking collapsed]` indicator when `--hide-thoughts` flag is used
2. Fixed `viewer.go:renderAssistantMessage()` to show tool collapsed summary when `--hide-tools` flag is used, reusing `formatToolSummary()`
3. Fixed `viewer.go:renderAssistantMessageStatic()` with identical changes for async rendering
4. Fixed `plain.go:renderAssistantMessagePlain()` to show collapsed indicators for both flags
5. Updated `TestRenderPlainWithOptions` in `viewer_test.go` to verify collapsed indicators
6. Updated `TestRenderPlainWithHideOptions` in `plain_test.go` to verify collapsed indicators
7. Added `TestRenderAssistantMessage_HideThoughtsFlag` for AC-1
8. Added `TestRenderAssistantMessage_HideToolsFlag` for AC-2
9. Added `TestRenderAssistantMessage_BothFlagsShowCollapsed` for AC-6
10. Added `TestRenderPlain_BothFlagsShowsCollapsedIndicators` for AC-6
11. Code review fixes: Added `TestRenderAssistantMessage_FlagVsToggleConsistency` for AC-5
12. Code review fixes: Renamed shadowed `header` variable to `toolHeader` in viewer.go
13. Code review fixes: Updated story Dev Notes to reflect actual `ThinkingIcon = "[T]"`

### File List

| File | Change |
|------|--------|
| `internal/tui/viewer.go` | Fixed HideThoughts/HideTools flags to show collapsed indicators (lines ~1428-1451, ~1590-1623) |
| `internal/tui/plain.go` | Fixed HideThoughts/HideTools flags to show collapsed indicators (lines ~95-130) |
| `internal/tui/viewer_test.go` | Updated test and added 3 new tests for visibility flag behavior |
| `internal/tui/plain_test.go` | Updated test and added 1 new test for visibility flag behavior |
