# Story 1.9: Pipeline Visibility Flags with Richer Collapsed Info

Status: done

## Story

As a **developer integrating cclv with other tools**,
I want **flags to control thought/tool visibility and richer collapsed tool summaries**,
So that **I can customize output for pipelines and quickly understand what tools did**.

## Acceptance Criteria

### AC 1.9.1: --hide-thoughts flag
- **Given** the cclv CLI in pipeline mode
- **When** I run `cclv --hide-thoughts <file>`
- **Then** thinking blocks are not rendered in output
- **And** other content types render normally

### AC 1.9.2: --hide-tools flag
- **Given** the cclv CLI in pipeline mode
- **When** I run `cclv --hide-tools <file>`
- **Then** tool use/result blocks are not rendered in output
- **And** other content types render normally

### AC 1.9.3: Flags combinable
- **Given** the cclv CLI
- **When** I run `cclv --hide-thoughts --hide-tools <file>`
- **Then** both thoughts and tools are hidden
- **And** only user/assistant messages render

### AC 1.9.4: Richer collapsed tool info
- **Given** a collapsed tool block (default state)
- **When** it renders
- **Then** it shows a brief summary like `Read: viewer.go (lines 1-50)` or `Edit: styles.go (+15/-3 lines)`
- **And** the summary provides actionable context without expanding

## Tasks / Subtasks

- [x] Task 1: Add CLI flags for visibility control (AC: 1.9.1, 1.9.2, 1.9.3)
  - [x] 1.1: Add `--hide-thoughts` bool flag to main.go (line ~36) with description "Hide thinking blocks in output"
  - [x] 1.2: Add `--hide-tools` bool flag to main.go (line ~37) with description "Hide tool use blocks in output"
  - [x] 1.3: Create RenderOptions in main.go from flag values before calling tui functions
  - [x] 1.4: Pass opts to `tui.NewViewerModel()` call at line 128
  - [x] 1.5: Pass opts to `tui.RenderPlain()` call at line 122

- [x] Task 2: Create RenderOptions struct and update signatures (AC: 1.9.1, 1.9.2, 1.9.3)
  - [x] 2.1: Add RenderOptions struct in viewer.go after imports (HideThoughts, HideTools bool)
  - [x] 2.2: Add DefaultRenderOptions() function returning show-all defaults
  - [x] 2.3: Update NewViewerModel signature: add `opts RenderOptions` as 4th parameter
  - [x] 2.4: Update NewViewerModelWithBack to pass opts through
  - [x] 2.5: Update NewViewerModelWithBackNavigation to pass opts through
  - [x] 2.6: Store renderOpts in ViewerModel struct
  - [x] 2.7: Update RenderPlain signature in plain.go: add `opts RenderOptions` as 3rd parameter
  - [x] 2.8: Update all callers (see Caller List section)

- [x] Task 3: Implement --hide-thoughts flag behavior (AC: 1.9.1)
  - [x] 3.1: In viewer.go renderAssistantMessage (line ~543), skip ContentTypeThinking if m.renderOpts.HideThoughts
  - [x] 3.2: In plain.go renderAssistantMessagePlain (line ~68), skip ContentTypeThinking if opts.HideThoughts
  - [x] 3.3: In renderAssistantMessageStatic (line ~654), add hideThoughts parameter and skip accordingly

- [x] Task 4: Implement --hide-tools flag behavior (AC: 1.9.2)
  - [x] 4.1: In viewer.go renderAssistantMessage (line ~543), skip ContentTypeToolUse if m.renderOpts.HideTools
  - [x] 4.2: In plain.go renderAssistantMessagePlain (line ~68), skip ContentTypeToolUse if opts.HideTools
  - [x] 4.3: In renderAssistantMessageStatic (line ~654), add hideTools parameter and skip accordingly

- [x] Task 5: Add TruncateToWidth utility function (Required for Task 6)
  - [x] 5.1: TruncateToWidth already exists in stringwidth.go
  - [x] 5.2: Handle multi-byte characters correctly using lipgloss.Width() - already implemented
  - [x] 5.3: Add "..." suffix when truncated - already implemented

