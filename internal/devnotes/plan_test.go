package devnotes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var today = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)

const planNote = `---
title: "Cursor paging"
id: DOC-0021
date: 2026-11-01
updated: 2026-11-05
author: "Someone"
# a comment that must survive
status: in-progress
tags:
  - billing-api
  - plan
stages:
  - name: Design
    steps:
      - {step: Contract, state: done}
      - Review
  - name: Develop
    steps:
      - {step: Schema, state: done}
      - {step: Cursor logic, state: now}
      - {step: Validation, state: todo}
  - {name: Release, state: todo}
waiting:
  - {what: Kafka topic created, who: DevOps, since: 2026-11-09, blocks: Release}
---

# Cursor paging

Body text with **markdown**.
`

func writePlan(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "plan", "2026-11-01-cursor.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlanView(t *testing.T) {
	doc, updated, err := ParsePlan([]byte(planNote))
	if err != nil {
		t.Fatal(err)
	}
	v := doc.View(updated, today)
	if v.Done != 2 || v.Total != 6 || v.Percent != 33 {
		t.Fatalf("progress = %d/%d (%d%%)", v.Done, v.Total, v.Percent)
	}
	if v.Now != "Cursor logic" || v.Next != "Review" {
		t.Fatalf("now %q next %q", v.Now, v.Next)
	}
	want := []string{"active", "active", "blocked"} // Design has a step still to do, Develop is being worked on, Release is held up by a wait
	for i, st := range v.Stages {
		if st.State != want[i] {
			t.Errorf("stage %s = %s, want %s", st.Name, st.State, want[i])
		}
	}
	if !v.Stages[0].Current || v.Stages[1].Current {
		t.Errorf("the first stage that is not done is the current one: %+v", v.Stages)
	}
	if v.Stages[2].Waiting != 1 || v.Waiting[0].Days != 3 || v.Days != 7 {
		t.Errorf("waiting %+v days %d", v.Waiting, v.Days)
	}
	if v.Stages[0].Steps[1].State != StepTodo {
		t.Errorf("a bare string step is to do: %+v", v.Stages[0].Steps[1])
	}
}

