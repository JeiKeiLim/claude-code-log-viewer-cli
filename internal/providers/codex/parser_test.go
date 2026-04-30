package codex

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

// fixturePath returns the path to a test fixture file.
func fixturePath(name string) string {
	return filepath.Join("..", "..", "..", "testdata", "fixtures", "codex", name)
}

// --- Compile-time interface check ---

func TestProviderImplementsAgentProvider(t *testing.T) {
	var _ agent.AgentProvider = (*Provider)(nil)
}

// --- Provider method tests ---

func TestProviderType(t *testing.T) {
	p := NewProvider()
	if p.Type() != agent.AgentCodex {
		t.Errorf("Type() = %q, want %q", p.Type(), agent.AgentCodex)
	}
}

func TestProviderDisplayName(t *testing.T) {
	p := NewProvider()
	if p.DisplayName() != "Codex" {
		t.Errorf("DisplayName() = %q, want %q", p.DisplayName(), "Codex")
	}
}

func TestProviderBadge(t *testing.T) {
	p := NewProvider()
	if p.Badge() != "[X]" {
		t.Errorf("Badge() = %q, want %q", p.Badge(), "[X]")
	}
}

func TestProviderIsAvailableNonExistent(t *testing.T) {
	p := NewProvider(WithBasePath("/nonexistent/path"))
	if p.IsAvailable() {
		t.Error("IsAvailable() = true for nonexistent path, want false")
	}
}

func TestProviderIsAvailableWithCodexDir(t *testing.T) {
	tmpDir := t.TempDir()
	codexSessions := filepath.Join(tmpDir, ".codex", "sessions")
	if err := os.MkdirAll(codexSessions, 0o755); err != nil {
		t.Fatalf("failed to create sessions dir: %v", err)
	}
	p := NewProvider(WithBasePath(tmpDir))
	if !p.IsAvailable() {
		t.Error("IsAvailable() = false when .codex/sessions exists, want true")
	}
}

// --- Simple session fixture test ---

func TestParseSimpleSession(t *testing.T) {
	data, err := os.ReadFile(fixturePath("simple-session.jsonl"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	result, err := ParseCodexJSONL(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}

	if result.SessionID != "sess-simple-001" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-simple-001")
	}
	if result.CWD != "/home/user/myproject" {
		t.Errorf("CWD = %q, want %q", result.CWD, "/home/user/myproject")
	}
	if result.Model != "o3" {
		t.Errorf("Model = %q, want %q", result.Model, "o3")
	}
	if len(result.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(result.Entries))
	}

	// First entry: user message
	entry0 := result.Entries[0]
	if entry0.Type() != agent.EntryTypeUser {
		t.Errorf("entry[0] Type() = %q, want %q", entry0.Type(), agent.EntryTypeUser)
	}
	if entry0.Role() != "user" {
		t.Errorf("entry[0] Role() = %q, want %q", entry0.Role(), "user")
	}
	blocks0 := entry0.ContentBlocks()
	if len(blocks0) != 1 || blocks0[0].Text() != "Fix the bug in main.go" {
		t.Errorf("entry[0] text = %q, want %q", blocks0[0].Text(), "Fix the bug in main.go")
	}

	// Second entry: assistant message
	entry1 := result.Entries[1]
	if entry1.Type() != agent.EntryTypeAssistant {
		t.Errorf("entry[1] Type() = %q, want %q", entry1.Type(), agent.EntryTypeAssistant)
	}
	blocks1 := entry1.ContentBlocks()
	if len(blocks1) != 1 || blocks1[0].Text() != "I'll fix the bug for you." {
		t.Errorf("entry[1] text = %q, want %q", blocks1[0].Text(), "I'll fix the bug for you.")
	}
	if blocks1[0].Phase() != "final" {
		t.Errorf("entry[1] Phase() = %q, want %q", blocks1[0].Phase(), "final")
	}
}

// --- Full session fixture test ---

