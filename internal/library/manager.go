package library

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/render"
	"github.com/migara0/docent/internal/slug"
)

// Manager owns every open source: the registered libraries (remembered in the config file) and the folders and files opened for this run only.
type Manager struct {
	store *config.Store
	rend  *render.Renderer
	hub   *Hub
	ctx   context.Context
	hooks Hooks // profile behaviour (nil: only the plain profile)
	idRe  atomic.Pointer[regexp.Regexp]

	mu      sync.RWMutex
	sources map[string]*Source
	cancels map[string]context.CancelFunc
}

// Option customises a Manager.
type Option func(*Manager)

// WithHooks installs the behaviour of the profiles other than "plain".
func WithHooks(h Hooks) Option { return func(m *Manager) { m.hooks = h } }

func NewManager(ctx context.Context, store *config.Store, opts ...Option) *Manager {
	m := &Manager{store: store, rend: render.New(), hub: NewHub(), ctx: ctx, sources: map[string]*Source{}, cancels: map[string]context.CancelFunc{}}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *Manager) idRegex() *regexp.Regexp { return m.idRe.Load() }

// rebuildIDRegex recomputes the pattern that finds note IDs of every open library's prefix. Callers hold m.mu.
func (m *Manager) rebuildIDRegex() {
	var prefixes []string
	for _, s := range m.sources {
		if s.IDPrefix != "" && !slices.Contains(prefixes, s.IDPrefix) {
			prefixes = append(prefixes, s.IDPrefix)
		}
	}
	if len(prefixes) == 0 {
		m.idRe.Store(nil)
		return
	}
	slices.Sort(prefixes)
	quoted := make([]string, len(prefixes))
	for i, p := range prefixes {
		quoted[i] = regexp.QuoteMeta(p)
	}
	m.idRe.Store(regexp.MustCompile(`\b((?:` + strings.Join(quoted, "|") + `)-\d{4,})\b`))
}

// ResolveID finds a note by its ID in any open library: DOC-0007 can be linked from a note of another library.
func (m *Manager) ResolveID(id string) (url, title string, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sources {
		if pg, found := s.PageByID(id); found {
			return s.PageURL(pg.Path), pg.Title, true
		}
	}
	return "", "", false
}

// Sources returns the open libraries (registered and opened once).
func (m *Manager) Sources() []*Source {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Source, 0, len(m.sources))
	for _, s := range m.sources {
		out = append(out, s)
	}
	return out
}

func (m *Manager) wire(s *Source) {
	s.idRegex = m.idRegex
	s.resolveID = m.ResolveID
}

// Hub returns the event hub the server streams to browsers.
func (m *Manager) Hub() *Hub { return m.hub }

func (m *Manager) start(s *Source) {
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancels[s.ID] = cancel
	m.sources[s.ID] = s
	m.wire(s)
	m.rebuildIDRegex()
	go func() {
		_, _ = s.Rescan() // the first scan; errors show up in Info
		s.Watch(ctx)
	}()
}

func (m *Manager) stop(id string) {
	if c, ok := m.cancels[id]; ok {
		c()
		delete(m.cancels, id)
	}
	delete(m.sources, id)
	m.rebuildIDRegex()
}

func (m *Manager) event(e Event) { m.hub.Publish(e) }

func (m *Manager) prefixOf(l config.Library) string {
	if m.hooks == nil || l.Profile != "devnotes" {
		return ""
	}
	return m.hooks.IDPrefix(l)
}

// Sync makes the open registered libraries match config.json. Call it at start-up and whenever the file changes (for example after `docent add`).
func (m *Manager) Sync() error {
	cfg, err := m.store.Load()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	want := map[string]config.Library{}
	for _, l := range cfg.Libraries {
		want[l.ID] = l
	}
	for id, s := range m.sources {
		if s.Kind != KindLibrary {
			continue
		}
		// A running source is never modified in place (its scan reads these fields concurrently). If anything about it changed, it is stopped
		// here and started again below, which re-reads it in a few milliseconds.
		if l, ok := want[id]; !ok || l.Path != s.Root || l.Name != s.Name || l.Profile != s.Profile || l.Settings != s.Lib.Settings || m.prefixOf(l) != s.IDPrefix {
			m.stop(id)
			m.event(Event{Type: "source", Source: id})
		}
	}
	for _, l := range cfg.Libraries {
		if _, ok := m.sources[l.ID]; ok {
			continue
		}
		// a folder opened earlier in this run becomes the registered library
		for id, s := range m.sources {
			if s.Kind == KindFolder && s.Root == l.Path {
				m.stop(id)
			}
		}
		src := newSource(l.ID, l.Name, KindLibrary, l.Profile, l.Path, "", m.rend, m.event)
		src.IDPrefix, src.Lib = m.prefixOf(l), l
		if l.Profile == "devnotes" {
			src.hooks = m.hooks
		}
		m.start(src)
		m.event(Event{Type: "source", Source: l.ID})
	}
	return nil
}

