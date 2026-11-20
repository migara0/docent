package render

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func mdHTML(t *testing.T, md string) string {
	t.Helper()
	res, err := New().Process([]byte(md), Options{HTML: true})
	if err != nil {
		t.Fatal(err)
	}
	return res.HTML
}

func contains(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func notContains(t *testing.T, got string, bad ...string) {
	t.Helper()
	for _, b := range bad {
		if strings.Contains(got, b) {
			t.Errorf("must not contain %q in:\n%s", b, got)
		}
	}
}

func TestTitleRules(t *testing.T) {
	r := New()
	cases := []struct{ name, src, title, htmlHas, htmlNot string }{
		{"first H1 becomes the title and leaves the body", "# Hello\n\ntext", "Hello", "<p>text</p>", "<h1"},
		{"front matter title wins; an equal H1 is dropped", "---\ntitle: Hello\n---\n# hello\ntext", "Hello", "text", "<h1"},
		{"a different H1 is kept", "---\ntitle: Page title\n---\n# Another\ntext", "Page title", "<h1", ""},
		{"no title at all", "just text", "", "just text", ""},
		{"markup in an H1 is stripped from the title", "# The `code` **title**", "The code title", "", "<h1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := r.Process([]byte(c.src), Options{HTML: true})
			if err != nil || res.Title != c.title {
				t.Fatalf("title %q (err %v), want %q", res.Title, err, c.title)
			}
			if c.htmlHas != "" {
				contains(t, res.HTML, c.htmlHas)
			}
			if c.htmlNot != "" {
				notContains(t, res.HTML, c.htmlNot)
			}
		})
	}
}

func TestFrontMatter(t *testing.T) {
	r := New()
	res, _ := r.Process([]byte("---\ntitle: A\ndate: 2026-11-03\ntags: [x, y]\nupdated: 2026-11-05 10:30:00\n---\nbody"), Options{})
	if res.Meta["date"] != "2026-11-03" {
		t.Errorf("a date must stay a plain date string, got %#v", res.Meta["date"])
	}
	if tags, ok := res.Meta["tags"].([]any); !ok || len(tags) != 2 {
		t.Errorf("tags: %#v", res.Meta["tags"])
	}
	bad, _ := r.Process([]byte("---\ntitle: [unclosed\n---\nbody"), Options{HTML: true})
	if len(bad.Warnings) != 1 || bad.Meta != nil {
		t.Errorf("invalid YAML must warn and be ignored: %+v", bad.Warnings)
	}
	none, _ := r.Process([]byte("---\nno closing line\n\ntext"), Options{HTML: true})
	if none.Meta != nil || len(none.Warnings) != 0 {
		t.Errorf("an opening --- without a closing one is not front matter")
	}
	crlf, _ := r.Process([]byte("\xef\xbb\xbf---\r\ntitle: Win\r\n---\r\n# x\r\n\r\ntext\r\n"), Options{HTML: true})
	if crlf.Title != "Win" {
		t.Errorf("BOM and CRLF must be handled, got title %q", crlf.Title)
	}
}

func TestHeadings(t *testing.T) {
	res, _ := New().Process([]byte("## Same\n\n## Same\n\n### With `code`\n\n## Custom { #my-id .wide data-no-print }\n\n## Same"), Options{HTML: true})
	ids := []string{}
	for _, h := range res.Headings {
		ids = append(ids, h.ID)
	}
	if got := strings.Join(ids, ","); got != "same,same-1,with-code,my-id,same-2" {
		t.Errorf("ids = %s", got)
	}
	contains(t, res.HTML, `id="my-id"`, `class="wide"`, `data-no-print=""`, `href="#my-id"`)
	notContains(t, res.HTML, "{ #my-id")
	if res.Headings[3].Text != "Custom" {
		t.Errorf("attribute list must not leak into the heading text: %q", res.Headings[3].Text)
	}
	// a { ... } inside a code fence is not an attribute list
	code := mdHTML(t, "```\n## Not a heading { #x }\n```\n")
	contains(t, code, "{ #x }")
}

