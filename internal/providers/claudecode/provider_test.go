package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// --- Compile-time interface check ---

func TestClaudeCodeProviderImplementsInterface(t *testing.T) {
	// This line will fail to compile if ClaudeCodeProvider does not implement AgentProvider.
	var _ agent.AgentProvider = (*ClaudeCodeProvider)(nil)
}

// --- Provider metadata methods ---

func TestType(t *testing.T) {
	p := &ClaudeCodeProvider{}
	if got := p.Type(); got != agent.AgentClaudeCode {
		t.Errorf("Type() = %q, want %q", got, agent.AgentClaudeCode)
	}
}

func TestDisplayName(t *testing.T) {
	p := &ClaudeCodeProvider{}
	if got := p.DisplayName(); got != "Claude Code" {
		t.Errorf("DisplayName() = %q, want %q", got, "Claude Code")
	}
}

func TestBadge(t *testing.T) {
	p := &ClaudeCodeProvider{}
	if got := p.Badge(); got != "[C]" {
		t.Errorf("Badge() = %q, want %q", got, "[C]")
	}
}

// --- IsAvailable ---

func TestIsAvailable_NoClaudeDir(t *testing.T) {
	// Create a temp dir and set HOME to it so ~/.claude/projects/ does not exist.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	p := &ClaudeCodeProvider{}
	if p.IsAvailable() {
		t.Error("IsAvailable() = true, want false when ~/.claude/projects/ does not exist")
	}
}

func TestIsAvailable_WithClaudeDir(t *testing.T) {
	tmpDir := t.TempDir()
	projectsPath := filepath.Join(tmpDir, ".claude", "projects")
	if err := os.MkdirAll(projectsPath, 0o755); err != nil {
		t.Fatalf("failed to create projects dir: %v", err)
	}
	t.Setenv("HOME", tmpDir)

	p := &ClaudeCodeProvider{}
	if !p.IsAvailable() {
		t.Error("IsAvailable() = false, want true when ~/.claude/projects/ exists")
	}
}

// --- convertProject ---

func TestConvertProject(t *testing.T) {
	input := types.Project{
		EncodedName:       "-Users-alice-myproject",
		DecodedPath:       "/Users/alice/myproject",
		DisplayName:       "myproject",
		DirPath:           "/Users/alice/.claude/projects/-Users-alice-myproject",
		ConversationCount: 7,
	}

	got := convertProject(input)

	if got.Path != "/Users/alice/myproject" {
		t.Errorf("Path = %q, want %q", got.Path, "/Users/alice/myproject")
	}
	if got.Directory != "/Users/alice/.claude/projects/-Users-alice-myproject" {
		t.Errorf("Directory = %q, want %q", got.Directory, "/Users/alice/.claude/projects/-Users-alice-myproject")
	}
	if got.DisplayName != "myproject" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "myproject")
	}
	if got.AgentType != agent.AgentClaudeCode {
		t.Errorf("AgentType = %q, want %q", got.AgentType, agent.AgentClaudeCode)
	}
	if got.SessionCount != 7 {
		t.Errorf("SessionCount = %d, want 7", got.SessionCount)
	}
}

// --- convertConversation ---

