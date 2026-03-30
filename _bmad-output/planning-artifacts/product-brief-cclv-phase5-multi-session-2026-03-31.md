---
stepsCompleted: [1, 2, 3, 4, 5, 6]
inputDocuments:
  - '_bmad-output/planning-artifacts/prd-phase3.md'
  - '_bmad-output/project-context.md'
  - 'docs/lessons-learned.md'
  - 'CLAUDE.md'
date: '2026-03-31'
author: 'Jongkuk Lim'
phase: 5
status: complete
---

# Product Brief: cclv Phase 5 - Multi-Agent Session Viewer

## Executive Summary

cclv Phase 5 introduces a **single-project multi-session dashboard** that auto-detects active Claude Code sessions, displays them in a live split-view, and dynamically manages pane lifecycle as sessions open and close. This addresses the growing need to monitor concurrent Claude agents - especially with Claude Code's multi-agent (subagent) capability and custom plugins that spawn multiple simultaneous Claude sessions.

**Key Insight:** Claude Code (>= v2.1.81) maintains active session metadata in `~/.claude/sessions/{pid}.json` with PID-based liveness. The `sessionId` directly matches the JSONL filename, making session-to-file mapping trivial. Older sessions require fallback file-activity detection.

---

## Core Vision

### Problem Statement

Claude Code now supports multi-agent workflows where a parent session spawns subagents, and power users run custom plugins that launch multiple Claude sessions simultaneously against the same project. Current cclv has no way to:

1. **Monitor concurrent sessions** - Dashboard mode shows one conversation per project. When 3-5 agents run simultaneously, the user must manually switch between conversations to see what each agent is doing.

2. **Detect active vs. closed sessions** - All conversations appear the same whether actively being written to or finished hours ago. No visual distinction between live and archived sessions.

3. **Follow multi-agent workflows** - Parent-child agent relationships are invisible. Subagent JSONL files exist in `{uuid}/subagents/` but cclv doesn't discover or display them.

4. **Auto-manage session lifecycle** - When an agent finishes, the user must manually navigate away. No automatic cleanup or session-end indication.

### Motivation

- **Daily pain point** - Running multiple agents across the same project is increasingly common but impossible to follow in real-time
- **Multi-agent is the future** - Claude Code's Agent tool spawns specialized subagents; this pattern will only grow
- **Plugin integration** - Custom plugins (e.g., vibe-dash) that orchestrate multiple Claude sessions need visibility into all concurrent activities

### Proposed Solution

A **Single-Project Session Dashboard** mode that:

1. Enters from project selection (new action: "Watch Active Sessions")
2. Scans `~/.claude/sessions/` for PIDs matching the selected project's working directory
3. Maps active session IDs to conversation JSONL files
4. Displays each active session in a split-view pane with live content streaming
5. Watches for new session creation and auto-adds panes
6. Detects session closure (PID exit) and auto-removes panes
7. Optionally shows subagent sessions as nested/child panes

---

## Target Users

### Primary User

**Jongkuk Lim** - Developer who runs multiple Claude Code agents simultaneously via custom plugins and Claude Code's native multi-agent feature.

**Usage Context:**
- Launches 2-5 concurrent Claude agents against the same project
- Needs real-time visibility into what each agent is doing
- Wants automatic lifecycle management (open/close panes as sessions start/stop)
- Uses custom plugins that orchestrate multi-agent workflows

---

## Feature Sets

### Feature Set 1: Active Session Detection (Foundation)

**Goal:** Reliably detect which Claude Code sessions are currently active for a given project.

**Approach (Dual-Mode - validated 2026-03-31):**

**Primary (Claude >= v2.1.81):**
- Scan `~/.claude/sessions/{pid}.json` files
- Parse session metadata: `pid`, `sessionId`, `cwd`, `startedAt`, `kind`, `entrypoint`
- Verify PID liveness via `syscall.Kill(pid, 0)` (cross-platform)
- Filter sessions by `cwd` matching selected project path
- JSONL file = `{sessionId}.jsonl` (direct filename match - no cross-referencing needed!)

