# 0009. Index design: immutable snapshots, slim pages, targeted refresh

Status: accepted (2026-11-10)

## Decision
- **Immutable snapshots.** The page index, tree, backlinks and reading order live in one snapshot that is never modified. A change builds a new snapshot and swaps it in atomically,
  so request handlers read without locks and never see a half-updated index. The race detector runs on the whole suite.
- **Slim pages.** The index keeps about 0.3 KB per page (title, a few well-known front matter fields, word count, the relative link targets). Headings, link text and the body are
  read from disk when a page is opened or searched. Only indexed files are ever served, which is also the path-confinement check.
- **Two ways to update.** A full scan (stat every file, re-read what differs) for the first scan and folder-level changes; a targeted refresh of just the changed files for edits.
  A content-only edit keeps the tree and backlinks; a retitle, an added or removed page or a changed link target rebuilds them.
- **Parallel analysis** with up to 8 workers, using a lighter parser (no URL auto-linking, no plain-text building) because analysis only needs structure.
- **Symbolic links are never followed**, hidden folders and `node_modules` are skipped, files over 8 MB and libraries over 200,000 pages are refused: an odd folder must not hurt the machine.
- **Garbage collector.** The server runs with `GOGC=40` to stay small while resident. Large scans (500 or more pages) relax it to 200 for their duration, then restore it and return memory to the OS.
- **File watching** uses the kernel's notifications (no polling). When the system's watch limit is reached it falls back to a 4-second poll.

## Consequences
A 10,000-page library is indexed in about one second, edits show up in about 70 ms, and an idle server uses no CPU. The cost is that opening a page re-reads and re-renders it (about 10 ms),
which also guarantees it is never stale.
