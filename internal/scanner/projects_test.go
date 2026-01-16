package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountConversations(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dir string) // Setup function for more complex cases
		files []string         // Simple file names to create (for basic cases)
		want  int
	}{
		{
			name:  "empty directory",
			files: nil,
			want:  0,
		},
		{
			name:  "only jsonl files",
			files: []string{"a.jsonl", "b.jsonl"},
			want:  2,
		},
		{
			name:  "mixed file types",
			files: []string{"a.jsonl", "b.txt", "c.jsonl", "d.md"},
			want:  2,
		},
		{
			name:  "single jsonl file",
			files: []string{"only.jsonl"},
			want:  1, // Tests singular path
		},
		{
			name: "nested jsonl not counted",
			setup: func(dir string) {
				// Create a subdirectory with .jsonl files
				subdir := filepath.Join(dir, "subdir")
				if err := os.Mkdir(subdir, 0755); err != nil {
					t.Fatalf("failed to create subdir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dir, "top.jsonl"), []byte{}, 0644); err != nil {
					t.Fatalf("failed to create top.jsonl: %v", err)
				}
				if err := os.WriteFile(filepath.Join(subdir, "nested.jsonl"), []byte{}, 0644); err != nil {
					t.Fatalf("failed to create nested.jsonl: %v", err)
				}
			},
			want: 1, // Only counts top-level
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.setup != nil {
				tt.setup(dir)
			} else {
				for _, f := range tt.files {
					if err := os.WriteFile(filepath.Join(dir, f), []byte{}, 0644); err != nil {
						t.Fatalf("failed to create file %s: %v", f, err)
					}
				}
			}
			got := countConversations(dir)
			if got != tt.want {
				t.Errorf("countConversations() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountConversationsNonExistent(t *testing.T) {
	// Non-existent directory should return 0, not error
	got := countConversations("/nonexistent/path/that/does/not/exist")
	if got != 0 {
		t.Errorf("countConversations(nonexistent) = %d, want 0", got)
	}
}
