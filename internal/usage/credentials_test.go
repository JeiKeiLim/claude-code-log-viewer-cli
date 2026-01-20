package usage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseAndValidateCredentials(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr error
	}{
		{
			name: "valid credentials with RFC3339 string",
			data: `{"claudeAiOauth":{"accessToken":"sk-ant-oauth-test","refreshToken":"refresh","expiresAt":"2099-01-01T00:00:00Z"}}`,
			want: "sk-ant-oauth-test",
		},
		{
			name: "valid credentials with Unix ms timestamp (Keychain format)",
			data: `{"claudeAiOauth":{"accessToken":"sk-ant-oauth-test","refreshToken":"refresh","expiresAt":9999999999999}}`,
			want: "sk-ant-oauth-test",
		},
		{
			name:    "malformed JSON",
			data:    `{invalid json}`,
			wantErr: ErrInvalidCredentials,
		},
		{
			name:    "missing claudeAiOauth",
			data:    `{}`,
			wantErr: ErrEmptyToken,
		},
		{
			name:    "empty accessToken",
			data:    `{"claudeAiOauth":{"accessToken":"","refreshToken":"refresh"}}`,
			wantErr: ErrEmptyToken,
		},
		{
			name:    "expired token with RFC3339",
			data:    `{"claudeAiOauth":{"accessToken":"sk-ant-oauth-test","refreshToken":"refresh","expiresAt":"2020-01-01T00:00:00Z"}}`,
			wantErr: ErrTokenExpired,
		},
		{
			name:    "expired token with Unix ms timestamp",
			data:    `{"claudeAiOauth":{"accessToken":"sk-ant-oauth-test","refreshToken":"refresh","expiresAt":1000000000000}}`,
			wantErr: ErrTokenExpired,
		},
		{
			name: "no expiresAt field - assumes valid",
			data: `{"claudeAiOauth":{"accessToken":"sk-ant-oauth-test","refreshToken":"refresh"}}`,
			want: "sk-ant-oauth-test",
		},
		{
			name: "malformed expiresAt string - assumes valid",
			data: `{"claudeAiOauth":{"accessToken":"sk-ant-oauth-test","refreshToken":"refresh","expiresAt":"invalid-date"}}`,
			want: "sk-ant-oauth-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAndValidateCredentials([]byte(tt.data))
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("parseAndValidateCredentials() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("parseAndValidateCredentials() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("parseAndValidateCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTokenExpired(t *testing.T) {
	tests := []struct {
		name         string
		expiresAtRaw string // JSON representation of expiresAt
		want         bool
	}{
		{
			name:         "empty expiresAt - not expired",
			expiresAtRaw: "",
			want:         false,
		},
		{
			name:         "future RFC3339 string - not expired",
			expiresAtRaw: `"` + time.Now().Add(24*time.Hour).Format(time.RFC3339) + `"`,
			want:         false,
		},
		{
			name:         "past RFC3339 string - expired",
			expiresAtRaw: `"` + time.Now().Add(-24*time.Hour).Format(time.RFC3339) + `"`,
			want:         true,
		},
		{
			name:         "future Unix ms timestamp - not expired",
			expiresAtRaw: "9999999999999", // Far future
			want:         false,
		},
		{
			name:         "past Unix ms timestamp - expired",
			expiresAtRaw: "1000000000000", // Sept 2001
			want:         true,
		},
		{
			name:         "malformed string - not expired (fallback)",
			expiresAtRaw: `"not-a-date"`,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oauth := &OAuthToken{
				AccessToken:  "test-token",
				ExpiresAtRaw: json.RawMessage(tt.expiresAtRaw),
			}
			if got := isTokenExpired(oauth); got != tt.want {
				t.Errorf("isTokenExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTokenFromFile(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	credPath := filepath.Join(claudeDir, ".credentials.json")

	tests := []struct {
		name     string
		content  string
		setup    func()
		teardown func()
		want     string
		wantErr  error
	}{
		{
			name:    "valid credentials file",
			content: `{"claudeAiOauth":{"accessToken":"sk-ant-oauth-file","refreshToken":"refresh","expiresAt":"2099-01-01T00:00:00Z"}}`,
			want:    "sk-ant-oauth-file",
		},
		{
			name:    "empty accessToken in file",
			content: `{"claudeAiOauth":{"accessToken":"","refreshToken":"refresh"}}`,
			wantErr: ErrEmptyToken,
		},
		{
			name:    "expired token in file",
			content: `{"claudeAiOauth":{"accessToken":"sk-ant-oauth-file","refreshToken":"refresh","expiresAt":"2020-01-01T00:00:00Z"}}`,
			wantErr: ErrTokenExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test credentials file
			if err := os.WriteFile(credPath, []byte(tt.content), 0600); err != nil {
				t.Fatalf("failed to write test credentials: %v", err)
			}

			// Override HOME for test (t.Setenv handles cleanup automatically)
			t.Setenv("HOME", tmpDir)

			got, err := getTokenFromFile()
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("getTokenFromFile() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("getTokenFromFile() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("getTokenFromFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTokenFromFile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Override HOME to empty directory (no .claude folder)
	t.Setenv("HOME", tmpDir)

	_, err := getTokenFromFile()
	if err != ErrNoCredentials {
		t.Errorf("getTokenFromFile() error = %v, want %v", err, ErrNoCredentials)
	}
}

func TestGetTokenForNonMac_EnvVarPrecedence(t *testing.T) {
	// Create temp directory with valid credentials file
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	credPath := filepath.Join(claudeDir, ".credentials.json")
	fileContent := `{"claudeAiOauth":{"accessToken":"sk-ant-oauth-file","refreshToken":"refresh","expiresAt":"2099-01-01T00:00:00Z"}}`
	if err := os.WriteFile(credPath, []byte(fileContent), 0600); err != nil {
		t.Fatalf("failed to write test credentials: %v", err)
	}

	t.Setenv("HOME", tmpDir)

	tests := []struct {
		name   string
		envVar string
		want   string
	}{
		{
			name:   "env var set - uses env var",
			envVar: "sk-ant-oauth-env",
			want:   "sk-ant-oauth-env",
		},
		{
			name:   "env var empty - uses file",
			envVar: "",
			want:   "sk-ant-oauth-file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", tt.envVar)
			} else {
				// Clear env var - using empty string with Setenv for test cleanup
				t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
				// Actually unset it (Setenv sets to empty, not unset)
				if err := os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN"); err != nil {
					t.Fatalf("failed to unset env var: %v", err)
				}
			}

			got, err := getTokenForNonMac()
			if err != nil {
				t.Errorf("getTokenForNonMac() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("getTokenForNonMac() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTokenFromFile_ReadError(t *testing.T) {
	// Test file permission error (non-NotExist error path)
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	credPath := filepath.Join(claudeDir, ".credentials.json")
	// Create file with valid content but no read permission
	if err := os.WriteFile(credPath, []byte(`{"claudeAiOauth":{"accessToken":"test"}}`), 0000); err != nil {
		t.Fatalf("failed to write test credentials: %v", err)
	}

	t.Setenv("HOME", tmpDir)

	_, err := getTokenFromFile()
	// Should get a wrapped error, not ErrNoCredentials
	if err == nil {
		t.Error("getTokenFromFile() expected error for unreadable file, got nil")
	}
	if err == ErrNoCredentials {
		t.Errorf("getTokenFromFile() error = %v, should not be ErrNoCredentials for permission error", err)
	}
}

func TestGetTokenFromFile_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	credPath := filepath.Join(claudeDir, ".credentials.json")
	// Write malformed JSON
	if err := os.WriteFile(credPath, []byte(`{invalid`), 0600); err != nil {
		t.Fatalf("failed to write test credentials: %v", err)
	}

	t.Setenv("HOME", tmpDir)

	_, err := getTokenFromFile()
	if err != ErrInvalidCredentials {
		t.Errorf("getTokenFromFile() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestCredentialsStruct(t *testing.T) {
	// Test that Credentials struct properly unmarshals all fields
	t.Run("RFC3339 string format", func(t *testing.T) {
		data := `{
			"claudeAiOauth": {
				"accessToken": "test-access",
				"refreshToken": "test-refresh",
				"expiresAt": "2099-12-31T23:59:59Z"
			}
		}`

		var creds Credentials
		if err := json.Unmarshal([]byte(data), &creds); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if creds.ClaudeAiOauth.AccessToken != "test-access" {
			t.Errorf("AccessToken = %q, want %q", creds.ClaudeAiOauth.AccessToken, "test-access")
		}
		if creds.ClaudeAiOauth.RefreshToken != "test-refresh" {
			t.Errorf("RefreshToken = %q, want %q", creds.ClaudeAiOauth.RefreshToken, "test-refresh")
		}

		// Check GetExpiresAtTime parses correctly
		expiresAt := creds.ClaudeAiOauth.GetExpiresAtTime()
		if expiresAt.Year() != 2099 {
			t.Errorf("ExpiresAt year = %d, want 2099", expiresAt.Year())
		}
	})

	t.Run("Unix ms timestamp format (actual Keychain format)", func(t *testing.T) {
		data := `{
			"claudeAiOauth": {
				"accessToken": "test-access",
				"refreshToken": "test-refresh",
				"expiresAt": 1768889845550
			}
		}`

		var creds Credentials
		if err := json.Unmarshal([]byte(data), &creds); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if creds.ClaudeAiOauth.AccessToken != "test-access" {
			t.Errorf("AccessToken = %q, want %q", creds.ClaudeAiOauth.AccessToken, "test-access")
		}

		// Check GetExpiresAtTime parses the Unix ms timestamp correctly
		expiresAt := creds.ClaudeAiOauth.GetExpiresAtTime()
		if expiresAt.IsZero() {
			t.Error("ExpiresAt should not be zero for valid Unix timestamp")
		}
		// 1768889845550 ms = Jan 2026 (approximately)
		if expiresAt.Year() < 2026 || expiresAt.Year() > 2030 {
			t.Errorf("ExpiresAt year = %d, want ~2026", expiresAt.Year())
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors have expected messages
	tests := []struct {
		err  error
		want string
	}{
		{ErrNoCredentials, "no Claude Code credentials found"},
		{ErrKeychainNotFound, "credential not found in Keychain"},
		{ErrKeychainTimeout, "keychain access timed out"},
		{ErrInvalidCredentials, "invalid credentials format"},
		{ErrTokenExpired, "OAuth token has expired - run 'claude' to re-login"},
		{ErrEmptyToken, "credentials file exists but accessToken is empty"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("error message = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetTokenFromKeychain_MockedExecutor(t *testing.T) {
	// Save and restore original executor
	origExecutor := keychainExecutor
	defer func() { keychainExecutor = origExecutor }()

	tests := []struct {
		name       string
		executor   commandExecutor
		want       string
		wantErr    error
		wantErrMsg string
	}{
		{
			name: "success with valid credentials",
			executor: func(_ context.Context) ([]byte, error) {
				return []byte(`{"claudeAiOauth":{"accessToken":"sk-ant-oauth-mock","refreshToken":"refresh","expiresAt":"2099-01-01T00:00:00Z"}}`), nil
			},
			want: "sk-ant-oauth-mock",
		},
		{
			name: "invalid credentials format",
			executor: func(_ context.Context) ([]byte, error) {
				return []byte(`{invalid}`), nil
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "empty token in credentials",
			executor: func(_ context.Context) ([]byte, error) {
				return []byte(`{"claudeAiOauth":{"accessToken":""}}`), nil
			},
			wantErr: ErrEmptyToken,
		},
		{
			name: "expired token in credentials",
			executor: func(_ context.Context) ([]byte, error) {
				return []byte(`{"claudeAiOauth":{"accessToken":"sk-ant-oauth-mock","expiresAt":"2020-01-01T00:00:00Z"}}`), nil
			},
			wantErr: ErrTokenExpired,
		},
		{
			name: "generic error",
			executor: func(_ context.Context) ([]byte, error) {
				return nil, errors.New("some other error")
			},
			wantErrMsg: "keychain access failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keychainExecutor = tt.executor

			got, err := getTokenFromKeychain()
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("getTokenFromKeychain() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if tt.wantErrMsg != "" {
				if err == nil {
					t.Errorf("getTokenFromKeychain() expected error containing %q, got nil", tt.wantErrMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("getTokenFromKeychain() error = %v, want error containing %q", err, tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("getTokenFromKeychain() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("getTokenFromKeychain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandleKeychainError(t *testing.T) {
	tests := []struct {
		name       string
		ctxErr     error
		err        error
		wantErr    error
		wantErrMsg string
	}{
		{
			name:    "timeout - deadline exceeded",
			ctxErr:  context.DeadlineExceeded,
			err:     errors.New("context deadline exceeded"),
			wantErr: ErrKeychainTimeout,
		},
		{
			name:       "generic error - not ExitError",
			ctxErr:     nil,
			err:        errors.New("permission denied"),
			wantErrMsg: "keychain access failed: permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if tt.ctxErr == context.DeadlineExceeded {
				// Simulate timeout by using an already-cancelled context with deadline
				ctx, cancel = context.WithTimeout(context.Background(), 0)
				<-ctx.Done() // Wait for it to timeout
			}
			defer cancel()

			got := handleKeychainError(ctx, tt.err)
			if tt.wantErr != nil {
				if got != tt.wantErr {
					t.Errorf("handleKeychainError() = %v, want %v", got, tt.wantErr)
				}
				return
			}
			if tt.wantErrMsg != "" {
				if got.Error() != tt.wantErrMsg {
					t.Errorf("handleKeychainError() = %v, want %v", got.Error(), tt.wantErrMsg)
				}
			}
		})
	}
}

// TestHandleKeychainError_ExitCode44 tests the exit code 44 path using a real command
func TestHandleKeychainError_ExitCode44(t *testing.T) {
	// Use a shell command that exits with code 44
	cmd := exec.Command("sh", "-c", "exit 44")
	err := cmd.Run()

	if exitErr, ok := err.(*exec.ExitError); ok {
		ctx := context.Background()
		got := handleKeychainError(ctx, exitErr)
		if got != ErrKeychainNotFound {
			t.Errorf("handleKeychainError() with exit 44 = %v, want %v", got, ErrKeychainNotFound)
		}
	} else {
		t.Errorf("expected *exec.ExitError, got %T", err)
	}
}

// TestHandleKeychainError_NonZeroExitCode tests non-44 exit codes
func TestHandleKeychainError_NonZeroExitCode(t *testing.T) {
	// Use a shell command that exits with a non-44 code
	cmd := exec.Command("sh", "-c", "exit 1")
	err := cmd.Run()

	if exitErr, ok := err.(*exec.ExitError); ok {
		ctx := context.Background()
		got := handleKeychainError(ctx, exitErr)
		if got == ErrKeychainNotFound {
			t.Errorf("handleKeychainError() with exit 1 should not return ErrKeychainNotFound")
		}
		if !strings.Contains(got.Error(), "keychain access failed") {
			t.Errorf("handleKeychainError() = %v, want error containing 'keychain access failed'", got)
		}
	} else {
		t.Errorf("expected *exec.ExitError, got %T", err)
	}
}