- [x] Task 6: Implement richer collapsed tool summaries (AC: 1.9.4)
  - [x] 6.1: Add `import "path/filepath"` to viewer.go
  - [x] 6.2: Add formatToolSummary function in viewer.go (after renderToolUseBlockStatic)
  - [x] 6.3: Implement Read tool summary: `Read: {basename} (lines {offset}-{offset+limit})` or `(full file)`
  - [x] 6.4: Implement Edit tool summary: `Edit: {basename} (+{added}/-{removed} lines)`
  - [x] 6.5: Implement Glob tool summary: `Glob: {pattern}`
  - [x] 6.6: Implement Grep tool summary: `Grep: "{pattern}" in {path}`
  - [x] 6.7: Implement Write tool summary: `Write: {basename}`
  - [x] 6.8: Implement Bash tool summary: `Bash: {cmd truncated to 40 chars}`
  - [x] 6.9: Implement Task tool summary: `Task: {subagent_type} - "{description}"`
  - [x] 6.10: Implement TodoWrite summary: `TodoWrite: {count} items`
  - [x] 6.11: Implement WebFetch summary: `WebFetch: {url truncated}`
  - [x] 6.12: Implement WebSearch summary: `WebSearch: "{query}"`
  - [x] 6.13: Default fallback: `{ToolName}: [collapsed]`
  - [x] 6.14: Replace collapsed text in renderToolUseBlock (line ~591) with formatToolSummary call
  - [x] 6.15: Replace collapsed text in renderToolUseBlockStatic (line ~716) with formatToolSummary call

- [x] Task 7: Update tests and verify (all ACs)
  - [x] 7.1: Add unit tests for formatToolSummary in viewer_test.go (table-driven)
  - [x] 7.2: TruncateToWidth tests already exist in stringwidth_test.go
  - [x] 7.3: Update existing viewer_test.go NewViewerModel calls with DefaultRenderOptions()
  - [x] 7.4: Run `make test` - all tests pass
  - [x] 7.5: Run `make lint` - no lint errors
  - [x] 7.6: Run `make build` - build succeeds
  - [ ] 7.7: Manual test: `./bin/cclv --hide-thoughts <file>` - verify no thinking blocks
  - [ ] 7.8: Manual test: `./bin/cclv --hide-tools <file>` - verify no tool blocks
  - [ ] 7.9: Manual test: `./bin/cclv <file>` - verify rich collapsed tool summaries

## Dev Notes

### Caller List (All Must Be Updated)

| File | Line | Current Call | New Call |
|------|------|--------------|----------|
| `cmd/cclv/main.go` | 122 | `tui.RenderPlain(entries, source)` | `tui.RenderPlain(entries, source, opts)` |
| `cmd/cclv/main.go` | 128 | `tui.NewViewerModel(entries, errors, source)` | `tui.NewViewerModel(entries, errors, source, opts)` |
| `internal/tui/app.go` | 184 | `NewViewerModelWithBackNavigation(entries, errors, title)` | `NewViewerModelWithBackNavigation(entries, errors, title, DefaultRenderOptions())` |
| `internal/tui/viewer_test.go` | 70 | `NewViewerModel(entries, 0, "Test")` | `NewViewerModel(entries, 0, "Test", DefaultRenderOptions())` |

### TruncateToWidth Implementation

```go
// Add to internal/tui/utils.go

// TruncateToWidth truncates a string to fit within maxWidth visual characters.
// Uses lipgloss.Width() to handle multi-byte characters correctly.
// Appends "..." if truncation occurs.
func TruncateToWidth(s string, maxWidth int) string {
    if maxWidth <= 3 {
        return "..."
    }

    visualWidth := lipgloss.Width(s)
    if visualWidth <= maxWidth {
        return s
    }

    // Binary search for the right truncation point
    runes := []rune(s)
    for i := len(runes); i > 0; i-- {
        truncated := string(runes[:i]) + "..."
        if lipgloss.Width(truncated) <= maxWidth {
            return truncated
        }
    }
    return "..."
}
```

### formatToolSummary Implementation

```go
// Add to internal/tui/viewer.go (after renderToolUseBlockStatic)

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

    default:
        return fmt.Sprintf("%s: [collapsed]", toolName)
    }
}
```

### RenderOptions Implementation

```go
// Add to internal/tui/viewer.go (after imports, before ViewerModel struct)

// RenderOptions controls visibility of content types during rendering.
type RenderOptions struct {
    HideThoughts bool // Hide thinking blocks
    HideTools    bool // Hide tool use blocks
}

// DefaultRenderOptions returns options that show all content types.
func DefaultRenderOptions() RenderOptions {
    return RenderOptions{
        HideThoughts: false,
        HideTools:    false,
    }
}
```

