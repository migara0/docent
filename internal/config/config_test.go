package config

import (
	"os"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("DOCENT_HOME", t.TempDir())
	p, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	return NewStore(p)
}

func TestMissingFileIsEmpty(t *testing.T) {
	c, err := newStore(t).Load()
	if err != nil || len(c.Libraries) != 0 || c.Version != currentVersion {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestAddRemovePersist(t *testing.T) {
	s := newStore(t)
	a, err := s.AddLibrary("/notes/a", "Team wiki", "")
	if err != nil || a.ID != "team-wiki" || a.Profile != "plain" {
		t.Fatalf("add: %+v %v", a, err)
	}
	again, _ := s.AddLibrary("/notes/a", "Other name", "devnotes", Settings{Prefix: "DOC"})
	if again.ID != a.ID || again.Profile != "devnotes" || again.Settings.Prefix != "DOC" {
		t.Fatalf("registering the same folder twice returns the same library, with the new profile and settings, got %+v", again)
	}
	b, _ := s.AddLibrary("/notes/b", "Team wiki", "devnotes")
	if b.ID != "team-wiki-2" {
		t.Fatalf("duplicate names must get unique ids, got %q", b.ID)
	}
	// a second Store (another process) sees the same data
	c, _ := NewStore(s.Paths()).Load()
	if len(c.Libraries) != 2 {
		t.Fatalf("want 2 libraries on disk, got %d", len(c.Libraries))
	}
	if _, err := s.RemoveLibrary("team-wiki-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RemoveLibrary("nope"); err == nil {
		t.Fatal("removing an unknown library must fail")
	}
	c, _ = s.Load()
	if len(c.Libraries) != 1 || c.Libraries[0].Path != "/notes/a" {
		t.Fatalf("after remove: %+v", c.Libraries)
	}
}

func TestRecentsAreDedupedAndCapped(t *testing.T) {
	s := newStore(t)
	for i := range 30 {
		_ = s.Touch(Recent{Path: filepath.Join("/x", string(rune('a'+i%26)), string(rune('0'+i/26))), Kind: "folder"})
	}
	_ = s.Touch(Recent{Path: "/x/a/0", Kind: "folder"}) // already there: moves to the front
	c, _ := s.Load()
	if len(c.Recents) != maxRecents || c.Recents[0].Path != "/x/a/0" {
		t.Fatalf("recents: len %d, first %q", len(c.Recents), c.Recents[0].Path)
	}
}

func TestCorruptFileGivesAHelpfulError(t *testing.T) {
	s := newStore(t)
	_ = os.MkdirAll(s.Paths().ConfigDir, 0o755)
	_ = os.WriteFile(s.Paths().ConfigFile, []byte("{nope"), 0o644)
	if _, err := s.Load(); err == nil {
		t.Fatal("a corrupt config must be reported, not silently replaced")
	}
}
