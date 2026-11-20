// Package ui embeds the browser app. `make build` copies web/dist into ui/dist; the placeholder page is only there so the Go code builds before the UI exists.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the embedded UI. Paths without a file extension fall back to index.html so the app's own routes (/<library>/<page>) work on reload.
func Handler() http.Handler {
	sub, _ := fs.Sub(dist, "dist")
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" && path.Ext(p) == "" {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			files.ServeHTTP(w, r2)
			return
		}
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // build output is content-hashed
		}
		files.ServeHTTP(w, r)
	})
}
