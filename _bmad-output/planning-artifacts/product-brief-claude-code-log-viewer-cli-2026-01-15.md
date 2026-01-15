---
stepsCompleted: [1, 2, 3, 4, 5, 6]
inputDocuments:
  - '_bmad-output/planning-artifacts/research/CONTINUATION-PLAN.md'
  - '_bmad-output/planning-artifacts/research/technical-tui-streaming-markdown-polish-research-2026-01-15.md'
  - '_bmad-output/project-context.md'
date: '2026-01-15'
author: 'Jongkuk Lim'
phase: 2
status: complete
---

# Product Brief: cclv Visual & Streaming Enhancements (Phase 2)

## Executive Summary

cclv (Claude Code Log Viewer) Phase 2 is a craft project to enhance a personal developer tool. The goal is simple: make it look good, explore what's possible, and add quality-of-life improvements that spark curiosity. This isn't about solving market problems - it's about building something to be proud of.

---

## Core Vision

### Problem Statement

cclv works, but it could feel better. The visuals are utilitarian, there's no way to watch logs update in real-time, and markdown displays as raw text. These aren't urgent problems - they're opportunities to make the tool more polished and capable.

### Motivation

- **Visual Polish** - Personal pride. A tool worth looking at.
- **Real-time Log Watching** - Curiosity. "Can I make this work?"
- **Markdown Rendering** - Nice to have. Better readability.

### Context

cclv is used standalone and integrated into vibe-dash (which calls `cclv --color=always <path>`). Visual improvements benefit both use cases. Real-time watching is for standalone exploration.

### Proposed Solution

Three enhancements, prioritized by personal interest:
1. **Visual Polish** - AdaptiveColor design tokens, rounded borders, segmented status bar, spinner animations
2. **Real-time File Watching** - Auto-update when log file changes (fsnotify-based)
3. **Markdown Rendering** - Glamour-based rendering for assistant text

### Success Criteria

It feels finished. Pride in the craft.

---

## Target Users

### Primary User

**Jongkuk Lim** - The developer who built it for personal use.

**Usage Context:**
- Reviews Claude Code conversation logs after coding sessions
- Uses cclv both standalone (`cclv <file>`) and integrated through vibe-dash
- Wants the tool to feel polished and capable

**What Success Looks Like:**
- Tool feels finished and worth using
- Visual output is something to be proud of
- Curiosity-driven features work as expected

### Secondary Users

**Open-source users** - Developers who discover cclv on GitHub and find it useful.

**Approach:** "If they need it as is, they will use it." No custom features or special accommodation. The tool serves its purpose; those who find it valuable can use it.

### User Journey

**Discovery:** GitHub search or word-of-mouth
**Onboarding:** `go install` and run
**Core Usage:** View Claude Code logs in terminal
**Value Moment:** "This is cleaner than reading raw JSONL"

---

## Success Metrics

### Completion Criteria

Phase 2 is complete when:
- [ ] Visual Polish implemented (AdaptiveColor, rounded borders, status bar, spinners)
- [ ] Real-time File Watching works (fsnotify-based auto-update)
- [ ] Markdown Rendering works (Glamour integration)
- [ ] Visual appearance matches personal taste

### Technical Targets

| Metric | Target | Priority |
|--------|--------|----------|
| Startup time | < 100ms | Target |
| Memory (10k entries) | < 50MB | Target |
| Render FPS | 60 FPS | Ideal (best effort) |
| Binary size | < 15MB | Not a concern |

### Quality Bar

Success is subjective: "I'll know it when I see it."

No business objectives or KPIs - this is a personal craft project.

---

## Phase 2 Scope

### Core Features

**Feature 1: Visual Polish** (Priority: High, Risk: Low)
- AdaptiveColor design token system for light/dark theme support
- RoundedBorder() card styling for messages
- Segmented status bar with colored sections
- Spinner animation for loading states

**Feature 2: Real-time File Watching** (Priority: High, Risk: Medium)
- Auto-update when log file changes (fsnotify-based)
- No restart needed - entries appear as Claude writes them
- New `--watch` or `--live` CLI flag

**Feature 3: Markdown Rendering** (Priority: Medium, Risk: Medium)
- Glamour-based rendering for assistant text content
- WithAutoStyle() for automatic theme detection
- Cache rendered output, invalidate on terminal resize

### Implementation Phases

| Phase | Features | Risk |
|-------|----------|------|
| 1 | Visual polish (borders, colors, adaptive theme) | Low |
| 2 | Real-time file watching | Medium |
| 3 | Glamour markdown rendering | Medium |
| 4 | Integration + optimization | Low |

### Out of Scope

- Streaming stdin (`tail -f | cclv`) - file watching is sufficient
- Custom themes/configuration files
- Plugin system
- Multi-file viewing

### Future Vision

Phase 2 represents the "finished" state for cclv. No additional features planned beyond these three. If new ideas emerge during use, they'll be evaluated then.

---

## Technical Reference

Detailed implementation guidance available in:
- `_bmad-output/planning-artifacts/research/technical-tui-streaming-markdown-polish-research-2026-01-15.md`

Key patterns:
- TEA (Elm Architecture) for all state management
- Channel + tea.Cmd chaining for real-time updates
- Glamour with WithAutoStyle() for markdown
- AdaptiveColor for theme-aware styling

---

## Next Steps

1. **PRD** - `/bmad:bmm:workflows:prd` - Detailed requirements
2. **Architecture** - `/bmad:bmm:workflows:create-architecture` - Technical decisions
3. **Stories** - `/bmad:bmm:workflows:create-epics-and-stories` - Implementation tasks

---

*Product Brief Complete - 2026-01-15*
