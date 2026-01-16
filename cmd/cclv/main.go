// Package main is the entry point for the cclv application.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/tui"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version"
)

// Output mode for display
type outputMode int

const (
	modeTUI outputMode = iota
	modePlain
)

// Width validation constants
const (
	minWidth     = 40
	maxWidth     = 500
	defaultWidth = 80
)

// validateWidth ensures width is within reasonable bounds.
// Returns the validated width (possibly clamped) and prints warning if clamped.
// Negative values are treated as auto-detect (0).
func validateWidth(w int) int {
	if w <= 0 {
		return 0 // Auto-detect mode (including negative values)
	}
	if w < minWidth {
		fmt.Fprintf(os.Stderr, "Warning: --width %d too small, using %d\n", w, defaultWidth)
		return defaultWidth
	}
	if w > maxWidth {
		fmt.Fprintf(os.Stderr, "Warning: --width %d too large, using %d\n", w, maxWidth)
		return maxWidth
	}
	return w
}

func main() {
	// Parse command-line flags
	plainFlag := flag.Bool("plain", false, "Output plain text without TUI")
	tuiFlag := flag.Bool("tui", false, "Force TUI mode even when stdout is piped")
	colorFlag := flag.String("color", "auto", "Color output: auto, always, never")
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	versionShortFlag := flag.Bool("v", false, "Print version information and exit (shorthand)")
	hideThoughtsFlag := flag.Bool("hide-thoughts", false, "Hide thinking blocks in output")
	hideToolsFlag := flag.Bool("hide-tools", false, "Hide tool use blocks in output")
	widthFlag := flag.Int("width", 0, "Override rendering width (0=auto-detect)")
	flag.Parse()

	// Handle version flag - print and exit before any other processing
	if *versionFlag || *versionShortFlag {
		fmt.Println(version.String())
		os.Exit(0)
	}

	// Configure color output based on --color flag
	configureColorOutput(*colorFlag)

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

	// Validate and apply width override
	validatedWidth := validateWidth(*widthFlag)

	// Create render options from flags
	opts := tui.RenderOptions{
		HideThoughts: *hideThoughtsFlag,
		HideTools:    *hideToolsFlag,
		Width:        validatedWidth,
	}

	// Pipeline/file mode with determined output mode
	if err := runPipelineMode(args, mode, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runPipelineMode handles viewing logs from stdin or a file argument.
func runPipelineMode(args []string, mode outputMode, opts tui.RenderOptions) error {
	var reader io.Reader
	var source string

	if len(args) > 0 {
		// File argument provided
		filePath := args[0]
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer func() { _ = file.Close() }()
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
		output := tui.RenderPlain(result.Entries, source, opts)
		fmt.Print(output)
		return nil
	}

	// TUI mode
	model := tui.NewViewerModel(result.Entries, result.ParseErrors, source, opts)

	// Use alternate screen buffer for TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run viewer: %w", err)
	}

	return nil
}

// configureColorOutput sets the Lipgloss color profile based on the --color flag.
func configureColorOutput(colorMode string) {
	switch colorMode {
	case "always":
		// Force colors even when not a TTY
		lipgloss.SetColorProfile(termenv.TrueColor)
	case "never":
		// Disable colors completely
		lipgloss.SetColorProfile(termenv.Ascii)
	case "auto":
		// Default behavior - Lipgloss auto-detects
		// No action needed, Lipgloss handles this automatically
	default:
		// Invalid option, treat as auto
		fmt.Fprintf(os.Stderr, "Warning: invalid --color value '%s', using 'auto'\n", colorMode)
	}
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
