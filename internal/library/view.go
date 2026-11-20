package library

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/migara0/docent/internal/render"
)

// Crumb is one step of the breadcrumb trail.
type Crumb struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// PageView is a rendered page with everything the reader needs around it.
type PageView struct {
	Source    string           `json:"source"`
	Path      string           `json:"path"`
	URL       string           `json:"url"`
	Title     string           `json:"title"`
	Meta      map[string]any   `json:"meta,omitempty"`
	HTML      string           `json:"html"`
	TOC       []render.Heading `json:"toc"`
	Words     int              `json:"words"`
	Modified  time.Time        `json:"modified"`
	CreatedAt string           `json:"createdAt,omitempty"` // created and last updated with the time of day (RFC 3339), when known
	UpdatedAt string           `json:"updatedAt,omitempty"`
	Crumbs    []Crumb          `json:"crumbs"`
	Links     []Link           `json:"links"`
	Backlinks []Ref            `json:"backlinks"`
	Prev      *Ref             `json:"prev,omitempty"`
	Next      *Ref             `json:"next,omitempty"`
	Warnings  []string         `json:"warnings,omitempty"`
	Notices   []Notice         `json:"notices,omitempty"`
	Plan      any              `json:"plan,omitempty"`  // the status board of a plan note (developer notes)
	File      string           `json:"file,omitempty"`  // absolute path on this machine; set by the server in loopback mode only
	Place     *Place           `json:"place,omitempty"` // where a developer note lives (profile libraries only)
}

// Place is the service (or feature, or technology) and type of a developer note.
type Place struct {
	Group   string `json:"group,omitempty"`
	Service string `json:"service"`
	Type    string `json:"type"`
}

// ErrNoPage means the path is not an indexed page of this source.
var ErrNoPage = fmt.Errorf("page not found")

// Raw returns the Markdown of an indexed page exactly as it is on disk, and its absolute path (for "open in editor").
func (s *Source) Raw(p string) (data []byte, abs string, err error) {
	snap := s.snap.Load()
	pg, ok := snap.pages[p]
	if !ok {
		return nil, "", ErrNoPage
	}
	if pg.Size > maxFileBytes {
		return nil, "", fmt.Errorf("%s is larger than %d MB", p, maxFileBytes>>20)
	}
	abs = filepath.Join(s.Root, filepath.FromSlash(p))
	data, err = os.ReadFile(abs)
	return data, abs, err
}

// RenderPage renders one page from the file as it is on disk right now.
func (s *Source) RenderPage(p string) (*PageView, error) {
	snap := s.snap.Load()
	pg, ok := snap.pages[p] // only indexed files are ever served: this is the path-confinement check
	if !ok {
		return nil, ErrNoPage
	}
	key := fmt.Sprintf("%d|%s|%d|%d", snap.gen, p, pg.Modified.UnixNano(), pg.Size)
	if v, ok := s.cache.get(key); ok {
		return v, nil
	}
	if pg.Size > maxFileBytes {
		return nil, fmt.Errorf("%s is larger than %d MB", p, maxFileBytes>>20)
	}
	src, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(p)))
	if err != nil {
		return nil, err
	}
	res, err := s.rend.Process(src, render.Options{HTML: true, Resolve: resolverFor(s.ID, snap, p), IDs: s.idLinker(pg)})
	if err != nil {
		return nil, err
	}
	v := &PageView{Source: s.ID, Path: p, URL: pageURL(s.ID, p), Title: pg.Title, Meta: res.Meta, HTML: res.HTML, Words: res.Words, Modified: pg.Modified, CreatedAt: pg.CreatedAt(), UpdatedAt: pg.UpdatedAt(),
		TOC: []render.Heading{}, Links: classifyLinks(snap.pages, p, res.Links), Backlinks: snap.back[p], Warnings: res.Warnings}
	if v.Backlinks == nil {
		v.Backlinks = []Ref{} // arrays in the API are never null
	}
	dedupeTags(v.Meta)
	if s.hooks != nil {
		v.Warnings = append(v.Warnings, s.hooks.Validate(s.Lib, p, pg)...)
		v.Notices = s.hooks.Notices(s.Lib, p, pg)
		v.Plan = s.hooks.Plan(s.Lib, p, src)
	}
	if res.Title != "" {
		v.Title = res.Title
	}
	for _, h := range res.Headings {
		if h.Level >= 2 && h.Level <= 4 {
			v.TOC = append(v.TOC, h)
		}
	}
	v.Crumbs = []Crumb{{Name: s.Name, URL: "/" + urlEscape(s.ID) + "/"}}
	if dir := path.Dir(p); dir != "." {
		for _, seg := range strings.Split(dir, "/") {
			v.Crumbs = append(v.Crumbs, Crumb{Name: seg})
		}
	}
	v.Crumbs = append(v.Crumbs, Crumb{Name: v.Title})
	for i, op := range snap.order {
		if op != p {
			continue
		}
		if i > 0 {
			q := snap.order[i-1]
			v.Prev = &Ref{Path: q, Title: snap.pages[q].Title, URL: pageURL(s.ID, q)}
		}
		if i+1 < len(snap.order) {
			q := snap.order[i+1]
			v.Next = &Ref{Path: q, Title: snap.pages[q].Title, URL: pageURL(s.ID, q)}
		}
		break
	}
	s.cache.put(key, v)
	return v, nil
}

// pageCache keeps the last few rendered pages. Entries are keyed by snapshot generation, so any change invalidates them.
type pageCache struct {
	mu    sync.Mutex
	max   int
	m     map[string]*PageView
	order []string
}

func newPageCache(max int) *pageCache { return &pageCache{max: max, m: map[string]*PageView{}} }

func (c *pageCache) get(k string) (*PageView, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok
}

func (c *pageCache) put(k string, v *PageView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.m[k]; !ok {
		c.order = append(c.order, k)
	}
	c.m[k] = v
	for len(c.order) > c.max {
		delete(c.m, c.order[0])
		c.order = c.order[1:]
	}
}

// idLinker links note IDs in a page: to pages of this library, and (through the Manager) to pages of the other open libraries.
func (s *Source) idLinker(pg *Page) *render.IDLinker {
	var re *regexp.Regexp
	if s.idRegex != nil {
		re = s.idRegex()
	}
	if re == nil {
		re = s.ownIDPattern()
	}
	if re == nil {
		return nil
	}
	return &render.IDLinker{
		Pattern: re,
		Self:    pg.Meta.ID,
		Own:     func(id string) bool { return s.IDPrefix != "" && strings.HasPrefix(id, s.IDPrefix+"-") },
		Resolve: func(id string) (string, string, bool) {
			if p, ok := s.snap.Load().ids[id]; ok {
				return s.PageURL(p), s.snap.Load().pages[p].Title, true
			}
			if s.resolveID != nil {
				return s.resolveID(id)
			}
			return "", "", false
		},
	}
}

// dedupeTags lists each tag of the front matter once (the UI keys its chips by name, so a repeat would break the page).
func dedupeTags(meta map[string]any) {
	list, ok := meta["tags"].([]any)
	if !ok {
		return
	}
	seen := make(map[any]bool, len(list))
	out := make([]any, 0, len(list))
	for _, x := range list {
		if h, hashable := x.(string); hashable {
			if seen[h] {
				continue
			}
			seen[h] = true
		}
		out = append(out, x)
	}
	meta["tags"] = out
}
