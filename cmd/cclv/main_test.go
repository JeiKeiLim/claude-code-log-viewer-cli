package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/providers/claudecode"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/providers/codex"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/providers/opencode"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/tui"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

func TestValidateWidth(t *testing.T) {
	tests := []struct {
		name        string
		input       int
		want        int
		wantWarning bool
	}{
		{"zero returns zero", 0, 0, false},
		{"negative returns zero", -1, 0, false},
		{"valid 50", 50, 50, false},
		{"valid 80", 80, 80, false},
		{"valid 120", 120, 120, false},
		{"valid 500 max", 500, 500, false},
		{"min boundary 40", 40, 40, false},
		{"too small 30", 30, 80, true},
		{"too small 39", 39, 80, true},
		{"too large 501", 501, 500, true},
		{"too large 600", 600, 500, true},
		{"too large 1000", 1000, 500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			got := validateWidth(tt.input)

			_ = w.Close()
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			os.Stderr = oldStderr

			if got != tt.want {
				t.Errorf("validateWidth(%d) = %d, want %d", tt.input, got, tt.want)
			}

			hasWarning := strings.Contains(buf.String(), "Warning")
			if hasWarning != tt.wantWarning {
				t.Errorf("validateWidth(%d) warning = %v, wantWarning %v", tt.input, hasWarning, tt.wantWarning)
			}
		})
	}
}

func TestWatchFlagParsing(t *testing.T) {
	// Save original flag state and restore after test
	origArgs := os.Args
	t.Cleanup(func() {
		os.Args = origArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	})

	tests := []struct {
		name     string
		args     []string
		wantFlag bool
	}{
		{"watch flag sets watch mode", []string{"cmd", "--watch", "file.jsonl"}, true},
		{"live flag sets watch mode", []string{"cmd", "--live", "file.jsonl"}, true},
		{"both flags set watch mode", []string{"cmd", "--watch", "--live", "file.jsonl"}, true},
		{"no flag means no watch mode", []string{"cmd", "file.jsonl"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flag.CommandLine for each test
			flag.CommandLine = flag.NewFlagSet(tt.args[0], flag.ContinueOnError)

			watchFlag := flag.Bool("watch", false, "Watch file for changes")
			liveFlag := flag.Bool("live", false, "Alias for --watch")

			err := flag.CommandLine.Parse(tt.args[1:])
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			watchMode := *watchFlag || *liveFlag
			if watchMode != tt.wantFlag {
				t.Errorf("watchMode = %v, want %v", watchMode, tt.wantFlag)
			}
		})
	}
}

func TestUsageFlagParsing(t *testing.T) {
	// Save original flag state and restore after test
	origArgs := os.Args
	t.Cleanup(func() {
		os.Args = origArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	})

	tests := []struct {
		name     string
		args     []string
		wantFlag bool
	}{
		{"usage flag", []string{"cmd", "--usage"}, true},
		{"u shorthand flag", []string{"cmd", "-u"}, true},
		{"both flags", []string{"cmd", "--usage", "-u"}, true},
		{"no flag", []string{"cmd"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flag.CommandLine for each test
			flag.CommandLine = flag.NewFlagSet(tt.args[0], flag.ContinueOnError)

			usageFlag := flag.Bool("usage", false, "Print usage limits and exit")
			usageShortFlag := flag.Bool("u", false, "Print usage limits and exit (shorthand)")

			err := flag.CommandLine.Parse(tt.args[1:])
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			usageMode := *usageFlag || *usageShortFlag
			if usageMode != tt.wantFlag {
				t.Errorf("usageMode = %v, want %v", usageMode, tt.wantFlag)
			}
		})
	}
}

func TestUsageFlagsIdenticalBehavior(t *testing.T) {
	flagSets := []struct {
		name string
		args []string
	}{
		{"usage flag only", []string{"cmd", "--usage"}},
		{"u flag only", []string{"cmd", "-u"}},
	}

	var results []bool
	for _, fs := range flagSets {
		flag.CommandLine = flag.NewFlagSet(fs.args[0], flag.ContinueOnError)

		usageFlag := flag.Bool("usage", false, "Print usage limits and exit")
		usageShortFlag := flag.Bool("u", false, "Print usage limits and exit (shorthand)")

		err := flag.CommandLine.Parse(fs.args[1:])
		if err != nil {
			t.Fatalf("Failed to parse flags for %s: %v", fs.name, err)
		}

		usageMode := *usageFlag || *usageShortFlag
		results = append(results, usageMode)

		if !usageMode {
			t.Errorf("%s: expected usage mode to be true", fs.name)
		}
	}

	if len(results) >= 2 && results[0] != results[1] {
		t.Error("-u and --usage should produce identical behavior")
	}
}

