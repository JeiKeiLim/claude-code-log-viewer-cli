package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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

// flatEventPayload is the flat payload format used by real Codex CLI v0.116.0+:
//
//	{"type":"event_msg","payload":{"type":"user_message","message":"hi"}}
type flatEventPayload struct {
	Type    string          `json:"type"`
	Message string          `json:"message"`
	Phase   string          `json:"phase"`
	Info    json.RawMessage `json:"info"`
	Command string          `json:"command"`
	Output  string          `json:"output"`
}

// eventPayload is the nested structure within payload for legacy event_msg lines:
//
//	{"type":"event_msg","payload":{"event":{"type":"user_message","content":"hi"}}}
type eventPayload struct {
	Event json.RawMessage `json:"event"`
}

// eventDetail is the event structure inside a nested event_msg payload.
type eventDetail struct {
	Type     string          `json:"type"`
	Content  string          `json:"content"`
	Phase    string          `json:"phase"`
	Command  string          `json:"command"`
	ExitCode int             `json:"exit_code"`
	Output   string          `json:"output"`
	Info     json.RawMessage `json:"info"`
}

// sessionMetaPayload is the nested payload structure for real Codex CLI v0.116.0+
// session_meta lines: {"type":"session_meta","payload":{"id":"...","cwd":"/path",...}}.
type sessionMetaPayload struct {
	ID    string `json:"id"`
	CWD   string `json:"cwd"`
	Model string `json:"model"`
}

// tokenInfo holds token usage information.
type tokenInfo struct {
	TotalTokenUsage tokenUsageDetail `json:"total_token_usage"`
}

