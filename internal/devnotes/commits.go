package devnotes

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
)

// Commit tracking: a `commits` note records the commits of one feature or fix, per repo, so they can be cherry-picked into a release.
// It is generated from git (read-only: only `git log` and `git show`; it never stages, commits or cherry-picks), in a block the note's hand-written part never touches:
//
//	<!-- commits:begin {"repo": "billing-api", "range": "develop..feat/x", ...} -->
//	...a table of commits and the cherry-pick commands...
//	<!-- commits:end -->
//
// The block is written exactly as the earlier Python tool wrote it (including its JSON style), so refreshing an existing note changes nothing but the commits.

// CommitSpec says which commits a block records.
type CommitSpec struct {
	Repo   string
	Range  string
	Shas   []string
	Grep   string
	Author string
}

// CommitInfo is one recorded commit.
type CommitInfo struct {
	Sha, Date, Author, Subject string
}

// CommitBlock is a recorded repo of a note.
type CommitBlock struct {
	Spec    CommitSpec
	Commits []CommitInfo
}

var blockRe = regexp.MustCompile(`(?s)<!-- commits:begin (.*?) -->(.*?)<!-- commits:end -->`)

const commitFields = "%H%x1f%ad%x1f%an%x1f%s"

// Commits works on the commits notes of one library.
type Commits struct{ S Settings }

func (c Commits) git(repo string, args ...string) (string, error) {
	dir := filepath.Join(c.S.Workspace, repo)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return "", fmt.Errorf("%q is not a git repo under %s", repo, c.S.Workspace)
	}
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed in %s:\n%s", strings.Join(args, " "), repo, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (c Commits) collect(spec CommitSpec) ([]CommitInfo, error) {
	var lines []string
	if len(spec.Shas) > 0 {
		for _, sha := range spec.Shas {
			out, err := c.git(spec.Repo, "show", "-s", "--date=short", "--format="+commitFields, sha)
			if err != nil {
				return nil, err
			}
			lines = append(lines, strings.TrimSpace(out))
		}
	} else {
		args := []string{"log", "--topo-order", "--reverse", "--no-merges", "--date=short", "--format=" + commitFields, spec.Range}
		if spec.Grep != "" {
			args = append(args, "-i", "--grep="+spec.Grep)
		}
		if spec.Author != "" {
			args = append(args, "--author="+spec.Author)
		}
		out, err := c.git(spec.Repo, args...)
		if err != nil {
			return nil, err
		}
		lines = strings.Split(strings.TrimRight(out, "\n"), "\n")
	}
	var commits []CommitInfo
	for _, l := range lines {
		if l == "" {
			continue
		}
		p := strings.SplitN(l, "\x1f", 4)
		if len(p) != 4 {
			return nil, fmt.Errorf("unexpected git output: %q", l)
		}
		commits = append(commits, CommitInfo{p[0], p[1], p[2], p[3]})
	}
	return commits, nil
}

func pickLines(repo string, shas []string) []string {
	out := []string{fmt.Sprintf("git -C %s cherry-pick -x \\", repo)}
	for i, sha := range shas {
		if i < len(shas)-1 {
			out = append(out, "  "+sha+" \\")
		} else {
			out = append(out, "  "+sha)
		}
	}
	return out
}

// ───────── the JSON of a block, written the way Python's json.dumps wrote it ─────────

func pyString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r < 0x20 || (r >= 0x7f && r < 0x10000):
			fmt.Fprintf(&b, `\u%04x`, r)
		case r >= 0x10000:
			r1, r2 := utf16.EncodeRune(r)
			fmt.Fprintf(&b, `\u%04x\u%04x`, r1, r2)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func pyOpt(s string) string {
	if s == "" {
		return "null"
	}
	return pyString(s)
}

