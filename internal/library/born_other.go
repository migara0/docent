//go:build !linux

package library

import "time"

// birthTime is not read on this platform: notes show the date of their front matter without a time.
func birthTime(string) time.Time { return time.Time{} }