func TestParseFullSession(t *testing.T) {
	data, err := os.ReadFile(fixturePath("full-session.jsonl"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	result, err := ParseCodexJSONL(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}

	if result.SessionID != "sess-full-001" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-full-001")
	}
	if result.CWD != "/home/user/fullproject" {
		t.Errorf("CWD = %q, want %q", result.CWD, "/home/user/fullproject")
	}

	// Expected entries: user_msg, commentary, agent_msg(final), exec_begin/endo, exec_begin/end(top), user_msg, agent_msg(final) = 8
	// But exec_command_begin/end pairs produce tool_use entries, so:
	// 1. user_message "Run the test suite..."
	// 2. agent_message commentary
	// 3. agent_message final
	// 4. exec_command_begin/end (via event_msg) -> tool_use
	// 5. exec_command_begin/end (top-level) -> tool_use
	// 6. user_message "Great, now commit..."
	// 7. agent_message final
	// Total: 7 entries
	if len(result.Entries) != 7 {
		t.Fatalf("len(Entries) = %d, want 7", len(result.Entries))
	}

	// Check commentary phase on second entry.
	commentary := result.Entries[1]
	if commentary.Type() != agent.EntryTypeAssistant {
		t.Errorf("commentary Type() = %q, want %q", commentary.Type(), agent.EntryTypeAssistant)
	}
	cBlocks := commentary.ContentBlocks()
	if len(cBlocks) != 1 {
		t.Fatalf("commentary ContentBlocks() len = %d, want 1", len(cBlocks))
	}
	if cBlocks[0].ContentType() != agent.ContentBlockCommentary {
		t.Errorf("commentary ContentType() = %q, want %q", cBlocks[0].ContentType(), agent.ContentBlockCommentary)
	}
	if cBlocks[0].Phase() != "commentary" {
		t.Errorf("commentary Phase() = %q, want %q", cBlocks[0].Phase(), "commentary")
	}

	// Check tool_use entry from event_msg exec_command_begin/end.
	toolUse1 := result.Entries[3]
	toolBlocks1 := toolUse1.ContentBlocks()
	if len(toolBlocks1) != 1 {
		t.Fatalf("tool_use[3] ContentBlocks() len = %d, want 1", len(toolBlocks1))
	}
	if toolBlocks1[0].ContentType() != agent.ContentBlockToolUse {
		t.Errorf("tool_use[3] ContentType() = %q, want %q", toolBlocks1[0].ContentType(), agent.ContentBlockToolUse)
	}
	if toolBlocks1[0].ToolName() != "exec" {
		t.Errorf("tool_use[3] ToolName() = %q, want %q", toolBlocks1[0].ToolName(), "exec")
	}
	if toolBlocks1[0].ToolInput()["command"] != "go test ./..." {
		t.Errorf("tool_use[3] command = %v, want %q", toolBlocks1[0].ToolInput()["command"], "go test ./...")
	}

	// Check tool_use entry from top-level exec_command_begin/end.
	toolUse2 := result.Entries[4]
	toolBlocks2 := toolUse2.ContentBlocks()
	if len(toolBlocks2) != 1 {
		t.Fatalf("tool_use[4] ContentBlocks() len = %d, want 1", len(toolBlocks2))
	}
	if toolBlocks2[0].ContentType() != agent.ContentBlockToolUse {
		t.Errorf("tool_use[4] ContentType() = %q, want %q", toolBlocks2[0].ContentType(), agent.ContentBlockToolUse)
	}
	if toolBlocks2[0].ToolOutput() != "" {
		t.Errorf("tool_use[4] ToolOutput() = %q, want empty", toolBlocks2[0].ToolOutput())
	}

	// Check token accumulation.
	// event_msg token_count: 1500 input, 800 output, 200 cached
	// top-level token_count: 3000 input, 1200 output, 500 cached
	// Total: 4500 input, 2000 output, 700 cached
	if result.Tokens.InputTokens != 4500 {
		t.Errorf("Tokens.InputTokens = %d, want 4500", result.Tokens.InputTokens)
	}
	if result.Tokens.OutputTokens != 2000 {
		t.Errorf("Tokens.OutputTokens = %d, want 2000", result.Tokens.OutputTokens)
	}
	if result.Tokens.CachedTokens != 700 {
		t.Errorf("Tokens.CachedTokens = %d, want 700", result.Tokens.CachedTokens)
	}
}

// --- Corrupted data test ---

func TestParseCorrupted(t *testing.T) {
	data, err := os.ReadFile(fixturePath("corrupted.jsonl"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	result, err := ParseCodexJSONL(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}

	// Should have parsed session_meta + 2 valid user messages + 1 agent message = 3 conversation entries.
	if len(result.Entries) != 3 {
		t.Fatalf("len(Entries) = %d, want 3", len(result.Entries))
	}

	// Should have counted the 2 corrupted lines.
	if result.ParseErrors != 2 {
		t.Errorf("ParseErrors = %d, want 2", result.ParseErrors)
	}

	// Session metadata should still be extracted.
	if result.SessionID != "sess-corrupt-001" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-corrupt-001")
	}

	// Check first user message survived.
	if result.Entries[0].Type() != agent.EntryTypeUser {
		t.Errorf("entry[0] Type() = %q, want %q", result.Entries[0].Type(), agent.EntryTypeUser)
	}
	blocks := result.Entries[0].ContentBlocks()
	if len(blocks) != 1 || blocks[0].Text() != "Hello" {
		t.Errorf("entry[0] text = %q, want %q", blocks[0].Text(), "Hello")
	}
}

// --- Empty input test ---

func TestParseEmpty(t *testing.T) {
	result, err := ParseCodexJSONL(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseCodexJSONL(empty) error: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0", len(result.Entries))
	}
	if result.ParseErrors != 0 {
		t.Errorf("ParseErrors = %d, want 0", result.ParseErrors)
	}
}

// --- Blank lines only test ---

func TestParseBlankLines(t *testing.T) {
	result, err := ParseCodexJSONL(strings.NewReader("\n\n\n"))
	if err != nil {
		t.Fatalf("ParseCodexJSONL(blanks) error: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0", len(result.Entries))
	}
}

// --- User message test ---

func TestParseUserMessage(t *testing.T) {
	input := `{"type":"event_msg","timestamp":"2026-04-30T12:00:00Z","payload":{"event":{"type":"user_message","content":"Hello world"}}}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(result.Entries))
	}
	e := result.Entries[0]
	if e.Type() != agent.EntryTypeUser {
		t.Errorf("Type() = %q, want %q", e.Type(), agent.EntryTypeUser)
	}
	if e.Role() != "user" {
		t.Errorf("Role() = %q, want %q", e.Role(), "user")
	}
	if e.AgentType() != agent.AgentCodex {
		t.Errorf("AgentType() = %q, want %q", e.AgentType(), agent.AgentCodex)
	}

	ts := e.Timestamp()
	expected := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	if !ts.Equal(expected) {
		t.Errorf("Timestamp() = %v, want %v", ts, expected)
	}
}

// --- Agent message final phase test ---

func TestParseAgentMessageFinal(t *testing.T) {
	input := `{"type":"event_msg","timestamp":"2026-04-30T12:01:00Z","payload":{"event":{"type":"agent_message","content":"Done!","phase":"final"}}}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(result.Entries))
	}
	e := result.Entries[0]
	if e.Type() != agent.EntryTypeAssistant {
		t.Errorf("Type() = %q, want %q", e.Type(), agent.EntryTypeAssistant)
	}
	blocks := e.ContentBlocks()
	if len(blocks) != 1 {
		t.Fatalf("ContentBlocks() len = %d, want 1", len(blocks))
	}
	if blocks[0].ContentType() != agent.ContentBlockText {
		t.Errorf("ContentType() = %q, want %q (final phase = text)", blocks[0].ContentType(), agent.ContentBlockText)
	}
	if blocks[0].Phase() != "final" {
		t.Errorf("Phase() = %q, want %q", blocks[0].Phase(), "final")
	}
}

