// Package opencode implements the OpenCode agent provider using SQLite.
package opencode

import (
	"fmt"
	"io"
	"os"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"

	_ "modernc.org/sqlite"
)

// Provider implements agent.AgentProvider for OpenCode.
type Provider struct {
	dbPath string
}

// Option configures a Provider.
type Option func(*Provider)

// WithDBPath sets the SQLite database path for OpenCode session discovery.
func WithDBPath(path string) Option {
	return func(p *Provider) { p.dbPath = path }
}

// NewProvider creates a new OpenCode provider with the given options.
func NewProvider(opts ...Option) *Provider {
	p := &Provider{
		dbPath: defaultDBPath,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *Provider) Type() agent.AgentType { return agent.AgentOpenCode }
func (p *Provider) DisplayName() string   { return "OpenCode" }
func (p *Provider) Badge() string         { return "[O]" }
func (p *Provider) IsAvailable() bool     { return dbExists(p.dbPath) }
func (p *Provider) ParseSessionStream(_ io.Reader) ([]agent.ConversationEntry, error) {
	return nil, fmt.Errorf("opencode: ParseSessionStream not supported; sessions are stored in SQLite")
}

// DiscoverProjects returns all projects found in the OpenCode database.
func (p *Provider) DiscoverProjects() ([]agent.Project, error) {
	db, err := openDB(p.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return queryProjects(db)
}

// DiscoverSessions returns all sessions within the given project.
func (p *Provider) DiscoverSessions(project agent.Project) ([]agent.Session, error) {
	db, err := openDB(p.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	dir := project.Path
	if dir == "" {
		dir = project.Directory
	}

	sessions, err := querySessions(db, dir)
	if err != nil {
		return nil, err
	}

	if err := enrichSessions(db, sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

// ParseSession parses a complete session from the OpenCode database.
func (p *Provider) ParseSession(session agent.Session) ([]agent.ConversationEntry, error) {
	db, err := openDB(p.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return parseSessionFromDB(db, session.FilePath)
}

// dbExists checks whether the database file is accessible.
func dbExists(dbPath string) bool {
	expanded := expandHome(dbPath)
	_, err := os.Stat(expanded)
	return err == nil
}

// compile-time check that Provider satisfies AgentProvider.
var _ agent.AgentProvider = (*Provider)(nil)
