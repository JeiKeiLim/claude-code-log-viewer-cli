package watcher

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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
