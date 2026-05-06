package tui

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// --- Test helpers for agent selector integration ---

type testProvider struct {
	agentType   agent.AgentType
	displayName string
	badge       string
	available   bool
	projects    []agent.Project
	sessions    []agent.Session
	entries     []agent.ConversationEntry
}

func (p *testProvider) Type() agent.AgentType                      { return p.agentType }
func (p *testProvider) DisplayName() string                        { return p.displayName }
func (p *testProvider) Badge() string                              { return p.badge }
func (p *testProvider) IsAvailable() bool                          { return p.available }
func (p *testProvider) DiscoverProjects() ([]agent.Project, error) { return p.projects, nil }
func (p *testProvider) DiscoverSessions(_ agent.Project) ([]agent.Session, error) {
	return p.sessions, nil
}
func (p *testProvider) ParseSession(_ agent.Session) ([]agent.ConversationEntry, error) {
	return p.entries, nil
}
func (p *testProvider) ParseSessionStream(_ io.Reader) ([]agent.ConversationEntry, error) {
	return p.entries, nil
}

type testSessionWatcher struct {
	entries []agent.ConversationEntry
	closed  bool
}

func (w *testSessionWatcher) NewEntries() ([]agent.ConversationEntry, error) {
	entries := w.entries
	w.entries = nil
	return entries, nil
}

func (w *testSessionWatcher) Close() error {
	w.closed = true
	return nil
}

type watchableTestProvider struct {
	*testProvider
	watcher        *testSessionWatcher
	watchedSession agent.Session
}

func (p *watchableTestProvider) WatchSession(session agent.Session) (agent.SessionWatcher, error) {
	p.watchedSession = session
	return p.watcher, nil
}

func makeTestProvider(name string, badge string, projectCount int, sessionCount int) *testProvider {
	projects := make([]agent.Project, projectCount)
	for i := range projects {
		projects[i] = agent.Project{
			Path:         "/tmp/test-project",
			Directory:    "/tmp/test-project",
			DisplayName:  name + "-project",
			AgentType:    agent.AgentClaudeCode,
			SessionCount: sessionCount,
		}
	}
	sessions := make([]agent.Session, sessionCount)
	for i := range sessions {
		sessions[i] = agent.Session{
			ID:           "session-" + string(rune('A'+i)),
			FilePath:     "/tmp/test-session.jsonl",
			AgentType:    agent.AgentClaudeCode,
			CreatedAt:    time.Now().Add(-time.Hour),
			LastModified: time.Now(),
		}
	}
	return &testProvider{
		agentType:   agent.AgentClaudeCode,
		displayName: name,
		badge:       badge,
		available:   true,
		projects:    projects,
		sessions:    sessions,
	}
}

// --- Behavioral tests ---

func TestNewAppModelWithProviders_StartsAtAgentSelector(t *testing.T) {
	p := makeTestProvider("Test", "[T]", 1, 1)
	model := NewAppModelWithProviders([]agent.AgentProvider{p})

	if model.state != viewAgentSelector {
		t.Errorf("expected viewAgentSelector, got %d", model.state)
	}
	if !model.usingProviders {
		t.Error("expected usingProviders = true")
	}
}

func TestAgentSelectedMsg_DisCOVERSProjectsAndTransitionsToProjects(t *testing.T) {
	p := makeTestProvider("Test", "[T]", 2, 3)
	model := NewAppModelWithProviders([]agent.AgentProvider{p})
	model.width = 80
	model.height = 24

	// Simulate agent selection
	newModel, _ := model.Update(AgentSelectedMsg{Provider: p})
	m := newModel.(AppModel)

	if m.state != viewProjects {
		t.Errorf("expected viewProjects after AgentSelectedMsg, got %d", m.state)
	}
	if m.selectedProvider != p {
		t.Error("expected selectedProvider to be set")
	}
}

func TestAgentSelectedMsg_EmptyProjects_StaysOnSelector(t *testing.T) {
	p := &testProvider{
		agentType:   agent.AgentClaudeCode,
		displayName: "Empty",
		badge:       "[E]",
		available:   true,
		projects:    []agent.Project{},
	}
	model := NewAppModelWithProviders([]agent.AgentProvider{p})

	newModel, _ := model.Update(AgentSelectedMsg{Provider: p})
	m := newModel.(AppModel)

	if m.state != viewAgentSelector {
		t.Errorf("expected viewAgentSelector with empty projects, got %d", m.state)
	}
}

func TestViewAgentSelector_RendersSelectorView(t *testing.T) {
	p := makeTestProvider("Test", "[T]", 1, 1)
	model := NewAppModelWithProviders([]agent.AgentProvider{p})
	model.width = 80
	model.height = 24

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view for agent selector state")
	}
}

