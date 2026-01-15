---
stepsCompleted: ['step-01-init', 'step-02-discovery', 'step-03-success', 'step-04-users', 'step-05-functional', 'step-06-nonfunctional', 'step-07-constraints', 'step-08-scope', 'step-09-risks', 'step-10-dependencies', 'step-11-complete']
inputDocuments:
  - '_bmad-output/planning-artifacts/product-brief-claude-code-log-viewer-cli-2026-01-15.md'
  - '_bmad-output/planning-artifacts/research/technical-tui-streaming-markdown-polish-research-2026-01-15.md'
  - '_bmad-output/project-context.md'
  - '_bmad-output/planning-artifacts/research/CONTINUATION-PLAN.md'
workflowType: 'prd'
projectType: 'brownfield'
phase: 2
status: complete
classification:
  projectType: 'CLI/TUI Application'
  domain: 'Developer Tools'
  complexity: 'Low-Medium'
  projectContext: 'brownfield'
---

# Product Requirements Document
## cclv Visual & Streaming Enhancements (Phase 2)

**Author:** Jongkuk Lim
**Date:** 2026-01-15
**Version:** 1.0
**Status:** Complete

---

## 1. Executive Summary

cclv (Claude Code Log Viewer) Phase 2 enhances an existing personal developer tool with visual polish, real-time log watching, and markdown rendering. This is a craft project focused on making the tool feel finished and exploring what's possible with the Bubbletea TUI framework.

**Scope:** Three features, four implementation phases
**Timeline:** Self-paced personal project
**Success Criteria:** Subjective satisfaction + technical targets

---

## 2. Product Vision

### Problem Statement

cclv works but could feel better. The visuals are utilitarian, logs must be reloaded to see updates, and markdown displays as raw text.

### Solution

Enhance cclv with:
1. **Visual Polish** - Modern TUI aesthetics with theme-aware colors
2. **Real-time File Watching** - Auto-update when log files change
3. **Markdown Rendering** - Glamour-based rendering for assistant text

### Success Criteria

- All three features implemented and working
- Visual appearance matches personal taste
- Technical targets met (startup < 100ms, memory < 50MB)

---

## 3. Target Users

### Primary User

**Jongkuk Lim** - Developer who built and uses cclv daily

**Usage Patterns:**
- Reviews Claude Code conversation logs after coding sessions
- Uses standalone (`cclv <file>`) and via vibe-dash integration
- Runs `cclv --color=always <path>` from vibe-dash

### Secondary Users

Open-source users who discover cclv on GitHub. No special accommodation - "if they need it as is, they will use it."

---

## 4. Functional Requirements

### FR-100: Visual Polish

#### FR-101: Adaptive Color System
- **Description:** Implement design token system using `lipgloss.AdaptiveColor` for automatic light/dark theme support
- **Implementation:** Create `Theme` struct with Primary, Secondary, Accent, Text, Muted, Background colors
- **Acceptance Criteria:**
  - [ ] Colors automatically adapt to terminal theme (light/dark)
  - [ ] Consistent color palette across all UI elements
  - [ ] No hardcoded color values in view code

#### FR-102: Rounded Border Styling
- **Description:** Replace `NormalBorder()` with `RoundedBorder()` for message cards
- **Implementation:** Update styles.go to use rounded corners (╭ ╮ ╰ ╯)
- **Acceptance Criteria:**
  - [ ] User messages display with rounded border cards
  - [ ] Assistant messages display with rounded border cards
  - [ ] Tool calls display with rounded border cards

#### FR-103: Segmented Status Bar
- **Description:** Create visually distinct status bar sections with colored backgrounds
- **Implementation:** Use `lipgloss.JoinHorizontal()` with styled segments
- **Acceptance Criteria:**
  - [ ] Status bar shows distinct colored sections
  - [ ] Keyboard shortcuts visible in status bar
  - [ ] Entry count and position displayed

