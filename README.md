# cclv - Claude Code Log Viewer

A beautiful terminal UI for browsing and viewing Claude Code conversation logs.

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)
[![Release](https://img.shields.io/github/v/release/JeiKeiLim/claude-code-log-viewer-cli)](https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest)

## Features

- **Interactive Project Browser** - Navigate all your Claude Code projects from `~/.claude/projects/`
- **Conversation Timeline** - Browse conversations sorted by most recent
- **Beautiful Log Viewer** - View messages with syntax highlighting and proper formatting
- **Vim-style Navigation** - `j/k`, `gg/G`, `/search`, and more
- **CJK Support** - Proper display of Korean, Japanese, and Chinese characters
- **Pipeline Mode** - Pipe JSONL logs directly: `cat file.jsonl | cclv`
- **Plain Text Output** - Export logs without TUI for scripting

## Installation

### Download Binary (Recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest).

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest/download/cclv_darwin_arm64.tar.gz | tar xz
sudo mv cclv /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -L https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest/download/cclv_darwin_amd64.tar.gz | tar xz
sudo mv cclv /usr/local/bin/
```

**Linux (amd64):**
```bash
curl -L https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest/download/cclv_linux_amd64.tar.gz | tar xz
sudo mv cclv /usr/local/bin/
```

**Linux (arm64):**
```bash
curl -L https://github.com/JeiKeiLim/claude-code-log-viewer-cli/releases/latest/download/cclv_linux_arm64.tar.gz | tar xz
sudo mv cclv /usr/local/bin/
```

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
sudo mv cclv /usr/local/bin/
```

## Usage

### Interactive Mode

Simply run `cclv` to browse all your Claude Code projects:

```bash
cclv
```

This opens an interactive browser where you can:
1. Select a project from the list
2. Choose a conversation session
3. View the full conversation log

### View a Specific File

```bash
cclv path/to/conversation.jsonl
```

### Pipeline Mode

```bash
cat conversation.jsonl | cclv
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

### Version Information

```bash
# Show version
cclv --version
cclv -v
```

## Keyboard Shortcuts

### Project/Conversation List

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate down / up |
| `Enter` / `l` | Select item |
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
| `/` | Search |
| `n` / `N` | Next / previous match |
| `t` | Toggle thinking blocks |
| `i` | Toggle tool inputs |
| `h` / `Esc` | Go back |
| `q` | Quit |

## Message Types

The viewer renders different message types with distinct styling:

- **User messages** - Your prompts and questions
- **Assistant responses** - Claude's text responses
- **Thinking blocks** - Claude's reasoning (collapsible with `t`)
- **Tool use** - Tool calls and inputs (collapsible with `i`)

## How It Works

Claude Code stores conversation logs in `~/.claude/projects/` as JSONL files. Each project directory is named using an encoded path format. `cclv` automatically:

1. Scans the projects directory
2. Decodes project paths (handles hyphens, underscores, and special characters)
3. Parses JSONL conversation logs
4. Renders them in a beautiful TUI

## Requirements

- Go 1.21 or later
- Terminal with ANSI color support

## License

MIT

## Acknowledgments

Built with:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
