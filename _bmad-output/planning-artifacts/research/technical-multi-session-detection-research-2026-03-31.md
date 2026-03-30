---
date: '2026-03-31'
type: 'technical-research'
topic: 'Multi-Agent Session Detection for cclv Phase 5'
status: complete
---

# Technical Research: Multi-Agent Session Detection

## 1. Claude Code Session File System Layout

### Session Metadata (`~/.claude/sessions/`)

Active Claude Code sessions maintain a JSON metadata file named by PID:

```
~/.claude/sessions/
├── 60696.json
├── 72541.json
└── ...
```

**File format:**
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

**Key fields:**
- `pid` - OS process ID (used for liveness check)
- `sessionId` - UUID matching JSONL entry `sessionId` fields
- `cwd` - Working directory (used to filter by project)
- `startedAt` - Unix timestamp in milliseconds
- `kind` - Session type: "interactive", possibly others
- `entrypoint` - How session was started: "sdk-cli", "cli", etc.

**Lifecycle:** File is created when session starts, and removed when session closes normally. If Claude crashes, the file may remain with a dead PID.

### Conversation JSONL Files (`~/.claude/projects/`)

```
~/.claude/projects/
├── -Users-limjk-GitHub-project-name/
│   ├── {uuid}.jsonl                    # Main conversation log
│   ├── {uuid}/                         # Companion directory
│   │   ├── subagents/
│   │   │   ├── agent-{id}.jsonl        # Subagent conversation
│   │   │   └── agent-{id}.meta.json    # Subagent metadata
│   │   └── tool-results/               # Cached tool outputs
│   └── ...
```

### Subagent Metadata

```json
{
  "agentType": "Explore",
  "description": "Explore podcast-gen-web codebase"
}
```

Agent types observed: "Explore", "general-purpose", "Plan"

### JSONL Entry Structure (Relevant Fields)

```json
{
  "type": "user|assistant|file-history-snapshot|progress",
  "sessionId": "99756956-11e9-4768-9ac0-29dd172f23dd",
  "uuid": "unique-message-id",
  "parentUuid": "parent-message-uuid",
  "timestamp": "2026-03-30T22:25:54.634Z",
  "isSidechain": false,
  "message": {
    "role": "user|assistant",
    "model": "claude-opus-4-6",
    "usage": { "input_tokens": 123, "output_tokens": 456 }
  }
}
```

---

## 2. Session Detection Methods

### Method 1: PID File + Process Check (Recommended Primary)

**How it works:**
1. Read all `~/.claude/sessions/{pid}.json` files
2. Parse JSON to extract `pid`, `sessionId`, `cwd`
3. Call `syscall.Kill(pid, 0)` - returns nil if process exists, error if not
4. Filter by `cwd` matching the target project path

**Go implementation sketch:**
```go
func (d *SessionDetector) DetectActiveSessions(projectPath string) ([]ActiveSession, error) {
    entries, _ := os.ReadDir(sessionsDir)
    var active []ActiveSession
    for _, entry := range entries {
        data, _ := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
        var meta SessionMeta
        json.Unmarshal(data, &meta)

        // Check if PID is alive
        if err := syscall.Kill(meta.PID, 0); err != nil {
            continue // Dead process
        }

        // Check if session belongs to target project
        if meta.CWD == projectPath {
            active = append(active, ActiveSession{Meta: meta})
        }
    }
    return active, nil
}
```

**Pros:** Definitive, fast (~1ms per check), no false positives for dead processes
**Cons:** Relies on `~/.claude/sessions/` format; orphaned files exist briefly after crash
**Cross-platform:** `syscall.Kill(pid, 0)` works on macOS/Linux. Windows needs `os.FindProcess` + handle check.

### Method 2: fsnotify on JSONL Files (Recommended Secondary)

**How it works:**
- Already implemented in `internal/watcher/watcher.go`
- Watch JSONL files for `Write` events
- Active sessions produce writes; closed sessions stop writing

**Pros:** Real-time, already battle-tested in cclv
**Cons:** Cannot detect session start (file must already exist); idle sessions may appear closed

### Method 3: File Modification Time Staleness

**How it works:**
- Stat JSONL files, check `ModTime`
- If not modified within threshold (e.g., 60 seconds), consider inactive

**Pros:** Simple, no dependencies
**Cons:** Unreliable - agent may be waiting for long tool execution; threshold is arbitrary

### Recommended Strategy: Tier 1 + Tier 2 Hybrid

