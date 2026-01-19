# Story 6.2: Token Calculation Service

Status: done

## Story

As a **developer building statistics**,
I want **a token calculation service with caching integrated into the application**,
So that **tokens can be calculated efficiently for entries missing usage data**.

## Acceptance Criteria

1. **Given** text content **When** `Calculate()` is called **Then** token count is returned

2. **Given** the same text is calculated twice **When** `Calculate()` is called the second time **Then** cached result is returned (no recalculation)

3. **Given** a calculation request **When** processed **Then** calculation completes in < 50ms

## Tasks / Subtasks

- [x] Task 1: Extend token.Service with batch calculation capability (AC: #1, #3)
  - [x] Add `CalculateBatch(texts []string) []int` method to `internal/token/service.go`
  - [x] Ensure batch method uses same cache as individual calculations
  - [x] Maintain thread safety with existing `sync.RWMutex`
  - [x] Verify batch calculation completes in < 50ms for typical log entries

- [x] Task 2: Add entry-level token calculation helper (AC: #1, #2)
  - [x] Create `CalculateEntry(entry types.LogEntry) int` method
  - [x] Handle `EntryTypeUser`: tokenize `entry.Message.TextContent`
  - [x] Handle `EntryTypeAssistant`: sum tokens from all content blocks
    - Include `ContentTypeText` (`.Text` field)
    - Include `ContentTypeThinking` (`.Thinking` field)
    - Include `ContentTypeToolUse`: serialize `ToolInput` map to JSON and tokenize
  - [x] Handle `EntryTypeFileHistorySnapshot`: return 0 (no user-facing text)
  - [x] Return cached result for repeated calls with same entry content

- [x] Task 3: Create conversation-level token aggregation (AC: #1, #2)
  - [x] Add `CalculateConversation(entries []types.LogEntry) (calculated int, estimated bool)` method
  - [x] Sum tokens from entries with existing `Usage` data when available
  - [x] Fall back to calculation for entries with empty `Usage`
  - [x] Return whether result includes any estimated values

- [x] Task 4: Integrate token service into AppModel (AC: #1, #2)
  - [x] Add `tokenService *token.Service` field to AppModel struct
  - [x] Initialize in `NewAppModel()` with soft-fail (log warning, set to nil on error)
  - [x] ~~Add nil-guard helper~~ (deferred to Story 6.3 when actually used)
  - [x] Clear cache at these trigger points:
    - In `ConversationSelectedMsg` handler before loading new conversation
    - In `GoBackMsg` handler when leaving viewer (returning to conversation list or dashboard)
  - [x] ~~Pass service reference to ViewerModel when entering viewer~~ (deferred to Story 6.3)

- [x] Task 5: Write comprehensive tests (AC: #1, #2, #3)
  - [x] Test `CalculateBatch()` with various batch sizes (0, 1, 10, 100)
  - [x] Test `CalculateEntry()` with:
    - User entry with TextContent
    - Assistant entry with text content
    - Assistant entry with thinking block
    - Assistant entry with tool_use and ToolInput map
    - file-history-snapshot entry (returns 0)
  - [x] Test `CalculateConversation()` with:
    - All entries with Usage data → estimated=false
    - All entries without Usage data → estimated=true
    - Mixed entries → estimated=true
  - [x] Test cache behavior: same entry returns cached result
  - [x] Add benchmark: `BenchmarkCalculateEntry` must complete < 50ms
  - [x] Ensure 95% coverage for new code (achieved 100%)

- [x] Task 6: Build and validation (AC: #1, #2, #3)
  - [x] Run `make test` - all tests pass
  - [x] Run `make lint` - no linting errors
  - [x] Run `make ci` - full CI validation passes
  - [x] Verify benchmark meets 50ms target (~3.8μs/op)

## Dev Notes

### Existing Foundation (Story 6.1)

```go
// internal/token/service.go - EXISTING
type Service struct {
    encoder *tiktoken.Tiktoken
    cache   map[string]int
    mu      sync.RWMutex
}

func New() (*Service, error)
func NewWithEncoding(encoding string) (*Service, error)
func (s *Service) Calculate(text string) int
func (s *Service) ClearCache()
```

### New Methods Implementation Guide

**CalculateBatch** - Simple loop over Calculate():
```go
func (s *Service) CalculateBatch(texts []string) []int {
    results := make([]int, len(texts))
    for i, text := range texts {
        results[i] = s.Calculate(text)
    }
    return results
}
```

**CalculateEntry** - Extract text from all content types:
```go
func (s *Service) CalculateEntry(entry types.LogEntry) int {
    switch entry.Type {
    case types.EntryTypeUser:
        return s.Calculate(entry.Message.TextContent)
    case types.EntryTypeAssistant:
        var total int
        for _, content := range entry.Message.Content {
            switch content.Type {
            case types.ContentTypeText:
                total += s.Calculate(content.Text)
            case types.ContentTypeThinking:
                total += s.Calculate(content.Thinking)
            case types.ContentTypeToolUse:
                // Serialize ToolInput to JSON for tokenization
                if len(content.ToolInput) > 0 {
                    if data, err := json.Marshal(content.ToolInput); err == nil {
                        total += s.Calculate(string(data))
                    }
                }
            }
        }
        return total
    default:
        // EntryTypeFileHistorySnapshot: no user-facing text
        return 0
    }
}
```

**CalculateConversation** - Use log data when available:
```go
func (s *Service) CalculateConversation(entries []types.LogEntry) (int, bool) {
    var total int
    var estimated bool
    for _, entry := range entries {
        if !entry.Usage.IsEmpty() {
            total += entry.Usage.Total()
        } else {
            total += s.CalculateEntry(entry)
            estimated = true
        }
    }
    return total, estimated
}
```

### AppModel Integration Pattern

```go
// internal/tui/app.go modifications

type AppModel struct {
    // ... existing fields ...
    tokenService *token.Service
}

func NewAppModel(projects []types.Project) AppModel {
    // ... existing init ...
    tokenSvc, err := token.New()
    if err != nil {
        // Log warning but continue - token features disabled
        // Use log.Printf in dev, or stderr in production
    }
    return AppModel{
        // ... existing fields ...
        tokenService: tokenSvc,
    }
}

// Nil-guard helper
func (m *AppModel) hasTokenService() bool {
    return m.tokenService != nil
}
```

**Cache Clear Trigger Points in Update():**
```go
case ConversationSelectedMsg:
    // Clear cache before loading new conversation
    if m.tokenService != nil {
        m.tokenService.ClearCache()
    }
    // ... existing handling ...

case GoBackMsg:
    if m.state == viewViewer {
        // Clear cache when leaving viewer
        if m.tokenService != nil {
            m.tokenService.ClearCache()
        }
    }
    // ... existing handling ...
```

### Type Dependencies

Import required in token package:
```go
import (
    "encoding/json"
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)
```

Dependency flow: `token → types` (acceptable per architecture)

### Testing Patterns

**Table-driven test for CalculateEntry:**
```go
func TestCalculateEntry(t *testing.T) {
    s, _ := token.New()
    tests := []struct {
        name    string
        entry   types.LogEntry
        wantMin int
        wantMax int
    }{
        {
            name: "user message",
            entry: types.LogEntry{
                Type: types.EntryTypeUser,
                Message: types.Message{TextContent: "Hello, world!"},
            },
            wantMin: 2, wantMax: 6,
        },
        {
            name: "assistant with tool_use",
            entry: types.LogEntry{
                Type: types.EntryTypeAssistant,
                Message: types.Message{
                    Content: []types.MessageContent{{
                        Type:      types.ContentTypeToolUse,
                        ToolInput: map[string]any{"file": "test.go", "content": "package main"},
                    }},
                },
            },
            wantMin: 5, wantMax: 20,
        },
        {
            name: "file-history-snapshot",
            entry: types.LogEntry{Type: types.EntryTypeFileHistorySnapshot},
            wantMin: 0, wantMax: 0,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := s.CalculateEntry(tt.entry)
            if got < tt.wantMin || got > tt.wantMax {
                t.Errorf("got %d, want [%d, %d]", got, tt.wantMin, tt.wantMax)
            }
        })
    }
}
```

**Benchmark test:**
```go
func BenchmarkCalculateEntry(b *testing.B) {
    s, _ := token.New()
    entry := types.LogEntry{
        Type: types.EntryTypeAssistant,
        Message: types.Message{
            Content: []types.MessageContent{
                {Type: types.ContentTypeText, Text: strings.Repeat("Hello world ", 100)},
                {Type: types.ContentTypeThinking, Thinking: strings.Repeat("Let me think ", 50)},
            },
        },
    }
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        s.CalculateEntry(entry)
    }
    // Verify: go test -bench=. should show < 50ms/op
}
```

### Files to Modify

| File | Changes |
|------|---------|
| `internal/token/service.go` | Add CalculateBatch, CalculateEntry, CalculateConversation |
| `internal/token/service_test.go` | Add tests for all new methods + benchmark |
| `internal/tui/app.go` | Add tokenService field, initialize in NewAppModel, add cache clear triggers |

No new files created.

### References

- [internal/token/service.go - existing implementation from Story 6.1]
- [internal/types/entry.go - LogEntry, TokenUsage, ContentType definitions]
- [internal/tui/app.go - AppModel struct for integration]
- [architecture-phase3.md Decision #7 - token package design]
- [project-context.md - testing rules, coverage targets]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

1. **CalculateBatch**: Simple loop over existing Calculate() method, leverages cache automatically
2. **CalculateEntry**: Handles all entry types - user messages via TextContent, assistant messages with text/thinking/tool_use content blocks, file-history-snapshot returns 0
3. **CalculateConversation**: Uses actual Usage data when available (from API response), falls back to CalculateEntry for entries without usage data, returns estimated flag
4. **AppModel Integration**: Token service initialized with soft-fail (logs warning, continues with nil), cache cleared on conversation selection and when leaving viewer
5. **hasTokenService helper**: Deferred to Story 6.3 - removed to pass lint since unused in this story
6. **ViewerModel integration**: Deferred to Story 6.3 - service reference not passed yet as viewer doesn't use it in this story
7. **Test Coverage**: 100% coverage on token package, all 16 new tests pass with race detection
8. **Benchmark**: ~3.8μs per CalculateEntry operation (well under 50ms target)

### Code Review Fixes Applied

1. **[HIGH FIXED]** CalculateConversation now correctly handles file-history-snapshot entries without marking them as "estimated" - they are 0 tokens by design
2. **[MEDIUM FIXED]** Added cache clear to ConversationSelectedWithWatchMsg handler for consistency with non-watch mode
3. **[MEDIUM FIXED]** JSON marshal errors in ToolInput serialization now logged instead of silently ignored
4. **[TEST ADDED]** Added test case for unknown content type handling (returns 0 tokens)

### File List

| File | Changes |
|------|---------|
| `internal/token/service.go` | Added CalculateBatch, CalculateEntry, CalculateConversation methods; added encoding/json, log, and types imports; improved error logging for ToolInput marshal |
| `internal/token/service_test.go` | Added 9 new test functions covering all new methods, cache behavior, benchmark tests, unknown content type handling |
| `internal/tui/app.go` | Added tokenService field, initialization in NewAppModel/NewAppModelWithError, cache clear in ConversationSelectedMsg, ConversationSelectedWithWatchMsg, and GoBackMsg handlers |
