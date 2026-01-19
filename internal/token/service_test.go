package token

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestNew(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s == nil {
		t.Fatal("New() returned nil Service")
	}
	if s.encoder == nil {
		t.Fatal("New() encoder is nil")
	}
	if s.cache == nil {
		t.Fatal("New() cache is nil")
	}
}

func TestCalculate(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantMin int
		wantMax int
	}{
		{"empty string", "", 0, 0},
		{"single word", "hello", 1, 2},
		{"simple sentence", "Hello, world!", 2, 5},
		{"longer text", "The quick brown fox jumps over the lazy dog.", 8, 12},
		{"code snippet", "func main() { fmt.Println(\"Hello\") }", 10, 20},
		{"unicode text", "안녕하세요", 3, 10},
		{"whitespace only", "   ", 1, 3},
		{"newlines", "line1\nline2\nline3", 4, 10},
		{"special characters", "!@#$%^&*()", 5, 15},
		{"mixed content", "Hello 世界 123 !", 5, 12},
	}

	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.Calculate(tt.text)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("Calculate(%q) = %v, want between %v and %v", tt.text, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculate_CacheBehavior(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	text := "this is test text for caching behavior"

	// First call - calculates and caches
	count1 := s.Calculate(text)

	// Second call - should return cached result
	count2 := s.Calculate(text)

	if count1 != count2 {
		t.Errorf("Calculate() cache inconsistency: first=%v, second=%v", count1, count2)
	}

	// Verify cache contains the entry
	s.mu.RLock()
	cachedCount, exists := s.cache[text]
	s.mu.RUnlock()

	if !exists {
		t.Error("Calculate() did not cache the result")
	}
	if cachedCount != count1 {
		t.Errorf("Cached value %v != calculated value %v", cachedCount, count1)
	}
}

func TestClearCache(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Populate cache
	texts := []string{"text one", "text two", "text three"}
	for _, text := range texts {
		_ = s.Calculate(text)
	}

	// Verify cache is populated
	s.mu.RLock()
	cacheLen := len(s.cache)
	s.mu.RUnlock()

	if cacheLen != len(texts) {
		t.Errorf("Cache should have %d entries, got %d", len(texts), cacheLen)
	}

	// Clear cache
	s.ClearCache()

	// Verify cache is empty
	s.mu.RLock()
	cacheLenAfter := len(s.cache)
	s.mu.RUnlock()

	if cacheLenAfter != 0 {
		t.Errorf("ClearCache() did not clear cache, has %d entries", cacheLenAfter)
	}

	// Verify service still works after clear
	count := s.Calculate("new text after clear")
	if count <= 0 {
		t.Error("Calculate() should return positive count after ClearCache()")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent Calculate calls with different texts
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			text := fmt.Sprintf("concurrent text number %d with some extra words", n)
			count := s.Calculate(text)
			if count <= 0 {
				t.Errorf("Calculate() returned non-positive count: %d", count)
			}
		}(i)
	}

	// Concurrent Calculate calls with same text (cache hits)
	sharedText := "shared text for cache testing"
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Calculate(sharedText)
		}()
	}

	// Concurrent ClearCache calls (stress test)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.ClearCache()
		}()
	}

	wg.Wait()
	// Test passes if no race condition panic occurs
}

func TestCalculate_LargeText(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Generate large text (approximately 10KB)
	var largeText string
	for i := 0; i < 1000; i++ {
		largeText += "This is a line of text to test large input handling. "
	}

	count := s.Calculate(largeText)
	if count <= 0 {
		t.Errorf("Calculate() should return positive count for large text, got %d", count)
	}

	// Large text should have many tokens
	if count < 1000 {
		t.Errorf("Calculate() for large text should return >1000 tokens, got %d", count)
	}
}

func TestCalculate_Consistency(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	text := "consistent text for verification"

	// Multiple calls should return exact same count
	counts := make([]int, 10)
	for i := 0; i < 10; i++ {
		counts[i] = s.Calculate(text)
	}

	for i := 1; i < len(counts); i++ {
		if counts[i] != counts[0] {
			t.Errorf("Calculate() inconsistent: call %d returned %d, expected %d", i, counts[i], counts[0])
		}
	}
}

func TestEncoder_cl100k_base(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// cl100k_base known tokenization test
	// "Hello, world!" is commonly 4 tokens in cl100k_base
	text := "Hello, world!"
	count := s.Calculate(text)

	// cl100k_base should tokenize this as 4 tokens: "Hello", ",", " world", "!"
	if count != 4 {
		t.Errorf("Calculate(%q) = %d, expected 4 for cl100k_base encoding", text, count)
	}
}

func TestNewWithInvalidEncoding(t *testing.T) {
	// Test that NewWithEncoding returns error for invalid encoding name
	_, err := NewWithEncoding("invalid_encoding_that_does_not_exist")
	if err == nil {
		t.Error("NewWithEncoding() with invalid encoding should return error")
	}
}

