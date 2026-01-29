# claude-code-log-viewer-cli Development Guidelines

Quick reference for AI agents implementing features in cclv (Claude Code Log Viewer).
For detailed guidance, see `_bmad-output/project-context.md` and `docs/lessons-learned.md`.

---

## Technology Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.24.3+ | Language (minimum 1.21 for generics) |
| Bubbletea | v1.3.10 | TUI Framework (Elm Architecture) |
| Lipgloss | v1.1.1 | Terminal styling |
| Bubbles | v0.21.0 | UI components (list, viewport, textinput) |
| Glamour | - | Markdown rendering |
| fsnotify | v1.9.0 | File watching for live mode |

**Key:** Charm stack (Bubbletea, Lipgloss, Bubbles) must be updated together.

---

## Project Structure

```
cmd/cclv/              # CLI entry point only (flag parsing, TTY detection, mode routing)
internal/
  ├── parser/          # JSONL parsing logic
  ├── scanner/         # Project/conversation discovery, birthtime detection
  ├── tui/             # Bubbletea components (app, project, conversation, viewer, dashboard)
  ├── types/           # Core data structures (LogEntry, Conversation, Project)
  ├── token/           # Token usage calculations
  ├── usage/           # Claude API usage analytics
  ├── version/         # Version info (injected at build time)
  └── watcher/         # File/project watching for live mode
```

---

## Build System (CRITICAL)

**Always use Makefile, never raw go commands:**

```bash
make build      # Build for current platform (injects version via ldflags)
make test       # Run with race detection + coverage
make fmt        # Format code
make lint       # Run linter (errcheck, etc.)
make ci         # Full CI validation - MUST PASS before work is complete
```

**CRITICAL:** `make ci` MUST pass before any code changes are considered complete.

Version is injected at build time via `-ldflags`. Never hardcode version strings.

---

## Go Coding Rules

### Package Organization
- `cmd/cclv/`: CLI entry point, TTY detection, mode routing ONLY
- `internal/`: All business logic (Go visibility enforced, never export)
- No circular imports: `cmd → tui → parser/scanner → types`

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | lowercase, underscores | `entry.go`, `projects.go` |
| Packages | single lowercase word | `parser`, `scanner`, `tui` |
| Exported types | PascalCase | `LogEntry`, `ViewerModel` |
| Unexported | camelCase | `parseUserMessage` |
| Receivers | short lowercase | `m` (model), `e` (entry) |

### Error Handling
- Return errors, never panic (except truly unrecoverable)
- Wrap with context: `fmt.Errorf("failed to X: %w", err)`
- Parser: skip malformed lines and count errors (`result.ParseErrors++`), don't fail entirely

---

## Bubbletea Framework Rules

### Elm Architecture (Required for All Components)

```go
type MyModel struct { /* state */ }
func (m MyModel) Init() tea.Cmd { return nil }
func (m MyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { /* handle messages */ }
func (m MyModel) View() string { /* render UI */ }
```

### CRITICAL BUG: List Component Height

**The bubbles/list component does NOT respect SetSize() height. It outputs MORE lines than specified.**

```go
// WRONG - list.View() outputs MORE lines than height
m.list.SetSize(width, height-2)
return m.list.View()

// CORRECT - manually truncate output
listView := truncateToLines(m.list.View(), listHeight)
return listView
```

See `docs/lessons-learned.md` for full details.

### ViewerModel Constructor Pattern

ViewerModel uses constructor chaining - specialized constructors call the base constructor:

```go
// Base constructor - full initialization
func NewViewerModel(entries, parseErrors, title, opts, tokenSvc) ViewerModel

// Specialized - adds back navigation
func NewViewerModelWithBack(...) ViewerModel {
    m := NewViewerModel(...)  // Always call base first
    m.canGoBack = true        // Add specialization only
    return m
}
```

**Rule:** Always add new ViewerModel features to `NewViewerModel`. Specialized constructors only set flags.

---

## UI/Styling Rules

- **NO EMOJI** - Use text icons only: `[U]`, `[A]`, `[T]`, `[>]`
- All styles defined in `internal/tui/styles.go`
- Use Lipgloss for ALL visual styling
- Box-drawing characters for borders: `╭`, `╮`, `╰`, `╯`, `│`, `─`

---

## Absolute Rules (Never Violate)

1. **NO EMOJI IN UI** - Text icons only (`[U]`, `[A]`, `[T]`, `[>]`) per FR-017
2. **TRUNCATE LIST OUTPUT** - `list.View()` lies about height, always truncate
3. **USE MAKEFILE** - Never raw `go build/test`, version injection required
4. **NO NEW DEPENDENCIES** - Charm stack only (constitution approved)
5. **CLI SMOKE TEST** - For CLI flag changes, test against real environment
6. **MAKE CI MUST PASS** - Code is not complete until `make ci` passes successfully
7. **Birthtime for Latest** - Use `CreationTime` (not `LastModified`) for "latest conversation"
8. **FD Cleanup Pattern** - Remove watched paths before Close() (macOS kqueue leak prevention)

---

## TTY Detection Flow

Mode detection order in `cmd/cclv/main.go`:

1. `--version` flag? → print and exit
2. `--plain` flag? → Plain text mode
3. `--tui` flag? → Force TUI mode
4. stdin is TTY + no args? → Interactive mode (dashboard)
5. stdout is TTY? → Pipeline TUI mode
6. else → Pipeline plain text mode

---

## Lazy Loading Rules

| Context | Threshold | Batch Size |
|---------|-----------|------------|
| Conversations | >50 | 20 |
| Log messages | >100 | 20 |

**When adding new view modes:** Each mode needs its own count tracking, scroll triggers, and position tracking.

---

## Testing Requirements

### Coverage Targets (CI Hard Gate: 90% minimum)

| Package | Target |
|---------|--------|
| `parser/` | 95% |
| `scanner/` | 90% |
| `types/` | 95% |
| `tui/` (excluding View()) | 90% |
| `cmd/cclv/` | 85% |

### Test Locations

```
*_test.go                # Unit tests alongside source
tests/integration/       # Cross-package tests
tests/e2e/               # CLI behavior tests
tests/testdata/fixtures/ # Sample input files
```

### Test Commands

```bash
make test              # Unit + race detection + coverage
make test-short        # Fast subset
make coverage          # HTML report
```

Exclude `View()` methods and `main()` from coverage calculations.

---

## Development Workflow

### Git Branch Naming

Format: `{number}-{short-description}`
Examples: `001-claude-log-viewer`, `011-live-updates`

### Required Before Completion

1. `make ci` passes (all checks)
2. Coverage >= 90%
3. Race detector clean
4. All tests pass

---

## Quick Links

- **Detailed rules:** `_bmad-output/project-context.md`
- **Lessons learned:** `docs/lessons-learned.md`
- **Sprint status:** `_bmad-output/implementation-artifacts/sprint-status.yaml`

---

*Last Updated: 2026-01-30*
