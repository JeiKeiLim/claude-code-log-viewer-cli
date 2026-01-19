// Package token provides token counting capabilities.
package token

import (
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
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
