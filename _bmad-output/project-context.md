---
project_name: 'claude-code-log-viewer-cli'
user_name: 'Jongkuk Lim'
date: '2026-01-13'
sections_completed: ['technology_stack', 'go_language_rules', 'bubbletea_framework_rules', 'code_quality', 'development_workflow', 'critical_rules', 'testing_rules']
status: 'complete'
rule_count: 47
optimized_for_llm: true
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Technology Stack & Versions

| Technology | Version | Notes |
|------------|---------|-------|
| Go | 1.24.3 | Minimum 1.21+ required (uses generics) |
| Bubbletea | v1.3.10 | TUI framework - Elm Architecture (Model-Update-View) |
| Lipgloss | v1.1.1 | Terminal styling - use for all visual elements |
| Bubbles | v0.21.0 | UI components (list, viewport, textinput) |
| golang.org/x/term | v0.39.0 | Terminal/TTY detection - only in main.go |
| Make | Any | Build system - ALWAYS use Makefile, not raw go commands |

### Version Constraints

- **Charm Stack Coupling**: Bubbletea, Lipgloss, Bubbles must be updated together
- **No New Dependencies**: Constitution V prohibits adding deps beyond Charm stack
- **No Glamour**: Markdown rendering explicitly out of scope
- **No Direct x/ Imports**: Never import `charmbracelet/x/*` packages directly

### Build System Rules

- Use `make build` - injects version via ldflags
- Use `make test` - includes race detection
- Never use raw `go build` or `go test` directly

---

## Go Language Rules

### Package Organization

