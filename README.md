# Docent

A fast, local reader for folders of Markdown. Point it at a folder (or a single file) and it shows everything as a clean, searchable, linked
documentation site in your browser. No build step, no config, no front matter required. Folders you register are remembered for the next run.

> **Status: beta (0.10.0-beta.3), Linux and Windows.** It is used every day on real notes collections, but it is new in public:
> expect rough edges, and please [open an issue](https://github.com/migara0/docent/issues) when something is wrong.
>
> **Not there yet:** images inside Markdown are shown as their alt text, not drawn; Docent reads but does not edit (it opens your editor);
> no macOS build yet.

## What it does

- **Reads plain Markdown**: CommonMark and GitHub Flavored Markdown, tables, task lists, footnotes, GitHub-style alerts, syntax highlighting,
  Mermaid diagrams, and links between pages (broken links are marked).
- **Finds things quickly**: search titles and text of every page (`Ctrl K`), filter the page tree, find pages created on a given day.
- **Reads comfortably**: a contents list, wide tables that can be enlarged or copied as Markdown, code blocks with copy and wrap buttons,
  dark and light themes, text size, focus mode, print and save as PDF.
- **Stays live**: pages update as you edit them in your own editor.
- **Phone friendly**: show a QR code and read the same pages on your phone over your local network (off by default).
- **Developer notes (optional)**: a profile for teams that keep engineering notes: stable note IDs (`DOC-0007`) that never change or get reused,
  note templates, overview tables, plan boards ("where is this task, what is next"), commit tracking for cherry-picking releases,
  checks for stale notes, and bundles of notes as one Markdown or PDF file.

## Install

Download the archive for your machine from the [releases page](https://github.com/migara0/docent/releases): `amd64` for most PCs,
`arm64` for ARM machines (Raspberry Pi, Snapdragon laptops).

**Linux**

```bash
tar xzf docent-0.10.0-beta.3-linux-amd64.tar.gz
install -Dm755 docent-0.10.0-beta.3-linux-amd64/docent ~/.local/bin/docent
```

Check the download against `SHA256SUMS` from the same release with `sha256sum -c SHA256SUMS --ignore-missing`.

**Windows**

1. Extract `docent-0.10.0-beta.3-windows-amd64.zip` to a folder of your choice, for example `%LOCALAPPDATA%\Programs\Docent`.
2. In the extracted `docent-0.10.0-beta.3-windows-amd64` folder, double-click `docent.exe`: it opens Docent in your browser. Its console window shows what it is doing; closing that window stops Docent.
   To use the commands below in a terminal, add the folder to your `Path`.
3. The program is not signed yet, so Windows SmartScreen may say "Windows protected your PC" the first time: choose **More info**, then **Run anyway**.

To check the download, compare `Get-FileHash docent-0.10.0-beta.3-windows-amd64.zip` (PowerShell) with its line in `SHA256SUMS`.
PDF export and diagram checks use Google Chrome, or Microsoft Edge when Chrome is not installed.

Or build it with Go (1.21 or newer fetches the Go version it needs):

```bash
go install github.com/migara0/docent/cmd/docent@latest
```

## Use

```bash
docent add ~/notes                  # remember this folder as a library
docent open ~/Downloads/readme.md   # open any folder or file once, without registering it
docent serve                        # open the app in your browser (http://127.0.0.1:8787)
```

Press `?` in the app for the guide (also in [internal/help/guide.md](internal/help/guide.md)).

Developer-notes libraries (`docent add <folder> --profile devnotes`) also get these commands:

```bash
docent new <service> <type> "<title>"     # a note from the template, with a fresh ID that is never reused
docent id resolve|open|find|list|next|check|backfill|guides
docent commits add|refresh|pick|list      # record a feature's commits, print cherry-pick plans
docent plan show|list|start|done|wait ... # the status board of a plan note: stages, steps, now, next, waiting on others
docent lint | docent stale | docent bundle <note>... --out pack.md|pack.pdf
docent check <library> --diagrams         # draw every Mermaid diagram in headless Chrome
```

## Principles

- **One small binary.** No runtime to install. A single Go executable with the UI embedded.
- **Light on the machine.** Idle memory under 40 MB, no CPU when idle, no background polling.
- **Your folders stay yours.** Docent never writes into a folder you open. Its own state lives in your OS config and cache folders.
  (The developer-notes commands that create or update notes are the one exception, and only when you run them.)
- **Works with plain Markdown.** No front matter, config or build step needed. Conventions (note IDs, templates, ...) are optional *profiles*.
- **Local by default.** Listens on `127.0.0.1` only. No telemetry, no accounts, nothing sent anywhere.

## Build from source

You need Go and [pnpm](https://pnpm.io) (Node.js 20.19+ or 22.12+). The browser tests also need Google Chrome.

```bash
make build        # the UI, then the Go binary into ./bin/docent (the UI is embedded)
make test         # go vet, Go tests with the race detector, UI type check and unit tests
make e2e          # build, then the Playwright suite against the real binary (needs Chrome)
make dev          # UI development with hot reload; run `docent serve --no-open --addr 127.0.0.1:8787` alongside
make bench CORPUS=~/notes   # measure against the performance budgets
make install      # ~/.local/bin/docent, a launcher entry and an icon (PREFIX=/usr/local to change it)
make release      # dist/: Linux (.tar.gz) and Windows (.zip) archives for amd64 and arm64, and SHA256SUMS (needs zip)
```

The built UI is kept in `internal/ui/dist` (the Go binary embeds it), so a plain `go build ./cmd/docent` works without Node.

```text
cmd/docent/     the program (CLI + server)
internal/       Go packages (library registry, scanner, renderer, search, server, profiles)
web/            the UI: Svelte 5 + TypeScript + Vite, hand-written CSS from design tokens
design/         design tokens, component CSS and the clickable prototype
docs/           architecture, Markdown dialect, roadmap, decision records (docs/adr)
testdata/       small synthetic fixtures
licenses/       licences of the fonts and of the third-party code compiled in
```

Versions and why they are pinned: [docs/versions.md](docs/versions.md). How it works: [docs/architecture.md](docs/architecture.md).

## Licence

MIT, see [LICENSE](LICENSE). Docent includes third-party code and fonts under their own licences: see [NOTICE.md](NOTICE.md).
