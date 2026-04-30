package opencode

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

const (
	defaultDBPath = "~/.local/share/opencode/opencode.db"
)

// openDB opens the SQLite database in read-only mode.
// It expands ~ to the user's home directory.
func openDB(dbPath string) (*sql.DB, error) {
	expanded := expandHome(dbPath)
	if _, err := os.Stat(expanded); err != nil {
		return nil, fmt.Errorf("opencode database not found at %s: %w", expanded, err)
	}

	dsn := expanded + "?mode=ro&_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open opencode database: %w", err)
	}

	// Verify the connection actually works.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping opencode database: %w", err)
	}

	return db, nil
}

// expandHome replaces a leading ~ with the home directory.
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// queryProjects queries the database for all distinct projects (directories)
// that have at least one session.
func queryProjects(db *sql.DB) ([]agent.Project, error) {
	rows, err := db.Query(`
		SELECT s.directory, COUNT(s.id) as session_count
		FROM session s
		WHERE s.directory != ''
		GROUP BY s.directory
		ORDER BY s.directory
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query opencode projects: %w", err)
	}
	defer rows.Close()

	var projects []agent.Project
	for rows.Next() {
		var dir string
		var count int
		if err := rows.Scan(&dir, &count); err != nil {
			return nil, fmt.Errorf("failed to scan project row: %w", err)
		}
		projects = append(projects, agent.Project{
			Path:         dir,
			Directory:    dir,
			DisplayName:  filepath.Base(dir),
			AgentType:    agent.AgentOpenCode,
			SessionCount: count,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project rows: %w", err)
	}

	return projects, nil
}

// querySessions queries all sessions for a given project directory.
func querySessions(db *sql.DB, projectDir string) ([]agent.Session, error) {
	rows, err := db.Query(`
		SELECT s.id, s.title, s.time_created, s.time_updated
		FROM session s
		WHERE s.directory = ?
		ORDER BY s.time_created DESC
	`, projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to query opencode sessions for %s: %w", projectDir, err)
	}
	defer rows.Close()

	var sessions []agent.Session
	for rows.Next() {
		var id, title string
		var created, updated int64
		if err := rows.Scan(&id, &title, &created, &updated); err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}

		sess := agent.Session{
			ID:           id,
			ProjectPath:  projectDir,
			FilePath:     id, // Store session ID since there's no file path for SQLite
			AgentType:    agent.AgentOpenCode,
			CreatedAt:    time.Unix(created, 0),
			LastModified: time.Unix(updated, 0),
		}

		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating session rows: %w", err)
	}

	return sessions, nil
}

// enrichSessions fills in MessageCount, FirstUserMessage, Model, Tokens, and TurnCount
// by querying messages for each session.
func enrichSessions(db *sql.DB, sessions []agent.Session) error {
	for i := range sessions {
		msgCount, firstMsg, model, tokens, turns, err := querySessionStats(db, sessions[i].ID)
		if err != nil {
			return err
		}
		sessions[i].MessageCount = msgCount
		sessions[i].FirstUserMessage = firstMsg
		sessions[i].Model = model
		sessions[i].Tokens = tokens
		sessions[i].TurnCount = turns
	}
	return nil
}

// querySessionStats returns message count, first user message, model, tokens,
// and turn count for a session.
func querySessionStats(db *sql.DB, sessionID string) (int, string, string, agent.TokenStats, int, error) {
	// Get total message count.
	var msgCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM message WHERE session_id = ?`, sessionID).Scan(&msgCount)
	if err != nil {
		return 0, "", "", agent.TokenStats{}, 0, fmt.Errorf("failed to count messages: %w", err)
	}

	// Get first user message.
	var firstMsg string
	err = db.QueryRow(`
		SELECT COALESCE(json_extract(data, '$.content'), '')
		FROM message
		WHERE session_id = ? AND json_extract(data, '$.role') = 'user'
		ORDER BY time_created ASC
		LIMIT 1
	`, sessionID).Scan(&firstMsg)
	if err != nil && err != sql.ErrNoRows {
		return 0, "", "", agent.TokenStats{}, 0, fmt.Errorf("failed to get first user message: %w", err)
	}

	// Truncate first message for display.
	if len(firstMsg) > 200 {
		firstMsg = firstMsg[:200]
	}

	// Get the most common model used.
	var model string
	err = db.QueryRow(`
		SELECT json_extract(data, '$.modelID')
		FROM message
		WHERE session_id = ? AND json_extract(data, '$.modelID') != ''
		GROUP BY json_extract(data, '$.modelID')
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, sessionID).Scan(&model)
	if err != nil && err != sql.ErrNoRows {
		return 0, "", "", agent.TokenStats{}, 0, fmt.Errorf("failed to get model: %w", err)
	}

	// Get turn count (number of user messages).
	var turnCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM message
		WHERE session_id = ? AND json_extract(data, '$.role') = 'user'
	`, sessionID).Scan(&turnCount)
	if err != nil {
		return 0, "", "", agent.TokenStats{}, 0, fmt.Errorf("failed to count turns: %w", err)
	}

	return msgCount, firstMsg, model, agent.TokenStats{}, turnCount, nil
}