// Register adds a folder to the libraries and remembers it for the next run.
func (m *Manager) Register(path, name, profile string, settings ...config.Settings) (*Source, error) {
	abs, err := absDir(path)
	if err != nil {
		return nil, err
	}
	lib, err := m.store.AddLibrary(abs, name, profile, settings...)
	if err != nil {
		return nil, err
	}
	if err := m.Sync(); err != nil {
		return nil, err
	}
	m.touch(abs, lib.Name, "folder")
	s, _ := m.Get(lib.ID)
	return s, nil
}

// Remove forgets a registered library. The folder on disk is untouched.
func (m *Manager) Remove(key string) error {
	if _, err := m.store.RemoveLibrary(key); err != nil {
		return err
	}
	return m.Sync()
}

// Open opens a folder or a single Markdown file for this run only. Opening a folder that is already open returns the existing source.
func (m *Manager) Open(p string) (*Source, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", p, err)
	}
	root, file, kind := abs, "", KindFolder
	if !info.IsDir() {
		if !isMarkdown(abs) {
			return nil, fmt.Errorf("%s is not a folder or a Markdown file", p)
		}
		root, file, kind = filepath.Dir(abs), filepath.Base(abs), KindFile
	}
	m.mu.Lock()
	for _, s := range m.sources {
		if s.Root == root && s.File == file {
			m.mu.Unlock()
			m.touch(abs, s.Name, string(kind))
			return s, nil
		}
	}
	h := sha1.Sum([]byte(abs))
	name := filepath.Base(abs)
	s := newSource("open-"+hex.EncodeToString(h[:4]), name, kind, "plain", root, file, m.rend, m.event)
	m.start(s)
	m.mu.Unlock()
	m.event(Event{Type: "source", Source: s.ID})
	m.touch(abs, name, string(kind))
	return s, nil
}

// HelpID is the id of the built-in guide.
const HelpID = "help"

// OpenHelp opens the folder holding the built-in guide as a source of its own: it is not remembered and the UI keeps it out of the library lists.
func (m *Manager) OpenHelp(dir string) *Source {
	m.mu.Lock()
	if s, ok := m.sources[HelpID]; ok {
		m.mu.Unlock()
		return s
	}
	s := newSource(HelpID, "Docent guide", KindHelp, "plain", dir, "", m.rend, m.event)
	m.start(s)
	m.mu.Unlock()
	return s
}

// DroppedID is the id of the one source that holds dropped files.
const DroppedID = "dropped"

// OpenDropped opens the folder that holds the dropped copies as a source of its own. It is not remembered (no recent entry) and is returned as is if already open.
func (m *Manager) OpenDropped(dir string) *Source {
	m.mu.Lock()
	if s, ok := m.sources[DroppedID]; ok {
		m.mu.Unlock()
		return s
	}
	s := newSource(DroppedID, "Dropped files", KindDrop, "plain", dir, "", m.rend, m.event)
	m.start(s)
	m.mu.Unlock()
	m.event(Event{Type: "source", Source: s.ID})
	return s
}

func (m *Manager) touch(path, name, kind string) {
	_ = m.store.Touch(config.Recent{Path: path, Name: name, Kind: kind})
}

// Get returns an open source by id.
func (m *Manager) Get(id string) (*Source, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sources[id]
	return s, ok
}

// List returns every open source: registered libraries first (in the order they were added), then the ones opened this run.
func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Info, 0, len(m.sources))
	for _, s := range m.sources {
		out = append(out, s.Info())
	}
	slices.SortStableFunc(out, func(a, b Info) int {
		if (a.Kind == KindLibrary) != (b.Kind == KindLibrary) {
			if a.Kind == KindLibrary {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}

func absDir(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("cannot add %s: %w", p, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is a file: open it with `docent open`, or add the folder that contains it", p)
	}
	return abs, nil
}

var _ = slug.Make
