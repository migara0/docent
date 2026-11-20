package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestCorpus renders every note of a real collection and checks that nothing in the source went missing or leaked through unrendered.
// The collections are private, so they are referenced by path: DOCENT_CORPUS=/path/to/workspace go test ./internal/render -run Corpus -v
func TestCorpus(t *testing.T) {
	root := os.Getenv("DOCENT_CORPUS")
	if root == "" {
		t.Skip("set DOCENT_CORPUS to a folder containing Markdown notes to run this test")
	}
	var files []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") && strings.Contains(p, "_developer_notes/docs/") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		t.Skip("no *_developer_notes/docs notes under DOCENT_CORPUS")
	}
	r := New()
	var (
		admonition = regexp.MustCompile(`(?m)^[ \t]*(!!!|\?\?\?\+?)[ \t]+[A-Za-z]`)
		headingSrc = regexp.MustCompile(`(?m)^ {0,3}#{1,6}[ \t]+\S`)
		headingOut = regexp.MustCompile(`<h[1-6][ >]`)
		leaked     = []string{"!!! ", "??? ", "{ #", "{ data-", "{ .", "&lt;div", "&lt;details", "markdown=", "&lt;!--"}
		problems   []string
		spent      time.Duration
		totals     = map[string]int{}
	)
	for _, f := range files {
		src, _ := os.ReadFile(f)
		name := strings.TrimPrefix(f, root+"/")
		t0 := time.Now()
		res, err := r.Process(src, Options{HTML: true})
		spent += time.Since(t0)
		if err != nil {
			problems = append(problems, name+": render error: "+err.Error())
			continue
		}
		if res.Title == "" {
			problems = append(problems, name+": no title")
		}
		for _, w := range res.Warnings {
			problems = append(problems, name+": warning: "+w)
		}
		// the source without fenced code, for counting constructs
		_, body, _ := splitFrontMatter([]byte(strings.ReplaceAll(string(src), "\r\n", "\n")))
		plain := stripFences(string(body))

		adm := len(admonition.FindAllString(plain, -1))
		got := strings.Count(res.HTML, `class="callout `) + strings.Count(res.HTML, `class="fold `)
		if adm != got {
			problems = append(problems, name+": admonitions in source "+itoa(adm)+", in output "+itoa(got))
		}
		mer := topLevelFences(string(body), "mermaid")
		if gotM := strings.Count(res.HTML, `<pre class="mermaid">`); mer != gotM {
			problems = append(problems, name+": mermaid blocks in source "+itoa(mer)+", in output "+itoa(gotM))
		}
		h := len(headingSrc.FindAllString(plain, -1))
		hOut := len(headingOut.FindAllString(res.HTML, -1))
		if d := h - hOut; d < 0 || d > 1 { // one H1 may be removed because it became the page title
			problems = append(problems, name+": headings in source "+itoa(h)+", in output "+itoa(hOut))
		}
		visible := stripTags(res.HTML)
		for _, l := range leaked {
			if strings.Contains(visible, l) && !strings.Contains(string(src), "```text") {
				problems = append(problems, name+": syntax leaked into the output: "+l)
			}
		}
		totals["pages"]++
		totals["callouts"] += got
		totals["diagrams"] += mer
		totals["headings"] += hOut
	}
	t.Logf("rendered %v in %v (%v per page)", totals, spent.Round(time.Millisecond), (spent / time.Duration(max(totals["pages"], 1))).Round(10*time.Microsecond))
	for _, p := range problems {
		t.Error(p)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

var fenceLine = regexp.MustCompile("^[ \t]*(```+|~~~+)")

// stripFences removes fenced code blocks so that examples of syntax inside code are not counted as syntax.
func stripFences(s string) string {
	var out []string
	in, mark := false, ""
	for _, line := range strings.Split(s, "\n") {
		if m := fenceLine.FindStringSubmatch(line); m != nil {
			if !in {
				in, mark = true, m[1]
			} else if m[1][0] == mark[0] && len(m[1]) >= len(mark) {
				in = false
			}
			continue
		}
		if !in {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

var tag = regexp.MustCompile(`<pre[\s\S]*?</pre>|<code[\s\S]*?</code>|<[^>]+>`)

// stripTags returns the visible text outside code, for leak detection.
func stripTags(h string) string { return tag.ReplaceAllString(h, " ") }

// topLevelFences counts fenced blocks of the given language that are not inside another fence.
func topLevelFences(s, lang string) int {
	n, in, mark := 0, false, ""
	for _, line := range strings.Split(s, "\n") {
		if m := fenceLine.FindStringSubmatch(line); m != nil {
			if !in {
				in, mark = true, m[1]
				if strings.EqualFold(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "`~")), lang) {
					n++
				}
			} else if m[1][0] == mark[0] && len(m[1]) >= len(mark) {
				in = false
			}
		}
	}
	return n
}