func marker(spec CommitSpec, commits []CommitInfo) string {
	var b strings.Builder
	b.WriteString(`{"repo": ` + pyString(spec.Repo))
	b.WriteString(`, "range": ` + pyOpt(spec.Range))
	if len(spec.Shas) == 0 {
		b.WriteString(`, "shas": null`)
	} else {
		parts := make([]string, len(spec.Shas))
		for i, s := range spec.Shas {
			parts[i] = pyString(s)
		}
		b.WriteString(`, "shas": [` + strings.Join(parts, ", ") + `]`)
	}
	b.WriteString(`, "grep": ` + pyOpt(spec.Grep) + `, "author": ` + pyOpt(spec.Author) + `, "commits": [`)
	for i, c := range commits {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"sha": ` + pyString(c.Sha) + `, "date": ` + pyString(c.Date) + `, "author": ` + pyString(c.Author) + `, "subject": ` + pyString(c.Subject) + `}`)
	}
	b.WriteString("]}")
	return strings.ReplaceAll(b.String(), "--", `\u002d\u002d`)
}

func render(spec CommitSpec, commits []CommitInfo, today string) string {
	source := "`" + spec.Range + "`"
	if len(spec.Shas) > 0 {
		source = "explicit SHAs"
	}
	filters := ""
	if spec.Grep != "" {
		filters += " · grep `" + spec.Grep + "`"
	}
	if spec.Author != "" {
		filters += " · author `" + spec.Author + "`"
	}
	body := []string{fmt.Sprintf("**Source:** %s%s · **Commits:** %d · **Refreshed:** %s", source, filters, len(commits), today), ""}
	if len(commits) > 0 {
		body = append(body, "| # | SHA | Date | Author | Subject |", "| - | --- | ---- | ------ | ------- |")
		shas := make([]string, len(commits))
		for i, c := range commits {
			shas[i] = c.Sha
			body = append(body, fmt.Sprintf("| %d | `%s` | %s | %s | %s |", i+1, c.Sha[:min(10, len(c.Sha))], c.Date, c.Author, strings.ReplaceAll(c.Subject, "|", `\|`)))
		}
		body = append(body, "", "```bash", "# from the workspace root; oldest first; delete a line to skip that commit")
		body = append(body, pickLines(spec.Repo, shas)...)
		body = append(body, "```")
	} else {
		body = append(body, "_No commits matched._")
	}
	return "<!-- commits:begin " + marker(spec, commits) + " -->\n" + strings.Join(body, "\n") + "\n<!-- commits:end -->"
}

// ───────── reading blocks back ─────────

var (
	reRepo   = regexp.MustCompile(`^\{"repo": ("(?:[^"\\]|\\.)*")`)
	reString = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
)

// Blocks reads the recorded repos of a commits note.
func Blocks(text string) []CommitBlock {
	var out []CommitBlock
	for _, m := range blockRe.FindAllStringSubmatch(text, -1) {
		if b, ok := parseMarker(m[1]); ok {
			out = append(out, b)
		}
	}
	return out
}

func unquote(s string) string {
	if len(s) < 2 {
		return ""
	}
	var b strings.Builder
	r := []rune(s[1 : len(s)-1])
	for i := 0; i < len(r); i++ {
		if r[i] != '\\' || i+1 >= len(r) {
			b.WriteRune(r[i])
			continue
		}
		i++
		switch r[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'u':
			if i+4 < len(r) {
				var code int
				fmt.Sscanf(string(r[i+1:i+5]), "%04x", &code)
				i += 4
				if utf16.IsSurrogate(rune(code)) && i+6 < len(r) && r[i+1] == '\\' && r[i+2] == 'u' {
					var low int
					fmt.Sscanf(string(r[i+3:i+7]), "%04x", &low)
					b.WriteRune(utf16.DecodeRune(rune(code), rune(low)))
					i += 6
				} else {
					b.WriteRune(rune(code))
				}
			}
		default:
			b.WriteRune(r[i])
		}
	}
	return b.String()
}

