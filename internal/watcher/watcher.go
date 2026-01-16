// Package watcher provides file watching capabilities for live log updates.
package watcher

import (
	"errors"
	"io"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// ErrFileTruncated is returned when the watched file is truncated.
var ErrFileTruncated = errors.New("file was truncated")

// Watcher monitors a log file for changes and sends new entries to Bubbletea.
type Watcher struct {
	filePath    string
	fsWatcher   *fsnotify.Watcher
	lastReadPos int64
	mu          sync.Mutex // Protects lastReadPos and closed
	closed      bool
}

// New creates a new file watcher for the given file path.
// The watcher starts at the current end of file (skips existing content).
func New(filePath string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fsw.Add(filePath); err != nil {
		_ = fsw.Close()
		return nil, err
	}
	// Initialize lastReadPos to current file size (skip existing content)
	stat, err := os.Stat(filePath)
	if err != nil {
		_ = fsw.Close()
		return nil, err
	}
	return &Watcher{
		filePath:    filePath,
		fsWatcher:   fsw,
		lastReadPos: stat.Size(),
	}, nil
}

// WaitForEvent returns a tea.Cmd that blocks until a file event occurs.
// Call this in a chain: Update() handles message, returns WaitForEvent() for next event.
func (w *Watcher) WaitForEvent() tea.Cmd {
	return func() tea.Msg {
		// Use loop instead of recursion to avoid stack overflow on many non-write events
		for {
			select {
			case event, ok := <-w.fsWatcher.Events:
				if !ok {
					return nil // Watcher closed
				}
				if event.Has(fsnotify.Write) {
					entries, err := w.readNewEntries()
					if errors.Is(err, ErrFileTruncated) {
						return FileResetMsg{}
					}
					if err != nil {
						return WatcherErrorMsg{Err: err}
					}
					if len(entries) > 0 {
						return NewEntriesMsg{Entries: entries}
					}
				}
				// Non-write event (chmod, rename, etc.), continue loop
			case err, ok := <-w.fsWatcher.Errors:
				if !ok {
					return nil
				}
				return WatcherErrorMsg{Err: err}
			}
		}
	}
}

// readNewEntries reads and parses any new content appended to the file.
func (w *Watcher) readNewEntries() ([]types.LogEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	file, err := os.Open(w.filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	// Check for truncation
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() < w.lastReadPos {
		w.lastReadPos = 0
		return nil, ErrFileTruncated
	}

	// No new content
	if stat.Size() == w.lastReadPos {
		return nil, nil
	}

	// Seek to last position
	if _, err := file.Seek(w.lastReadPos, io.SeekStart); err != nil {
		return nil, err
	}

	// Parse new content
	result := parser.ParseJSONL(file)

	// Update position
	newPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	w.lastReadPos = newPos

	return result.Entries, nil
}

// Close releases all resources associated with the watcher.
// Close is idempotent - it can be called multiple times safely.
func (w *Watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil // Idempotent
	}
	w.closed = true
	return w.fsWatcher.Close()
}

// IsClosed returns whether the watcher has been closed.
func (w *Watcher) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}
