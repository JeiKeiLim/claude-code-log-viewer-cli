# Story 2.1: Add Watch Mode CLI Flag

Status: done

## Validation Summary (2026-01-16)

| Check | Result |
|-------|--------|
| FR-201 alignment | PASS |
| Epic 2.1 alignment | PASS |
| Codebase accuracy | CORRECTED |
| Task clarity | IMPROVED |

**Key corrections applied:**
- RenderOptions is in `viewer.go`, not `plain.go`
- ViewerModel.watchMode field already exists (viewer.go:74-75)
- Status bar "LIVE" indicator already implemented (viewer.go:400-405)
- Project uses stdlib `flag` package, not cobra
- Updated file references and line numbers

## Story

As a **developer**,
I want **to enable watch mode via CLI flag**,
So that **I can monitor live Claude sessions**.

## Acceptance Criteria

### AC 2.1.1: --watch flag
- **Given** the cclv CLI
- **When** I run `cclv --watch <file>`
- **Then** watch mode is enabled
- **And** file monitoring starts after initial load

### AC 2.1.2: --live alias
- **Given** the cclv CLI
- **When** I run `cclv --live <file>`
- **Then** it behaves identically to --watch

### AC 2.1.3: Help documentation
- **Given** I run `cclv --help`
- **When** viewing the output
- **Then** --watch and --live flags are documented
- **And** description explains real-time monitoring

### AC 2.1.4: Flag stored in model
- **Given** watch mode is enabled via flag
- **When** the Bubbletea model initializes
- **Then** a `watchMode bool` field is set to true
- **And** this triggers watcher initialization (in Story 2.2)

## Tasks / Subtasks

- [x] Task 1: Add CLI flags (AC: 2.1.1, 2.1.2)
  - [x] 1.1: Add `--watch` flag: `watchFlag := flag.Bool("watch", false, "Watch file for changes (real-time monitoring)")`
  - [x] 1.2: Add `--live` flag: `liveFlag := flag.Bool("live", false, "Alias for --watch")`
  - [x] 1.3: Combine flags after parsing: `watchMode := *watchFlag || *liveFlag`
  - [x] 1.4: Pass `watchMode` to pipeline mode function

- [x] Task 2: Update help documentation (AC: 2.1.3)
  - [x] 2.1: Add `--watch` to OPTIONS section: `--watch           Watch file for changes (real-time monitoring)`
  - [x] 2.2: Add `--live` to OPTIONS section: `--live            Alias for --watch`
  - [x] 2.3: Add example to EXAMPLES section: `cclv --watch conversation.jsonl        Monitor live session`

- [x] Task 3: Update RenderOptions struct (AC: 2.1.4)
  - [x] 3.1: Add `WatchMode bool` field to `tui.RenderOptions` struct in `internal/tui/viewer.go` (lines 21-25)
  - [x] 3.2: Set `WatchMode: watchMode` when creating opts in main.go
  - [x] 3.3: Pass opts to `NewViewerModel` (already done, just ensure WatchMode is included)

- [x] Task 4: Update ViewerModel to use watch state (AC: 2.1.4)
  - [x] 4.1: ALREADY EXISTS: `watchMode bool` field exists in `ViewerModel` struct (viewer.go:74-75)
  - [x] 4.2: Set field in `NewViewerModel` from `opts.WatchMode` (currently not wired up)
  - [x] 4.3: ALREADY EXISTS: "LIVE" indicator already implemented in `buildModeSegment()` (viewer.go:400-405)

- [x] Task 5: Handle watch mode constraints
  - [x] 5.1: Validate that --watch/--live requires a file argument (not stdin)
  - [x] 5.2: Print error if used without file: `Error: --watch requires a file path argument`
  - [x] 5.3: Document constraint in help: file watching only works with file arguments, not stdin

- [x] Task 6: Add tests and validation (all ACs)
  - [x] 6.1: Add test verifying --watch flag is parsed correctly
  - [x] 6.2: Add test verifying --live flag works as alias
  - [x] 6.3: Add test that --watch with stdin shows appropriate error
  - [x] 6.4: Verify help output includes --watch and --live documentation
  - [x] 6.5: Run `make test` - all tests pass
  - [x] 6.6: Run `make lint` - no lint errors
  - [x] 6.7: Run `make build` - build succeeds
  - [x] 6.8: Manual test: `./bin/cclv --help` shows watch flags

