package main

import (
	"os"
	"runtime"
	"testing"

	"github.com/migara0/docent/internal/config"
)

// What the process holds before any library is open: everything the package initialisers built.
func TestIdleHeap(t *testing.T) {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("idle: heapAlloc=%.1fMB heapInuse=%.1fMB objects=%d sys=%.1fMB", float64(m.HeapAlloc)/1e6, float64(m.HeapInuse)/1e6, m.HeapObjects, float64(m.Sys)/1e6)
}

func TestRemoveInfoOnlyRemovesItsOwnFile(t *testing.T) {
	t.Setenv("DOCENT_HOME", t.TempDir())
	paths, err := config.ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	// a newer instance owns the file: this one must leave it alone
	if err := writeInfo(paths, serverInfo{URL: "http://127.0.0.1:1", PID: os.Getpid() + 1}); err != nil {
		t.Fatal(err)
	}
	removeInfo(paths)
	if _, err := os.Stat(paths.ServerInfo); err != nil {
		t.Fatalf("another instance's info file was removed: %v", err)
	}
	if err := writeInfo(paths, serverInfo{URL: "http://127.0.0.1:1", PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	removeInfo(paths)
	if _, err := os.Stat(paths.ServerInfo); err == nil {
		t.Fatal("its own info file should be removed")
	}
}
