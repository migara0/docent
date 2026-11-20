package library

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/render"
)

const fixtures = "../../testdata/fixtures/basic"

func openFixture(t *testing.T) *Source {
	t.Helper()
	s := newSource("basic", "Basic", KindLibrary, "plain", mustAbs(t, fixtures), "", render.New(), nil)
	if _, err := s.Rescan(); err != nil {
		t.Fatal(err)
	}
	return s
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFixtures(t *testing.T) {
	s := openFixture(t)
	info := s.Info()
	if info.Pages != 4 || info.Folders != 2 || info.Indexing {
		t.Fatalf("info: %+v", info)
	}
	want := map[string]string{"index.md": "Basic fixture library", "guide/getting-started.md": "Getting started", "guide/syntax.md": "Syntax tour", "reference/api.md": "API reference"}
	for p, title := range want {
		pg, ok := s.Page(p)
		if !ok || pg.Title != title {
			t.Errorf("%s: %+v (ok=%v), want title %q", p, pg, ok, title)
		}
	}
	if pg, _ := s.Page("guide/syntax.md"); pg.Meta.ID != "DOC-0001" || pg.Meta.Status != "stable" {
		t.Errorf("front matter not read: %v", pg.Meta)
	}
}

func TestTreeOrder(t *testing.T) {
	tree := openFixture(t).Tree()
	var names []string
	for _, c := range tree.Children {
		names = append(names, c.Kind+":"+c.Name)
	}
	if got := strings.Join(names, " "); got != "page:index.md folder:guide folder:reference" {
		t.Errorf("root order = %s (index first, then folders; the empty folder is hidden)", got)
	}
	if tree.Count != 4 || tree.Children[1].Count != 2 {
		t.Errorf("counts: root %d, guide %d", tree.Count, tree.Children[1].Count)
	}
}

func TestLinksAndBacklinks(t *testing.T) {
	s := openFixture(t)
	view, err := s.RenderPage("reference/api.md")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, l := range view.Links {
		kinds[l.Href] = l.Kind
	}
	if kinds["../guide/getting-started.md"] != "page" || kinds["../nope.md"] != "broken" || kinds["../guide/syntax.md#table"] != "page" {
		t.Errorf("link kinds: %v", kinds)
	}
	if !strings.Contains(view.HTML, `href="/basic/guide/getting-started" class="internal"`) || !strings.Contains(view.HTML, `class="broken"`) {
		t.Errorf("rendered links:\n%s", view.HTML)
	}
	if !strings.Contains(view.HTML, `href="/basic/guide/syntax#table"`) {
		t.Errorf("a fragment must survive resolution:\n%s", view.HTML)
	}
	gs, _ := s.RenderPage("guide/getting-started.md")
	var from []string
	for _, b := range gs.Backlinks {
		from = append(from, b.Path)
	}
	slices.Sort(from)
	if strings.Join(from, ",") != "guide/syntax.md,index.md,reference/api.md" {
		t.Errorf("backlinks of getting-started: %v", from)
	}
	if gs.Prev == nil || gs.Prev.Path != "index.md" || gs.Next == nil || gs.Next.Path != "guide/syntax.md" {
		t.Errorf("prev/next: %+v %+v", gs.Prev, gs.Next)
	}
	if got := gs.Crumbs; len(got) != 3 || got[0].Name != "Basic" || got[1].Name != "guide" || got[2].Name != "Getting started" {
		t.Errorf("crumbs: %+v", got)
	}
}

func TestOnlyIndexedPagesAreServed(t *testing.T) {
	s := openFixture(t)
	for _, p := range []string{"../../go.mod", "/etc/passwd", "guide/../../../go.mod", "nope.md", "guide"} {
		if _, err := s.RenderPage(p); err == nil {
			t.Errorf("%q must not be served", p)
		}
	}
}

func TestRescanSeesChanges(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.md", "# A\n")
	var events []Event
	s := newSource("t", "T", KindFolder, "plain", dir, "", render.New(), func(e Event) { events = append(events, e) })
	if _, err := s.Rescan(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("the first scan must not emit events: %v", events)
	}
	if changed, _ := s.Rescan(); changed {
		t.Fatal("nothing changed, yet Rescan reported a change")
	}

	write(t, dir, "sub/b.md", "# B\n")
	if changed, _ := s.Rescan(); !changed || s.Info().Pages != 2 {
		t.Fatalf("new file not seen: %+v", s.Info())
	}
	if events[0].Type != "tree" || events[1].Type != "page" || events[1].Path != "sub/b.md" {
		t.Errorf("events after adding: %v", events)
	}

	events = nil
	time.Sleep(10 * time.Millisecond)
	write(t, dir, "a.md", "# A renamed\n\nmore text so the size differs\n")
	_, _ = s.Rescan()
	if pg, _ := s.Page("a.md"); pg.Title != "A renamed" {
		t.Errorf("edit not seen: %q", pg.Title)
	}
	if len(events) != 2 || events[0].Type != "tree" {
		t.Errorf("a retitled page changes the tree: %v", events)
	}

	events = nil
	_ = os.Remove(filepath.Join(dir, "sub", "b.md"))
	_, _ = s.Rescan()
	if s.Info().Pages != 1 || s.Info().Folders != 0 || events[0].Type != "tree" {
		t.Errorf("delete not seen: %+v %v", s.Info(), events)
	}
}

func TestSymlinksHiddenAndIgnoredAreSkipped(t *testing.T) {
	outside := t.TempDir()
	write(t, outside, "secret.md", "# Secret\n")
	dir := t.TempDir()
	write(t, dir, "ok.md", "# Ok\n")
	write(t, dir, ".hidden/h.md", "# Hidden\n")
	write(t, dir, "node_modules/pkg/readme.md", "# Dep\n")
	write(t, dir, ".dotfile.md", "# Dot\n")
	if err := os.Symlink(outside, filepath.Join(dir, "link-to-outside")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(dir, "secret-link.md")); err != nil {
		t.Skip("symlinks unavailable")
	}
	s := newSource("t", "T", KindFolder, "plain", dir, "", render.New(), nil)
	_, _ = s.Rescan()
	var got []string
	for p := range s.snap.Load().pages {
		got = append(got, p)
	}
	if len(got) != 1 || got[0] != "ok.md" {
		t.Errorf("only ok.md should be indexed, got %v", got)
	}
}

func TestSingleFileSource(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "one.md", "# One\n\n[two](two.md) and [ext](https://x.y)\n")
	write(t, dir, "two.md", "# Two\n")
	s := newSource("f", "one.md", KindFile, "plain", dir, "one.md", render.New(), nil)
	_, _ = s.Rescan()
	if s.Info().Pages != 1 {
		t.Fatalf("a file source has exactly one page: %+v", s.Info())
	}
	v, err := s.RenderPage("one.md")
	if err != nil || !strings.Contains(v.HTML, `class="broken"`) {
		t.Errorf("a link to a sibling that is not part of the source is broken: %v\n%s", err, v.HTML)
	}
}

