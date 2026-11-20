package library

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/migara0/docent/internal/render"
)

type fileStat struct {
	size int64
	mod  time.Time
}

// ignoredDir names are never scanned. Hidden folders (".git", ".obsidian" ...) are skipped too.
var ignoredDir = map[string]bool{"node_modules": true}

var datePrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[-_ ]+`)

// titleFromFilename makes a readable title when a page has neither front matter nor a heading:
// "2026-10-14-bread-crumb_plan.md" becomes "Bread crumb plan".
func titleFromFilename(p string) string {
	base := strings.TrimSuffix(path.Base(p), path.Ext(p))
	t := datePrefix.ReplaceAllString(base, "")
	t = strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(t))
	if t == "" {
		t = base
	}
	return strings.ToUpper(t[:1]) + t[1:]
}

// listFiles finds every Markdown file of the source. Symbolic links are not followed, so a source can never reach outside its folder.
func (s *Source) listFiles() (map[string]fileStat, error) {
	files := map[string]fileStat{}
	if s.File != "" {
		info, err := os.Stat(filepath.Join(s.Root, s.File))
		if err != nil {
			return nil, err
		}
		files[s.File] = fileStat{info.Size(), info.ModTime()}
		return files, nil
	}
	err := filepath.WalkDir(s.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == s.Root {
				return err
			}
			return nil // an unreadable sub-folder is skipped, not fatal
		}
		name := d.Name()
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if p != s.Root && (strings.HasPrefix(name, ".") || ignoredDir[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdown(name) || strings.HasPrefix(name, ".") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(s.Root, p)
		files[filepath.ToSlash(rel)] = fileStat{info.Size(), info.ModTime()}
		if len(files) > maxPages {
			return fmt.Errorf("more than %d Markdown files: open a smaller folder", maxPages)
		}
		return nil
	})
	return files, err
}

// analyse reads one file and learns what the index needs. It never fails: a bad file becomes a page with a warning.
func (s *Source) analyse(rel string, st fileStat) *Page {
	pg := &Page{Path: rel, Modified: st.mod, Size: st.size, Title: titleFromFilename(rel), Born: birthTime(filepath.Join(s.Root, filepath.FromSlash(rel)))}
	if st.size > maxFileBytes {
		pg.Warnings = []string{fmt.Sprintf("file is larger than %d MB and was not read", maxFileBytes>>20)}
		return pg
	}
	src, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(rel)))
	if err != nil {
		pg.Warnings = []string{"could not read the file: " + err.Error()}
		return pg
	}
	opts := render.Options{}
	if re := s.ownIDPattern(); re != nil {
		opts.IDs = &render.IDLinker{Pattern: re}
	}
	res, err := s.rend.Process(src, opts)
	if err != nil {
		pg.Warnings = []string{err.Error()}
		return pg
	}
	if res.Title != "" {
		pg.Title = res.Title
	}
	pg.Meta, pg.Words, pg.Warnings, pg.rels = slimMeta(res.Meta, &s.strs), res.Words, res.Warnings, relTargets(res.Links, &s.strs)
	for _, id := range res.IDs {
		if id != pg.Meta.ID {
			pg.ids = append(pg.ids, s.strs.get(id))
		}
	}
	return pg
}

// relTargets returns the distinct relative link paths of a page.
func relTargets(links []render.Link, in *interner) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range links {
		if l.Kind == render.LinkRelative && l.Path != "" && !seen[l.Path] {
			seen[l.Path] = true
			out = append(out, in.get(l.Path)) // shared: many pages link to the same targets
		}
	}
	return out
}

// Rescan brings the index in line with the disk: it stats every file, re-reads only what changed, and swaps in a new snapshot.
// It reports whether anything changed. Use it for the first scan and after folder-level changes; Refresh handles single file edits much faster.
func (s *Source) Rescan() (bool, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	first := !s.isReady()
	defer func() {
		if first {
			close(s.ready)
		}
	}()

	files, err := s.listFiles()
	if err != nil {
		s.errMsg.Store(err.Error())
		if first {
			return false, err
		}
		files = map[string]fileStat{} // the folder vanished: show it empty rather than stale
	} else {
		s.errMsg.Store("")
	}
	old := s.snap.Load()
	changed := map[string]*fileStat{}
	for p, st := range files {
		if o, ok := old.pages[p]; !ok || !o.Modified.Equal(st.mod) || o.Size != st.size {
			st := st
			changed[p] = &st
		}
	}
	for p := range old.pages {
		if _, ok := files[p]; !ok {
			changed[p] = nil
		}
	}
	return s.update(old, changed, first), nil
}

// Refresh re-reads just the named pages (paths relative to the root, slash separated), which is what a file edit needs: no walk of the whole folder.
func (s *Source) Refresh(paths []string) bool {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if !s.isReady() {
		return false
	}
	old := s.snap.Load()
	changed := map[string]*fileStat{}
	for _, p := range paths {
		if s.File != "" && p != s.File {
			continue
		}
		info, err := os.Lstat(filepath.Join(s.Root, filepath.FromSlash(p)))
		switch {
		case err != nil || info.IsDir() || info.Mode()&fs.ModeSymlink != 0:
			if _, known := old.pages[p]; known {
				changed[p] = nil // deleted (or replaced by something that is not a file)
			}
		default:
			if o, ok := old.pages[p]; !ok || !o.Modified.Equal(info.ModTime()) || o.Size != info.Size() {
				changed[p] = &fileStat{info.Size(), info.ModTime()}
			}
		}
	}
	return s.update(old, changed, false)
}

// update applies a set of changed pages (nil = removed) to the index and publishes the result.
func (s *Source) update(old *snapshot, changed map[string]*fileStat, first bool) bool {
	if len(changed) == 0 && !first {
		return false
	}
	var todo []string
	removed := 0
	for p, st := range changed {
		if st == nil {
			removed++
		} else {
			todo = append(todo, p)
		}
	}
	if len(todo) >= bigScanPages {
		defer relaxGC()()
	}
	fresh := make(map[string]*Page, len(todo))
	var mu sync.Mutex
	var wg sync.WaitGroup
	jobs := make(chan string)
	for range min(runtime.GOMAXPROCS(0), max(len(todo), 1), 8) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				pg := s.analyse(p, *changed[p])
				mu.Lock()
				fresh[p] = pg
				mu.Unlock()
			}
		}()
	}
	for _, p := range todo {
		jobs <- p
	}
	close(jobs)
	wg.Wait()

	pages := make(map[string]*Page, len(old.pages)+len(todo))
	for p, pg := range old.pages {
		if st, touched := changed[p]; !touched || st != nil {
			pages[p] = pg
		}
	}
	structural := removed > 0 // pages added or removed, or a title or a link target changed: the tree and backlinks must be rebuilt
	for p, pg := range fresh {
		if o, existed := old.pages[p]; !existed || o.Title != pg.Title || o.Meta.ID != pg.Meta.ID || !slices.Equal(o.rels, pg.rels) || !slices.Equal(o.ids, pg.ids) {
			structural = true
		}
		pages[p] = pg
	}

	var next *snapshot
	if structural || first {
		next = s.build(pages)
	} else { // only the content of existing pages changed: keep the tree and backlinks, swap the pages
		next = &snapshot{gen: s.gen.Add(1), pages: pages, back: old.back, ids: old.ids, dupIDs: old.dupIDs, order: old.order, tree: old.tree, folders: old.folders, built: time.Now()}
	}

	s.snap.Store(next) // publish first, then announce: a listener that reacts to an event must see the new data

	if !first {
		sort.Strings(todo)
		treeChanged := removed > 0
		for _, p := range todo {
			if o, existed := old.pages[p]; !existed || o.Title != fresh[p].Title {
				treeChanged = true
			}
		}
		if treeChanged {
			s.emit(Event{Type: "tree", Source: s.ID})
		}
		for _, p := range todo {
			s.emit(Event{Type: "page", Source: s.ID, Path: p})
		}
	}
	return true
}

func (s *Source) build(pages map[string]*Page) *snapshot {
	ids, dups := indexIDs(pages)
	back := resolveBacklinks(s.ID, pages, ids)
	tree, order, folders := buildTree(s.Name, pages, s.ID)
	return &snapshot{gen: s.gen.Add(1), pages: pages, back: back, ids: ids, dupIDs: dups, order: order, tree: tree, folders: folders, built: time.Now()}
}

func (s *Source) emit(e Event) {
	if s.events != nil {
		s.events(e)
	}
}
