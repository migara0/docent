package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDropMakesTemporaryCopies(t *testing.T) {
	e := newEnv(t, false)
	body := `{"files":[{"name":"Notes ü.md","text":"# Dropped title\n\nhello [other](other.md)\n"},{"name":"other.markdown","text":"# Other\n"}]}`
	code, data := e.do(t, "POST", "/_api/drop", body, nil)
	if code != 200 {
		t.Fatalf("drop: %d %s", code, data)
	}
	var out struct {
		Source string
		Pages  []struct{ Name, Path, URL string }
	}
	if err := json.Unmarshal(data, &out); err != nil || out.Source != "dropped" || len(out.Pages) != 2 || out.Pages[0].URL == "" {
		t.Fatalf("response: %s", data)
	}
	// readable straight away, rendered like any page, and the link between the two copies resolves
	code, data = e.do(t, "GET", "/_api/page?src=dropped&path="+"Notes%20%C3%BC.md", "", nil)
	if code != 200 || !strings.Contains(string(data), "Dropped title") {
		t.Fatalf("page: %d %s", code, data)
	}
	// the same name replaces the copy
	if code, _ = e.do(t, "POST", "/_api/drop", `{"files":[{"name":"other.markdown","text":"# Replaced\n"}]}`, nil); code != 200 {
		t.Fatal("second drop")
	}
	if _, data = e.do(t, "GET", "/_api/page?src=dropped&path=other.markdown", "", nil); !strings.Contains(string(data), "Replaced") {
		t.Errorf("a second drop of the same name must replace it: %s", data)
	}
	// it is listed as its own source, is not among the remembered libraries, and not in the recents
	_, data = e.do(t, "GET", "/_api/libraries", "", nil)
	if !strings.Contains(string(data), `"kind":"dropped"`) {
		t.Errorf("libraries: %s", data)
	}
	cfg, _ := e.store.Load()
	if len(cfg.Libraries) != 0 || len(cfg.Recents) != 0 {
		t.Errorf("dropped files must not be remembered: %+v", cfg)
	}
}

func TestDropRefusesWhatIsNotAPlainMarkdownName(t *testing.T) {
	e := newEnv(t, false)
	for _, name := range []string{"../evil.md", "a/b.md", `a\b.md`, ".hidden.md", "notes.txt", "x.md\x00.txt", "", "a:b.md", "con\ntrol.md", strings.Repeat("a", 300) + ".md"} {
		b, _ := json.Marshal(map[string]any{"files": []map[string]string{{"name": name, "text": "x"}}})
		if code, _ := e.do(t, "POST", "/_api/drop", string(b), nil); code != 400 {
			t.Errorf("name %q: status %d, want 400", name, code)
		}
	}
	if code, _ := e.do(t, "POST", "/_api/drop", `{"files":[]}`, nil); code != 400 {
		t.Errorf("no files: %d", code)
	}
	big := strings.Repeat("x", maxDropFile+1)
	b, _ := json.Marshal(map[string]any{"files": []map[string]string{{"name": "big.md", "text": big}}})
	if code, _ := e.do(t, "POST", "/_api/drop", string(b), nil); code != 413 {
		t.Errorf("a file over the limit: %d, want 413", code)
	}
	// refused before anything is created: no folder, no source
	if n := len(e.mgr.Sources()); n != 0 {
		t.Errorf("refused drops must not open anything, but %d source(s) exist", n)
	}
}

func TestDropFolderIsDeletedOnCleanup(t *testing.T) {
	t.Setenv("DOCENT_HOME", t.TempDir())
	e := newEnv(t, false)
	if code, _ := e.do(t, "POST", "/_api/drop", `{"files":[{"name":"a.md","text":"# A"}]}`, nil); code != 200 {
		t.Fatal("drop")
	}
	root := e.mgr.Sources()[0].Root
	if _, err := os.Stat(filepath.Join(root, "a.md")); err != nil {
		t.Fatalf("the copy should exist: %v", err)
	}
	e2 := &Server{mgr: e.mgr}
	e2.drops.dir = root
	e2.Cleanup()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("Cleanup must delete the dropped copies: %v", err)
	}
}

