// Package config stores the things Docent remembers between runs: the libraries you registered and the folders and files you opened recently.
// It lives in the user's config directory; nothing is ever written into the folders you open.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/migara0/docent/internal/slug"
)

// Settings are the optional conventions of a library's profile. For "plain" they are empty. Docent keeps them here, never inside your folder.
type Settings struct {
	Prefix    string `json:"prefix,omitempty"`    // devnotes: the note ID prefix, e.g. DOC (IDs look like DOC-0007)
	Workspace string `json:"workspace,omitempty"` // devnotes: the folder that holds the service repos (for commit tracking and stale checks)
	Templates string `json:"templates,omitempty"` // devnotes: a folder of note templates that replaces the built-in ones
}

// Library is a folder the user registered. It persists across runs.
type Library struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Profile  string    `json:"profile"` // "plain" (default) or "devnotes"
	Settings Settings  `json:"settings,omitempty"`
	AddedAt  time.Time `json:"addedAt"`
}

// Recent is something opened lately, whether registered or not.
type Recent struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Kind     string    `json:"kind"` // "folder" or "file"
	OpenedAt time.Time `json:"openedAt"`
}

// Config is the content of config.json.
type Config struct {
	Version   int       `json:"version"`
	Libraries []Library `json:"libraries"`
	Recents   []Recent  `json:"recents"`
}

const (
	currentVersion = 1
	maxRecents     = 20
)

// Paths are the files and folders Docent uses. DOCENT_HOME (used by tests) puts everything under one folder.
type Paths struct {
	ConfigDir  string
	CacheDir   string
	ConfigFile string
	ServerInfo string // written by a running server so the CLI can find it
}

// ResolvePaths finds the config and cache folders (XDG on Linux).
func ResolvePaths() (Paths, error) {
	var cfg, cache string
	if home := os.Getenv("DOCENT_HOME"); home != "" {
		cfg, cache = filepath.Join(home, "config"), filepath.Join(home, "cache")
	} else {
		c, err := os.UserConfigDir()
		if err != nil {
			return Paths{}, fmt.Errorf("find config directory: %w", err)
		}
		k, err := os.UserCacheDir()
		if err != nil {
			return Paths{}, fmt.Errorf("find cache directory: %w", err)
		}
		cfg, cache = filepath.Join(c, "docent"), filepath.Join(k, "docent")
	}
	return Paths{ConfigDir: cfg, CacheDir: cache, ConfigFile: filepath.Join(cfg, "config.json"), ServerInfo: filepath.Join(cache, "server.json")}, nil
}

// Store reads and updates config.json. Every change re-reads the file first, so the CLI and a running server never overwrite each other.
type Store struct {
	paths Paths
	mu    sync.Mutex
}

func NewStore(p Paths) *Store { return &Store{paths: p} }

// Paths returns where this store keeps its files.
func (s *Store) Paths() Paths { return s.paths }

// Load returns the current config; a missing file is an empty config.
func (s *Store) Load() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

func (s *Store) read() (Config, error) {
	data, err := os.ReadFile(s.paths.ConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		return Config{Version: currentVersion}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("%s is not valid JSON (fix or delete it): %w", s.paths.ConfigFile, err)
	}
	if c.Version == 0 {
		c.Version = currentVersion
	}
	return c, nil
}

func (s *Store) write(c Config) error {
	if err := os.MkdirAll(s.paths.ConfigDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.paths.ConfigDir, ".config-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.paths.ConfigFile) // atomic: a reader never sees half a file
}

// update runs fn on the freshly read config and saves the result.
func (s *Store) update(fn func(*Config) error) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.read()
	if err != nil {
		return Config{}, err
	}
	if err := fn(&c); err != nil {
		return Config{}, err
	}
	return c, s.write(c)
}

// AddLibrary registers a folder. Registering the same folder again returns the existing library, updating its settings when new ones are given.
func (s *Store) AddLibrary(path, name, profile string, settings ...Settings) (Library, error) {
	var out Library
	var st Settings
	if len(settings) > 0 {
		st = settings[0]
	}
	_, err := s.update(func(c *Config) error {
		for i, l := range c.Libraries {
			if l.Path == path {
				if st != (Settings{}) {
					c.Libraries[i].Settings = st
				}
				if profile != "" {
					c.Libraries[i].Profile = profile
				}
				out = c.Libraries[i]
				return nil
			}
		}
		if name == "" {
			name = filepath.Base(path)
		}
		if profile == "" {
			profile = "plain"
		}
		out = Library{ID: uniqueID(c.Libraries, slug.Make(name)), Name: name, Path: path, Profile: profile, Settings: st, AddedAt: time.Now().UTC().Truncate(time.Second)}
		c.Libraries = append(c.Libraries, out)
		return nil
	})
	return out, err
}

func uniqueID(existing []Library, base string) string {
	if base == "" {
		base = "library"
	}
	id := base
	for n := 2; slices.ContainsFunc(existing, func(l Library) bool { return l.ID == id }); n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

// RemoveLibrary forgets a library by id, name or path. The folder itself is never touched.
func (s *Store) RemoveLibrary(key string) (Library, error) {
	var removed Library
	_, err := s.update(func(c *Config) error {
		i := slices.IndexFunc(c.Libraries, func(l Library) bool { return l.ID == key || l.Name == key || l.Path == key })
		if i < 0 {
			return fmt.Errorf("no library %q", key)
		}
		removed = c.Libraries[i]
		c.Libraries = slices.Delete(c.Libraries, i, i+1)
		return nil
	})
	return removed, err
}

// Touch records an opened path at the top of the recents list.
func (s *Store) Touch(r Recent) error {
	_, err := s.update(func(c *Config) error {
		r.OpenedAt = time.Now().UTC().Truncate(time.Second)
		c.Recents = slices.DeleteFunc(c.Recents, func(x Recent) bool { return x.Path == r.Path })
		c.Recents = append([]Recent{r}, c.Recents...)
		if len(c.Recents) > maxRecents {
			c.Recents = c.Recents[:maxRecents]
		}
		return nil
	})
	return err
}
