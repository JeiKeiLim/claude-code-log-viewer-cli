package opencode

import (
	"database/sql"
	"encoding/json"
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
	lastTime  int64 // unix epoch of the last-seen message

	mu      sync.Mutex
	closed  bool
	ticker  *time.Ticker
	done    chan struct{}
	pending []agent.ConversationEntry
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

	entries, err := parseSessionFromDBSince(w.db, w.sessionID, w.lastTime)
	if err != nil {
		return
	}

	if len(entries) > 0 {
		w.pending = append(w.pending, entries...)
		// Update lastTime to the latest entry's timestamp.
		last := entries[len(entries)-1]
		if !last.Timestamp().IsZero() {
			w.lastTime = last.Timestamp().Unix()
		}
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

// parseSessionFromDBSince queries messages created after the given unix epoch
// timestamp and converts them into ConversationEntry values.
func parseSessionFromDBSince(db *sql.DB, sessionID string, sinceTime int64) ([]agent.ConversationEntry, error) {
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
		return nil, fmt.Errorf("failed to query new messages: %w", err)
	}

	var msgRows []msgRow
	for rows.Next() {
		var mr msgRow
		if err := rows.Scan(&mr.id, &mr.dataJSON, &mr.timeCreated); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		msgRows = append(msgRows, mr)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}
	rows.Close()

	var entries []agent.ConversationEntry

	for _, mr := range msgRows {
		var mData messageData
		if err := json.Unmarshal([]byte(mr.dataJSON), &mData); err != nil {
			continue
		}

		var eType agent.EntryType
		switch mData.Role {
		case "user":
			eType = agent.EntryTypeUser
		case "assistant":
			eType = agent.EntryTypeAssistant
		default:
			continue
		}

		ts := time.Unix(mr.timeCreated, 0)
		if mData.Timestamp != "" {
			if parsed, err := time.Parse(time.RFC3339, mData.Timestamp); err == nil {
				ts = parsed
			}
		}

		blocks, err := queryPartsForMessage(db, mr.id)
		if err != nil {
			return nil, fmt.Errorf("failed to query parts for message %s: %w", mr.id, err)
		}

		if len(blocks) == 0 && mData.Content != "" {
			blocks = append(blocks, agent.BasicBlock{
				BlockType: agent.ContentBlockText,
				BlockText: mData.Content,
			})
		}

		entries = append(entries, agent.BasicEntry{
			EntryType:      eType,
			EntryTimestamp: ts,
			EntryRole:      mData.Role,
			Blocks:         blocks,
			EntryTokens:    agent.TokenStats{},
			EntryAgent:     agent.AgentOpenCode,
			EntrySession:   sessionID,
		})
	}

	return entries, nil
}
