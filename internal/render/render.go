// Package render turns Markdown into HTML and into the facts the library needs about a page (title, headings, links, text).
//
// The dialect is CommonMark and GitHub Flavored Markdown plus callouts (!!! / ??? and > [!NOTE]), attribute lists on headings, definition lists,
// footnotes, task lists, Mermaid fences and optional YAML front matter. See docs/dialect.md. Images are out of scope: they show their alt text only.
package render

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.yaml.in/yaml/v3"

	"github.com/migara0/docent/internal/slug"
)

// Heading is one heading of a page, for the table of contents.
type Heading struct {
	Level int    `json:"level"`
	ID    string `json:"id"`
	Text  string `json:"text"`
}

// LinkKind says what a link points to.
type LinkKind int

const (
	LinkExternal LinkKind = iota // http, https, mailto, ...
	LinkAnchor                   // #section on the same page
	LinkRelative                 // a path relative to this page (a page, a folder or some other file)
)

// Link is a link found in a page.
type Link struct {
	Href     string
	Text     string
	Kind     LinkKind
	Path     string // for LinkRelative: the path part as written, URL-decoded, without #fragment
	Fragment string
}

// Resolved tells the renderer what to emit for a relative link.
type Resolved struct {
	URL   string // the new href; empty leaves the link as written
	Class string // "internal", "broken" or "link-file"
	Title string
}

// Resolver maps a relative link to a Docent URL. The library supplies one per page.
type Resolver func(Link) Resolved

// Options control one Process call.
type Options struct {
	Resolve Resolver  // optional
	HTML    bool      // false analyses only (title, headings, links, word count) and skips HTML generation, which is much cheaper
	Text    bool      // also return the plain text of the page (for search indexes); analysis alone does not need it
	IDs     *IDLinker // optional: find note IDs in the text (and link them when rendering)
}

// Result is everything Process learns about a page.
type Result struct {
	Title    string
	Meta     map[string]any
	HTML     string
	Headings []Heading
	Links    []Link
	Words    int
	Summary  string
	Text     string
	Warnings []string
	IDs      []string // note IDs mentioned in the text (not in code), without the page's own
}

// Renderer is safe for concurrent use.
type Renderer struct {
	md     goldmark.Markdown // everything, for rendering pages
	fast   goldmark.Markdown // without URL auto-linking (which only ever makes external links): same structure, cheaper, for analysis
	policy *bluemonday.Policy
}

var (
	keyResolver = parser.NewContextKey()
	keyAttrs    = parser.NewContextKey()
	keyLinks    = parser.NewContextKey()
)

// New builds a renderer. Create one and reuse it.
func New() *Renderer {
	build := func(linkify bool) goldmark.Markdown {
		exts := []goldmark.Extender{
			extension.NewTable(extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
			extension.Strikethrough, extension.TaskList, extension.Footnote, extension.DefinitionList,
			admonitionExtension{}, codeExtension{}, nodesExtension{},
		}
		if linkify {
			exts = append(exts, extension.Linkify)
		}
		return goldmark.New(goldmark.WithExtensions(exts...), goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(docTransformer{}, 100))))
	}
	return &Renderer{md: build(true), fast: build(false), policy: newPolicy()}
}

