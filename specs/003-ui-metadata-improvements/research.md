# Research: UI Decoration and Metadata Improvements

**Date**: 2026-01-13
**Feature**: 003-ui-metadata-improvements

## Research Tasks

### 1. Go Version Flag Best Practices

**Decision**: Use build-time variable injection via `-ldflags`

**Rationale**:
- Standard Go practice for embedding version information
- No runtime overhead - version compiled into binary
- Works with goreleaser for automated releases
- Fallback to git commit hash for development builds

**Implementation Pattern**:
```go
// internal/version/version.go
var (
    Version   = "dev"           // Set via -ldflags
    Commit    = "unknown"       // Set via -ldflags
    BuildDate = "unknown"       // Set via -ldflags
)

func String() string {
    if Version == "dev" {
        return fmt.Sprintf("cclv dev-%s", Commit[:7])
    }
    return fmt.Sprintf("cclv %s", Version)
}
```

**Build Command**:
```bash
go build -ldflags "-X github.com/.../version.Version=v1.0.0 -X github.com/.../version.Commit=$(git rev-parse HEAD)"
```

**Alternatives Considered**:
- Embed file with version: More complex, requires go:embed directive
- Runtime git detection: Adds git dependency, slower startup

---

### 2. Claude Code JSONL Token Usage Structure

**Decision**: Extract `usage` field from assistant message entries

**Rationale**:
- Token data is already present in assistant entries under `message.usage`
- Structure includes `input_tokens`, `output_tokens`, and cache-related fields
- Model name available in `message.model` field

**JSONL Structure** (discovered from actual files):
```json
{
  "type": "assistant",
  "message": {
    "role": "assistant",
    "model": "claude-opus-4-5-20251101",
    "usage": {
      "input_tokens": 3,
      "cache_creation_input_tokens": 8736,
      "cache_read_input_tokens": 12781,
      "output_tokens": 1,
      "service_tier": "standard"
    },
    "content": [...]
  }
}
```

**Token Calculation**:
- Total input = `input_tokens` + `cache_creation_input_tokens` + `cache_read_input_tokens`
- Total output = `output_tokens`
- Total = input + output

**Alternatives Considered**:
- Parse entire file for summary: Too slow for large files
- Cache metadata separately: Adds complexity, violates Principle V

---

### 3. Bubbletea Lazy Loading Patterns

**Decision**: Use viewport-based virtualization with cursor tracking

**Rationale**:
- Bubbles list component supports item delegates for custom rendering
- Load items in batches (e.g., 20 at a time) as user scrolls
- Track visible range and pre-fetch next batch

**Implementation Pattern**:
```go
type LazyList struct {
    items       []Item
    loadedCount int
    totalCount  int
    loading     bool
    batchSize   int
}

func (m LazyList) Update(msg tea.Msg) (LazyList, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if m.needsMoreItems() && !m.loading {
            return m, m.loadNextBatch()
        }
    case ItemsLoadedMsg:
        m.items = append(m.items, msg.items...)
        m.loadedCount += len(msg.items)
        m.loading = false
    }
    return m, nil
}
```

**Threshold Decision** (per spec clarification):
- Conversation list: Lazy load when >50 conversations
- Log viewer: Lazy load when >100 messages

**Alternatives Considered**:
- Load all then virtualize display: Still loads all into memory
- External paging library: Adds dependency, violates Principle V

---

### 4. Lipgloss Border and Decoration Patterns

**Decision**: Use box-drawing characters with lipgloss Border() method

**Rationale**:
- Lipgloss provides built-in border styles (Rounded, Normal, Double, etc.)
- Box-drawing characters are widely supported in modern terminals
- No emoji per spec requirement

**Border Styles Available**:
```go
lipgloss.NormalBorder()   // ┌─┐│└─┘
lipgloss.RoundedBorder()  // ╭─╮│╰─╯
lipgloss.DoubleBorder()   // ╔═╗║╚═╝
lipgloss.ThickBorder()    // ┏━┓┃┗━┛
```

**List Item Separator Pattern**:
```go
// Use horizontal line between items
separator := lipgloss.NewStyle().
    Foreground(dimColor).
    Render("────────────────────")
```

**Header Pattern**:
```go
header := lipgloss.NewStyle().
    Border(lipgloss.NormalBorder(), false, false, true, false). // Bottom border only
    BorderForeground(primaryColor).
    Bold(true).
    Render(fmt.Sprintf("Projects (%d)", count))
```

**Alternatives Considered**:
- ASCII-only borders: Less visually appealing
- Custom Unicode characters: More complex, less portable

---

### 5. Number Formatting with Thousands Separators

**Decision**: Use golang.org/x/text/message for locale-aware formatting

**Rationale**:
- Standard Go approach for number formatting
- Handles locale-specific separators (comma vs period)
- Already in extended standard library

**Implementation**:
```go
import "golang.org/x/text/message"
import "golang.org/x/text/language"

func formatTokenCount(n int) string {
    p := message.NewPrinter(language.English)
    return p.Sprintf("%d", n) // "12,345"
}
```

**Alternative (simpler, no dependency)**:
```go
func formatWithCommas(n int) string {
    s := strconv.Itoa(n)
    if len(s) <= 3 {
        return s
    }
    // Insert commas every 3 digits from right
    var result strings.Builder
    for i, c := range s {
        if i > 0 && (len(s)-i)%3 == 0 {
            result.WriteRune(',')
        }
        result.WriteRune(c)
    }
    return result.String()
}
```

**Decision**: Use simpler custom function to avoid new dependency (Principle V).

---

## Summary

All technical unknowns have been resolved:

| Topic | Decision | Dependency Impact |
|-------|----------|-------------------|
| Version flag | Build-time ldflags | None |
| Token data | Extract from message.usage | None |
| Lazy loading | Viewport-based batching | None |
| Borders | Lipgloss built-in borders | None (existing) |
| Number format | Custom comma function | None |

**No new dependencies required** - all implementations use existing Go standard library and Charm stack.