// --- Agent message commentary phase test ---

func TestParseAgentMessageCommentary(t *testing.T) {
	input := `{"type":"event_msg","timestamp":"2026-04-30T12:01:00Z","payload":{"event":{"type":"agent_message","content":"Thinking...","phase":"commentary"}}}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(result.Entries))
	}
	blocks := result.Entries[0].ContentBlocks()
	if blocks[0].ContentType() != agent.ContentBlockCommentary {
		t.Errorf("ContentType() = %q, want %q", blocks[0].ContentType(), agent.ContentBlockCommentary)
	}
	if blocks[0].Phase() != "commentary" {
		t.Errorf("Phase() = %q, want %q", blocks[0].Phase(), "commentary")
	}
}

// --- Token count via event_msg test ---

func TestParseTokenCountViaEventMsg(t *testing.T) {
	input := `{"type":"event_msg","timestamp":"2026-04-30T12:00:00Z","payload":{"event":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"output_tokens":500,"cached_tokens":100}}}}}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if result.Tokens.InputTokens != 1000 {
		t.Errorf("Tokens.InputTokens = %d, want 1000", result.Tokens.InputTokens)
	}
	if result.Tokens.OutputTokens != 500 {
		t.Errorf("Tokens.OutputTokens = %d, want 500", result.Tokens.OutputTokens)
	}
	if result.Tokens.CachedTokens != 100 {
		t.Errorf("Tokens.CachedTokens = %d, want 100", result.Tokens.CachedTokens)
	}
}

// --- Token count top-level test ---

func TestParseTokenCountTopLevel(t *testing.T) {
	input := `{"type":"token_count","timestamp":"2026-04-30T12:00:00Z","info":{"total_token_usage":{"input_tokens":2000,"output_tokens":1000,"cached_tokens":300}}}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if result.Tokens.InputTokens != 2000 {
		t.Errorf("Tokens.InputTokens = %d, want 2000", result.Tokens.InputTokens)
	}
	if result.Tokens.OutputTokens != 1000 {
		t.Errorf("Tokens.OutputTokens = %d, want 1000", result.Tokens.OutputTokens)
	}
	if result.Tokens.CachedTokens != 300 {
		t.Errorf("Tokens.CachedTokens = %d, want 300", result.Tokens.CachedTokens)
	}
}

// --- Exec command via event_msg test ---

func TestParseExecCommandViaEventMsg(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"type":"event_msg","timestamp":"2026-04-30T12:00:00Z","payload":{"event":{"type":"exec_command_begin","command":"go test ./..."}}}` + "\n")
	sb.WriteString(`{"type":"event_msg","timestamp":"2026-04-30T12:00:05Z","payload":{"event":{"type":"exec_command_end","exit_code":0,"output":"PASS\nok  2.345s"}}}` + "\n")

	result, err := ParseCodexJSONL(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1 (begin+end merged)", len(result.Entries))
	}

	tool := result.Entries[0]
	blocks := tool.ContentBlocks()
	if len(blocks) != 1 {
		t.Fatalf("ContentBlocks() len = %d, want 1", len(blocks))
	}
	if blocks[0].ContentType() != agent.ContentBlockToolUse {
		t.Errorf("ContentType() = %q, want %q", blocks[0].ContentType(), agent.ContentBlockToolUse)
	}
	if blocks[0].ToolName() != "exec" {
		t.Errorf("ToolName() = %q, want %q", blocks[0].ToolName(), "exec")
	}
	if blocks[0].ToolInput()["command"] != "go test ./..." {
		t.Errorf("command = %v, want %q", blocks[0].ToolInput()["command"], "go test ./...")
	}
	if blocks[0].ToolOutput() != "PASS\nok  2.345s" {
		t.Errorf("ToolOutput() = %q, want %q", blocks[0].ToolOutput(), "PASS\nok  2.345s")
	}
}

