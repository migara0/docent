# 0005. Markdown pipeline and dialect

Status: accepted (2026-11-10)

## Decision
CommonMark + GFM via goldmark, with extensions for the features measured in the real notes (see `docs/dialect.md`): admonitions (`!!!` and GitHub alerts), collapsible
`???` blocks, attribute lists, definition lists, footnotes, task lists, Mermaid fences, front matter. Code is coloured on the server with chroma (CSS classes, no client script).
Rendered HTML is sanitised. Images are out of scope (0002).

## Consequences
The renderer is tested with golden files built from a small public fixture set and, when available, a private collection of real notes referenced by path (`DOCENT_CORPUS`).
Syntax outside the dialect renders as plain CommonMark rather than failing.
