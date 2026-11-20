# Markdown dialect

CommonMark plus GitHub Flavored Markdown, plus a small set of extensions chosen from what real notes use. Measured on a private collection of 78 real
developer notes, so the list reflects actual need.

| Feature | Syntax | Used in the corpus | Status |
| ------- | ------ | ------------------ | ------ |
| Tables | GFM pipes | 4,913 rows, 74/78 notes | v1 |
| Fenced code, syntax highlighting | ```` ```lang ```` | 854 fences, 71/78 | v1 (server side, no client script) |
| Mermaid diagrams | ```` ```mermaid ```` | 131 diagrams, 57/78 | v1, bundled, loaded only on pages that have one |
| Task lists | `- [ ]` | 231 items, 18/78 | v1 |
| Relative links between pages | `[x](../other.md#anchor)` | 313 links, 59/78 | v1, with backlinks |
| Admonitions | `!!! note "Title"` and GitHub alerts `> [!NOTE]` | 111 uses, 55/78 | v1 (both syntaxes) |
| Collapsible blocks | `??? note "Title"` / `???+` | 30 uses, 5/78 | v1 |
| Attribute lists | `{ #id .class data-x }` | 35 uses, 16/78 | v1 (headings and blocks) |
| Definition lists | `Term` then `:   definition` | 80 uses, 5/78 | v1 |
| Footnotes | `[^1]` | 1 use | v1 |
| Front matter | YAML between `---` | 78/78 | v1, optional |
| Tabs, snippets, `==mark==`, keys, raw HTML blocks | | 0 uses | not planned for v1 |
| **Images** (`![alt](src)`) | | 0 uses | **out of scope** (decision: 0002). Rendered as their alt text with a note, never fetched |

Anything outside this list renders as the CommonMark default. The dialect is checked against the real notes by golden tests (the notes are referenced by path,
never copied into this repository because they are private).
