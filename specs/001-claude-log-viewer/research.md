# Research: Claude Code Log Viewer CLI

**Phase**: 0 - Research
**Date**: 2026-01-12

## Overview

This document consolidates research findings for implementing the CCLV application. All technical choices are pre-defined by the constitution; this research focuses on implementation patterns and best practices.

---

## 1. Bubbletea Multi-View Navigation Pattern

### Decision
Use a **state machine pattern** with a root model that delegates to child models based on current view state.

### Rationale
- Bubbletea's Elm architecture requires a single root `Model` with `Update` and `View` methods
- Multi-view apps use an enum-based state to route messages to the appropriate child model
- This pattern is well-documented in Charm's examples (e.g., `bubbletea-examples/multi-view`)

### Implementation Pattern
```go
type viewState int

const (
    viewProjects viewState = iota
    viewConversations
    viewLog
)

type model struct {
    state        viewState
    projects     projectModel
    conversations conversationModel
    viewer       viewerModel
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch m.state {
    case viewProjects:
        return m.projects.Update(msg)
    // ... delegate to appropriate child
    }
}
```

### Alternatives Considered
- **Separate programs per view**: Rejected - violates single binary principle, poor UX
- **Global message bus**: Rejected - over-engineering for 3 views

---

## 2. JSONL Streaming Parser for Large Files

### Decision
Use **line-by-line buffered reading** with `bufio.Scanner`, parsing each line independently.

### Rationale
- JSONL format is one JSON object per line - no need for streaming JSON parser
- `bufio.Scanner` handles line buffering efficiently
- Memory usage stays O(line_size) not O(file_size)
- Supports streaming from stdin (tail -f compatibility)

### Implementation Pattern
```go
func ParseJSONL(r io.Reader) <-chan LogEntry {
    entries := make(chan LogEntry)
    go func() {
        defer close(entries)
        scanner := bufio.NewScanner(r)
        // Increase buffer for long lines (tool inputs can be large)
        scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
        for scanner.Scan() {
            var entry LogEntry
            if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
                continue // Skip malformed lines per spec
            }
            entries <- entry
        }
    }()
    return entries
}
```

### Alternatives Considered
- **Load entire file into memory**: Rejected - fails 100MB file requirement
- **Memory-mapped file**: Rejected - over-engineering, doesn't work with stdin

---

## 3. Large File Viewport Handling

### Decision
Use **Bubbles viewport** with lazy rendering - only render visible lines plus buffer.

### Rationale
- Bubbles `viewport` widget handles scrolling, vim keys (gg, G), and resize
- For large files, store parsed entries in a slice but only render visible portion
- Viewport manages scroll position; we provide content on demand

### Implementation Pattern
```go
type viewerModel struct {
    viewport viewport.Model
    entries  []LogEntry  // All parsed entries
    rendered string      // Pre-rendered visible content
}

func (m *viewerModel) updateContent() {
    // Only render entries[viewport.YOffset : viewport.YOffset + viewport.Height]
    // This keeps rendering fast even with 10k+ entries
}
```

### Alternatives Considered
- **Virtual scrolling**: Investigated - Bubbles viewport already handles this
- **Pagination**: Rejected - breaks vim navigation expectations (gg should go to actual top)

---

## 4. Project Directory Name Decoding

### Decision
Decode project paths by replacing `-` with `/`, then extract last path component(s).

### Rationale
- Claude Code encodes paths as `-Users-limjk-GitHub-...`
- Simple string replacement restores original path
- Display last component (`swealog`), disambiguate with parent if collision

### Implementation Pattern
```go
func DecodeProjectPath(encodedName string) string {
    // "-Users-limjk-GitHub-foo" -> "/Users/limjk/GitHub/foo"
    if strings.HasPrefix(encodedName, "-") {
        return strings.ReplaceAll(encodedName, "-", "/")
    }
    return encodedName
}

func DisplayName(projects []Project) {
    // Group by last component, add parent components for collisions
    lastComponents := make(map[string][]Project)
    for _, p := range projects {
        last := filepath.Base(p.DecodedPath)
        lastComponents[last] = append(lastComponents[last], p)
    }
    for name, ps := range lastComponents {
        if len(ps) > 1 {
            // Disambiguate: show parent/name
            for _, p := range ps {
                p.DisplayName = filepath.Join(filepath.Base(filepath.Dir(p.DecodedPath)), name)
            }
        }
    }
}
```

