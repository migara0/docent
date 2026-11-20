package library

import (
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
)

// The server keeps the garbage collector tight so a resident library stays small, but a collector that tight makes a large scan about four times slower
// (measured: 5.6 s instead of 1.4 s for 10,000 pages, to save 12 MB of peak). So large scans relax it for their duration, then restore it and give memory back.
var (
	gcMu    sync.Mutex
	gcUsers int
	gcSaved int
)

const bigScanPages = 500
const scanGCPercent = 200

// relaxGC loosens the collector until the returned function is called. It is safe to nest and to call from several scans at once.
func relaxGC() (restore func()) {
	gcMu.Lock()
	if gcUsers == 0 {
		gcSaved = debug.SetGCPercent(scanGCPercent)
	}
	gcUsers++
	gcMu.Unlock()
	return func() {
		gcMu.Lock()
		gcUsers--
		last := gcUsers == 0
		if last {
			debug.SetGCPercent(gcSaved)
		}
		gcMu.Unlock()
		if last {
			debug.FreeOSMemory() // hand back the garbage a big scan leaves, instead of holding it for the life of the process
			capMemory()
		}
	}
}

// capMemory sets a soft limit just above what the live library needs. Between scans nothing allocates much, so the limit costs nothing; it makes the runtime
// return memory that searches and page renders leave free instead of keeping it (a measured 49 MB resident for 10,000 heavy pages came down by about a quarter).
func capMemory() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	limit := int64(m.HeapAlloc)*2 + 4<<20
	if v, err := strconv.Atoi(os.Getenv("DOCENT_MEMLIMIT_MB")); err == nil { // experiments and the benchmark
		limit = int64(v) << 20
	}
	debug.SetMemoryLimit(limit)
}