## Dev Notes

### CLI Flag Implementation Pattern

Following the established pattern from Story 1.11 (--width flag) and Story 1.9 (--hide-thoughts/--hide-tools):

```go
// In main.go, after existing flag definitions (line ~117)
watchFlag := flag.Bool("watch", false, "Watch file for changes (real-time monitoring)")
liveFlag := flag.Bool("live", false, "Alias for --watch")

// After flag.Parse() (line ~118)
watchMode := *watchFlag || *liveFlag
```

### Help Documentation Update

Current help text location: `cmd/cclv/main.go` lines 26-72 (`printHelp` function)

Add to OPTIONS section (after `--width`):
```
  --watch           Watch file for changes (real-time monitoring)
  --live            Alias for --watch
```

Add to EXAMPLES section:
```
  cclv --watch conversation.jsonl        Monitor live session
```

### RenderOptions Update

Current struct in `internal/tui/viewer.go` (lines 21-25):
```go
type RenderOptions struct {
	HideThoughts bool // Hide thinking blocks
	HideTools    bool // Hide tool use blocks
	Width        int  // Width override for rendering (0=auto-detect)
}
```

Add WatchMode field:
```go
type RenderOptions struct {
	HideThoughts bool // Hide thinking blocks
	HideTools    bool // Hide tool use blocks
	Width        int  // Width override for rendering (0=auto-detect)
	WatchMode    bool // Enable file watching mode
}
```

### ViewerModel Update

**ALREADY IMPLEMENTED:** The `watchMode bool` field already exists in ViewerModel (viewer.go:74-75):
```go
// Watch mode (for future Story 2.1)
watchMode bool
```

**NEEDS WIRING:** Add to `NewViewerModel()` around line 105-118:
```go
m := ViewerModel{
    // ... existing fields
    renderOpts:     opts,
    watchMode:      opts.WatchMode,  // ADD THIS LINE
}
```

The `buildModeSegment()` method (viewer.go:400-405) already renders "LIVE" when watchMode is true.

### Watch Mode Constraints

**Critical**: Watch mode only works with file arguments, NOT stdin.

Reason: fsnotify watches file descriptors on the filesystem. Stdin is a pipe/stream that doesn't support file notifications.

Validation in main.go before calling runPipelineMode:
```go
if watchMode && len(args) == 0 && !stdinTTY {
    fmt.Fprintf(os.Stderr, "Error: --watch requires a file path argument (cannot watch stdin)\n")
    os.Exit(1)
}
```

### Status Bar - ALREADY IMPLEMENTED

**NO ACTION NEEDED:** The status bar "LIVE" indicator is already implemented in viewer.go:

```go
// buildModeSegment returns the mode indicator segment (for watch mode).
func (m ViewerModel) buildModeSegment() string {
    if !m.watchMode {
        return "" // Empty when not in watch mode
    }
    return Styles.StatusBarSegment.Mode.Render("LIVE")
}
```

This is called in `View()` (viewer.go:454) and renders in the footer. No changes needed here - just wire up `opts.WatchMode` to `m.watchMode` in `NewViewerModel()`.

### Previous Story Intelligence

**From Story 1.12 (Richer Help Output):**
- Help uses raw string literal in `printHelp()` function
- Section format: USAGE, OPTIONS, EXAMPLES, KEYBOARD SHORTCUTS
- `flag.Usage = printHelp` set in `init()`

**From Story 1.11 (Pipeline Width Override):**
- Flag validation pattern: validate after parsing, before use
- Error messages go to stderr: `fmt.Fprintf(os.Stderr, ...)`
- RenderOptions struct carries flag values to tui package

**From Story 1.9 (Pipeline Visibility Flags):**
- Boolean flags: `flag.Bool(name, default, description)`
- Combined in opts: `RenderOptions{HideThoughts: *hideThoughtsFlag, ...}`