1. **Discovery:** PID file scan (authoritative list of active sessions)
2. **Activity:** fsnotify on JSONL files (real-time content updates)
3. **Closure:** PID polling every 2-5 seconds (detect session end)

---

## 3. Session-to-Conversation JSONL Mapping

### ~~Challenge~~ SOLVED: sessionId == Filename

**Initial assumption:** Session files contain a `sessionId` that differs from the JSONL filename, requiring cross-referencing.

**Validated reality:** The `sessionId` in the session file IS the JSONL filename. Mapping is a simple string concatenation:

```go
jsonlPath := filepath.Join(projectDir, sessionMeta.SessionID + ".jsonl")
```

**Verified example:**
- Session file: `{"sessionId": "fa2e2259-4493-4f50-9525-e792c5b979e6", ...}`
- JSONL file: `~/.claude/projects/{project}/fa2e2259-4493-4f50-9525-e792c5b979e6.jsonl`

No scanning, no first-line parsing, no mtime optimization needed. Zero-cost mapping.

---

## 4. Subagent Detection

### Discovery

When a conversation `{uuid}.jsonl` has a companion directory `{uuid}/subagents/`, that directory contains subagent JSONL files.

### Challenges

1. **No session file for subagents** - Subagents likely don't create entries in `~/.claude/sessions/`. They run within the parent process.
2. **Liveness detection** - Must rely on file activity (fsnotify or mtime), not PID
3. **Relationship** - Need to map parent conversation UUID to subagent directory

### Approach

1. For each active parent session, check if `{conversation-uuid}/subagents/` exists
2. Watch that directory with fsnotify for new `agent-*.jsonl` files
3. Read `agent-*.meta.json` for display metadata
4. Consider subagent "active" if its JSONL has been modified recently or is being watched

---

## 5. Existing cclv Architecture Compatibility

### Direct Reuse (No Modification Needed)

| Component | Location | Reuse |
|-----------|----------|-------|
| Grid layout algorithm | `dashboard.go` | Exact same grid logic |
| PaneModel pattern | `dashboard.go` | One pane per session |
| Subscription polling | `dashboard.go` | 100ms tick for events |
| File watcher | `watcher/watcher.go` | Watch session JSONL files |
| Project watcher | `watcher/project_watcher.go` | Watch sessions directory |
| JSONL parser | `parser/jsonl.go` | Parse session entries |
| Entry rendering | `dashboard.go` | renderPaneEntry reuse |
| Context lifecycle | `dashboard.go` | Goroutine management |

### Requires Extension

| Component | Change Needed |
|-----------|--------------|
| `app.go` | New `viewSessionDashboard` state |
| `project.go` | "Watch Active Sessions" action (when active sessions exist) |
| `scanner/` | New `sessions.go` for session detection |
| `watcher/` | New `session_watcher.go` for sessions dir monitoring |
| `types/` | New `Session` type |

### New Components

| Component | Purpose |
|-----------|---------|
| `scanner/sessions.go` | Session file parsing, PID liveness, project filtering |
| `tui/session_dashboard.go` | Dynamic pane management, session lifecycle |
| `watcher/session_watcher.go` | Watch `~/.claude/sessions/` for changes |
| `types/session.go` | ActiveSession, SessionMeta types |

---

## 6. Cross-Platform Considerations

### PID Liveness Check

| Platform | Method | Notes |
|----------|--------|-------|
| macOS | `syscall.Kill(pid, 0)` | Works perfectly |
| Linux | `syscall.Kill(pid, 0)` | Works perfectly |
| Windows | `os.FindProcess(pid)` then `process.Signal(syscall.Signal(0))` | Needs testing |

### File Birthtime (Existing Pattern)

Already handled in `scanner/birthtime_*.go` with platform-specific implementations.

### fsnotify

Already works cross-platform via the `fsnotify` dependency.

---

## 7. Performance Estimates

| Operation | Estimated Time | Frequency |
|-----------|---------------|-----------|
| Scan sessions dir | <5ms | On entry + fsnotify events |
| PID liveness check | <1ms per PID | Every 2-5 seconds |
| Session-to-JSONL mapping | <10ms (first scan) | On new session detection |
| JSONL first-line parse | <1ms per file | On new session detection |
| Pane content update | <5ms per pane | On fsnotify write events |
| Grid reflow | <1ms | On pane add/remove |

**Total overhead:** Negligible. The 100ms subscription tick is the bottleneck (same as existing dashboard).

---

## 8. Live Validation Results (2026-03-31)

Empirical testing against 8 running Claude processes revealed critical corrections to initial assumptions:

