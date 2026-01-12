# Research: CCLV Fixes and Enhancements

**Feature**: 002-cclv-fixes-enhancements
**Date**: 2026-01-12

## Research Topics

### 1. Claude Code Path Encoding Scheme

**Decision**: Claude Code encodes paths by replacing `/` with `-` and escapes literal hyphens by doubling them (`--`).

**Rationale**: Examined the existing `DecodeProjectPath` function and Claude Code's actual directory structure. The current implementation incorrectly replaces all `-` with `/` first, then `//` with `-`. This order is wrong - it should first identify escaped hyphens (`--`) and preserve them, then convert single `-` to `/`.

**Evidence**: Directory names in `~/.claude/projects/` show patterns like:
- `-Users-me-GitHub-foo` → `/Users/me/GitHub/foo`
- `-Users-me-my--project` → `/Users/me/my-project` (double hyphen = literal hyphen)

**Alternatives Considered**:
- URL encoding: Not used by Claude Code
- Base64 encoding: Not used by Claude Code

### 2. Navigation Double-Skip Root Cause

**Decision**: The bug is caused by handling navigation in both the custom Update handler AND letting the bubbles list component handle the same key event.

**Rationale**: In `project.go` and `conversation.go`, the code calls `m.list.CursorDown()` explicitly for j/down keys, but then also passes the message to `m.list.Update(msg)` at the bottom of the function. The list component processes the same key event again, causing double movement.

**Fix Strategy**:
- Option A: Return early after manual cursor movement (don't pass to list.Update)
- Option B: Remove manual cursor calls and let list.Update handle everything
- **Selected**: Option B - Let the bubbles list component handle all navigation. It already supports j/k and arrow keys. Remove explicit CursorDown/CursorUp calls.

**Alternatives Considered**:
- Option A rejected because manual control is unnecessary - bubbles list already handles vim keys

### 3. Plain Text Output Implementation

**Decision**: Create a `RenderPlain` function that outputs formatted text to stdout without TUI.

**Rationale**: The plain text renderer should reuse existing rendering logic from the viewer but output directly to stdout instead of through the Bubbletea viewport.

**Implementation Approach**:
1. Add `--plain` and `--tui` flags using Go's `flag` package
2. Check stdout TTY status using `term.IsTerminal(os.Stdout.Fd())`
3. Mode selection logic:
   - `--plain` flag: always plain mode
   - `--tui` flag: always TUI mode
   - Neither flag + stdout is TTY: TUI mode
   - Neither flag + stdout is not TTY: plain mode
4. Create `internal/tui/plain.go` with `RenderPlain(entries []types.LogEntry, source string)` function
5. Reuse `renderUserMessage`, `renderAssistantMessage` logic with Lipgloss styling

**Alternatives Considered**:
- JSON output mode: Deferred to future enhancement (not requested)
- Markdown output: Deferred (ANSI colors work better for terminal piping)

### 4. Viewer Title Context

**Decision**: Pass source context through the model chain and display in viewer title.

**Rationale**: The viewer needs to know its source to display appropriate context. Three source types:
1. Interactive browser: project name + conversation timestamp
2. File argument: filename
3. Stdin pipe: "stdin"

**Implementation Approach**:
1. Add `title string` field to ViewerModel
2. Modify `NewViewerModel` to accept title parameter
3. Update call sites to pass appropriate title:
   - `app.go`: Pass project name + conversation info
   - `main.go` (pipeline): Pass filename or "stdin"
4. Render title in `View()` function header

**Alternatives Considered**:
- Full path in title: Too long, use basename only
- Timestamp only: Not enough context for multiple conversations
