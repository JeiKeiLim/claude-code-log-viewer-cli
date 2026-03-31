package session

import (
	"testing"
	"time"
)

func TestSessionMeta_ProjectName(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{
			name: "standard project path",
			cwd:  "/Users/user/projects/my-project",
			want: "my-project",
		},
		{
			name: "single component path",
			cwd:  "/project",
			want: "project",
		},
		{
			name: "deep nested path",
			cwd:  "/home/user/GitHub/org/repo-name",
			want: "repo-name",
		},
		{
			name: "path with dots",
			cwd:  "/Users/me/projects/my.project.name",
			want: "my.project.name",
		},
		{
			name: "path with hyphens",
			cwd:  "/Users/limjk/GitHub/JeiKeiLim/claude-code-log-viewer-cli",
			want: "claude-code-log-viewer-cli",
		},
		{
			name: "empty CWD returns empty string",
			cwd:  "",
			want: "",
		},
		{
			name: "real-world example from product brief",
			cwd:  "/Users/limjk/GitHub/JeiKeiLim/podcast-gen-web",
			want: "podcast-gen-web",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := SessionMeta{CWD: tt.cwd, SessionID: "test"}
			got := meta.ProjectName()
			if got != tt.want {
				t.Errorf("ProjectName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSessionMeta_ProjectName_ZeroValue(t *testing.T) {
	var meta SessionMeta
	if meta.ProjectName() != "" {
		t.Errorf("ProjectName() on zero-value meta should return empty string, got %q", meta.ProjectName())
	}
}

func TestSessionMeta_StartedAtTime(t *testing.T) {
	// 2026-03-31T00:00:00Z in milliseconds
	ts := int64(1774934400000)
	meta := SessionMeta{
		PID:        1234,
		SessionID:  "test-session-id",
		CWD:        "/test/project",
		StartedAt:  ts,
		Kind:       "interactive",
		Entrypoint: "sdk-cli",
	}

	result := meta.StartedAtTime()
	expected := time.UnixMilli(ts)

	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestSessionMeta_StartedAtTime_Zero(t *testing.T) {
	meta := SessionMeta{StartedAt: 0}
	result := meta.StartedAtTime()
	expected := time.UnixMilli(0)

	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestSessionEventType_Constants(t *testing.T) {
	// Verify enum values are distinct
	if SessionOpened == SessionClosed {
		t.Error("SessionOpened and SessionClosed should be different values")
	}
}

func TestActiveSession_Fields(t *testing.T) {
	session := ActiveSession{
		Meta: SessionMeta{
			PID:        42,
			SessionID:  "abc-def",
			CWD:        "/home/user/project",
			StartedAt:  1774909391881,
			Kind:       "interactive",
			Entrypoint: "cli",
		},
		FilePath: "/home/user/.claude/sessions/42.json",
		JSONLDir: "/home/user/.claude/projects/-home-user-project",
	}

	if session.Meta.PID != 42 {
		t.Errorf("expected PID 42, got %d", session.Meta.PID)
	}
	if session.FilePath != "/home/user/.claude/sessions/42.json" {
		t.Errorf("unexpected FilePath: %s", session.FilePath)
	}
	if session.JSONLDir != "/home/user/.claude/projects/-home-user-project" {
		t.Errorf("unexpected JSONLDir: %s", session.JSONLDir)
	}
}

func TestSessionEvent_Fields(t *testing.T) {
	session := ActiveSession{
		Meta: SessionMeta{PID: 123, SessionID: "test"},
	}
	event := SessionEvent{
		Type:    SessionClosed,
		Session: session,
	}

	if event.Type != SessionClosed {
		t.Errorf("expected SessionClosed, got %d", event.Type)
	}
	if event.Session.Meta.PID != 123 {
		t.Errorf("expected PID 123, got %d", event.Session.Meta.PID)
	}
}
