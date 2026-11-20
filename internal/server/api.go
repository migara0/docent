package server

import (
	"context"
	"errors"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/devnotes"
	"github.com/migara0/docent/internal/library"
)

func (s *Server) health(w http.ResponseWriter, r *http.Request) error {
	return ok(w, map[string]any{"ok": true, "version": s.opt.Version, "uptimeSeconds": int(time.Since(s.start).Seconds()), "lan": s.opt.LAN})
}

func (s *Server) libraries(w http.ResponseWriter, r *http.Request) error {
	cfg, err := s.store.Load()
	if err != nil {
		return err
	}
	recents := cfg.Recents
	if recents == nil {
		recents = []config.Recent{}
	}
	return ok(w, map[string]any{"libraries": s.mgr.List(), "recents": recents, "readOnly": s.opt.LAN})
}

type addRequest struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Profile   string `json:"profile"`
	Prefix    string `json:"prefix"`
	Workspace string `json:"workspace"`
	Templates string `json:"templates"`
}

func (s *Server) addLibrary(w http.ResponseWriter, r *http.Request) error {
	if err := s.mutationsAllowed(); err != nil {
		return err
	}
	var req addRequest
	if err := readJSON(w, r, &req); err != nil {
		return err
	}
	if req.Profile != "" && req.Profile != "plain" && req.Profile != "devnotes" {
		return fail(http.StatusBadRequest, "unknown profile %q (use plain or devnotes)", req.Profile)
	}
	src, err := s.mgr.Register(req.Path, req.Name, req.Profile, config.Settings{Prefix: req.Prefix, Workspace: req.Workspace, Templates: req.Templates})
	if err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	return ok(w, src.Info())
}

func (s *Server) removeLibrary(w http.ResponseWriter, r *http.Request) error {
	if err := s.mutationsAllowed(); err != nil {
		return err
	}
	if err := s.mgr.Remove(r.PathValue("id")); err != nil {
		return fail(http.StatusNotFound, "%v", err)
	}
	return ok(w, map[string]bool{"ok": true})
}

func (s *Server) open(w http.ResponseWriter, r *http.Request) error {
	if err := s.mutationsAllowed(); err != nil {
		return err
	}
	var req addRequest
	if err := readJSON(w, r, &req); err != nil {
		return err
	}
	src, err := s.mgr.Open(req.Path)
	if err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	return ok(w, src.Info())
}

// source finds the source named by ?src= and waits (briefly) for its first scan.
func (s *Server) source(r *http.Request) (*library.Source, error) {
	id := r.URL.Query().Get("src")
	src, found := s.mgr.Get(id)
	if !found {
		return nil, fail(http.StatusNotFound, "no open library %q", id)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := src.WaitReady(ctx); err != nil {
		return nil, fail(http.StatusServiceUnavailable, "%q is still being indexed", id)
	}
	return src, nil
}

func (s *Server) tree(w http.ResponseWriter, r *http.Request) error {
	src, err := s.source(r)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"info": src.Info(), "tree": src.Tree()})
}

func (s *Server) page(w http.ResponseWriter, r *http.Request) error {
	src, err := s.source(r)
	if err != nil {
		return err
	}
	v, err := src.RenderPage(r.URL.Query().Get("path"))
	if errors.Is(err, library.ErrNoPage) {
		return fail(http.StatusNotFound, "no such page")
	}
	if err != nil {
		return err
	}
	cp := *v
	if !s.opt.LAN { // an absolute path is only useful (and only shared) when the browser runs on this machine
		_, cp.File, _ = src.Raw(v.Path)
	}
	if src.Lib.Profile == "devnotes" {
		if p := devnotes.Locate(v.Path); p.IsNotePlace() {
			cp.Place = &library.Place{Group: p.Group, Service: p.Service, Type: p.Type}
		}
	}
	return ok(w, &cp)
}