func TestRootMissingIsReportedNotFatal(t *testing.T) {
	s := newSource("t", "T", KindFolder, "plain", filepath.Join(t.TempDir(), "gone"), "", render.New(), nil)
	if _, err := s.Rescan(); err == nil || s.Info().Error == "" || s.Info().Indexing {
		t.Fatalf("a missing folder must surface an error and finish indexing: %+v", s.Info())
	}
}

func TestSearchIndex(t *testing.T) {
	raw, err := openFixture(t).SearchIndex()
	if err != nil {
		t.Fatal(err)
	}
	var idx struct {
		Docs []struct {
			Path, Title, Text, Headings string
		} `json:"docs"`
	}
	if err := json.Unmarshal(raw, &idx); err != nil || len(idx.Docs) != 4 {
		t.Fatalf("index: %v %s", err, raw)
	}
	found := false
	for _, d := range idx.Docs {
		if d.Path == "guide/syntax.md" && strings.Contains(d.Text, "strikethrough") && strings.Contains(d.Headings, "Admonitions") {
			found = true
		}
	}
	if !found {
		t.Errorf("syntax page not searchable: %s", raw)
	}
}

func TestNaturalOrderAndTitles(t *testing.T) {
	in := []string{"note10", "note2", "Note1", "a", "B", "2026-10-14-x", "2026-9-1-x"}
	slices.SortFunc(in, func(a, b string) int {
		switch {
		case naturalLess(a, b):
			return -1
		case naturalLess(b, a):
			return 1
		}
		return 0
	})
	if got := strings.Join(in, " "); got != "2026-9-1-x 2026-10-14-x a B Note1 note2 note10" {
		t.Errorf("natural order: %s", got)
	}
	for in, want := range map[string]string{"2026-10-14-bread-crumb_plan.md": "Bread crumb plan", "README.md": "README", "my-notes.md": "My notes", "2026-01-01-.md": "2026-01-01-"} {
		if got := titleFromFilename(in); got != want {
			t.Errorf("titleFromFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWatchPicksUpChanges(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.md", "# A\n")
	events := make(chan Event, 16)
	s := newSource("w", "W", KindFolder, "plain", dir, "", render.New(), func(e Event) { events <- e })
	if _, err := s.Rescan(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Watch(ctx)
	time.Sleep(150 * time.Millisecond) // let the watcher attach

	write(t, dir, "new-folder/page.md", "# Fresh\n")
	waitFor(t, events, func(e Event) bool { return e.Type == "page" && e.Path == "new-folder/page.md" })

	write(t, dir, "new-folder/page.md", "# Fresh again, longer\n")
	waitFor(t, events, func(e Event) bool { return e.Type == "tree" })
	if pg, _ := s.Page("new-folder/page.md"); pg == nil || pg.Title != "Fresh again, longer" {
		t.Errorf("the page in a folder created after start-up must be watched: %+v", pg)
	}
}

func waitFor(t *testing.T, ch <-chan Event, ok func(Event) bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case e := <-ch:
			if ok(e) {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for a file event")
		}
	}
}

func newManager(t *testing.T) (*Manager, *config.Store) {
	t.Helper()
	t.Setenv("DOCENT_HOME", t.TempDir())
	paths, _ := config.ResolvePaths()
	store := config.NewStore(paths)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return NewManager(ctx, store), store
}

func TestManagerRegisterOpenRemove(t *testing.T) {
	m, store := newManager(t)
	fix := mustAbs(t, fixtures)

	s, err := m.Register(fix, "Fixture lib", "")
	if err != nil || s.ID != "fixture-lib" || s.Kind != KindLibrary {
		t.Fatalf("register: %+v %v", s, err)
	}
	if err := s.WaitReady(context.Background()); err != nil || s.Info().Pages != 4 {
		t.Fatalf("scan: %+v", s.Info())
	}
	if _, err := m.Register(filepath.Join(fix, "index.md"), "", ""); err == nil {
		t.Error("registering a file must be refused (open it instead)")
	}

	// a different process registers another library; Sync picks it up
	other := t.TempDir()
	write(t, other, "x.md", "# X\n")
	_, _ = store.AddLibrary(other, "Other", "devnotes")
	_ = m.Sync()
	if _, ok := m.Get("other"); !ok || len(m.List()) != 2 {
		t.Fatalf("Sync must add libraries registered elsewhere: %+v", m.List())
	}

	// opening a registered folder returns the library, not a duplicate
	if o, _ := m.Open(fix); o.ID != "fixture-lib" || len(m.List()) != 2 {
		t.Errorf("opening an already open folder: %+v", o)
	}
	// opening a single file creates a temporary source
	f, err := m.Open(filepath.Join(fix, "guide", "syntax.md"))
	if err != nil || f.Kind != KindFile || !strings.HasPrefix(f.ID, "open-") {
		t.Fatalf("open file: %+v %v", f, err)
	}
	if _, err := m.Open(filepath.Join(fix, "..", "..", "..", "go.mod")); err == nil {
		t.Error("a non-Markdown file must be refused")
	}

	if err := m.Remove("other"); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Get("other"); ok {
		t.Error("removed library still open")
	}
	cfg, _ := store.Load()
	if len(cfg.Libraries) != 1 || len(cfg.Recents) == 0 {
		t.Errorf("config: %d libraries, %d recents", len(cfg.Libraries), len(cfg.Recents))
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("removing a library must never touch the folder")
	}
}

func TestServerSideSearch(t *testing.T) {
	s := openFixture(t)
	ctx := context.Background()
	hits, err := s.Search(ctx, "strikethrough", 10)
	if err != nil || len(hits) != 1 || hits[0].Path != "guide/syntax.md" || !strings.Contains(strings.ToLower(hits[0].Snippet), "strikethrough") {
		t.Fatalf("single word: %+v %v", hits, err)
	}
	// every word must be present; order and case do not matter
	if hits, _ := s.Search(ctx, "SYNTAX tour", 10); len(hits) == 0 || hits[0].Path != "guide/syntax.md" {
		t.Errorf("multi word: %+v", hits)
	}
	if hits, _ := s.Search(ctx, "strikethrough zzzznotthere", 10); len(hits) != 0 {
		t.Errorf("a missing word must exclude the page: %+v", hits)
	}
	// a title match outranks a body-only match
	if hits, _ := s.Search(ctx, "getting", 10); len(hits) == 0 || hits[0].Path != "guide/getting-started.md" {
		t.Errorf("title match should rank first: %+v", hits)
	}
	for _, q := range []string{"", "   ", ",,"} {
		if hits, err := s.Search(ctx, q, 10); err != nil || len(hits) != 0 {
			t.Errorf("blank query %q: %+v %v", q, hits, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Search(cancelled, "syntax", 10); err == nil {
		t.Error("a cancelled search must stop with an error")
	}
	// front matter is not part of a snippet
	if hits, _ := s.Search(ctx, "stable", 5); len(hits) > 0 && strings.Contains(hits[0].Snippet, "DOC-0001") {
		t.Errorf("snippet leaked front matter: %q", hits[0].Snippet)
	}
}

func TestSearchIndexModeFollowsSize(t *testing.T) {
	dir := t.TempDir()
	big := "# Big\n\n" + strings.Repeat("lorem ipsum dolor sit amet ", 400) + "\n"
	for i := 0; i < 520; i++ {
		write(t, dir, filepath.Join("d", "p"+strings.Repeat("0", 3)+string(rune('a'+i%26))+string(rune('a'+i/26))+".md"), big)
	}
	s := newSource("big", "Big", KindFolder, "plain", dir, "", render.New(), nil)
	_, _ = s.Rescan()
	raw, err := s.SearchIndex()
	if err != nil || !strings.Contains(string(raw), `"mode":"server"`) || len(raw) > 1000 {
		t.Errorf("a library too large for the browser must say so, not ship a huge index (%d bytes): %.200s", len(raw), raw)
	}
	small, _ := openFixture(t).SearchIndex()
	if !strings.Contains(string(small), `"mode":"client"`) {
		t.Errorf("a small library gets the client index: %.100s", small)
	}
}

func TestPageIsSmall(t *testing.T) {
	s := openFixture(t)
	pg, _ := s.Page("guide/syntax.md")
	if pg.Meta.ID != "DOC-0001" || pg.Meta.Status != "stable" || pg.Meta.Tags != nil {
		t.Errorf("meta: %+v", pg.Meta)
	}
	gs, _ := s.Page("guide/getting-started.md")
	if len(gs.Meta.Tags) != 2 || gs.Meta.Tags[0] != "guide" {
		t.Errorf("tags: %+v", gs.Meta)
	}
	if len(pg.rels) == 0 || pg.rels[0] != "getting-started.md" {
		t.Errorf("relative link targets are kept for backlinks: %v", pg.rels)
	}
}

func TestSearchNonASCII(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.md", "# Café menu\n\nÉCLAIR and Crème brûlée — served daily.\n")
	write(t, dir, "b.md", "# Other\n\nNothing relevant, but an em dash — and an arrow → here, plus ASCII words like Lambda.\n")
	s := newSource("u", "U", KindFolder, "plain", dir, "", render.New(), nil)
	_, _ = s.Rescan()
	ctx := context.Background()
	for _, q := range []string{"éclair", "ÉCLAIR", "brûlée", "café"} {
		if hits, _ := s.Search(ctx, q, 5); len(hits) != 1 || hits[0].Path != "a.md" {
			t.Errorf("non-ASCII query %q: %+v", q, hits)
		}
	}
	if hits, _ := s.Search(ctx, "LAMBDA", 5); len(hits) != 1 || hits[0].Path != "b.md" {
		t.Errorf("ASCII query on a page with non-ASCII text: %+v", hits)
	}
}

func TestRefreshHandlesSingleFileEdits(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.md", "# A\n\nsee [b](b.md)\n")
	write(t, dir, "b.md", "# B\n")
	var events []Event
	s := newSource("r", "R", KindFolder, "plain", dir, "", render.New(), func(e Event) { events = append(events, e) })
	_, _ = s.Rescan()
	before := s.snap.Load()

	// 1. a content-only edit: the page is re-read, the tree and backlinks are reused as they are
	time.Sleep(10 * time.Millisecond)
	write(t, dir, "b.md", "# B\n\nmore text\n")
	if !s.Refresh([]string{"b.md"}) {
		t.Fatal("an edit must be noticed")
	}
	after := s.snap.Load()
	if after.tree != before.tree || len(events) != 1 || events[0].Type != "page" {
		t.Errorf("a content-only edit must not rebuild the tree: events %v", events)
	}
	if pg, _ := s.Page("b.md"); pg.Words <= before.pages["b.md"].Words {
		t.Error("the page was not re-read")
	}

	// 2. a retitle changes the tree
	events = nil
	time.Sleep(10 * time.Millisecond)
	write(t, dir, "b.md", "# B retitled\n\nmore text\n")
	s.Refresh([]string{"b.md"})
	if s.snap.Load().tree == after.tree || events[0].Type != "tree" || s.Info().Pages != 2 {
		t.Errorf("a retitle must rebuild the tree: %v", events)
	}

	// 3. a new link changes the backlinks
	events = nil
	time.Sleep(10 * time.Millisecond)
	write(t, dir, "b.md", "# B retitled\n\nback to [a](a.md)\n")
	s.Refresh([]string{"b.md"})
	if refs := s.snap.Load().back["a.md"]; len(refs) != 1 || refs[0].Path != "b.md" {
		t.Errorf("backlinks after a new link: %+v", refs)
	}

	// 4. a deleted file disappears; a path that never existed is ignored; nothing changed means no work
	events = nil
	_ = os.Remove(filepath.Join(dir, "b.md"))
	if !s.Refresh([]string{"b.md", "ghost.md"}) || s.Info().Pages != 1 || events[0].Type != "tree" {
		t.Errorf("delete: %+v %v", s.Info(), events)
	}
	if s.Refresh([]string{"ghost.md", "a.md"}) {
		t.Error("nothing changed, yet Refresh reported a change")
	}
	// a file that appears
	write(t, dir, "c.md", "# C\n")
	if !s.Refresh([]string{"c.md"}) || s.Info().Pages != 2 {
		t.Errorf("new file via Refresh: %+v", s.Info())
	}
}

func TestEventsAreAnnouncedAfterPublishing(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.md", "# Before\n")
	var s *Source
	var seen []string
	s = newSource("o", "O", KindFolder, "plain", dir, "", render.New(), func(e Event) {
		if pg, ok := s.Page("a.md"); ok { // what a browser would fetch the moment it hears the event
			seen = append(seen, e.Type+":"+pg.Title)
		}
	})
	_, _ = s.Rescan()
	time.Sleep(10 * time.Millisecond)
	write(t, dir, "a.md", "# After the edit\n")
	s.Refresh([]string{"a.md"})
	if len(seen) != 2 || seen[0] != "tree:After the edit" || seen[1] != "page:After the edit" {
		t.Errorf("a listener must see the new data when it hears the event, saw %v", seen)
	}
}

const devnotesFixture = "../../testdata/fixtures/devnotes"

func openDevnotes(t *testing.T, prefix string) *Source {
	t.Helper()
	s := newSource("dev", "Dev notes", KindLibrary, "devnotes", mustAbs(t, devnotesFixture), "", render.New(), nil)
	s.IDPrefix = prefix
	if _, err := s.Rescan(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIDIndexAndDuplicates(t *testing.T) {
	s := openDevnotes(t, "DOC")
	if s.Info().Pages != 9 || s.Info().IDPrefix != "DOC" {
		t.Fatalf("info: %+v", s.Info())
	}
	if pg, ok := s.PageByID("DOC-0006"); !ok || pg.Title != "KafkaJS" {
		t.Errorf("PageByID: %+v %v", pg, ok)
	}
	if _, ok := s.PageByID("DOC-0099"); ok {
		t.Error("an unknown ID must not resolve")
	}
	var kinds []string
	for _, p := range s.Problems() {
		kinds = append(kinds, p.Kind+":"+p.Page)
	}
	slices.Sort(kinds)
	want := []string{
		"duplicate-id:platform/adr/2026-09-01-choice.md", "duplicate-id:platform/runbook/2026-10-20-duplicate-id.md",
		"unknown-id:billing-api/implementation/2026-10-01-profile-events.md",
	}
	if strings.Join(kinds, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems:\n%s\nwant:\n%s", strings.Join(kinds, "\n"), strings.Join(want, "\n"))
	}
}

func TestBacklinksFromIDMentions(t *testing.T) {
	s := openDevnotes(t, "DOC")
	v, err := s.RenderPage("billing-api/bugfix/2026-10-05-null-profile.md")
	if err != nil {
		t.Fatal(err)
	}
	var from []string
	for _, b := range v.Backlinks {
		from = append(from, b.Title)
	}
	if strings.Join(from, ",") != "Profile events" { // it mentions DOC-0002 in prose; the code mention does not count
		t.Errorf("backlinks of DOC-0002: %v", from)
	}
	badge, _ := s.RenderPage("features/badge/implementation/2026-10-10-badge.md")
	if len(badge.Backlinks) != 2 { // the QA note mentions DOC-0004 and the profile-events note lists it under "Related notes"
		t.Errorf("badge backlinks: %+v", badge.Backlinks)
	}
}

func TestIDsInTextAreLinked(t *testing.T) {
	s := openDevnotes(t, "DOC")
	v, _ := s.RenderPage("billing-api/implementation/2026-10-01-profile-events.md")
	if !strings.Contains(v.HTML, `href="/dev/billing-api/bugfix/2026-10-05-null-profile"`) || !strings.Contains(v.HTML, `class="note-ref"`) {
		t.Errorf("DOC-0002 should link to its page:\n%s", v.HTML)
	}
	if !strings.Contains(v.HTML, "<code>DOC-0002</code>") {
		t.Errorf("a mention in code stays plain")
	}
	if len(v.Warnings) != 1 || !strings.Contains(v.Warnings[0], "DOC-0099") {
		t.Errorf("an unknown ID of this library's prefix is a page warning: %v", v.Warnings)
	}
}

func TestResolveIDAcrossLibraries(t *testing.T) {
	m, store := newManager(t)
	m2 := NewManager(context.Background(), store, WithHooks(prefixHooks{}))
	_ = m
	doc, err := m2.Register(mustAbs(t, devnotesFixture), "doc-notes", "devnotes")
	if err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	write(t, other, "api/2026-10-30-x.md", "---\ntitle: Other API\nid: OPS-0001\ndate: 2026-10-30\n---\n\n# Other API\n\nPoints back at DOC-0006 in the other library.\n")
	ops, err := m2.Register(other, "ops-notes", "devnotes")
	if err != nil {
		t.Fatal(err)
	}
	_ = doc.WaitReady(context.Background())
	_ = ops.WaitReady(context.Background())
	if url, title, ok := m2.ResolveID("DOC-0006"); !ok || title != "KafkaJS" || !strings.HasPrefix(url, "/doc-notes/technologies/") {
		t.Errorf("ResolveID across libraries: %q %q %v", url, title, ok)
	}
	v, _ := ops.RenderPage("api/2026-10-30-x.md")
	if !strings.Contains(v.HTML, `href="/doc-notes/technologies/kafkajs/guide/2026-10-12-kafkajs"`) {
		t.Errorf("an ID of another library links across:\n%s", v.HTML)
	}
	if len(v.Warnings) != 0 {
		t.Errorf("an ID resolved elsewhere is not a problem: %v", v.Warnings)
	}
}

// prefixHooks is a minimal profile for tests: the ID prefix is the first word of the library name.
type prefixHooks struct{}

func (prefixHooks) IDPrefix(l config.Library) string {
	return strings.ToUpper(strings.SplitN(l.Name, "-", 2)[0])
}
func (prefixHooks) Validate(config.Library, string, *Page) []string { return nil }
func (prefixHooks) Notices(config.Library, string, *Page) []Notice  { return nil }
func (prefixHooks) Plan(config.Library, string, []byte) any         { return nil }

// A note that lists a tag twice used to crash the page view (the UI keys chips by tag name): the tag is kept once and the repeat is counted.
func TestDuplicateTagsAreKeptOnce(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.md", "---\ntitle: A\ntags:\n  - kafka\n  - nestjs\n  - kafka\n  - nestjs\n  - kafka\n---\n\n# A\n")
	s := newSource("dup", "Dup", KindLibrary, "plain", root, "", render.New(), nil)
	if _, err := s.Rescan(); err != nil {
		t.Fatal(err)
	}
	pg, _ := s.Page("a.md")
	if !slices.Equal(pg.Meta.Tags, []string{"kafka", "nestjs"}) || pg.Meta.DupTags != 3 {
		t.Errorf("tags %v, repeats %d", pg.Meta.Tags, pg.Meta.DupTags)
	}
	v, err := s.RenderPage("a.md")
	if got, _ := v.Meta["tags"].([]any); err != nil || len(got) != 2 {
		t.Errorf("the page view lists each tag once: %v %v", v.Meta["tags"], err)
	}
}