func TestPrintHelpUsageFlagFormat(t *testing.T) {
	originalOutput := flag.CommandLine.Output()
	t.Cleanup(func() {
		flag.CommandLine.SetOutput(originalOutput)
	})

	var buf bytes.Buffer
	flag.CommandLine.SetOutput(&buf)
	printHelp()

	output := buf.String()

	if !strings.Contains(output, "-u, --usage") {
		t.Error("help text should contain '-u, --usage' showing both flag forms together")
	}
}

func TestPrintHelp(t *testing.T) {
	originalOutput := flag.CommandLine.Output()
	t.Cleanup(func() {
		flag.CommandLine.SetOutput(originalOutput)
	})

	var buf bytes.Buffer
	flag.CommandLine.SetOutput(&buf)
	printHelp()

	output := buf.String()

	requiredFlags := []string{
		"--plain",
		"--tui",
		"--color",
		"--hide-thoughts",
		"--hide-tools",
		"--width",
		"--watch",
		"--live",
		"--usage",
		"-u",
		"--version",
		"--help",
	}
	for _, f := range requiredFlags {
		if !strings.Contains(output, f) {
			t.Errorf("help output missing flag: %s", f)
		}
	}

	requiredExamples := []string{
		"cclv conversation.jsonl",
		"cclv --plain",
		"--hide-thoughts",
		"--watch",
		"cclv --usage",
		"cclv -u",
	}
	for _, ex := range requiredExamples {
		if !strings.Contains(output, ex) {
			t.Errorf("help output missing example containing: %s", ex)
		}
	}

	requiredSections := []string{
		"USAGE:",
		"OPTIONS:",
		"EXAMPLES:",
		"KEYBOARD SHORTCUTS",
	}
	for _, section := range requiredSections {
		if !strings.Contains(output, section) {
			t.Errorf("help output missing section: %s", section)
		}
	}

	shortcutGroups := []string{
		"Navigation:",
		"Scrolling:",
		"Toggles:",
		"Search:",
		"Actions:",
	}
	for _, group := range shortcutGroups {
		if !strings.Contains(output, group) {
			t.Errorf("help output missing shortcut group: %s", group)
		}
	}

	requiredShortcuts := []string{
		"j/k",
		"gg/G",
		"q, Ctrl+c",
		"/",
		"n/N",
		"t",
		"i",
		"Ctrl+d/u",
		"h/esc",
		"l/enter",
	}
	for _, shortcut := range requiredShortcuts {
		if !strings.Contains(output, shortcut) {
			t.Errorf("help output missing shortcut: %s", shortcut)
		}
	}
}

// --- Auto-detection streaming tests ---

// TestStreamingAutoDetectClaudeCode verifies that streaming mode auto-detects
// Claude Code format and produces rendered output containing the expected text.
func TestStreamingAutoDetectClaudeCode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	claudeCodeJSONL := `{"type":"user","message":{"role":"user","content":"Hello from Claude Code"},"timestamp":"2026-01-16T10:00:00Z"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Hi there from Claude!"}]},"timestamp":"2026-01-16T10:01:00Z"}
`
	if err := os.WriteFile(testFile, []byte(claudeCodeJSONL), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test detectFormatFromReader detects Claude Code
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}

	detected, newReader, detectErr := detectFormatFromReader(file)
	if detectErr != nil {
		t.Fatalf("detectFormatFromReader error: %v", detectErr)
	}
	if detected != agent.AgentClaudeCode {
		t.Errorf("detectFormatFromReader() = %q, want %q", detected, agent.AgentClaudeCode)
	}

	// Verify the full content is still readable through newReader
	all, err := io.ReadAll(newReader)
	_ = file.Close()
	if err != nil {
		t.Fatalf("failed to read from newReader: %v", err)
	}
	if !bytes.Contains(all, []byte("Hello from Claude Code")) {
		t.Error("newReader should contain the original file data")
	}
}