func TestCalculateBatch(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name     string
		texts    []string
		wantLen  int
		checkSum bool // if true, verify sum > 0 for non-empty texts
	}{
		{"empty batch", []string{}, 0, false},
		{"single text", []string{"hello"}, 1, true},
		{"multiple texts", []string{"hello", "world", "test"}, 3, true},
		{"batch with empty", []string{"hello", "", "world"}, 3, true},
		{"larger batch", make([]string, 100), 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For larger batch test, fill with actual text
			if tt.name == "larger batch" {
				for i := range tt.texts {
					tt.texts[i] = fmt.Sprintf("text number %d", i)
				}
			}

			results := s.CalculateBatch(tt.texts)
			if len(results) != tt.wantLen {
				t.Errorf("CalculateBatch() returned %d results, want %d", len(results), tt.wantLen)
			}

			if tt.checkSum {
				sum := 0
				for _, r := range results {
					sum += r
				}
				if sum == 0 && len(tt.texts) > 0 {
					hasNonEmpty := false
					for _, text := range tt.texts {
						if text != "" {
							hasNonEmpty = true
							break
						}
					}
					if hasNonEmpty {
						t.Error("CalculateBatch() sum should be > 0 for non-empty texts")
					}
				}
			}
		})
	}
}

func TestCalculateBatch_UsesSameCache(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	texts := []string{"shared text one", "shared text two"}

	// First batch call
	s.CalculateBatch(texts)

	// Verify cache contains entries
	s.mu.RLock()
	cacheLen := len(s.cache)
	s.mu.RUnlock()

	if cacheLen != 2 {
		t.Errorf("Cache should have 2 entries after batch, got %d", cacheLen)
	}

	// Second individual call should hit cache
	count := s.Calculate(texts[0])
	if count == 0 {
		t.Error("Calculate() should return cached result from batch")
	}
}