// parseMarker reads the JSON of a block. It is the fixed shape this package writes (and the Python tool wrote), so it is read with a small scanner,
// which keeps the exact formatting stable on a round trip.
func parseMarker(s string) (CommitBlock, bool) {
	var b CommitBlock
	field := func(name string) string {
		i := strings.Index(s, `"`+name+`": `)
		if i < 0 {
			return ""
		}
		return s[i+len(name)+4:]
	}
	if m := reRepo.FindStringSubmatch(s); m != nil {
		b.Spec.Repo = unquote(m[1])
	} else {
		return b, false
	}
	str := func(name string) string {
		rest := field(name)
		if strings.HasPrefix(rest, "null") || rest == "" {
			return ""
		}
		if m := reString.FindString(rest); m != "" && strings.HasPrefix(rest, m) {
			return unquote(m)
		}
		return ""
	}
	b.Spec.Range, b.Spec.Grep, b.Spec.Author = str("range"), str("grep"), str("author")
	if rest := field("shas"); strings.HasPrefix(rest, "[") {
		if end := strings.Index(rest, "]"); end > 0 {
			for _, q := range reString.FindAllString(rest[:end], -1) {
				b.Spec.Shas = append(b.Spec.Shas, unquote(q))
			}
		}
	}
	if rest := field("commits"); strings.HasPrefix(rest, "[") {
		for _, obj := range regexp.MustCompile(`\{"sha": (".*?"), "date": (".*?"), "author": ("(?:[^"\\]|\\.)*"), "subject": ("(?:[^"\\]|\\.)*")\}`).FindAllStringSubmatch(rest, -1) {
			b.Commits = append(b.Commits, CommitInfo{unquote(obj[1]), unquote(obj[2]), unquote(obj[3]), unquote(obj[4])})
		}
	}
	return b, true
}

// FrontMatterValue reads one top-level key of a note's front matter ("" when absent).
func FrontMatterValue(text, key string) string {
	if !strings.HasPrefix(text, "---") {
		return ""
	}
	parts := strings.SplitN(text, "---", 3)
	if len(parts) < 3 {
		return ""
	}
	m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*(.*)$`).FindStringSubmatch(parts[1])
	if m == nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(m[1]), `"`)
}

// ───────── operations ─────────

