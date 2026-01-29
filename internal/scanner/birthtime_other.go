//go:build !darwin && !linux && !windows

// Package scanner handles scanning Claude Code project directories.
package scanner

import (
	"os"
	"time"
)

// GetBirthtime returns the file creation time on unsupported platforms.
// Returns zero time to trigger fallback to modification time.
// Story 10.3: Exported for use by dashboard's paneDirWatcherEventMsg handler.
func GetBirthtime(info os.FileInfo) time.Time {
	_ = info // unused on unsupported platforms
	return time.Time{}
}
