// Package token provides token counting capabilities.
package token

import (
	"encoding/json"
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// Service provides token counting with caching for performance.
type Service struct {
	encoder *tiktoken.Tiktoken
	cache   map[string]int
	mu      sync.RWMutex
}

// New creates a new token counting Service using cl100k_base encoding.
// Returns an error if the encoder fails to initialize.
func New() (*Service, error) {
	return NewWithEncoding("cl100k_base")
}

// NewWithEncoding creates a new token counting Service with a custom encoding.
// Returns an error if the encoder fails to initialize.
func NewWithEncoding(encoding string) (*Service, error) {
	enc, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return nil, err
	}
	return &Service{
		encoder: enc,
		cache:   make(map[string]int),
	}, nil
}

// Calculate returns the token count for the given text.
// Results are cached for repeated calls with the same text.
func (s *Service) Calculate(text string) int {
	// Fast path: check cache with read lock
	s.mu.RLock()
	if count, ok := s.cache[text]; ok {
		s.mu.RUnlock()
		return count
	}
	s.mu.RUnlock()

	// Calculate tokens
	tokens := s.encoder.Encode(text, nil, nil)
	count := len(tokens)

	// Store in cache with write lock
	s.mu.Lock()
	s.cache[text] = count
	s.mu.Unlock()

	return count
}

// ClearCache clears the internal cache to free memory.
func (s *Service) ClearCache() {
	s.mu.Lock()
	s.cache = make(map[string]int)
	s.mu.Unlock()
}

// CalculateBatch returns token counts for multiple texts.
// Uses the same cache as individual calculations for efficiency.
func (s *Service) CalculateBatch(texts []string) []int {
	results := make([]int, len(texts))
	for i, text := range texts {
		results[i] = s.Calculate(text)
	}
	return results
}

// CalculateEntry returns the token count for a log entry.
// Handles user messages, assistant messages (including text, thinking, tool_use),
// and file-history-snapshot entries (returns 0).
func (s *Service) CalculateEntry(entry types.LogEntry) int {
	switch entry.Type {
	case types.EntryTypeUser:
		return s.Calculate(entry.Message.TextContent)
	case types.EntryTypeAssistant:
		var total int
		for _, content := range entry.Message.Content {
			switch content.Type {
			case types.ContentTypeText:
				total += s.Calculate(content.Text)
			case types.ContentTypeThinking:
				total += s.Calculate(content.Thinking)
			case types.ContentTypeToolUse:
				// Serialize ToolInput to JSON for tokenization
				if len(content.ToolInput) > 0 {
					data, err := json.Marshal(content.ToolInput)
					if err != nil {
						continue
					}
					total += s.Calculate(string(data))
				}
			}
		}
		return total
	default:
		// EntryTypeFileHistorySnapshot: no user-facing text
		return 0
	}
}

// CalculateConversation returns the total token count for a conversation.
// Uses actual Usage data from entries when available, falls back to calculation otherwise.
// Returns (total tokens, estimated) where estimated is true if any entry required calculation.
// Note: file-history-snapshot entries always return 0 tokens and are not marked as estimated.
func (s *Service) CalculateConversation(entries []types.LogEntry) (int, bool) {
	var total int
	var estimated bool
	for _, entry := range entries {
		if !entry.Usage.IsEmpty() {
			total += entry.Usage.Total()
		} else if entry.Type == types.EntryTypeFileHistorySnapshot {
			// file-history-snapshot has no text content by design - not estimated
			continue
		} else {
			total += s.CalculateEntry(entry)
			estimated = true
		}
	}
	return total, estimated
}
