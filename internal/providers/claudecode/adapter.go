// Package claudecode provides the Claude Code agent provider adapter.
package claudecode

import (
	"path/filepath"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// convertProject maps a types.Project to an agent.Project.
func convertProject(p types.Project) agent.Project {
	return agent.Project{
		Path:         p.DecodedPath,
		Directory:    p.DirPath,
		DisplayName:  p.DisplayName,
		AgentType:    agent.AgentClaudeCode,
		SessionCount: p.ConversationCount,
	}
}

// convertConversation maps a types.Conversation to an agent.Session.
// The projectPath parameter is used to populate Session.ProjectPath.
func convertConversation(c types.Conversation, projectPath string) agent.Session {
	id := ""
	if c.FilePath != "" {
		id = filepath.Base(c.FilePath)
		if ext := filepath.Ext(id); ext != "" {
			id = id[:len(id)-len(ext)]
		}
	}

	return agent.Session{
		ID:               id,
		ProjectPath:      projectPath,
		FilePath:         c.FilePath,
		AgentType:        agent.AgentClaudeCode,
		CreatedAt:        c.CreationTime,
		LastModified:     c.LastModified,
		MessageCount:     c.MessageCount,
		FirstUserMessage: c.FirstUserMessage,
		Model:            c.Model,
		Tokens:           convertTokenUsage(c.TotalTokens),
		TurnCount:        c.TurnCount,
	}
}

// convertTokenUsage maps types.TokenUsage to agent.TokenStats.
func convertTokenUsage(u types.TokenUsage) agent.TokenStats {
	return agent.TokenStats{
		InputTokens:     u.InputTokens,
		OutputTokens:    u.OutputTokens,
		CachedTokens:    u.CacheCreationInputTokens + u.CacheReadInputTokens,
		ReasoningTokens: 0, // Claude Code logs don't separate reasoning tokens
	}
}

// convertLogEntry maps a types.LogEntry to an agent.ConversationEntry.
func convertLogEntry(e types.LogEntry) agent.ConversationEntry {
	var entryType agent.EntryType
	switch e.Type {
	case types.EntryTypeUser:
		entryType = agent.EntryTypeUser
	case types.EntryTypeAssistant:
		entryType = agent.EntryTypeAssistant
	default:
		entryType = agent.EntryType(e.Type)
	}

	return agent.BasicEntry{
		EntryType:      entryType,
		EntryTimestamp: e.Timestamp,
		EntryRole:      e.Message.Role,
		Blocks:         convertMessageContent(e.Message),
		EntryTokens:    convertTokenUsage(e.Usage),
		EntryAgent:     agent.AgentClaudeCode,
		EntrySession:   e.SessionID,
	}
}

// convertMessageContent converts the message content (string or array) into ContentBlocks.
func convertMessageContent(msg types.Message) []agent.ContentBlock {
	// User messages store plain text in TextContent
	if msg.TextContent != "" && len(msg.Content) == 0 {
		return []agent.ContentBlock{
			agent.BasicBlock{
				BlockType: agent.ContentBlockText,
				BlockText: msg.TextContent,
			},
		}
	}

	// Assistant messages use the Content array
	blocks := make([]agent.ContentBlock, 0, len(msg.Content))
	for _, c := range msg.Content {
		blocks = append(blocks, convertContentBlock(c))
	}

	if len(blocks) == 0 {
		return nil
	}
	return blocks
}

// convertContentBlock maps a single types.MessageContent to an agent.ContentBlock.
func convertContentBlock(c types.MessageContent) agent.ContentBlock {
	switch c.Type {
	case types.ContentTypeText:
		return agent.BasicBlock{
			BlockType: agent.ContentBlockText,
			BlockText: c.Text,
		}
	case types.ContentTypeThinking:
		return agent.BasicBlock{
			BlockType: agent.ContentBlockThinking,
			BlockText: c.Thinking,
		}
	case types.ContentTypeToolUse:
		return agent.BasicBlock{
			BlockType:   agent.ContentBlockToolUse,
			BlockTool:   c.ToolName,
			BlockInput:  c.ToolInput,
			BlockOutput: "", // Tool output is not stored in the LogEntry content block
		}
	default:
		// Fallback: treat unknown types as text
		return agent.BasicBlock{
			BlockType: agent.ContentBlockText,
			BlockText: c.Text,
		}
	}
}

// convertEntries converts a slice of types.LogEntry to []agent.ConversationEntry.
func convertEntries(entries []types.LogEntry) []agent.ConversationEntry {
	result := make([]agent.ConversationEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, convertLogEntry(e))
	}
	return result
}