// --- Exec command top-level test ---

func TestParseExecCommandTopLevel(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"type":"exec_command_begin","timestamp":"2026-04-30T12:00:00Z","command":"ls -la","call_id":"call-abc"}` + "\n")
	sb.WriteString(`{"type":"exec_command_end","timestamp":"2026-04-30T12:00:01Z","exit_code":0,"output":"file.go","call_id":"call-abc"}` + "\n")

	result, err := ParseCodexJSONL(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(result.Entries))
	}

	blocks := result.Entries[0].ContentBlocks()
	if len(blocks) != 1 {
		t.Fatalf("ContentBlocks() len = %d, want 1", len(blocks))
	}
	if blocks[0].ToolName() != "exec" {
		t.Errorf("ToolName() = %q, want %q", blocks[0].ToolName(), "exec")
	}
	if blocks[0].ToolInput()["command"] != "ls -la" {
		t.Errorf("command = %v, want %q", blocks[0].ToolInput()["command"], "ls -la")
	}
	if blocks[0].ToolOutput() != "file.go" {
		t.Errorf("ToolOutput() = %q, want %q", blocks[0].ToolOutput(), "file.go")
	}
}

// --- Exec command without matching end (pending flush) test ---

func TestParseExecCommandPending(t *testing.T) {
	input := `{"type":"exec_command_begin","timestamp":"2026-04-30T12:00:00Z","command":"sleep 10","call_id":"call-pending"}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1 (pending flushed)", len(result.Entries))
	}
	blocks := result.Entries[0].ContentBlocks()
	if blocks[0].ToolOutput() != "" {
		t.Errorf("ToolOutput() = %q, want empty (no end received)", blocks[0].ToolOutput())
	}
}

// --- Session meta test ---

func TestParseSessionMeta(t *testing.T) {
	input := `{"type":"session_meta","session_id":"sess-42","cwd":"/home/dev/app","model":"o3","timestamp":"2026-04-30T09:00:00Z"}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if result.SessionID != "sess-42" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-42")
	}
	if result.CWD != "/home/dev/app" {
		t.Errorf("CWD = %q, want %q", result.CWD, "/home/dev/app")
	}
	if result.Model != "o3" {
		t.Errorf("Model = %q, want %q", result.Model, "o3")
	}
	if len(result.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0 (session_meta is not a conversation entry)", len(result.Entries))
	}
}

// --- Session meta nested payload test (real Codex CLI v0.116.0+ format) ---

func TestParseSessionMetaNestedPayload(t *testing.T) {
	input := `{"type":"session_meta","timestamp":"2026-04-30T09:00:00Z","payload":{"id":"019a6ba6-9fd8-71d1-bcf7-1e3286abc131","cwd":"/Users/dev/project","cli_version":"0.124.0","model_provider":"openai"}}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if result.SessionID != "019a6ba6-9fd8-71d1-bcf7-1e3286abc131" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "019a6ba6-9fd8-71d1-bcf7-1e3286abc131")
	}
	if result.CWD != "/Users/dev/project" {
		t.Errorf("CWD = %q, want %q", result.CWD, "/Users/dev/project")
	}
	if len(result.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0 (session_meta is not a conversation entry)", len(result.Entries))
	}
}

// --- Session meta nested payload with model ---

func TestParseSessionMetaNestedPayloadWithModel(t *testing.T) {
	input := `{"type":"session_meta","timestamp":"2026-04-30T09:00:00Z","payload":{"id":"sess-nested-001","cwd":"/proj","model":"o3"}}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if result.SessionID != "sess-nested-001" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-nested-001")
	}
	if result.CWD != "/proj" {
		t.Errorf("CWD = %q, want %q", result.CWD, "/proj")
	}
	if result.Model != "o3" {
		t.Errorf("Model = %q, want %q", result.Model, "o3")
	}
}

// --- Session meta flat format still works (backward compat) ---

func TestParseSessionMetaFlatFormat(t *testing.T) {
	input := `{"type":"session_meta","session_id":"sess-flat","cwd":"/home/user/flat","model":"o3","timestamp":"2026-04-30T09:00:00Z"}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if result.SessionID != "sess-flat" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-flat")
	}
	if result.CWD != "/home/user/flat" {
		t.Errorf("CWD = %q, want %q", result.CWD, "/home/user/flat")
	}
	if result.Model != "o3" {
		t.Errorf("Model = %q, want %q", result.Model, "o3")
	}
}

// --- Unknown line type test ---

