//go:build windows

// Package scanner handles scanning Claude Code project directories.
package scanner

import (
	"os"
	"syscall"
	"time"
)

// GetBirthtime returns the file creation time on Windows.
// Returns zero time if the type assertion fails (graceful degradation).
// Story 10.3: Exported for use by dashboard's paneDirWatcherEventMsg handler.
func GetBirthtime(info os.FileInfo) time.Time {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || data == nil {
		return time.Time{}
	}
	return time.Unix(0, data.CreationTime.Nanoseconds())
}
