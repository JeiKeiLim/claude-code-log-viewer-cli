# Data Model: Claude Code Log Viewer CLI

**Phase**: 1 - Design
**Date**: 2026-01-12

## Overview

This document defines the data structures for CCLV. All types are read-only views of Claude Code's JSONL log files.

---

## Entity Relationship Diagram

```
┌─────────────────┐
│     Project     │
├─────────────────┤         ┌──────────────────┐
│ EncodedName     │────────>│   Conversation   │
│ DecodedPath     │   1:N   ├──────────────────┤         ┌─────────────────┐
│ DisplayName     │         │ FilePath         │────────>│    LogEntry     │
│ DirPath         │         │ LastModified     │   1:N   ├─────────────────┤
└─────────────────┘         │ MessageCount     │         │ Type            │
                            │ FirstUserMessage │         │ UUID            │
                            └──────────────────┘         │ ParentUUID      │
                                                         │ Timestamp       │
                                                         │ Message         │
                                                         └────────┬────────┘
                                                                  │
                                                                  │ contains
                                                                  ▼
                                                         ┌─────────────────┐
                                                         │ MessageContent  │
                                                         ├─────────────────┤
                                                         │ Type            │
                                                         │ Text            │
                                                         │ Thinking        │
                                                         │ ToolName        │
                                                         │ ToolInput       │
                                                         └─────────────────┘
```

---

## Entity Definitions

### Project

Represents a directory under `~/.claude/projects/` containing conversation logs.

| Field | Type | Description |
|-------|------|-------------|
| EncodedName | string | Directory name as stored (e.g., `-Users-limjk-GitHub-foo`) |
| DecodedPath | string | Decoded original path (e.g., `/Users/limjk/GitHub/foo`) |
| DisplayName | string | Short display name (e.g., `foo` or `limjk/foo` if disambiguated) |
| DirPath | string | Full path to project directory under `~/.claude/projects/` |

**Validation Rules**:
- EncodedName MUST not be empty
- DirPath MUST exist and be a directory
- DisplayName derived at runtime based on collision detection

**State**: Stateless (read from filesystem)

---

### Conversation

Represents a single JSONL file containing a conversation session.

| Field | Type | Description |
|-------|------|-------------|
| FilePath | string | Full path to the .jsonl file |
| LastModified | time.Time | File modification timestamp (for sorting) |
| MessageCount | int | Number of log entries (computed on load) |
| FirstUserMessage | string | Preview of first user message (truncated to 80 chars) |

**Validation Rules**:
- FilePath MUST end with `.jsonl`
- FilePath MUST exist and be readable
- LastModified from filesystem stat

**State**: Stateless (read from filesystem)

---

### LogEntry

Represents a single line/record from a JSONL file.

| Field | Type | Description |
|-------|------|-------------|
| Type | EntryType | One of: `user`, `assistant`, `file-history-snapshot` |
| UUID | string | Unique identifier for this entry |
| ParentUUID | string | UUID of parent entry (for threading), may be empty |
| Timestamp | time.Time | When this entry was created |
| SessionID | string | Session identifier |
| Message | Message | The message content (for user/assistant types) |

**Validation Rules**:
- Type MUST be one of the known EntryType values
- UUID SHOULD be a valid UUID format (warn if not, don't fail)
- Timestamp MUST be parseable as RFC3339

**State**: Immutable once parsed

---

### EntryType (enum)

```go
type EntryType string

const (
    EntryTypeUser             EntryType = "user"
    EntryTypeAssistant        EntryType = "assistant"
    EntryTypeFileHistorySnapshot EntryType = "file-history-snapshot"
)
```

---

### Message

Wrapper for the message content in user/assistant entries.

| Field | Type | Description |
|-------|------|-------------|
| Role | string | `user` or `assistant` |
| Content | []MessageContent | Array of content blocks (for assistant) or single text (for user) |

**Note**: User messages have Content as a plain string in the JSON; assistant messages have Content as an array of objects.

---

### MessageContent

Individual content block within an assistant message.

| Field | Type | Description |
|-------|------|-------------|
| Type | ContentType | One of: `text`, `thinking`, `tool_use` |
| Text | string | Text content (for `text` type) |
| Thinking | string | Reasoning content (for `thinking` type) |
| ToolName | string | Name of tool invoked (for `tool_use` type) |
| ToolInput | map[string]any | Tool input parameters as JSON (for `tool_use` type) |
| ToolID | string | Unique ID for tool invocation |

**Validation Rules**:
- Type MUST be one of the known ContentType values
- For `text`: Text MUST not be empty
- For `thinking`: Thinking MUST not be empty
- For `tool_use`: ToolName MUST not be empty, ToolInput may be empty

---

### ContentType (enum)

```go
type ContentType string

const (
    ContentTypeText     ContentType = "text"
    ContentTypeThinking ContentType = "thinking"
    ContentTypeToolUse  ContentType = "tool_use"
)
```

---

## JSON Mapping

### Raw JSONL Structure (from Claude Code logs)

```json
{
  "type": "user",
  "uuid": "abc-123",
  "parentUuid": null,
  "timestamp": "2026-01-12T10:30:00.000Z",
  "sessionId": "session-456",
  "message": {
    "role": "user",
    "content": "Help me with X"
  }
}
```

```json
{
  "type": "assistant",
  "uuid": "def-456",
  "parentUuid": "abc-123",
  "timestamp": "2026-01-12T10:30:05.000Z",
  "sessionId": "session-456",
  "message": {
    "role": "assistant",
    "content": [
      {"type": "thinking", "thinking": "Let me analyze..."},
      {"type": "text", "text": "Here is my response..."},
      {"type": "tool_use", "id": "tool-1", "name": "Read", "input": {"file_path": "/foo"}}
    ]
  }
}
```

### Go Struct Mapping

```go
type RawLogEntry struct {
    Type       string          `json:"type"`
    UUID       string          `json:"uuid"`
    ParentUUID *string         `json:"parentUuid"`
    Timestamp  string          `json:"timestamp"`
    SessionID  string          `json:"sessionId"`
    Message    json.RawMessage `json:"message"`
}

type RawUserMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type RawAssistantMessage struct {
    Role    string              `json:"role"`
    Content []RawMessageContent `json:"content"`
}

type RawMessageContent struct {
    Type     string         `json:"type"`
    Text     string         `json:"text,omitempty"`
    Thinking string         `json:"thinking,omitempty"`
    Name     string         `json:"name,omitempty"`
    ID       string         `json:"id,omitempty"`
    Input    map[string]any `json:"input,omitempty"`
}
```

---

## Display Formatting

### Timestamp Display
- Parse as UTC from JSON
- Display in local timezone
- Format: `2006-01-02 15:04` (short) or `2006-01-02 15:04:05` (full)

### Text Truncation
- Tool inputs: 200 characters max when expanded, with `... (X chars total)` suffix
- First user message preview: 80 characters max
- Project display name: No truncation, but use collision disambiguation

### Collapsed State Indicators
- Thinking: `[thinking - press 't' to expand]`
- Tool inputs: `Tool: <name> [inputs - press 'i' to expand]`

---

## Summary

| Entity | Source | Mutability | Key Fields |
|--------|--------|------------|------------|
| Project | Directory scan | Read-only | EncodedName, DisplayName |
| Conversation | File listing | Read-only | FilePath, LastModified |
| LogEntry | JSONL parsing | Immutable | Type, UUID, Timestamp, Message |
| MessageContent | JSON parsing | Immutable | Type, Text/Thinking/ToolName |

**Next Step**: Generate quickstart.md
