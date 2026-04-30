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

		cwd, err := readSessionCWD(path)
		if err != nil || cwd == "" {
			return nil // skip files without valid session_meta
		}

		byCWD[cwd] = append(byCWD[cwd], path)
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

// readSessionCWD reads the first line of a rollout file to extract the cwd
// from the session_meta line.
func readSessionCWD(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open %s: %w", filePath, err)
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
			return "", nil // not valid JSON, skip
		}
		if cl.Type == "session_meta" {
			return cl.CWD, nil
		}
		// If first non-empty line isn't session_meta, stop looking.
		return "", nil
	}
	return "", nil
}

// buildSession creates a Session from a rollout file path by parsing metadata.
func buildSession(filePath, cwd string) (agent.Session, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return agent.Session{}, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	result, err := ParseCodexJSONL(f)
	if err != nil {
		return agent.Session{}, fmt.Errorf("failed to parse session: %w", err)
	}

	// Use file modification time for timestamps.
	fi, err := os.Stat(filePath)
	if err != nil {
		return agent.Session{}, fmt.Errorf("failed to stat session file: %w", err)
	}

	modTime := fi.ModTime()
	// Use the first entry's timestamp as creation time if available.
	createdAt := modTime
	if len(result.Entries) > 0 {
		if ts := result.Entries[0].Timestamp(); !ts.IsZero() {
			createdAt = ts
		}
	}

	// Count user messages and turns.
	msgCount := 0
	turnCount := 0
	firstUserMsg := ""
	for _, e := range result.Entries {
		if e.Type() == agent.EntryTypeUser {
			msgCount++
			if firstUserMsg == "" {
				for _, b := range e.ContentBlocks() {
					if b.Text() != "" {
						firstUserMsg = truncateString(b.Text(), 80)
						break
					}
				}
			}
		}
		if e.Type() == agent.EntryTypeAssistant {
			msgCount++
			turnCount++
		}
	}

	sessionID := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")
	sessionID = strings.TrimPrefix(sessionID, "rollout-")

	return agent.Session{
		ID:               sessionID,
		ProjectPath:      cwd,
		FilePath:         filePath,
		AgentType:        agent.AgentCodex,
		CreatedAt:        createdAt,
		LastModified:     modTime,
		MessageCount:     msgCount,
		FirstUserMessage: firstUserMsg,
		Model:            result.Model,
		Tokens:           result.Tokens,
		TurnCount:        turnCount,
	}, nil
}

// truncateString truncates a string to maxLen characters with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// getDefaultBaseDir returns the user's home directory.
func getDefaultBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}
	return home, nil
}
