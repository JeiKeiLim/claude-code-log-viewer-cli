package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

// ParseResult holds the output of parsing a Codex JSONL stream.
type ParseResult struct {
	Entries     []agent.ConversationEntry
	SessionID   string
	CWD         string
	Model       string
	Tokens      agent.TokenStats
	ParseErrors int
}

// codexLine represents the top-level structure of a Codex JSONL line.
type codexLine struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"session_id"`
	CWD       string          `json:"cwd"`
	Model     string          `json:"model"`
	Payload   json.RawMessage `json:"payload"`
	Info      json.RawMessage `json:"info"`
	Command   string          `json:"command"`
	CallID    string          `json:"call_id"`
	ExitCode  int             `json:"exit_code"`
	Output    string          `json:"output"`
}

// eventPayload is the structure within payload for event_msg lines.
type eventPayload struct {
	Event json.RawMessage `json:"event"`
}

// eventDetail is the event structure inside an event_msg payload.
type eventDetail struct {
	Type     string          `json:"type"`
	Content  string          `json:"content"`
	Phase    string          `json:"phase"`
	Command  string          `json:"command"`
	ExitCode int             `json:"exit_code"`
	Output   string          `json:"output"`
	Info     json.RawMessage `json:"info"`
}

// tokenInfo holds token usage information.
type tokenInfo struct {
	TotalTokenUsage tokenUsageDetail `json:"total_token_usage"`
}

// tokenUsageDetail holds the individual token counts.
type tokenUsageDetail struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CachedTokens int `json:"cached_tokens"`
}

// pendingCommand tracks a tool-use command awaiting its result.
type pendingCommand struct {
	timestamp time.Time
	command   string
	callID    string
}

// ParseCodexJSONL reads a Codex JSONL stream and returns parsed conversation entries.
// It processes the stream line-by-line using bufio.Scanner to handle large files.
// Malformed lines are skipped and counted; the parser never fails on individual lines.
func ParseCodexJSONL(r io.Reader) (*ParseResult, error) {
	result := &ParseResult{}
	scanner := bufio.NewScanner(r)
	// Allow up to 1MB per line (Codex exec_command_end output can be large).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// FIFO queue for event_msg exec_command_begin/end (no call_id available).
	var eventPending []*pendingCommand
	// Map for top-level exec_command_begin/end with call_id.
	pendingByCallID := make(map[string]*pendingCommand)
	// FIFO queue for top-level exec_command_begin/end without call_id.
	var pendingNoID []*pendingCommand
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		var cl codexLine
		if err := json.Unmarshal([]byte(line), &cl); err != nil {
			result.ParseErrors++
			continue
		}

		ts := parseTimestamp(cl.Timestamp)

		switch cl.Type {
		case "session_meta":
			handleSessionMeta(result, &cl)

		case "event_msg":
			handleEventMsg(result, &cl, ts, &eventPending)

		case "token_count":
			handleTokenCount(result, &cl)

		case "exec_command_begin":
			if cl.CallID != "" {
				pendingByCallID[cl.CallID] = &pendingCommand{timestamp: ts, command: cl.Command, callID: cl.CallID}
			} else {
				pendingNoID = append(pendingNoID, &pendingCommand{timestamp: ts, command: cl.Command})
			}

		case "exec_command_end":
			if cl.CallID != "" {
				if pc, ok := pendingByCallID[cl.CallID]; ok {
					result.Entries = append(result.Entries, makeToolUseEntry(pc.command, cl.CallID, cl.Output, pc.timestamp, result.SessionID))
					delete(pendingByCallID, cl.CallID)
				} else {
					result.Entries = append(result.Entries, makeToolUseEntry(cl.Command, cl.CallID, cl.Output, ts, result.SessionID))
				}
			} else if len(pendingNoID) > 0 {
				pc := pendingNoID[0]
				pendingNoID = pendingNoID[1:]
				result.Entries = append(result.Entries, makeToolUseEntry(pc.command, "", cl.Output, pc.timestamp, result.SessionID))
			} else {
				result.Entries = append(result.Entries, makeToolUseEntry(cl.Command, "", cl.Output, ts, result.SessionID))
			}

		default:
			// Unknown line type — skip silently.
		}
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("scanner error after line %d: %w", lineNum, err)
	}

	// Flush any pending top-level commands that never received an end event.
	for callID, pc := range pendingByCallID {
		result.Entries = append(result.Entries, makeToolUseEntry(pc.command, pc.callID, "", pc.timestamp, result.SessionID))
		delete(pendingByCallID, callID)
	}
	// Flush any pending no-ID commands.
	for _, pc := range pendingNoID {
		result.Entries = append(result.Entries, makeToolUseEntry(pc.command, "", "", pc.timestamp, result.SessionID))
	}
	// Flush any pending event_msg commands.
	for _, pc := range eventPending {
		result.Entries = append(result.Entries, makeToolUseEntry(pc.command, "", "", pc.timestamp, result.SessionID))
	}

	return result, nil
}

// handleSessionMeta extracts session metadata from a session_meta line.
func handleSessionMeta(result *ParseResult, cl *codexLine) {
	if cl.SessionID != "" {
		result.SessionID = cl.SessionID
	}
	if cl.CWD != "" {
		result.CWD = cl.CWD
	}
	if cl.Model != "" {
		result.Model = cl.Model
	}
}

// handleEventMsg processes event_msg lines by dispatching based on event type.
// eventPending is a FIFO queue used for exec_command_begin/end pairs within
// event_msg payloads, since they lack call_id for keyed matching.
func handleEventMsg(result *ParseResult, cl *codexLine, ts time.Time, eventPending *[]*pendingCommand) {
	var ep eventPayload
	if err := json.Unmarshal(cl.Payload, &ep); err != nil {
		result.ParseErrors++
		return
	}

	var ed eventDetail
	if err := json.Unmarshal(ep.Event, &ed); err != nil {
		result.ParseErrors++
		return
	}

	switch ed.Type {
	case "user_message":
		result.Entries = append(result.Entries, agent.BasicEntry{
			EntryType:      agent.EntryTypeUser,
			EntryTimestamp: ts,
			EntryRole:      "user",
			Blocks: []agent.ContentBlock{
				agent.BasicBlock{BlockType: agent.ContentBlockText, BlockText: ed.Content},
			},
			EntryAgent:   agent.AgentCodex,
			EntrySession: result.SessionID,
		})

	case "agent_message":
		blockType := agent.ContentBlockText
		if ed.Phase == "commentary" {
			blockType = agent.ContentBlockCommentary
		}
		result.Entries = append(result.Entries, agent.BasicEntry{
			EntryType:      agent.EntryTypeAssistant,
			EntryTimestamp: ts,
			EntryRole:      "assistant",
			Blocks: []agent.ContentBlock{
				agent.BasicBlock{BlockType: blockType, BlockText: ed.Content, BlockPhase: ed.Phase},
			},
			EntryAgent:   agent.AgentCodex,
			EntrySession: result.SessionID,
		})

	case "token_count":
		var ti tokenInfo
		if err := json.Unmarshal(ed.Info, &ti); err == nil {
			result.Tokens = result.Tokens.Add(agent.TokenStats{
				InputTokens:  ti.TotalTokenUsage.InputTokens,
				OutputTokens: ti.TotalTokenUsage.OutputTokens,
				CachedTokens: ti.TotalTokenUsage.CachedTokens,
			})
		}

	case "exec_command_begin":
		*eventPending = append(*eventPending, &pendingCommand{timestamp: ts, command: ed.Command})

	case "exec_command_end":
		// Pop the oldest pending command (FIFO) since event_msg exec events
		// don't carry call_id for keyed matching.
		if len(*eventPending) > 0 {
			pc := (*eventPending)[0]
			*eventPending = (*eventPending)[1:]
			result.Entries = append(result.Entries, makeToolUseEntry(pc.command, "", ed.Output, pc.timestamp, result.SessionID))
		}
	}
}

// handleTokenCount processes top-level token_count lines.
func handleTokenCount(result *ParseResult, cl *codexLine) {
	var ti tokenInfo
	if err := json.Unmarshal(cl.Info, &ti); err != nil {
		result.ParseErrors++
		return
	}
	result.Tokens = result.Tokens.Add(agent.TokenStats{
		InputTokens:  ti.TotalTokenUsage.InputTokens,
		OutputTokens: ti.TotalTokenUsage.OutputTokens,
		CachedTokens: ti.TotalTokenUsage.CachedTokens,
	})
}

// makeToolUseEntry creates a ConversationEntry for an exec_command pair.
func makeToolUseEntry(command, callID, output string, ts time.Time, sessionID string) agent.BasicEntry {
	return agent.BasicEntry{
		EntryType:      agent.EntryTypeAssistant,
		EntryTimestamp: ts,
		EntryRole:      "assistant",
		Blocks: []agent.ContentBlock{
			agent.BasicBlock{
				BlockType:   agent.ContentBlockToolUse,
				BlockTool:   "exec",
				BlockInput:  map[string]any{"command": command},
				BlockOutput: output,
			},
		},
		EntryAgent:   agent.AgentCodex,
		EntrySession: sessionID,
	}
}

// parseTimestamp parses a timestamp string, returning zero time on failure.
func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Try RFC3339 first.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// Try RFC3339Nano.
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	return time.Time{}
}
