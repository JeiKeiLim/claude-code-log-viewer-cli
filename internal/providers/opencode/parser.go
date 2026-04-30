package opencode

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

// messageData represents the JSON data field of a message row.
type messageData struct {
	Role       string `json:"role"`
	ModelID    string `json:"modelID"`
	ProviderID string `json:"providerID"`
	Timestamp  string `json:"timestamp"`
	Content    string `json:"content"`
}

// partData represents the JSON data field of a part row.
// OpenCode uses both flat and nested formats depending on version:
//   - Flat:   {"type":"tool","toolName":"bash","output":"..."}
//   - Nested: {"type":"tool","tool":"bash","state":{"input":"...","output":"..."}}
type partData struct {
	Type       string          `json:"type"`
	Text       string          `json:"text"`
	ToolName   string          `json:"toolName"`
	Tool       string          `json:"tool"`
	ToolCallID string          `json:"toolCallID"`
	Output     string          `json:"output"`
	StateRaw   json.RawMessage `json:"state"`

	// Parsed from StateRaw after unmarshaling.
	stateString string
	stateObj    *stateData
}

type stateData struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// UnmarshalJSON handles the dual-format state field (string or object).
func (p *partData) UnmarshalJSON(data []byte) error {
	type alias partData
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	p.Type = a.Type
	p.Text = a.Text
	p.ToolName = a.ToolName
	p.Tool = a.Tool
	p.ToolCallID = a.ToolCallID
	p.Output = a.Output
	p.StateRaw = a.StateRaw

	if len(p.StateRaw) > 0 {
		// Try parsing as object first.
		var obj stateData
		if err := json.Unmarshal(p.StateRaw, &obj); err == nil && (obj.Input != "" || obj.Output != "") {
			p.stateObj = &obj
		} else {
			// Fall back to string.
			_ = json.Unmarshal(p.StateRaw, &p.stateString)
		}
	}

	return nil
}

// toolName returns the tool name from whichever field is populated.
func (p partData) toolName() string {
	if p.Tool != "" {
		return p.Tool
	}
	return p.ToolName
}

// toolOutput returns the tool output from whichever field is populated.
func (p partData) toolOutput() string {
	if p.stateObj != nil && p.stateObj.Output != "" {
		return p.stateObj.Output
	}
	return p.Output
}

// parseSessionFromDB queries all messages and parts for a session and converts
// them into ConversationEntry values.
func parseSessionFromDB(db *sql.DB, sessionID string) ([]agent.ConversationEntry, error) {
	// Collect all messages first, then close rows before querying parts.
	// This avoids holding a connection open while trying to query parts
	// on the same pool (deadlock risk with MaxOpenConns=1 or shared cache).
	type msgRow struct {
		id          string
		dataJSON    string
		timeCreated int64
	}

	rows, err := db.Query(`
		SELECT id, data, time_created
		FROM message
		WHERE session_id = ?
		ORDER BY time_created ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages for session %s: %w", sessionID, err)
	}

	var msgRows []msgRow
	for rows.Next() {
		var mr msgRow
		if err := rows.Scan(&mr.id, &mr.dataJSON, &mr.timeCreated); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		msgRows = append(msgRows, mr)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}
	rows.Close()

	var entries []agent.ConversationEntry

	for _, mr := range msgRows {
		var mData messageData
		if err := json.Unmarshal([]byte(mr.dataJSON), &mData); err != nil {
			// Skip malformed message data, continue processing others.
			continue
		}

		// Determine entry type from role.
		var eType agent.EntryType
		switch mData.Role {
		case "user":
			eType = agent.EntryTypeUser
		case "assistant":
			eType = agent.EntryTypeAssistant
		default:
			continue
		}

		// Parse the timestamp from the data, fall back to time_created.
		ts := time.Unix(mr.timeCreated, 0)
		if mData.Timestamp != "" {
			if parsed, err := time.Parse(time.RFC3339, mData.Timestamp); err == nil {
				ts = parsed
			}
		}

		// Query parts for this message.
		blocks, err := queryPartsForMessage(db, mr.id)
		if err != nil {
			return nil, fmt.Errorf("failed to query parts for message %s: %w", mr.id, err)
		}

		// If no parts were found but the message has content, create a text block.
		if len(blocks) == 0 && mData.Content != "" {
			blocks = append(blocks, agent.BasicBlock{
				BlockType: agent.ContentBlockText,
				BlockText: mData.Content,
			})
		}

		entry := agent.BasicEntry{
			EntryType:      eType,
			EntryTimestamp: ts,
			EntryRole:      mData.Role,
			Blocks:         blocks,
			EntryTokens:    agent.TokenStats{},
			EntryAgent:     agent.AgentOpenCode,
			EntrySession:   sessionID,
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// queryPartsForMessage queries all parts for a given message ID and converts
// them into ContentBlock values.
func queryPartsForMessage(db *sql.DB, messageID string) ([]agent.ContentBlock, error) {
	rows, err := db.Query(`
		SELECT data FROM part WHERE message_id = ? ORDER BY id
	`, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query parts: %w", err)
	}
	defer rows.Close()

	var blocks []agent.ContentBlock

	for rows.Next() {
		var partJSON string
		if err := rows.Scan(&partJSON); err != nil {
			return nil, fmt.Errorf("failed to scan part row: %w", err)
		}

		var pData partData
		if err := json.Unmarshal([]byte(partJSON), &pData); err != nil {
			continue
		}

		switch pData.Type {
		case "text":
			if pData.Text != "" {
				blocks = append(blocks, agent.BasicBlock{
					BlockType: agent.ContentBlockText,
					BlockText: pData.Text,
				})
			}

		case "tool":
			name := pData.toolName()
			input := map[string]any{
				"toolName": name,
			}
			if pData.stateObj != nil && pData.stateObj.Input != "" {
				input["input"] = pData.stateObj.Input
			}
			blocks = append(blocks, agent.BasicBlock{
				BlockType:   agent.ContentBlockToolUse,
				BlockTool:   name,
				BlockInput:  input,
				BlockOutput: pData.toolOutput(),
			})

		case "reasoning":
			if pData.Text != "" {
				blocks = append(blocks, agent.BasicBlock{
					BlockType: agent.ContentBlockReasoning,
					BlockText: pData.Text,
				})
			}

		case "step-start":
			// step-start parts are metadata, skip them.
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating part rows: %w", err)
	}

	return blocks, nil
}