// tokenUsageDetail holds the individual token counts.
// Supports both "cached_tokens" (legacy) and "cached_input_tokens" (real Codex CLI v0.116.0+).
type tokenUsageDetail struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	CachedTokens      int `json:"cached_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	ReasoningTokens   int `json:"reasoning_output_tokens"`
}

// cachedTokens returns the cached token count, preferring cached_input_tokens (real format)
// and falling back to cached_tokens (legacy).
func (d tokenUsageDetail) cachedTokens() int {
	if d.CachedInputTokens > 0 {
		return d.CachedInputTokens
	}
	return d.CachedTokens
}

// responseItemPayload is the payload for response_item lines (Codex CLI v0.116.0+).
// Variants: message, function_call, function_call_output, reasoning.
type responseItemPayload struct {
	Type    string                `json:"type"`
	Role    string                `json:"role"`
	Content []responseContentItem `json:"content"`
	Name    string                `json:"name"`
	CallID  string                `json:"call_id"`
	Output  string                `json:"output"`
}

// responseContentItem is a single content block within a response_item message.
type responseContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// pendingCommand tracks a tool-use command awaiting its result.
type pendingCommand struct {
	timestamp time.Time
	command   string
	callID    string
}

// pendingFunctionCall tracks a response_item function_call awaiting its output.
type pendingFunctionCall struct {
	timestamp time.Time
	name      string
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
	// Map for response_item function_call awaiting function_call_output.
	pendingFnCalls := make(map[string]*pendingFunctionCall)
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

		case "response_item":
			handleResponseItem(result, &cl, ts, pendingFnCalls)

		default:
			// Unknown line type -- skip silently.
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
	// Flush any pending function_call entries that never got output.
	for callID, fc := range pendingFnCalls {
		result.Entries = append(result.Entries, makeFunctionCallEntry(fc.name, fc.callID, "", fc.timestamp, result.SessionID))
		delete(pendingFnCalls, callID)
	}

	result.Entries = dedupeMirroredMessages(result.Entries)

	return result, nil
}

// handleSessionMeta extracts session metadata from a session_meta line.
// Supports both nested payload format (Codex CLI v0.116.0+):
//
//	{"type":"session_meta","payload":{"id":"...","cwd":"/path","model":"o3"}}
//
// and flat format (legacy/test):
//
//	{"type":"session_meta","session_id":"...","cwd":"/path","model":"o3"}
func handleSessionMeta(result *ParseResult, cl *codexLine) {
	// Try nested payload first (real Codex CLI format).
	if cl.Payload != nil {
		var mp sessionMetaPayload
		if err := json.Unmarshal(cl.Payload, &mp); err == nil {
			if mp.ID != "" {
				result.SessionID = mp.ID
			}
			if mp.CWD != "" {
				result.CWD = mp.CWD
			}
			if mp.Model != "" {
				result.Model = mp.Model
			}
			return
		}
	}
	// Fallback to flat fields for backward compatibility.
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
// Supports both the flat payload format (real Codex CLI v0.116.0+) and the
// nested event format (legacy fixtures):
//
//	Flat:    {"type":"event_msg","payload":{"type":"user_message","message":"hi"}}
//	Nested:  {"type":"event_msg","payload":{"event":{"type":"user_message","content":"hi"}}}
//
// eventPending is a FIFO queue used for exec_command_begin/end pairs within
// event_msg payloads, since they lack call_id for keyed matching.
func handleEventMsg(result *ParseResult, cl *codexLine, ts time.Time, eventPending *[]*pendingCommand) {
	// Try flat payload format first (real Codex CLI v0.116.0+).
	var flat flatEventPayload
	if err := json.Unmarshal(cl.Payload, &flat); err == nil && flat.Type != "" {
		handleFlatEventMsg(result, &flat, ts, eventPending)
		return
	}

	// Fallback to nested format (legacy fixtures).
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
				CachedTokens: ti.TotalTokenUsage.cachedTokens(),
			})
		}

	case "exec_command_begin":
		*eventPending = append(*eventPending, &pendingCommand{timestamp: ts, command: ed.Command})

	case "exec_command_end":
		if len(*eventPending) > 0 {
			pc := (*eventPending)[0]
			*eventPending = (*eventPending)[1:]
			result.Entries = append(result.Entries, makeToolUseEntry(pc.command, "", ed.Output, pc.timestamp, result.SessionID))
		}
	}
}

// handleFlatEventMsg dispatches flat-format event_msg types (real Codex CLI v0.116.0+).
func handleFlatEventMsg(result *ParseResult, flat *flatEventPayload, ts time.Time, eventPending *[]*pendingCommand) {
	switch flat.Type {
	case "user_message":
		result.Entries = append(result.Entries, agent.BasicEntry{
			EntryType:      agent.EntryTypeUser,
			EntryTimestamp: ts,
			EntryRole:      "user",
			Blocks: []agent.ContentBlock{
				agent.BasicBlock{BlockType: agent.ContentBlockText, BlockText: flat.Message},
			},
			EntryAgent:   agent.AgentCodex,
			EntrySession: result.SessionID,
		})

	case "agent_message":
		blockType := agent.ContentBlockText
		if flat.Phase == "commentary" {
			blockType = agent.ContentBlockCommentary
		}
		result.Entries = append(result.Entries, agent.BasicEntry{
			EntryType:      agent.EntryTypeAssistant,
			EntryTimestamp: ts,
			EntryRole:      "assistant",
			Blocks: []agent.ContentBlock{
				agent.BasicBlock{BlockType: blockType, BlockText: flat.Message, BlockPhase: flat.Phase},
			},
			EntryAgent:   agent.AgentCodex,
			EntrySession: result.SessionID,
		})

	case "token_count":
		var ti tokenInfo
		if flat.Info != nil {
			if err := json.Unmarshal(flat.Info, &ti); err == nil {
				result.Tokens = result.Tokens.Add(agent.TokenStats{
					InputTokens:  ti.TotalTokenUsage.InputTokens,
					OutputTokens: ti.TotalTokenUsage.OutputTokens,
					CachedTokens: ti.TotalTokenUsage.cachedTokens(),
				})
			}
		}

	case "exec_command_begin":
		*eventPending = append(*eventPending, &pendingCommand{timestamp: ts, command: flat.Command})

	case "exec_command_end":
		if len(*eventPending) > 0 {
			pc := (*eventPending)[0]
			*eventPending = (*eventPending)[1:]
			result.Entries = append(result.Entries, makeToolUseEntry(pc.command, "", flat.Output, pc.timestamp, result.SessionID))
		}

	case "task_started":
		// Informational only, no conversation entry.
	}
}

// handleResponseItem processes response_item lines (Codex CLI v0.116.0+).
// Handles message, function_call, function_call_output, and reasoning payloads.
func handleResponseItem(result *ParseResult, cl *codexLine, ts time.Time, pendingFnCalls map[string]*pendingFunctionCall) {
	var rip responseItemPayload
	if err := json.Unmarshal(cl.Payload, &rip); err != nil {
		result.ParseErrors++
		return
	}

	switch rip.Type {
	case "message":
		if len(rip.Content) == 0 {
			return
		}
		entryType := agent.EntryTypeUser
		role := "user"
		if rip.Role == "assistant" {
			entryType = agent.EntryTypeAssistant
			role = "assistant"
		} else if rip.Role == "developer" {
			// developer messages are system context, skip as conversation entries.
			return
		}

		var blocks []agent.ContentBlock
		for _, ci := range rip.Content {
			switch ci.Type {
			case "input_text", "output_text":
				blocks = append(blocks, agent.BasicBlock{
					BlockType: agent.ContentBlockText,
					BlockText: ci.Text,
				})
			}
		}
		if len(blocks) == 0 {
			return
		}

		result.Entries = append(result.Entries, agent.BasicEntry{
			EntryType:      entryType,
			EntryTimestamp: ts,
			EntryRole:      role,
			Blocks:         blocks,
			EntryAgent:     agent.AgentCodex,
			EntrySession:   result.SessionID,
		})

	case "function_call":
		if rip.CallID != "" {
			pendingFnCalls[rip.CallID] = &pendingFunctionCall{
				timestamp: ts,
				name:      rip.Name,
				callID:    rip.CallID,
			}
		}

	case "function_call_output":
		if rip.CallID != "" {
			if fc, ok := pendingFnCalls[rip.CallID]; ok {
				result.Entries = append(result.Entries, makeFunctionCallEntry(fc.name, fc.callID, rip.Output, fc.timestamp, result.SessionID))
				delete(pendingFnCalls, fc.callID)
			}
		}

	case "reasoning":
		// Reasoning items are internal model thinking, skip as conversation entries.
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
		CachedTokens: ti.TotalTokenUsage.cachedTokens(),
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

// makeFunctionCallEntry creates a ConversationEntry for a response_item function_call/output pair.
func makeFunctionCallEntry(name, callID, output string, ts time.Time, sessionID string) agent.BasicEntry {
	return agent.BasicEntry{
		EntryType:      agent.EntryTypeAssistant,
		EntryTimestamp: ts,
		EntryRole:      "assistant",
		Blocks: []agent.ContentBlock{
			agent.BasicBlock{
				BlockType:   agent.ContentBlockToolUse,
				BlockTool:   name,
				BlockOutput: output,
			},
		},
		EntryAgent:   agent.AgentCodex,
		EntrySession: sessionID,
	}
}

func dedupeMirroredMessages(entries []agent.ConversationEntry) []agent.ConversationEntry {
	if len(entries) < 2 {
		return entries
	}

	deduped := make([]agent.ConversationEntry, 0, len(entries))
	for _, entry := range entries {
		if len(deduped) > 0 && isMirroredMessageDuplicate(deduped[len(deduped)-1], entry) {
			continue
		}
		deduped = append(deduped, entry)
	}
	return deduped
}

func isMirroredMessageDuplicate(prev, current agent.ConversationEntry) bool {
	if prev.Type() != current.Type() || prev.Role() != current.Role() {
		return false
	}
	if prev.Type() != agent.EntryTypeUser && prev.Type() != agent.EntryTypeAssistant {
		return false
	}
	if hasToolBlock(prev) || hasToolBlock(current) {
		return false
	}
	if !timestampsClose(prev.Timestamp(), current.Timestamp(), 2*time.Second) {
		return false
	}
	return messageTextSignature(prev) == messageTextSignature(current)
}

func hasToolBlock(entry agent.ConversationEntry) bool {
	for _, block := range entry.ContentBlocks() {
		if block.ContentType() == agent.ContentBlockToolUse {
			return true
		}
	}
	return false
}

func timestampsClose(a, b time.Time, threshold time.Duration) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	delta := a.Sub(b)
	if delta < 0 {
		delta = -delta
	}
	return delta <= threshold
}

func messageTextSignature(entry agent.ConversationEntry) string {
	blocks := entry.ContentBlocks()
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if text := block.Text(); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
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
