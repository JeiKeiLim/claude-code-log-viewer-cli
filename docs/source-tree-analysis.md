# Source Tree Analysis: cclv

**Generated**: 2026-01-13 | **Scan Level**: Exhaustive

## Directory Structure

```
claude-code-log-viewer-cli/
├── cmd/
│   └── cclv/
│       └── main.go              # Entry point, mode detection, flag parsing
│
├── internal/
│   ├── parser/                  # JSONL parsing
│   │   ├── entry.go             # Single entry parsing, message type handling
│   │   └── jsonl.go             # Batch/stream parsing, file reading
│   │
│   ├── scanner/                 # Project discovery
│   │   └── projects.go          # Directory scanning, path decoding, metadata
│   │
│   ├── tui/                     # Terminal UI components
│   │   ├── app.go               # Root model, view state routing
│   │   ├── conversation.go      # Conversation list view
│   │   ├── plain.go             # Plain text rendering (non-TUI)
│   │   ├── project.go           # Project list view
│   │   ├── styles.go            # Lipgloss styles, colors, icons
│   │   ├── utils.go             # Formatting helpers, border drawing
│   │   └── viewer.go            # Log viewer with search, toggles
│   │
│   ├── types/                   # Core data types
│   │   ├── conversation.go      # Conversation struct
│   │   ├── entry.go             # LogEntry, Message, TokenUsage
│   │   └── project.go           # Project struct
│   │
│   └── version/                 # Version information
│       └── version.go           # Build-time version injection
│
├── docs/                        # Generated documentation (output folder)
│   ├── index.md                 # Documentation index
│   ├── project-overview.md      # Project summary
│   ├── architecture.md          # Architecture documentation
│   ├── source-tree-analysis.md  # This file
│   ├── development-guide.md     # Development instructions
│   ├── lessons-learned.md       # Technical notes (existing)
│   └── project-scan-report.json # Workflow state file
│
├── specs/                       # Speckit feature specifications
│   ├── 001-claude-log-viewer/   # Initial implementation spec
│   │   ├── spec.md              # Feature specification
│   │   ├── plan.md              # Implementation plan
│   │   ├── research.md          # Research notes
│   │   ├── data-model.md        # Data model design
│   │   ├── quickstart.md        # Quick start guide
│   │   ├── tasks.md             # Implementation tasks
│   │   └── checklists/
│   │       └── requirements.md  # Requirements checklist
│   │
│   ├── 002-cclv-fixes-enhancements/  # Bug fixes spec
│   │   └── [same structure]
│   │
│   └── 003-ui-metadata-improvements/ # Current feature spec
│       └── [same structure]
│
├── .claude/                     # Claude Code settings
│   └── commands/
│       └── bmad/                # BMAD commands (if installed)
│
├── _bmad/                       # BMAD installation
│   ├── core/                    # Core BMAD components
│   └── bmm/                     # BMM module
│
├── .github/
│   └── workflows/               # GitHub Actions (if any)
│
├── .specify/                    # Speckit configuration
│
├── cclv                         # Built binary (gitignored)
├── coverage.out                 # Test coverage output
├── CLAUDE.md                    # Development guidelines
├── go.mod                       # Go module definition
├── go.sum                       # Go module checksums
├── Makefile                     # Build automation
└── README.md                    # Project readme
```

## Critical Directories

### cmd/cclv/ (Entry Point)

**Purpose**: Application entry point and mode routing

| File | LOC | Responsibility |
|------|-----|----------------|
| main.go | 165 | Flag parsing, TTY detection, mode routing |

Key entry points:
- `main()` - Application start
- `runInteractiveMode()` - Launch project browser
- `runPipelineMode()` - Handle stdin/file input

### internal/parser/ (JSONL Parsing)

**Purpose**: Parse Claude Code JSONL log format

| File | LOC | Responsibility |
|------|-----|----------------|
| entry.go | 121 | Single entry parsing, user/assistant handling |
| jsonl.go | 106 | Batch parsing, stream parsing, file I/O |

Data flow: `JSONL bytes → RawLogEntry → LogEntry → Message/Content`

### internal/scanner/ (Project Discovery)

**Purpose**: Find and decode Claude Code projects

| File | LOC | Responsibility |
|------|-----|----------------|
| projects.go | 403 | Project scanning, path decoding, metadata extraction |

Key algorithms:
- Path decoding with filesystem validation
- Display name disambiguation
- Lazy metadata loading

### internal/tui/ (Terminal UI)

**Purpose**: All TUI components using Bubbletea

| File | LOC | Responsibility |
|------|-----|----------------|
| app.go | 261 | Root model, view state machine |
| project.go | 300 | Project list, filtering |
| conversation.go | 327 | Conversation list, lazy loading |
| viewer.go | 551 | Log viewer, search, toggles |
| styles.go | 229 | Colors, styles, icons, lazy config |
| utils.go | 163 | Formatting, borders, truncation |
| plain.go | 108 | Plain text output rendering |

View hierarchy: `AppModel` → `ProjectModel` / `ConversationModel` / `ViewerModel`

### internal/types/ (Data Types)

**Purpose**: Core domain models

| File | LOC | Responsibility |
|------|-----|----------------|
| entry.go | 127 | LogEntry, Message, MessageContent, TokenUsage |
| conversation.go | 25 | Conversation metadata |
| project.go | 15 | Project metadata |

### internal/version/ (Version Info)

**Purpose**: Build-time version injection

| File | LOC | Responsibility |
|------|-----|----------------|
| version.go | 35 | Version, Commit, BuildDate variables |

## File Statistics

| Category | Count | Total LOC |
|----------|-------|-----------|
| Go source files | 15 | ~2,400 |
| Packages | 6 | - |
| Spec files (md) | ~20 | - |
| Config files | 4 | - |

## Integration Points

### External Dependencies

| Package | Purpose |
|---------|---------|
| github.com/charmbracelet/bubbletea | TUI framework |
| github.com/charmbracelet/lipgloss | Styling |
| github.com/charmbracelet/bubbles | UI components (list, viewport, textinput) |
| golang.org/x/term | Terminal detection |

### File System Access

| Path | Purpose | Access |
|------|---------|--------|
| `~/.claude/projects/` | Claude Code project storage | Read |
| `*.jsonl` | Conversation log files | Read |
| stdin | Pipeline input | Read |
| stdout | Plain mode output | Write |

### No Network Access

This is a local-only tool with no network dependencies.