### Alternatives Considered
- **Show full path**: Rejected - too long for TUI list display
- **Use directory creation date**: Rejected - unreliable, not always available

---

## 5. TTY Detection for Mode Selection

### Decision
Use `term.IsTerminal(os.Stdin.Fd())` from `golang.org/x/term`.

### Rationale
- Standard Go approach for TTY detection
- `golang.org/x/term` is part of the extended standard library (acceptable per constitution)
- Clear boolean result: TTY = interactive mode, non-TTY = pipeline mode

### Implementation Pattern
```go
import "golang.org/x/term"

func main() {
    if term.IsTerminal(int(os.Stdin.Fd())) && len(os.Args) == 1 {
        runInteractiveMode()
    } else {
        runPipelineMode()
    }
}
```

### Alternatives Considered
- **Check for -i flag**: Rejected - adds unnecessary flag, not Unix-idiomatic
- **Environment variable**: Rejected - over-engineering

---

## 6. Vim Key Handling in Bubbletea

### Decision
Use **key sequence detection** for multi-key commands (`gg`, `G`).

### Rationale
- Bubbletea provides `tea.KeyMsg` for individual keypresses
- Multi-key sequences (gg) require tracking previous key + timeout
- Single keys (j, k, /, n) can be handled directly

### Implementation Pattern
```go
type viewerModel struct {
    lastKey  string
    lastTime time.Time
}

func (m viewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        key := msg.String()
        if m.lastKey == "g" && key == "g" && time.Since(m.lastTime) < 500*time.Millisecond {
            // Handle gg - go to top
            m.viewport.GotoTop()
            m.lastKey = ""
        } else if key == "G" {
            // Handle G - go to bottom
            m.viewport.GotoBottom()
        } else if key == "g" {
            m.lastKey = "g"
            m.lastTime = time.Now()
        }
        // ... other keys
    }
}
```

### Alternatives Considered
- **Vim mode library**: None found for Bubbletea; would add dependency
- **Single-key only**: Rejected - gg is essential for vim users

---

## 7. Collapsible Thinking/Tool Blocks

### Decision
Use **inline toggle state** per entry type, global toggle keys (`t`, `i`).

### Rationale
- Global toggle is simpler than per-block toggle (click not available in TUI)
- Matches spec: `t` toggles all thinking blocks, `i` toggles all tool inputs
- State is boolean flag on viewer model, re-render on toggle

### Implementation Pattern
```go
type viewerModel struct {
    showThinking bool  // Default: false per spec
    showToolInputs bool // Default: false per spec
}

func (m viewerModel) renderEntry(e LogEntry) string {
    switch e.ContentType {
    case "thinking":
        if !m.showThinking {
            return "[thinking collapsed - press 't' to expand]"
        }
        return renderThinking(e.Content)
    case "tool_use":
        header := fmt.Sprintf("Tool: %s", e.ToolName)
        if !m.showToolInputs {
            return header + " [inputs collapsed - press 'i' to expand]"
        }
        return header + "\n" + truncate(e.Input, 200)
    }
}
```

### Alternatives Considered
- **Per-block toggle**: Rejected - no mouse, would need numbered shortcuts
- **Always show everything**: Rejected - clutters view, user requested collapse

---

## Summary

All research items resolved. No NEEDS CLARIFICATION items remain.

| Topic | Decision | Key Dependency |
|-------|----------|----------------|
| Multi-view navigation | State machine with child models | bubbletea |
| JSONL parsing | Line-by-line bufio.Scanner | encoding/json |
| Large file handling | Bubbles viewport with lazy render | bubbles/viewport |
| Path decoding | String replacement + collision detection | filepath |
| TTY detection | term.IsTerminal | golang.org/x/term |
| Vim keys | Sequence detection with timeout | bubbletea tea.KeyMsg |
| Collapsible blocks | Global toggle state, re-render | bubbletea |

**Next Step**: Proceed to Phase 1 - Data Model and Contracts
