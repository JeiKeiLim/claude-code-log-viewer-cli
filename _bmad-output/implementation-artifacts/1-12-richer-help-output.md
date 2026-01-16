# Story 1.12: Richer Help Output

Status: done

## Story

As a **developer learning cclv**,
I want **comprehensive help with examples**,
So that **I can quickly understand available options**.

## Acceptance Criteria

### AC 1.12.1: Expanded flag descriptions
- **Given** I run `cclv --help`
- **When** viewing the output
- **Then** each flag has a clear description
- **And** descriptions explain the purpose, not just the syntax

### AC 1.12.2: Usage examples
- **Given** I run `cclv --help`
- **When** viewing the output
- **Then** common usage examples are shown
- **And** examples cover: basic usage, pipeline mode, filtering

### AC 1.12.3: Keyboard shortcuts section
- **Given** I run `cclv --help`
- **When** viewing the output
- **Then** TUI keyboard shortcuts are documented
- **And** grouped logically (navigation, toggles, actions)

## Tasks / Subtasks

- [x] Task 1: Create custom usage function (AC: 1.12.1, 1.12.2, 1.12.3)
  - [x] 1.1: Create `printHelp()` function that outputs formatted help text
  - [x] 1.2: Replace default `flag.Usage` with custom `printHelp` in main.go: `flag.Usage = printHelp`
  - [x] 1.3: Structure help output with sections: USAGE, OPTIONS, EXAMPLES, KEYBOARD SHORTCUTS

- [x] Task 2: Expand flag descriptions (AC: 1.12.1)
  - [x] 2.1: Update `--plain` description: "Output plain text without TUI (suitable for piping)"
  - [x] 2.2: Update `--tui` description: "Force interactive TUI mode even when stdout is piped"
  - [x] 2.3: Update `--color` description: "Color output mode: auto, always, never"
  - [x] 2.4: Update `--version` description: "Print version information and exit"
  - [x] 2.5: Update `--hide-thoughts` description: "Hide Claude's thinking blocks from output"
  - [x] 2.6: Update `--hide-tools` description: "Hide tool use/result blocks from output"
  - [x] 2.7: Update `--width` description: "Set rendering width in characters (40-500, 0=auto)"

- [x] Task 3: Add usage examples section (AC: 1.12.2)
  - [x] 3.1: Add "Interactive mode" example: `cclv` alone browses all projects
  - [x] 3.2: Add "View specific file" example: `cclv conversation.jsonl`
  - [x] 3.3: Add "Pipeline mode" example: `cclv --plain file.jsonl | less`
  - [x] 3.4: Add "Filter output" example: `cclv --hide-thoughts --hide-tools file.jsonl`
  - [x] 3.5: Add "Custom width" example: `cclv --width=100 file.jsonl` (use = syntax for consistency)
  - [x] 3.6: Add "Force colors in pipe" example: `cclv --color=always file.jsonl | less -R`
  - [x] 3.7: Add "Read from stdin" example: `cat conversation.jsonl | cclv`

- [x] Task 4: Add keyboard shortcuts section (AC: 1.12.3)
  - [x] 4.1: Add "Navigation" shortcuts: j/k (up/down), gg/G (top/bottom), h/l (back/forward)
  - [x] 4.2: Add "Toggles" shortcuts: t (thoughts), i (tool inputs)
  - [x] 4.3: Add "Actions" shortcuts: enter (select), q (quit), / (search), n/N (next/prev match), esc (cancel)
  - [x] 4.4: Add "Scrolling" shortcuts: d/u (half-page), PgUp/PgDn/Space (page), Home/End

- [x] Task 5: Format help output consistently (AC: all)
  - [x] 5.1: Use consistent indentation (2 spaces for options/examples)
  - [x] 5.2: Use uppercase for section headers (USAGE, OPTIONS, EXAMPLES, etc.)
  - [x] 5.3: Align option descriptions for readability
  - [x] 5.4: Keep total width under 80 characters for terminal compatibility

- [x] Task 6: Add tests and validation (all ACs)
  - [x] 6.1: Add test that verifies help output contains expected sections
  - [x] 6.2: Verify all flags are documented in help
  - [x] 6.3: Run `make test` - all tests pass
  - [x] 6.4: Run `make lint` - no lint errors
  - [x] 6.5: Run `make build` - build succeeds
  - [x] 6.6: Manual test: `./bin/cclv --help` shows formatted output

