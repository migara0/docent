package render

import (
	"regexp"
	"strings"
	"testing"
)

var (
	dangerousTag  = regexp.MustCompile(`(?i)<\s*(script|iframe|style|object|embed|form|link|meta|base)\b`)
	handlerAttr   = regexp.MustCompile(`(?i)<[^>]*\son[a-z]+\s*=`)
	dangerousHref = regexp.MustCompile(`(?i)\b(href|src|action|formaction|xlink:href|srcset|poster)\s*=\s*["']?\s*(javascript|vbscript|data):`)
)

// FuzzProcess: whatever bytes a Markdown file holds, rendering must not panic or hang, and the HTML must never contain what the sanitiser forbids.
// Run longer with: go test ./internal/render -run '^$' -fuzz FuzzProcess -fuzztime 60s
func FuzzProcess(f *testing.F) {
	for _, s := range []string{
		"", "# T\n\ntext", "---\ntitle: x\ntags: [a, b]\n---\n\n# H", "---\n: : :\n---\n", "---\ntitle: [unclosed\n---\n",
		"!!! note \"x\"\n    body\n", "| a | b |\n| - | - |\n| 1 | 2 |\n", "```mermaid\ngraph TD; A-->B\n```", "```go title=\"x\"\nfunc main() {}\n```",
		"<script>alert(1)</script>", "[x](javascript:alert(1))", "![x](data:text/html,<script>1</script>)", "## H { #id .c data-no-print }\n", "DOC-0001 and DOC-9999999999",
		"[a](../b.md#c) [d](<e f.md>) [g](http://h) [i](mailto:j@k)", "- [ ] a\n  - [x] b\n", "~~~\nunclosed", "\x00\xff\xfe", strings.Repeat("> ", 500) + "deep",
	} {
		f.Add([]byte(s))
	}
	r := New()
	f.Fuzz(func(t *testing.T, src []byte) {
		res, err := r.Process(src, Options{HTML: true})
		if err != nil {
			return
		}
		for name, re := range map[string]*regexp.Regexp{"a dangerous tag": dangerousTag, "an event handler": handlerAttr, "a script URL": dangerousHref} {
			if m := re.FindString(res.HTML); m != "" {
				t.Fatalf("the sanitiser let %s through (%q) for input %q", name, m, src)
			}
		}
	})
}
