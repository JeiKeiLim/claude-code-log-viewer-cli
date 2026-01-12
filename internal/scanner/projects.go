// Package scanner handles scanning Claude Code project directories.
package scanner

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// DefaultProjectsPath returns the default Claude projects directory path.
func DefaultProjectsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

// ScanProjects scans the Claude projects directory and returns a list of projects.
func ScanProjects(projectsPath string) ([]types.Project, error) {
	if projectsPath == "" {
		projectsPath = DefaultProjectsPath()
	}

	// Check if directory exists
	info, err := os.Stat(projectsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ProjectsNotFoundError{Path: projectsPath}
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, &ProjectsNotFoundError{Path: projectsPath}
	}

	// Read directory entries
	entries, err := os.ReadDir(projectsPath)
	if err != nil {
		return nil, err
	}

	projects := make([]types.Project, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		project := types.Project{
			EncodedName: name,
			DecodedPath: DecodeProjectPath(name),
			DirPath:     filepath.Join(projectsPath, name),
		}
		projects = append(projects, project)
	}

	// Assign display names with disambiguation
	assignDisplayNames(projects)

	// Sort by display name
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].DisplayName < projects[j].DisplayName
	})

	return projects, nil
}

// DecodeProjectPath converts an encoded project name to the original path.
// e.g., "-Users-limjk-GitHub-foo" -> "/Users/limjk/GitHub/foo"
func DecodeProjectPath(encodedName string) string {
	if encodedName == "" {
		return ""
	}

	// Replace dashes with slashes
	// The encoded name starts with a dash which becomes the root /
	decoded := strings.ReplaceAll(encodedName, "-", "/")

	// Handle double dashes (escaped dashes in original path)
	decoded = strings.ReplaceAll(decoded, "//", "-")

	return decoded
}

// assignDisplayNames assigns display names to projects with collision disambiguation.
func assignDisplayNames(projects []types.Project) {
	// Group projects by their last path component
	lastComponents := make(map[string][]*types.Project)
	for i := range projects {
		last := filepath.Base(projects[i].DecodedPath)
		lastComponents[last] = append(lastComponents[last], &projects[i])
	}

	// Assign display names
	for _, ps := range lastComponents {
		if len(ps) == 1 {
			// No collision, use just the last component
			ps[0].DisplayName = filepath.Base(ps[0].DecodedPath)
		} else {
			// Collision detected, use parent/name format
			for _, p := range ps {
				parent := filepath.Base(filepath.Dir(p.DecodedPath))
				name := filepath.Base(p.DecodedPath)
				p.DisplayName = filepath.Join(parent, name)
			}
		}
	}
}

// ScanConversations scans a project directory for conversation files.
func ScanConversations(projectPath string) ([]types.Conversation, error) {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return nil, err
	}

	conversations := make([]types.Conversation, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		filePath := filepath.Join(projectPath, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		conv := types.Conversation{
			FilePath:     filePath,
			LastModified: info.ModTime(),
		}

		// Extract first user message preview and count messages
		conv.FirstUserMessage, conv.MessageCount = extractConversationPreview(filePath)

		conversations = append(conversations, conv)
	}

	// Sort by last modified, most recent first
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].LastModified.After(conversations[j].LastModified)
	})

	return conversations, nil
}

// extractConversationPreview reads the first user message and counts entries.
func extractConversationPreview(filePath string) (string, int) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var firstUserMessage string
	messageCount := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Quick parse to get type
		var raw struct {
			Type    string `json:"type"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		if raw.Type == "user" || raw.Type == "assistant" {
			messageCount++
		}

		// Get first user message
		if raw.Type == "user" && firstUserMessage == "" {
			firstUserMessage = raw.Message.Content
			if len(firstUserMessage) > 80 {
				firstUserMessage = firstUserMessage[:80] + "..."
			}
		}
	}

	return firstUserMessage, messageCount
}

// ProjectsNotFoundError is returned when the projects directory doesn't exist.
type ProjectsNotFoundError struct {
	Path string
}

func (e *ProjectsNotFoundError) Error() string {
	return "Claude Code projects directory not found: " + e.Path
}