func TestCallouts(t *testing.T) {
	got := mdHTML(t, "!!! warning \"Careful\"\n    Line one.\n\n    ```go\n    x := 1\n    ```\n\nAfter.\n")
	contains(t, got, `class="callout callout--warn"`, `<p class="callout__title">Careful</p>`, "x :=", "<p>After.</p>")
	if strings.Index(got, "After.") < strings.Index(got, "x :=") {
		t.Errorf("content after the callout must stay outside it")
	}
	contains(t, mdHTML(t, "!!! note \"\"\n    No title.\n"), `callout--note`, "No title.")
	notContains(t, mdHTML(t, "!!! note \"\"\n    No title.\n"), "callout__title")
	contains(t, mdHTML(t, "??? tip \"Fold\"\n    hidden\n"), `<details class="fold callout--tip">`, "<summary>")
	contains(t, mdHTML(t, "???+ tip \"Fold\"\n    shown\n"), `open=""`)
	contains(t, mdHTML(t, "> [!IMPORTANT]\n> Read this.\n> Second line.\n\nplain"), `callout--warn`, "Read this.", "Second line.")
	contains(t, mdHTML(t, "> [!CAUTION]\n> boom\n"), "callout--danger")
	contains(t, mdHTML(t, "> Just a quote\n"), "<blockquote>")
	// four spaces before !!! is a code block, not a callout
	notContains(t, mdHTML(t, "para\n\n    !!! note\n"), "callout")
	// nested in a list item
	contains(t, mdHTML(t, "- item\n\n    !!! note\n        inside\n"), "callout--note", "inside")
}

func TestCodeAndMermaid(t *testing.T) {
	got := mdHTML(t, "```ts title=\"a.ts\"\nconst x: number = 1 // hi\n```\n")
	contains(t, got, `data-lang="ts"`, `<span class="name">a.ts</span>`, `tok-k`, `tok-c`, `tok-n`)
	contains(t, mdHTML(t, "```unknownlang\n<b>&</b>\n```"), "&lt;b&gt;&amp;&lt;/b&gt;")
	contains(t, mdHTML(t, "```\nplain\n```"), `data-lang=""`, "plain")
	contains(t, mdHTML(t, "```mermaid\nA --> B\n```"), `<pre class="mermaid">A --&gt; B</pre>`)
	contains(t, mdHTML(t, "    indented code\n"), "indented code")
}

func TestImagesAreAltTextOnly(t *testing.T) {
	got := mdHTML(t, "before ![A diagram](http://x/y.png) after")
	contains(t, got, `class="image-omitted"`, "A diagram")
	notContains(t, got, "<img", "y.png")
}

func TestHostileInput(t *testing.T) {
	bad := []string{
		"<script>alert(1)</script>",
		"<img src=x onerror=alert(1)>",
		"[x](javascript:alert(1))",
		"[x](  JaVaScRiPt:alert(1))",
		"[x](data:text/html;base64,PHNjcmlwdD4=)",
		"<iframe src=//evil></iframe>",
		"<a href=\"javascript:alert(1)\">x</a>",
		"<div onclick=\"x()\">y</div>",
		"![x](javascript:alert(1))",
		"<style>body{display:none}</style>",
		"<svg onload=alert(1)>",
		"<details open ontoggle=alert(1)>",
	}
	for _, md := range bad {
		got := strings.ToLower(mdHTML(t, md))
		for _, frag := range []string{"<script", "onerror", "onclick", "onload", "ontoggle", "javascript:", "<iframe", "<style", "data:text"} {
			if strings.Contains(got, frag) {
				t.Errorf("input %q leaked %q:\n%s", md, frag, got)
			}
		}
	}
	// the sanitiser also guards the callout icons: only the page's own sprite is allowed
	g := mdHTML(t, "!!! note\n    x\n")
	contains(t, g, `<use href="#i-info"`)
}

func TestLinks(t *testing.T) {
	r := New()
	md := "[a](other.md#sec) [b](../up/x.md) [c](https://example.com) [d](#here) [e](some%20file.txt) [f](mailto:a@b.c) [g](gone.md)"
	res, _ := r.Process([]byte(md), Options{HTML: true, Resolve: func(l Link) Resolved {
		switch l.Path {
		case "other.md":
			return Resolved{URL: "/lib/other#" + l.Fragment, Class: "internal"}
		case "gone.md":
			return Resolved{Class: "broken", Title: "Page not found"}
		case "some file.txt":
			return Resolved{Class: "link-file"}
		}
		return Resolved{}
	}})
	want := []struct {
		kind LinkKind
		path string
	}{{LinkRelative, "other.md"}, {LinkRelative, "../up/x.md"}, {LinkExternal, ""}, {LinkAnchor, ""}, {LinkRelative, "some file.txt"}, {LinkExternal, ""}, {LinkRelative, "gone.md"}}
	if len(res.Links) != len(want) {
		t.Fatalf("links: %+v", res.Links)
	}
	for i, w := range want {
		if res.Links[i].Kind != w.kind || res.Links[i].Path != w.path {
			t.Errorf("link %d = %+v, want %+v", i, res.Links[i], w)
		}
	}
	if res.Links[0].Href != "other.md#sec" || res.Links[0].Fragment != "sec" {
		t.Errorf("links must be reported as written, got %+v", res.Links[0])
	}
	contains(t, res.HTML, `href="/lib/other#sec" class="internal"`, `class="broken"`, `title="Page not found"`, `class="link-file"`, `href="https://example.com"`, `href="#here"`)
}

