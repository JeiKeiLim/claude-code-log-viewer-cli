package agent

import (
	"bytes"
	"encoding/json"
)

// DetectFormat inspects JSONL data and returns the most likely AgentType.
// It examines the first non-blank line to identify the format.
// Returns AgentClaudeCode by default if no specific format is detected.
func DetectFormat(data []byte) AgentType {
	if len(data) == 0 {
		return AgentClaudeCode
	}

	// Find the first non-empty line
	lines := bytes.Split(data, []byte{'\n'})
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		return detectLine(line)
	}

	return AgentClaudeCode
}

// detectLine classifies a single JSONL line by checking known structural markers.
func detectLine(line []byte) AgentType {
	// Fast path: substring checks before full JSON parse
	if bytes.Contains(line, []byte(`"type":"event_msg"`)) ||
		bytes.Contains(line, []byte(`"type":"session_meta"`)) ||
		bytes.Contains(line, []byte(`"type":"token_count"`)) ||
		bytes.Contains(line, []byte(`"type":"exec_command_begin"`)) ||
		bytes.Contains(line, []byte(`"type":"exec_command_end"`)) {
		return AgentCodex
	}

	// Structural check: Claude Code entries have a "message" field and
	// a "type" of "user" or "assistant" at the top level.
	var probe struct {
		Type      string          `json:"type"`
		SessionID string          `json:"sessionId"`
		Message   json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		return AgentClaudeCode
	}

	switch probe.Type {
	case "user", "assistant":
		return AgentClaudeCode
	case "event_msg", "session_meta", "token_count",
		"exec_command_begin", "exec_command_end",
		"response_item", "compacted":
		return AgentCodex
	}

	return AgentClaudeCode
}
