package devnotes

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Notes saved by Windows editors end their lines with \r\n. They must read like any other note, and a command that rewrites one
// must keep its line endings (not leave a mix of \r\n and \n behind).

func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

// onlyCRLF reports whether every line break of b is \r\n.
func onlyCRLF(b []byte) bool {
	return bytes.Count(b, []byte("\n")) == bytes.Count(b, []byte("\r\n"))
}

func TestWindowsLineEndingsInAPlan(t *testing.T) {
	doc, updated, err := ParsePlan([]byte(crlf(planNote)))
	if err != nil || updated != "2026-11-05" || len(doc.Stages) != 3 || doc.Stages[1].Steps[1].State != StepNow {
		t.Fatalf("a plan with \\r\\n reads like any other: %v %q %+v", err, updated, doc)
	}

	p := writePlan(t, crlf(planNote))
	if _, err := Edit(p, today, func(d *PlanDoc) error { _, e := d.SetState("Cursor logic", StepDone); return e }); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, p)
	if !onlyCRLF(got) {
		t.Errorf("the note now mixes line endings:\n%q", got)
	}
	doc, _, err = ParsePlan(got)
	if err != nil || doc.Stages[1].Steps[1].State != StepDone {
		t.Errorf("the edit was not saved: %v %+v", err, doc)
	}
	if !bytes.Contains(got, []byte("# a comment that must survive\r\n")) || !bytes.Contains(got, []byte("Body text with **markdown**.\r\n")) {
		t.Errorf("the rest of the note changed:\n%q", got)
	}
}

func TestWindowsLineEndingsWhenAnIDIsAdded(t *testing.T) {
	p := writePlan(t, crlf("---\ntitle: \"A note\"\ndate: 2026-11-01\n---\n\n# A note\n"))
	if err := InsertID(p, "DOC-0042"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != crlf("---\ntitle: \"A note\"\nid: DOC-0042\ndate: 2026-11-01\n---\n\n# A note\n") {
		t.Errorf("the ID goes after the title, with the note's own line endings:\n%q", got)
	}
}

func TestReadNote(t *testing.T) {
	p := writePlan(t, "---\r\nid: DOC-0001\r\n---\r\n\r\n# T\r\n")
	if got, err := ReadNote(p); err != nil || string(got) != "---\nid: DOC-0001\n---\n\n# T\n" {
		t.Errorf("ReadNote = %q, %v", got, err)
	}
	unix := writePlan(t, "# Plain\n")
	if got, _ := ReadNote(unix); string(got) != "# Plain\n" {
		t.Errorf("a \\n file is returned as it is: %q", got)
	}
}
