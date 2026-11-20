# 0006. File-like API, static export, and hybrid search

Status: accepted (2026-11-10), revised after measurement (2026-11-10)

## Decision
The server exposes file-like JSON resources (`docs/architecture.md`). A static export writes the same resources to disk and the same UI reads them, so "export a site to share"
costs almost nothing.

**Search is hybrid, chosen by library size.** The first version assumed one compact index loaded by the browser (MiniSearch). Measured on a generated 10,000-page library
that index was **50 MB** to download and pushed the server to **360 MB**, so one mechanism cannot serve both a 50-page notes folder and a 10,000-page one:

| Library | Mechanism | Why |
| ------- | --------- | --- |
| Small (the index would be under 3 MB) | `/_api/search-index` returns the documents; the browser builds a MiniSearch index | Instant, typo tolerant, works offline and in static exports |
| Large | `/_api/search?q=` scans the files on the server for each query | Holds **nothing** in memory, never stale, no idle cost. A 4-core burst of about 0.1 to 0.2 s for 10,000 pages, cancelled when the user types another letter |

`/_api/search-index` answers `{"mode":"client","docs":[...]}` or `{"mode":"server"}`, so the UI picks the mechanism without knowing the size.
Server search ranks title matches first, then heading matches, then frequency; every word must be present; it returns a snippet.
