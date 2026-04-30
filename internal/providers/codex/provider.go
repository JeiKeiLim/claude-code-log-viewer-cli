// Package codex implements the Codex agent provider for OpenAI Codex CLI logs.
package codex

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

// Provider implements agent.AgentProvider for OpenAI Codex.
type Provider struct {
	basePath string
}

// Option configures a Provider.
type Option func(*Provider)

// WithBasePath sets the base directory for Codex session discovery.
// If not set, defaults to the user's home directory.
func WithBasePath(path string) Option {
	return func(p *Provider) { p.basePath = path }
}

// NewProvider creates a new Codex provider with the given options.
func NewProvider(opts ...Option) *Provider {
	p := &Provider{}
	for _, o := range opts {
		o(p)
	}
	return p
}

// compile-time interface check.
var _ agent.AgentProvider = (*Provider)(nil)

func (p *Provider) Type() agent.AgentType { return agent.AgentCodex }
func (p *Provider) DisplayName() string   { return "Codex" }
func (p *Provider) Badge() string         { return "[X]" }

// IsAvailable returns true if the Codex sessions directory exists.
func (p *Provider) IsAvailable() bool {
	baseDir := p.basePath
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		baseDir = home
	}
	sessionsDir := filepath.Join(baseDir, ".codex", "sessions")
	info, err := os.Stat(sessionsDir)
	return err == nil && info.IsDir()
}

// DiscoverProjects walks the Codex sessions directory and groups sessions by
// their cwd, returning one Project per unique cwd.
func (p *Provider) DiscoverProjects() ([]agent.Project, error) {
	baseDir := p.basePath
	if baseDir == "" {
		var err error
		baseDir, err = getDefaultBaseDir()
		if err != nil {
			return nil, fmt.Errorf("codex discover projects: %w", err)
		}
	}

	byCWD, err := discoverCodexSessions(baseDir)
	if err != nil {
		return nil, fmt.Errorf("codex discover projects: %w", err)
	}

	projects := make([]agent.Project, 0, len(byCWD))
	for cwd, files := range byCWD {
		displayName := cwd
		if idx := strings.LastIndex(cwd, "/"); idx >= 0 && idx < len(cwd)-1 {
			displayName = cwd[idx+1:]
		}
		projects = append(projects, agent.Project{
			Path:         cwd,
			Directory:    cwd,
			DisplayName:  displayName,
			AgentType:    agent.AgentCodex,
			SessionCount: len(files),
		})
	}

	// Sort projects by display name for deterministic ordering.
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].DisplayName < projects[j].DisplayName
	})

	return projects, nil
}

// DiscoverSessions returns all sessions for a given project (identified by cwd).
func (p *Provider) DiscoverSessions(project agent.Project) ([]agent.Session, error) {
	baseDir := p.basePath
	if baseDir == "" {
		var err error
		baseDir, err = getDefaultBaseDir()
		if err != nil {
			return nil, fmt.Errorf("codex discover sessions: %w", err)
		}
	}

	byCWD, err := discoverCodexSessions(baseDir)
	if err != nil {
		return nil, fmt.Errorf("codex discover sessions: %w", err)
	}

	cwd := project.Path
	if project.Directory != "" {
		cwd = project.Directory
	}

	files, ok := byCWD[cwd]
	if !ok {
		return nil, nil
	}

	sessions := make([]agent.Session, 0, len(files))
	for _, fp := range files {
		sess, err := buildSession(fp, cwd)
		if err != nil {
			// Skip sessions that fail to parse rather than failing entirely.
			continue
		}
		sessions = append(sessions, sess)
	}

	return sessions, nil
}

// ParseSession reads and parses a complete session file into conversation entries.
func (p *Provider) ParseSession(session agent.Session) ([]agent.ConversationEntry, error) {
	f, err := os.Open(session.FilePath)
	if err != nil {
		return nil, fmt.Errorf("codex parse session: %w", err)
	}
	defer f.Close()

	result, err := ParseCodexJSONL(f)
	if err != nil {
		return nil, fmt.Errorf("codex parse session: %w", err)
	}

	return result.Entries, nil
}

// ParseSessionStream parses a session from an io.Reader (pipeline mode).
func (p *Provider) ParseSessionStream(r io.Reader) ([]agent.ConversationEntry, error) {
	result, err := ParseCodexJSONL(r)
	if err != nil {
		return nil, fmt.Errorf("codex parse stream: %w", err)
	}
	return result.Entries, nil
}

// ParseBytes parses raw JSONL bytes into entries. Returns entries and error count.
func (p *Provider) ParseBytes(data []byte) ([]agent.ConversationEntry, int) {
	result, err := ParseCodexJSONL(newBytesReader(data))
	if err != nil {
		return nil, 1
	}
	return result.Entries, result.ParseErrors
}

// newBytesReader creates an io.Reader from a byte slice.
func newBytesReader(data []byte) *strings.Reader {
	return strings.NewReader(string(data))
}
