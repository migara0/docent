package devnotes

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/migara0/docent/internal/slug"
)

//go:embed templates/*.md
var builtinTemplates embed.FS

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// NewNote describes a note to create.
type NewNote struct {
	Service string // "billing-api", "platform", "features/<feature>" or "technologies/<technology>"
	Type    string
	Title   string
	Date    string // YYYY-MM-DD; today when empty
	Author  string // git's user.name when empty
}

// Scaffold creates notes from templates.
type Scaffold struct {
	S        Settings
	Registry Registry
	IDs      func() []string // every note ID currently in the library
}

// templateFor returns the template of a type: the library's own folder wins over the built-in one.
func (sc Scaffold) templateFor(typ string) (string, error) {
	if sc.S.Templates != "" {
		if data, err := os.ReadFile(filepath.Join(sc.S.Templates, typ+".md")); err == nil {
			return string(data), nil
		}
	}
	data, err := builtinTemplates.ReadFile("templates/" + typ + ".md")
	if err != nil {
		return "", fmt.Errorf("no template for the type %q", typ)
	}
	return string(data), nil
}

// TemplateTypes lists the note types that have a template (the built-in ones plus any in the library's template folder).
func (sc Scaffold) TemplateTypes() []string {
	set := map[string]bool{}
	entries, _ := builtinTemplates.ReadDir("templates")
	for _, e := range entries {
		set[strings.TrimSuffix(e.Name(), ".md")] = true
	}
	if sc.S.Templates != "" {
		files, _ := filepath.Glob(filepath.Join(sc.S.Templates, "*.md"))
		for _, f := range files {
			set[strings.TrimSuffix(filepath.Base(f), ".md")] = true
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	slices.Sort(out)
	return out
}

// services lists the places a note can go: platform, the folders already in docs/, and the repos in the workspace.
func (sc Scaffold) services() []string {
	set := map[string]bool{Platform: true}
	if entries, err := os.ReadDir(sc.S.Root); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !slices.Contains(GroupFolders, e.Name()) {
				set[e.Name()] = true
			}
		}
	}
	if entries, err := os.ReadDir(sc.S.Workspace); err == nil {
		for _, e := range entries {
			if _, err := os.Stat(filepath.Join(sc.S.Workspace, e.Name(), ".git")); err == nil && !strings.HasPrefix(e.Name(), ".") {
				set[e.Name()] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

func yamlQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func gitUser() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Create writes the note and returns its path (relative to the library root) and its new ID.
func (sc Scaffold) Create(n NewNote) (rel, id string, err error) {
	date := n.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if !dayRe.MatchString(date) {
		return "", "", fmt.Errorf("the date %q must be YYYY-MM-DD", date)
	}
	titleSlug := slug.Make(n.Title)
	if titleSlug == "" {
		return "", "", fmt.Errorf("the title produced an empty file name")
	}
	tmpl, err := sc.templateFor(n.Type)
	if err != nil {
		return "", "", fmt.Errorf("%w (available: %s)", err, strings.Join(sc.TemplateTypes(), ", "))
	}

	group, name, tag := "", "", n.Service
	switch {
	case strings.HasPrefix(n.Service, Technologies+"/"):
		group, name = Technologies, strings.TrimPrefix(n.Service, Technologies+"/")
		if !kebab.MatchString(name) {
			return "", "", fmt.Errorf("technology %q must be lowercase kebab-case (e.g. kafkajs)", name)
		}
		if n.Type != "guide" {
			return "", "", fmt.Errorf("only `guide` notes go under %s/<technology>", Technologies)
		}
		if existing, _ := filepath.Glob(filepath.Join(sc.S.Root, Technologies, name, "guide", "*.md")); len(existing) > 0 {
			return "", "", fmt.Errorf("a guide for %q already exists: %s\nThere is one living guide per technology: update it (and its verified date) instead of creating another", name, existing[0])
		}
		tag = name
	case strings.HasPrefix(n.Service, Features+"/"):
		group, name = Features, strings.TrimPrefix(n.Service, Features+"/")
		if !kebab.MatchString(name) {
			return "", "", fmt.Errorf("feature %q must be lowercase kebab-case (e.g. badge-print-v2)", name)
		}
		tag = name
	default:
		if n.Type == "guide" {
			return "", "", fmt.Errorf("guides live under %s/<technology> (for example %s/kafkajs)", Technologies, Technologies)
		}
		if !slices.Contains(sc.services(), n.Service) {
			return "", "", fmt.Errorf("unknown service %q. Choose one of:\n  %s\n  %s/<feature>\n  %s/<technology> (guide only)", n.Service, strings.Join(sc.services(), "\n  "), Features, Technologies)
		}
	}

	dir := filepath.Join(sc.S.Root, filepath.FromSlash(n.Service), n.Type)
	if group == "" {
		dir = filepath.Join(sc.S.Root, n.Service, n.Type)
	}
	target := filepath.Join(dir, date+"-"+titleSlug+".md")
	if _, err := os.Stat(target); err == nil {
		return "", "", fmt.Errorf("%s already exists", target)
	}
	if n.Type == "module" {
		if same, _ := filepath.Glob(filepath.Join(dir, "*-"+titleSlug+".md")); len(same) > 0 {
			return "", "", fmt.Errorf("a module note %q already exists: %s\nThere is one living note per module: update it (and set `updated:`) instead of creating another", n.Title, same[0])
		}
	}

	id, err = sc.Registry.Next(sc.IDs())
	if err != nil {
		return "", "", fmt.Errorf("could not reserve an ID: %w", err)
	}
	author := n.Author
	if author == "" {
		author = gitUser()
	}
	body := strings.NewReplacer(
		"{{id}}", id, "{{title_q}}", yamlQuote(n.Title), "{{author_q}}", yamlQuote(author), "{{title}}", n.Title,
		"{{service}}", tag, "{{type}}", n.Type, "{{date}}", date,
	).Replace(tmpl)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		return "", "", err
	}
	rel, _ = filepath.Rel(sc.S.Root, target)
	return filepath.ToSlash(rel), id, nil
}

// InsertID writes `id: X` into a note's front matter, right after its title line (or at the top of the front matter).
func InsertID(path, id string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return fmt.Errorf("%s has no front matter", path)
	}
	at := 1
	for i := 1; i < len(lines) && strings.TrimRight(lines[i], "\r") != "---"; i++ {
		if strings.HasPrefix(lines[i], "id:") {
			return fmt.Errorf("%s already has an id", path)
		}
		if strings.HasPrefix(lines[i], "title:") {
			at = i + 1
		}
	}
	lines = slices.Insert(lines, at, "id: "+id)
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
