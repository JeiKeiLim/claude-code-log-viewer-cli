package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/providers/codex"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/providers/opencode"
)

// S1: Browse Codex sessions alongside Claude Code
func TestCodexProvider_DiscoverAndParse(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "fixtures", "codex")
	provider := codex.NewProvider(codex.WithBasePath(fixtureDir))

	if !provider.IsAvailable() {
		t.Fatal("Codex provider should be available with fixture data")
	}

	projects, err := provider.DiscoverProjects()
	if err != nil {
		t.Fatalf("DiscoverProjects failed: %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("Expected at least one project from Codex fixtures")
	}

	// Verify project has expected fields
	p := projects[0]
	if p.Directory == "" {
		t.Fatal("Project directory should not be empty")
	}

	sessions, err := provider.DiscoverSessions(p)
	if err != nil {
		t.Fatalf("DiscoverSessions failed: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("Expected at least one session")
	}

	entries, err := provider.ParseSession(sessions[0])
	if err != nil {
		t.Fatalf("ParseSession failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("Expected at least one entry from Codex session")
	}

	// Verify entries implement ConversationEntry
	for _, e := range entries {
		if e.AgentType() != agent.AgentCodex {
			t.Errorf("Expected AgentCodex, got %v", e.AgentType())
		}
		if e.Timestamp().IsZero() {
			t.Error("Entry timestamp should not be zero")
		}
	}
}

// S2: View OpenCode session from SQLite
func TestOpenCodeProvider_DiscoverAndParse(t *testing.T) {
	fixtureDB := filepath.Join("..", "..", "testdata", "fixtures", "opencode", "test.db")
	if _, err := os.Stat(fixtureDB); os.IsNotExist(err) {
		t.Skip("OpenCode fixture DB not yet created")
	}

	provider := opencode.NewProvider(opencode.WithDBPath(fixtureDB))
	if !provider.IsAvailable() {
		t.Fatal("OpenCode provider should be available with fixture DB")
	}

	projects, err := provider.DiscoverProjects()
	if err != nil {
		t.Fatalf("DiscoverProjects failed: %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("Expected at least one project from OpenCode fixtures")
	}

	sessions, err := provider.DiscoverSessions(projects[0])
	if err != nil {
		t.Fatalf("DiscoverSessions failed: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("Expected at least one session")
	}

	entries, err := provider.ParseSession(sessions[0])
	if err != nil {
		t.Fatalf("ParseSession failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("Expected at least one entry from OpenCode session")
	}

	// Verify entries have content blocks
	for _, e := range entries {
		if e.AgentType() != agent.AgentOpenCode {
			t.Errorf("Expected AgentOpenCode, got %v", e.AgentType())
		}
	}
}

// S5: Token usage display for all agents
func TestTokenUsage_AcrossProviders(t *testing.T) {
	// Verify TokenStats has correct fields for all agents
	stats := agent.TokenStats{
		InputTokens:     100,
		OutputTokens:    50,
		CachedTokens:    25,
		ReasoningTokens: 10,
	}
	total := stats.Total()
	if total != 185 {
		t.Errorf("Expected total 185, got %d", total)
	}

	// Verify TokenStats is zero-value safe
	empty := agent.TokenStats{}
	if empty.Total() != 0 {
		t.Error("Empty TokenStats should have zero total")
	}
}

// S6: Agent auto-hide when no sessions
func TestProviderAvailability_HideWhenEmpty(t *testing.T) {
	// Codex with non-existent path should not be available
	provider := codex.NewProvider(codex.WithBasePath("/nonexistent/path"))
	if provider.IsAvailable() {
		t.Error("Codex provider should not be available with non-existent path")
	}
}

// S7: Pipeline mode auto-detection
func TestFormatAutoDetection_CodexJSONL(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "codex", "rollout-sample.jsonl")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Skip("Codex fixture not yet created")
	}

	detected := agent.DetectFormat(data)
	if detected != agent.AgentCodex {
		t.Errorf("Expected AgentCodex, got %v", detected)
	}
}

func TestFormatAutoDetection_ClaudeCodeJSONL(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "claude-code", "sample.jsonl")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Skip("Claude Code fixture not yet created")
	}

	detected := agent.DetectFormat(data)
	if detected != agent.AgentClaudeCode {
		t.Errorf("Expected AgentClaudeCode, got %v", detected)
	}
}

// A1: No crash on corrupted JSONL
func TestCodexParser_CorruptedLines(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "codex", "corrupted.jsonl")
	if _, err := os.Stat(fixture); os.IsNotExist(err) {
		t.Skip("Corrupted fixture not yet created")
	}
	provider := codex.NewProvider(codex.WithBasePath(filepath.Dir(fixture)))

	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}

	// Parser should not panic or return error for corrupted lines
	entries, errs := provider.ParseBytes(data)
	if len(entries) == 0 {
		t.Error("Should parse at least some valid entries from mixed file")
	}
	if errs == 0 {
		t.Error("Should report parse errors for corrupted lines")
	}
}

// A8: No regression on Claude Code functionality
func TestClaudeCodeProvider_NoRegression(t *testing.T) {
	// After refactoring, existing Claude Code test fixtures must still parse correctly
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "claude-code", "sample.jsonl")
	if _, err := os.Stat(fixture); os.IsNotExist(err) {
		t.Skip("Claude Code fixture not yet created")
	}

	// The Claude Code provider should produce the same results as the current parser
	// This test will be expanded once the provider interface is defined
}
