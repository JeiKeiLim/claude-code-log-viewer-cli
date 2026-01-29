package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

// TestScanConversationsLazyStableSort tests that equal timestamps produce deterministic order
// Story 10.1: AC-2 requires stable sorting with filename tiebreaker
func TestScanConversationsLazyStableSort(t *testing.T) {
	dir := t.TempDir()

	// Create files with exact same modification time
	files := []string{"aaa.jsonl", "ccc.jsonl", "bbb.jsonl"}
	commonTime := time.Now().Truncate(time.Second) // Truncate to ensure equal

	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
		// Set identical modification time
		if err := os.Chtimes(path, commonTime, commonTime); err != nil {
			t.Fatalf("failed to set time for %s: %v", f, err)
		}
	}

	// Run multiple times to verify determinism
	for i := 0; i < 5; i++ {
		convs, err := ScanConversationsLazy(dir)
		if err != nil {
			t.Fatalf("ScanConversationsLazy() error = %v", err)
		}

		if len(convs) != 3 {
			t.Fatalf("expected 3 conversations, got %d", len(convs))
		}

		// With filename descending tiebreaker, order should be: ccc, bbb, aaa
		expectedOrder := []string{"ccc.jsonl", "bbb.jsonl", "aaa.jsonl"}
		for j, conv := range convs {
			got := filepath.Base(conv.FilePath)
			if got != expectedOrder[j] {
				t.Errorf("run %d, position %d: got %s, want %s", i, j, got, expectedOrder[j])
			}
		}
	}
}

// TestScanConversationsLazySortByModTime tests that conversations are sorted by modification time
func TestScanConversationsLazySortByModTime(t *testing.T) {
	dir := t.TempDir()

	// Create files with different modification times
	baseTime := time.Now().Truncate(time.Second)
	filesAndOffsets := []struct {
		name   string
		offset time.Duration
	}{
		{"old.jsonl", -2 * time.Hour},
		{"newest.jsonl", 0},
		{"middle.jsonl", -1 * time.Hour},
	}

	for _, f := range filesAndOffsets {
		path := filepath.Join(dir, f.name)
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f.name, err)
		}
		modTime := baseTime.Add(f.offset)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("failed to set time for %s: %v", f.name, err)
		}
	}

	convs, err := ScanConversationsLazy(dir)
	if err != nil {
		t.Fatalf("ScanConversationsLazy() error = %v", err)
	}

	if len(convs) != 3 {
		t.Fatalf("expected 3 conversations, got %d", len(convs))
	}

	// Should be sorted newest first: newest, middle, old
	expectedOrder := []string{"newest.jsonl", "middle.jsonl", "old.jsonl"}
	for i, conv := range convs {
		got := filepath.Base(conv.FilePath)
		if got != expectedOrder[i] {
			t.Errorf("position %d: got %s, want %s", i, got, expectedOrder[i])
		}
	}
}