func TestAnalyseOnlySkipsHTML(t *testing.T) {
	src := []byte("# T\n\nBody with a [link](x.md) and `code`.\n\n```go\nfmt.Println()\n```\n\nSee https://example.com now.\n")
	res, _ := New().Process(src, Options{})
	if res.HTML != "" || res.Title != "T" || len(res.Links) != 1 || res.Words < 10 || res.Text != "" {
		t.Errorf("analysis without text: %+v", res)
	}
	withText, _ := New().Process(src, Options{Text: true})
	if !strings.Contains(withText.Text, "fmt.Println()") || withText.Words != res.Words {
		t.Errorf("analysis with text: %+v", withText)
	}
	// a bare URL is auto-linked when rendering, but analysis (which skips auto-linking) must report the same explicit links either way
	full, _ := New().Process(src, Options{HTML: true})
	if !strings.Contains(full.HTML, `<a href="https://example.com">`) || len(full.Links) != len(res.Links) {
		t.Errorf("rendering auto-links bare URLs: %d links\n%s", len(full.Links), full.HTML)
	}
	if res.Summary != "Body with a link and code." {
		t.Errorf("summary = %q", res.Summary)
	}
}

func TestGoldenFixtures(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/fixtures/basic/**/*.md")
	more, _ := filepath.Glob("../../testdata/fixtures/basic/*.md")
	files = append(files, more...)
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		t.Run(name, func(t *testing.T) {
			src, _ := os.ReadFile(f)
			res, err := New().Process(src, Options{HTML: true})
			if err != nil {
				t.Fatal(err)
			}
			got := "title: " + res.Title + "\n" + res.HTML
			golden := filepath.Join("testdata", name+".golden.html")
			if *update {
				_ = os.MkdirAll("testdata", 0o755)
				_ = os.WriteFile(golden, []byte(got), 0o644)
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("missing golden file (run: go test ./internal/render -update): %v", err)
			}
			if string(want) != got {
				t.Errorf("output differs from %s (run with -update after checking the change)\n--- got ---\n%s", golden, got)
			}
		})
	}
}

func idLinker(known map[string]string, own string) *IDLinker {
	return &IDLinker{
		Pattern: regexp.MustCompile(`\b((?:DOC|OPS)-\d{4,})\b`),
		Resolve: func(id string) (string, string, bool) {
			if u, ok := known[id]; ok {
				return u, "Title of " + id, true
			}
			return "", "", false
		},
		Own: func(id string) bool { return strings.HasPrefix(id, own+"-") },
	}
}

func TestNoteIDsBecomeLinks(t *testing.T) {
	l := idLinker(map[string]string{"DOC-0007": "/doc/runbook/cutover", "OPS-0003": "/ops/api/x"}, "DOC")
	md := "See DOC-0007 and, in the other library, OPS-0003. Also `DOC-0007` in code, [DOC-0007](x.md) in a link,\nand DOC-0007 at a line start. DOC-9999 is unknown, OPS-9999 belongs to another library.\n\n## Heading about DOC-0007\n\n```\nDOC-0007 in a fence\n```\n"
	res, _ := New().Process([]byte(md), Options{HTML: true, IDs: l})
	if n := strings.Count(res.HTML, `class="note-ref"`); n != 3 {
		t.Errorf("want 3 linked mentions (prose, other library, line start), got %d:\n%s", n, res.HTML)
	}
	contains(t, res.HTML, `<a href="/doc/runbook/cutover" title="Title of DOC-0007" class="note-ref">DOC-0007</a>`, `<a href="/ops/api/x"`, "<code>DOC-0007</code>")
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "DOC-9999") {
		t.Errorf("an unresolved ID of the library's own prefix is a warning; another prefix is not: %v", res.Warnings)
	}
	if got := strings.Join(res.IDs, ","); got != "DOC-0007,OPS-0003,DOC-9999,OPS-9999" {
		t.Errorf("mentioned IDs: %s", got)
	}
}

func TestOwnIDIsNotLinkedToItself(t *testing.T) {
	l := idLinker(map[string]string{"DOC-0007": "/x"}, "DOC")
	l.Self = "DOC-0007"
	res, _ := New().Process([]byte("This note is DOC-0007."), Options{HTML: true, IDs: l})
	if strings.Contains(res.HTML, "note-ref") || len(res.IDs) != 0 {
		t.Errorf("a note must not link or count itself: %s %v", res.HTML, res.IDs)
	}
}

func TestIDsAreCollectedWithoutRendering(t *testing.T) {
	res, _ := New().Process([]byte("Mentions DOC-0001, DOC-0002 and DOC-0001 again."), Options{IDs: idLinker(nil, "DOC")})
	if strings.Join(res.IDs, ",") != "DOC-0001,DOC-0002" || res.HTML != "" {
		t.Errorf("analysis mode: %v", res.IDs)
	}
}
