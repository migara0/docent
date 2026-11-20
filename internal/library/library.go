// Package library turns a folder (or a single file) of Markdown into something the UI can browse: a page index, a folder tree,
// resolved links with backlinks, rendered pages and search documents. A Manager holds every source that is open.
package library

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/render"
)

// Kind says how a source came to be open.
type Kind string

const (
	KindLibrary Kind = "library" // registered: remembered for the next run
	KindFolder  Kind = "folder"  // opened once
	KindFile    Kind = "file"    // a single Markdown file, opened once
	KindHelp    Kind = "help"    // the built-in user guide
	KindDrop    Kind = "dropped" // temporary copies of files dropped into the browser; the folder is deleted when Docent exits
)

// Limits that keep a huge or odd folder from hurting the machine.
const (
	maxFileBytes = 8 << 20
	maxPages     = 200_000
)

// Meta is the part of a page's front matter the index keeps: titles, IDs, dates and kinds. A struct of shared strings is about a tenth the size of a map.
type Meta struct {
	ID          string   `json:"id,omitempty"`
	Date        string   `json:"date,omitempty"`
	Updated     string   `json:"updated,omitempty"`
	Status      string   `json:"status,omitempty"`
	Type        string   `json:"type,omitempty"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	Release     string   `json:"release,omitempty"`
	Versions    string   `json:"versions,omitempty"`
	Verified    string   `json:"verified,omitempty"`
	Technology  string   `json:"technology,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	HasTitle    bool     `json:"-"` // the front matter has a title (a page without one takes it from its first heading)
	HasTags     bool     `json:"-"`
	DupTags     int      `json:"-"` // tags that were listed more than once
}

// Page is what the scan keeps about one Markdown file. It is deliberately small (about 0.3 KB for a typical page): headings, link text and the body
// are re-read from disk when a page is opened or searched, so a 10,000-page library stays light. Immutable once created.
type Page struct {
	Path     string // slash-separated, relative to the source root, e.g. "guide/syntax.md"
	Title    string
	Meta     Meta // a few well-known front matter fields only (the full front matter is read when a page is opened)
	Words    int
	Modified time.Time
	Born     time.Time // when the file was created on disk (zero when unknown): the time of day of the created date
	Size     int64
	Warnings []string
	rels     []string // the relative link targets (paths as written, de-duplicated), kept to build backlinks and to find broken links
	ids      []string // note IDs of this library's own prefix mentioned in the text, for backlinks
}

// Link is an outgoing link of a page after resolution.
type Link struct {
	Text   string `json:"text"`
	Href   string `json:"href"`
	Kind   string `json:"kind"`             // page, broken, file, external, anchor
	Target string `json:"target,omitempty"` // for kind "page": the target page path
}

