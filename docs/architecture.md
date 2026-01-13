# Architecture Documentation: cclv

**Generated**: 2026-01-13 | **Scan Level**: Exhaustive

## Overview

cclv follows a standard Go CLI architecture with clear separation of concerns across internal packages. The application uses the Elm Architecture pattern via Bubbletea for its TUI components.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         cmd/cclv/main.go                        │
│                    (Entry Point & Mode Detection)               │
└─────────────────────────────┬───────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│  Interactive  │    │   Pipeline    │    │    Plain      │
│     Mode      │    │     Mode      │    │     Mode      │
│  (TUI/App)    │    │  (TUI/Viewer) │    │  (stdout)     │
└───────┬───────┘    └───────┬───────┘    └───────┬───────┘
        │                    │                    │
        └────────────────────┼────────────────────┘
                             ▼
                  ┌─────────────────────┐
                  │   internal/tui/     │
                  │   (Bubbletea TUI)   │
                  └──────────┬──────────┘
                             │
        ┌────────────────────┼────────────────────┐
        ▼                    ▼                    ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│internal/parser│    │internal/scanner│   │internal/types │
│  (JSONL)      │    │  (Projects)   │    │  (Data Types) │
└───────────────┘    └───────────────┘    └───────────────┘
```

## Package Structure

### cmd/cclv (Entry Point)

**File**: `cmd/cclv/main.go` (165 lines)

Responsibilities:
- Parse command-line flags (`--plain`, `--tui`, `--version`, `-v`)
- Detect TTY status for stdin/stdout
- Route to appropriate mode (Interactive, Pipeline, Plain)
- Initialize Bubbletea program with alternate screen buffer

Mode Detection Flow:
```
1. --version flag? → Print version, exit
2. --plain flag? → Plain Mode
3. --tui flag? → TUI Mode
4. stdin is TTY + no args → Interactive Mode (TUI)
5. Otherwise → Pipeline Mode: stdout is TTY? → TUI, else Plain
```

### internal/types (Data Types)

**Files**: `entry.go`, `conversation.go`, `project.go`

Core domain types representing Claude Code log data:

| Type | Description | Key Fields |
|------|-------------|------------|
| `LogEntry` | Single JSONL line | Type, UUID, ParentUUID, Timestamp, Message, Model, Usage |
| `Message` | Message content | Role, Content[], TextContent |
| `MessageContent` | Content block | Type (text/thinking/tool_use), Text, ToolName, ToolInput |
| `TokenUsage` | Token statistics | InputTokens, OutputTokens, CacheCreationInputTokens, CacheReadInputTokens |
| `Conversation` | JSONL file metadata | FilePath, LastModified, MessageCount, TotalTokens, Model, Duration |
| `Project` | Project directory | EncodedName, DecodedPath, DisplayName, DirPath |

### internal/parser (JSONL Parsing)

**Files**: `jsonl.go`, `entry.go`

Responsibilities:
- Line-by-line JSONL parsing with error tolerance
- Type-specific message parsing (user vs assistant)
- Token usage extraction from assistant messages
- Streaming parser for large files

Key Functions:
- `ParseJSONL(io.Reader) ParseResult` - Batch parsing
- `ParseJSONLFile(string) (ParseResult, error)` - File parsing
- `ParseJSONLStream(io.Reader)` - Streaming channel-based parsing
- `ParseEntry([]byte) (LogEntry, error)` - Single line parsing

### internal/scanner (Project Discovery)

**File**: `projects.go` (403 lines)

Responsibilities:
- Scan `~/.claude/projects/` for project directories
- Decode project paths (handle hyphens, underscores, special chars)
- Assign display names with collision disambiguation
- Extract conversation metadata (lazy loading support)

Path Decoding Algorithm:
```
1. Replace -- with placeholder (for underscore sequences)
2. Replace - with /
3. Replace placeholder with _ or -
4. Validate against filesystem
5. Use backtracking to find valid path
```

Key Functions:
- `ScanProjects(string) ([]Project, error)`
- `DecodeProjectPath(string) string`
- `ScanConversations(string) ([]Conversation, error)`
- `ScanConversationsLazy(string) ([]Conversation, error)`
- `ExtractConversationMetadataBatch([]Conversation, int, int)`

### internal/tui (Terminal UI)

**Files**: `app.go`, `project.go`, `conversation.go`, `viewer.go`, `styles.go`, `utils.go`, `plain.go`

Implements Bubbletea's Elm Architecture (Model-Update-View):

| Component | Model | Description |
|-----------|-------|-------------|
| AppModel | `app.go` | Root model, view state routing |
| ProjectModel | `project.go` | Project list with filtering |
| ConversationModel | `conversation.go` | Conversation list with lazy loading |
| ViewerModel | `viewer.go` | Log viewer with search, toggles |

View State Machine:
```
viewProjects → [select] → viewConversations → [select] → viewViewer
     ↑                          |                            |
     └────────[back]────────────┴────────────[back]──────────┘