func TestConvertConversation(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	input := types.Conversation{
		FilePath:         "/home/.claude/projects/-home-proj/abc-123.jsonl",
		LastModified:     now.Add(2 * time.Hour),
		CreationTime:     now,
		MessageCount:     10,
		FirstUserMessage: "Fix the bug",
		TotalTokens: types.TokenUsage{
			InputTokens:              500,
			OutputTokens:             200,
			CacheCreationInputTokens: 50,
			CacheReadInputTokens:     30,
		},
		Model:     "claude-opus-4-5-20251101",
		TurnCount: 5,
	}

	got := convertConversation(input, "/home/proj")

	if got.ID != "abc-123" {
		t.Errorf("ID = %q, want %q", got.ID, "abc-123")
	}
	if got.ProjectPath != "/home/proj" {
		t.Errorf("ProjectPath = %q, want %q", got.ProjectPath, "/home/proj")
	}
	if got.FilePath != "/home/.claude/projects/-home-proj/abc-123.jsonl" {
		t.Errorf("FilePath = %q, want original path", got.FilePath)
	}
	if got.AgentType != agent.AgentClaudeCode {
		t.Errorf("AgentType = %q, want %q", got.AgentType, agent.AgentClaudeCode)
	}
	if !got.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, now)
	}
	if !got.LastModified.Equal(now.Add(2 * time.Hour)) {
		t.Errorf("LastModified = %v, want %v", got.LastModified, now.Add(2*time.Hour))
	}
	if got.MessageCount != 10 {
		t.Errorf("MessageCount = %d, want 10", got.MessageCount)
	}
	if got.FirstUserMessage != "Fix the bug" {
		t.Errorf("FirstUserMessage = %q, want %q", got.FirstUserMessage, "Fix the bug")
	}
	if got.Model != "claude-opus-4-5-20251101" {
		t.Errorf("Model = %q, want %q", got.Model, "claude-opus-4-5-20251101")
	}
	// CacheCreationInputTokens + CacheReadInputTokens = CachedTokens
	if got.Tokens.CachedTokens != 80 {
		t.Errorf("Tokens.CachedTokens = %d, want 80 (50+30)", got.Tokens.CachedTokens)
	}
	if got.Tokens.InputTokens != 500 {
		t.Errorf("Tokens.InputTokens = %d, want 500", got.Tokens.InputTokens)
	}
	if got.Tokens.OutputTokens != 200 {
		t.Errorf("Tokens.OutputTokens = %d, want 200", got.Tokens.OutputTokens)
	}
	if got.TurnCount != 5 {
		t.Errorf("TurnCount = %d, want 5", got.TurnCount)
	}
}

// --- convertLogEntry ---

func TestConvertLogEntry_User(t *testing.T) {
	ts := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	input := types.LogEntry{
		Type:      types.EntryTypeUser,
		UUID:      "uuid-1",
		SessionID: "sess-1",
		Timestamp: ts,
		Message: types.Message{
			Role:        "user",
			TextContent: "Hello, Claude!",
		},
	}

	got := convertLogEntry(input)

	if got.Type() != agent.EntryTypeUser {
		t.Errorf("Type() = %q, want %q", got.Type(), agent.EntryTypeUser)
	}
	if !got.Timestamp().Equal(ts) {
		t.Errorf("Timestamp() = %v, want %v", got.Timestamp(), ts)
	}
	if got.Role() != "user" {
		t.Errorf("Role() = %q, want %q", got.Role(), "user")
	}
	if got.AgentType() != agent.AgentClaudeCode {
		t.Errorf("AgentType() = %q, want %q", got.AgentType(), agent.AgentClaudeCode)
	}
	if got.SessionID() != "sess-1" {
		t.Errorf("SessionID() = %q, want %q", got.SessionID(), "sess-1")
	}

	blocks := got.ContentBlocks()
	if len(blocks) != 1 {
		t.Fatalf("ContentBlocks() len = %d, want 1", len(blocks))
	}
	if blocks[0].ContentType() != agent.ContentBlockText {
		t.Errorf("block ContentType() = %q, want %q", blocks[0].ContentType(), agent.ContentBlockText)
	}
	if blocks[0].Text() != "Hello, Claude!" {
		t.Errorf("block Text() = %q, want %q", blocks[0].Text(), "Hello, Claude!")
	}
}

