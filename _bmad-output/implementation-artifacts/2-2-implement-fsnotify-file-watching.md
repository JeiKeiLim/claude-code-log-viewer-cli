# Story 2.2: Implement fsnotify File Watching

Status: done

## Story

As a **developer in watch mode**,
I want **new log entries to appear automatically**,
So that **I can monitor Claude's work in real-time**.

## Acceptance Criteria

### AC 2.2.1: fsnotify watcher setup
- **Given** watch mode is enabled (via --watch or --live flag)
- **When** cclv starts and loads a file
- **Then** fsnotify watcher is created for the log file
- **And** watcher runs in a managed goroutine via `tea.Cmd`

### AC 2.2.2: File change detection
- **Given** the watcher is running
- **When** the log file is modified (fsnotify.Write event)
- **Then** new content is read and parsed
- **And** only newly appended entries are processed (tail-read from last position)

### AC 2.2.3: Channel to Bubbletea
- **Given** new entries are parsed
- **When** they are ready for display
- **Then** they are sent via custom message type to Bubbletea Update()
- **And** `tea.Cmd` chaining pattern is used (never raw goroutines)

### AC 2.2.4: File truncation handling
- **Given** the log file is truncated (new session started)
- **When** detected by watcher (file size smaller than last read position)
- **Then** entry list is cleared and file is reloaded from beginning
- **And** no crash or error occurs

### AC 2.2.5: Watcher cleanup
- **Given** the user quits cclv (q, Ctrl+C)
- **When** the application exits
- **Then** fsnotify watcher is properly closed
- **And** no goroutine leaks occur

## Tasks / Subtasks

- [x] Task 1: Add fsnotify dependency (AC: 2.2.1)
  - [x] 1.1: Add `github.com/fsnotify/fsnotify v1.8.0` to go.mod
  - [x] 1.2: Run `go mod tidy` to fetch dependency
  - [x] 1.3: Verify `make build` succeeds
  - [x] 1.4: Update project-context.md Approved Dependencies table (see Dev Notes)

- [x] Task 2: Create watcher package (AC: 2.2.1, 2.2.2, 2.2.3)
  - [x] 2.1: Create `internal/watcher/watcher.go`
  - [x] 2.2: Define `Watcher` struct with fsnotify.Watcher, file path, last read position, mutex
  - [x] 2.3: Define `ErrFileTruncated` sentinel error
  - [x] 2.4: Implement `New(filePath string) (*Watcher, error)` constructor
  - [x] 2.5: Implement `WaitForEvent() tea.Cmd` that returns cmd to listen for single event
  - [x] 2.6: Implement `readNewEntries() ([]types.LogEntry, error)` that reads from last position
  - [x] 2.7: Implement `Close() error` for cleanup

- [x] Task 3: Define message types for Bubbletea integration (AC: 2.2.3)
  - [x] 3.1: Create `internal/watcher/messages.go`
  - [x] 3.2: Define `NewEntriesMsg` struct with entries slice
  - [x] 3.3: Define `WatcherErrorMsg` struct with error field
  - [x] 3.4: Define `FileResetMsg` struct for truncation events (AC: 2.2.4)

- [x] Task 4: Implement file change detection (AC: 2.2.2)
  - [x] 4.1: In `WaitForEvent()`, create command that blocks on Events/Errors channels
  - [x] 4.2: On `fsnotify.Write` event, call `readNewEntries()`
  - [x] 4.3: Track file offset using `lastReadPos int64` field with mutex protection
  - [x] 4.4: Use `file.Seek(lastReadPos, io.SeekStart)` to position at last read
  - [x] 4.5: Parse new lines with `parser.ParseJSONL(file)`
  - [x] 4.6: Update `lastReadPos` after successful read using `file.Seek(0, io.SeekCurrent)`
  - [x] 4.7: Return `NewEntriesMsg` with parsed entries

- [x] Task 5: Implement truncation detection (AC: 2.2.4)
  - [x] 5.1: On each write event, check file size via `os.Stat` before reading
  - [x] 5.2: If file size < lastReadPos, file was truncated
  - [x] 5.3: Reset lastReadPos to 0
  - [x] 5.4: Return `FileResetMsg` to signal full reload needed