func TestNewAppModelWithProviders_StartsWithLoadingSelector(t *testing.T) {
	p := makeTestProvider("Test", "[T]", 1, 1)
	model := NewAppModelWithProviders([]agent.AgentProvider{p})

	if len(model.agentSelectorModel.VisibleProviders()) != 0 {
		t.Fatalf("expected provider discovery to be deferred, got %d visible providers", len(model.agentSelectorModel.VisibleProviders()))
	}

	view := model.View()
	if !strings.Contains(view, "Loading agents") {
		t.Fatalf("expected loading selector view, got %q", view)
	}
}

func TestLoadAgentProvidersLoadsSelectorAndAutoSelectsSingleProvider(t *testing.T) {
	p := makeTestProvider("Test", "[T]", 1, 1)
	model := NewAppModelWithProviders([]agent.AgentProvider{p})

	msg := model.loadAgentProviders()()
	loaded, ok := msg.(agentProvidersLoadedMsg)
	if !ok {
		t.Fatalf("expected agentProvidersLoadedMsg, got %T", msg)
	}
	if len(loaded.selector.VisibleProviders()) != 1 {
		t.Fatalf("expected 1 visible provider, got %d", len(loaded.selector.VisibleProviders()))
	}

	newModel, cmd := model.Update(loaded)
	m := newModel.(AppModel)
	if len(m.agentSelectorModel.VisibleProviders()) != 1 {
		t.Fatalf("expected loaded selector to be installed")
	}
	if cmd == nil {
		t.Fatal("expected single provider shortcut command")
	}
	cmdMsg := cmd()
	selected, ok := cmdMsg.(AgentSelectedMsg)
	if !ok {
		t.Fatalf("expected AgentSelectedMsg, got %T", cmdMsg)
	}
	if selected.Provider != p {
		t.Fatal("expected selected provider to be the loaded provider")
	}
}

func TestBackToAgentSelectorMsg_ReturnsToSelector(t *testing.T) {
	p := makeTestProvider("Test", "[T]", 1, 1)
	model := NewAppModelWithProviders([]agent.AgentProvider{p})
	model.width = 80
	model.height = 24

	// First select an agent to go to projects
	newModel, _ := model.Update(AgentSelectedMsg{Provider: p})
	m := newModel.(AppModel)

	if m.state != viewProjects {
		t.Fatalf("expected viewProjects, got %d", m.state)
	}

	// Now go back
	newModel2, _ := m.Update(BackToAgentSelectorMsg{})
	m2 := newModel2.(AppModel)

	if m2.state != viewAgentSelector {
		t.Errorf("expected viewAgentSelector after BackToAgentSelectorMsg, got %d", m2.state)
	}
	if m2.selectedProvider != nil {
		t.Error("expected selectedProvider to be nil after going back")
	}
}

func TestProjectModelWithBack_EscEmitsBackToAgentSelector(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/p1"}}
	model := NewProjectModelWithBack(projects)
	model.SetSize(80, 24)

	// Press esc
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected non-nil command for esc when canGoBack is true")
	}
	msg := cmd()
	if _, ok := msg.(BackToAgentSelectorMsg); !ok {
		t.Errorf("expected BackToAgentSelectorMsg, got %T", msg)
	}
}

func TestProjectModelWithBack_HEmitsBackToAgentSelector(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/p1"}}
	model := NewProjectModelWithBack(projects)
	model.SetSize(80, 24)

	// Press 'h'
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if cmd == nil {
		t.Fatal("expected non-nil command for 'h' when canGoBack is true")
	}
	msg := cmd()
	if _, ok := msg.(BackToAgentSelectorMsg); !ok {
		t.Errorf("expected BackToAgentSelectorMsg, got %T", msg)
	}
}

func TestProjectModelWithoutBack_EscDoesNotGoBack(t *testing.T) {
	projects := []types.Project{{DisplayName: "proj1", DirPath: "/tmp/p1"}}
	model := NewProjectModel(projects) // Without back
	model.SetSize(80, 24)

	// Press esc - should NOT emit BackToAgentSelectorMsg
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Errorf("expected nil command for esc when canGoBack is false, got %v", cmd)
	}
}

func TestProjectSelectedMsg_ClaudeProviderUsesSessionDashboard(t *testing.T) {
	p := makeTestProvider("Claude", "[C]", 1, 1)
	p.agentType = agent.AgentClaudeCode
	model := NewAppModelWithProviders([]agent.AgentProvider{p})
	model.width = 80
	model.height = 24

	newModel, _ := model.Update(AgentSelectedMsg{Provider: p})
	m := newModel.(AppModel)

	newModel, cmd := m.Update(ProjectSelectedMsg{
		Project: types.Project{
			DisplayName: "test-project",
			DirPath:     "/tmp/test-project",
			DecodedPath: "/tmp/test-project",
		},
	})
	m = newModel.(AppModel)

	if m.state != viewSessionDashboard {
		t.Fatalf("expected viewSessionDashboard for Claude provider project selection, got %d", m.state)
	}
	if m.loading {
		t.Fatal("expected dashboard route not provider session loading")
	}
	if cmd == nil {
		t.Fatal("expected session dashboard init command")
	}
}

