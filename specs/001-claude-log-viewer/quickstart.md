# Quickstart: Claude Code Log Viewer CLI

**Phase**: 1 - Design
**Date**: 2026-01-12

## Prerequisites

- Go 1.21 or later installed
- Claude Code installed with existing conversation logs in `~/.claude/projects/`

## Installation

### From Source

```bash
# Clone and build
git clone https://github.com/JeiKeiLim/claude-code-log-viewer-cli.git
cd claude-code-log-viewer-cli
go build -o cclv ./cmd/cclv

# Move to PATH
sudo mv cclv /usr/local/bin/
```

### Using go install

```bash
go install github.com/JeiKeiLim/claude-code-log-viewer-cli/cmd/cclv@latest
```

## Basic Usage

### Interactive Mode (Project Browser)

```bash
# Launch with project browser
cclv

# Navigate:
# j/k or ↑/↓  - Move selection
# Enter or l  - Select project/conversation
# Escape or h - Go back
# /           - Search/filter
# q           - Quit
```

### Pipeline Mode (Direct File Viewing)

```bash
# Pipe a JSONL file
cat ~/.claude/projects/-Users-limjk-.../conversation.jsonl | cclv

# Or pass as argument
cclv ~/.claude/projects/-Users-limjk-.../conversation.jsonl

# Stream live logs
tail -f ~/.claude/projects/-Users-limjk-.../conversation.jsonl | cclv
```

### Quick View Latest Conversation

```bash
# View most recent conversation across all projects
cclv "$(ls -t ~/.claude/projects/**/*.jsonl | head -1)"
```

## Navigation Keys

### All Views

| Key | Action |
|-----|--------|
| `q` | Quit application |
| `Ctrl+C` | Force quit |
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `/` | Open search |

### Project Browser & Conversation List

| Key | Action |
|-----|--------|
| `Enter` / `l` | Select item |
| `Escape` / `h` | Go back |
| `gg` | Jump to top |
| `G` | Jump to bottom |

### Log Viewer

| Key | Action |
|-----|--------|
| `Escape` / `h` | Return to list |
| `gg` | Jump to top of log |
| `G` | Jump to bottom of log |
| `t` | Toggle thinking blocks visibility |
| `i` | Toggle tool input visibility |
| `n` | Next search match |
| `N` | Previous search match |
| `Page Up` | Scroll up one page |
| `Page Down` | Scroll down one page |

## Visual Elements

### Message Types

```
┌─────────────────────────────────────────────────────────────┐
│ 👤 User                                    2026-01-12 10:30 │
├─────────────────────────────────────────────────────────────┤
│ Help me understand this code                                │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ 🤖 Assistant                               2026-01-12 10:31 │
├─────────────────────────────────────────────────────────────┤
│ [thinking - press 't' to expand]                            │
│                                                             │
│ I'll help you understand the code. Let me first read it...  │
│                                                             │
│ Tool: Read [inputs - press 'i' to expand]                   │
└─────────────────────────────────────────────────────────────┘
```

### Expanded Thinking Block

```
┌─────────────────────────────────────────────────────────────┐
│ 💭 Thinking                                                 │
├─────────────────────────────────────────────────────────────┤
│ The user wants to understand the code. I should first read  │
│ the file to see its contents, then provide a clear          │
│ explanation of what each part does...                       │
└─────────────────────────────────────────────────────────────┘
```

### Expanded Tool Input

```
┌─────────────────────────────────────────────────────────────┐
│ 🔧 Tool: Read                                               │
├─────────────────────────────────────────────────────────────┤
│ file_path: /Users/limjk/project/main.go                     │
│ ... (45 chars total)                                        │
└─────────────────────────────────────────────────────────────┘
```

## Troubleshooting

### "No projects found"

Claude Code stores projects in `~/.claude/projects/`. This error appears if:
- Claude Code has not been used yet
- The directory doesn't exist

**Solution**: Use Claude Code to have at least one conversation, or use pipeline mode directly with a JSONL file.

### "X lines skipped due to parse errors"

Some lines in the JSONL file couldn't be parsed. This can happen with:
- Corrupted log files
- Incompatible Claude Code version

**Solution**: The viewer will skip unparseable lines and show the rest. Check if you're using a very old Claude Code version.

### Slow scrolling with large files

For files >50MB, initial parsing may take a moment. Once parsed, scrolling should be smooth.

**Solution**: Wait for initial load, then navigation will be fast.

## Development

### Build from Source

```bash
# Build
go build -o cclv ./cmd/cclv

# Run tests
go test ./...

# Run with race detector
go run -race ./cmd/cclv
```

### Cross-compilation

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o cclv-linux-amd64 ./cmd/cclv

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o cclv-linux-arm64 ./cmd/cclv

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o cclv-darwin-amd64 ./cmd/cclv

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o cclv-darwin-arm64 ./cmd/cclv
```

## Next Steps

- Run `cclv` to explore your Claude Code conversation history
- Use `cclv <file>` for quick access to specific conversations
- Integrate into your workflow: `alias latest-claude='cclv "$(ls -t ~/.claude/projects/**/*.jsonl | head -1)"'`
