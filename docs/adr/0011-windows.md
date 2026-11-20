# 0011. Windows support

Status: accepted (2026-11-20)

## Context
Docent was built and tested on Linux only (0002). The code mostly used portable Go (`filepath`, `os.UserConfigDir`, fsnotify), so Windows was meant to be a
build-and-test task. Building for Windows and running the test suite as Windows programs (under Wine locally, on `windows-latest` in CI) found what was left.

## Decisions
- **ID allocation lock.** `syscall.Flock` does not exist on Windows. `lockFile` (`internal/devnotes/lock_unix.go`, `lock_windows.go`) uses `flock` on Unix and
  `LockFileEx` on Windows, so two processes still can never hand out the same note ID.
- **Creation time.** NTFS records when a file was created (`born_windows.go`), so created dates get their time of day as on Linux (statx). Other systems show the day only.
- **Line endings.** Windows editors save `\r\n`. Rendering already turned them into `\n`; the developer-notes commands now read notes through `ReadNote`, and the three
  commands that rewrite a note (plan board edits, commit-log refresh, ID backfill) write it back with the line endings it had, never a mix (`internal/devnotes/eol.go`).
- **Chrome.** PDF export and `docent check --diagrams` look for Chrome in its usual Windows and macOS install folders, and use Microsoft Edge (Chromium, always on Windows) last.
- **Paths in the browser.** Editor links turn `C:\x\a.md` into `vscode://file/C:/x/a.md`; the folder picker shows drive buttons, names a drive `C:` and uses `\` between folders.
- **Repository.** `.gitattributes` keeps `\n` in every checkout, because golden files and fixtures are compared byte for byte.
- **Packaging.** `make release` adds `windows/amd64` and `windows/arm64` zips with `docent.exe` (a console program: double-clicking it starts the app and opens the browser).
  The program is not code-signed, so SmartScreen warns on first run; signing needs a certificate and is left for later.
- **Testing.** CI runs vet, the Go tests and a start-up check (start, read a page, load the app) on Linux and Windows for every push to main and every pull request.
  The browser tests stay local (screenshots depend on the machine).
- **Double-click (added in 0.10.0-beta.3).** Cobra refuses to run a program started from Explorer by default ("This is a command line tool ..."), which made
  0.10.0-beta.2 close 5 seconds after a double-click; CI could not see it, because it starts the program from a shell. `cmd/docent/explorer_windows.go` turns
  that off, tells a double-clicked window to stay open while reading, and waits for Enter after an error. Reproduced and checked by starting docent.exe from
  a stand-in `explorer.exe` (cobra looks at the parent process's name).
- **Licences per system.** `scripts/go-licenses.sh` lists the modules of the Linux and the Windows build: `mousetrap` is only in the Windows one.

## Consequences
Windows users get the same features. Not covered by any test on real Windows yet: file watching while an editor holds a file open, and the desktop launcher
(Linux only: `make install` writes a `.desktop` entry; there is no Start menu entry or installer on Windows).
