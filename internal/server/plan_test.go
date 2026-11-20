package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testPlan = `---
title: "Cursor paging"
id: DOC-0021
date: 2026-11-01
updated: 2026-11-05
status: in-progress
tags:
  - billing-api
  - plan
stages:
  - name: Develop
    steps:
      - {step: Schema, state: done}
      - {step: Cursor logic, state: now}
  - {name: Release, state: todo}
waiting:
  - {what: Kafka topic created, who: DevOps, since: 2026-11-09}
---

# Cursor paging

Body.
`

func planLibrary(t *testing.T, e *env) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "billing-api", "plan")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-11-01-cursor-paging.md"), []byte(testPlan), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "billing-api", "adr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "billing-api", "adr", "2026-11-01-other.md"), []byte("---\ntitle: Other\nid: DOC-0022\ndate: 2026-11-01\ntags:\n  - x\n---\n\n# Other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, body := e.do(t, "POST", "/_api/libraries", `{"path":`+jsonStr(root)+`,"name":"doc-notes","profile":"devnotes"}`, nil); code != 200 {
		t.Fatalf("add: %d %s", code, body)
	}
	time.Sleep(200 * time.Millisecond)
	return filepath.Join(dir, "2026-11-01-cursor-paging.md")
}

func TestPlanBoardInPageAndList(t *testing.T) {
	e := newEnv(t, false)
	planLibrary(t, e)

	code, body := e.do(t, "GET", "/_api/page?src=doc-notes&path=billing-api/plan/2026-11-01-cursor-paging.md", "", nil)
	if code != 200 {
		t.Fatalf("page: %d %s", code, body)
	}
	var page struct {
		Plan struct {
			Status      string
			Done, Total int
			Now, Next   string
			Stages      []struct{ Name, State string }
			Waiting     []struct{ What string }
		}
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatal(err)
	}
	if p := page.Plan; p.Status != "in-progress" || p.Done != 1 || p.Total != 3 || p.Now != "Cursor logic" || p.Next != "Release" || len(p.Stages) != 2 || p.Stages[0].State != "active" || len(p.Waiting) != 1 {
		t.Errorf("board: %+v", page.Plan)
	}
	_, body = e.do(t, "GET", "/_api/page?src=doc-notes&path=billing-api/adr/2026-11-01-other.md", "", nil)
	if strings.Contains(string(body), `"plan"`) {
		t.Errorf("only plan notes have a board: %s", body)
	}

	_, body = e.do(t, "GET", "/_api/plans", "", nil)
	var list struct {
		Plans []struct {
			ID, Title, Service, URL, Source string
			Board                           struct{ Percent int }
		}
	}
	if err := json.Unmarshal(body, &list); err != nil || len(list.Plans) != 1 {
		t.Fatalf("plans: %s", body)
	}
	if p := list.Plans[0]; p.ID != "DOC-0021" || p.Title != "Cursor paging" || p.Service != "billing-api" || p.Board.Percent != 33 || p.Source != "doc-notes" || p.URL == "" {
		t.Errorf("row: %+v", p)
	}
}

func TestTickingAStepWritesTheNote(t *testing.T) {
	e := newEnv(t, false)
	file := planLibrary(t, e)
	post := func(body string) (int, []byte) { return e.do(t, "POST", "/_api/plan", body, nil) }
	const path = `"path":"billing-api/plan/2026-11-01-cursor-paging.md"`

	code, body := post(`{"source":"doc-notes",` + path + `,"op":"state","ref":"Develop/Cursor logic","state":"done"}`)
	if code != 200 {
		t.Fatalf("tick: %d %s", code, body)
	}
	var board struct{ Done, Total int }
	_ = json.Unmarshal(body, &board)
	if board.Done != 2 || board.Total != 3 {
		t.Errorf("the response is the new board: %s", body)
	}
	got, _ := os.ReadFile(file)
	if !strings.Contains(string(got), "{step: Cursor logic, state: done}") || !strings.Contains(string(got), "# Cursor paging\n\nBody.") {
		t.Errorf("note on disk:\n%s", got)
	}

	if code, body = post(`{"source":"doc-notes",` + path + `,"op":"resolve","index":1}`); code != 200 || strings.Contains(string(mustReadFile(t, file)), "Kafka topic") {
		t.Errorf("resolve: %d %s", code, body)
	}
	for name, req := range map[string]string{
		"unknown step":    `{"source":"doc-notes",` + path + `,"op":"state","ref":"Nothing","state":"done"}`,
		"unknown state":   `{"source":"doc-notes",` + path + `,"op":"state","ref":"Schema","state":"finished"}`,
		"unknown op":      `{"source":"doc-notes",` + path + `,"op":"delete"}`,
		"not a plan":      `{"source":"doc-notes","path":"billing-api/adr/2026-11-01-other.md","op":"state","ref":"x","state":"done"}`,
		"outside library": `{"source":"doc-notes","path":"../../etc/plan/passwd.md","op":"state","ref":"x","state":"done"}`,
		"unknown library": `{"source":"nope",` + path + `,"op":"state","ref":"Schema","state":"done"}`,
		"waiting 0":       `{"source":"doc-notes",` + path + `,"op":"resolve","index":0}`,
	} {
		if code, _ := post(req); code < 400 || code >= 500 {
			t.Errorf("%s: status %d, want a 4xx", name, code)
		}
	}
	// a cross-site page cannot tick steps
	if code, _ := e.do(t, "POST", "/_api/plan", `{"source":"doc-notes",`+path+`,"op":"state","ref":"Schema","state":"todo"}`, map[string]string{"Origin": "http://evil.example"}); code < 400 {
		t.Errorf("a request from another site: %d", code)
	}
}

func TestTickingIsOffInLANMode(t *testing.T) {
	e := newEnv(t, true)
	if code, _ := e.do(t, "POST", "/_api/plan", `{"source":"x","path":"a/plan/b.md","op":"state","ref":"a","state":"done"}`, nil); code != 403 {
		t.Errorf("LAN mode must not write notes: %d", code)
	}
}

func mustReadFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLatestListsTheNewestNotesPerLibrary(t *testing.T) {
	e := newEnv(t, false)
	planLibrary(t, e) // DOC-0021 (plan) and DOC-0022 (adr)
	code, body := e.do(t, "GET", "/_api/latest?limit=1", "", nil)
	if code != 200 {
		t.Fatalf("latest: %d %s", code, body)
	}
	var out struct {
		Sources []struct {
			Source, Prefix string
			MaxID          int
			Notes          []struct {
				ID, Title, URL, Type, Service string
				Num                           int
			}
		}
	}
	if err := json.Unmarshal(body, &out); err != nil || len(out.Sources) != 1 {
		t.Fatalf("latest: %s", body)
	}
	s := out.Sources[0]
	if s.Source != "doc-notes" || s.Prefix != "DOC" || s.MaxID != 22 || len(s.Notes) != 1 || s.Notes[0].ID != "DOC-0022" || s.Notes[0].Type != "adr" || s.Notes[0].Service != "billing-api" || s.Notes[0].URL == "" {
		t.Errorf("source: %+v", s)
	}
	_, body = e.do(t, "GET", "/_api/latest", "", nil)
	if !strings.Contains(string(body), `"DOC-0021"`) {
		t.Errorf("without a limit both notes are listed: %s", body)
	}
	// plain libraries have no note IDs and are left out; the answer is never null
	e2 := newEnv(t, false)
	if code, body := e2.do(t, "GET", "/_api/latest", "", nil); code != 200 || !strings.Contains(string(body), `"sources":[]`) {
		t.Errorf("no libraries: %d %s", code, body)
	}
}
