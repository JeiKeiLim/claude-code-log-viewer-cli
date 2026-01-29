// Package watcher provides file watching capabilities for live log updates.
package watcher

import (
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
)

// ProjectWatcher monitors a project directory for new conversation files (Story 11.2).
// Unlike Watcher which monitors a single file for changes, ProjectWatcher
// watches a directory for CREATE events on new .jsonl files.
type ProjectWatcher struct {
	projectPath string
	fsWatcher   *fsnotify.Watcher
	mu          sync.Mutex
	closed      bool
}

// NewProjectWatcher creates a new directory watcher for the given project path.
// It monitors the directory for new .jsonl file creation events.
func NewProjectWatcher(projectPath string) (*ProjectWatcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fsw.Add(projectPath); err != nil {
		_ = fsw.Close()
		return nil, err
	}
	return &ProjectWatcher{
		projectPath: projectPath,
		fsWatcher:   fsw,
	}, nil
}

// WaitForNewConversation returns a tea.Cmd that blocks until a new .jsonl file is created.
// Returns NewConversationMsg with the file path and creation time (birthtime).
func (w *ProjectWatcher) WaitForNewConversation() tea.Cmd {
	return func() tea.Msg {
		for {
			select {
			case event, ok := <-w.fsWatcher.Events:
				if !ok {
					return nil // Watcher closed
				}
				// Only handle CREATE events for .jsonl files
				if event.Has(fsnotify.Create) && strings.HasSuffix(event.Name, ".jsonl") {
					// Get file info for birthtime
					info, err := os.Stat(event.Name)
					if err != nil {
						// Skip this event, continue watching
						continue
					}

					// Get birthtime, fall back to ModTime if zero
					creationTime := scanner.GetBirthtime(info)
					if creationTime.IsZero() {
						creationTime = info.ModTime()
					}

					return NewConversationMsg{
						FilePath:     event.Name,
						CreationTime: creationTime,
					}
				}
				// Non-create event or non-jsonl file, continue loop
			case _, ok := <-w.fsWatcher.Errors:
				if !ok {
					return nil
				}
				// On error, continue watching (graceful degradation)
			}
		}
	}
}

// Close releases all resources associated with the watcher.
// Close is idempotent - it can be called multiple times safely.
// CRITICAL: On macOS, fsnotify uses kqueue which opens a file descriptor for EACH
// watched path. Calling Close() only closes the kqueue FD itself, not the individual
// watch FDs. We must call Remove() on each watched path before Close() to properly
// release all file descriptors. (Story 9.1 macOS kqueue fix)
func (w *ProjectWatcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil // Idempotent
	}
	w.closed = true

	// Remove all watched paths before closing (macOS kqueue FD fix)
	if w.fsWatcher != nil {
		for _, path := range w.fsWatcher.WatchList() {
			_ = w.fsWatcher.Remove(path) // Ignore errors - path may be deleted
		}
		return w.fsWatcher.Close()
	}
	return nil
}

// IsClosed returns whether the watcher has been closed.
func (w *ProjectWatcher) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}