```

Styling System (`styles.go`):
- Color palette: Purple (primary), Green (secondary), Amber (accent)
- Message styles: User (blue), Assistant (green), Thinking (purple), Tool (amber)
- Text-based icons: `[U]`, `[A]`, `[T]`, `[>]` (no emoji per FR-017)

Lazy Loading Config:
- Conversation threshold: 50 items
- Message threshold: 100 items
- Batch size: 20 items

### internal/version (Version Info)

**File**: `version.go` (35 lines)

Build-time version injection via ldflags:
- `Version` - Semantic version (e.g., "v1.0.0") or "dev"
- `Commit` - Git commit hash
- `BuildDate` - Build timestamp

Output formats:
- `String()` → "cclv v1.0.0" or "cclv dev-abc1234"
- `Full()` → "cclv v1.0.0 (commit: abc1234, built: 2026-01-13)"

## Data Flow

### Interactive Mode
```
main.go → scanner.ScanProjects() → AppModel
                                      │
    ┌─────────────────────────────────┤
    ▼                                 ▼
ProjectModel ──[select]──► ConversationModel ──[select]──► ViewerModel
    │                           │                              │
scanner.ScanProjects()    scanner.ScanConversations()    parser.ParseJSONLFile()
```

### Pipeline Mode
```
main.go → parser.ParseJSONL(stdin/file) → ViewerModel or RenderPlain()
```

## Key Design Decisions

### 1. Dual-Mode Architecture
The application supports both interactive TUI and pipeline modes, determined by TTY detection and flags. This enables both browsing and scripting use cases.

### 2. Lazy Loading
Large conversation lists (>50) and log views (>100 messages) use progressive loading to maintain responsiveness. Metadata is extracted on-demand as users scroll.

### 3. Path Decoding with Validation
Claude Code's path encoding is lossy (both `/` and `_` become `-`). The scanner uses filesystem validation with backtracking to reconstruct the original path.

### 4. Charm Stack
All TUI components use the Charm ecosystem (Bubbletea, Lipgloss, Bubbles) for consistent styling and the Elm Architecture pattern. This provides:
- Immutable state management
- Declarative rendering
- Message-based updates

### 5. Text-Based Icons
Per FR-017, all icons use ASCII/Unicode text (`[U]`, `[A]`, etc.) instead of emoji for terminal compatibility.

## Error Handling

| Scenario | Handling |
|----------|----------|
| Projects directory not found | Show error in TUI with help text |
| No conversations in project | Show empty state message |
| Malformed JSONL lines | Skip and track parse errors |
| Large files (>100MB) | Lazy loading with progressive render |
| Terminal resize | Reflow and maintain scroll position |

## Performance Characteristics

| Metric | Target | Implementation |
|--------|--------|----------------|
| Startup time | <100ms | Single binary, lazy loading |
| Navigation | <50ms | Bubbletea message handling |
| Scrolling | 60fps | Viewport component |
| Memory | <100MB | Lazy loading, streaming parser |
| File size | 100MB | Streaming parser, batch rendering |
