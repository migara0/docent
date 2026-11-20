package devnotes

import (
	"strings"
	"testing"

	"github.com/migara0/docent/internal/library"
)

func note(rel string, m library.Meta) *library.Page { return &library.Page{Path: rel, Meta: m} }

func good() library.Meta {
	return library.Meta{ID: "DOC-0007", Date: "2026-10-14", HasTitle: true, HasTags: true, Tags: []string{"svc", "implementation"}}
}

func TestValidate(t *testing.T) {
	s := Settings{Prefix: "DOC"}
	cases := []struct {
		name string
		rel  string
		mut  func(*library.Meta)
		want string // substring of the single expected problem; "" means no problem
	}{
		{"a good note", "billing-api/implementation/2026-10-14-x.md", nil, ""},
		{"index.md and tags.md are not notes", "index.md", func(m *library.Meta) { *m = library.Meta{} }, ""},
		{"a page where notes live, without a date", "billing-api/implementation/2026-10-14-x.md", func(m *library.Meta) { m.Date = "" }, "not treated as a note"},
		{"no id", "billing-api/implementation/2026-10-14-x.md", func(m *library.Meta) { m.ID = "" }, "no id"},
		{"malformed id", "billing-api/implementation/2026-10-14-x.md", func(m *library.Meta) { m.ID = "DOC-7" }, "malformed id"},
		{"another library's prefix", "billing-api/implementation/2026-10-14-x.md", func(m *library.Meta) { m.ID = "OPS-0001" }, "prefix OPS"},
		{"bad date", "billing-api/implementation/2026-10-14-x.md", func(m *library.Meta) { m.Date = "14/10/2026" }, "not YYYY-MM-DD"},
		{"bad updated", "billing-api/implementation/2026-10-14-x.md", func(m *library.Meta) { m.Updated = "yesterday" }, "updated"},
		{"no title", "billing-api/implementation/2026-10-14-x.md", func(m *library.Meta) { m.HasTitle = false }, "no title"},
		{"no tags", "billing-api/implementation/2026-10-14-x.md", func(m *library.Meta) { m.HasTags = false }, "no tags"},
		{"a tag listed twice", "billing-api/implementation/2026-10-14-x.md", func(m *library.Meta) { m.DupTags = 4 }, "4 tag(s) more than once"},
		{"a folder with a name of its own is fine", "billing-api/bread-crumb/2026-10-14-x.md", nil, ""},
		{"a typo of a type is flagged", "billing-api/implmentation/2026-10-14-x.md", nil, `typo of the note type "implementation"`},
		{"another near miss", "billing-api/bugfixes/2026-10-14-x.md", nil, `"bugfix"`},
		{"bad file name", "billing-api/implementation/Some Title.md", nil, "file name"},
		{"commits status", "billing-api/commits/2026-10-14-x.md", func(m *library.Meta) { m.Status = "done" }, "in-progress or released"},
		{"released without release", "billing-api/commits/2026-10-14-x.md", func(m *library.Meta) { m.Status = "released" }, "needs a release"},
		{"guide outside technologies", "billing-api/guide/2026-10-14-x.md", func(m *library.Meta) { m.Versions, m.Verified = "1", "2026-10-14" }, "technologies/"},
		{"guide without versions", "technologies/kafkajs/guide/2026-10-14-x.md", func(m *library.Meta) { m.Verified = "2026-10-14" }, "versions"},
		{"guide without verified", "technologies/kafkajs/guide/2026-10-14-x.md", func(m *library.Meta) { m.Versions = "2.2" }, "verified"},
		{"guide technology mismatch", "technologies/kafkajs/guide/2026-10-14-x.md", func(m *library.Meta) { m.Versions, m.Verified, m.Technology = "2", "2026-10-14", "mongoose" }, "does not match"},
		{"a good guide", "technologies/kafkajs/guide/2026-10-14-x.md", func(m *library.Meta) { m.Versions, m.Verified, m.Technology = "2", "2026-10-14", "kafkajs" }, ""},
		{"a good feature note", "features/badge/implementation/2026-10-14-x.md", nil, ""},
		{"a sub-folder index is not a note", "svc/project/blocks/index.md", func(m *library.Meta) { *m = library.Meta{} }, ""},
		{"a deeper path is allowed: its folder is not a note type", "svc/project/blocks/2026-10-14-x.md", nil, ""},
		{"a deeper path still needs an id", "svc/project/blocks/2026-10-14-x.md", func(m *library.Meta) { m.ID = "" }, "no id"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := good()
			if c.mut != nil {
				c.mut(&m)
			}
			got := validate(s, c.rel, note(c.rel, m))
			if c.want == "" {
				if len(got) != 0 {
					t.Fatalf("want no problems, got %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], c.want) {
				t.Fatalf("want one problem containing %q, got %v", c.want, got)
			}
		})
	}
}

func TestLocate(t *testing.T) {
	for rel, want := range map[string]Place{
		"billing-api/bugfix/x.md":         {Service: "billing-api", Type: "bugfix", Depth: 3},
		"features/badge/qa/x.md":          {Group: "features", Service: "badge", Type: "qa", Depth: 4},
		"technologies/kafkajs/guide/x.md": {Group: "technologies", Service: "kafkajs", Type: "guide", Depth: 4},
		"index.md":                        {Depth: 1},
		"platform/index.md":               {Depth: 2},
		"features/index.md":               {Depth: 2},
		"features/badge/index.md":         {Service: "features", Type: "badge", Depth: 3}, // looks like service/type: harmless, it has no date
	} {
		if got := Locate(rel); got != want {
			t.Errorf("Locate(%q) = %+v, want %+v", rel, got, want)
		}
	}
	if (Place{Group: "features", Service: "badge"}).Label() != "features/badge" {
		t.Error("Label")
	}
}
