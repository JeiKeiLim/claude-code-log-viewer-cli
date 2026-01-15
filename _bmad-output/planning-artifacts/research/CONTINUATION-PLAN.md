# cclv Enhancement Continuation Plan

**Created:** 2026-01-15
**Status:** Research Complete - Ready for Product Brief

---

## What Was Completed

### Technical Research (DONE)
- **Document:** `technical-tui-streaming-markdown-polish-research-2026-01-15.md`
- **Topics Covered:**
  1. Streaming stdin (`tail -f` support)
  2. Markdown rendering with Glamour
  3. Visual polish with Lipgloss
  4. Integration patterns
  5. Architectural patterns
  6. Implementation recommendations

---

## Key Decisions from Research

### 1. Streaming stdin
- **Approach:** Polling stdin (100ms interval) + channel + tea.Cmd chaining
- **Dependencies:** None (use stdlib)
- **Pattern:** `tea.WithInputTTY()` for piped input with keyboard

### 2. Markdown Rendering
- **Approach:** Add Glamour dependency
- **Config:** `WithAutoStyle()` + `WithWordWrap(width)`
- **Caching:** Cache rendered output, invalidate on resize
- **NOTE:** Reverses "No Glamour" constraint in project-context.md

### 3. Visual Polish
- **Colors:** Implement `AdaptiveColor` design token system
- **Borders:** Switch from `NormalBorder()` to `RoundedBorder()`
- **Animation:** Add spinner for loading states
- **Status bar:** Create segmented colored sections

---

## Next Steps (BMAD Method)

### Step 1: Create Product Brief (NEXT)
Run: `/bmad:bmm:workflows:create-product-brief`

**Brief should cover:**
- Feature: "cclv Visual & Streaming Enhancements"
- Three features: Visual Polish, Streaming stdin, Markdown Rendering
- Reference the research document for technical details

### Step 2: PRD
Run: `/bmad:bmm:workflows:prd`

### Step 3: Architecture
Run: `/bmad:bmm:workflows:create-architecture`

### Step 4: Epics & Stories
Run: `/bmad:bmm:workflows:create-epics-and-stories`

### Step 5: Implementation
Run: `/bmad:bmm:workflows:dev-story`

---

## Implementation Phases (from Research)

| Phase | Features | Risk |
|-------|----------|------|
| **Phase 1** | Visual polish (borders, colors, adaptive theme) | Low |
| **Phase 2** | Streaming stdin support | Medium |
| **Phase 3** | Glamour markdown rendering | Medium |
| **Phase 4** | Integration + optimization | Low |

---

## Files to Reference

- **Research:** `_bmad-output/planning-artifacts/research/technical-tui-streaming-markdown-polish-research-2026-01-15.md`
- **Current styles:** `internal/tui/styles.go`
- **Viewer:** `internal/tui/viewer.go`
- **Parser:** `internal/parser/jsonl.go`
- **Project context:** `_bmad-output/project-context.md`

---

## Constitution Update Required

Before implementation, update `_bmad-output/project-context.md`:
- Remove or update the "No Glamour" constraint
- Add Glamour as approved dependency

---

## Quick Resume Command

To continue from here, start a new session and run:
```
/bmad:bmm:workflows:create-product-brief
```

Then reference this file and the research document for context.

---

## Product Brief Creation Plan

### When Creating the Product Brief, Provide This Context:

**Product Name:** cclv Visual & Streaming Enhancements

**Problem Statement:**
cclv (Claude Code Log Viewer) has passed MVP phase but needs polish:
1. Visual appearance is functional but not appealing
2. No support for `tail -f` streaming input
3. No markdown rendering for assistant responses

**Target Users:**
- Developers using Claude Code who want to review conversation logs
- Users who want to tail live log files while Claude is running

**Features to Include:**

#### Feature 1: Visual Polish (Priority: High)
- Implement AdaptiveColor design token system (light/dark theme aware)
- Switch from NormalBorder() to RoundedBorder() for card styling
- Add segmented status bar with colored sections
- Add spinner animation for loading states

#### Feature 2: Streaming stdin Support (Priority: High)
- Add `--streaming` CLI flag
- Support `tail -f log.jsonl | cclv --streaming`
- Use polling stdin (100ms interval) with channel + tea.Cmd pattern
- Use `tea.WithInputTTY()` for keyboard input while piped

#### Feature 3: Markdown Rendering (Priority: Medium)
- Add Glamour dependency (already approved in project-context.md)
- Render assistant text content through Glamour
- Use WithAutoStyle() for automatic theme detection
- Cache rendered output, invalidate on terminal resize

**Technical Constraints:**
- Must follow Bubbletea TEA (Elm Architecture) pattern
- No emoji (text icons only per FR-017)
- Use Makefile for all builds
- Maintain 90% test coverage

**Success Metrics:**
- Startup time < 100ms
- Memory usage < 50MB for 10k entries
- Stable 60 FPS rendering
- Binary size < 15MB

**Reference Documents:**
- Technical Research: `_bmad-output/planning-artifacts/research/technical-tui-streaming-markdown-polish-research-2026-01-15.md`
- Project Context: `_bmad-output/project-context.md`

---

## After Product Brief

### PRD Creation
Run: `/bmad:bmm:workflows:prd`

Reference the product brief and break down into:
- Functional requirements for each feature
- Non-functional requirements (performance, etc.)
- User stories overview

### Architecture
Run: `/bmad:bmm:workflows:create-architecture`

Key decisions to document:
- Design token system structure
- Streaming integration with existing parser
- Glamour renderer initialization and caching
- State management for new features

### Stories
Run: `/bmad:bmm:workflows:create-epics-and-stories`

Suggested epic structure:
1. Epic: Visual Polish
2. Epic: Streaming stdin
3. Epic: Markdown Rendering
4. Epic: Integration & Testing