func TestParseUnknownType(t *testing.T) {
	input := `{"type":"some_future_type","data":"whatever"}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0", len(result.Entries))
	}
	if result.ParseErrors != 0 {
		t.Errorf("ParseErrors = %d, want 0 (unknown types are silently skipped)", result.ParseErrors)
	}
}

// --- Malformed JSON test ---

func TestParseMalformedJSON(t *testing.T) {
	input := "not json at all\n{broken\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0", len(result.Entries))
	}
	if result.ParseErrors != 2 {
		t.Errorf("ParseErrors = %d, want 2", result.ParseErrors)
	}
}

// --- Mixed realistic session test ---

func TestParseMixedRealisticSession(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"type":"session_meta","timestamp":"2026-04-30T08:00:00Z","payload":{"id":"sess-mix-001","cwd":"/proj","model":"o3"}}` + "\n")
	sb.WriteString(`{"type":"event_msg","timestamp":"2026-04-30T08:00:01Z","payload":{"event":{"type":"user_message","content":"Build the project"}}}` + "\n")
	sb.WriteString(`{"type":"event_msg","timestamp":"2026-04-30T08:00:02Z","payload":{"event":{"type":"agent_message","content":"I'll build it.","phase":"commentary"}}}` + "\n")
	sb.WriteString(`{"type":"event_msg","timestamp":"2026-04-30T08:00:03Z","payload":{"event":{"type":"agent_message","content":"Building now.","phase":"final"}}}` + "\n")
	sb.WriteString(`{"type":"exec_command_begin","timestamp":"2026-04-30T08:00:04Z","command":"make build","call_id":"call-1"}` + "\n")
	sb.WriteString(`{"type":"exec_command_end","timestamp":"2026-04-30T08:00:10Z","exit_code":0,"output":"Build successful","call_id":"call-1"}` + "\n")
	sb.WriteString(`{"type":"token_count","timestamp":"2026-04-30T08:00:11Z","info":{"total_token_usage":{"input_tokens":500,"output_tokens":250,"cached_tokens":50}}}` + "\n")
	sb.WriteString(`{"type":"event_msg","timestamp":"2026-04-30T08:00:12Z","payload":{"event":{"type":"token_count","info":{"total_token_usage":{"input_tokens":300,"output_tokens":150,"cached_tokens":30}}}}}` + "\n")
	sb.WriteString(`{"type":"event_msg","timestamp":"2026-04-30T08:00:15Z","payload":{"event":{"type":"user_message","content":"Run tests"}}}` + "\n")
	sb.WriteString(`{"type":"event_msg","timestamp":"2026-04-30T08:00:16Z","payload":{"event":{"type":"agent_message","content":"Running tests.","phase":"final"}}}` + "\n")

	result, err := ParseCodexJSONL(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}

	// Expected entries: user, commentary, agent(final), tool_use, user, agent(final) = 6
	if len(result.Entries) != 6 {
		t.Fatalf("len(Entries) = %d, want 6", len(result.Entries))
	}

	// Check token accumulation: top-level(500+250+50) + event_msg(300+150+30) = 800+400+80
	if result.Tokens.InputTokens != 800 {
		t.Errorf("Tokens.InputTokens = %d, want 800", result.Tokens.InputTokens)
	}
	if result.Tokens.OutputTokens != 400 {
		t.Errorf("Tokens.OutputTokens = %d, want 400", result.Tokens.OutputTokens)
	}
	if result.Tokens.CachedTokens != 80 {
		t.Errorf("Tokens.CachedTokens = %d, want 80", result.Tokens.CachedTokens)
	}

	// Verify entry types in order.
	expected := []agent.EntryType{
		agent.EntryTypeUser,      // "Build the project"
		agent.EntryTypeAssistant, // commentary
		agent.EntryTypeAssistant, // final
		agent.EntryTypeAssistant, // tool_use
		agent.EntryTypeUser,      // "Run tests"
		agent.EntryTypeAssistant, // final
	}
	for i, want := range expected {
		if result.Entries[i].Type() != want {
			t.Errorf("entry[%d] Type() = %q, want %q", i, result.Entries[i].Type(), want)
		}
	}
}

// --- Multiple token count accumulation test ---

