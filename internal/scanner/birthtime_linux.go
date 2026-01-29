//go:build linux

// Package scanner handles scanning Claude Code project directories.
package scanner

import (
	"os"
	"time"
)

// GetBirthtime returns the file creation time on Linux.
// Linux ext4 supports birthtime via statx, but this requires CGO.
// Returns zero time to trigger fallback to modification time.
// Story 10.3: Exported for use by dashboard's paneDirWatcherEventMsg handler.
func GetBirthtime(info os.FileInfo) time.Time {
	_ = info // unused on Linux
	return time.Time{}
}
