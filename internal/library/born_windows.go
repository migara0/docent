//go:build windows

package library

import (
	"os"
	"syscall"
	"time"
)

// birthTime is when the file was created on this disk (NTFS records it), or zero when it cannot be read.
func birthTime(path string) time.Time {
	info, err := os.Lstat(path)
	if err != nil {
		return time.Time{}
	}
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, d.CreationTime.Nanoseconds())
	}
	return time.Time{}
}
