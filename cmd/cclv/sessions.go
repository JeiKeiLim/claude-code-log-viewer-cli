// Package main provides the sessions mode for the cclv application.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/tui"
)

// runSessionsMode launches the session dashboard for the current directory.
// It auto-detects active Claude Code sessions and displays them in split-view panes.
func runSessionsMode() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Derive the Claude project directory from the CWD
	projectDir := session.CWDToProjectDir(cwd)
	if projectDir == "" {
		return fmt.Errorf("could not determine Claude project directory for %s", cwd)
	}

	// Create and run the session dashboard via the integrated AppModel
	// This provides usage bar, navigation back to projects, and viewer support
	model := tui.NewAppModelForSessions(cwd, projectDir)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running session dashboard: %w", err)
	}

	return nil
}
