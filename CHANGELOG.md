# Changelog

## 0.10.0-beta.3 (2026-11-20)

- **Windows: double-clicking `docent.exe` now starts Docent.** 0.10.0-beta.2 showed "This is a command line tool. You need to open cmd.exe
  and run it from there." and closed after 5 seconds (a default of the command-line library Docent uses).
- When started by a double-click, the window says to keep it open while you read (closing it stops Docent), and after an error it waits for
  Enter instead of closing before the message can be read.
- The licence list now covers the Windows build too: it adds `mousetrap` (Apache-2.0), which only the Windows program contains.

## 0.10.0-beta.2 (2026-11-20)

- **Windows**: `docent.exe` for Windows (amd64 and arm64), as a zip. Double-click it to open Docent in your browser. The program is not
  signed yet, so SmartScreen asks once ("More info", then "Run anyway").
- Notes saved with Windows line endings (`\r\n`) work everywhere, and the commands that update a note (plan board, commit log, ID backfill)
  keep the note's line endings.
- PDF export and diagram checks find Chrome where Windows and macOS install it, and use Microsoft Edge when Chrome is missing.
- The folder picker shows the drives on Windows; "open in your editor" links work with Windows paths.
- Checks on every change: tests on Linux and Windows run on GitHub for each push and pull request.

## 0.10.0-beta.1 (2026-11-20)

The first public release, for testing. Linux (amd64 and arm64) only.

- **Reader**: open any folder or Markdown file, or register folders as libraries; page tree with filter, search (`Ctrl K`), contents list,
  backlinks, previous and next page, live updates when files change, print and save as PDF, dark and light themes, phone view by QR code.
- **Markdown**: CommonMark and GitHub Flavored Markdown, task lists, footnotes, alerts, syntax highlighting, Mermaid diagrams (enlarge,
  open in a window), wide tables (enlarge, copy as Markdown), code blocks with copy and wrap.
- **Dates**: created and updated dates with the time of day in your time zone, and a search filter for pages created on a day.
- **Developer notes profile** (optional): stable note IDs, templates, overview tables, plan boards, new-note markers, commit tracking,
  stale-note checks, lint, bundles.
- **Known gaps**: images inside Markdown are not drawn (their alt text is shown); no editing; no macOS or Windows builds.
