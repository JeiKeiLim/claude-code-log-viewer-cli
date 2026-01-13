# cclv Documentation Index

**Generated**: 2026-01-13 | **Scan Level**: Exhaustive | **Mode**: Initial Scan

## Project Overview

| Attribute | Value |
|-----------|-------|
| **Type** | CLI (Monolith) |
| **Primary Language** | Go 1.24.3 |
| **Framework** | Charm Stack (Bubbletea, Lipgloss, Bubbles) |
| **Architecture** | Standard Go CLI (cmd/ + internal/) |

## Quick Reference

| Metric | Value |
|--------|-------|
| **Go Files** | 15 |
| **Packages** | 6 |
| **Total LOC** | ~2,400 |
| **Dependencies** | 4 direct |
| **Entry Point** | cmd/cclv/main.go |

### Key Commands

```bash
# Interactive mode (browse projects)
cclv

# View specific file
cclv path/to/conversation.jsonl

# Pipeline mode
cat file.jsonl | cclv

# Plain text output
cclv --plain file.jsonl

# Version
cclv --version
```

## Generated Documentation

| Document | Description |
|----------|-------------|
| [Project Overview](./project-overview.md) | Executive summary, features, tech stack |
| [Architecture](./architecture.md) | System design, packages, data flow |
| [Source Tree Analysis](./source-tree-analysis.md) | Annotated directory structure |
| [Development Guide](./development-guide.md) | Setup, build, test instructions |

## Existing Documentation

| Document | Description |
|----------|-------------|
| [README.md](../README.md) | Project readme with usage instructions |
| [CLAUDE.md](../CLAUDE.md) | Development guidelines for AI agents |
| [Lessons Learned](./lessons-learned.md) | Technical insights and solutions |

## Feature Specifications (Speckit)

| Feature | Branch | Status |
|---------|--------|--------|
| [001-claude-log-viewer](../specs/001-claude-log-viewer/) | Initial implementation | Complete |
| [002-cclv-fixes-enhancements](../specs/002-cclv-fixes-enhancements/) | Bug fixes | Complete |
| [003-ui-metadata-improvements](../specs/003-ui-metadata-improvements/) | UI enhancements | In Progress |

## Getting Started

### For Users

```bash
# Install
go install github.com/JeiKeiLim/claude-code-log-viewer-cli/cmd/cclv@latest

# Run
cclv
```

### For Developers

```bash
# Clone and build
git clone https://github.com/JeiKeiLim/claude-code-log-viewer-cli.git
cd claude-code-log-viewer-cli
make build

# Run tests
make test

# Development mode
make dev
```

See [Development Guide](./development-guide.md) for detailed instructions.

## Architecture At-a-Glance

```
cmd/cclv/main.go          → Entry point, mode detection
internal/parser/          → JSONL parsing
internal/scanner/         → Project discovery
internal/tui/             → Bubbletea UI components
internal/types/           → Domain models
internal/version/         → Version info
```

See [Architecture](./architecture.md) for detailed documentation.

## Navigation

| I want to... | Go to... |
|--------------|----------|
| Understand the project | [Project Overview](./project-overview.md) |
| Learn the architecture | [Architecture](./architecture.md) |
| Explore the codebase | [Source Tree Analysis](./source-tree-analysis.md) |
| Set up development | [Development Guide](./development-guide.md) |
| Read feature specs | [specs/](../specs/) |
| See known issues/solutions | [Lessons Learned](./lessons-learned.md) |

---

**Next Steps for Brownfield PRD:**

When ready to plan new features, run the PRD workflow and provide this index as input:
```
/bmad:bmm:workflows:prd
```

The documentation will inform the PRD about existing architecture, patterns, and constraints.
