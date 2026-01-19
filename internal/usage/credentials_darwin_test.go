//go:build darwin

package usage

import (
	"testing"
)

// TestKeychainAccess_Integration tests actual Keychain access on macOS.
// This test only runs on macOS and verifies the Keychain integration works.
// Note: This test may fail if Claude Code is not logged in on the test machine.
func TestKeychainAccess_Integration(t *testing.T) {
	// Skip in short mode - this requires actual Keychain access
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	token, err := getTokenFromKeychain()

	// We cannot guarantee the token exists, so we just verify:
	// 1. We don't get a timeout error
	// 2. We get one of the expected error types or a valid token
	if err == ErrKeychainTimeout {
		t.Errorf("getTokenFromKeychain() timed out - this should not happen")
		return
	}

	// Valid outcomes:
	// - ErrKeychainNotFound: Claude Code not logged in (expected on CI)
	// - ErrInvalidCredentials: Credential format changed
	// - ErrEmptyToken: Credentials exist but malformed
	// - ErrTokenExpired: Token exists but expired
	// - nil with valid token: Success
	if err == nil {
		if token == "" {
			t.Errorf("getTokenFromKeychain() returned empty token with nil error")
		}
		t.Logf("Successfully retrieved token from Keychain (length: %d)", len(token))
		return
	}

	// Log which error occurred for debugging
	switch err {
	case ErrKeychainNotFound:
		t.Logf("Claude Code credentials not found in Keychain (expected if not logged in)")
	case ErrInvalidCredentials:
		t.Logf("Keychain credentials have invalid format")
	case ErrEmptyToken:
		t.Logf("Keychain credentials have empty access token")
	case ErrTokenExpired:
		t.Logf("Keychain OAuth token has expired")
	default:
		t.Logf("Keychain access returned error: %v", err)
	}
}

// TestGetOAuthToken_Darwin verifies GetOAuthToken routes to Keychain on macOS.
func TestGetOAuthToken_Darwin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// On macOS, GetOAuthToken should use Keychain (no env var override)
	// Set env var to verify it's NOT used on macOS
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "sk-ant-oauth-should-not-be-used")

	token, err := GetOAuthToken()

	// If we got the env var token, that's a bug - macOS should use Keychain only
	if err == nil && token == "sk-ant-oauth-should-not-be-used" {
		t.Errorf("GetOAuthToken() used env var on macOS - should use Keychain only")
	}

	// Success or expected errors are fine
	if err != nil {
		switch err {
		case ErrKeychainNotFound, ErrInvalidCredentials, ErrEmptyToken, ErrTokenExpired:
			// Expected possible errors
		default:
			if err != ErrKeychainTimeout {
				t.Logf("GetOAuthToken() error (may be expected): %v", err)
			}
		}
	}
}
