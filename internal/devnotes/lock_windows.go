//go:build windows

package devnotes

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFile takes an exclusive lock on f, waiting for another process to release it.
func lockFile(f *os.File) (unlock func(), err error) {
	h := windows.Handle(f.Fd())
	ol := new(windows.Overlapped)
	// the whole file (all 64 bits of length); without LOCKFILE_FAIL_IMMEDIATELY the call waits
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, ^uint32(0), ^uint32(0), ol); err != nil {
		return nil, err
	}
	return func() { _ = windows.UnlockFileEx(h, 0, ^uint32(0), ^uint32(0), ol) }, nil
}
