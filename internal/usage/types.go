// Package usage provides OAuth credential access for Claude Code usage monitoring.
package usage

import "errors"

// Sentinel errors for credential operations.
var (
	ErrNoCredentials      = errors.New("no Claude Code credentials found")
	ErrKeychainNotFound   = errors.New("credential not found in Keychain")
	ErrKeychainTimeout    = errors.New("keychain access timed out")
	ErrInvalidCredentials = errors.New("invalid credentials format")
	ErrTokenExpired       = errors.New("OAuth token has expired - run 'claude' to re-login")
	ErrEmptyToken         = errors.New("credentials file exists but accessToken is empty")
)

// OAuthToken represents the nested OAuth token structure in credentials.
type OAuthToken struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt"`
}

// Credentials represents the Claude Code credentials file format.
type Credentials struct {
	ClaudeAiOauth OAuthToken `json:"claudeAiOauth"`
}