- [x] Task 6: Integrate watcher into ViewerModel (AC: 2.2.1, 2.2.3, 2.2.5)
  - [x] 6.1: Add `watcher *watcher.Watcher` field to ViewerModel
  - [x] 6.2: Add `FilePath string` field to RenderOptions struct
  - [x] 6.3: Update `NewViewerModel` to accept filePath from opts, create watcher if watchMode
  - [x] 6.4: In `Init()`, return `watcher.WaitForEvent()` cmd if watcher exists
  - [x] 6.5: Handle `NewEntriesMsg` in `Update()`: append entries, updateContent()
  - [x] 6.6: Handle `FileResetMsg` in `Update()`: clear entries, reload full file
  - [x] 6.7: Handle `WatcherErrorMsg` in `Update()`: show error in status bar

- [x] Task 7: Update main.go to pass file path (AC: 2.2.1)
  - [x] 7.1: Set `opts.FilePath = args[0]` when file argument provided in runPipelineMode
  - [x] 7.2: Ensure file path is absolute (use filepath.Abs if needed)

- [x] Task 8: Implement cleanup on quit (AC: 2.2.5)
  - [x] 8.1: Handle "q" and "ctrl+c" to close watcher before `tea.Quit`
  - [x] 8.2: In ViewerModel quit handler, check if watcher exists and call Close()
  - [x] 8.3: Ensure watcher.Close() is idempotent (safe to call multiple times)

- [x] Task 9: Add tests (all ACs)
  - [x] 9.1: Add unit tests for watcher package in `internal/watcher/watcher_test.go`
  - [x] 9.2: Test New() creates watcher successfully
  - [x] 9.3: Test readNewEntries() reads from correct position
  - [x] 9.4: Test truncation detection returns FileResetMsg
  - [x] 9.5: Test Close() properly cleans up resources and is idempotent
  - [x] 9.6: Run `make test` - all tests pass
  - [x] 9.7: Run `make lint` - no lint errors
  - [x] 9.8: Run `make build` - build succeeds

- [ ] Task 10: Manual testing and validation
  - [ ] 10.1: Test with actual Claude session: `./bin/cclv --watch ~/.claude/projects/*/conversations/*.jsonl`
  - [ ] 10.2: Verify new entries appear as Claude writes to log
  - [ ] 10.3: Verify "LIVE" indicator shows in status bar
  - [ ] 10.4: Verify clean exit with no goroutine warnings
  - [ ] 10.5: Verify file truncation handling by starting new session

## Dev Notes

### fsnotify Dependency (v1.8.0)

Use latest stable version compatible with Go 1.24.3:
```bash
go get github.com/fsnotify/fsnotify@v1.8.0
```

### project-context.md Dependency Update (Task 1.4)

Add to the Approved Additional Dependencies table in project-context.md:
```markdown
| `github.com/fsnotify/fsnotify` | File watching for live mode | 2026-01-XX |
```

### Watcher Implementation

```go
// internal/watcher/watcher.go
package watcher

import (
    "errors"
    "io"
    "os"
    "sync"

    "github.com/fsnotify/fsnotify"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// ErrFileTruncated is returned when the watched file is truncated
var ErrFileTruncated = errors.New("file was truncated")

type Watcher struct {
    filePath    string
    fsWatcher   *fsnotify.Watcher
    lastReadPos int64
    mu          sync.Mutex  // Protects lastReadPos
    closed      bool
}

func New(filePath string) (*Watcher, error) {
    fsw, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }
    if err := fsw.Add(filePath); err != nil {
        fsw.Close()
        return nil, err
    }
    // Initialize lastReadPos to current file size (skip existing content)
    stat, err := os.Stat(filePath)
    if err != nil {
        fsw.Close()
        return nil, err
    }
    return &Watcher{
        filePath:    filePath,
        fsWatcher:   fsw,
        lastReadPos: stat.Size(),
    }, nil
}

// WaitForEvent returns a tea.Cmd that blocks until a file event occurs.
// Call this in a chain: Update() handles message, returns WaitForEvent() for next event.
func (w *Watcher) WaitForEvent() tea.Cmd {
    return func() tea.Msg {
        select {
        case event, ok := <-w.fsWatcher.Events:
            if !ok {
                return nil // Watcher closed
            }
            if event.Has(fsnotify.Write) {
                entries, err := w.readNewEntries()
                if errors.Is(err, ErrFileTruncated) {
                    return FileResetMsg{}
                }
                if err != nil {
                    return WatcherErrorMsg{Err: err}
                }
                if len(entries) > 0 {
                    return NewEntriesMsg{Entries: entries}
                }
            }
            // Non-write event, continue waiting
            return w.WaitForEvent()()
        case err, ok := <-w.fsWatcher.Errors:
            if !ok {
                return nil
            }
            return WatcherErrorMsg{Err: err}
        }
    }
}

func (w *Watcher) readNewEntries() ([]types.LogEntry, error) {
    w.mu.Lock()
    defer w.mu.Unlock()

    file, err := os.Open(w.filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    // Check for truncation
    stat, err := file.Stat()
    if err != nil {
        return nil, err
    }
    if stat.Size() < w.lastReadPos {
        w.lastReadPos = 0
        return nil, ErrFileTruncated
    }

    // Seek to last position
    if _, err := file.Seek(w.lastReadPos, io.SeekStart); err != nil {
        return nil, err
    }

    // Parse new content
    result := parser.ParseJSONL(file)

    // Update position
    newPos, _ := file.Seek(0, io.SeekCurrent)
    w.lastReadPos = newPos

    return result.Entries, nil
}

func (w *Watcher) Close() error {
    w.mu.Lock()
    defer w.mu.Unlock()
    if w.closed {
        return nil // Idempotent
    }
    w.closed = true
    return w.fsWatcher.Close()
}
```

