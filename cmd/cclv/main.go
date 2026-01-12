// Package main is the entry point for the cclv application.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/tui"
)

// Output mode for display
type outputMode int

const (
	modeTUI outputMode = iota
	modePlain
)

func main() {
	// Parse command-line flags
	plainFlag := flag.Bool("plain", false, "Output plain text without TUI")
	tuiFlag := flag.Bool("tui", false, "Force TUI mode even when stdout is piped")
	flag.Parse()

	// Get remaining args after flag parsing
	args := flag.Args()

	// Detect TTY status
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	stdoutTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// Determine output mode per data-model.md flow:
	// 1. --plain flag? → Plain Mode
	// 2. --tui flag? → TUI Mode
	// 3. stdin is TTY + no args → Interactive Mode (TUI)
	// 4. Otherwise → Pipeline Mode: stdout is TTY? → TUI, else Plain
	var mode outputMode
	if *plainFlag {
		mode = modePlain
	} else if *tuiFlag {
		mode = modeTUI
	} else if stdinTTY && len(args) == 0 {
		// Interactive mode: launch project browser
		if err := runInteractiveMode(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	} else {
		// Pipeline mode: check stdout TTY
		if stdoutTTY {
			mode = modeTUI
		} else {
			mode = modePlain
		}
	}

	// Pipeline/file mode with determined output mode
	if err := runPipelineMode(args, mode); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runPipelineMode handles viewing logs from stdin or a file argument.
func runPipelineMode(args []string, mode outputMode) error {
	var reader io.Reader
	var source string

	if len(args) > 0 {
		// File argument provided
		filePath := args[0]
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		reader = file
		source = filepath.Base(filePath)
	} else {
		// Read from stdin
		reader = os.Stdin
		source = "stdin"
	}

	// Parse the JSONL content
	result := parser.ParseJSONL(reader)

	if len(result.Entries) == 0 {
		if result.ParseErrors > 0 {
			return fmt.Errorf("no valid entries found (%d parse errors)", result.ParseErrors)
		}
		return fmt.Errorf("no entries found in input")
	}

	// Output based on mode
	if mode == modePlain {
		// Plain text output to stdout
		output := tui.RenderPlain(result.Entries, source)
		fmt.Print(output)
		return nil
	}

	// TUI mode
	model := tui.NewViewerModel(result.Entries, result.ParseErrors, source)

	// Use alternate screen buffer for TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run viewer: %w", err)
	}

	return nil
}

// runInteractiveMode launches the interactive project browser.
func runInteractiveMode() error {
	// Scan for projects
	projects, err := scanner.ScanProjects("")
	if err != nil {
		// Show error in the TUI
		model := tui.NewAppModelWithError(err)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("failed to run app: %w", err)
		}
		return nil
	}

	if len(projects) == 0 {
		// Show empty state in the TUI
		model := tui.NewAppModelWithError(fmt.Errorf("no projects found in ~/.claude/projects/"))
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("failed to run app: %w", err)
		}
		return nil
	}

	// Create and run the app with projects
	model := tui.NewAppModel(projects)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run app: %w", err)
	}

	return nil
}