func TestConvertLogEntry_Assistant(t *testing.T) {
	ts := time.Date(2026, 4, 30, 10, 1, 0, 0, time.UTC)
	input := types.LogEntry{
		Type:      types.EntryTypeAssistant,
		UUID:      "uuid-2",
		SessionID: "sess-1",
		Timestamp: ts,
		Message: types.Message{
			Role: "assistant",
			Content: []types.MessageContent{
				{Type: types.ContentTypeText, Text: "Hi there!"},
				{Type: types.ContentTypeThinking, Thinking: "Let me think about this..."},
				{Type: types.ContentTypeToolUse, ToolName: "Bash", ToolInput: map[string]any{"cmd": "ls"}},
			},
		},
		Model: "claude-opus-4-5-20251101",
		Usage: types.TokenUsage{
			InputTokens:              100,
			OutputTokens:             50,
			CacheCreationInputTokens: 10,
			CacheReadInputTokens:     5,
		},
	}

	got := convertLogEntry(input)

	if got.Type() != agent.EntryTypeAssistant {
		t.Errorf("Type() = %q, want %q", got.Type(), agent.EntryTypeAssistant)
	}

	blocks := got.ContentBlocks()
	if len(blocks) != 3 {
		t.Fatalf("ContentBlocks() len = %d, want 3", len(blocks))
	}

	// Text block
	if blocks[0].ContentType() != agent.ContentBlockText {
		t.Errorf("blocks[0] ContentType() = %q, want %q", blocks[0].ContentType(), agent.ContentBlockText)
	}
	if blocks[0].Text() != "Hi there!" {
		t.Errorf("blocks[0] Text() = %q, want %q", blocks[0].Text(), "Hi there!")
	}

	// Thinking block
	if blocks[1].ContentType() != agent.ContentBlockThinking {
		t.Errorf("blocks[1] ContentType() = %q, want %q", blocks[1].ContentType(), agent.ContentBlockThinking)
	}
	if blocks[1].Text() != "Let me think about this..." {
		t.Errorf("blocks[1] Text() = %q, want %q", blocks[1].Text(), "Let me think about this...")
	}

	// Tool use block
	if blocks[2].ContentType() != agent.ContentBlockToolUse {
		t.Errorf("blocks[2] ContentType() = %q, want %q", blocks[2].ContentType(), agent.ContentBlockToolUse)
	}
	if blocks[2].ToolName() != "Bash" {
		t.Errorf("blocks[2] ToolName() = %q, want %q", blocks[2].ToolName(), "Bash")
	}
	if blocks[2].ToolInput()["cmd"] != "ls" {
		t.Errorf("blocks[2] ToolInput()['cmd'] = %v, want %q", blocks[2].ToolInput()["cmd"], "ls")
	}

	// Token usage: CachedTokens = CacheCreation (10) + CacheRead (5) = 15
	tokens := got.TokenUsage()
	if tokens.InputTokens != 100 {
		t.Errorf("TokenUsage().InputTokens = %d, want 100", tokens.InputTokens)
	}
	if tokens.OutputTokens != 50 {
		t.Errorf("TokenUsage().OutputTokens = %d, want 50", tokens.OutputTokens)
	}
	if tokens.CachedTokens != 15 {
		t.Errorf("TokenUsage().CachedTokens = %d, want 15", tokens.CachedTokens)
	}
}

// --- convertContentBlocks ---

func TestConvertMessageContent_StringContent(t *testing.T) {
	msg := types.Message{
		Role:        "user",
		TextContent: "plain text message",
	}

	blocks := convertMessageContent(msg)
	if len(blocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1", len(blocks))
	}
	if blocks[0].ContentType() != agent.ContentBlockText {
		t.Errorf("ContentType() = %q, want %q", blocks[0].ContentType(), agent.ContentBlockText)
	}
	if blocks[0].Text() != "plain text message" {
		t.Errorf("Text() = %q, want %q", blocks[0].Text(), "plain text message")
	}
}

func TestConvertMessageContent_ArrayContent(t *testing.T) {
	msg := types.Message{
		Role: "assistant",
		Content: []types.MessageContent{
			{Type: types.ContentTypeText, Text: "response text"},
			{Type: types.ContentTypeThinking, Thinking: "inner thoughts"},
		},
	}

	blocks := convertMessageContent(msg)
	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(blocks))
	}
	if blocks[0].Text() != "response text" {
		t.Errorf("blocks[0] Text() = %q, want %q", blocks[0].Text(), "response text")
	}
	if blocks[1].ContentType() != agent.ContentBlockThinking {
		t.Errorf("blocks[1] ContentType() = %q, want %q", blocks[1].ContentType(), agent.ContentBlockThinking)
	}
	if blocks[1].Text() != "inner thoughts" {
		t.Errorf("blocks[1] Text() = %q, want %q", blocks[1].Text(), "inner thoughts")
	}
}

func TestConvertMessageContent_EmptyContent(t *testing.T) {
	msg := types.Message{Role: "assistant"}
	blocks := convertMessageContent(msg)
	if blocks != nil {
		t.Errorf("blocks = %v, want nil for empty content", blocks)
	}
}