func TestParseTokenCountAccumulation(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"output_tokens":50,"cached_tokens":10}}}` + "\n")
	sb.WriteString(`{"type":"token_count","info":{"total_token_usage":{"input_tokens":200,"output_tokens":100,"cached_tokens":20}}}` + "\n")
	sb.WriteString(`{"type":"token_count","info":{"total_token_usage":{"input_tokens":300,"output_tokens":150,"cached_tokens":30}}}` + "\n")

	result, err := ParseCodexJSONL(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}

	if result.Tokens.InputTokens != 600 {
		t.Errorf("Tokens.InputTokens = %d, want 600", result.Tokens.InputTokens)
	}
	if result.Tokens.OutputTokens != 300 {
		t.Errorf("Tokens.OutputTokens = %d, want 300", result.Tokens.OutputTokens)
	}
	if result.Tokens.CachedTokens != 60 {
		t.Errorf("Tokens.CachedTokens = %d, want 60", result.Tokens.CachedTokens)
	}
}

// --- Timestamp parsing test ---

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"rfc3339", "2026-04-30T12:00:00Z", time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)},
		{"rfc3339_nano", "2026-04-30T12:00:00.123456789Z", time.Date(2026, 4, 30, 12, 0, 0, 123456789, time.UTC)},
		{"empty", "", time.Time{}},
		{"invalid", "not-a-timestamp", time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTimestamp(tt.input)
			if !got.Equal(tt.want) {
				t.Errorf("parseTimestamp(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- DiscoverProjects test with temp dir ---

func TestDiscoverProjects(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, ".codex", "sessions", "2026", "04", "30")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("failed to create sessions dir: %v", err)
	}

	// Create rollout files with nested payload format (real Codex CLI v0.116.0+).
	session1 := `{"type":"session_meta","payload":{"id":"s1","cwd":"/home/user/proj-a","model":"o3"}}` + "\n"
	session2 := `{"type":"session_meta","payload":{"id":"s2","cwd":"/home/user/proj-b","model":"o3"}}` + "\n"
	session3 := `{"type":"session_meta","payload":{"id":"s3","cwd":"/home/user/proj-a","model":"o3"}}` + "\n"

	if err := os.WriteFile(filepath.Join(sessionsDir, "rollout-001.jsonl"), []byte(session1), 0o644); err != nil {
		t.Fatalf("write session1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "rollout-002.jsonl"), []byte(session2), 0o644); err != nil {
		t.Fatalf("write session2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "rollout-003.jsonl"), []byte(session3), 0o644); err != nil {
		t.Fatalf("write session3: %v", err)
	}

	p := NewProvider(WithBasePath(tmpDir))
	projects, err := p.DiscoverProjects()
	if err != nil {
		t.Fatalf("DiscoverProjects() error: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2", len(projects))
	}

	// Projects should be sorted by display name.
	if projects[0].DisplayName != "proj-a" {
		t.Errorf("projects[0].DisplayName = %q, want %q", projects[0].DisplayName, "proj-a")
	}
	if projects[0].SessionCount != 2 {
		t.Errorf("projects[0].SessionCount = %d, want 2", projects[0].SessionCount)
	}
	if projects[1].DisplayName != "proj-b" {
		t.Errorf("projects[1].DisplayName = %q, want %q", projects[1].DisplayName, "proj-b")
	}
	if projects[1].SessionCount != 1 {
		t.Errorf("projects[1].SessionCount = %d, want 1", projects[1].SessionCount)
	}
}

// --- DiscoverSessions test ---

func TestDiscoverSessions(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, ".codex", "sessions", "2026", "04", "30")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("failed to create sessions dir: %v", err)
	}

	sessionContent := `{"type":"session_meta","timestamp":"2026-04-30T10:00:00Z","payload":{"id":"sess-disc-1","cwd":"/home/user/testproj","model":"o3"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-04-30T10:00:01Z","payload":{"event":{"type":"user_message","content":"Hello"}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-04-30T10:00:02Z","payload":{"event":{"type":"agent_message","content":"Hi","phase":"final"}}}` + "\n"

	if err := os.WriteFile(filepath.Join(sessionsDir, "rollout-disc-001.jsonl"), []byte(sessionContent), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	p := NewProvider(WithBasePath(tmpDir))
	project := agent.Project{Path: "/home/user/testproj", Directory: "/home/user/testproj"}
	sessions, err := p.DiscoverSessions(project)
	if err != nil {
		t.Fatalf("DiscoverSessions() error: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(sessions))
	}

	sess := sessions[0]
	if sess.ID != "disc-001" {
		t.Errorf("ID = %q, want %q", sess.ID, "disc-001")
	}
	if sess.AgentType != agent.AgentCodex {
		t.Errorf("AgentType = %q, want %q", sess.AgentType, agent.AgentCodex)
	}
	if sess.Model != "o3" {
		t.Errorf("Model = %q, want %q", sess.Model, "o3")
	}
	if sess.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", sess.MessageCount)
	}
	if sess.TurnCount != 1 {
		t.Errorf("TurnCount = %d, want 1", sess.TurnCount)
	}
	if sess.FirstUserMessage != "Hello" {
		t.Errorf("FirstUserMessage = %q, want %q", sess.FirstUserMessage, "Hello")
	}
}

// --- ParseSession test ---

func TestParseSession(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "rollout-test.jsonl")
	content := `{"type":"session_meta","payload":{"id":"s-parse","cwd":"/proj","model":"o3"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-04-30T10:00:00Z","payload":{"event":{"type":"user_message","content":"test"}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-04-30T10:00:01Z","payload":{"event":{"type":"agent_message","content":"ok","phase":"final"}}}` + "\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	p := NewProvider()
	session := agent.Session{FilePath: tmpFile, AgentType: agent.AgentCodex}
	entries, err := p.ParseSession(session)
	if err != nil {
		t.Fatalf("ParseSession() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Type() != agent.EntryTypeUser {
		t.Errorf("entry[0] Type() = %q, want %q", entries[0].Type(), agent.EntryTypeUser)
	}
}

// --- ParseSessionStream test ---

func TestParseSessionStream(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"s-stream","cwd":"/proj","model":"o3"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-04-30T10:00:00Z","payload":{"event":{"type":"user_message","content":"stream test"}}}` + "\n"

	p := NewProvider()
	entries, err := p.ParseSessionStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseSessionStream() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	blocks := entries[0].ContentBlocks()
	if len(blocks) != 1 || blocks[0].Text() != "stream test" {
		t.Errorf("text = %q, want %q", blocks[0].Text(), "stream test")
	}
}

// --- ParseBytes test ---

func TestParseBytes(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"s-bytes","cwd":"/proj","model":"o3"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-04-30T10:00:00Z","payload":{"event":{"type":"user_message","content":"bytes test"}}}` + "\n"

	p := NewProvider()
	entries, errCount := p.ParseBytes([]byte(input))
	if errCount != 0 {
		t.Errorf("error count = %d, want 0", errCount)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
}

func TestParseBytesCorrupted(t *testing.T) {
	input := "bad json\n" +
		`{"type":"event_msg","timestamp":"2026-04-30T10:00:00Z","payload":{"event":{"type":"user_message","content":"ok"}}}` + "\n"

	p := NewProvider()
	entries, errCount := p.ParseBytes([]byte(input))
	if errCount != 1 {
		t.Errorf("error count = %d, want 1", errCount)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
}

// --- DiscoverProjects empty dir ---

func TestDiscoverProjectsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewProvider(WithBasePath(tmpDir))
	projects, err := p.DiscoverProjects()
	if err != nil {
		t.Fatalf("DiscoverProjects() error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("len(projects) = %d, want 0 for empty dir", len(projects))
	}
}

// --- DiscoverSessions unknown project ---

func TestDiscoverSessionsUnknownProject(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewProvider(WithBasePath(tmpDir))
	project := agent.Project{Path: "/nonexistent"}
	sessions, err := p.DiscoverSessions(project)
	if err != nil {
		t.Fatalf("DiscoverSessions() error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("len(sessions) = %d, want 0 for unknown project", len(sessions))
	}
}

// --- Non-rollout files are ignored ---

func TestNonRolloutFilesIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, ".codex", "sessions", "2026", "04", "30")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("failed to create sessions dir: %v", err)
	}

	// Create a file that doesn't match rollout-*.jsonl pattern.
	if err := os.WriteFile(filepath.Join(sessionsDir, "other-file.jsonl"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// Create a file that matches rollout but not .jsonl extension.
	if err := os.WriteFile(filepath.Join(sessionsDir, "rollout-001.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	p := NewProvider(WithBasePath(tmpDir))
	projects, err := p.DiscoverProjects()
	if err != nil {
		t.Fatalf("DiscoverProjects() error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("len(projects) = %d, want 0 (non-rollout files should be ignored)", len(projects))
	}
}

// --- Event msg with invalid payload ---

func TestEventMsgInvalidPayload(t *testing.T) {
	// event_msg with non-JSON payload should be counted as parse error.
	input := `{"type":"event_msg","timestamp":"2026-04-30T12:00:00Z","payload":"not-json-object"}` + "\n"
	result, err := ParseCodexJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if result.ParseErrors < 1 {
		t.Errorf("ParseErrors = %d, want >= 1", result.ParseErrors)
	}
	if len(result.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0", len(result.Entries))
	}
}

// --- Verify JSON structure matches fixture format ---

func TestFixtureJSONStructure(t *testing.T) {
	data, err := os.ReadFile(fixturePath("full-session.jsonl"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Errorf("full-session.jsonl line %d is not valid JSON: %s", i+1, line)
		}
	}
}

// --- UTF-8 truncation tests (bug fix: byte-based slicing broke multi-byte chars) ---

func TestTruncateStringMultiByteUTF8(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "korean_short_enough",
			input:  "안녕하세요",
			maxLen: 80,
			want:   "안녕하세요",
		},
		{
			name:   "korean_truncated",
			input:  "안녕하세요 세상입니다 이것은 긴 메시지입니다",
			maxLen: 10,
			want:   "안녕하세요 세...",
		},
		{
			name:   "korean_exact_boundary",
			input:  "안녕하세요",
			maxLen: 5,
			want:   "안녕하세요",
		},
		{
			name:   "korean_one_rune_over",
			input:  "안녕하세요",
			maxLen: 4,
			want:   "안...",
		},
		{
			name:   "mixed_ascii_cjk",
			input:  "Hello 안녕하세요 World 세계",
			maxLen: 10,
			want:   "Hello 안...",
		},
		{
			name:   "ascii_only",
			input:  "This is a long ASCII string for testing",
			maxLen: 20,
			want:   "This is a long AS...",
		},
		{
			name:   "empty_string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "short_string",
			input:  "abc",
			maxLen: 10,
			want:   "abc",
		},
		{
			name:   "emoji_multi_byte",
			input:  "🎉🎊🎈🎁🎀🎉🎊🎈🎁🎀🎉",
			maxLen: 5,
			want:   "🎉🎊...",
		},
		{
			name:   "maxlen_very_small",
			input:  "안녕하세요",
			maxLen: 2,
			want:   "안녕",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
			// Verify the result is valid UTF-8 (the core bug being fixed).
			if !utf8.ValidString(got) {
				t.Errorf("truncateString produced invalid UTF-8: %q", got)
			}
		})
	}
}

func TestTruncateStringResultRuneCount(t *testing.T) {
	// Verify that the truncated result never exceeds maxLen runes.
	for _, input := range []string{
		"안녕하세요 세상입니다 이것은 아주 긴 한국어 메시지입니다",
		"Hello World this is a very long English message for testing",
		strings.Repeat("🎉", 50),
	} {
		for maxLen := 1; maxLen <= 100; maxLen++ {
			result := truncateString(input, maxLen)
			runeCount := utf8.RuneCountInString(result)
			if runeCount > maxLen {
				t.Errorf("truncateString(%q, %d) = %q has %d runes, exceeds maxLen", input, maxLen, result, runeCount)
			}
		}
	}
}

// --- Exec command key collision tests (bug fix: command text as fallback key) ---

func TestParseExecCommandNoCallIDNoCollision(t *testing.T) {
	// Two exec_command_begin with the SAME command and NO call_id must both
	// produce entries (not collide in the pending map).
	var sb strings.Builder
	sb.WriteString(`{"type":"exec_command_begin","timestamp":"2026-04-30T12:00:00Z","command":"go test ./..."}` + "\n")
	sb.WriteString(`{"type":"exec_command_end","timestamp":"2026-04-30T12:00:01Z","exit_code":0,"output":"PASS","command":"go test ./..."}` + "\n")
	sb.WriteString(`{"type":"exec_command_begin","timestamp":"2026-04-30T12:00:02Z","command":"go test ./..."}` + "\n")
	sb.WriteString(`{"type":"exec_command_end","timestamp":"2026-04-30T12:00:03Z","exit_code":0,"output":"ok","command":"go test ./..."}` + "\n")

	result, err := ParseCodexJSONL(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2 (both commands must produce entries)", len(result.Entries))
	}

	// Both entries should have the correct command.
	for i, e := range result.Entries {
		blocks := e.ContentBlocks()
		if len(blocks) != 1 {
			t.Fatalf("entry[%d] ContentBlocks() len = %d, want 1", i, len(blocks))
		}
		if blocks[0].ToolInput()["command"] != "go test ./..." {
			t.Errorf("entry[%d] command = %v, want %q", i, blocks[0].ToolInput()["command"], "go test ./...")
		}
	}

	// First entry gets "PASS" output, second gets "ok".
	out0 := result.Entries[0].ContentBlocks()[0].ToolOutput()
	out1 := result.Entries[1].ContentBlocks()[0].ToolOutput()
	if out0 != "PASS" {
		t.Errorf("entry[0] output = %q, want %q", out0, "PASS")
	}
	if out1 != "ok" {
		t.Errorf("entry[1] output = %q, want %q", out1, "ok")
	}
}

func TestParseExecCommandNoCallIDThreeCommands(t *testing.T) {
	// Three different commands without call_id — each must produce its own entry.
	var sb strings.Builder
	sb.WriteString(`{"type":"exec_command_begin","timestamp":"2026-04-30T12:00:00Z","command":"ls"}` + "\n")
	sb.WriteString(`{"type":"exec_command_end","timestamp":"2026-04-30T12:00:01Z","exit_code":0,"output":"file1.go","command":"ls"}` + "\n")
	sb.WriteString(`{"type":"exec_command_begin","timestamp":"2026-04-30T12:00:02Z","command":"ls"}` + "\n")
	sb.WriteString(`{"type":"exec_command_end","timestamp":"2026-04-30T12:00:03Z","exit_code":0,"output":"file2.go","command":"ls"}` + "\n")
	sb.WriteString(`{"type":"exec_command_begin","timestamp":"2026-04-30T12:00:04Z","command":"ls"}` + "\n")
	sb.WriteString(`{"type":"exec_command_end","timestamp":"2026-04-30T12:00:05Z","exit_code":0,"output":"file3.go","command":"ls"}` + "\n")

	result, err := ParseCodexJSONL(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("len(Entries) = %d, want 3", len(result.Entries))
	}

	expectedOutputs := []string{"file1.go", "file2.go", "file3.go"}
	for i, want := range expectedOutputs {
		blocks := result.Entries[i].ContentBlocks()
		if len(blocks) != 1 {
			t.Fatalf("entry[%d] ContentBlocks() len = %d, want 1", i, len(blocks))
		}
		got := blocks[0].ToolOutput()
		if got != want {
			t.Errorf("entry[%d] output = %q, want %q", i, got, want)
		}
	}
}

func TestParseExecCommandMixedWithAndWithoutCallID(t *testing.T) {
	// Mix commands with and without call_id to verify they don't interfere.
	var sb strings.Builder
	sb.WriteString(`{"type":"exec_command_begin","timestamp":"2026-04-30T12:00:00Z","command":"make","call_id":"call-1"}` + "\n")
	sb.WriteString(`{"type":"exec_command_begin","timestamp":"2026-04-30T12:00:01Z","command":"make"}` + "\n")
	sb.WriteString(`{"type":"exec_command_end","timestamp":"2026-04-30T12:00:02Z","exit_code":0,"output":"built","call_id":"call-1"}` + "\n")
	sb.WriteString(`{"type":"exec_command_end","timestamp":"2026-04-30T12:00:03Z","exit_code":0,"output":"built again","command":"make"}` + "\n")

	result, err := ParseCodexJSONL(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("ParseCodexJSONL() error: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(result.Entries))
	}
}
