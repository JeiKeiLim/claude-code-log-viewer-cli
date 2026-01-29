//go:build darwin

// Package scanner handles scanning Claude Code project directories.
package scanner

import (
	"os"
	"syscall"
	"time"
)

// GetBirthtime returns the file creation time (birthtime) on macOS.
// Returns zero time if the type assertion fails (graceful degradation).
// Story 10.3: Exported for use by dashboard's paneDirWatcherEventMsg handler.
func GetBirthtime(info os.FileInfo) time.Time {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return time.Time{}
	}
	return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}
