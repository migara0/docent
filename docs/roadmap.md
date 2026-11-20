# Roadmap

| # | What | Done when |
| - | ---- | --------- |
| **M0** | Decisions, scaffold, tokens, clickable prototype, fixtures | The owner approves the look (built; awaiting approval) |
| **M1** | Core: library registry (persistent) and ad hoc open of any folder or file, scanner and watcher, renderer, link graph, API, CLI | Real notes render with the expected structure; golden tests pass. **Done** (see ADR 0008 for measurements) |
| **M2** | Reader UI: tree, page, TOC, search palette, themes, reading controls, diagram viewer, print, mobile, folder picker | Parity with the current notes sites; budgets met. **Done**: 24 end-to-end tests, first load 139 KB. Developer-notes features (tags page, stale-module banner, overview tables, graphs) belong to M3 |
| **M3** | `devnotes` profile: safe IDs, front-matter validation, templates, backlinks, stale check, lint, commit tracking, bundles, overview pages | two real notes sites run as libraries; the old sites are retired. **Done** |
| **M4** | Hardening and release: end-to-end and visual tests, packaging, LAN mode, docs, 1.0 | Ready to hand to someone else. **Done** as 0.9.0 (see [0010](adr/0010-hardening-and-release.md)); calling it 1.0 is the owner's decision |
| **M5** | Plan notes: a status board per task (stages with steps, now, next, waiting on others), an "Active plans" list on the home page, `docent plan` commands, ticking steps in the page | One look shows where a task is. **Done** as 0.10.0 |
| Later | Editor, plugins, import (Obsidian, MkDocs), git, access control, other platforms, desktop shell | Owner's call |
