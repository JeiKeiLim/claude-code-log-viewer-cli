# Known Issues

## Usage API rate limiting (429) causes stale usage data

**Status:** Known limitation of undocumented API
**First noticed:** 2026-03-19

### Background

The `/api/oauth/usage` endpoint is an undocumented internal Anthropic API. It has aggressive rate limiting that affects all third-party tools trying to monitor Claude Code usage. This is a widely reported issue:

- [Issue #31637](https://github.com/anthropics/claude-code/issues/31637) — "aggressively rate limits, making usage monitoring unusable"
- [Issue #31021](https://github.com/anthropics/claude-code/issues/31021) — "persistent 429 rate limit"
- [Issue #30930](https://github.com/anthropics/claude-code/issues/30930) — "persistent 429 for Claude Max users"

Even polling every 5-10 minutes can trigger 429 after 1-2 successful responses. There is no documented rate limit, no useful `Retry-After` header, and no way to reliably avoid it.

### What cclv does to mitigate

1. **Shared file cache** (`~/.cache/cclv/usage.json`) — all cclv instances share one cache, so N instances generate at most 1 API call per 60s
2. **Claim mechanism** — when the cache expires, the first instance "claims" the refresh by touching the cache timestamp, preventing other instances from also hitting the API
3. **Stale data fallback** — when the API is rate-limited, cclv shows the last known data with a "(stale)" indicator
4. **Retry jitter** — rate limit retries have 0-15s random jitter to prevent synchronized retries

### Symptoms

- Usage bar shows "(stale)" for extended periods
- Usage percentages don't update after window resets (e.g., shows old 85% after 5-hour reset)
- `cclv --usage` returns "usage API rate limited (retry after 1m0s)"

### What users can expect

- Usage data works intermittently — sometimes fine for hours, sometimes rate-limited
- When rate-limited, stale data is shown until the API recovers on its own
- Multiple cclv instances do NOT make the problem worse (shared cache)
- This affects all third-party Claude Code usage monitoring tools, not just cclv

### Alternatives

- **Claude Code v2.1.80+** exposes `rate_limits` in the statusline JSON input, but this data is only available to scripts configured as Claude Code statusline commands — not to standalone tools
- **[claude-web-usage](https://github.com/skibidiskib/claude-web-usage)** uses Claude Desktop's web cookies to call a different API (claude.ai web API) which has a separate rate limit bucket. macOS only, requires Claude Desktop app