func TestShareLifecycleAndReadOnlyListener(t *testing.T) {
	if len(LANAddrs()) == 0 {
		t.Skip("no local network address on this machine")
	}
	e := newEnv(t, false)
	var info shareInfo
	get := func() shareInfo {
		_, data := e.do(t, "GET", "/_api/share", "", nil)
		var i shareInfo
		_ = json.Unmarshal(data, &i)
		return i
	}
	if info = get(); !info.Available || info.Active || info.URLs == nil {
		t.Fatalf("initially: %+v", info)
	}
	if code, _ := e.do(t, "GET", "/_api/share/qr?url=http://192.168.1.2:8788/x", "", nil); code != 400 {
		t.Errorf("the QR endpoint must refuse when not sharing: %d", code)
	}
	if code, data := e.do(t, "POST", "/_api/share", `{"on":true,"minutes":10}`, nil); code != 200 {
		t.Fatalf("start: %d %s", code, data)
	}
	defer e.do(t, "POST", "/_api/share", `{"on":false}`, nil)
	info = get()
	if !info.Active || len(info.URLs) == 0 || info.Until.IsZero() {
		t.Fatalf("active: %+v", info)
	}
	base := info.URLs[0]

	// the shared listener is read-only and reads the same libraries
	if code, _ := e.do(t, "POST", "/_api/libraries", `{"path":"/tmp"}`, nil); code != 200 && code != 400 {
		t.Logf("main server still accepts changes: %d", code)
	}
	for _, c := range []struct{ method, path, body string }{{"POST", "/_api/open", `{"path":"/tmp"}`}, {"POST", "/_api/drop", `{"files":[{"name":"a.md","text":"x"}]}`}, {"POST", "/_api/share", `{"on":false}`}, {"GET", "/_api/fs?dir=/", ""}} {
		req, _ := http.NewRequest(c.method, base+c.path, strings.NewReader(c.body))
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("the shared address %s should answer: %v", base, err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Errorf("%s %s on the shared listener: %d, want 403", c.method, c.path, res.StatusCode)
		}
	}
	res, err := http.Get(base + "/_api/libraries")
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("reading through the shared listener: %v %v", res, err)
	}
	res.Body.Close()

	// QR: only for the running share's own address
	code, data := e.do(t, "GET", "/_api/share/qr?url="+base+"/lib/page", "", nil)
	if code != 200 || !strings.HasPrefix(string(data), "<svg") || !strings.Contains(string(data), "<path") {
		t.Errorf("qr: %d %.80s", code, data)
	}
	for _, bad := range []string{"http://evil.example.com/", "https://" + strings.TrimPrefix(base, "http://") + "/", "javascript:alert(1)", ""} {
		if code, _ := e.do(t, "GET", "/_api/share/qr?url="+bad, "", nil); code != 400 {
			t.Errorf("qr for %q: %d, want 400", bad, code)
		}
	}

	// stop closes the port
	if code, _ := e.do(t, "POST", "/_api/share", `{"on":false}`, nil); code != 200 {
		t.Fatal("stop")
	}
	if info = get(); info.Active {
		t.Errorf("still active: %+v", info)
	}
	if res, err := http.Get(base + "/_api/health"); err == nil {
		res.Body.Close()
		t.Error("the shared port must be closed after stopping")
	}
}

func TestFindNotesByID(t *testing.T) {
	e := newEnv(t, false)
	fix, _ := filepath.Abs("../../testdata/fixtures/devnotes")
	if code, body := e.do(t, "POST", "/_api/libraries", `{"path":`+jsonStr(fix)+`,"name":"doc-notes","profile":"devnotes"}`, nil); code != 200 {
		t.Fatalf("add: %d %s", code, body)
	}
	src, _ := e.mgr.Get("doc-notes")
	if err := src.WaitReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	find := func(q string) []string {
		_, data := e.do(t, "GET", "/_api/ids?q="+url.QueryEscape(q)+"&src=doc-notes", "", nil)
		var out struct {
			Matches []struct{ ID, Title, URL string }
		}
		if err := json.Unmarshal(data, &out); err != nil || out.Matches == nil {
			t.Fatalf("%q: %s", q, data)
		}
		var ids []string
		for _, m := range out.Matches {
			if m.URL == "" || m.Title == "" {
				t.Errorf("%q: incomplete match %+v", q, m)
			}
			ids = append(ids, m.ID)
		}
		return ids
	}
	for _, q := range []string{"DOC-0003", "doc-3", "doc 3", "DOC-3", "3", "0003", " doc-0003 "} {
		if got := find(q); len(got) != 1 || got[0] != "DOC-0003" {
			t.Errorf("%q found %v, want DOC-0003", q, got)
		}
	}
	// a part of an ID lists the IDs that start with it; a prefix alone lists the newest first
	if got := find("DOC-000"); len(got) < 3 || got[0] != "DOC-0001" {
		t.Errorf("a part of an ID: %v", got)
	}
	if got := find("doc"); len(got) < 3 || got[0] < got[len(got)-1] {
		t.Errorf("a prefix lists the newest first: %v", got)
	}
	for _, q := range []string{"", "nonsense", "OPS-0001", "9999", "DOC-9999", "xx", "a b c", "DOC-0003; drop"} {
		if got := find(q); len(got) != 0 {
			t.Errorf("%q should find nothing, found %v", q, got)
		}
	}

	// /_id/<ID> goes to the note; an unknown ID is a plain 404, and the ID is not reflected as HTML
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(e.srv.URL + "/_id/doc-3")
	if err != nil || res.StatusCode != 302 || !strings.HasPrefix(res.Header.Get("Location"), "/doc-notes/") {
		t.Fatalf("redirect: %v %v", res, err)
	}
	res.Body.Close()
	res, _ = client.Get(e.srv.URL + "/_id/%3Cscript%3E")
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 404 || strings.Contains(string(body), "<script>") {
		t.Errorf("unknown ID: %d %q", res.StatusCode, body)
	}
}
