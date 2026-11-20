package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/devnotes"
	"github.com/migara0/docent/internal/library"
)

const fixtures = "../../testdata/fixtures/basic"

type env struct {
	srv   *httptest.Server
	mgr   *library.Manager
	store *config.Store
}

func newEnv(t *testing.T, lan bool) *env {
	t.Helper()
	t.Setenv("DOCENT_HOME", t.TempDir())
	paths, _ := config.ResolvePaths()
	store := config.NewStore(paths)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr := library.NewManager(ctx, store, library.WithHooks(devnotes.NewHooks()))
	app := New(mgr, store, Options{Version: "test", LAN: lan, DropDir: t.TempDir()})
	srv := httptest.NewServer(app.Handler())
	t.Cleanup(srv.Close)
	t.Cleanup(app.Cleanup)
	return &env{srv, mgr, store}
}

func (e *env) do(t *testing.T, method, path, body string, headers map[string]string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data
}

func fixtureAbs(t *testing.T) string {
	a, err := filepath.Abs(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAPIFlow(t *testing.T) {
	e := newEnv(t, false)
	if code, body := e.do(t, "GET", "/_api/health", "", nil); code != 200 || !strings.Contains(string(body), `"version":"test"`) {
		t.Fatalf("health: %d %s", code, body)
	}
	code, body := e.do(t, "POST", "/_api/libraries", `{"path":`+jsonStr(fixtureAbs(t))+`,"name":"Basic"}`, nil)
	if code != 200 {
		t.Fatalf("add: %d %s", code, body)
	}
	var info library.Info
	_ = json.Unmarshal(body, &info)
	if info.ID != "basic" || info.Kind != library.KindLibrary {
		t.Fatalf("info: %+v", info)
	}

	code, body = e.do(t, "GET", "/_api/tree?src=basic", "", nil)
	if code != 200 || !strings.Contains(string(body), `"pages":4`) {
		t.Fatalf("tree: %d %s", code, body)
	}
	code, body = e.do(t, "GET", "/_api/page?src=basic&path=guide/syntax.md", "", nil)
	var pv library.PageView
	_ = json.Unmarshal(body, &pv)
	if code != 200 || pv.Title != "Syntax tour" || !strings.Contains(pv.HTML, "callout") || len(pv.TOC) == 0 {
		t.Fatalf("page: %d %+v", code, pv)
	}
	code, body = e.do(t, "GET", "/_api/search-index?src=basic", "", nil)
	if code != 200 || !strings.Contains(string(body), `"docs"`) {
		t.Fatalf("search index: %d", code)
	}
	if code, _ = e.do(t, "GET", "/_api/page?src=basic&path=../../go.mod", "", nil); code != 404 {
		t.Errorf("path traversal must 404, got %d", code)
	}
	if code, _ = e.do(t, "GET", "/_api/tree?src=nope", "", nil); code != 404 {
		t.Errorf("unknown library must 404, got %d", code)
	}
	code, body = e.do(t, "GET", "/_api/libraries", "", nil)
	if code != 200 || !strings.Contains(string(body), `"libraries"`) || !strings.Contains(string(body), `"recents"`) {
		t.Fatalf("libraries: %d %s", code, body)
	}
	if code, _ = e.do(t, "DELETE", "/_api/libraries/basic", "", nil); code != 200 {
		t.Errorf("remove: %d", code)
	}
	if code, _ = e.do(t, "DELETE", "/_api/libraries/basic", "", nil); code != 404 {
		t.Errorf("removing twice must 404, got %d", code)
	}
}

func jsonStr(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestOpenOnceDoesNotPersist(t *testing.T) {
	e := newEnv(t, false)
	if code, body := e.do(t, "POST", "/_api/open", `{"path":`+jsonStr(filepath.Join(fixtureAbs(t), "guide", "syntax.md"))+`}`, nil); code != 200 {
		t.Fatalf("open: %d %s", code, body)
	}
	cfg, _ := e.store.Load()
	if len(cfg.Libraries) != 0 || len(cfg.Recents) != 1 {
		t.Errorf("an opened file is remembered as recent only: %+v", cfg)
	}
	if code, _ := e.do(t, "POST", "/_api/open", `{"path":"/etc/hostname"}`, nil); code != 400 {
		t.Errorf("a non-Markdown file must be refused, got %d", code)
	}
}

func TestSecurity(t *testing.T) {
	e := newEnv(t, false)
	body := `{"path":` + jsonStr(fixtureAbs(t)) + `}`
	cases := []struct {
		name    string
		method  string
		path    string
		body    string
		headers map[string]string
		want    int
	}{
		{"forged Host (DNS rebinding)", "GET", "/_api/libraries", "", map[string]string{"Host": "evil.example.com"}, 403},
		{"cross-origin write", "POST", "/_api/libraries", body, map[string]string{"Origin": "https://evil.example.com"}, 403},
		{"cross-site fetch metadata", "POST", "/_api/libraries", body, map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"form post instead of JSON", "POST", "/_api/libraries", body, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 415},
		{"same-origin write is fine", "POST", "/_api/libraries", body, map[string]string{"Sec-Fetch-Site": "same-origin"}, 200},
		{"bad profile", "POST", "/_api/libraries", `{"path":"/tmp","profile":"x"}`, nil, 400},
		{"garbage JSON", "POST", "/_api/open", `{nope`, nil, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, _ := http.NewRequest(c.method, e.srv.URL+c.path, strings.NewReader(c.body))
			req.Header.Set("Content-Type", "application/json")
			for k, v := range c.headers {
				if k == "Host" {
					req.Host = v
				} else {
					req.Header.Set(k, v)
				}
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != c.want {
				t.Errorf("status %d, want %d", res.StatusCode, c.want)
			}
		})
	}
	// every response carries the hardening headers
	res, err := http.Get(e.srv.URL + "/_api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if res.Header.Get(h) == "" {
			t.Errorf("missing %s header", h)
		}
	}
}

func TestLANModeIsReadOnly(t *testing.T) {
	e := newEnv(t, true)
	for _, c := range []struct{ method, path, body string }{
		{"POST", "/_api/libraries", `{"path":"/tmp"}`}, {"POST", "/_api/open", `{"path":"/tmp"}`}, {"DELETE", "/_api/libraries/x", ""}, {"GET", "/_api/fs?dir=/", ""},
	} {
		if code, _ := e.do(t, c.method, c.path, c.body, nil); code != 403 {
			t.Errorf("%s %s in LAN mode: %d, want 403", c.method, c.path, code)
		}
	}
	for _, c := range []struct{ method, path, body string }{{"POST", "/_api/drop", `{"files":[{"name":"a.md","text":"x"}]}`}, {"POST", "/_api/share", `{"on":true}`}} {
		if code, _ := e.do(t, c.method, c.path, c.body, nil); code != 403 {
			t.Errorf("%s %s in LAN mode: %d, want 403", c.method, c.path, code)
		}
	}
	// reading still works, and a non-loopback Host is accepted (that is the point of LAN mode)
	req, _ := http.NewRequest("GET", e.srv.URL+"/_api/libraries", nil)
	req.Host = "192.168.1.6:8787"
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Errorf("LAN read: %d", res.StatusCode)
	}
}

func TestFolderPicker(t *testing.T) {
	e := newEnv(t, false)
	dir := fixtureAbs(t)
	code, body := e.do(t, "GET", "/_api/fs?dir="+dir, "", nil)
	if code != 200 {
		t.Fatalf("fs: %d %s", code, body)
	}
	var out struct {
		Dir     string
		Parent  string
		Entries []fsEntry
		Crumbs  []fsCrumb
	}
	_ = json.Unmarshal(body, &out)
	names := []string{}
	for _, en := range out.Entries {
		names = append(names, en.Kind+":"+en.Name)
		if en.Name == "guide" && en.MD != 2 {
			t.Errorf("guide holds 2 Markdown files, got %d", en.MD)
		}
	}
	if got := strings.Join(names, " "); got != "dir:empty-folder dir:guide dir:reference file:index.md" {
		t.Errorf("entries: %s", got)
	}
	if len(out.Crumbs) == 0 || out.Parent == "" {
		t.Errorf("crumbs/parent missing: %+v", out)
	}
	if code, _ = e.do(t, "GET", "/_api/fs?dir=relative/path", "", nil); code != 400 {
		t.Errorf("a relative dir must be refused, got %d", code)
	}
	if code, _ = e.do(t, "GET", "/_api/fs?dir=/definitely/not/here", "", nil); code != 404 {
		t.Errorf("a missing dir must 404, got %d", code)
	}
	hidden := t.TempDir()
	_ = os.Mkdir(filepath.Join(hidden, ".secret"), 0o755)
	_ = os.Mkdir(filepath.Join(hidden, "visible"), 0o755)
	_, body = e.do(t, "GET", "/_api/fs?dir="+hidden, "", nil)
	if strings.Contains(string(body), ".secret") || !strings.Contains(string(body), "visible") {
		t.Errorf("hidden folders must not be listed by default: %s", body)
	}
}

func TestLiveEvents(t *testing.T) {
	e := newEnv(t, false)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n"), 0o644)
	src, err := e.mgr.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = src.WaitReady(context.Background())
	time.Sleep(200 * time.Millisecond) // let the watcher attach

	res, err := http.Get(e.srv.URL + "/_api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	got := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "data: ") {
				got <- sc.Text()
			}
		}
	}()
	time.Sleep(100 * time.Millisecond)
	_ = os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B\n"), 0o644)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case line := <-got:
			if strings.Contains(line, `"path":"b.md"`) {
				return
			}
		case <-deadline:
			t.Fatal("no live event after creating a file")
		}
	}
}

