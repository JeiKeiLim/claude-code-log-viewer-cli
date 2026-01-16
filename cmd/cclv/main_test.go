package main

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"
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

func TestPrintHelp(t *testing.T) {
	// Save original output and restore after test
	originalOutput := flag.CommandLine.Output()
	t.Cleanup(func() {
		flag.CommandLine.SetOutput(originalOutput)
	})

	// Capture help output
	var buf bytes.Buffer
	flag.CommandLine.SetOutput(&buf)
	printHelp()

	output := buf.String()

	// Test AC 1.12.1: Each flag has a clear description
	requiredFlags := []string{
		"--plain",
		"--tui",
		"--color",
		"--hide-thoughts",
		"--hide-tools",
		"--width",
		"--version",
		"--help",
	}
	for _, f := range requiredFlags {
		if !strings.Contains(output, f) {
			t.Errorf("help output missing flag: %s", f)
		}
	}

	// Test AC 1.12.2: Common usage examples are shown
	requiredExamples := []string{
		"cclv conversation.jsonl",
		"cclv --plain",
		"--hide-thoughts",
	}
	for _, ex := range requiredExamples {
		if !strings.Contains(output, ex) {
			t.Errorf("help output missing example containing: %s", ex)
		}
	}

	// Test AC 1.12.3: Keyboard shortcuts section with groups
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

	// Test keyboard shortcut groups
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

	// Test specific shortcuts are documented
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