### Git Intelligence

Recent commit pattern:
- `76d7014 feat: add comprehensive help output with examples`
- `f95b1e2 feat: add --width flag for pipeline width override`

Suggested commit:
```
feat: add --watch and --live CLI flags for file watching mode

- Add --watch flag to enable real-time file monitoring
- Add --live as alias for --watch
- Add WatchMode to RenderOptions and ViewerModel
- Update help documentation with watch mode info
- Validate watch mode requires file argument (not stdin)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Test Strategy

```bash
# Build first
make build

# Verify flags parse without error
./bin/cclv --watch test.jsonl  # Should work (file watching placeholder)
./bin/cclv --live test.jsonl   # Should work (same as --watch)
./bin/cclv --watch             # Should error (no file)
cat test.jsonl | ./bin/cclv --watch  # Should error (stdin not supported)

# Verify help
./bin/cclv --help 2>&1 | grep watch  # Should show --watch
./bin/cclv --help 2>&1 | grep live   # Should show --live

# Run tests
make test
make lint
```

### Project Structure Notes

Files to modify:
1. `cmd/cclv/main.go` - Add flags, validation, help text, pass WatchMode in opts
2. `internal/tui/viewer.go` - Add WatchMode to RenderOptions, wire watchMode field in NewViewerModel
3. `cmd/cclv/main_test.go` - Add tests for new flags

**NOTE:** RenderOptions is in viewer.go, NOT plain.go. ViewerModel.watchMode field already exists - just needs wiring.

No new files needed. This is a foundation story for Epic 2's file watching feature.

### Risk Assessment

**Risk: LOW**

- Simple flag addition following established patterns
- No new dependencies
- No complex logic - just flag parsing and struct fields
- Actual watcher implementation deferred to Story 2.2
- Easy to test manually

### Edge Cases

1. **--watch --live together**: Both flags enable watch mode, no conflict
2. **--watch with --plain**: Watch mode works with plain output too
3. **--watch with stdin**: Must error - fsnotify can't watch stdin
4. **Interactive mode (no args)**: --watch ignored (browse mode has no file to watch)
5. **Invalid file path**: Normal file-not-found error, not watch-specific

### References

- [Source: epics.md lines 651-694] - Story 2.1 requirements and technical notes
- [Source: prd.md lines 115-122] - FR-201: File Watch Mode Flag requirements
- [Source: project-context.md] - NO EMOJI, USE MAKEFILE, Bubbletea patterns
- [Source: cmd/cclv/main.go] - Current flag definitions (stdlib flag, not cobra) and help text
- [Source: internal/tui/viewer.go:21-25] - RenderOptions struct location (NOT plain.go)
- [Source: internal/tui/viewer.go:74-75] - ViewerModel.watchMode field (already exists)
- [Source: internal/tui/viewer.go:400-405] - buildModeSegment() "LIVE" indicator (already exists)
- [Source: Story 1.11, 1.12] - Recent flag addition patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- Added `--watch` and `--live` CLI flags to main.go following existing flag patterns
- Updated help documentation with new flags and example
- Added `WatchMode bool` field to `RenderOptions` struct
- Wired `opts.WatchMode` to `ViewerModel.watchMode` in `NewViewerModel()`
- Added stdin validation: `--watch` requires file argument, errors on stdin pipe
- Added tests for `--watch` and `--live` flags in help output
- Added `TestNewViewerModelWatchMode` test to verify WatchMode propagation
- All existing tests pass (make test)
- Lint passes (make lint)
- Build succeeds (make build)
- Manual verification: `./cclv --help` shows --watch and --live flags
- Manual verification: `echo "test" | ./cclv --watch` shows appropriate error

### File List

- cmd/cclv/main.go (modified: added flags, validation, WatchMode in opts)
- cmd/cclv/main_test.go (modified: added --watch and --live to required flags test)
- internal/tui/viewer.go (modified: added WatchMode to RenderOptions, wired watchMode in NewViewerModel)
- internal/tui/viewer_test.go (modified: added TestNewViewerModelWatchMode)
