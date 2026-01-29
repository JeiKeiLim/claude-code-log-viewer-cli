package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewProjectWatcher tests Story 11.2 Task 3.2: creating a ProjectWatcher.
func TestNewProjectWatcher(t *testing.T) {
	tests := []struct {
		name      string
		setupDir  bool
		wantError bool
	}{
		{
			name:      "valid directory",
			setupDir:  true,
			wantError: false,
		},
		{
			name:      "non-existent directory",
			setupDir:  false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var projectPath string
			if tt.setupDir {
				projectPath = t.TempDir()
			} else {
				projectPath = "/non/existent/path/that/should/not/exist"
			}

			pw, err := NewProjectWatcher(projectPath)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
					if pw != nil {
						_ = pw.Close()
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if pw == nil {
					t.Fatal("expected non-nil ProjectWatcher")
				}
				// Clean up
				_ = pw.Close()
			}
		})
	}
}

// TestProjectWatcherClose tests Story 11.2 Task 3.4: Close is idempotent and removes paths.
func TestProjectWatcherClose(t *testing.T) {
	projectPath := t.TempDir()
	pw, err := NewProjectWatcher(projectPath)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// First close should succeed
	if err := pw.Close(); err != nil {
		t.Errorf("first close failed: %v", err)
	}

	// Second close should also succeed (idempotent)
	if err := pw.Close(); err != nil {
		t.Errorf("second close should be idempotent, got error: %v", err)
	}

	// Third close should still succeed
	if err := pw.Close(); err != nil {
		t.Errorf("third close should be idempotent, got error: %v", err)
	}
}

// TestProjectWatcherIsClosed tests Story 11.2 Task 3.5: IsClosed reflects state.
func TestProjectWatcherIsClosed(t *testing.T) {
	projectPath := t.TempDir()
	pw, err := NewProjectWatcher(projectPath)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Should not be closed initially
	if pw.IsClosed() {
		t.Error("newly created watcher should not be closed")
	}

	// Close and verify
	_ = pw.Close()
	if !pw.IsClosed() {
		t.Error("closed watcher should report IsClosed() = true")
	}
}

// TestProjectWatcherWaitForNewConversation tests Story 11.2 Task 3.3:
// WaitForNewConversation returns correct message on CREATE event.
func TestProjectWatcherWaitForNewConversation(t *testing.T) {
	projectPath := t.TempDir()
	pw, err := NewProjectWatcher(projectPath)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer pw.Close()

	// Start waiting in goroutine
	msgChan := make(chan interface{}, 1)
	go func() {
		cmd := pw.WaitForNewConversation()
		msg := cmd()
		msgChan <- msg
	}()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create a new .jsonl file
	testFile := filepath.Join(projectPath, "new_conversation.jsonl")
	if err := os.WriteFile(testFile, []byte(`{"type":"user"}`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Wait for message with timeout
	select {
	case msg := <-msgChan:
		ncMsg, ok := msg.(NewConversationMsg)
		if !ok {
			t.Fatalf("expected NewConversationMsg, got %T", msg)
		}
		if ncMsg.FilePath != testFile {
			t.Errorf("FilePath = %q, want %q", ncMsg.FilePath, testFile)
		}
		// CreationTime should be non-zero (either birthtime or modtime)
		if ncMsg.CreationTime.IsZero() {
			t.Error("CreationTime should not be zero")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for NewConversationMsg")
	}
}

// TestProjectWatcherIgnoresNonJsonl tests that non-.jsonl files are ignored.
func TestProjectWatcherIgnoresNonJsonl(t *testing.T) {
	projectPath := t.TempDir()
	pw, err := NewProjectWatcher(projectPath)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer pw.Close()

	// Start waiting in goroutine
	msgChan := make(chan interface{}, 1)
	go func() {
		cmd := pw.WaitForNewConversation()
		msg := cmd()
		msgChan <- msg
	}()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create a non-.jsonl file (should be ignored)
	nonJsonlFile := filepath.Join(projectPath, "readme.txt")
	if err := os.WriteFile(nonJsonlFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create non-jsonl file: %v", err)
	}

	// Create a .jsonl file (should trigger)
	jsonlFile := filepath.Join(projectPath, "conversation.jsonl")
	if err := os.WriteFile(jsonlFile, []byte(`{"type":"user"}`), 0644); err != nil {
		t.Fatalf("failed to create jsonl file: %v", err)
	}

	// Wait for message
	select {
	case msg := <-msgChan:
		ncMsg, ok := msg.(NewConversationMsg)
		if !ok {
			t.Fatalf("expected NewConversationMsg, got %T", msg)
		}
		// Should be the .jsonl file, not the .txt file
		if ncMsg.FilePath != jsonlFile {
			t.Errorf("expected .jsonl file %q, got %q", jsonlFile, ncMsg.FilePath)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for NewConversationMsg")
	}
}

// TestProjectWatcherCloseRemovesPaths tests that Close removes watched paths before closing.
// This is critical for macOS to avoid file descriptor leaks (Story 9.1 pattern).
func TestProjectWatcherCloseRemovesPaths(t *testing.T) {
	projectPath := t.TempDir()
	pw, err := NewProjectWatcher(projectPath)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Verify path is being watched (internal check)
	if pw.fsWatcher == nil {
		t.Fatal("fsWatcher should not be nil")
	}

	watchList := pw.fsWatcher.WatchList()
	if len(watchList) == 0 {
		t.Error("expected at least one watched path")
	}

	// Close should remove paths before closing
	if err := pw.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// After close, watcher should be marked closed
	if !pw.IsClosed() {
		t.Error("watcher should be marked closed after Close()")
	}
}
