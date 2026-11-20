package devnotes

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/migara0/docent/internal/library"
)

// Lint finds leftover template text in notes before they are shared:
//
//	error    an unreplaced template variable such as {{title}}
//	warning  TODO, TBD or FIXME; placeholder IDs such as DOC-xxxx; placeholder lines (`...`, `- ...`, a table row of only `...`);
//	         an empty section; a `<!-- 3. ... -->` instruction comment left over from a template
//
// Code blocks and front matter are ignored, `commits` notes are skipped (their hand-written part is a fill-in checklist), and `<!-- lint: ok -->`
// on a line keeps a deliberate occurrence.

// Finding is one lint result.
type Finding struct {
	Page    string `json:"page"`
	Line    int    `json:"line"`
	Level   string `json:"level"` // error or warning
	Message string `json:"message"`
}

var (
	inlineCode    = regexp.MustCompile("`[^`\n]*`")
	templateVar   = regexp.MustCompile(`\{\{\s*[a-z_]+\s*\}\}`)
	todoMarker    = regexp.MustCompile(`\b(TODO|TBD|FIXME)\b`)
	instruction   = regexp.MustCompile(`<!--\s*\d+\.\s`)
	headingLine   = regexp.MustCompile(`^(#{1,6})\s+\S`)
	fenceLine     = regexp.MustCompile("^\\s*(```|~~~)")
	placeholderID = func(prefix string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(prefix) + `-x{2,}\b|\b[A-Z]{2,5}-x{4}\b`)
	}
)

type bodyLine struct {
	n      int
	text   string
	inCode bool
}

func bodyLines(text string) []bodyLine {
	lines := strings.Split(text, "\n")
	start := 0
	if len(lines) > 0 && lines[0] == "---" {
		for i := 1; i < len(lines); i++ {
			if lines[i] == "---" {
				start = i + 1
				break
			}
		}
	}
	var out []bodyLine
	fence := ""
	for i := start; i < len(lines); i++ {
		l := lines[i]
		if m := fenceLine.FindStringSubmatch(l); m != nil {
			if fence == "" {
				fence = m[1]
			} else if m[1] == fence {
				fence = ""
			}
			out = append(out, bodyLine{i + 1, l, true})
			continue
		}
		out = append(out, bodyLine{i + 1, l, fence != ""})
	}
	return out
}

// LintText lints one note's text.
func LintText(page, text, prefix string) []Finding {
	var found []Finding
	lines := bodyLines(text)
	idRe := placeholderID(prefix)
	var prose []bodyLine
	for _, l := range lines {
		if !l.inCode && !strings.Contains(l.text, "lint: ok") {
			prose = append(prose, l)
		}
	}
	add := func(n int, level, format string, a ...any) {
		found = append(found, Finding{Page: page, Line: n, Level: level, Message: fmt.Sprintf(format, a...)})
	}
	for _, l := range prose {
		raw := l.text
		l.text = inlineCode.ReplaceAllString(l.text, "") // `{{title}}` in code is documentation, not a leftover
		if m := templateVar.FindString(l.text); m != "" {
			add(l.n, "error", "unreplaced template variable %s", m)
		}
		if m := todoMarker.FindString(l.text); m != "" {
			add(l.n, "warning", "%s left in the text", m)
		}
		if m := idRe.FindString(l.text); m != "" {
			add(l.n, "warning", "placeholder ID %s", m)
		}
		s := strings.TrimSpace(raw) // a table row of only `code` cells is not an empty placeholder row
		isRow := false
		if strings.HasPrefix(s, "|") {
			cells := strings.Split(strings.Trim(s, "|"), "|")
			only := true
			has := false
			for _, c := range cells {
				c = strings.TrimSpace(c)
				if c == "..." {
					has = true
				} else if c != "" {
					only = false
				}
			}
			isRow = only && has
		}
		if s == "..." || s == "- ..." || s == "* ..." || s == "- [ ] ..." || isRow {
			add(l.n, "warning", "placeholder line (...)")
		}
		if instruction.MatchString(raw) {
			add(l.n, "warning", "template instruction comment left in")
		}
	}
	// empty sections: a heading, then only blank lines and comments until a heading of the same or a higher level; code counts as content
	type head struct{ idx, level int }
	var heads []head
	for i, l := range lines {
		if !l.inCode {
			if m := headingLine.FindStringSubmatch(l.text); m != nil {
				heads = append(heads, head{i, len(m[1])})
			}
		}
	}
	for k, h := range heads {
		end, next := len(lines), 0
		if k+1 < len(heads) {
			end, next = heads[k+1].idx, heads[k+1].level
		}
		real, inComment := false, false
		for _, l := range lines[h.idx+1 : end] {
			s := strings.TrimSpace(l.text)
			if l.inCode {
				real = true
				break
			}
			if inComment {
				inComment = !strings.Contains(s, "-->")
				continue
			}
			if strings.HasPrefix(s, "<!--") && !strings.Contains(s, "-->") {
				inComment = true
				continue
			}
			if s != "" && !(strings.HasPrefix(s, "<!--") && strings.HasSuffix(s, "-->")) {
				real = true
				break
			}
		}
		if !real && next <= h.level && !strings.Contains(lines[h.idx].text, "lint: ok") {
			add(lines[h.idx].n, "warning", "empty section: %s", strings.TrimSpace(lines[h.idx].text))
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Line < found[j].Line })
	return found
}

// LintLibrary lints every note of a library.
func LintLibrary(s Settings, src *library.Source) []Finding {
	var out []Finding
	for _, pg := range src.Pages() {
		if pg.Meta.Date == "" || Locate(pg.Path).Type == "commits" { // only notes: index pages and plain Markdown are not linted
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(pg.Path)))
		if err != nil {
			continue
		}
		out = append(out, LintText(pg.Path, string(data), s.Prefix)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Page != out[j].Page {
			return out[i].Page < out[j].Page
		}
		return out[i].Line < out[j].Line
	})
	return out
}