func TestUIFallsBackToIndex(t *testing.T) {
	e := newEnv(t, false)
	for _, p := range []string{"/", "/some-library/some/page", "/open-1234/"} {
		code, body := e.do(t, "GET", p, "", nil)
		if code != 200 || !strings.Contains(string(body), "<title>Docent</title>") {
			t.Errorf("GET %s: %d", p, code)
		}
	}
	if code, _ := e.do(t, "GET", "/_api/nonexistent", "", nil); code == 200 {
		t.Error("unknown API paths must not fall back to the UI")
	}
}

func TestCreatedEndpoint(t *testing.T) {
	e := newEnv(t, false)
	_, _ = e.do(t, "POST", "/_api/libraries", `{"path":`+jsonStr(fixtureAbs(t))+`,"name":"Basic"}`, nil)
	code, body := e.do(t, "GET", "/_api/created?src=basic", "", nil)
	var out struct{ Pages []library.Row }
	_ = json.Unmarshal(body, &out)
	if code != 200 || len(out.Pages) == 0 {
		t.Fatalf("created: %d %s", code, body)
	}
	for _, p := range out.Pages {
		if p.Created == "" || p.URL == "" {
			t.Errorf("every listed page has its created day and URL: %+v", p)
		}
		if p.CreatedAt != "" && !strings.HasPrefix(p.CreatedAt, p.Created) {
			t.Errorf("the moment falls on the created day: %+v", p)
		}
	}
	if code, _ = e.do(t, "GET", "/_api/created?src=nope", "", nil); code != 404 {
		t.Errorf("unknown library: %d", code)
	}
}

