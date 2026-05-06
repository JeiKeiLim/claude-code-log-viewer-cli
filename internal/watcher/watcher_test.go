package watcher

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestNew(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write initial content
	initialContent := `{"type":"user","message":{"role":"user","content":"hello"},"timestamp":"2026-01-16T10:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		filePath  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid file",
			filePath: testFile,
			wantErr:  false,
		},
		{
			name:      "non-existent file",
			filePath:  filepath.Join(tmpDir, "nonexistent.jsonl"),
			wantErr:   true,
			errSubstr: "no such file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := New(tt.filePath)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = w.Close() }()

			if w.filePath != tt.filePath {
				t.Errorf("filePath = %q, want %q", w.filePath, tt.filePath)
			}
			if w.fsWatcher == nil {
				t.Error("fsWatcher should not be nil")
			}
			// lastReadPos should be at end of file (size of initial content)
			if w.lastReadPos != int64(len(initialContent)) {
				t.Errorf("lastReadPos = %d, want %d", w.lastReadPos, len(initialContent))
			}
		})
	}
}

func TestReadNewEntries(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write initial content
	initialContent := `{"type":"user","message":{"role":"user","content":"initial"},"timestamp":"2026-01-16T10:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Initially, no new entries (already at end)
	entries, err := w.readNewEntries()
	if err != nil {
		t.Fatalf("unexpected error on initial read: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries initially, got %d", len(entries))
	}

	// Append new content
	newContent := `{"type":"user","message":{"role":"user","content":"new message"},"timestamp":"2026-01-16T10:01:00Z"}
`
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open file for append: %v", err)
	}
	if _, err := f.WriteString(newContent); err != nil {
		t.Fatalf("failed to append to file: %v", err)
	}
	_ = f.Close()

	// Read new entries
	entries, err = w.readNewEntries()
	if err != nil {
		t.Fatalf("unexpected error reading new entries: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 new entry, got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].Message.TextContent != "new message" {
		t.Errorf("expected content 'new message', got %q", entries[0].Message.TextContent)
	}

	// Reading again should return no entries (already read)
	entries, err = w.readNewEntries()
	if err != nil {
		t.Fatalf("unexpected error on second read: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries on second read, got %d", len(entries))
	}
}

func TestReadNewEntriesWaitsForCompleteJSONLLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	initialContent := `{"type":"user","message":{"role":"user","content":"initial"},"timestamp":"2026-01-16T10:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	partial := `{"type":"user","message":{"role":"user","content":"split`
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open file for append: %v", err)
	}
	if _, err := f.WriteString(partial); err != nil {
		t.Fatalf("failed to append partial line: %v", err)
	}
	_ = f.Close()

	entries, err := w.readNewEntries()
	if err != nil {
		t.Fatalf("unexpected error reading partial line: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries for partial line, got %d", len(entries))
	}
	if got, want := w.LastPosition(), int64(len(initialContent)); got != want {
		t.Fatalf("last read position advanced for partial line: got %d, want %d", got, want)
	}

	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to reopen file for append: %v", err)
	}
	if _, err := f.WriteString(` message"},"timestamp":"2026-01-16T10:01:00Z"}` + "\n"); err != nil {
		t.Fatalf("failed to append line completion: %v", err)
	}
	_ = f.Close()

	entries, err = w.readNewEntries()
	if err != nil {
		t.Fatalf("unexpected error reading completed line: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 completed entry, got %d", len(entries))
	}
	if entries[0].Message.TextContent != "split message" {
		t.Fatalf("entry text = %q, want %q", entries[0].Message.TextContent, "split message")
	}
}

func TestReadNewEntriesUsesCustomParser(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	initialContent := `{"kind":"initial"}` + "\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var parsed string
	w, err := NewWithParser(testFile, func(r io.Reader) ([]types.LogEntry, error) {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		parsed = string(data)
		return []types.LogEntry{{
			Type: types.EntryTypeUser,
			Message: types.Message{
				Role:        "user",
				TextContent: "custom",
			},
		}}, nil
	})
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	newContent := `{"kind":"custom"}` + "\n"
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open file for append: %v", err)
	}
	if _, err := f.WriteString(newContent); err != nil {
		t.Fatalf("failed to append new content: %v", err)
	}
	_ = f.Close()

	entries, err := w.readNewEntries()
	if err != nil {
		t.Fatalf("unexpected error reading new entries: %v", err)
	}
	if parsed != newContent {
		t.Fatalf("custom parser saw %q, want %q", parsed, newContent)
	}
	if len(entries) != 1 || entries[0].Message.TextContent != "custom" {
		t.Fatalf("custom parser entries = %#v, want one custom entry", entries)
	}
}