## Dev Notes

### Help Output Format

The current help output uses Go's default `flag.Usage` which produces:

```
Usage of ./bin/cclv:
  -color string
        Color output: auto, always, never (default "auto")
  -hide-thoughts
        Hide thinking blocks in output
  ...
```

Target format:

```
cclv - Claude Code Log Viewer

USAGE:
  cclv                          Interactive mode - browse all projects
  cclv [options] <file>         View a specific conversation file
  cat file.jsonl | cclv         Read from stdin

OPTIONS:
  --plain           Output plain text without TUI (for piping)
  --tui             Force interactive TUI mode even when piped
  --color=MODE      Color mode: auto (default), always, never
  --hide-thoughts   Hide Claude's thinking blocks
  --hide-tools      Hide tool use/result blocks
  --width=N         Set rendering width (40-500, 0=auto)
  -v, --version     Print version information
  -h, --help        Show this help message

EXAMPLES:
  cclv                                    Browse all Claude projects
  cclv conversation.jsonl                 View a conversation file
  cat file.jsonl | cclv                   Read from stdin
  cclv --plain file.jsonl | less          Pipeline with pager
  cclv --hide-thoughts --hide-tools f.jsonl  Show only messages
  cclv --width=100 file.jsonl             Fixed 100-char width
  cclv --color=always file.jsonl | less -R  Force colors in pipe

KEYBOARD SHORTCUTS (TUI mode):
  Navigation:   j/k             Move up/down
                gg/G            Jump to top/bottom
                h/l or esc      Go back / forward

  Scrolling:    d/u             Half-page down/up
                PgDn/PgUp/Space Page down/up
                Home/End        Jump to start/end

  Toggles:      t               Toggle thinking blocks
                i               Toggle tool inputs

  Search:       /               Start search
                n/N             Next/previous match

  Actions:      enter           Select item
                q               Quit
```

### Implementation Strategy

Use `flag.Usage` override rather than adding a separate `--help` flag.

**Complete implementation template:**

```go
// Add this after imports, before main()
func init() {
    flag.Usage = printHelp
}

// printHelp outputs comprehensive help with examples and keyboard shortcuts.
func printHelp() {
    w := flag.CommandLine.Output()
    fmt.Fprintln(w, "cclv - Claude Code Log Viewer")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "USAGE:")
    fmt.Fprintln(w, "  cclv                          Interactive mode - browse all projects")
    fmt.Fprintln(w, "  cclv [options] <file>         View a specific conversation file")
    fmt.Fprintln(w, "  cat file.jsonl | cclv         Read from stdin")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "OPTIONS:")
    fmt.Fprintln(w, "  --plain           Output plain text without TUI (for piping)")
    fmt.Fprintln(w, "  --tui             Force interactive TUI mode even when piped")
    fmt.Fprintln(w, "  --color=MODE      Color mode: auto (default), always, never")
    fmt.Fprintln(w, "  --hide-thoughts   Hide Claude's thinking blocks")
    fmt.Fprintln(w, "  --hide-tools      Hide tool use/result blocks")
    fmt.Fprintln(w, "  --width=N         Set rendering width (40-500, 0=auto)")
    fmt.Fprintln(w, "  -v, --version     Print version information")
    fmt.Fprintln(w, "  -h, --help        Show this help message")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "EXAMPLES:")
    fmt.Fprintln(w, "  cclv                                    Browse all Claude projects")
    fmt.Fprintln(w, "  cclv conversation.jsonl                 View a conversation file")
    fmt.Fprintln(w, "  cat file.jsonl | cclv                   Read from stdin")
    fmt.Fprintln(w, "  cclv --plain file.jsonl | less          Pipeline with pager")
    fmt.Fprintln(w, "  cclv --hide-thoughts --hide-tools f.jsonl  Show only messages")
    fmt.Fprintln(w, "  cclv --width=100 file.jsonl             Fixed 100-char width")
    fmt.Fprintln(w, "  cclv --color=always file.jsonl | less -R  Force colors in pipe")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "KEYBOARD SHORTCUTS (TUI mode):")
    fmt.Fprintln(w, "  Navigation:   j/k             Move up/down")
    fmt.Fprintln(w, "                gg/G            Jump to top/bottom")
    fmt.Fprintln(w, "                h/l or esc      Go back / forward")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "  Scrolling:    d/u             Half-page down/up")
    fmt.Fprintln(w, "                PgDn/PgUp/Space Page down/up")
    fmt.Fprintln(w, "                Home/End        Jump to start/end")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "  Toggles:      t               Toggle thinking blocks")
    fmt.Fprintln(w, "                i               Toggle tool inputs")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "  Search:       /               Start search")
    fmt.Fprintln(w, "                n/N             Next/previous match")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "  Actions:      enter           Select item")
    fmt.Fprintln(w, "                q               Quit")
}
```

