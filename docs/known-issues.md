# Known Issues

## Usage bar shows percentage without remaining time after 5-hour window reset

**Status:** Investigating — needs more observation data
**First noticed:** 2026-03-19
**Frequency:** Intermittent ("sometimes")

### Symptom

After the 5-hour usage window resets, the TUI usage bar shows utilization percentage but no remaining time (e.g., `5h: 0%` instead of `5h: 0% 4h59m`). No "(stale)" indicator is shown, meaning the fetch appears to succeed. The condition persists beyond the expected 60s cache TTL.

### Expected Behavior

Within ~60s of the reset, the next fetch should return a new `resets_at` timestamp in the future, and the remaining time should reappear.

### What to Collect When Observed

When this happens again, run these commands to capture state:

```bash
# 1. Check file cache contents and age
cat ~/.cache/cclv/usage.json | python3 -m json.tool

# 2. Make a fresh API call via CLI (bypasses TUI cache)
./cclv --usage

# 3. Does pressing R (manual refresh) in the TUI fix it?

# 4. Note the current time and the expected reset time
date
```

### Possible Causes

1. **API returns `resets_at: null` or past timestamp** after a window reset — the cache correctly stores what the API returns, but the API data is stale
2. **Cache somehow not expiring** — the 60s TTL check (`nowFunc().Sub(cacheTime) > cacheTTL`) might have an edge case
3. **Fetch silently failing** — some error path that doesn't update the bar but also doesn't show stale/error

### Relevant Code

- `internal/usage/bar.go:242-244` — `formatDuration` returns `""` when `ResetsAt` is in the past
- `internal/usage/client.go:89-101` — `getCached()` TTL check
- `internal/tui/app.go:342-354` — `usageTickMsg` handler (tick scheduling)
- `internal/tui/app.go:373-436` — `usageFetchedMsg` handler (result processing)
