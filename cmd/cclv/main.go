// Package main is the entry point for the cclv application.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/tui"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/usage"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

func init() {
	flag.Usage = printHelp
}

func printHelp() {
	w := flag.CommandLine.Output()
	_, _ = fmt.Fprint(w, `cclv - Claude Code Log Viewer

USAGE:
  cclv                          Interactive mode - browse all projects
  cclv [options] <file>         View a specific conversation file
  cat file.jsonl | cclv         Read from stdin

OPTIONS:
  --plain           Output plain text without TUI (for piping)
  --tui             Force interactive TUI mode even when piped
  --color=MODE      Color mode: auto (default), always, never
  --hide-thoughts   Hide Claude's thinking blocks
  --hide-tools      Hide tool use/result blocks
  --width=N         Set rendering width (40-500, 0=auto)
  --watch           Watch file for changes (real-time monitoring)
  --live            Alias for --watch
  -u, --usage       Print usage limits and exit (no TUI)
  -v, --version     Print version information
  -h, --help        Show this help message

EXAMPLES:
  cclv                                    Browse all Claude projects
  cclv conversation.jsonl                 View a conversation file
  cat file.jsonl | cclv                   Read from stdin
  cclv --plain file.jsonl | less          Pipeline with pager
  cclv --hide-thoughts --hide-tools file.jsonl  Show only messages
  cclv --width=100 file.jsonl             Fixed 100-char width
  cclv --color=always file.jsonl | less -R  Force colors in pipe
  cclv --watch conversation.jsonl           Monitor live session
  cclv --usage                              Quick check on limits
  cclv -u                                   Shorthand for --usage

KEYBOARD SHORTCUTS (TUI mode):
  Navigation:   j/k             Move up/down
                gg/G            Jump to top/bottom
                h/esc           Go back
                l/enter         Select / go forward

  Scrolling:    d/u, Ctrl+d/u   Half-page down/up
                PgDn/PgUp/Space Page down/up
                Home/End        Jump to start/end

  Toggles:      t               Toggle thinking blocks
                i               Toggle tool inputs
                w               Toggle watch mode

  Search:       /               Start search
                n/N             Next/previous match

  Actions:      enter           Select item
                q, Ctrl+c       Quit
`)
}

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
	watchFlag := flag.Bool("watch", false, "Watch file for changes (real-time monitoring)")
	liveFlag := flag.Bool("live", false, "Alias for --watch")
	usageFlag := flag.Bool("usage", false, "Print usage limits and exit")
	usageShortFlag := flag.Bool("u", false, "Print usage limits and exit (shorthand)")
	flag.Parse()

	// Combine watch flags
	watchMode := *watchFlag || *liveFlag

	// Handle version flag - print and exit before any other processing
	if *versionFlag || *versionShortFlag {
		fmt.Println(version.String())
		os.Exit(0)
	}

	// Handle usage flag - print usage limits and exit
	if *usageFlag || *usageShortFlag {
		if err := runUsageMode(*colorFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Configure color output based on --color flag
	configureColorOutput(*colorFlag)

	// Get remaining args after flag parsing
	args := flag.Args()

	// Detect TTY status
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	stdoutTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// Validate watch mode requires a file argument (cannot watch stdin or interactive mode)
	if watchMode && len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: --watch requires a file path argument (cannot watch stdin)\n")
		os.Exit(1)
	}

	// Handle streaming plain mode (--watch --plain)
	if watchMode && *plainFlag {
		// Validate and apply width before streaming (color already configured at line 157)
		validatedWidth := validateWidth(*widthFlag)
		opts := tui.RenderOptions{
			HideThoughts: *hideThoughtsFlag,
			HideTools:    *hideToolsFlag,
			Width:        validatedWidth,
		}
		if err := runStreamingPlainMode(args[0], opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

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
		WatchMode:    watchMode,
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
		// Make path absolute for file watching
		absPath, err := filepath.Abs(filePath)
		if err == nil {
			opts.FilePath = absPath
		} else {
			opts.FilePath = filePath // Fall back to original path
		}
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

	// TUI mode - nil token service for pipeline mode (no statistics needed)
	model := tui.NewViewerModel(result.Entries, result.ParseErrors, source, opts, nil)

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

// runUsageMode fetches and displays usage limits in plain text.
func runUsageMode(colorMode string) error {
	// Configure color output first
	configureColorOutput(colorMode)

	// Get OAuth token
	token, err := usage.GetOAuthToken()
	if err != nil {
		return err
	}

	// Create client and fetch usage
	client := usage.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	limits, _, err := client.FetchUsage(ctx, token)
	if err != nil {
		return err
	}

	// Render and print
	output := tui.RenderUsagePlain(limits)
	fmt.Print(output)
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

// Polling interval for streaming mode
const streamingPollInterval = 100 * time.Millisecond

// runStreamingPlainMode outputs formatted entries continuously.
// It renders existing entries, then watches for new ones.
func runStreamingPlainMode(filePath string, opts tui.RenderOptions) error {
	// 1. Parse and render existing entries
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	result := parser.ParseJSONL(file)
	endPos, _ := file.Seek(0, io.SeekCurrent)
	_ = file.Close()

	source := filepath.Base(filePath)
	output := tui.RenderPlain(result.Entries, source, opts)
	fmt.Print(output)
	_ = os.Stdout.Sync()

	// 2. Create watcher starting from current position
	w, err := watcher.NewWithPosition(absPath, endPos)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer w.Close()

	// 3. Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 4. Event loop using watcher's exported method
	for {
		select {
		case <-sigChan:
			signal.Stop(sigChan)
			return nil // Clean exit
		default:
			entries, err := w.ReadNewEntries()
			if err != nil {
				if errors.Is(err, watcher.ErrFileTruncated) {
					// File reset - not an error for streaming, continue watching
					continue
				}
				return err
			}
			for _, entry := range entries {
				fmt.Print(tui.RenderEntryPlain(entry, opts))
				_ = os.Stdout.Sync()
			}
			time.Sleep(streamingPollInterval)
		}
	}
}