func TestConvertContentBlock_ToolUse(t *testing.T) {
	input := types.MessageContent{
		Type:      types.ContentTypeToolUse,
		ToolName:  "Read",
		ToolInput: map[string]any{"file_path": "/tmp/test.go"},
	}

	got := convertContentBlock(input)
	if got.ContentType() != agent.ContentBlockToolUse {
		t.Errorf("ContentType() = %q, want %q", got.ContentType(), agent.ContentBlockToolUse)
	}
	if got.ToolName() != "Read" {
		t.Errorf("ToolName() = %q, want %q", got.ToolName(), "Read")
	}
	if got.ToolInput()["file_path"] != "/tmp/test.go" {
		t.Errorf("ToolInput()['file_path'] = %v, want %q", got.ToolInput()["file_path"], "/tmp/test.go")
	}
}

func TestConvertContentBlock_Thinking(t *testing.T) {
	input := types.MessageContent{
		Type:     types.ContentTypeThinking,
		Thinking: "analyzing the code...",
	}

	got := convertContentBlock(input)
	if got.ContentType() != agent.ContentBlockThinking {
		t.Errorf("ContentType() = %q, want %q", got.ContentType(), agent.ContentBlockThinking)
	}
	if got.Text() != "analyzing the code..." {
		t.Errorf("Text() = %q, want %q", got.Text(), "analyzing the code...")
	}
}

// --- ParseSessionStream ---

