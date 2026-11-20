package devnotes

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/migara0/docent/internal/config"
)

func TestParseFormatNormalize(t *testing.T) {
	if p, n, ok := ParseID("DOC-0007"); !ok || p != "DOC" || n != 7 {
		t.Errorf("ParseID: %v %v %v", p, n, ok)
	}
	for _, bad := range []string{"doc-0007", "DOC-7", "IGT0007", "I-0007", "TOOLONG-0001", ""} {
		if _, _, ok := ParseID(bad); ok {
			t.Errorf("%q must not parse", bad)
		}
	}
	if FormatID("OPS", 38) != "OPS-0038" || FormatID("OPS", 12345) != "OPS-12345" {
		t.Error("FormatID")
	}
	for in, want := range map[string]string{"7": "DOC-0007", "0007": "DOC-0007", "doc-7": "DOC-0007", " OPS-12 ": "OPS-0012", "DOC-0007": "DOC-0007"} {
		if got, ok := NormalizeID(in, "DOC"); !ok || got != want {
			t.Errorf("NormalizeID(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	if _, ok := NormalizeID("7", ""); ok {
		t.Error("a bare number with no default prefix is not an ID")
	}
	if _, ok := NormalizeID("seven", "DOC"); ok {
		t.Error("a word is not an ID")
	}
}

func TestRegistryNeverReusesANumber(t *testing.T) {
	r := Registry{Dir: t.TempDir(), Library: "lib", Prefix: "DOC"}
	if id, _ := r.Next([]string{"DOC-0003", "OPS-0009", "garbage"}); id != "DOC-0004" {
		t.Fatalf("first id above the highest existing one of this prefix: %s", id)
	}
	// the note with DOC-0004 is deleted: the next call still moves on
	if id, _ := r.Next(nil); id != "DOC-0005" {
		t.Fatalf("a deleted note's number must not be reused: %s", id)
	}
	if id, _ := r.Next([]string{"DOC-0040"}); id != "DOC-0041" {
		t.Fatalf("notes added elsewhere with higher numbers are respected: %s", id)
	}
	if got := r.Peek(nil); got != "DOC-0042" {
		t.Errorf("Peek must not reserve: %s", got)
	}
	if got := r.Peek(nil); got != "DOC-0042" {
		t.Errorf("Peek twice: %s", got)
	}
}

func TestRegistryIsSafeAcrossGoroutinesAndProcesses(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// each goroutine uses its own Registry value, like separate processes would
			id, err := Registry{Dir: dir, Library: "x", Prefix: "DOC"}.Next(nil)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			if seen[id] {
				t.Errorf("duplicate id handed out: %s", id)
			}
			seen[id] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(seen) != 40 {
		t.Errorf("got %d distinct ids", len(seen))
	}
}

func TestResolveDefaultsFollowTheSiteLayout(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "doc", "doc_developer_notes", "docs")
	_ = os.MkdirAll(docs, 0o755)
	_ = os.MkdirAll(filepath.Join(root, "doc", "doc_developer_notes", "templates"), 0o755)
	s := Resolve(config.Library{Name: "doc-notes", Path: docs})
	if s.Prefix != "DOC" || s.Workspace != filepath.Join(root, "doc") || s.Templates != filepath.Join(root, "doc", "doc_developer_notes", "templates") || s.Site == "" {
		t.Errorf("site layout: %+v", s)
	}
	o := Resolve(config.Library{Name: "Team wiki", Path: "/somewhere/wiki", Settings: config.Settings{Prefix: "kb", Workspace: "/repos"}})
	if o.Prefix != "KB" || o.Workspace != "/repos" || o.Templates != "" || o.Site != "" {
		t.Errorf("explicit settings win: %+v", o)
	}
	if p := Resolve(config.Library{Name: "123", Path: "/x"}).Prefix; p != "DOC" {
		t.Errorf("fallback prefix: %s", p)
	}
}