#### FR-104: Spinner Animation
- **Description:** Add animated spinner for loading states
- **Implementation:** Use `bubbles/spinner` with `spinner.Tick` command
- **Acceptance Criteria:**
  - [ ] Spinner displays during file loading
  - [ ] Spinner displays during file watching initialization
  - [ ] Spinner stops when operation completes

### FR-200: Real-time File Watching

#### FR-201: File Watch Mode Flag
- **Description:** Add CLI flag to enable file watching mode
- **Implementation:** Add `--watch` or `--live` flag to cmd/cclv
- **Acceptance Criteria:**
  - [ ] `cclv --watch <file>` enables watch mode
  - [ ] `cclv --live <file>` as alias
  - [ ] Flag documented in `--help` output

#### FR-202: fsnotify Integration
- **Description:** Watch log file for changes using fsnotify
- **Implementation:**
  - Create watcher goroutine with fsnotify
  - Send new entries via channel to Bubbletea
  - Use `tea.Cmd` chaining pattern for updates
- **Acceptance Criteria:**
  - [ ] New entries appear automatically when file changes
  - [ ] No restart required to see updates
  - [ ] Handles file truncation gracefully

#### FR-203: Auto-scroll on New Entries
- **Description:** Optionally auto-scroll to bottom when new entries arrive
- **Implementation:** Track user scroll position, auto-scroll if at bottom
- **Acceptance Criteria:**
  - [ ] Auto-scrolls when user is at bottom of list
  - [ ] Does not auto-scroll when user has scrolled up
  - [ ] Visual indicator when new entries available but not visible

### FR-300: Markdown Rendering

#### FR-301: Glamour Integration
- **Description:** Add Glamour dependency for markdown rendering
- **Implementation:**
  - Add `github.com/charmbracelet/glamour` to go.mod
  - Create renderer with `glamour.WithAutoStyle()`
  - Apply to assistant text content
- **Acceptance Criteria:**
  - [ ] Glamour dependency added and builds successfully
  - [ ] Assistant text renders as formatted markdown
  - [ ] Code blocks have syntax highlighting

#### FR-302: Dynamic Word Wrap
- **Description:** Wrap markdown content to terminal width
- **Implementation:** Use `glamour.WithWordWrap(width)` with viewport width
- **Acceptance Criteria:**
  - [ ] Markdown wraps to current terminal width
  - [ ] Re-renders on terminal resize
  - [ ] No horizontal scrolling needed

#### FR-303: Render Caching
- **Description:** Cache rendered markdown to avoid re-rendering
- **Implementation:**
  - Store rendered output per entry
  - Invalidate cache on terminal resize
  - Re-render only visible entries
- **Acceptance Criteria:**
  - [ ] Rendered content cached per entry
  - [ ] Cache invalidates on resize
  - [ ] Scroll performance remains smooth

---

## 5. Non-Functional Requirements

### NFR-001: Performance

| Metric | Target | Measurement |
|--------|--------|-------------|
| Startup time | < 100ms | `time cclv file.jsonl` |
| Memory (10k entries) | < 50MB | `go tool pprof` |
| Render FPS | 60 FPS (best effort) | Bubbletea default |
| File watch latency | < 200ms | Time from write to display |

### NFR-002: Compatibility

- **Terminal Support:** Works in standard terminals (iTerm2, Terminal.app, Alacritty, etc.)
- **Theme Support:** Adapts to light and dark terminal themes
- **Platform Support:** macOS, Linux (existing platforms)
- **Integration:** Maintains compatibility with vibe-dash (`--color=always` output)

### NFR-003: Code Quality

- **Test Coverage:** Maintain 90% coverage (existing requirement)
- **Architecture:** Follow TEA (Elm Architecture) pattern
- **Style:** Follow existing project conventions from project-context.md

---

## 6. Technical Constraints

### From project-context.md

