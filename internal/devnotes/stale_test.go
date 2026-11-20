package devnotes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/migara0/docent/internal/library"
)

// needGit skips a test that runs git when it is not installed (for example the Windows tests run under Wine).
func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

func git(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	needGit(t)
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=T", "-c", "user.email=t@t"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestStaleDetection(t *testing.T) {
	ws := t.TempDir()
	repo := filepath.Join(ws, "svc")
	_ = os.MkdirAll(filepath.Join(repo, "src", "claims"), 0o755)
	git(t, ws, "", "init", "-q", "svc")
	write := func(rel, content string) {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(repo, rel)), 0o755)
		_ = os.WriteFile(filepath.Join(repo, rel), []byte(content), 0o644)
	}
	write("src/claims/a.ts", "1")
	write("src/old.ts", "1")
	git(t, repo, "2026-10-01T12:00:00", "add", "-A")
	git(t, repo, "2026-10-01T12:00:00", "commit", "-qm", "first")
	write("src/claims/a.ts", "2")
	git(t, repo, "2026-11-04T12:00:00", "commit", "-qam", "change claims")

	docs := filepath.Join(ws, "notes_developer_notes", "docs")
	_ = os.MkdirAll(filepath.Join(docs, "svc", "module"), 0o755)
	rel := "svc/module/2026-10-02-claims.md"
	body := "---\ntitle: Claims\nid: DOC-0001\ndate: 2026-10-02\nupdated: 2026-10-02\n---\n\n# Claims\n\n## Where it lives\n\n" +
		"| What | Where |\n| ---- | ----- |\n| Claims | `svc/src/claims/` (`a.ts`) |\n| Old | `svc/src/old.ts` |\n| Missing | `svc/src/gone.ts` |\n| URL | `https://x.y/z` |\n\n## Status\n"
	_ = os.WriteFile(filepath.Join(docs, filepath.FromSlash(rel)), []byte(body), 0o644)

	s := Settings{Prefix: "DOC", Workspace: ws, Root: docs}
	pg := &library.Page{Path: rel, Title: "Claims", Meta: library.Meta{ID: "DOC-0001", Date: "2026-10-02", Updated: "2026-10-02"}}
	r := CheckModule(s, rel, pg)
	if r.Checked != 2 { // svc/src/claims/ and svc/src/old.ts; a bare "a.ts" has no slash, the missing file and the URL do not exist as paths
		t.Errorf("paths checked: %d", r.Checked)
	}
	if len(r.Changes) != 1 || r.Changes[0].Changed != "2026-11-04" || r.Changes[0].Path != "svc/src/claims" { // old.ts last changed before the note was updated
		t.Errorf("changes after the note was updated: %+v", r.Changes)
	}
	// a path that does not resolve from the workspace is tried inside the first path on the same line
	viaBase := pathsIn(ws, "## Where it lives\n\n| x | `svc/src/` (`claims/a.ts`) |\n")
	if len(viaBase) != 2 || !strings.HasSuffix(viaBase[1], filepath.Join("svc", "src", "claims", "a.ts")) {
		t.Errorf("resolution relative to the first path of the line: %v", viaBase)
	}
	notice := NewStaleChecker().Check(s, rel, pg)
	if notice == nil || notice.Kind != "warning" || len(notice.Items) != 1 || !strings.Contains(notice.Items[0], "last changed 2026-11-04") {
		t.Errorf("notice: %+v", notice)
	}

	// once the note is updated after the change, it is no longer stale
	pg.Meta.Updated = "2026-11-05"
	if r := CheckModule(s, rel, pg); len(r.Changes) != 0 {
		t.Errorf("an updated note is not stale: %+v", r.Changes)
	}
	if NewStaleChecker().Check(s, rel, pg) != nil {
		t.Error("no notice for an up-to-date note")
	}
	// a path outside any git repo is skipped, not an error
	write2 := filepath.Join(ws, "plain")
	_ = os.MkdirAll(write2, 0o755)
	if lastChange(write2) != "" {
		t.Error("not in a repo: no date")
	}
}

func exec_git(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return string(out), err
}
