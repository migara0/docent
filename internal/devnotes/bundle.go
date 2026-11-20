package devnotes

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/migara0/docent/internal/library"
)

// Bundling: combine a feature's notes into one Markdown file (or the list a PDF is printed from), for a release hand-off.
// Which notes belong with a `commits` note: under features/<feature>/ every note of that feature; otherwise the notes it mentions (by ID or
// relative link) and the notes that mention it. Order: module, implementation, bugfix, upgrade, api, adr, qa, runbook, guide, commits last.

// DefaultPrintOmit names the sections a printed bundle leaves out (the same list the reader's print uses).
var DefaultPrintOmit = []string{"related notes", "related", "see also"}

var bundleTypeOrder = []string{"module", "implementation", "bugfix", "upgrade", "api", "adr", "qa", "runbook", "guide", "commits"}

// BundleNote is one note of a bundle.
type BundleNote struct {
	ID, Title, Type, Service, Date, Updated, Status string
	Path                                            string // relative to the library root
	Abs                                             string
}

var (
	mdLink      = regexp.MustCompile(`\]\(\s*<?([^)#>\s]+\.md)(?:#[^)]*)?>?\s*\)`)
	fencedBlock = regexp.MustCompile("(?ms)^[ \\t]*(```|~~~).*?^[ \\t]*(```|~~~)[ \\t]*$")
	frontRe     = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n`)
	bHeading    = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*$`)
	bFence      = regexp.MustCompile("^\\s*(```|~~~)")
	bAttr       = regexp.MustCompile(`\s*\{[^{}]*\}\s*$`)
	bLabel      = regexp.MustCompile(`^\*\*(.+?)\*\*`)
	noPrintDiv  = regexp.MustCompile(`(?s)<div class="no-print"[^>]*>.*?</div>\s*`)
)