| Constraint | Requirement |
|------------|-------------|
| No emoji in UI | Text icons only: `[U]`, `[A]`, `[T]`, `[>]` |
| Build system | Use Makefile (`make build`, `make test`) |
| Dependencies | Charm stack + Glamour only |
| List component | Must truncate `list.View()` output (known bug) |
| Import order | stdlib → external → internal |

### New Dependencies

| Package | Purpose | Status |
|---------|---------|--------|
| `github.com/charmbracelet/glamour` | Markdown rendering | Approved in project-context.md |
| `github.com/fsnotify/fsnotify` | File watching | To be approved |

### Architecture Constraints

- All state changes via `Update()` function
- Side effects only via `tea.Cmd`
- Never use raw goroutines in Bubbletea code
- Cache rendered content, never render in `View()`

---

## 7. Implementation Phases

| Phase | Features | Risk | Dependencies |
|-------|----------|------|--------------|
| **1** | Visual polish (AdaptiveColor, RoundedBorder, status bar) | Low | None |
| **2** | Real-time file watching (fsnotify, channel pattern) | Medium | Phase 1 styles |
| **3** | Markdown rendering (Glamour integration) | Medium | None |
| **4** | Integration, optimization, polish | Low | Phases 1-3 |

---

## 8. Out of Scope

- Streaming stdin (`tail -f | cclv`) - file watching sufficient
- Custom themes/configuration files
- Plugin system
- Multi-file viewing
- Remote file support
- Export functionality

---

## 9. Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Glamour adds binary bloat | Low | Low | Monitor size; Glamour is well-optimized |
| fsnotify platform issues | Medium | Low | Test on macOS and Linux |
| File watching memory leak | Medium | Medium | Use buffered channels, implement backpressure |
| Render cache invalidation bugs | Low | Medium | Comprehensive resize testing |

---

## 10. Dependencies

### External Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| Bubbletea | v1.3.10 | TUI framework |
| Lipgloss | v1.1.1 | Styling |
| Bubbles | v0.21.0 | UI components |
| Glamour | latest | Markdown rendering (new) |
| fsnotify | latest | File watching (new) |

### Internal Dependencies

| Component | Dependency |
|-----------|------------|
| Visual polish | styles.go refactor |
| File watching | parser integration |
| Markdown | viewport integration |

---

## 11. Acceptance Criteria Summary

### Phase 2 Complete When:

- [ ] **Visual Polish**
  - [ ] AdaptiveColor design tokens implemented
  - [ ] RoundedBorder styling on all message cards
  - [ ] Segmented status bar with colored sections
  - [ ] Spinner animation for loading states

- [ ] **Real-time File Watching**
  - [ ] `--watch`/`--live` CLI flag works
  - [ ] New entries appear without restart
  - [ ] Auto-scroll behavior correct

- [ ] **Markdown Rendering**
  - [ ] Glamour renders assistant text
  - [ ] Code blocks have syntax highlighting
  - [ ] Dynamic word wrap on resize
  - [ ] Render caching works

- [ ] **Quality**
  - [ ] All tests pass
  - [ ] 90% test coverage maintained
  - [ ] Startup time < 100ms
  - [ ] Memory < 50MB for 10k entries
  - [ ] Visual appearance satisfactory

---

## 12. Reference Documents

- **Product Brief:** `_bmad-output/planning-artifacts/product-brief-claude-code-log-viewer-cli-2026-01-15.md`
- **Technical Research:** `_bmad-output/planning-artifacts/research/technical-tui-streaming-markdown-polish-research-2026-01-15.md`
- **Project Context:** `_bmad-output/project-context.md`

---

## Next Steps

1. **Architecture** - `/bmad:bmm:workflows:create-architecture` - Document technical decisions
2. **Epics & Stories** - `/bmad:bmm:workflows:create-epics-and-stories` - Break into implementation tasks

---

*PRD Complete - 2026-01-15*
