package claudecode

import (
	"fmt"
	"io"
	"os"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
)

// ClaudeCodeProvider implements agent.AgentProvider for Claude Code logs.
type ClaudeCodeProvider struct{}

// Compile-time interface compliance check.
var _ agent.AgentProvider = (*ClaudeCodeProvider)(nil)

// Type returns the Claude Code agent type identifier.
func (p *ClaudeCodeProvider) Type() agent.AgentType {
	return agent.AgentClaudeCode
}

// DisplayName returns the human-readable name for display in the TUI.
func (p *ClaudeCodeProvider) DisplayName() string {
	return "Claude Code"
}

// Badge returns the text badge for the TUI.
func (p *ClaudeCodeProvider) Badge() string {
	return "[C]"
}

// IsAvailable returns true if the Claude Code projects directory exists.
func (p *ClaudeCodeProvider) IsAvailable() bool {
	projectsPath := scanner.DefaultProjectsPath()
	if projectsPath == "" {
		return false
	}
	info, err := os.Stat(projectsPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// DiscoverProjects scans the Claude Code projects directory and returns
// all discovered projects as agent.Project values.
func (p *ClaudeCodeProvider) DiscoverProjects() ([]agent.Project, error) {
	projectsPath := scanner.DefaultProjectsPath()

	projects, err := scanner.ScanProjects(projectsPath)
	if err != nil {
		return nil, fmt.Errorf("claude-code: failed to discover projects: %w", err)
	}

	result := make([]agent.Project, 0, len(projects))
	for _, proj := range projects {
		result = append(result, convertProject(proj))
	}

	return result, nil
}

// DiscoverSessions returns all sessions within the given project directory.
func (p *ClaudeCodeProvider) DiscoverSessions(project agent.Project) ([]agent.Session, error) {
	// Use the Directory field (DirPath in types.Project) for scanning
	dirPath := project.Directory
	if dirPath == "" {
		dirPath = project.Path
	}

	conversations, err := scanner.ScanConversationsLazy(dirPath)
	if err != nil {
		return nil, fmt.Errorf("claude-code: failed to discover sessions in %s: %w", dirPath, err)
	}

	result := make([]agent.Session, 0, len(conversations))
	for _, conv := range conversations {
		result = append(result, convertConversation(conv, project.Path))
	}

	return result, nil
}

// ParseSession reads and parses a complete session file into conversation entries.
func (p *ClaudeCodeProvider) ParseSession(session agent.Session) ([]agent.ConversationEntry, error) {
	if session.FilePath == "" {
		return nil, fmt.Errorf("claude-code: session has no file path")
	}

	result, err := parser.ParseJSONLFile(session.FilePath)
	if err != nil {
		return nil, fmt.Errorf("claude-code: failed to parse session file %s: %w", session.FilePath, err)
	}

	return convertEntries(result.Entries), nil
}

// ParseSessionStream parses a session from an io.Reader (pipeline mode).
func (p *ClaudeCodeProvider) ParseSessionStream(r io.Reader) ([]agent.ConversationEntry, error) {
	result := parser.ParseJSONL(r)
	return convertEntries(result.Entries), nil
}
