package server

import (
	"errors"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/migara0/docent/internal/devnotes"
)

// Plans: the status boards of the plan notes of the developer-notes libraries. Reading is open; ticking a step writes the note and is only allowed on this machine.

type planRow struct {
	Source  string             `json:"source"`
	Library string             `json:"library"`
	Path    string             `json:"path"`
	URL     string             `json:"url"`
	ID      string             `json:"id,omitempty"`
	Title   string             `json:"title"`
	Service string             `json:"service"`
	Board   *devnotes.PlanView `json:"board"`
}

// plans lists the plans that are not finished or dropped (all=1 lists every plan), the most recently touched first.
func (s *Server) plans(w http.ResponseWriter, r *http.Request) error {
	all := r.URL.Query().Get("all") == "1"
	rows := []planRow{}
	for _, src := range s.mgr.Sources() {
		if src.Lib.Profile != "devnotes" {
			continue
		}
		for _, pg := range src.Pages() {
			place := devnotes.Locate(pg.Path)
			if place.Type != "plan" || !place.Exact() {
				continue
			}
			data, _, err := src.Raw(pg.Path)
			if err != nil {
				continue
			}
			doc, updated, err := devnotes.ParsePlan(data)
			if err != nil {
				continue
			}
			v := doc.View(updated, time.Now())
			if !all && (v.Status == "done" || v.Status == "dropped") {
				continue
			}
			rows = append(rows, planRow{Source: src.ID, Library: src.Name, Path: pg.Path, URL: src.PageURL(pg.Path), ID: pg.Meta.ID, Title: pg.Title, Service: place.Label(), Board: v})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if a, b := rows[i].Board.Updated, rows[j].Board.Updated; a != b {
			return a > b
		}
		return rows[i].Path < rows[j].Path
	})
	return ok(w, map[string]any{"plans": rows})
}

type planEdit struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	Op     string `json:"op"`    // "state" or "resolve"
	Ref    string `json:"ref"`   // for "state": Stage/Step
	State  string `json:"state"` // for "state": todo, now, done, blocked or skipped
	Index  int    `json:"index"` // for "resolve": the waiting item, counting from 1
}

// editPlan changes one plan note: a step's state, or removes a waiting item that is no longer waited for. It returns the new board.
func (s *Server) editPlan(w http.ResponseWriter, r *http.Request) error {
	if err := s.mutationsAllowed(); err != nil {
		return err
	}
	var req planEdit
	if err := readJSON(w, r, &req); err != nil {
		return err
	}
	src, ok2 := s.mgr.Get(req.Source)
	if !ok2 || src.Lib.Profile != "devnotes" {
		return fail(http.StatusNotFound, "no such library")
	}
	place := devnotes.Locate(req.Path)
	if place.Type != "plan" || !place.Exact() {
		return fail(http.StatusBadRequest, "that page is not a plan")
	}
	_, abs, err := src.Raw(req.Path) // only indexed pages are served: the path cannot leave the library
	if err != nil {
		return fail(http.StatusNotFound, "no such page")
	}
	if !slices.Contains([]string{"state", "resolve"}, req.Op) {
		return fail(http.StatusBadRequest, "op must be state or resolve")
	}
	doc, err := devnotes.Edit(abs, time.Now(), func(p *devnotes.PlanDoc) error {
		if req.Op == "resolve" {
			if req.Index < 1 {
				return fail(http.StatusBadRequest, "index counts from 1")
			}
			_, err := p.RemoveWait(strconv.Itoa(req.Index))
			return err
		}
		_, err := p.SetState(req.Ref, req.State)
		return err
	})
	if err != nil {
		var he *httpError
		if errors.As(err, &he) {
			return err
		}
		return fail(http.StatusBadRequest, "%v", err)
	}
	return ok(w, doc.View(time.Now().Format("2006-01-02"), time.Now()))
}
