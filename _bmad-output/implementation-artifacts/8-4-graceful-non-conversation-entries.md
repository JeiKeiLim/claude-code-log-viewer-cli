# Story 8.4: Graceful Handling of Non-Conversation Entries

Status: done

## Story

As a **developer integrating cclv with other tools (like vibe-dash)**,
I want **non-conversation JSONL entries to be handled gracefully**,
So that **piping mixed log content doesn't cause errors**.

## Acceptance Criteria

1. **AC-1: Non-Conversation Entries Skipped Silently**
   - Given I pipe a JSONL line without a recognized `type` field
   - When cclv processes it
   - Then the entry is skipped (no output for that line)
   - And no error message is shown

2. **AC-2: Exit Code 0 for Valid JSONL**
   - Given I pipe only non-conversation JSONL (e.g., session metadata)
   - When cclv finishes processing
   - Then exit code is 0 (not 1)
   - And no "no entries found" error is shown

3. **AC-3: Mixed Input Works Correctly**
   - Given a file with both session metadata and conversation entries
   - When processed by cclv
   - Then conversation entries are formatted normally
   - And non-conversation entries are silently skipped

4. **AC-4: Consistent Behavior File vs Stdin**
   - Given non-conversation JSONL
   - When processed via file path OR stdin pipe
   - Then behavior is identical (skip silently, exit 0)

5. **AC-5: Streaming Mode Handles Non-Conversation**
   - Given `cclv --watch --plain file.jsonl` is running
   - When a non-conversation entry is appended
   - Then no output is produced for that entry
   - And no error is shown
   - And streaming continues normally

## Tasks / Subtasks