func TestPlanEditKeepsTheRestOfTheNote(t *testing.T) {
	p := writePlan(t, planNote)
	if _, err := Edit(p, today, func(d *PlanDoc) error {
		if _, err := d.SetState("Cursor logic", StepDone); err != nil {
			return err
		}
		_, err := d.SetState("validation", StepNow)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	text := string(got)
	for _, keep := range []string{"# a comment that must survive", "author: \"Someone\"", "tags:\n  - billing-api\n  - plan", "Body text with **markdown**.", "id: DOC-0021"} {
		if !strings.Contains(text, keep) {
			t.Errorf("lost %q:\n%s", keep, text)
		}
	}
	if !strings.Contains(text, "updated: 2026-11-12") {
		t.Errorf("updated was not set:\n%s", text)
	}
	doc, _, err := ParsePlan(got)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Stages[1].Steps[1].State != StepDone || doc.Stages[1].Steps[2].State != StepNow {
		t.Errorf("steps = %+v", doc.Stages[1].Steps)
	}
	if strings.Count(text, "stages:") != 1 || strings.Count(text, "waiting:") != 1 {
		t.Errorf("a block was duplicated:\n%s", text)
	}
	// a second edit changes nothing but what was asked
	before := text
	if _, err := Edit(p, today, func(d *PlanDoc) error { return nil }); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p)
	if string(after) != before {
		t.Errorf("an edit that changes nothing rewrote the note:\n%s\n---\n%s", before, after)
	}
}

func TestNowAndNextLinesDoNotOutliveAStepChange(t *testing.T) {
	p := writePlan(t, strings.Replace(planNote, "status: in-progress", "status: in-progress\nnow: Writing the logic\nnext: Then validation", 1))
	doc, _, _ := ParsePlan(mustRead(t, p))
	if v := doc.View("", today); v.Now != "Writing the logic" || v.Next != "Then validation" {
		t.Fatalf("overrides are shown: %q %q", v.Now, v.Next)
	}
	if _, err := Edit(p, today, func(d *PlanDoc) error { _, e := d.SetState("Cursor logic", StepDone); return e }); err != nil {
		t.Fatal(err)
	}
	text := string(mustRead(t, p))
	if strings.Contains(text, "now:") || strings.Contains(text, "next:") {
		t.Errorf("the hand-written lines are cleared when a step changes:\n%s", text)
	}
}

func TestPlanStatusFollowsTheWork(t *testing.T) {
	p := writePlan(t, strings.Replace(planNote, "status: in-progress", "status: planning", 1))
	doc, err := Edit(p, today, func(d *PlanDoc) error { _, e := d.SetState("Contract", StepDone); return e })
	if err != nil || doc.Status != "in-progress" {
		t.Fatalf("status = %q, %v (the first finished step starts the plan)", doc.Status, err)
	}
	doc, err = Edit(p, today, func(d *PlanDoc) error {
		for _, s := range []string{"Design", "Develop", "Release"} {
			if _, e := d.SetState(s, StepDone); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil || doc.Status != "done" {
		t.Fatalf("status = %q, %v (every step done finishes the plan)", doc.Status, err)
	}
	doc, _ = Edit(p, today, func(d *PlanDoc) error { _, e := d.SetState("Review", StepTodo); return e })
	if doc.Status != "in-progress" {
		t.Errorf("undoing a step reopens the plan: %q", doc.Status)
	}
	if _, err := Edit(p, today, func(d *PlanDoc) error { return d.SetStatus("done") }); err == nil {
		t.Error("done cannot be set by hand")
	}
	if doc, _ = Edit(p, today, func(d *PlanDoc) error { return d.SetStatus("dropped") }); doc.Status != "dropped" {
		t.Errorf("dropped sticks: %q", doc.Status)
	}
	if _, err := Edit(p, today, func(d *PlanDoc) error { _, e := d.SetState("Contract", StepDone); return e }); err != nil {
		t.Fatal(err)
	}
	if doc, _, _ := ParsePlan(mustRead(t, p)); doc.Status != "dropped" {
		t.Errorf("a dropped plan stays dropped: %q", doc.Status)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPlanOperations(t *testing.T) {
	doc, _, _ := ParsePlan([]byte(planNote))
	if _, err := doc.SetState("nothing like it", StepDone); err == nil {
		t.Error("an unknown step is an error")
	}
	if _, err := doc.SetState("e", StepDone); err == nil || !strings.Contains(err.Error(), "more than one") {
		t.Errorf("an ambiguous reference lists the candidates: %v", err)
	}
	if _, err := doc.SetState("Design/Review", "finished"); err == nil {
		t.Error("an unknown state is an error")
	}
	if err := doc.AddStage("Docs", "Develop"); err != nil || doc.Stages[2].Name != "Docs" {
		t.Errorf("add-stage after: %v %+v", err, doc.Stages)
	}
	if err := doc.AddStage("docs", ""); err == nil {
		t.Error("a stage that exists cannot be added again")
	}
	if err := doc.AddSteps("Docs", []string{"README", "Guide"}, ""); err != nil || len(doc.Stages[2].Steps) != 2 || doc.Stages[2].State != "" {
		t.Errorf("add-step: %v %+v", err, doc.Stages[2])
	}
	if err := doc.AddSteps("Develop", []string{"Errors"}, "Schema"); err != nil || doc.Stages[1].Steps[1].Step != "Errors" {
		t.Errorf("add-step after: %v %+v", err, doc.Stages[1].Steps)
	}
	if _, err := doc.Remove("Docs"); err != nil || len(doc.Stages) != 3 {
		t.Errorf("rm stage: %v", err)
	}
	if err := doc.AddWait(Wait{What: "QA access", Blocks: "release"}, today); err != nil || doc.Waiting[1].Blocks != "Release" || doc.Waiting[1].Since != "2026-11-12" {
		t.Errorf("wait: %v %+v", err, doc.Waiting)
	}
	if err := doc.AddWait(Wait{What: "x", Blocks: "Nowhere"}, today); err == nil {
		t.Error("a wait cannot hold up a stage that is not in the plan")
	}
	if w, err := doc.RemoveWait("qa"); err != nil || w.What != "QA access" {
		t.Errorf("unwait by text: %v %+v", err, w)
	}
	if _, err := doc.RemoveWait("7"); err == nil {
		t.Error("unwait by a number that does not exist")
	}
}

func TestStageNamesMayContainASlash(t *testing.T) {
	doc, _, _ := ParsePlan([]byte("---\ntitle: x\nstages:\n  - name: QA/UAT\n    steps:\n      - {step: Smoke, state: todo}\n  - {name: Ops, state: todo}\n---\n"))
	if what, err := doc.SetState("QA/UAT/Smoke", StepDone); err != nil || what != "QA/UAT/Smoke" {
		t.Errorf("stage with a slash: %q %v", what, err)
	}
	if _, err := doc.SetState("Ops/", StepDone); err != nil || doc.Stages[1].State != StepDone {
		t.Errorf("a stage without steps by its name and a slash: %v %+v", err, doc.Stages[1])
	}
}

func TestPlanProblems(t *testing.T) {
	bad := `---
title: x
status: finished
stages:
  - name: A
    steps:
      - {step: One, state: dunno}
      - {step: one, state: done}
  - name: a
waiting:
  - {what: "", since: yesterday}
  - {what: Thing, blocks: Z}
---
`
	doc, _, err := ParsePlan([]byte(bad))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(PlanProblems(doc), "\n")
	for _, want := range []string{"status \"finished\"", "listed twice", "state \"dunno\"", "no `what`", "not YYYY-MM-DD", "not in the plan"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	good, _, _ := ParsePlan([]byte(planNote))
	if p := PlanProblems(good); len(p) != 0 {
		t.Errorf("a good plan has no problems: %v", p)
	}
}

func TestPlanTemplateIsAValidPlan(t *testing.T) {
	sc := Scaffold{}
	tmpl, err := sc.templateFor("plan")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.NewReplacer("{{id}}", "DOC-0001", "{{title_q}}", `"T"`, "{{author_q}}", `"A"`, "{{title}}", "T", "{{service}}", "svc", "{{type}}", "plan", "{{date}}", "2026-11-12").Replace(tmpl)
	doc, _, err := ParsePlan([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if p := PlanProblems(doc); len(p) != 0 {
		t.Errorf("the template has problems: %v", p)
	}
	if v := doc.View("2026-11-12", today); v.Total < 5 || v.Status != "planning" || len(v.Stages) != 5 {
		t.Errorf("template board: %+v", v)
	}
}

func TestFindPlan(t *testing.T) {
	root := t.TempDir()
	s := Settings{Root: root, Prefix: "DOC"}
	dir := filepath.Join(root, "svc", "plan")
	os.MkdirAll(dir, 0o755)
	a := filepath.Join(dir, "2026-11-01-alpha.md")
	b := filepath.Join(dir, "2026-11-02-beta.md")
	os.WriteFile(a, []byte("---\nid: DOC-0007\ntitle: a\n---\n"), 0o644)
	os.WriteFile(b, []byte("---\nid: DOC-0008\ntitle: b\n---\n"), 0o644)
	for ref, want := range map[string]string{"DOC-0008": b, "doc-7": a, "alpha": a, "svc/plan/2026-11-02-beta.md": b} {
		if got, err := s.FindPlan(ref); err != nil || got != want {
			t.Errorf("FindPlan(%q) = %q, %v; want %q", ref, got, err, want)
		}
	}
	if _, err := s.FindPlan("2026"); err == nil {
		t.Error("an ambiguous fragment is an error")
	}
	if _, err := s.FindPlan("nope"); err == nil {
		t.Error("an unknown note is an error")
	}
}
