package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Dropped files: a browser cannot tell a page where a dropped file lives on disk, only give its contents, so each drop is saved as a temporary copy in a
// folder of its own, which Docent opens as the "Dropped files" source. The copies are not linked to the originals (editing an original changes nothing here),
// they are never listed among the remembered libraries, and the folder is deleted when Docent exits (and any left by a crash is deleted at the next start).

const (
	maxDropFiles = 50
	maxDropFile  = 5 << 20  // per file
	maxDropTotal = 24 << 20 // per request
)

type dropper struct {
	mu  sync.Mutex
	dir string
}

type dropFile struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type dropRequest struct {
	Files []dropFile `json:"files"`
}

type dropPage struct {
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url"`
}

// dropName accepts only a plain Markdown file name: no folders, no hidden files, no control characters.
func dropName(name string) (string, bool) {
	if name == "" || len(name) > 200 || !utf8.ValidString(name) || strings.HasPrefix(name, ".") {
		return "", false
	}
	if strings.ContainsAny(name, "/\\\x00:*?\"<>|") || strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return "", false
	}
	low := strings.ToLower(name)
	if !strings.HasSuffix(low, ".md") && !strings.HasSuffix(low, ".markdown") {
		return "", false
	}
	return name, true
}

func (d *dropper) folder(base string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dir != "" {
		return d.dir, nil
	}
	var dir string
	var err error
	if base != "" {
		if err = os.MkdirAll(base, 0o755); err == nil {
			dir, err = os.MkdirTemp(base, "drop-")
		}
	} else {
		dir, err = os.MkdirTemp("", "docent-drop-")
	}
	if err != nil {
		return "", err
	}
	d.dir = dir
	return dir, nil
}

// Cleanup deletes the dropped copies. Call it when the server stops.
func (s *Server) Cleanup() {
	s.drops.mu.Lock()
	dir := s.drops.dir
	s.drops.dir = ""
	s.drops.mu.Unlock()
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	s.share.stop()
}

func (s *Server) drop(w http.ResponseWriter, r *http.Request) error {
	if err := s.mutationsAllowed(); err != nil {
		return err
	}
	var req dropRequest
	if err := readJSONLimit(w, r, &req, maxDropTotal+(1<<20)); err != nil {
		return err
	}
	if len(req.Files) == 0 {
		return fail(http.StatusBadRequest, "no files")
	}
	if len(req.Files) > maxDropFiles {
		return fail(http.StatusRequestEntityTooLarge, "at most %d files at a time", maxDropFiles)
	}
	total := 0
	for _, f := range req.Files {
		if _, ok := dropName(f.Name); !ok {
			return fail(http.StatusBadRequest, "%q is not a Markdown file name (.md or .markdown, no folders)", f.Name)
		}
		if len(f.Text) > maxDropFile {
			return fail(http.StatusRequestEntityTooLarge, "%q is larger than %d MB", f.Name, maxDropFile>>20)
		}
		total += len(f.Text)
	}
	if total > maxDropTotal {
		return fail(http.StatusRequestEntityTooLarge, "more than %d MB at once", maxDropTotal>>20)
	}
	dir, err := s.drops.folder(s.opt.DropDir)
	if err != nil {
		return err
	}
	src := s.mgr.OpenDropped(dir)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := src.WaitReady(ctx); err != nil {
		return fail(http.StatusServiceUnavailable, "the dropped files folder is not ready")
	}
	names := make([]string, 0, len(req.Files))
	pages := make([]dropPage, 0, len(req.Files))
	for _, f := range req.Files {
		name, _ := dropName(f.Name)
		// the same name again replaces the copy; a file name is never a path, so it cannot leave the folder
		if err := os.WriteFile(filepath.Join(dir, name), []byte(f.Text), 0o600); err != nil {
			return err
		}
		names = append(names, name)
		pages = append(pages, dropPage{Name: name, Path: name, URL: src.PageURL(name)})
	}
	src.Refresh(names) // indexed now, so the page can be opened straight away
	return ok(w, map[string]any{"source": src.ID, "pages": pages})
}
