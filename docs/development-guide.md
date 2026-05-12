# Development Guide: cclv

**Updated**: 2026-05-13

## Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Go | 1.25+ | `go.mod` currently declares `go 1.25.0` |
| Make | Any | Primary build/test interface |
| Git | Any | Version metadata uses git tags and commit hashes |
| golangci-lint | Latest | Optional locally; CI uses the GitHub Action |
| air | Latest | Optional, for `make dev` hot reload |

## Quick Start

```bash
git clone https://github.com/JeiKeiLim/claude-code-log-viewer-cli.git
cd claude-code-log-viewer-cli

make deps
make build
./cclv
```

## Runtime Data Sources

The app reads agent data from local provider storage:

| Provider | Location |
|----------|----------|
| Claude Code | `~/.claude/projects/`, `~/.claude/sessions/` |
| Codex | `~/.codex/sessions/` |
| OpenCode | `~/.local/share/opencode/opencode.db` |

Usage monitoring also reads Claude Code credentials and calls Anthropic's OAuth usage API. Responses are cached at `~/.cache/cclv/usage.json`.

## Build Commands

### Development Build

```bash
make build
./cclv --version
```

`make build` injects version metadata through Go `-ldflags` using:

- `VERSION` from `git describe --tags --always --dirty`
- `COMMIT` from `git rev-parse --short HEAD`
- `BUILD_DATE` from UTC time

You can override those values:

```bash
make build VERSION=v0.7.0 COMMIT=local BUILD_DATE=2026-05-13T00:00:00Z
```

### Cross-Platform Builds

```bash
make build-all
make build-darwin
make build-linux
```

Cross-platform binaries are written to `dist/` as:

```
cclv-darwin-amd64
cclv-darwin-arm64
cclv-linux-amd64
cclv-linux-arm64
```

### Release Artifacts

```bash
make release
```

Local release archives use the same underscore style as GitHub releases:

```
dist/cclv_darwin_amd64.tar.gz
dist/cclv_darwin_arm64.tar.gz
dist/cclv_linux_amd64.tar.gz
dist/cclv_linux_arm64.tar.gz
```

## Run Commands

```bash
make run
make run-file FILE=path/to/conversation.jsonl
make dev
```

Useful manual checks:

```bash
./cclv --help
./cclv --version
./cclv --plain testdata/fixtures/claude-code/sample.jsonl
./cclv --agent=codex --plain testdata/fixtures/codex/simple-session.jsonl
./cclv --agent=opencode
```

`--agent=opencode` is interactive-only. OpenCode sessions are loaded from SQLite and cannot be read from file/stdin pipeline mode.

## Testing

```bash
# Full test suite with race detector and coverage
make test

# Short tests only
make test-short

# HTML coverage report
make coverage

# Stress tests
make test-stress
```

Tests live alongside source files and under `tests/acceptance/`.

## Code Quality

```bash
make fmt
make fmt-check
make vet
make lint
make check
make ci
```

`make ci` runs dependency download, formatting check, vet, lint, tests, and build. It mirrors the intended local validation path, while GitHub Actions splits lint/test/build into separate jobs.

## Project Structure

```
cmd/cclv/                 CLI entry point
internal/agent/           Provider interfaces and normalized types
internal/providers/       Claude Code, Codex, OpenCode providers
internal/parser/          Claude Code JSONL parser
internal/scanner/         Claude Code project discovery
internal/session/         Active Claude Code session detection
internal/tui/             Bubble Tea UI
internal/token/           Token estimation
internal/usage/           Claude usage API integration
internal/version/         Build metadata
internal/watcher/         File/project watchers
```

## Common Tasks

### Add or Change a CLI Flag

1. Add the flag in `cmd/cclv/main.go`.
2. Update `printHelp()`.
3. Decide where it belongs in mode validation.
4. Add focused tests in `cmd/cclv/main_test.go`.
5. Update README if user-facing.

### Add a Provider

1. Implement `internal/agent.AgentProvider`.
2. Add discovery, session summary, and parsing tests.
3. Register it in `interactiveProviders()`.
4. Decide whether file/stdin pipeline mode is supported.
5. Document provider capabilities in README and docs.

### Add a TUI View

1. Create the model under `internal/tui/`.
2. Implement `Init()`, `Update()`, and `View()`.
3. Add routing in `AppModel`.
4. Include resize, back-navigation, and cleanup behavior.
5. Add tests for view transitions and key handling.

## Version Management

Version strings come from `internal/version` and are injected at build time.

```bash
make version
make build
./cclv --version
```

Release workflows inject the same fields in GitHub Actions. When tagging releases, use annotated tags and summarize the full commit range since the previous tag.

## Known Operational Caveats

- Usage monitoring depends on an internal/undocumented Anthropic OAuth endpoint and may be rate limited.
- OpenCode is SQLite-backed and does not support file/stdin parsing.
- Active session detection is Claude Code-specific.
- Large TUI views use lazy loading and view caching; changes to scrolling or rendering should include regression tests.
