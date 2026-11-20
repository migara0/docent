//go:build unix

package devnotes

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes an exclusive lock on f, waiting for another process to release it.
func lockFile(f *os.File) (unlock func(), err error) {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return nil, err
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }, nil
}
