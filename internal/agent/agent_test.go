package agent

import (
	"io"
	"strings"
	"testing"
	"time"
)

// --- AgentType tests ---

func TestAgentTypeConstants(t *testing.T) {
	tests := []struct {
		val   AgentType
		str   string
		badge string
	}{
		{AgentClaudeCode, "Claude Code", "[C]"},
		{AgentCodex, "Codex", "[X]"},
		{AgentOpenCode, "OpenCode", "[O]"},
	}
	for _, tt := range tests {
		if got := tt.val.String(); got != tt.str {
			t.Errorf("AgentType(%q).String() = %q, want %q", tt.val, got, tt.str)
		}
		if got := tt.val.Badge(); got != tt.badge {
			t.Errorf("AgentType(%q).Badge() = %q, want %q", tt.val, got, tt.badge)
		}
	}
}

func TestAgentTypeUnknown(t *testing.T) {
	unknown := AgentType("unknown-agent")
	if got := unknown.String(); got != "unknown-agent" {
		t.Errorf("unknown String() = %q, want %q", got, "unknown-agent")
	}
	if got := unknown.Badge(); got != "[?]" {
		t.Errorf("unknown Badge() = %q, want %q", got, "[?]")
	}
}

// --- TokenStats tests ---

func TestTokenStatsTotal(t *testing.T) {
	tests := []struct {
		name  string
		stats TokenStats
		total int
	}{
		{"zero", TokenStats{}, 0},
		{"input_only", TokenStats{InputTokens: 100}, 100},
		{"all_fields", TokenStats{
			InputTokens: 100, OutputTokens: 50,
			CachedTokens: 25, ReasoningTokens: 10,
		}, 185},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stats.Total(); got != tt.total {
				t.Errorf("Total() = %d, want %d", got, tt.total)
			}
		})
	}
}

func TestTokenStatsIsZero(t *testing.T) {
	zero := TokenStats{}
	if !zero.IsZero() {
		t.Error("zero-value TokenStats should be IsZero()")
	}
	nonzero := TokenStats{InputTokens: 1}
	if nonzero.IsZero() {
		t.Error("TokenStats{InputTokens:1} should not be IsZero()")
	}
}

func TestTokenStatsAdd(t *testing.T) {
	a := TokenStats{InputTokens: 100, OutputTokens: 50, CachedTokens: 10, ReasoningTokens: 5}
	b := TokenStats{InputTokens: 200, OutputTokens: 75, CachedTokens: 20, ReasoningTokens: 15}

	got := a.Add(b)
	want := TokenStats{InputTokens: 300, OutputTokens: 125, CachedTokens: 30, ReasoningTokens: 20}
	if got != want {
		t.Errorf("Add() = %+v, want %+v", got, want)
	}
}

// --- Project and Session structs ---

func TestProjectFields(t *testing.T) {
	p := Project{
		Path:         "/home/user/project",
		DisplayName:  "project",
		AgentType:    AgentClaudeCode,
		SessionCount: 5,
	}
	if p.Path != "/home/user/project" {
		t.Errorf("Path = %q, want %q", p.Path, "/home/user/project")
	}
	if p.AgentType != AgentClaudeCode {
		t.Errorf("AgentType = %q, want %q", p.AgentType, AgentClaudeCode)
	}
	if p.SessionCount != 5 {
		t.Errorf("SessionCount = %d, want 5", p.SessionCount)
	}
}

func TestSessionFields(t *testing.T) {
	now := time.Now()
	s := Session{
		ID:               "abc123",
		ProjectPath:      "/home/user/project",
		FilePath:         "/home/user/.claude/projects/-home-user-project/abc123.jsonl",
		AgentType:        AgentCodex,
		CreatedAt:        now,
		LastModified:     now.Add(time.Hour),
		MessageCount:     10,
		FirstUserMessage: "Fix the bug",
		Model:            "o3",
		Tokens:           TokenStats{InputTokens: 500, OutputTokens: 200},
		TurnCount:        5,
	}
	if s.ID != "abc123" {
		t.Errorf("ID = %q, want %q", s.ID, "abc123")
	}
	if s.AgentType != AgentCodex {
		t.Errorf("AgentType = %q, want %q", s.AgentType, AgentCodex)
	}
	if s.TurnCount != 5 {
		t.Errorf("TurnCount = %d, want 5", s.TurnCount)
	}
	if s.Tokens.Total() != 700 {
		t.Errorf("Tokens.Total() = %d, want 700", s.Tokens.Total())
	}
}

