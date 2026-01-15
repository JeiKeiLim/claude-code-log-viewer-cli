// Package tui provides the terminal user interface components.
package tui

import (
	"strings"
	"testing"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestBuildModeSegment(t *testing.T) {
	tests := []struct {
		name      string
		watchMode bool
		want      string
	}{
		{
			name:      "watch mode disabled returns empty",
			watchMode: false,
			want:      "",
		},
		{
			name:      "watch mode enabled returns LIVE",
			watchMode: true,
			want:      "LIVE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{watchMode: tt.watchMode}
			got := m.buildModeSegment()

			if tt.want == "" && got != "" {
				t.Errorf("buildModeSegment() = %q, want empty string", got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("buildModeSegment() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestBuildPositionSegment(t *testing.T) {
	tests := []struct {
		name    string
		entries int
		want    string
	}{
		{
			name:    "empty entries shows 0/0",
			entries: 0,
			want:    "0/0",
		},
		{
			name:    "single entry shows 1/1",
			entries: 1,
			want:    "1/1",
		},
		{
			name:    "multiple entries shows Entry format",
			entries: 42,
			want:    "/42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := make([]types.LogEntry, tt.entries)
			m := NewViewerModel(entries, 0, "Test")

			got := m.buildPositionSegment()
			if !strings.Contains(got, tt.want) {
				t.Errorf("buildPositionSegment() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestBuildShortcutsSegment(t *testing.T) {
	tests := []struct {
		name          string
		searchMatches []int
		canGoBack     bool
		wantContains  []string
	}{
		{
			name:          "basic shortcuts always present",
			searchMatches: nil,
			canGoBack:     false,
			wantContains:  []string{"j/k:scroll", "q:quit", "t:thinking", "i:inputs"},
		},
		{
			name:          "search navigation when matches exist",
			searchMatches: []int{1, 2, 3},
			canGoBack:     false,
			wantContains:  []string{"n/N:next/prev"},
		},
		{
			name:          "back navigation when canGoBack true",
			searchMatches: nil,
			canGoBack:     true,
			wantContains:  []string{"h/esc:back"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ViewerModel{
				searchMatches: tt.searchMatches,
				canGoBack:     tt.canGoBack,
			}

			got := m.buildShortcutsSegment()
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("buildShortcutsSegment() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}
