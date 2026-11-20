# 0003. Core in Go as one static binary

Status: accepted (2026-11-10)

## Context
Goals: fast, reliable, minimal impact on the PC, releasable to anyone, no runtime to install. Go 1.27 is installed (auto toolchain); Rust is not.

## Decision
Go, `CGO_ENABLED=0`, one executable with the UI embedded (`go:embed`). Standard library for HTTP (method and path routing), plus a few well-maintained libraries:
goldmark (Markdown), chroma (code colours), fsnotify (file watching), a YAML library (front matter), bluemonday (HTML sanitising), cobra (CLI).

## Alternatives
- **Rust:** slightly lower memory, much slower compile cycles, smaller contributor pool, not installed. Not needed for the budgets in 0008.
- **Python / Node:** need a runtime on the user's machine, or ship 60 to 100 MB binaries.
- **Electron / Tauri shell:** a window around the same app is possible later; the browser is the v1 front end (no extra process).
- **Hugo / Zola / mdBook:** static only, no registry, no folder picker, no live model.
