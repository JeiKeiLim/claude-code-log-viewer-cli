// Package usage provides OAuth credential access for Claude Code usage monitoring.
package usage

import (
	"encoding/json"
	"errors"
	"time"
)

// Sentinel errors for credential operations.
var (
	ErrNoCredentials      = errors.New("no Claude Code credentials found")
	ErrKeychainNotFound   = errors.New("credential not found in Keychain")
	ErrKeychainTimeout    = errors.New("keychain access timed out")
	ErrInvalidCredentials = errors.New("invalid credentials format")
	ErrTokenExpired       = errors.New("OAuth token has expired - run 'claude' to re-login")
	ErrEmptyToken         = errors.New("credentials file exists but accessToken is empty")
	ErrAPITimeout         = errors.New("usage API request timed out")
	ErrAPIError           = errors.New("usage API returned an error")
)

// OAuthToken represents the nested OAuth token structure in credentials.
type OAuthToken struct {
	AccessToken  string          `json:"accessToken"`
	RefreshToken string          `json:"refreshToken"`
	ExpiresAtRaw json.RawMessage `json:"expiresAt"` // Can be number (Unix ms) or string (RFC3339)
}

// GetExpiresAtTime parses the expiresAt field which can be:
// - Unix timestamp in milliseconds (number): 1768889845550
// - RFC3339 string: "2024-01-20T10:00:00Z"
// Returns zero time if parsing fails or field is empty.
func (t *OAuthToken) GetExpiresAtTime() time.Time {
	if len(t.ExpiresAtRaw) == 0 {
		return time.Time{}
	}

	raw := string(t.ExpiresAtRaw)

	// Try as number (Unix timestamp in milliseconds)
	var ms int64
	if err := json.Unmarshal(t.ExpiresAtRaw, &ms); err == nil {
		return time.UnixMilli(ms)
	}

	// Try as string (RFC3339 or numeric string)
	var str string
	if err := json.Unmarshal(t.ExpiresAtRaw, &str); err == nil {
		// Try RFC3339 first
		if parsed, err := time.Parse(time.RFC3339, str); err == nil {
			return parsed
		}
	}

	// Unparseable - treat as no expiry (let API reject if invalid)
	_ = raw // silence unused warning
	return time.Time{}
}

// Credentials represents the Claude Code credentials file format.
type Credentials struct {
	ClaudeAiOauth OAuthToken `json:"claudeAiOauth"`
}

// UsageLimits represents the API response from /api/oauth/usage.
type UsageLimits struct {
	FiveHour     *UsageWindow `json:"five_hour"`
	SevenDay     *UsageWindow `json:"seven_day"`
	SevenDayOpus *UsageWindow `json:"seven_day_opus,omitempty"`
}

// UsageWindow represents a single usage window (5-hour or 7-day).
type UsageWindow struct {
	Utilization float64    `json:"utilization"` // 0-100 percentage
	ResetsAt    *time.Time `json:"-"`           // Parsed from string
	ResetsAtRaw *string    `json:"resets_at"`   // Raw API response
}

// UnmarshalJSON handles custom parsing for UsageWindow to convert resets_at string to time.Time.
func (w *UsageWindow) UnmarshalJSON(data []byte) error {
	type alias UsageWindow
	aux := &struct {
		ResetsAtRaw *string `json:"resets_at"`
		*alias
	}{
		alias: (*alias)(w),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	w.ResetsAtRaw = aux.ResetsAtRaw
	if aux.ResetsAtRaw != nil && *aux.ResetsAtRaw != "" {
		t, err := time.Parse(time.RFC3339, *aux.ResetsAtRaw)
		if err == nil {
			w.ResetsAt = &t
		}
	}
	return nil
}
