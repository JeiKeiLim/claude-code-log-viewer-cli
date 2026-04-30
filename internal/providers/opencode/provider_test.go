package opencode

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

// --- Compile-time interface checks ---

func TestProviderImplementsAgentProvider(t *testing.T) {
	var _ agent.AgentProvider = (*Provider)(nil)
}

func TestOpenCodeWatcherImplementsSessionWatcher(t *testing.T) {
	var _ agent.SessionWatcher = (*OpenCodeWatcher)(nil)
}

// --- Test helpers ---

// setupTestDB creates an in-memory SQLite database with the OpenCode schema
// and returns the *sql.DB. The caller should defer db.Close().
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// Use file::memory: with shared cache so all connections see the same data.
	// This is needed because database/sql may use multiple connections internally,
	// and plain :memory: creates a separate database per connection.
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

	// Create schema — execute each statement separately since
	// modernc.org/sqlite may not support multiple statements in one Exec.
	statements := []string{
		`CREATE TABLE IF NOT EXISTS session (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			title TEXT,
			directory TEXT,
			time_created INTEGER,
			time_updated INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS message (
			id TEXT PRIMARY KEY,
			session_id TEXT,
			data TEXT,
			time_created INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS part (
			id TEXT PRIMARY KEY,
			message_id TEXT,
			data TEXT
		)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			t.Fatalf("failed to create schema (%s): %v", stmt, err)
		}
	}

	return db
}

// insertTestSession inserts a session row into the test database.
func insertTestSession(db *sql.DB, id, projectID, title, directory string, created, updated int64) error {
	_, err := db.Exec(
		`INSERT INTO session (id, project_id, title, directory, time_created, time_updated) VALUES (?, ?, ?, ?, ?, ?)`,
		id, projectID, title, directory, created, updated,
	)
	return err
}

// insertTestMessage inserts a message row into the test database.
func insertTestMessage(db *sql.DB, id, sessionID, data string, timeCreated int64) error {
	_, err := db.Exec(
		`INSERT INTO message (id, session_id, data, time_created) VALUES (?, ?, ?, ?)`,
		id, sessionID, data, timeCreated,
	)
	return err
}

// insertTestPart inserts a part row into the test database.
func insertTestPart(db *sql.DB, id, messageID, data string) error {
	_, err := db.Exec(
		`INSERT INTO part (id, message_id, data) VALUES (?, ?, ?)`,
		id, messageID, data,
	)
	return err
}

// --- Tests ---

func TestProviderType(t *testing.T) {
	p := NewProvider()
	if p.Type() != agent.AgentOpenCode {
		t.Errorf("Type() = %q, want %q", p.Type(), agent.AgentOpenCode)
	}
}

func TestProviderDisplayName(t *testing.T) {
	p := NewProvider()
	if p.DisplayName() != "OpenCode" {
		t.Errorf("DisplayName() = %q, want %q", p.DisplayName(), "OpenCode")
	}
}

func TestProviderBadge(t *testing.T) {
	p := NewProvider()
	if p.Badge() != "[O]" {
		t.Errorf("Badge() = %q, want %q", p.Badge(), "[O]")
	}
}

func TestProviderIsAvailableNonexistentDB(t *testing.T) {
	p := NewProvider(WithDBPath("/nonexistent/path/opencode.db"))
	if p.IsAvailable() {
		t.Error("IsAvailable() = true for nonexistent database, want false")
	}
}

func TestProviderParseSessionStreamNotSupported(t *testing.T) {
	p := NewProvider()
	_, err := p.ParseSessionStream(nil)
	if err == nil {
		t.Error("ParseSessionStream() should return error for SQLite provider")
	}
}

func TestQueryProjects(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	// Insert sessions in two different directories.
	if err := insertTestSession(db, "s1", "p1", "Session 1", "/home/user/project-a", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := insertTestSession(db, "s2", "p1", "Session 2", "/home/user/project-a", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := insertTestSession(db, "s3", "p2", "Session 3", "/home/user/project-b", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	projects, err := queryProjects(db)
	if err != nil {
		t.Fatalf("queryProjects() error: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2", len(projects))
	}

	// Check first project.
	if projects[0].Path != "/home/user/project-a" {
		t.Errorf("projects[0].Path = %q, want %q", projects[0].Path, "/home/user/project-a")
	}
	if projects[0].DisplayName != "project-a" {
		t.Errorf("projects[0].DisplayName = %q, want %q", projects[0].DisplayName, "project-a")
	}
	if projects[0].AgentType != agent.AgentOpenCode {
		t.Errorf("projects[0].AgentType = %q, want %q", projects[0].AgentType, agent.AgentOpenCode)
	}
	if projects[0].SessionCount != 2 {
		t.Errorf("projects[0].SessionCount = %d, want 2", projects[0].SessionCount)
	}

	// Check second project.
	if projects[1].Path != "/home/user/project-b" {
		t.Errorf("projects[1].Path = %q, want %q", projects[1].Path, "/home/user/project-b")
	}
	if projects[1].SessionCount != 1 {
		t.Errorf("projects[1].SessionCount = %d, want 1", projects[1].SessionCount)
	}
}

func TestQueryProjectsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	projects, err := queryProjects(db)
	if err != nil {
		t.Fatalf("queryProjects() error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("len(projects) = %d, want 0 for empty database", len(projects))
	}
}

func TestQuerySessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "First", "/proj", now-100, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := insertTestSession(db, "s2", "p1", "Second", "/proj", now-50, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	// Session in a different project — should not appear.
	if err := insertTestSession(db, "s3", "p2", "Other", "/other", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	sessions, err := querySessions(db, "/proj")
	if err != nil {
		t.Fatalf("querySessions() error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}

	// Sessions are ordered by time_created DESC, so s2 comes first.
	if sessions[0].ID != "s2" {
		t.Errorf("sessions[0].ID = %q, want %q", sessions[0].ID, "s2")
	}
	if sessions[1].ID != "s1" {
		t.Errorf("sessions[1].ID = %q, want %q", sessions[1].ID, "s1")
	}

	// Check FilePath is set to session ID.
	if sessions[0].FilePath != "s2" {
		t.Errorf("sessions[0].FilePath = %q, want %q", sessions[0].FilePath, "s2")
	}
}

func TestEnrichSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test Session", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Insert messages.
	userData := `{"role":"user","content":"Hello, fix this bug"}`
	if err := insertTestMessage(db, "m1", "s1", userData, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	asstData := `{"role":"assistant","modelID":"claude-3.5-sonnet","content":"I'll help you fix that"}`
	if err := insertTestMessage(db, "m2", "s1", asstData, now+1); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	sessions := []agent.Session{
		{ID: "s1", ProjectPath: "/proj", FilePath: "s1", AgentType: agent.AgentOpenCode},
	}

	if err := enrichSessions(db, sessions); err != nil {
		t.Fatalf("enrichSessions() error: %v", err)
	}

	if sessions[0].MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", sessions[0].MessageCount)
	}
	if sessions[0].FirstUserMessage != "Hello, fix this bug" {
		t.Errorf("FirstUserMessage = %q, want %q", sessions[0].FirstUserMessage, "Hello, fix this bug")
	}
	if sessions[0].Model != "claude-3.5-sonnet" {
		t.Errorf("Model = %q, want %q", sessions[0].Model, "claude-3.5-sonnet")
	}
	if sessions[0].TurnCount != 1 {
		t.Errorf("TurnCount = %d, want 1", sessions[0].TurnCount)
	}
}

func TestParseSessionFromDB(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// User message with text part.
	userData := `{"role":"user","content":"What does this code do?"}`
	if err := insertTestMessage(db, "m1", "s1", userData, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if err := insertTestPart(db, "p1", "m1", `{"type":"text","text":"What does this code do?"}`); err != nil {
		t.Fatalf("insert part: %v", err)
	}

	// Assistant message with reasoning + text + tool parts.
	asstData := `{"role":"assistant","modelID":"claude-3.5-sonnet","timestamp":"2026-04-30T12:00:00Z","content":""}`
	if err := insertTestMessage(db, "m2", "s1", asstData, now+1); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if err := insertTestPart(db, "p2", "m2", `{"type":"reasoning","text":"Let me analyze the code first."}`); err != nil {
		t.Fatalf("insert part: %v", err)
	}
	if err := insertTestPart(db, "p3", "m2", `{"type":"text","text":"This code implements a parser."}`); err != nil {
		t.Fatalf("insert part: %v", err)
	}
	if err := insertTestPart(db, "p4", "m2", `{"type":"tool","toolName":"bash","toolCallID":"tc1","state":"completed","output":"PASS"}`); err != nil {
		t.Fatalf("insert part: %v", err)
	}

	entries, err := parseSessionFromDB(db, "s1")
	if err != nil {
		t.Fatalf("parseSessionFromDB() error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	// Check user entry.
	user := entries[0]
	if user.Type() != agent.EntryTypeUser {
		t.Errorf("user Type() = %q, want %q", user.Type(), agent.EntryTypeUser)
	}
	if user.Role() != "user" {
		t.Errorf("user Role() = %q, want %q", user.Role(), "user")
	}
	if user.AgentType() != agent.AgentOpenCode {
		t.Errorf("user AgentType() = %q, want %q", user.AgentType(), agent.AgentOpenCode)
	}
	if user.SessionID() != "s1" {
		t.Errorf("user SessionID() = %q, want %q", user.SessionID(), "s1")
	}
	if len(user.ContentBlocks()) != 1 {
		t.Fatalf("user ContentBlocks() len = %d, want 1", len(user.ContentBlocks()))
	}
	if user.ContentBlocks()[0].ContentType() != agent.ContentBlockText {
		t.Errorf("user block[0] ContentType() = %q, want %q", user.ContentBlocks()[0].ContentType(), agent.ContentBlockText)
	}
	if user.ContentBlocks()[0].Text() != "What does this code do?" {
		t.Errorf("user block[0] Text() = %q, want %q", user.ContentBlocks()[0].Text(), "What does this code do?")
	}

	// Check assistant entry.
	asst := entries[1]
	if asst.Type() != agent.EntryTypeAssistant {
		t.Errorf("assistant Type() = %q, want %q", asst.Type(), agent.EntryTypeAssistant)
	}
	blocks := asst.ContentBlocks()
	if len(blocks) != 3 {
		t.Fatalf("assistant ContentBlocks() len = %d, want 3", len(blocks))
	}

	// Reasoning block.
	if blocks[0].ContentType() != agent.ContentBlockReasoning {
		t.Errorf("block[0] ContentType() = %q, want %q", blocks[0].ContentType(), agent.ContentBlockReasoning)
	}
	if blocks[0].Text() != "Let me analyze the code first." {
		t.Errorf("block[0] Text() = %q, want reasoning text", blocks[0].Text())
	}

	// Text block.
	if blocks[1].ContentType() != agent.ContentBlockText {
		t.Errorf("block[1] ContentType() = %q, want %q", blocks[1].ContentType(), agent.ContentBlockText)
	}
	if blocks[1].Text() != "This code implements a parser." {
		t.Errorf("block[1] Text() = %q, want %q", blocks[1].Text(), "This code implements a parser.")
	}

	// Tool block.
	if blocks[2].ContentType() != agent.ContentBlockToolUse {
		t.Errorf("block[2] ContentType() = %q, want %q", blocks[2].ContentType(), agent.ContentBlockToolUse)
	}
	if blocks[2].ToolName() != "bash" {
		t.Errorf("block[2] ToolName() = %q, want %q", blocks[2].ToolName(), "bash")
	}
	if blocks[2].ToolOutput() != "PASS" {
		t.Errorf("block[2] ToolOutput() = %q, want %q", blocks[2].ToolOutput(), "PASS")
	}
}

func TestParseSessionFromDBEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	entries, err := parseSessionFromDB(db, "nonexistent-session")
	if err != nil {
		t.Fatalf("parseSessionFromDB() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0 for nonexistent session", len(entries))
	}
}

func TestParseSessionFromDBMalformedMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Valid user message.
	if err := insertTestMessage(db, "m1", "s1", `{"role":"user","content":"hello"}`, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	// Malformed message (invalid JSON).
	if err := insertTestMessage(db, "m2", "s1", `{invalid json`, now+1); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	// Another valid message.
	if err := insertTestMessage(db, "m3", "s1", `{"role":"assistant","content":"response"}`, now+2); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	entries, err := parseSessionFromDB(db, "s1")
	if err != nil {
		t.Fatalf("parseSessionFromDB() error: %v", err)
	}

	// Should have 2 entries (malformed one skipped).
	if len(entries) != 2 {
		t.Errorf("len(entries) = %d, want 2 (malformed skipped)", len(entries))
	}
}

func TestParseSessionFromDBContentFallback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Message with no parts — content should become a text block.
	if err := insertTestMessage(db, "m1", "s1", `{"role":"user","content":"fallback text"}`, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	entries, err := parseSessionFromDB(db, "s1")
	if err != nil {
		t.Fatalf("parseSessionFromDB() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	blocks := entries[0].ContentBlocks()
	if len(blocks) != 1 {
		t.Fatalf("ContentBlocks() len = %d, want 1", len(blocks))
	}
	if blocks[0].Text() != "fallback text" {
		t.Errorf("block Text() = %q, want %q", blocks[0].Text(), "fallback text")
	}
}

func TestParseSessionFromDBStepStartSkipped(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	if err := insertTestMessage(db, "m1", "s1", `{"role":"assistant","content":""}`, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	// step-start should be ignored.
	if err := insertTestPart(db, "p1", "m1", `{"type":"step-start"}`); err != nil {
		t.Fatalf("insert part: %v", err)
	}
	// Actual text part.
	if err := insertTestPart(db, "p2", "m1", `{"type":"text","text":"real content"}`); err != nil {
		t.Fatalf("insert part: %v", err)
	}

	entries, err := parseSessionFromDB(db, "s1")
	if err != nil {
		t.Fatalf("parseSessionFromDB() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	blocks := entries[0].ContentBlocks()
	if len(blocks) != 1 {
		t.Fatalf("ContentBlocks() len = %d, want 1 (step-start skipped)", len(blocks))
	}
	if blocks[0].Text() != "real content" {
		t.Errorf("block Text() = %q, want %q", blocks[0].Text(), "real content")
	}
}

func TestWatcherNewEntries(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Insert an initial message before creating the watcher.
	if err := insertTestMessage(db, "m1", "s1", `{"role":"user","content":"initial"}`, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	w, err := NewOpenCodeWatcher(db, "s1")
	if err != nil {
		t.Fatalf("NewOpenCodeWatcher() error: %v", err)
	}
	defer w.Close()

	// No new entries yet (the initial message was before the watcher started).
	entries, err := w.NewEntries()
	if err != nil {
		t.Fatalf("NewEntries() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0 initially", len(entries))
	}

	// Insert a new message.
	if err := insertTestMessage(db, "m2", "s1", `{"role":"assistant","content":"response"}`, now+1); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	// Give the watcher time to poll.
	time.Sleep(200 * time.Millisecond)

	entries, err = w.NewEntries()
	if err != nil {
		t.Fatalf("NewEntries() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1 after new message", len(entries))
	}
	if entries[0].Role() != "assistant" {
		t.Errorf("entry Role() = %q, want %q", entries[0].Role(), "assistant")
	}
}

func TestWatcherClose(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	w, err := NewOpenCodeWatcher(db, "s1")
	if err != nil {
		t.Fatalf("NewOpenCodeWatcher() error: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// NewEntries should return error after close.
	_, err = w.NewEntries()
	if err == nil {
		t.Error("NewEntries() after Close() should return error")
	}

	// Double close should be fine.
	if err := w.Close(); err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		homeChar bool // whether the result should contain the home directory
	}{
		{"tilde_prefix", "~/some/path", true},
		{"no_tilde", "/absolute/path", false},
		{"empty_string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandHome(tt.input)
			if tt.homeChar {
				// Result should start with an absolute path (not ~).
				if len(result) > 0 && result[0] == '~' {
					t.Errorf("expandHome(%q) = %q, should have expanded ~", tt.input, result)
				}
			}
		})
	}
}

func TestDBExists(t *testing.T) {
	if dbExists("/nonexistent/path/to/db") {
		t.Error("dbExists() = true for nonexistent path")
	}

	// Create a temp file to verify dbExists returns true for an existing file.
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	f, err := createEmptyFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if !dbExists(tmpFile) {
		t.Errorf("dbExists() = false for existing file %q", tmpFile)
	}
}

func createEmptyFile(path string) (*os.File, error) {
	return os.Create(path)
}

func TestParseSessionFromDBSince(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Insert messages at different timestamps.
	if err := insertTestMessage(db, "m1", "s1", `{"role":"user","content":"old"}`, now-10); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if err := insertTestMessage(db, "m2", "s1", `{"role":"assistant","content":"new"}`, now+10); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	// Query messages since now — should only get m2.
	entries, err := parseSessionFromDBSince(db, "s1", now)
	if err != nil {
		t.Fatalf("parseSessionFromDBSince() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Role() != "assistant" {
		t.Errorf("entry Role() = %q, want %q", entries[0].Role(), "assistant")
	}
}

func TestQueryProjectsSkipsEmptyDirectory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	// Session with empty directory should be excluded.
	if err := insertTestSession(db, "s1", "p1", "No Dir", "", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	projects, err := queryProjects(db)
	if err != nil {
		t.Fatalf("queryProjects() error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("len(projects) = %d, want 0 (empty directory excluded)", len(projects))
	}
}

func TestParseSessionFromDBTimestampParsing(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Message with an RFC3339 timestamp in the data field.
	data := `{"role":"user","content":"test","timestamp":"2026-04-30T15:30:00Z"}`
	if err := insertTestMessage(db, "m1", "s1", data, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	entries, err := parseSessionFromDB(db, "s1")
	if err != nil {
		t.Fatalf("parseSessionFromDB() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	// The timestamp should be from the data field, not time_created.
	expected := time.Date(2026, 4, 30, 15, 30, 0, 0, time.UTC)
	if !entries[0].Timestamp().Equal(expected) {
		t.Errorf("Timestamp() = %v, want %v", entries[0].Timestamp(), expected)
	}
}

// --- Behavioral tests for real-world format handling ---

func TestParseToolPartNestedFormat(t *testing.T) {
	// Verify that tool parts using the nested "tool"/"state.input"/"state.output"
	// format (as documented in the OpenCode research) parse correctly.
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	if err := insertTestMessage(db, "m1", "s1", `{"role":"assistant","content":""}`, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	// Nested format: "tool" field (not "toolName"), state as object with input/output.
	nestedToolJSON := `{"type":"tool","tool":"bash","callID":"tc1","state":{"input":"ls -la","output":"total 42\ndrwxr-xr-x ..."}}`
	if err := insertTestPart(db, "p1", "m1", nestedToolJSON); err != nil {
		t.Fatalf("insert part: %v", err)
	}

	entries, err := parseSessionFromDB(db, "s1")
	if err != nil {
		t.Fatalf("parseSessionFromDB() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	blocks := entries[0].ContentBlocks()
	if len(blocks) != 1 {
		t.Fatalf("ContentBlocks() len = %d, want 1", len(blocks))
	}

	tool := blocks[0]
	if tool.ContentType() != agent.ContentBlockToolUse {
		t.Errorf("ContentType() = %q, want %q", tool.ContentType(), agent.ContentBlockToolUse)
	}
	if tool.ToolName() != "bash" {
		t.Errorf("ToolName() = %q, want %q", tool.ToolName(), "bash")
	}
	if tool.ToolOutput() != "total 42\ndrwxr-xr-x ..." {
		t.Errorf("ToolOutput() = %q, want nested output", tool.ToolOutput())
	}
	if tool.ToolInput()["input"] != "ls -la" {
		t.Errorf("ToolInput()['input'] = %q, want %q", tool.ToolInput()["input"], "ls -la")
	}
}

func TestParseToolPartFlatFormat(t *testing.T) {
	// Verify that the original flat format still works after the parser update.
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	if err := insertTestMessage(db, "m1", "s1", `{"role":"assistant","content":""}`, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	flatToolJSON := `{"type":"tool","toolName":"read","toolCallID":"tc2","state":"completed","output":"file contents here"}`
	if err := insertTestPart(db, "p1", "m1", flatToolJSON); err != nil {
		t.Fatalf("insert part: %v", err)
	}

	entries, err := parseSessionFromDB(db, "s1")
	if err != nil {
		t.Fatalf("parseSessionFromDB() error: %v", err)
	}

	blocks := entries[0].ContentBlocks()
	if len(blocks) != 1 {
		t.Fatalf("ContentBlocks() len = %d, want 1", len(blocks))
	}

	tool := blocks[0]
	if tool.ToolName() != "read" {
		t.Errorf("ToolName() = %q, want %q", tool.ToolName(), "read")
	}
	if tool.ToolOutput() != "file contents here" {
		t.Errorf("ToolOutput() = %q, want %q", tool.ToolOutput(), "file contents here")
	}
}

func TestWatcherDetectsMultipleNewMessages(t *testing.T) {
	// Verify watcher accumulates multiple messages inserted between polls.
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()

	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Seed one existing message.
	if err := insertTestMessage(db, "m0", "s1", `{"role":"user","content":"initial"}`, now); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	w, err := NewOpenCodeWatcher(db, "s1")
	if err != nil {
		t.Fatalf("NewOpenCodeWatcher() error: %v", err)
	}
	defer w.Close()

	// No new entries initially.
	entries, err := w.NewEntries()
	if err != nil {
		t.Fatalf("NewEntries() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("initial len(entries) = %d, want 0", len(entries))
	}

	// Insert two new messages.
	if err := insertTestMessage(db, "m1", "s1", `{"role":"assistant","content":"thinking..."}`, now+1); err != nil {
		t.Fatalf("insert m1: %v", err)
	}
	if err := insertTestMessage(db, "m2", "s1", `{"role":"assistant","content":"done"}`, now+2); err != nil {
		t.Fatalf("insert m2: %v", err)
	}

	// Wait for poll cycle.
	time.Sleep(200 * time.Millisecond)

	entries, err = w.NewEntries()
	if err != nil {
		t.Fatalf("NewEntries() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	// Both entries should be assistant type.
	for i, e := range entries {
		if e.Type() != agent.EntryTypeAssistant {
			t.Errorf("entries[%d].Type() = %q, want %q", i, e.Type(), agent.EntryTypeAssistant)
		}
		if e.AgentType() != agent.AgentOpenCode {
			t.Errorf("entries[%d].AgentType() = %q, want %q", i, e.AgentType(), agent.AgentOpenCode)
		}
	}

	// Subsequent call should return zero — already consumed.
	entries2, err := w.NewEntries()
	if err != nil {
		t.Fatalf("NewEntries() second call error: %v", err)
	}
	if len(entries2) != 0 {
		t.Errorf("second call len(entries) = %d, want 0 (already consumed)", len(entries2))
	}
}

func TestFullProviderPipelineWithTempDB(t *testing.T) {
	// End-to-end behavioral test: create a real SQLite file, insert data,
	// and verify the full Provider pipeline (DiscoverProjects -> DiscoverSessions -> ParseSession).
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "opencode.db")

	// Create and seed the database.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}

	for _, stmt := range []string{
		`CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT, title TEXT, directory TEXT, time_created INTEGER, time_updated INTEGER)`,
		`CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, data TEXT, time_created INTEGER)`,
		`CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, data TEXT)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			t.Fatalf("create table: %v", err)
		}
	}

	now := time.Now().Unix()
	// Two sessions in /my/project, one in /other/project.
	seed := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatalf("seed db: %v", err)
		}
	}
	seed(`INSERT INTO session VALUES ('s1','p1','Fix bug','/my/project',?,?)`, now-100, now-50)
	seed(`INSERT INTO session VALUES ('s2','p1','Refactor','/my/project',?,?)`, now-10, now)
	seed(`INSERT INTO session VALUES ('s3','p2','Other','/other/project',?,?)`, now, now)

	// Add messages to s1.
	seed(`INSERT INTO message VALUES ('m1','s1',?,?)`, `{"role":"user","content":"Fix the login bug"}`, now-90)
	seed(`INSERT INTO message VALUES ('m2','s1',?,?)`,
		`{"role":"assistant","modelID":"claude-3.5-sonnet","content":""}`, now-80)
	seed(`INSERT INTO part VALUES ('p1','m2',?)`, `{"type":"reasoning","text":"Looking at auth code..."}`)
	seed(`INSERT INTO part VALUES ('p2','m2',?)`, `{"type":"text","text":"The bug is on line 42."}`)
	seed(`INSERT INTO part VALUES ('p3','m2',?)`, `{"type":"tool","tool":"bash","state":{"input":"go test ./...","output":"PASS"}}`)

	db.Close()

	// Now test the full provider pipeline.
	p := NewProvider(WithDBPath(dbPath))

	if !p.IsAvailable() {
		t.Fatal("IsAvailable() = false, want true for temp database")
	}

	projects, err := p.DiscoverProjects()
	if err != nil {
		t.Fatalf("DiscoverProjects() error: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2", len(projects))
	}

	// Find /my/project (should have 2 sessions).
	var myProject agent.Project
	for _, pr := range projects {
		if pr.Path == "/my/project" {
			myProject = pr
		}
	}
	if myProject.Path != "/my/project" {
		t.Fatal("project /my/project not found")
	}
	if myProject.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2", myProject.SessionCount)
	}

	// Discover sessions.
	sessions, err := p.DiscoverSessions(myProject)
	if err != nil {
		t.Fatalf("DiscoverSessions() error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}

	// Most recent session should be s2 (Refactor).
	if sessions[0].ID != "s2" {
		t.Errorf("sessions[0].ID = %q, want %q (ordered by time_created DESC)", sessions[0].ID, "s2")
	}

	// Check enriched session data for s1.
	var s1 agent.Session
	for _, s := range sessions {
		if s.ID == "s1" {
			s1 = s
		}
	}
	if s1.MessageCount != 2 {
		t.Errorf("s1.MessageCount = %d, want 2", s1.MessageCount)
	}
	if s1.FirstUserMessage != "Fix the login bug" {
		t.Errorf("s1.FirstUserMessage = %q, want %q", s1.FirstUserMessage, "Fix the login bug")
	}
	if s1.Model != "claude-3.5-sonnet" {
		t.Errorf("s1.Model = %q, want %q", s1.Model, "claude-3.5-sonnet")
	}
	if s1.TurnCount != 1 {
		t.Errorf("s1.TurnCount = %d, want 1", s1.TurnCount)
	}

	// Parse the full session.
	entries, err := p.ParseSession(s1)
	if err != nil {
		t.Fatalf("ParseSession() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	// Verify user entry.
	if entries[0].Type() != agent.EntryTypeUser {
		t.Errorf("entries[0].Type() = %q, want user", entries[0].Type())
	}
	if entries[0].Role() != "user" {
		t.Errorf("entries[0].Role() = %q, want user", entries[0].Role())
	}

	// Verify assistant entry has 3 content blocks (reasoning + text + tool).
	asst := entries[1]
	if asst.Type() != agent.EntryTypeAssistant {
		t.Errorf("entries[1].Type() = %q, want assistant", asst.Type())
	}
	blocks := asst.ContentBlocks()
	if len(blocks) != 3 {
		t.Fatalf("assistant ContentBlocks() len = %d, want 3", len(blocks))
	}
	if blocks[0].ContentType() != agent.ContentBlockReasoning {
		t.Errorf("block[0] = %q, want reasoning", blocks[0].ContentType())
	}
	if blocks[1].ContentType() != agent.ContentBlockText {
		t.Errorf("block[1] = %q, want text", blocks[1].ContentType())
	}
	if blocks[2].ContentType() != agent.ContentBlockToolUse {
		t.Errorf("block[2] = %q, want tool_use", blocks[2].ContentType())
	}
	if blocks[2].ToolName() != "bash" {
		t.Errorf("block[2].ToolName() = %q, want bash", blocks[2].ToolName())
	}
	if blocks[2].ToolOutput() != "PASS" {
		t.Errorf("block[2].ToolOutput() = %q, want PASS", blocks[2].ToolOutput())
	}
}

func TestDiscoverSessionsForProjectWithNoMessages(t *testing.T) {
	// Sessions with no messages should still be discoverable, just with empty metadata.
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()
	if err := insertTestSession(db, "s1", "p1", "Empty", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	sessions, err := querySessions(db, "/proj")
	if err != nil {
		t.Fatalf("querySessions() error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(sessions))
	}

	if err := enrichSessions(db, sessions); err != nil {
		t.Fatalf("enrichSessions() error: %v", err)
	}

	if sessions[0].MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0", sessions[0].MessageCount)
	}
	if sessions[0].FirstUserMessage != "" {
		t.Errorf("FirstUserMessage = %q, want empty", sessions[0].FirstUserMessage)
	}
	if sessions[0].TurnCount != 0 {
		t.Errorf("TurnCount = %d, want 0", sessions[0].TurnCount)
	}
}

func TestWatcherDoesNotReportOldMessages(t *testing.T) {
	// The watcher should only report messages created AFTER it starts.
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Unix()
	if err := insertTestSession(db, "s1", "p1", "Test", "/proj", now, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Insert messages before creating watcher.
	if err := insertTestMessage(db, "m1", "s1", `{"role":"user","content":"old 1"}`, now-100); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := insertTestMessage(db, "m2", "s1", `{"role":"assistant","content":"old 2"}`, now-50); err != nil {
		t.Fatalf("insert: %v", err)
	}

	w, err := NewOpenCodeWatcher(db, "s1")
	if err != nil {
		t.Fatalf("NewOpenCodeWatcher() error: %v", err)
	}
	defer w.Close()

	// Old messages should not appear.
	entries, err := w.NewEntries()
	if err != nil {
		t.Fatalf("NewEntries() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0 (old messages ignored)", len(entries))
	}

	// Insert a new message — only this should appear.
	if err := insertTestMessage(db, "m3", "s1", `{"role":"user","content":"new message"}`, now+10); err != nil {
		t.Fatalf("insert: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	entries, err = w.NewEntries()
	if err != nil {
		t.Fatalf("NewEntries() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1 (only new message)", len(entries))
	}
	if entries[0].Role() != "user" {
		t.Errorf("entry Role() = %q, want user", entries[0].Role())
	}
}
