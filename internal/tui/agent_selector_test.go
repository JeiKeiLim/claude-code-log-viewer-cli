package tui

import (
	"errors"
	"io"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

// mockProvider implements agent.AgentProvider for testing.
type mockProvider struct {
	agentType    agent.AgentType
	displayName  string
	badge        string
	available    bool
	projects     []agent.Project
	projectsErr  error
	sessions     []agent.Session
	sessionsErr  error
	sessionCalls int
}

func (m *mockProvider) Type() agent.AgentType                      { return m.agentType }
func (m *mockProvider) DisplayName() string                        { return m.displayName }
func (m *mockProvider) Badge() string                              { return m.badge }
func (m *mockProvider) IsAvailable() bool                          { return m.available }
func (m *mockProvider) DiscoverProjects() ([]agent.Project, error) { return m.projects, m.projectsErr }
func (m *mockProvider) DiscoverSessions(_ agent.Project) ([]agent.Session, error) {
	m.sessionCalls++
	return m.sessions, m.sessionsErr
}
func (m *mockProvider) ParseSession(_ agent.Session) ([]agent.ConversationEntry, error) {
	return nil, nil
}
func (m *mockProvider) ParseSessionStream(_ io.Reader) ([]agent.ConversationEntry, error) {
	return nil, nil
}

func availableProvider(name string, at agent.AgentType, badge string, projCount int, sessCount int) *mockProvider {
	projects := make([]agent.Project, projCount)
	for i := range projects {
		projects[i] = agent.Project{SessionCount: sessCount}
	}
	sessions := make([]agent.Session, sessCount)
	return &mockProvider{
		agentType:   at,
		displayName: name,
		badge:       badge,
		available:   true,
		projects:    projects,
		sessions:    sessions,
	}
}

func TestNewAgentSelectorModel_DoesNotDiscoverSessions(t *testing.T) {
	p := availableProvider("Codex", agent.AgentCodex, "[X]", 2, 3)

	m := NewAgentSelectorModel([]agent.AgentProvider{p})

	if len(m.VisibleProviders()) != 1 {
		t.Fatalf("expected 1 visible provider, got %d", len(m.VisibleProviders()))
	}
	if p.sessionCalls != 0 {
		t.Fatalf("expected selector to avoid DiscoverSessions, got %d calls", p.sessionCalls)
	}
}

func unavailableProvider(name string, at agent.AgentType, badge string) *mockProvider {
	return &mockProvider{
		agentType:   at,
		displayName: name,
		badge:       badge,
		available:   false,
	}
}

func TestNewAgentSelectorModel_MultipleProviders(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 3, 5)
	p2 := availableProvider("Codex", agent.AgentCodex, "[X]", 2, 1)

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2})

	if len(m.VisibleProviders()) != 2 {
		t.Errorf("expected 2 visible providers, got %d", len(m.VisibleProviders()))
	}
	if m.Cursor() != 0 {
		t.Errorf("expected cursor at 0, got %d", m.Cursor())
	}
	if sp := m.SingleProvider(); sp != nil {
		t.Errorf("expected nil SingleProvider with 2+ providers, got %v", sp)
	}
}

func TestNewAgentSelectorModel_HidesUnavailable(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 2, 1)
	p2 := unavailableProvider("Codex", agent.AgentCodex, "[X]")
	p3 := &mockProvider{
		agentType:   agent.AgentOpenCode,
		displayName: "OpenCode",
		badge:       "[O]",
		available:   true,
		projects:    []agent.Project{}, // empty projects
	}

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2, p3})

	if len(m.VisibleProviders()) != 1 {
		t.Errorf("expected 1 visible provider, got %d", len(m.VisibleProviders()))
	}
	if m.VisibleProviders()[0].provider.DisplayName() != "Claude Code" {
		t.Errorf("expected Claude Code visible, got %s", m.VisibleProviders()[0].provider.DisplayName())
	}
}

