package parser

import (
	"strings"
	"testing"
)

func TestParseJSONL_OnlyMetadata_NoError(t *testing.T) {
	// AC-1 & AC-2: Non-conversation entries (session metadata) should be skipped
	// silently with no parse errors. This verifies the parser behavior that
	// enables graceful handling of mixed log content.
	tests := []struct {
		name       string
		input      string
		wantCount  int
		wantErrors int
	}{
		{
			name:       "session init entry",
			input:      `{"parentUuid":"abc","isSidechain":false,"userType":"external"}` + "\n",
			wantCount:  0,
			wantErrors: 0,
		},
		{
			name:       "session metadata with cwd",
			input:      `{"cwd":"/path/to/dir","timestamp":"2026-01-20T10:00:00Z"}` + "\n",
			wantCount:  0,
			wantErrors: 0,
		},
		{
			name:       "config entry",
			input:      `{"model":"claude-opus-4-5-20251101","apiKey":"sk-xxx"}` + "\n",
			wantCount:  0,
			wantErrors: 0,
		},
		{
			name:       "multiple non-conversation entries",
			input:      `{"parentUuid":"abc","isSidechain":false}` + "\n" + `{"cwd":"/foo"}` + "\n",
			wantCount:  0,
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result := ParseJSONL(reader)

			if len(result.Entries) != tt.wantCount {
				t.Errorf("expected %d entries, got %d", tt.wantCount, len(result.Entries))
			}
			if result.ParseErrors != tt.wantErrors {
				t.Errorf("expected %d parse errors, got %d", tt.wantErrors, result.ParseErrors)
			}
		})
	}
}

func TestParseJSONL_MixedEntries_SkipsUnknown(t *testing.T) {
	// AC-3: Mixed input with both conversation and non-conversation entries
	// should format conversation entries normally and skip non-conversation silently.
	input := `{"parentUuid":"abc","isSidechain":false}
{"type":"user","message":{"role":"user","content":"Hello"},"timestamp":"2026-01-20T10:00:00Z"}
{"cwd":"/foo"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Hi there!"}]},"timestamp":"2026-01-20T10:01:00Z"}
{"model":"opus"}
`
	reader := strings.NewReader(input)
	result := ParseJSONL(reader)

	// Should have 2 conversation entries (user + assistant)
	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result.Entries))
	}
	// No parse errors - non-conversation entries are skipped silently
	if result.ParseErrors != 0 {
		t.Errorf("expected 0 parse errors, got %d", result.ParseErrors)
	}
}

func TestParseJSONL_EmptyInput_NoError(t *testing.T) {
	// Empty input is valid (no lines to parse)
	reader := strings.NewReader("")
	result := ParseJSONL(reader)

	if len(result.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result.Entries))
	}
	if result.ParseErrors != 0 {
		t.Errorf("expected 0 parse errors, got %d", result.ParseErrors)
	}
}

func TestParseJSONL_InvalidJSON_CountsAsError(t *testing.T) {
	// Invalid JSON should increment ParseErrors
	input := `not valid json` + "\n"
	reader := strings.NewReader(input)
	result := ParseJSONL(reader)

	if len(result.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result.Entries))
	}
	if result.ParseErrors != 1 {
		t.Errorf("expected 1 parse error, got %d", result.ParseErrors)
	}
}

func TestParseJSONL_OnlyInvalidJSON_HasErrors(t *testing.T) {
	// All invalid JSON should show parse errors
	input := `not json 1
not json 2
{broken: json}
`
	reader := strings.NewReader(input)
	result := ParseJSONL(reader)

	if len(result.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result.Entries))
	}
	if result.ParseErrors != 3 {
		t.Errorf("expected 3 parse errors, got %d", result.ParseErrors)
	}
}

func TestParseJSONL_BlankLines_Skipped(t *testing.T) {
	// Blank lines should be ignored (not counted as errors)
	input := "\n\n" + `{"type":"user","message":{"role":"user","content":"Test"},"timestamp":"2026-01-20T10:00:00Z"}` + "\n\n"
	reader := strings.NewReader(input)
	result := ParseJSONL(reader)

	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.ParseErrors != 0 {
		t.Errorf("expected 0 parse errors, got %d", result.ParseErrors)
	}
}