// --- BasicEntry (ConversationEntry implementation) ---

func TestBasicEntryInterface(t *testing.T) {
	ts := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	blocks := []ContentBlock{
		BasicBlock{BlockType: ContentBlockText, BlockText: "hello"},
	}

	entry := BasicEntry{
		EntryType:      EntryTypeUser,
		EntryTimestamp: ts,
		EntryRole:      "user",
		Blocks:         blocks,
		EntryTokens:    TokenStats{InputTokens: 10},
		EntryAgent:     AgentClaudeCode,
		EntrySession:   "sess-001",
	}

	// Verify ConversationEntry interface compliance
	var _ ConversationEntry = entry

	if entry.Type() != EntryTypeUser {
		t.Errorf("Type() = %q, want %q", entry.Type(), EntryTypeUser)
	}
	if !entry.Timestamp().Equal(ts) {
		t.Errorf("Timestamp() = %v, want %v", entry.Timestamp(), ts)
	}
	if entry.Role() != "user" {
		t.Errorf("Role() = %q, want %q", entry.Role(), "user")
	}
	if len(entry.ContentBlocks()) != 1 {
		t.Fatalf("ContentBlocks() len = %d, want 1", len(entry.ContentBlocks()))
	}
	if entry.ContentBlocks()[0].Text() != "hello" {
		t.Errorf("ContentBlocks()[0].Text() = %q, want %q", entry.ContentBlocks()[0].Text(), "hello")
	}
	if entry.TokenUsage().InputTokens != 10 {
		t.Errorf("TokenUsage().InputTokens = %d, want 10", entry.TokenUsage().InputTokens)
	}
	if entry.AgentType() != AgentClaudeCode {
		t.Errorf("AgentType() = %q, want %q", entry.AgentType(), AgentClaudeCode)
	}
	if entry.SessionID() != "sess-001" {
		t.Errorf("SessionID() = %q, want %q", entry.SessionID(), "sess-001")
	}
}

func TestBasicEntryAssistant(t *testing.T) {
	entry := BasicEntry{
		EntryType: EntryTypeAssistant,
		EntryRole: "assistant",
		Blocks: []ContentBlock{
			BasicBlock{BlockType: ContentBlockText, BlockText: "response"},
			BasicBlock{BlockType: ContentBlockToolUse, BlockTool: "Bash", BlockInput: map[string]any{"cmd": "ls"}, BlockOutput: "file.go"},
			BasicBlock{BlockType: ContentBlockThinking, BlockText: "reasoning here"},
		},
		EntryTokens: TokenStats{InputTokens: 100, OutputTokens: 50, ReasoningTokens: 20},
		EntryAgent:  AgentCodex,
	}

	if entry.Type() != EntryTypeAssistant {
		t.Errorf("Type() = %q, want %q", entry.Type(), EntryTypeAssistant)
	}
	if len(entry.ContentBlocks()) != 3 {
		t.Fatalf("ContentBlocks() len = %d, want 3", len(entry.ContentBlocks()))
	}

	tool := entry.ContentBlocks()[1]
	if tool.ContentType() != ContentBlockToolUse {
		t.Errorf("tool ContentType() = %q, want %q", tool.ContentType(), ContentBlockToolUse)
	}
	if tool.ToolName() != "Bash" {
		t.Errorf("tool ToolName() = %q, want %q", tool.ToolName(), "Bash")
	}
	if tool.ToolOutput() != "file.go" {
		t.Errorf("tool ToolOutput() = %q, want %q", tool.ToolOutput(), "file.go")
	}

	thinking := entry.ContentBlocks()[2]
	if thinking.ContentType() != ContentBlockThinking {
		t.Errorf("thinking ContentType() = %q, want %q", thinking.ContentType(), ContentBlockThinking)
	}

	total := entry.TokenUsage().Total()
	if total != 170 {
		t.Errorf("TokenUsage().Total() = %d, want 170", total)
	}
}