**Alternative approach using embedded template:**

```go
func init() {
    flag.Usage = printHelp
}

func printHelp() {
    w := flag.CommandLine.Output()
    fmt.Fprintln(w, "cclv - Claude Code Log Viewer")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "USAGE:")
    fmt.Fprintln(w, "  cclv                          Interactive mode - browse all projects")
    fmt.Fprintln(w, "  cclv [options] <file>         View a specific conversation file")
    fmt.Fprintln(w, "  cat file.jsonl | cclv         Read from stdin")
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "OPTIONS:")
    // ... flag descriptions ...
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "EXAMPLES:")
    // ... examples ...
    fmt.Fprintln(w, "")
    fmt.Fprintln(w, "KEYBOARD SHORTCUTS (TUI mode):")
    // ... shortcuts ...
}
```

### Key Points

1. **flag.Usage override**: Go's `flag` package calls `flag.Usage` when `-h` or `--help` is passed. Override this function. Note: `-h` is automatically handled by Go's flag package - no need to define a separate `-h` flag.

2. **Write to flag.CommandLine.Output()**: This ensures output goes to the right place (stderr by default).

3. **No raw emoji**: Follow project rule - use text only for all output.

4. **80-char width**: Keep lines under 80 chars for terminal compatibility.

5. **Alphabetical options**: List options in a logical order (common first, then alphabetical).

6. **Flag syntax**: Support both `--flag value` and `--flag=value` forms. Go's flag package handles this automatically.

### Current Flag Definitions (Reference)

From main.go lines 57-65:
```go
plainFlag := flag.Bool("plain", false, "Output plain text without TUI")
tuiFlag := flag.Bool("tui", false, "Force TUI mode even when stdout is piped")
colorFlag := flag.String("color", "auto", "Color output: auto, always, never")
versionFlag := flag.Bool("version", false, "Print version information and exit")
versionShortFlag := flag.Bool("v", false, "Print version information and exit (shorthand)")
hideThoughtsFlag := flag.Bool("hide-thoughts", false, "Hide thinking blocks in output")
hideToolsFlag := flag.Bool("hide-tools", false, "Hide tool use blocks in output")
widthFlag := flag.Int("width", 0, "Override rendering width (0=auto-detect)")
```

### TUI Keyboard Shortcuts (From codebase)

Reference: `internal/tui/viewer.go` Update() function (lines 169-346)

| Key | Action |
|-----|--------|
| j, down | Scroll down one line |
| k, up | Scroll up one line |
| gg | Jump to top (double-tap g within 500ms) |
| G | Jump to bottom (loads all entries if lazy loading) |
| home | Jump to top |
| end | Jump to bottom |
| h, esc | Go back (when canGoBack is true) |
| d, ctrl+d | Half-page down |
| u, ctrl+u | Half-page up |
| pgdown, space | Page down |
| pgup | Page up |
| t | Toggle thinking blocks visibility |
| i | Toggle tool inputs visibility |
| / | Start search (opens search input) |
| n | Next search match |
| N | Previous search match |
| enter | (In search mode) Execute search |
| esc | (In search mode) Cancel search |
| q, ctrl+c | Quit |

**Note**: The `gg` sequence requires pressing `g` twice within 500ms.

### Previous Story Intelligence

**From Story 1.11 (Pipeline Width Override):**
- Flag pattern: define with `flag.Bool/Int/String`, parse, validate, pass to options
- Warning pattern: write to stderr with `fmt.Fprintf(os.Stderr, ...)`
- No changes needed to opts passing for this story - help is self-contained

