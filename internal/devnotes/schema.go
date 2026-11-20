package devnotes

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/library"
)

var (
	dayRe      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	fileNameRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-[a-z0-9]+(-[a-z0-9]+)*\.md$`)
	commitsOK  = []string{"in-progress", "released"}
)

// validate returns the problems of one note's front matter and placement. A page that is not where notes live (index.md, tags.md) is not a note.
func validate(s Settings, rel string, pg *library.Page) []string {
	place := Locate(rel)
	if !place.IsNotePlace() || isIndexName(rel) {
		return nil // index.md, tags.md and README.md are pages of the site, not notes
	}
	m := pg.Meta
	var out []string
	add := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }

	if m.Date == "" {
		return []string{"no date in the front matter, so this page is not treated as a note"}
	}
	if !m.HasTitle {
		add("no title in the front matter")
	}
	switch prefix, _, ok := ParseID(m.ID); {
	case m.ID == "":
		add("no id (run `docent id backfill --apply`)")
	case !ok:
		add("malformed id %q (expected %s-NNNN)", m.ID, s.Prefix)
	case prefix != s.Prefix:
		add("id %s has the prefix %s, this library uses %s", m.ID, prefix, s.Prefix)
	}
	if !dayRe.MatchString(m.Date) {
		add("date %q is not YYYY-MM-DD", m.Date)
	}
	if m.Updated != "" && !dayRe.MatchString(m.Updated) {
		add("updated %q is not YYYY-MM-DD", m.Updated)
	}
	if !m.HasTags {
		add("no tags (a note is tagged with its service and type)")
	}
	if m.DupTags > 0 {
		add("tags lists %d tag(s) more than once", m.DupTags)
	}
	// A folder with a name of its own (a project folder under a service, for example) is fine. Only a near miss of a real type is flagged: it is a typo.
	if place.Exact() && !slices.Contains(Types, place.Type) {
		if near := nearestType(place.Type); near != "" {
			add("the folder %q looks like a typo of the note type %q", place.Type, near)
		}
	}
	if !fileNameRe.MatchString(FileName(rel)) {
		add("the file name should be <yyyy-mm-dd>-<kebab-title>.md")
	}
	switch typeOf(place) {
	case "commits":
		if m.Status != "" && !slices.Contains(commitsOK, m.Status) {
			add("status %q must be in-progress or released", m.Status)
		}
		if m.Status == "released" && m.Release == "" {
			add("a released commits note needs a release")
		}
	case "plan":
		out = append(out, planProblems(s, rel)...)
	case "guide":
		if place.Group != Technologies {
			add("guides live under technologies/<technology>/guide/")
		}
		if m.Versions == "" {
			add("a guide needs versions (what it covers)")
		}
		if m.Verified == "" {
			add("a guide needs verified (when it was last checked)")
		} else if !dayRe.MatchString(m.Verified) {
			add("verified %q is not YYYY-MM-DD", m.Verified)
		}
		if m.Technology != "" && place.Group == Technologies && m.Technology != place.Service {
			add("technology %q does not match the folder %q", m.Technology, place.Service)
		}
	}
	return out
}

// Hooks plugs the profile into the library layer.
type Hooks struct {
	Stale *StaleChecker
}

func NewHooks() Hooks { return Hooks{Stale: NewStaleChecker()} }

func (Hooks) IDPrefix(lib config.Library) string { return Resolve(lib).Prefix }

func (Hooks) Validate(lib config.Library, rel string, pg *library.Page) []string {
	return validate(Resolve(lib), rel, pg)
}

func (h Hooks) Notices(lib config.Library, rel string, pg *library.Page) []library.Notice {
	switch Locate(rel).Type {
	case "plan":
		if typeOf(Locate(rel)) == "plan" {
			return planNotices(Resolve(lib), rel, pg)
		}
	case "module":
		if h.Stale == nil {
			return nil
		}
		if n := h.Stale.Check(Resolve(lib), rel, pg); n != nil {
			return []library.Notice{*n}
		}
	}
	return nil
}

// Plan returns the status board of a plan note (nil for any other page).
func (Hooks) Plan(lib config.Library, rel string, src []byte) any {
	if typeOf(Locate(rel)) != "plan" {
		return nil
	}
	doc, updated, err := ParsePlan(src)
	if err != nil {
		return nil
	}
	return doc.View(updated, time.Now())
}

func isIndexName(rel string) bool {
	n := strings.ToLower(strings.TrimSuffix(path.Base(rel), ".md"))
	return n == "index" || n == "readme"
}

// typeOf is the note type when the path follows the strict layout, else "" (the folder there is not necessarily a type).
func typeOf(p Place) string {
	if p.Exact() {
		return p.Type
	}
	return ""
}

// nearestType returns the note type a folder name is one or two letters away from, or "".
func nearestType(name string) string {
	for _, t := range Types {
		if d := editDistance(name, t); d > 0 && d <= 2 && len(name) >= 4 {
			return t
		}
	}
	return ""
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
