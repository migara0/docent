package server

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// The folder picker lets the browser UI choose a folder or file on this machine. It exists only in loopback mode.

type fsEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Kind string `json:"kind"`         // dir or file
	MD   int    `json:"md,omitempty"` // dirs: Markdown files inside (capped), so the list can show "134 .md"
	Cap  bool   `json:"cap,omitempty"`
}

type fsCrumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

const (
	countCap   = 999
	countDepth = 3
)

func (s *Server) browse(w http.ResponseWriter, r *http.Request) error {
	if err := s.mutationsAllowed(); err != nil {
		return err
	}
	dir := r.URL.Query().Get("dir")
	home, _ := os.UserHomeDir()
	switch {
	case dir == "" || dir == "~":
		dir = home
	case strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`):
		dir = filepath.Join(home, dir[2:])
	}
	if !filepath.IsAbs(dir) {
		return fail(http.StatusBadRequest, "dir must be an absolute path")
	}
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fail(http.StatusNotFound, "%s is not a folder", dir)
	}
	showHidden := r.URL.Query().Get("hidden") == "1"
	items, err := os.ReadDir(dir)
	if err != nil {
		return fail(http.StatusForbidden, "cannot read %s: %v", dir, err)
	}
	var entries []fsEntry
	for _, it := range items {
		name := it.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		isDir := it.IsDir()
		if it.Type()&fs.ModeSymlink != 0 {
			if st, err := os.Stat(full); err == nil {
				isDir = st.IsDir()
			}
		}
		switch {
		case isDir:
			entries = append(entries, fsEntry{Name: name, Path: full, Kind: "dir"})
		case strings.HasSuffix(strings.ToLower(name), ".md") || strings.HasSuffix(strings.ToLower(name), ".markdown"):
			entries = append(entries, fsEntry{Name: name, Path: full, Kind: "file"})
		}
	}
	slices.SortFunc(entries, func(a, b fsEntry) int {
		if a.Kind != b.Kind {
			return strings.Compare(a.Kind, b.Kind) // dir before file
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range entries {
		if entries[i].Kind != "dir" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			entries[i].MD, entries[i].Cap = countMarkdown(entries[i].Path)
		}()
	}
	wg.Wait()

	var crumbs []fsCrumb
	for p := dir; ; p = filepath.Dir(p) {
		name := filepath.Base(p)
		if p == home {
			name = "Home"
		} else if v := filepath.VolumeName(p); v != "" && filepath.Dir(p) == p {
			name = v // a drive: "C:", not the "\" filepath.Base gives
		}
		crumbs = append([]fsCrumb{{Name: name, Path: p}}, crumbs...)
		if filepath.Dir(p) == p || p == home {
			break
		}
	}
	var roots []string
	if runtime.GOOS == "windows" {
		roots = windowsDrives()
	}
	return ok(w, map[string]any{"dir": dir, "parent": filepath.Dir(dir), "crumbs": crumbs, "entries": nonNil(entries), "home": home,
		"sep": string(filepath.Separator), "roots": append([]string{}, roots...)})
}

// windowsDrives lists the drives that exist ("C:\", "D:\" ...), so the picker can switch between them.
func windowsDrives() []string {
	var out []string
	for c := 'A'; c <= 'Z'; c++ {
		root := string(c) + `:\`
		if _, err := os.Stat(root); err == nil {
			out = append(out, root)
		}
	}
	return out
}

func nonNil(e []fsEntry) []fsEntry {
	if e == nil {
		return []fsEntry{}
	}
	return e
}

// countMarkdown counts .md files below dir, to a limited depth and count, skipping hidden folders and symlinks.
func countMarkdown(dir string) (int, bool) {
	n := 0
	capped := false
	base := strings.Count(dir, string(filepath.Separator))
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if p != dir && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || strings.Count(p, string(filepath.Separator))-base >= countDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			n++
			if n >= countCap {
				capped = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return n, capped
}