**From Story 1.10 (Conversation Count):**
- Simple additions follow clean patterns
- Keep changes minimal and focused

**From Story 1.9 (Pipeline Visibility Flags):**
- CLI flag integration established
- RenderOptions pattern in place

### Git Intelligence

Recent commits show:
- `f95b1e2 feat: add --width flag for pipeline width override` - most recent flag addition
- Pattern: feat prefix for new features
- Clean commit messages with bullet points

Suggested commit:
```
feat: add comprehensive help output with examples

- Override flag.Usage with custom printHelp function
- Add USAGE, OPTIONS, EXAMPLES, KEYBOARD SHORTCUTS sections
- Expand flag descriptions for clarity
- Document all TUI keyboard shortcuts

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Risk Assessment

**Risk: LOW**

- Self-contained change to main.go only
- No runtime behavior changes
- No new dependencies
- Simple string formatting
- Easy to test manually

### Edge Cases

1. **--help with other flags**: `flag` package exits after showing help, other flags ignored
2. **-h shorthand**: Automatically handled by Go's `flag` package (no separate flag needed)
3. **Terminal width**: Keep output under 80 chars so it displays correctly everywhere
4. **Version in help**: Version is shown via `--version`, not in main help (keep help focused)
5. **Help goes to stderr**: Standard Unix convention - help output to stderr, data to stdout
6. **Exit code**: Help exits with code 0 (success) per convention

### Files to Modify Checklist

- [ ] `cmd/cclv/main.go` - Add `printHelp()` function and `flag.Usage = printHelp`
- [ ] `cmd/cclv/main_test.go` - Add test for help output (optional, given manual verification is straightforward)

### Testing Pattern

```bash
# Manual verification
make build                         # Build first
./bin/cclv --help                  # Should show new formatted help
./bin/cclv -h                      # Should show same help (shorthand)
./bin/cclv --help 2>&1             # Verify goes to stderr
./bin/cclv --help > /dev/null      # Confirm no stdout output
./bin/cclv --help 2>/dev/null; echo $?  # Should be 0 (success)

# Verify all flags documented
./bin/cclv --help 2>&1 | grep -E '^\s+--'  # List all flag lines

# Verify sections present
./bin/cclv --help 2>&1 | grep -E '^(USAGE|OPTIONS|EXAMPLES|KEYBOARD)'
```

### Project Context Reference

From `project-context.md`:
- **NO EMOJI IN UI** - Use text only
- **USE MAKEFILE** - `make build`, `make test`, `make lint`
- Naming: lowercase files, PascalCase exports

### References

- [Source: epics.md lines 595-636] - Story requirements and technical notes
- [Source: project-context.md] - NO EMOJI, USE MAKEFILE rules
- [Source: cmd/cclv/main.go lines 55-65] - Existing flag definitions
- [Source: internal/tui/viewer.go] - Keyboard shortcuts in Update()
- [Source: Story 1.11] - Most recent flag addition pattern

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- Implemented `printHelp()` function with raw string literal for clean, linter-compliant output
- Used `init()` to set `flag.Usage = printHelp` before main() runs
- All 8 flags documented with expanded descriptions
- 7 usage examples covering interactive, file, stdin, pipeline, filtering, width, and color modes
- Keyboard shortcuts organized into 5 groups: Navigation, Scrolling, Toggles, Search, Actions
- Added `TestPrintHelp` test verifying all sections, flags, and shortcuts are present
- All tests pass, lint clean, build succeeds
- Manual verification: `--help` and `-h` both show formatted output, exit code 0

### Code Review Fixes Applied (2026-01-16)

- Fixed keyboard shortcuts: separated `h/esc` (back) and `l/enter` (forward) for clarity
- Added `Ctrl+c` to quit shortcuts documentation
- Added `Ctrl+d/u` to scrolling shortcuts documentation
- Fixed example consistency: changed `f.jsonl` to `file.jsonl`
- Fixed test cleanup: added `t.Cleanup()` to restore `flag.CommandLine.Output()` after test
- Updated test assertions to verify new shortcut documentation

### File List

- `cmd/cclv/main.go` - Added `init()` and `printHelp()` functions (lines 22-72)
- `cmd/cclv/main_test.go` - Added `TestPrintHelp` test (lines 58-145)