// --- BasicBlock (ContentBlock implementation) ---

func TestBasicBlockInterface(t *testing.T) {
	var _ ContentBlock = BasicBlock{}
}

func TestBasicBlockText(t *testing.T) {
	b := BasicBlock{BlockType: ContentBlockText, BlockText: "some text"}
	if b.ContentType() != ContentBlockText {
		t.Errorf("ContentType() = %q, want %q", b.ContentType(), ContentBlockText)
	}
	if b.Text() != "some text" {
		t.Errorf("Text() = %q, want %q", b.Text(), "some text")
	}
	if b.ToolName() != "" {
		t.Errorf("ToolName() = %q, want empty", b.ToolName())
	}
	if b.ToolInput() != nil {
		t.Errorf("ToolInput() = %v, want nil", b.ToolInput())
	}
	if b.ToolOutput() != "" {
		t.Errorf("ToolOutput() = %q, want empty", b.ToolOutput())
	}
	if b.Phase() != "" {
		t.Errorf("Phase() = %q, want empty", b.Phase())
	}
}

func TestBasicBlockToolUse(t *testing.T) {
	input := map[string]any{"command": "go test ./..."}
	b := BasicBlock{
		BlockType:   ContentBlockToolUse,
		BlockTool:   "Bash",
		BlockInput:  input,
		BlockOutput: "PASS",
	}
	if b.ContentType() != ContentBlockToolUse {
		t.Errorf("ContentType() = %q, want %q", b.ContentType(), ContentBlockToolUse)
	}
	if b.ToolName() != "Bash" {
		t.Errorf("ToolName() = %q, want %q", b.ToolName(), "Bash")
	}
	if b.ToolInput()["command"] != "go test ./..." {
		t.Errorf("ToolInput()['command'] = %v, want %q", b.ToolInput()["command"], "go test ./...")
	}
	if b.ToolOutput() != "PASS" {
		t.Errorf("ToolOutput() = %q, want %q", b.ToolOutput(), "PASS")
	}
}

func TestBasicBlockCodexPhase(t *testing.T) {
	b := BasicBlock{BlockType: ContentBlockCommentary, BlockText: "commentary text", BlockPhase: "commentary"}
	if b.Phase() != "commentary" {
		t.Errorf("Phase() = %q, want %q", b.Phase(), "commentary")
	}
}

func TestBasicBlockReasoning(t *testing.T) {
	b := BasicBlock{BlockType: ContentBlockReasoning, BlockText: "step-by-step logic"}
	if b.ContentType() != ContentBlockReasoning {
		t.Errorf("ContentType() = %q, want %q", b.ContentType(), ContentBlockReasoning)
	}
}

// --- DetectFormat tests ---

func TestDetectFormatEmpty(t *testing.T) {
	got := DetectFormat([]byte{})
	if got != AgentClaudeCode {
		t.Errorf("DetectFormat(empty) = %q, want %q (default)", got, AgentClaudeCode)
	}
}

func TestDetectFormatClaudeCode(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"user_entry", `{"type":"user","uuid":"abc","sessionId":"s1","message":{"role":"user","content":"hello"}}`},
		{"assistant_entry", `{"type":"assistant","uuid":"def","sessionId":"s1","message":{"role":"assistant","content":[{"type":"text","text":"hi"}]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat([]byte(tt.line))
			if got != AgentClaudeCode {
				t.Errorf("DetectFormat() = %q, want %q", got, AgentClaudeCode)
			}
		})
	}
}

func TestDetectFormatCodex(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"event_msg", `{"type":"event_msg","timestamp":"2026-04-30T12:00:00Z","payload":{}}`},
		{"session_meta", `{"type":"session_meta","cwd":"/home/user","session_id":"abc"}`},
		{"token_count", `{"type":"token_count","input":100,"output":50}`},
		{"exec_command_begin", `{"type":"exec_command_begin","command":"go test"}`},
		{"exec_command_end", `{"type":"exec_command_end","exit_code":0}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat([]byte(tt.line))
			if got != AgentCodex {
				t.Errorf("DetectFormat() = %q, want %q", got, AgentCodex)
			}
		})
	}
}

