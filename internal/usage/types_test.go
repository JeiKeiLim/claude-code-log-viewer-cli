package usage

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUsageWindowUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name            string
		data            string
		wantUtilization float64
		wantResetsAt    bool // whether ResetsAt should be non-nil
		wantResetsAtRaw string
		wantErr         bool
	}{
		{
			name:            "full response with valid resets_at",
			data:            `{"utilization": 35.0, "resets_at": "2026-01-20T18:00:00Z"}`,
			wantUtilization: 35.0,
			wantResetsAt:    true,
			wantResetsAtRaw: "2026-01-20T18:00:00Z",
		},
		{
			name:            "utilization only - no resets_at",
			data:            `{"utilization": 12.5}`,
			wantUtilization: 12.5,
			wantResetsAt:    false,
			wantResetsAtRaw: "",
		},
		{
			name:            "null resets_at",
			data:            `{"utilization": 0.0, "resets_at": null}`,
			wantUtilization: 0.0,
			wantResetsAt:    false,
			wantResetsAtRaw: "",
		},
		{
			name:            "empty string resets_at",
			data:            `{"utilization": 50.0, "resets_at": ""}`,
			wantUtilization: 50.0,
			wantResetsAt:    false,
			wantResetsAtRaw: "",
		},
		{
			name:            "invalid date format - ResetsAt nil but ResetsAtRaw preserved",
			data:            `{"utilization": 25.0, "resets_at": "not-a-date"}`,
			wantUtilization: 25.0,
			wantResetsAt:    false, // fails to parse
			wantResetsAtRaw: "not-a-date",
		},
		{
			name:            "zero utilization",
			data:            `{"utilization": 0.0, "resets_at": "2026-01-27T00:00:00Z"}`,
			wantUtilization: 0.0,
			wantResetsAt:    true,
			wantResetsAtRaw: "2026-01-27T00:00:00Z",
		},
		{
			name:            "100% utilization",
			data:            `{"utilization": 100.0, "resets_at": "2026-01-20T12:00:00Z"}`,
			wantUtilization: 100.0,
			wantResetsAt:    true,
			wantResetsAtRaw: "2026-01-20T12:00:00Z",
		},
		{
			name:    "invalid JSON",
			data:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w UsageWindow
			err := json.Unmarshal([]byte(tt.data), &w)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if w.Utilization != tt.wantUtilization {
				t.Errorf("Utilization = %v, want %v", w.Utilization, tt.wantUtilization)
			}

			if tt.wantResetsAt {
				if w.ResetsAt == nil {
					t.Error("ResetsAt = nil, want non-nil")
				}
			} else {
				if w.ResetsAt != nil {
					t.Errorf("ResetsAt = %v, want nil", w.ResetsAt)
				}
			}

			if tt.wantResetsAtRaw != "" {
				if w.ResetsAtRaw == nil {
					t.Errorf("ResetsAtRaw = nil, want %q", tt.wantResetsAtRaw)
				} else if *w.ResetsAtRaw != tt.wantResetsAtRaw {
					t.Errorf("ResetsAtRaw = %q, want %q", *w.ResetsAtRaw, tt.wantResetsAtRaw)
				}
			}
		})
	}
}

func TestUsageLimitsUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name             string
		data             string
		wantFiveHour     bool
		wantSevenDay     bool
		wantSevenDayOpus bool
		wantErr          bool
	}{
		{
			name: "full API response",
			data: `{
				"five_hour": {"utilization": 35.0, "resets_at": "2026-01-20T18:00:00Z"},
				"seven_day": {"utilization": 12.0, "resets_at": "2026-01-27T00:00:00Z"},
				"seven_day_opus": {"utilization": 0.0, "resets_at": null},
				"seven_day_oauth_apps": null,
				"iguana_necktie": null
			}`,
			wantFiveHour:     true,
			wantSevenDay:     true,
			wantSevenDayOpus: true,
		},
		{
			name:             "minimal response - only required fields",
			data:             `{"five_hour": {"utilization": 50.0}, "seven_day": {"utilization": 25.0}}`,
			wantFiveHour:     true,
			wantSevenDay:     true,
			wantSevenDayOpus: false,
		},
		{
			name:             "null five_hour",
			data:             `{"five_hour": null, "seven_day": {"utilization": 10.0}}`,
			wantFiveHour:     false,
			wantSevenDay:     true,
			wantSevenDayOpus: false,
		},
		{
			name:             "empty object",
			data:             `{}`,
			wantFiveHour:     false,
			wantSevenDay:     false,
			wantSevenDayOpus: false,
		},
		{
			name: "unknown fields ignored (forward compatibility)",
			data: `{
				"five_hour": {"utilization": 35.0},
				"seven_day": {"utilization": 12.0},
				"some_future_field": {"value": 123},
				"another_field": "test"
			}`,
			wantFiveHour:     true,
			wantSevenDay:     true,
			wantSevenDayOpus: false,
		},
		{
			name:    "invalid JSON",
			data:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var limits UsageLimits
			err := json.Unmarshal([]byte(tt.data), &limits)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantFiveHour && limits.FiveHour == nil {
				t.Error("FiveHour = nil, want non-nil")
			}
			if !tt.wantFiveHour && limits.FiveHour != nil {
				t.Errorf("FiveHour = %v, want nil", limits.FiveHour)
			}

			if tt.wantSevenDay && limits.SevenDay == nil {
				t.Error("SevenDay = nil, want non-nil")
			}
			if !tt.wantSevenDay && limits.SevenDay != nil {
				t.Errorf("SevenDay = %v, want nil", limits.SevenDay)
			}

			if tt.wantSevenDayOpus && limits.SevenDayOpus == nil {
				t.Error("SevenDayOpus = nil, want non-nil")
			}
			if !tt.wantSevenDayOpus && limits.SevenDayOpus != nil {
				t.Errorf("SevenDayOpus = %v, want nil", limits.SevenDayOpus)
			}
		})
	}
}

func TestUsageLimitsFullParsing(t *testing.T) {
	// Test the full response parsing including time values
	data := `{
		"five_hour": {
			"utilization": 35.5,
			"resets_at": "2026-01-20T18:00:00Z"
		},
		"seven_day": {
			"utilization": 12.25,
			"resets_at": "2026-01-27T00:00:00Z"
		}
	}`

	var limits UsageLimits
	if err := json.Unmarshal([]byte(data), &limits); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify FiveHour
	if limits.FiveHour == nil {
		t.Fatal("FiveHour = nil")
	}
	if limits.FiveHour.Utilization != 35.5 {
		t.Errorf("FiveHour.Utilization = %v, want 35.5", limits.FiveHour.Utilization)
	}
	if limits.FiveHour.ResetsAt == nil {
		t.Error("FiveHour.ResetsAt = nil")
	} else {
		expected, _ := time.Parse(time.RFC3339, "2026-01-20T18:00:00Z")
		if !limits.FiveHour.ResetsAt.Equal(expected) {
			t.Errorf("FiveHour.ResetsAt = %v, want %v", limits.FiveHour.ResetsAt, expected)
		}
	}

	// Verify SevenDay
	if limits.SevenDay == nil {
		t.Fatal("SevenDay = nil")
	}
	if limits.SevenDay.Utilization != 12.25 {
		t.Errorf("SevenDay.Utilization = %v, want 12.25", limits.SevenDay.Utilization)
	}
	if limits.SevenDay.ResetsAt == nil {
		t.Error("SevenDay.ResetsAt = nil")
	} else {
		expected, _ := time.Parse(time.RFC3339, "2026-01-27T00:00:00Z")
		if !limits.SevenDay.ResetsAt.Equal(expected) {
			t.Errorf("SevenDay.ResetsAt = %v, want %v", limits.SevenDay.ResetsAt, expected)
		}
	}
}

func TestNewSentinelErrors(t *testing.T) {
	// Verify new sentinel errors have expected messages
	tests := []struct {
		err  error
		want string
	}{
		{ErrAPITimeout, "usage API request timed out"},
		{ErrAPIError, "usage API returned an error"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("error message = %q, want %q", got, tt.want)
			}
		})
	}
}
