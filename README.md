# cclv - Claude Code Log Viewer

A terminal UI for browsing Claude Code, Codex, and OpenCode coding-agent sessions.

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)
[![Release](https://img.shields.io/github/v/release/JeiKeiLim/claude-code-log-viewer-cli)](https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest)

![cclv demo](docs/res/demo.gif)

Browse your coding-agent projects, select a session, and view the full message history with collapsible thinking sections and tool call details - all from your terminal.

## Features

- **Multi-Agent Project Browser** - Navigate Claude Code, Codex, and OpenCode sessions from one TUI
- **Conversation Timeline** - Browse conversations sorted by most recent
- **Log Viewer** - View messages with markdown rendering and proper text wrapping
- **Dashboard Mode** - Monitor up to 9 projects simultaneously in a grid layout
- **Active Session Dashboard** - Auto-detect live Claude Code sessions for a selected project
- **Watch Mode** - Real-time updates as file-backed conversations grow (`-w` flag)
- **Usage Limit Monitor** - View Claude Code API usage limits in TUI status bar or via `--usage` flag
- **Developer Power Tools** - Line numbers, raw JSONL mode, vim-style `:N` navigation
- **Token Statistics** - View token counts per message and conversation totals
- **Vim-style Navigation** - `j/k`, `gg/G`, `/search`, and more
- **CJK Support** - Proper display of Korean, Japanese, and Chinese characters
- **Pipeline Mode** - Pipe Claude Code or Codex JSONL logs directly: `cat file.jsonl | cclv`
- **Plain Text Output** - Export logs without TUI for scripting

## Installation

### Download Binary (Recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest).

**macOS (Apple Silicon):**
```bash
curl -Lo cclv.tar.gz https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest/download/cclv_darwin_arm64.tar.gz && mkdir -p ~/.local/bin && tar -xzf cclv.tar.gz -C ~/.local/bin cclv && rm cclv.tar.gz
```

**macOS (Intel):**
```bash
curl -Lo cclv.tar.gz https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest/download/cclv_darwin_amd64.tar.gz && mkdir -p ~/.local/bin && tar -xzf cclv.tar.gz -C ~/.local/bin cclv && rm cclv.tar.gz
```

**Linux (amd64):**
```bash
curl -Lo cclv.tar.gz https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest/download/cclv_linux_amd64.tar.gz && mkdir -p ~/.local/bin && tar -xzf cclv.tar.gz -C ~/.local/bin cclv && rm cclv.tar.gz
```

**Linux (arm64):**
```bash
curl -Lo cclv.tar.gz https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest/download/cclv_linux_arm64.tar.gz && mkdir -p ~/.local/bin && tar -xzf cclv.tar.gz -C ~/.local/bin cclv && rm cclv.tar.gz
```

> **Note:** Make sure `~/.local/bin` is in your PATH. If not, add this to your `~/.bashrc` or `~/.zshrc`:
> ```bash
> export PATH="$HOME/.local/bin:$PATH"
> ```

### Go Install

If you have Go installed:

```bash
go install github.com/JeiKeiLim/claude-code-log-viewer-cli/cmd/cclv@latest
```

### Build from Source

```bash
git clone https://github.com/JeiKeiLim/claude-code-log-viewer-cli.git
cd claude-code-log-viewer-cli
make build
mv cclv ~/.local/bin/
```

## Usage

### Interactive Mode

Simply run `cclv` to browse available agent data:

```bash
cclv
```

This opens an interactive browser where you can:
1. Select an available agent provider when more than one has sessions
2. Select a project from the list
3. Open the active-session dashboard or conversation list
4. View the full conversation log

### Agent Support

| Agent | Interactive Browse | File/stdin Pipeline | Default Storage |
|-------|--------------------|---------------------|-----------------|
| Claude Code | Yes | Yes | `~/.claude/projects/` |
| Codex | Yes | Yes | `~/.codex/sessions/` |
| OpenCode | Yes | No | `~/.local/share/opencode/opencode.db` |

Use `--agent=claude-code` or `--agent=codex` to force a file/stdin parser. Use `--agent=opencode` only with interactive mode, because OpenCode stores sessions in SQLite rather than append-only JSONL files.

### View a Specific File

```bash
cclv path/to/conversation.jsonl
cclv --agent=codex path/to/rollout.jsonl
```

### Pipeline Mode

```bash
cat conversation.jsonl | cclv
cat rollout.jsonl | cclv --agent=codex
```

### Plain Text Output

```bash
# Force plain text output (no TUI)
cclv --plain conversation.jsonl

# Pipe to other tools
cclv --plain conversation.jsonl | grep "error"

# Force TUI even when piping
cat file.jsonl | cclv --tui
```

### Watch Mode

Monitor a conversation in real-time as it grows:

```bash
# Watch a specific file
cclv -w conversation.jsonl

# From interactive mode, press 'w' on a conversation to watch it
```

`--watch` works with file-backed Claude Code and Codex sessions. OpenCode live updates are handled through the interactive OpenCode provider.

### Dashboard Mode

Monitor multiple projects simultaneously:

1. Run `cclv` to open the project browser
2. Press `Space` to select projects (up to 9)
3. Press `Enter` to open dashboard view

The dashboard displays a grid layout that auto-sizes based on selection count:
- 1 project: Full screen
- 2-3 projects: 1 row
- 4 projects: 2x2 grid
- 5-6 projects: 2x3 grid
- 7-9 projects: 3x3 grid

Each pane shows the latest conversation and updates in real-time.

### Color Output

By default, colors are disabled when output is piped. Use `--color` to control this:

```bash
# Auto-detect (default) - colors when TTY, no colors when piped
cclv --plain conversation.jsonl

# Force colors even when piping
cclv --plain --color=always conversation.jsonl | cat

# Disable colors completely
cclv --plain --color=never conversation.jsonl
```

### Version Information

```bash
# Show version
cclv --version
cclv -v
```

### Usage Information

Check your Claude Code API usage limits directly from the terminal:

```bash
# Show current usage (requires Claude Code credentials)
cclv --usage
cclv -u
```

This displays your daily token usage, reset time, and usage percentage. Useful for monitoring limits in scripts or checking before starting long sessions.

`--usage` calls Anthropic's OAuth usage API and caches responses in `~/.cache/cclv/usage.json` to reduce rate-limit pressure.

### Streaming Plain Mode

For integration with external tools, combine watch and plain modes:

```bash
# Stream formatted output continuously
cclv --watch --plain conversation.jsonl

# With color output for tools that support ANSI
cclv --watch --plain --color=always conversation.jsonl
```

New entries appear formatted in real-time. The process continues until interrupted with Ctrl+C.

### Additional Options

```bash
# Hide verbose blocks
cclv --hide-thoughts --hide-tools conversation.jsonl

# Set render width for plain/pipeline output
cclv --width=100 conversation.jsonl

# Follow the newest conversation while watching
cclv -w -L conversation.jsonl

# Skip the active-session dashboard and go straight to conversations
cclv --no-multi-session
```

## Keyboard Shortcuts

### Project List

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate down / up |
| `Enter` / `l` | Select project |
| `Space` | Toggle project selection (for dashboard) |
| `g` / `G` | Jump to top / bottom |
| `/` | Filter list |
| `q` | Quit |

### Conversation List

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate down / up |
| `Enter` / `l` | Open conversation |
| `w` | Open with watch mode |
| `h` / `Esc` | Go back |
| `g` / `G` | Jump to top / bottom |
| `/` | Filter list |
| `q` | Quit |

### Log Viewer

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll down / up |
| `d` / `u` | Half page down / up |
| `gg` / `G` | Jump to top / bottom |
| `:N` | Jump to line/entry N |
| `/` | Search |
| `n` / `N` | Next / previous match |
| `t` | Toggle thinking blocks |
| `i` | Toggle tool inputs |
| `r` | Toggle raw JSONL mode |
| `p` | Show file path (toast) |
| `h` / `Esc` | Go back |
| `q` | Quit |

### Dashboard

| Key | Action |
|-----|--------|
| `h` / `j` / `k` / `l` | Navigate between panes |
| Arrow keys | Navigate between panes |
| `Enter` | Open focused pane in viewer |
| `r` | Refresh focused pane |
| `Esc` | Return to project list |
| `q` | Quit |

### Global (All Views)

| Key | Action |
|-----|--------|
| `R` | Refresh Claude Code usage limit |
| `q` | Quit |

## Message Types

The viewer renders different message types with distinct styling:

- **User messages** - Your prompts and questions
- **Assistant responses** - Claude's text responses with markdown rendering
- **Thinking blocks** - Claude's reasoning (collapsible with `t`)
- **Tool use** - Tool calls and inputs (collapsible with `i`)

Each message displays token usage when available:
- `Tokens: 1,234 (from log)` - Actual token count from Claude's API response
- `Tokens: ~1,200 (estimated)` - Calculated estimate using tiktoken

The status bar shows conversation totals with a `~` prefix when any tokens are estimated.

## How It Works

Each provider stores sessions differently. `cclv` automatically:

1. Scans available provider storage
2. Groups sessions by project directory
3. Parses Claude Code and Codex JSONL files or reads OpenCode sessions from SQLite
4. Renders messages in the TUI or as plain text

## Requirements

- Go 1.25 or later
- Terminal with ANSI color support

## License

MIT

## Acknowledgments

Built with:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Glamour](https://github.com/charmbracelet/glamour) - Markdown rendering
- [fsnotify](https://github.com/fsnotify/fsnotify) - File system notifications
- [tiktoken-go](https://github.com/pkoukk/tiktoken-go) - Token counting
