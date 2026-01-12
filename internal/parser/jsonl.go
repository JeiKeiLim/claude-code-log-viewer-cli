// Package parser handles parsing of Claude Code JSONL log files.
package parser

import (
	"bufio"
	"encoding/json"
	"io"
	"os"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// ParseResult contains the parsed entries and any errors encountered.
type ParseResult struct {
	Entries     []types.LogEntry
	ParseErrors int
}

// ParseJSONL reads a JSONL file and returns parsed log entries.
// It skips malformed lines and tracks the number of parse errors.
func ParseJSONL(r io.Reader) ParseResult {
	result := ParseResult{
		Entries: make([]types.LogEntry, 0),
	}

	scanner := bufio.NewScanner(r)
	// Increase buffer size for large lines (tool inputs can be very large)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // Max 1MB per line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		entry, err := ParseEntry(line)
		if err != nil {
			result.ParseErrors++
			continue
		}

		result.Entries = append(result.Entries, entry)
	}

	return result
}

// ParseJSONLFile reads a JSONL file from a file path.
func ParseJSONLFile(filePath string) (ParseResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return ParseResult{}, err
	}
	defer file.Close()

	return ParseJSONL(file), nil
}

// ParseJSONLStream returns a channel that streams parsed entries.
// Useful for large files or streaming input.
func ParseJSONLStream(r io.Reader) (<-chan types.LogEntry, <-chan int) {
	entries := make(chan types.LogEntry)
	errors := make(chan int, 1)

	go func() {
		defer close(entries)
		defer close(errors)

		parseErrors := 0
		scanner := bufio.NewScanner(r)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			// Quick check for relevant entry types
			var raw types.RawLogEntry
			if err := json.Unmarshal(line, &raw); err != nil {
				parseErrors++
				continue
			}

			// Only parse user and assistant entries
			if raw.Type != string(types.EntryTypeUser) && raw.Type != string(types.EntryTypeAssistant) {
				continue
			}

			entry, err := ParseEntry(line)
			if err != nil {
				parseErrors++
				continue
			}

			entries <- entry
		}

		errors <- parseErrors
	}()

	return entries, errors
}