// Notes lists the notes of a library as bundle candidates.
func Notes(s Settings, src *library.Source) []BundleNote {
	var out []BundleNote
	for _, pg := range src.Pages() {
		if pg.Meta.Date == "" {
			continue
		}
		pl := Locate(pg.Path)
		service := pl.Service
		if pl.Group != "" {
			service = pl.Group + "/" + pl.Service
		}
		updated := pg.Meta.Updated
		if updated == "" {
			updated = pg.Meta.Date
		}
		out = append(out, BundleNote{ID: pg.Meta.ID, Title: pg.Title, Type: pl.Type, Service: service, Date: pg.Meta.Date, Updated: updated, Status: pg.Meta.Status,
			Path: pg.Path, Abs: filepath.Join(s.Root, filepath.FromSlash(pg.Path))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func proseOf(text string) string {
	text = frontRe.ReplaceAllString(text, "")
	text = fencedBlock.ReplaceAllString(text, "")
	return inlineCode.ReplaceAllString(text, "")
}

// linkGraph is {note path: the paths of the notes it mentions}, by note ID or by a relative link to the .md file.
func linkGraph(notes []BundleNote, prefix string) map[string]map[string]bool {
	byID := map[string]string{}
	known := map[string]bool{}
	for _, n := range notes {
		if n.ID != "" {
			byID[n.ID] = n.Path
		}
		known[n.Path] = true
	}
	idRe := regexp.MustCompile(`\b(` + regexp.QuoteMeta(prefix) + `-\d{4,})\b`)
	graph := map[string]map[string]bool{}
	for _, n := range notes {
		data, _ := ReadNote(n.Abs)
		text := proseOf(string(data))
		targets := map[string]bool{}
		for _, m := range idRe.FindAllStringSubmatch(text, -1) {
			if p, ok := byID[m[1]]; ok {
				targets[p] = true
			}
		}
		for _, m := range mdLink.FindAllStringSubmatch(text, -1) {
			href := m[1]
			if strings.Contains(href, "://") || strings.HasPrefix(href, "mailto:") {
				continue
			}
			if r := path.Clean(path.Join(path.Dir(n.Path), href)); known[r] {
				targets[r] = true
			}
		}
		delete(targets, n.Path)
		graph[n.Path] = targets
	}
	return graph
}

// FindNote resolves a note ID (loose forms allowed) or a path to a note of the list.
func FindNote(ref string, notes []BundleNote, prefix string) (BundleNote, error) {
	id, _ := NormalizeID(ref, prefix)
	abs, _ := filepath.Abs(ref)
	for _, n := range notes {
		if (id != "" && n.ID == id) || n.Abs == abs || n.Path == filepath.ToSlash(ref) {
			return n, nil
		}
	}
	return BundleNote{}, fmt.Errorf("no note matches %q (use an ID such as %s-0007 or a path to a note)", ref, prefix)
}

// Members picks the notes a bundle holds, in print order.
func Members(refs []string, notes []BundleNote, prefix string, keepCommits bool) ([]BundleNote, error) {
	byPath := map[string]BundleNote{}
	for _, n := range notes {
		byPath[n.Path] = n
	}
	graph := linkGraph(notes, prefix)
	chosen := map[string]BundleNote{}
	for _, ref := range refs {
		note, err := FindNote(ref, notes, prefix)
		if err != nil {
			return nil, err
		}
		chosen[note.Path] = note
		if note.Type != "commits" {
			continue
		}
		if strings.HasPrefix(note.Service, "features/") {
			for _, o := range notes {
				if o.Service == note.Service {
					chosen[o.Path] = o
				}
			}
			continue
		}
		for p := range graph[note.Path] {
			chosen[p] = byPath[p]
		}
		for src, targets := range graph {
			if targets[note.Path] {
				chosen[src] = byPath[src]
			}
		}
	}
	var out []BundleNote
	for _, n := range chosen {
		if keepCommits || n.Type != "commits" { // --no-commits drops every commits note, even a named one
			out = append(out, n)
		}
	}
	rank := func(t string) int {
		for i, x := range bundleTypeOrder {
			if x == t {
				return i
			}
		}
		return 99
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if rank(a.Type) != rank(b.Type) {
			return rank(a.Type) < rank(b.Type)
		}
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		return a.Path < b.Path
	})
	return out, nil
}

// PrintOmit reads `extra.print_omit` from the site's mkdocs.yml while that file still exists, else the default list.
func PrintOmit(s Settings) []string {
	if s.Site != "" {
		if data, err := os.ReadFile(filepath.Join(s.Site, "mkdocs.yml")); err == nil {
			var cfg struct {
				Extra struct {
					PrintOmit []string `yaml:"print_omit"`
				} `yaml:"extra"`
			}
			if yaml.Unmarshal(data, &cfg) == nil && len(cfg.Extra.PrintOmit) > 0 {
				out := make([]string, len(cfg.Extra.PrintOmit))
				for i, n := range cfg.Extra.PrintOmit {
					out[i] = strings.ToLower(strings.TrimSpace(n))
				}
				return out
			}
		}
	}
	return DefaultPrintOmit
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// convert gives a note's body as Markdown for the bundle: no front matter or title, headings one level down, links between bundled notes as anchors.
func convert(n BundleNote, ids map[string]string, omit []string, keepOmitted bool) string {
	data, _ := ReadNote(n.Abs)
	text := frontRe.ReplaceAllString(string(data), "")
	if !keepOmitted {
		text = noPrintDiv.ReplaceAllString(text, "")
	}
	var out []string
	fence := ""
	skipLevel, droppedTitle, skipParagraph := 0, false, false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if m := bFence.FindStringSubmatch(line); m != nil {
			if fence == "" {
				fence = m[1]
			} else if m[1] == fence {
				fence = ""
			}
			if skipLevel == 0 {
				out = append(out, line)
			}
			continue
		}
		if fence != "" {
			if skipLevel == 0 {
				out = append(out, line)
			}
			continue
		}
		if skipParagraph { // a bold label line such as "**Related notes:** ..." and the paragraph it starts
			skipParagraph = strings.TrimSpace(line) != ""
			continue
		}
		if m := bLabel.FindStringSubmatch(line); m != nil && !keepOmitted && skipLevel == 0 &&
			contains(omit, strings.ToLower(strings.TrimRight(strings.TrimSpace(m[1]), ":"))) {
			skipParagraph = true
			continue
		}
		if m := bHeading.FindStringSubmatch(line); m != nil {
			level, title := len(m[1]), m[2]
			if !droppedTitle && level == 1 {
				droppedTitle = true
				continue
			}
			if skipLevel != 0 && level <= skipLevel {
				skipLevel = 0
			}
			if skipLevel == 0 && !keepOmitted {
				plain := strings.ToLower(strings.TrimRight(strings.TrimSpace(bAttr.ReplaceAllString(title, "")), ":"))
				if strings.Contains(title, "data-no-print") || contains(omit, plain) {
					skipLevel = level
				}
			}
			if skipLevel != 0 {
				continue
			}
			lv := level + 1
			if lv > 6 {
				lv = 6
			}
			out = append(out, strings.Repeat("#", lv)+" "+strings.TrimRight(strings.ReplaceAll(bAttr.ReplaceAllString(title, ""), "{ data-no-print }", ""), " \t"))
			continue
		}
		if skipLevel == 0 {
			out = append(out, line)
		}
	}
	body := strings.Trim(strings.Join(out, "\n"), "\n")
	here := path.Dir(n.Path)
	return mdLink.ReplaceAllStringFunc(body, func(m string) string {
		href := mdLink.FindStringSubmatch(m)[1]
		if id, ok := ids[path.Clean(path.Join(here, href))]; ok {
			return "](#" + strings.ToLower(id) + ")"
		}
		return m
	})
}

// MarkdownBundle is the whole bundle as one Markdown document. site names where the notes come from (shown under the title).
func MarkdownBundle(title, site string, chosen []BundleNote, omit []string, keepOmitted bool, today time.Time) string {
	ids := map[string]string{}
	for _, n := range chosen {
		ids[n.Path] = n.ID
	}
	var contents []string
	for _, n := range chosen {
		contents = append(contents, fmt.Sprintf("- [%s · %s](#%s) (%s, updated %s)", n.ID, n.Title, strings.ToLower(n.ID), n.Type, n.Updated))
	}
	parts := []string{"# " + title + "\n", fmt.Sprintf("Bundle of %d note(s) from %s, generated %s.\n", len(chosen), site, today.Format("2006-01-02")),
		"## Contents\n\n" + strings.Join(contents, "\n") + "\n"}
	for _, n := range chosen {
		var meta []string
		for _, x := range []string{n.Service, n.Type, "created " + n.Date, "updated " + n.Updated, n.Status} {
			if x != "" {
				meta = append(meta, x)
			}
		}
		parts = append(parts, fmt.Sprintf("---\n\n<a id=\"%s\"></a>\n\n## %s · %s\n\n*%s*\n\n%s\n", strings.ToLower(n.ID), n.ID, n.Title, strings.Join(meta, " · "), convert(n, ids, omit, keepOmitted)))
	}
	return strings.Join(parts, "\n")
}