func TestSearchEndpoint(t *testing.T) {
	e := newEnv(t, false)
	_, _ = e.do(t, "POST", "/_api/libraries", `{"path":`+jsonStr(fixtureAbs(t))+`,"name":"Basic"}`, nil)
	code, body := e.do(t, "GET", "/_api/search?src=basic&q=strikethrough&limit=5", "", nil)
	var out struct {
		Query string
		Hits  []library.SearchHit
	}
	_ = json.Unmarshal(body, &out)
	if code != 200 || len(out.Hits) != 1 || out.Hits[0].URL != "/basic/guide/syntax" {
		t.Fatalf("search: %d %s", code, body)
	}
	if code, _ = e.do(t, "GET", "/_api/search?src=nope&q=x", "", nil); code != 404 {
		t.Errorf("unknown library: %d", code)
	}
	if code, body = e.do(t, "GET", "/_api/search?src=basic&q=", "", nil); code != 200 || !strings.Contains(string(body), `"hits":[]`) {
		t.Errorf("empty query: %d %s", code, body)
	}
}

func TestRawAndFilePath(t *testing.T) {
	e := newEnv(t, false)
	_, _ = e.do(t, "POST", "/_api/libraries", `{"path":`+jsonStr(fixtureAbs(t))+`,"name":"Basic"}`, nil)
	code, body := e.do(t, "GET", "/_api/raw?src=basic&path=guide/getting-started.md", "", nil)
	if code != 200 || !strings.HasPrefix(string(body), "---\ntitle: Getting started") {
		t.Fatalf("raw: %d %.80s", code, body)
	}
	if code, _ = e.do(t, "GET", "/_api/raw?src=basic&path=../../go.mod", "", nil); code != 404 {
		t.Errorf("raw must only serve indexed pages, got %d", code)
	}
	_, body = e.do(t, "GET", "/_api/page?src=basic&path=guide/getting-started.md", "", nil)
	var pv library.PageView
	_ = json.Unmarshal(body, &pv)
	if !strings.HasSuffix(pv.File, "guide/getting-started.md") || !filepath.IsAbs(pv.File) {
		t.Errorf("a loopback page carries its absolute path: %q", pv.File)
	}
	lan := newEnv(t, true)
	_ = lan.mgr.Sync()
	if _, err := lan.mgr.Register(fixtureAbs(t), "Basic", ""); err != nil {
		t.Fatal(err)
	}
	_, body = lan.do(t, "GET", "/_api/page?src=basic&path=guide/getting-started.md", "", nil)
	if strings.Contains(string(body), `"file"`) {
		t.Errorf("LAN mode must not reveal local file paths: %s", body[:200])
	}
}