// TestStreamingAutoDetectCodex verifies that streaming mode auto-detects
// Codex format and uses selectProvider + convertConversationEntries.
func TestStreamingAutoDetectCodex(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	codexJSONL := `{"type":"session_meta","timestamp":"2026-04-30T10:00:00Z","payload":{"id":"sess-test","cwd":"/home/user/project","model":"o3"}}
{"type":"event_msg","timestamp":"2026-04-30T10:00:01Z","payload":{"event":{"type":"user_message","content":"Hello from Codex"}}}
{"type":"event_msg","timestamp":"2026-04-30T10:00:05Z","payload":{"event":{"type":"agent_message","content":"Hi from Codex!","phase":"final"}}}
`
	if err := os.WriteFile(testFile, []byte(codexJSONL), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test detectFormatFromReader detects Codex
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}

	detected, newReader, detectErr := detectFormatFromReader(file)
	if detectErr != nil {
		t.Fatalf("detectFormatFromReader error: %v", detectErr)
	}
	if detected != agent.AgentCodex {
		t.Errorf("detectFormatFromReader() = %q, want %q", detected, agent.AgentCodex)
	}

	// Verify selectProvider returns a Codex provider
	provider := selectProvider(detected)
	if provider.Type() != agent.AgentCodex {
		t.Errorf("selectProvider(%q).Type() = %q, want %q", detected, provider.Type(), agent.AgentCodex)
	}

	// Verify ParseSessionStream works through the reader
	convEntries, parseErr := provider.ParseSessionStream(newReader)
	_ = file.Close()
	if parseErr != nil {
		t.Fatalf("ParseSessionStream error: %v", parseErr)
	}
	if len(convEntries) == 0 {
		t.Error("ParseSessionStream should return entries for valid Codex JSONL")
	}

	// Verify convertConversationEntries produces renderable LogEntries
	entries := convertConversationEntries(convEntries)
	if len(entries) == 0 {
		t.Error("convertConversationEntries should produce LogEntry entries")
	}
}

