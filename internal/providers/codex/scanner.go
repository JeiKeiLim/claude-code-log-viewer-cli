package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

// discoverCodexSessions walks the Codex sessions directory and groups rollout
// files by their cwd (project directory). Returns a map of cwd -> []filePath.
func discoverCodexSessions(baseDir string) (map[string][]string, error) {
	sessionsDir := filepath.Join(baseDir, ".codex", "sessions")

	info, err := os.Stat(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat codex sessions dir: %w", err)
	}
	if !info.IsDir() {
		return nil, nil
	}

	byCWD := make(map[string][]string)

	err = filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasPrefix(d.Name(), "rollout-") || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		meta, err := readSessionMeta(path)
		if err != nil || meta.cwd == "" {
			return nil // skip files without valid session_meta
		}

		byCWD[meta.cwd] = append(byCWD[meta.cwd], path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk codex sessions: %w", err)
	}

	// Sort files within each cwd by name (which includes timestamp) for deterministic order.
	for cwd := range byCWD {
		sort.Strings(byCWD[cwd])
	}

	return byCWD, nil
}

type sessionMeta struct {
	cwd       string
	model     string
	timestamp time.Time
}

// readSessionMeta reads the first session_meta line from a rollout file.
func readSessionMeta(filePath string) (sessionMeta, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return sessionMeta{}, fmt.Errorf("failed to open %s: %w", filePath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var cl codexLine
		if err := json.Unmarshal([]byte(line), &cl); err != nil {
			return sessionMeta{}, nil // not valid JSON, skip
		}
		if cl.Type == "session_meta" {
			meta := sessionMeta{
				cwd:   cl.CWD,
				model: cl.Model,
			}
			if cl.Timestamp != "" {
				if ts, err := time.Parse(time.RFC3339, cl.Timestamp); err == nil {
					meta.timestamp = ts
				}
			}
			// Try nested payload first (real Codex CLI v0.116.0+ format).
			if cl.Payload != nil {
				var mp sessionMetaPayload
				if err := json.Unmarshal(cl.Payload, &mp); err == nil {
					if mp.CWD != "" {
						meta.cwd = mp.CWD
					}
					if mp.Model != "" {
						meta.model = mp.Model
					}
				}
			}
			// Fallback to flat field for backward compatibility.
			return meta, nil
		}
		// If first non-empty line isn't session_meta, stop looking.
		return sessionMeta{}, nil
	}
	return sessionMeta{}, nil
}

// buildSessionSummary creates a Session from cheap file and session_meta data.
// Full message parsing is deferred until the session is opened.
func buildSessionSummary(filePath, cwd string) (agent.Session, error) {
	meta, err := readSessionMeta(filePath)
	if err != nil {
		return agent.Session{}, err
	}

	fi, err := os.Stat(filePath)
	if err != nil {
		return agent.Session{}, fmt.Errorf("failed to stat session file: %w", err)
	}

	sessionID := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")
	sessionID = strings.TrimPrefix(sessionID, "rollout-")

	createdAt := meta.timestamp
	if createdAt.IsZero() {
		createdAt = fi.ModTime()
	}

	return agent.Session{
		ID:           sessionID,
		ProjectPath:  cwd,
		FilePath:     filePath,
		AgentType:    agent.AgentCodex,
		CreatedAt:    createdAt,
		LastModified: fi.ModTime(),
		Model:        meta.model,
	}, nil
}

// getDefaultBaseDir returns the user's home directory.
func getDefaultBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}
	return home, nil
}
