# 0001. Product shape: sources, libraries, profiles

Status: accepted (2026-11-10)

## Context
The tool started as a team's developer-notes sites (two MkDocs copies, kept in sync by hand). It should become one app that anyone can use on any folder of Markdown,
including people who are not developers.

## Decision
- The product is **Docent**, a local-first reader. One app, many folders.
- Any folder or single `.md` file can be **opened** (temporary). A folder can be **registered** as a **library**, which persists across runs.
- Conventions are optional **profiles** per library. `plain` is the default and needs nothing. `devnotes` carries the features of those sites (IDs, templates, types, lint, commit tracking, stale checks, bundles).
- Each notes site becomes a library with the `devnotes` profile. IDs resolve across libraries (`DOC-0003` can be linked from a note of another library).
- Docent never writes into a folder it opens. Its state lives in `~/.config/docent` and `~/.cache/docent`.
  The one exception is the `devnotes` profile, and only when you run a command that says so: `docent new` creates a note, `docent commits add|refresh` updates a commits note,
  `docent id backfill --apply` adds missing IDs. Reading, serving, checking, linting and bundling never write into the notes folder.

## Consequences
The core stays generic; team-specific conventions live behind the profile boundary. Non-developers get a folder picker and no setup; developers get a CLI.
Rejected: one app per workspace (the current state, hand-synced copies); a hosted service (local-first is the goal).
