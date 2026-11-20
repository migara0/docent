package library

import (
	"time"

	"golang.org/x/sys/unix"
)

// birthTime is when the file was created on this disk (statx), or zero when the filesystem does not record it.
func birthTime(path string) time.Time {
	var st unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BTIME, &st); err != nil || st.Mask&unix.STATX_BTIME == 0 {
		return time.Time{}
	}
	return time.Unix(st.Btime.Sec, int64(st.Btime.Nsec))
}
