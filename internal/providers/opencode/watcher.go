package opencode

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

// OpenCodeWatcher watches an OpenCode session for new messages by polling the
// SQLite database.
type OpenCodeWatcher struct {
	db        *sql.DB
	sessionID string
	lastTime  int64 // raw DB time_created value of the last-seen complete message

	mu      sync.Mutex
	closed  bool
	ticker  *time.Ticker
	done    chan struct{}
	pending []agent.ConversationEntry
	emitted map[string]struct{}
}

type dbClosingWatcher struct {
	watcher *OpenCodeWatcher
	closeDB func() error
}

func (w *dbClosingWatcher) NewEntries() ([]agent.ConversationEntry, error) {
	return w.watcher.NewEntries()
}

func (w *dbClosingWatcher) Close() error {
	watcherErr := w.watcher.Close()
	dbErr := w.closeDB()
	if watcherErr != nil {
		return watcherErr
	}
	return dbErr
}

// NewOpenCodeWatcher creates a watcher that polls for new messages in the
// given session. It starts a background goroutine that queries the database
// every 100ms.
func NewOpenCodeWatcher(db *sql.DB, sessionID string) (*OpenCodeWatcher, error) {
	// Determine the latest message timestamp so we only report new messages.
	var lastTime int64
	err := db.QueryRow(`
		SELECT COALESCE(MAX(time_created), 0) FROM message WHERE session_id = ?
	`, sessionID).Scan(&lastTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get last message time: %w", err)
	}

	w := &OpenCodeWatcher{
		db:        db,
		sessionID: sessionID,
		lastTime:  lastTime,
		ticker:    time.NewTicker(100 * time.Millisecond),
		done:      make(chan struct{}),
		emitted:   make(map[string]struct{}),
	}

	go w.poll()

	return w, nil
}

// poll runs in the background, periodically checking for new messages.
func (w *OpenCodeWatcher) poll() {
	for {
		select {
		case <-w.done:
			return
		case <-w.ticker.C:
			w.checkNewMessages()
		}
	}
}

// checkNewMessages queries for messages newer than lastTime and appends them
// to the pending buffer.
func (w *OpenCodeWatcher) checkNewMessages() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return
	}

	entries, maxTimeCreated, hasPending, err := parseSessionFromDBSinceWithSeen(w.db, w.sessionID, w.lastTime, w.emitted)
	if err != nil {
		return
	}

	if len(entries) > 0 {
		w.pending = append(w.pending, entries...)
	}
	if !hasPending && maxTimeCreated > 0 {
		w.lastTime = maxTimeCreated
	}
}

// NewEntries returns any entries appended since the last call.
func (w *OpenCodeWatcher) NewEntries() ([]agent.ConversationEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, fmt.Errorf("opencode watcher: already closed")
	}

	entries := w.pending
	w.pending = nil
	return entries, nil
}

// Close stops the polling and releases resources. It does not close the
// database connection (the caller owns that).
func (w *OpenCodeWatcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true
	w.ticker.Stop()
	close(w.done)
	return nil
}

func parseSessionFromDBSince(db *sql.DB, sessionID string, sinceTime int64) ([]agent.ConversationEntry, int64, error) {
	entries, maxTimeCreated, _, err := parseSessionFromDBSinceWithSeen(db, sessionID, sinceTime, nil)
	return entries, maxTimeCreated, err
}

// parseSessionFromDBSinceWithSeen queries messages created after the given raw
// DB timestamp and converts complete messages into ConversationEntry values.
// Messages without content are treated as pending so the watcher can retry
// after OpenCode inserts their part rows.
func parseSessionFromDBSinceWithSeen(db *sql.DB, sessionID string, sinceTime int64, emitted map[string]struct{}) ([]agent.ConversationEntry, int64, bool, error) {
	// Collect all messages first, then close rows before querying parts.
	type msgRow struct {
		id          string
		dataJSON    string
		timeCreated int64
	}

	rows, err := db.Query(`
		SELECT id, data, time_created
		FROM message
		WHERE session_id = ? AND time_created > ?
		ORDER BY time_created ASC
	`, sessionID, sinceTime)
	if err != nil {
		return nil, 0, false, fmt.Errorf("failed to query new messages: %w", err)
	}

	var msgRows []msgRow
	for rows.Next() {
		var mr msgRow
		if err := rows.Scan(&mr.id, &mr.dataJSON, &mr.timeCreated); err != nil {
			rows.Close()
			return nil, 0, false, fmt.Errorf("failed to scan message row: %w", err)
		}
		msgRows = append(msgRows, mr)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, false, fmt.Errorf("error iterating message rows: %w", err)
	}
	rows.Close()

	var entries []agent.ConversationEntry
	var maxTimeCreated int64
	hasPending := false

	for _, mr := range msgRows {
		if emitted != nil {
			if _, ok := emitted[mr.id]; ok {
				if mr.timeCreated > maxTimeCreated {
					maxTimeCreated = mr.timeCreated
				}
				continue
			}
		}

		entry, ok, pending, err := buildEntryForMessage(db, sessionID, mr.id, mr.dataJSON, mr.timeCreated)
		if err != nil {
			return nil, 0, false, err
		}
		if pending {
			hasPending = true
			continue
		}
		if !ok {
			if mr.timeCreated > maxTimeCreated {
				maxTimeCreated = mr.timeCreated
			}
			continue
		}

		if emitted != nil {
			emitted[mr.id] = struct{}{}
		}
		if mr.timeCreated > maxTimeCreated {
			maxTimeCreated = mr.timeCreated
		}
		entries = append(entries, entry)
	}

	return entries, maxTimeCreated, hasPending, nil
}