// Process renders or analyses one Markdown file.
func (r *Renderer) Process(src []byte, o Options) (*Result, error) {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	res := &Result{}
	meta, body, warn := splitFrontMatter(src)
	if warn != "" {
		res.Warnings = append(res.Warnings, warn)
	}
	res.Meta = meta
	body, attrs := preprocess(body)

	pc := parser.NewContext()
	pc.Set(keyAttrs, attrs)
	links := &[]Link{}
	pc.Set(keyLinks, links)
	if o.Resolve != nil {
		pc.Set(keyResolver, o.Resolve)
	}
	md := r.md
	if !o.HTML {
		md = r.fast
	}
	doc := md.Parser().Parse(text.NewReader(body), parser.WithContext(pc))

	// Title: front matter wins, then the first top-level H1. The H1 is dropped from the body when it is (or gave) the title,
	// because the page shows the title itself.
	var h1 *ast.Heading
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		if h, ok := c.(*ast.Heading); ok && h.Level == 1 {
			h1 = h
			break
		}
	}
	fmTitle, _ := meta["title"].(string)
	fmTitle = strings.TrimSpace(fmTitle)
	switch {
	case fmTitle != "":
		res.Title = fmTitle
		if h1 != nil && strings.EqualFold(plainText(h1, body), fmTitle) {
			doc.RemoveChild(doc, h1)
		}
	case h1 != nil:
		res.Title = plainText(h1, body)
		doc.RemoveChild(doc, h1)
	}

	processIDs(doc, body, o.IDs, o.HTML, res)
	collect(doc, body, res, o.Text)
	res.Links = *links

	if o.HTML {
		var buf bytes.Buffer
		if err := md.Renderer().Render(&buf, body, doc); err != nil {
			return nil, fmt.Errorf("render: %w", err)
		}
		out := tableWrap.Replace(buf.String())
		res.HTML = string(r.policy.SanitizeBytes([]byte(out)))
	}
	return res, nil
}

var tableWrap = strings.NewReplacer("<table>", `<div class="table"><table>`, "</table>", "</table></div>")

// ───────── front matter ─────────

func splitFrontMatter(src []byte) (map[string]any, []byte, string) {
	if !bytes.HasPrefix(src, []byte("---\n")) {
		return nil, src, ""
	}
	rest := src[4:]
	pos := 0
	for pos < len(rest) {
		end := bytes.IndexByte(rest[pos:], '\n')
		var line []byte
		if end < 0 {
			line, end = rest[pos:], len(rest)-pos
		} else {
			line = rest[pos : pos+end]
		}
		if t := bytes.TrimRight(line, " \t"); string(t) == "---" || string(t) == "..." {
			var m map[string]any
			if err := yaml.Unmarshal(rest[:pos], &m); err != nil {
				return nil, src, "front matter is not valid YAML and was ignored: " + firstLine(err.Error())
			}
			body := rest[min(pos+end+1, len(rest)):]
			return normalizeMeta(m), body, ""
		}
		pos += end + 1
	}
	return nil, src, "" // no closing line: not front matter
}

func firstLine(s string) string { return strings.SplitN(s, "\n", 2)[0] }

// normalizeMeta turns YAML dates into plain "2006-01-02" strings so they look the same in JSON as in the file.
func normalizeMeta(v map[string]any) map[string]any {
	for k, x := range v {
		v[k] = normalizeValue(x)
	}
	return v
}

func normalizeValue(x any) any {
	switch t := x.(type) {
	case time.Time:
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
			return t.Format("2006-01-02")
		}
		return t.Format(time.RFC3339)
	case map[string]any:
		return normalizeMeta(t)
	case []any:
		for i := range t {
			t[i] = normalizeValue(t[i])
		}
		return t
	}
	return x
}

// ───────── AST work done while parsing ─────────

type docTransformer struct{}

