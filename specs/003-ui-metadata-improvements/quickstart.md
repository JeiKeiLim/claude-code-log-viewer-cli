# Quickstart: UI Decoration and Metadata Improvements

**Date**: 2026-01-13
**Feature**: 003-ui-metadata-improvements

## Prerequisites

- Go 1.21 or later
- Existing cclv codebase checked out
- Claude Code logs available at `~/.claude/projects/`

## Quick Verification

After implementation, verify each feature:

### 1. Version Command

```bash
# Build with version info
go build -ldflags "-X github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version.Version=v0.2.0 -X github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version.Commit=$(git rev-parse HEAD)" -o cclv ./cmd/cclv

# Test version flag
./cclv --version
# Expected: cclv v0.2.0

./cclv -v
# Expected: cclv v0.2.0

# Dev build (no ldflags)
go build -o cclv ./cmd/cclv
./cclv --version
# Expected: cclv dev-<commit-hash>
```

### 2. Project List Decoration

```bash
# Launch interactive mode
./cclv

# Verify:
# - Header shows "Projects (N)" with count
# - Projects have visible borders/separators
# - Selected item is clearly highlighted
# - No emoji characters in UI
```

### 3. Conversation List Decoration

```bash
# From project list, select a project (Enter or l)

# Verify:
# - Header shows "Conversations: <project> (N)"
# - Each conversation shows:
#   - Timestamp (human-readable)
#   - Token count (formatted with commas)
#   - Message/turn count
#   - Duration (if available)
# - Selected item is highlighted
```

### 4. Token Usage Display

```bash
# In conversation list, check token display

# Verify:
# - Format: "12,345 tokens" (with thousands separator)
# - Shows "N/A" if no token data
# - In log viewer, model name shown in header
```

### 5. Lazy Loading

```bash
# Test with large project (50+ conversations)
./cclv

# Verify:
# - List appears within 1 second
# - Scrolling loads more items smoothly
# - Loading indicator shows during fetch

# Test with large log file (100+ messages)
./cclv ~/.claude/projects/<project>/<conversation>.jsonl

# Verify:
# - Initial content loads within 1 second
# - Scrolling down loads more content
# - Scrolling up to cached content is instant
```

## Build Commands

### Development Build

```bash
go build -o cclv ./cmd/cclv
```

### Release Build

```bash
VERSION=v0.2.0
COMMIT=$(git rev-parse HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

go build -ldflags "-X github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version.Version=$VERSION \
  -X github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version.Commit=$COMMIT \
  -X github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version.BuildDate=$DATE" \
  -o cclv ./cmd/cclv
```

### Cross-Compilation

```bash
# macOS ARM64
GOOS=darwin GOARCH=arm64 go build -ldflags "..." -o cclv-darwin-arm64 ./cmd/cclv

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -ldflags "..." -o cclv-darwin-amd64 ./cmd/cclv

# Linux AMD64
GOOS=linux GOARCH=amd64 go build -ldflags "..." -o cclv-linux-amd64 ./cmd/cclv

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags "..." -o cclv-linux-arm64 ./cmd/cclv
```

## Test Commands

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/version/...
go test ./internal/parser/...
go test ./internal/tui/...
```

## Common Issues

### Version shows "dev-unknown"

**Cause**: Built without git available or outside git repository
**Fix**: Ensure building from within git repo, or set Commit via ldflags

### Token counts all show "N/A"

**Cause**: Older Claude Code logs may not have usage data
**Fix**: This is expected behavior - only newer logs contain token usage

### Lazy loading not triggering

**Cause**: Item count below threshold (50 conversations or 100 messages)
**Fix**: This is expected - small lists load all at once for simplicity

### Borders not rendering correctly

**Cause**: Terminal doesn't support Unicode box-drawing characters
**Fix**: Ensure terminal uses UTF-8 encoding and supports Unicode
