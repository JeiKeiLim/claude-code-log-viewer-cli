// Package scanner handles scanning Claude Code project directories.
package scanner

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
// Claude Code encoding is lossy: both "/" and "_" become "-", and literal "-" also stays "-".
// We use filesystem validation to find the correct path.
// e.g., "-Users-limjk-GitHub-foo" -> "/Users/limjk/GitHub/foo"
// e.g., "-Users-limjk-GitHub-vibe-dash" -> "/Users/limjk/GitHub/vibe-dash" (if that path exists)
func DecodeProjectPath(encodedName string) string {
	if encodedName == "" {
		return ""
	}

	// First, handle the simple case: replace -- with placeholder for underscore sequences
	// (when / and _ are adjacent, they become --)
	const placeholder = "\x00"
	decoded := strings.ReplaceAll(encodedName, "--", placeholder)
	decoded = strings.ReplaceAll(decoded, "-", "/")
	decoded = strings.ReplaceAll(decoded, placeholder, "_")

	// Check if this path exists
	if pathExists(decoded) {
		return decoded
	}

	// Try alternative: -- might represent /- (slash followed by literal hyphen)
	decoded = strings.ReplaceAll(encodedName, "--", placeholder)
	decoded = strings.ReplaceAll(decoded, "-", "/")
	decoded = strings.ReplaceAll(decoded, placeholder, "-")

	if pathExists(decoded) {
		return decoded
	}

	// If simple decode doesn't work, try to find actual path by validating each component
	return decodeWithValidation(encodedName)
}

// pathExists checks if a path exists on the filesystem.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// decodeWithValidation attempts to decode by validating path components exist.
// It tries different interpretations of hyphens (as / or literal - or _).
// Uses recursive backtracking to find a valid path.
func decodeWithValidation(encodedName string) string {
	if encodedName == "" {
		return ""
	}

	// Start with empty path, first char should be - which becomes /
	if encodedName[0] != '-' {
		// Fallback to simple decode
		return strings.ReplaceAll(encodedName, "-", "/")
	}

	// Split by hyphen and try to reconstruct the path
	parts := strings.Split(encodedName[1:], "-") // Skip leading -
	if len(parts) == 0 {
		return "/"
	}

	// Use recursive backtracking to find the correct path
	result := findValidPath("", parts, 0)
	if result != "" {
		return result
	}

	// Fallback: simple decode
	return strings.ReplaceAll(encodedName, "-", "/")
}

// findValidPath recursively tries to find a valid filesystem path
// by interpreting hyphens as either path separators, literal hyphens, or underscores.
func findValidPath(currentPath string, parts []string, startIdx int) string {
	if startIdx >= len(parts) {
		// All parts consumed, check if path exists
		if currentPath != "" && pathExists(currentPath) {
			return currentPath
		}
		return ""
	}

	// Try consuming 1 to N remaining parts as a single path component
	for endIdx := startIdx + 1; endIdx <= len(parts); endIdx++ {
		// Build component name by joining parts with hyphens
		componentParts := parts[startIdx:endIdx]
		component := strings.Join(componentParts, "-")

		// Try as new directory
		var testPath string
		if currentPath == "" {
			testPath = "/" + component
		} else {
			testPath = filepath.Join(currentPath, component)
		}

		if pathExists(testPath) {
			// This component exists, try to complete the rest
			result := findValidPath(testPath, parts, endIdx)
			if result != "" {
				return result
			}
			// If rest failed but we consumed all parts and this exists, return it
			if endIdx == len(parts) {
				return testPath
			}
		}

		// Try with underscore prefix (for _bmad-output style paths)
		if currentPath != "" && len(componentParts) > 0 {
			underscoreComponent := "_" + strings.Join(componentParts, "-")
			testPath = filepath.Join(currentPath, underscoreComponent)
			if pathExists(testPath) {
				result := findValidPath(testPath, parts, endIdx)
				if result != "" {
					return result
				}
				if endIdx == len(parts) {
					return testPath
				}
			}
		}
	}

	return ""
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
// For projects with many conversations, use ScanConversationsLazy followed by
// ExtractConversationMetadataBatch for better performance.
func ScanConversations(projectPath string) ([]types.Conversation, error) {
	conversations, err := ScanConversationsLazy(projectPath)
	if err != nil {
		return nil, err
	}

	// Extract metadata for all conversations
	for i := range conversations {
		extractConversationMetadata(conversations[i].FilePath, &conversations[i])
	}

	return conversations, nil
}

// ScanConversationsLazy scans a project directory for conversation files without
// extracting metadata. This is faster for initial loading of large projects.
// Call ExtractConversationMetadataBatch to load metadata for specific conversations.
func ScanConversationsLazy(projectPath string) ([]types.Conversation, error) {
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

		conversations = append(conversations, conv)
	}

	// Sort by last modified, most recent first
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].LastModified.After(conversations[j].LastModified)
	})

	return conversations, nil
}