func TestProjectSelectedMsg_ClaudeProviderNoMultiSessionUsesProviderSessions(t *testing.T) {
	p := makeTestProvider("Claude", "[C]", 1, 1)
	p.agentType = agent.AgentClaudeCode
	model := NewAppModelWithProviders([]agent.AgentProvider{p})
	model.SetNoMultiSession(true)
	model.width = 80
	model.height = 24

	newModel, _ := model.Update(AgentSelectedMsg{Provider: p})
	m := newModel.(AppModel)

	newModel, cmd := m.Update(ProjectSelectedMsg{
		Project: types.Project{
			DisplayName: "test-project",
			DirPath:     "/tmp/test-project",
			DecodedPath: "/tmp/test-project",
		},
	})
	m = newModel.(AppModel)

	if !m.loading {
		t.Fatal("expected provider session loading when no multi-session is set")
	}
	if m.state == viewSessionDashboard {
		t.Fatal("did not expect session dashboard when no multi-session is set")
	}
	if cmd == nil {
		t.Fatal("expected provider session loading command")
	}
}

func TestFullFlow_AgentSelectorToViewer(t *testing.T) {
	entries := []agent.ConversationEntry{
		agent.BasicEntry{
			EntryType:      agent.EntryTypeUser,
			EntryTimestamp: time.Now(),
			EntryRole:      "user",
			Blocks: []agent.ContentBlock{
				agent.BasicBlock{BlockType: agent.ContentBlockText, BlockText: "Hello"},
			},
		},
	}
	p := &testProvider{
		agentType:   agent.AgentCodex,
		displayName: "Test",
		badge:       "[T]",
		available:   true,
		projects: []agent.Project{
			{
				Path:         "/tmp/test-project",
				Directory:    "/tmp/test-project",
				DisplayName:  "test-project",
				AgentType:    agent.AgentCodex,
				SessionCount: 1,
			},
		},
		sessions: []agent.Session{
			{
				ID:           "sess-1",
				FilePath:     "/tmp/test-session.jsonl",
				AgentType:    agent.AgentCodex,
				CreatedAt:    time.Now().Add(-time.Hour),
				LastModified: time.Now(),
				MessageCount: 5,
			},
		},
		entries: entries,
	}

	// Step 1: Create with providers
	model := NewAppModelWithProviders([]agent.AgentProvider{p})
	model.width = 80
	model.height = 24

	// Step 2: Select agent → projects
	newModel, _ := model.Update(AgentSelectedMsg{Provider: p})
	m := newModel.(AppModel)
	if m.state != viewProjects {
		t.Fatalf("step 2: expected viewProjects, got %d", m.state)
	}

	// Step 3: Select project → sessions loaded
	newModel, _ = m.Update(ProjectSelectedMsg{
		Project: types.Project{DisplayName: "test-project", DirPath: "/tmp/test-project", DecodedPath: "/tmp/test-project"},
	})
	m = newModel.(AppModel)
	// Should be loading
	if !m.loading {
		t.Error("step 3: expected loading after project selection")
	}

	// Step 4: Simulate sessions loaded
	newModel, _ = m.Update(conversationsLoadedMsg{
		conversations: []types.Conversation{
			{FilePath: "/tmp/test-session.jsonl", LastModified: time.Now()},
		},
	})
	m = newModel.(AppModel)
	if m.state != viewConversations {
		t.Fatalf("step 4: expected viewConversations, got %d", m.state)
	}

	// Step 5: Select conversation → load via provider
	newModel, _ = m.Update(ConversationSelectedMsg{
		Conversation: types.Conversation{FilePath: "/tmp/test-session.jsonl", LastModified: time.Now()},
	})
	m = newModel.(AppModel)
	if !m.loading {
		t.Error("step 5: expected loading after conversation selection")
	}

	// Step 6: Simulate entries loaded
	newModel, _ = m.Update(conversationLoadedMsg{
		entries:  []types.LogEntry{},
		filePath: "/tmp/test-session.jsonl",
	})
	m = newModel.(AppModel)
	if m.state != viewViewer {
		t.Fatalf("step 6: expected viewViewer, got %d", m.state)
	}
}

