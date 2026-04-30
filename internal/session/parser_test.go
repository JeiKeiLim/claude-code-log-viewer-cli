package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// writeParserJSON writes a SessionMeta as JSON to tmpDir/{pid}.json.
func writeParserJSON(t *testing.T, dir string, pid int, meta SessionMeta) string {
	t.Helper()
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	path := filepath.Join(dir, strconv.Itoa(pid)+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

// --- SessionParseError tests ---

func TestSessionParseError_Error(t *testing.T) {
	t.Run("with field name in message", func(t *testing.T) {
		err := &SessionParseError{
			Kind:    ParseErrMissingField,
			Field:   "sessionId",
			Message: "sessionId is required",
		}
		got := err.Error()
		if got == "" {
			t.Error("expected non-empty error string")
		}
		if !containsStr(got, "sessionId") {
			t.Errorf("expected field name in error string, got: %q", got)
		}
	})

	t.Run("without field", func(t *testing.T) {
		err := &SessionParseError{
			Kind:    ParseErrInvalidJSON,
			Message: "invalid JSON",
		}
		got := err.Error()
		if got == "" {
			t.Error("expected non-empty error string")
		}
	})

	t.Run("Unwrap returns cause for errors.Is", func(t *testing.T) {
		cause := errors.New("underlying io error")
		err := &SessionParseError{
			Kind:    ParseErrReadFailed,
			Message: "reading file",
			Cause:   cause,
		}
		if !errors.Is(err, cause) {
			t.Error("expected errors.Is to find cause via Unwrap")
		}
	})

	t.Run("Unwrap returns nil when no cause", func(t *testing.T) {
		err := &SessionParseError{
			Kind:    ParseErrMissingField,
			Field:   "sessionId",
			Message: "required",
		}
		if err.Unwrap() != nil {
			t.Errorf("expected nil Unwrap for no cause, got: %v", err.Unwrap())
		}
	})
}

func TestSessionParseError_KindConstants(t *testing.T) {
	kinds := []ParseErrorKind{
		ParseErrReadFailed,
		ParseErrInvalidJSON,
		ParseErrMissingField,
		ParseErrPIDMismatch,
	}
	seen := map[ParseErrorKind]bool{}
	for _, k := range kinds {
		if seen[k] {
			t.Errorf("duplicate ParseErrorKind value: %d", k)
		}
		seen[k] = true
	}
}

// --- IsParseError tests ---

func TestIsParseError(t *testing.T) {
	t.Run("SessionParseError returns true", func(t *testing.T) {
		err := &SessionParseError{Kind: ParseErrInvalidJSON, Message: "bad json"}
		if !IsParseError(err) {
			t.Error("expected IsParseError=true for *SessionParseError")
		}
	})

	t.Run("wrapped SessionParseError returns true", func(t *testing.T) {
		inner := &SessionParseError{Kind: ParseErrReadFailed, Message: "read failed"}
		wrapped := fmt.Errorf("context: %w", inner)
		if !IsParseError(wrapped) {
			t.Error("expected IsParseError=true for wrapped *SessionParseError")
		}
	})

	t.Run("plain error returns false", func(t *testing.T) {
		if IsParseError(errors.New("plain error")) {
			t.Error("expected IsParseError=false for plain error")
		}
	})

	t.Run("nil returns false", func(t *testing.T) {
		if IsParseError(nil) {
			t.Error("expected IsParseError=false for nil")
		}
	})
}

// --- ValidateSessionMeta tests ---

func TestValidateSessionMeta(t *testing.T) {
	t.Run("valid — all fields populated", func(t *testing.T) {
		meta := SessionMeta{
			PID:        42,
			SessionID:  "abc-123",
			CWD:        "/home/user/project",
			StartedAt:  1774909391881,
			Kind:       "interactive",
			Entrypoint: "claude",
		}
		if err := ValidateSessionMeta(meta); err != nil {
			t.Errorf("unexpected error for valid meta: %v", err)
		}
	})

	t.Run("valid — only sessionId required", func(t *testing.T) {
		if err := ValidateSessionMeta(SessionMeta{SessionID: "min-id"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid — empty sessionId", func(t *testing.T) {
		meta := SessionMeta{PID: 42, CWD: "/project"}
		err := ValidateSessionMeta(meta)
		if err == nil {
			t.Fatal("expected error for empty sessionId")
		}
		var pe *SessionParseError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *SessionParseError, got %T", err)
		}
		if pe.Kind != ParseErrMissingField {
			t.Errorf("expected ParseErrMissingField, got %d", pe.Kind)
		}
		if pe.Field != "sessionId" {
			t.Errorf("expected field='sessionId', got %q", pe.Field)
		}
	})

	t.Run("valid — zero PID is acceptable (set from filename later)", func(t *testing.T) {
		if err := ValidateSessionMeta(SessionMeta{PID: 0, SessionID: "test"}); err != nil {
			t.Errorf("unexpected error for zero PID: %v", err)
		}
	})

	t.Run("valid — zero startedAt is acceptable", func(t *testing.T) {
		if err := ValidateSessionMeta(SessionMeta{SessionID: "test", StartedAt: 0}); err != nil {
			t.Errorf("unexpected error for zero startedAt: %v", err)
		}
	})

	t.Run("valid — empty CWD is acceptable", func(t *testing.T) {
		if err := ValidateSessionMeta(SessionMeta{SessionID: "test", CWD: ""}); err != nil {
			t.Errorf("unexpected error for empty CWD: %v", err)
		}
	})
}

// --- ParseSessionFile tests ---

func TestParseSessionFile(t *testing.T) {
	t.Run("valid — all fields populated", func(t *testing.T) {
		tmpDir := t.TempDir()
		meta := SessionMeta{
			PID:        42,
			SessionID:  "session-abc-123",
			CWD:        "/home/user/project",
			StartedAt:  1774909391881,
			Kind:       "interactive",
			Entrypoint: "claude",
		}
		path := writeParserJSON(t, tmpDir, 42, meta)

		got, err := ParseSessionFile(path, 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Meta.PID != 42 {
			t.Errorf("PID: got %d, want 42", got.Meta.PID)
		}
		if got.Meta.SessionID != "session-abc-123" {
			t.Errorf("SessionID: got %q, want 'session-abc-123'", got.Meta.SessionID)
		}
		if got.Meta.CWD != "/home/user/project" {
			t.Errorf("CWD: got %q, want '/home/user/project'", got.Meta.CWD)
		}
		if got.Meta.StartedAt != 1774909391881 {
			t.Errorf("StartedAt: got %d, want 1774909391881", got.Meta.StartedAt)
		}
		if got.Meta.Kind != "interactive" {
			t.Errorf("Kind: got %q, want 'interactive'", got.Meta.Kind)
		}
		if got.Meta.Entrypoint != "claude" {
			t.Errorf("Entrypoint: got %q, want 'claude'", got.Meta.Entrypoint)
		}
		if got.FilePath != path {
			t.Errorf("FilePath: got %q, want %q", got.FilePath, path)
		}
	})

	t.Run("valid — CWD derives JSONLDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		meta := SessionMeta{
			PID:       100,
			SessionID: "with-cwd",
			CWD:       "/Users/test/myproject",
		}
		path := writeParserJSON(t, tmpDir, 100, meta)

		got, err := ParseSessionFile(path, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.JSONLDir == "" {
			t.Error("expected non-empty JSONLDir when CWD is present")
		}
		if !containsStr(got.JSONLDir, "myproject") {
			t.Errorf("expected JSONLDir to contain 'myproject', got %q", got.JSONLDir)
		}
	})

	t.Run("valid — empty CWD leaves JSONLDir empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		meta := SessionMeta{PID: 100, SessionID: "no-cwd"}
		path := writeParserJSON(t, tmpDir, 100, meta)

		got, err := ParseSessionFile(path, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.JSONLDir != "" {
			t.Errorf("expected empty JSONLDir for empty CWD, got %q", got.JSONLDir)
		}
	})

	t.Run("valid — PID absent in content uses filename PID", func(t *testing.T) {
		tmpDir := t.TempDir()
		// PID: 0 means the pid field is serialized as 0, treated as absent
		meta := SessionMeta{PID: 0, SessionID: "no-pid-in-content"}
		path := writeParserJSON(t, tmpDir, 55, meta)

		got, err := ParseSessionFile(path, 55)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Meta.PID != 55 {
			t.Errorf("expected PID 55 from filename, got %d", got.Meta.PID)
		}
	})

	t.Run("valid — partial JSON (only sessionId)", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "77.json")
		if err := os.WriteFile(path, []byte(`{"sessionId": "partial-session"}`), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		got, err := ParseSessionFile(path, 77)
		if err != nil {
			t.Fatalf("unexpected error for partial JSON: %v", err)
		}
		if got.Meta.SessionID != "partial-session" {
			t.Errorf("unexpected SessionID: %q", got.Meta.SessionID)
		}
		if got.Meta.PID != 77 {
			t.Errorf("expected PID 77 from filename, got %d", got.Meta.PID)
		}
		if got.Meta.CWD != "" {
			t.Errorf("expected empty CWD, got %q", got.Meta.CWD)
		}
	})

	t.Run("valid — extra unknown JSON fields are ignored", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "42.json")
		rawJSON := `{"pid": 42, "sessionId": "known-session", "unknownField": "value", "extra": 123}`
		if err := os.WriteFile(path, []byte(rawJSON), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		got, err := ParseSessionFile(path, 42)
		if err != nil {
			t.Fatalf("unexpected error for extra fields: %v", err)
		}
		if got.Meta.SessionID != "known-session" {
			t.Errorf("unexpected SessionID: %q", got.Meta.SessionID)
		}
	})

	t.Run("valid — real-world session file format from product brief", func(t *testing.T) {
		tmpDir := t.TempDir()
		rawJSON := `{
			"pid": 60696,
			"sessionId": "e10c86ca-6cd9-4716-b905-576810a52484",
			"cwd": "/Users/limjk/GitHub/JeiKeiLim/podcast-gen-web",
			"startedAt": 1774909391881,
			"kind": "interactive",
			"entrypoint": "sdk-cli"
		}`
		path := filepath.Join(tmpDir, "60696.json")
		if err := os.WriteFile(path, []byte(rawJSON), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		got, err := ParseSessionFile(path, 60696)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Meta.PID != 60696 {
			t.Errorf("PID: got %d, want 60696", got.Meta.PID)
		}
		if got.Meta.SessionID != "e10c86ca-6cd9-4716-b905-576810a52484" {
			t.Errorf("SessionID: got %q", got.Meta.SessionID)
		}
		if got.Meta.CWD != "/Users/limjk/GitHub/JeiKeiLim/podcast-gen-web" {
			t.Errorf("CWD: got %q", got.Meta.CWD)
		}
		if got.Meta.StartedAt != 1774909391881 {
			t.Errorf("StartedAt: got %d", got.Meta.StartedAt)
		}
		if got.Meta.Kind != "interactive" {
			t.Errorf("Kind: got %q", got.Meta.Kind)
		}
		if got.Meta.Entrypoint != "sdk-cli" {
			t.Errorf("Entrypoint: got %q", got.Meta.Entrypoint)
		}
		if got.JSONLDir == "" {
			t.Error("expected non-empty JSONLDir")
		}
	})

	t.Run("error — file does not exist → ParseErrReadFailed", func(t *testing.T) {
		_, err := ParseSessionFile("/nonexistent/path/42.json", 42)
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
		var pe *SessionParseError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *SessionParseError, got %T: %v", err, err)
		}
		if pe.Kind != ParseErrReadFailed {
			t.Errorf("expected ParseErrReadFailed, got %d", pe.Kind)
		}
	})

	t.Run("error — invalid JSON → ParseErrInvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "42.json")
		mustWriteFile(t, path, []byte("{invalid json"))

		_, err := ParseSessionFile(path, 42)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
		var pe *SessionParseError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *SessionParseError, got %T: %v", err, err)
		}
		if pe.Kind != ParseErrInvalidJSON {
			t.Errorf("expected ParseErrInvalidJSON, got %d", pe.Kind)
		}
	})

	t.Run("error — empty file → ParseErrInvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "42.json")
		mustWriteFile(t, path, []byte(""))

		_, err := ParseSessionFile(path, 42)
		if err == nil {
			t.Fatal("expected error for empty file")
		}
		if !IsParseError(err) {
			t.Errorf("expected IsParseError=true, got false")
		}
	})

	t.Run("error — missing sessionId → ParseErrMissingField", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "42.json")
		mustWriteFile(t, path, []byte(`{"pid": 42, "cwd": "/home/user"}`))

		_, err := ParseSessionFile(path, 42)
		if err == nil {
			t.Fatal("expected error for missing sessionId")
		}
		var pe *SessionParseError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *SessionParseError, got %T: %v", err, err)
		}
		if pe.Kind != ParseErrMissingField {
			t.Errorf("expected ParseErrMissingField, got %d", pe.Kind)
		}
		if pe.Field != "sessionId" {
			t.Errorf("expected field='sessionId', got %q", pe.Field)
		}
	})

	t.Run("error — empty sessionId string → ParseErrMissingField", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "42.json")
		mustWriteFile(t, path, []byte(`{"pid": 42, "sessionId": ""}`))

		_, err := ParseSessionFile(path, 42)
		if err == nil {
			t.Fatal("expected error for empty sessionId string")
		}
		var pe *SessionParseError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *SessionParseError, got %T", err)
		}
		if pe.Kind != ParseErrMissingField {
			t.Errorf("expected ParseErrMissingField, got %d", pe.Kind)
		}
	})

	t.Run("error — PID mismatch → ParseErrPIDMismatch", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Content PID 999 does not match filename PID 42
		meta := SessionMeta{PID: 999, SessionID: "mismatch-session"}
		path := writeParserJSON(t, tmpDir, 42, meta)

		_, err := ParseSessionFile(path, 42)
		if err == nil {
			t.Fatal("expected error for PID mismatch")
		}
		var pe *SessionParseError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *SessionParseError, got %T: %v", err, err)
		}
		if pe.Kind != ParseErrPIDMismatch {
			t.Errorf("expected ParseErrPIDMismatch, got %d", pe.Kind)
		}
		if pe.Field != "pid" {
			t.Errorf("expected field='pid', got %q", pe.Field)
		}
	})

	t.Run("error — null JSON → ParseErrMissingField (sessionId empty)", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "42.json")
		mustWriteFile(t, path, []byte("null"))

		_, err := ParseSessionFile(path, 42)
		if err == nil {
			t.Fatal("expected error for null JSON (sessionId empty)")
		}
	})

	t.Run("error — JSON array instead of object → ParseErrInvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "42.json")
		mustWriteFile(t, path, []byte(`["not", "an", "object"]`))

		_, err := ParseSessionFile(path, 42)
		if err == nil {
			t.Fatal("expected error for JSON array")
		}
		var pe *SessionParseError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *SessionParseError, got %T", err)
		}
		// Arrays cannot unmarshal into struct → JSON error
		if pe.Kind != ParseErrInvalidJSON && pe.Kind != ParseErrMissingField {
			t.Errorf("expected ParseErrInvalidJSON or ParseErrMissingField, got %d", pe.Kind)
		}
	})

	t.Run("IsParseError — true on any parse failure", func(t *testing.T) {
		_, err := ParseSessionFile("/nonexistent/42.json", 42)
		if !IsParseError(err) {
			t.Error("expected IsParseError=true for read failure")
		}
	})
}

