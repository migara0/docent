package library

import (
	"os"
	"runtime"
	"runtime/pprof"
	"testing"

	"github.com/migara0/docent/internal/render"
)

// DOCENT_MEM_DIR=<big folder> go test ./internal/library -run TestScaleMemory -v  prints the live heap after a scan (and writes a heap profile to DOCENT_MEM_PROFILE).
func TestScaleMemory(t *testing.T) {
	dir := os.Getenv("DOCENT_MEM_DIR")
	if dir == "" {
		t.Skip("DOCENT_MEM_DIR not set")
	}
	s := newSource("big", "Big", KindLibrary, "plain", dir, "", render.New(), nil)
	if _, err := s.Rescan(); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("pages=%d heapAlloc=%.1fMB heapInuse=%.1fMB heapSys=%.1fMB stackSys=%.1fMB sys=%.1fMB objects=%d", s.Info().Pages,
		float64(m.HeapAlloc)/1e6, float64(m.HeapInuse)/1e6, float64(m.HeapSys)/1e6, float64(m.StackSys)/1e6, float64(m.Sys)/1e6, m.HeapObjects)
	if p := os.Getenv("DOCENT_MEM_PROFILE"); p != "" {
		f, _ := os.Create(p)
		defer f.Close()
		_ = pprof.Lookup("heap").WriteTo(f, 0)
	}
}
