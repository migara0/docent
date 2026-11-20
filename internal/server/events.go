package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// events streams changes to the browser (server-sent events): "page", "tree" and "source".
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	ch, cancel := s.mgr.Hub().Subscribe()
	defer cancel()
	fmt.Fprint(w, "retry: 2000\n\n")
	fl.Flush()
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
			fl.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			fl.Flush()
		}
	}
}
