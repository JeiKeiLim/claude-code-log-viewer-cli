// Package main is the entry point for the cclv application.
package main

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/tui"
)

func main() {
	// Detect if stdin is a TTY
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	// Determine mode based on TTY and arguments
	args := os.Args[1:]

	if !isTTY || len(args) > 0 {
		// Pipeline mode: stdin is piped or file argument provided
		if err := runPipelineMode(args, isTTY); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Interactive mode: launch project browser
		if err := runInteractiveMode(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// runPipelineMode handles viewing logs from stdin or a file argument.
func runPipelineMode(args []string, isTTY bool) error {
	var reader io.Reader

	if len(args) > 0 {
		// File argument provided
		filePath := args[0]
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		reader = file
	} else {
		// Read from stdin
		reader = os.Stdin
	}

	// Parse the JSONL content
	result := parser.ParseJSONL(reader)

	if len(result.Entries) == 0 {
		if result.ParseErrors > 0 {
			return fmt.Errorf("no valid entries found (%d parse errors)", result.ParseErrors)
		}
		return fmt.Errorf("no entries found in input")
	}

	// Create and run the viewer
	model := tui.NewViewerModel(result.Entries, result.ParseErrors)

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