### Signature Changes Summary

| Function | Old Signature | New Signature |
|----------|---------------|---------------|
| NewViewerModel | `(entries []types.LogEntry, parseErrors int, title string) ViewerModel` | `(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions) ViewerModel` |
| NewViewerModelWithBack | `(entries []types.LogEntry, parseErrors int, title string) ViewerModel` | `(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions) ViewerModel` |
| NewViewerModelWithBackNavigation | `(entries []types.LogEntry, parseErrors int, title string) ViewerModel` | `(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions) ViewerModel` |
| RenderPlain | `(entries []types.LogEntry, source string) string` | `(entries []types.LogEntry, source string, opts RenderOptions) string` |
| renderEntryPlain | `(entry types.LogEntry) string` | `(entry types.LogEntry, opts RenderOptions) string` |
| renderAssistantMessagePlain | `(entry types.LogEntry) string` | `(entry types.LogEntry, opts RenderOptions) string` |

### main.go Flag Setup Pattern

```go
// Add after line 36 (versionShortFlag)
hideThoughtsFlag := flag.Bool("hide-thoughts", false, "Hide thinking blocks in output")
hideToolsFlag := flag.Bool("hide-tools", false, "Hide tool use blocks in output")

// In runPipelineMode, before mode check (around line 119):
opts := tui.RenderOptions{
    HideThoughts: *hideThoughtsFlag,
    HideTools:    *hideToolsFlag,
}
```

### Rendering Skip Logic Pattern

```go
// In renderAssistantMessage loop (viewer.go ~line 543):
for _, content := range entry.Message.Content {
    switch content.Type {
    case types.ContentTypeText:
        // ... existing text handling ...

    case types.ContentTypeThinking:
        if m.renderOpts.HideThoughts {
            continue // Skip thinking blocks
        }
        parts = append(parts, m.renderThinkingBlock(content))

    case types.ContentTypeToolUse:
        if m.renderOpts.HideTools {
            continue // Skip tool blocks
        }
        parts = append(parts, m.renderToolUseBlock(content))
    }
}
```

### Collapsed Tool Summary Update

```go
// In renderToolUseBlock (viewer.go line ~589-592):
// OLD:
if !m.showToolInputs {
    return Styles.ToolBlock.Render(
        header + " " + Styles.CollapsedIndicator.Render("[inputs - press 'i' to expand]"),
    )
}

// NEW:
if !m.showToolInputs {
    summary := formatToolSummary(content.ToolName, content.ToolInput)
    return Styles.ToolBlock.Render(
        header + " " + Styles.CollapsedIndicator.Render(summary),
    )
}
```

### Edge Cases

1. **Empty tool input**: Return `{ToolName}: [collapsed]`
2. **Missing file_path**: Return `{ToolName}: [collapsed]`
3. **Non-string values**: Type assert with ok check, use zero value on failure
4. **Very long patterns/commands**: TruncateToWidth handles truncation
5. **Combined flags**: Both skip their content types independently
6. **JSON number types**: Tool input numbers come as float64 from JSON, cast accordingly

### Testing Approach

```bash
# Manual verification commands
./bin/cclv --hide-thoughts test.jsonl | grep -c "thinking"  # Should be 0
./bin/cclv --hide-tools test.jsonl | grep -c "Tool:"  # Should be 0
./bin/cclv --hide-thoughts --hide-tools test.jsonl  # Only User/Assistant text
./bin/cclv test.jsonl  # Rich collapsed summaries like "Read: viewer.go (full file)"
```

### Test Patterns (Table-Driven)

```go
func TestFormatToolSummary(t *testing.T) {
    tests := []struct {
        name     string
        toolName string
        input    map[string]any
        want     string
    }{
        {
            name:     "Read full file",
            toolName: "Read",
            input:    map[string]any{"file_path": "/path/to/file.go"},
            want:     "Read: file.go (full file)",
        },
        {
            name:     "Read with offset",
            toolName: "Read",
            input:    map[string]any{"file_path": "/path/to/file.go", "offset": float64(10), "limit": float64(50)},
            want:     "Read: file.go (lines 10-60)",
        },
        {
            name:     "Edit with changes",
            toolName: "Edit",
            input:    map[string]any{"file_path": "/path/to/file.go", "old_string": "old", "new_string": "new\nline"},
            want:     "Edit: file.go (+2/-1 lines)",
        },
        {
            name:     "Bash truncated",
            toolName: "Bash",
            input:    map[string]any{"command": "make build && make test && make lint"},
            want:     "Bash: make build && make test && make l...",
        },
        {
            name:     "Unknown tool",
            toolName: "CustomTool",
            input:    map[string]any{},
            want:     "CustomTool: [collapsed]",
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := formatToolSummary(tt.toolName, tt.input)
            if got != tt.want {
                t.Errorf("formatToolSummary() = %q, want %q", got, tt.want)
            }
        })
    }
}
```

