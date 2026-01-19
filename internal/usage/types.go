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
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt"`
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
