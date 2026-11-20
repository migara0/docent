package server

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/migara0/docent/internal/devnotes"
)

// The newest notes of every library that has note IDs, for the "what's new" views. Note IDs only ever go up (they are never reused), so a higher ID is a newer note,
// which is what lets the browser tell which notes you have not seen yet without the server keeping any record of you.

type latestNote struct {
	ID      string `json:"id"`
	Num     int    `json:"num"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Path    string `json:"path"`
	Type    string `json:"type,omitempty"`
	Service string `json:"service,omitempty"`
	Created string `json:"created,omitempty"`
}

type latestSource struct {
	Source string       `json:"source"`
	Name   string       `json:"name"`
	Prefix string       `json:"prefix"`
	MaxID  int          `json:"maxId"`
	Notes  []latestNote `json:"notes"`
}

// latest lists, per library with note IDs, the highest-numbered notes (limit each, default 40).
func (s *Server) latest(w http.ResponseWriter, r *http.Request) error {
	limit := 40
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 200 {
		limit = n
	}
	out := []latestSource{}
	for _, src := range s.mgr.Sources() {
		if src.Lib.Profile != "devnotes" {
			continue
		}
		info := src.Info()
		if info.IDPrefix == "" || info.Indexing {
			continue
		}
		ls := latestSource{Source: src.ID, Name: src.Name, Prefix: info.IDPrefix, Notes: []latestNote{}}
		for _, pg := range src.Pages() {
			prefix, num, ok := devnotes.ParseID(pg.Meta.ID)
			if !ok || prefix != info.IDPrefix || pg.Meta.Date == "" {
				continue
			}
			if num > ls.MaxID {
				ls.MaxID = num
			}
			place := devnotes.Locate(pg.Path)
			n := latestNote{ID: pg.Meta.ID, Num: num, Title: pg.Title, URL: src.PageURL(pg.Path), Path: pg.Path, Created: pg.Meta.Date}
			if place.IsNotePlace() {
				n.Type, n.Service = place.Type, place.Label()
			}
			ls.Notes = append(ls.Notes, n)
		}
		sort.Slice(ls.Notes, func(i, j int) bool {
			if ls.Notes[i].Num != ls.Notes[j].Num {
				return ls.Notes[i].Num > ls.Notes[j].Num
			}
			return ls.Notes[i].Path < ls.Notes[j].Path
		})
		if len(ls.Notes) > limit {
			ls.Notes = ls.Notes[:limit]
		}
		out = append(out, ls)
	}
	return ok(w, map[string]any{"sources": out})
}
