package agent

import "time"

// EntryType distinguishes user messages from assistant responses.
type EntryType string

const (
	EntryTypeUser      EntryType = "user"
	EntryTypeAssistant EntryType = "assistant"
)

// ContentBlockType identifies the kind of content within a message block.
type ContentBlockType string

const (
	ContentBlockText       ContentBlockType = "text"
	ContentBlockThinking   ContentBlockType = "thinking"
	ContentBlockToolUse    ContentBlockType = "tool_use"
	ContentBlockReasoning  ContentBlockType = "reasoning"
	ContentBlockCommentary ContentBlockType = "commentary"
)

// ConversationEntry is a provider-agnostic representation of one message in a session.
type ConversationEntry interface {
	Type() EntryType
	Timestamp() time.Time
	Role() string
	ContentBlocks() []ContentBlock
	TokenUsage() TokenStats
	AgentType() AgentType
	SessionID() string
}

// ContentBlock is a single block within a conversation entry.
type ContentBlock interface {
	ContentType() ContentBlockType
	Text() string
	// ToolName returns the tool name for tool_use blocks, empty otherwise.
	ToolName() string
	// ToolInput returns the tool input for tool_use blocks, nil otherwise.
	ToolInput() map[string]any
	// ToolOutput returns the tool result for tool_use blocks, empty otherwise.
	ToolOutput() string
	// Phase returns the execution phase ("commentary", "final", "") for Codex blocks.
	Phase() string
}

// TokenStats holds token consumption data for a single entry or session.
type TokenStats struct {
	InputTokens     int
	OutputTokens    int
	CachedTokens    int
	ReasoningTokens int
}

// Total returns the sum of all token fields.
func (s TokenStats) Total() int {
	return s.InputTokens + s.OutputTokens + s.CachedTokens + s.ReasoningTokens
}

// IsZero returns true if all token fields are zero.
func (s TokenStats) IsZero() bool {
	return s.Total() == 0
}

// Add returns a new TokenStats with the other's fields added.
func (s TokenStats) Add(other TokenStats) TokenStats {
	return TokenStats{
		InputTokens:     s.InputTokens + other.InputTokens,
		OutputTokens:    s.OutputTokens + other.OutputTokens,
		CachedTokens:    s.CachedTokens + other.CachedTokens,
		ReasoningTokens: s.ReasoningTokens + other.ReasoningTokens,
	}
}

// BasicEntry is a concrete ConversationEntry implementation for provider adapters.
type BasicEntry struct {
	EntryType      EntryType
	EntryTimestamp time.Time
	EntryRole      string
	Blocks         []ContentBlock
	EntryTokens    TokenStats
	EntryAgent     AgentType
	EntrySession   string
}

func (e BasicEntry) Type() EntryType               { return e.EntryType }
func (e BasicEntry) Timestamp() time.Time          { return e.EntryTimestamp }
func (e BasicEntry) Role() string                  { return e.EntryRole }
func (e BasicEntry) ContentBlocks() []ContentBlock { return e.Blocks }
func (e BasicEntry) TokenUsage() TokenStats        { return e.EntryTokens }
func (e BasicEntry) AgentType() AgentType          { return e.EntryAgent }
func (e BasicEntry) SessionID() string             { return e.EntrySession }

// BasicBlock is a concrete ContentBlock implementation for provider adapters.
type BasicBlock struct {
	BlockType   ContentBlockType
	BlockText   string
	BlockTool   string
	BlockInput  map[string]any
	BlockOutput string
	BlockPhase  string
}

func (b BasicBlock) ContentType() ContentBlockType { return b.BlockType }
func (b BasicBlock) Text() string                  { return b.BlockText }
func (b BasicBlock) ToolName() string              { return b.BlockTool }
func (b BasicBlock) ToolInput() map[string]any     { return b.BlockInput }
func (b BasicBlock) ToolOutput() string            { return b.BlockOutput }
func (b BasicBlock) Phase() string                 { return b.BlockPhase }