- **cmd/cclv/**: Entry point only - flag parsing, TTY detection, mode routing
- **internal/**: All business logic - Go visibility enforced, never export
- **No circular imports**: Dependency flows `cmd → tui → parser/scanner → types`

### Import Conventions

```go
import (
    // stdlib first
    "fmt"
    "strings"

    // external deps
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    // internal packages last
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)
```

### Error Handling

- Return errors, never panic (except truly unrecoverable)
- Wrap with context: `fmt.Errorf("failed to parse entry: %w", err)`
- For parse errors: skip and count (`result.ParseErrors++`), don't fail entirely

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | lowercase, underscores | `entry.go`, `projects.go` |
| Packages | single lowercase word | `parser`, `scanner`, `tui` |
| Exported types | PascalCase | `LogEntry`, `ViewerModel` |
| Unexported | camelCase | `parseUserMessage`, `pathExists` |
| Receivers | short lowercase | `m` for model, `e` for entry |
| Constants | PascalCase or UPPER_SNAKE | `EntryTypeUser`, `UserIcon` |

---

## Bubbletea Framework Rules

### Elm Architecture Pattern

Every TUI component MUST implement `tea.Model`:

```go
type MyModel struct { /* state */ }
func (m MyModel) Init() tea.Cmd { return nil }
func (m MyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { /* handle messages */ }
func (m MyModel) View() string { /* render UI */ }
```

### Critical: List Component Height Bug

**The `bubbles/list` component does NOT respect `SetSize()` height.**

```go
// WRONG - list.View() outputs MORE lines than height
m.list.SetSize(width, height-2)
return m.list.View() // May output 74 lines when you set 63!

// CORRECT - manually truncate output
listView := truncateToLines(m.list.View(), listHeight)
return listView
```

See `docs/lessons-learned.md` for full details.

### Message-Based State Updates

- State is immutable - return new model from Update()
- Use custom message types for async operations
- Commands (`tea.Cmd`) for side effects (file I/O, etc.)

```go
// Define message type
type conversationLoadedMsg struct {
    entries []types.LogEntry
    err     error
}

// Return command that produces message
func loadConversation(path string) tea.Cmd {
    return func() tea.Msg {
        result, err := parser.ParseJSONLFile(path)
        return conversationLoadedMsg{entries: result.Entries, err: err}
    }
}
```

### View State Machine

```
viewProjects → [enter/l] → viewConversations → [enter/l] → viewViewer
     ↑                            |                            |
     └────────[h/esc]─────────────┴──────────[h/esc]───────────┘
```

### Styling Rules

- **NO EMOJI** - Use text icons only: `[U]`, `[A]`, `[T]`, `[>]`
- All styles defined in `internal/tui/styles.go`
- Use Lipgloss for ALL visual styling
- Box-drawing characters for borders: `╭`, `╮`, `╰`, `╯`, `│`, `─`

---

## Code Quality & Style Rules

### File Organization

```
internal/tui/
├── app.go           # Root model, view routing
├── project.go       # Project list view
├── conversation.go  # Conversation list view
├── viewer.go        # Log viewer
├── styles.go        # ALL styles, colors, icons, lazy config
├── utils.go         # Formatting helpers
└── plain.go         # Non-TUI plain text output
```

### Function Size

- Keep functions focused - single responsibility
- Extract helpers when logic is reused
- View rendering can be longer but should be readable

### Comments

- Package comments required: `// Package tui provides...`
- Function comments for exported functions
- Inline comments only for non-obvious logic

---

## Development Workflow Rules

### Git Branch Naming

Feature branches: `{number}-{short-description}`
- `001-claude-log-viewer`
- `002-cclv-fixes-enhancements`
- `003-ui-metadata-improvements`

### Build Commands

| Task | Command |
|------|---------|
| Build | `make build` |
| Test | `make test` |
| Format | `make fmt` |
| Lint | `make lint` |
| CI validation | `make ci` |

### Version Injection

Version info injected at build time - never hardcode:

```go
// internal/version/version.go
var Version = "dev"   // Set via -ldflags
var Commit = "unknown"
var BuildDate = "unknown"
```

---

## Critical Don't-Miss Rules

### ABSOLUTE RULES (Never Violate)

1. **NO EMOJI IN UI** - Text icons only (`[U]`, `[A]`, `[T]`, `[>]`) per FR-017
2. **TRUNCATE LIST OUTPUT** - `list.View()` lies about height, always truncate
3. **USE MAKEFILE** - Never raw `go build/test`, version injection required
4. **NO NEW DEPENDENCIES** - Constitution V: Charm stack only

### Lazy Loading Thresholds

| Context | Threshold | Batch Size |
|---------|-----------|------------|
| Conversations | >50 | 20 |
| Log messages | >100 | 20 |

Enable lazy loading when items exceed threshold. Load metadata on-demand as user scrolls.

### Path Decoding Algorithm

Claude Code encodes paths lossily (`/` and `_` both become `-`).

```go
// Use filesystem validation with backtracking
// See scanner.DecodeProjectPath() and scanner.findValidPath()
```

**Never assume simple string replacement works.**

### TTY Detection Flow

```go
// main.go mode detection order:
1. --version flag? → print and exit
2. --plain flag? → Plain Mode
3. --tui flag? → Force TUI
4. stdin is TTY + no args? → Interactive Mode
5. stdout is TTY? → Pipeline TUI Mode
6. else → Pipeline Plain Mode
```

### Content Type Defaults

| Content | Default State | Toggle Key |
|---------|---------------|------------|
| Thinking blocks | Collapsed | `t` |
| Tool inputs | Collapsed | `i` |

### Parser Resilience

- Skip malformed JSONL lines, don't fail
- Track parse errors: `result.ParseErrors`
- Buffer size: 1MB max per line (`scanner.Buffer(buf, 1024*1024)`)

---

## Testing Rules

### Test Pyramid

| Level | Coverage | Location | Run Command |
|-------|----------|----------|-------------|
| Unit | 70% of tests | `*_test.go` alongside source | `make test` |
| Integration | 20% of tests | `tests/integration/` | `make test-integration` |
| E2E | 10% of tests | `tests/e2e/` | `make test-e2e` |

### Coverage Requirements

- **Minimum**: 90% overall (CI gate - hard fail)
- **Package Targets**:
  - `parser/`: 95%
  - `scanner/`: 90%
  - `types/`: 95%
  - `tui/` (excluding View): 90%
  - `cmd/cclv/`: 85%

### Coverage Exclusions

Exclude from coverage calculation:
- `View()` methods - visual rendering tested manually
- `main()` - tested via E2E instead

### Test Patterns

**Table-Driven Tests (Required):**
```go
func TestX(t *testing.T) {
    tests := []struct {
        name    string
        input   X
        want    Y
        wantErr bool
    }{ /* cases */ }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { /* test */ })
    }
}
```

**Test File Organization:**
```
internal/parser/
├── entry.go
├── entry_test.go
├── jsonl.go
└── jsonl_test.go

tests/
├── integration/       # Cross-package tests
├── e2e/              # CLI behavior tests
└── testdata/
    ├── fixtures/     # Sample JSONL files
    └── golden/       # Expected output files
```

### Testability Patterns

- Extract interfaces for external dependencies (filesystem, clock)
- Use `testdata/fixtures/` for sample input files
- Golden file tests for CLI output verification
- Build tags for slow tests: `//go:build integration`

### Test Helpers

Create `internal/testutil/` package:
- `MustParseEntry(t, json) LogEntry` - test helper
- `TempProjectDir(t) string` - creates mock project structure
- `GoldenFile(t, name, got)` - golden file comparison

### Running Tests

```bash
make test              # Unit + race detection + coverage
make test-short        # Fast subset
make test-integration  # Integration tests
make test-e2e          # End-to-end CLI tests
make test-all          # Everything
make coverage          # HTML coverage report
```

### CI Requirements

1. All tests must pass
2. Coverage >= 90% (hard gate)
3. Race detector clean
4. Zero flaky tests (zero tolerance)

### Quality Over Quantity

AI agents MUST NOT write meaningless tests to hit coverage:
- ❌ Testing struct field assignment
- ❌ Testing simple getters
- ✅ Testing behavior and edge cases
- ✅ Testing error conditions
- ✅ Testing state transitions

### What to Test

| Package | Priority | Focus Areas |
|---------|----------|-------------|
| `parser` | Critical | All entry types, malformed input, edge cases |
| `scanner` | Critical | Path decoding, disambiguation, lazy loading |
| `types` | High | TokenUsage methods, struct behavior |
| `tui` | Medium | Update() logic, state transitions (not View()) |
| `version` | Low | String() and Full() output |
| `cmd/cclv` | High | Flag parsing, mode detection |

---

## Usage Guidelines

**For AI Agents:**

- Read this file BEFORE implementing any code
- Follow ALL rules exactly as documented
- When in doubt, prefer the more restrictive option
- Flag any conflicts between this file and task requirements

**For Humans:**

- Keep this file lean and focused on agent needs
- Update when technology stack changes
- Review quarterly for outdated rules
- Remove rules that become obvious over time

---

_Last Updated: 2026-01-13_