// Ref points at another page.
type Ref struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Node is an entry of the folder tree.
type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`            // for a page: its path; for a folder: its folder path
	Title    string  `json:"title,omitempty"` // pages only
	ID       string  `json:"id,omitempty"`    // pages only: the note ID, so the tree can be filtered by it
	Kind     string  `json:"kind"`            // folder or page
	Count    int     `json:"count,omitempty"` // folders: pages inside, at any depth
	URL      string  `json:"url,omitempty"`   // pages only
	Children []*Node `json:"children,omitempty"`
}

// snapshot is an immutable view of the index. Readers use it without locks; updates build a new one and swap it in.
type snapshot struct {
	gen      uint64
	pages    map[string]*Page
	back     map[string][]Ref
	ids      map[string]string   // note ID -> page path
	dupIDs   map[string][]string // an ID used by more than one page -> the paths
	order    []string
	tree     *Node
	folders  int
	built    time.Time
	searchMu sync.Mutex
	search   []byte
}

// Info describes a source for the library list.
type Info struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Kind     Kind      `json:"kind"`
	Profile  string    `json:"profile"`
	Root     string    `json:"root"`
	File     string    `json:"file,omitempty"`
	Pages    int       `json:"pages"`
	Folders  int       `json:"folders"`
	IDPrefix string    `json:"idPrefix,omitempty"`
	Updated  time.Time `json:"updated"`
	Indexing bool      `json:"indexing"`
	Error    string    `json:"error,omitempty"`
}

// Source is one open folder or file.
type Source struct {
	ID      string
	Name    string
	Kind    Kind
	Profile string
	Root    string // absolute folder
	File    string // base name for a single-file source, else ""

	IDPrefix string         // the prefix of this library's note IDs (devnotes profile), else ""
	Lib      config.Library // the registered library's settings (zero for something opened once)
	hooks    Hooks

	// set by the Manager: the pattern for every known ID prefix and the resolver across libraries
	idRegex   func() *regexp.Regexp
	resolveID func(id string) (url, title string, ok bool)

	rend   *render.Renderer
	snap   atomic.Pointer[snapshot]
	gen    atomic.Uint64
	ready  chan struct{}
	errMsg atomic.Value // string
	events func(Event)

	scanMu sync.Mutex // one scan at a time
	strs   interner
	cache  *pageCache
}

func newSource(id, name string, kind Kind, profile, root, file string, rend *render.Renderer, events func(Event)) *Source {
	s := &Source{ID: id, Name: name, Kind: kind, Profile: profile, Root: root, File: file, rend: rend, ready: make(chan struct{}), events: events, cache: newPageCache(48)}
	s.snap.Store(&snapshot{pages: map[string]*Page{}, back: map[string][]Ref{}, tree: &Node{Kind: "folder"}})
	return s
}

// WaitReady blocks until the first scan finished (or the context ends).
func (s *Source) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Source) isReady() bool {
	select {
	case <-s.ready:
		return true
	default:
		return false
	}
}

// Info returns the numbers shown on the home screen.
func (s *Source) Info() Info {
	snap := s.snap.Load()
	msg, _ := s.errMsg.Load().(string)
	return Info{ID: s.ID, Name: s.Name, Kind: s.Kind, Profile: s.Profile, Root: s.Root, File: s.File,
		Pages: len(snap.pages), Folders: snap.folders, IDPrefix: s.IDPrefix, Updated: snap.built, Indexing: !s.isReady(), Error: msg}
}

// Tree returns the folder tree.
func (s *Source) Tree() *Node { return s.snap.Load().tree }

// Page returns the indexed facts about a page.
func (s *Source) Page(p string) (*Page, bool) { pg, ok := s.snap.Load().pages[p]; return pg, ok }

// PageURL is the address of a page in the app: /<source>/<path without extension>.
func (s *Source) PageURL(p string) string { return pageURL(s.ID, p) }

func pageURL(id, p string) string {
	ext := path.Ext(p)
	segs := strings.Split(strings.TrimSuffix(p, ext), "/")
	for i, seg := range segs {
		segs[i] = urlEscape(seg)
	}
	return "/" + urlEscape(id) + "/" + strings.Join(segs, "/")
}

func urlEscape(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// Event tells the UI something changed.
type Event struct {
	Type   string `json:"type"` // "page" (one page changed), "tree" (pages were added, removed or retitled), "source" (the source itself changed)
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`
}

// ownIDPattern matches IDs of this library's own prefix, the only ones analysis needs (they feed backlinks and the duplicate/unknown ID checks).
func (s *Source) ownIDPattern() *regexp.Regexp {
	if s.IDPrefix == "" {
		return nil
	}
	return regexp.MustCompile(`\b(` + regexp.QuoteMeta(s.IDPrefix) + `-\d{4,})\b`)
}

// Pages returns every indexed page (the pages are immutable; do not modify them).
func (s *Source) Pages() []*Page {
	snap := s.snap.Load()
	out := make([]*Page, 0, len(snap.pages))
	for _, p := range snap.pages {
		out = append(out, p)
	}
	return out
}

// PageByID finds a page by its note ID in this library.
func (s *Source) PageByID(id string) (*Page, bool) {
	snap := s.snap.Load()
	if p, ok := snap.ids[id]; ok {
		return snap.pages[p], true
	}
	return nil, false
}

// IDs returns every note ID in this library with its page path.
func (s *Source) IDs() map[string]string { return s.snap.Load().ids }

// MentionedIDs returns the IDs (of this library's prefix) a page mentions in its text.
func (p *Page) MentionedIDs() []string { return p.ids }

// Notice is something worth telling the reader about a page, shown above it (for example "the code this note describes has changed").
type Notice struct {
	Kind  string   `json:"kind"` // warning or info
	Title string   `json:"title"`
	Text  string   `json:"text,omitempty"`
	Items []string `json:"items,omitempty"`
}

// Hooks let a profile (such as developer notes) extend a library without the core knowing about it.
type Hooks interface {
	// IDPrefix returns the note ID prefix of a library, or "" when it has none.
	IDPrefix(lib config.Library) string
	// Validate returns the problems of one page's front matter and placement (rel is the page path).
	Validate(lib config.Library, rel string, pg *Page) []string
	// Notices returns notices for a page that is being opened (for example a stale-module warning).
	Notices(lib config.Library, rel string, pg *Page) []Notice
	// Plan returns the status board of a page that is a plan (the page's Markdown is src), or nil.
	Plan(lib config.Library, rel string, src []byte) any
}