func TestParseSessionStream_RealisticJSONL(t *testing.T) {
	// Realistic Claude Code JSONL with user and assistant entries.
	var sb strings.Builder
	sb.WriteString(`{"type":"user","uuid":"u1","sessionId":"sess-abc","timestamp":"2026-04-30T10:00:00Z","message":{"role":"user","content":"Fix the test failure"}}` + "\n")
	sb.WriteString(`{"type":"assistant","uuid":"a1","sessionId":"sess-abc","timestamp":"2026-04-30T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"I will fix the test."},{"type":"tool_use","name":"Bash","id":"tool-1","input":{"command":"go test ./..."}}],"model":"claude-opus-4-5-20251101","usage":{"input_tokens":500,"output_tokens":200,"cache_creation_input_tokens":50,"cache_read_input_tokens":30}}}` + "\n")
	sb.WriteString(`{"type":"assistant","uuid":"a2","sessionId":"sess-abc","timestamp":"2026-04-30T10:00:10Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"The test needs a mock..."},{"type":"text","text":"Done!"}],"model":"claude-opus-4-5-20251101","usage":{"input_tokens":300,"output_tokens":100}}}` + "\n")

	p := &ClaudeCodeProvider{}
	entries, err := p.ParseSessionStream(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("ParseSessionStream() error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("ParseSessionStream() returned %d entries, want 3", len(entries))
	}

	// First entry: user
	if entries[0].Type() != agent.EntryTypeUser {
		t.Errorf("entries[0] Type() = %q, want %q", entries[0].Type(), agent.EntryTypeUser)
	}
	if entries[0].Role() != "user" {
		t.Errorf("entries[0] Role() = %q, want %q", entries[0].Role(), "user")
	}
	if entries[0].SessionID() != "sess-abc" {
		t.Errorf("entries[0] SessionID() = %q, want %q", entries[0].SessionID(), "sess-abc")
	}
	userBlocks := entries[0].ContentBlocks()
	if len(userBlocks) != 1 || userBlocks[0].Text() != "Fix the test failure" {
		t.Errorf("entries[0] ContentBlocks() = unexpected: %+v", userBlocks)
	}

	// Second entry: assistant with text + tool_use
	if entries[1].Type() != agent.EntryTypeAssistant {
		t.Errorf("entries[1] Type() = %q, want %q", entries[1].Type(), agent.EntryTypeAssistant)
	}
	assistantBlocks := entries[1].ContentBlocks()
	if len(assistantBlocks) != 2 {
		t.Fatalf("entries[1] ContentBlocks() len = %d, want 2", len(assistantBlocks))
	}
	if assistantBlocks[0].ContentType() != agent.ContentBlockText {
		t.Errorf("assistantBlocks[0] ContentType() = %q, want text", assistantBlocks[0].ContentType())
	}
	if assistantBlocks[1].ContentType() != agent.ContentBlockToolUse {
		t.Errorf("assistantBlocks[1] ContentType() = %q, want tool_use", assistantBlocks[1].ContentType())
	}
	if assistantBlocks[1].ToolName() != "Bash" {
		t.Errorf("assistantBlocks[1] ToolName() = %q, want Bash", assistantBlocks[1].ToolName())
	}
	// Check token usage: 500 input, 200 output, 50+30=80 cached
	tokens1 := entries[1].TokenUsage()
	if tokens1.InputTokens != 500 {
		t.Errorf("entries[1] InputTokens = %d, want 500", tokens1.InputTokens)
	}
	if tokens1.CachedTokens != 80 {
		t.Errorf("entries[1] CachedTokens = %d, want 80", tokens1.CachedTokens)
	}

	// Third entry: assistant with thinking + text
	assistantBlocks2 := entries[2].ContentBlocks()
	if len(assistantBlocks2) != 2 {
		t.Fatalf("entries[2] ContentBlocks() len = %d, want 2", len(assistantBlocks2))
	}
	if assistantBlocks2[0].ContentType() != agent.ContentBlockThinking {
		t.Errorf("assistantBlocks2[0] ContentType() = %q, want thinking", assistantBlocks2[0].ContentType())
	}
	if assistantBlocks2[1].Text() != "Done!" {
		t.Errorf("assistantBlocks2[1] Text() = %q, want Done!", assistantBlocks2[1].Text())
	}
}

func TestParseSessionStream_Empty(t *testing.T) {
	p := &ClaudeCodeProvider{}
	entries, err := p.ParseSessionStream(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseSessionStream() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("ParseSessionStream() returned %d entries, want 0", len(entries))
	}
}

func TestParseSessionStream_InvalidJSON(t *testing.T) {
	p := &ClaudeCodeProvider{}
	entries, err := p.ParseSessionStream(strings.NewReader("not json\n"))
	if err != nil {
		t.Fatalf("ParseSessionStream() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("ParseSessionStream() returned %d entries, want 0 for invalid JSON", len(entries))
	}
}

// --- ParseSession ---

func TestParseSession_EmptyFilePath(t *testing.T) {
	p := &ClaudeCodeProvider{}
	_, err := p.ParseSession(agent.Session{FilePath: ""})
	if err == nil {
		t.Error("ParseSession() with empty FilePath should return an error")
	}
}

func TestParseSession_NonexistentFile(t *testing.T) {
	p := &ClaudeCodeProvider{}
	_, err := p.ParseSession(agent.Session{FilePath: "/nonexistent/path.jsonl"})
	if err == nil {
		t.Error("ParseSession() with nonexistent file should return an error")
	}
}

func TestParseSession_ValidFile(t *testing.T) {
	// Create a temporary JSONL file
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test-session.jsonl")
	content := `{"type":"user","uuid":"u1","sessionId":"s1","timestamp":"2026-04-30T10:00:00Z","message":{"role":"user","content":"Hello"}}
{"type":"assistant","uuid":"a1","sessionId":"s1","timestamp":"2026-04-30T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"Hi!"}]},"model":"claude-sonnet-4-20250514"}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := &ClaudeCodeProvider{}
	entries, err := p.ParseSession(agent.Session{FilePath: jsonlPath})
	if err != nil {
		t.Fatalf("ParseSession() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ParseSession() returned %d entries, want 2", len(entries))
	}
	if entries[0].Type() != agent.EntryTypeUser {
		t.Errorf("entries[0] Type() = %q, want user", entries[0].Type())
	}
	if entries[1].Type() != agent.EntryTypeAssistant {
		t.Errorf("entries[1] Type() = %q, want assistant", entries[1].Type())
	}
}

// --- convertTokenUsage ---

func TestConvertTokenUsage(t *testing.T) {
	input := types.TokenUsage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheCreationInputTokens: 20,
		CacheReadInputTokens:     10,
	}

	got := convertTokenUsage(input)
	if got.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", got.InputTokens)
	}
	if got.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", got.OutputTokens)
	}
	if got.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d, want 30 (20+10)", got.CachedTokens)
	}
	if got.ReasoningTokens != 0 {
		t.Errorf("ReasoningTokens = %d, want 0", got.ReasoningTokens)
	}
}

func TestConvertTokenUsage_Zero(t *testing.T) {
	got := convertTokenUsage(types.TokenUsage{})
	if !got.IsZero() {
		t.Error("zero TokenUsage should convert to zero TokenStats")
	}
}
