package devnotes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// A note ID is the library's prefix and a four-digit number: DOC-0007. It never changes when a note is renamed, and a number is never reused.

var idPattern = regexp.MustCompile(`^([A-Z]{2,5})-(\d{4,})$`)

// ParseID splits a well-formed ID.
func ParseID(id string) (prefix string, n int, ok bool) {
	m := idPattern.FindStringSubmatch(id)
	if m == nil {
		return "", 0, false
	}
	n, _ = strconv.Atoi(m[2])
	return m[1], n, true
}

// FormatID builds an ID.
func FormatID(prefix string, n int) string { return fmt.Sprintf("%s-%04d", prefix, n) }

var loose = regexp.MustCompile(`(?i)^\s*(?:([a-z]{2,5})-)?(\d{1,6})\s*$`)

// NormalizeID accepts the loose forms people type ("7", "0007", "doc-7") and returns the canonical ID, using defaultPrefix when none is given.
func NormalizeID(text, defaultPrefix string) (string, bool) {
	m := loose.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	prefix := strings.ToUpper(m[1])
	if prefix == "" {
		prefix = defaultPrefix
	}
	if prefix == "" {
		return "", false // a bare number needs a prefix to mean anything
	}
	n, _ := strconv.Atoi(m[2])
	return FormatID(prefix, n), true
}

// Registry hands out new IDs. It remembers the highest number ever given out in a file (so deleting a note never frees its number)
// and takes a file lock (so two processes can never hand out the same one).
type Registry struct {
	Dir     string // where the per-library files live (the config folder)
	Library string // library id
	Prefix  string
}

type registryFile struct {
	Prefix    string `json:"prefix"`
	HighWater int    `json:"highWater"`
}

// Next returns the next free ID: one above the highest of every ID found in the notes and every ID ever given out here.
func (r Registry) Next(existing []string) (string, error) {
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return "", err
	}
	lock, err := os.OpenFile(filepath.Join(r.Dir, r.Library+".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return "", err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	path := filepath.Join(r.Dir, r.Library+".json")
	var f registryFile
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &f)
	}
	high := f.HighWater
	for _, id := range existing {
		if p, n, ok := ParseID(id); ok && p == r.Prefix && n > high {
			high = n
		}
	}
	high++
	out, _ := json.Marshal(registryFile{Prefix: r.Prefix, HighWater: high})
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return FormatID(r.Prefix, high), nil
}

// Peek returns what Next would return, without reserving it.
func (r Registry) Peek(existing []string) string {
	high := 0
	if data, err := os.ReadFile(filepath.Join(r.Dir, r.Library+".json")); err == nil {
		var f registryFile
		if json.Unmarshal(data, &f) == nil {
			high = f.HighWater
		}
	}
	for _, id := range existing {
		if p, n, ok := ParseID(id); ok && p == r.Prefix && n > high {
			high = n
		}
	}
	return FormatID(r.Prefix, high+1)
}