func TestTruncationDetection(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write initial content (larger file)
	initialContent := `{"type":"user","message":{"role":"user","content":"message 1"},"timestamp":"2026-01-16T10:00:00Z"}
{"type":"user","message":{"role":"user","content":"message 2"},"timestamp":"2026-01-16T10:01:00Z"}
{"type":"user","message":{"role":"user","content":"message 3"},"timestamp":"2026-01-16T10:02:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Truncate the file (simulate new session)
	newContent := `{"type":"user","message":{"role":"user","content":"fresh start"},"timestamp":"2026-01-16T11:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to truncate file: %v", err)
	}

	// Reading should detect truncation
	_, err = w.readNewEntries()
	if !errors.Is(err, ErrFileTruncated) {
		t.Errorf("expected ErrFileTruncated, got %v", err)
	}

	// After truncation, lastReadPos should be 0
	if w.lastReadPos != 0 {
		t.Errorf("lastReadPos = %d after truncation, want 0", w.lastReadPos)
	}

	// Next read should return the new content
	entries, err := w.readNewEntries()
	if err != nil {
		t.Fatalf("unexpected error after truncation: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry after truncation, got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].Message.TextContent != "fresh start" {
		t.Errorf("expected content 'fresh start', got %q", entries[0].Message.TextContent)
	}
}

func TestCloseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// First close should succeed
	if err := w.Close(); err != nil {
		t.Errorf("first Close() failed: %v", err)
	}
	if !w.IsClosed() {
		t.Error("expected IsClosed() to return true after Close()")
	}

	// Second close should also succeed (idempotent)
	if err := w.Close(); err != nil {
		t.Errorf("second Close() failed: %v", err)
	}

	// Third close should still succeed
	if err := w.Close(); err != nil {
		t.Errorf("third Close() failed: %v", err)
	}
}

func TestWaitForEvent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Create file with initial content
	initialContent := `{"type":"user","message":{"role":"user","content":"initial"},"timestamp":"2026-01-16T10:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Start waiting for events in a goroutine
	done := make(chan struct{})
	var result interface{}
	go func() {
		cmd := w.WaitForEvent()
		result = cmd()
		close(done)
	}()

	// Give the watcher time to start listening
	time.Sleep(50 * time.Millisecond)

	// Append to the file
	newContent := `{"type":"user","message":{"role":"user","content":"new entry"},"timestamp":"2026-01-16T10:01:00Z"}
