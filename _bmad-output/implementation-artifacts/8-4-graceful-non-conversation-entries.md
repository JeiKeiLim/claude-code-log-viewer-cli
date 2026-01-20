# Story 8.4: Graceful Handling of Non-Conversation Entries

Status: ready-for-dev

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

- [ ] Task 1: Update parser to skip unknown entry types (AC: #1, #3)
  - [ ] Subtask 1.1: In `internal/parser/entry.go`, modify `ParseEntry()` to return `nil, nil` for unrecognized entries
  - [ ] Subtask 1.2: In `internal/parser/jsonl.go`, skip nil entries in `ParseJSONL()` loop
  - [ ] Subtask 1.3: Ensure parse error count does NOT increment for skipped entries

- [ ] Task 2: Update "no entries" error handling (AC: #2)
  - [ ] Subtask 2.1: In `cmd/cclv/main.go`, change error condition from "no entries" to "no valid JSONL lines"
  - [ ] Subtask 2.2: Allow empty output (exit 0) when JSONL was valid but had no conversation entries

- [ ] Task 3: Add tests for non-conversation entry handling (AC: #1-5)
  - [ ] Subtask 3.1: Add `TestParseEntry_UnknownType_ReturnsNil` in `entry_test.go`
  - [ ] Subtask 3.2: Add `TestParseJSONL_MixedEntries_SkipsUnknown` in `jsonl_test.go`
  - [ ] Subtask 3.3: Add `TestParseJSONL_OnlyMetadata_NoError` in `jsonl_test.go`
  - [ ] Subtask 3.4: Update streaming mode tests if needed

## Dev Notes

### Root Cause Analysis

**Current behavior in `internal/parser/entry.go`:**
```go
func ParseEntry(data []byte) (*types.LogEntry, error) {
    var raw map[string]interface{}
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, err
    }

    entryType, ok := raw["type"].(string)
    if !ok {
        return nil, fmt.Errorf("missing or invalid type field")
    }
    // ... switch on entryType
}
```

The `return nil, fmt.Errorf("missing or invalid type field")` causes:
1. Parse error count to increment
2. When ALL entries fail, "no entries found" error

**Fix approach:**
```go
func ParseEntry(data []byte) (*types.LogEntry, error) {
    var raw map[string]interface{}
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, err // Still return error for invalid JSON
    }

    entryType, ok := raw["type"].(string)
    if !ok {
        // Not a conversation entry - skip silently (not an error)
        return nil, nil
    }
    // ... switch on entryType
}
```

**In `internal/parser/jsonl.go`:**
```go
for scanner.Scan() {
    entry, err := ParseEntry(scanner.Bytes())
    if err != nil {
        result.ParseErrors++
        continue
    }
    if entry == nil {
        // Skipped entry (non-conversation) - don't count as error
        continue
    }
    result.Entries = append(result.Entries, *entry)
}
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

**Current (buggy):**
```go
if len(result.Entries) == 0 {
    return fmt.Errorf("no entries found in input")
}
```

**Fixed:**
```go
// Only error if we couldn't parse ANY valid JSONL
// Empty conversation list is OK if JSONL was valid
if result.TotalLines == 0 {
    return fmt.Errorf("no valid JSONL lines in input")
}
// len(result.Entries) == 0 is fine - just means no conversation entries
```

### Files to Modify

| File | Change |
|------|--------|
| `internal/parser/entry.go` | Return `nil, nil` for missing type field |
| `internal/parser/jsonl.go` | Skip nil entries, don't count as error |
| `cmd/cclv/main.go` | Update "no entries" error logic |
| `internal/parser/entry_test.go` | Add test for unknown type handling |
| `internal/parser/jsonl_test.go` | Add tests for mixed/metadata-only input |

### Files to NOT Modify

- `internal/tui/` - No rendering changes needed
- `internal/watcher/` - Streaming already uses parser, will inherit fix
- `internal/types/` - No type changes needed

### Test Strategy

```go
// entry_test.go
func TestParseEntry_UnknownType_ReturnsNil(t *testing.T) {
    tests := []struct {
        name  string
        input string
    }{
        {"session init", `{"parentUuid":"abc","isSidechain":false}`},
        {"session metadata", `{"cwd":"/foo","timestamp":"2026-01-20"}`},
        {"empty object", `{}`},
        {"random fields", `{"foo":"bar","baz":123}`},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            entry, err := ParseEntry([]byte(tt.input))
            if err != nil {
                t.Errorf("expected no error, got %v", err)
            }
            if entry != nil {
                t.Errorf("expected nil entry for non-conversation JSONL")
            }
        })
    }
}

// jsonl_test.go
func TestParseJSONL_MixedEntries_SkipsUnknown(t *testing.T) {
    input := strings.NewReader(
        `{"parentUuid":"abc","isSidechain":false}` + "\n" +
        `{"type":"user","message":{"content":"Hello"}}` + "\n" +
        `{"cwd":"/foo"}` + "\n",
    )
    result := ParseJSONL(input)

    if len(result.Entries) != 1 {
        t.Errorf("expected 1 conversation entry, got %d", len(result.Entries))
    }
    if result.ParseErrors != 0 {
        t.Errorf("expected 0 parse errors, got %d", result.ParseErrors)
    }
}

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
    // Result is valid - no error should occur downstream
}
```

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
- Parser changes are low-risk
- Streaming mode inherits parser behavior automatically
- Test both file and stdin paths

### Complexity Assessment

**Low complexity** - Parser-only change:
- Modify return value for unknown entries
- Skip nil entries in loop
- Update exit code logic
- Add tests

### Expected Commit Format

```
fix: handle non-conversation JSONL entries gracefully (Story 8.4)

Skip session metadata and other non-conversation entries silently
instead of returning an error. Enables clean piping of mixed logs.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: vibe-dash feature request](/Users/limjk/GitHub/JeiKeiLim/vibe-dash/docs/external-requests/cclv-feature-metadata-handling.md)
- [Source: Epic 8 Retrospective](epic-8-retro-2026-01-20.md) - Discovery context
- [Source: internal/parser/entry.go] - Current ParseEntry implementation
- [Source: internal/parser/jsonl.go] - Current ParseJSONL implementation
- [Source: _bmad-output/project-context.md] - Critical rules

### Critical Rules (from project-context.md)

- NO EMOJI in any output
- Use `make test` not raw `go test`
- Table-driven tests required
- 90%+ test coverage required
- CLI smoke test required for flag changes (this story affects CLI behavior)