func TestNewAgentSelectorModel_HidesZeroSessions(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 2, 1)
	p2 := availableProvider("Codex", agent.AgentCodex, "[X]", 1, 0) // has projects but zero sessions

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2})

	if len(m.VisibleProviders()) != 1 {
		t.Errorf("expected 1 visible provider (zero-session provider hidden), got %d", len(m.VisibleProviders()))
	}
	if m.VisibleProviders()[0].provider.DisplayName() != "Claude Code" {
		t.Errorf("expected Claude Code visible, got %s", m.VisibleProviders()[0].provider.DisplayName())
	}
}

func TestNewAgentSelectorModel_SingleProviderSkipsWhenOthersHaveNoSessions(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 3)
	p2 := availableProvider("Codex", agent.AgentCodex, "[X]", 2, 0) // projects but no sessions

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2})

	// Only Claude Code has sessions → single provider shortcut should fire
	if sp := m.SingleProvider(); sp == nil {
		t.Error("expected single provider shortcut with one session-bearing provider")
	}
	cmd := m.Init()
	msg := cmd()
	selected, ok := msg.(AgentSelectedMsg)
	if !ok {
		t.Fatalf("expected AgentSelectedMsg, got %T", msg)
	}
	if selected.Provider.DisplayName() != "Claude Code" {
		t.Errorf("expected Claude Code, got %s", selected.Provider.DisplayName())
	}
}

func TestNewAgentSelectorModel_HidesDiscoverProjectsError(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 1)
	p2 := &mockProvider{
		agentType:   agent.AgentCodex,
		displayName: "Codex",
		badge:       "[X]",
		available:   true,
		projectsErr: errors.New("fs error"),
	}

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2})

	if len(m.VisibleProviders()) != 1 {
		t.Errorf("expected 1 visible provider when DiscoverProjects errors, got %d", len(m.VisibleProviders()))
	}
}

func TestCursorMovement_ArrowKeys(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 1)
	p2 := availableProvider("Codex", agent.AgentCodex, "[X]", 1, 1)
	p3 := availableProvider("OpenCode", agent.AgentOpenCode, "[O]", 1, 1)

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2, p3})

	// Down moves cursor from 0 to 1
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(AgentSelectorModel)
	if m.Cursor() != 1 {
		t.Errorf("expected cursor at 1 after down, got %d", m.Cursor())
	}
	_ = cmd

	// Down again moves to 2
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(AgentSelectorModel)
	if m.Cursor() != 2 {
		t.Errorf("expected cursor at 2 after second down, got %d", m.Cursor())
	}

	// Down at end stays at 2
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(AgentSelectorModel)
	if m.Cursor() != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", m.Cursor())
	}

	// Up moves to 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(AgentSelectorModel)
	if m.Cursor() != 1 {
		t.Errorf("expected cursor at 1 after up, got %d", m.Cursor())
	}

	// Up at 0 stays at 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(AgentSelectorModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(AgentSelectorModel)
	if m.Cursor() != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", m.Cursor())
	}
}

func TestCursorMovement_JK(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 1)
	p2 := availableProvider("Codex", agent.AgentCodex, "[X]", 1, 1)

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2})

	// 'j' moves down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(AgentSelectorModel)
	if m.Cursor() != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", m.Cursor())
	}

	// 'k' moves up
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(AgentSelectorModel)
	if m.Cursor() != 0 {
		t.Errorf("expected cursor at 0 after k, got %d", m.Cursor())
	}
}

func TestSelection_ReturnsCorrectProvider(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 1)
	p2 := availableProvider("Codex", agent.AgentCodex, "[X]", 1, 1)

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2})

	// Move to second item
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(AgentSelectorModel)

	// Select second item
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Execute the command and verify it returns AgentSelectedMsg with Codex
	msg := cmd()
	selected, ok := msg.(AgentSelectedMsg)
	if !ok {
		t.Fatalf("expected AgentSelectedMsg, got %T", msg)
	}
	if selected.Provider.DisplayName() != "Codex" {
		t.Errorf("expected Codex selected, got %s", selected.Provider.DisplayName())
	}
}