func TestCalculateEntry(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name    string
		entry   types.LogEntry
		wantMin int
		wantMax int
	}{
		{
			name: "user message",
			entry: types.LogEntry{
				Type:    types.EntryTypeUser,
				Message: types.Message{TextContent: "Hello, world!"},
			},
			wantMin: 2,
			wantMax: 6,
		},
		{
			name: "assistant with text content",
			entry: types.LogEntry{
				Type: types.EntryTypeAssistant,
				Message: types.Message{
					Content: []types.MessageContent{
						{Type: types.ContentTypeText, Text: "Hello from assistant"},
					},
				},
			},
			wantMin: 2,
			wantMax: 6,
		},
		{
			name: "assistant with thinking block",
			entry: types.LogEntry{
				Type: types.EntryTypeAssistant,
				Message: types.Message{
					Content: []types.MessageContent{
						{Type: types.ContentTypeThinking, Thinking: "Let me think about this..."},
					},
				},
			},
			wantMin: 3,
			wantMax: 10,
		},
		{
			name: "assistant with tool_use",
			entry: types.LogEntry{
				Type: types.EntryTypeAssistant,
				Message: types.Message{
					Content: []types.MessageContent{
						{
							Type:      types.ContentTypeToolUse,
							ToolName:  "Read",
							ToolInput: map[string]any{"file": "test.go", "content": "package main"},
						},
					},
				},
			},
			wantMin: 5,
			wantMax: 25,
		},
		{
			name: "assistant with multiple content blocks",
			entry: types.LogEntry{
				Type: types.EntryTypeAssistant,
				Message: types.Message{
					Content: []types.MessageContent{
						{Type: types.ContentTypeText, Text: "I will help you"},
						{Type: types.ContentTypeThinking, Thinking: "Let me analyze"},
						{
							Type:      types.ContentTypeToolUse,
							ToolInput: map[string]any{"command": "ls"},
						},
					},
				},
			},
			wantMin: 8,
			wantMax: 30,
		},
		{
			name:    "file-history-snapshot",
			entry:   types.LogEntry{Type: types.EntryTypeFileHistorySnapshot},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "empty user message",
			entry: types.LogEntry{
				Type:    types.EntryTypeUser,
				Message: types.Message{TextContent: ""},
			},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "assistant with empty tool_input",
			entry: types.LogEntry{
				Type: types.EntryTypeAssistant,
				Message: types.Message{
					Content: []types.MessageContent{
						{Type: types.ContentTypeToolUse, ToolInput: map[string]any{}},
					},
				},
			},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "assistant with unknown content type",
			entry: types.LogEntry{
				Type: types.EntryTypeAssistant,
				Message: types.Message{
					Content: []types.MessageContent{
						{Type: "unknown_type", Text: "ignored"},
					},
				},
			},
			wantMin: 0,
			wantMax: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.CalculateEntry(tt.entry)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculateEntry() = %d, want [%d, %d]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateEntry_CacheBehavior(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	entry := types.LogEntry{
		Type:    types.EntryTypeUser,
		Message: types.Message{TextContent: "Test message for caching"},
	}

	// First call
	count1 := s.CalculateEntry(entry)

	// Second call with same entry content
	count2 := s.CalculateEntry(entry)

	if count1 != count2 {
		t.Errorf("CalculateEntry() inconsistent: first=%d, second=%d", count1, count2)
	}

	// Verify underlying Calculate cache was used
	s.mu.RLock()
	_, exists := s.cache[entry.Message.TextContent]
	s.mu.RUnlock()

	if !exists {
		t.Error("CalculateEntry() should cache via Calculate()")
	}
}

func TestCalculateConversation(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name          string
		entries       []types.LogEntry
		wantEstimated bool
		wantMin       int
	}{
		{
			name:          "empty conversation",
			entries:       []types.LogEntry{},
			wantEstimated: false,
			wantMin:       0,
		},
		{
			name: "all entries with usage data",
			entries: []types.LogEntry{
				{
					Type:  types.EntryTypeAssistant,
					Usage: types.TokenUsage{InputTokens: 100, OutputTokens: 50},
				},
				{
					Type:  types.EntryTypeAssistant,
					Usage: types.TokenUsage{InputTokens: 200, OutputTokens: 75},
				},
			},
			wantEstimated: false,
			wantMin:       425, // 100+50+200+75
		},
		{
			name: "all entries without usage data",
			entries: []types.LogEntry{
				{
					Type:    types.EntryTypeUser,
					Message: types.Message{TextContent: "Hello"},
				},
				{
					Type: types.EntryTypeAssistant,
					Message: types.Message{
						Content: []types.MessageContent{
							{Type: types.ContentTypeText, Text: "Hi there"},
						},
					},
				},
			},
			wantEstimated: true,
			wantMin:       2, // At least some tokens
		},
		{
			name: "mixed entries",
			entries: []types.LogEntry{
				{
					Type:  types.EntryTypeAssistant,
					Usage: types.TokenUsage{InputTokens: 100, OutputTokens: 50},
				},
				{
					Type:    types.EntryTypeUser,
					Message: types.Message{TextContent: "Hello"},
				},
			},
			wantEstimated: true,
			wantMin:       151, // 150 from usage + at least 1 from calculated
		},
		{
			name: "with file-history-snapshot",
			entries: []types.LogEntry{
				{Type: types.EntryTypeFileHistorySnapshot},
				{
					Type:  types.EntryTypeAssistant,
					Usage: types.TokenUsage{InputTokens: 50},
				},
			},
			wantEstimated: false, // file-history-snapshot is 0 by design, not estimated
			wantMin:       50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, estimated := s.CalculateConversation(tt.entries)

			if estimated != tt.wantEstimated {
				t.Errorf("CalculateConversation() estimated = %v, want %v", estimated, tt.wantEstimated)
			}

			if total < tt.wantMin {
				t.Errorf("CalculateConversation() total = %d, want >= %d", total, tt.wantMin)
			}
		})
	}
}

func TestCalculateConversation_UsesActualUsageWhenAvailable(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{Type: types.ContentTypeText, Text: "This is a test message"},
			},
		},
		Usage: types.TokenUsage{InputTokens: 1000, OutputTokens: 500},
	}

	total, estimated := s.CalculateConversation([]types.LogEntry{entry})

	// Should use actual usage (1500), not calculated tokens
	if total != 1500 {
		t.Errorf("CalculateConversation() should use actual usage, got %d, want 1500", total)
	}
	if estimated {
		t.Error("CalculateConversation() should not be estimated when usage data available")
	}
}

func BenchmarkCalculateEntry(b *testing.B) {
	s, err := New()
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}

	entry := types.LogEntry{
		Type: types.EntryTypeAssistant,
		Message: types.Message{
			Content: []types.MessageContent{
				{Type: types.ContentTypeText, Text: strings.Repeat("Hello world ", 100)},
				{Type: types.ContentTypeThinking, Thinking: strings.Repeat("Let me think ", 50)},
				{
					Type: types.ContentTypeToolUse,
					ToolInput: map[string]any{
						"file":    "test.go",
						"content": strings.Repeat("package main\n", 10),
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.CalculateEntry(entry)
	}
	// Verify: go test -bench=BenchmarkCalculateEntry should show < 50ms/op
}

func BenchmarkCalculateBatch(b *testing.B) {
	s, err := New()
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}

	texts := make([]string, 100)
	for i := range texts {
		texts[i] = fmt.Sprintf("This is text number %d with some content to tokenize", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ClearCache() // Clear to measure actual calculation time
		s.CalculateBatch(texts)
	}
}