func TestDetectFormatMultipleLines(t *testing.T) {
	data := []byte("\n\n" + `{"type":"user","uuid":"abc","sessionId":"s1","message":{"role":"user","content":"hi"}}` + "\n")
	got := DetectFormat(data)
	if got != AgentClaudeCode {
		t.Errorf("DetectFormat() = %q, want %q", got, AgentClaudeCode)
	}
}

func TestDetectFormatCodexBeforeClaude(t *testing.T) {
	// Multi-line input: Codex line first, then Claude
	codex := `{"type":"event_msg","timestamp":"2026-04-30T12:00:00Z","payload":{}}`
	claude := `{"type":"user","uuid":"abc","sessionId":"s1","message":{"role":"user","content":"hi"}}`
	data := []byte(codex + "\n" + claude)
	got := DetectFormat(data)
	if got != AgentCodex {
		t.Errorf("DetectFormat() = %q, want %q (first line wins)", got, AgentCodex)
	}
}

func TestDetectFormatInvalidJSON(t *testing.T) {
	// Non-JSON falls back to Claude Code default
	got := DetectFormat([]byte("not json at all"))
	if got != AgentClaudeCode {
		t.Errorf("DetectFormat(invalid) = %q, want %q (default)", got, AgentClaudeCode)
	}
}

func TestDetectFormatUnknownType(t *testing.T) {
	got := DetectFormat([]byte(`{"type":"unknown_type","data":"stuff"}`))
	if got != AgentClaudeCode {
		t.Errorf("DetectFormat(unknown type) = %q, want %q (default)", got, AgentClaudeCode)
	}
}

func TestDetectFormatMultilineCodexPayload(t *testing.T) {
	// Realistic Codex JSONL with nested content
	input := `{"type":"event_msg","timestamp":"2026-04-30T12:00:00Z","event":{"type":"agent_message","message":{"role":"assistant","content":"response"}}}`
	got := DetectFormat([]byte(input))
	if got != AgentCodex {
		t.Errorf("DetectFormat() = %q, want %q", got, AgentCodex)
	}
}

// --- AgentProvider interface compile check ---

func TestAgentProviderInterface(t *testing.T) {
	// Verify mockProvider satisfies AgentProvider at compile time
	var _ AgentProvider = mockProvider{}
}

type mockProvider struct {
	agentType   AgentType
	displayName string
	badge       string
	available   bool
	projects    []Project
	sessions    []Session
	entries     []ConversationEntry
}

func (m mockProvider) Type() AgentType                                     { return m.agentType }
func (m mockProvider) DisplayName() string                                 { return m.displayName }
func (m mockProvider) Badge() string                                       { return m.badge }
func (m mockProvider) IsAvailable() bool                                   { return m.available }
func (m mockProvider) DiscoverProjects() ([]Project, error)                { return m.projects, nil }
func (m mockProvider) DiscoverSessions(_ Project) ([]Session, error)       { return m.sessions, nil }
func (m mockProvider) ParseSession(_ Session) ([]ConversationEntry, error) { return m.entries, nil }
func (m mockProvider) ParseSessionStream(_ io.Reader) ([]ConversationEntry, error) {
	return m.entries, nil
}

