package devnotes

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseMarker(f *testing.F) {
	for _, s := range []string{"", "{}", `{"repo": "r", "range": "a..b", "shas": null, "grep": null, "author": null, "commits": []}`, `{"repo": 1}`, `{"commits": [{"sha": "\ud83d"}]}`, `[`, `"x"`, "\x00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if b, ok := parseMarker(s); ok {
			// what parses must write back and parse again to the same thing
			again, ok2 := parseMarker(marker(b.Spec, b.Commits))
			if !ok2 || again.Spec.Repo != b.Spec.Repo || len(again.Commits) != len(b.Commits) {
				t.Fatalf("marker does not round-trip for %q", s)
			}
		}
	})
}

func FuzzIDsAndPlaces(f *testing.F) {
	for _, s := range []string{"DOC-0007", "7", "doc-7", "", "DOC-", "A-1", "docs/a/b/c.md", "features/x/qa/2026-01-01-a.md", "../../x", "//", "a\x00b"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if id, ok := NormalizeID(s, "DOC"); ok {
			if _, _, ok := ParseID(id); !ok {
				t.Fatalf("NormalizeID(%q) gave %q, which does not parse", s, id)
			}
		}
		_ = Locate(s)
		_ = LintText("p.md", s, "DOC")
		_ = FrontMatterValue(s, "status")
		_ = Blocks(s)
	})
}

// Whatever front matter is thrown at the plan code, it must not panic, and a plan that parses and is saved must parse again with the same board.
func FuzzPlanRoundTrip(f *testing.F) {
	for _, s := range []string{
		"---\ntitle: x\nstages:\n  - name: A\n    steps:\n      - {step: One, state: done}\n---\nbody\n",
		"---\nstages: []\nwaiting: []\n---\n",
		"---\nstages:\n  - A\n  - {name: B, state: now}\n---\n",
		"---\nstages:\n  - name: \"a: b # c\"\n    steps: [\"x\", {step: \"y: z\", state: blocked}]\nwaiting:\n  - {what: \"q\\nr\", since: 2026-01-02}\nstatus: dropped\n---\n",
		"", "---\n", "---\n---\n", "no front matter", "---\nstages: 5\n---\n", "---\n\x00\n---\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		doc, updated, err := ParsePlan([]byte(text))
		if err != nil {
			return
		}
		before := doc.View(updated, today)
		_ = PlanProblems(doc)
		_ = before.Text("t")
		// the same plan, written into a note and read back, shows the same board
		path := filepath.Join(t.TempDir(), "plan.md")
		if err := os.WriteFile(path, []byte("---\ntitle: t\ndate: 2026-01-01\nupdated: 2026-01-01\n---\n\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := SavePlan(path, doc, today); err != nil {
			return
		}
		data, _ := os.ReadFile(path)
		back, _, err := ParsePlan(data)
		if err != nil {
			t.Fatalf("a saved plan must parse: %v\n%s", err, data)
		}
		after := back.View(updated, today)
		if len(after.Stages) != len(before.Stages) || after.Done != before.Done || after.Total != before.Total || len(after.Waiting) != len(before.Waiting) {
			t.Fatalf("the board changed in a save: %+v vs %+v\n%s", before, after, data)
		}
	})
}