// TestParseSessionFile_ScannerCompatibility verifies that ParseSessionFile
// produces the same results as the scanner's inline logic before the
// refactoring. Ensures behavioral compatibility.
func TestParseSessionFile_ScannerCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		filenamePID int
		meta        SessionMeta
		wantErr     bool
		wantPID     int
		wantJSONL   bool
	}{
		{
			name:        "normal session",
			filenamePID: 100,
			meta:        SessionMeta{PID: 100, SessionID: "sess-a", CWD: "/home/user/project"},
			wantPID:     100,
			wantJSONL:   true,
		},
		{
			name:        "PID absent from content — uses filename",
			filenamePID: 100,
			meta:        SessionMeta{SessionID: "sess-b", CWD: "/home/user"},
			wantPID:     100,
			wantJSONL:   true,
		},
		{
			name:        "no CWD — JSONLDir empty",
			filenamePID: 200,
			meta:        SessionMeta{PID: 200, SessionID: "sess-c"},
			wantPID:     200,
			wantJSONL:   false,
		},
		{
			name:        "PID mismatch — error",
			filenamePID: 100,
			meta:        SessionMeta{PID: 999, SessionID: "mismatch"},
			wantErr:     true,
		},
		{
			name:        "missing sessionId — error",
			filenamePID: 100,
			meta:        SessionMeta{PID: 100},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			data, err := json.Marshal(tt.meta)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			path := filepath.Join(tmpDir, strconv.Itoa(tt.filenamePID)+".json")
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}

			got, err := ParseSessionFile(path, tt.filenamePID)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Meta.PID != tt.wantPID {
				t.Errorf("PID: got %d, want %d", got.Meta.PID, tt.wantPID)
			}
			if tt.wantJSONL && got.JSONLDir == "" {
				t.Error("expected non-empty JSONLDir")
			}
			if !tt.wantJSONL && got.JSONLDir != "" {
				t.Errorf("expected empty JSONLDir, got %q", got.JSONLDir)
			}
		})
	}
}

// containsStr returns true if s contains sub.
func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
