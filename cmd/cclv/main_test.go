package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestValidateWidth(t *testing.T) {
	tests := []struct {
		name        string
		input       int
		want        int
		wantWarning bool
	}{
		{"zero returns zero", 0, 0, false},
		{"negative returns zero", -1, 0, false},
		{"valid 50", 50, 50, false},
		{"valid 80", 80, 80, false},
		{"valid 120", 120, 120, false},
		{"valid 500 max", 500, 500, false},
		{"min boundary 40", 40, 40, false},
		{"too small 30", 30, 80, true},
		{"too small 39", 39, 80, true},
		{"too large 501", 501, 500, true},
		{"too large 600", 600, 500, true},
		{"too large 1000", 1000, 500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			got := validateWidth(tt.input)

			_ = w.Close()
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			os.Stderr = oldStderr

			if got != tt.want {
				t.Errorf("validateWidth(%d) = %d, want %d", tt.input, got, tt.want)
			}

			hasWarning := strings.Contains(buf.String(), "Warning")
			if hasWarning != tt.wantWarning {
				t.Errorf("validateWidth(%d) warning = %v, wantWarning %v", tt.input, hasWarning, tt.wantWarning)
			}
		})
	}
}