**Fallback (Older Claude versions / Subagents):**
- Scan project JSONL files by modification time
- Watch with fsnotify for write events
- Files actively being written to = active session

**Session File Structure:**
```json
{
  "pid": 60696,
  "sessionId": "e10c86ca-6cd9-4716-b905-576810a52484",
  "cwd": "/Users/limjk/GitHub/JeiKeiLim/podcast-gen-web",
  "startedAt": 1774909391881,
  "kind": "interactive",
  "entrypoint": "sdk-cli"
}
```

**Validated: sessionId == JSONL filename.** No mapping logic needed.

**Detection Tiers:**

| Tier | Method | Reliability | Use Case |
|------|--------|-------------|----------|
| 1 | PID file + kill(pid, 0) | Definitive | Primary (>= v2.1.81) |
| 2 | fsnotify on JSONL files | High | Real-time activity + fallback |
| 3 | File mtime staleness | Medium | Last resort |

**Version Note:** Session files were introduced ~v2.1.81 (around Mar 28, 2026). Sessions started on older versions have no session file. As users update Claude Code, this becomes less of an issue.

### Feature Set 2: Session Dashboard View (Core UX)

**Goal:** Split-view display of all active sessions within a single project.

**Layout:**
- Reuses existing dashboard grid layout algorithm (1x1 to 3x3)
- Each pane shows one active session's live content
- Pane header shows: session start time, agent type (if subagent), PID
- Pane content shows latest conversation entries (same as current dashboard panes)

**Dynamic Pane Management:**
- New session detected → animate pane addition, reflow grid
- Session closes → animate pane removal, reflow grid
- If all sessions close → show "No active sessions" with option to return to project view

**Navigation:**
- Arrow keys / hjkl to focus panes (existing pattern)
- Enter to open focused session in full viewer with watch mode
- Esc to return to project list
- `r` / `R` for manual refresh (existing pattern)

### Feature Set 3: Subagent Discovery (Enhancement)

**Goal:** Discover and optionally display subagent sessions alongside parent sessions.

**Approach:**
- When a parent conversation has a `{uuid}/subagents/` directory, scan for `agent-*.jsonl` files
- Read `agent-*.meta.json` for agent type and description
- Display subagent panes with visual nesting indicator (indented header or parent reference)

**Subagent Metadata (validated - simpler than expected):**
```json
{"agentType": "Explore"}
```

Note: Only `agentType` field observed in practice. No `description` field.

**Display:** Parent session pane shows "[+N subagents]" indicator. Toggle with `s` key to expand/collapse subagent panes.

---

## Technical Feasibility

### Architecture Alignment

The existing dashboard architecture (`internal/tui/dashboard.go`) provides direct reuse:

| Existing Pattern | Reuse |
|-----------------|-------|
| Grid layout (1x1 to 3x3) | Direct reuse |
| PaneModel with watcher | One pane per active session |
| Context-based goroutine lifecycle | Session open/close |
| Subscription polling (100ms) | Session liveness + content |
| Pane-indexed message routing | Already dispatches by index |
| Non-blocking channel I/O | Handles burst events |

### New Components

| Component | Estimated Size | Purpose |
|-----------|---------------|---------|
| `internal/scanner/sessions.go` | ~150 lines | Session detection + PID liveness |
| `internal/tui/session_dashboard.go` | ~500 lines | Dynamic session dashboard |
| `internal/watcher/session_watcher.go` | ~100 lines | Watch sessions dir for changes |
| `app.go` modifications | ~80 lines | New view state + transitions |
| `project.go` modifications | ~30 lines | "Watch Active Sessions" action |

### Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Session file race (created before JSONL) | Pane shows empty briefly | Retry JSONL discovery with short backoff |
| Orphaned session files (crash) | False "active" detection | PID liveness check handles dead PIDs |
| >9 concurrent sessions | Exceeds grid layout | Show most recent N; cap at 3x3 = 9 |
| Session file format changes | Detection breaks | Defensive parsing with graceful degradation |
| Subagent detection complexity | Scope creep | Phase subagent support separately |
| Cross-platform PID checking | Linux/Windows variance | Go `os.FindProcess` + Signal(0) is portable; test on CI |

---

## Scope & Phasing

### Phase 5a: MVP - Session Detection + Dashboard (Priority: Must Have)

**Stories:**
1. **Session Detector** - Scan `~/.claude/sessions/`, parse metadata, check PID liveness, filter by project
2. **Session-to-Conversation Mapper** - Cross-reference sessionId to find JSONL file path
3. **Session Dashboard Model** - Dynamic pane dashboard with session lifecycle management
4. **Session Directory Watcher** - Watch `~/.claude/sessions/` for new file creation / deletion
5. **PID Liveness Polling** - Periodic check (every 2-5s) for session closure detection
6. **Project View Integration** - "Watch Active Sessions" action in project list
7. **Graceful Empty State** - Handle zero active sessions, all sessions closing

### Phase 5b: Subagent Support (Priority: Should Have)

**Stories:**
1. **Subagent Directory Scanner** - Discover `{uuid}/subagents/` and parse agent JSONL + meta.json
2. **Parent-Child Pane Display** - Visual nesting indicator, expand/collapse toggle
3. **Subagent Lifecycle** - Detect subagent completion (file stops growing)

### Phase 5c: Polish (Priority: Nice to Have)

**Stories:**
1. **Session Summary on Close** - Show duration, token usage, message count when session ends
2. **Session Type Indicators** - Visual distinction for interactive vs. SDK vs. subagent sessions
3. **Keyboard Shortcuts** - Quick-switch between sessions by number (1-9)
4. **Session Timeline** - Compact timeline showing when sessions started/stopped

---

## Success Criteria

- Active sessions detected within 2 seconds of cclv entering session dashboard mode
- Session closure detected within 5 seconds of PID exit
- New session appearance within 3 seconds of creation
- Dashboard renders at 60fps with up to 9 concurrent session panes
- No goroutine leaks on session open/close cycles
- `make ci` passes with >= 90% coverage on new code

---

## Dependencies

- No new external dependencies (uses existing fsnotify + Charm stack)
- Relies on Claude Code's `~/.claude/sessions/` file format (undocumented but stable)
- Cross-platform: macOS primary, Linux secondary, Windows best-effort

---

## Open Questions (Partially Resolved via Validation)

1. **Session file stability** - ~~Is it a stable interface?~~ **PARTIALLY ANSWERED:** Session files exist in current versions (>= v2.1.81) but are an undocumented internal detail. Format may change. **Decision:** Use them as primary detection with file-activity fallback. Accept the coupling.

2. **Subagent session files** - ~~Do subagents create their own entries?~~ **ANSWERED: NO.** Subagents only create `agent-{id}.jsonl` and `agent-{id}.meta.json` in the parent conversation's `subagents/` directory. Detection relies on file activity.

3. **Session-to-conversation mapping edge cases** - ~~How long is the gap?~~ **ANSWERED: NOT AN ISSUE.** `sessionId` == JSONL filename. No cross-referencing needed. If the JSONL file doesn't exist yet, we simply wait for its creation via fsnotify.

4. **Plugin sessions** - Do plugin-spawned Claude sessions (e.g., from vibe-dash) create standard session files? **STILL OPEN** - Needs testing with vibe-dash. The `entrypoint: "sdk-cli"` field suggests SDK-spawned sessions DO create files.

5. **Maximum concurrent sessions** - **STILL OPEN** - Observed 8 simultaneous claude processes in a typical workflow. Grid cap of 9 seems adequate, but may need scrollable list for heavy multi-agent plugins.

---

*Last Updated: 2026-03-31*
