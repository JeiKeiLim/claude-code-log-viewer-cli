package token

import (
	"fmt"
	"sync"
	"testing"
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
