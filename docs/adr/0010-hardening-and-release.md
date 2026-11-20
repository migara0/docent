# 0010. Hardening and release (M4)

Status: accepted (2026-11-10)

## Context
M0 to M3 built the app. M4 makes it something that can be handed to someone else: it has to stay inside its budgets on real data, resist hostile input, be accessible, keep its
look stable, and install in one step.

## Decisions
- **Memory (see 0008).** Replace chroma's eager `lexers` package with `internal/highlight` (curated 69 languages, parsed on first use) and cap the runtime's memory after large scans.
  10,000 heavy pages went from 42 to 50 MB to 34 to 36 MB resident; idle from 20.8 to 13.6 MB; the binary from 19.0 to 16.2 MB. The price: languages outside the set show as plain text.
- **Robustness.** Fuzz targets for the Markdown renderer (the sanitiser invariant: no dangerous tag, event handler or script URL ever reaches the HTML), the commit-block parser, and the ID and
  path parsers. `go test` runs their seed inputs every time; `go test -fuzz` explores further (280,000 renderer inputs and 560,000 parser inputs ran clean). `govulncheck` reports no known vulnerabilities.
  The server also limits request headers (32 KB) and read time (30 s), on top of the existing 64 KB body limit, Host and Origin checks, and path confinement.
- **Accessibility is a test, not a promise.** `web/e2e/a11y.spec.ts` runs axe-core (WCAG 2.1 A and AA) over the main views in both themes and fails on any serious or critical finding.
  Its first run found real problems, all fixed: faint text, syntax colours and a few labels below the 4.5:1 contrast ratio (the design tokens changed), heading anchors reachable by Tab,
  unlabelled task-list checkboxes, scrolling code blocks that the keyboard could not reach, and A+/A- buttons whose names did not contain their visible text.
- **Visual regression.** `web/e2e/visual.spec.ts` compares 40 screenshots (11 views, desktop and mobile, light and dark) with baselines kept in the repository. Anything time-relative is masked.
  Baselines are for Linux with the bundled fonts, which is the only platform for now.
- **Packaging.** `make release` builds static `linux/amd64` and `linux/arm64` archives with checksums; `make install` also adds a launcher entry and icon. An optional systemd user unit
  runs it in the background with a memory ceiling and a read-only view of the home directory. The version comes from the `VERSION` file.

## Consequences
Design changes that alter pixels need `--update-snapshots` and a look at the new images. Adding a language to the highlighter means copying its XML from chroma into `internal/highlight/xml/`
(and nothing else; the alias table is built from the files). Windows and macOS builds, signing and an auto-updater are deliberately not part of this milestone.

## Addendum: dropping files and sharing with a phone (2026-11-10)
- **Dropped files** are uploaded as temporary copies (`POST /_api/drop`, `internal/server/drop.go`), because a browser never reveals where a dropped file lives. They live in a folder of their own under
  `~/.cache/docent/drops/`, are opened as the `dropped` source (never remembered, never in recents), and are deleted on exit; a crash's leftovers are deleted at the next start. File names must be plain
  `.md`/`.markdown` names, so nothing can be written outside the folder. Folders cannot be dropped (they would need relative paths, which is a larger attack surface than this feature is worth).
- **Share with a phone** (`internal/server/share.go`) starts a second listener on the local network only when the user presses the button, serves the same libraries with the LAN-mode rules (read-only; no drop, no folder picker,
  no way to start or stop sharing), stops after a chosen limit (15 minutes to 4 hours) or when Docent exits, and lists only addresses on real networks (container and VM bridges are skipped). The QR code is drawn on the server
  with `rsc.io/qr` (BSD-3) as black on white SVG, only for an address of the running share. Tests decode the generated code back to the URL with OpenCV during development; the end-to-end test checks it loads.
- The visual-regression tolerance was tightened from 1% of pixels to 60 pixels, because the old setting did not notice a new toolbar icon.

## Addendum: finding pages in long folders (2026-11-11)
The page tree's narrow column made look-alike titles ("Payments block 5: users", "Payments block 6: reconcile") indistinguishable. Three changes: titles show from where they differ and keep their last words visible
(`web/src/lib/treenav.ts`), a filter box narrows the tree by title, folder or note ID, and the column can be dragged wider. Then a standalone **Browse** popup (`Navigator.svelte`, logic in `lib/navigator.ts`):
Finder-style columns over the libraries already loaded in the browser (no new API; the tree now carries note IDs), a preview that fetches the selected page (cut to its first blocks), keyboard-first
(the focus stays in the search box, as in the command palette), and one column at a time on a phone. It browses the content of libraries; the "Open folder or file" dialog still browses the disk.
Visual-regression tolerance is 60 pixels, so each new control means refreshing baselines on purpose.
