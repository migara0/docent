package library

import (
	"path"
	"sort"
	"strings"

	"github.com/migara0/docent/internal/render"
)

var markdownExt = map[string]bool{".md": true, ".markdown": true}

func isMarkdown(p string) bool { return markdownExt[strings.ToLower(path.Ext(p))] }

// resolveTarget finds which page a relative link points to. from is the linking page's path.
// It understands "other.md", "../x/y.md", "folder/" (its index or README) and "page" (without the extension).
func resolveTarget(pages map[string]*Page, from, href string) (string, bool) {
	if href == "" {
		return "", false
	}
	var p string
	if strings.HasPrefix(href, "/") {
		p = path.Clean(strings.TrimPrefix(href, "/"))
	} else {
		p = path.Join(path.Dir(from), href)
	}
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", false // leaves the source
	}
	if _, ok := pages[p]; ok {
		return p, true
	}
	for _, cand := range []string{p + ".md", p + ".markdown", path.Join(p, "index.md"), path.Join(p, "README.md"), path.Join(p, "readme.md")} {
		if _, ok := pages[cand]; ok {
			return cand, true
		}
	}
	return "", false
}

// indexIDs maps every note ID to its page, and lists the IDs that more than one page claims.
func indexIDs(pages map[string]*Page) (map[string]string, map[string][]string) {
	ids := map[string]string{}
	var dups map[string][]string
	for p, pg := range pages {
		id := pg.Meta.ID
		if id == "" {
			continue
		}
		if prev, taken := ids[id]; taken {
			if dups == nil {
				dups = map[string][]string{}
			}
			if len(dups[id]) == 0 {
				dups[id] = []string{prev}
			}
			dups[id] = append(dups[id], p)
			if p < prev { // keep the choice of "the" page for an ID deterministic
				ids[id] = p
			}
			continue
		}
		ids[id] = p
	}
	for id := range dups {
		sort.Strings(dups[id])
	}
	return ids, dups
}

// resolveBacklinks finds, for every page, which other pages link to it: by a relative link, or by mentioning its note ID.
func resolveBacklinks(srcID string, pages map[string]*Page, ids map[string]string) map[string][]Ref {
	back := map[string][]Ref{}
	seen := map[[2]string]bool{}
	add := func(target, from string, pg *Page) {
		if target == from || seen[[2]string{target, from}] {
			return
		}
		seen[[2]string{target, from}] = true
		back[target] = append(back[target], Ref{Path: from, Title: pg.Title, URL: pageURL(srcID, from)})
	}
	for p, pg := range pages {
		for _, rel := range pg.rels {
			if target, ok := resolveTarget(pages, p, rel); ok {
				add(target, p, pg)
			}
		}
		for _, id := range pg.ids {
			if target, ok := ids[id]; ok {
				add(target, p, pg)
			}
		}
	}
	for t := range back {
		sortRefs(back[t])
	}
	return back
}

// classifyLinks resolves the links of one opened page against the page index.
func classifyLinks(pages map[string]*Page, from string, links []render.Link) []Link {
	out := make([]Link, 0, len(links))
	for _, l := range links {
		switch l.Kind {
		case render.LinkExternal:
			out = append(out, Link{Text: l.Text, Href: l.Href, Kind: "external"})
		case render.LinkAnchor:
			out = append(out, Link{Text: l.Text, Href: l.Href, Kind: "anchor"})
		default:
			if target, ok := resolveTarget(pages, from, l.Path); ok {
				out = append(out, Link{Text: l.Text, Href: l.Href, Kind: "page", Target: target})
			} else if isMarkdown(l.Path) || strings.HasSuffix(l.Path, "/") {
				out = append(out, Link{Text: l.Text, Href: l.Href, Kind: "broken"})
			} else {
				out = append(out, Link{Text: l.Text, Href: l.Href, Kind: "file"})
			}
		}
	}
	return out
}

func sortRefs(refs []Ref) {
	for i := 1; i < len(refs); i++ {
		for j := i; j > 0 && naturalLess(refs[j].Title, refs[j-1].Title); j-- {
			refs[j], refs[j-1] = refs[j-1], refs[j]
		}
	}
}

// resolverFor builds the render.Resolver for one page.
func resolverFor(srcID string, snap *snapshot, from string) render.Resolver {
	return func(l render.Link) render.Resolved {
		if target, ok := resolveTarget(snap.pages, from, l.Path); ok {
			u := pageURL(srcID, target)
			if l.Fragment != "" {
				u += "#" + l.Fragment
			}
			return render.Resolved{URL: u, Class: "internal"}
		}
		if isMarkdown(l.Path) || strings.HasSuffix(l.Path, "/") {
			return render.Resolved{URL: "#", Class: "broken", Title: "Page not found: " + l.Path}
		}
		return render.Resolved{URL: "#", Class: "link-file", Title: l.Path + " (file links are not opened)"}
	}
}