// noteRow is a page row with the developer-notes type, for overview tables.
type noteRow struct {
	library.Row
	Type    string `json:"type,omitempty"`
	Service string `json:"service,omitempty"`
}

func (s *Server) rows(src *library.Source, rows []library.Row) []noteRow {
	out := make([]noteRow, len(rows))
	dev := src.Lib.Profile == "devnotes"
	for i, r := range rows {
		out[i] = noteRow{Row: r}
		if dev {
			p := devnotes.Locate(r.Path)
			out[i].Type, out[i].Service = p.Type, p.Label()
		}
	}
	return out
}

// overview lists the pages under a folder with the facts an overview table shows.
func (s *Server) overview(w http.ResponseWriter, r *http.Request) error {
	src, err := s.source(r)
	if err != nil {
		return err
	}
	folder := strings.Trim(r.URL.Query().Get("folder"), "/")
	rows := src.Rows(folder, src.Lib.Profile == "devnotes")
	return ok(w, map[string]any{"folder": folder, "profile": src.Lib.Profile, "notes": s.rows(src, rows)})
}

// recent lists the most recently updated pages.
func (s *Server) recent(w http.ResponseWriter, r *http.Request) error {
	src, err := s.source(r)
	if err != nil {
		return err
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return ok(w, map[string]any{"notes": s.rows(src, src.Recent(limit, src.Lib.Profile == "devnotes"))})
}

// created lists every page with the day (and, when known, the moment) it was created, for the search filter "created on".
// The browser picks the day in its own time zone, so it filters the list itself.
func (s *Server) created(w http.ResponseWriter, r *http.Request) error {
	src, err := s.source(r)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"pages": s.rows(src, src.CreatedRows())})
}

// tags lists every tag with its pages.
func (s *Server) tags(w http.ResponseWriter, r *http.Request) error {
	src, err := s.source(r)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"tags": src.Tags()})
}

// raw returns a page's Markdown source as text, for "copy as Markdown" and "download".
func (s *Server) raw(w http.ResponseWriter, r *http.Request) error {
	src, err := s.source(r)
	if err != nil {
		return err
	}
	data, _, err := src.Raw(r.URL.Query().Get("path"))
	if errors.Is(err, library.ErrNoPage) {
		return fail(http.StatusNotFound, "no such page")
	}
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, err = w.Write(data)
	return err
}

func (s *Server) searchIndex(w http.ResponseWriter, r *http.Request) error {
	src, err := s.source(r)
	if err != nil {
		return err
	}
	raw, err := src.SearchIndex()
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(raw)
	return err
}

// search runs a server-side search (used for large libraries; small ones search in the browser). The request's context is cancelled when the user types another letter.
func (s *Server) search(w http.ResponseWriter, r *http.Request) error {
	src, err := s.source(r)
	if err != nil {
		return err
	}
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	start := time.Now()
	hits, err := src.Search(r.Context(), q, limit)
	if err != nil {
		if r.Context().Err() != nil {
			return nil // the client went away
		}
		return err
	}
	return ok(w, map[string]any{"query": q, "hits": hits, "tookMs": time.Since(start).Milliseconds()})
}

// ids finds notes by (part of) an ID, for the palette and for /_id/<ID>.
func (s *Server) ids(w http.ResponseWriter, r *http.Request) error {
	return ok(w, map[string]any{"matches": s.mgr.FindIDs(r.URL.Query().Get("q"), r.URL.Query().Get("src"), 0)})
}

// gotoID redirects /_id/DOC-0015 (or /_id/15) to the note, so an ID can be bookmarked, typed into the address bar or opened from a terminal.
func (s *Server) gotoID(w http.ResponseWriter, r *http.Request) {
	matches := s.mgr.FindIDs(r.PathValue("q"), "", 1)
	if len(matches) == 0 || !matches[0].Exact {
		http.Error(w, "No note has the ID "+html.EscapeString(r.PathValue("q")), http.StatusNotFound)
		return
	}
	http.Redirect(w, r, matches[0].URL, http.StatusFound)
}