- [x] Task 1: Update "no entries" error handling in main.go (AC: #2)
  - [x] Subtask 1.1: In `cmd/cclv/main.go`, remove the "no entries found" error for valid JSONL
  - [x] Subtask 1.2: Allow empty output (exit 0) when JSONL was valid but had no conversation entries

- [x] Task 2: Add/verify tests for non-conversation entry handling (AC: #1-5)
  - [x] Subtask 2.1: Add `TestParseJSONL_OnlyMetadata_NoError` in `jsonl_test.go` (if not exists)
  - [x] Subtask 2.2: CLI smoke test for exit code behavior

## Dev Notes

### Root Cause Analysis

**IMPORTANT: The parser already handles non-conversation entries correctly.**

**Current behavior in `internal/parser/jsonl.go` (lines 44-47):**
```go
// Only parse user and assistant entries
if raw.Type != string(types.EntryTypeUser) && raw.Type != string(types.EntryTypeAssistant) {
    continue  // Skips silently - no error, no ParseErrors increment
}
```

This means AC-1, AC-3, AC-4, and AC-5 are **already satisfied** by the existing code.

**The actual problem is in `cmd/cclv/main.go` (lines 263-268):**
```go
if len(result.Entries) == 0 {
    if result.ParseErrors > 0 {
        return fmt.Errorf("no valid entries found (%d parse errors)", result.ParseErrors)
    }
    return fmt.Errorf("no entries found in input")  // <-- THIS IS THE BUG
}
```

When piping JSONL that contains only session metadata (valid JSON, but no conversation entries), this returns exit code 1 with an error message. The fix is to treat empty conversation results as valid when the JSONL itself was valid.

**Fix approach:**
```go
// Remove the "no entries found" error entirely, or only error on parse failures
if len(result.Entries) == 0 && result.ParseErrors > 0 {
    return fmt.Errorf("no valid entries found (%d parse errors)", result.ParseErrors)
}
// Empty entries with no parse errors = valid JSONL with no conversation content = exit 0
```

### Non-Conversation Entry Types

From Claude Code logs, these are known non-conversation entries:

| Entry Pattern | Example |
|---------------|---------|
| Session init | `{"parentUuid":"...","isSidechain":false,"userType":"external"}` |
| Session metadata | `{"cwd":"/path","timestamp":"..."}` |
| Config entries | `{"model":"...","apiKey":"..."}` |

All lack the `type` field that conversation entries have.

### Exit Code Logic

**Current (buggy) in `cmd/cclv/main.go`:**
```go
if len(result.Entries) == 0 {
    if result.ParseErrors > 0 {
        return fmt.Errorf("no valid entries found (%d parse errors)", result.ParseErrors)
    }
    return fmt.Errorf("no entries found in input")  // Bug: exits 1 for valid JSONL
}
```

**Fixed:**
```go
// Only error if ALL lines failed to parse
// Empty conversation list is OK if JSONL was valid (no parse errors)
if len(result.Entries) == 0 && result.ParseErrors > 0 {
    return fmt.Errorf("no valid entries found (%d parse errors)", result.ParseErrors)
}
// len(result.Entries) == 0 with ParseErrors == 0 means valid JSONL with no conversation entries
```

### Files to Modify

| File | Change |
|------|--------|
| `cmd/cclv/main.go` | Update "no entries" error logic (only error on parse failures) |
| `internal/parser/jsonl_test.go` | Add test for metadata-only input (verify no ParseErrors) |

### Files to NOT Modify

- `internal/parser/entry.go` - Already works correctly (no validation of type field)
- `internal/parser/jsonl.go` - Already skips non-conversation entries silently
- `internal/tui/` - No rendering changes needed
- `internal/watcher/` - Streaming already uses parser, will inherit fix
- `internal/types/` - No type changes needed

### Test Strategy

**Parser tests (jsonl_test.go) - verify existing behavior:**
```go
func TestParseJSONL_OnlyMetadata_NoError(t *testing.T) {
    input := strings.NewReader(
        `{"parentUuid":"abc","isSidechain":false}` + "\n",
    )
    result := ParseJSONL(input)

    if len(result.Entries) != 0 {
        t.Errorf("expected 0 entries, got %d", len(result.Entries))
    }
    if result.ParseErrors != 0 {
        t.Errorf("expected 0 parse errors, got %d", result.ParseErrors)
    }
    // This confirms non-conversation entries are skipped without error
}
```

**Note:** `TestParseJSONL_MixedEntries_SkipsUnknown` may already exist - verify before adding.

### Manual Integration Test

```bash
# Test 1: Session metadata only - should exit 0 with no output
echo '{"parentUuid":"abc","isSidechain":false}' | ./bin/cclv --plain
echo $?  # Should be 0

# Test 2: Mixed input - should show only conversation entry
cat << 'EOF' | ./bin/cclv --plain
{"parentUuid":"abc","isSidechain":false}
{"type":"user","message":{"role":"user","content":"Hello"}}
{"cwd":"/foo"}
EOF
# Should show formatted "Hello" message only

# Test 3: Streaming with metadata
echo '{"parentUuid":"abc"}' >> test.jsonl
./bin/cclv --watch --plain test.jsonl &
echo '{"type":"user","message":{"role":"user","content":"Test"}}' >> test.jsonl
# Should show "Test" message, ignore the metadata line
```

### Architecture Compliance

- **NO EMOJI** - No UI changes, not applicable
- **Use Makefile** - `make test` for testing
- **90%+ coverage** - Add tests for new code paths
- **CLI smoke test** - Manual test required per new rule

### Previous Story Learnings

From Story 8.3:
- Streaming mode inherits parser behavior automatically
- Test both file and stdin paths

**Validation Finding (2026-01-20):**
The original story incorrectly analyzed the parser. The parser already skips non-conversation entries silently (jsonl.go lines 44-47). Only the main.go error handling needed correction.

### Complexity Assessment

**Very low complexity** - Single file change:
- Update error condition in `cmd/cclv/main.go` (1 line change)
- Add/verify test in `jsonl_test.go`
- CLI smoke test

The parser already handles non-conversation entries correctly. Only the main.go error handling needs adjustment.

### Expected Commit Format

```
fix: allow empty conversation output for valid JSONL (Story 8.4)

Remove "no entries found" error when piping JSONL that contains only
session metadata. Exit 0 when JSONL is valid but has no conversation
entries. Enables clean piping of mixed logs from vibe-dash.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: vibe-dash feature request](/Users/limjk/GitHub/JeiKeiLim/vibe-dash/docs/external-requests/cclv-feature-metadata-handling.md)
- [Source: Epic 8 Retrospective](epic-8-retro-2026-01-20.md) - Discovery context
- [Source: internal/parser/entry.go] - Current ParseEntry implementation
- [Source: internal/parser/jsonl.go] - Current ParseJSONL implementation
- [Source: _bmad-output/project-context.md] - Critical rules

## Dev Agent Record

### File List

| File | Change Type | Description |
|------|-------------|-------------|
| `cmd/cclv/main.go` | Modified | Updated error handling to allow empty entries with no parse errors (exit 0) |
| `internal/parser/jsonl_test.go` | Added | New test file with 6 table-driven tests for non-conversation entry handling |

### Change Log

| Date | Change | Reason |
|------|--------|--------|
| 2026-01-20 | Updated main.go error condition | AC-2: Allow exit 0 for valid JSONL with no conversation entries |
| 2026-01-20 | Added jsonl_test.go | AC-1 through AC-5: Verify parser behavior for non-conversation entries |

### Critical Rules (from project-context.md)

- NO EMOJI in any output
- Use `make test` not raw `go test`
- Table-driven tests required
- 90%+ test coverage required
- CLI smoke test required for flag changes (this story affects CLI behavior)