### Finding 1: Session Files Are Version-Dependent (CRITICAL)

| PID | Started | Claude Version | Session File? | Entry Point |
|-----|---------|---------------|---------------|-------------|
| 61296 | Mar 16 | ~2.1.76 | NO | `claude` |
| 69819 | Mar 17 | 2.1.76 | NO | `claude` (our session) |
| 67381 | Mar 17 | ~2.1.76 | NO | `claude` |
| 19732 | Mar 20 | ~2.1.76 | NO | `claude` |
| 89198 | Mar 23 | ~2.1.78 | NO | `claude` |
| 88801 | Mar 25 | ~2.1.78 | NO | `claude --resume` |
| 75905 | Mar 28 | 2.1.81 | **YES** | `claude --resume` |
| 37297 | Mar 31 | 2.1.87 | **YES** | `claude -p` |

**Conclusion:** Session files were introduced around Claude Code **v2.1.79-2.1.81** (between Mar 25-28). Sessions started on older versions do NOT retroactively create session files. Going forward, all new sessions will have them.

**Impact on design:** Must support fallback detection (file activity) for sessions started on older Claude versions, at least during transition period.

### Finding 2: sessionId == JSONL Filename (SIMPLIFICATION)

The `sessionId` in session files DIRECTLY MATCHES the JSONL filename:

```
Session file:  {"sessionId": "fa2e2259-4493-4f50-9525-e792c5b979e6", ...}
JSONL file:    fa2e2259-4493-4f50-9525-e792c5b979e6.jsonl
```

**No cross-referencing needed.** The session file's `sessionId` IS the JSONL filename. This eliminates the entire "session-to-conversation mapping" problem described in Section 3.

### Finding 3: Subagents Do NOT Create Session Files

Confirmed: subagents only produce:
- `{parent-uuid}/subagents/agent-{id}.jsonl` - conversation log
- `{parent-uuid}/subagents/agent-{id}.meta.json` - metadata

Meta.json contains ONLY `{"agentType": "Explore"}` - no `description` field observed despite earlier assumptions. Agent types seen: "Explore".

Subagent liveness must rely on file activity detection only.

### Finding 4: PID Liveness Check Works Reliably

```bash
kill -0 75905  # alive → exit 0
kill -0 99999  # dead → exit 1, "no such process"
```

Go equivalent: `syscall.Kill(pid, 0)` returns `nil` for alive, error for dead.

### Finding 5: Multiple cclv Instances Also Running

4 cclv instances were running alongside 8 claude processes. The session dashboard must not confuse cclv processes with Claude sessions.

### Finding 6: session-env Directory Exists

`~/.claude/session-env/` contains ~1484 UUID-named directories. Purpose unclear - not session detection related. Should be ignored.

---

## 9. Revised Detection Strategy

Based on validation findings, the detection strategy must be **dual-mode**:

### Primary: PID Session Files (Claude >= 2.1.81)

1. Scan `~/.claude/sessions/{pid}.json`
2. Parse JSON → extract `pid`, `sessionId`, `cwd`
3. `syscall.Kill(pid, 0)` for liveness
4. Filter by `cwd` matching project path
5. JSONL file = `{sessionId}.jsonl` (direct filename match!)

### Fallback: File Activity Detection (Older Sessions / Subagents)

1. Scan project's JSONL files by mtime (most recent first)
2. Watch with fsnotify for write events
3. Files actively being written to = active session
4. Consider "active" if modified within last 30 seconds
5. Mark as "activity-detected" (lower confidence than PID-verified)

### Visual Distinction

| Detection Method | Confidence | Pane Indicator |
|-----------------|-----------|----------------|
| PID verified | High | Solid border |
| File activity | Medium | Dashed border or dimmed header |
| Subagent (file activity) | Medium | Nested indicator + activity dot |

---

## 10. Revised Key Findings Summary

1. **Session files work but are version-gated** - Only Claude >= ~2.1.81 creates them. Fallback needed during transition.
2. **sessionId == filename** - No mapping logic needed! Session file's `sessionId` directly names the JSONL file.
3. **Subagents have no session files** - Must use file activity detection for subagent liveness.
4. **PID check is reliable** - `kill(pid, 0)` works perfectly on macOS/Linux.
5. **Meta.json is minimal** - Only contains `agentType`, not `description`.
6. **Dual-mode detection required** - PID files (primary) + file activity (fallback).
7. **Dashboard architecture still scales** - No architectural changes needed despite revised detection.

---

*Last Updated: 2026-03-31 (includes live validation results)*
