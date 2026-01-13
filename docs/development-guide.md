# Development Guide: cclv

**Generated**: 2026-01-13 | **Scan Level**: Exhaustive

## Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Go | 1.21+ | Required (project uses 1.24.3) |
| Make | Any | For build automation |
| Git | Any | For version control |
| golangci-lint | Latest | Optional, for linting |
| air | Latest | Optional, for hot reload |

## Quick Start

```bash
# Clone repository
git clone https://github.com/JeiKeiLim/claude-code-log-viewer-cli.git
cd claude-code-log-viewer-cli

# Install dependencies
make deps

# Build
make build

# Run
./cclv
```

## Project Setup

### Environment Setup

No special environment variables required. The application reads from:
- `~/.claude/projects/` - Claude Code project storage (read-only)

### Install Dependencies

```bash
# Download Go modules
make deps
# or
go mod download

# Verify dependencies
make verify
# or
go mod verify

# Tidy go.mod
make tidy
# or
go mod tidy
```

## Build Commands

### Development Build

```bash
# Build for current platform
make build

# Output: ./cclv
```

### Production Build

```bash
# Build with optimizations (strips debug info)
make build

# Build flags applied: -ldflags "-s -w"
```

### Cross-Platform Builds

```bash
# Build for all platforms
make build-all
# Outputs in dist/:
#   cclv-darwin-amd64
#   cclv-darwin-arm64
#   cclv-linux-amd64
#   cclv-linux-arm64

# macOS only
make build-darwin

# Linux only
make build-linux
```

### Release Build

```bash
# Create release artifacts with tar.gz
make release
# Outputs: dist/cclv-{platform}.tar.gz
```

## Run Commands

```bash
# Build and run
make run

# Run with specific file
make run-file FILE=path/to/conversation.jsonl

# Development mode with hot reload (requires air)
make dev
```

## Testing

### Run Tests

```bash
# Run all tests with coverage
make test
# Output: coverage.out

# Run short tests only
make test-short

# Generate HTML coverage report
make coverage
# Output: coverage.html
```

### Test Structure

Tests are located alongside source files or in a `tests/` directory:

```
internal/
├── parser/
│   ├── entry.go
│   └── entry_test.go
├── scanner/
│   ├── projects.go
│   └── projects_test.go
└── ...
```

## Code Quality

### Formatting

```bash
# Format code
make fmt

# Check formatting (CI)
make fmt-check
```

### Linting

```bash
# Run linter (golangci-lint or go vet)
make lint

# Run go vet only
make vet
```

### Quick Validation

```bash
# Fast validation (fmt-check + vet)
make check
```

### CI Pipeline

```bash
# Full CI checks: deps + fmt-check + vet + test + build
make ci
```

## Installation

### Install to GOPATH/bin

```bash
make install
# or
go install ./cmd/cclv
```

### Install to /usr/local/bin

```bash
# Requires sudo
make install-global
```

### Uninstall

```bash
make uninstall        # From GOPATH/bin
make uninstall-global # From /usr/local/bin (requires sudo)
```

## Version Management

Version information is injected at build time via ldflags:

```bash
# View current version info
make version

# Build with version (example)
go build -ldflags "-X github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version.Version=v1.0.0 -X ...Version.Commit=$(git rev-parse --short HEAD)" ./cmd/cclv
```

## Development Workflow

### 1. Feature Development

```bash
# Create feature branch
git checkout -b feature/my-feature

# Make changes...

# Format and lint
make check

# Run tests
make test

# Build and test manually
make build
./cclv
```

### 2. Speckit Workflow

This project uses Speckit for feature planning:

```bash
# Feature specs are in specs/{feature-name}/
# Each spec contains:
#   - spec.md       # Feature specification
#   - plan.md       # Implementation plan
#   - research.md   # Research notes
#   - data-model.md # Data model design
#   - tasks.md      # Implementation tasks
```

### 3. Before Committing

```bash
# Full validation
make ci

# Or quick check
make check && make test
```

## Debugging

### Debug Output Height Issues

If you encounter screen clipping issues with the list component:

```go
// Add debug output to View()
listLines := strings.Count(listView, "\n") + 1
footer := fmt.Sprintf("h:%d list:%d", m.height, listLines)
```

See [lessons-learned.md](./lessons-learned.md) for the Bubbletea list height issue solution.

### Debug JSONL Parsing

```go
// Enable parse error tracking
result := parser.ParseJSONL(reader)
fmt.Printf("Entries: %d, Errors: %d\n", len(result.Entries), result.ParseErrors)
```

## Common Tasks

### Add a New TUI View

1. Create `internal/tui/newview.go`
2. Implement `tea.Model` interface: `Init()`, `Update()`, `View()`
3. Add view state to `viewState` enum in `app.go`
4. Add routing in `AppModel.Update()` and `AppModel.View()`

### Add a New Command Flag

1. Add flag in `cmd/cclv/main.go`:
   ```go
   myFlag := flag.Bool("myflag", false, "Description")
   ```
2. Handle before mode detection if it should exit early
3. Pass to appropriate mode function if needed

### Add a New Type

1. Add to appropriate file in `internal/types/`
2. Add raw JSON struct if parsing from JSONL
3. Update parser in `internal/parser/entry.go`

## Performance Tips

### Large File Handling

The application uses lazy loading for large files:
- Conversations: Load metadata for first 20, then on-demand
- Messages: Render first 40, load more on scroll

Thresholds can be adjusted in `internal/tui/styles.go`:
```go
func DefaultLazyLoadConfig() LazyLoadConfig {
    return LazyLoadConfig{
        BatchSize:             20,
        ConversationThreshold: 50,
        MessageThreshold:      100,
    }
}
```

### Memory Optimization

- Use `ParseJSONLStream()` for very large files
- Avoid storing full content in memory
- Let viewport handle scrolling (it virtualizes content)

## Makefile Reference

| Target | Description |
|--------|-------------|
| `all` | Clean, lint, test, build |
| `build` | Build for current platform |
| `build-all` | Build for all platforms |
| `build-darwin` | Build for macOS |
| `build-linux` | Build for Linux |
| `test` | Run tests with coverage |
| `test-short` | Run short tests |
| `coverage` | Generate HTML coverage report |
| `fmt` | Format code |
| `fmt-check` | Check formatting |
| `vet` | Run go vet |
| `lint` | Run linter |
| `check` | Quick validation |
| `ci` | Full CI checks |
| `deps` | Download dependencies |
| `tidy` | Tidy go.mod |
| `verify` | Verify dependencies |
| `install` | Install to GOPATH/bin |
| `install-global` | Install to /usr/local/bin |
| `uninstall` | Uninstall from GOPATH/bin |
| `clean` | Clean build artifacts |
| `run` | Build and run |
| `run-file` | Run with FILE= argument |
| `dev` | Development mode with hot reload |
| `size` | Show binary size |
| `version` | Show version info |
| `release` | Create release artifacts |
| `help` | Show help |
