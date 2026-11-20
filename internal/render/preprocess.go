package render

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// Two source-level rewrites run before parsing. Both skip fenced code and keep the line count unchanged for headings.

var (
	alertOpen    = regexp.MustCompile(`^(\s*)>\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*$`)
	headingAttrs = regexp.MustCompile(`^(#{1,6}[ \t]+.*?)[ \t]+\{([^{}]*)\}[ \t]*$`)
)

// fenceMatch returns the fence marker ("```" or "~~~" with its full length) when the line opens or closes a fenced block, else "".
func fenceMatch(line string) string {
	s := strings.TrimLeft(line, " \t")
	if len(s) < 3 || (s[0] != '`' && s[0] != '~') {
		return ""
	}
	n := 0
	for n < len(s) && s[n] == s[0] {
		n++
	}
	if n < 3 {
		return ""
	}
	return s[:n]
}

// attrList is a parsed `{ #id .class key=value bare }` list.
type attrList struct {
	ID      string
	Classes []string
	Attrs   [][2]string // key, value ("" for a bare attribute such as data-no-print)
}

func parseAttrList(s string) attrList {
	var a attrList
	for _, f := range strings.Fields(s) {
		switch {
		case strings.HasPrefix(f, "#") && len(f) > 1:
			a.ID = f[1:]
		case strings.HasPrefix(f, ".") && len(f) > 1:
			a.Classes = append(a.Classes, f[1:])
		case strings.Contains(f, "="):
			k, v, _ := strings.Cut(f, "=")
			a.Attrs = append(a.Attrs, [2]string{k, strings.Trim(v, `"'`)})
		default:
			a.Attrs = append(a.Attrs, [2]string{f, ""})
		}
	}
	return a
}

// preprocess rewrites GitHub alerts (> [!NOTE]) into the !!! form and strips trailing attribute lists from ATX headings.
// It returns the new source and the attributes found, keyed by the 0-based line of the heading.
func preprocess(src []byte) ([]byte, map[int]attrList) {
	lines := strings.Split(string(src), "\n")
	var out []string
	inFence, fenceMark := false, ""
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if m := fenceMatch(line); m != "" {
			if !inFence {
				inFence, fenceMark = true, m
			} else if m[0] == fenceMark[0] && len(m) >= len(fenceMark) {
				inFence = false
			}
			out = append(out, line)
			continue
		}
		if !inFence && strings.Contains(line, "[!") {
			if m := alertOpen.FindStringSubmatch(line); m != nil {
				indent, kind := m[1], strings.ToLower(m[2])
				if kind == "caution" { // GitHub's CAUTION is the severe one (red); keep that meaning and the title
					out = append(out, indent+`!!! danger "Caution"`)
				} else {
					out = append(out, indent+"!!! "+kind)
				}
				for i+1 < len(lines) && strings.HasPrefix(strings.TrimLeft(lines[i+1], " \t"), ">") {
					body := strings.TrimLeft(lines[i+1], " \t")[1:]
					body = strings.TrimPrefix(body, " ")
					out = append(out, indent+"    "+body)
					i++
				}
				continue
			}
		}
		out = append(out, line)
	}
	attrs := map[int]attrList{}
	inFence = false
	for i, line := range out {
		if m := fenceMatch(line); m != "" {
			if !inFence {
				inFence, fenceMark = true, m
			} else if m[0] == fenceMark[0] && len(m) >= len(fenceMark) {
				inFence = false
			}
			continue
		}
		if inFence {
			continue
		}
		if !strings.HasSuffix(strings.TrimRight(line, " \t"), "}") || !strings.Contains(line, "#") {
			continue // a heading attribute list ends the line with }
		}
		if m := headingAttrs.FindStringSubmatch(line); m != nil {
			attrs[i] = parseAttrList(m[2])
			out[i] = m[1]
		}
	}
	return []byte(strings.Join(out, "\n")), attrs
}

// lineIndex maps byte offsets to 0-based line numbers.
type lineIndex []int

func newLineIndex(src []byte) lineIndex {
	idx := lineIndex{0}
	for i, b := range src {
		if b == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

func (l lineIndex) lineOf(offset int) int { return sort.SearchInts(l, offset+1) - 1 }

// headingLine finds the 0-based source line of an ATX heading from its text segment.
func headingLine(h *ast.Heading, idx lineIndex) (int, bool) {
	if h.Lines().Len() == 0 {
		return 0, false
	}
	return idx.lineOf(h.Lines().At(0).Start), true
}

var _ = bytes.MinRead
