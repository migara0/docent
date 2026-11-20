package devnotes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/migara0/docent/internal/library"
)

// A module note's "Where it lives" section names the code in backticks (relative to the workspace, e.g. `billing-api/src/claims/`; a path that does not
// resolve there, such as `src/handler.ts`, is tried inside the first path on the same line). Git (read-only: `git log -1`) says when each path last changed;
// a note whose `updated:` date (or `date:`) is older than a change is stale.

var (
	whereSection = regexp.MustCompile(`(?m)^##\s+Where it lives\s*$`)
	nextSection  = regexp.MustCompile(`(?m)^##\s`)
	codeSpan     = regexp.MustCompile("`([^`\n]+)`")
)

func section(text string) string {
	start := whereSection.FindStringIndex(text)
	if start == nil {
		return ""
	}
	rest := text[start[1]:]
	if end := nextSection.FindStringIndex(rest); end != nil {
		return rest[:end[0]]
	}
	return rest
}

func looksLikePath(span string) bool {
	return strings.Contains(span, "/") && !strings.Contains(span, " ") && !strings.HasPrefix(span, "http") && !strings.HasPrefix(span, "--") &&
		!strings.HasPrefix(span, "$") && !strings.HasPrefix(span, "<") && !strings.Contains(span, "*")
}

// pathsIn returns the existing paths named in a module note's "Where it lives" section.
func pathsIn(workspace, text string) []string {
	var found []string
	for _, line := range strings.Split(section(text), "\n") {
		base := ""
		for _, m := range codeSpan.FindAllStringSubmatch(line, -1) {
			span := strings.TrimRight(strings.TrimSpace(m[1]), ",;:")
			if !looksLikePath(span) {
				continue
			}
			candidates := []string{filepath.Join(workspace, span)}
			if base != "" {
				candidates = append(candidates, filepath.Join(base, span))
			}
			hit := ""
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					hit = c
					break
				}
			}
			if hit == "" {
				continue
			}
			if base == "" {
				if info, _ := os.Stat(hit); info != nil && info.IsDir() {
					base = hit
				} else {
					base = filepath.Dir(hit)
				}
			}
			dup := false
			for _, f := range found {
				dup = dup || f == hit
			}
			if !dup {
				found = append(found, hit)
			}
		}
	}
	return found
}

// lastChange is the date (YYYY-MM-DD) of the last commit that touched the path on the checked-out branch, or "" (not in a repo, never committed).
func lastChange(path string) string {
	dir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%cs", "--", path).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// StaleChange is a path of a module that changed after the note was last updated.
type StaleChange struct {
	Path    string // relative to the workspace
	Changed string
}

// StaleModule is the result for one module note.
type StaleModule struct {
	Page    string
	ID      string
	Title   string
	Since   string
	Checked int
	Changes []StaleChange
}

// CheckModule compares a module note with the code it names.
func CheckModule(s Settings, rel string, pg *library.Page) StaleModule {
	r := StaleModule{Page: rel, ID: pg.Meta.ID, Title: pg.Title, Since: pg.Meta.Updated}
	if r.Since == "" {
		r.Since = pg.Meta.Date
	}
	data, err := ReadNote(filepath.Join(s.Root, filepath.FromSlash(rel)))
	if err != nil {
		return r
	}
	paths := pathsIn(s.Workspace, string(data))
	r.Checked = len(paths)
	for _, p := range paths {
		if changed := lastChange(p); changed != "" && changed > r.Since {
			rp, _ := filepath.Rel(s.Workspace, p)
			r.Changes = append(r.Changes, StaleChange{Path: filepath.ToSlash(rp), Changed: changed})
		}
	}
	sort.SliceStable(r.Changes, func(i, j int) bool { return r.Changes[i].Changed > r.Changes[j].Changed })
	return r
}

// StaleChecker adds a short cache, so opening a module page does not run git on every request.
type StaleChecker struct {
	mu    sync.Mutex
	cache map[string]cached
	ttl   time.Duration
}

type cached struct {
	at     time.Time
	notice *library.Notice
	mod    time.Time
}

func NewStaleChecker() *StaleChecker {
	return &StaleChecker{cache: map[string]cached{}, ttl: 30 * time.Second}
}

// Check returns a notice when the module's code changed after the note was updated, else nil.
func (c *StaleChecker) Check(s Settings, rel string, pg *library.Page) *library.Notice {
	key := s.Root + "\x00" + rel
	c.mu.Lock()
	if e, ok := c.cache[key]; ok && time.Since(e.at) < c.ttl && e.mod.Equal(pg.Modified) {
		c.mu.Unlock()
		return e.notice
	}
	c.mu.Unlock()
	r := CheckModule(s, rel, pg)
	var n *library.Notice
	if len(r.Changes) > 0 {
		n = &library.Notice{Kind: "warning", Title: "Code changed since this note was last updated", Text: fmt.Sprintf("Updated %s. Check that this module note still matches the code, then set updated:.", r.Since)}
		for i, ch := range r.Changes {
			if i == 8 {
				n.Items = append(n.Items, fmt.Sprintf("and %d more", len(r.Changes)-8))
				break
			}
			n.Items = append(n.Items, fmt.Sprintf("%s last changed %s", ch.Path, ch.Changed))
		}
	}
	c.mu.Lock()
	c.cache[key] = cached{at: time.Now(), notice: n, mod: pg.Modified}
	c.mu.Unlock()
	return n
}

// StaleModules checks every module note of a library.
func StaleModules(s Settings, src *library.Source) []StaleModule {
	var out []StaleModule
	for _, pg := range src.Pages() {
		if Locate(pg.Path).Type == "module" {
			out = append(out, CheckModule(s, pg.Path, pg))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Page < out[j].Page })
	return out
}