func TestAPIArraysAreNeverNull(t *testing.T) {
	e := newEnv(t, false)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "plain.md"), []byte("Just a line, no headings, no links.\n"), 0o644)
	_, _ = e.do(t, "POST", "/_api/libraries", `{"path":`+jsonStr(dir)+`,"name":"P"}`, nil)
	time.Sleep(100 * time.Millisecond)
	_, body := e.do(t, "GET", "/_api/page?src=p&path=plain.md", "", nil)
	for _, field := range []string{`"toc":[]`, `"links":[]`, `"backlinks":[]`, `"crumbs":[`} {
		if !strings.Contains(string(body), field) {
			t.Errorf("%s missing from %s", field, body)
		}
	}
	if strings.Contains(string(body), ":null") {
		t.Errorf("no field may be null: %s", body)
	}
}

func TestDevnotesViews(t *testing.T) {
	e := newEnv(t, false)
	fix, _ := filepath.Abs("../../testdata/fixtures/devnotes")
	if code, body := e.do(t, "POST", "/_api/libraries", `{"path":`+jsonStr(fix)+`,"name":"doc-notes","profile":"devnotes"}`, nil); code != 200 {
		t.Fatalf("add: %d %s", code, body)
	}
	time.Sleep(150 * time.Millisecond)

	var info struct{ Libraries []library.Info }
	_, body := e.do(t, "GET", "/_api/libraries", "", nil)
	_ = json.Unmarshal(body, &info)
	if len(info.Libraries) != 1 || info.Libraries[0].IDPrefix != "DOC" {
		t.Fatalf("the ID prefix is derived from the library name: %+v", info.Libraries)
	}

	var ov struct {
		Profile string
		Notes   []struct {
			ID, Type, Service, Title, Created, Updated string
		}
	}
	_, body = e.do(t, "GET", "/_api/overview?src=doc-notes&folder=billing-api", "", nil)
	_ = json.Unmarshal(body, &ov)
	types := []string{}
	for _, n := range ov.Notes {
		types = append(types, n.Type)
		if n.Service != "billing-api" || n.ID == "" || n.Created == "" || n.Updated == "" {
			t.Errorf("row: %+v", n)
		}
	}
	if ov.Profile != "devnotes" || strings.Join(types, ",") != "bugfix,commits,implementation,module" {
		t.Errorf("overview of a service folder: %v", types)
	}
	_, body = e.do(t, "GET", "/_api/overview?src=doc-notes&folder=features/badge", "", nil)
	if !strings.Contains(string(body), `"service":"features/badge"`) {
		t.Errorf("a feature folder: %s", body)
	}

	var rec struct {
		Notes []struct{ Title, Updated string }
	}
	_, body = e.do(t, "GET", "/_api/recent?src=doc-notes&limit=3", "", nil)
	_ = json.Unmarshal(body, &rec)
	if len(rec.Notes) != 3 || rec.Notes[0].Updated < rec.Notes[1].Updated {
		t.Errorf("recent: newest first, limited: %+v", rec.Notes)
	}

	var tg struct {
		Tags []struct {
			Name  string
			Count int
		}
	}
	_, body = e.do(t, "GET", "/_api/tags?src=doc-notes", "", nil)
	_ = json.Unmarshal(body, &tg)
	counts := map[string]int{}
	for _, x := range tg.Tags {
		counts[x.Name] = x.Count
	}
	if counts["billing-api"] != 4 || counts["platform"] != 2 || counts["badge"] != 2 {
		t.Errorf("tags: %v", counts)
	}

	var pv library.PageView
	_, body = e.do(t, "GET", "/_api/page?src=doc-notes&path=features/badge/qa/2026-10-11-badge-qa.md", "", nil)
	_ = json.Unmarshal(body, &pv)
	if pv.Place == nil || pv.Place.Group != "features" || pv.Place.Service != "badge" || pv.Place.Type != "qa" {
		t.Errorf("place: %+v", pv.Place)
	}
	_, body = e.do(t, "GET", "/_api/page?src=doc-notes&path=billing-api/implementation/2026-10-01-profile-events.md", "", nil)
	_ = json.Unmarshal(body, &pv)
	if !strings.Contains(pv.HTML, `class="note-ref"`) || len(pv.Warnings) != 1 || !strings.Contains(pv.Warnings[0], "DOC-0099") {
		t.Errorf("IDs link, and an unknown one is a warning: %v", pv.Warnings)
	}
}
