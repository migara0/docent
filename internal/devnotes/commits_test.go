package devnotes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commitRepo(t *testing.T) (Commits, string, string) {
	t.Helper()
	ws := t.TempDir()
	repo := filepath.Join(ws, "svc")
	git(t, ws, "", "init", "-q", "-b", "develop", "svc")
	var shas []string
	for i, msg := range []string{"base", "feat: first | with a pipe", "feat: second -- dashes é 🚀"} {
		_ = os.WriteFile(filepath.Join(repo, "f.txt"), []byte(strings.Repeat("x", i+1)), 0o644)
		git(t, repo, "2026-10-0"+string(rune('1'+i))+"T12:00:00", "add", "-A")
		git(t, repo, "2026-10-0"+string(rune('1'+i))+"T12:00:00", "commit", "-qm", msg)
		if i == 0 {
			git(t, repo, "", "checkout", "-qb", "feat/x")
		}
		shas = append(shas, gitOut(t, repo, "rev-parse", "HEAD"))
	}
	docs := filepath.Join(ws, "notes", "docs")
	_ = os.MkdirAll(filepath.Join(docs, "svc", "commits"), 0o755)
	note := filepath.Join(docs, "svc", "commits", "2026-10-06-feat-x.md")
	_ = os.WriteFile(note, []byte("---\ntitle: \"Feat X\"\nid: DOC-0001\ndate: 2026-10-06\nstatus: in-progress\nrelease: \"\"\ntags:\n  - svc\n---\n\n# Feat X\n\n## Cherry-pick notes\n\nHand-written, safe from refreshes.\n\n## Commits\n"), 0o644)
	return Commits{S: Settings{Prefix: "DOC", Workspace: ws, Root: docs}}, note, strings.Join(shas, ",")
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec_git(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(out)
}

func TestRecordRefreshAndPlan(t *testing.T) {
	c, note, shaList := commitRepo(t)
	shas := strings.Split(shaList, ",")

	n, err := c.Upsert(note, CommitSpec{Repo: "svc", Range: "develop..feat/x"})
	if err != nil || n != 2 {
		t.Fatalf("recorded %d: %v", n, err)
	}
	data, _ := os.ReadFile(note)
	text := string(data)
	for _, want := range []string{"### svc", "**Source:** `develop..feat/x` · **Commits:** 2", "| 1 | `" + shas[1][:10] + "` | 2026-10-02 |", `feat: first \| with a pipe`,
		"git -C svc cherry-pick -x \\", "  " + shas[1] + " \\", "  " + shas[2] + "\n```", "Hand-written, safe from refreshes."} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	blocks := Blocks(text)
	if len(blocks) != 1 || len(blocks[0].Commits) != 2 || blocks[0].Commits[1].Subject != "feat: second -- dashes é 🚀" || blocks[0].Spec.Range != "develop..feat/x" {
		t.Fatalf("the block must read back exactly as written: %+v", blocks)
	}
	if strings.Contains(strings.SplitN(text, "<!-- commits:begin ", 2)[1][:400], "--\"") && strings.Contains(strings.SplitN(text, "-->", 2)[0], "dashes --") {
		t.Error("`--` inside the marker must be escaped so it cannot close the HTML comment")
	}

	// a new commit, then refresh: the block is replaced in place, the hand-written part is untouched
	_ = os.WriteFile(filepath.Join(c.S.Workspace, "svc", "f.txt"), []byte("xxxx"), 0o644)
	git(t, filepath.Join(c.S.Workspace, "svc"), "2026-10-04T12:00:00", "add", "-A")
	git(t, filepath.Join(c.S.Workspace, "svc"), "2026-10-04T12:00:00", "commit", "-qm", "feat: third")
	done, err := c.Refresh(note, "")
	if err != nil || len(done) != 1 || done[0] != "svc: 3 commit(s)" {
		t.Fatalf("refresh: %v %v", done, err)
	}
	data, _ = os.ReadFile(note)
	if strings.Count(string(data), "<!-- commits:begin") != 1 || !strings.Contains(string(data), "Hand-written, safe from refreshes.") || !strings.Contains(string(data), "**Commits:** 3") {
		t.Errorf("refresh must update the one block in place:\n%s", data)
	}

	// a second repo gets its own block; a filter narrows the commits
	if _, err := c.Upsert(note, CommitSpec{Repo: "svc", Range: "develop..feat/x", Grep: "SECOND"}); err != nil {
		t.Fatal(err)
	}
	b := Blocks(readFile(t, note))
	if len(b) != 1 || len(b[0].Commits) != 1 || b[0].Spec.Grep != "SECOND" {
		t.Errorf("grep is case-insensitive and replaces the repo's block: %+v", b)
	}
	if _, err := c.Upsert(note, CommitSpec{Repo: "svc", Shas: []string{shas[2]}}); err != nil {
		t.Fatal(err)
	}
	if b = Blocks(readFile(t, note)); len(b[0].Spec.Shas) != 1 || !strings.Contains(readFile(t, note), "explicit SHAs") {
		t.Errorf("explicit SHAs: %+v", b)
	}
	if _, err := c.Upsert(note, CommitSpec{Repo: "nope", Range: "a..b"}); err == nil || !strings.Contains(err.Error(), "not a git repo") {
		t.Errorf("an unknown repo is an error: %v", err)
	}
	if _, err := c.Upsert(note, CommitSpec{Repo: "svc", Range: "develop..no-such-branch"}); err == nil {
		t.Error("a bad range is an error")
	}

	// plan: two notes, the same commit once
	other := filepath.Join(filepath.Dir(note), "2026-10-07-other.md")
	_ = os.WriteFile(other, []byte("---\ntitle: o\nstatus: in-progress\n---\n\n## Commits\n"), 0o644)
	_, _ = c.Upsert(other, CommitSpec{Repo: "svc", Range: "develop..feat/x"})
	_, _ = c.Upsert(note, CommitSpec{Repo: "svc", Range: "develop..feat/x"})
	plan, warns, err := c.Plan([]string{"feat-x", "other"})
	if err != nil || len(warns) != 0 {
		t.Fatalf("plan: %v %v", warns, err)
	}
	if !strings.Contains(plan, "3 commit(s) across 1 repo(s)") || strings.Count(plan, shas[1]) != 1 {
		t.Errorf("a commit appears once however many notes record it:\n%s", plan)
	}

	// released notes are frozen
	data, _ = os.ReadFile(note)
	_ = os.WriteFile(note, []byte(strings.Replace(string(data), "status: in-progress", "status: released", 1)), 0o644)
	if _, err := c.Refresh(note, ""); err == nil || !strings.Contains(err.Error(), "released") {
		t.Errorf("a released note is not refreshed: %v", err)
	}
	if _, warns, _ := c.Plan([]string{"feat-x"}); len(warns) != 1 {
		t.Errorf("planning from a released note warns: %v", warns)
	}
	if line := c.ListLine(note); !strings.HasPrefix(line, "released") || !strings.Contains(line, "svc:3") {
		t.Errorf("list line: %q", line)
	}
	if _, err := c.Find("zzz"); err == nil {
		t.Error("an unknown note")
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMarkerMatchesPythonJSONStyle(t *testing.T) {
	spec := CommitSpec{Repo: "billing-api", Range: "develop..feat/x"}
	commits := []CommitInfo{{"abc", "2026-10-01", "Jö", `say "hi" -- ok \ done 🚀`}}
	got := marker(spec, commits)
	want := `{"repo": "billing-api", "range": "develop..feat/x", "shas": null, "grep": null, "author": null, "commits": [{"sha": "abc", "date": "2026-10-01", "author": "J\u00f6", "subject": "say \"hi\" \u002d\u002d ok \\ done \ud83d\ude80"}]}`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	b, ok := parseMarker(got)
	if !ok || b.Commits[0] != commits[0] || b.Spec.Range != spec.Range {
		t.Errorf("round trip: %+v", b)
	}
}

// With DOCENT_CORPUS set, every commits block written by the Python tool must re-serialise byte for byte.
func TestCorpusMarkersRoundTrip(t *testing.T) {
	root := os.Getenv("DOCENT_CORPUS")
	if root == "" {
		t.Skip("DOCENT_CORPUS not set")
	}
	n := 0
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if n := d.Name(); n == "node_modules" || n == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		data, _ := os.ReadFile(p)
		text := string(data)
		if !strings.Contains(text, "<!-- commits:begin ") {
			return nil
		}
		for _, line := range strings.Split(text, "\n") {
			if !strings.HasPrefix(line, "<!-- commits:begin ") {
				continue
			}
			orig := strings.TrimSuffix(strings.TrimPrefix(line, "<!-- commits:begin "), " -->")
			b, ok := parseMarker(orig)
			if !ok {
				t.Errorf("%s: cannot parse marker", p)
				continue
			}
			if got := marker(b.Spec, b.Commits); got != orig {
				t.Errorf("%s: marker differs\n got: %.200s\nwant: %.200s", p, got, orig)
			}
			n++
		}
		return nil
	})
	t.Logf("%d real markers round-trip", n)
}