func TestSelection_FirstProvider(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 1)
	p2 := availableProvider("Codex", agent.AgentCodex, "[X]", 1, 1)

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2})

	// Select first item (cursor defaults to 0)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	msg := cmd()
	selected, ok := msg.(AgentSelectedMsg)
	if !ok {
		t.Fatalf("expected AgentSelectedMsg, got %T", msg)
	}
	if selected.Provider.DisplayName() != "Claude Code" {
		t.Errorf("expected Claude Code selected, got %s", selected.Provider.DisplayName())
	}
}

func TestQuitKey(t *testing.T) {
	p := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 1)
	m := NewAgentSelectorModel([]agent.AgentProvider{p})

	// Test 'q' key
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected non-nil command for q key")
	}
	// The command should be tea.Quit
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestQuitKey_CtrlC(t *testing.T) {
	p := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 1)
	m := NewAgentSelectorModel([]agent.AgentProvider{p})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected non-nil command for ctrl+c")
	}
}

func TestSingleProviderShortcut(t *testing.T) {
	p := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 2, 3)

	m := NewAgentSelectorModel([]agent.AgentProvider{p})

	// SingleProvider should return the provider
	sp := m.SingleProvider()
	if sp == nil {
		t.Fatal("expected SingleProvider to return the provider")
	}
	if sp.DisplayName() != "Claude Code" {
		t.Errorf("expected Claude Code, got %s", sp.DisplayName())
	}

	// Init should immediately return AgentSelectedMsg
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a command for single provider")
	}
	msg := cmd()
	selected, ok := msg.(AgentSelectedMsg)
	if !ok {
		t.Fatalf("expected AgentSelectedMsg from Init, got %T", msg)
	}
	if selected.Provider.DisplayName() != "Claude Code" {
		t.Errorf("expected Claude Code, got %s", selected.Provider.DisplayName())
	}
}

func TestSingleProviderShortcut_WithUnavailable(t *testing.T) {
	p1 := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 1)
	p2 := unavailableProvider("Codex", agent.AgentCodex, "[X]")

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2})

	// Only Claude Code is visible, so single-provider shortcut should trigger
	if sp := m.SingleProvider(); sp == nil {
		t.Error("expected single provider shortcut with one visible provider")
	}

	cmd := m.Init()
	msg := cmd()
	selected, ok := msg.(AgentSelectedMsg)
	if !ok {
		t.Fatalf("expected AgentSelectedMsg, got %T", msg)
	}
	if selected.Provider.DisplayName() != "Claude Code" {
		t.Errorf("expected Claude Code, got %s", selected.Provider.DisplayName())
	}
}

func TestNoVisibleProviders(t *testing.T) {
	p1 := unavailableProvider("Claude Code", agent.AgentClaudeCode, "[C]")
	p2 := unavailableProvider("Codex", agent.AgentCodex, "[X]")

	m := NewAgentSelectorModel([]agent.AgentProvider{p1, p2})

	if len(m.VisibleProviders()) != 0 {
		t.Errorf("expected 0 visible providers, got %d", len(m.VisibleProviders()))
	}
	if sp := m.SingleProvider(); sp != nil {
		t.Error("expected nil SingleProvider with 0 visible")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	p := availableProvider("Claude Code", agent.AgentClaudeCode, "[C]", 1, 1)
	m := NewAgentSelectorModel([]agent.AgentProvider{p})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(AgentSelectorModel)

	if m2.width != 120 {
		t.Errorf("expected width 120, got %d", m2.width)
	}
	if m2.height != 40 {
		t.Errorf("expected height 40, got %d", m2.height)
	}
}
