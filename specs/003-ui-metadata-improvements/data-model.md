# Data Model: UI Decoration and Metadata Improvements

**Date**: 2026-01-13
**Feature**: 003-ui-metadata-improvements

## Entity Changes

### 1. Version (NEW)

**Package**: `internal/version`

```go
// Version information embedded at build time
var (
    // Version is the semantic version (e.g., "v1.0.0")
    // Set via: -ldflags "-X .../version.Version=v1.0.0"
    Version = "dev"

    // Commit is the git commit hash
    // Set via: -ldflags "-X .../version.Commit=$(git rev-parse HEAD)"
    Commit = "unknown"

    // BuildDate is the build timestamp
    // Set via: -ldflags "-X .../version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    BuildDate = "unknown"
)
```

**Functions**:
- `String() string` - Returns formatted version string (e.g., "cclv v1.0.0" or "cclv dev-abc1234")
- `Full() string` - Returns full version with commit and date

---

### 2. TokenUsage (NEW)

**Package**: `internal/types`

```go
// TokenUsage represents token consumption for a message or conversation
type TokenUsage struct {
    InputTokens              int `json:"input_tokens"`
    OutputTokens             int `json:"output_tokens"`
    CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
    CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// Total returns the total token count (input + output + cache)
func (u TokenUsage) Total() int {
    return u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// TotalInput returns total input tokens including cache
func (u TokenUsage) TotalInput() int {
    return u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}
```

---

### 3. LogEntry (MODIFIED)

**Package**: `internal/types`

**New Fields**:
```go
type LogEntry struct {
    // ... existing fields ...

    // Model is the Claude model used (e.g., "claude-opus-4-5-20251101")
    // Only populated for assistant entries
    Model string

    // Usage contains token usage statistics
    // Only populated for assistant entries
    Usage TokenUsage
}
```

---

### 4. Conversation (MODIFIED)

**Package**: `internal/types`

**New Fields**:
```go
type Conversation struct {
    // ... existing fields ...

    // TotalTokens is the sum of all token usage in the conversation
    TotalTokens TokenUsage

    // Model is the primary model used (from first assistant message)
    Model string

    // Duration is the time between first and last message
    Duration time.Duration

    // TurnCount is the number of user-assistant turn pairs
    TurnCount int
}
```

---

### 5. RawAssistantMessage (MODIFIED)

**Package**: `internal/types`

**New Fields**:
```go
type RawAssistantMessage struct {
    Role    string              `json:"role"`
    Content []RawMessageContent `json:"content"`
    Model   string              `json:"model,omitempty"`   // NEW
    Usage   *RawTokenUsage      `json:"usage,omitempty"`   // NEW
}

// RawTokenUsage is the raw JSON structure for token usage
type RawTokenUsage struct {
    InputTokens              int    `json:"input_tokens"`
    OutputTokens             int    `json:"output_tokens"`
    CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
    CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
    ServiceTier              string `json:"service_tier,omitempty"`
}
```

---

### 6. LoadingState (NEW)

**Package**: `internal/tui`

```go
// LoadingState represents the state of lazy loading
type LoadingState int

const (
    LoadingStateIdle LoadingState = iota
    LoadingStateLoading
    LoadingStateComplete
    LoadingStateError
)

// LazyLoadConfig contains configuration for lazy loading
type LazyLoadConfig struct {
    BatchSize           int // Number of items to load per batch
    ConversationThreshold int // Threshold for lazy loading conversations (50)
    MessageThreshold    int // Threshold for lazy loading messages (100)
}

// DefaultLazyLoadConfig returns the default lazy loading configuration
func DefaultLazyLoadConfig() LazyLoadConfig {
    return LazyLoadConfig{
        BatchSize:            20,
        ConversationThreshold: 50,
        MessageThreshold:      100,
    }
}
```

---

## Relationships

```
┌─────────────────┐
│    Version      │ (standalone, no relationships)
└─────────────────┘

┌─────────────────┐     1:N     ┌─────────────────┐
│    Project      │────────────▶│  Conversation   │
└─────────────────┘             └─────────────────┘
                                        │
                                        │ 1:N
                                        ▼
                                ┌─────────────────┐
                                │    LogEntry     │
                                └─────────────────┘
                                        │
                                        │ 1:1 (optional)
                                        ▼
                                ┌─────────────────┐
                                │   TokenUsage    │
                                └─────────────────┘
```

---

## Validation Rules

### TokenUsage
- All token counts must be >= 0
- If all counts are 0, display "N/A" instead of "0"

### Conversation Metadata
- Duration: Only calculated if both first and last timestamps are valid
- Model: Use first assistant message's model; if none, display "Unknown"
- TurnCount: Count of user entries (each user message = 1 turn)

### Version
- If Version == "dev" and Commit != "unknown": Display "dev-{commit[:7]}"
- If Version == "dev" and Commit == "unknown": Display "dev"
- Otherwise: Display Version as-is

---

## State Transitions

### LoadingState

```
┌──────────┐   startLoad()   ┌───────────┐
│   Idle   │────────────────▶│  Loading  │
└──────────┘                 └───────────┘
     ▲                            │
     │                            │ onSuccess() / onError()
     │                            ▼
     │                    ┌───────────────┐
     │                    │   Complete    │
     │                    │   or Error    │
     │                    └───────────────┘
     │                            │
     └────────────────────────────┘
           reset() or retry()
```

---

## Data Volume Assumptions

| Entity | Expected Count | Memory Impact |
|--------|----------------|---------------|
| Projects | 10-100 | Negligible |
| Conversations per project | 10-1000 | ~1KB metadata each |
| Messages per conversation | 10-10000 | ~2KB rendered each |
| Token counts | Per assistant message | 32 bytes each |

**Memory Budget**:
- Metadata: ~1MB for 1000 conversations
- Lazy loading keeps only visible + buffer items in memory
- Target: <100MB total (per Constitution IV)
