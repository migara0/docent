package library

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	debounce     = 60 * time.Millisecond
	maxTargeted  = 300 // more changed files than this (a branch switch, a bulk save) is cheaper to handle with a full scan
	pollInterval = 4 * time.Second
)

// Watch keeps the index current until ctx ends. It uses the kernel's file notifications (no polling, no idle CPU).
// If the system's watch limit is reached it falls back to a slow poll, so a very large folder still works.
func (s *Source) Watch(ctx context.Context) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("file watching unavailable, polling instead", "source", s.ID, "err", err)
		s.poll(ctx)
		return
	}
	defer w.Close()

	if !s.addDirs(w) {
		slog.Warn("too many folders to watch, polling instead (raise fs.inotify.max_user_watches to avoid this)", "source", s.ID)
		s.poll(ctx)
		return
	}

	var timer *time.Timer
	var fire <-chan time.Time
	pending := map[string]struct{}{} // changed Markdown files, relative to the root
	needFull := false                // a folder was created, removed or renamed: only a walk can tell what changed
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if !s.relevant(ev) {
				continue
			}
			if isMarkdown(ev.Name) {
				if rel, err := filepath.Rel(s.Root, ev.Name); err == nil {
					pending[filepath.ToSlash(rel)] = struct{}{}
				}
			} else {
				needFull = true // no extension: a folder
				if ev.Has(fsnotify.Create) {
					s.addDirs(w)
				}
			}
			if timer == nil {
				timer = time.NewTimer(debounce)
			} else {
				timer.Reset(debounce)
			}
			fire = timer.C
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			slog.Warn("watch error", "source", s.ID, "err", err)
		case <-fire:
			fire = nil
			if needFull || len(pending) > maxTargeted {
				if _, err := s.Rescan(); err != nil {
					slog.Warn("rescan failed", "source", s.ID, "err", err)
				}
			} else if len(pending) > 0 {
				paths := make([]string, 0, len(pending))
				for p := range pending {
					paths = append(paths, p)
				}
				s.Refresh(paths)
			}
			clear(pending)
			needFull = false
		}
	}
}

// relevant filters out editor scratch files and anything that is not a Markdown file or a possible folder.
func (s *Source) relevant(ev fsnotify.Event) bool {
	name := filepath.Base(ev.Name)
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") {
		return false
	}
	if s.File != "" && name != s.File {
		return false
	}
	return isMarkdown(name) || filepath.Ext(name) == "" // no extension: could be a folder that was created, renamed or removed
}

// addDirs watches the source's folders. It returns false when the system limit stops it.
func (s *Source) addDirs(w *fsnotify.Watcher) bool {
	if s.File != "" {
		return w.Add(s.Root) == nil
	}
	ok := true
	_ = filepath.WalkDir(s.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if p != s.Root && (strings.HasPrefix(d.Name(), ".") || ignoredDir[d.Name()]) {
			return filepath.SkipDir
		}
		if err := w.Add(p); err != nil && isLimit(err) {
			ok = false
			return filepath.SkipAll
		}
		return nil
	})
	return ok
}

func isLimit(err error) bool {
	return strings.Contains(err.Error(), "no space left on device") || strings.Contains(err.Error(), "too many open files")
}

func (s *Source) poll(ctx context.Context) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := s.Rescan(); err != nil {
				slog.Warn("rescan failed", "source", s.ID, "err", err)
			}
		}
	}
}