// ExtractConversationMetadataBatch extracts metadata for a batch of conversations.
// Modifies the conversations in place.
func ExtractConversationMetadataBatch(conversations []types.Conversation, startIdx, count int) {
	endIdx := startIdx + count
	if endIdx > len(conversations) {
		endIdx = len(conversations)
	}

	for i := startIdx; i < endIdx; i++ {
		extractConversationMetadata(conversations[i].FilePath, &conversations[i])
	}
}

// extractConversationMetadata reads metadata from a conversation file.
// This includes first user message, message counts, token usage, duration, and model.
func extractConversationMetadata(filePath string, conv *types.Conversation) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var firstTimestamp, lastTimestamp time.Time
	var totalTokens types.TokenUsage

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Quick parse to get type and relevant fields
		var raw struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Message   struct {
				Content string `json:"content"`
				Model   string `json:"model"`
				Usage   *struct {
					InputTokens              int `json:"input_tokens"`
					OutputTokens             int `json:"output_tokens"`
					CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
					CacheReadInputTokens     int `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		// Parse timestamp
		if raw.Timestamp != "" {
			t, err := time.Parse(time.RFC3339, raw.Timestamp)
			if err != nil {
				t, _ = time.Parse("2006-01-02T15:04:05.000Z", raw.Timestamp)
			}
			if !t.IsZero() {
				if firstTimestamp.IsZero() {
					firstTimestamp = t
				}
				lastTimestamp = t
			}
		}

		switch raw.Type {
		case "user":
			conv.MessageCount++
			conv.TurnCount++

			// Get first user message (normalize: collapse newlines to spaces for single-line preview)
			if conv.FirstUserMessage == "" {
				preview := strings.ReplaceAll(raw.Message.Content, "\n", " ")
				preview = strings.Join(strings.Fields(preview), " ") // Collapse multiple spaces
				if len(preview) > 80 {
					preview = preview[:80] + "..."
				}
				conv.FirstUserMessage = preview
			}
		case "assistant":
			conv.MessageCount++

			// Extract model (use first one we find)
			if conv.Model == "" && raw.Message.Model != "" {
				conv.Model = raw.Message.Model
			}

			// Sum token usage
			if raw.Message.Usage != nil {
				totalTokens.InputTokens += raw.Message.Usage.InputTokens
				totalTokens.OutputTokens += raw.Message.Usage.OutputTokens
				totalTokens.CacheCreationInputTokens += raw.Message.Usage.CacheCreationInputTokens
				totalTokens.CacheReadInputTokens += raw.Message.Usage.CacheReadInputTokens
			}
		}
	}

	conv.TotalTokens = totalTokens

	// Calculate duration
	if !firstTimestamp.IsZero() && !lastTimestamp.IsZero() {
		conv.Duration = lastTimestamp.Sub(firstTimestamp)
	}
}

// ProjectsNotFoundError is returned when the projects directory doesn't exist.
type ProjectsNotFoundError struct {
	Path string
}

func (e *ProjectsNotFoundError) Error() string {
	return "Claude Code projects directory not found: " + e.Path
}