### Message Types

```go
// internal/watcher/messages.go
package watcher

import "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"

// NewEntriesMsg signals new entries were appended to the log file
type NewEntriesMsg struct {
    Entries []types.LogEntry
}

// WatcherErrorMsg signals an error occurred in the watcher
type WatcherErrorMsg struct {
    Err error
}

// FileResetMsg signals the file was truncated (new session started)
type FileResetMsg struct{}
```

### RenderOptions Update (viewer.go)

```go
type RenderOptions struct {
    HideThoughts bool
    HideTools    bool
    Width        int
    WatchMode    bool
    FilePath     string  // Full path for file watching
}
```

### ViewerModel Integration (viewer.go)

**Add field:**
```go
type ViewerModel struct {
    // ... existing fields
    watcher   *watcher.Watcher
}
```

**Constructor update (NewViewerModel):**
```go
func NewViewerModel(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions) ViewerModel {
    m := ViewerModel{
        // ... existing initialization
        watchMode: opts.WatchMode,
    }

    // Create watcher if watch mode enabled and file path provided
    if opts.WatchMode && opts.FilePath != "" {
        w, err := watcher.New(opts.FilePath)
        if err == nil {
            m.watcher = w
        }
        // If watcher creation fails, continue without it (graceful degradation)
    }

    return m
}
```

**Init() update:**
```go
func (m ViewerModel) Init() tea.Cmd {
    cmds := []tea.Cmd{}
    if m.watcher != nil {
        cmds = append(cmds, m.watcher.WaitForEvent())
    }
    // ... existing init logic
    if len(cmds) > 0 {
        return tea.Batch(cmds...)
    }
    return nil
}
```

**Update() additions:**
```go
case watcher.NewEntriesMsg:
    m.entries = append(m.entries, msg.Entries...)
    m.loadedCount = len(m.entries) // Update to show all entries
    m.updateContent()
    return m, m.watcher.WaitForEvent() // Chain next wait

case watcher.FileResetMsg:
    // Full reload - file was truncated
    if result, err := parser.ParseJSONLFile(m.filePath); err == nil {
        m.entries = result.Entries
        m.loadedCount = len(m.entries)
        m.updateContent()
    }
    return m, m.watcher.WaitForEvent()

case watcher.WatcherErrorMsg:
    // Could display in status bar (future enhancement)
    return m, m.watcher.WaitForEvent()
```

**Quit handler update:**
```go
case "q", "ctrl+c":
    if m.watcher != nil {
        m.watcher.Close()
    }
    return m, tea.Quit
```

### main.go Update (runPipelineMode)

```go
// After opening file, before creating model
if len(args) > 0 {
    absPath, _ := filepath.Abs(args[0])
    opts.FilePath = absPath
}
```

### Why Not Use Existing ParseJSONLStream?

The existing `parser.ParseJSONLStream()` is designed for initial load with channel-based streaming. It processes the entire file from start to end. For file watching, we need:
- Tail-read from last position (not full file)
- Repeated reads on each file change
- Position tracking between reads

The simple `ParseJSONL(io.Reader)` works perfectly when we control the file position.

### Package Structure

```
internal/
├── watcher/
│   ├── watcher.go      # Watcher struct, New(), WaitForEvent(), Close()
│   ├── watcher_test.go # Unit tests
│   └── messages.go     # NewEntriesMsg, WatcherErrorMsg, FileResetMsg
├── tui/
│   └── viewer.go       # Updated with watcher integration
└── parser/
    └── jsonl.go        # Existing - reuse ParseJSONL()
```

### Critical Constraints