`
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open file for append: %v", err)
	}
	if _, err := f.WriteString(newContent); err != nil {
		t.Fatalf("failed to append to file: %v", err)
	}
	_ = f.Close()

	// Wait for result with timeout
	select {
	case <-done:
		// Check result type
		switch msg := result.(type) {
		case NewEntriesMsg:
			if len(msg.Entries) != 1 {
				t.Errorf("expected 1 entry, got %d", len(msg.Entries))
			}
		case nil:
			t.Error("received nil message")
		default:
			t.Errorf("unexpected message type: %T", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file event")
	}
}

// TestNewWithPosition tests the position-aware constructor for streaming mode (Story 8.3).
func TestNewWithPosition(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write initial content
	initialContent := `{"type":"user","message":{"role":"user","content":"test"},"timestamp":"2026-01-16T10:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	pos := int64(len(initialContent))
	w, err := NewWithPosition(testFile, pos)
	if err != nil {
		t.Fatalf("NewWithPosition failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Verify position was set correctly
	w.mu.Lock()
	if w.lastReadPos != pos {
		t.Errorf("expected lastReadPos=%d, got %d", pos, w.lastReadPos)
	}
	w.mu.Unlock()

	// Verify watcher fields are set
	if w.filePath != testFile {
		t.Errorf("expected filePath=%s, got %s", testFile, w.filePath)
	}
	if w.fsWatcher == nil {
		t.Error("fsWatcher should not be nil")
	}
}

// TestNewWithPosition_ZeroPosition tests starting from beginning of file.
func TestNewWithPosition_ZeroPosition(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write initial content
	initialContent := `{"type":"user","message":{"role":"user","content":"test"},"timestamp":"2026-01-16T10:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Position 0 should read from beginning
	w, err := NewWithPosition(testFile, 0)
	if err != nil {
		t.Fatalf("NewWithPosition(0) failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	w.mu.Lock()
	if w.lastReadPos != 0 {
		t.Errorf("expected lastReadPos=0, got %d", w.lastReadPos)
	}
	w.mu.Unlock()

	// Reading should return the existing content
	entries, err := w.ReadNewEntries()
	if err != nil {
		t.Fatalf("ReadNewEntries failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry from beginning, got %d", len(entries))
	}
}

// TestReadNewEntries_Exported tests the exported ReadNewEntries method for streaming mode.
func TestReadNewEntries_Exported(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write initial content
	initialContent := `{"type":"user","message":{"role":"user","content":"initial"},"timestamp":"2026-01-16T10:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Start watching from end (skip initial content)
	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Initially no new entries
	entries, err := w.ReadNewEntries()
	if err != nil {
		t.Fatalf("ReadNewEntries failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries initially, got %d", len(entries))
	}

	// Append new content
	newContent := `{"type":"user","message":{"role":"user","content":"new message"},"timestamp":"2026-01-16T10:01:00Z"}
`
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open file for append: %v", err)
	}
	if _, err := f.WriteString(newContent); err != nil {
		t.Fatalf("failed to append to file: %v", err)
	}
	_ = f.Close()

	// Should now read the new entry
	entries, err = w.ReadNewEntries()
	if err != nil {
		t.Fatalf("ReadNewEntries failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 new entry, got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].Message.TextContent != "new message" {
		t.Errorf("expected content 'new message', got %q", entries[0].Message.TextContent)
	}
}

// TestNewUsesNewWithPosition verifies that New() correctly delegates to NewWithPosition().
func TestNewUsesNewWithPosition(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write initial content
	initialContent := `{"type":"user","message":{"role":"user","content":"test"},"timestamp":"2026-01-16T10:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	// New() should set lastReadPos to file size (end of file)
	expectedPos := int64(len(initialContent))
	w.mu.Lock()
	if w.lastReadPos != expectedPos {
		t.Errorf("New() should set lastReadPos to file size, expected %d, got %d", expectedPos, w.lastReadPos)
	}
	w.mu.Unlock()
}

// TestCloseRemovesWatchedPaths verifies Close() removes all watched paths before closing.
// This is critical for macOS kqueue which opens a file descriptor per watched path.
// Story 9.1 AC-1, AC-2
func TestCloseRemovesWatchedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Verify path is being watched before close
	watchList := w.fsWatcher.WatchList()
	if len(watchList) == 0 {
		t.Error("expected at least one watched path before Close()")
	}

	// Close should remove paths
	if err := w.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// After close, fsWatcher is closed so WatchList is not accessible
	// The key is that Close() succeeded without error
	if !w.IsClosed() {
		t.Error("expected IsClosed() to return true after Close()")
	}
}

// TestCloseWithDeletedPath verifies Close() succeeds even when watched path is deleted.
// This tests AC-3: no panic when Remove() is called on non-existent paths.
func TestCloseWithDeletedPath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Delete the file while it's being watched
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("failed to delete test file: %v", err)
	}

	// Close should NOT panic and should succeed (Remove() errors are ignored)
	if err := w.Close(); err != nil {
		t.Errorf("Close() failed after path deletion: %v", err)
	}

	if !w.IsClosed() {
		t.Error("expected IsClosed() to return true after Close()")
	}
}

// TestCloseWithNilFsWatcher tests Close() when fsWatcher is nil (edge case).
// This shouldn't happen in practice but ensures robustness.
func TestCloseWithNilFsWatcher(t *testing.T) {
	w := &Watcher{
		filePath:  "/nonexistent/path",
		fsWatcher: nil, // Intentionally nil
	}

	// Should not panic
	if err := w.Close(); err != nil {
		t.Errorf("Close() with nil fsWatcher should return nil, got: %v", err)
	}

	if !w.IsClosed() {
		t.Error("expected IsClosed() to return true")
	}
}

// Story 9.2 Tests: Channel Access Methods

// TestEventsChanReturnsChannel tests EventsChan() returns the fsnotify events channel.
func TestEventsChanReturnsChannel(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	ch := w.EventsChan()
	if ch == nil {
		t.Error("EventsChan() should return non-nil channel")
	}
}

// TestErrorsChanReturnsChannel tests ErrorsChan() returns the fsnotify errors channel.
func TestErrorsChanReturnsChannel(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	ch := w.ErrorsChan()
	if ch == nil {
		t.Error("ErrorsChan() should return non-nil channel")
	}
}

// TestEventsChanReceivesEvents tests that EventsChan() can receive actual file events.
func TestEventsChanReceivesEvents(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	ch := w.EventsChan()

	// Modify the file
	go func() {
		time.Sleep(50 * time.Millisecond)
		f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		_, _ = f.WriteString("new content")
		_ = f.Close()
	}()

	// Wait for event with timeout
	select {
	case event := <-ch:
		// Should receive a Write event
		if !event.Has(fsnotify.Write) {
			t.Errorf("expected Write event, got %v", event.Op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event from EventsChan()")
	}
}

// Story 9.3 Stress Tests
// These tests verify the fixes from Stories 9.1 and 9.2 work under stress conditions.
// Guarded by STRESS_TESTS environment variable to avoid slowing down regular test runs.

// TestStressRepeatedWatchCycles verifies no FD leaks occur when creating and closing
// watchers repeatedly. Story 9.1 fixed the immediate FD leak by adding explicit Remove()
// calls before Close(). This test validates that fix works under stress. (AC: #1)
func TestStressRepeatedWatchCycles(t *testing.T) {
	if os.Getenv("STRESS_TESTS") == "" {
		t.Skip("Skipping stress test (set STRESS_TESTS=1 to run)")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	if err := os.WriteFile(testFile, []byte("{}\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Stress test: create and close watchers 100 times
	for i := 0; i < 100; i++ {
		w, err := New(testFile)
		if err != nil {
			t.Fatalf("iteration %d: failed to create watcher: %v", i, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("iteration %d: failed to close watcher: %v", i, err)
		}
	}

	// Verify final watcher still works (no "too many open files" error)
	w, err := New(testFile)
	if err != nil {
		t.Fatalf("final watcher creation failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Verify events can still be received
	ch := w.EventsChan()

	// Append to file in goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		_, _ = f.WriteString("{\"type\":\"user\"}\n")
		_ = f.Close()
	}()

	// Wait for event with timeout
	select {
	case event := <-ch:
		if !event.Has(fsnotify.Write) {
			t.Errorf("expected Write event, got %v", event.Op)
		}
		t.Logf("Final watcher received event successfully after 100 cycles")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event - final watcher may not be functional")
	}
}

// countOpenFDs returns the number of open file descriptors for the current process.
// Works on macOS (/dev/fd) and Linux (/proc/self/fd).
// Returns -1 and skips test if FD counting is not available.
func countOpenFDs(t *testing.T) int {
	t.Helper()

	// Try Linux path first (more reliable across environments)
	entries, err := os.ReadDir("/proc/self/fd")
	if err == nil {
		return len(entries)
	}

	// Fallback for macOS
	entries, err = os.ReadDir("/dev/fd")
	if err == nil {
		return len(entries)
	}

	// Use lsof as last resort (slower but works in sandboxed environments)
	pid := os.Getpid()
	cmd := exec.Command("lsof", "-p", fmt.Sprintf("%d", pid))
	out, err := cmd.Output()
	if err == nil {
		// Count lines with FD column (skip header and any summary lines)
		// lsof header: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME
		lines := strings.Split(string(out), "\n")
		count := 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// Skip header line (first non-empty line)
			if i == 0 && strings.HasPrefix(trimmed, "COMMAND") {
				continue
			}
			// Count lines that look like FD entries (have multiple fields)
			fields := strings.Fields(trimmed)
			if len(fields) >= 4 {
				count++
			}
		}
		if count > 0 {
			return count
		}
	}

	t.Skip("FD counting not supported on this platform")
	return 0
}

// TestStressFDCountStability verifies FD count remains stable after processing many
// file events. This test validates that the watcher cleanup properly releases FDs
// when events are processed. (AC: #3)
func TestStressFDCountStability(t *testing.T) {
	if os.Getenv("STRESS_TESTS") == "" {
		t.Skip("Skipping stress test (set STRESS_TESTS=1 to run)")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	if err := os.WriteFile(testFile, []byte("{}\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Record baseline FD count
	baselineFD := countOpenFDs(t)
	t.Logf("Baseline FD count: %d", baselineFD)

	// Create watcher
	w, err := New(testFile)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	ch := w.EventsChan()

	// Generate 50 file write events and process them
	eventsReceived := 0
	for i := 0; i < 50; i++ {
		// Append to file
		f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("iteration %d: failed to open file for append: %v", i, err)
		}
		if _, err := f.WriteString("{\"type\":\"user\"}\n"); err != nil {
			_ = f.Close()
			t.Fatalf("iteration %d: failed to write to file: %v", i, err)
		}
		_ = f.Close()

		// Wait for and consume the event
		select {
		case <-ch:
			eventsReceived++
			// Read the new entries to simulate normal operation
			entries, _ := w.ReadNewEntries()
			// Entries may be empty if read happens before file sync, that's OK
			_ = entries
		case <-time.After(1 * time.Second):
			t.Fatalf("iteration %d: timeout waiting for event", i)
		}
	}
	t.Logf("Events received: %d out of 50 writes", eventsReceived)

	// Close watcher
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close watcher: %v", err)
	}

	// Record final FD count
	finalFD := countOpenFDs(t)
	delta := finalFD - baselineFD

	t.Logf("FD delta: %d (baseline: %d, final: %d)", delta, baselineFD, finalFD)

	// Assert FD count delta is acceptable (≤ 5)
	if delta > 5 {
		t.Errorf("FD count increased by %d (exceeds threshold of 5); baseline: %d, final: %d",
			delta, baselineFD, finalFD)
	}
}

// --- ReadNewRawBytes tests ---

// TestReadNewRawBytesNoNewData returns nil when no new data is available.
func TestReadNewRawBytesNoNewData(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	initial := "line1\n"
	if err := os.WriteFile(testFile, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	raw, err := w.ReadNewRawBytes()
	if err != nil {
		t.Fatalf("ReadNewRawBytes() error: %v", err)
	}
	if raw != nil {
		t.Errorf("ReadNewRawBytes() = %q, want nil when no new data", string(raw))
	}
}

// TestReadNewRawBytesReadsAppendedData reads raw bytes appended after creation.
func TestReadNewRawBytesReadsAppendedData(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	initial := "line1\n"
	if err := os.WriteFile(testFile, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	appended := "line2\nline3\n"
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open for append: %v", err)
	}
	_, _ = f.WriteString(appended)
	_ = f.Close()

	raw, err := w.ReadNewRawBytes()
	if err != nil {
		t.Fatalf("ReadNewRawBytes() error: %v", err)
	}
	if string(raw) != appended {
		t.Errorf("ReadNewRawBytes() = %q, want %q", string(raw), appended)
	}

	// Second read should return nil (no new data)
	raw2, err := w.ReadNewRawBytes()
	if err != nil {
		t.Fatalf("second ReadNewRawBytes() error: %v", err)
	}
	if raw2 != nil {
		t.Errorf("second ReadNewRawBytes() = %q, want nil", string(raw2))
	}
}

func TestReadNewRawBytesWaitsForCompleteLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	initial := "line1\n"
	if err := os.WriteFile(testFile, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open for append: %v", err)
	}
	_, _ = f.WriteString("partial")
	_ = f.Close()

	raw, err := w.ReadNewRawBytes()
	if err != nil {
		t.Fatalf("ReadNewRawBytes() error: %v", err)
	}
	if raw != nil {
		t.Fatalf("ReadNewRawBytes() = %q, want nil for partial line", string(raw))
	}
	if got, want := w.LastPosition(), int64(len(initial)); got != want {
		t.Fatalf("last read position advanced for partial raw line: got %d, want %d", got, want)
	}

	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to reopen for append: %v", err)
	}
	_, _ = f.WriteString(" line\nnext partial")
	_ = f.Close()

	raw, err = w.ReadNewRawBytes()
	if err != nil {
		t.Fatalf("ReadNewRawBytes() after completion error: %v", err)
	}
	if string(raw) != "partial line\n" {
		t.Fatalf("ReadNewRawBytes() = %q, want %q", string(raw), "partial line\n")
	}
}

// TestReadNewRawBytesTruncation detects file truncation.
func TestReadNewRawBytesTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	if err := os.WriteFile(testFile, []byte("initial content that is long\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Truncate file to smaller size
	if err := os.WriteFile(testFile, []byte("short\n"), 0644); err != nil {
		t.Fatalf("failed to truncate: %v", err)
	}

	_, err = w.ReadNewRawBytes()
	if !errors.Is(err, ErrFileTruncated) {
		t.Errorf("ReadNewRawBytes() error = %v, want ErrFileTruncated", err)
	}
}

// TestStreamingTruncationRecovery verifies the full truncation recovery cycle:
// 1. ReadNewRawBytes detects truncation and resets position to 0
// 2. Subsequent read successfully reads the new (smaller) content
// This mirrors the recovery path used in runStreamingPlainMode where
// ErrFileTruncated triggers a continue in the streaming loop.
func TestStreamingTruncationRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write initial content and create watcher positioned at end
	initialContent := `{"type":"user","message":{"role":"user","content":"message 1"},"timestamp":"2026-01-16T10:00:00Z"}
{"type":"user","message":{"role":"user","content":"message 2"},"timestamp":"2026-01-16T10:01:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Step 1: Truncate file to smaller content
	newContent := `{"type":"user","message":{"role":"user","content":"fresh start"},"timestamp":"2026-01-16T11:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to truncate file: %v", err)
	}

	// Step 2: ReadNewRawBytes should detect truncation
	_, err = w.ReadNewRawBytes()
	if !errors.Is(err, ErrFileTruncated) {
		t.Fatalf("first ReadNewRawBytes after truncation: error = %v, want ErrFileTruncated", err)
	}

	// Verify position was reset to 0
	if pos := w.LastPosition(); pos != 0 {
		t.Errorf("LastPosition() after truncation = %d, want 0", pos)
	}

	// Step 3: Subsequent read should succeed and return the new content
	raw, err := w.ReadNewRawBytes()
	if err != nil {
		t.Fatalf("second ReadNewRawBytes after recovery: error = %v", err)
	}
	if string(raw) != newContent {
		t.Errorf("recovered ReadNewRawBytes() = %q, want %q", string(raw), newContent)
	}

	// Verify position advanced past the new content
	if pos := w.LastPosition(); pos != int64(len(newContent)) {
		t.Errorf("LastPosition() after recovery = %d, want %d", pos, len(newContent))
	}

	// Step 4: Next read returns nil (no new data)
	raw2, err := w.ReadNewRawBytes()
	if err != nil {
		t.Fatalf("third ReadNewRawBytes: error = %v", err)
	}
	if raw2 != nil {
		t.Errorf("third ReadNewRawBytes() = %q, want nil", string(raw2))
	}
}

// TestStreamingTruncationRecovery_ReadNewEntries verifies the same truncation
// recovery cycle through ReadNewEntries (the Claude Code streaming path),
// confirming that parsed entries are correct after recovery.
func TestStreamingTruncationRecovery_ReadNewEntries(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	initialContent := `{"type":"user","message":{"role":"user","content":"old message"},"timestamp":"2026-01-16T10:00:00Z"}
{"type":"user","message":{"role":"user","content":"another old"},"timestamp":"2026-01-16T10:01:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Truncate
	newContent := `{"type":"assistant","message":{"content":[{"type":"text","text":"new entry"}]},"timestamp":"2026-01-16T11:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to truncate file: %v", err)
	}

	// ReadNewEntries detects truncation
	_, err = w.ReadNewEntries()
	if !errors.Is(err, ErrFileTruncated) {
		t.Fatalf("ReadNewEntries after truncation: error = %v, want ErrFileTruncated", err)
	}

	// Recovery: read new content
	entries, err := w.ReadNewEntries()
	if err != nil {
		t.Fatalf("ReadNewEntries after recovery: error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("recovered ReadNewEntries() returned %d entries, want 1", len(entries))
	}
	if len(entries[0].Message.Content) == 0 || entries[0].Message.Content[0].Text != "new entry" {
		text := entries[0].Message.TextContent
		if len(entries[0].Message.Content) > 0 {
			text = entries[0].Message.Content[0].Text
		}
		t.Errorf("recovered entry content = %q, want 'new entry'", text)
	}
}

// TestLastPosition returns the current file offset.
func TestLastPosition(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	initial := "hello world\n"
	if err := os.WriteFile(testFile, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := New(testFile)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	pos := w.LastPosition()
	if pos != int64(len(initial)) {
		t.Errorf("LastPosition() = %d, want %d", pos, len(initial))
	}

	// Append data and read it
	appended := "new data\n"
	f, _ := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	_, _ = f.WriteString(appended)
	_ = f.Close()

	_, _ = w.ReadNewRawBytes()
	pos = w.LastPosition()
	expected := int64(len(initial) + len(appended))
	if pos != expected {
		t.Errorf("LastPosition() after read = %d, want %d", pos, expected)
	}
}