// Find resolves a note given as a path, a path under docs/, or a unique fragment of the file name.
func (c Commits) Find(note string) (string, error) {
	for _, cand := range []string{note, filepath.Join(c.S.Root, note)} {
		if info, err := os.Stat(cand); err == nil && !info.IsDir() {
			return filepath.Abs(cand)
		}
	}
	var matches []string
	for _, p := range c.All() {
		if strings.Contains(filepath.Base(p), note) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	rel := make([]string, len(matches))
	for i, m := range matches {
		rel[i], _ = filepath.Rel(c.S.Root, m)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no commit notes match %q", note)
	}
	return "", fmt.Errorf("several commit notes match %q:\n  %s", note, strings.Join(rel, "\n  "))
}

// All lists every commits note of the library (a note in a folder named commits, at any depth).
func (c Commits) All() []string {
	var out []string
	_ = filepath.WalkDir(c.S.Root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") && filepath.Base(filepath.Dir(p)) == "commits" {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// Upsert records (or re-reads) one repo in a note and returns the number of commits.
func (c Commits) Upsert(path string, spec CommitSpec) (int, error) {
	commits, err := c.collect(spec)
	if err != nil {
		return 0, err
	}
	block := render(spec, commits, time.Now().Format("2006-01-02"))
	data, crlf, err := readNoteEOL(path)
	if err != nil {
		return 0, err
	}
	text := string(data)
	replaced := false
	text = blockRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := blockRe.FindStringSubmatch(m)
		if b, ok := parseMarker(sub[1]); ok && b.Spec.Repo == spec.Repo && !replaced {
			replaced = true
			return block
		}
		return m
	})
	if !replaced {
		if !regexp.MustCompile(`(?m)^## Commits\s*$`).MatchString(text) {
			text = strings.TrimRight(text, " \n\t") + "\n\n## Commits\n"
		}
		text = strings.TrimRight(text, " \n\t") + "\n\n### " + spec.Repo + "\n\n" + block + "\n"
	}
	return len(commits), os.WriteFile(path, withEOL([]byte(text), crlf), 0o644)
}

// Refresh re-reads every recorded repo of a note from git (a note marked status: released is skipped).
func (c Commits) Refresh(path, onlyRepo string) ([]string, error) {
	data, err := ReadNote(path)
	if err != nil {
		return nil, err
	}
	if FrontMatterValue(string(data), "status") == "released" {
		return nil, fmt.Errorf("skipped: %s is marked status: released (change it to refresh anyway)", filepath.Base(path))
	}
	var done []string
	for _, b := range Blocks(string(data)) {
		if onlyRepo != "" && b.Spec.Repo != onlyRepo {
			continue
		}
		n, err := c.Upsert(path, b.Spec)
		if err != nil {
			return done, err
		}
		done = append(done, fmt.Sprintf("%s: %d commit(s)", b.Spec.Repo, n))
	}
	if len(done) == 0 {
		return nil, fmt.Errorf("no recorded repos to refresh; use `add` first")
	}
	return done, nil
}

// Plan combines the recorded commits of several notes into cherry-pick commands per repo (notes in the order given, oldest commit first).
func (c Commits) Plan(notes []string) (string, []string, error) {
	type repoPlan struct {
		repo string
		shas []string
	}
	var order []*repoPlan
	by := map[string]*repoPlan{}
	var warnings []string
	for _, n := range notes {
		path, err := c.Find(n)
		if err != nil {
			return "", nil, err
		}
		data, _ := ReadNote(path)
		if FrontMatterValue(string(data), "status") == "released" {
			warnings = append(warnings, fmt.Sprintf("# warning: %s is already released", filepath.Base(path)))
		}
		for _, b := range Blocks(string(data)) {
			p := by[b.Spec.Repo]
			if p == nil {
				p = &repoPlan{repo: b.Spec.Repo}
				by[b.Spec.Repo] = p
				order = append(order, p)
			}
			for _, cm := range b.Commits {
				dup := false
				for _, s := range p.shas {
					dup = dup || s == cm.Sha
				}
				if !dup {
					p.shas = append(p.shas, cm.Sha)
				}
			}
		}
	}
	total := 0
	for _, p := range order {
		total += len(p.shas)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "# Cherry-pick plan: %d commit(s) across %d repo(s), run from the workspace root.\n", total, len(order))
	out.WriteString("# Order: notes as given, oldest commit first within each. Resolve conflicts, then `--continue`.\n")
	for _, p := range order {
		out.WriteString("\n")
		if len(p.shas) == 0 {
			fmt.Fprintf(&out, "# %s: nothing recorded\n", p.repo)
		} else {
			out.WriteString(strings.Join(pickLines(p.repo, p.shas), "\n") + "\n")
		}
	}
	return out.String(), warnings, nil
}

// ListLine describes a commits note for `docent commits list`.
func (c Commits) ListLine(path string) string {
	data, _ := ReadNote(path)
	text := string(data)
	var counts []string
	for _, b := range Blocks(text) {
		counts = append(counts, fmt.Sprintf("%s:%d", b.Spec.Repo, len(b.Commits)))
	}
	cs := strings.Join(counts, ", ")
	if cs == "" {
		cs = "no repos yet"
	}
	base := c.S.Root
	if c.S.Site != "" {
		base = c.S.Site
	}
	rel, _ := filepath.Rel(base, path)
	rel = filepath.ToSlash(rel)
	status, release := FrontMatterValue(text, "status"), FrontMatterValue(text, "release")
	if status == "" {
		status = "?"
	}
	if release == "" {
		release = "-"
	}
	return fmt.Sprintf("%-12s %-10s %-50s %s", status, release, cs, rel)
}
