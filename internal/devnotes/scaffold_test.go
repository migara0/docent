package devnotes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newScaffold(t *testing.T) (Scaffold, string) {
	t.Helper()
	ws := t.TempDir()
	docs := filepath.Join(ws, "doc_developer_notes", "docs")
	_ = os.MkdirAll(docs, 0o755)
	_ = os.MkdirAll(filepath.Join(ws, "billing-api"), 0o755)
	_ = exec.Command("git", "-C", filepath.Join(ws, "billing-api"), "init", "-q").Run()
	s := Settings{Prefix: "DOC", Workspace: ws, Root: docs}
	var ids []string
	sc := Scaffold{S: s, Registry: Registry{Dir: filepath.Join(ws, "reg"), Library: "doc", Prefix: "DOC"}, IDs: func() []string { return ids }}
	return sc, docs
}

func TestCreateFillsTheTemplate(t *testing.T) {
	sc, docs := newScaffold(t)
	rel, id, err := sc.Create(NewNote{Service: "billing-api", Type: "implementation", Title: `Profile "events" & more`, Date: "2026-10-14", Author: "A Dev"})
	if err != nil {
		t.Fatal(err)
	}
	if rel != "billing-api/implementation/2026-10-14-profile-events-more.md" || id != "DOC-0001" {
		t.Fatalf("rel %q id %q", rel, id)
	}
	data, _ := os.ReadFile(filepath.Join(docs, filepath.FromSlash(rel)))
	text := string(data)
	for _, want := range []string{"title: \"Profile \\\"events\\\" & more\"", "id: DOC-0001", "date: 2026-10-14", "author: \"A Dev\"", "  - billing-api", "  - implementation", "# Profile \"events\" & more"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "{{") {
		t.Errorf("an unreplaced placeholder is left:\n%s", text)
	}
	// IDs move on, and never repeat
	_, id2, _ := sc.Create(NewNote{Service: "platform", Type: "adr", Title: "Another", Date: "2026-10-15"})
	if id2 != "DOC-0002" {
		t.Errorf("second id: %s", id2)
	}
}

func TestCreateRules(t *testing.T) {
	sc, docs := newScaffold(t)
	_, _, _ = sc.Create(NewNote{Service: "technologies/kafkajs", Type: "guide", Title: "KafkaJS", Date: "2026-10-14"})
	_, _, _ = sc.Create(NewNote{Service: "billing-api", Type: "module", Title: "Claims", Date: "2026-10-14"})
	cases := []struct {
		name string
		n    NewNote
		want string
	}{
		{"a second guide for a technology", NewNote{Service: "technologies/kafkajs", Type: "guide", Title: "Another"}, "already exists"},
		{"a second module with the same title", NewNote{Service: "billing-api", Type: "module", Title: "Claims", Date: "2026-11-01"}, "one living note per module"},
		{"a guide outside technologies", NewNote{Service: "billing-api", Type: "guide", Title: "X"}, "technologies/"},
		{"a non-guide under technologies", NewNote{Service: "technologies/kafkajs", Type: "adr", Title: "X"}, "only `guide`"},
		{"an unknown service", NewNote{Service: "nope", Type: "adr", Title: "X"}, "unknown service"},
		{"a bad technology name", NewNote{Service: "technologies/Kafka JS", Type: "guide", Title: "X"}, "kebab-case"},
		{"a bad feature name", NewNote{Service: "features/Bad Name", Type: "adr", Title: "X"}, "kebab-case"},
		{"an unknown type", NewNote{Service: "platform", Type: "poem", Title: "X"}, "no template"},
		{"a bad date", NewNote{Service: "platform", Type: "adr", Title: "X", Date: "tomorrow"}, "YYYY-MM-DD"},
		{"an empty title", NewNote{Service: "platform", Type: "adr", Title: "  !!  "}, "empty file name"},
	}
	for _, c := range cases {
		if _, _, err := sc.Create(c.n); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error containing %q", c.name, err, c.want)
		}
	}
	// the same note twice
	_, _, _ = sc.Create(NewNote{Service: "platform", Type: "adr", Title: "Same", Date: "2026-10-14"})
	if _, _, err := sc.Create(NewNote{Service: "platform", Type: "adr", Title: "Same", Date: "2026-10-14"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("an existing note must not be overwritten: %v", err)
	}
	// features and guides are placed in their groups and tagged with the group name
	rel, _, err := sc.Create(NewNote{Service: "features/badge-v2", Type: "qa", Title: "QA", Date: "2026-10-14"})
	if err != nil || rel != "features/badge-v2/qa/2026-10-14-qa.md" {
		t.Errorf("feature note: %q %v", rel, err)
	}
	data, _ := os.ReadFile(filepath.Join(docs, "features", "badge-v2", "qa", "2026-10-14-qa.md"))
	if !strings.Contains(string(data), "  - badge-v2") {
		t.Errorf("a feature note is tagged with the feature name:\n%s", data)
	}
	g, _ := os.ReadFile(filepath.Join(docs, "technologies", "kafkajs", "guide", "2026-10-14-kafkajs.md"))
	if !strings.Contains(string(g), `technology: "kafkajs"`) || !strings.Contains(string(g), "verified: 2026-10-14") {
		t.Errorf("guide template:\n%s", g)
	}
}

func TestLibraryTemplatesOverrideTheBuiltInOnes(t *testing.T) {
	sc, _ := newScaffold(t)
	tdir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tdir, "adr.md"), []byte("---\ntitle: {{title_q}}\nid: {{id}}\n---\nCUSTOM {{title}}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tdir, "poem.md"), []byte("---\ntitle: {{title_q}}\nid: {{id}}\ndate: {{date}}\n---\nroses\n"), 0o644)
	sc.S.Templates = tdir
	rel, _, err := sc.Create(NewNote{Service: "platform", Type: "adr", Title: "Custom", Date: "2026-10-14"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(sc.S.Root, rel))
	if !strings.Contains(string(data), "CUSTOM Custom") {
		t.Errorf("the library's template wins:\n%s", data)
	}
	if _, _, err := sc.Create(NewNote{Service: "platform", Type: "poem", Title: "Verse", Date: "2026-10-14"}); err != nil {
		t.Errorf("a type that exists only in the library's folder works: %v", err)
	}
	types := strings.Join(sc.TemplateTypes(), ",")
	if !strings.Contains(types, "poem") || !strings.Contains(types, "implementation") {
		t.Errorf("template types: %s", types)
	}
}
