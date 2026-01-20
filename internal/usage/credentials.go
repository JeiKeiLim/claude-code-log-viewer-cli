package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// keychainTimeout is the maximum duration to wait for Keychain access.
const keychainTimeout = 5 * time.Second

// commandExecutor runs shell commands. Used for testing.
type commandExecutor func(ctx context.Context) ([]byte, error)

// defaultKeychainExecutor is the production implementation using security command.
var defaultKeychainExecutor commandExecutor = func(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "security", "find-generic-password",
		"-s", "Claude Code-credentials", "-w")
	return cmd.Output()
}

// keychainExecutor is the current executor (can be swapped for testing).
var keychainExecutor = defaultKeychainExecutor

// GetOAuthToken retrieves the Claude Code OAuth access token.
// Platform-specific behavior:
// - macOS: Reads from Keychain only (no env var override)
// - Linux/Windows: Checks CLAUDE_CODE_OAUTH_TOKEN env var first, then file
func GetOAuthToken() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return getTokenFromKeychain()
	default:
		return getTokenForNonMac()
	}
}

// getTokenForNonMac handles Linux/Windows credential retrieval.
// Priority: env var > file-based credentials.
func getTokenForNonMac() (string, error) {
	// Check env var first (Linux/Windows only)
	if token := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"); token != "" {
		return token, nil
	}
	return getTokenFromFile()
}

// getTokenFromFile reads credentials from ~/.claude/.credentials.json.
func getTokenFromFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	credPath := filepath.Join(home, ".claude", ".credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoCredentials
		}
		return "", fmt.Errorf("failed to read credentials file: %w", err)
	}

	return parseAndValidateCredentials(data)
}

// getTokenFromKeychain retrieves credentials from macOS Keychain.
func getTokenFromKeychain() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
	defer cancel()

	output, err := keychainExecutor(ctx)
	if err != nil {
		return "", handleKeychainError(ctx, err)
	}

	return parseAndValidateCredentials(output)
}

// handleKeychainError converts Keychain errors to appropriate sentinel errors.
func handleKeychainError(ctx context.Context, err error) error {
	if ctx.Err() == context.DeadlineExceeded {
		return ErrKeychainTimeout
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 44 {
			return ErrKeychainNotFound
		}
	}
	return fmt.Errorf("keychain access failed: %w", err)
}

// parseAndValidateCredentials parses JSON data and validates the token.
func parseAndValidateCredentials(data []byte) (string, error) {
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", ErrInvalidCredentials
	}

	token := creds.ClaudeAiOauth.AccessToken
	if token == "" {
		return "", ErrEmptyToken
	}

	if isTokenExpired(&creds.ClaudeAiOauth) {
		return "", ErrTokenExpired
	}

	return token, nil
}

// isTokenExpired checks if the token has expired using OAuthToken.GetExpiresAtTime().
func isTokenExpired(oauth *OAuthToken) bool {
	expiresAt := oauth.GetExpiresAtTime()
	if expiresAt.IsZero() {
		return false // No expiry or unparseable = assume valid
	}
	return time.Now().After(expiresAt)
}