func TestMockProviderReturnsValues(t *testing.T) {
	p := mockProvider{
		agentType:   AgentCodex,
		displayName: "Codex",
		badge:       "[X]",
		available:   true,
		projects:    []Project{{Path: "/proj", DisplayName: "proj", AgentType: AgentCodex}},
		sessions:    []Session{{ID: "s1", AgentType: AgentCodex}},
		entries: []ConversationEntry{
			BasicEntry{EntryType: EntryTypeUser, EntryRole: "user", EntryAgent: AgentCodex},
		},
	}

	if p.Type() != AgentCodex {
		t.Errorf("Type() = %q, want %q", p.Type(), AgentCodex)
	}
	if p.DisplayName() != "Codex" {
		t.Errorf("DisplayName() = %q, want %q", p.DisplayName(), "Codex")
	}
	if p.Badge() != "[X]" {
		t.Errorf("Badge() = %q, want %q", p.Badge(), "[X]")
	}
	if !p.IsAvailable() {
		t.Error("IsAvailable() = false, want true")
	}

	projects, err := p.DiscoverProjects()
	if err != nil {
		t.Fatalf("DiscoverProjects() error: %v", err)
	}
	if len(projects) != 1 || projects[0].Path != "/proj" {
		t.Errorf("DiscoverProjects() = %+v, unexpected", projects)
	}

	sessions, err := p.DiscoverSessions(Project{})
	if err != nil {
		t.Fatalf("DiscoverSessions() error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "s1" {
		t.Errorf("DiscoverSessions() = %+v, unexpected", sessions)
	}

	entries, err := p.ParseSession(Session{})
	if err != nil {
		t.Fatalf("ParseSession() error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("ParseSession() returned %d entries, want 1", len(entries))
	}

	streamEntries, err := p.ParseSessionStream(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseSessionStream() error: %v", err)
	}
	if len(streamEntries) != 1 {
		t.Errorf("ParseSessionStream() returned %d entries, want 1", len(streamEntries))
	}
}

// --- SessionWatcher interface compile check ---

func TestSessionWatcherInterface(t *testing.T) {
	var _ SessionWatcher = &mockWatcher{}
}

type mockWatcher struct {
	entries []ConversationEntry
	closed  bool
}

func (m *mockWatcher) NewEntries() ([]ConversationEntry, error) { return m.entries, nil }
func (m *mockWatcher) Close() error                             { m.closed = true; return nil }

func TestMockWatcherBehavior(t *testing.T) {
	w := &mockWatcher{
		entries: []ConversationEntry{
			BasicEntry{EntryType: EntryTypeAssistant, EntryRole: "assistant"},
		},
	}

	entries, err := w.NewEntries()
	if err != nil {
		t.Fatalf("NewEntries() error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("NewEntries() len = %d, want 1", len(entries))
	}
	if entries[0].Type() != EntryTypeAssistant {
		t.Errorf("entry Type() = %q, want %q", entries[0].Type(), EntryTypeAssistant)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if !w.closed {
		t.Error("Close() did not set closed flag")
	}
}

// --- ContentBlockType constants ---

func TestContentBlockTypeConstants(t *testing.T) {
	types := map[ContentBlockType]string{
		ContentBlockText:       "text",
		ContentBlockThinking:   "thinking",
		ContentBlockToolUse:    "tool_use",
		ContentBlockReasoning:  "reasoning",
		ContentBlockCommentary: "commentary",
	}
	for ct, want := range types {
		if string(ct) != want {
			t.Errorf("ContentBlockType constant = %q, want %q", ct, want)
		}
	}
}

// --- EntryType constants ---

func TestEntryTypeConstants(t *testing.T) {
	if string(EntryTypeUser) != "user" {
		t.Errorf("EntryTypeUser = %q, want %q", EntryTypeUser, "user")
	}
	if string(EntryTypeAssistant) != "assistant" {
		t.Errorf("EntryTypeAssistant = %q, want %q", EntryTypeAssistant, "assistant")
	}
}

// --- DetectFormat with realistic multi-line Codex session ---

func TestDetectFormatCodexRealisticSession(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"type":"session_meta","session_id":"sess-abc","cwd":"/home/dev/proj","model":"o3"}` + "\n")
	sb.WriteString(`{"type":"event_msg","timestamp":"2026-04-30T10:00:00Z","event":{"type":"user_message","content":"Fix tests"}}` + "\n")
	sb.WriteString(`{"type":"token_count","input":1500,"output":800}` + "\n")

	got := DetectFormat([]byte(sb.String()))
	if got != AgentCodex {
		t.Errorf("DetectFormat(realistic codex) = %q, want %q", got, AgentCodex)
	}
}

func TestDetectFormatClaudeRealisticSession(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"type":"user","uuid":"u1","sessionId":"s1","timestamp":"2026-04-30T10:00:00Z","message":{"role":"user","content":"hello"}}` + "\n")
	sb.WriteString(`{"type":"assistant","uuid":"a1","sessionId":"s1","timestamp":"2026-04-30T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"hi there"}]}}` + "\n")

	got := DetectFormat([]byte(sb.String()))
	if got != AgentClaudeCode {
		t.Errorf("DetectFormat(realistic claude) = %q, want %q", got, AgentClaudeCode)
	}
}
