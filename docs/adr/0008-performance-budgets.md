# 0008. Performance budgets

Status: accepted (2026-11-10); first measurements 2026-11-10

Measured by `scripts/bench.sh`, which builds a synthetic library by repeating real Markdown (nothing private is copied into this repository). Linux, 12 cores.

| Measure | Budget | Measured | |
| ------- | ------ | -------- | - |
| Cold start to serving | under 150 ms | **22 to 28 ms** | met |
| Idle memory, no library | under 40 MB | **13.6 MB** (was 19 to 20 before M4) | met |
| CPU when idle | none | **0 ticks in 5 s** | met |
| Index 10,000 pages (about 100 MB of notes) | under 3 s | **0.9 to 1.0 s** for light notes; **2.6 to 3.2 s** for heavy real notes (19 to 22 CPU-seconds, over half of it goldmark's parser) on a machine under load | met, with little margin on heavy notes |
| Edit reaching the browser, 10,000 pages | under 100 ms | **66 to 69 ms** | met |
| Open one page | under 100 ms | **about 10 ms** | met |
| Binary size | under 25 MB | **16.2 MB** | met |
| Resident memory, 10,000 pages | under 40 MB | **34 to 36 MB** with heavy real notes (104 MB of Markdown); 24 to 25 MB of it private, the rest the executable's file-backed pages | met (was 45 to 50 MB before M4, see below) |
| First load of the UI | under 150 KB gzip | **139 KB** including both fonts (52 KB of it the app's own code) | met |

What measuring changed (each was a budget miss or a trap that a plan on paper would not have found):

- Search index for the browser: 50 MB and 360 MB of memory at 10,000 pages, so search became hybrid (0006).
- Per-page front matter as a `map[string]any` cost 1 KB a page; a struct of shared strings fixed it. Full link lists and headings are no longer held for every page.
- A tight garbage collector (`GOGC=40`) kept memory low but made scans 4 times slower. The collector is now relaxed only during large scans (see 0009).
- Every file event re-statted every file (371 ms at 10,000 pages). A targeted refresh of just the changed files brought it to 66 ms.
- A non-ASCII character anywhere in a page forced Unicode lower-casing of the whole file in search; ASCII search words now fold in place (3 times faster).

Remaining memory: the Go runtime and an empty server are already 19 MB; the live index for 10,000 pages is 13 MB; the rest is collector headroom and fragmentation.

## M4 memory work

Heavy real notes (a private notes collection repeated to 10,000 pages, 104 MB) took 42 to 50 MB resident. What was found:

- The live heap after a scan is only 12.5 MB; the idle process is 1.5 MB of heap. The rest was fragmentation, collector headroom and the executable's own pages.
- Chroma registered and parsed about 280 lexers at start-up whether or not a page used one. Replacing its `lexers` package with a curated, lazily parsed set (69 languages, `internal/highlight`)
  saved 5.6 MB at idle (1.9 private, 3.7 file-backed) and 2.9 MB of binary. Languages outside the set show as plain text; the highlighter tests compare token for token against chroma for the
  languages the real notes use.
- After a large scan the runtime now gets a soft memory limit of twice the live heap plus 4 MB (`capMemory`), which makes it return the memory searches and renders leave free (1 to 2 MB).
- Resident memory includes the executable's file-backed pages (9 to 11 MB). The kernel can drop those at any time, but they are counted here because that is what `ps` and the task manager show.

`internal/library/mem_test.go` (`DOCENT_MEM_DIR=<folder> go test ./internal/library -run TestScaleMemory -v`) prints the live heap and writes a heap profile for the next round.
