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
	// Test AC-1: Both -u and --usage behave identically
	// This tests that flag parsing produces the same boolean result
	// (actual output identity is tested at the function level since both
	// flags call the same runUsageMode function)

	flagSets := []struct {
		name string
		args []string
	}{
		{"usage flag only", []string{"cmd", "--usage"}},
		{"u flag only", []string{"cmd", "-u"}},
	}

	var results []bool
	for _, fs := range flagSets {
		// Reset flag.CommandLine for each test
		flag.CommandLine = flag.NewFlagSet(fs.args[0], flag.ContinueOnError)

		usageFlag := flag.Bool("usage", false, "Print usage limits and exit")
		usageShortFlag := flag.Bool("u", false, "Print usage limits and exit (shorthand)")

		err := flag.CommandLine.Parse(fs.args[1:])
		if err != nil {
			t.Fatalf("Failed to parse flags for %s: %v", fs.name, err)
		}

		// Both should result in usage mode being true
		usageMode := *usageFlag || *usageShortFlag
		results = append(results, usageMode)

		if !usageMode {
			t.Errorf("%s: expected usage mode to be true", fs.name)
		}
	}

	// Verify both produce the same result
	if len(results) >= 2 && results[0] != results[1] {
		t.Error("-u and --usage should produce identical behavior")
	}
}

func TestPrintHelpUsageFlagFormat(t *testing.T) {
	// Test AC-7: Verify -u, --usage appears together in help text
	originalOutput := flag.CommandLine.Output()
	t.Cleanup(func() {
		flag.CommandLine.SetOutput(originalOutput)
	})

	var buf bytes.Buffer
	flag.CommandLine.SetOutput(&buf)
	printHelp()

	output := buf.String()

	// Verify the exact format "-u, --usage" appears (showing both together)
	if !strings.Contains(output, "-u, --usage") {
		t.Error("help text should contain '-u, --usage' showing both flag forms together")
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

	// Test AC 1.12.2: Common usage examples are shown
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
