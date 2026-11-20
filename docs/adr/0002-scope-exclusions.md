# 0002. Scope exclusions for v1

Status: accepted (2026-11-10)

## Decision
Out of scope for v1, by decision:
- **Images inside Markdown.** `![alt](src)` is not fetched or rendered; the alt text is shown with a small "image not shown" marker. Revisit after v1.
- **Editing.** v1 reads; "open in your editor" is provided. A built-in editor (CodeMirror, later a friendlier editor for non-developers) comes after v1.
- **Git integration and access control.** On hold at the owner's request. The server has one request hook where a check can be added later; the default is loopback only.
- **Platforms other than Linux.** The code avoids Linux-only assumptions (paths, file watching) so macOS and Windows are a later build-and-test task, not a rewrite.
  Windows was added on 2026-11-20 (see 0011); macOS is still open.
- **Licence and publication.** Decided on 2026-11-20: MIT, published on GitHub (`github.com/migara0/docent`), first public release 0.10.0-beta.1.
  Third-party licences are collected in `licenses/` (see `NOTICE.md`).

## Consequences
A smaller v1 that can be finished and tested properly. Each exclusion is listed in the roadmap as a possible later milestone.
