# Notices

Docent is released under the MIT licence; see [LICENSE](LICENSE). Copyright (c) 2026 migara0.

It includes and embeds third-party code and fonts under their own licences. All of them allow redistribution in a project under the MIT licence;
their copyright notices and licence texts are kept in [`licenses/`](licenses).

## Code copied into this repository

`internal/highlight` contains lexer definitions copied from [chroma](https://github.com/alecthomas/chroma) v2.27.0 (MIT licence, Copyright (C) 2017 Alec Thomas):
the XML files in `internal/highlight/xml/` and the Go lexers `go.go`, `http.go` and `markdown.go` (adapted: they register with Docent's own lazy registry, and the Go
lexer does not highlight Go templates inside raw strings). Chroma's licence text is in [`licenses/THIRD_PARTY_GO.md`](licenses/THIRD_PARTY_GO.md).

## Fonts

The interface embeds two fonts, both under the SIL Open Font License 1.1:

- **Inter** (Copyright 2016 The Inter Project Authors): [`licenses/Inter-OFL-1.1.txt`](licenses/Inter-OFL-1.1.txt)
- **JetBrains Mono** (Copyright 2020 The JetBrains Mono Project Authors): [`licenses/JetBrainsMono-OFL-1.1.txt`](licenses/JetBrainsMono-OFL-1.1.txt)

## Libraries compiled into the binary

- Go modules: [`licenses/THIRD_PARTY_GO.md`](licenses/THIRD_PARTY_GO.md) (MIT, BSD-3-Clause and Apache-2.0).
- npm packages in the web interface: [`licenses/THIRD_PARTY_WEB.md`](licenses/THIRD_PARTY_WEB.md) (mostly MIT and ISC; also BSD-3-Clause, Apache-2.0,
  DOMPurify under Apache-2.0 or MPL-2.0, and robust-predicates under the Unlicense).
- **elkjs** (the optional "elk" layout engine of Mermaid diagrams) is dual licensed EPL-2.0 or GPL-3.0-or-later. Docent uses it under the
  **Eclipse Public License 2.0**, unmodified. Its source code is available at <https://github.com/kieler/elkjs> (and the Java original at
  <https://github.com/eclipse-elk/elk>).

Both lists are generated: `make licenses` (the Go list) and `pnpm build` in `web/` (the npm list, from the modules that were actually bundled).