### References

- [Source: epics.md lines 461-509] - Story requirements
- [Source: project-context.md] - NO EMOJI, USE MAKEFILE rules
- [Source: cmd/cclv/main.go lines 30-37] - Existing flag parsing
- [Source: internal/tui/viewer.go lines 582-606] - renderToolUseBlock
- [Source: internal/tui/viewer.go lines 708-730] - renderToolUseBlockStatic
- [Source: internal/tui/plain.go lines 68-89] - Plain mode tool rendering
- [Source: internal/types/entry.go lines 52-59] - MessageContent struct

### Previous Story Intelligence

From Epic 1.5 retrospective:
- Code reviews caught real issues (overlay wasn't overlaying)
- Terminal overlay requires custom approach
- Simple stories can be wins when properly scoped

From Story 1.8:
- Pre-rendering pattern for async operations (used in markAllMessagesLoadedCmd)
- renderEntryStatic pattern separates model state from rendering

### Git Commit Pattern

```
feat: add pipeline visibility flags and rich tool summaries

- Add --hide-thoughts and --hide-tools CLI flags
- Implement RenderOptions for visibility control
- Add formatToolSummary for actionable collapsed tool info
- Update all callers to new signatures

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Risk Assessment

**Risk: LOW**

- CLI flag parsing is well-established pattern in main.go
- Filtering content types is straightforward conditional logic
- formatToolSummary is self-contained with clear fallbacks
- No async complexity or goroutine management
- Backward compatibility maintained with DefaultRenderOptions()
- All existing tests updated via single call pattern

### Files to Modify Checklist

- [x] `cmd/cclv/main.go` - Add flags, create opts, pass to tui functions
- [x] `internal/tui/viewer.go` - Add RenderOptions, formatToolSummary, update signatures
- [x] `internal/tui/plain.go` - Update RenderPlain and helper signatures, add filtering
- [x] `internal/tui/utils.go` - TruncateToWidth already exists in stringwidth.go
- [x] `internal/tui/app.go` - Update NewViewerModelWithBackNavigation call
- [x] `internal/tui/viewer_test.go` - Update NewViewerModel calls, add formatToolSummary tests

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- TruncateToWidth function already existed in stringwidth.go with proper implementation, no need to duplicate
- RenderOptions struct added to viewer.go with HideThoughts and HideTools flags
- formatToolSummary implements rich summaries for Read, Edit, Glob, Grep, Write, Bash, Task, TodoWrite, WebFetch, WebSearch, with fallback for unknown tools
- Static rendering functions (renderEntryStatic, renderAssistantMessageStatic) updated to accept and use RenderOptions
- All callers updated: main.go, app.go, viewer_test.go
- 17 unit tests added for formatToolSummary covering all tool types
- make test, make lint, make build all pass

### Code Review Fixes (2026-01-16)

- Added NotebookEdit tool to formatToolSummary (shows notebook name and edit mode)
- Added TestRenderPlainWithOptions test for plain mode visibility filtering (4 test cases)
- Added "Read with offset only" test case for default limit behavior
- Added 3 NotebookEdit test cases to TestFormatToolSummary
- Total test cases now: 21 formatToolSummary tests + 4 RenderPlain visibility tests
- Coverage improved from 22.9% to 27.2%

### File List

- `cmd/cclv/main.go` - Added --hide-thoughts and --hide-tools flags, RenderOptions creation, updated runPipelineMode signature
- `internal/tui/viewer.go` - Added RenderOptions struct, DefaultRenderOptions(), formatToolSummary(), updated all viewer model constructors and rendering functions
- `internal/tui/plain.go` - Updated RenderPlain, renderEntryPlain, renderAssistantMessagePlain to use RenderOptions
- `internal/tui/app.go` - Updated NewViewerModelWithBackNavigation call to use DefaultRenderOptions()
- `internal/tui/viewer_test.go` - Updated NewViewerModel call, added TestFormatToolSummary with 17 test cases
