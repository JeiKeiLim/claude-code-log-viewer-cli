---
stepsCompleted: [discovery, analysis, feasibility]
inputDocuments: []
workflowType: 'research'
lastStep: 3
research_type: 'technical'
research_topic: 'Claude Code Subscription Usage Limits Monitoring'
research_goals: 'Determine feasibility of integrating 5-hour and weekly usage limit monitoring into cclv'
user_name: 'Jongkuk Lim'
date: '2026-01-20'
web_research_enabled: true
source_verification: true
---

# Technical Research Report: Claude Code Subscription Usage Limits Monitoring

**Date:** 2026-01-20
**Author:** Jongkuk Lim
**Research Type:** Technical Feasibility
**Project:** cclv (Claude Code Log Viewer)

---

## Executive Summary

**Feasibility: HIGH** - Monitoring Claude Code subscription usage limits (5-hour and weekly) is technically feasible for integration into cclv.

### Key Findings

| Finding | Status | Confidence |
|---------|--------|------------|
| OAuth Usage API exists | Confirmed | High |
| Returns 5-hour and 7-day limits | Confirmed | High |
| Includes reset timestamps | Confirmed | High |
| Credential access possible | Platform-dependent | High |
| No official documentation | Confirmed | High |

### Recommendation

Implement a new feature in cclv to display Claude Code subscription usage limits using the undocumented OAuth usage endpoint. This directly addresses the user's pain point of not wanting to visit the Claude web page to check limit status.

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Claude Code Usage Limit Architecture](#2-claude-code-usage-limit-architecture)
3. [OAuth Usage API Discovery](#3-oauth-usage-api-discovery)
4. [Credential Access Methods](#4-credential-access-methods)
5. [Technical Implementation Approach](#5-technical-implementation-approach)
6. [Existing Tools Analysis](#6-existing-tools-analysis)
7. [Risks and Limitations](#7-risks-and-limitations)
8. [Implementation Recommendation](#8-implementation-recommendation)
9. [Sources](#9-sources)

---

## 1. Problem Statement

### User Pain Point

> "I don't want to visit Claude's web page frequently just to check if I am reaching the limit of Claude's usage."

### Requirements

1. Monitor **5-hour rolling window** usage limits
2. Monitor **7-day weekly** usage limits
3. Display **when limits will reset** (not just current usage)
4. Integrate into cclv as a utility feature
5. Target: Claude Code CLI users (Pro/Max subscribers)

---

## 2. Claude Code Usage Limit Architecture

### Dual-Layer Usage Framework

Claude Code subscription limits operate on two concurrent windows:

| Window | Purpose | Reset Behavior |
|--------|---------|----------------|
| **5-Hour** | Burst activity control | Rolling window from first message |
| **7-Day** | Total usage ceiling | Rolling window (not calendar week) |

### 5-Hour Window Details

- Clock starts with **first message** in session (not midnight)
- Resets exactly 5 hours from first request
- Claude rounds to the hour (10:10 start → 15:00 reset)
- Plan limits:
  - Pro: ~10-40 Claude Code prompts per window
  - Max 5x: ~50-200 prompts per window
  - Max 20x: ~200-800 prompts per window

### 7-Day Window Details

- Rolling window, **not** calendar week
- Usage from 7 days ago continuously "expires"
- No single reset day - gradual refresh
- Shared across Claude web and Claude Code

### Shared Limits

Both Pro and Max plans offer usage limits that are **shared across Claude and Claude Code**, meaning all activity in both tools counts against the same usage limits.

**Source:** [Using Claude Code with your Pro or Max plan](https://support.claude.com/en/articles/11145838-using-claude-code-with-your-pro-or-max-plan)

---

## 3. OAuth Usage API Discovery

### Endpoint

```
GET https://api.anthropic.com/api/oauth/usage
```

**Note:** This is an **undocumented internal API** used by Claude Code. Not part of the official public API.

### Required Headers

| Header | Value |
|--------|-------|
| `Accept` | `application/json, text/plain, */*` |
| `Content-Type` | `application/json` |
| `User-Agent` | `claude-code/2.0.32` (or current version) |
| `Authorization` | `Bearer {oauth_access_token}` |
| `anthropic-beta` | `oauth-2025-04-20` |
| `Accept-Encoding` | `gzip, compress, deflate, br` |

### Response Format

```json
{
  "five_hour": {
    "utilization": 6.0,
    "resets_at": "2025-11-04T04:59:59.943648+00:00"
  },
  "seven_day": {
    "utilization": 35.0,
    "resets_at": "2025-11-06T03:59:59.943679+00:00"
  },
  "seven_day_oauth_apps": null,
  "seven_day_opus": {
    "utilization": 0.0,
    "resets_at": null
  },
  "iguana_necktie": null
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `five_hour.utilization` | float | Percentage of 5-hour limit used (0-100) |
| `five_hour.resets_at` | string/null | ISO 8601 timestamp when limit resets |
| `seven_day.utilization` | float | Percentage of weekly limit used (0-100) |
| `seven_day.resets_at` | string/null | ISO 8601 timestamp for weekly reset |
| `seven_day_opus` | object | Opus-specific weekly limits |

### TypeScript Interface

```typescript
export interface UsageLimits {
  five_hour: {
    utilization: number;
    resets_at: string | null;
  } | null;
  seven_day: {
    utilization: number;
    resets_at: string | null;
  } | null;
  seven_day_opus?: {
    utilization: number;
    resets_at: string | null;
  } | null;
}
```

**Source:** [How to Show Claude Code Usage Limits in Your Statusline](https://codelynx.dev/posts/claude-code-usage-limits-statusline)

---

## 4. Credential Access Methods

### macOS

Credentials stored in **macOS Keychain** under `"Claude Code-credentials"`.

**Retrieval command:**
```bash
security find-generic-password -s "Claude Code-credentials" -w
```

**Returns JSON:**
```json
{
  "claudeAiOauth": {
    "accessToken": "sk-ant-oaut...",
    "refreshToken": "...",
    "expiresAt": "..."
  }
}
```

### Linux

**No keychain support** - credentials stored in:
- Environment variable: `CLAUDE_CODE_OAUTH_TOKEN`
- Credentials file: `~/.claude/.credentials.json`

**Credentials file format:**
```json
{
  "claudeAiOauth": {
    "accessToken": "...",
    "refreshToken": "...",
    "expiresAt": "..."
  }
}
```

### Windows

Similar to Linux - uses credentials file in `%USERPROFILE%\.claude\.credentials.json`

### Go Implementation for Credential Access

```go
package usage

import (
    "encoding/json"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
)

type Credentials struct {
    ClaudeAiOauth struct {
        AccessToken  string `json:"accessToken"`
        RefreshToken string `json:"refreshToken"`
        ExpiresAt    string `json:"expiresAt"`
    } `json:"claudeAiOauth"`
}

func GetOAuthToken() (string, error) {
    switch runtime.GOOS {
    case "darwin":
        return getTokenFromKeychain()
    default:
        return getTokenFromFile()
    }
}

func getTokenFromKeychain() (string, error) {
    cmd := exec.Command("security", "find-generic-password",
        "-s", "Claude Code-credentials", "-w")
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }

    var creds Credentials
    if err := json.Unmarshal(output, &creds); err != nil {
        return "", err
    }
    return creds.ClaudeAiOauth.AccessToken, nil
}

func getTokenFromFile() (string, error) {
    // Check env var first
    if token := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"); token != "" {
        return token, nil
    }

    // Fall back to credentials file
    home, _ := os.UserHomeDir()
    credPath := filepath.Join(home, ".claude", ".credentials.json")

    data, err := os.ReadFile(credPath)
    if err != nil {
        return "", err
    }

    var creds Credentials
    if err := json.Unmarshal(data, &creds); err != nil {
        return "", err
    }
    return creds.ClaudeAiOauth.AccessToken, nil
}
```

**Source:** [Claude Code IAM Documentation](https://code.claude.com/docs/en/iam)

---

## 5. Technical Implementation Approach

### Architecture for cclv

```
┌─────────────────────────────────────────────────────────────┐
│                         cclv                                │
├─────────────────────────────────────────────────────────────┤
│  internal/usage/                                            │
│  ├── credentials.go    (OAuth token retrieval)              │
│  ├── client.go         (API client for usage endpoint)      │
│  └── types.go          (UsageLimits struct)                 │
├─────────────────────────────────────────────────────────────┤
│  internal/ui/                                               │
│  └── usage_widget.go   (TUI component for display)          │
└─────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────┐
│  https://api.anthropic.com/api/oauth/usage                  │
│  (Undocumented OAuth endpoint)                              │
└─────────────────────────────────────────────────────────────┘
```

### Feature Options

#### Option A: Status Bar Integration
Add usage info to existing cclv status bar:
```
[cclv] file.jsonl | 5h: 35% (resets 2h 15m) | 7d: 12% | Tokens: ~15,234
```

#### Option B: Dedicated Command/View
New `-u` or `--usage` flag for dedicated usage display:
```bash
cclv --usage
# or
cclv -u
```

Output:
```
Claude Code Usage Limits
========================

5-Hour Window
  Usage:    ████████░░░░░░░░░░░░  35%
  Resets:   2h 15m (2026-01-20 18:00 UTC)

7-Day Window
  Usage:    ██░░░░░░░░░░░░░░░░░░  12%
  Resets:   Rolling (oldest usage: 2026-01-13)

Opus Weekly
  Usage:    ░░░░░░░░░░░░░░░░░░░░  0%
  Status:   Not used this week
```

#### Option C: Dashboard Integration
Add usage pane to existing dashboard mode.

### Recommended Approach

**Option B (Dedicated Command)** - Reasons:
1. Clean separation of concerns
2. Non-intrusive to existing functionality
3. Can be used standalone without opening logs
4. Aligns with user's pain point (quick check without web)

---

## 6. Existing Tools Analysis

### ccusage
- **Approach:** Analyzes local JSONL files
- **Limitation:** Does NOT track reset times, only historical usage
- **Source:** [GitHub - ryoppippi/ccusage](https://github.com/ryoppippi/ccusage)

### Claude-Code-Usage-Monitor
- **Approach:** Real-time monitoring with predictions
- **Features:** Token consumption, cost estimates, ML-based predictions
- **Limitation:** Python-based, separate tool
- **Source:** [GitHub - Maciek-roboblog/Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor)

### claude-code-limit-tracker
- **Approach:** Status line integration
- **Features:** Per-model quotas, real-time display
- **Source:** [GitHub - TylerGallenbeck/claude-code-limit-tracker](https://github.com/TylerGallenbeck/claude-code-limit-tracker)

### Gap Analysis

| Feature | ccusage | Usage-Monitor | limit-tracker | cclv (proposed) |
|---------|---------|---------------|---------------|-----------------|
| 5-hour utilization | Yes | Yes | Yes | Yes |
| 5-hour reset time | No | Predicted | Yes | Yes (actual) |
| 7-day utilization | No | No | No | Yes |
| 7-day reset time | No | No | No | Yes |
| Go-based | No | No | No | Yes |
| Integrated viewer | No | No | No | Yes |

**Conclusion:** cclv can provide unique value by offering **actual reset times** for both windows in a **Go-based, integrated** tool.

---

## 7. Risks and Limitations

### Technical Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Undocumented API may change | Medium | Version-pin User-Agent, graceful degradation |
| OAuth token expiration | Low | Handle 401, prompt re-login |
| Cross-platform credential access | Medium | Test thoroughly, document requirements |
| API rate limiting | Low | Cache responses, reasonable polling |

### User Experience Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| User not logged into Claude Code | Medium | Clear error message with instructions |
| Stale credentials | Low | Detect and prompt refresh |
| Privacy concerns | Low | Document what data is accessed |

### API Stability Warning

> **Important:** The OAuth usage endpoint (`/api/oauth/usage`) is **not officially documented** by Anthropic. It is an internal API used by Claude Code. While stable for now, it could change without notice. Implementation should include graceful error handling.

---

## 8. Implementation Recommendation

### Recommended Scope (Epic 7 Candidate)

**Story 7.1: OAuth Credential Access**
- Implement cross-platform credential retrieval
- macOS Keychain, Linux/Windows file-based
- Error handling for missing credentials

**Story 7.2: Usage API Client**
- HTTP client for OAuth usage endpoint
- Response parsing with proper types
- Caching to avoid excessive API calls

**Story 7.3: Usage Display UI**
- New `--usage` / `-u` flag
- Progress bar visualization
- Time-until-reset formatting
- Optional status bar integration

### Estimated Effort

| Story | Complexity | Dependencies |
|-------|------------|--------------|
| 7.1 | Medium | None |
| 7.2 | Low | 7.1 |
| 7.3 | Medium | 7.2 |

### Success Criteria

1. User can run `cclv -u` to see current usage limits
2. Both 5-hour and 7-day utilization percentages displayed
3. Reset times shown in human-readable format ("2h 15m" or "resets at 18:00")
4. Works on macOS and Linux
5. Graceful error handling when not logged in

---

## 9. Sources

### Primary Sources

1. [How to Show Claude Code Usage Limits in Your Statusline](https://codelynx.dev/posts/claude-code-usage-limits-statusline) - OAuth endpoint discovery, response format
2. [Rate limits - Claude Docs](https://platform.claude.com/docs/en/api/rate-limits) - Official rate limit documentation
3. [Using Claude Code with your Pro or Max plan](https://support.claude.com/en/articles/11145838-using-claude-code-with-your-pro-or-max-plan) - Subscription limit structure

### Secondary Sources

4. [About Claude's Max Plan Usage](https://support.claude.com/en/articles/11014257-about-claude-s-max-plan-usage) - Max plan specifics
5. [Claude Code IAM Documentation](https://code.claude.com/docs/en/iam) - Credential storage
6. [GitHub - ryoppippi/ccusage](https://github.com/ryoppippi/ccusage) - Existing tool analysis
7. [GitHub - Maciek-roboblog/Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor) - Existing tool analysis
8. [GitHub - TylerGallenbeck/claude-code-limit-tracker](https://github.com/TylerGallenbeck/claude-code-limit-tracker) - Existing tool analysis
9. [When does Claude Code usage reset?](https://www.cometapi.com/when-does-claude-code-usage-reset/) - Reset behavior details

### GitHub Issues (Context)

10. [Issue #8620 - Usage limit shows 6-day reset instead of 5-hour reset](https://github.com/anthropics/claude-code/issues/8620)
11. [Issue #9424 - Weekly Usage Limits Making Claude Subscriptions Unusable](https://github.com/anthropics/claude-code/issues/9424)

---

*Research completed: 2026-01-20*
*Confidence Level: HIGH*
*Ready for implementation planning*