// TestStreamingAgentOverride verifies that agent override skips auto-detection
// and uses the specified provider directly.
func TestStreamingAgentOverride(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	claudeCodeJSONL := `{"type":"user","message":{"role":"user","content":"Hello world"},"timestamp":"2026-01-16T10:00:00Z"}
`
	if err := os.WriteFile(testFile, []byte(claudeCodeJSONL), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// When override is set, detectFormatFromReader should NOT be called
	// Instead, selectProvider should use the override directly
	provider := selectProvider(agent.AgentCodex)
	if provider.Type() != agent.AgentCodex {
		t.Errorf("selectProvider with override AgentCodex = %q, want %q", provider.Type(), agent.AgentCodex)
	}

	// Verify Claude Code override still works
	providerCC := selectProvider(agent.AgentClaudeCode)
	if providerCC.Type() != agent.AgentClaudeCode {
		t.Errorf("selectProvider with override AgentClaudeCode = %q, want %q", providerCC.Type(), agent.AgentClaudeCode)
	}
}

// TestStreamingInitialOutputRendering verifies that the initial output
// from streaming plain mode actually renders entries to stdout.
func TestStreamingInitialOutputRendering(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	initialContent := `{"type":"user","message":{"role":"user","content":"Hello world"},"timestamp":"2026-01-16T10:00:00Z"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Hi there!"}]},"timestamp":"2026-01-16T10:01:00Z"}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		opts     tui.RenderOptions
		contains []string
	}{
		{
			name:     "renders initial entries with default options",
			opts:     tui.RenderOptions{},
			contains: []string{"Hello world", "Hi there!"},
		},
		{
			name:     "respects width option",
			opts:     tui.RenderOptions{Width: 60},
			contains: []string{"Hello world", "Hi there!"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the file the same way runStreamingPlainMode does
			file, err := os.Open(testFile)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}

			detected, newReader, detectErr := detectFormatFromReader(file)
			if detectErr != nil {
				_ = file.Close()
				t.Fatalf("detectFormatFromReader error: %v", detectErr)
			}
			if detected != agent.AgentClaudeCode {
				_ = file.Close()
				t.Fatalf("detected format = %q, want %q", detected, agent.AgentClaudeCode)
			}

			// Parse and render (same path as runStreamingPlainMode for Claude Code)
			result := parser.ParseJSONL(newReader)
			entries := result.Entries
			parseErrors := result.ParseErrors
			_ = file.Close()

			if len(entries) == 0 {
				t.Fatal("expected entries to be parsed, got 0")
			}
			if parseErrors > 0 {
				t.Errorf("unexpected parse errors: %d", parseErrors)
			}

			source := filepath.Base(testFile)
			output := tui.RenderPlain(entries, source, tt.opts)

			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("RenderPlain output missing expected text %q", want)
				}
			}
		})
	}
}

// --- selectProvider tests ---

func TestSelectProviderClaudeCode(t *testing.T) {
	p := selectProvider(agent.AgentClaudeCode)
	if p.Type() != agent.AgentClaudeCode {
		t.Errorf("selectProvider(AgentClaudeCode).Type() = %q, want %q", p.Type(), agent.AgentClaudeCode)
	}
}

func TestSelectProviderCodex(t *testing.T) {
	p := selectProvider(agent.AgentCodex)
	if _, ok := p.(*codex.Provider); !ok {
		t.Errorf("selectProvider(AgentCodex) = %T, want *codex.Provider", p)
	}
	if p.Type() != agent.AgentCodex {
		t.Errorf("selectProvider(AgentCodex).Type() = %q, want %q", p.Type(), agent.AgentCodex)
	}
}

// TestSelectProviderOpenCode verifies that selectProvider returns an OpenCode
// provider when given AgentOpenCode.
func TestSelectProviderOpenCode(t *testing.T) {
	p := selectProvider(agent.AgentOpenCode)
	if _, ok := p.(*opencode.Provider); !ok {
		t.Errorf("selectProvider(AgentOpenCode) = %T, want *opencode.Provider", p)
	}
	if p.Type() != agent.AgentOpenCode {
		t.Errorf("selectProvider(AgentOpenCode).Type() = %q, want %q", p.Type(), agent.AgentOpenCode)
	}
}

// --- parseAgentFlag tests ---

func TestParseAgentFlag(t *testing.T) {
	tests := []struct {
		input string
		want  agent.AgentType
	}{
		{"claude-code", agent.AgentClaudeCode},
		{"codex", agent.AgentCodex},
		{"opencode", agent.AgentOpenCode},
		{"", agent.AgentType("")},
		{"unknown", agent.AgentType("")},
		{"Claude-Code", agent.AgentType("")}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseAgentFlag(tt.input)
			if got != tt.want {
				t.Errorf("parseAgentFlag(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInteractiveProvidersDefaultIncludesAllAgents(t *testing.T) {
	providers := interactiveProviders("")

	if len(providers) != 3 {
		t.Fatalf("interactiveProviders(\"\") returned %d providers, want 3", len(providers))
	}
	if _, ok := providers[0].(*claudecode.ClaudeCodeProvider); !ok {
		t.Fatalf("provider[0] = %T, want *claudecode.ClaudeCodeProvider", providers[0])
	}
	if _, ok := providers[1].(*codex.Provider); !ok {
		t.Fatalf("provider[1] = %T, want *codex.Provider", providers[1])
	}
	if _, ok := providers[2].(*opencode.Provider); !ok {
		t.Fatalf("provider[2] = %T, want *opencode.Provider", providers[2])
	}
}

func TestInteractiveProvidersAgentOverride(t *testing.T) {
	tests := []struct {
		name string
		at   agent.AgentType
		want agent.AgentType
	}{
		{"claude-code", agent.AgentClaudeCode, agent.AgentClaudeCode},
		{"codex", agent.AgentCodex, agent.AgentCodex},
		{"opencode", agent.AgentOpenCode, agent.AgentOpenCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := interactiveProviders(tt.at)
			if len(providers) != 1 {
				t.Fatalf("interactiveProviders(%q) returned %d providers, want 1", tt.at, len(providers))
			}
			if got := providers[0].Type(); got != tt.want {
				t.Fatalf("interactiveProviders(%q)[0].Type() = %q, want %q", tt.at, got, tt.want)
			}
		})
	}
}

// --- Helper for tests ---

// TestFollowLatestFlagParsing tests Story 11.2 AC-1: flag parsing for -L/--follow-latest.
func TestFollowLatestFlagParsing(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() {
		os.Args = origArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	})

	tests := []struct {
		name     string
		args     []string
		wantFlag bool
	}{
		{"L short flag sets follow-latest", []string{"cmd", "-L", "file.jsonl"}, true},
		{"follow-latest long flag", []string{"cmd", "--follow-latest", "file.jsonl"}, true},
		{"both flags set follow-latest", []string{"cmd", "-L", "--follow-latest", "file.jsonl"}, true},
		{"no flag means no follow-latest", []string{"cmd", "file.jsonl"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(tt.args[0], flag.ContinueOnError)

			followLatestFlag := flag.Bool("follow-latest", false, "Follow to newest conversation")
			followLatestShortFlag := flag.Bool("L", false, "Follow to newest conversation")

			err := flag.CommandLine.Parse(tt.args[1:])
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			followLatest := *followLatestFlag || *followLatestShortFlag
			if followLatest != tt.wantFlag {
				t.Errorf("followLatest = %v, want %v", followLatest, tt.wantFlag)
			}
		})
	}
}

// TestFollowLatestRequiresWatchMode tests Story 11.2 AC-5: validation that
// --follow-latest requires --watch mode.
func TestFollowLatestRequiresWatchMode(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantValid bool
	}{
		{
			name:      "L with watch is valid",
			args:      []string{"cmd", "-w", "-L", "file.jsonl"},
			wantValid: true,
		},
		{
			name:      "follow-latest with watch is valid",
			args:      []string{"cmd", "--watch", "--follow-latest", "file.jsonl"},
			wantValid: true,
		},
		{
			name:      "L without watch is invalid",
			args:      []string{"cmd", "-L", "file.jsonl"},
			wantValid: false,
		},
		{
			name:      "follow-latest without watch is invalid",
			args:      []string{"cmd", "--follow-latest", "file.jsonl"},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(tt.args[0], flag.ContinueOnError)

			watchFlag := flag.Bool("watch", false, "Watch file for changes")
			wFlag := flag.Bool("w", false, "Watch file for changes (shorthand)")
			liveFlag := flag.Bool("live", false, "Alias for --watch")
			followLatestFlag := flag.Bool("follow-latest", false, "Follow to newest conversation")
			followLatestShortFlag := flag.Bool("L", false, "Follow to newest conversation")

			err := flag.CommandLine.Parse(tt.args[1:])
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			watchMode := *watchFlag || *wFlag || *liveFlag
			followLatest := *followLatestFlag || *followLatestShortFlag

			isValid := !followLatest || watchMode

			if isValid != tt.wantValid {
				t.Errorf("isValid = %v, want %v", isValid, tt.wantValid)
			}
		})
	}
}

// TestPrintHelpFollowLatestFlag tests Story 11.2 AC-6: verify follow-latest flag appears in help.
func TestPrintHelpFollowLatestFlag(t *testing.T) {
	originalOutput := flag.CommandLine.Output()
	t.Cleanup(func() {
		flag.CommandLine.SetOutput(originalOutput)
	})

	var buf bytes.Buffer
	flag.CommandLine.SetOutput(&buf)
	printHelp()

	output := buf.String()

	if !strings.Contains(output, "-L, --follow-latest") {
		t.Error("help output should contain '-L, --follow-latest' showing both flag forms together")
	}

	if !strings.Contains(output, "-w -L") {
		t.Error("help output should contain example with '-w -L'")
	}

	if !strings.Contains(output, "Toggle follow-latest") {
		t.Error("help output should document 'L' key toggle for follow-latest in keyboard shortcuts")
	}
}

// TestStreamingCodexIncrementalRead verifies that the Codex format's incremental
// streaming loop reads new raw bytes and parses them with the provider,
// producing renderable LogEntry entries.
func TestStreamingCodexIncrementalRead(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	initialContent := `{"type":"session_meta","timestamp":"2026-04-30T10:00:00Z","payload":{"id":"sess-inc","cwd":"/proj","model":"o3"}}
{"type":"event_msg","timestamp":"2026-04-30T10:00:01Z","payload":{"event":{"type":"user_message","content":"First message"}}}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Parse initial content to get end position (simulating runStreamingPlainMode)
	absPath, _ := filepath.Abs(testFile)
	file, err := os.Open(absPath)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	detected, newReader, detectErr := detectFormatFromReader(file)
	if detectErr != nil {
		_ = file.Close()
		t.Fatalf("detectFormatFromReader error: %v", detectErr)
	}
	if detected != agent.AgentCodex {
		_ = file.Close()
		t.Fatalf("detected = %q, want %q", detected, agent.AgentCodex)
	}

	provider := selectProvider(detected)
	convEntries, _ := provider.ParseSessionStream(newReader)
	initialEntries := convertConversationEntries(convEntries)
	_ = file.Close()

	if len(initialEntries) == 0 {
		t.Fatal("initial parse should produce entries")
	}

	// Verify initial entries render
	rendered := tui.RenderPlain(initialEntries, filepath.Base(testFile), tui.RenderOptions{})
	if !strings.Contains(rendered, "First message") {
		t.Error("initial RenderPlain should contain 'First message'")
	}

	// Now test incremental read with watcher (same path as streaming loop)
	w, err := watcher.NewWithPosition(absPath, int64(len(initialContent)))
	if err != nil {
		t.Fatalf("NewWithPosition failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Append new Codex entries
	appended := `{"type":"event_msg","timestamp":"2026-04-30T10:00:05Z","payload":{"event":{"type":"agent_message","content":"Incremental reply","phase":"final"}}}
`
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to append: %v", err)
	}
	_, _ = f.WriteString(appended)
	_ = f.Close()

	// Simulate the non-Claude-Code streaming loop: ReadNewRawBytes + provider parsing
	rawBytes, err := w.ReadNewRawBytes()
	if err != nil {
		t.Fatalf("ReadNewRawBytes error: %v", err)
	}
	if len(rawBytes) == 0 {
		t.Fatal("ReadNewRawBytes should return appended data")
	}
	if !bytes.Contains(rawBytes, []byte("Incremental reply")) {
		t.Errorf("raw bytes should contain 'Incremental reply', got: %q", string(rawBytes))
	}

	// Parse the raw bytes with the Codex provider
	convEntries, parseErr := provider.ParseSessionStream(bytes.NewReader(rawBytes))
	if parseErr != nil {
		t.Fatalf("ParseSessionStream error on incremental data: %v", parseErr)
	}
	if len(convEntries) == 0 {
		t.Fatal("ParseSessionStream should return entries for appended Codex data")
	}

	incEntries := convertConversationEntries(convEntries)
	renderedEntry := tui.RenderEntryPlain(incEntries[0], tui.RenderOptions{})
	if !strings.Contains(renderedEntry, "Incremental reply") {
		t.Errorf("RenderEntryPlain should contain 'Incremental reply', got: %q", renderedEntry)
	}
}

// TestStreamingOpenCodeIncrementalRead verifies that OpenCode format's provider
// is correctly selected and that the streaming pipeline handles the OpenCode case.
// Since OpenCode stores sessions in SQLite, ParseSessionStream errors for file-based
// data — this test validates provider selection and the error path.
func TestStreamingOpenCodeIncrementalRead(t *testing.T) {
	// Verify selectProvider returns an OpenCode provider
	provider := selectProvider(agent.AgentOpenCode)
	if provider.Type() != agent.AgentOpenCode {
		t.Fatalf("selectProvider(AgentOpenCode).Type() = %q, want %q", provider.Type(), agent.AgentOpenCode)
	}

	// OpenCode ParseSessionStream should error for non-SQLite input
	_, err := provider.ParseSessionStream(strings.NewReader("{}"))
	if err == nil {
		t.Error("ParseSessionStream on non-SQLite data should return an error for OpenCode")
	}
	if err != nil && !strings.Contains(err.Error(), "ParseSessionStream not supported") {
		t.Errorf("ParseSessionStream error = %q, want error containing 'ParseSessionStream not supported'", err.Error())
	}

	// Verify that incremental raw bytes still work for OpenCode-selected format
	// (even though ParseSessionStream fails, the watcher layer is format-agnostic)
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	initialContent := "line1\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := watcher.NewWithPosition(testFile, int64(len(initialContent)))
	if err != nil {
		t.Fatalf("NewWithPosition failed: %v", err)
	}
	defer func() { _ = w.Close() }()

	appended := "line2\n"
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open for append: %v", err)
	}
	_, _ = f.WriteString(appended)
	_ = f.Close()

	rawBytes, err := w.ReadNewRawBytes()
	if err != nil {
		t.Fatalf("ReadNewRawBytes error: %v", err)
	}
	if string(rawBytes) != appended {
		t.Errorf("ReadNewRawBytes() = %q, want %q", string(rawBytes), appended)
	}
}

// TestStreamingBranchSelection verifies that both streaming code paths
// (Claude Code: ReadNewEntries, and non-Claude Code: ReadNewRawBytes + ParseSessionStream)
// produce correct results through the watcher layer.
func TestStreamingBranchSelection(t *testing.T) {
	t.Run("ClaudeCode_uses_ReadNewEntries", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.jsonl")
		initialContent := `{"type":"user","message":{"role":"user","content":"Hello"},"timestamp":"2026-01-16T10:00:00Z"}
`
		if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Start watcher at end of initial content (Claude Code streaming path)
		w, err := watcher.NewWithPosition(testFile, int64(len(initialContent)))
		if err != nil {
			t.Fatalf("NewWithPosition failed: %v", err)
		}
		defer func() { _ = w.Close() }()

		// Append Claude Code format data
		appended := `{"type":"assistant","message":{"content":[{"type":"text","text":"World"}]},"timestamp":"2026-01-16T10:01:00Z"}
`
		f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("failed to append: %v", err)
		}
		_, _ = f.WriteString(appended)
		_ = f.Close()

		// Claude Code path: ReadNewEntries parses JSONL directly
		entries, err := w.ReadNewEntries()
		if err != nil {
			t.Fatalf("ReadNewEntries error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("ReadNewEntries should return 1 entry, got %d", len(entries))
		}
		// Verify the parsed entry has the correct content
		text := entries[0].Message.TextContent
		if len(entries[0].Message.Content) > 0 {
			text = entries[0].Message.Content[0].Text
		}
		if !strings.Contains(text, "World") {
			t.Errorf("entry content = %q, want to contain 'World'", text)
		}
	})

	t.Run("Codex_uses_ReadNewRawBytes_plus_ParseSessionStream", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.jsonl")
		initialContent := `{"type":"session_meta","timestamp":"2026-04-30T10:00:00Z","payload":{"id":"sess-test","cwd":"/proj","model":"o3"}}
`
		if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		w, err := watcher.NewWithPosition(testFile, int64(len(initialContent)))
		if err != nil {
			t.Fatalf("NewWithPosition failed: %v", err)
		}
		defer func() { _ = w.Close() }()

		// Append Codex format data
		appended := `{"type":"event_msg","timestamp":"2026-04-30T10:00:05Z","payload":{"event":{"type":"agent_message","content":"Branch reply","phase":"final"}}}
`
		f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("failed to append: %v", err)
		}
		_, _ = f.WriteString(appended)
		_ = f.Close()

		// Codex path: ReadNewRawBytes then provider parsing
		rawBytes, err := w.ReadNewRawBytes()
		if err != nil {
			t.Fatalf("ReadNewRawBytes error: %v", err)
		}
		if len(rawBytes) == 0 {
			t.Fatal("ReadNewRawBytes should return appended data")
		}

		provider := selectProvider(agent.AgentCodex)
		convEntries, parseErr := provider.ParseSessionStream(bytes.NewReader(rawBytes))
		if parseErr != nil {
			t.Fatalf("ParseSessionStream error: %v", parseErr)
		}
		entries := convertConversationEntries(convEntries)
		if len(entries) == 0 {
			t.Fatal("convertConversationEntries should produce entries from Codex data")
		}

		rendered := tui.RenderEntryPlain(entries[0], tui.RenderOptions{})
		if !strings.Contains(rendered, "Branch reply") {
			t.Errorf("rendered entry should contain 'Branch reply', got: %q", rendered)
		}
	})
}