func TestConvertConversationEntryToLogEntry(t *testing.T) {
	entry := agent.BasicEntry{
		EntryType:      agent.EntryTypeUser,
		EntryTimestamp: time.Now(),
		EntryRole:      "user",
		Blocks: []agent.ContentBlock{
			agent.BasicBlock{BlockType: agent.ContentBlockText, BlockText: "Hello"},
		},
		EntryAgent:   agent.AgentClaudeCode,
		EntrySession: "sess-123",
	}

	logEntry := convertConversationEntryToLogEntry(entry)

	if logEntry.Type != types.EntryTypeUser {
		t.Errorf("expected EntryTypeUser, got %v", logEntry.Type)
	}
	if logEntry.Message.Role != "user" {
		t.Errorf("expected role 'user', got %s", logEntry.Message.Role)
	}
	if logEntry.SessionID != "sess-123" {
		t.Errorf("expected session ID 'sess-123', got %s", logEntry.SessionID)
	}
	if len(logEntry.Message.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(logEntry.Message.Content))
	}
	if logEntry.Message.Content[0].Type != types.ContentTypeText {
		t.Errorf("expected ContentTypeText, got %v", logEntry.Message.Content[0].Type)
	}
	if logEntry.Message.Content[0].Text != "Hello" {
		t.Errorf("expected 'Hello', got %s", logEntry.Message.Content[0].Text)
	}
}

func TestConversationLoadedWithWatchUsesProviderWatcher(t *testing.T) {
	watcherEntry := agent.BasicEntry{
		EntryType:      agent.EntryTypeAssistant,
		EntryTimestamp: time.Now(),
		EntryRole:      "assistant",
		Blocks: []agent.ContentBlock{
			agent.BasicBlock{BlockType: agent.ContentBlockText, BlockText: "live update"},
		},
		EntryAgent:   agent.AgentOpenCode,
		EntrySession: "opencode-session-1",
	}
	p := &watchableTestProvider{
		testProvider: &testProvider{
			agentType:   agent.AgentOpenCode,
			displayName: "OpenCode",
			badge:       "[O]",
			available:   true,
		},
		watcher: &testSessionWatcher{entries: []agent.ConversationEntry{watcherEntry}},
	}
	model := NewAppModelWithProviders([]agent.AgentProvider{p})
	model.width = 80
	model.height = 24
	model.selectedProvider = p
	model.selectedConversation = types.Conversation{FilePath: "opencode-session-1"}

	newModel, cmd := model.Update(conversationLoadedWithWatchMsg{
		entries:  []types.LogEntry{},
		filePath: "opencode-session-1",
	})
	updated := newModel.(AppModel)

	if updated.state != viewViewer {
		t.Fatalf("state = %d, want viewViewer", updated.state)
	}
	if p.watchedSession.ID != "opencode-session-1" {
		t.Fatalf("watched session ID = %q, want opencode-session-1", p.watchedSession.ID)
	}
	if !updated.viewerModel.watchMode {
		t.Fatal("viewer watchMode should be enabled")
	}
	if updated.viewerModel.providerWatcher == nil {
		t.Fatal("viewer providerWatcher should be set")
	}
	if updated.viewerModel.watcher != nil {
		t.Fatal("viewer file watcher should not be set for provider watch")
	}
	if cmd == nil {
		t.Fatal("expected viewer init command")
	}
}

func TestConvertConversationEntry_ToolUse(t *testing.T) {
	entry := agent.BasicEntry{
		EntryType:      agent.EntryTypeAssistant,
		EntryTimestamp: time.Now(),
		EntryRole:      "assistant",
		Blocks: []agent.ContentBlock{
			agent.BasicBlock{
				BlockType:  agent.ContentBlockToolUse,
				BlockTool:  "bash",
				BlockInput: map[string]any{"command": "ls"},
			},
		},
		EntryTokens: agent.TokenStats{InputTokens: 100, OutputTokens: 50, CachedTokens: 10},
	}

	logEntry := convertConversationEntryToLogEntry(entry)

	if logEntry.Type != types.EntryTypeAssistant {
		t.Errorf("expected EntryTypeAssistant, got %v", logEntry.Type)
	}
	if len(logEntry.Message.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(logEntry.Message.Content))
	}
	if logEntry.Message.Content[0].Type != types.ContentTypeToolUse {
		t.Errorf("expected ContentTypeToolUse, got %v", logEntry.Message.Content[0].Type)
	}
	if logEntry.Message.Content[0].ToolName != "bash" {
		t.Errorf("expected tool name 'bash', got %s", logEntry.Message.Content[0].ToolName)
	}
	if logEntry.Usage.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", logEntry.Usage.InputTokens)
	}
	if logEntry.Usage.CacheCreationInputTokens != 10 {
		t.Errorf("expected 10 cached tokens, got %d", logEntry.Usage.CacheCreationInputTokens)
	}
}
