// Package server is the HTTP API and the embedded UI. It listens on loopback unless the user asks for LAN mode.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/library"
	"github.com/migara0/docent/internal/ui"
)

// Options configure a Server.
type Options struct {
	Version string
	LAN     bool   // listening beyond loopback: the folder picker and every change to the library list are disabled
	DropDir string // where dropped files are kept (a folder of its own is made inside it); empty uses the system's temporary folder
}

// Server serves the API under /_api and the UI everywhere else.
type Server struct {
	mgr   *library.Manager
	store *config.Store
	opt   Options
	start time.Time
	mux   *http.ServeMux
	drops dropper
	share sharer
}

func New(mgr *library.Manager, store *config.Store, opt Options) *Server {
	s := &Server{mgr: mgr, store: store, opt: opt, start: time.Now(), mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler returns the full handler with the security middleware applied.
func (s *Server) Handler() http.Handler { return s.secure(s.mux) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /_api/health", s.api(s.health))
	s.mux.HandleFunc("GET /_api/libraries", s.api(s.libraries))
	s.mux.HandleFunc("POST /_api/libraries", s.api(s.addLibrary))
	s.mux.HandleFunc("DELETE /_api/libraries/{id}", s.api(s.removeLibrary))
	s.mux.HandleFunc("POST /_api/open", s.api(s.open))
	s.mux.HandleFunc("POST /_api/drop", s.api(s.drop))
	s.mux.HandleFunc("GET /_api/share", s.api(s.shareGet))
	s.mux.HandleFunc("POST /_api/share", s.api(s.shareSet))
	s.mux.HandleFunc("GET /_api/share/qr", s.api(s.shareQR))
	s.mux.HandleFunc("GET /_api/tree", s.api(s.tree))
	s.mux.HandleFunc("GET /_api/page", s.api(s.page))
	s.mux.HandleFunc("GET /_api/raw", s.api(s.raw))
	s.mux.HandleFunc("GET /_api/overview", s.api(s.overview))
	s.mux.HandleFunc("GET /_api/recent", s.api(s.recent))
	s.mux.HandleFunc("GET /_api/created", s.api(s.created))
	s.mux.HandleFunc("GET /_api/tags", s.api(s.tags))
	s.mux.HandleFunc("GET /_api/ids", s.api(s.ids))
	s.mux.HandleFunc("GET /_api/latest", s.api(s.latest))
	s.mux.HandleFunc("GET /_api/plans", s.api(s.plans))
	s.mux.HandleFunc("POST /_api/plan", s.api(s.editPlan))
	s.mux.HandleFunc("GET /_id/{q}", s.gotoID)
	s.mux.HandleFunc("GET /_api/search-index", s.api(s.searchIndex))
	s.mux.HandleFunc("GET /_api/search", s.api(s.search))
	s.mux.HandleFunc("GET /_api/fs", s.api(s.browse))
	s.mux.HandleFunc("GET /_api/events", s.events)
	s.mux.HandleFunc("/_api/", s.api(func(w http.ResponseWriter, r *http.Request) error {
		return fail(http.StatusNotFound, "unknown API path %s", r.URL.Path)
	}))
	s.mux.Handle("/", ui.Handler())
}

// ───────── errors and JSON ─────────

type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string { return e.msg }

func fail(code int, format string, a ...any) error {
	return &httpError{code, fmt.Sprintf(format, a...)}
}

type apiFunc func(w http.ResponseWriter, r *http.Request) error

// api wraps a handler: errors become {"error": "..."} with the right status, and nothing is cached.
func (s *Server) api(fn apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if err := s.authorize(r); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		if err := fn(w, r); err != nil {
			var he *httpError
			code := http.StatusInternalServerError
			if errors.As(err, &he) {
				code = he.code
			} else {
				slog.Error("api", "path", r.URL.Path, "err", err)
			}
			writeJSON(w, code, map[string]string{"error": err.Error()})
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, v any) error { writeJSON(w, http.StatusOK, v); return nil }

func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	return readJSONLimit(w, r, v, 64<<10)
}

func readJSONLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fail(http.StatusBadRequest, "invalid JSON body: %v", err)
	}
	return nil
}

// ───────── security ─────────

// authorize is the single place where access control can be added later. Today every request that passed the checks in secure is allowed.
func (s *Server) authorize(r *http.Request) error { return nil }

func isLoopbackHost(h string) bool {
	host, _, err := net.SplitHostPort(h)
	if err != nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// secure applies the protections that matter for a local server:
//   - Host check (DNS rebinding): in the default loopback mode only localhost names are served.
//   - Origin / Sec-Fetch-Site check: a page on another site cannot make the browser change your libraries.
//   - JSON-only writes, no sniffing, no framing, no referrer, and a Content-Security-Policy.
func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.opt.LAN && !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
				http.Error(w, "cross-site request refused", http.StatusForbidden)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && !sameHost(origin, r.Host) {
				http.Error(w, "cross-origin request refused", http.StatusForbidden)
				return
			}
			if ct := r.Header.Get("Content-Type"); r.ContentLength != 0 && !strings.HasPrefix(ct, "application/json") {
				http.Error(w, "JSON required", http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func sameHost(origin, host string) bool {
	i := strings.Index(origin, "://")
	return i >= 0 && origin[i+3:] == host
}

// mutationsAllowed is false in LAN mode: someone else on the network must not be able to add folders or browse your disk.
func (s *Server) mutationsAllowed() error {
	if s.opt.LAN {
		return fail(http.StatusForbidden, "disabled in LAN mode")
	}
	return nil
}
