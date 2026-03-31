// Package session provides multi-session detection and lifecycle management
// for Claude Code sessions (Phase 5a).
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ParseErrorKind categorizes session file parse failures, allowing callers to
// distinguish between different types of errors and handle them appropriately.
type ParseErrorKind int

const (
	// ParseErrReadFailed indicates the file could not be read (e.g., permission
	// denied, file deleted between detection and read).
	ParseErrReadFailed ParseErrorKind = iota
	// ParseErrInvalidJSON indicates the file contains invalid JSON. This may
	// occur transiently if the file is detected before Claude Code finishes
	// writing it.
	ParseErrInvalidJSON
	// ParseErrMissingField indicates a required field is absent or empty.
	// The Field field of SessionParseError names the missing field.
	ParseErrMissingField
	// ParseErrPIDMismatch indicates that the pid field in the JSON content
	// does not match the PID derived from the filename. This may indicate a
	// stale or misplaced session file.
	ParseErrPIDMismatch
)

// SessionParseError is a structured error returned when parsing a session file
// fails. It carries the kind of failure and, for field validation errors, the
// name of the offending field.
type SessionParseError struct {
	// Kind categorizes the failure.
	Kind ParseErrorKind
	// Field is the JSON field name that caused a ParseErrMissingField failure.
	// Empty for other error kinds.
	Field string
	// Message is a human-readable description of the failure.
	Message string
	// Cause is the underlying error, if any (e.g., an os.PathError or
	// json.SyntaxError). May be nil for validation errors.
	Cause error
}

// Error implements the error interface.
func (e *SessionParseError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("session parse error (field %q): %s", e.Field, e.Message)
	}
	return fmt.Sprintf("session parse error: %s", e.Message)
}

// Unwrap returns the underlying cause, enabling errors.Is / errors.As chains.
func (e *SessionParseError) Unwrap() error {
	return e.Cause
}

// IsParseError returns true if err contains a *SessionParseError anywhere in
// its chain (supports wrapped errors via errors.As).
func IsParseError(err error) bool {
	var pe *SessionParseError
	return errors.As(err, &pe)
}

// ParseSessionFile reads, parses, and validates a {pid}.json session file,
// returning a fully-populated ActiveSession on success.
//
// filenamePID is the PID extracted from the filename (e.g., 12345 for
// "12345.json"). It is used to:
//   - Validate consistency with the optional pid field in the JSON content.
//   - Populate the PID field when it is absent from the JSON content.
//
// Validation rules applied (in order):
//  1. File must be readable (returns ParseErrReadFailed on failure).
//  2. File must contain valid JSON (returns ParseErrInvalidJSON on failure).
//  3. sessionId must be non-empty — it maps directly to the JSONL filename
//     that holds the conversation content (returns ParseErrMissingField).
//  4. If the pid field is present in content, it must equal filenamePID
//     (returns ParseErrPIDMismatch); if absent, filenamePID is used.
//
// Transient errors (file not yet fully written) will surface as
// ParseErrReadFailed or ParseErrInvalidJSON; callers should retry on the next
// write event rather than treating these as permanent failures.
func ParseSessionFile(filePath string, filenamePID int) (ActiveSession, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ActiveSession{}, &SessionParseError{
			Kind:    ParseErrReadFailed,
			Message: fmt.Sprintf("reading file %q", filePath),
			Cause:   err,
		}
	}

	var meta SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return ActiveSession{}, &SessionParseError{
			Kind:    ParseErrInvalidJSON,
			Message: fmt.Sprintf("invalid JSON in %q", filePath),
			Cause:   err,
		}
	}

	// Validate required semantic fields.
	if err := ValidateSessionMeta(meta); err != nil {
		return ActiveSession{}, err
	}

	// Validate PID consistency between filename and JSON content.
	if meta.PID != 0 && meta.PID != filenamePID {
		return ActiveSession{}, &SessionParseError{
			Kind:  ParseErrPIDMismatch,
			Field: "pid",
			Message: fmt.Sprintf(
				"pid in file (%d) does not match filename pid (%d)",
				meta.PID, filenamePID,
			),
		}
	}
	// Populate PID from filename when absent from content.
	if meta.PID == 0 {
		meta.PID = filenamePID
	}

	sess := ActiveSession{
		Meta:     meta,
		FilePath: filePath,
	}
	if meta.CWD != "" {
		sess.JSONLDir = CWDToProjectDir(meta.CWD)
	}

	return sess, nil
}

// ValidateSessionMeta checks that a SessionMeta has all required fields.
// Returns a *SessionParseError if validation fails, nil if valid.
//
// Required fields:
//   - SessionID — must be non-empty. The sessionId maps directly to the JSONL
//     filename that holds the conversation content; without it, the session
//     cannot be associated with any log file.
func ValidateSessionMeta(meta SessionMeta) error {
	if meta.SessionID == "" {
		return &SessionParseError{
			Kind:    ParseErrMissingField,
			Field:   "sessionId",
			Message: "sessionId is required to locate the conversation JSONL file",
		}
	}
	return nil
}