func (docTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	src := reader.Source()
	attrs, _ := pc.Get(keyAttrs).(map[int]attrList)
	resolve, _ := pc.Get(keyResolver).(Resolver)
	links, _ := pc.Get(keyLinks).(*[]Link)
	idx := newLineIndex(src)
	used := map[string]int{}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Heading:
			var a attrList
			if line, ok := headingLine(n, idx); ok {
				a = attrs[line]
			}
			id := a.ID
			if id == "" {
				id = slug.Make(plainText(n, src))
			}
			if id == "" {
				id = "section"
			}
			base, count := id, used[id]
			used[base]++
			if count > 0 {
				id = fmt.Sprintf("%s-%d", base, count)
			}
			n.SetAttributeString("id", []byte(id))
			if len(a.Classes) > 0 {
				n.SetAttributeString("class", []byte(strings.Join(a.Classes, " ")))
			}
			for _, kv := range a.Attrs {
				if validAttrName(kv[0]) {
					n.SetAttributeString(kv[0], []byte(kv[1]))
				}
			}
		case *ast.Link:
			l := classify(string(n.Destination), plainText(n, src))
			if links != nil {
				*links = append(*links, l) // as written, before any rewriting below
			}
			if resolve == nil || l.Kind != LinkRelative {
				break
			}
			if res := resolve(l); res.URL != "" || res.Class != "" {
				if res.URL != "" {
					n.Destination = []byte(res.URL)
				}
				if res.Class != "" {
					n.SetAttributeString("class", []byte(res.Class))
				}
				if res.Title != "" {
					n.Title = []byte(res.Title)
				}
			}
		}
		return ast.WalkContinue, nil
	})
}

func validAttrName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_' || r == ':' || (i > 0 && (r >= '0' && r <= '9' || r == '-' || r == '.'))
		if !ok {
			return false
		}
	}
	return true
}

// classify says what a link destination points to.
func classify(dest, text string) Link {
	l := Link{Href: dest, Text: text}
	switch {
	case dest == "" || strings.HasPrefix(dest, "#"):
		l.Kind, l.Fragment = LinkAnchor, strings.TrimPrefix(dest, "#")
	case strings.HasPrefix(dest, "//"):
		l.Kind = LinkExternal
	default:
		if u, err := url.Parse(dest); err == nil && u.Scheme != "" {
			l.Kind = LinkExternal
			return l
		}
		p, frag, _ := strings.Cut(dest, "#")
		p, _, _ = strings.Cut(p, "?")
		if dec, err := url.PathUnescape(p); err == nil {
			p = dec
		}
		l.Kind, l.Path, l.Fragment = LinkRelative, p, frag
	}
	return l
}

// ───────── facts collected after parsing ─────────

func collect(doc ast.Node, src []byte, res *Result, wantText bool) {
	var text strings.Builder
	words := 0
	summaryDone := false
	addText := func(b []byte) {
		words += countWords(b)
		if wantText {
			text.Write(b)
		}
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Heading:
			id, _ := n.AttributeString("id")
			b, _ := id.([]byte)
			res.Headings = append(res.Headings, Heading{Level: n.Level, ID: string(b), Text: plainText(n, src)})
		case *ast.Paragraph:
			if !summaryDone && n.Parent() != nil && n.Parent().Kind() == ast.KindDocument {
				res.Summary = truncate(plainText(n, src), 200)
				summaryDone = true
			}
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			for i := 0; i < n.Lines().Len(); i++ {
				seg := n.Lines().At(i)
				addText(seg.Value(src))
				if wantText {
					text.WriteByte(' ')
				}
			}
		case *ast.Text:
			addText(n.Segment.Value(src))
			if wantText && (n.SoftLineBreak() || n.HardLineBreak()) {
				text.WriteByte(' ')
			}
		case *ast.String:
			addText(n.Value)
		}
		if wantText && n.Type() == ast.TypeBlock {
			text.WriteByte(' ')
		}
		return ast.WalkContinue, nil
	})
	res.Words = words
	if wantText {
		res.Text = strings.Join(strings.Fields(text.String()), " ")
	}
}

// countWords counts runs of non-space bytes without allocating.
func countWords(b []byte) int {
	n, in := 0, false
	for _, c := range b {
		space := c == ' ' || c == '\n' || c == '\t' || c == '\r'
		if !space && !in {
			n++
		}
		in = !space
	}
	return n
}

// plainText returns the visible text of a node, without markup.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			b.Write(c.Segment.Value(src))
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.CodeSpan:
			for ch := c.FirstChild(); ch != nil; ch = ch.NextSibling() {
				if t, ok := ch.(*ast.Text); ok {
					b.Write(t.Segment.Value(src))
				}
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(b.String()), " ")
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n])) + "…"
}