| Constraint | Requirement |
|------------|-------------|
| **NO EMOJI in UI** | Text icons only: `[U]`, `[A]`, `[T]`, `[>]` |
| **Use Makefile** | `make build`, `make test`, never raw go commands |
| **tea.Cmd for side effects** | Never use raw goroutines in Bubbletea code |
| **Import order** | stdlib -> external -> internal |
| **Mutex for shared state** | lastReadPos accessed from tea.Cmd goroutine |
| **Idempotent Close()** | Safe to call watcher.Close() multiple times |

### Edge Cases

| Case | Handling |
|------|----------|
| Rapid writes | Multiple writes in quick succession processed in order |
| Empty file | Watch mode on empty file works, waits for first write |
| File deleted | WatcherErrorMsg with error, graceful degradation |
| Permission denied | WatcherErrorMsg with error displayed |
| Very large appends | ParseJSONL handles incrementally, 1MB line buffer |
| Truncation | FileResetMsg triggers full reload from position 0 |

### Testing Strategy

```bash
# Build and basic validation
make build
make test
make lint

# Manual testing with live Claude session
# Terminal 1: Run cclv in watch mode
./bin/cclv --watch ~/.claude/projects/*/conversations/latest.jsonl

# Terminal 2: Simulate Claude writing
echo '{"type":"user","message":{"role":"user","content":"test"},"timestamp":"2026-01-16T10:00:00Z"}' >> test.jsonl

# Verify entry appears in cclv without restart
```

### Previous Story Context

**From Story 2.1:**
- `--watch` and `--live` flags implemented (main.go lines 121-126)
- `ViewerModel.watchMode` field exists and wired to opts.WatchMode
- Status bar "LIVE" indicator renders when watchMode is true (lines 402-408)
- Validation ensures --watch requires file argument (main.go lines 144-148)

### Git Intelligence

Recent commits:
```
82ac5ae feat: add --watch and --live CLI flags for file watching mode
76d7014 feat: add comprehensive help output with examples
f95b1e2 feat: add --width flag for pipeline width override
```

Suggested commit:
```
feat: implement fsnotify file watching for live log updates

- Add fsnotify v1.8.0 dependency
- Create internal/watcher package for file monitoring
- Integrate watcher with ViewerModel via tea.Cmd pattern
- Handle file modifications and truncation gracefully
- Ensure clean watcher shutdown on quit

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Performance Considerations

- fsnotify is kernel-level (kqueue on macOS, inotify on Linux)
- Only read new content (tail-read from lastReadPos)
- Target: < 200ms latency from write to display (per NFR-001)
- No debouncing needed - fsnotify events are already batched by OS

### References

- [Source: epics.md lines 696-748] - Story 2.2 requirements
- [Source: prd.md lines 123-133] - FR-202: fsnotify Integration requirements
- [Source: project-context.md] - Architecture constraints, tea.Cmd pattern
- [Source: Story 2.1] - Watch mode flag implementation, model field wiring
- [Source: internal/tui/viewer.go:76-80] - Existing watchMode field and status bar
- [Source: pkg.go.dev/fsnotify] - fsnotify API documentation

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- Added fsnotify v1.9.0 (latest stable, minor version bump from specified v1.8.0)
- Created internal/watcher package with watcher.go and messages.go
- Integrated watcher into ViewerModel with proper tea.Cmd chaining pattern
- All watcher message types handled: NewEntriesMsg, FileResetMsg, WatcherErrorMsg
- Watcher cleanup on quit (q/ctrl+c) is idempotent
- All tests pass, lint clean, build succeeds
- Manual testing pending (Task 10)

### Code Review Fixes Applied (2026-01-16)

- **H1 Fixed**: Replaced recursive call in WaitForEvent() with loop to prevent stack overflow
- **M1 Fixed**: Added error handling for Seek() in readNewEntries()
- **M3 Fixed**: Removed redundant filePath field from ViewerModel, now uses renderOpts.FilePath
- **L1 Fixed**: Updated test to use errors.Is() instead of direct comparison

### File List

- go.mod (modified - added fsnotify dependency)
- go.sum (modified - added fsnotify checksums)
- internal/watcher/watcher.go (new - Watcher struct, New(), WaitForEvent(), readNewEntries(), Close())
- internal/watcher/messages.go (new - NewEntriesMsg, WatcherErrorMsg, FileResetMsg)
- internal/watcher/watcher_test.go (new - unit tests for watcher package)
- internal/tui/viewer.go (modified - added watcher field, FilePath to RenderOptions, Init/Update handlers)
- cmd/cclv/main.go (modified - pass absolute file path via opts.FilePath)
- _bmad-output/project-context.md (modified - added fsnotify to Approved Dependencies)
